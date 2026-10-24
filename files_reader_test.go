package scanii_test

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/scanii/scanii-go/v2"
)

// These tests run against net/http/httptest servers, not scanii-cli, because
// they assert on what arrives on the wire and on how the upload behaves when
// the server misbehaves.

const readerTestTimeout = 10 * time.Second

func newReaderTestClient(t *testing.T, srv *httptest.Server) *scanii.Client {
	t.Helper()
	c, err := scanii.NewClient(scanii.ClientOpts{
		Target:     scanii.NewTarget(srv.URL),
		Key:        testKey,
		Secret:     testSecret,
		HTTPClient: srv.Client(),
	})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	return c
}

type receivedUpload struct {
	path             string
	contentLength    int64
	transferEncoding []string
	filename         string
	fileSHA256       [32]byte
	fileSize         int64
	fields           map[string]string
	partOrder        []string
}

// parseUpload reads a multipart upload the way the Scanii API does, hashing
// the file part as it streams by.
func parseUpload(t *testing.T, r *http.Request) receivedUpload {
	t.Helper()
	got := receivedUpload{
		path:             r.URL.Path,
		contentLength:    r.ContentLength,
		transferEncoding: r.TransferEncoding,
		fields:           map[string]string{},
	}
	mr, err := r.MultipartReader()
	if err != nil {
		t.Errorf("MultipartReader: %v", err)
		return got
	}
	for {
		part, err := mr.NextPart()
		if err == io.EOF {
			return got
		}
		if err != nil {
			t.Errorf("NextPart: %v", err)
			return got
		}
		got.partOrder = append(got.partOrder, part.FormName())
		if part.FormName() == "file" {
			got.filename = part.FileName()
			h := sha256.New()
			n, err := io.Copy(h, part)
			if err != nil {
				t.Errorf("reading file part: %v", err)
				return got
			}
			got.fileSize = n
			copy(got.fileSHA256[:], h.Sum(nil))
			continue
		}
		v, err := io.ReadAll(part)
		if err != nil {
			t.Errorf("reading field %q: %v", part.FormName(), err)
			return got
		}
		got.fields[part.FormName()] = string(v)
	}
}

func writeJSON(w http.ResponseWriter, status int, body string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = io.WriteString(w, body)
}

func randomBytes(t *testing.T, n int) []byte {
	t.Helper()
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		t.Fatalf("rand.Read: %v", err)
	}
	return b
}

// infiniteReader never reaches EOF and counts its reads, so a test can tell
// whether anything is still consuming it after an upload returns.
type infiniteReader struct {
	reads atomic.Int64
}

func (r *infiniteReader) Read(p []byte) (int, error) {
	r.reads.Add(1)
	// Pace the stream so an abandoned upload cannot spin a core.
	time.Sleep(time.Millisecond)
	for i := range p {
		p[i] = 'x'
	}
	return len(p), nil
}

// assertAbandoned fails when r is still being read once the upload returned,
// which would mean the multipart writer goroutine outlived the call.
func assertAbandoned(t *testing.T, r *infiniteReader) {
	t.Helper()
	before := r.reads.Load()
	time.Sleep(200 * time.Millisecond)
	if after := r.reads.Load(); after != before {
		t.Fatalf("reader still being consumed after the call returned: %d reads, then %d", before, after)
	}
}

