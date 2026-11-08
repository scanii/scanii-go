# The SDK is the root module. tools/ is a module of its own that pins the analysis tools, so the
# SDK's go directive (and the Go versions it supports) stay independent of the tools' go 1.27.3.

## help: print this help message
.PHONY: help
help:
	@echo 'Usage:' && sed -n 's/^##//p' ${MAKEFILE_LIST} | column -t -s ':' | sed -e 's/^/ /'

# Analysis tools are pinned in tools/go.mod (tool directives, checksummed in tools/go.sum) and run
# with go tool, so nothing is installed. gosec is pinned next to golang.org/x/tools v0.52.0: the
# x/tools its own release pins cannot read Go 1.27 export data and silently analyzes nothing.
GOTOOL = tool -modfile=$(CURDIR)/tools/go.mod

## tidy: format code and tidy both modules
.PHONY: tidy
tidy:
	go fmt ./...
	go mod tidy
	go -C tools mod tidy

# gosec rule classes excluded as false positives across the repository: none so far. Accept a single
# site with `// #nosec Gxxx -- reason` instead.
GOSEC_EXCLUDE =

# The dependency license allowlist as SPDX identifiers, the one jackfruit, noni and breadfruit share:
# permissive licenses plus the file-scoped weak copyleft CDDL and MPL. GPL, AGPL, LGPL and EPL are out
# by policy (Rafael, 2026-11-08). A dependency, test-only ones included, with an unlisted or
# unidentifiable license fails lint: review it, then extend the list. Every license a LICENSE file
# declares must be listed, so an "MIT or GPL" dependency fails too. The SDK has no third-party
# dependencies today and ships its own Apache-2.0 LICENSE, so nothing is ignored. The klog flags drop
# go-licenses' warnings about assembly files.
LICENSES_ALLOWED = 0BSD,Apache-2.0,BSD-2-Clause,BSD-3-Clause,CC0-1.0,MIT,MIT-0,CDDL-1.0,MPL-1.1,MPL-2.0
LICENSE_CHECK = go-licenses check ./... --allowed_licenses=$(LICENSES_ALLOWED) --include_tests --logtostderr=false --stderrthreshold=ERROR --log_file=/dev/null

# The integration build tag adds integration_test.go, so lint covers it without a server.
TAGS = -tags=integration

## lint: gofmt, go vet, staticcheck, govulncheck, gosec and the license allowlist (no server needed)
.PHONY: lint
lint:
	@unformatted=$$(gofmt -l .); if [ -n "$$unformatted" ]; then echo "not gofmt-formatted (run make tidy):"; echo "$$unformatted"; exit 1; fi
	go vet ./...
	go vet $(TAGS) ./...
	go $(GOTOOL) staticcheck $(TAGS) ./...
	go $(GOTOOL) govulncheck $(TAGS) ./...
	go $(GOTOOL) gosec -quiet -exclude=$(GOSEC_EXCLUDE) $(TAGS) ./...
	go $(GOTOOL) $(LICENSE_CHECK)

## audit: lint plus module hygiene and the race suite
.PHONY: audit
audit: lint test/ci

## test/ci: module hygiene and the unit race suite, what CI's test check runs (no server needed)
.PHONY: test/ci
test/ci:
	go mod tidy -diff
	go mod verify
	$(TOOLS_TIDY)
	go test -race -vet=off ./...

# tools/go.mod is go 1.27.3. CI's setup-go exports GOTOOLCHAIN=local, under which the Go 1.26 legs
# would refuse the tools module, so this one check lets go fetch the toolchain tools/go.mod names.
TOOLS_TIDY = GOTOOLCHAIN=auto go -C tools mod tidy -diff

## test/e2e: the integration tests against a scanii-cli server on port 4000, what CI's test/e2e check runs
.PHONY: test/e2e
test/e2e:
	go test -race -vet=off -count=1 $(TAGS) ./...
