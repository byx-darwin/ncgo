package doctor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/byx-darwin/ncgo/internal/manifest"
)

// seedCheckProject builds a healthy mono service: manifest + one domain with
// a usecase file carrying anchors.
func seedCheckProject(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := manifest.Save(root, &manifest.Manifest{
		Ncgo:    manifest.Meta{Version: "0.1.0-test", AssetsVersion: "test"},
		Mode:    manifest.ModeMono,
		Module:  "github.com/x/demo",
		Service: manifest.Service{Name: "demo", Kind: manifest.KindHertz},
		Domains: []string{"device"},
	}); err != nil {
		t.Fatalf("seed manifest: %v", err)
	}
	usecase := filepath.Join(root, "internal", "usecase", "device", "device.go")
	if err := os.MkdirAll(filepath.Dir(usecase), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	body := `package device

type UseCase struct{}

// ncgo:methods:start
// ncgo:methods:end
`
	if err := os.WriteFile(usecase, []byte(body), 0o644); err != nil {
		t.Fatalf("write usecase: %v", err)
	}
	return root
}

func TestRunCheckHealthyProject(t *testing.T) {
	root := seedCheckProject(t)
	rep, err := RunCheck(root)
	if err != nil {
		t.Fatalf("RunCheck: %v", err)
	}
	if !rep.OK() {
		t.Fatalf("rep.OK() = false, want true; checks=%+v", rep.Checks)
	}
	if rep.Root != root || rep.Scope != ScopeService {
		t.Fatalf("rep.Root/Scope = %q/%q, want %q/%q", rep.Root, rep.Scope, root, ScopeService)
	}
	found := map[string]bool{}
	for _, c := range rep.Checks {
		found[c.ID] = true
	}
	for _, id := range []string{"check.anchor", "check.manifest.consistency", "check.context.stale"} {
		if !found[id] {
			t.Errorf("checks missing %s: %+v", id, rep.Checks)
		}
	}
}

func TestRunCheckBrokenAnchors(t *testing.T) {
	root := seedCheckProject(t)
	p := filepath.Join(root, "internal", "usecase", "device", "device.go")
	b, _ := os.ReadFile(p)
	if err := os.WriteFile(p, []byte(strings.ReplaceAll(string(b), "// ncgo:methods:start\n", "")), 0o644); err != nil {
		t.Fatalf("rewrite usecase: %v", err)
	}
	rep, err := RunCheck(root)
	if err != nil {
		t.Fatalf("RunCheck: %v", err)
	}
	if rep.OK() {
		t.Fatal("rep.OK() = true, want false (broken anchors)")
	}
}

func TestRunCheckStaleContext(t *testing.T) {
	root := seedCheckProject(t)
	claude := filepath.Join(root, "CLAUDE.md")
	content := "<!-- ncgo:managed -->\n# Project Context for Claude Code\n\n## Project Facts\n\n- domains: `[device, ghost]`\n"
	if err := os.WriteFile(claude, []byte(content), 0o644); err != nil {
		t.Fatalf("write CLAUDE.md: %v", err)
	}
	rep, err := RunCheck(root)
	if err != nil {
		t.Fatalf("RunCheck: %v", err)
	}
	if rep.OK() {
		t.Fatal("rep.OK() = true, want false (stale context)")
	}
}

func TestRunCheckMissingManifest(t *testing.T) {
	root := t.TempDir()
	if _, err := RunCheck(root); err == nil {
		t.Fatal("RunCheck should error when manifest is missing")
	}
}

func TestParseContextDomains(t *testing.T) {
	tests := []struct{ in, want string }{
		{"- domains: `[device, order]`", "device,order"},
		{"- domains: `[device]`", "device"},
		{"- domains: `[]`", ""},
		{"no domains line here", ""},
	}
	for _, tt := range tests {
		got := strings.Join(parseContextDomains(tt.in), ",")
		if got != tt.want {
			t.Errorf("parseContextDomains(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
