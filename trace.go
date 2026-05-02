package scanii

import (
	"context"
	"net/http"
)

// RetrieveTrace fetches the ordered processing event trace for a scan by id.
//
// Returns (nil, nil) when no trace exists for the given id (HTTP 404), matching
// the nil-on-404 convention used across the SDK family.
//
// This is a v2.2 preview surface; the API shape may shift before it is marked
// stable. See https://scanii.github.io/openapi/v22/ — GET /files/{id}/trace.
func (c *Client) RetrieveTrace(ctx context.Context, id string) (*TraceResult, error) {
	req, err := c.newRequest(ctx, http.MethodGet, c.target.resolve("/files/"+id+"/trace"), nil)
	if err != nil {
		return nil, err
	}

	var result TraceResult
	if _, err := c.do(req, &result, http.StatusOK); err != nil {
		if sErr, ok := err.(*Error); ok && sErr.StatusCode == http.StatusNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &result, nil
}
