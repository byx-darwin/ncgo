# PR3 设计 — 生成项目适配 go-tools v0.1.0：logging + redis 接入 go-common/log 与 go-middleware/redis

- 状态：设计已确认
- 日期：2026-07-24
- 关联：Issue #8；工作流 `wf-2026-07-23-001`；总设计文档 `specs/017-go-tools-v0.1.0-adaptation.md`
- 前置：PR1（#11，已合并）、PR2（#12，已合并）

## 1. 背景与范围重估

Issue #8 原始范围：logging 删除内嵌 slog+lumberjack 手写实现接入 go-common/log；redis 删除内嵌 go-redis/v9 手写接入 go-middleware/redis。

经核对 origin/main 现状（PR1+PR2 已合并后）：

| Issue #8 原始范围项 | main 现状 |
|------|------|
| base logging（main.go goclog.Init/ReleaseInfo/Close） | ✅ 已完成（hertz/kitex main 模板均用 go-common/log） |
| optional observability_logging add-on | ❌ 仍为手写 slog+lumberjack（~360 行共享 + ~100 行 hertz + ~115 行 kitex） |
| redis 客户端接入 go-middleware/redis | ❌ 仍为手写 go-redis/v9（redis_shared.go + redis.go） |
| conf RateLimitRedisConfig 对齐 redis.Config | ❌ 仍为 ~40 字段自有结构（PR2 已 duration 化） |
| goGetDeps 含 go-middleware | ❌ 未包含 |

**决策（用户确认）**：

- **1A — logging add-on 替换**：用 go-common/log 重写 `optional/observability_logging.go`，保留 hertz/kitex 中间件骨架。
- **2A — redis 换用 redis.Config**：删除 RateLimitRedisConfig，conf 新定义 RedisConfig（镜像 redis.Config，time 用 config.Duration）+ ToMiddlewareConfig() 转换层。
- **3A — 保留 redis_shared 共享池**：SharedRedisClient/CloseSharedRedisClient 保留，内部改用 go-middleware/redis.NewUniversalClient。

## 2. go-tools v0.1.0 相关 API（实测自 module cache）

### go-common/log

- `Init(cfg Config, release ReleaseInfo) error` — 全局初始化
- `L() *Logger` — 获取全局 Logger（嵌入 `*slog.Logger`）
- `Logger.WithCategory(category string) *Logger` — 分类子 Logger
- `Logger.ErrorContext(ctx, msg, err, args...)` — 自动提取 oops 错误属性
- `Close() error` — 关闭全局 Logger
- 分类常量：`CategoryAccess/Error/Biz/RPC/DB/Panic/Audit/Security/App/Cache/MQ`
- 层 helper：`App(ctx)/DB(ctx)/Access(ctx)/RPC(ctx)/MQ(ctx)/Cache(ctx)` — 返回分类 Logger
- `WithRequestID(ctx, requestID)` / `RequestIDFromContext(ctx)` — context 注入/提取
- `ErrorAttrs(err) []any` — oops 错误属性提取
- Config：Level/Format/Mode/AddSource/File(FileConfig)/Categories/Masking(MaskConfig)
- ReleaseInfo：ServiceName/Version/GitSHA/BuildTime/Environment/Extra
- Handler 链：output → contextHandler（request_id）→ releaseHandler → maskHandler
- 文件轮转：file_writer.go / rotation.go（内置 lumberjack）
- 注意：`NewLogger`（Init 使用）不含 otelHandler（trace_id/span_id 注入）；`New()`（Options 模式）含 otelHandler。base logging 已用 Init，OTel trace 注入为已知缺口（不阻塞 PR3）。

### go-middleware/redis

- `NewUniversalClient(ctx context.Context, cfg *Config) (Client, func(), error)` — 创建客户端，自动 Ping，返回 closeFn
- `NewClient(ctx, cfg, opts...) (*redis.Client, error)` — 单节点，支持 WithTrace()
- `Client` 接口嵌入 `redis.UniversalClient`
- Config（~24 字段，time.Duration）：Addrs/Username/Password/DB/MasterName/Sentinel*/Protocol/ClientName/PoolSize/MinIdleConns + DialTimeout/ReadTimeout/WriteTimeout/PoolTimeout/ConnMaxIdleTime/ConnMaxLifetime/IdleCheckFrequency/MinRetryBackoff/MaxRetryBackoff（time.Duration）+ MaxRetries
- yaml.v3 不支持 time.Duration 字符串解析 → conf 层用 config.Duration + 转换

## 3. 设计

### 3.1 Logging add-on 重写

**原则**：删除手写 slog+lumberjack 实现，用 go-common/log 替代；保留 hertz/kitex 中间件骨架（RequestID/AccessLog/Recovery）。

