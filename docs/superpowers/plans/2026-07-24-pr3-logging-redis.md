# PR3 实施计划 — 生成项目适配 go-tools v0.1.0：logging + redis 接入 go-common/log 与 go-middleware/redis

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 把生成项目的 optional logging add-on 从手写 slog+lumberjack 迁移到 go-common/log，把 redis add-on 从手写 go-redis/v9 迁移到 go-middleware/redis（保留共享池模式），conf 的 RateLimitRedisConfig（~40 字段）替换为 RedisConfig（~24 字段，镜像 redis.Config，time 用 config.Duration + ToMiddlewareConfig 转换层），使 `ncgo new` / `ncgo add infra redis|logging` 生成项目完整对接 go-tools v0.1.0。

**Architecture:** 纯模板/生成代码改动（`internal/assets/_data/` 模板 + 受影响生成代码 + `internal/scaffold/infra/infra.go` goGetDeps），不改 ncgo 业务逻辑。conf 层保留 config.Duration（YAML 写 `"5s"`），通过 ToMiddlewareConfig() 转换为 go-middleware/redis.Config（time.Duration）。redis_shared.go 保留共享池模式，内部改用 go-middleware/redis.NewUniversalClient。logging add-on 删除手写 slog+lumberjack，改用 go-common/log 的 L()/WithCategory/ErrorContext。

**Tech Stack:** Go 1.25（ncgo 构建）/ 生成项目 go 1.26.5 · go-common v0.1.0（log）· go-middleware v0.1.0（redis）· go-framework v0.1.0（config.Duration）· hertz-template per-file yaml + kitex-template · golden 测试 + e2e 编译测试。

## Global Constraints

- **conf 层 config.Duration**：RedisConfig 超时字段用 `config.Duration`（YAML 写 `"5s"`/`"8ms"`），通过 `ToMiddlewareConfig() *mwredis.Config` 转换为 `time.Duration`。避免 yaml.v3 不支持 time.Duration 字符串解析的问题。
- **YAML key 变化**：`dial_timeout_seconds` → `dial_timeout`、`min_retry_backoff_milliseconds` → `min_retry_backoff`（干净切换，沿用总设计决策）。删除 ~16 个 go-redis 调优字段（PoolFIFO/MaxRedirects/ReadOnly/RouteByLatency 等）。
- **保留 redis_shared 共享池**：SharedRedisClient/CloseSharedRedisClient 保留，内部改用 `mwredis.NewUniversalClient(ctx, cfg.ToMiddlewareConfig())`。删除 `RedisUniversalOptions()`。
- **logging 用 go-common/log**：`goclog.L().WithCategory(category).InfoContext(ctx, msg, args...)` / `ErrorContext(ctx, msg, err, args...)`。保留 SinceMS/WithTrafficLane 本地 helper。conf LoggingConfig 保留（映射到 goclog.Config）。
- **go-middleware 依赖**：生成 go.mod 新增 `go-middleware v0.1.0`。goGetDeps 更新（redis 加 go-middleware，logging 删 lumberjack）。
- 模板/脚手架输出 contract-sensitive：golden diff 逐提交审查；golden 更新用精确包路径 `-update-golden`（不传 `./internal/scaffold/...` 全树）。
- 验证链：`go build ./... && go build . && go vet ./... && go test ./... -count=1 && ./scripts/smoke.sh`。
- **不做**：删除 conf LoggingConfig 结构（保留映射）、改 ncgo 业务逻辑、改 go-common/log 或 go-middleware/redis 库本身。

## go-middleware/redis.Config 参考（实测自 module cache）

```go
type Config struct {
    Addrs            []string      `yaml:"addrs"`
    Username         string        `yaml:"username"`
    Password         string        `yaml:"password"`
    DB               int           `yaml:"db"`
    MasterName       string        `yaml:"master_name"`
    SentinelUsername string        `yaml:"sentinel_username"`
    SentinelPassword string        `yaml:"sentinel_password"`
    Protocol         int           `yaml:"protocol"`
    ClientName       string        `yaml:"client_name"`
    PoolSize         int           `yaml:"pool_size"`
    MinIdleConns     int           `yaml:"min_idle_conns"`
    DialTimeout      time.Duration `yaml:"dial_timeout"`
    ReadTimeout      time.Duration `yaml:"read_timeout"`
    WriteTimeout     time.Duration `yaml:"write_timeout"`
    PoolTimeout      time.Duration `yaml:"pool_timeout"`
    ConnMaxIdleTime  time.Duration `yaml:"conn_max_idle_time"`
    ConnMaxLifetime  time.Duration `yaml:"conn_max_lifetime"`
    IdleCheckFrequency time.Duration `yaml:"idle_check_frequency"`
    MaxRetries       int           `yaml:"max_retries"`
    MinRetryBackoff  time.Duration `yaml:"min_retry_backoff"`
    MaxRetryBackoff  time.Duration `yaml:"max_retry_backoff"`
}
```

`NewUniversalClient(ctx context.Context, cfg *Config) (Client, func(), error)` — 自动 Ping，返回 closeFn。

## go-common/log 参考

- `Init(cfg Config, release ReleaseInfo) error` / `L() *Logger` / `Close() error`
- `Logger.WithCategory(category string) *Logger` — 分类子 Logger
- `Logger.ErrorContext(ctx, msg, err, args...)` — 自动提取 oops 错误属性
- 分类常量：`CategoryAccess/Error/Biz/RPC/DB/Panic/Audit/Security/App/Cache/MQ`
- 层 helper：`Access(ctx)/RPC(ctx)/DB(ctx)` 等
- `WithRequestID(ctx, requestID)` / `RequestIDFromContext(ctx)`
- Config：Level/Format/Mode/AddSource/File(FileConfig)/Categories/Masking(MaskConfig)
- ReleaseInfo：ServiceName/Version/GitSHA/BuildTime/Environment/Extra

## File Structure

