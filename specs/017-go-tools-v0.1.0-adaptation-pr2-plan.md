# PR2 实施计划 — 生成项目适配 go-tools v0.1.0：config duration 化 + redis 铺垫（R-A）

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 把 hertz + kitex 生成项目 conf 里的 int 秒/毫秒 duration 字段统一迁移到 `config.Duration`（YAML 写 `"30s"`/`"8ms"`），并对 redis 配置块做 R-A 铺垫（duration 化、不换类型、客户端接线留 PR3），使 `ncgo new` 生成项目可编译且 duration 配置统一为 go-framework `config.Duration`。

**Architecture:** 纯模板/生成代码改动（`internal/assets/_data/` 模板 + 受影响生成代码），不改 ncgo 业务逻辑、不新增 go.mod 依赖（`config.Duration` 属已 require 的 `go-framework/config`）。字段采取「保名换类型」：字段名与 YAML key 不变，类型 `int`→`config.Duration`，YAML 值由裸整数改为 `"30s"` 字符串（干净切换，沿用总设计决策）。消费方按统一规则适配（见 Global Constraints），由 PR1 的 e2e 编译测试兜底。

**Tech Stack:** Go 1.25（ncgo 构建）/ 生成项目 go 1.26.5 · go-framework v0.1.0（`config.Duration`/`config.LoadYAML`）· hertz-template per-file yaml + kitex-template · golden 测试 + e2e 编译测试。

## Global Constraints

- **保名换类型**：字段名/YAML key 保持不变（如 `window_seconds`），仅类型 `int`→`config.Duration`，YAML 值改字符串（`60` → `"60s"`、`8` → `"8ms"`）。避免大面积 key 重命名 diff。
- **统一适配规则**（凡读取被迁移字段 `X` 的生成代码）：
  - 取 `time.Duration`：`time.Duration(cfg.X) * time.Second` → `cfg.X.Duration`（毫秒字段同理，因 `config.Duration` 已含单位）。
  - 本地 `durationSeconds(cfg.X)` / `durationMilliseconds(cfg.X)` 包装 → `cfg.X.Duration`；若 helper 无其它引用则删除。
  - 比较：`cfg.X <= 0` / `cfg.X < 0` / `cfg.X > 0` → `cfg.X.Duration <= 0` 等。
  - 默认值（`Default()` / fallback 赋值）：`cfg.X = 300` → `cfg.X = config.Duration{Duration: 300 * time.Second}`（毫秒：`200 * time.Millisecond`）。
  - 需要 int 秒/毫秒的边界（lua 脚本参数、DB 行、protobuf、add-on 自有选项）：`cfg.X` → `int(cfg.X.Seconds())` 或 `int(cfg.X.Milliseconds())`；反向由 int 构造 `config.Duration{Duration: time.Duration(n) * time.Second}`。
- conf 模板需 `import "time"`（用于 `Default()` 的 `config.Duration{Duration: ...}`）与 `config "github.com/byx-darwin/go-tools/go-framework/config"`（已 import）。
- **不迁移**（非时长）：`MaxConns/MinConns`、`MaxEntries`、`MaxRequests`、`MaxBodyBytes`、`PoolSize`、`Priority`、`Burst`、`RequestsPerSecond`、`DB`、`Protocol`、端口等计数/尺寸字段。
- **不做**：换 redis 类型别名到 `go-middleware/redis.Config`、重写客户端构造、接线 rate_limit/idempotency/nonce 的 redis 消费方、新增 captcha/observability 选项类型（均见设计文档 §3.3/§3.4，属 PR3 或 YAGNI）。
- 模板/脚手架输出 contract-sensitive：golden diff 逐提交审查；golden 更新用精确包路径 `-update-golden`（不传 `./internal/scaffold/...` 全树）。
- 验证链：`go build ./... && go build . && go vet ./... && go test ./... -count=1 && ./scripts/smoke.sh`。
- **add-on 独立配置**：自带 duration 字段且不读被迁移 conf 字段的 add-on（如 kitex `registry_etcd.go` 自有 `DialTimeoutSeconds`）不改；仅在读取被迁移 conf 字段的边界处适配。

## 被迁移字段清单（源：设计文档 §3.1/§3.3）

