package assets

import (
	"go/parser"
	"go/token"
	"io/fs"
	"strings"
	"testing"
)

func TestReleaseOpsAssetSanity(t *testing.T) {
	b, err := fs.ReadFile(FS(), "optional/release_ops.go")
	if err != nil {
		t.Fatalf("read asset: %v", err)
	}
	src := string(b)
	if _, err := parser.ParseFile(token.NewFileSet(), "release_ops.go", src, parser.ImportsOnly); err != nil {
		t.Fatalf("asset does not parse: %v", err)
	}
	for _, want := range []string{"package release", "func NewCachingDiscoverer(", "func NewCachingRuleProvider(", "type Observer interface", "func NewSlogObserver(", "func NewEngine(", "FailOpen", "FailFast"} {
		if !strings.Contains(src, want) {
			t.Errorf("release_ops.go missing %q", want)
		}
	}
	// SDK-neutral: must not import polaris-go or otel.
	for _, bad := range []string{"polarismesh/polaris-go", "go.opentelemetry.io"} {
		if strings.Contains(src, bad) {
			t.Errorf("release_ops.go must stay dependency-neutral, found %q", bad)
		}
	}
}

func TestPolarisOTelObserverAssetSanity(t *testing.T) {
	b, err := fs.ReadFile(FS(), "kitex/optional/polaris_canary_observer_otel.go")
	if err != nil {
		t.Fatalf("read asset: %v", err)
	}
	src := string(b)
	if _, err := parser.ParseFile(token.NewFileSet(), "polaris_canary_observer_otel.go", src, parser.ImportsOnly); err != nil {
		t.Fatalf("asset does not parse: %v", err)
	}
	for _, want := range []string{"package release", "func NewOTelObserver(", "go.opentelemetry.io/otel/metric"} {
		if !strings.Contains(src, want) {
			t.Errorf("observer asset missing %q", want)
		}
	}
	for _, bad := range []string{"POLARIS_TOKEN", "password", "token = \""} {
		if strings.Contains(src, bad) {
			t.Errorf("observer asset appears to leak credentials: %q", bad)
		}
	}
}
