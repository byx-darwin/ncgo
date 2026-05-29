# ai sync → `.claude/generated/project-context.md` 方案

本文档定义 `ncgo ai sync` 首个 `.claude` 集成步骤的方案。
目标有意保持狭窄：添加一个面向 Claude 的生成项目事实文件，
而不改变 hand-authored 的规则、技能、hooks、agent 或命令的所有权模型。

## 1. 目标

未来 `ncgo ai sync` 可以添加恰好一个新生成的文件：

- `.claude/generated/project-context.md`

该文件应是累加的，不应替换当前输出：

- `AGENTS.md`
- `CLAUDE.md`
- `.cursor/rules/ncgo.mdc`

## 2. 为什么这是第一步

这是最安全的 `.claude` 集成方式，因为它只添加确定性的项目事实。
它不会尝试生成仓库策略、工作流定义、角色边界或个人本地笔记。

## 3. 范围

### 在范围内

- 一个生成的文件：`.claude/generated/project-context.md`
- 从仓库状态派生的确定性内容
- 复用当前 `ai sync` 的所有权规则
- 与现有 AI 上下文文件的累加关系

### 不在范围内

- 生成 `.claude/rules/*`
- 生成 `.claude/skills/*`、`.claude/hooks/*`、`.claude/agents/*` 或 `.claude/commands/*`
- 替换 `CLAUDE.md`
- 超出当前 manifest + 嵌入文档输入的 AST 派生架构扫描
- `.claude/local/*` 内的个人覆盖层

## 4. 输入

首个版本应仅使用当前 `ai sync` 已信任的输入：

- `.ncgo/manifest.yaml`
- 与服务种类和所选语言匹配的嵌入式设计文档

首个版本 **不应** 消费本地覆盖层，例如：

- `AGENTS.local.md`
- `.claude/local/*`

原因：`.claude/generated/project-context.md` 应保持确定性和工具所有。

## 5. 输出形状

该文件应携带标准的管理标记并保持 Markdown 可读。

建议的章节：

1. `# Claude Project Context`
2. `## Project Facts`
   - module
   - service name
   - service kind
   - mode
   - service IDL（如果存在）
   - infra 列表
   - domain 列表
   - ncgo/assets 版本
3. `## Architecture & Built-in Features`
   - 简短的嵌入式设计文档摘要或正文摘录
4. `## Repository Rules`
   - 链接到 `.claude/rules/go.md`
   - 链接到 `.claude/rules/agent-engineering.md`
5. `## Notes`
   - 生成文件免责声明
   - 指向 `.claude/local/*` 的个人覆盖指引

该文件应强调事实和稳定引用，而非长篇策略文本。

## 6. 所有权和覆盖规则

该文件应遵循与当前 `ai sync` 目标相同的覆盖模型：

- 它 **必须** 携带 `<!-- ncgo:managed -->`
- 没有该标记的已有文件 **必须** 被跳过，除非使用 `--force`
- `--dry-run` **必须** 报告预期写入而不修改文件

该文件属于生成/工具所有层：

- `.claude/generated/project-context.md`

它 **不得** 覆盖或合并到以下 hand-authored 文件中：

- `.claude/rules/*`
- `.claude/skills/*`
- `.claude/hooks/*`
- `.claude/agents/*`
- `.claude/commands/*`
- `.claude/local/*`

## 7. 与当前文件的关系

`AGENTS.md` 和 `CLAUDE.md` 保持为长篇跨工具上下文文件。

`.claude/generated/project-context.md` 应是简短的、面向 Claude 的、生成的项目事实表。

建议的解释方式：

- `AGENTS.md`：广泛的多 agent 上下文
- `CLAUDE.md`：长篇 Claude 上下文
- `.claude/generated/project-context.md`：简短生成的项目事实
- `.claude/rules/*`：hand-authored 仓库策略
- `.claude/local/*`：个人本地覆盖层

## 8. CLI / MCP 行为预期

首个实现 **应** 保持当前命令和 MCP 契约稳定。

- `ncgo ai sync` CLI 仍报告 written/skipped 文件
- `ncgo_ai_sync` MCP 可以继续返回纯文本输出
- 新文件仅在适用时出现在 written/skipped 结果中

首个步骤不需要新标志。

## 9. 建议的实现顺序

1. 为 `.claude/generated/project-context.md` 添加新的渲染目标
2. 保持内容仅从当前 manifest + 嵌入式文档派生
3. 复用现有的 managed-marker、`--force` 和 `--dry-run` 行为
4. 为新渲染目标添加聚焦的单元测试
5. 仅在需要时更新 CLI/MCP 测试以适配额外的写入路径

## 10. 未来步骤（延期）

在首个步骤稳定后，后续阶段可以添加：

- `.claude/generated/manifest-summary.md`
- `.claude/generated/architecture.md`（来自 AST 派生的项目扫描）
- 用于启动 `.claude/skills/*` 或 `.claude/commands/*` 的显式 scaffold 命令

除非这些仍然纯粹是派生的且可安全覆盖，否则它们应与 `ai sync` 保持分离。
