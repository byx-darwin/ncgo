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

// TestReapplyTemplateFiles_NoHyphenServiceName reproduces a regression where
// reapplyTemplateFiles deleted the very file it had just correctly rendered.
// Its trailing "clean up hyphenated files hz created" pass compares a
// hardcoded path (opts.Name + suffix) against the same path with hyphens
// replaced by underscores. When opts.Name has no hyphen (e.g. "scratch"),
// both paths are identical, so the "both files exist" check trivially
// matches itself and the file gets removed. See base-hertz/ratelimit-hertz
// in byx-darwin/ncgo-templates, which both use plain (non-hyphenated)
// service names and hit this exact case.
func TestReapplyTemplateFiles_NoHyphenServiceName(t *testing.T) {
	dir := t.TempDir()
	tplDir := filepath.Join(dir, "template", "hertz-template")
	if err := os.MkdirAll(tplDir, 0o755); err != nil {
		t.Fatalf("mkdir tplDir: %v", err)
	}

	handlerYAML := "path: internal/handler/pb/{{ToLower .ServiceName}}_service.go\n" +
		"update_behavior:\n  type: skip\n" +
		"loop_service: true\n" +
		"body: |-\n  package pb\n"
	if err := os.WriteFile(filepath.Join(tplDir, "handler_pb.yaml"), []byte(handlerYAML), 0o644); err != nil {
		t.Fatalf("write handler_pb.yaml: %v", err)
	}

	routerYAML := "path: internal/router/pb/{{ToLower .ServiceName}}.go\n" +
		"update_behavior:\n  type: cover\n" +
		"loop_service: true\n" +
		"body: |-\n  package pb\n"
	if err := os.WriteFile(filepath.Join(tplDir, "router_pb.yaml"), []byte(routerYAML), 0o644); err != nil {
		t.Fatalf("write router_pb.yaml: %v", err)
	}

	opts := Options{
		Kind:   manifest.KindHertz,
		Module: "github.com/acme/scratch",
		Name:   "scratch", // no hyphen: correctName == f for the cleanup pass
	}
	if err := reapplyTemplateFiles(dir, opts); err != nil {
		t.Fatalf("reapplyTemplateFiles: %v", err)
	}

	for _, rel := range []string{
		filepath.Join("internal", "handler", "pb", "scratch_service.go"),
		filepath.Join("internal", "router", "pb", "scratch.go"),
	} {
		full := filepath.Join(dir, rel)
		if _, err := os.Stat(full); err != nil {
			t.Errorf("expected %s to survive reapplyTemplateFiles, got: %v", rel, err)
		}
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
