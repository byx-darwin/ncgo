package framework

import (
	"context"
	"testing"

	"github.com/byx-darwin/ncgo/internal/exec"
	"github.com/byx-darwin/ncgo/internal/manifest"
)

// fakeAdapter implements Adapter with trivial stub values, exercising only
// the registry — it must NOT depend on hertz/kitex-specific behavior.
type fakeAdapter struct{ kind string }

func (f fakeAdapter) Kind() string { return f.kind }
func (f fakeAdapter) OptionalAssetPath(infraKind string) (string, bool) {
	return f.kind + "/optional/" + infraKind + ".go", true
}
func (f fakeAdapter) HertzConfigAssetPath(infraKind string) (string, bool) { return "", false }
func (f fakeAdapter) MergeHertzConfig(infraKind, hertzConfigKey string, snippet, current []byte, force bool) (*ConfigWrite, error) {
	return nil, nil
}
func (f fakeAdapter) MergeRateLimitConfig(current []byte, force bool) (*ConfigWrite, error) {
	return nil, nil
}
func (f fakeAdapter) DockerConfigBlocks(m *manifest.Manifest) []string { return nil }
func (f fakeAdapter) ContainerPort() int                               { return 0 }
func (f fakeAdapter) ComposeFeatures(withDatabase bool) ComposeFeatureFlags {
	return ComposeFeatureFlags{}
}
func (f fakeAdapter) IDLPath(opts GeneratorOptions) string      { return "idl/fake.proto" }
func (f fakeAdapter) IDLNameToken(opts GeneratorOptions) string { return opts.Name }
func (f fakeAdapter) GeneratorCommand(opts GeneratorOptions, idl string) string {
	return "fake-gen " + idl
}
func (f fakeAdapter) RenderIDLPlaceholder(opts GeneratorOptions) string { return "" }
func (f fakeAdapter) WriteIDLSupportFiles(dir string) error             { return nil }
func (f fakeAdapter) RunGenerator(ctx context.Context, r exec.Runner, dir string, opts GeneratorOptions, idl string) (exec.Result, error) {
	return exec.Result{}, nil
}
func (f fakeAdapter) RequiresSQLCBeforeTidy(withDatabase bool) bool { return withDatabase }
func (f fakeAdapter) ServerFilePath() string                        { return "internal/base/server/server.go" }

func TestRegistryRegisterAndGet(t *testing.T) {
	r := newRegistry()
	r.register(fakeAdapter{kind: "widget"})

	a, ok := r.get("widget")
	if !ok || a.Kind() != "widget" {
		t.Fatalf("get(widget) = %v, %v; want widget adapter", a, ok)
	}
	if _, ok := r.get("missing"); ok {
		t.Fatalf("get(missing) = ok=true; want ok=false")
	}
}

func TestRegistryRegisterDuplicateKindPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic on duplicate Kind() registration")
		}
	}()
	r := newRegistry()
	r.register(fakeAdapter{kind: "widget"})
	r.register(fakeAdapter{kind: "widget"})
}

func TestMustGetPanicsWhenMissing(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic from MustGet on unknown kind")
		}
	}()
	MustGet("does-not-exist")
}

func TestGetMissingReturnsFalse(t *testing.T) {
	if _, ok := Get("does-not-exist"); ok {
		t.Fatalf("Get(does-not-exist) = ok=true; want ok=false")
	}
}
