# ncgo 上下文交接说明

本文用于新对话 / 新 Agent 快速恢复 ncgo 项目上下文。

## 工作目录

仓库路径：

```text
/Users/byx/Documents/workspace/github.com/byx-darwin/ncgo
```

## 项目定位

`ncgo` 是面向 AI Agent 的 Go 微服务脚手架 CLI。它内置 Hertz / Kitex 模板，生成项目 manifest，调用 `hz` / `kitex`，并为 Claude / Cursor / 其他 Agent 渲染 AI 上下文文件。

## 当前完成状态

- v0.1：Hertz mono scaffold、`add domain`、`add infra`、`doctor`、golden tests。
- v0.2：Kitex mono scaffold、embedded design docs、`ai sync`。
- v0.3：micro workspace、`add rpc`、`add bff`、anchor system、`mcp serve`。
- v0.4：`upgrade` metadata-only + `--plan`、`extract domain` plan/apply-copy。
- v0.5：LoongSuite Go Agent observability optional MVP（保留 `observability_otel` / `otel` 名称兼容）。

MCP 当前 tools：`ncgo_version`、`ncgo_doctor`、`ncgo_ai_sync`、`ncgo_add_infra`、`ncgo_add_method`。

## LoongSuite Go Agent observability optional

已完成：

- `ncgo add infra observability_otel --root .`
- `ncgo add infra otel --root .`

- `otel` 是 alias。
- manifest 统一记录 `observability_otel`。
- Hertz / Kitex 都支持。
- `observability_otel` / `otel` 现在面向 Alibaba LoongSuite Go Agent。
- 通过 embedded template 实现。
- 模板源：`internal/assets/_data/optional/observability_otel.go`。
- 生成目标：`internal/base/observability/otel.go`。
- 不自动 `go get` OTel SDK。
- 不自动安装 LoongSuite `otel` CLI。
- next steps 提示安装 `otel` CLI、`otel go build ./...`、配置 `OTEL_*` 环境变量。
- 不自动改 `main.go`。
- 不自动加 Hertz / Kitex middleware。

暂缓但保留在文档里：~~NATS~~、~~Mongo~~、~~MinIO~~。不要正式删除它们。

## 最近完成的文档收尾

已完成 README / PRD 总体收尾，并新增中文文档：`README.md`、`README.zh-CN.md`、`docs/prd.md`、`docs/prd.zh-CN.md`。

文档覆盖当前 v0.5 MVP 状态、Hertz / Kitex / micro、domain、infra、LoongSuite observability、AI sync、doctor、MCP、upgrade、extract、开发检查。验证通过：Markdown code fence 检查、`go test ./...`、`go build ./...`、`go vet ./...`。

## 重要实现约定

### Infra optional

`ncgo add infra` 机制：

1. 从 embedded asset 复制 Go 文件。
2. 写入目标项目。
3. 更新 `.ncgo/manifest.yaml` 的 `infra`。
4. 打印依赖或工具 setup next steps。
5. 不自动安装依赖。

LoongSuite observability 的 source of truth 是 `internal/assets/_data/optional/observability_otel.go`，不要再使用旧 Kitex-only OTel 模板，也不要恢复成手写 OTel SDK 初始化模板。

### Micro workspace

根目录：`ncgo.workspace`、`services/`。服务目录：`services/<name>/.ncgo/manifest.yaml`。

### Anchor system

Usecase 文件中使用：

```go
// ncgo:methods:start
// ncgo:methods:end
```

`ncgo add method` 只在 marker 中插入 usecase 方法桩。

## 关键文件

建议新 Agent 优先查看：

- `README.md`
- `README.zh-CN.md`
- `docs/prd.md`
- `docs/prd.zh-CN.md`
- `cmd/ncgo/add.go`
- `cmd/ncgo/main.go`
- `internal/scaffold/infra/infra.go`
- `internal/scaffold/infra/infra_test.go`
- `internal/assets/_data/optional/observability_otel.go`
- `internal/mcp/server.go`
- `internal/mcp/tools.go`
- `internal/extract/domain.go`
- `internal/upgrade/upgrade.go`

