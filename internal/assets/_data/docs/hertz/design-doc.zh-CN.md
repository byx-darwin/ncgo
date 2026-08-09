# Hertz 模板详细设计

读者:ncgo 维护者及读取或修改 `internal/assets/_data/hertz/` 嵌入模板树
的 AI Agent。本文档描述每个文件的作用、暴露给脚手架的契约,以及在不破
坏已生成项目的前提下如何演进。

Kitex 对应文档见
[`docs/kitex/design-doc.zh-CN.md`](../kitex/design-doc.zh-CN.md)。

动态限流专题见
[`rate-limit-dynamic-design.zh-CN.md`](./rate-limit-dynamic-design.zh-CN.md)。

## 1. 总览

Hertz 模板族支撑 `ncgo new --mode mono`(HTTP 服务),由
`internal/scaffold/mono` 消费,经 `hz` 渲染。
最小支持的 `hz` 版本定义在 `internal/exec/exec.go` 中。

模板通过 `//go:embed all:_data` 嵌入 ncgo 二进制(见
`internal/assets/assets.go`)。目录名带前导下划线 (`_data/`) 是为了让
`go build ./...` 忽略 `optional/*.go` —— 这些文件是模板素材,不是参与
ncgo 二进制编译的 Go 源码。

生成的项目构建在 **go-tools v0.1.0** 之上(是其上的薄业务层):`go.mod` 声明
`go 1.26.5`,并 require `go-common v0.1.0` + `go-framework v0.1.0`
(`go-middleware v0.1.0` 在 `WithDatabase=true` 时由 `go mod tidy` 补齐)。
响应使用 `go-framework/hertz.Responder`,配置使用 `go-framework/config`,
日志使用 `go-common/log`,错误码 re-export `go-framework/error` 的框架码。

资产版本见 `_data/VERSION`;当前嵌入资产版本由 `assets.Version()` 暴露。

## 2. 生成项目架构

### 2.1 目录结构

`ncgo new --mode mono` 加内置 `hz` 调用之后,产物如下:

```
<project>/
├── main.go                          # conf.Init() → server.Run()
├── conf/dev/conf.yaml               # 按 GO_ENV 加载的 YAML 配置
├── idl/app/                         # Protobuf IDL(如 svc.proto)
├── template/                        # layout.yaml / package.yaml / data.json(`hz update` 复用)
├── internal/
│   ├── base/
│   │   ├── conf/                    # 配置类型与 Init/Load/Default/Validate
│   │   ├── data/                    # pgxpool + sqlc Queries(可叠加 optional 客户端)
│   │   └── server/                  # samber/do injector、Hertz server、中间件链
│   ├── handler/
│   │   ├── health/                  # /healthz、/readyz
│   │   └── <service>/               # 由 hz 按 IDL 生成
│   ├── usecase/<service>/           # 业务逻辑;依赖 repo / adapter 端口接口
│   ├── adapter/                     # 出站 RPC 客户端、第三方网关
│   ├── repository/                  # usecase 端口的 sqlc 实现
│   ├── pb/                          # hz 生成的 protobuf 类型
│   ├── db/{schema,query,migrations,gen}/
│   ├── pkg/
│   │   ├── response/                # JSON/Protobuf 响应辅助 + 错误码注册表
│   │   ├── errcode/                 # 业务错误码登记
│   │   ├── i18n/locales/            # make i18n 输入的默认/扩展语言 JSON
│   │   └── middleware/              # signature、token、rate_limit、idempotency、cors 等
│   └── router/register.go           # hz 插入点
├── tools/i18n/{gen,sync,report,check,util}/ # i18n 工具链与 catalog 生成器
├── Makefile                         # build / dev / update / sqlc / migrate / lint / test
├── go.mod
├── .hz                              # hz 标识文件(不要删)
└── .gitignore
```

### 2.2 层边界

允许的 import 方向(自顶向下):

