package orchestrator

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/byx-darwin/ncgo/internal/manifest"
)

func writeManifest(t *testing.T, root string, kind string) {
	t.Helper()
	if err := manifest.Save(root, &manifest.Manifest{
		Ncgo:   manifest.Meta{Version: "0.0.0-test", AssetsVersion: "test-assets"},
		Mode:   manifest.ModeMono,
		Module: "github.com/acme/user-api",
		Service: manifest.Service{
			Name: "user-api", Kind: kind, IDL: "idl/app/user-api.proto",
		},
		GeneratedAt: time.Date(2026, 5, 7, 0, 0, 0, 0, time.UTC),
	}); err != nil {
		t.Fatalf("Save manifest: %v", err)
	}
}

func TestRunAIInitClaude(t *testing.T) {
	root := t.TempDir()

	result, err := RunAIInitClaude(context.Background(), AIInitClaudeOptions{Root: root})
	if err != nil {
		t.Fatalf("RunAIInitClaude: %v", err)
	}
	if len(result.Written) == 0 {
		t.Fatalf("written = %+v, want starter files", result.Written)
	}
	hasReadme := false
	for _, p := range result.Written {
		if p == ".claude/README.md" {
			hasReadme = true
			break
		}
	}
	if !hasReadme {
		t.Fatalf("written = %+v, want .claude/README.md", result.Written)
	}
	if len(result.NextSteps) != 1 || !strings.HasPrefix(result.NextSteps[0], "run ncgo ai sync") {
		t.Fatalf("nextSteps = %v, want ai sync hint", result.NextSteps)
	}
	if !strings.Contains(strings.Join(result.Notes, "\n"), "detected project shape: unknown") {
		t.Fatalf("notes = %v, want project-shape note", result.Notes)
	}
	// Verify files actually exist.
	if _, err := os.Stat(filepath.Join(root, ".claude/rules/agent-engineering.md")); err != nil {
		t.Fatalf("starter file missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, ".claude/local/.gitignore")); err != nil {
		t.Fatalf("local .gitignore missing: %v", err)
	}
}

func TestRunAIInitClaudeTeam(t *testing.T) {
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

	result, err := RunAIInitClaude(context.Background(), AIInitClaudeOptions{Root: root, Preset: "team"})
	if err != nil {
		t.Fatalf("RunAIInitClaude team: %v", err)
	}
	hasSkill := false
	for _, p := range result.Written {
		if p == ".claude/skills/plan-change.md" {
			hasSkill = true
			break
		}
	}
	if !hasSkill {
		t.Fatalf("written = %+v, want team skill file", result.Written)
	}
	if !strings.Contains(strings.Join(result.Notes, "\n"), "micro workspace root") {
		t.Fatalf("notes = %v, want micro workspace shape", result.Notes)
	}
	if _, err := os.Stat(filepath.Join(root, ".claude/agents/reviewer.md")); err != nil {
		t.Fatalf("team preset file missing: %v", err)
	}
}

func TestRunAIInitClaudeDryRun(t *testing.T) {
	root := t.TempDir()

	result, err := RunAIInitClaude(context.Background(), AIInitClaudeOptions{Root: root, DryRun: true})
	if err != nil {
		t.Fatalf("RunAIInitClaude dry-run: %v", err)
	}
	if len(result.NextSteps) != 0 {
		t.Fatalf("dry-run should not produce next steps: %v", result.NextSteps)
	}
	if len(result.Written) != 0 {
		t.Fatalf("dry-run should not write files: %v", result.Written)
	}
}

func TestRunAISyncText(t *testing.T) {
	root := t.TempDir()
	writeManifest(t, root, manifest.KindHertz)

	result, err := RunAISync(context.Background(), AISyncOptions{Root: root, DryRun: true})
	if err != nil {
		t.Fatalf("RunAISync: %v", err)
	}
	if result.Scope != "service" || result.SourceRef != ".ncgo/manifest.yaml" {
		t.Fatalf("result = %+v, want scope=service sourceRef=.ncgo/manifest.yaml", result)
	}
	if len(result.Written) != 0 || len(result.Skipped) != 7 {
		t.Fatalf("result = %+v, want 0 writes and 7 skips (4 targets + 3 standalone docs)", result)
	}
}
