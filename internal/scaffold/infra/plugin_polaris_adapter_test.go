package infra

import (
	"reflect"
	"testing"

	"github.com/byx-darwin/ncgo/internal/manifest"
)

func TestPolarisAdapterPluginMetadata(t *testing.T) {
	p, ok := pluginByKind(KindPolarisAdapter)
	if !ok {
		t.Fatal("polaris_adapter plugin not registered")
	}
	if !reflect.DeepEqual(p.Aliases(), []string{KindPolarisAdapterAlias}) {
		t.Errorf("Aliases() = %v, want [%s]", p.Aliases(), KindPolarisAdapterAlias)
	}
	want := []string{
		"go get github.com/polarismesh/polaris-go",
		"go get gopkg.in/yaml.v3",
		"go get github.com/byx-darwin/go-tools/go-common",
		"go get go.opentelemetry.io/otel/metric",
		"set POLARIS_TOKEN / POLARIS_NAMESPACE env vars (never hardcode credentials)",
		"wire release.NewPolarisSelector(...) into KitexCanaryLoadBalancer.RuleProvider",
		"verify kitex resolver returns full stable+canary instance set (see troubleshooting)",
		"go mod tidy",
	}
	if !reflect.DeepEqual(p.SetupSteps(), want) {
		t.Errorf("SetupSteps() = %v, want %v", p.SetupSteps(), want)
	}
	if _, ok := p.(hertzServerWirer); ok {
		t.Error("polaris_adapter must not implement hertzServerWirer")
	}
	if _, ok := p.(kitexServerWirer); ok {
		t.Error("polaris_adapter must not implement kitexServerWirer (no --wire support today)")
	}
}

func TestPolarisAdapterPluginAssetFiles(t *testing.T) {
	p, _ := pluginByKind(KindPolarisAdapter)
	files, err := p.AssetFiles(manifest.KindKitex)
	if err != nil {
		t.Fatalf("AssetFiles: %v", err)
	}
	want := []addOnFile{
		{SourcePath: "kitex/optional/polaris_canary_adapter.go", OutputRelPath: "internal/base/release/polaris_adapter.go"},
		{SourcePath: "kitex/optional/polaris_canary_observer_otel.go", OutputRelPath: "internal/base/release/polaris_observer_otel.go"},
	}
	if !reflect.DeepEqual(files, want) {
		t.Errorf("AssetFiles = %+v, want %+v", files, want)
	}
}