**Hertz conf（`internal/assets/_data/hertz/hertz-template/conf_go.yaml`）**：
`ConfigCenter.TimeoutMilliseconds`；`Database.{MaxConnLifetimeSeconds,MaxConnIdleTimeSeconds,HealthCheckPeriodSeconds}`；`CORS.MaxAgeSeconds`；`RateLimit.Source.CacheTTLSeconds`；`RateLimit.GRPC.TimeoutMilliseconds`；`RateLimit.Database.QueryTimeoutMilliseconds`；`RateLimit.RuleCenter.QueryTimeoutMilliseconds`；`RateLimitRule.{WindowSeconds,ClientTTLSeconds}`；`Idempotency.TTLSeconds`；`Signature.MaxClockSkewSeconds`；`Signature.Nonce.TTLSeconds`；`Token.{BufferSeconds,ExpiresSeconds}`；redis（R-A）：`{MinRetryBackoffMilliseconds,MaxRetryBackoffMilliseconds,DialTimeoutSeconds,DialerRetryTimeoutMilliseconds,ReadTimeoutSeconds,WriteTimeoutSeconds,PoolTimeoutSeconds,ConnMaxIdleTimeSeconds,ConnMaxLifetimeSeconds,ConnMaxLifetimeJitterSeconds,FailingTimeoutSeconds}`。

**Kitex conf（`internal/assets/_data/kitex/kitex-template/conf.yaml`）**：
`RPC.RequestTimeoutSeconds`；`Database.{MaxConnLifetimeSeconds,MaxConnIdleTimeSeconds,HealthCheckPeriodSeconds}`；`RateLimit.Source.CacheTTLSeconds`；`RateLimit.GRPC.TimeoutMilliseconds`；`RateLimit.Database.QueryTimeoutMilliseconds`；`RateLimitRule.{WindowSeconds,ClientTTLSeconds}`。（kitex `RateLimitRedisConfig` 6 字段无超时项，不迁移；Server 超时已 `time.Duration`。）

## File Structure

| 文件 | 动作 | 责任 |
|------|------|------|
| `internal/assets/_data/hertz/hertz-template/conf_go.yaml` | Modify | hertz conf 结构体字段类型 + `Default()` + `Validate()` |
| `internal/assets/_data/hertz/hertz-template/conf_dev_yaml.yaml` | Modify | hertz dev YAML 样例值 → `"30s"` 字符串 |
| `internal/assets/_data/hertz/hertz-template/data_go.yaml` | Modify | database/redis 超时消费方适配 |
| `internal/assets/_data/hertz/layout.yaml` | Modify | CORS/签名/幂等/限流（含动态规则源、lua、DB 行边界）消费方适配 |
| `internal/assets/_data/hertz/optional/redis_shared.go` | Modify | R-A：redis 超时字段 `.Duration` + 删本地 helper |
| `internal/assets/_data/hertz/optional/rule_center_client.go` | Modify | RPC/Connect 超时边界（读 conf 处）适配 |
| `internal/assets/_data/kitex/kitex-template/conf.yaml` | Modify | kitex conf 结构体 + `Default()` + `Validate()` |
| `internal/assets/_data/kitex/kitex-template/conf_dev.yaml` | Modify | kitex dev YAML 样例值 → 字符串 |
| `internal/assets/_data/kitex/kitex-template/server.yaml` | Modify | `RequestTimeout` 消费方 + 本地 helper 处理 |
| `internal/assets/_data/kitex/kitex-template/ratelimit_usecase.yaml` | Modify（按需） | 若消费被迁移 conf 字段则适配（多为 IDL/pb 类型，grep 确认） |
| `internal/scaffold/{mono,bff,rpc,infra}/testdata/**` | Regenerate | golden 副本 |
| `README.md`/`README.zh-CN.md`/`docs/examples.md`/`docs/examples.zh-CN.md` | Modify | duration 配置说明（中英对齐） |
| `internal/assets/_data/docs/{hertz,kitex}/design-doc.*.md` | Modify（按需） | 内嵌设计文档 conf 字段说明同步 |

---

## Task 1: Hertz conf 结构体 duration 字段类型迁移（conf_go.yaml）

**Files:**
- Modify: `internal/assets/_data/hertz/hertz-template/conf_go.yaml`
- Regenerate: 含 conf.go 的 hertz golden（mono/bff）

**Interfaces:**
- Produces: `Config` 各 duration 字段类型变为 `config.Duration`；`Default()` 返回带 `config.Duration{Duration: ...}` 的默认值；`Validate()` 用 `.Duration` 比较。下游 Task 3/6 依赖这些字段名不变、类型为 `config.Duration`。

- [ ] **Step 1: 跑 hertz golden 基线**

Run: `go test ./internal/scaffold/mono/... ./internal/scaffold/bff/... -count=1`
Expected: PASS（改动前基线）

