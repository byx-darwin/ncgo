# Hertz 模板详细设计

读者:ncgo 维护者及读取或修改 `internal/assets/_data/hertz/` 嵌入模板树
的 AI Agent。本文档描述每个文件的作用、暴露给脚手架的契约,以及在不破
坏已生成项目的前提下如何演进。

Kitex 对应文档见
[`docs/kitex/design-doc.zh-CN.md`](../kitex/design-doc.zh-CN.md)。

## 1. 总览

Hertz 模板族支撑 `ncgo new --mode mono`(HTTP 服务),由
`internal/scaffold/mono` 消费,经 `hz` ≥ v0.9.7 渲染。

模板通过 `//go:embed all:_data` 嵌入 ncgo 二进制(见
`internal/assets/assets.go`)。目录名带前导下划线 (`_data/`) 是为了让
`go build ./...` 忽略 `optional/*.go` —— 这些文件是模板素材,不是参与
ncgo 二进制编译的 Go 源码。

资产版本:`_data/VERSION`(`ncgo_assets_version: 0.1.1`),由
`assets.Version()` 暴露。

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
│   │   └── middleware/              # signature、token、rate_limit、idempotency、cors 等
│   └── router/register.go           # hz 插入点
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
- 所有返回错误必须是带 `Code` + `Public` 的 `samber/oops` 链。

### 2.3 依赖注入(`samber/do`)

`internal/base/server/server.go` 构建 injector,按固定顺序注册 provider。
模板里给出的骨架:

```go
injector := do.New()
defer func() { _ = injector.Shutdown() }()
do.ProvideValue(injector, cfg)                 // *conf.Config

// DB 服务追加:
do.ProvideValue[context.Context](injector, startupCtx)
do.ProvideValue(injector, pgCfg)               // *pgxpool.Config
do.Provide(injector, data.NewPostgres)         // *pgxpool.Pool
do.Provide(injector, data.New)                 // *data.Data + cleanup
do.Provide(injector, repository.NewUserRepo)
do.Provide(injector, adapter.NewDeviceAdapter)
do.Provide(injector, usecase.NewUserUseCase)
do.Provide(injector, handler.NewUserHandler)
```

`defer injector.Shutdown()` 会执行 provider 返回的 cleanup(例如
`data.New` 关闭 pgx 连接池)。

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
  `oops.Code(10308) Public("config_invalid")`。
- `Init()` 由 `main.go` 调用一次(`sync.Once`),`Get()` 返回缓存的
  `*Config`。
- `Validate()` 校验:超时非负、签名 / 令牌密钥、CORS 通配 +
  `allow_credentials` 互斥、`rate_limit` 后端与至少一条规则、
  `idempotency` 后端。

### 3.2 响应与错误码(`internal/pkg/response`)

- `Reply` / `OK` / `Err` / `ErrorCode` / `BindError` 根据 `Accept` 头返回
  JSON 或 Protobuf(payload 实现 `proto.Message` 时优先 protobuf)。
- 内置 5 位错误码:
  - `1xxxx` —— 请求 / 认证 / 限流 / 依赖错误。
  - 脚手架保留段:`10000–10399`(在此区间 `Register` 会 panic)。
- 业务码注册写在进程启动期:
  ```go
  var CodeOrderConflict = response.MustRegister(response.Definition{
      Code: 39001, Msg: "order_conflict", Status: consts.StatusConflict,
  }).Code
  ```
- 错误生产端用 `oops.In("...").Code(code).Public("msg")…`;
  `response.Err` 提取 `(code, msg)` 并经 `StatusFromCode` 映射 HTTP 状态。

### 3.3 安全中间件(`internal/pkg/middleware`)

| 中间件 | 触发条件 | 后端 | 失败码 |
|---|---|---|---|
| `SignatureAuth` | `cfg.Auth.Signature.Enabled` | HMAC-SHA256(`method\npath\nquery\nts\nnonce\nbody`);nonce 存储 `memory`(LRU)或 `redis` | `10101 signature_missing` / `10102 signature_expired` / `10103 signature_invalid` / `10202 replay_request` |
| `JWTAuth` | `cfg.Auth.Token.Enabled` | `golang-jwt/jwt/v5` HS256;读 `cfg.Auth.Token.Header`;claims 写入 context key `tokenClaims` | `10104 token_missing` / `10105 token_invalid` / `10106 token_expired` / `10107 claims_invalid` |
| `RateLimit` | `cfg.RateLimit.Enabled` + 单条规则开关 | 令牌桶;`memory`(samber/hot LRU)或 `redis` | `10200 rate_limited`(`fail_open=false` 且后端故障时报 `10304 cache_unavailable`) |
| `Idempotency` | `cfg.Idempotency.Enabled` | 重放缓存,key = `header + method + path + body-hash`;`memory` 或 `redis` | `10203 idempotency_key_missing` / `10204 idempotency_conflict` |
| `InternalOnly` | 始终启用 | CIDR + 路径白名单 | `10108 permission_denied` |
| `CORS` | `cfg.CORS.Enabled` | 静态配置;通配 origin 与 credentials 互斥 | n/a |

`Unless(mw, skipper)` 与 `PathSkipper(paths...)` 包住认证中间件,使
public 路径(默认 `/healthz`、`/readyz`,加上 `cfg.Auth.PublicPaths`)
绕过签名 / JWT。

`fail_open`(rate-limit / signature nonce / idempotency)把"依赖故障 →
拒绝请求"改为"依赖故障 → 放行"。仅当后端 Redis 不是关键路径时才用。

### 3.4 数据库(`with_database: true`)

脚手架带 `--db postgres` 时:

- `internal/base/data/data.go` 暴露 `Data{ Pool *pgxpool.Pool, Queries *gen.Queries }`,
  `New(pool)` 返回的 cleanup 会关闭连接池。
