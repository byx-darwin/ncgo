package ai

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

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
	if !strings.Contains(string(readme), "## Using External Go Skills") {
		t.Errorf("starter .claude/README.md missing external skills guidance")
	}
	if !strings.Contains(string(readme), "Repository shape could not be detected yet") {
		t.Errorf("starter .claude/README.md should explain unknown project shape")
	}
	if len(res.Notes) == 0 || !strings.Contains(res.Notes[0], "detected project shape: unknown") {
		t.Errorf("InitClaude notes = %v, want unknown project shape", res.Notes)
	}
	gitignore, _ := os.ReadFile(filepath.Join(root, ".claude/local/.gitignore"))
	if !strings.Contains(string(gitignore), "!.gitignore") {
		t.Errorf("starter local/.gitignore missing self-keep rule")
	}
}

func TestInitClaudeReadmeDetectsMonoServiceRoot(t *testing.T) {
	root := t.TempDir()
	writeManifest(t, root, manifest.KindHertz)
	res, err := InitClaude(InitOptions{Root: root})
	if err != nil {
		t.Fatalf("InitClaude mono: %v", err)
	}
	readme, err := os.ReadFile(filepath.Join(root, ".claude/README.md"))
	if err != nil {
		t.Fatalf("read starter README: %v", err)
	}
	body := string(readme)
	if !strings.Contains(body, "Detected repository shape: **service root**") || !strings.Contains(body, ".ncgo/manifest.yaml") {
		t.Fatalf("mono README missing service-root guidance:\n%s", body)
	}
	if len(res.Notes) == 0 || !strings.Contains(res.Notes[0], "service root") {
		t.Fatalf("InitClaude mono notes = %v, want service root", res.Notes)
	}
}