| 文件 | 动作 | 责任 |
|------|------|------|
| `internal/assets/_data/hertz/hertz-template/conf_go.yaml` | Modify | 删除 RateLimitRedisConfig，新增 RedisConfig + ToMiddlewareConfig + defaultRedisConfig 简化 + mergeRedisConfig 简化 |
| `internal/assets/_data/hertz/hertz-template/conf_dev_yaml.yaml` | Modify | redis YAML key 对齐 redis.Config（删旧 key，改新 key） |
| `internal/assets/_data/hertz/optional-config/redis.yaml` | Modify | 同上 |
| `internal/assets/_data/hertz/optional/redis_shared.go` | Modify | 共享池内部改用 mwredis.NewUniversalClient，删 RedisUniversalOptions |
| `internal/assets/_data/hertz/optional/redis.go` | Modify | NewRedis 用 SharedRedisClient（不变），删 NewRedisWithOptions 或改用 mwredis |
| `internal/assets/_data/hertz/layout.yaml` | Modify | 删 redisUniversalOptions helper，RateLimitRedisConfig→RedisConfig，TestRedisUniversalOptionsMapping→TestRedisConfigToMiddleware |
| `internal/assets/_data/kitex/kitex-template/conf.yaml` | Modify | RateLimitRedisConfig→RedisConfig（kitex 精简版） |
| `internal/assets/_data/kitex/optional/redis.go` | Modify | 改用 mwredis.NewUniversalClient |
| `internal/assets/_data/optional/observability_logging.go` | Modify | 用 go-common/log 重写（~360→~80 行） |
| `internal/assets/_data/hertz/optional/observability_logging.go` | Modify | API 调用改为 goclog |
| `internal/assets/_data/kitex/optional/observability_logging.go` | Modify | API 调用改为 goclog |
| `internal/scaffold/infra/infra.go` | Modify | goGetDeps 更新 |
| `internal/scaffold/{mono,bff,rpc,infra}/testdata/**` | Regenerate | golden 副本 |
| `README.md`/`README.zh-CN.md`/`docs/examples.md`/`docs/examples.zh-CN.md` | Modify | logging/redis 说明（中英对齐） |
| `internal/assets/_data/docs/{hertz,kitex}/design-doc.*.md` | Modify（按需） | 内嵌设计文档同步 |

---

## Task 1: Hertz conf RedisConfig 类型替换（conf_go.yaml）

**Files:**
- Modify: `internal/assets/_data/hertz/hertz-template/conf_go.yaml`

**Interfaces:**
- Produces: `RedisConfig` 结构体（~24 字段，config.Duration）+ `ToMiddlewareConfig()` 转换方法 + 简化 `defaultRedisConfig()` + 简化 `mergeRedisConfig()`。下游 Task 2/3/4/5 依赖这些类型。

- [ ] **Step 1: 跑 hertz golden 基线**

Run: `go test ./internal/scaffold/mono/... ./internal/scaffold/bff/... -count=1`
Expected: PASS（改动前基线）

- [ ] **Step 2: 删除 RateLimitRedisConfig，新增 RedisConfig**

在 `conf_go.yaml` body 中：

1. 删除 `type RateLimitRedisConfig struct { ... }`（~40 字段，约 181–220 行）
2. 删除 `type RedisConfig = RateLimitRedisConfig`（约 222 行）
3. 新增：

```go
type RedisConfig struct {
    Addrs              []string        `yaml:"addrs"`
    Username           string          `yaml:"username"`
    Password           string          `yaml:"password"`
    DB                 int             `yaml:"db"`
    MasterName         string          `yaml:"master_name"`
    SentinelUsername   string          `yaml:"sentinel_username"`
    SentinelPassword   string          `yaml:"sentinel_password"`
    Protocol           int             `yaml:"protocol"`
    ClientName         string          `yaml:"client_name"`
    PoolSize           int             `yaml:"pool_size"`
    MinIdleConns       int             `yaml:"min_idle_conns"`
    DialTimeout        config.Duration `yaml:"dial_timeout"`
    ReadTimeout        config.Duration `yaml:"read_timeout"`
    WriteTimeout       config.Duration `yaml:"write_timeout"`
    PoolTimeout        config.Duration `yaml:"pool_timeout"`
    ConnMaxIdleTime    config.Duration `yaml:"conn_max_idle_time"`
    ConnMaxLifetime    config.Duration `yaml:"conn_max_lifetime"`
    IdleCheckFrequency config.Duration `yaml:"idle_check_frequency"`
    MaxRetries         int             `yaml:"max_retries"`
    MinRetryBackoff    config.Duration `yaml:"min_retry_backoff"`
    MaxRetryBackoff    config.Duration `yaml:"max_retry_backoff"`
}
```

4. conf 需新增 import：`mwredis "github.com/byx-darwin/go-tools/go-middleware/redis"`

5. 新增转换方法：

```go
func (c RedisConfig) ToMiddlewareConfig() *mwredis.Config {
    return &mwredis.Config{
        Addrs:              c.Addrs,
        Username:           c.Username,
        Password:           c.Password,
        DB:                 c.DB,
        MasterName:         c.MasterName,
        SentinelUsername:   c.SentinelUsername,
        SentinelPassword:   c.SentinelPassword,
        Protocol:           c.Protocol,
        ClientName:         c.ClientName,
        PoolSize:           c.PoolSize,
        MinIdleConns:       c.MinIdleConns,
        DialTimeout:        c.DialTimeout.Duration,
        ReadTimeout:        c.ReadTimeout.Duration,
        WriteTimeout:       c.WriteTimeout.Duration,
        PoolTimeout:        c.PoolTimeout.Duration,
        ConnMaxIdleTime:    c.ConnMaxIdleTime.Duration,
        ConnMaxLifetime:    c.ConnMaxLifetime.Duration,
        IdleCheckFrequency: c.IdleCheckFrequency.Duration,
        MaxRetries:         c.MaxRetries,
        MinRetryBackoff:    c.MinRetryBackoff.Duration,
        MaxRetryBackoff:    c.MaxRetryBackoff.Duration,
    }
}
```

- [ ] **Step 3: 更新所有 RateLimitRedisConfig 引用**

conf_go.yaml 中所有 `RateLimitRedisConfig` 改为 `RedisConfig`：
- `RateLimitConfig.Redis` 字段类型
- `IdempotencyConfig.Redis` 字段类型
- `SignatureNonceConfig.Redis` 字段类型
- `Config.Redis` 顶层字段类型（已是 `RedisConfig` alias，现在直接用）

