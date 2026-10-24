package scanii

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

const formFieldFile = "file"

// Process submits a file for synchronous scanning.
//
// path is the local file path to upload. metadata is optional caller-supplied
// key/value pairs that will be attached to the result and passed back to any
// callback. callback is an optional URL that Scanii will POST the result to;
// pass an empty string to disable.
//
// See https://scanii.github.io/openapi/v22/ — POST /files.
func (c *Client) Process(ctx context.Context, path string, metadata map[string]string, callback string) (*ProcessingResult, error) {
	body, contentType, err := buildFileMultipart(path, metadata, callback)
	if err != nil {
		return nil, err
	}

	req, err := c.newRequest(ctx, http.MethodPost, c.target.resolve("/files"), body)
	if err != nil {
		return nil, err
	}
	req.Header.Set(headerContentType, contentType)

	var result ProcessingResult
	if _, err := c.do(req, &result, http.StatusCreated); err != nil {
		return nil, err
	}
	return &result, nil
}

// ProcessAsync submits a file for asynchronous scanning. It returns a
// PendingResult immediately; use Retrieve (or the optional callback URL) to
// fetch the final ProcessingResult.
//
// See https://scanii.github.io/openapi/v22/ — POST /files/async.
func (c *Client) ProcessAsync(ctx context.Context, path string, metadata map[string]string, callback string) (*PendingResult, error) {
	body, contentType, err := buildFileMultipart(path, metadata, callback)
	if err != nil {
		return nil, err
	}

	req, err := c.newRequest(ctx, http.MethodPost, c.target.resolve("/files/async"), body)
	if err != nil {
		return nil, err
	}
	req.Header.Set(headerContentType, contentType)

	var result PendingResult
	if _, err := c.do(req, &result, http.StatusAccepted); err != nil {
		return nil, err
	}
	return &result, nil
}

// ProcessReader submits the content read from r for synchronous scanning. It
// is the streaming counterpart of Process for content that is not a local
// file, such as an HTTP request body, an object storage download or an
// in-memory buffer.
//
// filename is sent as the file name of the uploaded part and must not be
// empty. metadata and callback behave exactly as in Process.
//
// The multipart body is produced from r while it is being sent, so the SDK
// never holds the whole content in memory. Because the length is not known up
// front, the request is sent without a Content-Length header: as chunked
// transfer encoding on HTTP/1.1, or as a streamed body on HTTP/2, which has no
// Transfer-Encoding header and which the default client negotiates with the
// Scanii API. The body cannot be replayed, which has two consequences.
// Redirects that require resending it (307 and 308) are not followed and
// surface as an *Error with the redirect status. And net/http will not
// transparently retry the request when a pooled keep-alive connection turns
// out to be stale or is closing (an HTTP/1.1 reset, an HTTP/2 GOAWAY or
// REFUSED_STREAM), as it does for Process; the call fails with a transport
// error instead. Callers that need that resilience should retry with a fresh
// reader, or use Process for content that can be buffered.
//
// An error returned by r aborts the upload and is returned wrapped, so
// errors.Is and errors.As match it. ProcessReader reads r until io.EOF or until
// the request ends, and never closes it; closing r remains the caller's
// responsibility. Canceling ctx aborts the upload, but ProcessReader waits for
// an in-progress r.Read to return before it does, so a reader that can block
// indefinitely should stop blocking when ctx is done.
//
// See https://scanii.github.io/openapi/v22/ — POST /files.
func (c *Client) ProcessReader(ctx context.Context, r io.Reader, filename string, metadata map[string]string, callback string) (*ProcessingResult, error) {
	var result ProcessingResult
	if err := c.postStreamingMultipart(ctx, "/files", r, filename, metadata, callback, &result, http.StatusCreated); err != nil {
		return nil, err
	}
	return &result, nil
}

