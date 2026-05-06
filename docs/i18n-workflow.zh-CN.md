# ncgo Hertz i18n 工作流

本文说明在 `ncgo` Hertz 模板中，如何配合 `locales/*.json`、`make i18n-*` 命令和 AI Agent 维护多语言文案。

## 1. 基本约定

- source locale：`zh-CN`
- 正式翻译源：`internal/pkg/i18n/locales/*.json`
- 生成命令：`make i18n`
- 同步命令：`make i18n-sync`
- 报告命令：`make i18n-report`
- 校验命令：`make i18n-check`

补充说明：

- `i18n-sync` 会扫描静态 `Msg: "..."` 和 `Public("...")`
- `i18n-report` 会输出 `summary` 和 `glossary_hints`
- `i18n-check` 会输出 glossary warning，但当前默认不因 warning 失败

原则：

- `zh-CN.json` 是源文案真值
- 其他语言由 source locale 派生
- Agent 负责补译，工具链负责同步、校验、生成

## 2. 新增一个消息 key

以新增 `order_conflict` 为例，推荐流程如下：

1. 在业务代码中注册新的消息 key
2. 在 `internal/pkg/i18n/locales/zh-CN.json` 中补 source 文案
3. 执行 `make i18n-sync`
4. 执行 `make i18n-report`
5. 让 Agent 根据报告补齐其他语言文案
6. 执行 `make i18n-check`
7. 执行 `make i18n`

### 2.1 source locale 必须先补

不要只新增 key 而不补 `zh-CN` 原文。

原因：

- 其他语言翻译依赖 source 文案
- `i18n-check` 会对缺失 source 文案报错
- 没有 source 文案时，Agent 只能猜测语义

### 2.2 哪些代码会被扫描

当前阶段，`i18n-sync` 主要会从这些静态位置提取 key：

- `Definition{ Msg: "..." }`
- `response.Definition{ Msg: "..." }`
- `oops.Public("...")`

如果是运行时拼接出来的字符串，默认不会自动纳入同步。

## 3. 修改 source locale 文案

当 `zh-CN.json` 中已有文案发生变化时，推荐流程如下：

1. 修改 `zh-CN.json`
2. 执行 `make i18n-sync`
3. 让工具把受影响项标记为 `stale`
4. 执行 `make i18n-report`
5. 让 Agent 处理 `stale` 项
6. 执行 `make i18n-check`
7. 执行 `make i18n`

常见场景：

- `订单冲突` 改为 `订单状态冲突`
- 原文新增占位符，如 `{id}` 或 `%s`
- 原文语气从通用报错调整为业务化表达

## 4. 新增一种语言

以新增 `it-IT` 为例，推荐流程如下：

1. 新建 `internal/pkg/i18n/locales/it-IT.json`
2. 填写 `language`、`aliases` 和空的 `messages`
3. 执行 `make i18n-sync`
4. 执行 `make i18n-report`
5. 让 Agent 批量补齐 `it-IT` 文案
6. 执行 `make i18n-check`
7. 执行 `make i18n`

## 5. 推荐命令顺序

### 5.1 日常开发

推荐顺序：

1. `make i18n-sync`
2. `make i18n-report`
3. Agent 补译
4. `make i18n-check`
5. `make i18n`

如果要进入更严格的发布前检查，可再执行：

6. `make i18n-check-release`

### 5.2 生成前检查

如果只是想确认当前 locale 是否健康，至少执行：

1. `make i18n-check`
2. `make i18n`

如果你想先看问题分布，再决定是否让 Agent 处理，推荐先看：

1. `make i18n-report`
2. `internal/pkg/i18n/.meta/report.md`
3. `internal/pkg/i18n/.meta/report.json`

## 6. 常见校验失败

### 6.1 source locale 缺原文

现象：

- `i18n-check` 报 source locale missing

处理：

- 先补 `zh-CN.json`
- 再重新执行 `sync -> report -> check`

### 6.2 目标语言缺 key

现象：

- `i18n-check` 报某个 locale 缺少 key

处理：

- 先执行 `make i18n-sync`
- 再让 Agent 补齐文案

### 6.3 占位符不一致

现象：

- `%s`、`%d`、`{id}` 等数量或名称不一致

处理：

- 优先以 `zh-CN` 为准
- 修正目标语言中的占位符
- 再执行 `make i18n-check`

### 6.4 stale 项未处理

现象：

- 发布模式下 `i18n-check` 报 stale

处理：

- 让 Agent 或人工重新翻译 stale 项
- 完成后再进入生成或发布流程

### 6.5 glossary warning

现象：

- `i18n-check` 输出某个语言/消息可能没有使用推荐术语

处理：

- 先查看 `report.md` / `report.json` 中的 `glossary_hints`
- 对照 `internal/pkg/i18n/glossary.json` 判断是否需要修正
- 如果当前译法更自然，也可以保留并在 review 时说明

注意：

- 当前阶段 glossary warning 默认不阻断 `make i18n-check`
- 但它应被视为需要 review 的信号

## 7. 推荐团队约定

建议团队统一以下规则：

- 新 key 必须先补 `zh-CN`
- 不直接手改 `catalog_gen.go`
- 所有 locale 变更最终都要经过 `make i18n-check`
- Agent 补译后必须再执行 `make i18n`
- 关键术语统一参考 `internal/pkg/i18n/glossary.json`

如果生成项目中修改了 i18n 工具本身，建议额外执行：

- `go test ./tools/...`

## 8. 相关文档

- [i18n Hybrid 方案](i18n-hybrid-plan.zh-CN.md)
- [i18n Agent 协作](i18n-agent-workflow.zh-CN.md)
- [i18n Agent 协议](i18n-agent-protocol.zh-CN.md)
- [i18n Agent JSON Schema](i18n-agent-schema.zh-CN.md)
- [i18n CLI / MCP Payload](i18n-payload.zh-CN.md)