```
main          → base/server
base/server   → router、handler/*、base/conf、base/data、pkg/middleware
handler/*     → usecase/*、pkg/response、pb           (不得 import repo / data)
usecase/*     → adapter、repository(端口接口)、pb
adapter,      → base/data、db/gen、第三方 SDK
repository/*    (不得 import usecase;端口在 usecase 包里定义)
```

由 `ncgo doctor` 输出的硬规则(锚回
`nc-skills-golang/SKILL.md#layer-rules`):

- handler 严禁 import `internal/repository/*` 或 `internal/base/data`。
- usecase 严禁 import `github.com/cloudwego/hertz/...`。
- repository / adapter 实现严禁 import `internal/usecase/*`。
- 所有返回错误必须是带 `Code` + `Public` 的 `go-common/error`(`goerror`)链。
  (`goerror` 内部包装 `samber/oops`;不要直接 `import "samber/oops"` 构造错误。)

### 2.3 依赖注入(`samber/do`)

`internal/base/server/server.go` 构建 injector,按固定顺序注册 provider。
模板里给出的骨架:

```go
injector := do.New()
defer func() { _ = injector.Shutdown() }()
do.ProvideValue(injector, cfg)                 // *conf.Config

// DB 服务追加:
if cfg.Database.Enabled {
    startupCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()
    pgCfg, _ := data.NewPostgresConfigFromDatabase(cfg.Database)
    pool, _ := data.NewPostgres(startupCtx, pgCfg)
    dbData, cleanup, _ := data.New(pool)
    defer cleanup()
    do.ProvideValue[context.Context](injector, startupCtx)
    do.ProvideValue(injector, pgCfg)
    do.ProvideValue(injector, pool)
    do.ProvideValue(injector, dbData)
}
do.Provide(injector, repository.NewUserRepo)
do.Provide(injector, adapter.NewDeviceAdapter)
do.Provide(injector, usecase.NewUserUseCase)
do.Provide(injector, handler.NewUserHandler)
```

数据库连接池的 cleanup 现在由 `server.Run()` 在创建 `dbData` 后直接
`defer cleanup()` 挂到服务生命周期上;`injector.Shutdown()` 仍负责其它实现了
shutdown 接口的 provider。

### 2.4 请求生命周期

```
HTTP 请求
  → Hertz 引擎(server.Default,超时来自 cfg.Server.*)
  → Recovery → RequestID → AccessLog → RequestTimeout
  → InternalOnly       (CIDR / 路径白名单,源自 cfg.Security)
  → CORS               (cfg.CORS.Enabled 时)
  → RateLimit pre_auth (启用时)
  → SignatureAuth      (启用时;命中 cfg.Auth.PublicPaths 跳过)
  → JWTAuth            (启用时;命中 cfg.Auth.PublicPaths 跳过)
  → RateLimit post_auth(启用时)
  → Idempotency        (启用时;方法 POST/PUT/PATCH/DELETE)
  → router.GeneratedRegister(hz 生成的路由)
  → Handler.Method(ctx, c):
       var req T
       c.BindAndValidate(&req)            → 失败:response.BindError
       resp, err := h.uc.Method(ctx, &req)
       err != nil                         → response.Err(c, err)
       成功                               → response.OK(c, resp)
```

链路顺序在 `server.Run` 里写死,功能由配置开关控制,不通过新增 import。

## 3. 内置功能

### 3.1 配置(`internal/base/conf`)

- 环境识别:`GO_ENV`,缺省 `dev`。
- 文件解析:`CONFIG_PATH` 优先,否则 `conf/<env>/conf.yaml`。
- 解析路径不存在 **且** 未设 `CONFIG_PATH` 时使用 `Default()`;否则报
  `goerror.In("config").Code(frameworkerror.CodeConfigInvalid).Public("config_invalid")`
  (`frameworkerror.CodeConfigInvalid` = 10004)。
- `Init()` 由 `main.go` 调用一次(`sync.Once`),`Get()` 返回缓存的
  `*Config`。
