package infra

import (
	"reflect"
	"testing"
)

func TestESPluginMetadataMatchesLegacy(t *testing.T) {
	p, ok := pluginByKind(KindES)
	if !ok {
		t.Fatal("es plugin not registered")
	}
	wantDeps := []string{"github.com/byx-darwin/go-tools/go-middleware", "github.com/byx-darwin/go-tools/go-common", "github.com/byx-darwin/go-tools/go-framework"}
	if !reflect.DeepEqual(p.GoGetDeps(), wantDeps) {
		t.Errorf("GoGetDeps() = %v, want %v", p.GoGetDeps(), wantDeps)
	}
	if p.HertzConfigKey() != "es" {
		t.Errorf("HertzConfigKey() = %q, want es", p.HertzConfigKey())
	}
}

func TestESPluginAssetFiles(t *testing.T) {
	p, _ := pluginByKind(KindES)
	files, err := p.AssetFiles("kitex")
	if err != nil {
		t.Fatalf("AssetFiles: %v", err)
	}
	want := []addOnFile{{SourcePath: "kitex/optional/es.go", OutputRelPath: "internal/base/data/es.go"}}
	if !reflect.DeepEqual(files, want) {
		t.Errorf("AssetFiles = %+v, want %+v", files, want)
	}
}