- [ ] **Step 4: 简化 defaultRedisConfig()**

```go
func defaultRedisConfig() RedisConfig {
    return RedisConfig{
        Addrs:           []string{"127.0.0.1:6379"},
        Protocol:        3,
        MaxRetries:      3,
        MinRetryBackoff: config.Duration{Duration: 8 * time.Millisecond},
        MaxRetryBackoff: config.Duration{Duration: 512 * time.Millisecond},
        DialTimeout:     config.Duration{Duration: 5 * time.Second},
        ReadTimeout:     config.Duration{Duration: 3 * time.Second},
        WriteTimeout:    config.Duration{Duration: 3 * time.Second},
        PoolSize:        10,
        MinIdleConns:    2,
        PoolTimeout:     config.Duration{Duration: 4 * time.Second},
        ConnMaxIdleTime: config.Duration{Duration: 300 * time.Second},
        ConnMaxLifetime: config.Duration{Duration: 1800 * time.Second},
    }
}
```

- [ ] **Step 5: 简化 mergeRedisConfig()**

```go
func mergeRedisConfig(primary, fallback RedisConfig) RedisConfig {
    if len(primary.Addrs) == 0 && len(fallback.Addrs) > 0 {
        primary.Addrs = append([]string(nil), fallback.Addrs...)
    }
    if primary.ClientName == "" { primary.ClientName = fallback.ClientName }
    if primary.Username == "" { primary.Username = fallback.Username }
    if primary.Password == "" { primary.Password = fallback.Password }
    if primary.DB == 0 { primary.DB = fallback.DB }
    if primary.MasterName == "" { primary.MasterName = fallback.MasterName }
    if primary.SentinelUsername == "" { primary.SentinelUsername = fallback.SentinelUsername }
    if primary.SentinelPassword == "" { primary.SentinelPassword = fallback.SentinelPassword }
    if primary.Protocol == 0 { primary.Protocol = fallback.Protocol }
    if primary.MaxRetries == 0 { primary.MaxRetries = fallback.MaxRetries }
    if primary.MinRetryBackoff.Duration == 0 { primary.MinRetryBackoff = fallback.MinRetryBackoff }
    if primary.MaxRetryBackoff.Duration == 0 { primary.MaxRetryBackoff = fallback.MaxRetryBackoff }
    if primary.DialTimeout.Duration == 0 { primary.DialTimeout = fallback.DialTimeout }
    if primary.ReadTimeout.Duration == 0 { primary.ReadTimeout = fallback.ReadTimeout }
    if primary.WriteTimeout.Duration == 0 { primary.WriteTimeout = fallback.WriteTimeout }
    if primary.PoolSize == 0 { primary.PoolSize = fallback.PoolSize }
    if primary.MinIdleConns == 0 { primary.MinIdleConns = fallback.MinIdleConns }
    if primary.PoolTimeout.Duration == 0 { primary.PoolTimeout = fallback.PoolTimeout }
    if primary.ConnMaxIdleTime.Duration == 0 { primary.ConnMaxIdleTime = fallback.ConnMaxIdleTime }
    if primary.ConnMaxLifetime.Duration == 0 { primary.ConnMaxLifetime = fallback.ConnMaxLifetime }
    if primary.IdleCheckFrequency.Duration == 0 { primary.IdleCheckFrequency = fallback.IdleCheckFrequency }
    return primary
}
```

- [ ] **Step 6: 验证 ncgo 自身编译**

Run: `go build ./... && go vet ./internal/scaffold/...`
Expected: PASS（模板 YAML 本身不参与 go 编译；此步确保 ncgo 自身无碍）

- [ ] **Step 7: 提交（golden 待 Task 8 统一 regenerate）**

```bash
git add internal/assets/_data/hertz/hertz-template/conf_go.yaml
git commit -m "feat(scaffold): hertz conf RedisConfig 替换 RateLimitRedisConfig + ToMiddlewareConfig 转换层"
```

---

## Task 2: Hertz dev YAML + optional-config/redis.yaml key 对齐

**Files:**
- Modify: `internal/assets/_data/hertz/hertz-template/conf_dev_yaml.yaml`
- Modify: `internal/assets/_data/hertz/optional-config/redis.yaml`

- [ ] **Step 1: 改 conf_dev_yaml.yaml redis 块**

删除旧 key（~16 个 go-redis 调优字段），改为 redis.Config 对齐的 key：

```yaml
  # ncgo:add-infra:start redis
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
  # ncgo:add-infra:end redis
```

- [ ] **Step 2: 改 optional-config/redis.yaml**

同 Step 1 的 key 结构（此文件是 `ncgo add infra redis` 注入的参考配置）。

- [ ] **Step 3: 提交**

```bash
git add internal/assets/_data/hertz/hertz-template/conf_dev_yaml.yaml internal/assets/_data/hertz/optional-config/redis.yaml
git commit -m "feat(scaffold): hertz redis YAML key 对齐 go-middleware/redis.Config"
```

---

## Task 3: redis_shared.go 重写（hertz 共享池 + go-middleware/redis）

**Files:**
- Modify: `internal/assets/_data/hertz/optional/redis_shared.go`

**Interfaces:**
- Consumes: Task 1 产出的 `RedisConfig` + `ToMiddlewareConfig()`。
- Produces: `SharedRedisClient(cfg RedisConfig) redis.UniversalClient`（内部用 mwredis）、`CloseSharedRedisClient(cfg RedisConfig)`、`CloseSharedRedisClients()`。

- [ ] **Step 1: 重写 redis_shared.go**

