package ai

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/byx-darwin/ncgo/internal/manifest"
)

func TestInitClaudeWritesStarterFiles(t *testing.T) {
	root := t.TempDir()
	res, err := InitClaude(InitOptions{Root: root})
	if err != nil {
		t.Fatalf("InitClaude: %v", err)
	}
	wantPaths := []string{
		".claude/README.md",
		".claude/rules/agent-engineering.md",
		".claude/rules/go.md",
		".claude/local/.gitignore",
	}
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
		if strings.TrimSpace(body) == "" {
			t.Errorf("%s should not be empty", p)
		}
		if p != ".claude/local/.gitignore" && strings.Contains(body, ManagedMarker) {
			t.Errorf("%s must not carry managed marker", p)
		}
	}
	readme, _ := os.ReadFile(filepath.Join(root, ".claude/README.md"))
	if !strings.Contains(string(readme), "## Ownership") {
		t.Errorf("starter .claude/README.md missing ownership guidance")
	}
	gitignore, _ := os.ReadFile(filepath.Join(root, ".claude/local/.gitignore"))
	if !strings.Contains(string(gitignore), "!.gitignore") {
		t.Errorf("starter local/.gitignore missing self-keep rule")
	}
}

func TestInitClaudeSkipsExistingFileWithoutForce(t *testing.T) {
	root := t.TempDir()
	full := filepath.Join(root, ".claude/rules/go.md")
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	pre := "hand-written go rule\n"
	if err := os.WriteFile(full, []byte(pre), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}
	res, err := InitClaude(InitOptions{Root: root})
	if err != nil {
		t.Fatalf("InitClaude: %v", err)
	}
	got, _ := os.ReadFile(full)
	if string(got) != pre {
		t.Fatalf("go.md overwritten without --force")
	}
	var skipped bool
	for _, s := range res.Skipped {
		if s.Path == ".claude/rules/go.md" && strings.Contains(s.Reason, "--force") {
			skipped = true
		}
	}
	if !skipped {
		t.Fatalf("expected skip for existing go.md; got %+v", res.Skipped)
	}
}

func TestInitClaudeForceOverwritesExistingFile(t *testing.T) {
	root := t.TempDir()
	full := filepath.Join(root, ".claude/rules/go.md")
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(full, []byte("hand-written\n"), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if _, err := InitClaude(InitOptions{Root: root, Force: true}); err != nil {
		t.Fatalf("InitClaude force: %v", err)
	}
	body, _ := os.ReadFile(full)
	if !strings.Contains(string(body), "# Go Service Rules") {
		t.Fatalf("force overwrite did not write starter content: %q", string(body))
	}
}

func TestInitClaudeDryRunWritesNothing(t *testing.T) {
	root := t.TempDir()
	res, err := InitClaude(InitOptions{Root: root, DryRun: true})
	if err != nil {
		t.Fatalf("InitClaude dry-run: %v", err)
	}
	if len(res.Written) != 0 {
		t.Fatalf("dry-run wrote files: %v", res.Written)
	}
	files, err := claudeStarterFiles(InitPresetMinimal)
	if err != nil {
		t.Fatalf("claudeStarterFiles(minimal): %v", err)
	}
	if len(res.Skipped) != len(files) {
		t.Fatalf("dry-run skipped = %v", res.Skipped)
	}
	if _, err := os.Stat(filepath.Join(root, ".claude/README.md")); !os.IsNotExist(err) {
		t.Fatalf("starter files should not exist after dry-run: %v", err)
	}
}

func TestInitClaudeCoexistsWithSync(t *testing.T) {
	root := t.TempDir()
	writeManifest(t, root, manifest.KindHertz)
	if _, err := InitClaude(InitOptions{Root: root}); err != nil {
		t.Fatalf("InitClaude: %v", err)
	}
	before, err := os.ReadFile(filepath.Join(root, ".claude/README.md"))
	if err != nil {
		t.Fatalf("read starter README: %v", err)
	}
	if _, err := Sync(Options{Root: root}); err != nil {
		t.Fatalf("Sync: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, ".claude/generated/project-context.md")); err != nil {
		t.Fatalf("expected generated project-context after sync: %v", err)
	}
	after, err := os.ReadFile(filepath.Join(root, ".claude/README.md"))
	if err != nil {
		t.Fatalf("read starter README after sync: %v", err)
	}
	if string(after) != string(before) {
		t.Fatalf("starter .claude/README.md should be unchanged by sync")
	}
}

func TestInitClaudeTeamPresetWritesAdditionalStarterFiles(t *testing.T) {
	root := t.TempDir()
	res, err := InitClaude(InitOptions{Root: root, Preset: InitPresetTeam})
	if err != nil {
		t.Fatalf("InitClaude team: %v", err)
	}
	wantPaths := []string{
		".claude/skills/plan-change.md",
		".claude/skills/run-validation.md",
		".claude/skills/doc-sync.md",
		".claude/agents/implementer.md",
		".claude/agents/reviewer.md",
		".claude/commands/plan.md",
		".claude/commands/fix-failing-test.md",
		".claude/commands/update-docs.md",
		".claude/commands/review-diff.md",
		".claude/hooks/README.md",
	}
	for _, p := range wantPaths {
		if _, err := os.Stat(filepath.Join(root, p)); err != nil {
			t.Fatalf("missing team preset file %s: %v", p, err)
		}
	}
	if len(res.Written) <= 4 {
		t.Fatalf("team preset should write more than minimal set: %v", res.Written)
	}
	plan, _ := os.ReadFile(filepath.Join(root, ".claude/commands/plan.md"))
	if !strings.Contains(string(plan), "Plan before editing") {
		t.Fatalf("team preset command template missing expected content")
	}
}

func TestInitClaudeRejectsInvalidPreset(t *testing.T) {
	root := t.TempDir()
	if _, err := InitClaude(InitOptions{Root: root, Preset: "org"}); err == nil || !strings.Contains(err.Error(), "unsupported preset") {
		t.Fatalf("err = %v, want unsupported preset error", err)
	}
}
