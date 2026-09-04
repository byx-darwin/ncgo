package infra

import (
	"reflect"
	"testing"
)

func TestRateLimitPluginMetadata(t *testing.T) {
	p, ok := pluginByKind(KindRateLimit)
	if !ok {
		t.Fatal("rate_limit plugin not registered")
	}
	if !reflect.DeepEqual(p.Aliases(), []string{KindRateLimitAlias}) {
		t.Errorf("Aliases() = %v, want [%s]", p.Aliases(), KindRateLimitAlias)
	}
	if p.ServiceScope() != "kitex" {
		t.Errorf("ServiceScope() = %q, want kitex", p.ServiceScope())
	}
	want := []string{
		"review conf/dev/conf.yaml rate_limit block (source.type: config|database|rule_center)",
		"observe shadow logs (grep 'ratelimit shadow denied'), then set mode: enforce",
		"optional: set static.max_qps / static.max_connections for a global safety net",
		"go mod tidy",
	}
	if !reflect.DeepEqual(p.SetupSteps(), want) {
		t.Errorf("SetupSteps() = %v, want %v", p.SetupSteps(), want)
	}
}

func TestRateLimitPluginAssetFilesReturnsSharedFragmentsAndMiddleware(t *testing.T) {
	p, _ := pluginByKind(KindRateLimit)
	files, err := p.AssetFiles("kitex")
	if err != nil {
		t.Fatalf("AssetFiles: %v", err)
	}
	if len(files) != 5 {
		t.Fatalf("AssetFiles returned %d files, want 5 (4 shared fragments + 1 middleware)", len(files))
	}
	last := files[len(files)-1]
	if last.OutputRelPath == "" {
		t.Error("middleware template file missing OutputRelPath")
	}
}