**共享文件 `optional/observability_logging.go`（~360→~80 行）**：

删除：
- `Logger` 结构体、`Config`/`ConsoleConfig`/`FileConfig`/`CategoryConfig`/`ReleaseInfo` 类型
- `Init(cfg, release)`、`L()`、`setDefault()`
- `Logger.Info/Warn/Error/log()` 方法
- `buildHandler()`、`newHandler()`、`multiHandler`、`parseLevel()`、`firstNonEmpty()`
- `ErrorAttrs()`、`ContextAttrs()`（go-common/log 内置）
- lumberjack import、slog handler 相关

保留/新增：
- 分类常量：re-export `goclog.CategoryAccess` 等（或直接引用 go-common/log 常量）
- `SinceMS(started time.Time) slog.Attr` — 延迟 helper（go-common/log 无此功能）
- `WithRequestID(ctx, requestID, source string) context.Context` — 保留 source 参数（go-common/log 只有 requestID）
- `WithTrafficLane(ctx, lane string) context.Context` — 保留（go-common/log 无此功能）
- `InitFromConf(cfg conf.LoggingConfig, release goclog.ReleaseInfo) error` — 转换 conf.LoggingConfig → goclog.Config 并调用 goclog.Init
- context key 常量保留（`ContextKeyRequestID`/`ContextKeyRequestIDSource`/`ContextKeyTrafficLane`）

**Hertz 中间件 `hertz/optional/observability_logging.go`**：

- `HertzAccessLog()`：`L().Info(ctx, CategoryAccess, "hertz access", ...)` → `goclog.Access(ctx).InfoContext(ctx, "hertz access", ...)`
- `HertzRecovery()`：`L().Error(ctx, CategoryPanic, "msg", err)` → `goclog.L().WithCategory(goclog.CategoryPanic).ErrorContext(ctx, "msg", err)`
- `HertzRequestID()`：`WithRequestID(ctx, requestID, source)` 保留（本地 helper）
- slog.Attr 参数改为 any（slog.Logger 风格）

**Kitex 中间件 `kitex/optional/observability_logging.go`**：

- `KitexAccessLog()`：`L().Info(ctx, CategoryRPC, "kitex rpc", ...)` → `goclog.RPC(ctx).InfoContext(ctx, "kitex rpc", ...)`
- `KitexRecovery()`：同理用 ErrorContext
- `KitexRequestID()`：WithRequestID 保留

**conf LoggingConfig**：保留现有结构（Enabled/Mode/Format/Level/AddSource/Console/File/Categories），字段与 goclog.Config 基本对齐。`InitFromConf` 做映射：
- `LoggingFileConfig.MaxSizeMB` → `goclog.FileConfig.MaxSize`
- `LoggingFileConfig.MaxAgeDays` → `goclog.FileConfig.MaxAge`
- `LoggingCategoryConfig` → `goclog.CategoryConfig`
- 新增 Masking 映射（conf 暂无 Masking 字段，可选新增或留空）

### 3.2 Redis conf 类型替换

**删除**：`RateLimitRedisConfig`（~40 字段）、`RedisConfig = RateLimitRedisConfig` alias、`defaultRedisConfig()`、`mergeRedisConfig()`。

**新增**：`RedisConfig` 结构体（~24 字段，镜像 go-middleware/redis.Config）：

```go
type RedisConfig struct {
    Addrs            []string        `yaml:"addrs"`
    Username         string          `yaml:"username"`
    Password         string          `yaml:"password"`
    DB               int             `yaml:"db"`
    MasterName       string          `yaml:"master_name"`
    SentinelUsername string          `yaml:"sentinel_username"`
    SentinelPassword string          `yaml:"sentinel_password"`
    Protocol         int             `yaml:"protocol"`
    ClientName       string          `yaml:"client_name"`
    PoolSize         int             `yaml:"pool_size"`
    MinIdleConns     int             `yaml:"min_idle_conns"`
    DialTimeout      config.Duration `yaml:"dial_timeout"`
    ReadTimeout      config.Duration `yaml:"read_timeout"`
    WriteTimeout     config.Duration `yaml:"write_timeout"`
    PoolTimeout      config.Duration `yaml:"pool_timeout"`
    ConnMaxIdleTime  config.Duration `yaml:"conn_max_idle_time"`
    ConnMaxLifetime  config.Duration `yaml:"conn_max_lifetime"`
    IdleCheckFrequency config.Duration `yaml:"idle_check_frequency"`
    MaxRetries       int             `yaml:"max_retries"`
    MinRetryBackoff  config.Duration `yaml:"min_retry_backoff"`
    MaxRetryBackoff  config.Duration `yaml:"max_retry_backoff"`
}
```

