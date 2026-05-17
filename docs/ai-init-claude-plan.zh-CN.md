# `ncgo ai init claude` 最小化方案

本文档定义了 `ncgo ai init claude` 命令的最小化引导方案。

该命令的目的与 `ncgo ai sync` 不同：

- `ai init claude` 引导 hand-authored（手写）的 Claude 启动文件
- `ai sync` 刷新确定性生成的项目事实

## 1. 目标

为需要初始 `.claude/` 布局的仓库添加一个显式的引导命令，而不会将这些启动文件视为自动生成的内容。

提议的命令形式：

- `ncgo ai init claude --root .`

## 2. 为什么需要独立命令

`ncgo ai sync` 的设计是保守的。它只应生成那些确定的、从仓库状态派生的、可以安全覆盖的文件。

`.claude/rules/*` 下的启动文件则不同：

- 它们定义仓库策略和工作流
- 预期在引导后会被编辑
- 不能仅从 `.ncgo/manifest.yaml` 推导出来

因此它们更适合通过显式的 scaffold/init 命令创建，而非由 `ai sync` 生成。

## 3. 最小范围

### 在范围内

- 一个新的显式命令：`ncgo ai init claude`
- 引导最小化的手写 `.claude/` 启动文件集
- 可选创建安全的本地所有权标记文件，如 `.claude/local/.gitignore`
- 默认跳过已存在的文件
- 支持 `--dry-run` 和 `--force`

### 不在范围内

- 生成 `.claude/generated/*`（由 `ai sync` 处理）
- 默认生成 `.claude/skills/*`、`.claude/agents/*` 或 `.claude/commands/*` 下的工作流启动文件
- 默认生成 `.claude/hooks/*` 下的活跃 hook 内容
- 生成用户手写的本地笔记内容（`.claude/local/*`）
- v1 中除单一默认语言外的模板本地化

## 4. 启动文件集

### 4.1 默认最小集

首个版本应在文件缺失时创建以下文件：

- `.claude/README.md`
- `.claude/rules/agent-engineering.md`
- `.claude/rules/go.md`
- `.claude/local/.gitignore`（推荐的安全默认值）

理由：

- `.claude/README.md` 解释目录布局、所有权和优先级
- `agent-engineering.md` 定义 agent 在仓库中应如何工作
- `go.md` 定义针对此代码库风格的 Go 编码约束
- `.claude/local/.gitignore` 让 `local/` 区域的用户所有权更加明确，而不生成个人内容

默认最小集 **不应** 在以下目录创建工作流启动文件：

- `.claude/skills/*`
- `.claude/agents/*`
- `.claude/commands/*`

### 4.2 推荐的 `team` 预设（可选/按需启用）

如果命令支持 `--preset team` 这样的显式预设，推荐的启动文件集为：

- `.claude/skills/plan-change.md`
- `.claude/skills/run-validation.md`
- `.claude/skills/doc-sync.md`
- `.claude/skills/write-tests.md`
- `.claude/agents/planner.md`
- `.claude/agents/implementer.md`
- `.claude/agents/reviewer.md`
- `.claude/agents/debugger.md`
- `.claude/agents/doc-writer.md`
- `.claude/commands/plan.md`
- `.claude/commands/implement-change.md`
- `.claude/commands/fix-failing-test.md`
- `.claude/commands/update-docs.md`
- `.claude/commands/review-diff.md`
- `.claude/hooks/README.md`

理由：

- `skills/*` 可提供可复用的工作流模板，而不假装它们是生成的事实
- 启动工作流文件应保持项目通用性，依赖 `.claude/generated/project-context.md`、`.ncgo/manifest.yaml` 和 `ncgo.workspace` 获取项目特定事实
- `agents/*` 可提供角色模板，团队可在引导后编辑
- `commands/*` 可提供用于规划、审查和验证的可复用提示入口
- 命令集应覆盖主要协作链：`planner -> implementer -> reviewer`，以及 debugger/doc-writer 的辅助路径
- `.claude/hooks/README.md` 保守地记录 hooks，不默认启用任何行为

`.claude/agents/*` 下的 agent 启动文件应使用 Claude Code 自定义子 agent 格式，以便引导后可直接调度。每个文件应以 YAML frontmatter 开头，至少包含：

