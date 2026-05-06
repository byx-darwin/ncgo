# Proto 输入输出 lint 技术设计（T001 ~ T003）

本文聚焦 `docs/proto-io-implementation.zh-CN.md` 中的三个前置任务：

- `T001`：确定 proto 解析后端
- `T002`：建立统一 Proto Index / Model
- `T003`：建立 Diagnostic 结构与规则注册表

目标是尽快把 proto lint 的底层技术路线定下来，为后续 `PIO101` ~ `PIO301` 的实现提供稳定基础。

## 1. 设计约束

在当前仓库语境下，proto lint 的底层设计至少要满足以下约束：

- **纯静态分析优先**：不依赖已生成 Go 代码，直接面向 `.proto` 源文件工作
- **支持 Hertz 自定义 annotation**：必须能识别 `api.proto` 中的 field / method options
- **支持 OpenAPI annotation**：至少要能读取 `openapi/annotations.proto` 中的方法、message、field options
- **支持 Kitex 普通 protobuf 服务**：不能只服务于 Hertz HTTP 场景
- **能拿到 source 位置信息**：diagnostic 需要稳定定位到 file / line / rpc / field
- **尽量不依赖本机 `protoc`**：当前仓库的静态分析与 doctor 风格更偏“纯 Go 内部能力”，不适合把 `protoc` 变成 lint 的硬前置
- **与现有 JSON / diagnostic 风格兼容**：后续若进入 CLI 或 MCP，需要能平滑对接现有结构化输出模式

## 2. T001：proto 解析后端选择

## 2.1 候选方案

### 方案 A：外部调用 `protoc`

基本思路：

- 调用 `protoc --descriptor_set_out`
- 打开 `--include_imports`
- 打开 `--include_source_info`
- 再从 `FileDescriptorSet` 中读取 service / rpc / message / field / options

优点：

- 结果与官方编译器最接近
- `descriptor set` 语义清晰
- `SourceCodeInfo` 能力成熟

缺点：

- 需要本机安装 `protoc`
- lint / doctor / CI 将多出系统依赖
- 进程启动与 I/O 成本更高
- import 路径、工作目录与跨平台调用更繁琐
- 与当前仓库“纯 Go 静态扫描优先”的风格不一致

结论：**不推荐作为主实现方案**。可以作为调试或对照手段，但不适合做默认运行路径。

### 方案 B：`github.com/jhump/protoreflect/desc/protoparse`

基本思路：

- 用 `protoparse` 直接解析 `.proto`
- 拿到 descriptor / option / message / service 信息

优点：

- 社区使用历史较长
- API 相对直接
- 能覆盖常见 proto 解析需求

缺点：

- 对 custom options 的处理边界更复杂
- 历史包袱较重，和新版 protobuf 运行时的结合不如更新方案自然
- 在 custom options / unknown options 细节上更容易出现实现层 caveat

结论：**可用，但不是首选**。

### 方案 C：`github.com/bufbuild/protocompile`

基本思路：

- 使用纯 Go 编译器直接解析并链接 `.proto`
- 在内部完成 import 解析、option 解释、source info 生成
- 输出可消费的 linked descriptors

优点：

- 纯 Go，无需依赖本机 `protoc`
- 支持 `SourceCodeInfo`
- 能处理 linked descriptors 与 custom options
- 与 Buf 内部编译器同源，能力边界更现代
- 更适合作为仓库内长期维护的静态分析底座

缺点：

- 引入新依赖
- API 比单纯“跑 protoc”稍复杂，需要额外封装

结论：**推荐作为主实现方案**。

## 2.2 推荐结论

`T001` 推荐采用：

- **主方案：`bufbuild/protocompile`**
- **不采用：外部 `protoc` 作为默认路径**
- **备选但不优先：`jhump/protoreflect/protoparse`**

### 推荐原因

核心原因有四个：

1. **满足纯 Go 静态分析需求**
   - 不把 `protoc` 变成 lint/doctor 的运行前提
2. **更适合处理 custom options**
   - Hertz 的 `api.*` / OpenAPI options 是刚需
3. **具备 source info 能力**
   - 能支撑稳定 diagnostic 定位
4. **更符合长期演进路线**
   - 后续若扩展到更复杂 proto 规则，`protocompile` 的技术路线更稳

## 2.3 不采用 `protoc` 主路径的原因

即便 `protoc` 结果最“官方”，当前仍不建议把它作为主路径。主要原因：

- 当前 `go.mod` 仍非常轻，不适合把系统工具依赖直接引入 proto lint 基础路径
- `doctor` 当前检查工具缺失，但不依赖工具来完成自身静态扫描；proto lint 若改成强依赖 `protoc`，会破坏这种一致性
- 后续如果 proto lint 进入 CLI / doctor / MCP，外部进程调用会放大复杂度

因此：

- `protoc` 可以作为人工对照或未来 debug 手段
- 不应成为第一版实现依赖

## 3. T002：统一 Proto Index / Model

## 3.1 目标

统一模型层的职责不是“完整重建 protobuf 世界”，而是为规则执行提供一个**稳定、窄而够用**的数据面。

换句话说，规则实现不应直接操作底层解析库对象，而应依赖一个仓库内可控的中间结构。

## 3.2 推荐模型边界

建议最小模型至少覆盖以下概念：

- `File`
- `Service`
- `RPC`
- `Message`
- `Field`
- `Binding`
- `Location`

建议统一模型中显式保留这些字段：

### File 级

- `Path`
- `Package`
- `Syntax`
- `Services`
- `Messages`

### Service 级

- `Name`
- `File`
- `RPCs`
- `Location`

### RPC 级

