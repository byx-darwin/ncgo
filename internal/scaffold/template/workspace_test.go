package template

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOverlayWorkspaceTemplates(t *testing.T) {
	pkgDir := t.TempDir()
	os.MkdirAll(filepath.Join(pkgDir, "workspace", "scripts"), 0o755)
	os.WriteFile(filepath.Join(pkgDir, "workspace", "compose.yaml.tpl"),
		[]byte("name: {{.ServiceName}}\nmodule: {{.Module}}\n"), 0o644)
	os.WriteFile(filepath.Join(pkgDir, "workspace", "scripts", "build.sh.tpl"),
		[]byte("#!/bin/sh\n# {{.ServiceName}} build\n"), 0o644)
	os.WriteFile(filepath.Join(pkgDir, "workspace", "extra.txt"),
		[]byte("verbatim copy\n"), 0o644)

	pkg := &Package{
		Dir:         pkgDir,
		TemplateDir: filepath.Join(pkgDir, "workspace"),
	}

	targetDir := t.TempDir()
	os.WriteFile(filepath.Join(targetDir, "compose.yaml"), []byte("original\n"), 0o644)

	data := RenderData{ServiceName: "shop", Module: "github.com/acme/shop"}
	if err := OverlayWorkspaceTemplates(targetDir, pkg, data); err != nil {
		t.Fatalf("OverlayWorkspaceTemplates: %v", err)
	}

	compose, err := os.ReadFile(filepath.Join(targetDir, "compose.yaml"))
	if err != nil {
		t.Fatalf("read compose: %v", err)
	}
	if !strings.Contains(string(compose), "name: shop") || !strings.Contains(string(compose), "module: github.com/acme/shop") {
		t.Errorf("compose not rendered: %s", compose)
	}

	build, err := os.ReadFile(filepath.Join(targetDir, "scripts", "build.sh"))
	if err != nil {
		t.Fatalf("read build.sh: %v", err)
	}
	if !strings.Contains(string(build), "shop build") {
		t.Errorf("build.sh not rendered: %s", build)
	}

	extra, err := os.ReadFile(filepath.Join(targetDir, "extra.txt"))
	if err != nil {
		t.Fatalf("read extra: %v", err)
	}
	if string(extra) != "verbatim copy\n" {
		t.Errorf("extra.txt = %q, want verbatim", extra)
	}
}

func TestOverlayWorkspaceTemplatesMissingDir(t *testing.T) {
	pkgDir := t.TempDir()
	pkg := &Package{Dir: pkgDir, TemplateDir: filepath.Join(pkgDir, "workspace")}
	err := OverlayWorkspaceTemplates(t.TempDir(), pkg, RenderData{})
	if err == nil || !strings.Contains(err.Error(), "workspace") {
		t.Errorf("want missing workspace error, got %v", err)
	}
}

func TestOverlayWorkspaceTemplatesEmpty(t *testing.T) {
	pkgDir := t.TempDir()
	os.MkdirAll(filepath.Join(pkgDir, "workspace"), 0o755)
	pkg := &Package{Dir: pkgDir, TemplateDir: filepath.Join(pkgDir, "workspace")}
	targetDir := t.TempDir()
	os.WriteFile(filepath.Join(targetDir, "existing"), []byte("keep\n"), 0o644)
	if err := OverlayWorkspaceTemplates(targetDir, pkg, RenderData{}); err != nil {
		t.Fatalf("empty overlay should not error: %v", err)
	}
	if _, err := os.Stat(filepath.Join(targetDir, "existing")); err != nil {
		t.Errorf("existing file lost: %v", err)
	}
}