- `Validate()` 校验:超时非负、签名 / 令牌密钥、CORS 通配 +
  `allow_credentials` 互斥、`rate_limit` 的 source/backend/phase/strategy
  合法性、`idempotency` 后端。
- `Config` 中所有时长类字段(例如 `rpc.request_timeout_seconds`、
  `database.health_check_period_seconds`、`rate_limit.rule.window_seconds`、
  `redis.dial_timeout_seconds`)统一使用 `go-framework/config` 的
  `config.Duration`。`conf/dev/conf.yaml` 中这些字段以 duration 字符串
  (如 `"30s"` / `"200ms"`)填写,由 `time.ParseDuration` 解析;不再接受裸整数。

### 3.2 响应与错误码(`internal/pkg/response`)

- `Reply` / `OK` / `Err` / `ErrorCode` / `BindError` 委托给
  `go-framework/hertz.Responder`(HTTP 状态由错误码派生),并根据 `Accept` 头
  返回 JSON 或 Protobuf(payload 实现 `proto.Message` 时优先 protobuf)。
- 错误码遵循 go-tools 码段:
  - Framework `10000–10499` —— `go-framework/error` 框架码,由
    `internal/pkg/errcode` 与 `internal/pkg/response` re-export:
    `CodeSystem=10000`、`CodeParamInvalid=10001`、`CodeAuthFailed=10002`、
    `CodeConfigInvalid=10004`、`CodeRPCUnavailable=10010`、
    `CodeRPCTimeout=10011`。
  - Middleware `20000–20699`、Auth `40000–40099`。
  - Project `40100–59999` —— 业务自定义码。**业务码必须 `>= 40100`
    (`goerror.ProjectCodeMin`)。**
- 业务码注册写在进程启动期:
  ```go
  var CodeOrderConflict = response.MustRegister(response.Definition{
      Code: 40101, Msg: "order_conflict", Status: consts.StatusConflict,
  }).Code
  ```
- 错误生产端用 `goerror.In("...").Code(code).Public("msg")…`
  (`goerror` = `go-common/error`,内部包装 `samber/oops`);
  `response.Err` 提取 `(code, msg)` 并经 `StatusFromCode` 映射 HTTP 状态。
- **行为说明:** 业务码(`>= 40100`)经 `goerror.HTTPStatus` 兜底返回
  **HTTP 200** —— go-tools 将其视为「业务错误、RPC 调用成功」。如需非 200
  响应,请用 `goerror.RegisterHTTPStatuses` 注册细粒度 HTTP 状态。
- `internal/pkg/i18n` 根据 `Accept-Language` 选择 `en` / `zh-CN` / `zh-TW` /
  `ja-JP` / `ko-KR` / `fr-FR` / `de-DE` / `es-ES`,并由 `response` 对 JSON 响应的
  `msg` 做翻译、写入 `Content-Language`。
  未命中翻译时回退为原始错误 key;业务消息可用 `i18n.Register` 扩展。
- 默认语言与动态新增语言统一走构建期生成:在
  `internal/pkg/i18n/locales/*.json` 维护 `{language, aliases, messages}`
  文件,执行 `make i18n` 生成 `internal/pkg/i18n/catalog_gen.go`。
  生成代码会调用 `i18n.RegisterLanguage(language, aliases...)` 与
  `i18n.Register(...)`; `i18n.go` 只保留注册与协商机制,不再内置翻译表。
  因此 `FromAcceptLanguage` 可以识别新语言别名,例如 `it` → `it-IT`。

### 3.3 安全中间件(`internal/pkg/middleware`)