- `Name`
- `ServiceName`
- `InputMessageName`
- `OutputMessageName`
- `InputMessage`
- `OutputMessage`
- `HTTPMethod`
- `HTTPPath`
- `PathParams`
- `Location`

### Message 级

- `Name`
- `File`
- `Fields`
- `TopLevelFieldNames`
- `Location`

### Field 级

- `Name`
- `Number`
- `TypeName`
- `ParentMessage`
- `Bindings`
- `Location`

### Binding 级

建议统一成枚举化结构，而不是规则里到处直接比字符串：

- `query`
- `path`
- `header`
- `cookie`
- `body`
- `raw_body`
- `form`
- `unknown`

## 3.3 统一模型应提供的派生能力

建议不要把所有逻辑塞进规则函数里。模型层或 helper 层应直接提供这些派生结果：

- 某个 rpc 是否为 Hertz 风格（是否声明 HTTP method annotation）
- 某个 rpc 的 path 参数集合
- 某个字段的 binding 集合
- 某个 message 是否是顶层 request / response
- 某个 response 是否命中 transport envelope 字段集合
- 某个 message 中 `raw_body` 字段数量

这样能显著降低规则函数复杂度。

## 3.4 与底层解析库的隔离原则

统一模型应满足：

- 规则层**不直接依赖** `protocompile` 类型
- `protocompile` 只在输入层 / adapter 层出现
- 后续若更换解析后端，规则层和大部分测试无需重写

## 4. T003：规则执行框架与 Diagnostic 结构

## 4.1 不建议直接复用 `doctor.Check`

当前仓库已有：

- `doctor.Check`
- `doctor.Report`
- `SeverityError / SeverityWarn`

这些设计很适合“环境检查 / 项目检查”，但不完全适合 proto lint。主要区别是：

- `doctor` 的结果里同时有“通过项”和“失败项”
- proto lint 更自然的输出是“只输出命中的问题”

因此推荐：

- **不要直接复用 `doctor.Check` 作为内部主模型**
- 但可以**对齐字段命名和 severity 语义**，便于未来适配到 `doctor`

## 4.2 推荐的 Diagnostic 结构

建议内部主结构至少包含：

- `RuleID`
- `Level`
- `Phase`
- `File`
- `Line`
- `Column`
- `Service`
- `RPC`
- `Message`
- `Field`
- `Summary`
- `Hint`

可选扩展字段：

- `Path`
- `Annotation`
- `Details`

### Level 建议

建议沿用：

- `error`
- `warning`

不要在第一版引入更多级别。

## 4.3 规则接口建议

第一版建议规则接口保持简单：

- 每条规则有稳定 metadata
- 每条规则接受统一模型输入
- 每条规则返回若干 diagnostics

推荐能力边界：

- 规则可独立执行
- 规则可批量注册执行
- 可按 `phase` / `level` 做过滤

不建议第一版就做：

- 自动修复
- 规则依赖图
- 复杂多阶段 pipeline

## 4.4 规则注册表建议

建议注册表至少支持：

- 列出全部规则 metadata
- 按 `phase1` / `phase2` 过滤
- 按 `rule_id` 精确执行
- 批量执行并返回汇总 diagnostics

这样后续无论是：

- 单测某条规则
- CLI 只跑第一批规则
- doctor 嵌入 proto lint

都更容易落地。

## 4.5 结果包装建议

建议区分两层：

### 内部层

- `[]Diagnostic`

### 对外层

如后续进入 CLI/JSON，可再包装成：

- `ok`
- `diagnostics`
- `summary`
- `rulesRun`
- `filesScanned`

这样可以保持内部规则引擎简洁，同时兼容外部消费。

## 5. 推荐包布局

本文不强制最终落点，但建议至少按职责拆成：

- `internal/protolint/loader`
- `internal/protolint/model`
- `internal/protolint/rules`
- `internal/protolint/diag`
- `internal/protolint/run`

其中：

- `loader` 负责 `protocompile` 接入
- `model` 负责统一索引
- `rules` 负责 `PIO*` 规则实现
- `diag` 负责 diagnostic 类型
- `run` 负责注册与执行

如果后续觉得包太碎，也可以先合并成：

- `internal/protolint`
- `internal/protolint/rules`

第一版关键不是目录多精细，而是**职责边界清楚**。

## 6. 测试建议

## 6.1 T001 测试

应先验证：

- 能解析 Hertz 示例 proto
- 能解析 Kitex 示例 proto
- 能识别 `api.get`、`api.path`、`api.body`
- 能拿到 source 位置信息

## 6.2 T002 测试

应验证统一模型能稳定给出：

- rpc 输入 / 输出 message
- path 参数集合
- field binding 集合
- response 顶层字段集合

## 6.3 T003 测试

应验证：

- 单规则执行
- 多规则执行
- `phase` 过滤
- `level` 过滤
- diagnostic 字段完整性

## 7. 最终建议

对 `T001 ~ T003`，当前推荐的落地路线是：

1. **解析后端选择：`bufbuild/protocompile`**
2. **统一模型层隔离底层解析库**
3. **规则层基于统一模型执行，而不是直接操作 descriptor**
4. **diagnostic 结构对齐现有 doctor severity 风格，但不直接复用 `doctor.Check`**

按这个路线继续推进，后续实现 `PIO101 ~ PIO301` 时，风险最低，结构也最稳。

## 8. 相关文档

- [Proto 输入输出参数校验方案](proto-io-validation.zh-CN.md)
- [Proto 输入输出 lint / 校验规则清单](proto-io-lint-rules.zh-CN.md)
- [Proto 输入输出 lint 实现任务拆分](proto-io-implementation.zh-CN.md)