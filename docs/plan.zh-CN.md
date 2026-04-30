# ncgo 后续计划

本文记录当前已完成能力、未完成任务和建议优先级，作为后续开发的路线图。

## 1. 当前已完成

### 1.1 基础能力

- 已完成初始仓库提交：`feat: initialize ncgo scaffold CLI`。
- 已支持 Hertz / Kitex mono scaffold、micro workspace、add rpc / bff / domain / method、doctor、upgrade、extract domain、MCP server。

### 1.2 Optional infra

- 已支持 `redis`、`kafka`、`es`、`clickhouse`、`registry_etcd`。
- 已支持 LoongSuite Go Agent 方向的 `observability_otel` / `otel`。
- 已支持 `observability_logging` / `logging`：
  - `slog`；
  - console / file / both / none；
  - lumberjack rotate + gzip；
  - category routing；
  - `samber/oops` 结构化；
  - request / trace / release / canary 字段；
  - Hertz / Kitex adapter。
- 已支持 `release_canary` / `canary`：
  - release metadata；
  - traffic context；
  - Hertz header adapter；
  - Kitex metadata adapter；
  - canary rules；
  - Nacos / Polaris provider 标识；
  - `Discoverer` / `RuleProvider` / `Selector` 抽象；
  - stable / canary pool；
  - weighted / sticky selection；
  - `fallback=stable|fail_fast`。

### 1.3 Wiring / preview / plan

- 已支持 `ncgo add infra logging --wire`。
- 已支持 `ncgo add infra canary --wire`。
- 已支持 `--dry-run`，不写 optional 文件、不保存 manifest、不修改 server/client 源码。
- 已支持 `--output json`，输出机器可读结果。
- 已支持 `--plan`，等价于 `--dry-run --output json`。
- 已支持 `infra.Result.Plan`：
  - `file/create`；
  - `file/overwrite`；
  - `manifest/add`；
  - `manifest/already_present`；
  - `wire/update`；
  - `wire/already_wired`；
  - `wire/add_import`；
  - `wire/insert_logging_init`；
  - `wire/replace_middleware`；
  - `wire/insert_traffic_middleware`；
  - `wire/insert_client_middleware`；
  - `next_step/run`。
- 已为 wire operation-level plan 增加 `anchorSource` / `anchor` 字段，用于标明命中的是 `marker` 还是 `legacy` anchor。
- 已支持 MCP `ncgo_add_infra` 返回结构化字段：
  - `dryRun`；
  - `updated`；
  - `writtenPaths`；
  - `wiredPaths`；
  - `nextSteps`；
  - `plan`。
- 已在默认 Hertz / Kitex 模板中加入基础 `// ncgo:wire:*` marker，并让 `wire.go` 优先使用 marker、缺失时回退 legacy anchor：
  - `// ncgo:wire:logging:init`；
  - `// ncgo:wire:logging:server-middleware`；
  - `// ncgo:wire:canary:server-traffic`；
  - `// ncgo:wire:kitex-client:middleware`。

## 2. 推荐优先级

### P0：继续细化 wiring plan detail

当前已在 `wire/update` / `wire/already_wired` 之外补充 operation-level action：`wire/add_import`、`wire/insert_logging_init`、`wire/replace_middleware`、`wire/insert_traffic_middleware`、`wire/insert_client_middleware`。

基础结构化字段已补：wire operation-level plan 保留原有 `detail`，并增加 `anchorSource` / `anchor` 标明命中 `marker` 还是 `legacy` anchor。后续可继续补 `from` / `to` / `insertAfter`，让 `--plan` 更接近真实 patch preview。

### P0（基础版已完成）：继续完善模板 wiring marker

基础 marker 已加入默认模板，例如：

```text
// ncgo:wire:logging:init
// ncgo:wire:logging:server-middleware
// ncgo:wire:canary:server-traffic
// ncgo:wire:kitex-client:middleware
```

当前要求：仍需兼容旧模板，不能只依赖 marker。

收益：减少对具体源码片段的脆弱匹配，降低模板演进成本。

已完成：

- Hertz / Kitex 默认模板已加入 marker；
- mono golden testdata 已同步；
- `wire.go` 已优先使用 marker，缺失时 fallback legacy anchor；
- 已补 marker path 和 legacy fallback 测试；
- wire plan 已通过 `anchorSource` / `anchor` 暴露 marker/legacy 来源；
- smoke 与全量 `go test ./...` 已验证通过。

后续可继续：

1. 将 marker helper 抽象得更通用，便于后续 registry / selector / otel wiring 复用；
2. 继续减少对具体 middleware 行的字符串替换依赖；
3. 继续扩展 plan patch 信息，例如 `from` / `to` / `insertAfter`。

### P1：Kitex canary selector adapter MVP

基于已有 `Selector` / `Discoverer` / `RuleProvider` 抽象，补 Kitex client selector adapter skeleton。

注意：优先保持 SDK-neutral，避免引入重依赖导致生成模板不可编译。

### P1：Nacos / Polaris adapter skeleton

补 SDK adapter seam 或 skeleton：

- Nacos config / adapter 接口；
- Polaris config / adapter 接口；
- 文档说明如何接真实 SDK。

建议先做 skeleton 和文档，不急于引入真实 SDK 依赖。

### P1（已开始）：故障排查文档

在 logging / canary 专题文档补 troubleshooting：

- `--wire could not find ... anchor` 如何处理；
- 已手动改过 server.go / client.go 怎么办；
- 什么时候使用 `--force`；
- `wire/already_wired` 的含义；
- 为什么 ncgo 不自动执行 `go get`；
- dry-run 与真实执行输出对照。

### P2：CLI / MCP 统一 result renderer

当前 CLI 和 MCP 各自渲染 add infra 结果。建议抽共享 formatter / DTO，减少重复逻辑。

注意包依赖方向，避免 `internal` 反向依赖 `cmd`。

### P2：扩展 plan 到其它 add 子命令

后续可考虑：

- `ncgo add domain --plan`；
- `ncgo add rpc --plan`；
- `ncgo add bff --plan`。

需要先统一各 scaffold 的 plan schema。

### P2：CI / release 工程化

建议补：

- GitHub Actions：`go test ./...`；
- smoke 脚本执行；
- `go build ./cmd/ncgo`；
- release / tag 流程文档。

注意：部署、发布、push、tag 均需人工确认后执行。

## 3. 建议下一步

建议优先做：

1. logging / canary troubleshooting 文档；
2. 继续细化 wiring plan detail 为结构化 patch 信息；
3. Kitex canary selector adapter MVP。