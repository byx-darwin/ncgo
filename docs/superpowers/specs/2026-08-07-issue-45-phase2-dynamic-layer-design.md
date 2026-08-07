# Issue #45 Phase 2 — 动态层：`ncgo_ai_context` + `ncgo check`

**日期**: 2026-08-07
**范围**: 实现 Issue #45 Phase 2（动态层）：新增 MCP 工具 `ncgo_ai_context` 和 CLI 命令 `ncgo check`，共享同一 `internal/scan` 扫描包
**前置**: Phase 1 已合并（PR #47），静态工作流 + 命令契约已就绪。Phase 1 设计文档：`2026-08-07-issue-45-ai-agent-ncgo-dev-design.md` §7
**关联**: 新建 Issue（从本文 §7 复制需求），工作流合同 `wf-2026-08-07-003`

---

## 1. 背景与目标

Phase 1 给了 agent「常识与工作流」（静态层）+「命令契约层」，但 agent 落地功能后缺少对「当前状态」的感知：它不知道哪些 domain 真实存在、usecase 里有哪些方法、锚点是否完整、manifest 声明的和实际代码是否漂移。

Phase 2 目标：让 agent（经 MCP）随时读取真实代码状态（`ncgo_ai_context`），并在改动后校验自身产出是否合法（`ncgo check`，失败返回非零退出码）。

## 2. 设计决策

| # | 决策 | 结论 | 理由 |
|---|------|------|------|
| D1 | 扫描逻辑归属 | 独立 `internal/scan` 包 | 单一扫描逻辑，MCP 与 check 两处消费；不污染 ai 渲染层；符合 Phase 1 设计文档 §7.2「同一 scan 包」约定 |
| D2 | 遍历工具复用 | scan 包自建轻量 go/parser 遍历 | doctor 的 walk 工具是包私有且服务特定规则，提升为共享包收益小；scan 按需自建 ~40 行 |
| D3 | 上下文过期判定 | C1：`ai sync` 渲染文件注入 `<!-- ncgo:generated-at -->` 时间戳标记 | 显式、可靠、可测试；代价是 ai sync 输出为契约变更（需同步 golden tests + docs） |
| D4 | `ncgo check` 输出结构 | 复用 `doctor.Report` / `Check` / `Summary` / `WriteJSON` / `WriteText` | 代码少、输出格式与 doctor 完全一致；check 是校验语义，不并入 doctor 命令本身 |
| D5 | 退出码 | 0 通过 / 1 校验失败 / 2 命令错误 | CI 可区分「校验不过」和「用法错误」；对齐 doctor 的 errSilentFailure + cobra error 模式 |
| D6 | `--target` 过滤 | `ncgo check` 不支持 | 校验所有上下文文件，agent 改动后一次全量校验即可；避免增加选项复杂度 |
| D7 | 缓存 | `ncgo_ai_context` 不缓存 | go/parser 扫描 usecase 毫秒级；缓存引入失效判断复杂度 |
| D8 | `check.context.stale` 严重度 | error | 符合 §7.2「任一失败返回非零」；过期即阻塞 |
| D9 | 服务级范围 | `ncgo_ai_context` / `ncgo check` 均为服务级（mono） | workspace 根无单一 manifest；传 workspace 根 → 命令错误 |

## 3. 详细设计

### 3.1 新增 `internal/scan` 包

纯函数式扫描，不依赖 MCP/CLI。类型带 JSON tag，MCP 结构化字段直接复用。

```go
// scan.go
type Domain struct {
    Name           string   `json:"name"`
    ManifestListed bool     `json:"manifestListed"`
    UsecaseExists  bool     `json:"usecaseExists"`
    RepoExists     bool     `json:"repoExists"`
    Methods        []Method `json:"methods"`
    AnchorsOK      bool     `json:"anchorsOk"`
}

type Method struct {
    Name string `json:"name"`
    File string `json:"file,omitempty"`
    Line int    `json:"line,omitempty"`
}

type Issue struct {
    Kind    string `json:"kind"`   // missing_usecase | undeclared_domain | anchor_missing | anchor_unpaired | scan_error
    Message string `json:"message"`
    File    string `json:"file,omitempty"`
}

type Scan struct {
    Root    string   `json:"root"`
    Domains []Domain `json:"domains"`
    Issues  []Issue  `json:"issues"`
}

func Scan(root string) (*Scan, error)
```

