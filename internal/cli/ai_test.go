package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/byx-darwin/ncgo/internal/ai"
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
	if _, err := os.Stat(filepath.Join(root, ".claude/rules/agent-engineering.md")); err != nil {
		t.Fatalf("starter file missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, ".claude/local/.gitignore")); err != nil {
		t.Fatalf("local .gitignore missing: %v", err)
	}
}

func TestRunAIInitClaudeTeamPresetWritesTeamFiles(t *testing.T) {
	root := t.TempDir()
	var out bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&out)
	if err := runAIInitClaude(cmd, &aiInitClaudeOptions{root: root, preset: ai.InitPresetTeam}); err != nil {
		t.Fatalf("runAIInitClaude team: %v", err)
	}
	if !strings.Contains(out.String(), "wrote .claude/skills/plan-change.md") {
		t.Fatalf("unexpected output: %s", out.String())
	}
	if _, err := os.Stat(filepath.Join(root, ".claude/agents/reviewer.md")); err != nil {
		t.Fatalf("team preset file missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, ".claude/hooks/README.md")); err != nil {
		t.Fatalf("hooks README missing: %v", err)
	}
}
