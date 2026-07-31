package assets

import (
	"go/parser"
	"go/token"
	"io/fs"
	"regexp"
	"strings"
	"testing"
)

// TestKitexCanaryLBAssetHasDiscovererSeam locks the kitex release-canary LB
// adapter's support for the full-pool Discoverer seam (Task 5). The asset is
// kitex-coupled (imports kitex packages) so it is not compile-checked by CI;
// this parse + substring test is the guard-rail.
func TestKitexCanaryLBAssetHasDiscovererSeam(t *testing.T) {
	b, err := fs.ReadFile(FS(), "kitex/optional/release_canary.go")
	if err != nil {
		t.Fatalf("read asset: %v", err)
	}
	src := string(b)
	if _, err := parser.ParseFile(token.NewFileSet(), "release_canary.go", src, parser.ImportsOnly); err != nil {
		t.Fatalf("asset does not parse: %v", err)
	}
	// Exact substring checks (not affected by gofmt alignment).
	for _, want := range []string{"NewEngine(", "KitexResultDiscoverer", "kitexInstanceFromRelease", "parseReleaseAddr"} {
		if !strings.Contains(src, want) {
			t.Errorf("kitex canary LB asset missing %q", want)
		}
	}
	// Whitespace-agnostic struct field checks (gofmt aligns columns).
	for _, pattern := range []string{`Discoverer\s+Discoverer`, `Observer\s+Observer`} {
		if !regexp.MustCompile(pattern).MatchString(src) {
			t.Errorf("kitex canary LB asset missing field matching %q", pattern)
		}
	}
}
