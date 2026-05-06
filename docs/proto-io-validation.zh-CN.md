# Hertz / Kitex Proto 输入输出参数校验方案

本文定义 Hertz 与 Kitex 模板中 `.proto` 输入输出参数的校验范围、分层方式与最小规则集，目标是在不扩展到完整业务语义规范的前提下，建立一套稳定、可自动化、可渐进增强的 IDL 参数校验基础。

## 1. 背景

当前 Hertz 与 Kitex 模板都以 Protobuf IDL 作为接口契约来源。随着接口数量增加，仅依赖人工 review 难以稳定发现以下问题：

- request / response 命名不一致
- Hertz HTTP 参数绑定缺失、冲突或语义不清
- Kitex response 混入 transport envelope
- 输入字段约束没有在 IDL 中显式表达
- 接口契约在可读性、可生成性、可演化性方面不稳定

同时，`cloudwego/protoc-gen-validator`（以下简称 PGV）可以为 Protobuf message 生成 `Validate() error` 方法，适合承载字段级约束表达。因此需要形成一套统一的 Proto 输入输出参数校验方案。

## 2. 目标

本方案目标是：

- 统一 `.proto` 中 request / response 的基本契约约束
- 约束 Hertz 场景下 HTTP 参数绑定方式，降低生成与运行时歧义
- 约束 Kitex 场景下 RPC 输入输出设计风格，保持契约清晰
- 结合 PGV 表达字段级输入约束
- 为后续 lint、CI、Agent 辅助生成提供统一规则基础

## 3. 非目标

本方案不覆盖以下内容：

- `ncgo new`、模板输入、脚手架参数校验
- 运行时配置校验，例如 `conf.Validate()`
- handler 中 `BindAndValidate` 的运行时行为
- 业务语义正确性判断
- 历史版本兼容性 diff 校验
- 数据库、缓存、外部依赖的健康检查

## 4. 总体设计

本方案采用两层校验模型：

### 4.1 契约层校验

契约层用于校验 request / response 与 rpc 方法之间的结构关系，以及 Hertz / Kitex 各自的接口约定。该层关注的是“接口设计是否合理”，而不是“某个字段值是否合法”。

契约层主要覆盖：

- request / response 是否与方法一一对应
- message 命名是否稳定、清晰
- Hertz HTTP method / path / query / body 绑定是否明确且无冲突
- Kitex response 是否保持业务 payload 边界，不混入 transport 语义

### 4.2 字段约束层校验

字段约束层使用 PGV 表达字段规则，关注的是“字段值范围是否被显式声明”。第一版建议优先覆盖 request message，不要求对 response 全面启用 PGV。

建议优先覆盖的约束包括：

- string 长度限制
- 数值范围
- repeated / map 数量限制
- pattern
- 枚举值约束
- 非空类约束

## 5. 设计原则

### 5.1 分层治理

契约层与字段约束层职责不同：

- 契约层负责消息与方法关系
- PGV 负责字段值规则

两者不能相互替代。

### 5.2 先做高收益规则

第一版优先覆盖容易出错、自动化收益高、团队争议较小的规则，不一次性扩展为完整 API 风格规范。

### 5.3 优先约束输入参数

从收益角度看，request 的约束价值显著高于 response。第一版应优先把 request 校验做好，再评估是否扩展到 response。

### 5.4 输出分级

规则分为两类：

- `error`：必须修复，否则不通过
- `warning`：建议修复，作为设计提示

## 6. 通用契约规则

以下规则适用于 Hertz 与 Kitex。

### 6.1 Error 级规则

- 每个 rpc 必须有独立的 request / response message，建议统一采用 `<Method>Req` 与 `<Method>Resp`
- 请求 / 响应命名必须与方法名对应，例如 `GetUser` 对应 `GetUserReq` 与 `GetUserResp`
- 禁止公开 I/O 顶层使用不透明动态结构，如 `google.protobuf.Any`、`google.protobuf.Struct`、`google.protobuf.Value`
- 禁止将 transport 语义混入通用业务响应，例如顶层直接承载 `code`、`msg`、`success`、`error`

