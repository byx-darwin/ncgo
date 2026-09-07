# ai sync: 边界表格失真 + managed 文件全量覆盖丢失手工内容

- Issue: https://github.com/byx-darwin/ncgo/issues/114
- Date: 2026-09-07
- Scope: `internal/ai` (contract-sensitive: AI context 生成)

## 背景

`ncgo ai sync` 生成/更新 CLAUDE.md、AGENTS.md、Cursor rules 等托管文件。
下游项目在删除一个仍有手写代码依赖的 domain 后重新 sync，暴露出两个相关但
独立的问题：

1. 边界表格（`## Boundaries`）完全由 manifest 的 domain 列表驱动，不检查
   文件系统实际状态，导致文档与真实目录不一致。
2. workspace 级 sync 对已存在的 managed 文件（带 `<!-- ncgo:managed -->`
   marker）做整体重写，用户手工添加的自定义章节会被静默整体覆盖，没有
   任何保留机制或警告。

两个问题各自独立修复，不要求统一到同一个机制。

## Bug 1：边界表格并集渲染 + 标注来源

### 现状

`internal/ai/boundaries.go:14` 的 `EditBoundaries(source syncSource)` 只读
`source.Service.Domains` / `source.WorkspaceServices[].Domains`，逐个拼出
`internal/usecase/<d>/`、`internal/repository/<d>/` 两行，完全不检查这些
目录在文件系统里是否存在。manifest 删除一个 domain 后，即使目录仍然存在
（被手写代码引用），对应行也会从文档里消失。

### 设计

- **范围修正（计划阶段复核代码后发现）**：`EditBoundaries` 现有 workspace
  scope 分支实际上用的是 `svc.Name`（服务名）拼行，不是 `svc.Domains`——
  workspace 级表格本来就是"以整个服务目录为粒度"的粗粒度简化，`internal/repository/<domain>/`
  这种按 domain 细分的路径在 workspace 级根本不对应任何真实相对路径
  （真实路径是 `services/<svc>/internal/repository/<domain>/`，workspace
  级渲染并不知道这层前缀）。而 Issue #114 的复现步骤明确是在**单个服务
  目录下**跑 `ai sync`（`syncScopeService`）。因此本次并集渲染检查**只
  覆盖 service scope**，workspace scope 分支保持现状不变，避免引入范围
  外的行为变更。
- `EditBoundaries` 增加 `root string` 参数：
  - service scope（`source.Scope == syncScopeService`）→ 调用方
    `sync.go:171` 直接传入 `opts.Root`，用于并集渲染检查。
  - workspace scope → `root` 参数被忽略，逻辑与现状完全一致（仍用
    `svc.Name` 拼行，不做文件系统检查）。
- 渲染集合 = manifest domains ∪ 文件系统里实际存在的
  `internal/repository/<d>/` 和 `internal/usecase/<d>/` 子目录名集合。
- 对存在于文件系统但不在 manifest domains 里的行，`Reason` 字段追加
  标注，例如：`"Data access implementation (not in manifest; verify manual usage)"`。
- 纯只读的 `os.Stat`/`os.ReadDir`，无副作用；改动集中在
  `boundaries.go` 及其调用点（`sync.go:171`）。

### 测试

- `internal/ai/boundaries_test.go`：新增/扩展用例�covers
  - manifest 与文件系统一致（现状不变）
  - manifest 有、目录不存在（是否仍渲染？—— 维持现状渲染，纯本地新增
    "存在性"不影响 manifest 驱动的行，只做并集补充，不做删减，避免改变
    现有稳定输出）
  - manifest 没有、目录存在（新增行 + 标注 Reason）
- 使用临时目录 fixture 或 `t.TempDir()` 构造 `internal/repository/<d>/`
  等子目录。

## Bug 2：managed 文件多具名锚点合并

### 现状

`writeTarget`（`sync.go:487`）与 `writeStandaloneDocs`（`sync.go:531`）对
携带 `ManagedMarker` 的已存在文件直接整体覆盖写入，没有任何内容保留
机制。仓库里已有一个部分重叠的扩展点 `AGENTS.local.md`
（`LocalNotesFile`，渲染进 "Local Notes" 小节），但用户可能不知道该机制、
或想要在文档任意位置插入自定义章节而非固定的 "Local Notes" 位置。经确认，
本次仍按用户选择实现更灵活的锚点合并机制（方案 B），与 `AGENTS.local.md`
并存，不互相替代。

### 锚点语法

