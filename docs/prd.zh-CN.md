# ncgo 产品需求文档（v1.0）

## 1. 定位

`ncgo` 是面向 AI Agent 的 Go 微服务脚手架。它把 `nc-skills-golang` 中的 Hertz / Kitex / pgx / sqlc / samber/do / oops 约定，从“Agent 可读规则”落成“可执行工具”，并通过 CLI 与 MCP 暴露给 Agent 使用。

- **规则 / 约定**：`nc-skills-golang`，负责 review-mode prompt、分层规则、AI 调用说明。
- **模板 / 执行器**：`ncgo`，负责内置模板、生成项目、检查项目。
- **契约**：ncgo 生成的代码应能通过 `nc-skills-golang` review mode。

## 2. 用户

- 使用 Claude Code / Cursor / Codex 构建 Go 后端的工程师。
- 希望用统一方式创建服务的小团队。
- 已使用 nc-skills，希望一条命令启动服务骨架的用户。

## 3. 核心决策

| # | 决策 | 内容 |
|---|---|---|
| 1 | 策略 | 先封装 `hz` / `kitex`，后续再叠加 AI Runtime |
| 2 | 当前脚手架范围 | Hertz / Kitex mono 服务 + micro workspace（Kitex RPC、Hertz BFF） |
| 3 | CLI | cobra + viper |
| 4 | MCP | v0.3 引入 stdio MCP server MVP |
| 5 | Assets | 模板由 ncgo 持有，嵌入在 `internal/assets/_data/` |
| 6 | `extract` | v0.4 plan/apply-copy MVP |
| 7 | 项目元数据 | `.ncgo/manifest.yaml`；micro 根目录使用 `ncgo.workspace` |
| 8 | Optional infra | Redis/Kafka/ES/ClickHouse/LoongSuite Go Agent observability/结构化日志/金丝雀发布 helper；~~NATS~~/~~Mongo~~/~~MinIO~~ 暂缓 |

## 4. 命令面

```bash
ncgo new <name>            --module --mode mono|micro --kind hertz|kitex --db postgres|none --idl --dir --no-generate
ncgo add domain <name>     --root --force
ncgo add method <domain.Method> --root --in usecase
ncgo add infra <kind>      --root --force; common: redis|kafka|es|clickhouse|observability_otel|otel|observability_logging|logging|release_canary|canary; kitex-only: registry_etcd
ncgo add rpc <name>        --root --module --dir --no-generate
ncgo add bff <name>        --root --module --dir --no-generate
ncgo doctor                --json --root
ncgo ai sync               --root --lang en|zh-CN --force --dry-run
ncgo mcp serve             expose selected commands as MCP tools
ncgo upgrade               --root --dry-run --plan
ncgo extract domain <name> --root --to services/<name>-rpc --json --apply
ncgo version
```

CLI 是主 API。部分命令提供机器可读输出（例如 `doctor --json`、`extract domain --json`），部分能力通过 MCP 暴露。MCP MVP 暴露 `ncgo_version`、`ncgo_doctor`、`ncgo_ai_sync`、`ncgo_add_infra`、`ncgo_add_method`。`ncgo_add_infra` 参数为 `root`、`kind`、`force`，支持与 CLI 相同的 infra kind，并返回依赖安装 next steps，不自动安装依赖。

## 5. 项目元数据

Mono 服务使用 `.ncgo/manifest.yaml`：

```yaml
ncgo: {version: 0.1.0, assets_version: 0.1.1}
mode: mono
module: github.com/acme/user-api
service:
  name: user-api
  kind: hertz
  with_database: false
  idl: idl/app/user.proto
infra: [redis, observability_otel, observability_logging, release_canary]
domains: [device, theme]
generated_at: 2026-04-29T15:00:00+08:00
```

Micro 根目录使用 `ncgo.workspace`，每个服务仍有自己的 `.ncgo/manifest.yaml`。

## 6. AI 协作产物

`ncgo ai sync` 生成：

| 文件 | 受众 | 来源 |
|---|---|---|
| `AGENTS.md` | 任意 Agent | manifest + 内置设计文档 + `AGENTS.local.md` |
| `CLAUDE.md` | Claude Code | 同上，Claude 风格格式 |
| `.cursor/rules/ncgo.mdc` | Cursor | 同上，带 MDC frontmatter |

生成是幂等的。受管理文件顶部带 `<!-- ncgo:managed -->`。项目自定义内容放入 `AGENTS.local.md`。

## 7. Agent-friendly Anchors

生成的 Go 文件带稳定注释，便于 Agent 精准插入代码：

```go
// ncgo:domain=device kind=usecase
type UseCase struct { ... }

// ncgo:methods:start
// ncgo:methods:end
```

