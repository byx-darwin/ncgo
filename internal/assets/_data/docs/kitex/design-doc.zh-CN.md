# Kitex 模板详细设计

读者:ncgo 维护者及读取或修改 `internal/assets/_data/kitex/` 嵌入模板树
的 AI Agent。本文档描述每个文件的作用、暴露给脚手架的契约,以及在不破
坏已生成项目的前提下如何演进。

Hertz 对应文档见
[`docs/hertz/design-doc.zh-CN.md`](../hertz/design-doc.zh-CN.md)。

动态限流专题位于
[`docs/hertz/rate-limit-dynamic-design.zh-CN.md`](../hertz/rate-limit-dynamic-design.zh-CN.md),
但该专题针对 Hertz HTTP 模板,不直接适用于 Kitex RPC 模板。

## 1. 总览

Kitex 模板族支撑 `ncgo new --mode mono --kind kitex`(RPC 服务),由
`internal/scaffold/mono`(kitex 分支)消费,复制到生成项目的
`template/kitex-template/`,再经 `kitex` 通过
`--template-extension` 机制渲染。
最小支持的 `kitex` 版本定义在 `internal/exec/exec.go` 中。

模板通过 `//go:embed all:_data` 嵌入 ncgo 二进制(见
`internal/assets/assets.go`)。目录名带前导下划线 (`_data/`) 是为了让
`go build ./...` 忽略 `optional/*.go` —— 这些文件是模板素材,不是参与
ncgo 二进制编译的 Go 源码。

生成的项目构建在 **go-tools v0.1.0** 之上(是其上的薄业务层):`go.mod` 声明
`go 1.26.5`,并 require `go-common v0.1.0` + `go-framework v0.1.0`
(`go-middleware v0.1.0` 在项目使用数据库时由 `go mod tidy` 补齐)。配置使用
`go-framework/config`(+ `config/kitex`),日志使用 `go-common/log`,RPC 错误
映射使用 `go-framework/kitex/rpcerror`,框架码来自 `go-framework/error`。

资产版本见 `_data/VERSION`;当前嵌入资产版本由 `assets.Version()` 暴露。

## 2. 生成项目架构

### 2.1 目录结构

`ncgo new --mode mono --kind kitex` 加内置 `kitex` 调用之后,产物如下:

```
<project>/
├── main.go                             # conf.Init() → server.Run()
├── conf/dev/conf.yaml                  # 按 GO_ENV 加载的 YAML 配置
├── idl/<service>.proto                 # Protobuf IDL
├── template/kitex-template/            # YAML 模板(`make update` 复用)
├── internal/
│   ├── base/
│   │   ├── conf/                       # 配置类型与 Init/Load/Default/Validate
│   │   ├── data/                       # pgxpool + sqlc Queries(可叠加 optional 客户端)
│   │   └── server/                     # samber/do 装配 + kitex server 启动
│   ├── handler/<service>/              # 薄壳 RPC handler,委托 UseCase
│   ├── usecase/<service>/              # 业务逻辑;在此声明 `<Service>Repo` 端口
│   ├── repository/<service>/           # sqlc 实现 + WithTx 辅助
│   ├── pb/                             # kitex 生成的 protobuf 类型
│   ├── db/{schema,query,migrations,gen}/
│   └── pkg/
│       ├── interceptor/                # RequestID、AccessLog、Recovery、RequestTimeout、CallerAllowlist
│       └── rpcerror/                   # goerror → kitex BizStatusError 映射
├── pkg/client/<service>/               # 调用端客户端工厂 + Retry/熔断 配置
├── kitex_gen/                          # kitex 生成的 server 桩(不要改)
├── Makefile                            # build / dev / update / sqlc / migrate / lint / test
├── go.mod
└── .gitignore
```

### 2.2 层边界

允许的 import 方向(自顶向下):