func TestInitClaudeReadmeDetectsMicroWorkspaceRoot(t *testing.T) {
	root := t.TempDir()
	if err := manifest.SaveWorkspace(root, &manifest.Workspace{
		Ncgo:        manifest.Meta{Version: "0.0.0-test", AssetsVersion: "test-assets"},
		Mode:        manifest.ModeMicro,
		Name:        "commerce",
		Module:      "github.com/acme/commerce",
		Services:    []manifest.WorkspaceService{{Name: "user-rpc", Kind: manifest.KindKitex, Dir: "services/user-rpc"}},
		GeneratedAt: time.Date(2026, 5, 7, 0, 0, 0, 0, time.UTC),
	}); err != nil {
		t.Fatalf("SaveWorkspace: %v", err)
	}
	res, err := InitClaude(InitOptions{Root: root})
	if err != nil {
		t.Fatalf("InitClaude micro: %v", err)
	}
	readme, err := os.ReadFile(filepath.Join(root, ".claude/README.md"))
	if err != nil {
		t.Fatalf("read starter README: %v", err)
	}
	body := string(readme)
	if !strings.Contains(body, "Detected repository shape: **micro workspace root**") || !strings.Contains(body, "ncgo.workspace") || !strings.Contains(body, "services/*") {
		t.Fatalf("micro README missing workspace guidance:\n%s", body)
	}
	if len(res.Notes) == 0 || !strings.Contains(res.Notes[0], "micro workspace root") {
		t.Fatalf("InitClaude micro notes = %v, want micro workspace root", res.Notes)
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
		".claude/skills/write-tests.md",
		".claude/agents/planner.md",
		".claude/agents/implementer.md",
		".claude/agents/reviewer.md",
		".claude/agents/debugger.md",
		".claude/agents/doc-writer.md",
		".claude/commands/plan.md",
		".claude/commands/implement-change.md",
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
	if !strings.Contains(string(plan), "Plan before editing") || !strings.Contains(string(plan), ".claude/generated/project-context.md") || !strings.Contains(string(plan), "machine-consumed fields") {
		t.Fatalf("team preset command template missing expected content")
	}
	writeTests, _ := os.ReadFile(filepath.Join(root, ".claude/skills/write-tests.md"))
	if !strings.Contains(string(writeTests), "Mono vs Micro Scope") || !strings.Contains(string(writeTests), "table-driven tests") || !strings.Contains(string(writeTests), "go test -race") {
		t.Fatalf("team preset write-tests skill missing mono/micro guidance")
	}
	runValidation, _ := os.ReadFile(filepath.Join(root, ".claude/skills/run-validation.md"))
	if !strings.Contains(string(runValidation), "go test -run") || !strings.Contains(string(runValidation), "go test -race") {
		t.Fatalf("team preset run-validation skill missing targeted Go validation guidance")
	}
	planSkill, _ := os.ReadFile(filepath.Join(root, ".claude/skills/plan-change.md"))
	if !strings.Contains(string(planSkill), "Affected Surfaces") || !strings.Contains(string(planSkill), "templates, manifests, or codegen inputs") {
		t.Fatalf("team preset plan-change skill missing affected surface guidance")
	}
	docSync, _ := os.ReadFile(filepath.Join(root, ".claude/skills/doc-sync.md"))
	if !strings.Contains(string(docSync), "worked examples") || !strings.Contains(string(docSync), "Swagger or OpenAPI") {
		t.Fatalf("team preset doc-sync skill missing contract and API doc guidance")
	}
	implementChange, _ := os.ReadFile(filepath.Join(root, ".claude/commands/implement-change.md"))
	if !strings.Contains(string(implementChange), "Use the `implementer` agent") || !strings.Contains(string(implementChange), "write-tests") || !strings.Contains(string(implementChange), "run-validation") || !strings.Contains(string(implementChange), ".claude/generated/project-context.md") {
		t.Fatalf("team preset implement-change command missing implementer workflow")
	}
	fixFailing, _ := os.ReadFile(filepath.Join(root, ".claude/commands/fix-failing-test.md"))
	if !strings.Contains(string(fixFailing), "golden output") || !strings.Contains(string(fixFailing), "go test -race") {
		t.Fatalf("team preset fix-failing-test command missing debugger workflow guidance")
	}
	updateDocs, _ := os.ReadFile(filepath.Join(root, ".claude/commands/update-docs.md"))
	if !strings.Contains(string(updateDocs), "stable machine-consumed fields") || !strings.Contains(string(updateDocs), "Swagger/OpenAPI") {
		t.Fatalf("team preset update-docs command missing contract-doc guidance")
	}
	reviewDiff, _ := os.ReadFile(filepath.Join(root, ".claude/commands/review-diff.md"))
	if !strings.Contains(string(reviewDiff), "layering drift") || !strings.Contains(string(reviewDiff), "generated-output ownership mistakes") {
		t.Fatalf("team preset review-diff command missing review checklist guidance")
	}
	implementer, _ := os.ReadFile(filepath.Join(root, ".claude/agents/implementer.md"))
	if !strings.Contains(string(implementer), "name: implementer") || !strings.Contains(string(implementer), "tools: Read, Write, Edit, Bash") || !strings.Contains(string(implementer), ".claude/generated/project-context.md") || !strings.Contains(string(implementer), "context.Background()") {
		t.Fatalf("team preset implementer template missing Claude Code frontmatter")
	}
	planner, _ := os.ReadFile(filepath.Join(root, ".claude/agents/planner.md"))
	if !strings.Contains(string(planner), "name: planner") || !strings.Contains(string(planner), "tools: Read, Bash") || !strings.Contains(string(planner), "ncgo.workspace") {
		t.Fatalf("team preset planner template missing Claude Code frontmatter")
	}
	docWriter, _ := os.ReadFile(filepath.Join(root, ".claude/agents/doc-writer.md"))
	if !strings.Contains(string(docWriter), "name: doc-writer") || !strings.Contains(string(docWriter), "services/<name>/README.md") || !strings.Contains(string(docWriter), "stable machine-consumed field") || !strings.Contains(string(docWriter), "Swagger or OpenAPI") {
		t.Fatalf("team preset doc-writer template missing repository-aware doc guidance")
	}
	reviewer, _ := os.ReadFile(filepath.Join(root, ".claude/agents/reviewer.md"))
	if !strings.Contains(string(reviewer), "name: reviewer") || !strings.Contains(string(reviewer), "tools: Read, Bash") || !strings.Contains(string(reviewer), "context.Background()") {
		t.Fatalf("team preset reviewer template missing Claude Code frontmatter")
	}
	debugger, _ := os.ReadFile(filepath.Join(root, ".claude/agents/debugger.md"))
	if !strings.Contains(string(debugger), "name: debugger") || !strings.Contains(string(debugger), "tools: Read, Write, Edit, Bash") || !strings.Contains(string(debugger), "snapshot") || !strings.Contains(string(debugger), "go test -race") {
		t.Fatalf("team preset debugger template missing Claude Code frontmatter")
	}
}

func TestInitClaudeRejectsInvalidPreset(t *testing.T) {
	root := t.TempDir()
	if _, err := InitClaude(InitOptions{Root: root, Preset: "org"}); err == nil || !strings.Contains(err.Error(), "unsupported preset") {
		t.Fatalf("err = %v, want unsupported preset error", err)
	}
}

func TestInitClaudeGoMdIncludesHertzRules(t *testing.T) {
	root := t.TempDir()
	writeManifest(t, root, manifest.KindHertz)
	_, err := InitClaude(InitOptions{Root: root})
	if err != nil {
		t.Fatalf("InitClaude hertz: %v", err)
	}
	body, err := os.ReadFile(filepath.Join(root, ".claude/rules/go.md"))
	if err != nil {
		t.Fatalf("read go.md: %v", err)
	}
	s := string(body)
	want := []string{
		"Hertz HTTP Service Rules",
		"middleware",
		"*app.RequestContext",
		"response.OK",
		"router.GeneratedRegister",
	}
	for _, w := range want {
		if !strings.Contains(s, w) {
			t.Errorf("go.md missing %q for Hertz service", w)
		}
	}
	if strings.Contains(s, "interceptor") {
		t.Errorf("go.md should not mention interceptor for Hertz service")
	}
}

func TestInitClaudeGoMdIncludesKitexRules(t *testing.T) {
	root := t.TempDir()
	writeManifest(t, root, manifest.KindKitex)
	_, err := InitClaude(InitOptions{Root: root})
	if err != nil {
		t.Fatalf("InitClaude kitex: %v", err)
	}
	body, err := os.ReadFile(filepath.Join(root, ".claude/rules/go.md"))
	if err != nil {
		t.Fatalf("read go.md: %v", err)
	}
	s := string(body)
	want := []string{
		"Kitex RPC Service Rules",
		"interceptor",
		"rpcerror.ToBizError",
		"kitex.BizStatusError",
	}
	for _, w := range want {
		if !strings.Contains(s, w) {
			t.Errorf("go.md missing %q for Kitex service", w)
		}
	}
	if strings.Contains(s, "*app.RequestContext") {
		t.Errorf("go.md should not mention *app.RequestContext for Kitex service")
	}
}

func TestInitClaudeMicroWorkspaceCreatesServiceDirs(t *testing.T) {
	root := t.TempDir()
	if err := manifest.SaveWorkspace(root, &manifest.Workspace{
		Ncgo:        manifest.Meta{Version: "0.0.0-test", AssetsVersion: "test-assets"},
		Mode:        manifest.ModeMicro,
		Name:        "commerce",
		Module:      "github.com/acme/commerce",
		Services:    []manifest.WorkspaceService{{Name: "user-api", Kind: manifest.KindHertz, Dir: "services/user-api"}, {Name: "user-rpc", Kind: manifest.KindKitex, Dir: "services/user-rpc"}},
		GeneratedAt: time.Date(2026, 5, 7, 0, 0, 0, 0, time.UTC),
	}); err != nil {
		t.Fatalf("SaveWorkspace: %v", err)
	}
	res, err := InitClaude(InitOptions{Root: root})
	if err != nil {
		t.Fatalf("InitClaude micro: %v", err)
	}
	// Verify README lists both services with kind labels
	readme, _ := os.ReadFile(filepath.Join(root, ".claude/README.md"))
	body := string(readme)
	if !strings.Contains(body, "user-api") || !strings.Contains(body, "HTTP (Hertz)") {
		t.Errorf("README missing user-api with Hertz label:\n%s", body)
	}
	if !strings.Contains(body, "user-rpc") || !strings.Contains(body, "RPC (Kitex)") {
		t.Errorf("README missing user-rpc with Kitex label:\n%s", body)
	}
	// Verify per-service directories were created
	for _, svc := range []string{"user-api", "user-rpc"} {
		rulesPath := filepath.Join(root, ".claude", "services", svc, "rules.md")
		if _, err := os.Stat(rulesPath); err != nil {
			t.Errorf("missing service rules file %s: %v", rulesPath, err)
		}
		checklistPath := filepath.Join(root, ".claude", "services", svc, "reviewer-checklist.md")
		if _, err := os.Stat(checklistPath); err != nil {
			t.Errorf("missing service checklist file %s: %v", checklistPath, err)
		}
	}
	// Verify go.md does NOT contain arch-specific rules for micro workspace
	goMd, _ := os.ReadFile(filepath.Join(root, ".claude/rules/go.md"))
	if strings.Contains(string(goMd), "Hertz HTTP Service Rules") || strings.Contains(string(goMd), "Kitex RPC Service Rules") {
		t.Errorf("micro workspace go.md should not contain arch-specific rules")
	}
	// Verify at least the service dirs were written
	var svcWrites int
	for _, w := range res.Written {
		if strings.HasPrefix(w, ".claude/services/") {
			svcWrites++
		}
	}
	if svcWrites == 0 {
		t.Errorf("expected service dir writes, got none: %v", res.Written)
	}
}

func TestInitClaudeUnknownShapeHasEmptyArchRules(t *testing.T) {
	root := t.TempDir()
	_, err := InitClaude(InitOptions{Root: root})
	if err != nil {
		t.Fatalf("InitClaude unknown: %v", err)
	}
	body, err := os.ReadFile(filepath.Join(root, ".claude/rules/go.md"))
	if err != nil {
		t.Fatalf("read go.md: %v", err)
	}
	// The placeholder should be replaced with empty string for unknown shape
	if strings.Contains(string(body), "{{ARCHITECTURE_RULES}}") {
		t.Errorf("go.md should not contain raw placeholder for unknown shape")
	}
}