- `name`
- `description`
- `tools`

`description` 应使用 `Use when ...` 这样的任务语言编写，以便 Claude Code 可以发现该角色。这适用于当前的 `implementer` / `reviewer` 启动角色，以及 `team` 预设中未来可能添加的任何可选角色。

启动工作流文件应保持项目通用性，但 `ai init claude` 可以检测目标根目录看起来像 mono 服务（`.ncgo/manifest.yaml`）还是 micro 工作区（`ncgo.workspace`），并在 `.claude/README.md` 中渲染少量形状特定的指导。

### 4.3 默认不应引导的内容

该命令 **不应** 创建个人化或行为化的启动内容，例如：

- `.claude/local/notes.md`
- `.claude/local/prompt.md`
- `.claude/hooks/*` 下的活跃 hook 脚本或 hook 配置

这些内容要么过于工作流特定，要么带有副作用，要么明确属于用户所有。

## 5. 命令语义

建议的标志：

- `--root`：项目根目录，默认 `.`
- `--dry-run`：报告预期写入，不修改文件
- `--force`：显式覆盖已有的启动文件
- `--preset`：`minimal|team`（默认 `minimal`）

建议行为：

1. 文件不存在时创建默认的最小启动文件
2. 默认跳过已存在的文件
3. 仅在传入 `--force` 时才覆盖
4. 以与 `ai sync` 相同的风格报告 `wrote ...` / `skipped ...`

`team` 预设应为 `skills/*`、`agents/*` 和 `commands/*` 添加工作流启动文件，同时仍避免用户所有的本地内容和活跃 hooks。

最小命令不需要位置参数。

## 6. 所有权和覆盖模型

由 `ai init claude` 创建的文件在引导后应成为仓库所有。

这意味着：

- 它们 **不应** 携带 `<!-- ncgo:managed -->` 标记
- 它们应像普通源文件一样被审查和编辑
- 后续的 `ai sync` 运行 **不得** 覆盖它们

建议的覆盖模型：

- 文件缺失：写入
- 文件已存在：默认跳过
- 文件已存在 + `--force`：显式覆盖

这使命令保持引导导向，而非同步导向。

## 7. 与 `ai sync` 的关系

需要 Claude 支持的仓库的建议流程：

1. 运行一次 `ncgo ai init claude` 获取启动 `.claude` 文档
2. 反复运行 `ncgo ai sync` 刷新生成的事实

建议的责任划分：

- `ai init claude`：启动策略/布局文档
- `ai sync`：`.claude/generated/project-context.md` 及其他未来生成的事实

`ai sync` 应保持累加性，**不得** 被升级为重写启动策略文件的命令。

## 8. 实现形状

建议的命令树添加：

- `ncgo ai init claude`

理由：`ai init` 为未来的显式引导留下空间，而不会与 `ai sync` 耦合。

建议的实现形状：

- 在 `internal/cli/ai.go` 下添加 `newAIInitCmd()`
- 添加一个独立于 `Sync` 的 `internal/ai` 引导辅助函数
- 将启动模板存储为嵌入式资源，而非硬编码字符串

可能的资源布局：

- `internal/assets/_data/claude/README.md`
- `internal/assets/_data/claude/rules/agent-engineering.md`
- `internal/assets/_data/claude/rules/go.md`
- `internal/assets/_data/claude/local/.gitignore`

`team` 预设的资源还可包括：

- `internal/assets/_data/claude/skills/plan-change.md`
- `internal/assets/_data/claude/skills/run-validation.md`
- `internal/assets/_data/claude/skills/doc-sync.md`
- `internal/assets/_data/claude/skills/write-tests.md`
- `internal/assets/_data/claude/agents/planner.md`
- `internal/assets/_data/claude/agents/implementer.md`
- `internal/assets/_data/claude/agents/reviewer.md`
- `internal/assets/_data/claude/agents/debugger.md`
- `internal/assets/_data/claude/agents/doc-writer.md`
- `internal/assets/_data/claude/commands/plan.md`
- `internal/assets/_data/claude/commands/implement-change.md`
- `internal/assets/_data/claude/commands/fix-failing-test.md`
- `internal/assets/_data/claude/commands/update-docs.md`
- `internal/assets/_data/claude/commands/review-diff.md`
- `internal/assets/_data/claude/hooks/README.md`

