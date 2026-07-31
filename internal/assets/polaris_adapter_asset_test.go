package assets

import (
	"go/parser"
	"go/token"
	"io/fs"
	"strings"
	"testing"
)

func TestPolarisAdapterAssetSanity(t *testing.T) {
	b, err := fs.ReadFile(FS(), "kitex/optional/polaris_canary_adapter.go")
	if err != nil {
		t.Fatalf("read asset: %v", err)
	}
	src := string(b)

	// Must be valid Go (parses; imports need not resolve for parser).
	if _, err := parser.ParseFile(token.NewFileSet(), "polaris_canary_adapter.go", src, parser.ImportsOnly); err != nil {
		t.Fatalf("asset does not parse: %v", err)
	}
	for _, want := range []string{
		"package release",
		"func NewPolarisInstanceLister(",
		"func NewPolarisRuleLoader(",
		"func NewPolarisSelector(",
		"polarisAPI",
		"os.Getenv", // credentials via env only
	} {
		if !strings.Contains(src, want) {
			t.Errorf("asset missing %q", want)
		}
	}
	// No hardcoded credentials patterns.
	for _, bad := range []string{`"POLARIS_TOKEN_SECRET"`, "password = ", "token = \""} {
		if strings.Contains(src, bad) {
			t.Errorf("asset appears to hardcode credentials: %q", bad)
		}
	}
}
