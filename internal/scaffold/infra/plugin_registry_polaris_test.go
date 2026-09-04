package infra

import (
	"reflect"
	"testing"

	"github.com/byx-darwin/ncgo/internal/manifest"
)

func TestRegistryPolarisPluginMetadata(t *testing.T) {
	p, ok := pluginByKind(KindRegistryPolaris)
	if !ok {
		t.Fatal("registry_polaris plugin not registered")
	}
	if p.ServiceScope() != "kitex" {
		t.Errorf("ServiceScope() = %q, want kitex", p.ServiceScope())
	}
	wantDeps := []string{"github.com/kitex-contrib/polaris", "github.com/byx-darwin/go-tools/go-common"}
	if !reflect.DeepEqual(p.GoGetDeps(), wantDeps) {
		t.Errorf("GoGetDeps() = %v, want %v", p.GoGetDeps(), wantDeps)
	}
}

func TestRegistryPolarisPluginAssetFiles(t *testing.T) {
	p, _ := pluginByKind(KindRegistryPolaris)
	files, err := p.AssetFiles(manifest.KindKitex)
	if err != nil {
		t.Fatalf("AssetFiles(kitex): %v", err)
	}
	want := []addOnFile{
		{SourcePath: "kitex/optional/registry_polaris.go", OutputRelPath: "internal/base/registry/polaris.go"},
		{SourcePath: "kitex/optional/registry_polaris.yaml", OutputRelPath: "polaris.yaml"},
	}
	if !reflect.DeepEqual(files, want) {
		t.Errorf("AssetFiles(kitex) = %+v, want %+v", files, want)
	}
	if _, err := p.AssetFiles(manifest.KindHertz); err == nil {
		t.Error("AssetFiles(hertz) should error: registry_polaris is kitex-only")
	}
}
