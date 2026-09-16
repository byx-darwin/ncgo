package mono

import (
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/byx-darwin/ncgo/internal/assets"
	scaffoldtemplate "github.com/byx-darwin/ncgo/internal/scaffold/template"
)

// TestRuleCenterHandEditTemplatesUseSkipUpdateBehavior locks Issue #117's
// fix: any kitex-template file in the rule-center preset whose body invites
// hand-editing ("Edit business logic here") must declare
// update_behavior: skip, or `make update` (which invokes the vendored kitex
// -template-dir binary directly, bypassing ncgo) silently overwrites the
// user's business logic on every regeneration.
func TestRuleCenterHandEditTemplatesUseSkipUpdateBehavior(t *testing.T) {
	const handEditMarker = "Edit business logic here"
	srcFS := assets.FS()
	entries, err := fs.ReadDir(srcFS, "kitex/kitex-template")
	if err != nil {
		t.Fatalf("read kitex-template dir: %v", err)
	}
	checked := 0
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".yaml") {
			continue
		}
		b, err := fs.ReadFile(srcFS, "kitex/kitex-template/"+e.Name())
		if err != nil {
			t.Fatalf("read %s: %v", e.Name(), err)
		}
		var tpl scaffoldtemplate.TemplateFile
		if err := yaml.Unmarshal(b, &tpl); err != nil {
			t.Fatalf("parse %s: %v", e.Name(), err)
		}
		if !strings.Contains(tpl.Body, handEditMarker) {
			continue
		}
		checked++
		if tpl.UpdateBehavior.Type != "skip" {
			t.Errorf("%s: body invites hand-editing (%q) but update_behavior.type = %q, want \"skip\" — make update will silently overwrite user edits",
				e.Name(), handEditMarker, tpl.UpdateBehavior.Type)
		}
	}
	if checked == 0 {
		t.Fatal("no kitex-template file matched the hand-edit marker — test is not exercising the intended files; did the marker text change?")
	}
}

// TestRPCErrorTemplateUsesSkipUpdateBehavior locks Issue #123's fix:
// internal/pkg/rpcerror/rpcerror.go is a genuine hand-extension point (new
// business error codes/functions are added over a service's lifetime, with
// no merge mechanism), so update_behavior must be "skip" — otherwise
// `make update` silently discards every hand-added error code on the next
// IDL change, exactly as reported in Issue #123.
func TestRPCErrorTemplateUsesSkipUpdateBehavior(t *testing.T) {
	srcFS := assets.FS()
	b, err := fs.ReadFile(srcFS, "kitex/kitex-template/rpcerror.yaml")
	if err != nil {
		t.Fatalf("read kitex/kitex-template/rpcerror.yaml: %v", err)
	}
	var tpl scaffoldtemplate.TemplateFile
	if err := yaml.Unmarshal(b, &tpl); err != nil {
		t.Fatalf("parse rpcerror.yaml: %v", err)
	}
	if tpl.UpdateBehavior.Type != "skip" {
		t.Errorf("rpcerror.yaml: update_behavior.type = %q, want \"skip\" — make update will silently overwrite hand-added error codes", tpl.UpdateBehavior.Type)
	}
}