// ProcessAsyncReader submits the content read from r for asynchronous
// scanning. It is the streaming counterpart of ProcessAsync and returns a
// PendingResult; use Retrieve (or the optional callback URL) to fetch the
// final ProcessingResult.
//
// The streaming behavior is the same as ProcessReader: no Content-Length
// header (chunked transfer encoding on HTTP/1.1, a streamed body on HTTP/2), no
// replay on redirects or stale connections, reader errors returned wrapped, and
// r left open for the caller to close.
//
// See https://scanii.github.io/openapi/v22/ — POST /files/async.
func (c *Client) ProcessAsyncReader(ctx context.Context, r io.Reader, filename string, metadata map[string]string, callback string) (*PendingResult, error) {
	var result PendingResult
	if err := c.postStreamingMultipart(ctx, "/files/async", r, filename, metadata, callback, &result, http.StatusAccepted); err != nil {
		return nil, err
	}
	return &result, nil
}

// errUploadEnded is the error the multipart writer goroutine sees on its next
// pipe write once the HTTP round trip has returned without consuming the whole
// body, for example when the server answers before reading it.
var errUploadEnded = errors.New("scanii: upload ended before the body was fully sent")

// postStreamingMultipart POSTs the multipart encoding of r to path, writing it
// through an io.Pipe from a goroutine, and decodes the response into out the
// same way Process and ProcessAsync do.
func (c *Client) postStreamingMultipart(ctx context.Context, path string, r io.Reader, filename string, metadata map[string]string, callback string, out any, expected int) error {
	if r == nil {
		return errors.New("scanii: reader must not be nil")
	}
	if filename == "" {
		return errors.New("scanii: filename must not be empty")
	}

	pr, pw := io.Pipe()
	writer := multipart.NewWriter(pw)
	content := &readErrorRecorder{r: r}

	// The goroutine is the only writer of content.err; closing done publishes
	// it to this goroutine.
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = pw.CloseWithError(writeFileMultipart(writer, content, filename, metadata, callback))
	}()

	// Unblock the writer goroutine if the round trip ends before it has written
	// everything, then wait for it so it never outlives this call.
	finish := func() {
		_ = pr.CloseWithError(errUploadEnded)
		<-done
	}

	req, err := c.newRequest(ctx, http.MethodPost, c.target.resolve(path), pr)
	if err != nil {
		finish()
		return err
	}
	req.Header.Set(headerContentType, writer.FormDataContentType())

	_, err = c.do(req, out, expected)
	finish()
	// A failing reader is the root cause of whatever the transport reported,
	// typically a broken connection or a 4xx for a truncated body.
	if content.err != nil {
		return fmt.Errorf("scanii: reading upload content: %w", content.err)
	}
	return err
}

// readErrorRecorder remembers the first non-EOF error returned by the
// caller's reader so it can be told apart from pipe and transport errors.
type readErrorRecorder struct {
	r   io.Reader
	err error
}

func (e *readErrorRecorder) Read(p []byte) (int, error) {
	n, err := e.r.Read(p)
	if err != nil && err != io.EOF && e.err == nil {
		e.err = err
	}
	return n, err
}

