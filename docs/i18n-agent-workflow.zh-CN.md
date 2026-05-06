# ncgo Hertz i18n Agent 协作规范

本文定义 AI Agent 参与 Hertz i18n 补译时的输入、输出、边界和校验流程。若需要固定 prompt / 协议，请同时参考 [i18n Agent 协议](i18n-agent-protocol.zh-CN.md)；若需要机器可消费字段约束，请参考 [i18n Agent JSON Schema](i18n-agent-schema.zh-CN.md)。

## 1. 目标

Agent 在这套流程中的定位是：

- 读取工具链输出的缺口报告
- 结合 source locale、术语表和代码上下文补齐多语言文案
- 不负责定义工程规则
- 不负责跳过校验或直接修改生成物

## 2. Agent 的输入

执行补译前，Agent 应优先读取以下内容：

1. `internal/pkg/i18n/.meta/report.md`
2. `internal/pkg/i18n/.meta/report.json`
3. `internal/pkg/i18n/locales/zh-CN.json`
4. 目标 locale 文件，例如 `ja-JP.json`
5. `internal/pkg/i18n/glossary.json`
6. 涉及该 key 的业务代码上下文

补充建议：

- 优先先读 `report.json.summary`
- 再读 `missing_translations`、`stale_translations`、`draft_translations`
- 如果存在 `glossary_hints`，应把它们作为优先 review 项

如果缺少 source 文案，应先停止补译并提示开发者补 `zh-CN.json`。

## 3. Agent 的输出

Agent 默认只应修改以下文件：

- `internal/pkg/i18n/locales/*.json`
- `internal/pkg/i18n/.meta/status.json`

除非任务明确要求，否则不应直接修改：

- `internal/pkg/i18n/catalog_gen.go`
- `Makefile`
- 业务逻辑代码
- CI 配置

## 4. Agent 的处理原则

### 4.1 以 source locale 为准

- `zh-CN.json` 是 source truth
- 不应仅凭 key 名猜测最终语义
- 对于短消息，应尽量结合代码使用点理解语境
- 某些 key 可能来自 `oops.Public("...")`，这类场景更需要结合代码上下文理解语义

### 4.2 尽量保留已有人工翻译

如果目标语言已有可信文案：

- 不随意覆盖
- 优先只处理缺失项和 `stale` 项
- 对已有内容仅做必要修正

### 4.3 严格保持占位符一致

必须保持以下内容与 source locale 一致：

- `%s`
- `%d`
- `%v`
- `{id}`
- `{name}`

不得擅自删除、改名或增加占位符。

### 4.4 优先遵循 glossary

对于核心术语，优先参考：

- `token`
- `signature`
- `session`
- `permission`
- `route`
- `idempotency`
- `rate limit`

如果 glossary 与已有翻译冲突，应先保持 glossary 一致，再由人工 review。

## 5. 推荐补译流程

1. 读取 `report.md` / `report.json`
2. 识别待处理项：
   - missing translations
   - stale translations
   - draft translations
3. 先查看 `summary` 判断问题规模
4. 如果存在 `glossary_hints`，优先 review 这些提示项
5. 读取 `zh-CN.json` 中对应 source 文案
6. 阅读相关代码上下文
7. 参考 glossary 生成目标语言文案
8. 更新 locale 文件
9. 如有状态文件，则更新对应条目状态
10. 执行 `make i18n-check`
11. 执行 `make i18n`

## 6. 不建议 Agent 做的事

Agent 不应默认执行以下动作：

- 自动新增业务错误 key
- 绕过 `i18n-check`
- 直接手改生成文件
- 在 source 文案缺失时硬翻译其他语言
- 未经说明就批量重写所有 locale

## 7. 处理不同类型条目的建议

### 7.1 missing translation

处理方式：

- 直接根据 source 文案补齐目标语言
- 保持语气简洁，符合后端错误消息风格

### 7.2 stale translation

处理方式：

- 重新对照最新 source 文案翻译
- 检查旧译文是否仍然适用
- 如果 source 语义变化较大，应直接重写

### 7.3 draft translation

处理方式：

- 优先补足明显缺陷
- 统一术语和表达风格
- 若文案已足够自然，可维持不动并等待人工 review

## 8. 推荐输出风格

对于后端错误消息，建议遵循：

- 简洁
- 直接
- 不过度口语化
- 避免加入原文没有的解释
- 避免过长句式

## 9. 补译后的固定校验

每次补译后，Agent 应推动执行：

1. `make i18n-check`
2. `make i18n`

如果这次任务还改到了 i18n 工具实现本身，建议再执行：

3. `go test ./tools/...`

如果 `check` 失败，应优先修复：

- 缺 key
- 空值
- 占位符不一致
- stale 未处理

如果 `check` 只输出 glossary warning，则表示：

- 当前结果可以继续生成，但建议进入人工 review
- Agent 应优先检查 warning 对应条目，确认是否需要按 glossary 收敛术语

## 10. 相关文档

- [i18n Hybrid 方案](i18n-hybrid-plan.zh-CN.md)
- [i18n 工作流](i18n-workflow.zh-CN.md)
- [i18n Agent 协议](i18n-agent-protocol.zh-CN.md)
- [i18n Agent JSON Schema](i18n-agent-schema.zh-CN.md)