| 中间件 | 触发条件 | 后端 | 失败码 |
|---|---|---|---|
| `SignatureAuth` | `cfg.Auth.Signature.Enabled` | HMAC-SHA256(`method\npath\nquery\nts\nnonce\nbody`);nonce 存储 `memory`(LRU)或 `redis` | signature missing / expired / invalid → `CodeAuthFailed`(10002);replay → `CodeReplayRequest`(10202) |
| `JWTAuth` | `cfg.Auth.Token.Enabled` | `golang-jwt/jwt/v5` HS256;读 `cfg.Auth.Token.Header`;claims 写入 context key `tokenClaims` | token missing / invalid / expired / claims invalid → `CodeAuthFailed`(10002) |
| `RateLimit` | `cfg.RateLimit.Enabled` + 分阶段开关 | 动态规则解析(`config` / `grpc` / `database`) + `fixed_window` / `token_bucket`;执行状态落 `memory`(samber/hot LRU)或 `redis` | `CodeRateLimited`(10200,HTTP 429);`fail_open=false` 且后端故障时 → `CodeCacheUnavailable`(10304,HTTP 503) |
| `Idempotency` | `cfg.Idempotency.Enabled` | 重放缓存,key = `header + method + path + body-hash`;`memory` 或 `redis` | `CodeIdempotencyKeyMissing`(10203,HTTP 400)/ `CodeIdempotencyConflict`(10204,HTTP 409) |
| `InternalOnly` | 始终启用 | CIDR + 路径白名单 | `CodePermissionDenied`(10108,HTTP 403) |
| `CORS` | `cfg.CORS.Enabled` | 静态配置;通配 origin 与 credentials 互斥 | n/a |

认证失败统一收敛到 `frameworkerror.CodeAuthFailed`(10002);限流 / 幂等 /
权限 / 缓存类错误码保留各自的脚手架数值(10108、10200–10204、10304),HTTP
映射经 `StatusFromCode`。

`Unless(mw, skipper)` 与 `PathSkipper(paths...)` 包住认证中间件,使
public 路径(默认 `/healthz`、`/readyz`,加上 `cfg.Auth.PublicPaths`)
绕过签名 / JWT。

动态规则解析顺序、缓存/失效策略与接入示例,见
[`rate-limit-dynamic-design.zh-CN.md`](./rate-limit-dynamic-design.zh-CN.md)。

当前 Hertz 动态限流模型可简要概括为:

- 执行链路分为 `pre_auth` 与 `post_auth` 两个阶段
- 规则来源支持 `config`、`grpc`、`database`
- 本地配置始终作为最终兜底规则源
- 每个阶段同时支持 `default_rule` 与细粒度本地 `rules`
- 常用维度包括 `ak_path`、`ak_method_path`、`user_uuid`、`ip`
- 动态规则查询结果使用进程内 TTL 缓存
- `fallback_on_error` 决定规则回退,`fail_open` 决定限流存储故障时是否放行请求

`fail_open`(rate-limit / signature nonce / idempotency)把"依赖故障 →
拒绝请求"改为"依赖故障 → 放行"。仅当后端 Redis 不是关键路径时才用。

默认情况下，所有基于 Redis 的 middleware store 会复用一份由顶层
`cfg.Redis` 派生出的进程内 `redis.UniversalClient`。只有模块级 Redis
override，或显式独立接线，才会创建单独连接池。

### 3.4 数据库(`with_database: true`)

脚手架带 `--db postgres` 时:

- `internal/base/conf/conf.go` 暴露顶层 `DatabaseConfig`,对应 `conf/dev/conf.yaml`
  的 `database.*` 配置域,与 `rate_limit.database.*` 分离。
- `internal/base/data/data.go` 暴露 `Data{ Pool *pgxpool.Pool, Queries *gen.Queries }`,
  `New(pool)` 返回 cleanup 用于关闭连接池。
- `data.NewPostgresConfigFromDatabase(cfg.Database)` 会把顶层 `database.*`
  转成 `*pgxpool.Config`;`data.NewPostgres(ctx, *pgxpool.Config)` 负责打开并 ping
  连接池。默认模板会在 `cfg.Database.Enabled=true` 时自动完成这段 wiring,随后把
  `startupCtx` / `pgCfg` / `pool` / `dbData` 通过 `do.ProvideValue(...)` 注入。
