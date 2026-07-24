# Kitex Template Design Doc

Audience: ncgo maintainers and AI agents that read or modify the embedded
Kitex template tree under `internal/assets/_data/kitex/`. This document
describes what each file does, the contracts it exposes to the
scaffolder, and how to evolve it without breaking generated projects.

For the Hertz counterpart see
[`docs/hertz/design-doc.en.md`](../hertz/design-doc.en.md).

The dedicated dynamic rate-limit topic lives in
[`docs/hertz/rate-limit-dynamic-design.en.md`](../hertz/rate-limit-dynamic-design.en.md),
but it is specific to the Hertz HTTP template and does not directly apply to the
Kitex RPC template.

## 1. Overview

The Kitex template family backs `ncgo new --mode mono --kind kitex` (RPC
services). It is consumed by `internal/scaffold/mono` (kitex branch),
copied into generated projects under `template/kitex-template/`, and
rendered by `kitex` via the `--template-extension` mechanism.
The minimum supported `kitex` version is defined in `internal/exec/exec.go`.

Files ship inside the ncgo binary via `//go:embed all:_data` (see
`internal/assets/assets.go`). The directory name `_data/` (leading
underscore) makes `go build ./...` ignore the `optional/*.go` files —
they are template snippets, not Go source compiled into ncgo itself.

Generated projects build on **go-tools v0.1.0** (a thin business layer on top
of it): their `go.mod` declares `go 1.26.5` and requires
`go-common v0.1.0` + `go-framework v0.1.0` (`go-middleware v0.1.0` is added by
`go mod tidy` when the project uses a database). Config uses
`go-framework/config` (+ `config/kitex`), logging uses `go-common/log`, RPC
error mapping uses `go-framework/kitex/rpcerror`, and framework codes come from
`go-framework/error`.

Asset version: see `_data/VERSION`; the current embedded asset version is
surfaced via `assets.Version()`.

## 2. Generated Project Architecture

### 2.1 Directory Tree

After `ncgo new --mode mono --kind kitex` plus the embedded `kitex`
invocation, the project looks like this:

```
<project>/
├── main.go                             # conf.Init() → server.Run()
├── conf/dev/conf.yaml                  # GO_ENV-driven YAML config
├── idl/<service>.proto                 # Protobuf IDL
├── template/kitex-template/            # YAML templates (kept for `make update`)
├── internal/
│   ├── base/
│   │   ├── conf/                       # config types, Init/Load/Default/Validate
│   │   ├── data/                       # pgxpool + sqlc Queries (+ optional clients)
│   │   └── server/                     # samber/do wiring + kitex server bootstrap
│   ├── handler/<service>/              # thin RPC shell delegating to UseCase
│   ├── usecase/<service>/              # business logic; declares `<Service>Repo` port
│   ├── repository/<service>/           # sqlc-backed Repo + WithTx helper
│   ├── pb/                             # kitex-generated protobuf types
│   ├── db/{schema,query,migrations,gen}/
│   └── pkg/
│       ├── interceptor/                # RequestID, AccessLog, Recovery, RequestTimeout, CallerAllowlist
│       └── rpcerror/                   # goerror → kitex BizStatusError mapping
├── pkg/client/<service>/               # caller-side client factory + Retry/Circuit-Breaker config
├── kitex_gen/                          # kitex-generated server stubs (do not edit)
├── Makefile                            # build / dev / update / sqlc / migrate / lint / test
├── go.mod
└── .gitignore
```

### 2.2 Layer Boundaries

Allowed import direction (top-down only):

```
main          → base/server
base/server   → base/conf, base/data, handler/*, usecase/*, repository/*,
                pkg/interceptor, pkg/rpcerror
handler/*     → usecase/*, pkg/rpcerror, kitex_gen        (no repo / data import)
usecase/*     → repository (port interface declared here), pb
repository/*  → base/data, db/gen, pgx                    (no usecase import)
pkg/client/*  → kitex_gen, pkg/interceptor                (consumed by adapters)
```

