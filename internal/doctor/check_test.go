package doctor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/byx-darwin/ncgo/internal/ai"
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
	if _, err := ai.Sync(ai.Options{Root: root}); err != nil {
		t.Fatalf("ai.Sync: %v", err)
	}
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
	for _, id := range []string{"check.anchor", "check.manifest.consistency", "check.context.current"} {
		if !found[id] {
			t.Errorf("checks missing %s: %+v", id, rep.Checks)
		}
	}
}

func TestRunCheckAcceptsChineseManagedContext(t *testing.T) {
	root := seedCheckProject(t)
	if _, err := ai.Sync(ai.Options{Root: root, Lang: ai.LangZhCN}); err != nil {
		t.Fatalf("ai.Sync zh-CN: %v", err)
	}
	rep, err := RunCheck(root)
	if err != nil {
		t.Fatalf("RunCheck: %v", err)
	}
	if !rep.OK() {
		t.Fatalf("Chinese managed contexts should validate: %+v", rep.Checks)
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
	if _, err := ai.Sync(ai.Options{Root: root}); err != nil {
		t.Fatalf("ai.Sync: %v", err)
	}
	claude := filepath.Join(root, "CLAUDE.md")
	content, err := os.ReadFile(claude)
	if err != nil {
		t.Fatalf("read CLAUDE.md: %v", err)
	}
	content = []byte(strings.Replace(string(content), "- domains: `[device]`", "- domains: `[device, ghost]`", 1))
	if err := os.WriteFile(claude, content, 0o644); err != nil {
		t.Fatalf("write CLAUDE.md: %v", err)
	}
	rep, err := RunCheck(root)
	if err != nil {
		t.Fatalf("RunCheck: %v", err)
	}
	if rep.OK() {
		t.Fatal("rep.OK() = true, want false (stale context)")
	}
	found := false
	for _, c := range rep.Checks {
		if c.ID == "check.context.stale" && c.File == claude {
			found = true
		}
	}
	if !found {
		t.Fatalf("checks did not report stale CLAUDE.md after AGENTS.md was current: %+v", rep.Checks)
	}
}

func TestRunCheckReportsEveryMissingEnabledContext(t *testing.T) {
	root := seedCheckProject(t)
	if _, err := ai.Sync(ai.Options{Root: root, Target: ai.TargetAgents}); err != nil {
		t.Fatalf("ai.Sync: %v", err)
	}
	rep, err := RunCheck(root)
	if err != nil {
		t.Fatalf("RunCheck: %v", err)
	}
	if !rep.OK() {
		t.Fatalf("missing contexts are migration warnings, not hard failures: %+v", rep.Checks)
	}
	missing := map[string]bool{}
	for _, c := range rep.Checks {
		if c.ID == "check.context.missing" {
			if c.Severity != SeverityWarn || c.OK {
				t.Fatalf("missing context check = %+v, want failed warning", c)
			}
			if !strings.Contains(c.Hint, root) {
				t.Fatalf("missing context hint should target checked root %q: %+v", root, c)
			}
			missing[filepath.ToSlash(strings.TrimPrefix(c.File, root+string(filepath.Separator)))] = true
		}
	}
	for _, rel := range []string{"CLAUDE.md", ".claude/skills/ncgo-dev/SKILL.md", ".claude/generated/project-context.md", ".cursor/rules/ncgo.mdc"} {
		if !missing[rel] {
			t.Errorf("missing warning not reported for %s: %+v", rel, rep.Checks)
		}
	}
}

func TestRunCheckPreservesUnmanagedContextAsWarning(t *testing.T) {
	root := seedCheckProject(t)
	agents := filepath.Join(root, "AGENTS.md")
	want := []byte("# User-owned Agent instructions\n")
	if err := os.WriteFile(agents, want, 0o644); err != nil {
		t.Fatalf("write AGENTS.md: %v", err)
	}
	if _, err := ai.Sync(ai.Options{Root: root}); err != nil {
		t.Fatalf("ai.Sync: %v", err)
	}
	rep, err := RunCheck(root)
	if err != nil {
		t.Fatalf("RunCheck: %v", err)
	}
	found := false
	for _, c := range rep.Checks {
		if c.ID == "check.context.unmanaged" && c.File == agents && c.Severity == SeverityWarn {
			found = true
		}
	}
	if !found {
		t.Fatalf("unmanaged warning not found: %+v", rep.Checks)
	}
	got, err := os.ReadFile(agents)
	if err != nil {
		t.Fatalf("read AGENTS.md: %v", err)
	}
	if string(got) != string(want) {
		t.Fatalf("unmanaged AGENTS.md changed:\n%s", got)
	}
}

func TestRunCheckWorkspaceAuditsEveryEnabledContext(t *testing.T) {
	root := t.TempDir()
	if err := manifest.SaveWorkspace(root, &manifest.Workspace{
		Ncgo: manifest.Meta{Version: "0.1.0-test", AssetsVersion: "test"},
		Mode: manifest.ModeMicro, Name: "commerce", Module: "github.com/x/commerce",
	}); err != nil {
		t.Fatalf("SaveWorkspace: %v", err)
	}
	if _, err := ai.Sync(ai.Options{Root: root}); err != nil {
		t.Fatalf("ai.Sync: %v", err)
	}
	cursor := filepath.Join(root, ".cursor", "rules", "ncgo.mdc")
	body, err := os.ReadFile(cursor)
	if err != nil {
		t.Fatalf("read cursor context: %v", err)
	}
	if err := os.WriteFile(cursor, append(body, []byte("\nmanual stale edit\n")...), 0o644); err != nil {
		t.Fatalf("write cursor context: %v", err)
	}
	rep, err := RunCheck(root)
	if err != nil {
		t.Fatalf("RunCheck workspace: %v", err)
	}
	if rep.Scope != ScopeWorkspace || rep.OK() {
		t.Fatalf("workspace report scope/OK = %s/%v, checks=%+v", rep.Scope, rep.OK(), rep.Checks)
	}
	found := false
	for _, c := range rep.Checks {
		if c.ID == "check.context.stale" && c.File == cursor {
			found = true
		}
	}
	if !found {
		t.Fatalf("stale cursor context not reported: %+v", rep.Checks)
	}
}

func TestRunCheckMissingManifest(t *testing.T) {
	root := t.TempDir()
	if _, err := RunCheck(root); err == nil {
		t.Fatal("RunCheck should error when manifest is missing")
	}
}