- [ ] **Step 2: 改结构体字段类型（保名换类型）**

在 `conf_go.yaml` body 中，把「被迁移字段清单」中 hertz 各字段类型改为 `config.Duration`。示例（其余字段按清单逐一改）：

```go
type ConfigCenterConfig struct {
    Enabled             bool            `yaml:"enabled"`
    Provider            string          `yaml:"provider"`
    DataType            string          `yaml:"data_type"`
    TimeoutMilliseconds config.Duration `yaml:"timeout_milliseconds"`  // 原 int，保留字段名仅换类型
    FailFast            bool            `yaml:"fail_fast"`
    AllowEmpty          bool            `yaml:"allow_empty"`
    ...
}
```

> 保名换类型：Go 字段名与 YAML key 均保持不变（`TimeoutMilliseconds`、`WindowSeconds`、`DialTimeoutSeconds` 等），仅类型 `int`→`config.Duration`，避免消费方字段名连锁改动与大面积 key 重命名 diff。

redis（R-A）：`RateLimitRedisConfig` 中 11 个超时字段改 `config.Duration`（`MinRetryBackoffMilliseconds`、`MaxRetryBackoffMilliseconds`、`DialTimeoutSeconds`、`DialerRetryTimeoutMilliseconds`、`ReadTimeoutSeconds`、`WriteTimeoutSeconds`、`PoolTimeoutSeconds`、`ConnMaxIdleTimeSeconds`、`ConnMaxLifetimeSeconds`、`ConnMaxLifetimeJitterSeconds`、`FailingTimeoutSeconds`）；非超时字段（`Addrs`、`PoolSize`、`Protocol` 等）保持。

- [ ] **Step 3: 改 `Default()` 默认值**

conf 需 `import "time"`。把 int 字面量改 `config.Duration{Duration: ...}`：

```go
ConfigCenter: ConfigCenterConfig{
    DataType:            "yaml",
    TimeoutMilliseconds: config.Duration{Duration: 2000 * time.Millisecond},
    ...
},
Database: DatabaseConfig{
    MaxConnLifetimeSeconds:   config.Duration{Duration: 1800 * time.Second},
    MaxConnIdleTimeSeconds:   config.Duration{Duration: 300 * time.Second},
    HealthCheckPeriodSeconds: config.Duration{Duration: 30 * time.Second},
    ...
},
```

`defaultRedisConfig()` 同步：`DialTimeoutSeconds: config.Duration{Duration: 5 * time.Second}`、`MinRetryBackoffMilliseconds: config.Duration{Duration: 8 * time.Millisecond}` 等。

- [ ] **Step 4: 改 `Validate()` 比较**

`if c.Database.MaxConnLifetimeSeconds < 0` → `if c.Database.MaxConnLifetimeSeconds.Duration < 0`；其余被迁移字段的 `< 0`/`<= 0` 比较同理加 `.Duration`。

- [ ] **Step 5: 验证模板渲染无语法错误（先不跑 golden 比对）**

Run: `go build ./... && go vet ./internal/scaffold/...`
Expected: PASS（模板 YAML 本身不参与 go 编译；此步确保 ncgo 自身无碍）

- [ ] **Step 6: 暂不重新生成 golden**（消费方 Task 3/6 完成后再统一 regenerate，避免中间态 diff 噪音）。提交结构体改动：

```bash
git add internal/assets/_data/hertz/hertz-template/conf_go.yaml
git commit -m "feat(scaffold): hertz conf duration 字段迁移 config.Duration（结构体+Default+Validate）"
```

---

## Task 2: Hertz dev YAML 样例值字符串化（conf_dev_yaml.yaml）

**Files:**
- Modify: `internal/assets/_data/hertz/hertz-template/conf_dev_yaml.yaml`

- [ ] **Step 1: 把 duration 字段 YAML 值由裸整数改为字符串**

示例：

```yaml
database:
  max_conn_lifetime_seconds: "1800s"
  max_conn_idle_time_seconds: "300s"
  health_check_period_seconds: "30s"
redis:
  min_retry_backoff_milliseconds: "8ms"
  max_retry_backoff_milliseconds: "512ms"
  dial_timeout_seconds: "5s"
  dialer_retry_timeout_milliseconds: "100ms"
  read_timeout_seconds: "3s"
  write_timeout_seconds: "3s"
  pool_timeout_seconds: "4s"
  conn_max_idle_time_seconds: "300s"
  conn_max_lifetime_seconds: "1800s"
  conn_max_lifetime_jitter_seconds: "0s"
  failing_timeout_seconds: "15s"
```

