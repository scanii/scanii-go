package scanii

import (
	"context"
	"net/http"
)

// RetrieveAccountInfo fetches information about the calling account: balance,
// users, API keys, and subscription status.
//
// See https://scanii.github.io/openapi/v22/ — GET /account.json.
func (c *Client) RetrieveAccountInfo(ctx context.Context) (*AccountInfo, error) {
	req, err := c.newRequest(ctx, http.MethodGet, c.target.resolve("/account.json"), nil)
	if err != nil {
		return nil, err
	}

	var result AccountInfo
	if _, err := c.do(req, &result, http.StatusOK); err != nil {
		return nil, err
	}
	return &result, nil
}