```
main          → base/server
base/server   → base/conf、base/data、handler/*、usecase/*、repository/*、
                pkg/interceptor、pkg/rpcerror
handler/*     → usecase/*、pkg/rpcerror、kitex_gen        (不得 import repo / data)
usecase/*     → repository(端口接口在此声明)、pb
repository/*  → base/data、db/gen、pgx                    (不得 import usecase)
pkg/client/*  → kitex_gen、pkg/interceptor                (供 adapter 消费)
```

由 `ncgo doctor` 输出的硬规则:

- handler 严禁 import `internal/repository/*` 或 `internal/base/data`。
- usecase 严禁 import `github.com/cloudwego/kitex/...`。
- repository 实现严禁 import `internal/usecase/*`;端口接口
  (`<Service>Repo`)在 usecase 包内声明。
- 所有跨 RPC 边界的错误必须经过 `rpcerror.ToBizError(err)`,调用方拿到的
  是携带 5 位错误码的 `BizStatusError`。

### 2.3 依赖注入(`samber/do`)

`internal/base/server/server.go` 构建 injector,链路在函数内联完成
(没有独立 provider 列表)。模板里给出的骨架:

```go
injector := do.New()
defer func() { _ = injector.Shutdown() }()
do.ProvideValue(injector, cfg)                 // *conf.Config

var repo usecase.<Service>Repo
if cfg.Database.Enabled {
    repo, cleanup = provideRepository(cfg.Database)   // pgxpool + data.New + repository.New
    defer cleanup()
}
uc := usecase.New(repo)
svr := svc.NewServer(handler.New<Service>Impl(uc), opts...)
```

`provideRepository` 依次调用 `data.NewPostgresConfig(dsn)` →
`applyPostgresPoolConfig(...)` → `data.NewPostgres(ctx, cfg)` →
`data.New(pool)` → `repository.New(d.Queries, d.Pool)`。cleanup 闭包
在退出时关闭连接池。

### 2.4 请求生命周期

```
RPC 请求(TTHeader)
  → kitex server(NewServer 携 WithReadWriteTimeout / WithExitWaitTime)
  → MetaHandler(transmeta.ServerTTHeaderHandler)
  → endpoint.Chain:
       RequestID()                   → 确保 metainfo 有 x-request-id
       AccessLog()                   → klog 记录 service / method / latency / biz code
       Recovery()                    → recover → rpcerror.InternalErrorf
       CallerAllowlist(...)          → 校验 metainfo 中 x-caller-service
       RequestTimeout(d)             → context.WithTimeout;超时 → TimeoutError
  → Handler.<Method>(ctx, req):
       resp, err := s.uc.<Method>(ctx, req)
       err != nil                   → return rpcerror.ToBizError(err)
       成功                         → return resp, nil
  → ErrorHandler                    → rpcerror.ToBizError(err)
```

链路顺序在 `server.Run` 里写死。`WithErrorHandler` 兜底重新包装漏出的
错误,保证回包永远是 `BizStatusError`。

## 3. 内置功能

### 3.1 配置(`internal/base/conf`)

- 环境识别:`GO_ENV`,缺省 `dev`。
- 文件解析:`CONFIG_PATH` 优先,否则 `conf/<env>/conf.yaml`。
- 解析路径不存在 **且** 未设 `CONFIG_PATH` 时使用 `Default()`;否则报
  `goerror.In("config").Code(frameworkerror.CodeConfigInvalid).Public("config_invalid")`
  (`frameworkerror.CodeConfigInvalid` = 10004)。
- `Init()` 由 `main.go` 调用一次(`sync.Once`),`Get()` 返回缓存的
  `*Config`。
- `Validate()` 校验:`server.name` / `server.addr` 非空、超时非负、
  `caller_allowlist` 不变量(`header` 必填,`allowed_callers` 在
  `allow_missing=false` 时必须非空)、`Database.Enabled` 时连接池参数
  非负。
- `Config` 中所有时长类字段(例如 `rpc.request_timeout_seconds`、
  `database.health_check_period_seconds`、`rate_limit.rule.window_seconds`)
  统一使用 `go-framework/config` 的 `config.Duration`。`conf/dev/conf.yaml`
  中这些字段以 duration 字符串(如 `"3s"` / `"30s"`)填写,由
  `time.ParseDuration` 解析;不再接受裸整数。