Rules surfaced by `ncgo doctor`:

- Handlers MUST NOT import `internal/repository/*` or `internal/base/data`.
- Usecases MUST NOT import `github.com/cloudwego/kitex/...`.
- Repository implementations MUST NOT import `internal/usecase/*`; the
  port interface (`<Service>Repo`) is declared inside the usecase package.
- All errors crossing the RPC boundary go through
  `rpcerror.ToBizError(err)` so callers receive a `BizStatusError`
  carrying a 5-digit code.

### 2.3 Dependency Injection (`samber/do`)

`internal/base/server/server.go` builds an injector and wires the chain
inline (no separate provider list). Skeleton from the shipped template:

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

`provideRepository` calls `data.NewPostgresConfig(dsn)` →
`applyPostgresPoolConfig(...)` → `data.NewPostgres(ctx, cfg)` →
`data.New(pool)` → `repository.New(d.Queries, d.Pool)`. The cleanup
closure closes the pool on shutdown.

### 2.4 Request Lifecycle

```
RPC request (TTHeader)
  → kitex server (NewServer with WithReadWriteTimeout / WithExitWaitTime)
  → MetaHandler (transmeta.ServerTTHeaderHandler)
  → endpoint.Chain:
       RequestID()                   → ensures x-request-id metainfo
       AccessLog()                   → klog.CtxInfof / CtxWarnf with biz code
       Recovery()                    → recover → rpcerror.InternalErrorf
       CallerAllowlist(...)          → checks x-caller-service vs allowlist
       RequestTimeout(d)             → context.WithTimeout; deadline → TimeoutError
  → Handler.<Method>(ctx, req):
       resp, err := s.uc.<Method>(ctx, req)
       err != nil                   → return rpcerror.ToBizError(err)
       success                      → return resp, nil
  → ErrorHandler                    → rpcerror.ToBizError(err)
```

The chain order is hardcoded in `server.Run`. The `WithErrorHandler`
hook re-wraps any error that bypassed the chain so the wire response is
always a `BizStatusError`.

## 3. Built-in Features

### 3.1 Configuration (`internal/base/conf`)

- Env resolution: `GO_ENV` (defaults to `dev`).
- File resolution: `CONFIG_PATH` env var, otherwise `conf/<env>/conf.yaml`.
- If the resolved file is missing **and** `CONFIG_PATH` was not set, the
  defaults from `Default()` are used; otherwise loading fails with
  `goerror.In("config").Code(frameworkerror.CodeConfigInvalid).Public("config_invalid")`
  (`frameworkerror.CodeConfigInvalid` = 10004).
- `Init()` is called once from `main.go` (`sync.Once`); `Get()` returns
  the cached `*Config`.
- `Validate()` enforces non-empty `server.name` / `server.addr`,
  non-negative timeouts, the `caller_allowlist` invariants
  (`header` set; `allowed_callers` non-empty unless `allow_missing`),
  and database pool settings non-negative when `Database.Enabled`.
- Duration-typed fields on `Config` (for example `rpc.request_timeout_seconds`,
  `database.health_check_period_seconds`, `rate_limit.rule.window_seconds`)
  use `config.Duration` from `go-framework/config`. In `conf/dev/conf.yaml`
  these keys are written as duration strings such as `"3s"` or `"30s"`
  (parsed by `time.ParseDuration`); bare integers are no longer accepted.

### 3.2 RPC Error Mapping (`internal/pkg/rpcerror`)

- `ToBizError(err)` is the single conversion point from `go-common/error`
  (`goerror`) errors to `kerrors.BizStatusError`, delegated to
  `go-framework/kitex/rpcerror.OopsStatusAdapter`. If the error is already a
  `BizStatusError` it passes through unchanged.
