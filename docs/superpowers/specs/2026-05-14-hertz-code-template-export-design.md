# Code Template Export & Apply — Design Spec (Hertz + Kitex)

## Problem

Kitex 项目通过 `-template-dir` + 多个 `.yaml` 模板文件控制代码生成，
用户改完模板后新建服务自动拿到定制代码。Hertz 目前只有
`layout.yaml` / `package.yaml`（控制目录和包名），没有对应的代码模板机制。

两者都缺少一个关键能力：**从成熟项目反向提取模板**。
用户在现有项目上做的架构改动（新增中间件、配置结构、分层约定）
无法一键导出为可复用的模板供新项目使用。

## Goal

1. **`ncgo export templates`** — 从成熟 Hertz 或 Kitex 项目提取模板
2. **Hertz apply** — 新建时自动应用模板（`hz new` 后 overlay）
3. **Kitex apply** — 已有（`kitex -template-dir`），无需新增，但 export 结果格式需与之兼容

## Scope

### Scope by Framework

**Hertz 纳入模板的文件：**

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

**Hertz 排除：**

- `internal/pb/`（protobuf Go 代码，由 `hz new` / `hz update` 生成）

**Kitex 纳入模板的文件：**

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

**Kitex 排除：**

- `internal/pb/`（protobuf Go 代码）
- `kitex_gen/`（Kitex RPC stub 代码，由 `kitex` 工具生成）

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

**可用函数：** `ToLower`, `ToUpper`, `LowerFirst`, `exportName`（Kitex 原生已支持，Hertz 复用）

### 2. Export Flow: `ncgo export templates`

统一命令，自动识别 Hertz 或 Kitex：

```
输入：项目根目录（默认当前目录）
输出（Hertz）：<root>/template/hertz-template/
输出（Kitex）：<root>/template/kitex-template/
```

**步骤：**

1. 读取 `.ncgo/manifest.yaml`，识别 Kind（hertz | kitex）
2. 读取 proto IDL，解析其中的 `service` 定义
3. 按对应框架的 scope 规则扫描项目中的 `.go` / `.yaml` 文件
4. 对每个文件：
   a. 将 `module path` 替换为 `{{.Module}}`
   b. 将服务名标识符替换为 `{{.ServiceName}}` / `{{.ServiceInfo.*}}`
   c. 生成 `path`、`update_behavior`、`loop_service` 字段
   d. 写入 `.yaml` 到对应输出目录
5. 生成 `makefile.yaml` 从 `Makefile`

**文件 → YAML 映射规则（Hertz）：**

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

**文件 → YAML 映射规则（Kitex）：**

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

### 3. Apply Flow

**Hertz apply**（新增）：

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

**Kitex apply**（已有，无需改动）：

Kitex 原生通过 `kitex -template-dir template/kitex-template` 消费模板。
export 产出的 YAML 格式与现有模板格式完全一致，所以导出后的模板
可以直接被 `kitex -template-dir` 使用。

唯一需要注意的是：`ncgo new --kind kitex` 在 scaffold 阶段
copy 的是内置默认模板（`internal/assets/_data/kitex/kitex-template/`），
用户导出的自定义模板需要手动放回去或建立 symlink 机制。
这是 Kitex 已支持的行为，不需要 ncgo 额外改动。

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
| `internal/cli/export.go` | 新增 | `ncgo export templates` 命令（Hertz + Kitex） |
| `internal/scaffold/template/export.go` | 新增 | 模板提取核心逻辑（框架无关） |
| `internal/scaffold/template/render.go` | 新增 | YAML 模板渲染引擎（复用 Kitex 现有引擎） |
| `internal/scaffold/template/export_test.go` | 新增 | 单元测试 |
| `internal/scaffold/mono/mono.go` | 修改 | Hertz `Generate()` 中添加模板应用步骤 |
| `internal/scaffold/mono/files.go` | 修改 | 添加 `hertz-template/` 到模板写入逻辑 |
| `internal/assets/_data/hertz/hertz-template/` | 新增 | 内置默认 Hertz 模板（可选） |

### 6. Hertz vs Kitex 模板对比

| 特性 | Kitex | Hertz |
|------|-------|-------|
| 内置模板 | `kitex-template/*.yaml`（已有） | `hertz-template/*.yaml`（新增） |
| Export 命令 | `ncgo export templates`（复用） | `ncgo export templates`（复用） |
| Apply 方式 | `kitex -template-dir`（原生） | `hz new` 后 overlay（新增） |
| Apply 时机 | scaffold 阶段 | `hz new` 之后 |
| 模板格式 | `.yaml` | `.yaml`（同一引擎） |
| `loop_service` | 是 | 是 |
| `update_behavior` | skip / cover | skip / cover |

## Risks & Mitigations

| 风险 | 缓解 |
|------|------|
| 变量替换遗漏（module path 出现在非常规位置） | 用 AST 解析 import + 字符串扫描双保险 |
| 模板覆盖用户手写代码 | `update_behavior: skip` 保护已存在的文件 |
| proto 解析失败导致 `loop_service` 无法工作 | 降级为单文件输出，不报错 |
| Hertz/Ketex 模板格式不一致 | 共用同一个 YAML 解析/渲染引擎 |
| Kitex export 产出与内置模板冲突 | export 输出到 `template/kitex-template/`，与内置模板路径相同，覆盖即替换 |
| Hertz overlay 与 hz 生成的文件冲突 | 按 scope 排除 `internal/pb/`，其余由模板决定最终内容 |

## Testing

1. **单元测试**：`export_test.go` 验证 Go→YAML 转换正确性（Hertz + Kitex）
2. **集成测试**：端到端 export 流程，验证两种框架
3. **Golden 测试**：固定输入项目，验证输出模板快照
4. **Smoke 测试**：
   - Hertz 循环：`ncgo new` → 开发修改 → `export templates` → `ncgo new`
   - Kitex 循环：`ncgo new` → 开发修改 → `export templates` → `kitex -template-dir`