```
<!-- ncgo:custom:<name>:start -->
...用户自定义内容...
<!-- ncgo:custom:<name>:end -->
```

- `<name>` 必须匹配 `^[a-z][a-z0-9-]{0,62}$`（与仓库里 domain 命名校验
  风格一致）。
- 允许在文件中任意位置出现，允许多组不同 name 的锚点对。

### 新增组件

`internal/ai/anchors.go`：

```go
// mergeCustomAnchors 从 oldContent 里提取所有格式正确的具名锚点，
// 合并进 rendered（新渲染内容）末尾的 "## Custom Notes" 小节。
// 若检测到格式错误的锚点（缺失 end、name 不合法、重复 name），
// 返回 malformed 非空，调用方据此走"拒绝覆盖，需要 --force"路径，
// 不做静默丢弃。
func mergeCustomAnchors(oldContent, rendered []byte) (merged string, malformed []string)
```

### 数据流（写入路径改动点）

在 `writeTarget` / `writeStandaloneDocs` 现有的
"读取 existing → 检查 isManaged" 之后、`stampGeneratedAt` 之前插入一步：

1. `existing` 非空（旧文件存在）→ 调用
   `mergeCustomAnchors(existing, []byte(rendered))`。
2. `malformed` 非空 → 复用现有"无 marker 拒绝覆盖"的 skip 路径：
   `res.Skipped = append(..., Skip{Path: ..., Reason: "malformed ncgo:custom anchor <name>: <detail>; fix markers or pass --force"})`，
   `--force` 时放弃保留、按旧行为整体覆盖（不合并）。
3. `malformed` 为空 → 用 `merged` 替换 `rendered`，继续走原有
   `stampGeneratedAt` → 写盘流程。
4. 若渲染内容里没有任何被保留的锚点（旧文件本没有自定义锚点，或本来就
   是新文件），不生成空的 `## Custom Notes` 标题——只有存在至少一个待
   保留的合法锚点时才追加该小节。

### 适用范围

统一应用于所有携带 `ManagedMarker` 的 sync 目标：`writeTarget` 覆盖的
CLAUDE.md/AGENTS.md/Cursor rules，以及 `writeStandaloneDocs` 的独立设计
文档。单一合并点（`mergeCustomAnchors`），不按具体 target 特殊处理。

### 已知一次性迁移成本

启用本功能前就已存在、且没有用锚点包裹的自定义内容（例如本 issue 描述的
那种直接手工插入的章节），在第一次带有本功能的 sync 时仍会按旧行为被
覆盖一次——无法在没有锚点标记的前提下回溯识别"哪些旧内容是自定义的"。
这一点会在 CHANGELOG 与 `ncgo ai sync` 相关文档（README / docs/examples）
里明确提示：升级后需要把想保留的自定义内容手动包进
`<!-- ncgo:custom:<name>:start/end -->` 再执行一次 sync。

### 测试

- `internal/ai/anchors_test.go`：`mergeCustomAnchors` 单元测试
  - 无锚点（返回原样 rendered，无 Custom Notes 小节）
  - 单个合法具名锚点
  - 多个不同 name 的合法锚点
  - 格式错误：缺 end / name 不合法 / 重复 name（各自返回 malformed）
- `internal/ai/sync_test.go` 或现有 golden 测试扩展：端到端验证
  - 旧文件带合法锚点 → 新文件保留对应内容
  - 旧文件带格式错误锚点 → 不带 `--force` 时 skip，带 `--force` 时覆盖

## 文档同步

- README.md / README.zh-CN.md、docs/examples.md / docs/examples.zh-CN.md：
  补充 `<!-- ncgo:custom:<name>:start/end -->` 锚点用法说明，以及与
  `AGENTS.local.md` 的关系（两者并存，锚点适合"文档任意位置插入"，
  `AGENTS.local.md` 适合"固定 Local Notes 小节"）。
- CHANGELOG：记录本次修复及一次性迁移成本提醒。

## 风险与保守性

- Bug1 改动是纯只读文件系统检查 + 渲染集合扩大，不改变已有 manifest
  驱动行的语义（不删减，只并集补充），风险低。
- Bug2 改动集中在 `internal/ai` 一个新文件 + 两个调用点的一步插入，
  复用现有 "skip + --force" 保护语义而非引入新的失败模式，符合仓库对
  contract-sensitive 表面保守修改的原则。
- 两个 bug 修复相互独立，可分别提交、分别验证，不强行合并成一个大改动
  （但同属一个 Issue，可在同一个 PR 交付）。