> conf_dev_yaml.yaml 当前未列 rate_limit/auth/cors 的 duration 细节（多为注释或省略）；仅对实际出现的 duration 键改字符串。按文件实际内容逐一处理，不新增键。

- [ ] **Step 2: 提交**

```bash
git add internal/assets/_data/hertz/hertz-template/conf_dev_yaml.yaml
git commit -m "feat(scaffold): hertz dev conf 样例 duration 值改为 \"30s\" 字符串格式"
```

---

## Task 3: Hertz 消费方适配（data_go.yaml + layout.yaml）

**Files:**
- Modify: `internal/assets/_data/hertz/hertz-template/data_go.yaml`
- Modify: `internal/assets/_data/hertz/layout.yaml`

**Interfaces:**
- Consumes: Task 1 产出的 `config.Duration` 字段。
- Produces: 生成项目消费 conf duration 字段的代码编译通过（e2e 验证）。

- [ ] **Step 1: 全量定位消费方**

Run: `grep -rnE "\.(WindowSeconds|ClientTTLSeconds|CacheTTLSeconds|TimeoutMilliseconds|QueryTimeoutMilliseconds|TTLSeconds|MaxAgeSeconds|MaxClockSkewSeconds|BufferSeconds|ExpiresSeconds|MaxConnLifetimeSeconds|MaxConnIdleTimeSeconds|HealthCheckPeriodSeconds|ConnMaxLifetimeJitterSeconds|FailingTimeoutSeconds|DialerRetryTimeoutMilliseconds|MinRetryBackoffMilliseconds|MaxRetryBackoffMilliseconds|DialTimeoutSeconds|ReadTimeoutSeconds|WriteTimeoutSeconds|PoolTimeoutSeconds|ConnMaxIdleTimeSeconds|ConnMaxLifetimeSeconds)" internal/assets/_data/hertz/hertz-template/data_go.yaml internal/assets/_data/hertz/layout.yaml`

- [ ] **Step 2: 适配 data_go.yaml（database/redis 超时）**

```go
// database（原 time.Duration(cfg.MaxConnLifetimeSeconds) * time.Second）
pgCfg.MaxConnLifetime = cfg.MaxConnLifetimeSeconds.Duration
pgCfg.MaxConnIdleTime = cfg.MaxConnIdleTimeSeconds.Duration
pgCfg.HealthCheckPeriod = cfg.HealthCheckPeriodSeconds.Duration
// redis（原 time.Duration(cfg.MinRetryBackoffMilliseconds) * time.Millisecond）
MinRetryBackoff: cfg.MinRetryBackoffMilliseconds.Duration,
MaxRetryBackoff: cfg.MaxRetryBackoffMilliseconds.Duration,
DialTimeout:     cfg.DialTimeoutSeconds.Duration,
ReadTimeout:     cfg.ReadTimeoutSeconds.Duration,
WriteTimeout:    cfg.WriteTimeoutSeconds.Duration,
// 其余 PoolTimeout/ConnMaxIdleTime/ConnMaxLifetime/ConnMaxLifetimeJitter 同理
```

- [ ] **Step 3: 适配 layout.yaml — 直接取 duration 处**

```go
// 签名 nonce（原 durationSeconds(cfg.Nonce.TTLSeconds)）
nonceTTL := cfg.Nonce.TTLSeconds.Duration
// CORS Max-Age（原 strconv.Itoa(cfg.MaxAgeSeconds)）
c.Response.Header.Set("Access-Control-Max-Age", strconv.Itoa(int(cfg.MaxAgeSeconds.Seconds())))
// 限流 grpc/database/rule_center 源（原 durationMilliseconds(cfg.GRPC.TimeoutMilliseconds)）
dynamic = &grpcSource{client: opts.GRPC, timeout: cfg.GRPC.TimeoutMilliseconds.Duration}
dynamic = &databaseSource{hook: opts.Database, timeout: cfg.Database.QueryTimeoutMilliseconds.Duration}
dynamic = &grpcSource{client: opts.RuleCenter, timeout: cfg.RuleCenter.QueryTimeoutMilliseconds.Duration}
dynamic = newCachedSource(cfg.Source.CacheTTLSeconds.Duration, dynamic)  // 若 newCachedSource 收 time.Duration
// 幂等（原 durationSeconds(cfg.TTLSeconds)）
ttl := cfg.TTLSeconds.Duration
```