- Reserved scaffold codes (constants re-export `go-framework/error`):
  - `CodeInternalError` = `frameworkerror.CodeSystem` (10000) — fallback for
    non-goerror / panic.
  - `CodeNotImplemented` = 10010 — usecase stubs (placeholder; it intentionally
    shares its value with `frameworkerror.CodeRPCUnavailable`).
  - `CodePermissionDenied` = `frameworkerror.CodeAuthFailed` (10002) — caller
    allowlist rejects (go-tools v0.1.0 has no `CodePermissionDenied`; it maps to
    `CodeAuthFailed`).
  - `CodeRPCTimeout` = `frameworkerror.CodeRPCTimeout` (10011) — request
    deadline exceeded.
  - `CodeConfigInvalid` = `frameworkerror.CodeConfigInvalid` (10004) — config
    load / validate failure.
- Producers raise errors as `goerror.In("...").Code(code).Public("msg")…`
  (`goerror` = `go-common/error`, which wraps `samber/oops`; do not construct
  errors with `samber/oops` directly).
- Helpers: `InternalErrorf`, `TimeoutError`, `PermissionDenied`,
  `BizCode(err)`, `FormatBiz(err)` (used by `AccessLog`).
- Code segments follow go-tools: Framework `10000–10499`, Middleware
  `20000–20699`, Auth `40000–40099`, Project `40100–59999`. **Business-defined
  codes must be `>= 40100` (`goerror.ProjectCodeMin`).** Business codes fall
  back to **HTTP 200** via `goerror.HTTPStatus` ("business error, RPC call
  succeeded"); register fine-grained HTTP statuses with
  `goerror.RegisterHTTPStatuses` when needed.

### 3.3 Server-Side Interceptors (`internal/pkg/interceptor`)

| Interceptor | Behaviour | Failure / Output |
|---|---|---|
| `RequestID` | Reads `x-request-id` from metainfo; if absent generates 16-byte hex and writes `WithPersistentValue` | n/a |
| `AccessLog` | Wraps `next`, logs service / method / latency / request_id; warns on error with `rpcerror.FormatBiz(err)` | n/a |
| `Recovery` | `defer recover()`; converts panic to `rpcerror.InternalErrorf` then `ToBizError` | `CodeInternalError` (10000) |
| `RequestTimeout(d)` | `context.WithTimeout(ctx, d)`; on `DeadlineExceeded` with no error, returns `TimeoutError` via `ToBizError` | `CodeRPCTimeout` (10011) |
| `CallerAllowlist(enabled, header, allowed, allowMissing)` | Checks metainfo header (default `x-caller-service`) against allowlist | `CodePermissionDenied` (= `frameworkerror.CodeAuthFailed`, 10002) |

The chain is composed via `endpoint.Chain(...)` inside
`server.Run` and registered with `kitexserver.WithMiddleware`. The
default header for the caller allowlist is `x-caller-service`
(constant `HeaderCallerService`).

### 3.4 Database (`cfg.Database.Enabled`)

- `internal/base/data/data.go` exposes
  `Data{ Pool *pgxpool.Pool, Queries *gen.Queries }` and a `cleanup`
  closure that closes the pool.
- `data.NewPostgresConfig(dsn)` parses DSN; `applyPostgresPoolConfig`
  copies `MaxConns / MinConns / MaxConnLifetime / MaxConnIdleTime /
  HealthCheckPeriod` from `cfg.Database` onto the `*pgxpool.Config`.
- `data.NewPostgres(ctx, cfg)` opens + pings the pool. Errors come back
  with `goerror.Code("postgres_pool_open_failed" | "postgres_ping_failed")`.
- `internal/db/{schema,query,gen,migrations}` is identical in shape to
  the Hertz layout (same `sqlc.yaml`, same goose migration directory).
- Repositories own transactions via `WithTx(ctx, fn)`; the helper
  rolls back on error or panic and only commits on success.

### 3.5 Caller-Side Client (`pkg/client/<service>`)

`client.yaml` ships a typed wrapper used by other services / Hertz
gateways to call this RPC:

- `Config` covers `ServiceName`, `CallerService`, `HostPorts`, RPC and
  connect timeouts, `EnableMetaInfo` (TTHeader), and a `RetryConfig`
  block (`Backoff` ∈ `none|fixed|random`, circuit breaker error rate
  capped at 0.3, `MaxRetryTimes` ∈ [1, 5]).
- `New(ctx, cfg, opts...)` builds the kitex client, optionally adds
  `WithFailureRetry` from `failurePolicy()` and a caller-service
  middleware that injects `x-caller-service` into outgoing metainfo.
- `Validate()` raises
  `goerror.In("kitex.client").Code(frameworkerror.CodeConfigInvalid).Public("config_invalid")`
  for bad timeouts, bad backoff config, or missing service name.
- Errors during `NewClient` are wrapped as
  `goerror.In("kitex.client").Code(frameworkerror.CodeRPCUnavailable).Public("rpc_failed")`
  (`frameworkerror.CodeRPCUnavailable` = 10010).

### 3.6 Operations

- `Makefile` targets: `build`, `run`, `dev` (air or `go run .`),
  `update` (re-runs `kitex -template-dir template/kitex-template`),
  `sqlc`, `generate` (= `update` + `sqlc`), `migrate-{up,down,status,create}`,
  `lint`, `test`, `check`, `tidy`, `install-tools`, `clean`.
- Even in the default starter path, `internal/base/data` and repository wiring
  import `internal/db/gen`, so run `make sqlc` before the first `go mod tidy`
  or build; `make generate` already includes that step.
- `cmd` entry is `main.go`: `conf.Init()` → `server.Run()`. Any wiring
  the agent adds belongs in `internal/base/server/server.go`.
- Health / readiness probes are not built in (kitex services typically
  expose a sidecar or rely on TTHeader liveness); add them inside
  `server.Run` if the platform requires HTTP probes.

### 3.7 Optional Infra Snippets

`ncgo add infra <kind>` (or manual copy from
`internal/assets/_data/kitex/optional/<kind>.go`) drops a Go file under
`internal/base/{data,registry}/` depending on the kind.
Each file ships only the typed constructor; the agent registers the
config struct and the constructor into the `samber/do` injector inside
`server.Run` (`registry_polaris` is wired directly as a kitex server /
client option, not through `do`). Kitex add-ons use string
`goerror.Code` values (`<kind>_<reason>`), unlike Hertz which uses the
numeric errcode registry. Observability is provided by the kitex base
template itself (go-framework OTLP, driven by `cfg.Jaeger`) and no
longer exists as a standalone add-on.

#### Redis (`data/redis.go`)

- Same structure as the Hertz copy.
- Failures: `redis_config_missing` (nil opts), `redis_ping_failed`
  (`Ping` failed).
- Wiring:
  ```go
  do.ProvideValue[context.Context](inj, startupCtx)
  do.ProvideValue(inj, &redis.UniversalOptions{Addrs: cfg.Redis.Addrs})
  do.Provide(inj, data.NewRedis)
  ```

#### Kafka (`data/kafka.go`)

- Failures: `kafka_writer_missing` / `kafka_writer_addr_missing` /
  `kafka_writer_topic_missing` (producer);
  `kafka_reader_brokers_missing` / `kafka_reader_topic_missing`
  (consumer).
- Wiring identical to the Hertz copy; the default consumer `GroupID` in
  the example follows the RPC service name (e.g. `user-rpc`).

#### Elasticsearch (`data/es.go`)

- Failures: `elasticsearch_addresses_missing`,
  `elasticsearch_client_create_failed`, `elasticsearch_ping_failed`.
- Wiring identical to the Hertz copy.

#### ClickHouse (`data/clickhouse.go`)

- Carries `clickhouse.ClientInfo.Products` (sets the service name on the
  ClickHouse side); the Hertz copy does not.
- Failures: `clickhouse_config_missing`, `clickhouse_addresses_missing`,
  `clickhouse_open_failed`, `clickhouse_ping_failed`.

#### Polaris Registry / Discovery (`registry/polaris.go`, kitex-only)

- Provides: `NewRegistry(cfg)` returning `kitexregistry.Registry` and
  `NewResolver(cfg)` returning `discovery.Resolver`, delegating to
  `kitex-contrib/polaris`'s `NewPolarisRegistry` / `NewPolarisResolver`.
- Config struct: `PolarisConfig{ Addresses, Namespace, Protocol,
  TimeoutSeconds, ... }`, sourced from either `polaris.yaml` (project root)
  or `conf`; `Validate()` uses goerror.
- Dep: `github.com/kitex-contrib/polaris`; the add-on also drops
  `polaris.yaml` at the project root, which `kitex-contrib/polaris` reads
  from the working directory by default.
- Failure code: `registry_config_invalid` (empty addresses, illegal
  timeout, `polaris.yaml` parse failure).
- Wiring (only applied after `ncgo add infra registry_polaris --wire`,
  via the `// ncgo:wire:registry:server` /
  `// ncgo:wire:registry:client` anchors):
  ```go
  r, err := registry.NewRegistry(cfg.Registry)
  if err != nil { return goerror.In("kitex.registry").Wrap(err) }
  // server: kitexserver.WithRegistry(r)
  // client: kitexclient.WithResolver(registry.NewResolver(cfg.Registry))
  ```
- `--wire` inserts `WithRegistry` / `WithResolver` into the kitex base
  server option and client constructor. Without `--wire`, only
  `polaris.go` and `polaris.yaml` are emitted and the base server/client
  are untouched.

#### Observability (go-framework OTLP, built into the kitex base)

The kitex base template already wires go-framework OTLP; no extra
`ncgo add infra` add-on is needed. When `cfg.Jaeger != nil && cfg.Jaeger.Enable`,
`server.go` calls `kitexobs "github.com/byx-darwin/go-tools/go-framework/kitex/observability"`'s
`kitexobs.NewProvider(ctx, config.ObservabilityConfig{Enabled, Endpoint,
ServiceName})`, attaches `provider.ServerSuite()` as a kitex server option,
and `defer provider.Shutdown()` before `server.Run` exits.

> Historical note: the LoongSuite `observability_otel` / `otel` add-on and
> the `kitex-contrib/registry-etcd` add-on were removed in PR5.
> Observability is now unified on go-framework OTLP; registry / discovery
> is unified on Polaris. The legacy `otel` / `registry_etcd` kinds now
> return invalid kind.

## 4. Files

| File | Purpose |
|---|---|
| `kitex/kitex-template/main.yaml` | `main.go` entry; calls `conf.Init()` then `server.Run()` |
| `kitex/kitex-template/server.yaml` | `internal/base/server/server.go`: kitex server setup + samber/do wiring |
| `kitex/kitex-template/handler.yaml` | Handler shell that delegates to `usecase.UseCase` |
| `kitex/kitex-template/usecase.yaml` | Usecase stub |
| `kitex/kitex-template/repository.yaml` | Repository stub |
| `kitex/kitex-template/conf.yaml` / `conf_dev.yaml` | base/conf package + `conf/dev/conf.yaml` |
| `kitex/kitex-template/data.yaml` | base/data injector bootstrap |
| `kitex/kitex-template/interceptor.yaml` (+ `_test`) | Server-side interceptor scaffolding |
| `kitex/kitex-template/rpcerror.yaml` (+ `_test`) | RPC error mapping (analogous to hertz `pkg/response`) |
| `kitex/kitex-template/client.yaml` (+ `_test`) | Generated client wrapper |
| `kitex/kitex-template/migration_init.yaml` / `migration_keep.yaml` | sqlc/atlas migration placeholders |
| `kitex/kitex-template/makefile.yaml` | Makefile targets (`make dev`, `make sqlc`, ...) |
| `kitex/sqlc.yaml` | sqlc config, structurally identical to the Hertz version |
| `kitex/optional/{redis,kafka,es,clickhouse,registry_polaris}.go` | `add infra` snippets for the kitex family (`registry_polaris` also drops `polaris.yaml` at the project root) |

## 5. `kitex-template/*.yaml` Semantics

Each YAML file is a single record with this shape:

```yaml
path: <output path relative to project root>
update_behavior:
  type: cover   # cover | skip
body: |-
  <Go template body>
```

The kitex tool reads these via `--template-extension` and renders each
record to its `path`.

Render context (verified against the shipped templates):

| Variable | Meaning | Example |
|---|---|---|
| `{{.Module}}` | Go module path | `example.com/demo` |
| `{{.ServiceInfo.ServiceName}}` | Service name from IDL | `Demo` |
| `{{.ServiceInfo.ImportPath}}` | Generated kitex client import path | `example.com/demo/kitex_gen/...` |
| `{{ToLower x}}` | Lowercase helper | `demo` |

`update_behavior.type`:

- `cover` — kitex overwrites the file on regeneration. Use for files the
  user must not hand-edit (`main.go`, generated handler/usecase shells).
- `skip` — kitex leaves an existing file alone. Use for files the user
  is expected to edit (e.g., business logic in `usecase.yaml` after
  initial scaffold).

## 6. Optional Infra

Each `optional/*.go` file is byte-verbatim copy material. `infra.Add`
reads it from the embedded FS and writes it to its target path, usually
`internal/base/data/<kind>.go`, or a specialized package such as
`internal/base/registry/`.

Constraints for new optional files:

- Must not import project-specific packages.
- Package must match the target package (`data`, `registry`, etc.).
- Top-of-file comment must list required dependencies and wiring notes.

Currently shipped: `redis`, `kafka`, `es`, `clickhouse`, and Kitex-only
`registry_polaris`. Observability is provided by the kitex base template
(go-framework OTLP) and no longer ships as a standalone add-on.

## 7. Differences from Hertz

| Aspect | Hertz | Kitex |
|---|---|---|
| Variable name for module | `{{.GoModule}}` | `{{.Module}}` |
| Layout container | One `layout.yaml` listing every file | One YAML file per output path |
| Handler template | `--customize_package` (`package.yaml`) | Per-path template `handler.yaml` |
| Variables source | `data.json` (separate) | Inline kitex render context |
| Optional infra | 4 kinds (data add-ons; observability provided by base template) | 5 kinds (adds `registry_polaris`) |

## 8. Maintenance Contract

Any change under `_data/kitex/` must:

1. Update the file content.
2. Update the relevant section of this doc (and `design-doc.zh-CN.md`).
3. Bump `_data/VERSION` `ncgo_assets_version`.
4. Re-run golden tests:
   `go test ./internal/scaffold/mono/... -update-golden -count=1`, then
   `go test ./internal/scaffold/mono/... -count=1`.
5. Add the new path to `internal/assets/assets_test.go` if it is a
   structural addition.

The `Rule:` anchors emitted by `ncgo doctor` still point to
`nc-skills-golang/SKILL.md#layer-rules`. Templates are owned by ncgo;
review-mode rules stay in nc-skills-golang.

## 9. References

- `docs/prd.md` §3 (Decisions), §5 (Manifest), §9 (Repository Layout),
  §10 (v0.2 milestone)
- `internal/assets/assets.go` — embed wiring
- `internal/scaffold/infra/infra.go` — optional consumer
- `nc-skills-golang/SKILL.md` — review-mode rules and AI invocation guides
- `docs/hertz/design-doc.en.md` — Hertz counterpart