- `internal/db/`:
  - `schema/*.sql` —— 表 DDL(sqlc 输入)。
  - `query/*.sql` —— sqlc 注解的查询。
  - `gen/*.go` —— `make sqlc`(`sqlc generate -f internal/db/sqlc.yaml`)生成。
  - `migrations/*.sql` —— `make migrate-*` 驱动的 goose 迁移。
- 事务由 repository 通过 `WithTx(ctx, fn)` 持有(kitex 侧模板自带,
  Hertz repository 同模式实现)。

### 3.5 健康检查与运维

- `GET /healthz`、`GET /readyz` 返回
  `{"status":"ok"|"ready","time":RFC3339}`,经 `response.OK` 输出;两者
  默认在 `cfg.Auth.PublicPaths` 与 `cfg.Security.InternalPaths` 中。
- `Makefile` 目标:`build`、`run`、`dev`(air 或 `go run .`)、
  `i18n`(从 `locales/*.json` 生成 `catalog_gen.go`)、
  `update`(用内置 `package.yaml` 重跑 `hz update`)、`sqlc`、
  `migrate-{create,up,down,status}`、`lint`、`test`、`tidy`、
  `install-tools`。`generate` 会串联 `i18n update swagger sqlc`。
- 对 `WithDatabase=true` 的脚手架,生成的 `internal/base/data` / repository
  代码会 import `internal/db/gen`,因此首次 `go mod tidy` 或 build 前要先
  执行 `make sqlc`;如果直接跑 `make generate`,其中已包含这一步。
- `cmd/server/main.go` 只做 `conf.Init()` + `server.Run()`;Agent 增加
  接线一律落到 `internal/base/server/server.go`。

### 3.6 规则中心客户端

在 Hertz 服务上执行 `ncgo new` 并传入 `--rule-center-addr <address>` 时,
脚手架会生成一个 gRPC 客户端,用于连接规则中心 Kitex 服务以查询远程限流规则。

- 模板来源: `internal/assets/_data/hertz/optional/rule_center_client.go`
- 输出路径: `internal/pkg/middleware/rule_center_client.go`
- 接口: 实现 `ratelimit.GRPCClient`（`ResolveRateLimitRule`）
- 依赖: `google.golang.org/grpc` + `google.golang.org/grpc/credentials/insecure`
- 配置: 设置 `rate_limit.source.type = rule_center` 并在
  `conf/dev/conf.yaml` 中填充 `rule_center` 配置块

生成的客户端在启动时通过
`rlOpts.RuleCenter = middleware.NewRuleCenterClient(cfg.RateLimit.RuleCenter.Address)`
接入 resolver。

对于已有服务,可以使用以下命令添加规则中心客户端:

```bash
ncgo add rule-center --root ./user-api --addr rule-center:8888
```

规则中心服务端通过独立的 Kitex 脚手架生成:

```bash
ncgo new rule-center --module github.com/acme/rule-center \
  --kind kitex --db postgres --preset rule-center
```

这会在 `internal/assets/_data/kitex/kitex-template/` 下产出
`idl/rule-center.proto`、handler、usecase、repository、sqlc schema 和
query 文件。

### 3.7 可选基础设施片段

`ncgo add infra <kind>` 从
`internal/assets/_data/hertz/optional/<kind>.go` 复制一个 Go 文件。
数据客户端通常落到 `internal/base/data/`。

> **Observability(可观测性)不再以 add-on 形态存在。** Hertz 基础模板已内置接入
> go-framework OTLP:`cfg.Server.Jaeger != nil && cfg.Server.Jaeger.Enable`
> 时,`server.go` 调用
> `hertzobs "github.com/byx-darwin/go-tools/go-framework/hertz/observability"`
> 的 `hertzobs.NewProvider(ctx, config.ObservabilityConfig{Enabled, Endpoint,
> ServiceName})`,执行 `h.Use(provider.ServerMiddleware())`,并
> `defer provider.Shutdown()`。原 LoongSuite `observability_otel` / `otel`
> add-on 已在 PR5 中移除,既有的 `otel` kind 现在返回 invalid kind。

