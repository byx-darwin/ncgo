# ncgo Proto I/O 校验系统设计

- 状态：v1
- 适用范围：`ncgo protolint` 命令和 MCP 工具
- 源文档：合并自 `docs/proto-io-*.zh-CN.md`（4 篇）

## 1. 概述

Proto I/O 是 ncgo 内置的 protobuf 文件校验系统，通过 `ncgo protolint` CLI 命令和 `ncgo_protolint` MCP 工具暴露。

核心能力：
- 对 `.proto` 文件执行结构化 lint 规则
- 支持 text / json / SARIF 三种输出格式
- 支持规则过滤、文件过滤和忽略模式
- 基于 `bufbuild/protocompile` 进行 proto 解析

## 2. 规则体系

### 2.1 规则编号

规则使用 `PIO` 前缀编号（Proto I/O），分为两个阶段：

**Phase 1（错误级别）：**
- `PIO201` — 动态 I/O 校验（dynamic_io 注解）
- `PIO202` — HTTP Body 绑定校验
- `PIO203` — HTTP Methods 校验
- `PIO204` — Kitex Envelope 校验
- `PIO205` — 多绑定校验
- `PIO206` — OpenAPI 缺失校验
- `PIO207` — 分页缺失校验
- `PIO208` — Path Params 校验
- `PIO209` — PGV Constraints 校验
- `PIO210` — PGV Pagination 校验
- `PIO211` — Request/Response 校验
- `PIO212` — Response Bindings 校验
- `PIO213` — Unbound Fields 校验
- `PIO214` — Universal Request 校验
- `PIO215` — Raw Body 校验

**Phase 2（警告级别）：**
- 字段命名、注释完整性等建议性规则

### 2.2 规则实现

规则定义在 `internal/protolint/rules.go` 中，使用注册表模式：

```go
var rules = map[string]Rule{...}
```

每个规则实现 `Check(ctx, file)` 接口，返回 `[]Diagnostic`。

## 3. 技术设计

### 3.1 解析层

使用 `bufbuild/protocompile` 作为 proto 解析器，支持：
- 增量解析
- 导入路径解析
- 源位置信息保留（用于诊断定位）

### 3.2 诊断模型

```go
type Diagnostic struct {
    Rule     string
    Message  string
    File     string
    Line     int
    Column   int
    Severity Severity // Error | Warning
}
```

### 3.3 SARIF 输出

遵循 SARIF 2.1.0 标准，`internal/protolint/sarif.go` 负责序列化。

## 4. CLI / MCP 接口

### CLI 命令

```bash
# 检查所有 proto 文件
ncgo protolint --root .

# 检查指定文件
ncgo protolint --root . --file idl/app/user.proto

# 指定规则
ncgo protolint --root . --rule PIO201 --rule PIO202

# 忽略规则
ncgo protolint --root . --ignore-rule PIO207

# 输出格式
ncgo protolint --root . --output json
ncgo protolint --root . --output sarif
```

### MCP 工具

| 工具 | 说明 |
|------|------|
| `ncgo_protolint` | 检查 proto 文件，支持 text/json/sarif 输出 |

参数：`root`（必填）、`files`、`rules`、`ignoreRules`、`ignoreFiles`、`output`

## 5. 校验策略

### 5.1 忽略机制

- `--ignore-rule`：跳过指定规则
- `--ignore-file`：跳过指定文件
- 默认不因 warning 失败（Phase 2 规则）

### 5.2 工作区支持

在 micro 工作区中，`protolint` 会递归扫描所有服务的 IDL 目录。

## 6. 测试数据

测试用例位于 `internal/protolint/testdata/`，每个子目录包含：
- 触发特定规则的 proto 文件
- 预期的诊断输出

## 7. 相关文件

- `internal/protolint/rules.go` — 规则注册表
- `internal/protolint/sarif.go` — SARIF 序列化
- `internal/protolint/load.go` — Proto 文件加载
- `schemas/` — 相关 JSON Schema

## 8. 详细参考

原始设计文档已归档至 `specs/archive/`：
- `proto-io-validation.zh-CN.md` — 校验策略
- `proto-io-lint-rules.zh-CN.md` — 规则详细清单
- `proto-io-implementation.zh-CN.md` — 实现任务拆解
- `proto-io-tech-design.zh-CN.md` — 技术设计决策