### 3.2 RPC 错误映射(`internal/pkg/rpcerror`)

- `ToBizError(err)` 是 `go-common/error`(`goerror`)错误转
  `kerrors.BizStatusError` 的唯一入口,委托给
  `go-framework/kitex/rpcerror.OopsStatusAdapter`。已经是 `BizStatusError`
  的直接透传。
- 脚手架保留码(常量 re-export `go-framework/error`):
  - `CodeInternalError` = `frameworkerror.CodeSystem`(10000)—— 非 goerror /
    panic 兜底。
  - `CodeNotImplemented` = 10010 —— usecase 桩(占位;有意与
    `frameworkerror.CodeRPCUnavailable` 同值)。
  - `CodePermissionDenied` = `frameworkerror.CodeAuthFailed`(10002)—— caller
    allowlist 拒绝(go-tools v0.1.0 无 `CodePermissionDenied`,映射到
    `CodeAuthFailed`)。
  - `CodeRPCTimeout` = `frameworkerror.CodeRPCTimeout`(10011)—— 请求超时。
  - `CodeConfigInvalid` = `frameworkerror.CodeConfigInvalid`(10004)—— 配置
    加载 / 校验失败。
- 错误生产端用 `goerror.In("...").Code(code).Public("msg")…`
  (`goerror` = `go-common/error`,内部包装 `samber/oops`;不要直接
  `import "samber/oops"` 构造错误)。
- 辅助函数:`InternalErrorf`、`TimeoutError`、`PermissionDenied`、
  `BizCode(err)`、`FormatBiz(err)`(`AccessLog` 使用)。
- 码段遵循 go-tools:Framework `10000–10499`、Middleware `20000–20699`、
  Auth `40000–40099`、Project `40100–59999`。**业务自定义码必须 `>= 40100`
  (`goerror.ProjectCodeMin`)。** 业务码经 `goerror.HTTPStatus` 兜底返回
  **HTTP 200**(「业务错误、RPC 调用成功」);如需细粒度 HTTP 状态,用
  `goerror.RegisterHTTPStatuses` 注册。

### 3.3 服务端拦截器(`internal/pkg/interceptor`)

| 拦截器 | 行为 | 失败 / 输出 |
|---|---|---|
| `RequestID` | 读 metainfo `x-request-id`,缺失时生成 16 字节十六进制并 `WithPersistentValue` 写回 | n/a |
| `AccessLog` | 包住 `next`,记录 service / method / latency / request_id;失败用 `rpcerror.FormatBiz(err)` 警告 | n/a |
| `Recovery` | `defer recover()`;panic 转 `rpcerror.InternalErrorf` 再 `ToBizError` | `CodeInternalError`(10000) |
| `RequestTimeout(d)` | `context.WithTimeout(ctx, d)`;`DeadlineExceeded` 且无错误时返回 `TimeoutError` 经 `ToBizError` | `CodeRPCTimeout`(10011) |
| `CallerAllowlist(enabled, header, allowed, allowMissing)` | 校验 metainfo header(默认 `x-caller-service`)是否在白名单 | `CodePermissionDenied`(= `frameworkerror.CodeAuthFailed`,10002) |

链路通过 `endpoint.Chain(...)` 在 `server.Run` 里组合,经
`kitexserver.WithMiddleware` 注册。caller allowlist 默认 header 是
`x-caller-service`(常量 `HeaderCallerService`)。

### 3.4 数据库(`cfg.Database.Enabled`)

- `internal/base/data/data.go` 暴露
  `Data{ Pool *pgxpool.Pool, Queries *gen.Queries }` 与 cleanup 闭包,
  cleanup 关闭连接池。
