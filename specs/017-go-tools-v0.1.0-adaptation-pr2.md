# 017-PR2 — 生成项目适配 go-tools v0.1.0（PR2：config duration 化 + redis 铺垫）

- 状态：设计已确认
- 日期：2026-07-23
- 关联：Issue #7；工作流 `wf-2026-07-23-001`；总设计文档 `specs/017-go-tools-v0.1.0-adaptation.md`
- 前置：PR1（`specs/017-go-tools-v0.1.0-adaptation-pr1-plan.md`，已合并于 PR #11）
- 后续：PR3（redis 客户端接线 / 是否整体采用 `go-middleware/redis.Config`）

## 1. 背景与范围重估

Issue #7 原始范围描述为「conf 包加载与配置类型接入 `go-framework/config`」。经核对 `origin/main` 现状（PR1 已合并后），**原范围中的多数项已在 main 完成**：

| Issue #7 原始范围项 | main 现状 |
|------|------|
| conf 加载改用 `config.LoadYAML` / `config.MustLoadYAML` | ✅ 已完成（hertz `conf_go.yaml`、kitex `conf.yaml` 均用 `config.LoadYAML[Config]`） |
| 复用 go-framework 选项类型 — registry | ✅ 已完成（两边均用 `config.RegistryOption`） |
| 复用 go-framework 选项类型 — jaeger | ✅ kitex 已完成（`*config.JaegerOption`）；hertz base conf 本就无 jaeger 块 |
| duration 字段改用 `config.Duration` / `time.Duration` | ❌ **未完成**（conf 仍为 int 秒/毫秒字段） |
| 复用选项类型 — captcha / observability | ⚠️ base conf 无手写对应块（属「新增」而非「替换」） |
| redis 对齐 `go-middleware/redis.Config` | ❌ 未完成（手写 `RateLimitRedisConfig`，约 40 字段） |

**决策（用户确认）**：

- **1A — 重新聚焦**：PR2 专注剩余 config 工作 = **duration 字段迁移** + **redis 铺垫**；更新 Issue #7 正文说明收窄范围。选项类型复用已在 main 满足，不新增 captcha/observability（YAGNI）。
- **2A + R-A — redis 只铺垫**：redis 配置块 **duration 化**、字段命名向 `go-middleware/redis.Config` 靠拢，但**不**换类型别名、**不**重写客户端构造；客户端接线与是否整体采用 `redis.Config` 留给 PR3。

## 2. go-tools v0.1.0 相关 API（实测自 module cache）

`go-framework/config`：

- `config.Duration`：`struct{ time.Duration }`，实现 `UnmarshalYAML`，解析 `"30s"` / `"5m"` 等字符串为 `time.Duration`。
- `config.LoadYAML[T any](path) (*T, error)` / `config.MustLoadYAML[T any](path) *T`。
- 选项类型：`config.JaegerOption`、`config.RegistryOption`、`config.ObservabilityConfig`（OTel tracing/metrics，与 Logging 不同语义）、`config.CaptchaOption`。

`go-middleware/redis.Config`（约 24 字段，超时统一 `time.Duration`，D2 决策）：`Addrs/Username/Password/DB/MasterName/Sentinel*/Protocol/ClientName/PoolSize/MinIdleConns` + `DialTimeout/ReadTimeout/WriteTimeout/PoolTimeout/ConnMaxIdleTime/ConnMaxLifetime/IdleCheckFrequency/MinRetryBackoff/MaxRetryBackoff`（`time.Duration`）+ `MaxRetries`。

> `redis.Config` 是模板 `RateLimitRedisConfig`（约 40 字段）的**精简子集**，缺少 `DialerRetries`、`Read/WriteBufferSize`、`PoolFIFO`、`MaxIdleConns/MaxActiveConns`、`ConnMaxLifetimeJitter`、`MaxRedirects`、`ReadOnly/RouteByLatency/RouteRandomly`、`DisableIdentity/IdentitySuffix/UnstableResp3`、`FailingTimeoutSeconds`、`ContextTimeoutEnabled` 等 go-redis 调优项。**这是 R-A 选择不在 PR2 换类型的根因**（换类型会丢字段并打断 `redis_shared.go` 编译）。

## 3. 设计

### 3.1 Duration 字段迁移（核心）

**原则**：表示「时长」的 int 秒/毫秒（或 int64）字段 → `config.Duration`（YAML 写 `"30s"`/`"8ms"`）；计数/尺寸/端口保持 int。

**Hertz conf（`internal/assets/_data/hertz/hertz-template/conf_go.yaml`）** 迁移字段：

