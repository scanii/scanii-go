# Changelog

## v2.0.0

Rebrand and modernization release.

### Breaking

- **Module path renamed.** `github.com/uvasoftware/scanii-go` →
  `github.com/scanii/scanii-go`. Major version bump uses the module-path-change
  convention, so the import does **not** include a `/v2` suffix.
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
