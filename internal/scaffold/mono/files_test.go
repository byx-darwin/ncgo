package mono

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/byx-darwin/ncgo/internal/manifest"
)

func TestExpandIncludes(t *testing.T) {
	fragment := "# shared\npath: internal/pkg/ratelimit/resolver.go\nupdate_behavior:\n  type: cover\nbody: |-\n  package ratelimit\n\n  // module {{.Module}}/internal/base/conf\n"
	fsys := fstest.MapFS{
		"ratelimit/resolver.yaml": &fstest.MapFile{Data: []byte(fragment)},
	}
	layout := "layouts:\n  - path: internal/handler/\n    delims: [\"\", \"\"]\n    body: \"\"\n  # {{include: ratelimit/resolver}}\n  - path: internal/usecase/\n    delims: [\"\", \"\"]\n    body: \"\"\n"

	out, err := expandIncludes([]byte(layout), fsys)
	if err != nil {
		t.Fatalf("expandIncludes: %v", err)
	}
	got := string(out)

	wantEntry := "  - path: internal/pkg/ratelimit/resolver.go\n    delims: [\"{{\", \"}}\"]\n    body: |-\n      package ratelimit\n\n      // module {{.GoModule}}/internal/base/conf\n"
	if !strings.Contains(got, wantEntry) {
		t.Errorf("expanded entry mismatch\ngot:\n%s\nwant substring:\n%s", got, wantEntry)
	}
	if strings.Contains(got, "{{include:") {
		t.Errorf("directive not consumed:\n%s", got)
	}
	if !strings.Contains(got, "  - path: internal/usecase/") {
		t.Errorf("following entries lost:\n%s", got)
	}
}

func TestExpandIncludesMissingFragment(t *testing.T) {
	fsys := fstest.MapFS{}
	_, err := expandIncludes([]byte("layouts:\n  # {{include: ratelimit/missing}}\n"), fsys)
	if err == nil || !strings.Contains(err.Error(), "ratelimit/missing") {
		t.Fatalf("want missing-fragment error, got %v", err)
	}
}

func TestWriteKitexTemplate_SkipDefaultTemplates(t *testing.T) {
	dir := t.TempDir()
	opts := Options{
		Kind:                 manifest.KindKitex,
		Module:               "example.com/test",
		Name:                 "test",
		SkipDefaultTemplates: []string{"handler.yaml", "server.yaml"},
	}
	if err := writeKitexTemplate(dir, opts); err != nil {
		t.Fatalf("writeKitexTemplate: %v", err)
	}
	tplDir := filepath.Join(dir, "template", "kitex-template")
	if _, err := os.Stat(filepath.Join(tplDir, "handler.yaml")); err == nil {
		t.Error("handler.yaml should be skipped")
	}
	if _, err := os.Stat(filepath.Join(tplDir, "server.yaml")); err == nil {
		t.Error("server.yaml should be skipped")
	}
	if _, err := os.Stat(filepath.Join(tplDir, "usecase.yaml")); err != nil {
		t.Error("usecase.yaml should exist")
	}
}
