# Hertz Code Template Export & Apply — Design Spec

## Problem

Kitex 项目通过 `-template-dir` + 多个 `.yaml` 模板文件控制代码生成，
用户改完模板后新建服务自动拿到定制代码。

Hertz 目前只有 `layout.yaml` / `package.yaml`（控制目录和包名），
没有对应的代码模板机制。用户在现有 Hertz 项目上做的架构改动
（新增中间件、配置结构、分层约定）无法一键复用到新项目。

## Goal

让 Hertz 项目获得与 Kitex 同等级的代码模板能力：

1. **`ncgo export templates`** — 从成熟 Hertz 项目提取模板
2. **`ncgo new --kind hertz`** — 新建时自动应用模板（`hz new` 后 overlay）

## Scope

**纳入模板（ncgo 管理的文件）：**

- `main.go`
- `conf/dev/conf.yaml`
- `internal/base/conf/conf.go`
- `internal/base/data/*.go`
- `internal/base/server/server.go`
- `internal/handler/<service>/*.go`（按 service 循环）
- `internal/usecase/<service>/*.go`（按 service 循环）
- `internal/repository/<service>/*.go`（按 service 循环）
- `internal/router/<service>/*.go`（按 service 循环）
- `internal/pkg/interceptor/*.go`
- `internal/pkg/rpcerror/*.go`
- `internal/pkg/i18n/*.go`
- `internal/base/logging/*.go`（如果已添加 logging infra）
- `Makefile`

**排除（仅 protobuf 生成的 Go 代码）：**

- `internal/pb/`（protobuf Go 代码，由 `hz new` / `hz update` 生成，随 proto 自动刷新）

## Architecture

### 1. Template Format

复用 Kitex 的 `.yaml` 模板格式，保持一致性：

```yaml
# 说明注释，写入模板时保留
path: internal/handler/{{ToLower .ServiceName}}/handler.go
update_behavior:
  type: skip          # skip = 不覆盖已有用户文件, cover = 强制覆盖
loop_service: false   # true 时对 proto 中每个 service 生成一份
body: |-
  // 模板内容，含 Go template 变量...
```

**可用变量：**

| 变量 | 说明 | 示例 |
|------|------|------|
| `{{.Module}}` | Go module path | `github.com/acme/user-api` |
| `{{.ServiceName}}` | 服务名（PascalCase） | `UserApi` |
| `{{.ServiceInfo.*}}` | Kitex 风格 ServiceInfo 对象 | 供 `{{.ServiceInfo.Methods}}` 循环 |
| `{{.WithDatabase}}` | 是否启用数据库 | `true` / `false` |
| `{{.Infra}}` | 基础设施列表 | `["redis"]` |

**可用函数：** `ToLower`, `ToUpper`, `LowerFirst`, `exportName`（与 Kitex 一致）

### 2. Export Flow: `ncgo export templates`

```
输入：项目根目录（默认当前目录）
输出：<root>/template/hertz-template/
```

**步骤：**

1. 读取 `.ncgo/manifest.yaml`，确认是 Hertz mono 项目
2. 读取 proto IDL，解析其中的 `service` 定义
3. 按 scope 规则扫描项目中的 `.go` 文件
4. 对每个文件：
   a. 将 `module path` 替换为 `{{.Module}}`
   b. 将服务名标识符替换为 `{{.ServiceName}}` / `{{.ServiceInfo.*}}`
   c. 生成 `path`、`update_behavior`、`loop_service` 字段
   d. 写入 `.yaml` 到 `template/hertz-template/`
5. 生成 `hertz-template/makefile.yaml` 从 `Makefile`

**文件 → YAML 映射规则：**

| 文件路径模式 | `update_behavior` | `loop_service` |
|---|---|---|
| `main.go` | cover | false |
| `conf/dev/conf.yaml` | skip | false |
| `internal/base/conf/conf.go` | cover | false |
| `internal/base/data/*.go` | cover | false |
| `internal/base/server/server.go` | cover | false |
| `internal/handler/<svc>/*.go` | skip | true |
| `internal/usecase/<svc>/*.go` | skip | true |
| `internal/repository/<svc>/*.go` | skip | true |
| `internal/router/<svc>/*.go` | cover | true |
| `internal/pkg/**/*.go` | cover | false |

