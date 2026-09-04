package infra

import (
	"reflect"
	"testing"
)

func TestKafkaPluginMetadataMatchesLegacy(t *testing.T) {
	p, ok := pluginByKind(KindKafka)
	if !ok {
		t.Fatal("kafka plugin not registered")
	}
	wantDeps := []string{"github.com/byx-darwin/go-tools/go-middleware", "github.com/byx-darwin/go-tools/go-common", "github.com/byx-darwin/go-tools/go-framework"}
	if !reflect.DeepEqual(p.GoGetDeps(), wantDeps) {
		t.Errorf("GoGetDeps() = %v, want %v", p.GoGetDeps(), wantDeps)
	}
	if p.HertzConfigKey() != "kafka" {
		t.Errorf("HertzConfigKey() = %q, want kafka", p.HertzConfigKey())
	}
	if p.ServiceScope() != "common" {
		t.Errorf("ServiceScope() = %q, want common", p.ServiceScope())
	}
}

func TestKafkaPluginAssetFiles(t *testing.T) {
	p, _ := pluginByKind(KindKafka)
	files, err := p.AssetFiles("hertz")
	if err != nil {
		t.Fatalf("AssetFiles: %v", err)
	}
	want := []addOnFile{{SourcePath: "hertz/optional/kafka.go", OutputRelPath: "internal/base/data/kafka.go"}}
	if !reflect.DeepEqual(files, want) {
		t.Errorf("AssetFiles = %+v, want %+v", files, want)
	}
}
