# ncgo

面向 AI Agent 的 Go 微服务脚手架 CLI。`ncgo` 内置并维护 Hertz / Kitex 模板，生成项目元数据，调用上游生成器，并为 Claude、Cursor 等 Agent 渲染上下文文件。

English documentation: [README.md](README.md)。产品需求文档见 [docs/prd.md](docs/prd.md) 和 [docs/prd.zh-CN.md](docs/prd.zh-CN.md)。Agent 上下文交接见 [docs/context-handoff.zh-CN.md](docs/context-handoff.zh-CN.md)。

## 当前状态
v0.5 MVP 已完成：

- Mono 脚手架：Hertz HTTP 服务、Kitex RPC 服务。
- Micro 工作区：根 `ncgo.workspace`，并支持 `add rpc` / `add bff`。
- Domain 工作流：`add domain` 与基于 anchor 的 `add method`。
- Optional infra：Redis、Kafka、Elasticsearch、ClickHouse、LoongSuite Go Agent observability、结构化日志、金丝雀发布 helper、Kitex-only etcd registry。
- AI / Agent 工作流：`ai sync`、静态 `doctor`、MCP stdio server。
- 生命周期 MVP：metadata-only `upgrade --plan`、保守的 `extract domain --apply`。

暂缓但保留在路线图中：~~NATS~~、~~Mongo~~、~~MinIO~~。

## ncgo 会生成什么

| 范围 | 输出 |
|---|---|
| 项目元数据 | 服务内 `.ncgo/manifest.yaml`；micro 根目录 `ncgo.workspace` |
| Hertz | IDL 占位、`hz` 自定义 layout/package 输入、HTTP 服务骨架 |
| Kitex | IDL 占位、Kitex template tree、RPC 服务骨架 |
| Domain | `internal/usecase/<name>`、`internal/repository/<name>`、DI register 文件 |
| Infra | `internal/base/...` 下的可选 Go 文件 |
| AI 上下文 | `AGENTS.md`、`CLAUDE.md`、`.cursor/rules/ncgo.mdc` |

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

### Kitex RPC 服务
```bash
ncgo new user-api --module github.com/acme/user-api --kind kitex
cd user-api
go mod tidy
make dev
```

Kitex 服务名会被规范化为合法 proto / Go 标识符。例如 `user-api` 会生成 `idl/userapi.proto`、proto package `userapi`、service `UserApi`。

### Micro 工作区
```bash
ncgo new commerce --module github.com/acme/commerce --mode micro
cd commerce
ncgo add rpc user-rpc --root .
ncgo add bff web-bff --root .
```

根目录保存 `ncgo.workspace`，每个生成的服务保存自己的 `.ncgo/manifest.yaml`。

## Prepare vs Generate

默认情况下，`ncgo new` 分两步：

1. 准备确定性输入：`.ncgo/manifest.yaml`、IDL 占位、`template/` 下的自定义模板。
2. 调用上游生成器：Hertz 使用 `hz`，Kitex 使用 `kitex`。

如果只想准备输入，不调用生成器：

```bash
ncgo new user-api --module github.com/acme/user-api --kind kitex --no-generate
```

此时 ncgo 会打印之后可手动执行的生成器命令。生成器成功后会创建 `go.mod`，后续步骤从 `go mod tidy` 开始。

## AI 上下文

```bash
ncgo ai sync --root user-api --lang zh-CN
```

会写入受管理文件：

- `AGENTS.md`
- `CLAUDE.md`
- `.cursor/rules/ncgo.mdc`