**扫描流程：**

1. `manifest.Load(root)` — 失败返回错误（命令错误 / MCP error）。
2. 遍历 `internal/usecase/*/`，对每个目录：
   - 读 `<domain>/<domain>.go`（usecase 文件）。
   - go/parser 解析，收集 `func (u *UseCase) X(...)` 方法（排除 `New`、`Repo` 等构造函数/访问器）。
   - 字符串搜索 `// ncgo:methods:start` / `// ncgo:methods:end`，判定 `AnchorsOK`（都存在且 start 在 end 前）。
3. 构建 `Domains`：
   - 每个 manifest 声明的 domain → `DomainInfo{ManifestListed: true}`；目录存在则补 `UsecaseExists`/`Methods`/`AnchorsOK`。
   - 目录存在但未在 manifest 声明 → `Issue{Kind: undeclared_domain}`。
   - manifest 声明但目录缺失 → `Issue{Kind: missing_usecase}`。
   - 锚点不完整 → `Issue{Kind: anchor_missing|anchor_unpaired}`。
4. 返回 `Scan`。

**walk.go（自建）**：`walkGoFiles(dir, visit)` 复用 doctor 的遍历模式（`filepath.WalkDir` + 跳过 testdata/_/dot 目录 + `parser.ParseFile`，解析错误跳过）。仅 scan 包内部使用。

### 3.2 MCP `ncgo_ai_context`

**`internal/mcp/tools.go`** — 注册：

```go
{Name: "ncgo_ai_context", Description: "Scan real code and return structured context (domains/methods/anchors/consistency) for an ncgo service.", InputSchema: schemaObject([]string{"root"}, rootField("Service root containing .ncgo/manifest.yaml"), outputTextJSONField())},
```

**`internal/mcp/tool_ai_context.go`** — 新增：

- `callAIContext(raw)`：解析 `root` + `output`（text/json）。
- 调用 `scan.Scan(root)`：
  - 成功 → 双输出：`content[0].text` 可读摘要 + 顶层 `domains` / `methods` / `anchors` / `issues` 结构化字段。
  - `manifest.Load` 失败 → `textResult(err, true)`（isError）。
- 结构化字段格式：

```go
map[string]any{
    "root":    scan.Root,
    "domains": scan.Domains,
    "methods": flattenMethods(scan),   // [{domain, name, file, line}]
    "anchors": anchorSummaries(scan),  // [{domain, ok, file}]
    "issues":  scan.Issues,
}
```

- `content[0].text`：人类可读摘要（domain 列表、每 domain 方法数、锚点状态、issues 列表）。

### 3.3 CLI `ncgo check`

**`internal/cli/check.go`** — 新增：

```go
type checkOptions struct {
    root   string
    output string // text | json
}

func newCheckCmd() *cobra.Command
```

注册进 `root.go`：`cmd.AddCommand(newCheckCmd())`。

**Checks（复用 doctor.Report / Check / Summary）：**

| ID | Severity | 判定 |
|----|----------|------|
| `check.anchor` | error | 每个 usecase 文件 `start`/`end` 配对（缺失或顺序颠倒 → 失败） |
| `check.manifest.consistency` | error | manifest.Domains ↔ `internal/usecase/*/` 目录一致（缺失目录 / 未声明目录 → 失败） |
| `check.context.stale` | error | AGENTS.md / CLAUDE.md / .claude/skills/ncgo-dev/SKILL.md / .claude/generated/project-context.md 任一存在文件，其 `ncgo:generated-at` 标记 < manifest.GeneratedAt → 失败 |

**执行流程：**

