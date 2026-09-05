package framework

import (
	"testing"

	"github.com/byx-darwin/ncgo/internal/manifest"
)

func TestHertzAdapterRegistered(t *testing.T) {
	a, ok := Get(manifest.KindHertz)
	if !ok || a.Kind() != manifest.KindHertz {
		t.Fatalf("Get(hertz) = %v, %v; want hertz adapter", a, ok)
	}
}

func TestHertzAdapterContainerPort(t *testing.T) {
	if got := (hertzAdapter{}).ContainerPort(); got != 8080 {
		t.Fatalf("ContainerPort() = %d, want 8080", got)
	}
}

func TestHertzAdapterIDLPath(t *testing.T) {
	got := (hertzAdapter{}).IDLPath(GeneratorOptions{Name: "demo", Module: "github.com/x/demo"})
	want := "idl/app/demo.proto"
	if got != want {
		t.Fatalf("IDLPath = %q, want %q", got, want)
	}
}

func TestHertzAdapterIDLNameToken(t *testing.T) {
	got := (hertzAdapter{}).IDLNameToken(GeneratorOptions{Name: "Demo"})
	if got != "demo" {
		t.Fatalf("IDLNameToken = %q, want %q", got, "demo")
	}
}

func TestHertzAdapterRequiresSQLCBeforeTidy(t *testing.T) {
	a := hertzAdapter{}
	if a.RequiresSQLCBeforeTidy(false) {
		t.Fatal("RequiresSQLCBeforeTidy(false) = true, want false")
	}
	if !a.RequiresSQLCBeforeTidy(true) {
		t.Fatal("RequiresSQLCBeforeTidy(true) = false, want true")
	}
}

func TestHertzAdapterOptionalAssetPath(t *testing.T) {
	path, ok := (hertzAdapter{}).OptionalAssetPath("redis")
	if !ok || path != "hertz/optional/redis.go" {
		t.Fatalf("OptionalAssetPath(redis) = %q, %v; want hertz/optional/redis.go, true", path, ok)
	}
}

func TestHertzAdapterHertzConfigAssetPath(t *testing.T) {
	path, ok := (hertzAdapter{}).HertzConfigAssetPath("redis")
	if !ok || path != "hertz/optional-config/redis.yaml" {
		t.Fatalf("HertzConfigAssetPath(redis) = %q, %v; want hertz/optional-config/redis.yaml, true", path, ok)
	}
}

func TestHertzAdapterComposeFeatures(t *testing.T) {
	a := hertzAdapter{}
	got := a.ComposeFeatures(false)
	want := ComposeFeatureFlags{Nacos: true, Polaris: true, Vegeta: false}
	if got != want {
		t.Fatalf("ComposeFeatures(false) = %+v, want %+v", got, want)
	}
	got = a.ComposeFeatures(true)
	want = ComposeFeatureFlags{Nacos: true, Polaris: true, Vegeta: true}
	if got != want {
		t.Fatalf("ComposeFeatures(true) = %+v, want %+v", got, want)
	}
}

func TestHertzAdapterGeneratorCommand(t *testing.T) {
	got := (hertzAdapter{}).GeneratorCommand(GeneratorOptions{Module: "github.com/x/demo"}, "idl/app/demo.proto")
	want := "hz new --mod=github.com/x/demo --idl=idl/app/demo.proto -I idl --handler_dir=internal/handler --model_dir=internal/pb --router_dir=internal/router --customize_layout=template/layout.yaml --customize_layout_data_path=template/data.json --customize_package=template/package.yaml"
	if got != want {
		t.Fatalf("GeneratorCommand = %q, want %q", got, want)
	}
}

func TestHertzAdapterMergeHertzConfigCreatesWhenAbsent(t *testing.T) {
	write, err := (hertzAdapter{}).MergeHertzConfig("redis", "redis", []byte("addrs:\n  - localhost:6379\n"), nil, false)
	if err != nil {
		t.Fatalf("MergeHertzConfig: %v", err)
	}
	if write == nil || write.Action != "create" {
		t.Fatalf("MergeHertzConfig = %+v, want Action=create", write)
	}
}

func TestHertzAdapterMergeHertzConfigSkipsExistingKey(t *testing.T) {
	current := []byte("redis:\n  addrs:\n    - localhost:6379\n")
	write, err := (hertzAdapter{}).MergeHertzConfig("redis", "redis", []byte("addrs:\n  - localhost:6379\n"), current, false)
	if err != nil {
		t.Fatalf("MergeHertzConfig: %v", err)
	}
	if write != nil {
		t.Fatalf("MergeHertzConfig = %+v, want nil (key already present)", write)
	}
}

func TestHertzAdapterMergeRateLimitConfigIsNoop(t *testing.T) {
	write, err := (hertzAdapter{}).MergeRateLimitConfig(nil, false)
	if err != nil || write != nil {
		t.Fatalf("MergeRateLimitConfig = %+v, %v; want nil, nil", write, err)
	}
}
