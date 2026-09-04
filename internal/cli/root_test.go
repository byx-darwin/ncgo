package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	goexec "github.com/byx-darwin/ncgo/internal/exec"
	"github.com/byx-darwin/ncgo/internal/manifest"
	"github.com/byx-darwin/ncgo/internal/postgenerate"
)

func TestRootCmdIncludesProtolintCommand(t *testing.T) {
	cmd, _, err := newRootCmd().Find([]string{"protolint"})
	if err != nil {
		t.Fatalf("Find protolint: %v", err)
	}
	if cmd == nil || cmd.Name() != "protolint" {
		t.Fatalf("protolint command not registered")
	}
}

func TestRootCmdIncludesAIInitClaudeCommand(t *testing.T) {
	cmd, _, err := newRootCmd().Find([]string{"ai", "init", "claude"})
	if err != nil {
		t.Fatalf("Find ai init claude: %v", err)
	}
	if cmd == nil || cmd.Name() != "claude" {
		t.Fatalf("ai init claude command not registered")
	}
}

func TestVersionLineIncludesBuildMetadata(t *testing.T) {
	got := versionLine("v1.2.3", "assets-1", "abc1234", "2026-05-06T12:00:00Z")
	want := "ncgo v1.2.3 (build: abc1234, built: 2026-05-06T12:00:00Z, assets: assets-1)"
	if got != want {
		t.Fatalf("versionLine() = %q, want %q", got, want)
	}
}

func TestVersionLineNormalizesEmptyValues(t *testing.T) {
	got := versionLine("", "", "", "")
	want := "ncgo unknown (build: unknown, built: unknown, assets: unknown)"
	if got != want {
		t.Fatalf("versionLine() = %q, want %q", got, want)
	}
}

func TestResolveBuildInfoFallsBackToVCSSettings(t *testing.T) {
	buildVersion, buildTime := resolveBuildInfo("dev", "unknown", map[string]string{
		"vcs.revision": "abcdef1234567890",
		"vcs.modified": "true",
		"vcs.time":     "2026-05-06T12:00:00Z",
	})
	if buildVersion != "abcdef1-dirty" {
		t.Fatalf("buildVersion = %q, want abcdef1-dirty", buildVersion)
	}
	if buildTime != "2026-05-06T12:00:00Z" {
		t.Fatalf("buildTime = %q, want 2026-05-06T12:00:00Z", buildTime)
	}
}

func TestResolveBuildInfoKeepsInjectedValues(t *testing.T) {
	buildVersion, buildTime := resolveBuildInfo("release-build", "2026-05-06T13:00:00Z", map[string]string{
		"vcs.revision": "abcdef1234567890",
		"vcs.time":     "2026-05-06T12:00:00Z",
	})
	if buildVersion != "release-build" || buildTime != "2026-05-06T13:00:00Z" {
		t.Fatalf("resolveBuildInfo kept = (%q, %q)", buildVersion, buildTime)
	}
}

func TestNewCmdHasTemplateDirFlag(t *testing.T) {
	cmd := newNewCmd()
	f := cmd.Flags().Lookup("template-dir")
	if f == nil {
		t.Fatal("--template-dir flag not registered on ncgo new")
	}
	if f.DefValue != "" {
		t.Errorf("--template-dir default = %q, want empty", f.DefValue)
	}
}

func TestNewCmdHasTemplateFlag(t *testing.T) {
	cmd := newNewCmd()
	f := cmd.Flags().Lookup("template")
	if f == nil {
		t.Fatal("--template flag not registered on ncgo new")
	}
	if f.DefValue != "" {
		t.Errorf("--template default = %q, want empty", f.DefValue)
	}
}

func TestNewCmdHasAITargetFlag(t *testing.T) {
	cmd := newNewCmd()
	f := cmd.Flags().Lookup("ai-target")
	if f == nil {
		t.Fatal("--ai-target flag not registered on ncgo new")
	}
	if f.DefValue != "claude" {
		t.Errorf("--ai-target default = %q, want claude", f.DefValue)
	}
}

func TestNewCmdHasNoAutoStepsFlag(t *testing.T) {
	cmd := newNewCmd()
	f := cmd.Flags().Lookup("no-auto-steps")
	if f == nil {
		t.Fatal("--no-auto-steps flag not registered on ncgo new")
	}
	if f.DefValue != "false" {
		t.Errorf("--no-auto-steps default = %q, want false", f.DefValue)
	}
}

