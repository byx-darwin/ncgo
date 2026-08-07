# Issue #45 — AI Agent 端到端用 ncgo 做功能开发

**日期**: 2026-08-07
**范围**: 本期实现 Phase 1（静态工作流 + 命令契约）；顺带产出 Phase 2（动态层）设计草案，不实现代码
**关联 Issue**: [#45](https://github.com/byx-darwin/ncgo/issues/45)

---

## 1. 背景与目标

ncgo 生成的 AI 上下文（AGENTS.md/CLAUDE.md/project-context）目前只描述模板架构，没有教 agent「拿到功能需求后如何用 ncgo 落地」。目标：让 Claude Code（并兜底通用 agent）能端到端用 ncgo 实现功能开发。采用方案 C（混合）：静态层教「常识与工作流」，动态层教「当前状态」，命令契约层让 agent 程序化推进。

分两期：
- **Phase 1（本期实现）**：静态层（工作流内容）+ 命令契约层
- **Phase 2（本期仅设计草案）**：动态层（运行时上下文与校验）

## 2. 设计决策

| # | 决策 | 结论 | 理由 |
|---|------|------|------|
| D1 | 本期范围 | 只实现 Phase 1，Phase 2 只出设计草案 | 变更面大，分开评审更快 |
| D2 | 工作流内容存放位置 | 独立 embedded 资产 `internal/assets/_data/docs/ai/ncgo-dev-workflow.{en,zh-CN}.md` | 工作流是 CLI 用法，与 profile 架构解耦，跨 hertz/kitex/micro 共享，一次维护 |
| D3 | .cursor 精简规则 | 独立资产 `ncgo-dev-rules.{en,zh-CN}.md` | 替代「整篇复制 design-doc」的做法 |
| D4 | `ai sync --target` 过滤 | 按消费者分组；**默认 `claude`** | agent 只刷新它运行环境需要的文件 |
| D5 | SKILL.md 归属 | 归入 `claude` target | `.claude/skills/` 是 Claude Code 专属机制 |
| D6 | add method 契约 | 对齐 add domain：`NextSteps` + `--output json` | 命令契约层统一，agent 可程序化推进 |

## 3. Phase 1 详细设计

### 3.1 新增 embedded 资产

**A. 工作流教程** — `internal/assets/_data/docs/ai/ncgo-dev-workflow.en.md` / `.zh-CN.md`

内容结构（供 AGENTS.md/CLAUDE.md append 与 SKILL.md 复用）：

```markdown
## Implementing a Feature with ncgo

1. `ncgo add domain <name>` — 生成 domain（usecase / repository / DI register）
2. `ncgo add method <domain.Method>` — 在 // ncgo:methods:start|end 锚点插入方法 stub
3. `make sqlc` — 重新生成 DB 代码（with_database 时需要）
4. 验证 — go build ./... && go vet ./... && go test ./...
5. `ncgo ai sync --root .` — 刷新 AI 上下文
```

含锚点说明（`// ncgo:methods:start` / `// ncgo:methods:end` / `// ncgo:wire:domain`）、验证清单、失败处理（命令失败时的排查建议）。

**B. 精简规则** — `internal/assets/_data/docs/ai/ncgo-dev-rules.en.md` / `.zh-CN.md`

~15 行精简规则，替代 .cursor 整篇 design-doc：

- 不要手改生成文件（改模板/生成器）
- 遵守层边界（handler → usecase → repository）
- sqlc 先于 go mod tidy（Kitex 总是；Hertz 仅 WithDatabase）
- 用 ncgo 锚点插入方法，不要手写
- 改动后跑 `ncgo ai sync`
- 前导说明：design-doc 里的路径是模板内部路径（如 `kitex/kitex-template/main.yaml`），生成项目实际路径见 `docs/ncgo/<profile>/design-doc.*.md`
- 指向 design-doc 完整文档链接

**注册**：`internal/assets/assets.go` embed wiring 需要包含 `docs/ai/`（现有 embed 已覆盖 `docs/` 全目录则无需改）。

### 3.2 `internal/ai` 渲染层改动

**`render.go`:**

1. `target` 结构体增加 `Group string` 字段（agents | claude | cursor），`targets()` 返回 5 个目标：

| Group | RelPath | Render |
|-------|---------|--------|
| `agents` | `AGENTS.md` | renderAgents |
| `claude` | `CLAUDE.md` | renderClaude |
| `claude` | `.claude/skills/ncgo-dev/SKILL.md` | renderNcgoDevSkill（新） |
| `claude` | `.claude/generated/project-context.md` | renderProjectContext |
| `cursor` | `.cursor/rules/ncgo.mdc` | renderCursorMDC |

2. `renderInputs` 增加 `WorkflowBody`、`RulesBody` 字段。
3. `buildInputs` 读取两个新资产（按 `opts.Lang`）。
4. `renderAgents` / `renderClaude`：LongBody 后 append `WorkflowBody`。
5. `renderCursorMDC`：改用 `RulesBody`，不再用 LongBody（design-doc 全文）。
6. `renderNcgoDevSkill`（新）：frontmatter（`name: ncgo-dev` + `description`）+ ManagedMarker + WorkflowBody。

**`sync.go`:**

1. `Options` 增加 `Target string` 字段。
2. `Sync()`：`Target` 空值解析为 `claude`；过滤逻辑按 `target.Group` 匹配：

```go
// Target == "all" → 不过滤
// 否则只渲染 Group == opts.Target 的目标
```

3. `Result` 增加 `Target string` 字段（可选，便于结构化输出回显）。

**Managed marker 注意**：`renderNcgoDevSkill` 的 frontmatter 在前、`<!-- ncgo:managed -->` 在 frontmatter 之后、正文之前。`isManaged` 检查前 6 个非空行，frontmatter（4 行）+ marker（1 行）满足要求。

### 3.3 `ncgo add method` 命令契约改造

**`internal/scaffold/method/method.go`:**

```go
type Result struct {
    Path      string
    Domain    string
    Method    string
    NextSteps []string // 新增
}
```

`Add()` 填充 NextSteps：

```go
nextSteps := []string{
    "go build ./...",
    "replace the generated stub body with domain logic",
    "ncgo ai sync --root .",
}
```

**`internal/cli/add.go` — `runAddMethod`:**

- 增加 `--output json` flag（与 add domain 契约对齐的最小集）
- `--output json` 输出：`{path, domain, method, nextSteps}`
- text 输出保留现有格式 + next steps 列表

> 范围说明：add method 不引入 `--dry-run` / `--plan`——它是往已有 Go 文件插入 stub 的写操作，dry-run 的价值有限，且 AC 只要求 `NextSteps` + `--output json`。保持变更最小。

**`internal/mcp/tool_add.go` — `callAddMethod`:**

- 增加 `Output` 参数（text/json）
- JSON 模式返回结构化顶层字段：`path` / `domain` / `method` / `nextSteps`
- text 模式保持 `content[0].text` 现有格式

### 3.4 next-steps 引导 `ncgo ai sync`

**`internal/scaffold/mono/files.go` — `postGenerateNextSteps`:**
- 末尾追加 `ncgo ai sync --target all --root .`（new 场景需要全量上下文）

**`internal/scaffold/rpc` / `internal/scaffold/bff` 的 next-steps:**
- 末尾追加 `ncgo ai sync --root <service_dir>`（service 级上下文）

**`internal/scaffold/domain/domain.go` — `nextSteps`:**
- 末尾追加 `ncgo ai sync --root .`

这样闭环：new → add domain → add method → sqlc → 验证 → ai sync，每个命令的 next-steps 都引导到下一步。

> **为什么 new 用 `--target all`，add 系列用默认（claude）**：`ncgo new` 生成全新项目，AGENTS.md/.cursor 都是首次创建，需要全量上下文；`add domain`/`add method`/`add rpc`/`add bff` 发生在已有上下文的项目里，默认 claude 组刷新就够——agent 运行在哪个环境就刷新哪组。

### 3.5 MCP `ncgo_ai_sync` 改动

- InputSchema 增加 `enumField("target", ["all","agents","claude","cursor"])`，默认 `claude`
- 行为与 CLI 一致

## 4. 向后兼容说明

- `ai sync` 默认行为从「生成全部 4 文件」改为「生成 claude 组 3 文件」（CLAUDE.md + SKILL.md + project-context.md）。
- 现有用户若依赖默认生成 AGENTS.md / .mdc，需显式 `ncgo ai sync --target all`。
- **有意为之的契约变更**，需在 README/docs/examples 中明确标注迁移指引。
- `ncgo new` 的 next-steps 用 `--target all` 保证新项目拿到完整上下文，缓解兼容影响。

## 5. 测试计划

| 层 | 测试 | 覆盖 |
|----|------|------|
| 单元 | `internal/ai/sync_test.go` | target 过滤：`--target claude` 只生成 claude 组，`all` 生成全部 |
| 单元 | `internal/ai/render_test.go` | renderAgents/Claude append 工作流；renderCursorMDC 输出精简规则；renderNcgoDevSkill frontmatter+marker |
| 集成 | `internal/cli/ai_test.go` | `ai sync --target <name>` CLI 行为 |
| 集成 | `internal/mcp/server_test.go` | `ncgo_ai_sync` target 参数 schema + 行为 |
| 单元 | `internal/scaffold/method/method_test.go` | NextSteps 填充 |
| 集成 | `internal/cli/add_test.go` | `add method --output json` 输出 shape |
| 集成 | `internal/scaffold/mono/mono_test.go` | postGenerateNextSteps 含 `ncgo ai sync` → 更新 golden |

## 6. 文档计划

- `README.md` / `README.zh-CN.md`：`ai sync --target`、`add method --output json`
- `docs/examples.md` / `.zh-CN.md`：工作流示例更新
- 本设计文档（本期产出）

## 7. Phase 2 设计草案（本期不实现）

### 7.1 `ncgo_ai_context` MCP 工具

- 用 `go/parser` 扫描真实代码，返回结构化上下文：
  - `domains`：manifest 中的 domain 列表 + 实际文件存在性
  - `methods`：每个 domain 下实际存在的方法（解析 usecase）
  - `anchors`：`// ncgo:methods:start|end` 锚点完整性
  - `consistency`：manifest 声明的 domain/方法 与实际代码的差异
- 输出格式：`content[0].text` 可读 + 顶层结构化字段（与现有 MCP 双输出约定一致）
- InputSchema：`root`（必填）

### 7.2 `ncgo check` 命令

- 校验 agent 改动，任一失败返回**非零退出码**：
  - 锚点完整：每个 usecase 文件存在 `// ncgo:methods:start|end` 且配对
  - manifest 一致性：`manifest.Domains` 与实际 `internal/usecase/*/` 目录一致
  - 上下文过期：AGENTS.md/CLAUDE.md 是否落后于 manifest（generated_at 比对）
- 输出：结构化报告（text/json），类似 `ncgo doctor`
- 与 `ncgo_ai_context` 共享扫描逻辑（同一 `internal/ai/scan` 包）

### 7.3 开放问题（Phase 2 开工前确认）

- `ncgo check` 的退出码约定（0 通过 / 1 校验失败 / 2 命令错误）
- `--target` 是否需要在 `ncgo check` 也支持
- `ncgo_ai_context` 是否需要缓存

## 8. 非目标

- 不实现 Phase 2 代码
- 不改设计文档的架构内容（只在 ai 层 append 工作流，不重写 design-doc）
- 不动 hz/kitex 模板本身
