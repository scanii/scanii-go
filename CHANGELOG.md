# Changelog

## [Unreleased]

### Added

- `Client.ProcessReader(ctx, reader, filename, metadata, callback)` and
  `Client.ProcessAsyncReader(ctx, reader, filename, metadata, callback)` scan content from any
  `io.Reader`, so in-memory content or another stream no longer needs a temp file. The multipart
  body is streamed through an `io.Pipe` as it is read rather than buffered and is sent without a
  `Content-Length` (chunked transfer encoding on HTTP/1.1, a streamed body on HTTP/2). Because the
  body cannot be replayed, redirects that would resend it (307, 308) are not followed and net/http
  does not transparently retry the request on a stale or closing pooled connection. Reader errors
  abort the upload and are returned wrapped for `errors.Is`. `Process` and `ProcessAsync` are unchanged
  and still buffer the file and send a `Content-Length`.

### Changed

- CI now tests Go 1.26 and 1.27, the two supported Go releases. The `go.mod` directive stays at
  `go 1.25`, so toolchains pinned to 1.25 (`GOTOOLCHAIN=local`) still build the module, even
  though Go 1.25 is out of support and no longer tested here.
- Dropped the "v2.2 preview" designation from `RetrieveTrace`, `ProcessFromUrl` and
  `TraceResult`. The trace endpoint is no longer marked preview in the contract, and
  `ProcessFromUrl` was never preview; the methods themselves are unchanged.

## [v2.3.0] - Added delete methods for some resources 

### Added

- `Client.Delete(ctx, id)` deletes a processing result while leaving its trace intact.
- `Client.DeleteTrace(ctx, id)` deletes a processing trace independently.
- Both delete methods return `*scanii.Error` with status 404 for unknown IDs.

## [v2.2.2] — fix module path for v2

### Fixed

- **`go.mod` now declares `github.com/scanii/scanii-go/v2`.** Go's semantic
  import versioning requires the `/vN` suffix in the module path for every
  major version at or above v2. Without it, all v2.x tags were uninstallable:
  `go get github.com/scanii/scanii-go@v2.2.1` failed with `module path must
  match major version`, and `@latest` fell back to the broken v1.1.0 tag.
  Consumers must now import `github.com/scanii/scanii-go/v2`; the package name
  is unchanged, so `scanii.NewClient(...)` and every other call site stays the
  same.
- The internal `importPath` constant used to resolve the SDK version from build
  info was updated to match. Left stale, it would have reported `(devel)` in the
  `User-Agent` header for every consumer.

## [v2.2.1] — dependency refresh

- Bumped CI actions: `actions/checkout` v4 → v7, `actions/setup-go` v5 → v7.
- Relaxed `go.mod` directive to `go 1.25` so the previous-stable CI leg builds. No runtime dependencies — the module itself is unchanged.

## [v2.2.0] — deprecate AUTO endpoint

### Deprecated

- `TargetAuto` — latency-based routing does not guarantee regional data placement. Use
  `TargetUS1`, `TargetEU1`, `TargetEU2`, `TargetAP1`, `TargetAP2`, or `TargetCA1` for
  explicit data residency control. Will be removed in a future major version.

## [v2.1.0] — v2.2 surface

### New API

- `Client.RetrieveTrace(ctx, id)` → `(*TraceResult, error)` — retrieves the
  ordered processing event trace for a scan via `GET /files/{id}/trace`. Returns
  `(nil, nil)` on 404 (no trace for that id). v2.2 preview surface; API shape may
  shift before marked stable.
- `Client.ProcessFromUrl(ctx, location, metadata, callback)` →
  `(*ProcessingResult, error)` — submits a URL for synchronous scanning via
  `POST /files` with `location` as a multipart/form-data field. Distinct from
  `Fetch`, which submits to `/files/fetch` for asynchronous server-side fetching.
  `location` must be a string URL. v2.2 preview surface.
- `TraceResult` struct (`ID string`, `Events []TraceEvent`).
- `TraceEvent` struct (`Timestamp time.Time`, `Message string`).

### Deprecations

- `ProcessingResult.Error` — deprecated via godoc. The server never populates
  this field on successful responses; errors arrive as non-2xx HTTP responses
  returned as `*scanii.Error` by every client method. The field is retained for
  backwards compatibility. Will be removed in a future major version.

## v2.0.0

Rebrand and modernization release.

### Breaking

- **Module path renamed.** `github.com/uvasoftware/scanii-go` →
  `github.com/scanii/scanii-go`. This entry originally claimed the
  module-path-change convention meant the import did **not** need a `/v2`
  suffix. That was wrong — changing the module path lets a module restart at
  v1 under the new path, but it does not exempt v2+ tags from the suffix
  requirement. The result was that v2.0.0 through v2.2.1 were uninstallable.
  Corrected in v2.2.2; the import path is `github.com/scanii/scanii-go/v2`.
- **Package flattened.** Callers now write `scanii.NewClient(...)` instead of
  `client.NewClient(...)`. The old `pkg/client`, `pkg/endpoints`, and
  `pkg/models` subpackages are gone — every type lives in the root `scanii`
  package.
- **`context.Context` is required on every API method.** The first argument of
  `Process`, `ProcessAsync`, `Fetch`, `Retrieve`, `Ping`, `CreateAuthToken`,
  `RetrieveAuthToken`, `DeleteAuthToken`, and `RetrieveAccountInfo` is now
  `ctx context.Context`. Cancellation and deadlines are honored through
  `http.NewRequestWithContext`.
- **`NewClient` returns `(*Client, error)`.** It now validates the API key and
  rejects empty keys or keys containing a colon.
- **`ClientOpts` replaces `Opts`.**
- **`Process` / `ProcessAsync` / `Fetch` argument order.** The optional
  `callback` URL moved to the last position, after `metadata`, to match the
  Java reference SDK.
- **Endpoint constants renamed.** The grab-bag of `V20_*` / `V21_*` constants
  is gone — use `scanii.TargetAuto`, `scanii.TargetUS1`, `scanii.TargetEU1`,
  `scanii.TargetEU2`, `scanii.TargetAP1`, `scanii.TargetAP2`,
  `scanii.TargetCA1`, or `scanii.NewTarget(url)` for a custom endpoint.
- **API version pinned to v2.2** server-side. The path prefix is owned by the
  SDK; callers do not pick it.
- **`CreateAuthToken` takes `time.Duration`** instead of an `int` count of
  seconds.
- **Errors are typed.** Non-2xx responses return `*scanii.Error` with
  `StatusCode` and `Message` fields.

### Removed

- `RetrieveAccountInfo` is now `RetrieveAccountInfo(ctx)` in the root package
  (was `Client.RetrieveAccountInfo()` in `pkg/client`).
- `testify` dependency. All tests use stdlib `testing` only.
- All transitive `require` entries from `go.sum`.

### Added

- `go 1.26` directive.
- `context.Context` plumbed through every method.
- `scanii.Error` type for typed error handling.
- Integration tests target a local `scanii-cli` instance — no real credentials
  required, ever. CI uses `scanii/setup-cli-action@v1`.
- PR CI matrix: Go 1.25 + 1.26 × ubuntu-latest, macos-latest, windows-latest,
  with the race detector enabled.

### Migration

See the README's "Migration from `github.com/uvasoftware/scanii-go`" section.
