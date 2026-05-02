package scanii

import "time"

// ProcessingResult is the result of a synchronous file scan, returned by
// Process and Retrieve.
//
// See https://scanii.github.io/openapi/v22/ for the response schema.
type ProcessingResult struct {
	ID            string            `json:"id"`
	Checksum      string            `json:"checksum"`
	ContentLength int64             `json:"content_length"`
	Findings      []string          `json:"findings"`
	CreationDate  time.Time         `json:"creation_date"`
	ContentType   string            `json:"content_type"`
	Metadata      map[string]string `json:"metadata"`

	// Deprecated: The server never populates this field on successful responses.
	// Errors arrive as non-2xx HTTP responses returned as *scanii.Error by every
	// client method. Will be removed in a future major version.
	Error string `json:"error,omitempty"`
}

// TraceResult holds the ordered processing events for a scan, returned by
// RetrieveTrace.
//
// This is a v2.2 preview surface; the API shape may shift before it is marked
// stable. See https://scanii.github.io/openapi/v22/ — GET /files/{id}/trace.
type TraceResult struct {
	ID     string       `json:"id"`
	Events []TraceEvent `json:"events"`
}

// TraceEvent is a single processing event within a TraceResult.
type TraceEvent struct {
	Timestamp time.Time `json:"timestamp"`
	Message   string    `json:"message"`
}

// PendingResult is returned by ProcessAsync and Fetch — it carries only the
// resource id; the final ProcessingResult must be fetched via Retrieve once
// the scan finishes (or via the optional callback URL).
type PendingResult struct {
	ID string `json:"id"`
}

// AuthToken is a short-lived bearer token returned by CreateAuthToken.
type AuthToken struct {
	ID             string    `json:"id"`
	CreationDate   time.Time `json:"creation_date"`
	ExpirationDate time.Time `json:"expiration_date"`
}

// AccountInfo describes the calling account, returned by RetrieveAccountInfo.
type AccountInfo struct {
	Name             string            `json:"name"`
	Balance          int64             `json:"balance"`
	StartingBalance  int64             `json:"starting_balance"`
	BillingEmail     string            `json:"billing_email"`
	Subscription     string            `json:"subscription"`
	CreationDate     time.Time         `json:"creation_date"`
	ModificationDate time.Time         `json:"modification_date"`
	Users            map[string]User   `json:"users"`
	Keys             map[string]APIKey `json:"keys"`
}

// User is a user record nested in AccountInfo.
type User struct {
	CreationDate  time.Time `json:"creation_date"`
	LastLoginDate time.Time `json:"last_login_date"`
}

// APIKey is an API key record nested in AccountInfo.
type APIKey struct {
	Active                     bool      `json:"active"`
	CreationDate               time.Time `json:"creation_date"`
	LastSeenDate               time.Time `json:"last_seen_date"`
	DetectionCategoriesEnabled []string  `json:"detection_categories_enabled"`
	Tags                       []string  `json:"tags"`
}
