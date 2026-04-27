// Package scanii is the official Go SDK for the Scanii content processing
// service.
//
// SDK Principles:
//
//  1. Light. Zero runtime dependencies, stdlib only.
//  2. Up to date. Always current with the latest Scanii API.
//  3. Integration-only. Wraps the REST API — retries, concurrency, and
//     batching are the caller's responsibility.
//
// See https://scanii.github.io/openapi/v22/ for the full API contract.
package scanii

import (
	"encoding/base64"
	"fmt"
	"net/http"
)

// Version is the SDK version, used in the User-Agent header.
const Version = "2.0.0"

const (
	headerUserAgent     = "User-Agent"
	headerAuthorization = "Authorization"
	headerContentType   = "Content-Type"
)

// Client is a thread-safe handle for talking to the Scanii API.
//
// Construct one with NewClient and reuse it across goroutines.
type Client struct {
	target     Target
	authHeader string
	userAgent  string
	httpClient *http.Client
}

// ClientOpts configures a new Client. Key is required. Secret is required for
// API key/secret authentication and may be empty when Key is a temporary auth
// token. If Target is the zero value, TargetAuto is used. If HTTPClient is
// nil, http.DefaultClient is used.
type ClientOpts struct {
	Target     Target
	Key        string
	Secret     string
	HTTPClient *http.Client
	// UserAgent, if non-empty, is prepended to the default SDK user agent.
	UserAgent string
}

// NewClient creates a new Client. It returns an error when Key is empty or
// contains a colon (which would corrupt the Basic auth header).
func NewClient(opts ClientOpts) (*Client, error) {
	if opts.Key == "" {
		return nil, fmt.Errorf("scanii: key must not be empty")
	}
	if containsColon(opts.Key) {
		return nil, fmt.Errorf("scanii: key must not contain a colon")
	}

	target := opts.Target
	if target.endpoint == "" {
		target = TargetAuto
	}

	httpClient := opts.HTTPClient
	if httpClient == nil {
		httpClient = http.DefaultClient
	}

	creds := opts.Key + ":" + opts.Secret
	authHeader := "Basic " + base64.StdEncoding.EncodeToString([]byte(creds))

	defaultUA := fmt.Sprintf("scanii-go/v%s", Version)
	userAgent := defaultUA
	if opts.UserAgent != "" {
		userAgent = opts.UserAgent + " " + defaultUA
	}

	return &Client{
		target:     target,
		authHeader: authHeader,
		userAgent:  userAgent,
		httpClient: httpClient,
	}, nil
}

// Target returns the regional endpoint this client is talking to.
func (c *Client) Target() Target {
	return c.target
}

func containsColon(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] == ':' {
			return true
		}
	}
	return false
}