```go
package data

import (
	"context"
	"encoding/json"
	"sync"

	"github.com/redis/go-redis/v9"

	mwredis "github.com/byx-darwin/go-tools/go-middleware/redis"

	"{{.GoModule}}/internal/base/conf"
)

type RedisConfig = conf.RedisConfig
type Config = conf.Config

var (
	sharedRedisClientsMu sync.Mutex
	sharedRedisClients   = map[string]redis.UniversalClient{}
	sharedRedisCloseFns  = map[string]func(){}
)

func SharedRedisClient(cfg RedisConfig) redis.UniversalClient {
	key := redisClientKey(cfg)
	sharedRedisClientsMu.Lock()
	defer sharedRedisClientsMu.Unlock()
	if client := sharedRedisClients[key]; client != nil {
		return client
	}
	client, closeFn, err := mwredis.NewUniversalClient(context.Background(), cfg.ToMiddlewareConfig())
	if err != nil {
		// 连接失败时返回 nil，由调用方处理（与旧行为一致：Ping 失败由 NewRedis 报错）
		return nil
	}
	sharedRedisClients[key] = client
	sharedRedisCloseFns[key] = closeFn
	return client
}

func CloseSharedRedisClient(cfg RedisConfig) {
	key := redisClientKey(cfg)
	sharedRedisClientsMu.Lock()
	defer sharedRedisClientsMu.Unlock()
	if closeFn := sharedRedisCloseFns[key]; closeFn != nil {
		closeFn()
		delete(sharedRedisCloseFns, key)
	}
	delete(sharedRedisClients, key)
}

func CloseSharedRedisClients() {
	sharedRedisClientsMu.Lock()
	defer sharedRedisClientsMu.Unlock()
	for key, closeFn := range sharedRedisCloseFns {
		if closeFn != nil {
			closeFn()
		}
		delete(sharedRedisClients, key)
		delete(sharedRedisCloseFns, key)
	}
}

func redisClientKey(cfg RedisConfig) string {
	payload, _ := json.Marshal(cfg)
	return string(payload)
}
```

> 注意：`SharedRedisClient` 内部 `mwredis.NewUniversalClient` 已含 Ping。连接失败返回 nil（旧行为是返回 client 后由 NewRedis 的 Ping 报错）。调用方 `NewRedis` 需适配（Task 4）。

- [ ] **Step 2: 提交**

```bash
git add internal/assets/_data/hertz/optional/redis_shared.go
git commit -m "feat(scaffold): redis_shared 共享池改用 go-middleware/redis.NewUniversalClient"
```

---

## Task 4: hertz redis.go 改写

**Files:**
- Modify: `internal/assets/_data/hertz/optional/redis.go`

**Interfaces:**
- Consumes: Task 3 产出的 `SharedRedisClient`/`CloseSharedRedisClient`。
- Produces: `NewRedis(ctx, cfg)` 返回 `(*Redis, func(), error)`。

- [ ] **Step 1: 改写 redis.go**

```go
// Optional Redis add-on for Hertz HTTP services.
//
// To enable: copy this file to internal/base/data/redis.go in your project,
// then register with samber/do:
//
//	do.ProvideValue[context.Context](injector, startupCtx)
//	do.Provide(injector, data.NewRedis)
//
// NewRedis reads cfg.Redis and reuses the shared UniversalClient used by
// redis-backed middleware.
//
// Required dependency:
//
//	go get github.com/byx-darwin/go-tools/go-middleware
//	go get github.com/byx-darwin/go-tools/go-common
//	go get github.com/byx-darwin/go-tools/go-framework

package data

import (
	"context"

	goerror "github.com/byx-darwin/go-tools/go-common/error"
	frameworkerror "github.com/byx-darwin/go-tools/go-framework/error"
	"github.com/redis/go-redis/v9"
)

// CodeCacheUnavailable is the project-segment error code (>=40100) for cache
// backend unavailability.
const CodeCacheUnavailable = 40504

func init() {
	goerror.RegisterHTTPStatuses(map[int]int{CodeCacheUnavailable: 503})
}

// Redis wraps a go-redis UniversalClient (single / cluster / sentinel).
type Redis struct {
	Client redis.UniversalClient
}

// NewRedis reuses the shared Redis client derived from cfg.Redis, validates
// connectivity with the injected startup context, and returns a cleanup
// function for samber/do.
func NewRedis(ctx context.Context, cfg *Config) (*Redis, func(), error) {
	if cfg == nil {
		return nil, nil, goerror.
			In("redis").
			Tags("cache", "redis", "configuration").
			Code(frameworkerror.CodeConfigInvalid).
			Public("config_invalid").
			New("data.Config is nil")
	}
	cli := SharedRedisClient(cfg.Redis)
	if cli == nil {
		return nil, nil, goerror.
			In("redis").
			Tags("cache", "redis", "connection").
			Code(CodeCacheUnavailable).
			Public("cache_unavailable").
			With("addrs_count", len(cfg.Redis.Addrs)).
			New("redis connection failed")
	}
	if err := cli.Ping(ctx).Err(); err != nil {
		CloseSharedRedisClient(cfg.Redis)
		return nil, nil, goerror.
			In("redis").
			Tags("cache", "redis", "connection").
			Code(CodeCacheUnavailable).
			Public("cache_unavailable").
			With("addrs_count", len(cfg.Redis.Addrs)).
			Wrapf(err, "redis.Ping")
	}
	cleanup := func() { CloseSharedRedisClient(cfg.Redis) }
	return &Redis{Client: cli}, cleanup, nil
}
```

> 删除 `NewRedisWithOptions`（YAGNI；若需要专用客户端，直接用 `mwredis.NewClient`）。import 删除 `frameworkerror` 若未用（实际仍用）。

- [ ] **Step 2: 提交**

```bash
git add internal/assets/_data/hertz/optional/redis.go
git commit -m "feat(scaffold): hertz redis add-on 适配 go-middleware/redis 共享池"
```

---

## Task 5: Hertz layout.yaml middleware 适配

**Files:**
- Modify: `internal/assets/_data/hertz/layout.yaml`

**Interfaces:**
- Consumes: Task 1 产出的 `RedisConfig`。
- Produces: 生成项目 middleware 编译通过。

- [ ] **Step 1: 全量定位 RateLimitRedisConfig / redisUniversalOptions 引用**

Run: `grep -nE "RateLimitRedisConfig|redisUniversalOptions|RedisUniversalOptions" internal/assets/_data/hertz/layout.yaml`

- [ ] **Step 2: 删除 redisUniversalOptions helper**