### 6.2 Warning 级规则

- 谨慎使用 `google.protobuf.Empty`，普通业务接口建议显式定义空的 `XxxReq` / `XxxResp`
- 请求字段过多时给出提示，提醒接口可能职责过重
- 避免使用泛化顶层 message，如 `CommonReq`、`CommonResp`、`BaseResp`

## 7. Hertz 专属规则

以下规则适用于带 HTTP annotation 的 Protobuf 服务。

### 7.1 Error 级规则

- 每个 rpc 必须且只能有一个 HTTP method annotation，例如 `api.get`、`api.post`、`api.put`、`api.patch`、`api.delete`
- path 参数必须与 request 中的 `(api.path)` 字段严格对应：路径中出现的参数必须存在对应字段，请求中声明的 `(api.path)` 字段也必须出现在路径中
- `GET` / `DELETE` / `HEAD` 不允许绑定 `body` 或 `raw_body`
- 同一 request 字段不能同时声明多种 binding 语义，如 query/path/body/header/cookie/raw_body
- `raw_body` 最多只能出现一次
- response 中禁止出现 request binding annotation，如 `api.query`、`api.path`、`api.header`、`api.cookie`、`api.raw_body`

### 7.2 Warning 级规则

- request 字段建议显式绑定来源，否则提示该字段可能不会被稳定消费
- 建议补充 OpenAPI schema / property 信息，但第一版不阻断
- 复杂 request 不建议混杂过多 query 与 body 语义

## 8. Kitex 专属规则

以下规则适用于纯 RPC 风格的 Protobuf 服务。

### 8.1 Error 级规则

- 每个 rpc 必须有独立的 `Req/Resp`
- response 不应混入 transport 状态字段；在 Kitex 模板已有统一 `rpcerror` 机制时，顶层不应再承载明显 transport 语义的 `code`、`msg`、`success`

### 8.2 Warning 级规则

- 列表型接口建议具备明确分页或游标结构
- 避免设计“万能请求对象”，即把筛选、排序、分页、调试开关、扩展控制等多种职责集中在单个 request 中
- response 应尽量聚焦业务 payload，而不是将 transport 状态与业务数据混合表达

## 9. PGV 字段约束层规则

### 9.1 PGV 的定位

PGV 用于补充字段级约束表达，为 Protobuf message 生成 `Validate() error` 方法。它主要解决的问题是：

- 某个字段是否应限制长度
- 某个数值是否应限制范围
- 某个列表是否应限制数量
- 某个字符串是否应匹配 pattern
- 某个字段是否应限制枚举范围

PGV 不负责 request / response 命名、HTTP binding、path 对齐或 transport envelope 等契约层问题。

### 9.2 使用建议

- 第一版重点将 PGV 用于 request message
- 优先覆盖基础类型的高收益约束：string、数值、repeated、map、enum
- 对具有明确输入约束的字段，应优先在 proto 中声明 PGV 规则，例如页大小、关键字长度、ID 范围、枚举值范围
- response 不作为第一版 PGV 重点
- 对 `Any`、复杂 WKT、部分 `oneof` 场景应保守承诺，不在第一版中过度扩展

## 10. 输出分级建议

### 10.1 Error

以下问题建议作为阻断项：

- 缺失独立 `Req/Resp`
- 命名与方法名不匹配
- Hertz 缺失或重复 HTTP method annotation
- path 参数与 `(api.path)` 字段不一致
- `GET` / `DELETE` / `HEAD` 使用 body 或 raw_body
- request 字段存在多重 binding
- response 带 request binding annotation
- 顶层 I/O 使用动态不透明结构
- Kitex response 混入明显 transport envelope

### 10.2 Warning

以下问题建议作为提示项：