#### Redis(`redis.go`)

- 暴露:`*data.Redis{ Client redis.UniversalClient }`(`UniversalOptions`
  自动判定 single / cluster / sentinel)。
- 依赖:`github.com/redis/go-redis/v9`、`go-tools/go-common`(goerror)、
  `go-tools/go-framework`(frameworkerror)。
- 错误码:`frameworkerror.CodeConfigInvalid`(10004,cfg/opts 为 nil)、
  `CodeCacheUnavailable`(40504,`Ping` 失败;经 `goerror.RegisterHTTPStatuses`
  注册为 HTTP 503)。
- 默认接线(复用顶层 `cfg.Redis` 与 middleware 共用 client):
  ```go
  do.ProvideValue[context.Context](inj, startupCtx)
  do.Provide(inj, data.NewRedis)
  ```
- `ncgo add infra redis` 在缺少 `internal/base/data/redis_shared.go`
  时也会一并补齐，使 middleware 与 `data.Redis` 默认复用同一个共享
  client。
- 如需独立连接池，可显式使用 `data.NewRedisWithOptions`。

#### Kafka(`kafka.go`)

- 暴露:`*data.KafkaWriter{ W *kafka.Writer }` 与/或
  `*data.KafkaReader{ R *kafka.Reader }`(segmentio/kafka-go)。
- 依赖:`github.com/segmentio/kafka-go`、`go-tools/go-common`(goerror)、
  `go-tools/go-framework`(frameworkerror)。
- 错误码:`frameworkerror.CodeConfigInvalid`(10004,Writer 为 nil / 缺
  `Addr` / `Topic` / `Brokers`)。
- 接线(producer):
  ```go
  do.ProvideValue(inj, &kafka.Writer{Addr: kafka.TCP(cfg.Kafka.Brokers...), Topic: cfg.Kafka.Topic})
  do.Provide(inj, data.NewKafkaWriter)
  ```

#### Elasticsearch(`es.go`)

- 暴露:`*data.ES{ Client *elasticsearch.Client }`(go-elasticsearch v8)。
- 依赖:`github.com/elastic/go-elasticsearch/v8`、`go-tools/go-common`
  (goerror)、`go-tools/go-framework`(frameworkerror)。
- 错误码:`frameworkerror.CodeConfigInvalid`(10004,`Addresses` 空)、
  `CodeSearchUnavailable`(40506,`NewClient` / `Ping` 失败;经
  `goerror.RegisterHTTPStatuses` 注册为 HTTP 503)。
- 接线:
  ```go
  do.ProvideValue[context.Context](inj, startupCtx)
  do.ProvideValue(inj, elasticsearch.Config{Addresses: cfg.ES.Addresses, Username: cfg.ES.Username})
  do.Provide(inj, data.NewES)
  ```

#### ClickHouse(`clickhouse.go`)

- 暴露:`*data.ClickHouse{ Conn driver.Conn }`(clickhouse-go v2 原生协议)。
- 依赖:`github.com/ClickHouse/clickhouse-go/v2`、`go-tools/go-common`
  (goerror)、`go-tools/go-framework`(frameworkerror)。
- 错误码:`frameworkerror.CodeConfigInvalid`(10004,opts 为 nil / `Addr` 空)、
  `CodeDatabaseUnavailable`(40503,`Open` / `Ping` 失败;经
  `goerror.RegisterHTTPStatuses` 注册为 HTTP 503)。
- 接线:
  ```go
  do.ProvideValue[context.Context](inj, startupCtx)
  do.ProvideValue(inj, &clickhouse.Options{Addr: cfg.CH.Addrs, Auth: clickhouse.Auth{Database: cfg.CH.DB}})
  do.Provide(inj, data.NewClickHouse)
  ```

#### 结构化日志(`observability_logging.go` + `hertz.go`)

