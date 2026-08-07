# 017 — 生成项目适配 go-tools v0.1.0（修通编译 + 补全缺口）

- 状态：重规划（v2，基准 = origin/main）
- 日期：2026-07-23
- 关联：Issue #6；工作流 `wf-2026-07-22-001`
- 取代：本文件 v1（基于旧单体 layout.yaml 的「保留 scaffold 码重划分到 10100–10499」路线已废弃）

## 1. 背景与转向

`go-tools` v0.1.0 完成三库拆分（go-common / go-auth / go-middleware / go-framework，DAG）。**origin/main 已合并一套 hertz-template 重构 + go-tools 深度集成（PR #3/#4/#5）**：

- hertz 模板重构为 `hertz-template/`（12 个 per-file yaml）+ 瘦身 `layout.yaml`。
- 生成项目的核心基础设施直接采用 go-tools（**深度集成**）：
  - 响应层 = `go-framework/hertz.Responder`（`RespondFrom(c).Success/Error`，HTTP 状态由 `goerror.HTTPStatus` 派生）
  - 配置 = `go-framework/config` + `config/hertz`
  - 日志 = `go-common/log`
  - 数据层 = `go-middleware/db`（WithDatabase 时）
  - 错误码 = re-export `go-framework/error` 框架码
  - kitex = `go-framework/kitex/rpcerror.OopsStatusAdapter`

但 origin/main 的集成**带有让生成项目编译不过的 bug，且多处缺口未补**。本 PR 的目标转为：**修通编译 + 补全缺口**（不再自创新的码段方案）。

> **bug 不在 go-tools，在 ncgo 模板**：go-tools 库本身结构正确。是模板引用错了包/引用了不存在的常量（见 §3）。

## 2. 已确认的决策

| 维度 | 决策 |
|------|------|
| 集成深度 | **深度集成**（沿用 origin/main：生成项目 = go-tools 之上的薄业务层） |
| 基准 | 以 origin/main 为基准，修通并补全其集成；**不**自创码段方案 |
| 向后兼容 | 干净切换（沿用 v1 决策；ncgo 模板追踪 go-tools，旧生成项目自行迁移） |
| CodePermissionDenied | go-tools v0.1.0 无此常量 → **映射到 `go-framework/error.CodeAuthFailed`（10002）**，不发明新码、不改 go-tools |
| go.mod require | hertz 静态 go.mod 模板**显式 require** `go-common v0.1.0` + `go-framework v0.1.0`（钉死版本）；`go-middleware` 因依赖 WithDatabase 条件由 `go mod tidy` 补；kitex go.mod 由工具链生成，用「预写 go.mod 利用 initGoMod 短路」方式钉住（沿用旧分支已验证做法） |
| 无 go-framework 等价码的硬编码 | oops→goerror 机制迁移；语义匹配者改用 go-framework 码（10000→CodeSystem、10308→CodeConfigInvalid、10301→CodeRPCUnavailable）；无匹配者用项目段码（≥40100）并以 `goerror.RegisterHTTPStatuses` 注册 HTTP 状态 |

## 3. bug 分析（生成项目编译不过的根因）

go-common/error 是**纯机制包**（Code/In/Extract/HTTPStatus/RegisterHTTPStatuses + 码段边界常量），**不持有**模块具体错误码；框架码（CodeSystem 等）定义在 **go-framework/error**。

| 文件 | 错误 | 修复 |
|------|------|------|
| `hertz/hertz-template/errcode_go.yaml` | `import goerror "go-common/error"` 后引用 `goerror.CodeSystem/CodeParamInvalid/CodeAuthFailed/CodeConfigInvalid/CodeRPCTimeout/CodeRPCUnavailable`——这些常量在 go-common/error 中**不存在** | 改 `import frameworkerror "go-framework/error"`，全部 `goerror.Xxx` → `frameworkerror.Xxx` |
| `kitex/kitex-template/rpcerror.yaml` | `CodeInternalError=goerror.CodeSystem`、`CodeRPCTimeout=goerror.CodeRPCTimeout`、`CodeConfigInvalid=goerror.CodeConfigInvalid`（包引错）；`CodePermissionDenied=goerror.CodePermissionDenied`（go-tools **无此常量**） | 加 `frameworkerror "go-framework/error"`；前三者 → `frameworkerror.*`；`CodePermissionDenied` → `frameworkerror.CodeAuthFailed`；构造 `oops.In` → `goerror.In`，移除 `samber/oops` import |