删除 `func redisUniversalOptions(cfg conf.RateLimitRedisConfig) *redis.UniversalOptions { return data.RedisUniversalOptions(cfg) }`（约 4010–4012 行）。

- [ ] **Step 3: 替换 RateLimitRedisConfig → RedisConfig**

所有 `conf.RateLimitRedisConfig` 引用改为 `conf.RedisConfig`。

- [ ] **Step 4: 改写 TestRedisUniversalOptionsMapping**

将 `TestRedisUniversalOptionsMapping` 改为 `TestRedisConfigToMiddleware`：

```go
func TestRedisConfigToMiddleware(t *testing.T) {
    mwCfg := conf.RedisConfig{
        Addrs:          []string{"redis-1:6379", "redis-2:6379"},
        ClientName:     "api",
        Username:       "default",
        Password:       "secret",
        DB:             2,
        MasterName:     "mymaster",
        SentinelUsername: "sentinel-user",
        SentinelPassword: "sentinel-secret",
        Protocol:       3,
        MaxRetries:     4,
        MinRetryBackoff: config.Duration{Duration: 9 * time.Millisecond},
        MaxRetryBackoff: config.Duration{Duration: 600 * time.Millisecond},
        DialTimeout:    config.Duration{Duration: 6 * time.Second},
        ReadTimeout:    config.Duration{Duration: 4 * time.Second},
        WriteTimeout:   config.Duration{Duration: 5 * time.Second},
        PoolSize:       20,
        MinIdleConns:   3,
        PoolTimeout:    config.Duration{Duration: 7 * time.Second},
        ConnMaxIdleTime: config.Duration{Duration: 301 * time.Second},
        ConnMaxLifetime: config.Duration{Duration: 1801 * time.Second},
    }.ToMiddlewareConfig()
    if len(mwCfg.Addrs) != 2 || mwCfg.DB != 2 || mwCfg.MasterName != "mymaster" {
        t.Fatalf("unexpected redis config: %+v", mwCfg)
    }
    if mwCfg.DialTimeout != 6*time.Second || mwCfg.MinRetryBackoff != 9*time.Millisecond {
        t.Fatalf("unexpected redis durations: dial=%s min_retry=%s", mwCfg.DialTimeout, mwCfg.MinRetryBackoff)
    }
}
```

- [ ] **Step 5: 检查 applyRedisFallbacks 测试**

Run: `grep -n "TestApplyRedisFallbacks\|RedisConfig{" internal/assets/_data/hertz/layout.yaml | head -10`

更新 `TestApplyRedisFallbacksUsesTopLevelRedis` 中的 `RedisConfig{...}` 字段为新字段名（删除 `ContextTimeoutEnabled` 等已删字段）。

- [ ] **Step 6: 提交（golden 待 Task 8 统一 regenerate）**

```bash
git add internal/assets/_data/hertz/layout.yaml
git commit -m "feat(scaffold): hertz layout middleware 适配 RedisConfig + 删 redisUniversalOptions"
```

---

## Task 6: Kitex conf + redis.go 改写

**Files:**
- Modify: `internal/assets/_data/kitex/kitex-template/conf.yaml`
- Modify: `internal/assets/_data/kitex/optional/redis.go`

- [ ] **Step 1: 改 kitex conf.yaml RateLimitRedisConfig → RedisConfig**

kitex `RateLimitRedisConfig`（6 字段：Addrs/Password/DB/PoolSize/MinIdleConns/MaxActiveConns）改为 `RedisConfig`（与 hertz 同构或精简子集）。

```go
type RedisConfig struct {
    Addrs          []string        `json:"addrs" yaml:"addrs"`
    Password       string          `json:"password" yaml:"password"`
    DB             int             `json:"db" yaml:"db"`
    PoolSize       int             `json:"pool_size" yaml:"pool_size"`
    MinIdleConns   int             `json:"min_idle_conns" yaml:"min_idle_conns"`
    DialTimeout    config.Duration `json:"dial_timeout" yaml:"dial_timeout"`
    ReadTimeout    config.Duration `json:"read_timeout" yaml:"read_timeout"`
    WriteTimeout   config.Duration `json:"write_timeout" yaml:"write_timeout"`
    PoolTimeout    config.Duration `json:"pool_timeout" yaml:"pool_timeout"`
    ConnMaxIdleTime config.Duration `json:"conn_max_idle_time" yaml:"conn_max_idle_time"`
    ConnMaxLifetime config.Duration `json:"conn_max_lifetime" yaml:"conn_max_lifetime"`
    MaxRetries     int             `json:"max_retries" yaml:"max_retries"`
    MinRetryBackoff config.Duration `json:"min_retry_backoff" yaml:"min_retry_backoff"`
    MaxRetryBackoff config.Duration `json:"max_retry_backoff" yaml:"max_retry_backoff"`
}
```

> kitex 无 redis_shared，不需要 ToMiddlewareConfig（redis.go 直接构造）。但若 kitex redis.go 也用 go-middleware/redis，则同样需要 ToMiddlewareConfig。为一致性，kitex RedisConfig 也加 ToMiddlewareConfig。

- [ ] **Step 2: 改 kitex redis.go**

```go
// Optional Redis add-on for Kitex RPC services.
//
// Required dependency:
//
//	go get github.com/byx-darwin/go-tools/go-middleware
//	go get github.com/byx-darwin/go-tools/go-common

package data

import (
	"context"

	goerror "github.com/byx-darwin/go-tools/go-common/error"
	mwredis "github.com/byx-darwin/go-tools/go-middleware/redis"
	"github.com/redis/go-redis/v9"

	"{{.GoModule}}/internal/base/conf"
)

// Redis wraps a go-redis UniversalClient (single / cluster / sentinel).
type Redis struct {
	Client redis.UniversalClient
}

// NewRedis creates a Redis client from conf.RedisConfig via go-middleware/redis,
// validates connectivity, and returns a cleanup function for samber/do.
func NewRedis(ctx context.Context, cfg *conf.RedisConfig) (*Redis, func(), error) {
	if cfg == nil {
		return nil, nil, goerror.
			In("redis").
			Tags("cache", "redis", "configuration").
			Code("redis_config_missing").
			Public("redis configuration is invalid").
			New("conf.RedisConfig is nil")
	}
	client, closeFn, err := mwredis.NewUniversalClient(ctx, cfg.ToMiddlewareConfig())
	if err != nil {
		return nil, nil, goerror.
			In("redis").
			Tags("cache", "redis", "connection").
			Code("redis_ping_failed").
			Public("redis is unavailable").
			With("addrs_count", len(cfg.Addrs)).
			Wrapf(err, "redis connect")
	}
	return &Redis{Client: client}, closeFn, nil
}
```