func TestProcessReaderDeliversContentMetadataAndCallback(t *testing.T) {
	content := randomBytes(t, 3<<20+17)
	uploads := make(chan receivedUpload, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		uploads <- parseUpload(t, r)
		writeJSON(w, http.StatusCreated, `{"id":"abc123","checksum":"c","content_length":3145745,"findings":[],"content_type":"application/octet-stream","metadata":{"source":"unit-test","owner":"go"},"creation_date":"2026-01-02T03:04:05Z"}`)
	}))
	defer srv.Close()
	c := newReaderTestClient(t, srv)

	metadata := map[string]string{"source": "unit-test", "owner": "go"}
	result, err := c.ProcessReader(context.Background(), bytes.NewReader(content), "report.pdf", metadata, "https://example.com/callback")
	if err != nil {
		t.Fatalf("ProcessReader: %v", err)
	}
	got := <-uploads

	if got.path != "/v2.2/files" {
		t.Errorf("path = %q, want /v2.2/files", got.path)
	}
	if got.contentLength != -1 || !reflect.DeepEqual(got.transferEncoding, []string{"chunked"}) {
		t.Errorf("expected a chunked body with unknown length, got ContentLength=%d TransferEncoding=%v", got.contentLength, got.transferEncoding)
	}
	if got.filename != "report.pdf" {
		t.Errorf("filename = %q, want report.pdf", got.filename)
	}
	if got.fileSize != int64(len(content)) || got.fileSHA256 != sha256.Sum256(content) {
		t.Errorf("file part differs from the source: got %d bytes", got.fileSize)
	}
	wantFields := map[string]string{
		"callback":         "https://example.com/callback",
		"metadata[source]": "unit-test",
		"metadata[owner]":  "go",
	}
	if !reflect.DeepEqual(got.fields, wantFields) {
		t.Errorf("fields = %v, want %v", got.fields, wantFields)
	}
	if len(got.partOrder) != 4 || got.partOrder[0] != "file" || got.partOrder[1] != "callback" {
		t.Errorf("part order = %v, want file, callback, then metadata fields", got.partOrder)
	}

	if result.ID != "abc123" || result.Metadata["source"] != "unit-test" {
		t.Errorf("unexpected result: %+v", result)
	}
}

func TestProcessReaderOmitsCallbackWhenEmpty(t *testing.T) {
	uploads := make(chan receivedUpload, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		uploads <- parseUpload(t, r)
		writeJSON(w, http.StatusCreated, `{"id":"abc"}`)
	}))
	defer srv.Close()
	c := newReaderTestClient(t, srv)

	if _, err := c.ProcessReader(context.Background(), strings.NewReader("hello"), "hello.txt", nil, ""); err != nil {
		t.Fatalf("ProcessReader: %v", err)
	}
	got := <-uploads
	if !reflect.DeepEqual(got.partOrder, []string{"file"}) {
		t.Errorf("part order = %v, want only the file part", got.partOrder)
	}
}

// TestProcessReaderDecodesResponsesLikeProcess pins the decoding contract:
// the same 201 body must produce the same ProcessingResult whichever upload
// method sent the file, including bodies that carry an error field.
func TestProcessReaderDecodesResponsesLikeProcess(t *testing.T) {
	bodies := []string{
		`{"id":"clean","checksum":"abc","content_length":5,"findings":[],"content_type":"text/plain","metadata":{"k":"v"},"creation_date":"2026-01-02T03:04:05Z"}`,
		`{"id":"infected","findings":["content.malicious.local-test-file"],"creation_date":"2026-01-02T03:04:05.123Z"}`,
		`{"id":"failed","error":"Could not process the file","metadata":{"k":"v"}}`,
	}
	for _, body := range bodies {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = io.Copy(io.Discard, r.Body)
			writeJSON(w, http.StatusCreated, body)
		}))
		c := newReaderTestClient(t, srv)
		path := filepath.Join(t.TempDir(), "hello.txt")
		if err := os.WriteFile(path, []byte("hello"), 0o600); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}

		fromPath, pathErr := c.Process(context.Background(), path, nil, "")
		fromReader, readerErr := c.ProcessReader(context.Background(), strings.NewReader("hello"), "hello.txt", nil, "")
		srv.Close()

		if pathErr != nil || readerErr != nil {
			t.Fatalf("body %s: Process err=%v, ProcessReader err=%v", body, pathErr, readerErr)
		}
		if !reflect.DeepEqual(fromPath, fromReader) {
			t.Errorf("body %s decoded differently:\nProcess:       %+v\nProcessReader: %+v", body, fromPath, fromReader)
		}
	}
}

