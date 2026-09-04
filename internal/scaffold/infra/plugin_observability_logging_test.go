package infra

import (
	"reflect"
	"strings"
	"testing"
)

func TestObservabilityLoggingPluginMetadata(t *testing.T) {
	p, ok := pluginByKind(KindObservabilityLog)
	if !ok {
		t.Fatal("observability_logging plugin not registered")
	}
	if !reflect.DeepEqual(p.Aliases(), []string{KindLoggingAlias}) {
		t.Errorf("Aliases() = %v, want [%s]", p.Aliases(), KindLoggingAlias)
	}
	byAlias, ok := pluginByKind(KindLoggingAlias)
	if !ok || byAlias.Kind() != KindObservabilityLog {
		t.Errorf("pluginByKind(%s) = %v, %v; want observability_logging plugin", KindLoggingAlias, byAlias, ok)
	}
	wantDeps := []string{"github.com/byx-darwin/go-tools/go-common"}
	if !reflect.DeepEqual(p.GoGetDeps(), wantDeps) {
		t.Errorf("GoGetDeps() = %v, want %v", p.GoGetDeps(), wantDeps)
	}
	if p.HertzConfigKey() != "" {
		t.Errorf("HertzConfigKey() = %q, want empty", p.HertzConfigKey())
	}
}

func TestObservabilityLoggingPluginAssetFiles(t *testing.T) {
	p, _ := pluginByKind(KindObservabilityLog)
	files, err := p.AssetFiles("hertz")
	if err != nil {
		t.Fatalf("AssetFiles(hertz): %v", err)
	}
	want := []addOnFile{
		{SourcePath: "optional/observability_logging.go", OutputRelPath: "internal/base/logging/logging.go"},
		{SourcePath: "hertz/optional/observability_logging.go", OutputRelPath: "internal/base/logging/hertz.go"},
	}
	if !reflect.DeepEqual(files, want) {
		t.Errorf("AssetFiles(hertz) = %+v, want %+v", files, want)
	}
}

func TestObservabilityLoggingPluginWiresHertzServer(t *testing.T) {
	p, _ := pluginByKind(KindObservabilityLog)
	w, ok := p.(hertzServerWirer)
	if !ok {
		t.Fatal("observability_logging plugin must implement hertzServerWirer")
	}
	src := "package server\n\nimport (\n\t\"github.com/x/demo/internal/pkg/middleware\"\n)\n\nfunc New() {\n\tdo.ProvideValue(injector, cfg)\n\th.Use(middleware.Recovery())\n\th.Use(middleware.RequestID())\n\th.Use(middleware.AccessLog())\n}\n"
	var plan []PlanItem
	out, err := w.WireHertzServer(src, "example.com/mod", &plan)
	if err != nil {
		t.Fatalf("WireHertzServer: %v", err)
	}
	if !containsAll(out, []string{"logging.Init(", "logging.HertzRecovery()", "logging.HertzRequestID()", "logging.HertzAccessLog()"}) {
		t.Errorf("WireHertzServer output missing expected logging wiring: %s", out)
	}
	if len(plan) == 0 {
		t.Error("WireHertzServer should record plan items")
	}
}

func containsAll(s string, subs []string) bool {
	for _, sub := range subs {
		if !strings.Contains(s, sub) {
			return false
		}
	}
	return true
}
