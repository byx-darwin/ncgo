# Proto 输入输出 lint / 校验规则清单

本文把 [Proto 输入输出参数校验方案](proto-io-validation.zh-CN.md) 进一步拆解为面向实现的 lint / 校验规则清单，目标是为后续规则实现、任务拆分、CI 接入和 Agent 辅助修复提供稳定的 rule inventory。

## 1. 目标

本文只回答两个问题：

1. 哪些规则适合优先实现为静态 lint / 校验规则
2. 每条规则应以什么粒度、级别和输出字段落地

本文不重新讨论方案背景、原则与边界；这些内容以 `proto-io-validation.zh-CN.md` 为准。

## 2. 编号与分组

规则 ID 采用统一前缀 `PIO`（Proto I/O），并按分组划段：

- `PIO1xx`：通用契约规则
- `PIO2xx`：Hertz HTTP binding 规则
- `PIO3xx`：Kitex RPC 契约规则
- `PIO4xx`：PGV / 字段约束建议规则

规则级别沿用方案文档中的两级：

- `error`：阻断项
- `warning`：提示项

规则实施阶段分为：

- `phase1`：可直接静态实现，优先落地
- `phase2`：可实现，但包含一定启发式判断
- `phase3`：暂缓，待规则边界进一步收敛

## 3. 建议的 lint 输出字段

无论未来落为 CLI、MCP 还是内部库，建议每条命中结果至少输出以下字段：

- `rule_id`：规则 ID，如 `PIO201`
- `level`：`error` 或 `warning`
- `phase`：`phase1|phase2|phase3`
- `file`：proto 文件路径
- `service`：service 名称
- `rpc`：rpc 方法名
- `message`：命中的 message 名（如有）
- `field`：命中的字段名（如有）
- `summary`：单行问题摘要

可选字段：

- `path`：Hertz HTTP 路径
- `annotation`：命中的 annotation 名称
- `details`：结构化诊断补充信息
- `suggestion`：建议修改方向

## 4. Phase 1：优先实现的静态规则

以下规则建议作为第一批真正落地的 lint / 校验规则。

| Rule ID | Level | 范围 | 摘要 |
|---|---|---|---|
| `PIO101` | error | rpc | 每个 rpc 必须使用 `<Method>Req` 作为输入、`<Method>Resp` 作为输出 |
| `PIO102` | error | rpc | request / response message 名称必须与方法名严格对应 |
| `PIO103` | error | message | 顶层 request / response 不允许直接使用 `Any/Struct/Value` |
| `PIO201` | error | rpc | Hertz rpc 必须且只能声明一个 HTTP method annotation |
| `PIO202` | error | rpc+field | Hertz path 参数必须与 `(api.path)` 字段严格对齐 |
| `PIO203` | error | rpc+field | `GET/DELETE/HEAD` 不允许使用 `api.body` 或 `api.raw_body` |
| `PIO204` | error | field | 同一 request 字段不能声明多种 binding 语义 |
| `PIO205` | error | message | 单个 Hertz request 中 `raw_body` 最多只能出现一次 |
| `PIO206` | error | field | Hertz response 中不允许出现 request binding annotation |
| `PIO301` | error | message | Kitex response 不应混入 transport envelope 顶层字段 |

### 4.1 `PIO101`：rpc 输入输出必须采用 `<Method>Req/<Method>Resp`

- 级别：`error`
- 范围：rpc
- 阶段：`phase1`
- 检测条件：对每个 rpc，输入消息名称必须精确等于 `<MethodName>Req`，输出消息名称必须精确等于 `<MethodName>Resp`
- 输出摘要：`rpc GetUser must use GetUserReq/GetUserResp`
- 实现备注：该规则既约束命名，也间接要求方法级 request / response 独立建模

### 4.2 `PIO102`：请求 / 响应命名必须与方法名严格对应

- 级别：`error`
- 范围：rpc
- 阶段：`phase1`
- 检测条件：若输入或输出消息名称与方法名不一致，直接报错
- 输出摘要：`rpc Ping input PingRequest does not match expected PingReq`
- 实现备注：若后续允许 `Request/Response` 别名风格，应另行决策，不在第一版兼容

### 4.3 `PIO103`：顶层 request / response 禁止不透明动态结构

- 级别：`error`
- 范围：message
- 阶段：`phase1`
- 检测条件：rpc 输入或输出消息若直接为 `google.protobuf.Any`、`google.protobuf.Struct`、`google.protobuf.Value`，报错
- 输出摘要：`rpc Search top-level output must not use google.protobuf.Struct`
- 实现备注：第一版仅检查顶层 I/O，不递归检查嵌套字段

### 4.4 `PIO201`：Hertz rpc 必须且只能声明一个 HTTP method annotation

- 级别：`error`
- 范围：rpc
- 阶段：`phase1`
- 检测条件：统计 `api.get` / `api.post` / `api.put` / `api.patch` / `api.delete` / `api.options` / `api.head` 等 method annotation；计数不等于 1 时报错
- 输出摘要：`rpc Ping must declare exactly one HTTP method annotation`
- 实现备注：只要是 Hertz proto，就应按 method annotation 驱动；缺失或重复都不接受