**转换方法**：

```go
func (c RedisConfig) ToMiddlewareConfig() *mwredis.Config {
    return &mwredis.Config{
        Addrs:            c.Addrs,
        Username:         c.Username,
        Password:         c.Password,
        DB:               c.DB,
        MasterName:       c.MasterName,
        SentinelUsername: c.SentinelUsername,
        SentinelPassword: c.SentinelPassword,
        Protocol:         c.Protocol,
        ClientName:       c.ClientName,
        PoolSize:         c.PoolSize,
        MinIdleConns:     c.MinIdleConns,
        DialTimeout:      c.DialTimeout.Duration,
        ReadTimeout:      c.ReadTimeout.Duration,
        WriteTimeout:     c.WriteTimeout.Duration,
        PoolTimeout:      c.PoolTimeout.Duration,
        ConnMaxIdleTime:  c.ConnMaxIdleTime.Duration,
        ConnMaxLifetime:  c.ConnMaxLifetime.Duration,
        IdleCheckFrequency: c.IdleCheckFrequency.Duration,
        MaxRetries:       c.MaxRetries,
        MinRetryBackoff:  c.MinRetryBackoff.Duration,
        MaxRetryBackoff:  c.MaxRetryBackoff.Duration,
    }
}
```

**Default()**：`defaultRedisConfig()` 简化为 RedisConfig 默认值（`DialTimeout: config.Duration{Duration: 5 * time.Second}` 等）。

**applyRedisFallbacks()**：保留级联逻辑（top-level Redis → RateLimit.Redis / Idempotency.Redis / Nonce.Redis），但 mergeRedisConfig 简化为逐字段零值填充（或直接用 `if primary == (RedisConfig{}) { return fallback }`）。

**Validate()**：redis 相关验证适配新字段名。

**YAML key 变化**：`dial_timeout_seconds` → `dial_timeout`、`min_retry_backoff_milliseconds` → `min_retry_backoff`（值仍为 `"5s"`/`"8ms"` 字符串）。

### 3.3 redis_shared.go 重写（hertz）

**保留**：共享池模式（`sharedRedisClients` map + mutex）、`SharedRedisClient(cfg)`、`CloseSharedRedisClient(cfg)`、`CloseSharedRedisClients()`、`redisClientKey(cfg)`。

**改写**：
- `SharedRedisClient(cfg RedisConfig)`：内部改用 `mwredis.NewUniversalClient(context.Background(), cfg.ToMiddlewareConfig())`，返回 `redis.UniversalClient`（Client 接口嵌入 UniversalClient）。
- 删除 `RedisUniversalOptions(cfg)` — 不再需要手动映射到 `redis.UniversalOptions`。
- import 变化：删除 `"github.com/redis/go-redis/v9"`，新增 `mwredis "github.com/byx-darwin/go-tools/go-middleware/redis"`。
- `RedisConfig` 类型别名改为 `conf.RedisConfig`（不再是 RateLimitRedisConfig）。

### 3.4 redis.go 改写

**Hertz `hertz/optional/redis.go`**：
- `NewRedis(ctx, cfg)`：继续用 `SharedRedisClient(cfg.Redis)`（不变）。
- `NewRedisWithOptions(ctx, opts)`：改用 `mwredis.NewClient(ctx, cfg)` 或删除（YAGNI，保留则改签名）。
- import 变化：删除 `"github.com/redis/go-redis/v9"`，新增 `mwredis`。
- `Redis` struct：`Client redis.UniversalClient` → `Client mwredis.Client`（或保持 `redis.UniversalClient`，因 Client 嵌入它）。

**Kitex `kitex/optional/redis.go`**：
- `NewRedis(ctx, opts *redis.UniversalOptions)`：改签名为 `NewRedis(ctx context.Context, cfg *conf.RedisConfig)`，内部用 `mwredis.NewUniversalClient(ctx, cfg.ToMiddlewareConfig())`。
- 删除手动 `redis.NewUniversalClient(opts)` + `cli.Ping(ctx)` + `cli.Close()`（go-middleware 已含 Ping + closeFn）。
- import 变化同上。

### 3.5 layout.yaml middleware 适配

- `sharedRedisClient(cfg any) redis.UniversalClient { return nil }` 占位符保留（redis add-on 安装后由 data 包提供真实实现）。
- 删除 `redisUniversalOptions(cfg conf.RateLimitRedisConfig) *redis.UniversalOptions` helper（不再需要）。
- 限流/幂等/nonce 的 `sharedRedisClient(cfg.Redis)` 调用不变。
- `TestRedisUniversalOptionsMapping` 测试删除或改为 `TestRedisConfigToMiddleware`。
- `conf.RateLimitRedisConfig` 引用全部改为 `conf.RedisConfig`。

