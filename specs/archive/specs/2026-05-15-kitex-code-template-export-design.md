# Kitex Code Template Export — Design Spec

## Problem

Kitex 已有 `-template-dir` + `.yaml` 模板的代码生成能力，
但**缺少从成熟项目反向提取模板的机制**。
用户在现有 Kitex 项目上做的架构改动无法一键导出为可复用的模板。

## Goal

**`ncgo export templates`** — 从成熟 Kitex 项目提取模板到 `template/kitex-template/`

## Context

Kitex 已有模板体系（`internal/assets/_data/kitex/kitex-template/*.yaml`）：
- 每个 yaml 包含 `path`、`update_behavior`、`loop_service`、`body` 字段
- `body` 含 Go template 变量（`{{.Module}}`、`{{.ServiceInfo.*}}` 等）
- 通过 `kitex -template-dir template/kitex-template` 应用

**本设计只解决 export 反向提取，不修改 Kitex 已有的 apply 机制。**

## Scope

**纳入模板的文件：**

- `main.go`
- `conf/dev/conf.yaml`
- `internal/base/conf/conf.go`
- `internal/base/data/data.go`
- `internal/base/server/server.go`
- `internal/handler/<service>/*.go`（按 service 循环）
- `internal/usecase/<service>/*.go`（按 service 循环）
- `internal/repository/<service>/*.go`（按 service 循环）
- `pkg/client/<service>/*.go`（按 service 循环）
- `internal/pkg/interceptor/*.go`
- `internal/pkg/rpcerror/*.go`
- `internal/base/middleware/*.go`（如已添加 rate-limit 等）
- `internal/base/release/*.go`（如已添加 canary）
- `internal/base/logging/*.go`（如已添加 logging）
- `Makefile`

**排除：**

- `internal/pb/`（protobuf Go 代码）
- `kitex_gen/`（Kitex RPC stub 代码，由 `kitex` 工具生成）

## Architecture

### 1. Export Flow: `ncgo export templates`

```
输入：项目根目录（默认当前目录）
输出：<root>/template/kitex-template/
```

**步骤：**

1. 读取 `.ncgo/manifest.yaml`，确认是 Kitex mono 项目
2. 读取 proto IDL，解析其中的 `service` 定义
3. 按 scope 规则扫描项目中的 `.go` / `.yaml` 文件
4. 对每个文件：
   a. 将 `module path` 替换为 `{{.Module}}`
   b. 将服务名标识符替换为 `{{.ServiceName}}` / `{{.ServiceInfo.*}}`
   c. 生成 `path`、`update_behavior`、`loop_service` 字段
   d. 写入 `.yaml` 到 `template/kitex-template/`
5. 生成 `kitex-template/makefile.yaml` 从 `Makefile`

**文件 → YAML 映射规则：**

| 文件路径模式 | `update_behavior` | `loop_service` |
|---|---|---|
| `main.go` | cover | false |
| `conf/dev/conf.yaml` | skip | false |
| `internal/base/conf/conf.go` | cover | false |
| `internal/base/data/data.go` | cover | false |
| `internal/base/server/server.go` | cover | false |
| `internal/handler/<svc>/*.go` | skip | true |
| `internal/usecase/<svc>/*.go` | skip | true |
| `internal/repository/<svc>/*.go` | skip | true |
| `pkg/client/<svc>/*.go` | cover | true |
| `internal/pkg/**/*.go` | cover | false |
| `internal/base/middleware/*.go` | cover | false |
| `internal/base/release/*.go` | cover | false |
| `internal/base/logging/*.go` | cover | false |

### 2. Template Variable Substitution

导出时的变量替换策略（Go 源码 → template 变量）：

**模块路径替换：**
```
"github.com/acme/user-rpc/internal/base/conf" → "{{.Module}}/internal/base/conf"
import "github.com/acme/user-rpc/internal/pkg/interceptor" → import "{{.Module}}/internal/pkg/interceptor"
```

**服务名替换：**
```
UserRpcImpl → {{.ServiceName}}Impl
userrpc → {{ToLower .ServiceName}}
```

**ServiceInfo 变量（需要 proto 解析）：**

从 proto IDL 文件中解析 `service` 定义，提取
method name / arg types / return types，构建 `ServiceInfo` 结构体。

```
{{range .ServiceInfo.Methods}} → 循环 proto 中的 rpc 方法
{{.Name}} → 方法名
{{.Args}} → 参数列表
{{.Resp}} → 返回类型
{{.ServiceInfo.ImportPath}} → Go import 路径
{{.ServiceInfo.PkgRefName}} → 包引用名
```

### 2a. `loop_service` 路径参数化

导出时，如果文件路径包含服务名（如 `internal/handler/userrpc/handler.go`）：

1. 识别路径中哪个路径段是服务名（通过 manifest `service.name` 反推）
2. 将该段替换为 `{{ToLower .ServiceName}}`
3. 设置 `loop_service: true`

### 3. 与现有 Kitex 模板的关系

export 产出的 YAML 格式与现有 `internal/assets/_data/kitex/kitex-template/` 中的模板**完全一致**。

导出的模板可直接用于：
- `kitex -template-dir template/kitex-template` 命令行参数
- `ncgo new --kind kitex`（需手动替换内置模板，或由后续自动检测机制处理）

### 4. Implementation Plan Files

需要新增/修改的文件：

| 文件 | 变更类型 | 说明 |
|------|----------|------|
| `internal/cli/export.go` | 新增 | `ncgo export templates` 命令 |
| `internal/scaffold/template/export.go` | 新增 | 模板提取核心逻辑（框架无关） |
| `internal/scaffold/template/export_test.go` | 新增 | 单元测试 |
| `internal/scaffold/mono/files.go` | 修改 | 添加 Kitex export 路径识别 |

## Risks & Mitigations

| 风险 | 缓解 |
|------|------|
| 变量替换遗漏（module path 出现在非常规位置） | 用 AST 解析 import + 字符串扫描双保险 |
| 模板覆盖用户手写代码 | export 不修改源文件，只输出新模板 |
| proto 解析失败导致 `loop_service` 无法工作 | 降级为单文件输出，不报错 |
| export 产出与内置模板冲突 | export 输出到 `template/kitex-template/`，不影响内置模板 |

## Testing

1. **单元测试**：`export_test.go` 验证 Go→YAML 转换正确性
2. **集成测试**：端到端 export 流程
3. **Golden 测试**：固定输入项目，验证输出模板快照
4. **Smoke 测试**：`ncgo new --kind kitex` → 开发修改 → `export templates` → `kitex -template-dir` 验证一致性