- 使用 `google.protobuf.Empty`
- 顶层使用泛化 message
- request 字段过多
- Hertz 缺 OpenAPI schema / property 信息
- 可明确约束的 request 字段缺少 PGV 声明
- 列表 / 搜索型接口缺少分页提示

## 11. MVP 范围

第一版建议只落地最小闭环：

### 通用

- `<Method>Req` / `<Method>Resp`
- 顶层动态结构禁用
- 泛化顶层 message 警告

### Hertz

- HTTP method annotation 唯一性
- path 参数对齐
- `GET` / `DELETE` / `HEAD` 禁止 body / raw_body
- request binding 冲突检测
- response 禁止 request binding annotation

### Kitex

- response 中 transport 风格字段检测

### PGV

- request 优先
- 仅覆盖基础类型高收益字段约束

## 12. 不采用的方案

### 12.1 只依赖人工 review

不采用。原因是规则分散、稳定性差、难以自动化。

### 12.2 只使用 PGV

不采用。原因是 PGV 仅覆盖字段值约束，无法覆盖 Req/Resp 关系、Hertz binding、Kitex response 风格与顶层 message 契约问题。

### 12.3 一次性制定完整 API 风格规范

暂不采用。原因是范围过大、争议较多，不适合作为第一版落地目标。

## 13. 待定事项

以下为原始待定问题及当前决策状态：

- **`google.protobuf.Empty` 是否从 warning 升级为 error**：维持 `warning`（`PIO111`）；该类型在健康检查等场景有合理用法，直接阻断误报风险较高。
- **Hertz request 中未绑定字段是否直接报错**：维持 `warning`（`PIO211`）；框架默认处理字段的场景存在，直接报错误报风险较高。
- **Kitex response 中 `code/msg/success` 是否直接作为 hard error**：已作为 `error`（`PIO301`）；命中 2 个及以上 transport envelope 字段时阻断，单字段不触发以控制误报。
- **是否只校验对外接口，还是所有 rpc**：当前实现对所有 rpc 统一检查，不区分对外 / 内部；待有实际区分需求时再引入过滤机制。
- **response 是否需要系统性接入 PGV**：当前不在规划范围；`PIO4xx` 仅覆盖 request 侧分页字段（`PIO401`），response PGV 暂缓。
- **是否引入更强的列表分页 / 搜索接口风格规则**：已以最小实现落地为 `warning`（`PIO302`）；更强的强制规则暂缓，待收集真实 proto 反馈后评估。

## 14. 推进建议

建议按以下顺序推进：

### Phase 1：契约层最小闭环

先落地高收益低争议规则：

- Req/Resp
- Hertz binding
- Kitex response 风格

### Phase 2：PGV 接入

在 request message 上引入 PGV：

- 基础类型
- 高收益字段约束

### Phase 3：规则升级

根据实际使用反馈：

- 调整 warning / error 边界
- 扩大 PGV 覆盖范围
- 补充更细的接口风格规则

## 15. 相关文档

- [Proto 输入输出 lint / 校验规则清单](proto-io-lint-rules.zh-CN.md)
- [Proto 输入输出 lint 实现任务拆分](proto-io-implementation.zh-CN.md)
- [Proto 输入输出 lint 技术设计（T001 ~ T003）](proto-io-tech-design.zh-CN.md)
- [Hertz 模板设计](../internal/assets/_data/docs/hertz/design-doc.zh-CN.md)
- [Kitex 模板设计](../internal/assets/_data/docs/kitex/design-doc.zh-CN.md)

## 16. 结论

本方案采用“**契约层 + PGV 字段约束层**”的两层模型，对 Hertz / Kitex 的 `.proto` 输入输出参数进行治理。

其中：

- 契约层负责 request / response 与 rpc 的结构关系，以及 Hertz / Kitex 的接口风格约束
- PGV 负责字段值约束表达，第一版优先覆盖 request

该方案优先解决高收益、低争议的问题，不将第一版扩展为完整业务语义规范。