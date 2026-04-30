# Hertz / Kitex 统一日志方案

## 1. 观点

日志体系应该作为基础设施能力统一提供，而不是散落在 Hertz handler、Kitex handler、usecase 或 repository 中。

推荐模型：

```text
业务代码 / handler / usecase / repository
        |
        |-- 返回或包装 samber/oops error
        v
统一 logging helper
        |
        |-- console writer
        |-- typed file writers
        |-- rotated + compressed files
        |-- trace / request / release / canary context enrich
        v
日志采集系统 / 本地文件 / 控制台
```

核心原则：

- 业务继续使用 `github.com/samber/oops` 表达错误语义。
- 日志层负责解析 `oops`，并完成分类、结构化、落盘、压缩和链路关联。
- LoongSuite Go Agent 负责 trace / metrics 自动插桩；logging 只读取 trace context，不初始化 OTel SDK。
- 容器环境默认输出 console；VM / 物理机可启用 file + compress。

## 2. 当前状态

当前 ncgo 默认生成物（未执行 `ncgo add infra logging`）具备：

| 能力 | 状态 |
|---|---|
| Hertz access log | 有，基于 `hlog` |
| Kitex access log | 有，基于 `klog` |
| request_id | 有 |
| 控制台输出 | 有 |
| 文件输出 | 无 |
| 文件压缩 | 无 |
| 日志分类 | 未系统化 |
| JSON 结构化 | 无 |
| trace_id / span_id | 未标准化 |
| oops 结构化解析 | 无 |
| canary / release metadata | 无 |

结论：当前是 **控制台有、文件无、链路弱、错误语义未结构化**。

## 3. 目标能力

统一日志方案需要支持：

- console / file / both / none 输出模式；
- JSON / text 格式；
- access / error / biz / rpc / db / panic / audit / security 分类；
- 按类型写入不同文件；
- 文件 rotate；
- 历史文件 gzip 压缩；
- `oops` 解析；
- `request_id` / `trace_id` / `span_id`；
- `service.version` / `release.track` / `traffic.lane`；
- Hertz middleware；
- Kitex interceptor；
- LoongSuite Go Agent 兼容。

## 4. 输出策略

### 4.1 容器 / Kubernetes

推荐默认：

```text
mode: console
format: json
```

由平台采集 stdout / stderr：

```text
app stdout
  -> container runtime
  -> Fluent Bit / Vector / Filebeat
  -> Loki / Elasticsearch / SLS / OpenSearch
```

不建议容器环境默认写文件，避免挂载、清理、rotate 和磁盘爆满问题。

### 4.2 VM / 物理机

推荐默认：

```text
mode: both
format: json
file.compress: true
```

由文件采集器采集 `logs/*.log`，压缩历史文件只用于留存。

## 5. 配置模型

建议写入 `conf/dev/conf.yaml` 或等价配置：

```yaml
logging:
  enabled: true
  mode: both          # console | file | both | none
  format: json        # json | text
  level: info         # debug | info | warn | error
  add_source: true

  console:
    enabled: true
    color: false

  file:
    enabled: true
    dir: logs
    filename: app.log
    max_size_mb: 100
    max_backups: 10
    max_age_days: 30
    compress: true

  categories:
    access:
      enabled: true
      file: access.log
      level: info
    error:
      enabled: true
      file: error.log
      level: error
    biz:
      enabled: true
      file: biz.log
      level: warn
    rpc:
      enabled: true
      file: rpc.log
      level: info
    db:
      enabled: true
      file: db.log
      level: warn
    panic:
      enabled: true
      file: panic.log
      level: error
    audit:
      enabled: true
      file: audit.log
      level: info
    security:
      enabled: true
      file: security.log
      level: warn

  context:
    request_id: true
    trace_id: true
    span_id: true
    release: true
    canary: true
```

## 6. 日志分类

| 分类 | 文件 | 说明 |
|---|---|---|
| `access` | `logs/access.log` | HTTP/RPC 请求访问日志 |
| `error` | `logs/error.log` | 系统错误、未知错误、基础设施错误 |
| `biz` | `logs/biz.log` | 业务可预期错误 |
| `rpc` | `logs/rpc.log` | Kitex 调用、下游调用、超时、重试 |
| `db` | `logs/db.log` | 数据库、事务、慢查询、连接失败 |
| `panic` | `logs/panic.log` | panic 和 stack |
| `audit` | `logs/audit.log` | 登录、权限、配置、灰度规则变更 |
| `security` | `logs/security.log` | 签名失败、JWT 无效、频控、越权 |

默认可同时写入：

```text
console + app.log + category.log
```

是否写多份由配置决定。

## 7. 文件滚动与压缩

推荐使用：