// Fetch instructs Scanii to download a remote URL and scan it asynchronously.
// It returns a PendingResult; the final ProcessingResult is delivered via
// Retrieve or the optional callback URL.
//
// See https://scanii.github.io/openapi/v22/ — POST /files/fetch.
func (c *Client) Fetch(ctx context.Context, location string, metadata map[string]string, callback string) (*PendingResult, error) {
	form := url.Values{}
	form.Set("location", location)
	if callback != "" {
		form.Set("callback", callback)
	}
	for k, v := range metadata {
		form.Set(fmt.Sprintf("metadata[%s]", k), v)
	}

	req, err := c.newRequest(ctx, http.MethodPost, c.target.resolve("/files/fetch"), strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set(headerContentType, "application/x-www-form-urlencoded")

	var result PendingResult
	if _, err := c.do(req, &result, http.StatusAccepted); err != nil {
		return nil, err
	}
	return &result, nil
}

// Retrieve fetches the result of a previously processed file by id.
//
// See https://scanii.github.io/openapi/v22/ — GET /files/{id}.
func (c *Client) Retrieve(ctx context.Context, id string) (*ProcessingResult, error) {
	req, err := c.newRequest(ctx, http.MethodGet, c.target.resolve("/files/"+id), nil)
	if err != nil {
		return nil, err
	}

	var result ProcessingResult
	if _, err := c.do(req, &result, http.StatusOK); err != nil {
		return nil, err
	}
	return &result, nil
}

// Delete removes the processing result for a file id. The trace remains
// available and can be removed separately with DeleteTrace.
//
// See https://scanii.github.io/openapi/v22/ — DELETE /files/{id}.
func (c *Client) Delete(ctx context.Context, id string) error {
	req, err := c.newRequest(ctx, http.MethodDelete, c.target.resolve("/files/"+id), nil)
	if err != nil {
		return err
	}
	_, err = c.do(req, nil, http.StatusNoContent)
	return err
}

// ProcessFromUrl submits a remote URL for synchronous scanning.
//
// location must be a string URL. The URL is sent as a multipart/form-data
// field to POST /files; the Scanii server fetches and scans it synchronously.
// This is distinct from Fetch, which submits to POST /files/fetch for
// asynchronous server-side fetching.
//
// See https://scanii.github.io/openapi/v22/ — POST /files.
func (c *Client) ProcessFromUrl(ctx context.Context, location string, metadata map[string]string, callback string) (*ProcessingResult, error) {
	body, contentType, err := buildUrlMultipart(location, metadata, callback)
	if err != nil {
		return nil, err
	}

	req, err := c.newRequest(ctx, http.MethodPost, c.target.resolve("/files"), body)
	if err != nil {
		return nil, err
	}
	req.Header.Set(headerContentType, contentType)

	var result ProcessingResult
	if _, err := c.do(req, &result, http.StatusCreated); err != nil {
		return nil, err
	}
	return &result, nil
}

// buildUrlMultipart encodes location (and optional metadata/callback) as a
// multipart/form-data body for ProcessFromUrl. The cli rejects urlencoded
// bodies on POST /files with MethodNotAllowed; multipart is required.
func buildUrlMultipart(location string, metadata map[string]string, callback string) (io.Reader, string, error) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	if err := writer.WriteField("location", location); err != nil {
		return nil, "", err
	}
	if callback != "" {
		if err := writer.WriteField("callback", callback); err != nil {
			return nil, "", err
		}
	}
	for k, v := range metadata {
		if err := writer.WriteField(fmt.Sprintf("metadata[%s]", k), v); err != nil {
			return nil, "", err
		}
	}

	if err := writer.Close(); err != nil {
		return nil, "", err
	}
	return body, writer.FormDataContentType(), nil
}

func buildFileMultipart(path string, metadata map[string]string, callback string) (io.Reader, string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, "", err
	}
	defer file.Close()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	if err := writeFileMultipart(writer, file, filepath.Base(path), metadata, callback); err != nil {
		return nil, "", err
	}
	return body, writer.FormDataContentType(), nil
}

// writeFileMultipart writes the file part, then the callback and metadata[k]
// fields, and closes writer. Process and ProcessReader share it so the buffered
// and streamed uploads put the same parts on the wire in the same order.
func writeFileMultipart(writer *multipart.Writer, content io.Reader, filename string, metadata map[string]string, callback string) error {
	part, err := writer.CreateFormFile(formFieldFile, filename)
	if err != nil {
		return err
	}
	if _, err := io.Copy(part, content); err != nil {
		return err
	}

	if callback != "" {
		if err := writer.WriteField("callback", callback); err != nil {
			return err
		}
	}
	for k, v := range metadata {
		if err := writer.WriteField(fmt.Sprintf("metadata[%s]", k), v); err != nil {
			return err
		}
	}

	return writer.Close()
}