`internal/assets/_data/claude/agents/*` 下的任何嵌入式启动资源都应已包含所需的 YAML frontmatter，这样 `ncgo ai init claude --preset team` 就能生成无需手动编辑即可与 Claude Code 兼容的子 agent。

## 9. 验证方案

首个实现应包含针对以下场景的聚焦测试：

- 写入所有缺失的启动文件
- 默认跳过已存在的文件
- 仅在 `--force` 时覆盖
- `--dry-run` 报告但不写入
- 与 `ai sync` 共存

重要的兼容性检查：

- `ai init claude` 之后，后续的 `ai sync` 运行仍应写入 `.claude/generated/project-context.md`，而不会触碰启动文件

## 10. 延后的后续工作

在最小命令稳定后，后续阶段可以考虑：

- 除 `implementer` 和 `reviewer` 之外的额外 agent 角色的启动模板
- 可选的 zh-CN 启动变体
- 针对 hooks 的额外可选文档，但不默认启用 hook 行为

这些应保持为可选的脚手架功能，不属于 `ai sync` 的一部分。

## 11. 将 `cc-skills-golang` 映射到启动文件

`team` 预设现在有足够的覆盖面积（`.claude/rules/*`、`.claude/skills/*`、`.claude/agents/*`、`.claude/commands/*`），因此我们应该明确它与 `cc-skills-golang` 等外部 Go 技能库的关系。

本节定义预期的集成模型。

### 11.1 目标

目标 **不是** 将完整的外部 `SKILL.md` 文件复制到生成的仓库中。

目标是：

- 保持启动文件轻量且仓库通用
- 将一组稳定的、高频的 Go 规则提炼为仓库所有文档
- 强化 `team` 预设的工作流文件和子 agent
- 将深度、特定栈的知识保留为外部、按需获取的技能

换句话说：

- `ai init claude` 应生成启动策略和工作流文件
- `ai sync` 应保持生成确定性的项目事实
- 外部 Go 技能应保留为深度知识层，而非默认启动内容

### 11.2 选择标准

一个 Go 技能只有在以下情况下才适合提炼为启动文件：

- 在大多数生成的 Go 服务仓库中通用
- 足够稳定，可以成为仓库策略或工作流指导
- 对多个 agent（`implementer`、`reviewer`、`debugger`）有用
- 轻量到可以缩减为几条要点或一个小清单
- 不与特定框架或可选库紧密耦合

如果某个技能严重依赖特定的栈选择，它应在默认情况下保持外部化，仅在生成的仓库实际使用该栈时才被引用。

### 11.3 默认值得提炼的技能

最佳的默认候选是广泛的 Go 服务技能：

- `golang-code-style`
- `golang-naming`
- `golang-error-handling`
- `golang-testing`
- `golang-safety`
- `golang-project-layout`
- `golang-context`
- `golang-documentation`

这些是仓库所有启动文件的合适基础，因为它们可以干净地映射到：

- 编码规则
- 测试工作流
- 审查问题
- 调试纪律
- 文档同步预期

### 11.4 更适合应用于 Reviewer / Debugger 的技能

有些技能有价值，但应强化特定角色，而非全局 Go 规则文件：

- `golang-database`
- `golang-security`
- `golang-observability`
- `golang-troubleshooting`

这些更好地表达为：

- reviewer 检查清单
- debugger 工作流步骤
- 定向验证提示

而非始终开启的仓库策略。

### 11.5 可选的、特定栈的技能

以下技能可以作为可选扩展，但 **应仅** 在生成的仓库明确使用相关栈时才提炼：

- `golang-swagger`
- `golang-concurrency`
- `golang-dependency-injection`
- `golang-samber-do`
- `golang-stretchr-testify`

示例：

- 如果生成的 HTTP/BFF 仓库拥有 Swagger/OpenAPI 文档，提炼少量 `golang-swagger` 指导是合理的
- 如果生成的仓库标准化使用 `samber/do`，提炼少量 `golang-samber-do` 指导是合理的
- 如果仓库大量使用 `testify`，提炼少量 `golang-stretchr-testify` 指导是合理的

这些应保持条件性。它们不适合作为每个生成仓库的全局默认值。