### 3. Apply Flow: `ncgo new --kind hertz`

当前流程：
```
manifest → IDL → hz new → add infra → done
```

改造后：
```
manifest → IDL → hz new → apply hertz-templates → add infra → done
```

**应用时机：** 在 `hz new` 成功之后、`addSelectedInfra` 之前。

**应用逻辑：**

1. 检查 `template/hertz-template/*.yaml` 是否存在
2. 遍历每个 yaml：
   a. 解析 `path`（渲染 template 变量）
   b. 检查 `update_behavior`：`skip` 时若目标已存在则跳过
   c. 渲染 `body`（注入 `{{.Module}}`, `{{.ServiceName}}` 等）
   d. 写入目标文件
3. 如果 `loop_service: true`，对 proto 中每个 service 渲染一份

### 4. Template Variable Substitution

导出时的变量替换策略（Go 源码 → template 变量）：

**模块路径替换：**
```
"github.com/acme/user-api/internal/base/conf" → "{{.Module}}/internal/base/conf"
import "github.com/acme/user-api/internal/pkg/interceptor" → import "{{.Module}}/internal/pkg/interceptor"
```

**服务名替换：**
```
UserApiImpl → {{.ServiceName}}Impl
userapi → {{ToLower .ServiceName}}
userapiHandler → {{ToLower .ServiceName}}handler
```

**ServiceInfo 变量（需要 proto 解析）：**

在 **export** 阶段：从 proto IDL 文件中解析 `service` 定义，提取
method name / arg types / return types，构建 `ServiceInfo` 结构体。
这是 export 独有的步骤——Kitex 的 ServiceInfo 由 kitex tool 在生成时注入，
而 Hertz export 需要自己做 proto 解析。

在 **apply** 阶段：proto 已被 `hz new` 消费，直接从 manifest 中已有的
`service` 信息重建 ServiceInfo，用于渲染 `loop_service: true` 的模板。

```
{{range .ServiceInfo.Methods}} → 循环 proto 中的 rpc 方法
{{.Name}} → 方法名
{{.Args}} → 参数列表
{{.Resp}} → 返回类型
```

### 4a. `loop_service` 路径参数化

导出时，如果文件路径包含服务名（如 `internal/handler/userapi/handler.go`）：

1. 识别路径中哪个路径段是服务名（通过 manifest `service.name` 反推）
2. 将该段替换为 `{{ToLower .ServiceName}}`
3. 设置 `loop_service: true`

### 5. Implementation Plan Files

需要新增/修改的文件：

| 文件 | 变更类型 | 说明 |
|------|----------|------|
| `internal/cli/export.go` | 新增 | `ncgo export templates` 命令 |
| `internal/scaffold/hertz/template.go` | 新增 | Hertz 模板提取和应用核心逻辑 |
| `internal/scaffold/hertz/template_test.go` | 新增 | 单元测试 |
| `internal/scaffold/mono/mono.go` | 修改 | `Generate()` 中添加模板应用步骤 |
| `internal/scaffold/mono/files.go` | 修改 | 添加 `hertz-template/` 到模板写入逻辑 |
| `internal/assets/_data/hertz/hertz-template/` | 新增 | 内置默认 Hertz 模板（可选） |

## Risks & Mitigations

| 风险 | 缓解 |
|------|------|
| 变量替换遗漏（module path 出现在非常规位置） | 用 AST 解析 import + 字符串扫描双保险 |
| 模板覆盖用户手写代码 | `update_behavior: skip` 保护已存在的文件 |
| proto 解析失败导致 `loop_service` 无法工作 | 降级为单文件输出，不报错 |
| 与 Kitex 模板格式不兼容 | 共用同一个 YAML 解析/渲染引擎 |

## Testing

1. **单元测试**：`template_test.go` 验证 Go→YAML 转换正确性
2. **集成测试**：`export_test.go` 验证端到端 export 流程
3. **Golden 测试**：固定输入项目，验证输出模板快照
4. **Smoke 测试**：`ncgo new` + `export templates` + `ncgo new` 循环验证
