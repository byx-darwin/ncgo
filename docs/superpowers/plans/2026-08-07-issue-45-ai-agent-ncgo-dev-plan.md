# Issue #45 — AI Agent 端到端 ncgo 开发（Phase 1）Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 让 Claude Code（及通用 agent）能端到端用 ncgo 实现功能开发：静态工作流资产 + ai sync 渲染层（--target 过滤，默认 claude）+ add method 命令契约 + next-steps 引导 ai sync。

**Architecture:** 新增两个 embedded 资产（`docs/ai/ncgo-dev-workflow.*.md` 教程、`docs/ai/ncgo-dev-rules.*.md` 精简规则）；`internal/ai` 的 `target` 增加 `Group` 字段，`targets()` 从 4 目标变 5 目标，`Sync()` 按 `--target`（默认 claude）过滤；`renderCursorMDC` 改用 RulesBody；新增 `renderNcgoDevSkill`；`add method` 对齐 add domain 的 `NextSteps` + `--output json`，MCP 同步；各命令 next-steps 追加 `ncgo ai sync`。

**Tech Stack:** Go 1.25+, Cobra CLI, MCP stdio server, go/parser 不用（Phase 2）。

## Global Constraints

- `ai sync` 默认 target 从「全部 4 文件」变为「claude 组 3 文件」（CLAUDE.md + SKILL.md + project-context.md）——**有意为之的契约变更**（设计文档 §4）
- 所有 managed 文件必须带 `<!-- ncgo:managed -->` 标记；`isManaged` 扫描前 6 个非空行
- `ncgo add method` 不引入 `--dry-run` / `--plan`（设计文档 §3.3 范围说明）
- 新 next-steps 步 `ncgo ai sync --target all --root .` 必须加入 `skippedNextStepReasons`（scaffold 测试不得真实执行）
- 保持 gofmt 干净、黄金测试（golden）一致、EN/ZH 文档对齐

---

### Task 1: 新增 embedded 工作流与精简规则资产

**Files:**
- Create: `internal/assets/_data/docs/ai/ncgo-dev-workflow.en.md`
- Create: `internal/assets/_data/docs/ai/ncgo-dev-workflow.zh-CN.md`
- Create: `internal/assets/_data/docs/ai/ncgo-dev-rules.en.md`
- Create: `internal/assets/_data/docs/ai/ncgo-dev-rules.zh-CN.md`
- Modify: `internal/assets/assets_test.go`（EmbeddedFilesPresent 增加 4 个新路径）

**Interfaces:**
- Consumes: 无（纯资产）
- Produces: `docs/ai/ncgo-dev-workflow.<lang>.md`、`docs/ai/ncgo-dev-rules.<lang>.md`，Task 2 通过 `fs.ReadFile(assets.FS(), rel)` 读取

- [ ] **Step 1: 写失败测试 — assets_test.go 增加新路径断言**

在 `internal/assets/assets_test.go` 的 `want` 列表（`docs/micro/design-doc.zh-CN.md` 之后）追加：

```go
"docs/ai/ncgo-dev-workflow.en.md",
"docs/ai/ncgo-dev-workflow.zh-CN.md",
"docs/ai/ncgo-dev-rules.en.md",
"docs/ai/ncgo-dev-rules.zh-CN.md",
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./internal/assets/... -count=1`
Expected: FAIL — `ncgo-dev-workflow.en.md not present in embedded FS`

- [ ] **Step 3: 创建工作流资产 `internal/assets/_data/docs/ai/ncgo-dev-workflow.en.md`**

```markdown
## Implementing a Feature with ncgo

This project is generated and extended with the `ncgo` CLI. Follow this
workflow to add a new feature end-to-end. Every step has a programmatic
contract, so an AI agent can drive it directly.

### Workflow

1. **Add a domain** — `ncgo add domain <name> --root .`
   Generates `internal/usecase/<name>/`, `internal/repository/<name>/`, and
   `internal/base/data/<name>_register.go`, and records the domain in
   `.ncgo/manifest.yaml`. Domain names match `^[a-z][a-z0-9_]{0,62}$`.

2. **Add a usecase method** — `ncgo add method <domain>.<Method> --root .`
   Inserts a `func (u *UseCase) <Method>() error` stub between the
   `// ncgo:methods:start` and `// ncgo:methods:end` markers in the domain
   usecase file. Method names match `^[A-Z][A-Za-z0-9_]{0,62}$`.

3. **Regenerate database code** — `make sqlc`
   Required when the service uses a database (`cfg.Database.Enabled`). Kitex
   services always need this before `go mod tidy`; Hertz services need it
   only when the database scaffold is enabled.

4. **Verify** — `go build ./... && go vet ./... && go test ./... -count=1`
   The scaffold must stay buildable after each method insertion.

5. **Refresh AI context** — `ncgo ai sync --root .`
   Re-renders this project's AI artifacts (see below) so agent context
   reflects the new domain and methods.

### Anchors

- `// ncgo:methods:start` / `// ncgo:methods:end` — method insertion region
  in `internal/usecase/<domain>/<domain>.go`. Do not hand-edit generated
  methods; use `ncgo add method`.
- `// ncgo:wire:domain` — optional wiring marker for `data.Register<Name>`.
  When present, `ncgo add domain --wire` inserts the register call there.

### Verification Checklist

- [ ] `.ncgo/manifest.yaml` lists the new domain
- [ ] `internal/usecase/<domain>/<domain>.go` contains the new method between anchors
- [ ] `go build ./...` passes
- [ ] `ncgo ai sync --root .` completes and reports the managed files written

### Failure Handling

- `ncgo add domain` fails "already exists" → the domain is already present;
  run `ncgo add method` directly or pass `--force`.
- `ncgo add method` fails "missing markers" → the usecase file was hand-edited
  or never generated; regenerate the domain with `ncgo add domain <name> --force`.
