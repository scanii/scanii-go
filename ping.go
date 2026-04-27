package scanii

import (
	"context"
	"net/http"
)

// Ping verifies that the configured credentials can reach the Scanii API.
// It returns true iff the server responded with HTTP 200.
//
// See https://scanii.github.io/openapi/v22/ — GET /ping.
func (c *Client) Ping(ctx context.Context) (bool, error) {
	req, err := c.newRequest(ctx, http.MethodGet, c.target.resolve("/ping"), nil)
	if err != nil {
		return false, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		return true, nil
	}
	return false, &Error{StatusCode: resp.StatusCode, Message: resp.Status}
}