- `data.NewPostgres(ctx, *pgxpool.Config)` 打开并 ping 连接池。完整的
  `*pgxpool.Config` 通过 `samber/do` 注入,保留所有选项(hooks、超时、
  before-acquire 等)。
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
  `update`(用内置 `package.yaml` 重跑 `hz update`)、`sqlc`、
  `migrate-{create,up,down,status}`、`lint`、`test`、`tidy`、
  `install-tools`。
- `cmd/server/main.go` 只做 `conf.Init()` + `server.Run()`;Agent 增加
  接线一律落到 `internal/base/server/server.go`。

### 3.6 可选基础设施片段

`ncgo add infra <kind>` 从
`internal/assets/_data/hertz/optional/<kind>.go` 或 common
`internal/assets/_data/optional/<kind>.go` 字节级复制一个 Go 文件。数据客户端
通常落到 `internal/base/data/`;专门能力可以落到
`internal/base/observability/` 等包。

#### Redis(`redis.go`)

- 暴露:`*data.Redis{ Client redis.UniversalClient }`(`UniversalOptions`
  自动判定 single / cluster / sentinel)。
- 依赖:`github.com/redis/go-redis/v9`。
- 错误码:`10308 config_invalid`(opts 为 nil)、
  `10304 cache_unavailable`(`Ping` 失败)。
- 接线:
  ```go
  do.ProvideValue[context.Context](inj, startupCtx)
  do.ProvideValue(inj, &redis.UniversalOptions{Addrs: cfg.Redis.Addrs, DB: cfg.Redis.DB})
  do.Provide(inj, data.NewRedis)
  ```

#### Kafka(`kafka.go`)

- 暴露:`*data.KafkaWriter{ W *kafka.Writer }` 与/或
  `*data.KafkaReader{ R *kafka.Reader }`(segmentio/kafka-go)。
- 依赖:`github.com/segmentio/kafka-go`。
- 错误码:`10308 config_invalid`(Writer 为 nil / 缺 `Addr` / `Topic` /
  `Brokers`)。
- 接线(producer):
  ```go
  do.ProvideValue(inj, &kafka.Writer{Addr: kafka.TCP(cfg.Kafka.Brokers...), Topic: cfg.Kafka.Topic})
  do.Provide(inj, data.NewKafkaWriter)
  ```

#### Elasticsearch(`es.go`)

- 暴露:`*data.ES{ Client *elasticsearch.Client }`(go-elasticsearch v8)。
- 依赖:`github.com/elastic/go-elasticsearch/v8`。
- 错误码:`10308 config_invalid`(`Addresses` 空)、
  `10306 search_unavailable`(`NewClient` / `Ping` 失败)。
- 接线:
  ```go
  do.ProvideValue[context.Context](inj, startupCtx)
  do.ProvideValue(inj, elasticsearch.Config{Addresses: cfg.ES.Addresses, Username: cfg.ES.Username})
  do.Provide(inj, data.NewES)
  ```

#### ClickHouse(`clickhouse.go`)

- 暴露:`*data.ClickHouse{ Conn driver.Conn }`(clickhouse-go v2 原生协议)。
- 依赖:`github.com/ClickHouse/clickhouse-go/v2`。
- 错误码:`10308 config_invalid`(opts 为 nil / `Addr` 空)、
  `10303 database_unavailable`(`Open` / `Ping` 失败)。
- 接线:
  ```go
  do.ProvideValue[context.Context](inj, startupCtx)
  do.ProvideValue(inj, &clickhouse.Options{Addr: cfg.CH.Addrs, Auth: clickhouse.Auth{Database: cfg.CH.DB}})
  do.Provide(inj, data.NewClickHouse)
  ```

## 4. 文件清单

| 文件 | 作用 | 消费方 |
|---|---|---|
| `hertz/layout.yaml` | 自定义 `hz` 布局:工程目录树 + base/conf/server/middleware Go 源码 | `hz new --customize_layout` |
| `hertz/package.yaml` | 自定义 `hz` package 模板:handler.go 用 `Handler` struct 委托给 `useCase` 接口,并附带 usecase 桩 | `hz new --customize_package` |
| `hertz/data.json` | `layout.yaml` 渲染时变量(`GoModule`、`ServiceName`、`WithDatabase`) | `hz new --customize_layout_data_path` |
| `hertz/sqlc.yaml` | `--db postgres` 时复制到项目里的 sqlc 配置参考 | `mono` 脚手架 |
| `hertz/optional/{redis,kafka,es,clickhouse}.go`、`optional/observability_otel.go` | `ncgo add infra <kind>` 的 drop-in 文件 | `internal/scaffold/infra` |

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
hz new --mod=<module> --idl=<idl> \
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

每个 `optional/*.go` 文件都是字节级原样复制素材。`infra.Add` 从嵌入 FS
读取后写到对应目标路径,通常是 `internal/base/data/<kind>.go`,也可以是
`internal/base/observability/otel.go` 等专门包。

新增 optional 文件的约束:

- **不得** import 项目特有包,只允许 stdlib + 第三方依赖,且要由
  `next steps` 提示用户 `go get`。
- 包名必须匹配目标包(`data`、`observability` 等)。
- 文件顶部注释必须把注册调用片段原样列出。

当前已发布:`redis`、`kafka`、`es`、`clickhouse`、`observability_otel`
(`otel` alias)。

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
- `internal/assets/assets.go` —— embed 接线
- `internal/scaffold/mono/files.go` —— hertz 消费方
- `internal/scaffold/infra/infra.go` —— optional 消费方
- `nc-skills-golang/SKILL.md` —— review 模式规则与 AI 调用指南
- `docs/kitex/design-doc.zh-CN.md` —— Kitex 对应文档
