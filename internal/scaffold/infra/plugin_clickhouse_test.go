package infra

import (
	"reflect"
	"testing"
)

func TestClickHousePluginMetadataMatchesLegacy(t *testing.T) {
	p, ok := pluginByKind(KindClickHouse)
	if !ok {
		t.Fatal("clickhouse plugin not registered")
	}
	wantDeps := []string{"github.com/byx-darwin/go-tools/go-middleware", "github.com/byx-darwin/go-tools/go-common", "github.com/byx-darwin/go-tools/go-framework"}
	if !reflect.DeepEqual(p.GoGetDeps(), wantDeps) {
		t.Errorf("GoGetDeps() = %v, want %v", p.GoGetDeps(), wantDeps)
	}
	if p.HertzConfigKey() != "clickhouse" {
		t.Errorf("HertzConfigKey() = %q, want clickhouse", p.HertzConfigKey())
	}
}

func TestClickHousePluginAssetFiles(t *testing.T) {
	p, _ := pluginByKind(KindClickHouse)
	files, err := p.AssetFiles("hertz")
	if err != nil {
		t.Fatalf("AssetFiles: %v", err)
	}
	want := []addOnFile{{SourcePath: "hertz/optional/clickhouse.go", OutputRelPath: "internal/base/data/clickhouse.go"}}
	if !reflect.DeepEqual(files, want) {
		t.Errorf("AssetFiles = %+v, want %+v", files, want)
	}
}