// TestProcessStillSendsContentLength guards the path-based methods: they keep
// buffering the body and sending its length, unlike the reader methods.
func TestProcessStillSendsContentLength(t *testing.T) {
	uploads := make(chan receivedUpload, 2)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		uploads <- parseUpload(t, r)
		if strings.HasSuffix(r.URL.Path, "/async") {
			writeJSON(w, http.StatusAccepted, `{"id":"pending"}`)
			return
		}
		writeJSON(w, http.StatusCreated, `{"id":"done"}`)
	}))
	defer srv.Close()
	c := newReaderTestClient(t, srv)
	path := filepath.Join(t.TempDir(), "hello.txt")
	if err := os.WriteFile(path, []byte("hello"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	if _, err := c.Process(context.Background(), path, map[string]string{"k": "v"}, "https://example.com/cb"); err != nil {
		t.Fatalf("Process: %v", err)
	}
	if _, err := c.ProcessAsync(context.Background(), path, nil, ""); err != nil {
		t.Fatalf("ProcessAsync: %v", err)
	}
	for _, name := range []string{"Process", "ProcessAsync"} {
		got := <-uploads
		if got.contentLength <= 0 || len(got.transferEncoding) != 0 {
			t.Errorf("%s: ContentLength=%d TransferEncoding=%v, want a fixed length", name, got.contentLength, got.transferEncoding)
		}
		if got.filename != "hello.txt" || got.fileSize != 5 {
			t.Errorf("%s: file part = %q (%d bytes), want hello.txt (5 bytes)", name, got.filename, got.fileSize)
		}
	}
}

// gatedReader returns first, then blocks until released before returning
// rest. If the SDK read the whole body before sending, the server would never
// see first and the release would never come.
type gatedReader struct {
	first, rest []byte
	release     <-chan struct{}
	stage       int
}

var errBodyBuffered = errors.New("server never saw the first chunk: the body was buffered before sending")

func (g *gatedReader) Read(p []byte) (int, error) {
	switch g.stage {
	case 0:
		n := copy(p, g.first)
		g.first = g.first[n:]
		if len(g.first) == 0 {
			g.stage = 1
		}
		return n, nil
	case 1:
		select {
		case <-g.release:
		case <-time.After(readerTestTimeout):
			return 0, errBodyBuffered
		}
		g.stage = 2
		fallthrough
	default:
		if len(g.rest) == 0 {
			return 0, io.EOF
		}
		n := copy(p, g.rest)
		g.rest = g.rest[n:]
		return n, nil
	}
}

func TestProcessReaderStreamsBodyWhileReading(t *testing.T) {
	first := randomBytes(t, 64<<10)
	rest := randomBytes(t, 1<<20)
	release := make(chan struct{})
	uploads := make(chan receivedUpload, 1)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, params, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
		if err != nil {
			t.Errorf("ParseMediaType: %v", err)
			return
		}
		part, err := multipart.NewReader(r.Body, params["boundary"]).NextPart()
		if err != nil {
			t.Errorf("NextPart: %v", err)
			return
		}
		// Read less than the first chunk: the multipart reader holds back a
		// tail that might be the start of a boundary until more data arrives.
		head := make([]byte, 1024)
		if _, err := io.ReadFull(part, head); err != nil {
			t.Errorf("reading head of file part: %v", err)
			return
		}
		if !bytes.Equal(head, first[:len(head)]) {
			t.Errorf("head of file part differs from the source")
		}
		close(release)

		h := sha256.New()
		h.Write(head)
		n, err := io.Copy(h, part)
		if err != nil {
			t.Errorf("reading rest of file part: %v", err)
			return
		}
		got := receivedUpload{fileSize: n + int64(len(head))}
		copy(got.fileSHA256[:], h.Sum(nil))
		uploads <- got
		writeJSON(w, http.StatusCreated, `{"id":"streamed"}`)
	}))
	defer srv.Close()
	c := newReaderTestClient(t, srv)

	r := &gatedReader{first: first, rest: rest, release: release}
	result, err := c.ProcessReader(context.Background(), r, "big.bin", nil, "")
	if err != nil {
		t.Fatalf("ProcessReader: %v", err)
	}
	if result.ID != "streamed" {
		t.Errorf("ID = %q, want streamed", result.ID)
	}
	got := <-uploads
	want := sha256.Sum256(append(append([]byte{}, first...), rest...))
	if got.fileSize != int64(len(first)+len(rest)) || got.fileSHA256 != want {
		t.Errorf("file part differs from the source: got %d bytes", got.fileSize)
	}
}