| 结构体 | 字段 |
|------|------|
| `ConfigCenterConfig` | `TimeoutMilliseconds` |
| `DatabaseConfig` | `MaxConnLifetimeSeconds`、`MaxConnIdleTimeSeconds`、`HealthCheckPeriodSeconds` |
| `CORSConfig` | `MaxAgeSeconds` |
| `RateLimitSourceConfig` | `CacheTTLSeconds` |
| `RateLimitGRPCConfig` | `TimeoutMilliseconds` |
| `RateLimitDatabaseConfig` | `QueryTimeoutMilliseconds` |
| `RateLimitRuleConfig` | `WindowSeconds`、`ClientTTLSeconds` |
| `IdempotencyConfig` | `TTLSeconds` |
| `SignatureConfig` | `MaxClockSkewSeconds` |
| `SignatureNonceConfig` | `TTLSeconds` |
| `TokenConfig` | `BufferSeconds`、`ExpiresSeconds`（int64 → config.Duration） |
| `RateLimitRedisConfig`（R-A） | 见 §3.3 |

**Kitex conf（`internal/assets/_data/kitex/kitex-template/conf.yaml`）** 迁移字段：

| 结构体 | 字段 |
|------|------|
| `RPCConfig` | `RequestTimeoutSeconds` |
| `DatabaseConfig` | `MaxConnLifetimeSeconds`、`MaxConnIdleTimeSeconds`、`HealthCheckPeriodSeconds` |
| `RateLimitSourceConfig` | `CacheTTLSeconds` |
| `RateLimitGRPCConfig` | `TimeoutMilliseconds` |
| `RateLimitDatabaseConfig` | `QueryTimeoutMilliseconds` |
| `RateLimitRuleConfig` | `WindowSeconds`、`ClientTTLSeconds` |

> kitex `Server` 超时已用 `time.Duration`（`kitexconfig.ServerTimeout`），且已含 `*config.JaegerOption`，无需改动。

**保持不变**（非时长）：`MaxConns/MinConns`、`MaxEntries`、`MaxRequests`、`MaxBodyBytes`、`PoolSize`、`Priority`、`Burst`、端口号等。

**`Default()` 默认值**：int 字面量改为 `config.Duration{Duration: 30 * time.Second}` 形式（conf 需 `import "time"`）。

**`Validate()`**：对迁移字段的 `< 0` / `<= 0` 比较改为对 `.Duration` 比较（如 `rule.WindowSeconds <= 0` → `rule.Window.Duration <= 0`）。

### 3.2 爆炸半径（同步更新的消费方）

改字段类型会打断读取它们的生成代码，须同步更新（读取 `X` → `X.Duration`，或按语义调整）：

- **kitex**：`kitex-template/ratelimit_usecase.yaml`、`kitex-template/server.yaml`
- **hertz**：`hertz/layout.yaml`（redis_shared 相关段）、限流相关 optional
- **optional add-ons**：`hertz/optional/{clickhouse,kafka,rule_center_client}.go`、`kitex/optional/{clickhouse,kafka,registry_etcd}.go` 等读取超时字段处
- 实施时以 `grep -rnE "\.WindowSeconds|\.TTLSeconds|\.TimeoutMilliseconds|..."` 全量定位，逐一适配，确保 e2e 编译测试转绿。

### 3.3 R-A redis 铺垫

- 保留模板自有 `RateLimitRedisConfig` 结构（**不** alias 到 `go-middleware/redis.Config`）。
- 其中约 11 个超时 int 字段改 `config.Duration`：`MinRetryBackoffMilliseconds`、`MaxRetryBackoffMilliseconds`、`DialTimeoutSeconds`、`DialerRetryTimeoutMilliseconds`、`ReadTimeoutSeconds`、`WriteTimeoutSeconds`、`PoolTimeoutSeconds`、`ConnMaxIdleTimeSeconds`、`ConnMaxLifetimeSeconds`、`ConnMaxLifetimeJitterSeconds`、`FailingTimeoutSeconds`。
  - 字段名可顺势向 `redis.Config` 靠拢（如 `DialTimeoutSeconds`→`DialTimeout`），由实施时按 golden diff 最小化与命名一致性权衡决定；**重命名会放大 diff，倾向保守（保名换类型）**，除非命名对齐收益明确。