```text
gopkg.in/natefinch/lumberjack.v2
```

能力：

- 按大小切割；
- 保留 N 个历史文件；
- 保留 N 天；
- 自动 gzip 压缩；
- 适合单进程写日志。

示例：

```text
logs/
├── app.log
├── access.log
├── error.log
├── biz.log
├── rpc.log
├── db.log
├── panic.log
├── audit.log
├── security.log
├── access-2026-04-30T10-00-00.001.log.gz
└── error-2026-04-30T10-00-00.001.log.gz
```

压缩历史文件默认不采集，只用于留存、审计、下载。

## 8. 标准字段

所有日志都应包含基础字段：

```json
{
  "ts": "2026-04-30T10:00:00.000Z",
  "level": "info",
  "category": "access",
  "msg": "http request completed",
  "service.name": "user-api",
  "service.kind": "hertz",
  "service.version": "v1.2.3",
  "release.track": "canary",
  "release.git_sha": "abc1234",
  "request_id": "req-xxx",
  "request_id_source": "header",
  "trace_id": "4bf92f3577b34da6a3ce929d0e0e4736",
  "span_id": "00f067aa0ba902b7",
  "traffic.lane": "canary"
}
```

## 9. request_id 与 trace_id

不建议完全合并两者：

- `trace_id` 是链路追踪 ID，由 LoongSuite / OTel / 上游生成。
- `request_id` 是业务请求相关性 ID，用于日志检索、客服排查、跨系统关联。

推荐生成规则：

```text
request_id = incoming request/correlation id
          ?? trace_id
          ?? generated id
```

日志同时输出：

```text
request_id
request_id_source
trace_id
span_id
```

如果使用 `trace_id` 作为 fallback：

```json
{
  "request_id": "4bf92f3577b34da6a3ce929d0e0e4736",
  "request_id_source": "trace_id",
  "trace_id": "4bf92f3577b34da6a3ce929d0e0e4736"
}
```

## 10. 和 samber/oops 的结合

业务错误和基础设施错误继续使用 `github.com/samber/oops` 包装。

示例：

```go
oops.
  In("clickhouse").
  Tags("analytics", "clickhouse", "connection").
  Code(10303).
  Public("database_unavailable").
  With("addrs_count", len(opts.Addr)).
  Wrapf(err, "clickhouse.Ping")
```

日志层解析为：

| oops 信息 | 日志字段 |
|---|---|
| `Code(...)` | `error.code` |
| `Public(...)` | `error.public` |
| `Tags(...)` | `error.tags` |
| `In(...)` | `error.scope` |
| `With(k, v)` | `error.attrs.<k>` |
| wrapped cause | `error.cause` |
| error message | `error.message` |

示例输出：

```json
{
  "level": "error",
  "category": "db",
  "msg": "database unavailable",
  "error.code": 10303,
  "error.public": "database_unavailable",
  "error.scope": "clickhouse",
  "error.tags": ["analytics", "clickhouse", "connection"],
  "error.attrs.addrs_count": 2,
  "error.message": "clickhouse.Ping: context deadline exceeded"
}
```

## 11. oops 分类规则

### 11.1 按 tag 分类

| oops tag | category |
|---|---|
| `database` / `postgres` / `clickhouse` / `sql` | `db` |
| `redis` / `cache` | `error` 或 `cache` |
| `kafka` / `event` | `error` 或 `event` |
| `rpc` / `kitex` / `downstream` | `rpc` |
| `auth` / `jwt` / `signature` | `security` |
| `permission` / `forbidden` | `security` |
| `validation` / `biz` | `biz` |
| `config` | `error` |
| `panic` | `panic` |

### 11.2 按 code range 分类

建议长期统一 code range：

| code 范围 | category |
|---|---|
| `101xx` | auth / security |
| `102xx` | business |
| `103xx` | infrastructure / db / cache |
| `104xx` | downstream / rpc |
| `105xx` | data |

## 12. Hertz 集成

生成：

```text
internal/base/logging/hertz.go
```

职责：

1. 读取 `X-Request-ID` / `X-Correlation-ID`。
2. 从 context 读取 `trace_id` / `span_id`。
3. fallback 生成 `request_id`。
4. 写回响应头 `X-Request-ID`。
5. 读取 `X-Traffic-Lane`。
6. 请求结束后写 access log。
7. 状态码 >= 500 时写 error log。
8. panic 时写 panic log。
9. 解析 `oops` 并输出结构化字段。

Hertz access 示例：

```json
{
  "category": "access",
  "protocol": "http",
  "http.method": "GET",
  "http.path": "/api/users/1001",
  "http.status_code": 200,
  "latency_ms": 12.4,
  "request_id": "req-123",
  "trace_id": "4bf92f3577b34da6a3ce929d0e0e4736",
  "traffic.lane": "canary"
}
```

