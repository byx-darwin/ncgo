package infra

import (
	"reflect"
	"testing"

	"github.com/byx-darwin/ncgo/internal/manifest"
)

func TestRedisPluginMetadataMatchesLegacy(t *testing.T) {
	p, ok := pluginByKind(KindRedis)
	if !ok {
		t.Fatal("redis plugin not registered")
	}
	if p.Kind() != KindRedis {
		t.Errorf("Kind() = %q, want %q", p.Kind(), KindRedis)
	}
	if len(p.Aliases()) != 0 {
		t.Errorf("Aliases() = %v, want empty", p.Aliases())
	}
	if p.ServiceScope() != "common" {
		t.Errorf("ServiceScope() = %q, want common", p.ServiceScope())
	}
	wantDeps := []string{"github.com/byx-darwin/go-tools/go-middleware", "github.com/byx-darwin/go-tools/go-common"}
	if !reflect.DeepEqual(p.GoGetDeps(), wantDeps) {
		t.Errorf("GoGetDeps() = %v, want %v", p.GoGetDeps(), wantDeps)
	}
	if p.SetupSteps() != nil {
		t.Errorf("SetupSteps() = %v, want nil (default derivation)", p.SetupSteps())
	}
	if p.HertzConfigKey() != "redis" {
		t.Errorf("HertzConfigKey() = %q, want redis", p.HertzConfigKey())
	}
}

func TestRedisPluginAssetFiles(t *testing.T) {
	p, _ := pluginByKind(KindRedis)
	hertzFiles, err := p.AssetFiles(manifest.KindHertz)
	if err != nil {
		t.Fatalf("AssetFiles(hertz): %v", err)
	}
	want := []addOnFile{{SourcePath: "hertz/optional/redis.go", OutputRelPath: "internal/base/data/redis.go"}}
	if !reflect.DeepEqual(hertzFiles, want) {
		t.Errorf("AssetFiles(hertz) = %+v, want %+v", hertzFiles, want)
	}
	kitexFiles, err := p.AssetFiles(manifest.KindKitex)
	if err != nil {
		t.Fatalf("AssetFiles(kitex): %v", err)
	}
	want = []addOnFile{{SourcePath: "kitex/optional/redis.go", OutputRelPath: "internal/base/data/redis.go"}}
	if !reflect.DeepEqual(kitexFiles, want) {
		t.Errorf("AssetFiles(kitex) = %+v, want %+v", kitexFiles, want)
	}
}

func TestRedisPluginExtraFilesAddsHertzSharedHelperOnce(t *testing.T) {
	p, _ := pluginByKind(KindRedis)
	ep, ok := p.(extraFilesPlugin)
	if !ok {
		t.Fatal("redis plugin must implement extraFilesPlugin")
	}
	root := seedProject(t, nil)
	files, err := ep.ExtraFiles(root, manifest.KindHertz)
	if err != nil {
		t.Fatalf("ExtraFiles: %v", err)
	}
	want := []addOnFile{{SourcePath: "hertz/optional/redis_shared.go", OutputRelPath: "internal/base/data/redis_shared.go"}}
	if !reflect.DeepEqual(files, want) {
		t.Errorf("ExtraFiles = %+v, want %+v", files, want)
	}
	files, err = ep.ExtraFiles(root, manifest.KindKitex)
	if err != nil {
		t.Fatalf("ExtraFiles(kitex): %v", err)
	}
	if len(files) != 0 {
		t.Errorf("ExtraFiles(kitex) = %+v, want empty (kitex has no shared redis helper)", files)
	}
}