`ncgo add method device.ListThemes --in usecase` 会在 markers 之间插入方法桩。当前 MVP 只支持 usecase 方法插入。

## 8. Optional Infra、可观测与金丝雀

common infra：

- `redis`
- `kafka`
- `es`
- `clickhouse`
- `observability_otel`（支持 `otel` alias）
- `observability_logging`（支持 `logging` alias）
- `release_canary`（支持 `canary` alias）

Kitex-only：

- `registry_etcd`

`observability_otel` 现在面向 Alibaba LoongSuite Go Agent，通过 embedded template 实现，模板源：

```text
internal/assets/_data/optional/observability_otel.go
```

生成目标：

```text
internal/base/observability/otel.go
```

MVP 提供 `OTEL_*` 环境变量辅助，next steps 会提示安装 `otel` CLI、使用 `otel go build` 进行编译期自动插桩，并用 `OTEL_SERVICE_NAME` / `OTEL_EXPORTER_OTLP_ENDPOINT` 等环境变量运行服务。不会自动安装 agent、不会 `go get` OTel SDK、不会自动修改 `main.go`、不会自动加 Hertz / Kitex middleware。

`observability_logging` 生成统一结构化日志 facade，模板源：

```text
internal/assets/_data/optional/observability_logging.go
internal/assets/_data/hertz/optional/observability_logging.go
internal/assets/_data/kitex/optional/observability_logging.go
```

生成目标：

```text
internal/base/logging/logging.go
internal/base/logging/hertz.go  # Hertz 服务额外生成
internal/base/logging/kitex.go  # Kitex 服务额外生成
```

MVP 支持 `slog`、console/file/both/none、JSON/text、`lumberjack` rotate + gzip、access/error/biz/rpc/db/panic/audit/security 分类、`samber/oops` 结构化解析、request/trace/release/canary 字段。默认 Hertz / Kitex 模板仅预留安全 wiring 注释，不直接 import optional 包；`ncgo add infra logging --wire` 可 opt-in 修改已生成的默认 server/client wiring。依赖安装仍只作为 next steps 输出，不自动执行 `go get`。

`release_canary` 生成 SDK-neutral 金丝雀发布 helper，模板源：

```text
internal/assets/_data/optional/release_canary.go
```

生成目标：

```text
internal/base/release/canary.go
internal/base/release/hertz.go  # Hertz 服务额外生成
internal/base/release/kitex.go  # Kitex 服务额外生成
```

MVP 支持 release metadata、`Traffic` context、Hertz Header adapter、Kitex metadata adapter、统一 canary `RuleSet`、priority、header/cookie/user/tenant/region/weighted 分流、Nacos / Polaris provider 标识与 discovery config、统一 `Instance` 模型、`Discoverer` / `RuleProvider` / `Selector` 抽象、stable/canary/unknown pool 拆分、实例权重选择、`fallback=stable|fail_fast` 和 SDK-neutral Kitex client load balancer adapter skeleton。默认模板仅预留安全 wiring 注释；`ncgo add infra canary --wire` 可 opt-in 挂载默认 traffic middleware。它暂不直接依赖 Nacos / Polaris SDK；后续通过 SDK adapter 和 config watch 扩展。

## 9. 生命周期

- `ncgo upgrade`：metadata-only MVP，更新 `.ncgo/manifest.yaml` / `ncgo.workspace` 的 ncgo/assets 版本信息，不重写源码；`--plan` 输出 root/workspace 与 service manifest 的详细只读升级计划。
- `ncgo extract domain`：默认校验 mono domain 并输出迁移计划；`--apply` 会复制计划中的文件到已存在的 Kitex 目标服务，并重写域内 import，不删除源文件、不覆盖目标文件、不自动接跨服务 client。

## 10. 里程碑

| 版本 | 范围 | 状态 |
|---|---|---|
| v0.1 | Hertz mono、domain、infra、doctor、golden tests | done |
| v0.2 | Kitex mono、内置设计文档、`ai sync` | done |
| v0.3 | micro、add rpc、add bff、mcp serve、anchor system | done (MVP) |
| v0.4 | extract、upgrade | done (plan/apply-copy + metadata/plan MVP) |
| v0.5 | ~~NATS~~ / ~~Mongo~~ / ~~MinIO~~ / LoongSuite Go Agent observability optional | done (LoongSuite MVP; others deferred) |
| v0.6 | structured logging optional、release canary optional | done (SDK-neutral MVP) |

## 11. 非目标

- 内置 LLM 或调用 LLM API。
- IDE 插件。
- 对外暴露 `pkg/` Go API。
- 默认自动迁移或覆盖用户业务代码。