- [ ] **Step 4: 适配 layout.yaml — fallback/默认赋值与比较**

```go
// 比较（原 cfg.MaxClockSkewSeconds <= 0）
if cfg.MaxClockSkewSeconds.Duration <= 0 { cfg.MaxClockSkewSeconds = config.Duration{Duration: 300 * time.Second} }
if cfg.TTLSeconds.Duration <= 0 { cfg.TTLSeconds = config.Duration{Duration: 300 * time.Second} }
if cfg.MaxAgeSeconds.Duration == 0 { cfg.MaxAgeSeconds = config.Duration{Duration: 600 * time.Second} }
if cfg.Source.CacheTTLSeconds.Duration <= 0 { cfg.Source.CacheTTLSeconds = config.Duration{Duration: 60 * time.Second} }
if cfg.GRPC.TimeoutMilliseconds.Duration <= 0 { cfg.GRPC.TimeoutMilliseconds = config.Duration{Duration: 200 * time.Millisecond} }
if rule.ClientTTLSeconds.Duration <= 0 { rule.ClientTTLSeconds = config.Duration{Duration: 300 * time.Second} }
if rule.WindowSeconds.Duration <= 0 { rule.WindowSeconds = config.Duration{Duration: 60 * time.Second} }
```

- [ ] **Step 5: 适配 layout.yaml — 限流 fixed_window / lua / DB 行 / ruleTTL（int 边界）**

```go
// fixed window（原 time.Duration(rule.WindowSeconds)*time.Second）
return s.allowFixedWindow(ctx, key, rule.WindowSeconds.Duration, rule.MaxRequests, ruleTTL(rule))
// ruleTTL（原 time.Duration(rule.ClientTTLSeconds) * time.Second / time.Duration(rule.WindowSeconds) * time.Second）
if rule.ClientTTLSeconds.Duration > 0 { return rule.ClientTTLSeconds.Duration }
if ... && rule.WindowSeconds.Duration > 0 { return rule.WindowSeconds.Duration }
// lua 脚本参数需要 int 秒（原 rule.WindowSeconds）
redisFixedWindowScript.Run(ctx, s.client, []string{key}, int(rule.WindowSeconds.Seconds()), ttlSeconds, rule.MaxRequests, time.Now().Unix())
// DB 行 / 动态规则源：row.WindowSeconds（int 列）→ 构造 config.Duration
WindowSeconds: config.Duration{Duration: time.Duration(row.WindowSeconds) * time.Second},  // 由 DB int 读入
// 写回 DB / record（原 row.WindowSeconds = rule.WindowSeconds）→ int(rule.WindowSeconds.Seconds())
```

> 限流测试断言 `rule.MaxRequests != 9 || rule.WindowSeconds != 60`（layout.yaml:3750）改为 `rule.WindowSeconds.Duration != 60*time.Second`。逐处按 grep 结果适配。

- [ ] **Step 6: 适配 rule_center_client.go（边界）**

Run: `grep -nE "TimeoutMilliseconds|ConnectTimeoutMillis|time\.Duration\(cfg" internal/assets/_data/hertz/optional/rule_center_client.go`
该文件自有选项结构（`RPCTimeoutMilliseconds int`）从 conf 读取处适配：原 `cfg.RPCTimeoutMilliseconds = int(timeout.Milliseconds())` 类边界，若 `timeout` 来自被迁移 conf 字段则用 `.Duration`；自有 int 选项保持不变（仅边界转换）。

- [ ] **Step 7: 提交（暂不 regenerate golden，待 Task 8 统一）**

```bash
git add internal/assets/_data/hertz/hertz-template/data_go.yaml internal/assets/_data/hertz/layout.yaml internal/assets/_data/hertz/optional/rule_center_client.go
git commit -m "feat(scaffold): hertz 生成代码适配 conf.Duration 消费方（data/layout/rule_center）"
```

---

## Task 4: R-A redis 铺垫（redis_shared.go）

**Files:**
- Modify: `internal/assets/_data/hertz/optional/redis_shared.go`

**Interfaces:**
- Consumes: Task 1 产出的 redis `config.Duration` 字段。
- Produces: `RedisUniversalOptions(cfg)` 用 `.Duration` 构造 `redis.UniversalOptions`；删除不再使用的本地 `durationSeconds`/`durationMilliseconds`。

- [ ] **Step 1: 改 `RedisUniversalOptions` 映射**