- [ ] **Step 3: 提交**

```bash
git add internal/assets/_data/kitex/kitex-template/conf.yaml internal/assets/_data/kitex/optional/redis.go
git commit -m "feat(scaffold): kitex conf RedisConfig + redis add-on 改用 go-middleware/redis"
```

---

## Task 7: Logging add-on 重写（go-common/log）

**Files:**
- Modify: `internal/assets/_data/optional/observability_logging.go`
- Modify: `internal/assets/_data/hertz/optional/observability_logging.go`
- Modify: `internal/assets/_data/kitex/optional/observability_logging.go`

**Interfaces:**
- Produces: 生成项目 logging 基于 go-common/log；hertz/kitex 中间件保留骨架。

- [ ] **Step 1: 重写共享 optional/observability_logging.go**

```go
// Optional structured logging add-on for Hertz and Kitex services.
//
// It provides category-based structured logging through go-common/log,
// request/trace context injection, and go-common/error metadata extraction.
//
// Required dependencies:
//
//	go get github.com/byx-darwin/go-tools/go-common

package logging

import (
	"context"
	"log/slog"
	"time"

	goclog "github.com/byx-darwin/go-tools/go-common/log"
)

// Re-export category constants from go-common/log.
const (
	CategoryAccess   = goclog.CategoryAccess
	CategoryError    = goclog.CategoryError
	CategoryBiz      = goclog.CategoryBiz
	CategoryRPC      = goclog.CategoryRPC
	CategoryDB       = goclog.CategoryDB
	CategoryPanic    = goclog.CategoryPanic
	CategoryAudit    = goclog.CategoryAudit
	CategorySecurity = goclog.CategorySecurity
)

// InitFromConf initializes the global logger from conf.LoggingConfig.
func InitFromConf(cfg LoggingConfig, release goclog.ReleaseInfo) error {
	gcfg := goclog.Config{
		Level:     cfg.Level,
		Format:    cfg.Format,
		Mode:      cfg.Mode,
		AddSource: cfg.AddSource,
		File: goclog.FileConfig{
			Dir:        cfg.File.Dir,
			Filename:   cfg.File.Filename,
			MaxSize:    cfg.File.MaxSizeMB,
			MaxBackups: cfg.File.MaxBackups,
			MaxAge:     cfg.File.MaxAgeDays,
			Compress:   cfg.File.Compress,
		},
	}
	return goclog.Init(gcfg, release)
}

// L returns the global logger.
func L() *goclog.Logger { return goclog.L() }

type contextKey string

const (
	ContextKeyRequestID       contextKey = "request_id"
	ContextKeyRequestIDSource contextKey = "request_id_source"
	ContextKeyTrafficLane     contextKey = "traffic_lane"
)

func WithRequestID(ctx context.Context, requestID, source string) context.Context {
	ctx = context.WithValue(ctx, ContextKeyRequestID, requestID)
	ctx = context.WithValue(ctx, ContextKeyRequestIDSource, source)
	return goclog.WithRequestID(ctx, requestID)
}

func WithTrafficLane(ctx context.Context, lane string) context.Context {
	return context.WithValue(ctx, ContextKeyTrafficLane, lane)
}

func SinceMS(started time.Time) slog.Attr {
	return slog.Float64("latency_ms", float64(time.Since(started).Microseconds())/1000)
}
```

> 注意：`LoggingConfig` 类型来自 conf 包。共享文件不能 import conf（因 hertz/kitex conf 不同）。需要把 `InitFromConf` 的签名改为接受 `goclog.Config` 或在 hertz/kitex 各自文件中做转换。
>
> **修正**：共享文件不定义 `InitFromConf`（因依赖 conf 类型）。改为在 hertz/kitex 各自的中间件文件中提供 `InitFromConf`。共享文件只保留 category 常量、context helper、SinceMS。

修正后的共享文件：

```go
package logging

import (
	"context"
	"log/slog"
	"time"

	goclog "github.com/byx-darwin/go-tools/go-common/log"
)

const (
	CategoryAccess   = goclog.CategoryAccess
	CategoryError    = goclog.CategoryError
	CategoryBiz      = goclog.CategoryBiz
	CategoryRPC      = goclog.CategoryRPC
	CategoryDB       = goclog.CategoryDB
	CategoryPanic    = goclog.CategoryPanic
	CategoryAudit    = goclog.CategoryAudit
	CategorySecurity = goclog.CategorySecurity
)

type contextKey string

const (
	ContextKeyRequestID       contextKey = "request_id"
	ContextKeyRequestIDSource contextKey = "request_id_source"
	ContextKeyTrafficLane     contextKey = "traffic_lane"
)

func WithRequestID(ctx context.Context, requestID, source string) context.Context {
	ctx = context.WithValue(ctx, ContextKeyRequestID, requestID)
	ctx = context.WithValue(ctx, ContextKeyRequestIDSource, source)
	return goclog.WithRequestID(ctx, requestID)
}

func WithTrafficLane(ctx context.Context, lane string) context.Context {
	return context.WithValue(ctx, ContextKeyTrafficLane, lane)
}

func SinceMS(started time.Time) slog.Attr {
	return slog.Float64("latency_ms", float64(time.Since(started).Microseconds())/1000)
}
```

- [ ] **Step 2: 改写 hertz/optional/observability_logging.go**