`ncgo add infra observability_logging`(别名:`logging`)在 `internal/base/logging/`
下添加分类结构化日志中间件。

- **生成的文件:**
  - `internal/base/logging/logging.go` — 共享工具:`WithRequestID`、
    `WithTrafficLane`、`SinceMS`,以及分类常量(`CategoryAccess`、
    `CategoryError`、`CategoryBiz`、`CategoryRPC`、`CategoryDB`、
    `CategoryPanic`、`CategoryAudit`、`CategorySecurity`)。从
    `go-common/log` 重新导出。
  - `internal/base/logging/hertz.go` — Hertz 专用中间件。

- **中间件(在 `server.go` 中注册):**
  ```go
  import "my-api/internal/base/logging"

  h.Use(logging.HertzRequestID())    // 提取/生成 X-Request-ID,传递 X-Traffic-Lane
  h.Use(logging.HertzAccessLog())    // 结构化访问日志(方法、路径、状态码、延迟)
  h.Use(logging.HertzRecovery())     // panic 恢复 → HTTP 500 + CategoryPanic 日志
  ```

- **`HertzRequestID()`** — 从请求头读取 `X-Request-ID`;回退到 OTel span
  trace ID;都没有时生成 16 字节 hex ID。设置响应头。同时将
  `X-Traffic-Lane` 传递到 context 和 Hertz 本地存储。
- **`HertzAccessLog()`** — 输出 `goclog.Access` 日志,包含 `http.method`、
  `http.path`、`http.status_code` 和 `latency_ms`。
- **`HertzRecovery()`** — 捕获 panic,通过
  `goclog.L().WithCategory(CategoryPanic)` 记录 panic 值和堆栈,返回
  HTTP 500。
- **`HertzRequestIDFromContext(c)`** — 从 Hertz 本地存储读取请求 ID
  (handler 中使用)。

- **初始化(在 `main.go` 或 `server.go` 中):**
  ```go
  import goclog "github.com/byx-darwin/go-tools/go-common/log"

  logging.InitFromConf(cfg.Logging, goclog.ReleaseInfo{
      ServiceName: "my-api",
      ServiceKind: "hertz",
      Version:     release.Version,
  })
  ```

- **配置(`conf.yaml`):**
  ```yaml
  logging:
    level: info          # debug | info | warn | error
    format: json         # json | text
    mode: production     # production | development
    add_source: false
    file:
      dir: logs
      filename: app.log
      max_size_mb: 100
      max_backups: 3
      max_age_days: 7
      compress: true
  ```

- **handler 中使用:**
  ```go
  func (h *Handler) GetUser(ctx context.Context, c *app.RequestContext) {
      reqID := logging.HertzRequestIDFromContext(c)
      goclog.Biz(ctx).InfoContext(ctx, "getting user", "request_id", reqID)
      // ...
  }
  ```

- **依赖:`github.com/byx-darwin/go-tools/go-common`(log、error 包)、
  `go.opentelemetry.io/otel`(trace context,用于 request ID 回退)。

## 4. 文件清单

| 文件 | 作用 | 消费方 |
|---|---|---|
| `hertz/layout.yaml` | 自定义 `hz` 布局:工程目录树 + base/conf/server/middleware Go 源码 | `hz new --customize_layout` |
| `hertz/package.yaml` | 自定义 `hz` package 模板:handler.go 用 `Handler` struct 委托给 `useCase` 接口,并附带 usecase 桩 | `hz new --customize_package` |
| `hertz/data.json` | `layout.yaml` 渲染时变量(`GoModule`、`ServiceName`、`WithDatabase`) | `hz new --customize_layout_data_path` |
| `hertz/sqlc.yaml` | `--db postgres` 时复制到项目里的 sqlc 配置参考 | `mono` 脚手架 |
| `hertz/optional/{redis,kafka,es,clickhouse}.go` | `ncgo add infra <kind>` 的 drop-in 文件 | `internal/scaffold/infra` |
| `hertz/optional/rule_center_client.go` | `--rule-center-addr` 或 `ncgo add rule-center` 生成的 gRPC 客户端 | `mono` 脚手架 |