## 13. Kitex 集成

生成：

```text
internal/base/logging/kitex.go
```

职责：

1. 从 Kitex metadata 读取 `x-request-id` / `x-correlation-id`。
2. 从 context 读取 `trace_id` / `span_id`。
3. 读取 `traffic.lane`。
4. 读取 caller service。
5. 写 RPC access log。
6. 写 RPC error log。
7. panic 时写 panic log。
8. 解析 `oops`。
9. 下游调用继续透传 request/canary metadata。

Kitex RPC error 示例：

```json
{
  "category": "rpc",
  "level": "error",
  "rpc.system": "kitex",
  "rpc.service": "user.UserService",
  "rpc.method": "GetUser",
  "rpc.status": "error",
  "caller.service": "web-bff",
  "latency_ms": 3001,
  "request_id": "req-123",
  "trace_id": "4bf92f3577b34da6a3ce929d0e0e4736",
  "traffic.lane": "canary",
  "error.code": 10403,
  "error.public": "downstream_unavailable"
}
```

## 14. LoongSuite 兼容

LoongSuite Go Agent 负责：

- 编译期自动插桩；
- trace / metrics；
- OTel SDK 初始化。

logging 负责：

- 从 context 读取 `trace_id` / `span_id`；
- 写入日志字段；
- 不初始化 OTel SDK；
- 不创建 exporter。

建议只引入轻量依赖：

```text
go.opentelemetry.io/otel/trace
```

不引入：

```text
go.opentelemetry.io/otel/sdk
go.opentelemetry.io/otel/exporters/...
```

## 15. 和金丝雀结合

日志必须包含：

```text
release.track
service.version
traffic.lane
```

金丝雀排障常用查询：

```text
service.name=user-rpc AND release.track=canary AND category=error
```

对比查询：

```text
release.track=stable vs release.track=canary
```

## 16. 和 Nacos / Polaris metadata 结合

日志中的 release 字段应与注册中心 metadata 保持一致：

```text
SERVICE_NAME=user-rpc
SERVICE_KIND=kitex
SERVICE_VERSION=v1.2.3
RELEASE_TRACK=canary
GIT_SHA=abc1234
BUILD_TIME=2026-04-30T10:00:00Z
```

来源优先级：

```text
环境变量 > 构建注入 > manifest > 默认值
```

这些值同时用于：

- Nacos metadata；
- Polaris metadata；
- 日志字段；
- trace attributes；
- metrics labels。

## 17. ncgo 产品化状态

已新增 optional：

```bash
ncgo add infra observability_logging --root .
ncgo add infra logging --root .  # alias
ncgo add infra logging --root . --wire  # 可选：自动修改默认 server/client wiring
ncgo add infra logging --root . --wire --dry-run  # 预览，不修改文件
ncgo add infra logging --root . --wire --dry-run --output json  # 输出机器可读 plan
```

推荐落地流程：

1. 先执行 `--wire --dry-run`，确认会创建的 optional 文件、manifest 更新以及将修改的 server/client 文件；
2. 若需要给 CI / agent / 前端消费，使用 `--output json` 读取结构化 `plan`；
3. 确认无误后再执行不带 `--dry-run` 的 `--wire`；
4. 最后按 next steps 手动执行 `go get ...` / `go mod tidy`，ncgo 不会自动安装依赖。

`--wire` 在真实写入 optional 文件、manifest 和源码前会先做 preflight：如果默认模板 anchor 缺失、import block 找不到或格式化失败，会直接报错，并避免留下“optional 已写但 wiring 失败”的半完成状态。`--dry-run` 保证不写 optional 文件、不保存 manifest、不修改 server/client 源码。

JSON plan 常见条目：

```text
file/create                  # 将创建 logging.go / hertz.go / kitex.go
file/overwrite               # 加 --force 时将覆盖已存在 optional 文件
manifest/add                 # manifest 将记录 observability_logging
manifest/already_present     # manifest 已记录该 optional
wire/update                  # 将修改默认 server/client wiring
wire/already_wired           # 已接线，无需再次修改源码
next_step/run                # 需要用户手动执行的依赖安装命令
```

manifest 记录：

```yaml
infra:
  - observability_logging
```

当前生成文件：

```text
internal/base/logging/logging.go   # 通用 core：config/logger/context/category/file/oops
internal/base/logging/hertz.go     # Hertz 项目额外生成
internal/base/logging/kitex.go     # Kitex 项目额外生成
```

说明：第一版为方便安装和演进，把通用能力收敛在 `logging.go`；后续如果文件继续变大，可在不改变 API 的前提下拆成 `config.go`、`logger.go`、`file.go`、`oops.go` 等多个文件。