- `make sqlc` fails → confirm `sqlc` is installed and the schema files are
  intact; see the project's design doc at `docs/ncgo/<profile>/design-doc.en.md`.
- `ncgo ai sync` refuses to overwrite → a file lacks the
  `<!-- ncgo:managed -->` marker; pass `--force` only if you own the file.
```

- [ ] **Step 4: 创建中文工作流资产 `internal/assets/_data/docs/ai/ncgo-dev-workflow.zh-CN.md`**

与 en 版结构一致，正文译为中文（标题「用 ncgo 实现一个功能」；步骤、锚点、验证清单、失败处理逐项对应）。关键命令文本保持英文命令原样（`ncgo add domain <name> --root .`）。

- [ ] **Step 5: 创建精简规则资产 `internal/assets/_data/docs/ai/ncgo-dev-rules.en.md`**

```markdown
## ncgo Project Rules

> Paths in the embedded design doc (`docs/ncgo/<profile>/design-doc.*.md`)
> are **template-internal** paths (e.g. `kitex/kitex-template/main.yaml`);
> the generated project's actual paths differ. Read the design doc to see
> the mapping for this project.

- Do not hand-edit generated files. Fix the template or generator instead.
- Respect layer boundaries: handler → usecase → repository.
- Run `make sqlc` before `go mod tidy` (Kitex always; Hertz only with database).
- Add usecase methods via `ncgo add method <domain>.<Method>`, not by hand.
- After changing manifest or generated code, run `ncgo ai sync --root .`.
- Full workflow: see "Implementing a Feature with ncgo" in `AGENTS.md`.
- Architecture reference: `docs/ncgo/<profile>/design-doc.en.md`.
```

- [ ] **Step 6: 创建中文精简规则资产 `internal/assets/_data/docs/ai/ncgo-dev-rules.zh-CN.md`**

与 en 版逐条对应，译为中文。

- [ ] **Step 7: 运行测试确认通过**

Run: `go test ./internal/assets/... -count=1`
Expected: PASS

- [ ] **Step 8: 提交**

```bash
git add internal/assets/_data/docs/ai/ internal/assets/assets_test.go
git commit -m "feat(ai): add ncgo-dev workflow and rules embedded assets (#45)"
```

---

### Task 2: ai 渲染层 — target Group、5 目标、renderNcgoDevSkill、append 工作流

**Files:**
- Modify: `internal/ai/render.go`
- Test: `internal/ai/render_test.go`（若无则新建）

**Interfaces:**
- Consumes: Task 1 的两个新资产；现有 `renderInputs`、`ManagedMarker`、`target` 结构
- Produces: `target` 结构体带 `Group string` 字段；`targets()` 返回 5 目标；`renderInputs` 增加 `WorkflowBody`、`RulesBody`；`renderNcgoDevSkill(inputs) string`；`renderAgents`/`renderClaude` append 工作流；`renderCursorMDC` 改用 RulesBody。Task 3 消费 `target.Group` 做过滤。

- [ ] **Step 1: 写失败测试 — render_test.go 断言新行为**

新建 `internal/ai/render_test.go`：

```go
package ai

import (
	"strings"
	"testing"
)

func TestTargetsHasFiveWithGroups(t *testing.T) {
	ts := targets()
	if len(ts) != 5 {
		t.Fatalf("targets() = %d, want 5", len(ts))
	}
	groups := map[string]int{}
	for _, tg := range ts {
		groups[tg.Group]++
	}
	if groups["claude"] != 3 {
		t.Fatalf("claude group count = %d, want 3 (CLAUDE.md + SKILL.md + project-context)", groups["claude"])
	}
	if groups["agents"] != 1 || groups["cursor"] != 1 {
		t.Fatalf("groups = %v, want agents=1 cursor=1", groups)
	}
}

func TestRenderAgentsAppendsWorkflow(t *testing.T) {
	body := renderAgents(renderInputs{LongBody: "# Design\n\narch body\n", WorkflowBody: "## Implementing a Feature with ncgo\nsteps\n"})
	if !strings.Contains(body, "arch body") || !strings.Contains(body, "Implementing a Feature with ncgo") {
		t.Fatalf("renderAgents missing long body or workflow:\n%s", body)
	}
}

func TestRenderCursorMDCUsesRulesBody(t *testing.T) {
	body := renderCursorMDC(renderInputs{RulesBody: "rule one\nrule two\n", LongBody: "full design doc\n"})
	if !strings.Contains(body, "rule one") || strings.Contains(body, "full design doc") {
		t.Fatalf("renderCursorMDC should embed rules not long body:\n%s", body)
	}
	if !strings.HasPrefix(body, "---\n") {
		t.Fatalf(".mdc must start with frontmatter:\n%s", body)
	}
}

