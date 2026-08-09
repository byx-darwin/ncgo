## 概述

生成项目全面适配 go-tools v0.1.0 多 PR 分解的 **PR2（config 重构）**：把生成的 Hertz/Kitex 项目 conf 里的 int 秒/毫秒 duration 字段统一迁移到 go-framework `config.Duration`（YAML 值改为 `"30s"`/`"8ms"` 字符串），并对 redis 配置块做 **R-A 铺垫**（duration 化，不换类型、客户端接线留 PR3）。

Closes #7

## 范围重估（重要）

经核对 `origin/main`（PR1 已合并于 #11），Issue #7 原范围多数项**已完成**：

| 原范围项 | 状态 |
|---|---|
| `conf` 加载改用 `config.LoadYAML`/`MustLoadYAML` | ✅ 已由 PR1/main 完成 |
| 复用选项类型 — registry / jaeger | ✅ 已完成（`config.RegistryOption` 两边；`*config.JaegerOption` kitex） |
| duration 字段改用 `config.Duration`/`time.Duration` | ⬜ **本 PR 执行** |
| 复用选项类型 — captcha / observability | ➖ base conf 无手写对应块，按 YAGNI 不新增 |
| redis 对齐 `go-middleware/redis.Config` | 🟡 **本 PR 仅铺垫（R-A）**，客户端接线留 PR3 |

## 本 PR 实际范围

- **duration 字段迁移**（保名换类型：Go 字段名与 YAML/JSON key 不变，仅类型 `int`→`config.Duration`，值改字符串）：
  - Hertz conf 22 字段（ConfigCenter/Database/CORS/RateLimit/Idempotency/Signature/Token/redis 超时）
  - Kitex conf 9 字段（RPC/Database/RateLimit）
  - `Default()` 用 `config.Duration{Duration: N*time.Second/Millisecond}`（保持原量级）；`Validate()`/merge 用 `.Duration`
- **消费方适配**：hertz `data_go`/`layout`（限流 fixed_window/lua/DB 行边界、CORS、签名、幂等）/`rule_center_client`；kitex `server`；int 边界用 `int(.Seconds())`/`int(.Milliseconds())`
- **R-A redis 铺垫**：`redis_shared.go` 超时字段改用 `.Duration`，删除本地 `durationSeconds`/`durationMilliseconds` helper；**未**换类型到 `redis.Config`、**未**接线客户端、**未**引 `go-middleware`
- **golden 重新生成**（mono/bff/rpc/infra 全树）
- **文档中英对齐**：README/examples + 内嵌 hertz/kitex 设计文档

## 关键决策

- **保名换类型**：避免大面积 key 重命名 diff；YAML key 不变，仅值从裸整数改为 `"30s"` 字符串（干净切换，`config.Duration.UnmarshalYAML` 只接受字符串）。
- **R-A redis 铺垫**：`go-middleware/redis.Config` 是模板 `RateLimitRedisConfig`（~40 字段）的精简子集（~24），直接换类型会丢 go-redis 调优字段并打断 `redis_shared.go`；故本 PR 只做 duration 化铺垫，类型替换与客户端接线留 PR3。
- **无新增依赖**：`config.Duration` 属已 require 的 `go-framework/config`；ncgo 自身 `go.mod` 0 改动。

## 验证

- `gofmt -l` CLEAN；`go build ./... && go build .` OK；`go vet ./...` OK
- `go test ./... -count=1` PASS
- `./scripts/smoke.sh` OK
- **e2e 编译测试**（`TestGenerateHertzCompiles` / `TestGenerateHertzWithDatabaseCompiles` / `TestGenerateKitexCompiles`）全部 **ran-not-skipped 通过**（真实生成项目 + `go mod tidy` + `go build ./...`，验证 `config.Duration` 可编译且 `"30s"` 可加载）
- 最终全分支代码评审：**READY TO MERGE**，0 Critical/Important/Minor 阻塞项（契约面最小性、R-A 边界、golden 完整性、跨表面一致性均 PASS）

## 与后续 PR 的关系

- **前置 PR1**（go.mod 基础 + 错误机制）：已合并（#11）
- **后续 PR3**：redis 客户端接线 / 决定是否整体采用 `go-middleware/redis.Config`

## 已知取舍

- 限流动态规则源（DB 行/grpc/rule_center，int 秒）与 `config.Duration` 的边界转换已在各 int 接口处理（lua 参数 `int(.Seconds())`、DB 读入 `config.Duration{...}`）。
- go-redis `UniversalOptions.FailingTimeoutSeconds` 为 int 秒，取 `int(cfg.FailingTimeoutSeconds.Seconds())`。
- YAML duration 值格式从裸整数切换为字符串，旧格式不再兼容（干净切换，沿用总设计决策）。

设计文档：`specs/017-go-tools-v0.1.0-adaptation-pr2.md`；实施计划：`specs/017-go-tools-v0.1.0-adaptation-pr2-plan.md`