### 4.5 `PIO202`：Hertz path 参数必须与 `(api.path)` 字段严格对齐

- 级别：`error`
- 范围：rpc + field
- 阶段：`phase1`
- 检测条件：
  - 从 HTTP 路径中提取 path 参数
  - 从 request message 中提取 `(api.path)` 字段
  - 任一侧有而另一侧无，或名称不一致，报错
- 输出摘要：`rpc GetUser path param id does not match request api.path fields`
- 实现备注：第一版只做一一对应检查，不引入更复杂的 path 风格规范

### 4.6 `PIO203`：`GET/DELETE/HEAD` 不允许使用 `body/raw_body`

- 级别：`error`
- 范围：rpc + field
- 阶段：`phase1`
- 检测条件：若 rpc method 为 `GET`、`DELETE` 或 `HEAD`，且 request 任一字段声明 `(api.body)` 或 `(api.raw_body)`，报错
- 输出摘要：`rpc ListUsers uses api.body on GET request`
- 实现备注：这是高收益低争议规则，适合第一批实现

### 4.7 `PIO204`：request 字段禁止多重 binding

- 级别：`error`
- 范围：field
- 阶段：`phase1`
- 检测条件：单个字段若同时声明两个或以上 binding annotation（如 `api.query` + `api.path`，或 `api.body` + `api.header`），报错
- 输出摘要：`field user_id declares multiple bindings: api.query, api.path`
- 实现备注：建议把 `query/path/header/cookie/body/raw_body/form` 一并纳入同一绑定集合判断

### 4.8 `PIO205`：单个 request 中 `raw_body` 最多一次

- 级别：`error`
- 范围：message
- 阶段：`phase1`
- 检测条件：统计 request 中 `(api.raw_body)` 字段数量，大于 1 报错
- 输出摘要：`message UploadReq declares more than one raw_body field`
- 实现备注：这是 message 级规则，不依赖具体 HTTP method

### 4.9 `PIO206`：Hertz response 中禁止 request binding annotation

- 级别：`error`
- 范围：field
- 阶段：`phase1`
- 检测条件：response message 中任一字段若声明 `api.query`、`api.path`、`api.header`、`api.cookie`、`api.body`、`api.raw_body` 等请求侧 annotation，报错
- 输出摘要：`response field message must not use request binding annotation api.body`
- 实现备注：第一版可统一按“response 禁止所有 request binding annotation”处理

### 4.10 `PIO301`：Kitex response 不应混入 transport envelope 顶层字段

- 级别：`error`
- 范围：message
- 阶段：`phase1`
- 检测条件：若 Kitex response 顶层字段中命中 transport 风格字段集合中的两个或以上，例如 `code`、`msg`、`success`、`error`，报错
- 输出摘要：`response GetUserResp looks like transport envelope with fields code,msg,success`
- 实现备注：该规则本质带有轻微启发式，但命中条件足够稳定，可先按严格顶层字段名集合实现

## 5. Phase 2：可做但带启发式的规则

以下规则带有一定启发式判断，但截至当前已经落地了一版保守实现。它们默认以 `warning` 参与 `protolint` / `doctor` / MCP 输出；命中 warning 时仍会出现在 `diagnostics` 中，但 **warning-only 不会让 `ok=false`**。

| Rule ID | Level | 范围 | 摘要 |
|---|---|---|---|
| `PIO111` | warning | message | 谨慎使用 `google.protobuf.Empty` 作为 request / response |
| `PIO112` | warning | message | 顶层使用 `CommonReq/CommonResp/BaseResp` 等泛化 message |
| `PIO113` | warning | message | request 字段数量过多 |
| `PIO211` | warning | field | Hertz request 字段未声明 binding annotation |
| `PIO212` | warning | rpc+message | Hertz 缺 OpenAPI schema / property 元信息 |
| `PIO302` | warning | rpc+message | Kitex `List* / Search*` 风格方法缺少分页或游标结构 |
| `PIO303` | warning | message | Kitex request 结构过于“万能” |
| `PIO401` | warning | field | 分页类 request 字段缺少明显 PGV 范围约束 |

### 5.1 `PIO111`：谨慎使用 `google.protobuf.Empty`

- 级别：`warning`
- 阶段：`phase2`
- 检测条件：rpc 输入或输出直接使用 `google.protobuf.Empty`
- 说明：该规则存在场景差异，暂不建议第一版直接阻断
- 当前实现：仅检查顶层 rpc input / output 是否直接为 `google.protobuf.Empty`

### 5.2 `PIO112`：顶层使用泛化 message

- 级别：`warning`
- 阶段：`phase2`
- 检测条件：顶层 request / response message 命中常见泛化命名，如 `CommonReq`、`CommonResp`、`BaseResp`、`Result`
- 说明：该规则可静态实现，但误报边界需要团队再确认
- 当前实现：最小命名集合为 `CommonReq`、`CommonResp`、`BaseResp`、`Result`

### 5.3 `PIO113`：request 字段数量过多

