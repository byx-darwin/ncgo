package cli

import "testing"

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
