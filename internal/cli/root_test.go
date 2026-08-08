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
	os.MkdirAll(filepath.Join(pkg, "kitex-template"), 0o755)
	os.WriteFile(filepath.Join(pkg, "kitex-template", "main_go.yaml"),
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