- `data.NewPostgresConfig(dsn)` 解析 DSN;`applyPostgresPoolConfig` 把
  `cfg.Database` 中的 `MaxConns / MinConns / MaxConnLifetime /
  MaxConnIdleTime / HealthCheckPeriod` 复制到 `*pgxpool.Config`。
- `data.NewPostgres(ctx, cfg)` 打开并 ping 连接池。失败带
  `goerror.Code("postgres_pool_open_failed" | "postgres_ping_failed")`。
- `internal/db/{schema,query,gen,migrations}` 形态与 Hertz 一致(共用
  `sqlc.yaml` 结构、共用 goose 迁移目录)。
- 事务由 repository 通过 `WithTx(ctx, fn)` 持有;辅助函数在错误或 panic
  时回滚,只有成功时提交。

### 3.5 调用端客户端(`pkg/client/<service>`)

`client.yaml` 提供给其他服务 / Hertz 网关调用本 RPC 的类型化封装:

- `Config` 包含 `ServiceName`、`CallerService`、`HostPorts`、RPC 与连接
  超时、`EnableMetaInfo`(TTHeader),以及 `RetryConfig`(`Backoff` ∈
  `none|fixed|random`、熔断错误率上限 0.3、`MaxRetryTimes` ∈ [1, 5])。
- `New(ctx, cfg, opts...)` 构造 kitex client,可选挂上 `failurePolicy()`
  返回的 `WithFailureRetry` 与一个把 `x-caller-service` 写入出站 metainfo
  的中间件。
- `Validate()` 在超时非法、backoff 配置错、缺 service name 时返回
  `goerror.In("kitex.client").Code(frameworkerror.CodeConfigInvalid).Public("config_invalid")`。
- `NewClient` 失败时包装为
  `goerror.In("kitex.client").Code(frameworkerror.CodeRPCUnavailable).Public("rpc_failed")`
  (`frameworkerror.CodeRPCUnavailable` = 10010)。

### 3.6 运维

- `Makefile` 目标:`build`、`run`、`dev`(air 或 `go run .`)、
  `update`(重跑 `kitex -template-dir template/kitex-template`)、
  `sqlc`、`generate`(= `update` + `sqlc`)、
  `migrate-{up,down,status,create}`、`lint`、`test`、`check`、`tidy`、
  `install-tools`、`clean`。
- 即使是默认 starter 场景,`internal/base/data` 与 repository 接线也会
  import `internal/db/gen`,因此首次 `go mod tidy` 或 build 前要先执行
  `make sqlc`;如果直接跑 `make generate`,其中已包含这一步。
- 入口 `main.go` 只做 `conf.Init()` → `server.Run()`;Agent 增加接线一律
  落到 `internal/base/server/server.go`。
- 不内置 health / readiness 探针(kitex 服务通常依赖 sidecar 或 TTHeader
  探活);若平台需要 HTTP 探针,在 `server.Run` 内额外加。

### 3.7 可选基础设施片段

`ncgo add infra <kind>`(或手动从
`internal/assets/_data/kitex/optional/<kind>.go` 拷贝)按 kind 落到
`internal/base/{data,registry}/` 下。每个文件只发布类型化
构造函数;Agent 需要把 config struct 与构造函数一起注册到 `server.Run`
内的 `samber/do` injector(`registry_polaris` add-on 直接当 kitex server /
client option 接入,不走 `do`)。Kitex 侧 add-on 的
`goerror.Code` 用字符串(`<kind>_<reason>`),与 Hertz 的数字 errcode 注册
表不同。observability(可观测性)已由 kitex 基础模板(go-framework OTLP,
`cfg.Jaeger` 驱动)内置提供,不再以独立 add-on 形态存在。

#### Redis(`data/redis.go`)

- 结构同 Hertz 版。
- 错误码:`redis_config_missing`(opts 为 nil)、
  `redis_ping_failed`(`Ping` 失败)。
- 接线:
  ```go
  do.ProvideValue[context.Context](inj, startupCtx)
  do.ProvideValue(inj, &redis.UniversalOptions{Addrs: cfg.Redis.Addrs})
  do.Provide(inj, data.NewRedis)
  ```