## 下一步建议

最近完成：MCP tools 已新增 `ncgo_add_infra`：

- 参数：`root`、`kind`、`force`
- 复用 `internal/scaffold/infra.Add`
- 支持 `redis`、`kafka`、`es`、`clickhouse`、`observability_otel`、`otel`、`observability_logging`、`logging`、`release_canary`、`canary`、`registry_etcd`
- 已补 MCP tests
- 已更新 README / PRD / 中文文档
- 已跑 `go test ./...`、`go build ./...`、`go vet ./...`、MCP smoke

后续可考虑：

1. 为 `release_canary` 增加 Nacos / Polaris SDK adapter 与配置 watch
2. LoongSuite 接入示例增强
3. 将默认 Hertz / Kitex 模板的 access/recovery log 接入 `internal/base/logging`

最近完成：`extract domain --apply`。

- 默认仍是 plan-only。
- `--apply` 要求目标目录已有 Kitex service manifest。
- 复制 usecase / repository / register 三个计划文件到目标服务。
- 复制时把域内 import 从源 module 重写到目标 module。
- 不删除源文件、不覆盖目标文件、不自动接跨服务 client。

最近完成：`upgrade --plan`。

- `--plan` 是只读模式，不写 manifest / workspace。
- 输出 root/workspace manifest 和 service manifests 的 ncgo/assets from/to。
- `--dry-run` 保留旧的简洁 no-write 输出。
- 实际 `upgrade` 仍只更新 ncgo/assets metadata，不重写生成源码。

最近完成：smoke 脚本固化。

- 新增 `scripts/smoke.sh`。
- 脚本会先构建临时 `ncgo` binary。
- 覆盖 MCP `tools/list`、`upgrade --plan` 只读行为、`extract domain --apply` import 重写。
- README / 中文 README 的开发检查已加入 `./scripts/smoke.sh`。

最近完成：将 OTel optional 更换为 LoongSuite Go Agent。

- `observability_otel` / `otel` 名称保留，manifest 仍记录 `observability_otel`。
- 生成文件仍是 `internal/base/observability/otel.go`。
- 模板改为 LoongSuite `OTEL_*` 环境变量辅助，不再导入 `go.opentelemetry.io/*`。
- next steps 改为 LoongSuite `otel` CLI 安装、`otel go build ./...` 和运行时环境变量。
- smoke 已覆盖 `add infra otel` 生成 LoongSuite helper。

最近完成：日志方案文档。

- 新增 `docs/observability-logging.zh-CN.md`。
- 方案覆盖 Hertz / Kitex 统一日志、`samber/oops` 结构化、request_id / trace_id、文件分类、文件压缩、LoongSuite 兼容、金丝雀日志字段。
- 已实现 `observability_logging` / `logging` optional：生成 `internal/base/logging/logging.go`，并按服务类型额外生成 `hertz.go` 或 `kitex.go`。
- 已支持 `slog`、console/file/both/none、lumberjack rotate + gzip、category routing、`oops.AsOops`、trace/request/release/canary 字段。
- 已补 CLI / MCP 多文件输出、infra unit tests、`scripts/smoke.sh` logging 冒烟。
- 已在默认 Hertz / Kitex 模板中加入安全 wiring 注释，标出 `logging.HertzRequestID()` / `logging.HertzAccessLog()` / `logging.HertzRecovery()` 与 `logging.KitexRequestID()` / `logging.KitexAccessLog()` / `logging.KitexRecovery()` 的替换位置；默认项目不会 import optional 的 `internal/base/logging` 包。
- 已新增 `ncgo add infra logging --wire` / MCP `wire=true`，可 opt-in 修改已生成的默认 Hertz/Kitex server/client wiring；默认不改源码。

最近完成：`release_canary` / `canary` optional MVP。