### 11.6 不应作为默认启动内容的技能

以下技能通常过于特定于某个栈，或者更适合 `ncgo` 自身而非所有生成的仓库：

- `golang-cli`
- `golang-spf13-cobra`
- `golang-spf13-viper`
- `golang-google-wire`
- `golang-uber-dig`
- `golang-uber-fx`
- `golang-graphql`
- `golang-grpc`
- `golang-samber-lo`
- `golang-samber-mo`
- `golang-samber-oops`
- `golang-samber-ro`
- `golang-samber-slog`

原因包括：

- 该技能主要帮助 `ncgo` 自身的 CLI 代码库，而非生成的服务
- 该技能假设了特定的库或应用框架
- 该技能更适合按需触发，而非嵌入仓库策略

### 11.7 `--preset team` 的文件级映射

建议的提炼映射表：

| 启动文件 | 最佳外部技能输入 | 预期结果 |
| --- | --- | --- |
| `.claude/rules/go.md` | `code-style`, `naming`, `error-handling`, `safety`, `project-layout`, `context`, `documentation` | 小型的、始终开启的 Go 仓库规则 |
| `.claude/skills/write-tests.md` | `testing`，可选 `stretchr-testify` | 测试范围、golden 处理、断言纪律 |
| `.claude/skills/run-validation.md` | `testing`, `troubleshooting` | 验证顺序和反馈循环纪律 |
| `.claude/skills/doc-sync.md` | `documentation`，可选 `swagger` | 面向用户的文档同步和工作示例更新 |
| `.claude/skills/plan-change.md` | `project-layout`, `testing`, `documentation` | 影响面规划和测试/文档意识 |
| `.claude/agents/implementer.md` | `code-style`, `error-handling`, `context`, `project-layout` | 最小安全实现的实现指导 |
| `.claude/agents/reviewer.md` | `database`, `security`, `observability`, `error-handling`, `context`, `project-layout` | 更强的审查检查清单 |
| `.claude/agents/debugger.md` | `troubleshooting`, `testing`, `context`，可选 `concurrency` | 优先复现的调试工作流 |
| `.claude/agents/doc-writer.md` | `documentation`，可选 `swagger` | 文档所有权、配对和示例准确性 |
| `.claude/commands/*.md` | 无直接对应 | 保持精简；仅分发工作流 |

`commands/*` 应有意识地保持精简。它们不是复制 Go 规则的合适位置。它们应保持作为入口点的角色：选择一个 agent、命名工作流、定义预期输出形状。

### 11.8 这对 `minimal` 与 `team` 意味着什么

建议的划分：

- `minimal` 仅在 `.claude/rules/*` 下保留一个小而保守的规则层
- `team` 添加工作流模板和角色模板，然后将更多指导提炼到 `write-tests`、`run-validation`、`reviewer` 和 `debugger` 中

这意味着 `team` 的价值不只是"更多文件"。它是：

- 更好的规划提示
- 更强的测试编写纪律
- 更安全的审查检查清单
- 更可靠的调试步骤
- 更清晰的文档同步预期

### 11.9 启动文件应保持比完整技能更轻量

即使启动文件从外部 Go 技能中汲取灵感，它也 **应** 保持比上游技能短得多，且更具仓库特定性。

启动文件应回答这样的问题：

- 在 *这个* 仓库中 agent 必须做什么？
- 在这里编辑或审查之前应检查什么？
- 在这个生成的仓库形状中，哪些表面对契约敏感？

完整的外部技能仍然是以下内容的更合适位置：

- 更深层的库指导
- 扩展示例
- 长篇故障排除手册
- 基准和性能方法论
- 特定栈的实现细节

### 11.10 建议的首次发布

首次发布应保持保守且纯文本。

首先强化以下嵌入式资源：

- `internal/assets/_data/claude/rules/go.md`
- `internal/assets/_data/claude/agents/reviewer.md`
- `internal/assets/_data/claude/agents/debugger.md`

然后，如果结果有用且仍然轻量，再跟进：

- `internal/assets/_data/claude/skills/write-tests.md`
- `internal/assets/_data/claude/skills/run-validation.md`
- `internal/assets/_data/claude/skills/plan-change.md`

这使发布保持渐进式，避免在初始启动集中过载。