type failingReader struct {
	prefix []byte
	err    error
}

func (f *failingReader) Read(p []byte) (int, error) {
	if len(f.prefix) == 0 {
		return 0, f.err
	}
	n := copy(p, f.prefix)
	f.prefix = f.prefix[n:]
	return n, nil
}

func TestProcessReaderReturnsReaderError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := io.Copy(io.Discard, r.Body); err != nil {
			return
		}
		writeJSON(w, http.StatusCreated, `{"id":"should-not-succeed"}`)
	}))
	defer srv.Close()
	c := newReaderTestClient(t, srv)

	errDisk := errors.New("disk on fire")
	r := &failingReader{prefix: randomBytes(t, 256<<10), err: errDisk}
	result, err := c.ProcessReader(context.Background(), r, "broken.bin", nil, "")
	if !errors.Is(err, errDisk) {
		t.Fatalf("err = %v, want one wrapping %v", err, errDisk)
	}
	if result != nil {
		t.Errorf("result = %+v, want nil", result)
	}
}

func TestProcessReaderAbortsWhenContextCanceled(t *testing.T) {
	started := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf := make([]byte, 1024)
		if _, err := io.ReadFull(r.Body, buf); err != nil {
			return
		}
		close(started)
		// Keep consuming until the client goes away.
		_, _ = io.Copy(io.Discard, r.Body)
	}))
	defer srv.Close()
	c := newReaderTestClient(t, srv)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		select {
		case <-started:
			cancel()
		case <-time.After(readerTestTimeout):
		}
	}()

	r := &infiniteReader{}
	type outcome struct {
		result *scanii.ProcessingResult
		err    error
	}
	outcomes := make(chan outcome, 1)
	go func() {
		result, err := c.ProcessReader(ctx, r, "endless.bin", nil, "")
		outcomes <- outcome{result, err}
	}()

	select {
	case o := <-outcomes:
		if !errors.Is(o.err, context.Canceled) {
			t.Fatalf("err = %v, want context.Canceled", o.err)
		}
		if o.result != nil {
			t.Errorf("result = %+v, want nil", o.result)
		}
	case <-time.After(readerTestTimeout):
		t.Fatal("ProcessReader did not return after the context was canceled")
	}
	assertAbandoned(t, r)
}

func TestProcessReaderReturnsErrorWhenServerRespondsBeforeReadingBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusUnauthorized, `{"error":"bad credentials"}`)
	}))
	defer srv.Close()
	c := newReaderTestClient(t, srv)

	r := &infiniteReader{}
	type outcome struct {
		result *scanii.ProcessingResult
		err    error
	}
	outcomes := make(chan outcome, 1)
	go func() {
		result, err := c.ProcessReader(context.Background(), r, "endless.bin", nil, "")
		outcomes <- outcome{result, err}
	}()

	select {
	case o := <-outcomes:
		var apiErr *scanii.Error
		if !errors.As(o.err, &apiErr) || apiErr.StatusCode != http.StatusUnauthorized || apiErr.Message != "bad credentials" {
			t.Fatalf("err = %v, want *scanii.Error with status 401", o.err)
		}
		if o.result != nil {
			t.Errorf("result = %+v, want nil", o.result)
		}
	case <-time.After(readerTestTimeout):
		t.Fatal("ProcessReader did not return after the server answered")
	}
	assertAbandoned(t, r)
}

