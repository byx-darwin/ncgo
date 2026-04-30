package ai

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/byx-darwin/ncgo/internal/manifest"
)

func sampleManifest(kind string) *manifest.Manifest {
	return &manifest.Manifest{
		Ncgo:    manifest.Meta{Version: "0.1.0-dev", AssetsVersion: "test"},
		Mode:    manifest.ModeMono,
		Module:  "github.com/acme/user-api",
		Service: manifest.Service{Name: "user-api", Kind: kind, WithDatabase: true},
		Infra:   []string{"redis"},
		Domains: []string{"device"},
	}
}

func writeManifest(t *testing.T, root, kind string) {
	t.Helper()
	if err := manifest.Save(root, sampleManifest(kind)); err != nil {
		t.Fatalf("manifest.Save: %v", err)
	}
}

func TestSyncWritesAllTargets(t *testing.T) {
	root := t.TempDir()
	writeManifest(t, root, manifest.KindHertz)
	res, err := Sync(Options{Root: root})
	if err != nil {
		t.Fatalf("Sync: %v", err)
	}
	wantPaths := []string{"AGENTS.md", "CLAUDE.md", ".cursor/rules/ncgo.mdc"}
	if len(res.Written) != len(wantPaths) {
		t.Fatalf("Written = %v, want %v", res.Written, wantPaths)
	}
	for _, p := range wantPaths {
		full := filepath.Join(root, p)
		b, err := os.ReadFile(full)
		if err != nil {
			t.Fatalf("read %s: %v", p, err)
		}
		body := string(b)
		if !strings.Contains(body, ManagedMarker) {
			t.Errorf("%s missing managed marker", p)
		}
		if !strings.Contains(body, "module: `github.com/acme/user-api`") {
			t.Errorf("%s missing manifest module summary", p)
		}
		if !strings.Contains(body, "infra: `[redis]`") {
			t.Errorf("%s missing manifest infra summary", p)
		}
		if !strings.Contains(body, "domains: `[device]`") {
			t.Errorf("%s missing manifest domains summary", p)
		}
		if !strings.Contains(body, "Hertz Template Design Doc") &&
			!strings.Contains(body, "## 2. Generated Project Architecture") {
			t.Errorf("%s missing embedded design-doc body", p)
		}
	}
	mdc, _ := os.ReadFile(filepath.Join(root, ".cursor/rules/ncgo.mdc"))
	if !strings.HasPrefix(string(mdc), "---\n") {
		t.Errorf(".mdc must start with frontmatter; got: %q", string(mdc[:4]))
	}
}

func TestSyncPicksKitexDoc(t *testing.T) {
	root := t.TempDir()
	writeManifest(t, root, manifest.KindKitex)
	if _, err := Sync(Options{Root: root}); err != nil {
		t.Fatalf("Sync: %v", err)
	}
	body, _ := os.ReadFile(filepath.Join(root, "AGENTS.md"))
	if !strings.Contains(string(body), "Kitex") {
		t.Errorf("AGENTS.md should mention Kitex for kitex-kind manifest")
	}
	if strings.Contains(string(body), "## 6. `hz` Invocation Mapping") {
		t.Errorf("AGENTS.md should not embed hertz-specific section for kitex manifest")
	}
}

func TestSyncRefusesUnmanagedFile(t *testing.T) {
	root := t.TempDir()
	writeManifest(t, root, manifest.KindHertz)
	pre := "# user-owned AGENTS\n\nhand-written content\n"
	if err := os.WriteFile(filepath.Join(root, "AGENTS.md"), []byte(pre), 0o644); err != nil {
		t.Fatalf("seed AGENTS.md: %v", err)
	}
	res, err := Sync(Options{Root: root})
	if err != nil {
		t.Fatalf("Sync: %v", err)
	}
	if got, _ := os.ReadFile(filepath.Join(root, "AGENTS.md")); string(got) != pre {
		t.Errorf("AGENTS.md must not be overwritten without --force")
	}
	var skipped bool
	for _, s := range res.Skipped {
		if s.Path == "AGENTS.md" && strings.Contains(s.Reason, "ncgo:managed") {
			skipped = true
		}
	}
	if !skipped {
		t.Errorf("expected AGENTS.md skip in result; got %+v", res.Skipped)
	}
}

func TestSyncForceOverwritesUnmanagedFile(t *testing.T) {
	root := t.TempDir()
	writeManifest(t, root, manifest.KindHertz)
	if err := os.WriteFile(filepath.Join(root, "AGENTS.md"), []byte("hand-written\n"), 0o644); err != nil {
		t.Fatalf("seed AGENTS.md: %v", err)
	}
	if _, err := Sync(Options{Root: root, Force: true}); err != nil {
		t.Fatalf("Sync: %v", err)
	}
	body, _ := os.ReadFile(filepath.Join(root, "AGENTS.md"))
	if !strings.Contains(string(body), ManagedMarker) {
		t.Errorf("--force should overwrite with managed file; got %q", string(body))
	}
}

func TestSyncAppendsLocalNotes(t *testing.T) {
	root := t.TempDir()
	writeManifest(t, root, manifest.KindHertz)
	notes := "extra rule: avoid global variables.\n"
	if err := os.WriteFile(filepath.Join(root, LocalNotesFile), []byte(notes), 0o644); err != nil {
		t.Fatalf("seed local notes: %v", err)
	}
	if _, err := Sync(Options{Root: root}); err != nil {
		t.Fatalf("Sync: %v", err)
	}
	body, _ := os.ReadFile(filepath.Join(root, "AGENTS.md"))
	if !strings.Contains(string(body), "## Local Notes") || !strings.Contains(string(body), "avoid global variables") {
		t.Errorf("Local Notes section missing from AGENTS.md")
	}
}

func TestSyncDryRunWritesNothing(t *testing.T) {
	root := t.TempDir()
	writeManifest(t, root, manifest.KindHertz)
	res, err := Sync(Options{Root: root, DryRun: true})
	if err != nil {
		t.Fatalf("Sync: %v", err)
	}
	if len(res.Written) != 0 {
		t.Errorf("DryRun must not write; got %v", res.Written)
	}
	if len(res.Skipped) != len(targets()) {
		t.Errorf("DryRun should skip all targets; got %v", res.Skipped)
	}
	if _, err := os.Stat(filepath.Join(root, "AGENTS.md")); !os.IsNotExist(err) {
		t.Errorf("AGENTS.md should not exist after dry run")
	}
}

func TestSyncRejectsBadLang(t *testing.T) {
	root := t.TempDir()
	writeManifest(t, root, manifest.KindHertz)
	if _, err := Sync(Options{Root: root, Lang: "fr"}); err == nil {
		t.Fatalf("expected error for --lang fr")
	}
}

func TestSyncZhLangPicksZhDoc(t *testing.T) {
	root := t.TempDir()
	writeManifest(t, root, manifest.KindHertz)
	if _, err := Sync(Options{Root: root, Lang: LangZhCN}); err != nil {
		t.Fatalf("Sync: %v", err)
	}
	body, _ := os.ReadFile(filepath.Join(root, "AGENTS.md"))
	if !strings.Contains(string(body), "生成项目架构") {
		t.Errorf("zh-CN sync should embed Chinese design doc")
	}
}
