# 设计：补齐 infra add-on 与 Kitex+database 组合的 golden test 覆盖

Issue: https://github.com/byx-darwin/ncgo/issues/98

## 背景

- `internal/scaffold/mono/golden_test.go` 目前有 5 个场景（default、with-database、
  kitex-default、with-rulecenter、template-rulecenter），缺少 "Kitex + database" 组合。
- `internal/scaffold/infra` 下 `SupportedKinds()` 覆盖 redis/kafka/es/clickhouse/
  registry_polaris/observability_logging/release_canary/rate_limit/polaris_adapter 等
  add-on，但 golden test 只覆盖了 redis、logging（hertz+kitex）、canary（hertz+kitex）、
  polaris_adapter。

澄清确认：`redis` 已有 `TestGenerateGoldenInfraRedis` 覆盖（Issue 描述中的缺口列举不完全
准确）；实际缺口为 kafka、es、clickhouse。`registry_polaris` / `rate_limit` 是 Kitex-only
特化项，模式已由 `TestGenerateGoldenInfraPolarisAdapter` 代表性覆盖，本次不追加。

## 方案（bounded，遵循现有 golden test 模式，不改动生产代码/模板）

1. `internal/scaffold/mono/golden_test.go` 新增 `TestGenerateGoldenKitexWithDatabase`：
   `goldenOpts(t, "demo", true)` + `opts.Kind = manifest.KindKitex`，快照名
   `mono-kitex-with-database`，对齐 `TestGenerateGoldenWithDatabase` /
   `TestGenerateGoldenKitexDefault` 的写法。

2. `internal/scaffold/infra/golden_test.go` 仿照 `TestGenerateGoldenInfraRedis` 新增：
   - `TestGenerateGoldenInfraKafka` → 快照目录 `infra-kafka`
   - `TestGenerateGoldenInfraES` → 快照目录 `infra-es`
   - `TestGenerateGoldenInfraClickHouse` → 快照目录 `infra-clickhouse`

   均为 `seedProject(t, nil)` → `Add(Options{Kind: KindXxx})` → 遍历 `res.WrittenPaths`
   写入对应快照目录，与现有 Redis 用例结构一致。

3. 用 `go test ./internal/scaffold/... -update-golden -count=1` 生成新 testdata，再跑
   `go test ./internal/scaffold/... -count=1` 确认全绿。

## 范围之外

- 不新增 `registry_polaris` / `rate_limit` 覆盖（Kitex-only 特化项，AC 未要求）。
- 不改动任何模板文件或生成器逻辑，纯粹补测试。

## 测试

- `go test ./internal/scaffold/mono/... -count=1`
- `go test ./internal/scaffold/infra/... -count=1`
- `go test ./internal/scaffold/... -count=1`（整体确认）
