# Proto 输入输出 lint 实现任务拆分

本文把 [Proto 输入输出 lint / 校验规则清单](proto-io-lint-rules.zh-CN.md) 继续拆解为面向开发的实现任务，目标是让后续工作可以按模块、阶段和测试范围直接落地，而不是继续停留在规则描述层。

## 1. 目标

本文只关注实现层面的问题：

1. lint / 校验器的输入模型应如何组织
2. 规则执行器应如何分层
3. `phase1` 规则如何拆成独立开发任务
4. 测试样例、输出结构和集成顺序应如何安排

本文不重新定义规则本身；规则定义、Rule ID、级别与 phase 以 `proto-io-lint-rules.zh-CN.md` 为准。

## 2. 总体实现思路

建议把实现拆成四层：

1. **Proto 输入层**
   - 读取 `.proto` 文件
   - 构建 descriptor / AST / source info
   - 提取 service、rpc、message、field、annotation 等基础信息
2. **归一化模型层**
   - 把底层 descriptor 转成规则更容易消费的统一结构
   - 提供 Hertz path 参数、field binding、顶层 I/O 等常用派生信息
3. **规则执行层**
   - 注册规则
   - 遍历统一模型
   - 产出标准化 diagnostic
4. **集成与输出层**
   - 对接 CLI / JSON / MCP（如后续需要）
   - 控制只跑 `phase1`、是否包含 warning 等行为

第一版建议先完成前 3 层，并把输出层限定为内部库 + 测试，不急于先定义对外命令形态。

## 3. 模块切分建议

本文不强制最终包名，但建议至少拆出以下职责边界。

### 3.1 Proto 输入层

职责：

- 加载一个或多个 `.proto` 文件
- 能解析 import、service、rpc、message、field、option、extension
- 能拿到足够的 source 位置信息，便于 diagnostic 定位

建议优先完成：

- 单文件解析
- 项目内 import 解析
- Hertz `api.proto` / `openapi/annotations.proto` 扩展识别
- Kitex / 通用 proto message 基础结构识别

当前仓库尚未引入现成的 proto 解析依赖，因此第一项实现任务应先做“解析后端选择”。

### 3.2 归一化模型层

建议把底层解析结果归一化为更稳定的中间结构，例如：

- `ProtoFile`
- `ProtoService`
- `ProtoRPC`
- `ProtoMessage`
- `ProtoField`
- `HTTPBinding`
- `DiagnosticTarget`

建议至少提供以下派生信息：

- rpc 的输入/输出 message 名
- 顶层 request / response message
- Hertz method annotation
- Hertz 路径字符串
- 路径中的 path 参数集合
- request 字段的 binding 集合
- response 字段是否误用了 request binding
- message 顶层字段名集合

### 3.3 规则执行层

建议规则执行层至少提供：

- 规则元数据：`rule_id`、`level`、`phase`
- 统一执行入口
- 规则注册表
- 通用 helper（如 path 参数提取、binding 枚举、transport 字段集合判断）

建议每条规则保持“单一职责”，不要把多个规则揉成一个大检查函数。

### 3.4 诊断输出层

建议直接对齐 `proto-io-lint-rules.zh-CN.md` 中定义的最小字段：

- `rule_id`
- `level`
- `phase`
- `file`
- `service`
- `rpc`
- `message`
- `field`
- `summary`

可选字段：

- `path`
- `annotation`
- `details`
- `suggestion`

第一版建议保证输出结构稳定，再讨论 CLI 文本排版。

## 4. 第一阶段必须先做的基础任务

以下任务不直接对应某条 lint 规则，但属于所有规则的前置依赖。

### T001：确定 proto 解析后端

目标：选定一套能支持以下能力的解析方案：

- 读取 `.proto` 源文件
- 处理 import
- 识别自定义 extension / annotation
- 提供足够的位置信息

验收标准：

- 能正确解析 Hertz 示例 proto
- 能正确解析 Kitex 示例 proto
- 能稳定读取 `api.*` 与 OpenAPI annotation

说明：

- 这是唯一一个必须先拍板的底层实现选择
- 在未选定解析后端前，不建议贸然实现规则逻辑

### T002：建立统一 Proto Index / Model

目标：把底层 descriptor 或 AST 归一化成规则可直接消费的中间结构。