func TestProcessAsyncReaderReturnsPendingID(t *testing.T) {
	content := randomBytes(t, 200<<10)
	uploads := make(chan receivedUpload, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		uploads <- parseUpload(t, r)
		writeJSON(w, http.StatusAccepted, `{"id":"pending-42"}`)
	}))
	defer srv.Close()
	c := newReaderTestClient(t, srv)

	pending, err := c.ProcessAsyncReader(context.Background(), bytes.NewReader(content), "async.bin", map[string]string{"k": "v"}, "")
	if err != nil {
		t.Fatalf("ProcessAsyncReader: %v", err)
	}
	if pending.ID != "pending-42" {
		t.Errorf("ID = %q, want pending-42", pending.ID)
	}
	got := <-uploads
	if got.path != "/v2.2/files/async" {
		t.Errorf("path = %q, want /v2.2/files/async", got.path)
	}
	if got.fileSHA256 != sha256.Sum256(content) || got.fields["metadata[k]"] != "v" {
		t.Errorf("upload differs from the source: %d bytes, fields %v", got.fileSize, got.fields)
	}
}

func TestProcessAsyncReaderRejectsUnexpectedStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		writeJSON(w, http.StatusCreated, `{"id":"sync"}`)
	}))
	defer srv.Close()
	c := newReaderTestClient(t, srv)

	_, err := c.ProcessAsyncReader(context.Background(), strings.NewReader("hello"), "hello.txt", nil, "")
	var apiErr *scanii.Error
	if !errors.As(err, &apiErr) || apiErr.StatusCode != http.StatusCreated {
		t.Fatalf("err = %v, want *scanii.Error with status 201", err)
	}
}

func TestReaderMethodsRejectInvalidArgumentsWithoutRequest(t *testing.T) {
	var requests atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		writeJSON(w, http.StatusCreated, `{"id":"x"}`)
	}))
	defer srv.Close()
	c := newReaderTestClient(t, srv)
	ctx := context.Background()

	if _, err := c.ProcessReader(ctx, nil, "file.txt", nil, ""); err == nil {
		t.Error("ProcessReader accepted a nil reader")
	}
	if _, err := c.ProcessReader(ctx, strings.NewReader("x"), "", nil, ""); err == nil {
		t.Error("ProcessReader accepted an empty filename")
	}
	if _, err := c.ProcessAsyncReader(ctx, nil, "file.txt", nil, ""); err == nil {
		t.Error("ProcessAsyncReader accepted a nil reader")
	}
	if _, err := c.ProcessAsyncReader(ctx, strings.NewReader("x"), "", nil, ""); err == nil {
		t.Error("ProcessAsyncReader accepted an empty filename")
	}
	if n := requests.Load(); n != 0 {
		t.Errorf("server received %d requests, want 0", n)
	}
}

// newHTTP2ReaderTestClient returns a client talking HTTP/2 over TLS, which is
// what the default client negotiates with the real API. The plain-HTTP servers
// above only ever exercise HTTP/1.1.
func newHTTP2ReaderTestClient(t *testing.T, handler http.Handler) *scanii.Client {
	t.Helper()
	srv := httptest.NewUnstartedServer(handler)
	srv.EnableHTTP2 = true
	srv.StartTLS()
	t.Cleanup(srv.Close)
	return newReaderTestClient(t, srv)
}

