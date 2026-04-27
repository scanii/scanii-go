package scanii

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// CreateAuthToken mints a short-lived bearer token usable in place of the API
// key. timeout is how long the token should remain valid; values below one
// second are rounded up to one second.
//
// See https://scanii.github.io/openapi/v22/ — POST /auth/tokens.
func (c *Client) CreateAuthToken(ctx context.Context, timeout time.Duration) (*AuthToken, error) {
	if timeout <= 0 {
		return nil, fmt.Errorf("scanii: timeout must be positive")
	}
	seconds := int(timeout / time.Second)
	if seconds < 1 {
		seconds = 1
	}

	form := url.Values{}
	form.Set("timeout", strconv.Itoa(seconds))

	req, err := c.newRequest(ctx, http.MethodPost, c.target.resolve("/auth/tokens"), strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set(headerContentType, "application/x-www-form-urlencoded")

	var result AuthToken
	if _, err := c.do(req, &result, http.StatusCreated, http.StatusOK); err != nil {
		return nil, err
	}
	return &result, nil
}

// RetrieveAuthToken fetches metadata about a previously created auth token.
//
// See https://scanii.github.io/openapi/v22/ — GET /auth/tokens/{id}.
func (c *Client) RetrieveAuthToken(ctx context.Context, id string) (*AuthToken, error) {
	req, err := c.newRequest(ctx, http.MethodGet, c.target.resolve("/auth/tokens/"+id), nil)
	if err != nil {
		return nil, err
	}

	var result AuthToken
	if _, err := c.do(req, &result, http.StatusOK); err != nil {
		return nil, err
	}
	return &result, nil
}

// DeleteAuthToken revokes an existing auth token.
//
// See https://scanii.github.io/openapi/v22/ — DELETE /auth/tokens/{id}.
func (c *Client) DeleteAuthToken(ctx context.Context, id string) error {
	req, err := c.newRequest(ctx, http.MethodDelete, c.target.resolve("/auth/tokens/"+id), nil)
	if err != nil {
		return err
	}

	if _, err := c.do(req, nil, http.StatusNoContent); err != nil {
		return err
	}
	return nil
}
