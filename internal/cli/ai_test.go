package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/byx-darwin/ncgo/internal/ai"
	"github.com/byx-darwin/ncgo/internal/manifest"
	"github.com/spf13/cobra"
)

func TestAICmdIncludesInitClaudeCommand(t *testing.T) {
	cmd, _, err := newAICmd().Find([]string{"init", "claude"})
	if err != nil {
		t.Fatalf("Find ai init claude: %v", err)
	}
	if cmd == nil || cmd.Name() != "claude" {
		t.Fatalf("ai init claude command not registered")
	}
}

func TestRunAIInitClaudeWritesStarterFiles(t *testing.T) {
	root := t.TempDir()
	var out bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&out)
	if err := runAIInitClaude(cmd, &aiInitClaudeOptions{root: root}); err != nil {
		t.Fatalf("runAIInitClaude: %v", err)
	}
	if !strings.Contains(out.String(), "wrote .claude/README.md") {
		t.Fatalf("unexpected output: %s", out.String())
	}
	if !strings.Contains(out.String(), "info: detected project shape: unknown") {
		t.Fatalf("missing shape detection output: %s", out.String())
	}
	if !strings.Contains(out.String(), "next: run ncgo ai sync --root "+root+" --lang en") {
		t.Fatalf("missing ai sync next step output: %s", out.String())
	}
	if _, err := os.Stat(filepath.Join(root, ".claude/rules/agent-engineering.md")); err != nil {
		t.Fatalf("starter file missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, ".claude/local/.gitignore")); err != nil {
		t.Fatalf("local .gitignore missing: %v", err)
	}
}

func TestRunAIInitClaudeTeamPresetWritesTeamFiles(t *testing.T) {
	root := t.TempDir()
	if err := manifest.SaveWorkspace(root, &manifest.Workspace{
		Ncgo:        manifest.Meta{Version: "0.0.0-test", AssetsVersion: "test-assets"},
		Mode:        manifest.ModeMicro,
		Name:        "commerce",
		Module:      "github.com/acme/commerce",
		Services:    []manifest.WorkspaceService{},
		GeneratedAt: time.Date(2026, 5, 7, 0, 0, 0, 0, time.UTC),
	}); err != nil {
		t.Fatalf("SaveWorkspace: %v", err)
	}
	var out bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&out)
	if err := runAIInitClaude(cmd, &aiInitClaudeOptions{root: root, preset: ai.InitPresetTeam}); err != nil {
		t.Fatalf("runAIInitClaude team: %v", err)
	}
	if !strings.Contains(out.String(), "wrote .claude/skills/plan-change.md") {
		t.Fatalf("unexpected output: %s", out.String())
	}
	if !strings.Contains(out.String(), "info: detected project shape: micro workspace root") {
		t.Fatalf("missing micro workspace shape output: %s", out.String())
	}
	if !strings.Contains(out.String(), "next: run ncgo ai sync --root "+root+" --lang en") {
		t.Fatalf("missing ai sync next step output: %s", out.String())
	}
	if _, err := os.Stat(filepath.Join(root, ".claude/agents/reviewer.md")); err != nil {
		t.Fatalf("team preset file missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, ".claude/commands/implement-change.md")); err != nil {
		t.Fatalf("implement-change command missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, ".claude/skills/write-tests.md")); err != nil {
		t.Fatalf("write-tests skill missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, ".claude/hooks/README.md")); err != nil {
		t.Fatalf("hooks README missing: %v", err)
	}
}

func TestRunAIInitClaudeDryRunOmitsNextStepSuggestion(t *testing.T) {
	root := t.TempDir()
	var out bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&out)
	if err := runAIInitClaude(cmd, &aiInitClaudeOptions{root: root, dryRun: true}); err != nil {
		t.Fatalf("runAIInitClaude dry-run: %v", err)
	}
	if strings.Contains(out.String(), "next: run ncgo ai sync") {
		t.Fatalf("dry-run should not print next step suggestion: %s", out.String())
	}
}