```go
return &redis.UniversalOptions{
    Addrs:                 cfg.Addrs,
    ...
    MaxRetries:            cfg.MaxRetries,
    MinRetryBackoff:       cfg.MinRetryBackoffMilliseconds.Duration,
    MaxRetryBackoff:       cfg.MaxRetryBackoffMilliseconds.Duration,
    DialTimeout:           cfg.DialTimeoutSeconds.Duration,
    DialerRetries:         cfg.DialerRetries,
    DialerRetryTimeout:    cfg.DialerRetryTimeoutMilliseconds.Duration,
    ReadTimeout:           cfg.ReadTimeoutSeconds.Duration,
    WriteTimeout:          cfg.WriteTimeoutSeconds.Duration,
    ...
    PoolTimeout:           cfg.PoolTimeoutSeconds.Duration,
    ...
    ConnMaxIdleTime:       cfg.ConnMaxIdleTimeSeconds.Duration,
    ConnMaxLifetime:       cfg.ConnMaxLifetimeSeconds.Duration,
    ConnMaxLifetimeJitter: cfg.ConnMaxLifetimeJitterSeconds.Duration,
    ...
    FailingTimeoutSeconds: int(cfg.FailingTimeoutSeconds.Seconds()),  // go-redis 此项为 int 秒
}
```

- [ ] **Step 2: 删除不再使用的 helper**

若 `durationSeconds`/`durationMilliseconds` 无其它引用（grep 确认），删除这两个函数及可能不再使用的 `import "time"`（若 time 仍被它用则保留）。

Run: `grep -nE "durationSeconds|durationMilliseconds" internal/assets/_data/hertz/optional/redis_shared.go`

- [ ] **Step 3: 提交**

```bash
git add internal/assets/_data/hertz/optional/redis_shared.go
git commit -m "feat(scaffold): redis 铺垫（R-A）redis_shared 超时字段改用 .Duration"
```

---

## Task 5: Kitex conf 结构体 duration 字段迁移（conf.yaml）

**Files:**
- Modify: `internal/assets/_data/kitex/kitex-template/conf.yaml`
- Regenerate: 含 kitex conf.go 的 golden（mono/rpc）

- [ ] **Step 1: 改结构体字段类型（保名换类型）**

```go
type RPCConfig struct {
    RequestTimeoutSeconds config.Duration `json:"request_timeout_seconds" yaml:"request_timeout_seconds"`
}
type DatabaseConfig struct {
    ...
    MaxConnLifetimeSeconds   config.Duration `json:"max_conn_lifetime_seconds" yaml:"max_conn_lifetime_seconds"`
    MaxConnIdleTimeSeconds   config.Duration `json:"max_conn_idle_time_seconds" yaml:"max_conn_idle_time_seconds"`
    HealthCheckPeriodSeconds config.Duration `json:"health_check_period_seconds" yaml:"health_check_period_seconds"`
}
// RateLimitSourceConfig.CacheTTLSeconds / RateLimitGRPCConfig.TimeoutMilliseconds /
// RateLimitDatabaseConfig.QueryTimeoutMilliseconds / RateLimitRuleConfig.{WindowSeconds,ClientTTLSeconds}
// 同样 int → config.Duration
```

- [ ] **Step 2: 改 `Default()`**

```go
RPC: RPCConfig{RequestTimeoutSeconds: config.Duration{Duration: 3 * time.Second}},
Database: DatabaseConfig{
    MaxConnLifetimeSeconds:   config.Duration{Duration: 1800 * time.Second},
    MaxConnIdleTimeSeconds:   config.Duration{Duration: 300 * time.Second},
    HealthCheckPeriodSeconds: config.Duration{Duration: 30 * time.Second},
    ...
},
RateLimit: RateLimitConfig{
    Source:   RateLimitSourceConfig{Type: "database", CacheTTLSeconds: config.Duration{Duration: 60 * time.Second}, ...},
    GRPC:     RateLimitGRPCConfig{TimeoutMilliseconds: config.Duration{Duration: 200 * time.Millisecond}, ...},
    Database: RateLimitDatabaseConfig{QueryTimeoutMilliseconds: config.Duration{Duration: 200 * time.Millisecond}},
    ...
    PreAuth: RateLimitPhaseConfig{ DefaultRule: RateLimitRuleConfig{
        WindowSeconds: config.Duration{Duration: 60 * time.Second},
        ClientTTLSeconds: config.Duration{Duration: 300 * time.Second}, ... } },
},
```

- [ ] **Step 3: 改 `Validate()` 比较**