最小需要支持：

- file → service → rpc → message → field 关系
- rpc 输入 / 输出 message 名
- 顶层 I/O message 引用
- field annotation 集合
- message 顶层字段集合

验收标准：

- 能在测试中稳定读取 Hertz / Kitex 示例 proto 的全部 rpc 和 message 信息

### T003：建立 Diagnostic 结构与规则注册表

目标：提供规则执行框架，而不是让每条规则各自输出零散字符串。

最小需要支持：

- 单条规则独立执行
- 多规则批量执行
- 统一 diagnostic 输出
- 根据 level / phase 过滤规则

验收标准：

- 任意一条规则都能通过统一入口执行并返回结构化结果

## 5. Phase 1 规则实现任务拆分

以下任务直接对齐 `phase1` 规则，建议逐条拆成独立开发项。

### T101：实现 `PIO101` + `PIO102`

覆盖规则：

- `PIO101`
- `PIO102`

实现内容：

- 检查 rpc 输入是否为 `<Method>Req`
- 检查 rpc 输出是否为 `<Method>Resp`
- 输出结构化 diagnostic

测试样例：

- 正常 `Ping(PingReq) returns (PingResp)`
- 错误 `Ping(PingRequest) returns (PingResp)`
- 错误 `GetUser(CommonReq) returns (GetUserResp)`

### T102：实现 `PIO103`

覆盖规则：

- `PIO103`

实现内容：

- 检查顶层输入 / 输出是否直接为 `google.protobuf.Any`
- 检查顶层输入 / 输出是否直接为 `google.protobuf.Struct`
- 检查顶层输入 / 输出是否直接为 `google.protobuf.Value`

测试样例：

- 顶层 output 为 `Struct`
- 顶层 input 为 `Any`
- 嵌套字段使用动态结构但顶层未命中（第一版不报）

### T201：实现 `PIO201`

覆盖规则：

- `PIO201`

实现内容：

- 识别 Hertz rpc 的 HTTP method annotation
- 检查 method annotation 数量必须等于 1

测试样例：

- 仅有 `api.get`
- 同时声明 `api.get` + `api.post`
- 未声明任何 method annotation

### T202：实现 `PIO202`

覆盖规则：

- `PIO202`

实现内容：

- 解析 Hertz 路径中的 path 参数
- 收集 request 中 `(api.path)` 字段
- 做双向一致性检查

测试样例：

- `/users/:id` 与 `id[(api.path)="id"]`
- 路径中有 `id` 但 request 无字段
- request 有 `(api.path)="id"` 但路径无 `:id`

### T203：实现 `PIO203`

覆盖规则：

- `PIO203`

实现内容：

- 在 method 为 `GET/DELETE/HEAD` 时，扫描 request 字段是否使用 `(api.body)` 或 `(api.raw_body)`

测试样例：

- GET + `api.query`（通过）
- GET + `api.body`（报错）
- DELETE + `api.raw_body`（报错）

### T204：实现 `PIO204`

覆盖规则：

- `PIO204`

实现内容：

- 为每个字段收集 binding annotation 集合
- 当集合大小大于 1 时输出错误

测试样例：

- `api.query` + `api.path`
- `api.body` + `api.header`
- 单一 binding（通过）

### T205：实现 `PIO205`

覆盖规则：

- `PIO205`

实现内容：

- 按 request message 统计 `(api.raw_body)` 字段数
- 大于 1 时输出错误

测试样例：

- 0 个 raw_body
- 1 个 raw_body
- 2 个 raw_body

### T206：实现 `PIO206`

覆盖规则：

- `PIO206`

实现内容：

- 扫描 response message 中的字段 annotation
- 命中请求侧 binding annotation 时报错

测试样例：

- response 中字段含 `api.body`
- response 中字段含 `api.query`
- response 中仅有 OpenAPI property（通过）

### T301：实现 `PIO301`

覆盖规则：

- `PIO301`

实现内容：

- 检查 Kitex response 顶层字段名集合
- 命中 transport envelope 字段模式时报错

测试样例：

- 顶层字段包含 `code`、`msg`、`success`
- 顶层字段包含 `code`、`message`
- 普通业务响应（通过）

说明：

