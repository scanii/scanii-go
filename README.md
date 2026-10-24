# scanii-go

Official Go SDK for the [Scanii](https://www.scanii.com) content processing API.

## SDK Principles

1. **Light.** Zero runtime dependencies, stdlib only.
2. **Up to date.** Always current with the latest Scanii API.
3. **Integration-only.** Wraps the REST API — retries, concurrency, and batching are the caller's responsibility.

## Install

```bash
go get github.com/scanii/scanii-go/v2
```

Requires Go 1.25 or newer.

## Quickstart

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/scanii/scanii-go/v2"
)

func main() {
    client, err := scanii.NewClient(scanii.ClientOpts{
        Target: scanii.TargetUS1,
        Key:    "your-api-key",
        Secret: "your-api-secret",
    })
    if err != nil {
        log.Fatal(err)
    }

    ctx := context.Background()
    result, err := client.Process(ctx, "/path/to/file", nil, "")
    if err != nil {
        log.Fatal(err)
    }
    fmt.Printf("findings: %v\n", result.Findings)
}
```

Every client method takes a `context.Context` as its first argument so that callers can cancel in-flight requests or attach deadlines.

### Scanning content that is not a file

`ProcessReader` and `ProcessAsyncReader` take any `io.Reader` plus the file name to report, so content
that is already in memory or arriving from another stream can be scanned without a temp file:

```go
resp, err := http.Get("https://example.com/upload.pdf")
if err != nil {
    log.Fatal(err)
}
defer resp.Body.Close()

result, err := client.ProcessReader(ctx, resp.Body, "upload.pdf", map[string]string{"source": "web"}, "")
if err != nil {
    log.Fatal(err)
}
fmt.Printf("findings: %v\n", result.Findings)
```

The reader methods stream the body as it is read instead of buffering the whole file, so memory use
stays flat for large uploads. Because the length is not known up front the request carries no
`Content-Length` header: it uses chunked transfer encoding on HTTP/1.1 and a streamed body on
HTTP/2 (which the default client negotiates with the Scanii API, and which has no
`Transfer-Encoding` header). Since the body cannot be replayed, redirects that would resend it (307,
308) are not followed, and net/http will not transparently retry the request when a pooled
connection turns out to be stale or is closing, as it does for `Process`. Callers that need that
resilience should retry with a fresh reader, or use `Process` for content that can be buffered. An
error from the reader aborts the upload and is returned wrapped, so `errors.Is` matches it. The SDK never closes the reader. `Process` and `ProcessAsync` keep
buffering the file and sending a `Content-Length`.

## API

| Method | REST | Returns |
|---|---|---|
| `Process(ctx, path, metadata, callback)` | `POST /files` | `*ProcessingResult` |
| `ProcessAsync(ctx, path, metadata, callback)` | `POST /files/async` | `*PendingResult` |
| `ProcessReader(ctx, reader, filename, metadata, callback)` | `POST /files` | `*ProcessingResult` |
| `ProcessAsyncReader(ctx, reader, filename, metadata, callback)` | `POST /files/async` | `*PendingResult` |
| `ProcessFromUrl(ctx, location, metadata, callback)` | `POST /files` | `*ProcessingResult` |
| `Fetch(ctx, location, metadata, callback)` | `POST /files/fetch` | `*PendingResult` |
| `Retrieve(ctx, id)` | `GET /files/{id}` | `*ProcessingResult` |
| `Delete(ctx, id)` | `DELETE /files/{id}` | `error` (204 on success; result only) |
| `RetrieveTrace(ctx, id)` | `GET /files/{id}/trace` | `*TraceResult` or `nil` |
| `DeleteTrace(ctx, id)` | `DELETE /files/{id}/trace` | `error` (204 on success) |
| `Ping(ctx)` | `GET /ping` | `bool` |
| `CreateAuthToken(ctx, timeout)` | `POST /auth/tokens` | `*AuthToken` |
| `RetrieveAuthToken(ctx, id)` | `GET /auth/tokens/{id}` | `*AuthToken` |
| `DeleteAuthToken(ctx, id)` | `DELETE /auth/tokens/{id}` | `error` |
| `RetrieveAccountInfo(ctx)` | `GET /account` | `*AccountInfo` |

Full API reference: <https://scanii.github.io/openapi/v22/>.

## Regional endpoints

| Constant | Endpoint |
|---|---|
| `scanii.TargetUS1` | `https://api-us1.scanii.com` |
| `scanii.TargetEU1` | `https://api-eu1.scanii.com` |
| `scanii.TargetEU2` | `https://api-eu2.scanii.com` |
| `scanii.TargetAP1` | `https://api-ap1.scanii.com` |
| `scanii.TargetAP2` | `https://api-ap2.scanii.com` |
| `scanii.TargetCA1` | `https://api-ca1.scanii.com` |
| ~~`scanii.TargetAuto`~~ | ~~`https://api.scanii.com`~~ — **deprecated**, does not guarantee regional data placement |

For a custom or local endpoint, use `scanii.NewTarget("http://localhost:4000")`.

## Local development with scanii-cli

Run the integration tests against a local mock server — no real credentials needed:

```bash
docker run -d --name scanii-cli -p 4000:4000 ghcr.io/scanii/scanii-cli:latest server
go test -race ./...
```

Test credentials: key `key`, secret `secret`, endpoint `http://localhost:4000`.

## Migration from `github.com/uvasoftware/scanii-go`

Three changes are required:

1. **Update the import path:**

   ```diff
   - import "github.com/uvasoftware/scanii-go/pkg/client"
   + import "github.com/scanii/scanii-go/v2"
   ```

2. **Constructor and method names live under `scanii`** (the package was flattened from `pkg/client`):

   ```diff
   - c := client.NewClient(&client.Opts{Target: endpoints.LATEST, Key: k, Secret: s})
   + c, err := scanii.NewClient(scanii.ClientOpts{Target: scanii.TargetUS1, Key: k, Secret: s})
   ```

3. **Every method now takes `context.Context` as its first argument:**

   ```diff
   - r, err := c.Process(path, "", metadata)
   + r, err := c.Process(ctx, path, metadata, "")
   ```

   Note that the callback string moved to the last position to match the optional-argument convention.

The old module `github.com/uvasoftware/scanii-go` is deprecated and will not receive further updates.

## API documentation

See [https://scanii.github.io/openapi/v22/](https://scanii.github.io/openapi/v22/) for the full Scanii API contract.

## License

Apache 2.0 — see [LICENSE](LICENSE).
