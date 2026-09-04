package infra

import (
	"reflect"
	"strings"
	"testing"
)

func TestReleaseCanaryPluginMetadata(t *testing.T) {
	p, ok := pluginByKind(KindReleaseCanary)
	if !ok {
		t.Fatal("release_canary plugin not registered")
	}
	if !reflect.DeepEqual(p.Aliases(), []string{KindCanaryAlias}) {
		t.Errorf("Aliases() = %v, want [%s]", p.Aliases(), KindCanaryAlias)
	}
}

func TestReleaseCanaryPluginAssetFiles(t *testing.T) {
	p, _ := pluginByKind(KindReleaseCanary)
	files, err := p.AssetFiles("kitex")
	if err != nil {
		t.Fatalf("AssetFiles: %v", err)
	}
	want := []addOnFile{
		{SourcePath: "optional/release_canary.go", OutputRelPath: "internal/base/release/canary.go"},
		{SourcePath: "kitex/optional/release_canary.go", OutputRelPath: "internal/base/release/kitex.go"},
		{SourcePath: "optional/release_ops.go", OutputRelPath: "internal/base/release/ops.go"},
	}
	if !reflect.DeepEqual(files, want) {
		t.Errorf("AssetFiles = %+v, want %+v", files, want)
	}
}

func TestReleaseCanaryPluginWiresKitexClient(t *testing.T) {
	p, _ := pluginByKind(KindReleaseCanary)
	w, ok := p.(kitexClientWirer)
	if !ok {
		t.Fatal("release_canary plugin must implement kitexClientWirer")
	}
	src := "package client\n\nimport (\n\t\"context\"\n)\n\nfunc New() {\n\tif cfg.EnableMetaInfo {\n\t\toptions = append(options, kitexclient.WithMetaHandler(transmeta.ClientTTHeaderHandler))\n\t}\n}\n"
	var plan []PlanItem
	out, err := w.WireKitexClient(src, "example.com/mod", &plan)
	if err != nil {
		t.Fatalf("WireKitexClient: %v", err)
	}
	if !strings.Contains(out, "release.KitexTraffic()") {
		t.Errorf("WireKitexClient output missing release.KitexTraffic(): %s", out)
	}
}