```go
// Optional Hertz logging middleware for internal/base/logging.

package logging

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"time"

	goclog "github.com/byx-darwin/go-tools/go-common/log"
	goerror "github.com/byx-darwin/go-tools/go-common/error"
	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/middlewares/server/recovery"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	oteltrace "go.opentelemetry.io/otel/trace"
)

const (
	HeaderRequestID            = "X-Request-ID"
	HeaderTrafficLane          = "X-Traffic-Lane"
	HertzContextKeyRequestID   = "request_id"
	HertzContextKeyTrafficLane = "traffic_lane"
)

// InitFromConf initializes the global logger from hertz conf.LoggingConfig.
func InitFromConf(cfg LoggingConfig, release goclog.ReleaseInfo) error {
	gcfg := goclog.Config{
		Level:     cfg.Level,
		Format:    cfg.Format,
		Mode:      cfg.Mode,
		AddSource: cfg.AddSource,
		File: goclog.FileConfig{
			Dir:        cfg.File.Dir,
			Filename:   cfg.File.Filename,
			MaxSize:    cfg.File.MaxSizeMB,
			MaxBackups: cfg.File.MaxBackups,
			MaxAge:     cfg.File.MaxAgeDays,
			Compress:   cfg.File.Compress,
		},
	}
	return goclog.Init(gcfg, release)
}

func HertzRequestID() app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		requestID, source := hertzRequestID(ctx, c)
		lane := c.Request.Header.Get(HeaderTrafficLane)
		ctx = WithRequestID(ctx, requestID, source)
		if lane != "" {
			ctx = WithTrafficLane(ctx, lane)
			c.Set(HertzContextKeyTrafficLane, lane)
		}
		c.Set(HertzContextKeyRequestID, requestID)
		c.Response.Header.Set(HeaderRequestID, requestID)
		c.Next(ctx)
	}
}

func HertzAccessLog() app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		started := time.Now()
		c.Next(ctx)
		status := c.Response.StatusCode()
		if status == 0 {
			status = consts.StatusOK
		}
		goclog.Access(ctx).InfoContext(ctx, "hertz access",
			"http.method", string(c.Method()),
			"http.path", string(c.Path()),
			"http.status_code", status,
			"latency_ms", float64(time.Since(started).Microseconds())/1000,
		)
	}
}

func HertzRecovery() app.HandlerFunc {
	return recovery.Recovery(recovery.WithRecoveryHandler(func(ctx context.Context, c *app.RequestContext, recovered interface{}, stack []byte) {
		err := goerror.In("hertz.recovery").
			Tags("panic", "hertz").
			Code("panic_recovered").
			With("panic", fmt.Sprint(recovered)).
			With("stack", string(stack)).
			New("hertz panic recovered")
		goclog.L().WithCategory(goclog.CategoryPanic).ErrorContext(ctx, "hertz panic recovered", err)
		c.Response.SetStatusCode(consts.StatusInternalServerError)
		c.Abort()
	}))
}

func HertzRequestIDFromContext(c *app.RequestContext) string {
	value, ok := c.Get(HertzContextKeyRequestID)
	if !ok {
		return ""
	}
	requestID, _ := value.(string)
	return requestID
}

func hertzRequestID(ctx context.Context, c *app.RequestContext) (string, string) {
	if requestID := c.Request.Header.Get(HeaderRequestID); requestID != "" {
		return requestID, "header"
	}
	spanCtx := oteltrace.SpanContextFromContext(ctx)
	if spanCtx.IsValid() {
		return spanCtx.TraceID().String(), "trace_id"
	}
	return newHertzRequestID(), "generated"
}

func newHertzRequestID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return time.Now().UTC().Format("20060102150405.000000000")
	}
	return hex.EncodeToString(b[:])
}
```

> 注意：`LoggingConfig` 类型需从 conf 包 import。hertz 中间件文件在生成项目中位于 `internal/base/logging/`，可以 import `{{.GoModule}}/internal/base/conf`。

- [ ] **Step 3: 改写 kitex/optional/observability_logging.go**

同理，`KitexAccessLog` 改用 `goclog.RPC(ctx).InfoContext(...)`，`KitexRecovery` 改用 `goclog.L().WithCategory(goclog.CategoryPanic).ErrorContext(...)`。保留 `KitexRequestID`/`KitexMetaValue`/`kitexRPCNames` 骨架。

- [ ] **Step 4: 提交**

```bash
git add internal/assets/_data/optional/observability_logging.go internal/assets/_data/hertz/optional/observability_logging.go internal/assets/_data/kitex/optional/observability_logging.go
git commit -m "feat(scaffold): logging add-on 重写为 go-common/log（删 slog+lumberjack）"
```

---

## Task 8: goGetDeps 更新（infra.go）

**Files:**
- Modify: `internal/scaffold/infra/infra.go`
- Modify: `internal/scaffold/infra/infra_test.go`

- [ ] **Step 1: 写失败测试**

在 `infra_test.go` 加：

```go
func TestGoGetDepsRedisIncludesGoMiddleware(t *testing.T) {
    deps := goGetDeps[KindRedis]
    found := false
    for _, d := range deps {
        if d == "github.com/byx-darwin/go-tools/go-middleware" {
            found = true
        }
    }
    if !found {
        t.Errorf("goGetDeps[redis] missing go-middleware: %v", deps)
    }
}

func TestGoGetDepsLoggingNoLumberjack(t *testing.T) {
    deps := goGetDeps[KindObservabilityLog]
    for _, d := range deps {
        if d == "gopkg.in/natefinch/lumberjack.v2" {
            t.Errorf("goGetDeps[logging] should not contain lumberjack: %v", deps)
        }
    }
}
```

- [ ] **Step 2: 跑测试确认 FAIL**

Run: `go test ./internal/scaffold/infra/... -run 'TestGoGetDepsRedisIncludesGoMiddleware|TestGoGetDepsLoggingNoLumberjack' -count=1`
Expected: FAIL

- [ ] **Step 3: 更新 goGetDeps**

```go
var goGetDeps = map[string][]string{
    KindRedis:        {"github.com/byx-darwin/go-tools/go-middleware", "github.com/byx-darwin/go-tools/go-common"},
    KindKafka:        {"github.com/segmentio/kafka-go", "github.com/byx-darwin/go-tools/go-common"},
    KindES:           {"github.com/elastic/go-elasticsearch/v8", "github.com/byx-darwin/go-tools/go-common"},
    KindClickHouse:   {"github.com/ClickHouse/clickhouse-go/v2", "github.com/byx-darwin/go-tools/go-common"},
    KindRegistryEtcd: {"github.com/kitex-contrib/registry-etcd", "github.com/byx-darwin/go-tools/go-common"},
    KindObservabilityLog: {
        "github.com/byx-darwin/go-tools/go-common",
    },
}
```