// TestMakeUpdateBackupsCoverFilesBeforeOverwrite locks Issue #123's
// general safety net: before `make update` lets the vendored kitex binary
// overwrite any update_behavior:cover file, the Makefile's update target
// must back it up to .ncgo-backup/<timestamp>/ so a hand-edit is never
// lost without a recovery path. This also covers cover-type fragments
// whose path contains a kitex Go-template placeholder like
// {{ToLower .ServiceInfo.ServiceName}} (e.g. client.yaml, handler.yaml) —
// the recipe must resolve any {{...}} block in the extracted path via the
// Makefile's own $(SERVICE_NAME) variable before checking whether the
// file exists, or the backup is silently skipped for exactly the
// hand-edit-prone files (client wiring, handlers) this fix targets. The
// pattern must match generically (any {{...}} block, not one hardcoded
// spelling) since `ncgo export templates` re-parameterizes paths using a
// different spelling than kitex's own placeholders.
func TestMakeUpdateBackupsCoverFilesBeforeOverwrite(t *testing.T) {
	srcFS := assets.FS()
	b, err := fs.ReadFile(srcFS, "kitex/kitex-template/makefile.yaml")
	if err != nil {
		t.Fatalf("read kitex/kitex-template/makefile.yaml: %v", err)
	}
	var tpl scaffoldtemplate.TemplateFile
	if err := yaml.Unmarshal(b, &tpl); err != nil {
		t.Fatalf("parse makefile.yaml: %v", err)
	}

	const marker = "update: ; "
	idx := strings.Index(tpl.Body, marker)
	if idx < 0 {
		t.Fatal("makefile.yaml body has no \"update: ; \" recipe line — did the target name or spacing change?")
	}
	recipeStart := idx + len(marker)
	recipeEnd := strings.Index(tpl.Body[recipeStart:], "\n")
	if recipeEnd < 0 {
		recipeEnd = len(tpl.Body) - recipeStart
	}
	rawRecipe := tpl.Body[recipeStart : recipeStart+recipeEnd]

	// The recipe (extracted above, still raw source) is itself a Go
	// template fragment: the real kitex binary renders makefile.yaml's
	// whole body once into the project's actual root Makefile. It must be
	// rendered here too, not used as raw source, so any real template
	// action in it is exercised the same way. The full makefile.yaml body
	// isn't rendered here because it also references kitex's own
	// `.IDLName` field, which ncgo's scaffoldtemplate.RenderData has no
	// equivalent for (kitex's template context differs from ncgo's); the
	// extracted recipe substring needs no such fields.
	recipe, err := scaffoldtemplate.Render(rawRecipe, scaffoldtemplate.RenderData{})
	if err != nil {
		t.Fatalf("render update recipe: %v", err)
	}

	kitexIdx := strings.Index(recipe, "kitex -module")
	if kitexIdx < 0 {
		t.Fatal("update recipe has no \"kitex -module\" invocation — did the recipe structure change?")
	}
	backupSnippet := recipe[:kitexIdx]
	if !strings.Contains(backupSnippet, ".ncgo-backup") {
		t.Fatal("update recipe's pre-kitex portion has no .ncgo-backup logic — backup step missing or moved after the kitex call")
	}
	// $(SERVICE_NAME) is a genuine Make variable (single $, unlike the
	// doubled $$ used everywhere else for the shell). Make would substitute
	// it with the rendered service name before the shell ever runs; since
	// this test executes the extracted recipe directly via sh, bypassing
	// Make, it must perform that substitution itself.
	const fakeServiceName = "demo"
	backupSnippet = strings.ReplaceAll(backupSnippet, "$(SERVICE_NAME)", fakeServiceName)
	backupSnippet = strings.ReplaceAll(backupSnippet, "$$", "$")
	backupSnippet = strings.TrimPrefix(strings.TrimSpace(backupSnippet), "@")

	dir := t.TempDir()
	tplDir := filepath.Join(dir, "template", "kitex-template")
	if err := os.MkdirAll(tplDir, 0o755); err != nil {
		t.Fatal(err)
	}

	writeFragment := func(yamlName, path, updateType string) {
		content := "path: " + path + "\nupdate_behavior:\n  type: " + updateType + "\nbody: |-\n  package x\n"
		if err := os.WriteFile(filepath.Join(tplDir, yamlName), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	writeFragment("cover_one.yaml", "internal/pkg/rpcerror/rpcerror.go", "cover")
	writeFragment("cover_two.yaml", "main.go", "cover")
	writeFragment("cover_templated.yaml", "pkg/client/{{ToLower .ServiceInfo.ServiceName}}/client.go", "cover")
	writeFragment("skip_one.yaml", "internal/handler/handler.go", "skip")

	writeTarget := func(relPath, content string) {
		full := filepath.Join(dir, relPath)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	writeTarget("internal/pkg/rpcerror/rpcerror.go", "package rpcerror // hand-edited\n")
	writeTarget("main.go", "package main // hand-edited\n")
	writeTarget("pkg/client/"+fakeServiceName+"/client.go", "package client // hand-edited\n")
	writeTarget("internal/handler/handler.go", "package handler // hand-edited\n")

	cmd := exec.Command("sh", "-c", backupSnippet)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("backup snippet failed: %v\noutput:\n%s\nsnippet:\n%s", err, out, backupSnippet)
	}

	backupRoot := filepath.Join(dir, ".ncgo-backup")
	entries, err := os.ReadDir(backupRoot)
	if err != nil {
		t.Fatalf("read .ncgo-backup: %v (output: %s)", err, out)
	}
	if len(entries) != 1 {
		t.Fatalf(".ncgo-backup: got %d timestamp dirs, want 1", len(entries))
	}
	tsDir := filepath.Join(backupRoot, entries[0].Name())

	for _, want := range []string{"internal/pkg/rpcerror/rpcerror.go", "main.go", "pkg/client/" + fakeServiceName + "/client.go"} {
		gotPath := filepath.Join(tsDir, want)
		got, err := os.ReadFile(gotPath)
		if err != nil {
			t.Errorf("expected backup of cover file %s at %s: %v", want, gotPath, err)
			continue
		}
		wantContent, _ := os.ReadFile(filepath.Join(dir, want))
		if string(got) != string(wantContent) {
			t.Errorf("backup of %s = %q, want %q", want, got, wantContent)
		}
	}

	if _, err := os.Stat(filepath.Join(tsDir, "internal/handler/handler.go")); err == nil {
		t.Error("skip-type file internal/handler/handler.go was backed up; only cover-type files should be")
	}
}