依赖 next steps：

```bash
go get github.com/samber/oops
go get gopkg.in/natefinch/lumberjack.v2
go get go.opentelemetry.io/otel/trace
go mod tidy
```

默认 Hertz / Kitex 模板已经预留安全 wiring 注释：未启用 `observability_logging` 时不会 import `internal/base/logging`，项目仍可正常编译；执行 `ncgo add infra logging --root .` 后，再按注释或下方示例手动替换默认 access/recovery 日志。若希望由 CLI 自动替换默认生成代码，可显式加 `--wire`；该选项只支持已知的 ncgo 默认模板片段，失败时会报错并提示无法找到 anchor。建议真实执行前先用 `--wire --dry-run --output json` 审阅 plan。

## 18. 安全 wiring 示例

### 18.1 Hertz server

Hertz 服务建议在 `internal/base/server/server.go` 初始化 logger，然后把默认 `middleware.Recovery()` / `middleware.RequestID()` / `middleware.AccessLog()` 替换为 logging optional 的 middleware：

```go
import "<module>/internal/base/logging"

_, err := logging.Init(logging.DefaultConfig(), logging.ReleaseInfo{
    ServiceName: cfg.Server.Name,
    ServiceKind: "hertz",
    Version:     cfg.Server.Version,
})
if err != nil {
    panic(err)
}

h.Use(logging.HertzRecovery())
h.Use(logging.HertzRequestID())
h.Use(logging.HertzAccessLog())
```

注意不要同时保留默认 `middleware.AccessLog()`，否则 access log 会重复输出。

### 18.2 Kitex server

Kitex 服务建议在 `internal/base/server/server.go` 初始化 logger，然后把默认 `interceptor.RequestID()` / `interceptor.AccessLog()` / `interceptor.Recovery()` 替换为 logging optional 的 interceptor：

```go
import "<module>/internal/base/logging"

_, err := logging.Init(logging.DefaultConfig(), logging.ReleaseInfo{
    ServiceName: cfg.Server.Name,
    ServiceKind: "kitex",
})
if err != nil {
    panic(err)
}

kitexserver.WithMiddleware(endpoint.Chain(
    logging.KitexRequestID(),
    logging.KitexAccessLog(),
    logging.KitexRecovery(),
))
```

### 18.3 Kitex client wrapper

如果希望调用端也记录下游 RPC 日志，可以在 `pkg/client/<service>/client.go` 的 client options 中追加 client-side middleware：

```go
import "<module>/internal/base/logging"

options = append(options, kitexclient.WithMiddleware(endpoint.Chain(
    logging.KitexRequestID(),
    logging.KitexAccessLog(),
)))
```

这一步不是必须；如果 server-side access log 已经覆盖需求，可以只接 Kitex server。

## 19. 分阶段实现

### Phase 1：基础 optional

- 已完成：新增 `observability_logging` / `logging` kind。
- 已完成：生成 logging package。
- 已完成：支持 console / file / both / none。
- 已完成：支持 rotate + compress。
- 已完成：支持 category routing。

### Phase 2：oops 结构化

- 已完成：解析 `oops`。
- 已完成：输出 `error.code` / `error.public` / `error.tags` / `error.scope` / `error.attrs`。
- 待增强：根据 tag/code 自动分类。

### Phase 3：链路与发布上下文

- 已完成：输出 `request_id` / `trace_id` / `span_id`。
- 已完成：输出 release metadata。
- 已完成：输出 `traffic.lane`。

### Phase 4：Hertz / Kitex 集成

- 已完成：Hertz middleware。
- 已完成：Kitex interceptor。
- 已完成：recovery 按 `panic` category 路由。
- 已完成：在默认 Hertz / Kitex 模板中预留安全 wiring 注释，提示如何替换当前 `hlog` / `klog` access log。

### Phase 5：采集示例

- Fluent Bit / Vector 示例。
- Loki / Elasticsearch / SLS 字段映射。
- 金丝雀查询模板。

## 20. 推荐结论

建议最终形态：

```text
observability_logging optional
  |-- slog facade
  |-- console writer
  |-- file writer
  |-- lumberjack rotate + gzip compress
  |-- category routing
  |-- oops structured extractor
  |-- request_id / trace_id / span_id
  |-- release.track / service.version / traffic.lane
  |-- Hertz middleware
  |-- Kitex interceptor
```

默认策略：

```text
本地开发：console text
容器生产：console json
VM 生产：both json + compressed files
```

最终目标：

```text
业务用 oops 表达错误
日志层统一分类、结构化、落盘、压缩、链路关联
LoongSuite 负责 trace/metrics
logging 负责 trace_id/span_id 写入日志
```