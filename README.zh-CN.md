# ncgo

[![CI](https://github.com/byx-darwin/ncgo/actions/workflows/ci.yml/badge.svg)](https://github.com/byx-darwin/ncgo/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/byx-darwin/ncgo)](https://github.com/byx-darwin/ncgo/releases)
[![Go 1.25+](https://img.shields.io/badge/Go-1.25%2B-00ADD8?logo=go)](https://go.dev/)
[![License](https://img.shields.io/github/license/byx-darwin/ncgo)](LICENSE)

**官网：** [byx-darwin.github.io/ncgo](https://byx-darwin.github.io/ncgo/)

面向 AI Agent 的 Go 微服务脚手架 CLI。`ncgo` 内置并维护 Hertz / Kitex 模板，生成项目元数据，调用上游生成器，并为 Claude、Cursor 等 Agent 渲染上下文文件。

如果你希望用一个 CLI 同时解决可复现脚手架、可选基础设施能力以及 Agent 友好的项目上下文，`ncgo` 就是为这个场景设计的。

English documentation: [README.md](README.md)。产品需求文档见 [specs/prd.md](specs/prd.md) 和 [specs/prd.zh-CN.md](specs/prd.zh-CN.md)。Agent 上下文交接见 [specs/005-context-handoff.zh-CN.md](specs/005-context-handoff.zh-CN.md)。i18n 系统设计见 [specs/004-i18n-system.zh-CN.md](specs/004-i18n-system.zh-CN.md)。i18n 详细协议/Schema/Payload 见 [specs/archive/](specs/archive/)。

**快速导航：** [安装](#安装) · [30 秒上手](#30-秒上手) · [典型使用路径](#典型使用路径) · [i18n 系统](specs/004-i18n-system.zh-CN.md) · [示例文档](docs/examples.zh-CN.md) · [贡献指南](CONTRIBUTING.zh-CN.md) · [FAQ](#faq)

## 为什么用 ncgo

- **可复现的脚手架**：把 manifest、IDL 占位和模板输入纳入版本控制
- **理解生成器工作流**：可以协调 `hz` / `kitex`，也支持 `--no-generate` 先准备后生成
- **默认对 Agent 友好**：可生成 `AGENTS.md`、`CLAUDE.md`、`.claude/generated/project-context.md`、Cursor 规则，并提供 MCP 能力
- **内置生命周期工具**：同一个 CLI 里包含 `doctor`、`upgrade` 和保守的 `extract domain`

## 核心能力

| 范围 | 你能得到什么 |
| --- | --- |
| 服务脚手架 | 支持 Mono Hertz/Kitex 脚手架，也支持带 `add rpc` / `add bff` 的 Micro 工作区 |
| 项目上下文 | 可版本化的 manifest、模板输入，以及面向 AI 的协作文件 |
| 可选基础设施 | Redis、Kafka、Elasticsearch、ClickHouse、logging、canary 等 helper |
| 生命周期工具 | 内置 `doctor`、`upgrade`、`extract domain` 与 MCP 暴露能力 |

## 适用场景

如果你符合这些场景，`ncgo` 会比较合适：

- 你在用 Hertz 或 Kitex 构建 Go 服务
- 你希望脚手架结果可复现，而不是一次性的生成器输出
- 你希望 AI Agent 更稳定地理解和操作生成后的项目

如果你更接近下面这些诉求，`ncgo` 可能不是最合适的工具：

- 你需要框架无关或非 Go 的项目生成器
- 你完全不希望工作流依赖 Hertz / Kitex 生成器
- 你期待 CLI 自动安装依赖或自动重构任意已有服务

## 当前状态

v0.5 MVP 已完成：

- Mono 脚手架：Hertz HTTP 服务、Kitex RPC 服务。
- Micro 工作区：根 `ncgo.workspace`，并支持 `add rpc` / `add bff`。
- Domain 工作流：`add domain` 与基于 anchor 的 `add method`。
- Optional infra：Redis、Kafka、Elasticsearch、ClickHouse、结构化日志、金丝雀发布 helper、Kitex-only Polaris registry。
- AI / Agent 工作流：`ai init claude`、`ai sync`、静态 `doctor`、MCP stdio server。
- 生命周期 MVP：metadata-only `upgrade --plan`、保守的 `extract domain --apply`。

暂缓但保留在路线图中：~~NATS~~、~~Mongo~~、~~MinIO~~。

## ncgo 会生成什么

| 范围 | 输出 |
| --- | --- |
| 项目元数据 | 服务内 `.ncgo/manifest.yaml`；micro 根目录 `ncgo.workspace` |
| Hertz | IDL 占位、`hz` 自定义 layout/package 输入、HTTP 服务骨架 |
| Kitex | IDL 占位、Kitex template tree、RPC 服务骨架 |
| 容器化 | 服务级 `Dockerfile` / `.dockerignore`；mono 与 micro 根目录生成 `compose.yaml` |
| Domain | `internal/usecase/<name>`、`internal/repository/<name>`、DI register 文件 |
| Infra | `internal/base/...` 下的可选 Go 文件 |
| AI 上下文 | `AGENTS.md`、`CLAUDE.md`、`.claude/generated/project-context.md`、`.cursor/rules/ncgo.mdc` |

### 生成项目构建在 go-tools v0.1.0 之上

生成的 Hertz / Kitex 项目是 [go-tools](https://github.com/byx-darwin/go-tools) v0.1.0 之上的薄业务层。生成的 `go.mod` 声明 `go 1.26.5`，并 require `go-common v0.1.0` + `go-framework v0.1.0`（`go-middleware v0.1.0` 在 `WithDatabase=true` 时由 `go mod tidy` 补齐）。

| 关注点 | go-tools 模块 |
| --- | --- |
| HTTP 响应 | `go-framework/hertz` 的 `Responder`（`RespondFrom(c).Success` / `.Error`） |
| 配置 | `go-framework/config`（+ `config/hertz`、`config/kitex`） |
| 日志 | `go-common/log` |
| 错误码 | re-export `go-framework/error` 的框架码 |
| 数据库 | `go-middleware/db`（`WithDatabase=true` 时） |
| Kitex RPC 错误 | `go-framework/kitex/rpcerror` |

**配置中的 duration 字段。** 生成项目 `conf.Config` 里所有时长类字段（例如
`rpc.request_timeout_seconds`、`database.health_check_period_seconds`、
`rate_limit.rule.window_seconds`、`redis.dial_timeout_seconds`）统一使用
`go-framework/config` 的 `config.Duration`（内部包装 `time.Duration`）。
`conf/dev/conf.yaml` 中这些字段以 duration 字符串形式填写，例如 `"30s"`、
`"5m"`、`"8ms"`，由 `time.ParseDuration` 解析；这些字段不再接受裸整数，
但字段名 / YAML key 保持不变。Redis 超时选项同样采用该格式（R-A 铺垫，
客户端接线为后续 PR）。

**错误码。** 错误用 `go-common/error` 的 `goerror.In/Code` 构造（它内部包装 `samber/oops`；不要再直接 `import "samber/oops"` 构造错误）。框架码常量来自 `go-framework/error`，由生成的 `internal/pkg/errcode` / `internal/pkg/rpcerror` 包 re-export：`CodeSystem=10000`、`CodeParamInvalid=10001`、`CodeAuthFailed=10002`、`CodeConfigInvalid=10004`、`CodeRPCUnavailable=10010`、`CodeRPCTimeout=10011`。

码段划分：Framework `10000–10499`、Middleware `20000–20699`、Auth `40000–40099`、Project `40100–59999`。**业务自定义码必须 `>= 40100`**（`goerror.ProjectCodeMin`）。

> **行为说明：** 业务码（`>= 40100`）经 `goerror.HTTPStatus` 兜底返回 **HTTP 200**——go-tools 将其视为「业务错误、RPC 调用成功」。如需非 200 响应，请用 `goerror.RegisterHTTPStatuses` 注册细粒度 HTTP 状态。

## 要求

- 构建并运行 `ncgo` CLI 本身需要 Go `1.25+`。**生成的项目需要 Go `1.26.5`**，因为它们构建在 go-tools v0.1.0 之上（生成的 `go.mod` 声明 `go 1.26.5`，服务 `Dockerfile` 使用 `golang:1.26.5`）。
- 生成 Hertz 服务时需要 `hz >= v0.9.7`（缺失时自动安装）
- 生成 Kitex 服务时需要 `kitex >= v0.16.1`（缺失时自动安装）
- Hertz 模板中的 `make swagger` 需要本机已安装 `protoc`，并且 `protoc-gen-http-swagger` 位于 `PATH`

如果你暂时只想生成 manifest、IDL 占位和模板输入，可以先使用
`--no-generate`，后续再安装生成器。

## 安装

```bash
go install github.com/byx-darwin/ncgo@latest
ncgo version
```

如果安装后找不到 `ncgo` 命令，请确认 `GOBIN` 或 `$(go env GOPATH)/bin`
已经加入 `PATH`。

如果是在本地仓库中使用，也可以直接从根目录安装：

```bash
go install .
ncgo version
```

## 30 秒上手

如果你的环境里已经有 `hz`，最短的上手路径是：

```bash
go install github.com/byx-darwin/ncgo@latest
ncgo new user-api --module github.com/acme/user-api
cd user-api
go mod tidy
make dev
```

如果你要生成 Kitex、Micro 工作区，或者希望先不跑生成器，请看下面的详细示例。

## 常用命令总览

| 命令 | 作用 |
| --- | --- |
| `ncgo new` | 创建 mono 服务或 micro 工作区 |
| `ncgo import` | 为已有的 Hertz/Kitex 项目反向生成 `.ncgo/manifest.yaml` |
| `ncgo add domain` | 生成 usecase / repository / DI register 文件 |
| `ncgo add method` | 在 ncgo anchor 标记中插入方法桩 |
| `ncgo add infra` | 添加 Redis / logging / canary 等可选基础设施 helper |
| `ncgo add rpc` / `ncgo add bff` | 在 micro 工作区中新增服务 |
| `ncgo ai init claude` | 初始化 hand-authored `.claude` starter files（`--preset minimal` 或 `--preset team`） |
| `ncgo ai sync` | 生成 `AGENTS.md`、`CLAUDE.md`、`.claude/generated/project-context.md` 与 Cursor 规则 |
| `ncgo doctor` | 检查宿主机工具、项目元数据与默认 proto 契约问题 |
| `ncgo upgrade` | 更新 ncgo/assets 元数据 |
| `ncgo extract domain` | 规划或执行 mono-to-micro 迁移 |
| `ncgo export templates` | 从已有 ncgo 项目导出代码模板 |
| `ncgo mcp serve` | 通过 MCP stdio 暴露部分 ncgo 能力 |

## 典型使用路径

| 场景 | 起步命令 | 适合什么时候 |
| --- | --- | --- |
| 单个 Hertz 服务 | `ncgo new <name> --module <module>` | 你希望最快得到一个 HTTP 服务脚手架 |
| 单个 Kitex 服务 | `ncgo new <name> --module <module> --kind kitex` | 你在做以 RPC 为主的 Kitex 服务 |
| Micro 工作区 | `ncgo new <name> --module <module> --mode micro` | 你需要在一个工作区根目录下管理多个服务 |
| 先准备、后生成 | `ncgo new ... --no-generate` | 你想先落地 manifest/模板输入，之后再执行生成器 |
| 在已有项目上继续扩展 | `ncgo add domain`、`ncgo add infra`、`ncgo ai sync` | 你已经有 ncgo 项目，只想按需逐步增强 |
| 生成项目中的 i18n 补译 | `make i18n-report`、`ncgo i18n check --mode release --output json` | 你想把 locale/status 更新、Agent 辅助补译和最终校验串成稳定流程 |
| 生成项目中的 proto 契约校验 | `ncgo protolint --root . --file idl/app/demo.proto --output json` | 你想把 Req/Resp 命名、Hertz binding、Kitex response 结构等规则纳入自动检查 |
| Rule-center Kitex 服务 | `ncgo new rule-center --module github.com/acme/rule-center --kind kitex --db postgres --preset rule-center` | 需要集中式限流规则管理服务 |
| Hertz 接入规则中心 | `ncgo new user-api --module github.com/acme/user-api --kind hertz --db postgres --rule-center-addr rule-center:8888` | Hertz 服务从远程规则中心查询限流规则 |

如果你是从 0 开始，先从上表选一条路径，再继续看下面对应的详细快速开始。

## 快速开始

### Hertz HTTP 服务

```bash
ncgo new user-api --module github.com/acme/user-api --kind hertz
cd user-api
go mod tidy
make dev
```

`--kind hertz` 是默认值，所以也可以写成：

```bash
ncgo new user-api --module github.com/acme/user-api
```

Hertz 模板默认提供 `zh-CN`、`zh-TW`、`ja-JP`、`ko-KR`、`fr-FR`、`de-DE`、`es-ES`
这些语言；默认语言与新增语言统一在生成项目的
`internal/pkg/i18n/locales/*.json` 中维护，并通过 `make i18n` 生成
`internal/pkg/i18n/catalog_gen.go`。

新的服务脚手架还会额外生成服务级 `Dockerfile`、`.dockerignore` 和
`compose.yaml`，方便本地容器构建与运行。

### Kitex RPC 服务

```bash
ncgo new user-api --module github.com/acme/user-api --kind kitex
cd user-api
make sqlc
go mod tidy
make dev
```

Kitex 服务名会被规范化为合法 proto / Go 标识符。例如 `user-api` 会生成 `idl/userapi.proto`、proto package `userapi`、service `UserApi`。

Kitex 脚手架同样会附带服务级 `Dockerfile`、`.dockerignore` 和 `compose.yaml`。

### Micro 工作区

```bash
ncgo new commerce --module github.com/acme/commerce --mode micro
cd commerce
ncgo add rpc user-rpc --root .
ncgo add bff web-bff --root .
```

根目录保存 `ncgo.workspace`，并额外生成工作区级 `compose.yaml` 与
`.pre-commit-config.yaml`；每个生成的服务保存自己的 `.ncgo/manifest.yaml`、`Dockerfile`
以及服务级 `compose.yaml`。后续执行 `ncgo add rpc` / `ncgo add bff` 时，工作区根目录
`compose.yaml` 也会自动刷新。

当你在 micro 根目录执行 `ncgo doctor --root .` 或 `ncgo protolint --root .` 时，ncgo
现在会自动遍历 `ncgo.workspace` 里登记的服务，并聚合各服务 `manifest.service.idl`
上的 proto lint 结果。

## Prepare vs Generate

默认情况下，`ncgo new` 分两步：

1. 准备确定性输入：`.ncgo/manifest.yaml`、IDL 占位、`template/` 下的自定义模板。
2. 调用上游生成器：Hertz 使用 `hz`，Kitex 使用 `kitex`。

如果只想准备输入，不调用生成器：

```bash
ncgo new user-api --module github.com/acme/user-api --kind kitex --no-generate
```

此时 ncgo 会打印之后可手动执行的生成器命令。生成器成功后会创建 `go.mod`；如果生成出的脚手架已经 import `internal/db/gen`（所有 Kitex 服务，以及 `WithDatabase=true` 的 Hertz 服务），后续步骤应先从 `make sqlc` 开始，否则从 `go mod tidy` 开始。

生成后的单体服务和 micro 工作区还会包含根目录 `.pre-commit-config.yaml`
以及 `scripts/run-go-module-checks.sh`，方便协作者启用 `pre-commit` /
`pre-push` 对一个或多个 Go module 执行统一检查。

## AI 上下文

如需先初始化 hand-authored 的 `.claude` starter files，可先运行：

```bash
ncgo ai init claude --root user-api
```

如需在 init 阶段获取机器可读的 CLI 输出，可追加 `--output json`。

如需包含 workflow starter 的 `team` preset，可运行：

```bash
ncgo ai init claude --root user-api --preset team
```

命令会提示当前 `--root` 被识别为服务根目录、micro 工作区根目录，或暂时还无法识别。

在非 dry-run 且成功初始化后，还会追加下一步建议：`ncgo ai sync --root <root> --lang en`。

```bash
ncgo ai sync --root user-api --lang zh-CN
```

如需机器可读的 CLI 输出，可追加 `--output json`。

如果是 micro 工作区根目录，也是在工作区根目录执行同样的命令：

```bash
ncgo ai sync --root commerce --lang zh-CN
```

会写入受管理文件：

- `AGENTS.md`
- `CLAUDE.md`
- `.claude/generated/project-context.md`
- `.cursor/rules/ncgo.mdc`

这些文件带有 `<!-- ncgo:managed -->` 标记。没有该标记的已有文件默认不会覆盖，除非传 `--force`。项目私有说明放在 `AGENTS.local.md`，会附加到长版上下文文件；`.claude/generated/project-context.md` 保持 deterministic。

当 `--root` 指向 micro 工作区根目录时，生成文件会基于 `ncgo.workspace`
描述工作区级事实并列出已登记服务。如需服务级上下文，请进入对应服务目录执行
`ncgo ai sync --root services/<name> --lang zh-CN`。

当 `--root` 指向某个服务目录，且该服务同时登记在上层 micro 工作区里时，
`ncgo ai sync` 仍会基于本地 `.ncgo/manifest.yaml` 生成服务级上下文，但会额外
补充 workspace membership 信息，例如父工作区名称、module、相对根路径以及已登记的
服务目录。CLI 摘要也会输出对应的 `info:` 提示，方便人和 Agent 判断该服务属于更大的工作区。

`ncgo ai init claude` 创建的 starter files 属于 hand-authored 内容，不会被后续 `ncgo ai sync` 覆盖。

## 命令参考

### Domain 与方法 anchor

```bash
ncgo add domain <name> --root .
ncgo add method device.ListThemes --root . --in usecase
```

`add method` 当前会在 `// ncgo:methods:start` 与 `// ncgo:methods:end` 之间插入无参数 `UseCase` 方法桩。

### Optional infra

```bash
ncgo add infra redis --root .
ncgo add infra logging --root .              # observability_logging alias
ncgo add infra canary --root .               # release_canary alias
ncgo add infra logging --root . --wire       # 可选：自动接入默认 server/client 模板
ncgo add infra logging --root . --wire --dry-run  # 预览写入/接线，不修改文件
ncgo add infra logging --root . --wire --dry-run --output json  # 输出机器可读 plan
ncgo add infra logging --root . --wire --plan  # --dry-run --output json 简写
ncgo add infra registry_polaris --root . --wire  # kitex only：Polaris 注册/发现 + 自动接线
ncgo add infra rate-limit --root . --wire         # kitex only：共享 ratelimit 包 + 真实中间件（shadow 优先）
```

common infra：`redis`、`kafka`、`es`、`clickhouse`、`observability_logging`（`logging` alias）、`release_canary`（`canary` alias）。Kitex-only：`registry_polaris`、`rate-limit`。

`rate-limit` 为 **kitex only**（Hertz 侧使用另一套限流设计）。它把 Kitex 模板
里 pass-through 的 `RateLimit()` 占位符改写为真实的 `endpoint.Middleware`，底层
复用与 Hertz 侧同一份 `internal/pkg/ratelimit` 包（resolver + store，单一事实
来源）。中间件接在 server chain 的 `CallerAllowlist` 之后，并通过
`// ncgo:wire:ratelimit:static-limit` 标记按需挂载静态 `server.WithLimit` 兜底。
生成 conf 默认 `mode = shadow`（只计数不拒绝），观察 shadow 日志后再切
`mode = enforce`。详见
[rate-limit-dynamic-design.zh-CN.md](internal/assets/_data/docs/hertz/rate-limit-dynamic-design.zh-CN.md)
§19（双轨模型、shadow-first 运维流程、10429 拒绝语义）。

`ncgo test rate-limit e2e` 对运行中的生成项目做限流验证。对 Kitex 服务通过
grpcurl 走 gRPC 压测：

```bash
ncgo test rate-limit e2e --rpc-method MyService.Ping --rpc-payload '{"user":"alice"}'
```

两段式运行先确认 shadow 模式 0 拒绝（含 `ratelimit shadow denied` 日志），再确认
enforce 模式返回预期比例的 10429。

可观测性（tracing / OTel）现已内置到 Hertz 与 Kitex 基础模板，统一使用
go-framework OTLP（Hertz：`cfg.Server.Jaeger` → `hertz/observability.NewProvider`；
Kitex：`cfg.Jaeger` → `kitex/observability.NewProvider`）。原
`observability_otel` / `otel` add-on 已移除，`ncgo add infra otel` 现在返回
invalid kind。

对 Hertz 项目，`redis` 现在默认对应一份由顶层 `cfg.Redis` 经
`go-middleware/redis.NewUniversalClient` 派生的共享
`redis.UniversalClient`：signature nonce、rate-limit、idempotency，以及可选
`internal/base/data/redis.go` 默认都会复用它；只有模块级 Redis override
时，才会拆出独立连接池。

`kafka`、`es`、`clickhouse` add-on 现在将连接构建委托给 `go-middleware` 工厂方法（`go-middleware/kafka`、`go-middleware/es`、`go-middleware/clickhouse`）。生成的包装结构体（`KafkaWriter`、`KafkaReader`、`ES`、`ClickHouse`）通过 samber/do 接收 go-middleware `Config` 类型，不再直接接收第三方库原始结构体。错误码使用 `go-framework/error.CodeConfigInvalid` 做配置校验，连接失败使用 go-middleware 预定义码（ClickHouse）或项目段码（ES: `40506`）。

`observability_logging` 会生成 `internal/base/logging/logging.go`，并按服务类型额外生成 `hertz.go` 或 `kitex.go`。使用 `go-common/log` 提供结构化日志，支持 `WithCategory` 分类子 Logger、masking 脱敏、OTel trace context 注入，以及 `go-common/error`（`goerror`）结构化错误解析。

默认 Hertz / Kitex 模板只预留 logging wiring 注释，不会在未启用 optional 时 import `internal/base/logging`；也可以使用 opt-in 的 `--wire` 自动替换默认 access/recovery 日志。加 `--dry-run` 可预览 optional 文件、manifest 更新和 wiring 目标，不会修改文件。接入示例见 `specs/007-observability-logging.zh-CN.md`。

`release_canary` 会生成 `internal/base/release/canary.go`，并按服务类型额外生成 `hertz.go` 或 `kitex.go`。MVP 是 SDK-neutral helper，支持 release metadata、traffic context、Hertz Header adapter、Kitex metadata adapter、统一 canary rule、Nacos/Polaris discovery instance 模型、`Discoverer` / `RuleProvider` / `Selector` 抽象、stable/canary pool 拆分、权重选择、`fallback=stable|fail_fast`、Kitex load balancer 以及 Nacos/Polaris discoverer / rule-provider skeleton；后续再接真实 SDK adapter。

默认 Hertz / Kitex 模板只预留 canary wiring 注释，不会在未启用 optional 时 import `internal/base/release`；也可以使用 opt-in 的 `--wire` 自动挂载 traffic middleware。加 `--dry-run` 可预览会被接线修改的源码。接入示例见 `specs/006-canary-release.zh-CN.md`。

Hertz 模板内置轻量多语言响应处理：`internal/pkg/i18n` 会根据 `Accept-Language` 选择 `en`、`zh-CN`、`zh-TW`、`ja-JP`、`ko-KR`、`fr-FR`、`de-DE` 或 `es-ES`，`internal/pkg/response` 会对 JSON 响应里的 `msg` 做翻译并写入 `Content-Language`。这些默认语言同样来自 `internal/pkg/i18n/locales/*.json`，并由 `make i18n` 生成的 `catalog_gen.go` 注册。默认无请求头时仍返回原来的英文错误 key；业务错误可在启动期用 `i18n.Register("zh-CN", "order_conflict", "订单冲突")` 扩展翻译。

### Micro 服务

```bash
ncgo new commerce --module github.com/acme/commerce --mode micro
ncgo add rpc user-rpc --root commerce
ncgo add bff web-bff --root commerce
```

### 诊断、生命周期与 Agent

```bash
ncgo doctor --root .
ncgo doctor --root . --output json
ncgo doctor --root . --output sarif > doctor.sarif.json
ncgo protolint --root . --file idl/app/demo.proto --output json
ncgo mcp serve
ncgo upgrade --root . --plan
ncgo upgrade --root . --dry-run
ncgo extract domain device --root . --to services/device-rpc
ncgo extract domain device --root . --to services/device-rpc --apply
ncgo version
```

`ncgo doctor` 现在除了检查 Go 工具链、`hz` / `kitex`、manifest 与 `template/data.json` 之外，还会在 `manifest.service.idl` 存在时默认执行 proto lint，并把命中的 Proto I/O 规则映射到 doctor report 中。CLI 现在支持 `--output text|json|sarif`；其中 `--json` 仍保留为兼容别名，等价于 `--output json`。`sarif` 适合接入 code scanning、IDE 诊断面板或 CI 归档。

`ncgo import` 会为已有项目反向生成 `.ncgo/manifest.yaml`。类型自动检测依赖生成器标记文件：`router.go` 含 `// Code generated by hz.`（Hertz）或 `handler.go` 含 `// Code generated by kitex.`（Kitex）。用 `ncgo new --no-generate` 生成的脚手架还没有标记文件，导入时需要显式指定 `--kind`（例如 `ncgo import --root . --kind kitex`）。

`ncgo mcp serve` 会启动 stdio MCP server。当前暴露 `ncgo_version`、`ncgo_doctor`、`ncgo_ai_init_claude`、`ncgo_ai_sync`、`ncgo_i18n_report`、`ncgo_i18n_check`、`ncgo_protolint`、`ncgo_add_infra`、`ncgo_add_method`。现在这套 MCP 接口已经按 contract-first 方式集中整理到 [docs/examples.zh-CN.md#0-mcp-contract-first-参考](docs/examples.zh-CN.md#0-mcp-contract-first-参考) 的 `0. MCP contract-first 参考` 一节：会先说明每个工具的输入、支持的 `output`，以及稳定的顶层结果字段，再进入具体 workflow 示例。简而言之，结构化 MCP 工具会把 `content[0].text` 作为展示/转存载荷，同时保留同级顶层字段供 Agent 直接消费；`output` 只影响文本载荷格式。

如果你在生成后的 Hertz 项目里使用内置 i18n 工作流，现在也可以用 `ncgo i18n report` / `ncgo i18n check`，或通过 MCP 的 `ncgo_i18n_report` / `ncgo_i18n_check` 消费结构化结果。可直接参考 [docs/examples.zh-CN.md#5-生成项目中的-i18n-补译工作流](docs/examples.zh-CN.md#5-生成项目中的-i18n-补译工作流) 中的“生成项目中的 i18n 补译工作流”。

如果你希望把 `.proto` 契约校验也纳入 CLI / MCP 工作流，可以使用 `ncgo protolint --root . --file ...`，或通过 MCP 的 `ncgo_protolint` 消费同一份结构化 diagnostics。CLI 当前支持 `--output text|json|sarif`；其中 `sarif` 可直接接入 code scanning / IDE 分析工具。现在也支持用 `--ignore-rule` / `--ignore-file`（MCP 对应 `ignoreRules` / `ignoreFiles`）对已知历史问题做显式抑制；在 mono 服务或 micro workspace 根目录下，如果省略 `--file`，ncgo 会尝试从 manifest / workspace 自动发现要 lint 的 proto。可直接参考 [docs/examples.zh-CN.md#6-生成项目中的-proto-契约校验工作流](docs/examples.zh-CN.md#6-生成项目中的-proto-契约校验工作流) 中新增的“生成项目中的 Proto 契约校验工作流”。

当前内置的 Proto I/O 规则里，除了 `PIO101~PIO206`、`PIO301` 这类 `error` 规则外，也已经包含一批默认启用的 `phase2 warning`：`PIO111`、`PIO112`、`PIO113`、`PIO211`、`PIO212`、`PIO302`、`PIO303`、`PIO401`、`PIO402`、`PIO403`、`PIO404`。这些 warning 会继续出现在 `diagnostics` / doctor report 中，但 **warning-only 不会让 `ok=false`**；只有命中 `error` 级规则时，CLI / MCP / doctor 才会进入失败态。

`upgrade` 当前只更新 ncgo/assets 版本元数据。`--plan` 会输出 root/workspace 与 service manifest 的详细只读升级计划；`--dry-run` 保留较简洁的无写入输出。`extract domain` 默认输出迁移计划；加 `--apply` 后会把计划中的 domain 文件复制到已存在的 Kitex 目标服务，并把域内 import 重写为目标 module。它不会删除源文件、覆盖目标文件或自动接好跨服务 client。

## FAQ

### `ncgo: command not found`

请确认 `GOBIN` 或 `$(go env GOPATH)/bin` 已加入 `PATH`，然后重新执行：

```bash
ncgo version
```

### 找不到 `hz` 或 `kitex`

`ncgo new` 会自动检测缺失的生成器并提示你安装。在提示时输入 `Y` 即可自动安装，
或输入 `n` 中止并手动安装：

```bash
go install github.com/cloudwego/hertz/cmd/hz@latest
go install github.com/cloudwego/kitex/tool/cmd/kitex@latest
ncgo doctor
```

如果你想先准备文件、稍后再跑生成器，可以使用 `--no-generate`。

### Hertz 项目执行 `make swagger` 找不到 `protoc` 或插件

Hertz 模板的 `make swagger` 会调用 `protoc --http-swagger_out=...`，因此需要同时准备：

- `protoc`：Protocol Buffers 编译器，需要通过系统包管理器或官方 release 安装；
- `protoc-gen-http-swagger`：Go 插件，需要安装到 `GOBIN` 或 `$(go env GOPATH)/bin`，并确保该目录在 `PATH` 中。

常见安装方式：

```bash
# macOS / Homebrew
brew install protobuf

# Go 插件
go install github.com/hertz-contrib/swagger-generate/protoc-gen-http-swagger@latest

# 确认 PATH 能找到工具
protoc --version
protoc-gen-http-swagger --help
```

生成后的 Hertz 项目也提供 `make install-tools`，会安装 Go 侧开发工具和 `protoc-gen-http-swagger`；但 `protoc` 本身仍需你通过系统方式安装。

Swagger spec 会通过 `go:embed` 编译进二进制。执行 `make swagger` 更新 `internal/docs/swagger/openapi.yaml` 后，需要重新 `go run .` / `make dev` 或重新构建并重启服务，`/swagger/openapi.yaml` 才会返回最新内容。

## 开发检查

贡献者本地工作流、PR 约定与更多检查说明见
[`CONTRIBUTING.zh-CN.md`](CONTRIBUTING.zh-CN.md)。

```bash
go build .
go test ./... -count=1
./scripts/smoke.sh
```

CI 会在 GitHub Actions 中运行更完整的检查集合。Release 构建由 tag 触发；人工发布流程见 [specs/008-release-process.zh-CN.md](specs/008-release-process.zh-CN.md)。

模板或脚手架发生有意变更后，更新 golden：

```bash
go test ./internal/scaffold/mono/... -update-golden -count=1
```