func TestFilterAutoSteps(t *testing.T) {
	succeeded := &postgenerate.Result{
		Steps: []postgenerate.StepResult{
			{Name: "go mod tidy", Status: "succeeded"},
			{Name: "ai sync", Status: "succeeded"},
		},
	}
	steps := []string{"go mod tidy", "ncgo ai sync --target all --root .", "hz update", "make lint"}
	got := filterAutoSteps(steps, succeeded)
	want := []string{"hz update", "make lint"}
	if len(got) != len(want) {
		t.Fatalf("filterAutoSteps len = %d, want %d (got %v)", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("filterAutoSteps[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestFilterAutoStepsKeepsFailedAndSkipped(t *testing.T) {
	failed := &postgenerate.Result{
		Steps: []postgenerate.StepResult{
			{Name: "go mod tidy", Status: "failed"},
			{Name: "ai sync", Status: "skipped"},
		},
	}
	steps := []string{"go mod tidy", "ncgo ai sync --target all --root ."}
	got := filterAutoSteps(steps, failed)
	if len(got) != 2 {
		t.Errorf("filterAutoSteps should keep steps when not succeeded, got %v", got)
	}
}

func TestFilterAutoStepsNilResult(t *testing.T) {
	steps := []string{"go mod tidy", "ncgo ai sync --target all --root ."}
	got := filterAutoSteps(steps, nil)
	if len(got) != 2 {
		t.Errorf("filterAutoSteps(nil) should keep all steps, got %v", got)
	}
}

func TestRunNewMonoTemplateAndTemplateDirMutuallyExclusive(t *testing.T) {
	cmd := newNewCmd()
	cmd.SetOut(new(strings.Builder))
	opts := &newOptions{
		module:       "github.com/acme/demo",
		kind:         manifest.KindKitex,
		db:           "none",
		dir:          filepath.Join(t.TempDir(), "demo"),
		noGenerate:   true,
		templateDir:  t.TempDir(),
		templateName: "base-kitex",
	}
	err := runNewMono(cmd, "demo", opts)
	if err == nil || !strings.Contains(err.Error(), "mutually exclusive") {
		t.Errorf("want mutually exclusive error, got %v", err)
	}
}

func TestRunNewMonoTemplateNotCachedError(t *testing.T) {
	cmd := newNewCmd()
	cmd.SetOut(new(strings.Builder))
	opts := &newOptions{
		module:       "github.com/acme/demo",
		kind:         manifest.KindKitex,
		db:           "none",
		dir:          filepath.Join(t.TempDir(), "demo"),
		noGenerate:   true,
		templateName: "not-in-cache-xyz-98765",
	}
	err := runNewMono(cmd, "demo", opts)
	if err == nil || !strings.Contains(err.Error(), "ncgo template pull") {
		t.Errorf("want cache-miss error mentioning 'ncgo template pull', got %v", err)
	}
}

func TestRunNewMonoPrintsTemplateIDLFallbackNotice(t *testing.T) {
	const notice = "(template package has no idl/; used built-in IDL placeholder)"
	run := func(templateDir string) string {
		t.Helper()
		var out strings.Builder
		cmd := newNewCmd()
		cmd.SetOut(&out)
		opts := &newOptions{
			module:      "github.com/acme/demo",
			kind:        manifest.KindKitex,
			db:          "none",
			dir:         filepath.Join(t.TempDir(), "demo"),
			noGenerate:  true,
			templateDir: templateDir,
		}
		if err := runNewMono(cmd, "demo", opts); err != nil {
			t.Fatalf("runNewMono: %v", err)
		}
		return out.String()
	}

	// Package with a kitex-template dir but no idl/ → fallback notice prints.
	pkg := t.TempDir()
	_ = os.MkdirAll(filepath.Join(pkg, "kitex-template"), 0o755)
	_ = os.WriteFile(filepath.Join(pkg, "kitex-template", "main_go.yaml"),
		[]byte("path: main.go\nbody: |\n  package main\n"), 0o644)
	if got := run(pkg); !strings.Contains(got, notice) {
		t.Errorf("fallback notice missing for no-idl package:\n%s", got)
	}

	// No template package → no fallback notice.
	if got := run(""); strings.Contains(got, notice) {
		t.Errorf("fallback notice should not print without --template-dir:\n%s", got)
	}
}

func TestPreflightSkippedWhenNoGenerate(t *testing.T) {
	var out strings.Builder
	err := preflightTools(context.Background(), manifest.KindHertz, true, &out, strings.NewReader("n"))
	if err != nil {
		t.Fatalf("preflightTools with noGenerate=true: %v", err)
	}
	if out.Len() > 0 {
		t.Fatalf("expected no output with noGenerate=true, got: %s", out.String())
	}
}

func TestPreflightUserDeclineReturnsError(t *testing.T) {
	var out strings.Builder
	missing := []toolPreflight{
		{name: "hz", minVersion: goexec.MinHzVersion, installCmd: goexec.InstallHint("hz")},
	}
	err := preflightToolsWith(context.Background(), missing, &out, strings.NewReader("n"), func(ctx context.Context, name string) error { return nil })
	if err == nil {
		t.Fatal("expected error when user declines")
	}
	if !strings.Contains(out.String(), "Aborted") {
		t.Errorf("output should mention 'Aborted': %s", out.String())
	}
}

func TestPreflightInstallFailureReturnsError(t *testing.T) {
	var out strings.Builder
	missing := []toolPreflight{
		{name: "hz", minVersion: "v0.0.0", installCmd: goexec.InstallHint("hz")},
	}
	fakeErr := fmt.Errorf("network error")
	fakeInstall := func(ctx context.Context, name string) error {
		return fakeErr
	}
	err := preflightToolsWith(context.Background(), missing, &out, strings.NewReader("y"), fakeInstall)
	if err == nil {
		t.Fatal("expected error on install failure")
	}
	if !strings.Contains(err.Error(), "install hz") {
		t.Errorf("error = %q, should mention 'install hz'", err.Error())
	}
	if !strings.Contains(out.String(), "Failed to install") {
		t.Errorf("output should mention 'Failed to install': %s", out.String())
	}
	if !strings.Contains(out.String(), goexec.InstallHint("hz")) {
		t.Errorf("output should show manual install hint: %s", out.String())
	}
}

func TestRequiredToolsReturnsMissingForHertzKind(t *testing.T) {
	need := requiredTools(manifest.KindHertz)
	if len(need) > 1 {
		t.Fatalf("expected at most 1 missing tool for hertz, got %d", len(need))
	}
	if len(need) == 1 && need[0].name != "hz" {
		t.Fatalf("expected missing tool to be hz, got %s", need[0].name)
	}
}

func TestRequiredToolsReturnsMissingForKitexKind(t *testing.T) {
	need := requiredTools(manifest.KindKitex)
	if len(need) > 1 {
		t.Fatalf("expected at most 1 missing tool for kitex, got %d", len(need))
	}
	if len(need) == 1 && need[0].name != "kitex" {
		t.Fatalf("expected missing tool to be kitex, got %s", need[0].name)
	}
}

func TestRequiredToolsEmptyKindDefaultsToHertz(t *testing.T) {
	need := requiredTools("")
	if len(need) > 1 {
		t.Fatalf("expected at most 1 missing tool for empty kind, got %d", len(need))
	}
	if len(need) == 1 && need[0].name != "hz" {
		t.Fatalf("expected missing tool to be hz for empty kind, got %s", need[0].name)
	}
}

func TestRequiredToolsUsesInstallHint(t *testing.T) {
	need := requiredTools(manifest.KindHertz)
	if len(need) != 1 {
		t.Skip("hz must be installed for this assertion")
	}
	if need[0].installCmd != goexec.InstallHint("hz") {
		t.Errorf("installCmd %q should equal InstallHint(%q)", need[0].installCmd, "hz")
	}
}

func TestAddRPCCmdHasTemplateFlags(t *testing.T) {
	cmd := newAddRPCCmd()
	for _, name := range []string{"template", "template-dir"} {
		if f := cmd.Flags().Lookup(name); f == nil {
			t.Errorf("add rpc missing --%s flag", name)
		}
	}
}

func TestAddBFFCmdHasPresetAndTemplateFlags(t *testing.T) {
	cmd := newAddBFFCmd()
	for _, name := range []string{"preset", "template", "template-dir"} {
		if f := cmd.Flags().Lookup(name); f == nil {
			t.Errorf("add bff missing --%s flag", name)
		}
	}
}
