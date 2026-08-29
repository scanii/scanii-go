package scanii

import (
	"os"
	"strings"
	"testing"
)

// TestImportPathMatchesModulePath guards the invariant that ties the User-Agent
// to the module path. resolveVersion looks the SDK up in build info by matching
// dep.Path against importPath, so if go.mod's module path changes (a /vN major
// bump, an org rename) and importPath is not changed with it, the lookup misses
// and every consumer silently reports "scanii-go/v(devel)".
//
// This drift is invisible to the rest of the suite: tests run inside the module,
// where the version resolves through a different branch.
func TestImportPathMatchesModulePath(t *testing.T) {
	data, err := os.ReadFile("go.mod")
	if err != nil {
		t.Fatalf("read go.mod: %v", err)
	}

	var modulePath string
	for _, line := range strings.Split(string(data), "\n") {
		if rest, ok := strings.CutPrefix(strings.TrimSpace(line), "module "); ok {
			modulePath = strings.TrimSpace(rest)
			break
		}
	}
	if modulePath == "" {
		t.Fatal("no module directive found in go.mod")
	}

	if importPath != modulePath {
		t.Fatalf("importPath = %q, but go.mod declares %q; update the constant in client.go or the User-Agent will report (devel) for every consumer", importPath, modulePath)
	}
}