`c.RPC.RequestTimeoutSeconds < 0` → `c.RPC.RequestTimeoutSeconds.Duration < 0`；`c.Database.MaxConnLifetimeSeconds < 0` → `.Duration < 0`；其余被迁移字段同理。注意 kitex `Server.Timeout.ReadWriteTimeout` 已是 `time.Duration`，不动。

- [ ] **Step 4: 提交（golden 待 Task 8 统一 regenerate）**

```bash
git add internal/assets/_data/kitex/kitex-template/conf.yaml
git commit -m "feat(scaffold): kitex conf duration 字段迁移 config.Duration（结构体+Default+Validate）"
```

---

## Task 6: Kitex dev YAML + 消费方适配（conf_dev.yaml + server.yaml + ratelimit_usecase.yaml）

**Files:**
- Modify: `internal/assets/_data/kitex/kitex-template/conf_dev.yaml`
- Modify: `internal/assets/_data/kitex/kitex-template/server.yaml`
- Modify（按需）: `internal/assets/_data/kitex/kitex-template/ratelimit_usecase.yaml`

- [ ] **Step 1: conf_dev.yaml duration 值字符串化**

```yaml
rpc:
  request_timeout_seconds: "3s"
database:
  max_conn_lifetime_seconds: "1800s"
  max_conn_idle_time_seconds: "300s"
  health_check_period_seconds: "30s"
```

（`server.timeout.read_write_timeout`/`exit_wait_timeout` 已是 `"30s"`/`"10s"`，不动。）

- [ ] **Step 2: server.yaml 消费方适配**

```go
// 原 interceptor.RequestTimeout(durationSeconds(cfg.RPC.RequestTimeoutSeconds))
interceptor.RequestTimeout(cfg.RPC.RequestTimeoutSeconds.Duration),
```

若本地 `func durationSeconds(value int) time.Duration`（server.yaml:159）无其它引用则删除（grep 确认）。

Run: `grep -nE "durationSeconds" internal/assets/_data/kitex/kitex-template/server.yaml`

- [ ] **Step 3: ratelimit_usecase.yaml — grep 确认是否消费被迁移 conf 字段**

Run: `grep -nE "conf\.|cfg\.|RateLimitRuleConfig|WindowSeconds|ClientTTLSeconds" internal/assets/_data/kitex/kitex-template/ratelimit_usecase.yaml`

该文件 `WindowSeconds` 多为 kitex IDL/pb 类型（`req.WindowSeconds`、`int32(r.WindowSeconds)`），**不属 conf 迁移范围**，不改。仅当 grep 发现直接读取 `conf.RateLimit.*` 被迁移字段时，按 Global Constraints 适配；否则跳过并在本步记录「无 conf 消费方」。

- [ ] **Step 4: kitex optional add-on 边界检查**

Run: `grep -rnE "conf\.|cfg\.(WindowSeconds|TimeoutMilliseconds|CacheTTLSeconds|RequestTimeoutSeconds)" internal/assets/_data/kitex/optional/`
仅适配读取被迁移 conf 字段处；add-on 自有 duration 配置（如 registry_etcd 自有 `DialTimeoutSeconds`）不改。

- [ ] **Step 5: 提交**

```bash
git add internal/assets/_data/kitex/kitex-template/conf_dev.yaml internal/assets/_data/kitex/kitex-template/server.yaml internal/assets/_data/kitex/kitex-template/ratelimit_usecase.yaml
git commit -m "feat(scaffold): kitex 生成代码适配 conf.Duration 消费方（conf_dev/server）"
```

---

## Task 7: Golden 重新生成 + e2e 编译验证

**Files:**
- Regenerate: `internal/scaffold/{mono,bff,rpc,infra}/testdata/**`

- [ ] **Step 1: 重新生成受影响 golden（精确包路径）**

Run: `go test ./internal/scaffold/mono/... ./internal/scaffold/bff/... ./internal/scaffold/rpc/... ./internal/scaffold/infra/... -update-golden -count=1`

- [ ] **Step 2: 逐提交审查 golden diff**

Run: `git diff internal/scaffold/*/testdata/ | head -300`
Expected: 仅 conf.go（字段类型 config.Duration + Default 值）、conf dev YAML（值字符串化）、data.go/layout 消费方、redis_shared.go、server.go 的对应变化；无误 bless 的无关文件。

- [ ] **Step 3: 跑 golden 测试**

Run: `go test ./internal/scaffold/mono/... ./internal/scaffold/bff/... ./internal/scaffold/rpc/... ./internal/scaffold/infra/... -count=1`
Expected: PASS