- 同步 `hertz/optional/redis_shared.go`：`RedisUniversalOptions` 中 `durationSeconds(cfg.X)`/`durationMilliseconds(cfg.X)` → `cfg.X.Duration`；删除 `durationSeconds`/`durationMilliseconds` 辅助函数（若无其它引用）。
- 更新 `defaultRedisConfig()`、`mergeRedisConfig()`（字段零值判断从 `== 0` 保持对 `time.Duration` 仍成立）与 hertz `conf/dev/conf.yaml` 样例值（`dial_timeout_seconds: 5` → 若保名则值改为字符串 `"5s"`；`min_retry_backoff_milliseconds: 8` → `"8ms"`）。
- kitex `RateLimitRedisConfig`（6 字段：`Addrs/Password/DB/PoolSize/MinIdleConns/MaxActiveConns`）**无超时字段**，本 PR 无需 duration 化。
- **不做**：换类型别名到 `redis.Config`、重写客户端构造、接线 rate_limit/idempotency/nonce 的 redis 消费方 → 均属 PR3。

### 3.4 选项类型复用 —— 已满足，不新增

`config.RegistryOption`（两边）、`*config.JaegerOption`（kitex）已在 main 完成。base conf 无手写 captcha/observability 块可替换；Logging 与 `config.ObservabilityConfig` 语义不同，不混同。按 YAGNI，PR2 不新增。

### 3.5 go.mod —— 无新增依赖

`config.Duration` 属 `go-framework/config`，PR1 已 require `go-framework v0.1.0`。R-A 不引 `redis.Config`，故不需要 `go-middleware`。验收项「go.mod 增加 go-framework v0.1.0」已由 PR1 满足。

## 4. 测试策略

- **golden（文本）**：`go test ./internal/scaffold/{mono,rpc,bff,infra}/... -update-golden -count=1`（精确包路径，不传全树），逐提交审查 diff——仅 duration 字段类型/默认值/YAML 样例值变化。
- **e2e 编译**：`TestGenerateHertzCompiles` / `TestGenerateHertzWithDatabaseCompiles` / `TestGenerateKitexCompiles` 须绿，验证 `config.Duration` 真能编译且 `config.LoadYAML` 正确解析 `"30s"`。需 `hz`/`kitex`/`make` + go 1.26.5 工具链 + proxy 网络；环境缺失则 skip 并明确标注（不静默通过）。
- **YAML 解析验证**：确认 `config.LoadYAML`（yaml.v3 + `config.Duration.UnmarshalYAML`）对 `"30s"`/`"8ms"` 正确解析，对旧的整数值行为符合预期（字符串化后不再接受裸整数 → 样例须同步改为字符串）。
- **验证链**：`go build ./... && go build . && go vet ./... && go test ./... -count=1 && ./scripts/smoke.sh`。

## 5. 文档与契约

- `README.md` / `README.zh-CN.md`、`docs/examples.md` / `docs/examples.zh-CN.md`：补充「生成项目 conf 的 duration 字段 YAML 用 `"30s"` 格式（`config.Duration`）」；中英对齐。
- 内嵌设计文档（`internal/assets/_data/docs/{hertz,kitex}/design-doc.*.md`）若描述 conf 字段，同步 duration 说明。
- 更新 Issue #7 正文：标注范围重估（loading/registry/jaeger 已由 PR1/main 完成），本 PR 聚焦 duration 迁移 + redis 铺垫（R-A）。

## 6. 验收标准映射

| Issue #7 验收项 | 由谁满足 |
|------|------|
| 生成 go.mod 增加 `go-framework v0.1.0` | PR1 已满足 |
| conf 加载基于 `config.LoadYAML` | main 已满足 |
| duration 字段为 `config.Duration`/`time.Duration` | **本 PR** |
| hertz/kitex golden 测试通过 | **本 PR** |
| 完整验证链通过（build/vet/test/smoke） | **本 PR** |
| 中英文档对齐更新 | **本 PR** |

## 7. 风险

- **爆炸半径大**：conf + 多个 optional + kitex usecase/server 均读 duration 字段，golden diff 大 → 逐提交审查，避免误 bless。
- **`config.Duration` 用法适配**：嵌入 `time.Duration`，`Validate()` 比较与消费方读取（`X` → `X.Duration`）需逐一适配，漏改即编译失败（由 e2e 编译测试兜底）。
- **YAML 值格式切换**：duration 字段 YAML 从裸整数改为 `"30s"` 字符串；`config.Duration.UnmarshalYAML` 只接受字符串，故样例/默认须同步，旧格式不再兼容（干净切换，沿用总设计决策）。
- **redis 字段重命名取舍**：保名换类型 diff 小但命名不齐 `redis.Config`；重命名对齐则 diff 大。倾向保守，实施时定夺并在 PR 描述说明。
- 模板/脚手架输出 contract-sensitive；本 PR 不改 ncgo 业务逻辑，仅改模板与生成代码。