## 5. `data.json` 契约

```json
{ "*": { "GoModule": "...", "ServiceName": "...", "WithDatabase": false } }
```

字段名在 `layout.yaml` 中以 Go template 变量形式被引用
(`{{.GoModule}}`、`{{.ServiceName}}`)。`mono` 脚手架在生成时会通过
`renderDataJSON` 重写这个文件,把用户值传给 `hz`。新增变量需要:

1. 在 `data.json` 默认值里加字段。
2. 在 `layout.yaml` 中引用 `{{.NewField}}`。
3. 在 `mono.Options` 与 `renderDataJSON` 中加字段。
4. 升 `_data/VERSION`。

## 6. `hz` 调用映射

脚手架把 `layout.yaml`、`package.yaml` 与渲染好的 `data.json` 拷到
`<project>/template/`,然后要么直接调用 `hz`,要么把以下命令打印给用户
/Agent(见 `mono/files.go`):

```
hz new --mod=<module> --idl=<idl> -I idl \
       --handler_dir=internal/handler \
       --model_dir=internal/pb \
       --router_dir=internal/router \
       --customize_layout=template/layout.yaml \
       --customize_layout_data_path=template/data.json \
       --customize_package=template/package.yaml
```

flag 与文件的对应关系:

| Flag | 嵌入源 | 输出作用 |
|---|---|---|
| `--customize_layout` | `hertz/layout.yaml` | 工程骨架 + base 包 |
| `--customize_layout_data_path` | `hertz/data.json`(已渲染) | 模板变量 |
| `--customize_package` | `hertz/package.yaml` | 按 IDL 生成的 handler / usecase 桩 |

## 7. 可选基础设施

每个 `optional/*.go` 文件都是 `infra.Add` 的素材。`infra.Add` 从嵌入 FS
读取后写到对应目标路径,通常是 `internal/base/data/<kind>.go`。

新增 optional 文件的约束:

- **不得** import 项目特有包,只允许 stdlib + 第三方依赖,且要由
  `next steps` 提示用户 `go get`。
- 包名必须匹配目标包(`data` 等)。
- 文件顶部注释必须把注册调用片段原样列出。

当前已发布:`redis`、`kafka`、`es`、`clickhouse`、
`observability_logging`(结构化日志中间件)、`release_canary`
(Hertz 流量适配器)。可观测性追踪(OTLP)由 Hertz 基础模板
(go-framework OTLP,`cfg.Server.Jaeger` 驱动)内置提供,不再以独立
add-on 形态存在。

## 8. 维护契约

任何 `_data/hertz/` 下的改动都必须:

1. 更新文件内容。
2. 同步更新本文档(及 `design-doc.en.md`)对应章节。
3. 升 `_data/VERSION` 的 `ncgo_assets_version`。
4. 重跑 golden:
   `go test ./internal/scaffold/mono/... -update-golden -count=1`
   `go test ./internal/scaffold/infra/... -count=1`(验证 optional 文件
   与嵌入源字节一致)。
5. 若是结构性新增,把新路径加入 `internal/assets/assets_test.go`。

`ncgo doctor` 输出里的 `Rule:` 锚点继续指向
`nc-skills-golang/SKILL.md#layer-rules`。模板归 ncgo;review 规则留在
nc-skills-golang。

## 9. 引用

- `docs/prd.md` §3(决策)、§5(Manifest)、§9(仓库布局)
- `docs/hertz/rate-limit-dynamic-design.zh-CN.md` —— 动态限流专题
- `internal/assets/assets.go` —— embed 接线
- `internal/scaffold/mono/files.go` —— hertz 消费方
- `internal/scaffold/infra/infra.go` —— optional 消费方
- `nc-skills-golang/SKILL.md` —— review 模式规则与 AI 调用指南
- `docs/kitex/design-doc.zh-CN.md` —— Kitex 对应文档
