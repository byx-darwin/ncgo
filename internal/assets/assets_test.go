package assets

import (
	"io/fs"
	"strings"
	"testing"
)

func readAsset(t *testing.T, path string) string {
	t.Helper()
	b, err := fs.ReadFile(FS(), path)
	if err != nil {
		t.Fatalf("read embedded file %q: %v", path, err)
	}
	return string(b)
}

func assertInOrder(t *testing.T, body string, items ...string) {
	t.Helper()
	pos := 0
	for _, item := range items {
		i := strings.Index(body[pos:], item)
		if i < 0 {
			t.Fatalf("workflow is missing %q after byte %d", item, pos)
		}
		pos += i + len(item)
	}
}

func recipeBlock(t *testing.T, body, heading string) string {
	t.Helper()
	start := strings.Index(body, heading)
	if start < 0 {
		t.Fatalf("workflow is missing recipe heading %q", heading)
	}
	rest := body[start+len(heading):]
	if next := strings.Index(rest, "\n### Recipe"); next >= 0 {
		rest = rest[:next]
	}
	return rest
}

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
		"docs/ai/ncgo-dev-workflow.en.md",
		"docs/ai/ncgo-dev-workflow.zh-CN.md",
		"docs/ai/ncgo-dev-rules.en.md",
		"docs/ai/ncgo-dev-rules.zh-CN.md",
		"hertz/layout.yaml",
		"hertz/package.yaml",
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

func TestAgentWorkflowRecipesStaySeparatedAndActionable(t *testing.T) {
	tests := []struct {
		path     string
		headings []string
		sections []string
		boundary string
	}{
		{
			path: "docs/ai/ncgo-dev-workflow.en.md",
			headings: []string{
				"### Recipe: Hertz HTTP endpoint",
				"### Recipe: Kitex RPC endpoint",
				"### Recipe: internal domain capability",
				"### Recipe: BFF to RPC client",
				"### Recipe: infrastructure add-on and wiring",
			},
			sections: []string{"**Prerequisites**", "**Sequence**", "**Expected files**", "**Validation**", "**Failure recovery**"},
			boundary: "Do **not** run both commands as two ways to create the same endpoint method",
		},
		{
			path: "docs/ai/ncgo-dev-workflow.zh-CN.md",
			headings: []string{
				"### Recipe：Hertz HTTP endpoint",
				"### Recipe：Kitex RPC endpoint",
				"### Recipe：内部 domain capability",
				"### Recipe：BFF 调用 RPC client",
				"### Recipe：基础设施 add-on 与 wiring",
			},
			sections: []string{"**前置条件**", "**执行顺序**", "**预期文件**", "**验证**", "**失败恢复**"},
			boundary: "不要把两个命令当成创建同一个 endpoint 方法的两种方式而连续执行",
		},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			body := readAsset(t, tt.path)
			if !strings.Contains(body, tt.boundary) {
				t.Fatalf("workflow is missing command-boundary warning %q", tt.boundary)
			}
			if !strings.Contains(body, "ncgo add method") || !strings.Contains(body, "ncgo add rpc-method") {
				t.Fatal("workflow must describe both method commands")
			}
			for _, heading := range tt.headings {
				block := recipeBlock(t, body, heading)
				for _, section := range tt.sections {
					if !strings.Contains(block, section) {
						t.Errorf("recipe %q is missing section %q", heading, section)
					}
				}
			}

			hertz := recipeBlock(t, body, tt.headings[0])
			assertInOrder(t, hertz, "<manifest.service.idl>", "ncgo protolint", "make update", "ncgo add rpc-method")
			kitex := recipeBlock(t, body, tt.headings[1])
			assertInOrder(t, kitex, "<manifest.service.idl>", "ncgo protolint", "make update", "ncgo add rpc-method")
			domain := recipeBlock(t, body, tt.headings[2])
			assertInOrder(t, domain, "ncgo add domain", "--dry-run", "ncgo add domain", "ncgo add method")
			bff := recipeBlock(t, body, tt.headings[3])
			assertInOrder(t, bff, "ncgo add kitex-client", "--dry-run", "ncgo add kitex-client")
			infra := recipeBlock(t, body, tt.headings[4])
			assertInOrder(t, infra, "ncgo add infra", "--dry-run", "ncgo add infra")
		})
	}
}