#### Kafka(`data/kafka.go`)

- 错误码:`kafka_writer_missing` / `kafka_writer_addr_missing` /
  `kafka_writer_topic_missing`(producer);
  `kafka_reader_brokers_missing` / `kafka_reader_topic_missing`
  (consumer)。
- 接线同 Hertz 版;示例里默认消费者 `GroupID` 跟 RPC 服务名走
  (例如 `user-rpc`)。

#### Elasticsearch(`data/es.go`)

- 错误码:`elasticsearch_addresses_missing`、
  `elasticsearch_client_create_failed`、`elasticsearch_ping_failed`。
- 接线同 Hertz 版。

#### ClickHouse(`data/clickhouse.go`)

- 多带了 `clickhouse.ClientInfo.Products`(把服务名上报到 ClickHouse 端);
  Hertz 版没有。
- 错误码:`clickhouse_config_missing`、`clickhouse_addresses_missing`、
  `clickhouse_open_failed`、`clickhouse_ping_failed`。

#### Polaris 注册 / 发现(`registry/polaris.go`,kitex 专属)

- 暴露：`NewRegistry(cfg)` 返回 `kitexregistry.Registry`、
  `NewResolver(cfg)` 返回 `discovery.Resolver`,内部委托
  `kitex-contrib/polaris` 的 `NewPolarisRegistry` / `NewPolarisResolver`。
- 配置 struct：`PolarisConfig{ Addresses, Namespace, Protocol,
  TimeoutSeconds, ... }`,支持 `polaris.yaml`(项目根)与 `conf` 两种来源,
  `Validate()` 基于 goerror 校验。
- 依赖：`github.com/kitex-contrib/polaris`；`polaris.yaml` 由 add-on 一并
  产出到项目根,`kitex-contrib/polaris` 默认从工作目录读取。
- 错误码：`registry_config_invalid`(地址空 / 超时非法 / `polaris.yaml`
  解析失败)。
- 接线(仅在 `ncgo add infra registry_polaris --wire` 后,通过
  `// ncgo:wire:registry:server` / `// ncgo:wire:registry:client` 锚点注入):
  ```go
  r, err := registry.NewRegistry(cfg.Registry)
  if err != nil { return goerror.In("kitex.registry").Wrap(err) }
  // server: kitexserver.WithRegistry(r)
  // client: kitexclient.WithResolver(registry.NewResolver(cfg.Registry))
  ```
- `--wire` 会自动在 kitex base 的 server option 与 client 构造处插入
  `WithRegistry` / `WithResolver`;不加 `--wire` 时只产出 `polaris.go` 与
  `polaris.yaml`,不修改 base server/client。

#### Release 金丝雀 GA 加固(`release_ops.go`,SDK-neutral)

`ncgo add infra release_canary` 产出 `release_ops.go`(仅 stdlib),在 canonical
`release_canary.go` seam 之上叠加生产级加固装饰器:

- **TTL 缓存**(`CacheOptions`):默认 TTL 30s / stale-while-revalidate 窗口
  5m / 刷新抖动 ±20%(`Jitter=0.2`);`StaleTTL=0` 关闭 stale 窗口,负值回落到
  默认值。同 key 刷新走 single-flight。
- **FailPolicy(降级语义)**:`FailOpen`(默认)在 discovery / rule 出错时返回
  上次成功值或空池,保可用;`FailFast` 直接拒绝请求。
- **Observer(可观测性)**:`SlogObserver`(stdlib `log/slog`,默认)输出结构化
  决策 / 回退 / discovery / rule 日志;OTel metrics observer 随 `polaris_adapter`
  一同产出,通过 `NewOTelObserver(meter)` 构造(复用 kitex base 已接入的
  go-framework OTLP meter)。
- **Engine + DryRun**:`NewEngine(EngineOptions{...})` 组合 discovery + rules
  + observation。`DryRun=true` 进入 shadow 模式——记录决策意图但实际流量仍
  走 stable 池——首次上线新 `RuleSet` 时使用。

