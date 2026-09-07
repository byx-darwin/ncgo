package mono

import (
	"fmt"
	"io/fs"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/byx-darwin/ncgo/internal/assets"
)

// TestHzMiddlewareCallsJoinRateLimitKey guards against the class of drift
// reported alongside Issue #117 (Bug 2): a helper function defined in the hz
// layout template but never called anywhere in its own generated package,
// which lets an un-sanitized key slip through unnoticed. joinRateLimitKey is
// defined in the rate_limit.go layout entry but consumed by a same-package
// sibling file (idempotency.go) — a legitimate same-Go-package, cross-file
// call — so the check spans every layout entry under
// internal/pkg/middleware/, not a single entry's body. ncgo's own template
// is confirmed correct today; this test pins that so a future layout.yaml
// edit can't silently reintroduce a defined-but-uncalled helper without
// failing CI.
func TestHzMiddlewareCallsJoinRateLimitKey(t *testing.T) {
	srcFS := assets.FS()
	b, err := fs.ReadFile(srcFS, "hertz/layout.yaml")
	if err != nil {
		t.Fatalf("read hertz/layout.yaml: %v", err)
	}
	var doc struct {
		Layouts []struct {
			Path string `yaml:"path"`
			Body string `yaml:"body"`
		} `yaml:"layouts"`
	}
	if err := yaml.Unmarshal(b, &doc); err != nil {
		t.Fatalf("parse hertz/layout.yaml: %v", err)
	}
	const pkgDir = "internal/pkg/middleware/"
	var pkgBody strings.Builder
	found := false
	for _, l := range doc.Layouts {
		if strings.HasPrefix(l.Path, pkgDir) {
			pkgBody.WriteString(l.Body)
			pkgBody.WriteString("\n")
			found = true
		}
	}
	if !found {
		t.Fatalf("no layout entries found under %q", pkgDir)
	}
	body := pkgBody.String()
	const helper = "joinRateLimitKey"
	if !strings.Contains(body, fmt.Sprintf("func %s(", helper)) {
		t.Errorf("%s: %s is not defined anywhere under %q", pkgDir, helper, pkgDir)
	}
	if strings.Count(body, helper+"(") < 2 {
		t.Errorf("%s: %s is defined but never called anywhere under %q (want >= 2 occurrences: 1 definition + >= 1 call site, got %d) — this is the exact drift pattern flagged in Issue #117 (Bug 2)",
			pkgDir, helper, pkgDir, strings.Count(body, helper+"("))
	}
}