### 3.6 kitex conf 适配

kitex `RateLimitRedisConfig`（6 字段：Addrs/Password/DB/PoolSize/MinIdleConns/MaxActiveConns，无超时字段）→ 改用 `RedisConfig`（与 hertz 同构或精简子集）。kitex 无 redis_shared，redis.go 直接用 go-middleware/redis。

### 3.7 goGetDeps 与 setup steps 更新

`internal/scaffold/infra/infra.go`：

```go
KindRedis: {"github.com/byx-darwin/go-tools/go-middleware", "github.com/byx-darwin/go-tools/go-common"},
KindObservabilityLog: {"github.com/byx-darwin/go-tools/go-common"},  // 删除 lumberjack
```

删除 `"github.com/redis/go-redis/v9"`（go-middleware 传递依赖）和 `"gopkg.in/natefinish/lumberjack.v2"`（go-common/log 内置）。

### 3.8 go.mod 依赖

生成项目 go.mod 新增 `go-middleware v0.1.0`（WithDatabase 时已由 tidy 补；redis add-on 显式 require）。hertz 静态 go.mod 模板 require 块加 `go-middleware v0.1.0`。

### 3.9 optional-config/redis.yaml 更新

YAML key 对齐 redis.Config：

```yaml
redis:
  addrs:
    - 127.0.0.1:6379
  username: ""
  password: ""
  db: 0
  master_name: ""
  sentinel_username: ""
  sentinel_password: ""
  protocol: 3
  client_name: ""
  pool_size: 10
  min_idle_conns: 2
  dial_timeout: "5s"
  read_timeout: "3s"
  write_timeout: "3s"
  pool_timeout: "4s"
  conn_max_idle_time: "300s"
  conn_max_lifetime: "1800s"
  max_retries: 3
  min_retry_backoff: "8ms"
  max_retry_backoff: "512ms"
```

删除旧 key（dial_timeout_seconds、min_retry_backoff_milliseconds、pool_fifo、max_redirects、read_only 等 ~16 个 go-redis 调优字段）。

## 4. 测试策略

- **golden（文本）**：`go test ./internal/scaffold/{mono,bff,rpc,infra}/... -update-golden -count=1`（精确包路径），逐提交审查 diff。
- **e2e 编译**：`TestGenerateHertzCompiles` / `TestGenerateKitexCompiles` 须绿（验证 go-middleware/redis + go-common/log 真能编译）。
- **infra golden**：infra-redis / infra-logging-hertz / infra-logging-kitex 通过。
- **验证链**：`go build ./... && go build . && go vet ./... && go test ./... -count=1 && ./scripts/smoke.sh`。

## 5. 文档与契约

- `README.md` / `README.zh-CN.md`、`docs/examples.md` / `docs/examples.zh-CN.md`：补充「生成项目 logging 基于 go-common/log（WithCategory + masking + OTel trace context）；redis add-on 基于 go-middleware/redis（NewUniversalClient，redis.Config 用 config.Duration）」。
- 内嵌设计文档（`internal/assets/_data/docs/{hertz,kitex}/design-doc.*.md`）若描述 logging/redis，同步更新。
- 中英对齐。

## 6. 验收标准映射

| Issue #8 验收项 | 由谁满足 |
|------|------|
| 生成 go.mod 增加 go-middleware v0.1.0 | **本 PR** |
| 生成项目 logging 基于 go-common/log | **本 PR**（add-on 重写） |
| redis add-on 基于 go-middleware/redis | **本 PR** |
| 内嵌 logging/redis 模板移除或替换 | **本 PR** |
| infra golden 测试通过 | **本 PR** |
| 完整验证链通过（build/vet/test/smoke） | **本 PR** |
| 中英文档对齐更新 | **本 PR** |

## 7. 风险

- **YAML key 变化**：redis 配置 key 从 `xxx_seconds`/`xxx_milliseconds` 改为 `xxx`（config.Duration 字符串），旧格式不兼容（干净切换，沿用总设计决策）。
- **共享池 + go-middleware/redis**：`NewUniversalClient` 返回 closeFn，共享池需在 CloseSharedRedisClient 中调用 closeFn（而非直接 `client.Close()`）。
- **OTel trace 注入缺口**：go-common/log 的 `NewLogger`（Init 使用）不含 otelHandler；base logging 已如此，PR3 不修。
- **爆炸半径**：conf RedisConfig 类型变化影响 layout.yaml 限流/幂等/nonce、redis_shared.go、redis.go、optional-config/redis.yaml、golden 副本 → 逐提交审查。
- 模板/脚手架输出 contract-sensitive；本 PR 不改 ncgo 业务逻辑。