**Resolver 可见性说明。** `kitex-contrib/polaris` resolver 使用 `GetInstances`,
返回的是**路由过滤后的子集**。金丝雀 LB 因此通过
`KitexCanaryLoadBalancer.Discoverer` 获取**完整的 stable+canary 池**——底层
是 adapter 的 `GetAllInstances`(已缓存)。当 `Discoverer` 为 nil 时,沿用旧
的 resolver-based `KitexResultDiscoverer` 路径(向后兼容)。

生成项目中的典型接线:

```go
lister, _ := release.NewPolarisInstanceLister(discoveryCfg)
cached := release.NewCachingDiscoverer(
    release.PolarisDiscoverer{Config: discoveryCfg, ListInstances: lister},
    release.CacheOptions{}, release.FailOpen, obs)
lb := release.NewKitexCanaryLoadBalancer("svc", rulesProvider, fallbackLB)
lb.Discoverer = cached
lb.Observer = obs
```

**排障(troubleshooting)。**
- 注册中心不可达:`FailOpen` 下返回 stale 或空池(保可用);`FailFast` 下
  调用被拒绝。
- 指标基数问题已规避:rule 名称**只出现在结构化日志中**,不会成为 metric
  label。
- 首次上线新 `RuleSet` 时,将 `EngineOptions.DryRun=true` 开到 shadow 模式,
  直到 shadow 决策符合预期。
- 凭证仅环境变量(`POLARIS_TOKEN` / `POLARIS_NAMESPACE`);observer 与错误
  路径绝不记录。

#### Observability(go-framework OTLP,kitex 基础内置)

kitex 基础模板已经接入 go-framework OTLP,**不需要** `ncgo add infra` 额外
add-on。`cfg.Jaeger != nil && cfg.Jaeger.Enable` 时,`server.go` 调用
`kitexobs "github.com/byx-darwin/go-tools/go-framework/kitex/observability"`
的 `kitexobs.NewProvider(ctx, config.ObservabilityConfig{Enabled, Endpoint,
ServiceName})`,把 `provider.ServerSuite()` 挂到 kitex server option,并在
`server.Run` 退出前 `defer provider.Shutdown()`。

> 历史说明:原 LoongSuite `observability_otel` / `otel` add-on 与
> `kitex-contrib/registry-etcd` add-on 已在 PR5 中移除。可观测性统一使用
> go-framework OTLP;注册/发现统一切换到 Polaris。既有的 `otel` /
> `registry_etcd` kind 现在返回 invalid kind。

## 4. 文件清单

| 文件 | 作用 |
|---|---|
| `kitex/kitex-template/main.yaml` | `main.go` 入口;调用 `conf.Init()` 后 `server.Run()` |
| `kitex/kitex-template/server.yaml` | `internal/base/server/server.go`:kitex server 启动 + samber/do 装配 |
| `kitex/kitex-template/handler.yaml` | Handler 外壳,委托到 `usecase.UseCase` |
| `kitex/kitex-template/usecase.yaml` | Usecase 桩 |
| `kitex/kitex-template/repository.yaml` | Repository 桩 |
| `kitex/kitex-template/conf.yaml` / `conf_dev.yaml` | base/conf 包 + `conf/dev/conf.yaml` |
| `kitex/kitex-template/data.yaml` | base/data injector 引导 |
| `kitex/kitex-template/interceptor.yaml`(含 `_test`) | 服务端拦截器脚手架 |
| `kitex/kitex-template/rpcerror.yaml`(含 `_test`) | RPC 错误映射(对应 hertz `pkg/response`) |
| `kitex/kitex-template/client.yaml`(含 `_test`) | 生成的 client 包装 |
| `kitex/kitex-template/migration_init.yaml` / `migration_keep.yaml` | sqlc/atlas 迁移占位 |
| `kitex/kitex-template/makefile.yaml` | Makefile 目标(`make dev`、`make sqlc` 等) |
| `kitex/sqlc.yaml` | sqlc 配置,结构与 Hertz 版相同 |
| `kitex/optional/{redis,kafka,es,clickhouse,registry_polaris}.go` | kitex 族的 `add infra` 素材(`registry_polaris` 同时产出 `polaris.yaml` 到项目根) |