这些 bug 未被发现的原因：**ncgo golden 测试是纯文本比对，不编译生成代码**，CI 也无「生成项目可编译」检查（本 PR 补上，见 §6）。

## 4. 缺口清单（origin/main 未完成项）

| 级别 | 缺口 | 位置 |
|------|------|------|
| CRITICAL | errcode/rpcerror 引错包 + CodePermissionDenied 不存在（§3） | errcode_go.yaml、rpcerror.yaml + 7 个 golden 副本 |
| HIGH | go.mod `go 1.22` → 1.26.5 | hertz/layout.yaml（+ kitex 工具链生成的 go.mod） |
| HIGH | Docker 基础镜像 `golang:1.22(-alpine)` → 1.26.5 | shared/container.go×2、hertz/Dockerfile.vegeta |
| MEDIUM | go.mod 无 require 块 | hertz/layout.yaml |
| MEDIUM | optional add-on 仍 oops + 硬编码旧码（10308/10304/10306/10303）/字符串码 | hertz/optional/{redis,kafka,es,clickhouse,observability_logging}.go、kitex/optional/{registry_etcd,observability_logging}.go |
| MEDIUM | kitex client.yaml/conf.yaml 硬编码 10301/10308（oops） | kitex-template/{client,conf}.yaml |
| MEDIUM | repository 模板硬编码 `Code(10000)` | hertz/repository_go.yaml、kitex/repository.yaml |
| MEDIUM | infra goGetDeps 不含 go-common | internal/scaffold/infra/infra.go |
| LOW | 文档未记录 go-tools / 1.26.5 / Responder | README.md/.zh-CN、docs/examples*.md |
| LOW | optional 数值码未注册 HTTP 映射（→500 兜底而非 503/504） | hertz/optional/*.go |

## 5. 生成项目所需 go-tools 模块（从模板 import 反推）

- **必需**：`go-common v0.1.0`（log、error 机制）、`go-framework v0.1.0`（hertz Responder/middleware、config、error 框架码、kitex rpcerror）
- **条件**：`go-middleware v0.1.0`（WithDatabase 时 db；经 tidy 补）
- **传递**：`go-auth`（go-framework 依赖）

go.mod require（hertz 静态模板）：
```
require (
    github.com/byx-darwin/go-tools/go-common v0.1.0
    github.com/byx-darwin/go-tools/go-framework v0.1.0
)
```

## 6. 测试策略（含新增 e2e 编译检查）

- **golden（文本）**：模板改动同步 testdata 快照（`-update-golden`，精确包路径），逐提交审查 diff。
- **新增 e2e 编译检查（堵盲区）**：在 CI（或 smoke.sh / 一个 Go 测试）中加入「用 ncgo 生成一个 hertz（+kitex）项目 → `go mod tidy` → `go build ./...`」，确保生成代码可编译。需 go 1.26.5 工具链 + go-tools v0.1.0 可从 proxy 解析。这是防止 §3 类编译错误再漏网的关键。
- **验证链**：`go build ./... && go vet ./... && go test ./... -count=1 && ./scripts/smoke.sh`。

## 7. 与 Issue #6–#10 的关系（重规划后）

origin/main 已完成 hertz 深度集成（config/Responder/log/db），故原分解中 **PR2（config）、PR3 的 logging 部分基本被 origin/main 覆盖**，需重新核对/可能关闭。本 PR（#6）聚焦「修通编译 + 补全缺口」。其余 add-on（kafka/es/clickhouse 独立文件、observability、registry）的库替换仍可作后续 Issue，但需基于 origin/main 重新评估。PR 描述中标注此关系。

## 8. 风险

- **kitex go.mod 钉版本**依赖「预写 go.mod + initGoMod 短路」机制，需在 origin/main 的 mono.Generate 流程中验证可行（旧分支已验证过类似做法，但 base 已变）。
- **e2e 编译检查**需 CI 具备 go 1.26.5 + 网络（proxy 解析 go-tools）；若 CI 不具备，先落为本地/可选检查并标注。
- 模板/脚手架输出 contract-sensitive；golden diff 须逐提交审查，避免误 bless。
- optional add-on 的项目段码（≥40100）需注册 HTTP 映射，否则兜底 500。