func TestRenderNcgoDevSkillFrontmatterAndMarker(t *testing.T) {
	body := renderNcgoDevSkill(renderInputs{WorkflowBody: "## Implementing a Feature with ncgo\nsteps\n"})
	if !strings.HasPrefix(body, "---\nname: ncgo-dev\n") {
		t.Fatalf("SKILL.md must start with frontmatter name ncgo-dev:\n%s", body)
	}
	if !isManaged([]byte(body)) {
		t.Fatalf("SKILL.md must carry the managed marker within first 6 lines")
	}
	if !strings.Contains(body, "Implementing a Feature with ncgo") {
		t.Fatalf("SKILL.md missing workflow body:\n%s", body)
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./internal/ai/... -run 'TestTargetsHasFive|TestRenderAgents|TestRenderCursor|TestRenderNcgoDev' -count=1`
Expected: FAIL — `targets()` 仍返回 4、无 Group 字段、无 renderNcgoDevSkill

- [ ] **Step 3: 改 render.go — target 结构 + targets()**

```go
// target describes one rendered artifact.
type target struct {
	Group   string // target group: agents | claude | cursor
	RelPath string
	Render  func(inputs renderInputs) string
}

// targets returns the static set of artifacts produced by `ncgo ai sync`.
func targets() []target {
	return []target{
		{Group: "agents", RelPath: "AGENTS.md", Render: renderAgents},
		{Group: "claude", RelPath: "CLAUDE.md", Render: renderClaude},
		{Group: "claude", RelPath: ".claude/skills/ncgo-dev/SKILL.md", Render: renderNcgoDevSkill},
		{Group: "claude", RelPath: ".claude/generated/project-context.md", Render: renderProjectContext},
		{Group: "cursor", RelPath: ".cursor/rules/ncgo.mdc", Render: renderCursorMDC},
	}
}
```

- [ ] **Step 4: 改 render.go — renderInputs + 常量**

```go
type renderInputs struct {
	SourceRef          string
	LongBody           string
	ProjectContextBody string
	WorkflowBody       string
	RulesBody          string
}
```

- [ ] **Step 5: 改 render.go — renderAgents / renderClaude append 工作流**

在 `renderAgents` 中 `b.WriteString(inputs.LongBody)` 之后追加：

```go
b.WriteString(inputs.WorkflowBody)
```

`renderClaude` 同样在其 `b.WriteString(inputs.LongBody)` 之后追加 `b.WriteString(inputs.WorkflowBody)`。

- [ ] **Step 6: 改 render.go — renderCursorMDC 改用 RulesBody**

将 `renderCursorMDC` 中的 `b.WriteString(inputs.LongBody)` 改为 `b.WriteString(inputs.RulesBody)`；description 文案改为 "ncgo project rules and feature workflow (auto-generated)"（保持 `globs` / `alwaysApply` 不变）。

- [ ] **Step 7: 新增 renderNcgoDevSkill**

```go
// renderNcgoDevSkill produces the ncgo-dev Claude Code skill file. The
// frontmatter precedes the managed marker so the skill loads correctly and
// isManaged() still finds the marker within the first 6 lines.
func renderNcgoDevSkill(inputs renderInputs) string {
	var b strings.Builder
	b.WriteString("---\n")
	b.WriteString("name: ncgo-dev\n")
	b.WriteString("description: Implement a feature in this ncgo project (add domain → add method → sqlc → verify → ai sync)\n")
	b.WriteString("---\n")
	b.WriteString(ManagedMarker + "\n\n")
	b.WriteString(inputs.WorkflowBody)
	return ensureTrailingNewline(b.String())
}
```

- [ ] **Step 8: 运行 render_test 确认通过**

Run: `go test ./internal/ai/... -run 'TestTargetsHasFive|TestRenderAgents|TestRenderCursor|TestRenderNcgoDev' -count=1`
Expected: PASS

- [ ] **Step 9: 提交**

```bash
git add internal/ai/render.go internal/ai/render_test.go
git commit -m "feat(ai): target groups, workflow append, skill render (#45)"
```

---

### Task 3: ai sync — buildInputs 读资产 + --target 过滤（默认 claude）

**Files:**
- Modify: `internal/ai/sync.go`
- Modify: `internal/ai/render.go`（`buildInputs`）

**Interfaces:**
- Consumes: Task 2 的 `target.Group`、`renderInputs.WorkflowBody/RulesBody`；`assets.FS()`
- Produces: `Options.Target string`；常量 `TargetAll/TargetAgents/TargetClaude/TargetCursor`；`Sync` 按 target 过滤；`Result.Target string`。Task 4/5/6 消费这些。

- [ ] **Step 1: 写失败测试 — sync_test.go 增加 target 过滤用例**

在 `internal/ai/sync_test.go` 追加：

```go
func TestSyncDefaultTargetIsClaude(t *testing.T) {
	root := t.TempDir()
	writeManifest(t, root, manifest.KindHertz)
	res, err := Sync(Options{Root: root})
	if err != nil {
		t.Fatalf("Sync: %v", err)
	}
	if res.Target != TargetClaude {
		t.Fatalf("Target = %q, want claude default", res.Target)
	}
	want := []string{"CLAUDE.md", ".claude/skills/ncgo-dev/SKILL.md", ".claude/generated/project-context.md"}
	for _, p := range want {
		if _, err := os.Stat(filepath.Join(root, p)); err != nil {
			t.Errorf("default sync should write %s: %v", p, err)
		}
	}
	for _, p := range []string{"AGENTS.md", ".cursor/rules/ncgo.mdc"} {
		if _, err := os.Stat(filepath.Join(root, p)); !os.IsNotExist(err) {
			t.Errorf("default sync must NOT write %s", p)
		}
	}
}

func TestSyncTargetAllWritesAll(t *testing.T) {
	root := t.TempDir()
	writeManifest(t, root, manifest.KindHertz)
	res, err := Sync(Options{Root: root, Target: TargetAll})
	if err != nil {
		t.Fatalf("Sync: %v", err)
	}
	for _, p := range []string{"AGENTS.md", "CLAUDE.md", ".cursor/rules/ncgo.mdc", ".claude/skills/ncgo-dev/SKILL.md", ".claude/generated/project-context.md"} {
		if _, err := os.Stat(filepath.Join(root, p)); err != nil {
			t.Errorf("Target all should write %s: %v", p, err)
		}
	}
}

func TestSyncTargetAgentsOnly(t *testing.T) {
	root := t.TempDir()
	writeManifest(t, root, manifest.KindHertz)
	if _, err := Sync(Options{Root: root, Target: TargetAgents}); err != nil {
		t.Fatalf("Sync: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "AGENTS.md")); err != nil {
		t.Errorf("target agents should write AGENTS.md: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "CLAUDE.md")); !os.IsNotExist(err) {
		t.Errorf("target agents must not write CLAUDE.md")
	}
}

func TestSyncRejectsBadTarget(t *testing.T) {
	root := t.TempDir()
	writeManifest(t, root, manifest.KindHertz)
	if _, err := Sync(Options{Root: root, Target: "bogus"}); err == nil {
		t.Fatalf("expected error for invalid target")
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./internal/ai/... -run 'TestSyncDefaultTarget|TestSyncTargetAll|TestSyncTargetAgents|TestSyncRejectsBadTarget' -count=1`
Expected: FAIL — 无 Target 字段、默认行为仍是全量

- [ ] **Step 3: 改 sync.go — Options + 常量**

```go
// Target groups for `ncgo ai sync --target`.
const (
	TargetAll    = "all"
	TargetAgents = "agents"
	TargetClaude = "claude"
	TargetCursor = "cursor"
)
```

`Options` 增加 `Target string // target group to render; empty defaults to claude (all = every group)`。

`Result` 增加 `Target string \`json:"target,omitempty"\``。

新增校验函数：

```go
func validateTarget(target string) error {
	switch target {
	case TargetAll, TargetAgents, TargetClaude, TargetCursor:
		return nil
	default:
		return fmt.Errorf("ai sync: --target %q is invalid (all|agents|claude|cursor)", target)
	}
}
```

- [ ] **Step 4: 改 render.go — buildInputs 读两个新资产（加 lang 参数）**

`buildInputs` 增加 `lang string` 参数（`syncSource` 不含 lang 字段），签名与调用点同步更新：

```go
func buildInputs(source syncSource, local, lang string) renderInputs {
	inputs := renderInputs{SourceRef: source.SourceRef}
	switch source.Scope {
	case syncScopeWorkspace:
		inputs.LongBody = buildWorkspaceLongBody(source.Workspace, source.WorkspaceServices, source.DesignDoc, local)
		inputs.ProjectContextBody = buildWorkspaceProjectContextBody(source.Workspace, source.WorkspaceServices, source.DesignDoc)
	default:
		inputs.LongBody = buildServiceLongBody(source.Service, source.ServiceWorkspace, source.DesignDoc, local)
		inputs.ProjectContextBody = buildServiceProjectContextBody(source.Service, source.ServiceWorkspace, source.DesignDoc)
	}
	inputs.WorkflowBody = readAIDoc("ncgo-dev-workflow."+lang+".md")
	inputs.RulesBody = readAIDoc("ncgo-dev-rules."+lang+".md")
	return inputs
}

// readAIDoc reads one docs/ai/ asset; a missing asset falls back to "" so
// older embedded assets versions still render without a hard failure.
func readAIDoc(name string) string {
	rel := filepath.ToSlash(filepath.Join("docs", "ai", name))
	b, err := fs.ReadFile(assets.FS(), rel)
	if err != nil {
		return ""
	}
	return string(b)
}
```

调用点（`sync.go` 的 `Sync`）：`inputs := buildInputs(source, local, opts.Lang)`。

- [ ] **Step 5: 改 sync.go — Sync() 过滤**

```go
func Sync(opts Options) (*Result, error) {
	if opts.Root == "" {
		opts.Root = "."
	}
	if opts.Lang == "" {
		opts.Lang = LangEN
	}
	if opts.Target == "" {
		opts.Target = TargetClaude
	}
	if err := validateTarget(opts.Target); err != nil {
		return nil, err
	}
	if opts.Lang != LangEN && opts.Lang != LangZhCN {
		return nil, fmt.Errorf("ai sync: --lang %q is invalid (en|zh-CN)", opts.Lang)
	}
	source, err := resolveSyncSource(opts.Root, opts.Lang)
	if err != nil {
		return nil, err
	}
	local, err := readLocalNotes(opts.Root)
	if err != nil {
		return nil, err
	}
	inputs := buildInputs(source, local, opts.Lang)
	res := newSyncResult(source)
	res.Target = opts.Target
	for _, t := range targets() {
		if opts.Target != TargetAll && t.Group != opts.Target {
			continue
		}
		if err := writeTarget(opts, t, inputs, res); err != nil {
			return res, err
		}
	}
	profile := resolveProfile(source)
	if err := writeStandaloneDocs(opts, res, profile); err != nil {
		return res, err
	}
	return res, nil
}
```

- [ ] **Step 6: 运行新测试确认通过**

Run: `go test ./internal/ai/... -run 'TestSyncDefaultTarget|TestSyncTargetAll|TestSyncTargetAgents|TestSyncRejectsBadTarget' -count=1`
Expected: PASS

- [ ] **Step 7: 修复受默认值影响的既有测试**

以下测试调用 `Sync(Options{Root: root})` 期望全量写入，默认 claude 后必须显式传 `Target: TargetAll`：
- `TestSyncWritesAllTargets`（read AGENTS.md/.mdc）
- `TestSyncPicksKitexDoc`（read AGENTS.md）
- `TestSyncServiceUnderWorkspaceAddsMembershipFacts`（read AGENTS.md）
- `TestSyncServiceUnderWorkspaceSkipsUnlistedParentWorkspace`（read project-context，claude 组——默认即可，无需改；确认后跳过）
- `TestSyncRefusesUnmanagedFile`（read AGENTS.md）→ `Target: TargetAll`
- `TestSyncForceOverwritesUnmanagedFile` → `Target: TargetAll`
- `TestSyncRefusesSymlinkEscape` / `TestSyncForceStillRefusesSymlinkEscape` / `TestSyncRefusesDanglingSymlink` / `TestSyncAllowsSymlinkWithinRoot`（均 read AGENTS.md）→ `Target: TargetAll`
- `TestSyncAppendsLocalNotes`（read AGENTS.md）→ `Target: TargetAll`
- `TestSyncDryRunWritesNothing`（遍历 `targets()`，断言所有目标 skipped）→ 改为 `Target: TargetAll`，且 `res.Skipped` 需覆盖 5 目标 + 3 standalone
- `TestSyncZhLangPicksZhDoc`（read AGENTS.md）→ `Target: TargetAll`
- `TestSyncWorkspaceWritesAllTargets`（read AGENTS.md）→ `Target: TargetAll`
- `TestSyncWorkspaceAppendsLocalNotesOnlyToLongFormFiles`（read AGENTS.md）→ `Target: TargetAll`
- `TestSyncWorkspaceDryRunWritesNothing` → `Target: TargetAll`
- `TestSyncWorkspaceZhLangPicksZhDoc`（read project-context，claude 组）→ 默认即可

**重要**：`TestSyncWritesAllTargets` 中 `wantPaths` 需从 4 项变 5 项（追加 `.claude/skills/ncgo-dev/SKILL.md`），且对 .mdc 的断言从「含 design-doc body」改为「含精简规则」：

```go
wantPaths := []string{"AGENTS.md", "CLAUDE.md", ".cursor/rules/ncgo.mdc", ".claude/generated/project-context.md", ".claude/skills/ncgo-dev/SKILL.md"}
```

SKILL.md 断言（该文件不包含 manifest 摘要，跳过通用 body 断言，单独处理）：

```go
if p == ".claude/skills/ncgo-dev/SKILL.md" {
	if !strings.Contains(body, "name: ncgo-dev") || !strings.Contains(body, "Implementing a Feature with ncgo") {
		t.Errorf("%s missing skill frontmatter or workflow body", p)
	}
	continue
}
```

`.mdc` 断言改为：

```go
if !strings.Contains(body, "ncgo Project Rules") && !strings.Contains(body, "Do not hand-edit generated files") {
	t.Errorf("%s missing rules body", p)
}
```

- [ ] **Step 8: 运行 ai 全量测试**

Run: `go test ./internal/ai/... -count=1`
Expected: PASS

- [ ] **Step 9: 提交**

```bash
git add internal/ai/sync.go internal/ai/render.go internal/ai/sync_test.go
git commit -m "feat(ai): ai sync target filtering with claude default (#45)"
```

---

### Task 4: CLI `ai sync --target` flag

**Files:**
- Modify: `internal/cli/ai.go`
- Modify: `internal/cli/ai_test.go`

**Interfaces:**
- Consumes: Task 3 的 `ai.TargetAll/TargetAgents/TargetClaude/TargetCursor`、`Options.Target`
- Produces: `aiSyncOptions.target string`；`runAISync` 传 `Target: opts.target`

- [ ] **Step 1: 写失败测试 — ai_test.go 增加 --target 用例**

在 `internal/cli/ai_test.go` 追加：

```go
func TestRunAISyncTargetFlag(t *testing.T) {
	root := t.TempDir()
	writeCLIServiceManifest(t, root)
	var out bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&out)
	if err := runAISync(cmd, &aiSyncOptions{root: root, target: ai.TargetAgents, output: "text"}); err != nil {
		t.Fatalf("runAISync: %v", err)
	}
	got := out.String()
	if !strings.Contains(got, "wrote AGENTS.md") {
		t.Fatalf("--target agents should write AGENTS.md: %s", got)
	}
	if strings.Contains(got, "wrote CLAUDE.md") {
		t.Fatalf("--target agents must not write CLAUDE.md: %s", got)
	}
}

func TestRunAISyncDefaultTargetTextShowsClaudeFiles(t *testing.T) {
	root := t.TempDir()
	writeCLIServiceManifest(t, root)
	var out bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&out)
	if err := runAISync(cmd, &aiSyncOptions{root: root, output: "text"}); err != nil {
		t.Fatalf("runAISync: %v", err)
	}
	got := out.String()
	for _, want := range []string{"wrote CLAUDE.md", "wrote .claude/skills/ncgo-dev/SKILL.md", "wrote .claude/generated/project-context.md"} {
		if !strings.Contains(got, want) {
			t.Fatalf("default sync missing %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "wrote AGENTS.md") {
		t.Fatalf("default sync must not write AGENTS.md:\n%s", got)
	}
}

func TestRunAISyncRejectsInvalidTarget(t *testing.T) {
	root := t.TempDir()
	writeCLIServiceManifest(t, root)
	cmd := &cobra.Command{}
	if err := runAISync(cmd, &aiSyncOptions{root: root, target: "bogus"}); err == nil || !strings.Contains(err.Error(), "--target") {
		t.Fatalf("err = %v, want --target validation", err)
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./internal/cli/... -run 'TestRunAISyncTarget|TestRunAISyncDefaultTarget|TestRunAISyncRejectsInvalidTarget' -count=1`
Expected: FAIL — 无 target 字段

- [ ] **Step 3: 改 ai.go**

`aiSyncOptions` 增加 `target string`。`newAISyncCmd` 注册 flag：

```go
f.StringVar(&opts.target, "target", "", "Target group to render: all, agents, claude, cursor (default claude)")
```

`runAISync` 中传给 `ai.Sync`：

```go
res, err := ai.Sync(ai.Options{
	Root:   opts.root,
	Lang:   opts.lang,
	Force:  opts.force,
	DryRun: opts.dryRun,
	Target: opts.target,
})
```

- [ ] **Step 4: 运行测试确认通过**

Run: `go test ./internal/cli/... -run 'TestRunAISyncTarget|TestRunAISyncDefaultTarget|TestRunAISyncRejectsInvalidTarget' -count=1`
Expected: PASS

- [ ] **Step 5: 修复受影响的既有 CLI 测试**

- `TestRunAISyncTextOutput`（期望 `skipped AGENTS.md (dry-run)`）→ 加 `target: ai.TargetAll`，否则默认 claude 不报 AGENTS
- `TestRunAISyncJSONOutput`（期望 `len(res.Skipped) != 7`）→ 加 `target: ai.TargetAll`，且 7 改为 8（5 目标 + 3 standalone）

- [ ] **Step 6: 运行 cli 测试**

Run: `go test ./internal/cli/... -count=1`
Expected: PASS

- [ ] **Step 7: 提交**

```bash
git add internal/cli/ai.go internal/cli/ai_test.go
git commit -m "feat(cli): ai sync --target flag (#45)"
```

---

### Task 5: MCP `ncgo_ai_sync` target 参数

**Files:**
- Modify: `internal/mcp/tools.go`
- Modify: `internal/mcp/server_test.go`（或对应 ai_sync 测试所在文件）

**Interfaces:**
- Consumes: Task 3 的 `ai.Target*` 常量、`ai.Options.Target`
- Produces: `ncgo_ai_sync` 工具 InputSchema 增加 `enumField("target", ...)`

- [ ] **Step 1: 写失败测试 — server_test.go 增加 target 参数**

追加：

```go
func TestServeToolCallAISyncTargetParam(t *testing.T) {
	root := seedMCPProject(t, manifest.KindHertz)
	input := EncodeMessage(map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": "tools/call",
		"params": map[string]any{"name": "ncgo_ai_sync", "arguments": map[string]any{"root": root, "target": "agents"}},
	})
	var out bytes.Buffer
	if err := New("test-version", "test-assets").Serve(context.Background(), bytes.NewReader(input), &out); err != nil {
		t.Fatalf("Serve: %v", err)
	}
	responses, err := DecodeResponses(out.Bytes())
	if err != nil {
		t.Fatalf("DecodeResponses: %v", err)
	}
	result := responses[0].Result.(map[string]any)
	text := resultText(result)
	if !strings.Contains(text, "wrote AGENTS.md") {
		t.Fatalf("target agents should write AGENTS.md: %s", text)
	}
	if strings.Contains(text, "wrote CLAUDE.md") {
		t.Fatalf("target agents must not write CLAUDE.md: %s", text)
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./internal/mcp/... -run TestServeToolCallAISyncTargetParam -count=1`
Expected: FAIL — tool 忽略 target 参数

- [ ] **Step 3: 改 tools.go**

`ncgo_ai_sync` 的 InputSchema 增加：

```go
enumField("target", []string{ai.TargetAll, ai.TargetAgents, ai.TargetClaude, ai.TargetCursor}),
```

在 `callAISync`（tool_ai.go 或对应文件）的参数结构体增加 `Target string \`json:"target"\`` 并传给 `ai.Sync`。

- [ ] **Step 4: 运行测试确认通过**

Run: `go test ./internal/mcp/... -run TestServeToolCallAISyncTargetParam -count=1`
Expected: PASS

- [ ] **Step 5: 修复受影响的既有 MCP 测试**

`TestServeToolCallAISyncIncludesStructuredFields` 期望 `len(result["skipped"]) != 6`（4 目标 + 2 standalone）→ 加 `"target": "all"` 参数，且 6 改为 7（5 目标 + 2 standalone）。同样 `TestServeToolCallAISyncJSON` 若依赖默认全量需确认。

- [ ] **Step 6: 运行 mcp 测试**

Run: `go test ./internal/mcp/... -count=1`
Expected: PASS

- [ ] **Step 7: 提交**

```bash
git add internal/mcp/tools.go internal/mcp/server_test.go
git commit -m "feat(mcp): ncgo_ai_sync target parameter (#45)"
```

---

### Task 6: `ncgo add method` 契约 — NextSteps + `--output json`

**Files:**
- Modify: `internal/scaffold/method/method.go`
- Modify: `internal/cli/add.go`
- Modify: `internal/scaffold/method/method_test.go`
- Modify: `internal/cli/add_test.go`

**Interfaces:**
- Consumes: 现有 `method.Result`
- Produces: `method.Result.NextSteps []string`；`add method --output json` 输出 `{path, domain, method, nextSteps}`

- [ ] **Step 1: 写失败测试 — method_test.go 断言 NextSteps**

```go
func TestAddMethodResultHasNextSteps(t *testing.T) {
	// reuse existing Add test setup; assert res.NextSteps non-empty and
	// contains "ncgo ai sync"
	res, err := Add(Options{Root: root, Spec: "device.Get"})
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	if len(res.NextSteps) == 0 {
		t.Fatalf("Result.NextSteps must not be empty")
	}
	joined := strings.Join(res.NextSteps, "\n")
	if !strings.Contains(joined, "ncgo ai sync --root .") {
		t.Fatalf("NextSteps missing ai sync hint:\n%s", joined)
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./internal/scaffold/method/... -count=1`
Expected: FAIL — Result 无 NextSteps 字段

- [ ] **Step 3: 改 method.go — Result + Add**

```go
type Result struct {
	Path      string
	Domain    string
	Method    string
	NextSteps []string // follow-up commands for the agent
}
```

`Add` 末尾填充：

```go
return &Result{
	Path:      path,
	Domain:    domain,
	Method:    method,
	NextSteps: []string{
		"go build ./...",
		"replace the generated stub body with domain logic",
		"ncgo ai sync --root .",
	},
}, nil
```

- [ ] **Step 4: 写失败测试 — add_test.go 断言 --output json**

```go
func TestRunAddMethodJSONOutput(t *testing.T) {
	root := t.TempDir()
	// scaffold a project + domain so add method has a target file
	// (reuse existing add method test setup from add_test.go)
	var out bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&out)
	if err := runAddMethod(cmd, "device.Get", &addMethodOptions{root: root, output: "json"}); err != nil {
		t.Fatalf("runAddMethod json: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("json unmarshal: %v\n%s", err, out.String())
	}
	for _, k := range []string{"path", "domain", "method", "nextSteps"} {
		if _, ok := got[k]; !ok {
			t.Errorf("json output missing %q: %v", k, got)
		}
	}
}
```

- [ ] **Step 5: 运行测试确认失败**

Run: `go test ./internal/cli/... -run TestRunAddMethodJSONOutput -count=1`
Expected: FAIL — addMethodOptions 无 output 字段

- [ ] **Step 6: 改 add.go — flag + 输出**

`addMethodOptions` 增加 `output string`。`newAddMethodCmd` 注册：

```go
f.StringVar(&opts.output, "output", "text", "Output format: text or json")
```

`runAddMethod` 改为：

```go
func runAddMethod(cmd *cobra.Command, spec string, opts *addMethodOptions) error {
	if err := validateAddOutput("add method", opts.output); err != nil {
		return err
	}
	res, err := method.Add(method.Options{Root: opts.root, Spec: spec, Layer: opts.layer})
	if err != nil {
		return err
	}
	out := cmd.OutOrStdout()
	if opts.output == "json" {
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		return enc.Encode(struct {
			Path      string   `json:"path"`
			Domain    string   `json:"domain"`
			Method    string   `json:"method"`
			NextSteps []string `json:"nextSteps"`
		}{Path: res.Path, Domain: res.Domain, Method: res.Method, NextSteps: res.NextSteps})
	}
	fmt.Fprintf(out, "inserted %s.%s into %s\n", res.Domain, res.Method, res.Path)
	fmt.Fprintln(out, "\nnext steps:")
	for _, s := range res.NextSteps {
		fmt.Fprintf(out, "  - %s\n", s)
	}
	return nil
}
```

- [ ] **Step 7: 运行测试确认通过**

Run: `go test ./internal/scaffold/method/... ./internal/cli/... -count=1`
Expected: PASS

- [ ] **Step 8: 提交**

```bash
git add internal/scaffold/method/method.go internal/scaffold/method/method_test.go internal/cli/add.go internal/cli/add_test.go
git commit -m "feat(method): add method NextSteps + --output json (#45)"
```

---

### Task 7: MCP `ncgo_add_method` 结构化字段

**Files:**
- Modify: `internal/mcp/tool_add.go`
- Modify: `internal/mcp/server_test.go`

**Interfaces:**
- Consumes: Task 6 的 `method.Result.NextSteps`
- Produces: `ncgo_add_method` 支持 `output` 参数；JSON 输出顶层字段 `path/domain/method/nextSteps`

- [ ] **Step 1: 写失败测试 — server_test.go 断言 ncgo_add_method json**

```go
func TestServeToolCallAddMethodJSON(t *testing.T) {
	root := seedMCPProject(t, manifest.KindHertz)
	// ensure a domain exists so add method has a target (see seedMCP helpers)
	input := EncodeMessage(map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": "tools/call",
		"params": map[string]any{"name": "ncgo_add_method", "arguments": map[string]any{"root": root, "spec": "device.Get", "output": "json"}},
	})
	var out bytes.Buffer
	if err := New("test-version", "test-assets").Serve(context.Background(), bytes.NewReader(input), &out); err != nil {
		t.Fatalf("Serve: %v", err)
	}
	responses, err := DecodeResponses(out.Bytes())
	if err != nil {
		t.Fatalf("DecodeResponses: %v", err)
	}
	result := responses[0].Result.(map[string]any)
	if result["isError"].(bool) {
		t.Fatalf("add method returned error: %s", resultText(result))
	}
	obj := resultJSONObject(t, result)
	for _, k := range []string{"path", "domain", "method", "nextSteps"} {
		if _, ok := obj[k]; !ok {
			t.Errorf("add method json missing %q: %v", k, obj)
		}
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./internal/mcp/... -run TestServeToolCallAddMethodJSON -count=1`
Expected: FAIL — callAddMethod 无 output 参数、无结构化字段

- [ ] **Step 3: 改 tool_add.go**

```go
func callAddMethod(raw json.RawMessage) (map[string]any, error) {
	var args struct {
		Root   string `json:"root"`
		Spec   string `json:"spec"`
		Layer  string `json:"in"`
		Output string `json:"output"`
	}
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, err
	}
	output, err := resolveOutput(args.Output)
	if err != nil {
		return textResult(err.Error(), true), nil
	}
	res, err := method.Add(method.Options{Root: args.Root, Spec: args.Spec, Layer: args.Layer})
	if err != nil {
		return textResult(err.Error(), true), nil
	}
	if output == mcpOutputJSON {
		return map[string]any{
			"isError":   false,
			"path":      res.Path,
			"domain":    res.Domain,
			"method":    res.Method,
			"nextSteps": res.NextSteps,
			"content":   []any{map[string]any{"type": "text", "text": fmt.Sprintf("inserted %s.%s into %s", res.Domain, res.Method, res.Path)}},
		}, nil
	}
	return textResult(fmt.Sprintf("inserted %s.%s into %s", res.Domain, res.Method, res.Path), false), nil
}
```

> 确认 `mcpOutputText` / `mcpOutputJSON` 常量与 `resolveOutput` helper 的现有定义；若 `resolveOutput` 不存在，则复用 `structuredMCPTool` 模式或直接解析 `mcpOutputText/mcpOutputJSON` 字符串（参照 `callAddInfra` 的 `addInfraMCPTool.resolveOutput`）。

- [ ] **Step 4: 运行测试确认通过**

Run: `go test ./internal/mcp/... -run TestServeToolCallAddMethodJSON -count=1`
Expected: PASS

- [ ] **Step 5: 运行 mcp 全量测试**

Run: `go test ./internal/mcp/... -count=1`
Expected: PASS

- [ ] **Step 6: 提交**

```bash
git add internal/mcp/tool_add.go internal/mcp/server_test.go
git commit -m "feat(mcp): ncgo_add_method structured JSON output (#45)"
```

---

### Task 8: next-steps 引导 `ncgo ai sync`

**Files:**
- Modify: `internal/scaffold/mono/files.go`
- Modify: `internal/scaffold/domain/domain.go`
- Modify: `internal/scaffold/mono/mono_test.go`
- Modify: `internal/scaffold/domain/domain_test.go`

**Interfaces:**
- Consumes: 现有 `postGenerateNextSteps(opts)`、`domain.nextSteps(name, wired)`
- Produces: 两函数各追加一个 `ncgo ai sync` step

- [ ] **Step 1: 写失败测试 — mono_test.go 更新 stepSequenceShape 期望**

在 `TestNextStepsSequenceShapes` 的 want 序列中，post-* 用例末尾追加 `"ncgo ai sync --target all --root ."`：

```go
{name: "post-hertz-default", kind: manifest.KindHertz, postGenerate: true, want: []string{"cd", "go mod tidy", "make dev", "ncgo ai sync --target all --root ."}},
{name: "post-hertz-with-db", kind: manifest.KindHertz, withDB: true, postGenerate: true, want: []string{"cd", "make sqlc", "go mod tidy", "make migrate-up", "make dev", "ncgo ai sync --target all --root ."}},
{name: "post-kitex-default", kind: manifest.KindKitex, postGenerate: true, want: []string{"cd", "make sqlc", "go mod tidy", "make dev", "ncgo ai sync --target all --root ."}},
```

`TestShouldSkipNextStep` 增加用例：

```go
{step: "ncgo ai sync --target all --root .", want: true},
```

`skippedNextStepReasons` 增加条目：

```go
"ncgo ai sync --target all --root .": "requires the ncgo CLI binary in PATH; scaffold tests cover this path separately",
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./internal/scaffold/mono/... -run 'TestNextStepsSequenceShapes|TestShouldSkipNextStep' -count=1`
Expected: FAIL — post-* 序列缺 ai sync

- [ ] **Step 3: 改 files.go — postGenerateNextSteps 追加**

```go
steps = append(steps, "ncgo ai sync --target all --root .")
```

置于 `make dev` 之后（该步本身被 scaffold 测试跳过，实际运行前用户/agent 会替换为真实 `ncgo ai sync`）。

> 设计说明：`ncgo new` 是全新项目，需要全量上下文 → `--target all`。`add rpc` / `add bff` 复用 mono 的 `postGenerateNextSteps`（`rpc.Add`/`bff.Add` 直接透传 `monoRes.NextSteps`），因此它们自动继承 `--target all`——对**新生成的服务目录**这是正确的（同样需要全量上下文）。

- [ ] **Step 4: 写失败测试 — domain_test.go 断言 nextSteps 含 ai sync**

```go
func TestDomainNextStepsIncludeAISync(t *testing.T) {
	steps := nextSteps("device", true) // wired=true
	joined := strings.Join(steps, "\n")
	if !strings.Contains(joined, "ncgo ai sync --root .") {
		t.Fatalf("domain nextSteps missing ai sync:\n%s", joined)
	}
}
```

- [ ] **Step 5: 运行测试确认失败**

Run: `go test ./internal/scaffold/domain/... -run TestDomainNextStepsIncludeAISync -count=1`
Expected: FAIL — nextSteps 无 ai sync

- [ ] **Step 6: 改 domain.go — nextSteps 追加**

```go
func nextSteps(name string, wired bool) []string {
	export := exportName(name)
	steps := []string{"go mod tidy"}
	if !wired {
		steps = append([]string{
			fmt.Sprintf("wire %s into cmd/server/main.go: data.Register%s(injector)", name, export),
		}, steps...)
	}
	return append(steps, "ncgo ai sync --root .")
}
```

- [ ] **Step 7: 运行测试确认通过**

Run: `go test ./internal/scaffold/mono/... ./internal/scaffold/domain/... -count=1`
Expected: PASS

> 若 `TestNextStepsMakeTargetsMatchTemplates` / 其它 golden 断言受 postGenerateNextSteps 序列变化影响，需按实际失败更新其 want 序列。

- [ ] **Step 8: 提交**

```bash
git add internal/scaffold/mono/files.go internal/scaffold/mono/mono_test.go internal/scaffold/domain/domain.go internal/scaffold/domain/domain_test.go
git commit -m "feat(scaffold): next-steps guide ncgo ai sync (#45)"
```

---

### Task 9: 文档（README / docs/examples，EN/ZH）

**Files:**
- Modify: `README.md` / `README.zh-CN.md`
- Modify: `docs/examples.md` / `docs/examples.zh-CN.md`

**Interfaces:**
- Consumes: Task 3/4/5/6 的 CLI/MCP 契约变化
- Produces: 更新的命令文档

- [ ] **Step 1: README（EN）更新 `ai sync` 与 `add method` 段落**

`ai sync`：新增 `--target` 说明——默认 `claude`（CLAUDE.md + SKILL.md + project-context.md），`agents`/`cursor`/`all` 可选；迁移提示：旧全量行为需 `--target all`。

`add method`：新增 `--output json` 说明与 NextSteps 输出示例。

- [ ] **Step 2: README（ZH）同步更新**

对照 EN 逐条翻译。

- [ ] **Step 3: docs/examples（EN）更新工作流示例**

新增或更新「Implementing a Feature」示例，展示 `add domain → add method --output json → make sqlc → ai sync --target all`。

- [ ] **Step 4: docs/examples（ZH）同步更新**

- [ ] **Step 5: 运行 markdown 诊断 + smoke**

Run: `./scripts/smoke.sh`
Expected: PASS（若 smoke 依赖具体 next-steps 文本，按失败更新）

- [ ] **Step 6: 提交**

```bash
git add README.md README.zh-CN.md docs/examples.md docs/examples.zh-CN.md
git commit -m "docs(ai): ai sync --target, add method --output json (#45)"
```

---

### Task 10: 全仓库验证

- [ ] **Step 1: 全量构建与测试**

Run: `go build ./... && go build . && go vet ./... && go test ./... -count=1`
Expected: PASS

- [ ] **Step 2: golden 更新（如需）**

若 mono scaffold golden 因 next-steps 变化受影响：

Run: `go test ./internal/scaffold/mono/... -update-golden -count=1 && go test ./internal/scaffold/mono/... -count=1`

- [ ] **Step 3: smoke**

Run: `./scripts/smoke.sh`
Expected: PASS

- [ ] **Step 4: 提交任何遗留变更**

```bash
git add -A && git commit -m "chore: final validation for #45" || true
```
