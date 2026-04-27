# scanii-go

Official Go SDK for the [Scanii](https://www.scanii.com) content processing API.

## SDK Principles

1. **Light.** Zero runtime dependencies, stdlib only.
2. **Up to date.** Always current with the latest Scanii API.
3. **Integration-only.** Wraps the REST API — retries, concurrency, and batching are the caller's responsibility.

## Install

```bash
go get github.com/scanii/scanii-go@v2
```

Requires Go 1.25 or newer.

## Quickstart

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/scanii/scanii-go"
)

func main() {
    client, err := scanii.NewClient(scanii.ClientOpts{
        Target: scanii.TargetAuto,
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

## Regional endpoints

| Constant | Endpoint |
|---|---|
| `scanii.TargetAuto` | `https://api.scanii.com` |
| `scanii.TargetUS1` | `https://api-us1.scanii.com` |
| `scanii.TargetEU1` | `https://api-eu1.scanii.com` |
| `scanii.TargetEU2` | `https://api-eu2.scanii.com` |
| `scanii.TargetAP1` | `https://api-ap1.scanii.com` |
| `scanii.TargetAP2` | `https://api-ap2.scanii.com` |
| `scanii.TargetCA1` | `https://api-ca1.scanii.com` |

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
   + import "github.com/scanii/scanii-go"
   ```

2. **Constructor and method names live under `scanii`** (the package was flattened from `pkg/client`):

   ```diff
   - c := client.NewClient(&client.Opts{Target: endpoints.LATEST, Key: k, Secret: s})
   + c, err := scanii.NewClient(scanii.ClientOpts{Target: scanii.TargetAuto, Key: k, Secret: s})
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