func TestProcessReaderOverHTTP2SendsNoLengthAndNoTransferEncoding(t *testing.T) {
	content := randomBytes(t, 1<<20+3)
	uploads := make(chan receivedUpload, 1)
	protos := make(chan string, 1)
	c := newHTTP2ReaderTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		protos <- r.Proto
		uploads <- parseUpload(t, r)
		writeJSON(w, http.StatusCreated, `{"id":"h2"}`)
	}))

	result, err := c.ProcessReader(context.Background(), bytes.NewReader(content), "h2.bin", nil, "")
	if err != nil {
		t.Fatalf("ProcessReader: %v", err)
	}
	if result.ID != "h2" {
		t.Errorf("ID = %q, want h2", result.ID)
	}

	if proto := <-protos; proto != "HTTP/2.0" {
		t.Fatalf("request used %s, want HTTP/2.0; the rest of this test is meaningless otherwise", proto)
	}
	got := <-uploads
	// HTTP/2 has no Transfer-Encoding header, so the docs must not promise
	// "chunked" there: all a caller can rely on is that there is no length.
	if got.contentLength != -1 || len(got.transferEncoding) != 0 {
		t.Errorf("ContentLength=%d TransferEncoding=%v, want -1 and none", got.contentLength, got.transferEncoding)
	}
	if got.fileSize != int64(len(content)) || got.fileSHA256 != sha256.Sum256(content) {
		t.Errorf("file part has %d bytes with the wrong hash, want %d bytes", got.fileSize, len(content))
	}
}

func TestProcessReaderOverHTTP2ReturnsReaderError(t *testing.T) {
	c := newHTTP2ReaderTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := io.Copy(io.Discard, r.Body); err != nil {
			return
		}
		writeJSON(w, http.StatusCreated, `{"id":"should-not-succeed"}`)
	}))

	errDisk := errors.New("disk on fire")
	r := &failingReader{prefix: randomBytes(t, 256<<10), err: errDisk}
	result, err := c.ProcessReader(context.Background(), r, "broken.bin", nil, "")
	if !errors.Is(err, errDisk) {
		t.Fatalf("err = %v, want one wrapping %v", err, errDisk)
	}
	if result != nil {
		t.Errorf("result = %+v, want nil", result)
	}
}

func TestProcessReaderOverHTTP2ReturnsErrorWhenServerRespondsBeforeReadingBody(t *testing.T) {
	c := newHTTP2ReaderTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusUnauthorized, `{"error":"bad credentials"}`)
	}))

	r := &infiniteReader{}
	type outcome struct {
		result *scanii.ProcessingResult
		err    error
	}
	outcomes := make(chan outcome, 1)
	go func() {
		result, err := c.ProcessReader(context.Background(), r, "endless.bin", nil, "")
		outcomes <- outcome{result, err}
	}()

	select {
	case o := <-outcomes:
		var apiErr *scanii.Error
		if !errors.As(o.err, &apiErr) || apiErr.StatusCode != http.StatusUnauthorized {
			t.Fatalf("err = %v, want *scanii.Error with status 401", o.err)
		}
		if o.result != nil {
			t.Errorf("result = %+v, want nil", o.result)
		}
	case <-time.After(readerTestTimeout):
		t.Fatal("ProcessReader did not return after the server answered")
	}
	assertAbandoned(t, r)
}

func TestProcessReaderOverHTTP2AbortsWhenContextCanceled(t *testing.T) {
	started := make(chan struct{})
	c := newHTTP2ReaderTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf := make([]byte, 1024)
		if _, err := io.ReadFull(r.Body, buf); err != nil {
			return
		}
		close(started)
		_, _ = io.Copy(io.Discard, r.Body)
	}))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		select {
		case <-started:
			cancel()
		case <-time.After(readerTestTimeout):
		}
	}()

	r := &infiniteReader{}
	errs := make(chan error, 1)
	go func() {
		_, err := c.ProcessReader(ctx, r, "endless.bin", nil, "")
		errs <- err
	}()

	select {
	case err := <-errs:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("err = %v, want context.Canceled", err)
		}
	case <-time.After(readerTestTimeout):
		t.Fatal("ProcessReader did not return after the context was canceled")
	}
	assertAbandoned(t, r)
}