- 新增 `internal/assets/_data/optional/release_canary.go`，生成 `internal/base/release/canary.go`。
- 新增 `internal/assets/_data/hertz/optional/release_canary.go` 与 `internal/assets/_data/kitex/optional/release_canary.go`，按服务类型额外生成 `internal/base/release/hertz.go` 或 `internal/base/release/kitex.go`。
- 支持 release metadata、`Traffic` context、统一 canary `RuleSet`、priority、header/cookie/user/tenant/region/weighted 分流。
- 支持 Hertz Header adapter 读取 `X-Traffic-Lane` / `X-User-ID` / `X-Tenant-ID`，以及 Kitex metadata adapter 读取/透传 `traffic.lane` / `traffic.user_id` / `traffic.tenant_id`。
- 支持 Nacos / Polaris provider 标识与 discovery config，统一 `Instance` 模型，按 `release.track` 拆分 stable / canary / unknown pool。
- 支持 `Discoverer` / `RuleProvider` / `Selector` 抽象，后续 Nacos / Polaris SDK adapter 可直接接入。
- 支持实例权重选择、sticky key、一致回退语义：`fallback=stable` / `fallback=fail_fast`。
- 已补 infra unit tests、`scripts/smoke.sh` canary 冒烟，并确认模板在临时 Go module 中可编译。
- 已在默认 Hertz / Kitex 模板中加入安全 wiring 注释，标出 `release.HertzTraffic()`、`release.KitexTraffic()` 与后续 `release.Selector` 的插入位置；默认项目不会 import optional 的 `internal/base/release` 包。
- 已新增 `ncgo add infra canary --wire` / MCP `wire=true`，可 opt-in 挂载默认 traffic middleware；默认不改源码。
- 方向 A：已为 `ncgo add infra ... --wire` 增加 `--dry-run` / MCP `dryRun=true` 预览能力；会输出 `would write` / `would wire`、manifest 预期更新，并保证不写 optional 文件、不保存 manifest、不修改 server/client 源码。
- 已继续优化 `--wire`：真实写入前先做 `PreviewWire` preflight，避免 anchor/format 失败后留下 optional/manifest 半完成状态；logging wiring 的默认 middleware/init anchor 也从静默 no-op 改为缺失时报错，已接线场景保持幂等。
- 已为 `infra.Add` 增加结构化 `Plan`，包含 file create/overwrite、manifest add/already_present、wire update/already_wired、next_step run；MCP `ncgo_add_infra` 保留 text，同时返回 `dryRun`、`updated`、`writtenPaths`、`wiredPaths`、`nextSteps`、`plan` 字段，方便 agent/前端消费。
- CLI `ncgo add infra` 已新增 `--output text|json`（默认 text）和 `--plan`（等价 `--dry-run --output json`）；JSON 输出 `dryRun`、`updated`、`writtenPath(s)`、`wiredPaths`、`nextSteps`、`plan`，非法 output 会在调用核心写入逻辑前报错。
- Wiring plan 已保留 `wire/update` 兼容项，并增加细粒度 action：`wire/add_import`、`wire/insert_logging_init`、`wire/replace_middleware`、`wire/insert_traffic_middleware`、`wire/insert_client_middleware`。
- 默认 Hertz / Kitex 模板已加入基础 `// ncgo:wire:*` marker；`wire.go` 优先 marker、缺失时 fallback legacy anchor，并同步更新 mono golden testdata 与 marker path 测试。
- 专题文档 `docs/observability-logging.zh-CN.md` 与 `docs/canary-release.zh-CN.md` 已补 `--wire --dry-run`、`--output json`、preflight 失败语义和推荐的“先预览 plan 再真实执行”流程。
- 后续编码建议做 Nacos / Polaris SDK adapter、config watch、Kitex selector adapter，并考虑把默认模板中的 `hlog` / `klog` access/recovery log 替换为 logging optional 的适配层。

## 注意事项

- 不要提交 / push，除非用户明确要求。
- 不要自动安装依赖，除非用户明确允许。
- 修改代码后需要跑测试。
- 文档中 NATS / Mongo / MinIO 要保留删除线。
- CLI 保持薄封装，核心逻辑放在 `internal/...`。
- 生成代码优先走 embedded templates，不要在生成器里硬编码大段业务代码。
