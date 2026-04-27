package scanii

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// Error is returned by every Client method when the Scanii API responds with a
// non-2xx status code. It exposes the HTTP status and the parsed error message
// (when the response body was JSON) or the raw body otherwise.
type Error struct {
	StatusCode int
	Message    string
}

// Error implements the error interface.
func (e *Error) Error() string {
	return fmt.Sprintf("scanii: HTTP %d: %s", e.StatusCode, e.Message)
}

func (c *Client) newRequest(ctx context.Context, method, url string, body io.Reader) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set(headerUserAgent, c.userAgent)
	req.Header.Set(headerAuthorization, c.authHeader)
	return req, nil
}

// do executes req and decodes the response body into out when the status
// code matches one of expected. expected accepts one or more codes — the auth
// token endpoint, for example, returns either 200 or 201 depending on
// server version.
func (c *Client) do(req *http.Request, out any, expected ...int) ([]byte, error) {
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if !statusIn(resp.StatusCode, expected) {
		return body, parseError(resp, body)
	}

	if out != nil {
		if err := json.Unmarshal(body, out); err != nil {
			return body, fmt.Errorf("scanii: decoding response: %w", err)
		}
	}
	return body, nil
}

func statusIn(code int, expected []int) bool {
	for _, e := range expected {
		if code == e {
			return true
		}
	}
	return false
}

func parseError(resp *http.Response, body []byte) error {
	contentType := resp.Header.Get(headerContentType)
	if contentType == "application/json" || contentType == "application/json; charset=utf-8" {
		var payload struct {
			Error string `json:"error"`
		}
		if err := json.Unmarshal(body, &payload); err == nil && payload.Error != "" {
			return &Error{StatusCode: resp.StatusCode, Message: payload.Error}
		}
	}
	return &Error{StatusCode: resp.StatusCode, Message: string(body)}
}
