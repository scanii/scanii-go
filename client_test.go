package scanii_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/scanii/scanii-go/v2"
)

// The credentials scanii-cli's server accepts by default, shared by the
// httptest-backed unit tests and the integration tests.
const (
	testKey    = "key"
	testSecret = "secret"
)

// These tests need no scanii-cli server: they check the constructor's argument
// validation and run against net/http/httptest.

func TestNewClientRejectsEmptyKey(t *testing.T) {
	if _, err := scanii.NewClient(scanii.ClientOpts{Key: ""}); err == nil {
		t.Fatal("expected error for empty key")
	}
}

func TestNewClientRejectsKeyWithColon(t *testing.T) {
	if _, err := scanii.NewClient(scanii.ClientOpts{Key: "a:b", Secret: "x"}); err == nil {
		t.Fatal("expected error for key containing colon")
	}
}

// countingTransport records how many requests it carries before delegating to
// the default transport.
type countingTransport struct {
	mu    sync.Mutex
	count int
}

func (t *countingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	t.mu.Lock()
	t.count++
	t.mu.Unlock()
	return http.DefaultTransport.RoundTrip(req)
}

// TestNewClientUsesSuppliedHTTPClient guards the constructor against regressing
// to the v1.x behavior, where a caller-supplied *http.Client was assigned only
// inside the nil branch, leaving the field nil so the first request panicked.
// The v2.0.0 rewrite fixed it; this test keeps it fixed. See
// https://github.com/scanii/scanii-go/issues/15.
func TestNewClientUsesSuppliedHTTPClient(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	tr := &countingTransport{}
	c, err := scanii.NewClient(scanii.ClientOpts{
		Target:     scanii.NewTarget(srv.URL),
		Key:        testKey,
		Secret:     testSecret,
		HTTPClient: &http.Client{Transport: tr},
	})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	if _, err := c.Ping(context.Background()); err != nil {
		t.Fatalf("Ping: %v", err)
	}

	tr.mu.Lock()
	defer tr.mu.Unlock()
	if tr.count == 0 {
		t.Fatal("supplied HTTPClient was discarded; request did not use its transport")
	}
}
