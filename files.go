package scanii

import (
	"bytes"
	"context"
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

func buildFileMultipart(path string, metadata map[string]string, callback string) (io.Reader, string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, "", err
	}
	defer file.Close()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	part, err := writer.CreateFormFile(formFieldFile, filepath.Base(path))
	if err != nil {
		return nil, "", err
	}
	if _, err := io.Copy(part, file); err != nil {
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
