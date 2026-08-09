## 背景

「生成项目全面适配 go-tools v0.1.0」多 PR 分解的 **PR2（config 重构）**。总设计文档：`specs/017-go-tools-v0.1.0-adaptation.md`。

## 范围重估（2026-07-23，workflow `wf-2026-07-23-001`）

经核对 `origin/main`（PR1 已合并于 #11）现状，原范围多数项已完成，本 PR2 **重新聚焦**：

| 原范围项 | 状态 |
|---|---|
| `conf` 加载改用 `config.LoadYAML` / `config.MustLoadYAML` | ✅ 已由 PR1/main 完成 |
| 复用 go-framework 选项类型 — registry / jaeger | ✅ 已完成（`config.RegistryOption` 两边；`*config.JaegerOption` kitex） |
| duration 字段改用 `config.Duration` / `time.Duration` | ⬜ **本 PR 执行** |
| 复用选项类型 — captcha / observability | ➖ base conf 无手写对应块，按 YAGNI 不新增 |
| redis 对齐 `go-middleware/redis.Config` | 🟡 **本 PR 仅铺垫（R-A）**：duration 化、字段命名向 `redis.Config` 靠拢；不换类型、客户端接线留 PR3 |

**PR2 设计文档（已确认）**：`specs/017-go-tools-v0.1.0-adaptation-pr2.md`

### PR2 实际范围

- duration 字段迁移：hertz + kitex conf 的 int 秒/毫秒字段 → `config.Duration`（YAML 写 `"30s"`/`"8ms"`）
- redis 铺垫（R-A）：`RateLimitRedisConfig` 超时字段 duration 化、同步 `redis_shared.go`，不换类型别名
- 同步更新消费方（kitex `ratelimit_usecase`/`server`、hertz `layout`、各 optional add-on）与 `Validate()`
- 更新 golden、e2e 编译测试、中英文档

## 范围（原始描述，保留备查）

生成项目的 `conf` 包加载与配置类型接入 `go-framework/config`（新增依赖 `go-framework v0.1.0`）：

- `conf` 加载改用 `config.LoadYAML` / `config.MustLoadYAML`
- duration 字段改用 `config.Duration` / `time.Duration`（YAML 写 `"30s"`，替换现有 int 毫秒/秒字段）
- 复用 go-framework 选项类型（jaeger / registry / captcha / observability）替换 conf 中手写的对应配置
- redis 相关配置对齐 `go-middleware/redis.Config`（为 PR3 铺垫）

## 验收标准

- [x] 生成 `go.mod` 增加 `go-framework v0.1.0`（PR1 已完成）
- [x] 生成 conf 加载基于 `config.LoadYAML`（PR1/main 已完成）
- [ ] duration 字段为 `config.Duration`/`time.Duration`（**本 PR**）
- [ ] hertz/kitex golden 测试通过（**本 PR**）
- [ ] 完整验证链通过（build/vet/test/smoke）（**本 PR**）
- [ ] 中英文档对齐更新（**本 PR**）

## 依赖关系

- 前置：PR1（go.mod 基础 + 错误机制）— ✅ 已合并（#11）
- 相关：PR3（redis 客户端接线 / 是否整体采用 `go-middleware/redis.Config`）