- 第一版建议先使用严格的字段名集合判断
- 复杂启发式留到后续阶段

## 6. 测试组织建议

### 6.1 测试分层

建议至少分三层测试：

1. **解析层测试**
   - 验证 proto 输入层能读取 service/rpc/message/field/annotation
2. **规则单测**
   - 每条规则用最小 proto fixture 验证命中与不命中
3. **规则集集成测试**
   - 对一个 proto 文件同时运行多条规则，验证输出集合稳定

### 6.2 测试样例组织

建议为 proto lint 单独建立测试目录，例如：

- `internal/.../testdata/proto-lint/valid/...`
- `internal/.../testdata/proto-lint/invalid/...`

样例应至少覆盖：

- Hertz 正常示例
- Hertz method/path/body 冲突示例
- Kitex transport envelope 反例
- 动态结构顶层 I/O 反例

### 6.3 输出断言建议

建议测试断言至少包含：

- 命中规则 ID
- 命中 level
- 命中 rpc / message / field
- 诊断摘要包含核心问题信息

第一版不强依赖复杂 golden 文本，可优先断言结构化字段。

## 7. 集成顺序建议

建议按以下顺序集成：

### Phase A：内部库

- 完成输入层、模型层、规则层
- 提供一个内部入口，接收 proto 文件列表并返回 diagnostics

### Phase B：测试与稳定性

- 完成 `phase1` 规则单测与集成测试
- 固化 diagnostic 输出字段

### Phase C：对外包装

- 再决定是否需要 CLI 子命令、doctor 子命令扩展或 JSON 输出接口

说明：

- 不建议在解析层尚未稳定时先定义最终 CLI 命令名
- 优先保证规则库本身稳定，再讨论暴露方式

## 8. 第二阶段已落地的最小实现

在 `phase1` 稳定后，当前已经补上以下一批 `phase2` warning 规则的最小实现：

- `PIO111`：`google.protobuf.Empty` 提示
- `PIO112`：泛化顶层 message 提示
- `PIO113`：request 字段数量阈值提示
- `PIO211`：Hertz request 未绑定字段提示
- `PIO212`：OpenAPI 元信息缺失提示
- `PIO302`：列表 / 搜索型接口分页提示
- `PIO303`：Kitex “万能请求对象”提示
- `PIO401`：分页类字段缺少 PGV 约束提示

这些规则仍然带一定启发式判断，但已经具备可运行的保守版本：

- 默认以 `warning` 进入 `protolint` / `doctor` / MCP 的 `diagnostics`
- warning-only 不会让 `ok=false`
- 当前实现优先使用顶层 message / field 名称、binding、OpenAPI option、PGV range option 等稳定静态信号

后续若继续演进，可围绕真实 proto 样本进一步收敛 `PIO212` / `PIO303` / `PIO401` 等规则的误报边界。

## 9. 推荐的开发顺序

如果按最小可交付闭环推进，建议按如下顺序实施：

1. `T001` 解析后端选择
2. `T002` 统一模型
3. `T003` 规则注册表与 diagnostic 结构
4. `T101` / `T102` / `T201`
5. `T202` / `T203` / `T204` / `T205` / `T206`
6. `T301`
7. 解析层测试 + 规则单测 + 规则集集成测试
8. 再评估是否需要 CLI / JSON / MCP 包装

## 10. 相关文档

- [Proto 输入输出参数校验方案](proto-io-validation.zh-CN.md)
- [Proto 输入输出 lint / 校验规则清单](proto-io-lint-rules.zh-CN.md)
- [Proto 输入输出 lint 技术设计（T001 ~ T003）](proto-io-tech-design.zh-CN.md)
- [Hertz 模板设计](../internal/assets/_data/docs/hertz/design-doc.zh-CN.md)
- [Kitex 模板设计](../internal/assets/_data/docs/kitex/design-doc.zh-CN.md)

## 11. 结论

本文把 Proto 输入输出 lint 的下一步工作收敛为一份可直接拆任务的实现文档：

- 先完成解析、统一模型和规则执行框架
- 再逐条实现 `phase1` 规则
- 最后再考虑对外包装与 `phase2` 启发式规则

后续如进入开发阶段，可直接以 `T001` ～ `T301` 为基础拆分任务与安排测试覆盖。