1. `manifest.Load(opts.root)` — 失败 → 命令错误（exit 2），不打印 Report。
2. `scan.Scan(root)` 拿 Domains/Issues。
3. 组装三个 Check：
   - anchor / consistency 由 scan 的 Issues 派生。
   - context.stale 由 readGeneratedAt(各上下文文件) vs `manifest.GeneratedAt` 判定。
4. `doctor.WriteJSON` / `WriteText` 输出。
5. `rep.OK()` 为 false → exit 1；否则 exit 0。

**退出码机制：**

`internal/cli/root.go` 的 `Main()` 增加对 `exitCodeError` 的识别（~6 行改动）：

```go
if err := root.Execute(); err != nil {
    var ec *exitCodeError
    if errors.As(err, &ec) {
        if ec.msg != "" {
            fmt.Fprintln(os.Stderr, ec.msg)
        }
        os.Exit(ec.code)
    }
    if _, silent := err.(silentErr); !silent {
        fmt.Fprintln(os.Stderr, err)
    }
    os.Exit(1)
}
```

`exitCodeError` 定义在 `cli` 包：

```go
type exitCodeError struct {
    code int
    msg  string
}

func (e *exitCodeError) Error() string { return e.msg }
```

- `check` 命令：命令错误 → `&exitCodeError{code: 2, msg: ...}`；校验失败 → `&exitCodeError{code: 1}`（Report 已打印，msg 为空避免重复输出）。

### 3.4 `ai sync` 契约变更（C1 时间戳标记）

**`internal/ai/sync.go` — `writeTarget`：**

- 渲染完成后、写入前，在 `ManagedMarker` 行之后插入 `<!-- ncgo:generated-at: <RFC3339> -->`。
- 用 `time.Now().UTC().Format(time.RFC3339)`。
- `renderInputs` / render 函数签名不变；只改动最终落盘内容。

**影响面：**

- `internal/scaffold/mono/golden_test.go` + `testdata/` 快照（ai sync 输出新增一行）。
- `internal/ai/render_test.go`、`sync_test.go` 相关断言。
- README/docs 注明 `ai sync` 输出新增 `generated-at` 标记。

**readGeneratedAt（check 侧）：** 读目标文件，提取 `ncgo:generated-at:` 后的 RFC3339 时间；无标记 → 视为过期（stale）。

## 4. 向后兼容说明

- `ncgo check` 与 `ncgo_ai_context` 均为新增，不影响现有命令。
- `ai sync` 渲染输出新增一行 HTML 注释 `<!-- ncgo:generated-at: ... -->` —— 对读取方（agent、CLI、MCP）无破坏，但 golden tests 需更新。
- 无其它 CLI/MCP 契约变更。

## 5. 测试计划

| 层 | 测试 | 覆盖 |
|----|------|------|
| 单元 | `internal/scan/scan_test.go` | 完整项目、缺方法、锚点缺失/颠倒、目录漂移（missing/undeclared）、workspace root 报错 |
| 单元 | `internal/ai/sync_test.go` | generated-at 标记写入 |
| 单元 | `internal/ai/render_test.go` | 渲染输出含标记行 |
| 集成 | `internal/cli/check_test.go` | 退出码 0/1/2、json 输出、text 输出 |
| 集成 | `internal/mcp/server_test.go` | `ncgo_ai_context` schema（root 必填 + output）、call 返回结构化字段 |
| 集成 | `internal/scaffold/mono/mono_test.go` | ai sync golden 更新（generated-at 行） |

## 6. 文档计划

- `README.md` / `README.zh-CN.md`：`ncgo check` 命令 + `ncgo_ai_context` MCP 工具。
- `docs/examples.md` / `.zh-CN.md`：agent 工作流示例补充（改完 → `ncgo check` → `ai sync`）。
- 本设计文档（本期产出）。

## 7. 非目标

- 不改 hz/kitex 模板本身。
- 不支持 `ncgo check --target`。
- 不做扫描缓存。
- 不把 `ncgo check` 并入 `ncgo doctor`（语义、退出码均不同）。
