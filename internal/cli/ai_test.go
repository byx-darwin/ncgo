package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/byx-darwin/ncgo/internal/ai"
	"github.com/byx-darwin/ncgo/internal/manifest"
	"github.com/spf13/cobra"
)

func writeCLIServiceManifest(t *testing.T, root string) {
	t.Helper()
	if err := manifest.Save(root, &manifest.Manifest{
		Ncgo:   manifest.Meta{Version: "0.0.0-test", AssetsVersion: "test-assets"},
		Mode:   manifest.ModeMono,
		Module: "github.com/acme/user-api",
		Service: manifest.Service{
			Name: "user-api", Kind: manifest.KindHertz, IDL: "idl/app/user-api.proto",
		},
		GeneratedAt: time.Date(2026, 5, 7, 0, 0, 0, 0, time.UTC),
	}); err != nil {
		t.Fatalf("Save manifest: %v", err)
	}
}

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

func TestRunAIInitClaudeJSONOutput(t *testing.T) {
	root := t.TempDir()
	var out bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&out)
	if err := runAIInitClaude(cmd, &aiInitClaudeOptions{root: root, output: "json"}); err != nil {
		t.Fatalf("runAIInitClaude json: %v", err)
	}
	var res ai.Result
	if err := json.Unmarshal(out.Bytes(), &res); err != nil {
		t.Fatalf("json unmarshal: %v\n%s", err, out.String())
	}
	if len(res.Written) == 0 {
		t.Fatalf("result = %+v, want starter files to be written", res)
	}
	if len(res.NextSteps) != 1 || res.NextSteps[0] != "run ncgo ai sync --root "+root+" --lang en" {
		t.Fatalf("result = %+v, want ai sync next step", res)
	}
	if !strings.Contains(strings.Join(res.Notes, "\n"), "detected project shape: unknown") {
		t.Fatalf("result = %+v, want project-shape note", res)
	}
}

func TestRunAIInitClaudeRejectsInvalidOutput(t *testing.T) {
	root := t.TempDir()
	cmd := &cobra.Command{}
	if err := runAIInitClaude(cmd, &aiInitClaudeOptions{root: root, output: "xml"}); err == nil || err.Error() != `ai init claude: unsupported --output "xml"; want text or json` {
		t.Fatalf("err = %v", err)
	}
}

func TestRunAISyncTextOutput(t *testing.T) {
	root := t.TempDir()
	writeCLIServiceManifest(t, root)
	var out bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&out)
	if err := runAISync(cmd, &aiSyncOptions{root: root, output: "text", dryRun: true}); err != nil {
		t.Fatalf("runAISync text: %v", err)
	}
	got := out.String()
	if !strings.Contains(got, "skipped AGENTS.md (dry-run)") {
		t.Fatalf("text output missing dry-run summary: %s", got)
	}
	if strings.Contains(got, `"written"`) {
		t.Fatalf("text output should not be json: %s", got)
	}
}

func TestRunAISyncJSONOutput(t *testing.T) {
	root := t.TempDir()
	writeCLIServiceManifest(t, root)
	var out bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&out)
	if err := runAISync(cmd, &aiSyncOptions{root: root, output: "json", dryRun: true}); err != nil {
		t.Fatalf("runAISync json: %v", err)
	}
	var res ai.Result
	if err := json.Unmarshal(out.Bytes(), &res); err != nil {
		t.Fatalf("json unmarshal: %v\n%s", err, out.String())
	}
	if res.Scope != "service" || res.SourceRef != ".ncgo/manifest.yaml" {
		t.Fatalf("result = %+v, want service/.ncgo/manifest.yaml", res)
	}
	if len(res.Written) != 0 || len(res.Skipped) != 7 {
		t.Fatalf("result = %+v, want 0 writes and 7 skips (4 targets + 3 standalone docs)", res)
	}
}

func TestRunAISyncRejectsInvalidOutput(t *testing.T) {
	root := t.TempDir()
	writeCLIServiceManifest(t, root)
	cmd := &cobra.Command{}
	if err := runAISync(cmd, &aiSyncOptions{root: root, output: "xml"}); err == nil || err.Error() != `ai sync: unsupported --output "xml"; want text or json` {
		t.Fatalf("err = %v", err)
	}
}