- 级别：`warning`
- 阶段：`phase2`
- 检测条件：request 字段数超过阈值（阈值建议后续单独确定）
- 说明：该规则不是正确性校验，而是复杂度提示
- 当前实现：阈值固定为 **超过 12 个顶层字段** 时提示

### 5.4 `PIO211`：Hertz request 字段未声明 binding annotation

- 级别：`warning`
- 阶段：`phase2`
- 检测条件：Hertz request 字段未命中任何已识别 binding annotation
- 说明：该规则很有价值，但需先确认是否允许少量“由生成器默认处理”的字段存在
- 当前实现：仅检查 Hertz RPC 的顶层 request 字段；只要未命中 `api.query/path/header/cookie/body/raw_body/form` 中任一 binding 就提示

### 5.5 `PIO212`：Hertz 缺 OpenAPI 元信息

- 级别：`warning`
- 阶段：`phase2`
- 检测条件：rpc 缺 `openapi.operation`，或 message/field 缺 `openapi.schema` / `openapi.property`
- 说明：这更偏文档质量，不适合作为第一版阻断项
- 当前实现：检查 Hertz RPC 是否缺 `openapi.operation`、response message 是否缺 `openapi.schema`、以及 request/response 顶层字段是否缺 `openapi.property`

### 5.6 `PIO302`：列表 / 搜索型接口缺少分页结构

- 级别：`warning`
- 阶段：`phase2`
- 检测条件：方法名前缀匹配 `List` / `Search` / `Query`，但 request 中未发现常见分页字段，如 `page`、`page_size`、`limit`、`cursor`
- 说明：该规则为风格提示，不代表所有列表接口都必须分页
- 当前实现：仅检查非 Hertz RPC；若方法名前缀匹配 `List` / `Search` / `Query`，且 request 顶层字段中未发现 `page`、`page_size`、`limit`、`cursor`、`offset`，则给出提示

### 5.7 `PIO303`：Kitex request 结构过于“万能”

- 级别：`warning`
- 阶段：`phase2`
- 检测条件：request 同时包含过多筛选、排序、分页、调试开关、扩展字段，命中阈值后提示
- 说明：此规则高度启发式，建议后续结合真实 proto 再收敛
- 当前实现：仅检查非 Hertz request；按字段名把职责粗分为 `filter` / `sort` / `pagination` / `debug` / `extension`，当同一 request 同时命中 **至少 3 类职责且命中字段数至少 4 个** 时提示

### 5.8 `PIO401`：分页类字段缺少明显 PGV 范围约束

- 级别：`warning`
- 阶段：`phase2`
- 检测条件：request 命中典型分页字段名（如 `page`、`page_size`、`limit`、`offset`），但未声明明显范围约束
- 说明：第一版可先从分页类字段开始，而不是试图穷举所有“应该有 PGV”的输入字段
- 当前实现：检查 request 顶层字段名是否命中 `page`、`page_size`、`limit`、`offset`；若字段上未看到 `validate.rules` 中明显的 `gt/gte/lt/lte` 范围约束，则给出提示

## 6. Phase 3：暂缓实现的规则方向

以下方向不建议在当前阶段直接实现为 lint 规则：

- response 全面 PGV 覆盖建议
- 复杂 WKT / `Any` / `oneof` 的深层约束检查
- 业务 DTO 语义合理性判断
- 历史版本兼容性 diff
- 自动修复或 rewrite proto 结构

这些方向要么边界过大，要么更适合后续独立成专题工具。

## 7. 推荐落地顺序

### 7.1 第一批

建议先实现：

- `PIO101`
- `PIO102`
- `PIO103`
- `PIO201`
- `PIO202`
- `PIO203`
- `PIO204`
- `PIO205`
- `PIO206`
- `PIO301`

这批规则全部具备明确输入、明确输出、明确静态检测路径，适合作为第一轮 lint MVP。

### 7.2 第二批

这批规则已落地第一版最小实现；后续可继续根据真实 proto 收敛误报边界。当前已实现的 phase2 warning 包括：

- `PIO111`
- `PIO112`
- `PIO113`
- `PIO211`
- `PIO212`
- `PIO302`
- `PIO303`
- `PIO401`

## 8. 相关文档

- [Proto 输入输出参数校验方案](proto-io-validation.zh-CN.md)
- [Proto 输入输出 lint 实现任务拆分](proto-io-implementation.zh-CN.md)
- [Proto 输入输出 lint 技术设计（T001 ~ T003）](proto-io-tech-design.zh-CN.md)
- [Hertz 模板设计](../internal/assets/_data/docs/hertz/design-doc.zh-CN.md)
- [Kitex 模板设计](../internal/assets/_data/docs/kitex/design-doc.zh-CN.md)

## 9. 结论

本文将 Proto 输入输出参数方案收敛为一份可实施的规则清单：

- `phase1` 规则优先覆盖高收益、低争议、可纯静态实现的错误类问题
- `phase2` 规则承载带启发式的设计提示与 PGV 建议
- `phase3` 方向暂缓，避免第一版 lint 过重或边界失控

后续若进入代码实现阶段，可直接按本文中的 Rule ID 拆解任务、设计输出结构和安排测试覆盖。