这些文件带有 `<!-- ncgo:managed -->` 标记。没有该标记的已有文件默认不会覆盖，除非传 `--force`。项目私有说明放在 `AGENTS.local.md`。

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
ncgo add infra otel --root .                 # observability_otel alias
ncgo add infra logging --root .              # observability_logging alias
ncgo add infra canary --root .               # release_canary alias
ncgo add infra logging --root . --wire       # 可选：自动接入默认 server/client 模板
ncgo add infra logging --root . --wire --dry-run  # 预览写入/接线，不修改文件
ncgo add infra logging --root . --wire --dry-run --output json  # 输出机器可读 plan
ncgo add infra registry_etcd --root .        # kitex only
```

common infra：`redis`、`kafka`、`es`、`clickhouse`、`observability_otel`（`otel` alias）、`observability_logging`（`logging` alias）、`release_canary`（`canary` alias）。Kitex-only：`registry_etcd`。

`observability_otel` 现在面向 Alibaba LoongSuite Go Agent。它会生成 `internal/base/observability/otel.go`，提供 `OTEL_*` 环境变量辅助，并打印安装 `otel` CLI、使用 `otel go build` 的 next steps；不会自动安装 agent、修改启动代码或增加 SDK 依赖。

`observability_logging` 会生成 `internal/base/logging/logging.go`，并按服务类型额外生成 `hertz.go` 或 `kitex.go`。MVP 支持 `slog`、console/file/both/none、`lumberjack` rotate + gzip、日志分类、`samber/oops` 结构化解析，以及 request/trace/release/canary 字段。

默认 Hertz / Kitex 模板只预留 logging wiring 注释，不会在未启用 optional 时 import `internal/base/logging`；也可以使用 opt-in 的 `--wire` 自动替换默认 access/recovery 日志。加 `--dry-run` 可预览 optional 文件、manifest 更新和 wiring 目标，不会修改文件。接入示例见 `docs/observability-logging.zh-CN.md`。

`release_canary` 会生成 `internal/base/release/canary.go`，并按服务类型额外生成 `hertz.go` 或 `kitex.go`。MVP 是 SDK-neutral helper，支持 release metadata、traffic context、Hertz Header adapter、Kitex metadata adapter、统一 canary rule、Nacos/Polaris discovery instance 模型、`Discoverer` / `RuleProvider` / `Selector` 抽象、stable/canary pool 拆分、权重选择和 `fallback=stable|fail_fast`；后续再接 Nacos/Polaris SDK adapter。

默认 Hertz / Kitex 模板只预留 canary wiring 注释，不会在未启用 optional 时 import `internal/base/release`；也可以使用 opt-in 的 `--wire` 自动挂载 traffic middleware。加 `--dry-run` 可预览会被接线修改的源码。接入示例见 `docs/canary-release.zh-CN.md`。

### Micro 服务

```bash
ncgo new commerce --module github.com/acme/commerce --mode micro
ncgo add rpc user-rpc --root commerce
ncgo add bff web-bff --root commerce
```

### 诊断、生命周期与 Agent

```bash
ncgo doctor --root .
ncgo mcp serve
ncgo upgrade --root . --plan
ncgo upgrade --root . --dry-run
ncgo extract domain device --root . --to services/device-rpc
ncgo extract domain device --root . --to services/device-rpc --apply
ncgo version
```

`ncgo mcp serve` 会启动 stdio MCP server。MVP 暴露 `ncgo_version`、`ncgo_doctor`、`ncgo_ai_sync`、`ncgo_add_infra`、`ncgo_add_method`。`ncgo_add_infra` 参数为 `root`、`kind`、`force`、`wire`、`dryRun`，支持与 CLI 相同的 infra kind，只打印依赖安装 next steps，不自动执行 `go get`，并返回结构化 `plan` 字段供 agent 预览。

`upgrade` 当前只更新 ncgo/assets 版本元数据。`--plan` 会输出 root/workspace 与 service manifest 的详细只读升级计划；`--dry-run` 保留较简洁的无写入输出。`extract domain` 默认输出迁移计划；加 `--apply` 后会把计划中的 domain 文件复制到已存在的 Kitex 目标服务，并把域内 import 重写为目标 module。它不会删除源文件、覆盖目标文件或自动接好跨服务 client。

## 开发检查

```bash
go build ./...
go vet ./...
go test ./... -count=1
./scripts/smoke.sh
```

模板或脚手架发生有意变更后，更新 golden：

```bash
go test ./internal/scaffold/mono/... -update-golden -count=1
```