## 5. `kitex-template/*.yaml` 语义

每个 YAML 是一条形如下面的记录:

```yaml
path: <相对项目根的输出路径>
update_behavior:
  type: cover   # cover | skip
body: |-
  <Go template 主体>
```

kitex 工具通过 `--template-extension` 读取这些记录,然后把每条按 `path`
渲染输出。

渲染上下文(对照已发布模板核对):

| 变量 | 含义 | 示例 |
|---|---|---|
| `{{.Module}}` | Go 模块路径 | `example.com/demo` |
| `{{.ServiceInfo.ServiceName}}` | IDL 中的服务名 | `Demo` |
| `{{.ServiceInfo.ImportPath}}` | 生成的 kitex 客户端 import path | `example.com/demo/kitex_gen/...` |
| `{{ToLower x}}` | 转小写辅助函数 | `demo` |

`update_behavior.type`:

- `cover` —— kitex 再次生成时会覆盖该文件。用于不允许用户手改的文件
  (`main.go`、生成的 handler/usecase 外壳)。
- `skip` —— kitex 不动已存在的文件。用于初始脚手架后用户需要手改的文件
  (例如初版生成完后在 `usecase.yaml` 里写业务逻辑)。

## 6. 可选基础设施

每个 `optional/*.go` 文件都是字节级原样复制素材。`infra.Add` 从嵌入 FS
读取后写到对应目标路径,通常是 `internal/base/data/<kind>.go`,也可以是
`internal/base/registry/` 等专门包。

新增 optional 文件的约束:

- **不得** import 项目特有包。
- 包名必须匹配目标包(`data`、`registry` 等)。
- 文件顶部注释必须列出依赖和接线说明。

当前已发布:`redis`、`kafka`、`es`、`clickhouse`,以及 Kitex-only
`registry_polaris`。observability 已由 kitex 基础模板(go-framework OTLP)
直接提供,不再以独立 add-on 形态存在。

## 7. 与 Hertz 的差异

| 方面 | Hertz | Kitex |
|---|---|---|
| Module 变量名 | `{{.GoModule}}` | `{{.Module}}` |
| 布局容器 | 单个 `layout.yaml` 列出全部文件 | 每个输出路径一个 YAML 文件 |
| Handler 模板 | `--customize_package`(`package.yaml`) | per-path 模板 `handler.yaml` |
| 变量来源 | `data.json`(独立) | 内置于 kitex 渲染上下文 |
| 可选基础设施 | 4 种(数据类 add-on,observability 由基础模板提供) | 5 种(多 `registry_polaris`) |

## 8. 维护契约

任何 `_data/kitex/` 下的改动都必须:

1. 更新文件内容。
2. 同步更新本文档(及 `design-doc.en.md`)对应章节。
3. 升 `_data/VERSION` 的 `ncgo_assets_version`。
4. 重跑 golden:
   `go test ./internal/scaffold/mono/... -update-golden -count=1`,然后
   `go test ./internal/scaffold/mono/... -count=1`。
5. 若是结构性新增,把新路径加入 `internal/assets/assets_test.go`。

`ncgo doctor` 输出里的 `Rule:` 锚点继续指向
`nc-skills-golang/SKILL.md#layer-rules`。模板归 ncgo;review 规则留在
nc-skills-golang。

## 9. 引用

- `docs/prd.md` §3(决策)、§5(Manifest)、§9(仓库布局)、§10(v0.2)
- `internal/assets/assets.go` —— embed 接线
- `internal/scaffold/infra/infra.go` —— optional 消费方
- `nc-skills-golang/SKILL.md` —— review 模式规则与 AI 调用指南
- `docs/hertz/design-doc.zh-CN.md` —— Hertz 对应文档
