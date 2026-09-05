package framework

import (
	"strings"
	"testing"

	"github.com/byx-darwin/ncgo/internal/manifest"
)

func TestKitexAdapterRegistered(t *testing.T) {
	a, ok := Get(manifest.KindKitex)
	if !ok || a.Kind() != manifest.KindKitex {
		t.Fatalf("Get(kitex) = %v, %v; want kitex adapter", a, ok)
	}
}

func TestKitexAdapterContainerPort(t *testing.T) {
	if got := (kitexAdapter{}).ContainerPort(); got != 8888 {
		t.Fatalf("ContainerPort() = %d, want 8888", got)
	}
}

func TestKitexAdapterIDLPath(t *testing.T) {
	got := (kitexAdapter{}).IDLPath(GeneratorOptions{Name: "user-api", Module: "github.com/x/user-api"})
	want := "idl/userapi.proto"
	if got != want {
		t.Fatalf("IDLPath = %q, want %q", got, want)
	}
}

func TestKitexAdapterIDLNameToken(t *testing.T) {
	got := (kitexAdapter{}).IDLNameToken(GeneratorOptions{Name: "user-api"})
	if got != "userapi" {
		t.Fatalf("IDLNameToken = %q, want %q", got, "userapi")
	}
}

func TestKitexAdapterRequiresSQLCBeforeTidyAlwaysTrue(t *testing.T) {
	a := kitexAdapter{}
	if !a.RequiresSQLCBeforeTidy(false) {
		t.Fatal("RequiresSQLCBeforeTidy(false) = false, want true (kitex always requires sqlc-before-tidy)")
	}
	if !a.RequiresSQLCBeforeTidy(true) {
		t.Fatal("RequiresSQLCBeforeTidy(true) = false, want true")
	}
}

func TestKitexAdapterHertzConfigAssetPathAlwaysFalse(t *testing.T) {
	path, ok := (kitexAdapter{}).HertzConfigAssetPath("redis")
	if ok || path != "" {
		t.Fatalf("HertzConfigAssetPath(redis) = %q, %v; want \"\", false", path, ok)
	}
}

func TestKitexAdapterComposeFeaturesAllFalse(t *testing.T) {
	got := (kitexAdapter{}).ComposeFeatures(true)
	want := ComposeFeatureFlags{}
	if got != want {
		t.Fatalf("ComposeFeatures(true) = %+v, want %+v", got, want)
	}
}

func TestKitexAdapterGeneratorCommand(t *testing.T) {
	got := (kitexAdapter{}).GeneratorCommand(GeneratorOptions{Module: "github.com/x/demo"}, "idl/demo.proto")
	want := "kitex -module github.com/x/demo -template-dir template/kitex-template -type protobuf idl/demo.proto"
	if got != want {
		t.Fatalf("GeneratorCommand = %q, want %q", got, want)
	}
}

func TestKitexAdapterMergeRateLimitConfigCreatesWhenAbsent(t *testing.T) {
	write, err := (kitexAdapter{}).MergeRateLimitConfig(nil, false)
	if err != nil {
		t.Fatalf("MergeRateLimitConfig: %v", err)
	}
	if write == nil || write.Action != "create" {
		t.Fatalf("MergeRateLimitConfig = %+v, want Action=create", write)
	}
}

func TestKitexAdapterMergeRateLimitConfigFlipsExistingBlock(t *testing.T) {
	current := []byte("rate_limit:\n  enabled: false\n  mode: enforce\n")
	write, err := (kitexAdapter{}).MergeRateLimitConfig(current, false)
	if err != nil {
		t.Fatalf("MergeRateLimitConfig: %v", err)
	}
	if write == nil || write.Action != "update" {
		t.Fatalf("MergeRateLimitConfig = %+v, want Action=update", write)
	}
}

func TestKitexAdapterMergeHertzConfigIsNoop(t *testing.T) {
	write, err := (kitexAdapter{}).MergeHertzConfig("redis", "redis", nil, nil, false)
	if err != nil || write != nil {
		t.Fatalf("MergeHertzConfig = %+v, %v; want nil, nil", write, err)
	}
}

// TestMergeKitexRateLimitConfigPreservesNestedKeys exercises the I5 fix:
// top-level enabled:/mode: are flipped, but nested enabled:/mode: keys inside
// sub-blocks like pre_auth: must be left untouched. Moved verbatim from
// internal/scaffold/infra/infra_test.go when mergeKitexRateLimitConfig's
// infra.go copy was deleted as dead code (superseded by this adapter).
func TestMergeKitexRateLimitConfigPreservesNestedKeys(t *testing.T) {
	src := `env: dev
rate_limit:
  enabled: false
  mode: enforce
  backend: memory
  pre_auth:
    enabled: false
    default_rule:
      enabled: true
      mode: strict
  post_auth:
    enabled: false
`
	merged, changed := mergeKitexRateLimitConfig(src)
	if !changed {
		t.Fatalf("expected merge to report changed")
	}
	// Top-level keys flipped:
	if !strings.Contains(merged, "  enabled: true\n") {
		t.Errorf("expected top-level enabled: true, got:\n%s", merged)
	}
	if !strings.Contains(merged, "  mode: shadow\n") {
		t.Errorf("expected top-level mode: shadow, got:\n%s", merged)
	}
	// Nested pre_auth.enabled must remain false:
	if !strings.Contains(merged, "    enabled: false\n") {
		t.Errorf("expected nested pre_auth.enabled: false preserved, got:\n%s", merged)
	}
	// Nested post_auth.enabled must remain false:
	if strings.Count(merged, "    enabled: false\n") < 2 {
		t.Errorf("expected both nested enabled: false preserved, got:\n%s", merged)
	}
	// Nested default_rule.enabled must remain true (untouched):
	if !strings.Contains(merged, "      enabled: true\n") {
		t.Errorf("expected nested default_rule.enabled: true preserved, got:\n%s", merged)
	}
	// Nested mode: strict must remain (not flipped to shadow):
	if !strings.Contains(merged, "      mode: strict\n") {
		t.Errorf("expected nested mode: strict preserved, got:\n%s", merged)
	}
	// Backend preserved:
	if !strings.Contains(merged, "  backend: memory\n") {
		t.Errorf("expected backend preserved, got:\n%s", merged)
	}
}