- [ ] **Step 4: e2e 编译验证（验证 config.Duration 真能编译 + 加载 "30s"）**

Run: `go test ./internal/scaffold/mono/... -run 'TestGenerateHertzCompiles|TestGenerateHertzWithDatabaseCompiles|TestGenerateKitexCompiles' -count=1`
Expected: PASS（需 hz/kitex/make + go 1.26.5 + proxy 网络；环境缺失则 skip 并在报告明确标注，不静默通过）

- [ ] **Step 5: 提交 golden**

```bash
git add internal/scaffold/*/testdata/
git commit -m "test(scaffold): 重新生成 golden（conf duration 化 + redis 铺垫）"
```

---

## Task 8: 文档（中英对齐）

**Files:**
- Modify: `README.md`/`README.zh-CN.md`/`docs/examples.md`/`docs/examples.zh-CN.md`
- Modify（按需）: `internal/assets/_data/docs/{hertz,kitex}/design-doc.{en,zh-CN}.md`

- [ ] **Step 1: 先 Read 现状，增量补充**

记录：生成项目 conf 的 duration 字段统一用 `config.Duration`，YAML 写 `"30s"`/`"8ms"` 字符串格式（go-framework/config）；redis 配置超时字段已 duration 化（R-A 铺垫，客户端接线见后续 PR3）。

- [ ] **Step 2: EN/ZH 对齐 + markdown 诊断**

Run: `grep -rn "config.Duration\|30s" README.md README.zh-CN.md docs/examples.md docs/examples.zh-CN.md`（确认中英一致）；按仓库 pre-commit 文件钩子做 markdown 格式检查。

- [ ] **Step 3: 提交**

```bash
git add README.md README.zh-CN.md docs/examples.md docs/examples.zh-CN.md internal/assets/_data/docs/
git commit -m "docs: 生成项目 conf duration 字段用 config.Duration（\"30s\" 格式，中英对齐）"
```

---

## Task 9: 全量验证 + PR 准备

- [ ] **Step 1: 全量验证链**

Run: `go build ./... && go build . && go vet ./... && go test ./... -count=1 && ./scripts/smoke.sh`
Expected: 全绿。

- [ ] **Step 2: gofmt + 复核 diff 范围**

Run: `gofmt -l $(find . -name '*.go' -not -path './.git/*')`（无输出）；`git diff --stat origin/main...HEAD` 抽查仅模板/生成代码/golden/docs。

- [ ] **Step 3: 创建 PR（orchestrator 执行，body 含 `Closes #7`）**

PR 描述含：范围重估说明（loading/registry/jaeger 已由 PR1/main 完成）、duration 迁移字段清单、R-A redis 铺垫边界、保名换类型决策、golden diff 审查结论、e2e 编译结果、与 PR3 的关系（redis 客户端接线后续）、已知取舍（限流动态规则源 int↔config.Duration 边界转换、FailingTimeoutSeconds 取 int 秒）。

## 验证顺序

1. 聚焦：`go build ./... && go vet ./internal/scaffold/...`（模板改动不破坏 ncgo 自身）
2. 包级：mono/bff/rpc/infra golden 测试（Task 7）
3. e2e：`TestGenerate*Compiles`（Task 7 Step 4）
4. 全量：`go test ./... -count=1` + `go vet` + `go build`
5. smoke：`./scripts/smoke.sh`

## 风险

- **限流动态规则源边界**：`RateLimitRuleConfig.{WindowSeconds,ClientTTLSeconds}` 同时用于静态 YAML 与动态源（DB 行/grpc/rule_center，int 秒）；迁移后须在 int↔config.Duration 边界逐一转换（lua 参数 `int(.Seconds())`、DB 读入 `config.Duration{...}`），漏改即编译/运行错误（e2e 编译兜底，运行态需限流测试覆盖）。
- **爆炸半径大**：hertz `layout.yaml` 消费点密集（限流/CORS/签名/幂等），golden diff 大 → 逐提交审查，避免误 bless。
- **YAML 值格式切换**：`config.Duration.UnmarshalYAML` 只接受字符串，裸整数不再兼容 → 所有 dev/prod 样例值须同步字符串化（干净切换）。
- **FailingTimeoutSeconds**：go-redis `UniversalOptions.FailingTimeoutSeconds` 为 int 秒，取 `int(cfg.FailingTimeoutSeconds.Seconds())`。
- 模板/脚手架输出 contract-sensitive；本 PR 不改 ncgo 业务逻辑。
