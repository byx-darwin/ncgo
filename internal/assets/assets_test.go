package assets

import (
	"io/fs"
	"strings"
	"testing"
)

func TestVersionParsesAssetsVersion(t *testing.T) {
	v := Version()
	if v == "" || v == "unknown" {
		t.Fatalf("Version() = %q, want a parsed assets version", v)
	}
	if strings.ContainsRune(v, '/') || strings.ContainsRune(v, ' ') {
		t.Fatalf("Version() = %q, looks malformed", v)
	}
}

func TestEmbeddedFilesPresent(t *testing.T) {
	want := []string{
		"VERSION",
		"claude/README.md",
		"claude/skills/plan-change.md",
		"claude/skills/run-validation.md",
		"claude/skills/doc-sync.md",
		"claude/skills/write-tests.md",
		"claude/agents/planner.md",
		"claude/agents/implementer.md",
		"claude/agents/reviewer.md",
		"claude/agents/debugger.md",
		"claude/agents/doc-writer.md",
		"claude/commands/plan.md",
		"claude/commands/implement-change.md",
		"claude/commands/fix-failing-test.md",
		"claude/commands/update-docs.md",
		"claude/commands/review-diff.md",
		"claude/hooks/README.md",
		"claude/local/.gitignore",
		"claude/rules/agent-engineering.md",
		"claude/rules/go.md",
		"docs/hertz/design-doc.en.md",
		"docs/hertz/design-doc.zh-CN.md",
		"docs/hertz/rate-limit-dynamic-design.en.md",
		"docs/hertz/rate-limit-dynamic-design.zh-CN.md",
		"docs/kitex/design-doc.en.md",
		"docs/kitex/design-doc.zh-CN.md",
		"docs/micro/design-doc.en.md",
		"docs/micro/design-doc.zh-CN.md",
		"hertz/data.json",
		"hertz/layout.yaml",
		"hertz/package.yaml",
		"hertz/sqlc.yaml",
		"hertz/optional/redis.go",
		"hertz/optional/kafka.go",
		"hertz/optional/es.go",
		"hertz/optional/clickhouse.go",
		"hertz/validate/validate.proto",
		"kitex/sqlc.yaml",
		"kitex/query/health.sql",
		"kitex/schema/000001_placeholder.sql",
		"kitex/kitex-template/server.yaml",
		"kitex/kitex-template/handler.yaml",
		"kitex/kitex-template/usecase.yaml",
		"kitex/kitex-template/repository.yaml",
		"kitex/optional/registry_polaris.go",
		"kitex/optional/registry_polaris.yaml",
	}
	for _, p := range want {
		if _, err := fs.Stat(FS(), p); err != nil {
			t.Errorf("missing embedded file %q: %v", p, err)
		}
	}
}

func TestEmbeddedFilesNonEmpty(t *testing.T) {
	count := 0
	err := fs.WalkDir(FS(), ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		count++
		return nil
	})
	if err != nil {
		t.Fatalf("WalkDir: %v", err)
	}
	// Snapshot has VERSION + claude starters/presets + design docs + hertz + kitex + common optionals.
	if count < 47 {
		t.Fatalf("embedded file count = %d, expected >= 47", count)
	}
}
