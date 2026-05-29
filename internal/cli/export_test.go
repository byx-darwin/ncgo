package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"

	"github.com/byx-darwin/ncgo/internal/manifest"
)

// seedExportProject creates a minimal Hertz project with source files
// that match export template rules.
func seedExportProject(t *testing.T, kind string) string {
	t.Helper()
	root := t.TempDir()
	m := &manifest.Manifest{
		Ncgo:   manifest.Meta{Version: "0.0.0-test", AssetsVersion: "test"},
		Mode:   manifest.ModeMono,
		Module: "github.com/x/demo",
		Service: manifest.Service{
			Name: "demo", Kind: kind, IDL: "idl/app/demo.proto",
		},
		GeneratedAt: time.Date(2026, 4, 29, 0, 0, 0, 0, time.UTC),
	}
	if err := manifest.Save(root, m); err != nil {
		t.Fatalf("seed manifest: %v", err)
	}

	// Create files needed for export. The template rules match specific paths.
	// For Hertz: main.go, conf/dev/conf.yaml, internal/base/conf/conf.go,
	//   internal/base/server/server.go, internal/router/**/*.go, internal/pkg/**/*.go
	if kind == manifest.KindHertz {
		writeFile(t, root, "main.go", "package main\n")
		writeFile(t, root, "conf/dev/conf.yaml", "server:\n  host: localhost\n")
		writeFile(t, root, "internal/base/conf/conf.go", "package conf\n")
		writeFile(t, root, "internal/base/server/server.go", "package server\n")
		writeFile(t, root, "internal/base/data/data.go", "package data\n")
		writeFile(t, root, "internal/router/demo/router.go", "package router\n")
		writeFile(t, root, "internal/pkg/utils/helper.go", "package utils\n")
		writeFile(t, root, "internal/base/logging/log.go", "package logging\n")
	} else if kind == manifest.KindKitex {
		writeFile(t, root, "main.go", "package main\n")
		writeFile(t, root, "conf/dev/conf.yaml", "server:\n  host: localhost\n")
		writeFile(t, root, "internal/base/conf/conf.go", "package conf\n")
		writeFile(t, root, "internal/base/server/server.go", "package server\n")
		writeFile(t, root, "internal/base/data/data.go", "package data\n")
		writeFile(t, root, "internal/pkg/utils/helper.go", "package utils\n")
		writeFile(t, root, "internal/base/middleware/mw.go", "package middleware\n")
		writeFile(t, root, "internal/base/release/release.go", "package release\n")
		writeFile(t, root, "internal/base/logging/log.go", "package logging\n")
	}

	return root
}

func writeFile(t *testing.T, root, rel string, content string) {
	t.Helper()
	abs := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(abs), err)
	}
	if err := os.WriteFile(abs, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", rel, err)
	}
}

func TestRunExportTemplatesHertz(t *testing.T) {
	root := seedExportProject(t, manifest.KindHertz)

	var out bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&out)

	err := runExportTemplates(cmd, &exportTemplatesOptions{root: root, kind: manifest.KindHertz})
	if err != nil {
		t.Fatalf("runExportTemplates hertz: %v", err)
	}

	text := out.String()
	if !strings.Contains(text, "exported ") {
		t.Fatalf("text missing 'exported ': %s", text)
	}
	if !strings.Contains(text, "template/hertz-template") {
		t.Fatalf("text missing 'template/hertz-template': %s", text)
	}

	// Verify output directory was created.
	outDir := filepath.Join(root, "template", "hertz-template")
	if fi, err := os.Stat(outDir); err != nil || !fi.IsDir() {
		t.Fatalf("template dir not created: stat err = %v", err)
	}

	// Check that template YAML files exist.
	entries, err := os.ReadDir(outDir)
	if err != nil {
		t.Fatalf("readdir template: %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("no template files generated")
	}
}

func TestRunExportTemplatesKitex(t *testing.T) {
	root := seedExportProject(t, manifest.KindKitex)

	var out bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&out)

	err := runExportTemplates(cmd, &exportTemplatesOptions{root: root, kind: manifest.KindKitex})
	if err != nil {
		t.Fatalf("runExportTemplates kitex: %v", err)
	}

	text := out.String()
	if !strings.Contains(text, "exported ") {
		t.Fatalf("text missing 'exported ': %s", text)
	}
	if !strings.Contains(text, "template/kitex-template") {
		t.Fatalf("text missing 'template/kitex-template': %s", text)
	}

	// Verify output directory was created.
	outDir := filepath.Join(root, "template", "kitex-template")
	if fi, err := os.Stat(outDir); err != nil || !fi.IsDir() {
		t.Fatalf("template dir not created: stat err = %v", err)
	}
}

func TestRunExportTemplatesAutoDetectKind(t *testing.T) {
	root := seedExportProject(t, manifest.KindHertz)

	var out bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&out)

	// Don't specify --kind, it should auto-detect from manifest.
	err := runExportTemplates(cmd, &exportTemplatesOptions{root: root})
	if err != nil {
		t.Fatalf("runExportTemplates auto-detect: %v", err)
	}

	text := out.String()
	if !strings.Contains(text, "exported ") {
		t.Fatalf("text missing 'exported ': %s", text)
	}
}