> 删除 `"github.com/redis/go-redis/v9"`（go-middleware 传递依赖）、`"gopkg.in/natefinch/lumberjack.v2"`（go-common/log 内置）、`"go.opentelemetry.io/otel/trace"`（go-common/log 传递依赖）。

- [ ] **Step 4: 跑测试确认 PASS + 提交**

Run: `go test ./internal/scaffold/infra/... -count=1`
Expected: PASS

```bash
git add internal/scaffold/infra/infra.go internal/scaffold/infra/infra_test.go
git commit -m "feat(scaffold): infra goGetDeps 更新（redis→go-middleware，logging 删 lumberjack）"
```

---

## Task 9: Golden 重新生成 + e2e 编译验证

**Files:**
- Regenerate: `internal/scaffold/{mono,bff,rpc,infra}/testdata/**`

- [ ] **Step 1: 重新生成受影响 golden（精确包路径）**

Run: `go test ./internal/scaffold/mono/... ./internal/scaffold/bff/... ./internal/scaffold/rpc/... ./internal/scaffold/infra/... -update-golden -count=1`

- [ ] **Step 2: 逐提交审查 golden diff**

Run: `git diff internal/scaffold/*/testdata/ | head -500`
Expected: 仅 conf.go（RedisConfig 替换）、conf dev YAML（key 变化）、redis_shared.go、redis.go、observability_logging.go、layout 消费方、server.go 的对应变化；无误 bless 的无关文件。

- [ ] **Step 3: 跑 golden 测试**

Run: `go test ./internal/scaffold/mono/... ./internal/scaffold/bff/... ./internal/scaffold/rpc/... ./internal/scaffold/infra/... -count=1`
Expected: PASS

- [ ] **Step 4: e2e 编译验证**

Run: `go test ./internal/scaffold/mono/... -run 'TestGenerateHertzCompiles|TestGenerateHertzWithDatabaseCompiles|TestGenerateKitexCompiles' -count=1`
Expected: PASS（需 hz/kitex/make + go 1.26.5 + proxy 网络；环境缺失则 skip 并明确标注）

- [ ] **Step 5: 提交 golden**

```bash
git add internal/scaffold/*/testdata/
git commit -m "test(scaffold): 重新生成 golden（logging go-common/log + redis go-middleware/redis）"
```

---

## Task 10: 文档（中英对齐）

**Files:**
- Modify: `README.md`/`README.zh-CN.md`/`docs/examples.md`/`docs/examples.zh-CN.md`
- Modify（按需）: `internal/assets/_data/docs/{hertz,kitex}/design-doc.{en,zh-CN}.md`

- [ ] **Step 1: 先 Read 现状，增量补充**

记录：生成项目 logging 基于 go-common/log（WithCategory + masking + OTel trace context）；redis add-on 基于 go-middleware/redis（NewUniversalClient，redis.Config 用 config.Duration）；redis YAML key 变化（dial_timeout_seconds → dial_timeout）。

- [ ] **Step 2: EN/ZH 对齐 + markdown 诊断**

Run: `grep -rn "go-middleware\|go-common/log\|redis.Config\|dial_timeout" README.md README.zh-CN.md docs/examples.md docs/examples.zh-CN.md`（确认中英一致）

- [ ] **Step 3: 提交**

```bash
git add README.md README.zh-CN.md docs/examples.md docs/examples.zh-CN.md internal/assets/_data/docs/
git commit -m "docs: 生成项目 logging 基于 go-common/log + redis 基于 go-middleware/redis（中英对齐）"
```

---

## Task 11: 全量验证 + PR 准备

- [ ] **Step 1: 全量验证链**

Run: `go build ./... && go build . && go vet ./... && go test ./... -count=1 && ./scripts/smoke.sh`
Expected: 全绿。

- [ ] **Step 2: gofmt + 复核 diff 范围**

Run: `gofmt -l $(find . -name '*.go' -not -path './.git/*')`（无输出）；`git diff --stat origin/main...HEAD` 抽查仅模板/生成代码/golden/docs/infra.go。

- [ ] **Step 3: 创建 PR（orchestrator 执行，body 含 `Closes #8`）**

PR 描述含：范围重估说明（base logging 已由 main 完成）、logging add-on 重写、redis 类型替换 + 共享池保留、YAML key 变化（不兼容）、goGetDeps 更新、golden diff 审查结论、e2e 编译结果、与后续 PR 的关系、已知取舍（OTel trace 注入缺口、SharedRedisClient 连接失败返回 nil）。

## 验证顺序

1. 聚焦：`go build ./... && go vet ./internal/scaffold/...`（模板改动不破坏 ncgo 自身）
2. 包级：mono/bff/rpc/infra golden 测试（Task 9）
3. e2e：`TestGenerate*Compiles`（Task 9 Step 4）
4. 全量：`go test ./... -count=1` + `go vet` + `go build`
5. smoke：`./scripts/smoke.sh`

## 风险

- **SharedRedisClient 连接失败行为变化**：旧版返回 client（Ping 由 NewRedis 做），新版 `mwredis.NewUniversalClient` 内部 Ping 失败返回 error → SharedRedisClient 返回 nil。调用方需适配（Task 4 已处理）。
- **YAML key 不兼容**：`dial_timeout_seconds` → `dial_timeout`，旧配置不再兼容（干净切换，沿用总设计决策）。
- **OTel trace 注入缺口**：go-common/log 的 `NewLogger`（Init 使用）不含 otelHandler；base logging 已如此，PR3 不修。
- **爆炸半径**：conf RedisConfig 类型变化影响 layout.yaml 限流/幂等/nonce、redis_shared.go、redis.go、optional-config/redis.yaml、golden 副本 → 逐提交审查。
- 模板/脚手架输出 contract-sensitive；本 PR 不改 ncgo 业务逻辑。
