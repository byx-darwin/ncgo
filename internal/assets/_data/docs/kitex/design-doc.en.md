# Kitex Template Design Doc

Audience: ncgo maintainers and AI agents that read or modify the embedded
Kitex template tree under `internal/assets/_data/kitex/`. This document
describes what each file does, the contracts it exposes to the
scaffolder, and how to evolve it without breaking generated projects.

For the Hertz counterpart see
[`docs/hertz/design-doc.en.md`](../hertz/design-doc.en.md).

## 1. Overview

The Kitex template family backs `ncgo new --mode mono --kind kitex` (RPC
services). It is consumed by `internal/scaffold/mono` (kitex branch),
copied into generated projects under `template/kitex-template/`, and
rendered by `kitex` ≥ v0.16.1 via the `--template-extension` mechanism.

Files ship inside the ncgo binary via `//go:embed all:_data` (see
`internal/assets/assets.go`). The directory name `_data/` (leading
underscore) makes `go build ./...` ignore the `optional/*.go` files —
they are template snippets, not Go source compiled into ncgo itself.

Asset version: `_data/VERSION` (`ncgo_assets_version: 0.1.1`), surfaced
via `assets.Version()`.

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
│       └── rpcerror/                   # oops → kitex BizStatusError mapping
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
  `oops.Code(10308) Public("config_invalid")`.
- `Init()` is called once from `main.go` (`sync.Once`); `Get()` returns
  the cached `*Config`.
- `Validate()` enforces non-empty `server.name` / `server.addr`,
  non-negative timeouts, the `caller_allowlist` invariants
  (`header` set; `allowed_callers` non-empty unless `allow_missing`),
  and database pool settings non-negative when `Database.Enabled`.

### 3.2 RPC Error Mapping (`internal/pkg/rpcerror`)

- `ToBizError(err)` is the single conversion point from `samber/oops`
  errors to `kerrors.BizStatusError`. If the error is already a
  `BizStatusError` it passes through unchanged.
- Reserved scaffold codes:
  - `10000 internal_error` — fallback for non-oops / panic.
  - `10010 not_implemented` — usecase stubs.
  - `10108 permission_denied` — caller allowlist rejects.
  - `10302 rpc_timeout` — request deadline exceeded.
  - `10308 config_invalid` — config load / validate failure.
- Producers raise errors as
  `oops.In("...").Code(code).Public("msg")…`. `codeFromOops` accepts
  `int`, `int32`, `int64` codes and falls back to `CodeInternalError`
  outside `int32` range.
- Helpers: `InternalErrorf`, `TimeoutError`, `PermissionDenied`,
  `BizCode(err)`, `FormatBiz(err)` (used by `AccessLog`).
- Code reservations align with the Hertz `pkg/response` registry:
  scaffold owns `10000–10399`, business codes start at `10400+` (see
  Hertz §3.2 for the registration pattern).

### 3.3 Server-Side Interceptors (`internal/pkg/interceptor`)

| Interceptor | Behaviour | Failure / Output |
|---|---|---|
| `RequestID` | Reads `x-request-id` from metainfo; if absent generates 16-byte hex and writes `WithPersistentValue` | n/a |
| `AccessLog` | Wraps `next`, logs service / method / latency / request_id; warns on error with `rpcerror.FormatBiz(err)` | n/a |
| `Recovery` | `defer recover()`; converts panic to `rpcerror.InternalErrorf` then `ToBizError` | `10000 internal_error` |
| `RequestTimeout(d)` | `context.WithTimeout(ctx, d)`; on `DeadlineExceeded` with no error, returns `TimeoutError` via `ToBizError` | `10302 rpc_timeout` |
| `CallerAllowlist(enabled, header, allowed, allowMissing)` | Checks metainfo header (default `x-caller-service`) against allowlist | `10108 permission_denied` |

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
  with `oops.Code("postgres_pool_open_failed" | "postgres_ping_failed")`.
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
- `Validate()` raises `oops.Code(10308) Public("config_invalid")` for
  bad timeouts, bad backoff config, or missing service name.
- Errors during `NewClient` are wrapped as
  `oops.Code(10301) Public("rpc_failed")`.

### 3.6 Operations

- `Makefile` targets: `build`, `run`, `dev` (air or `go run .`),
  `update` (re-runs `kitex -template-dir template/kitex-template`),
  `sqlc`, `generate` (= `update` + `sqlc`), `migrate-{up,down,status,create}`,
  `lint`, `test`, `check`, `tidy`, `install-tools`, `clean`.
- `cmd` entry is `main.go`: `conf.Init()` → `server.Run()`. Any wiring
  the agent adds belongs in `internal/base/server/server.go`.
- Health / readiness probes are not built in (kitex services typically
  expose a sidecar or rely on TTHeader liveness); add them inside
  `server.Run` if the platform requires HTTP probes.

### 3.7 Optional Infra Snippets

`ncgo add infra <kind>` (or manual copy from
`internal/assets/_data/kitex/optional/<kind>.go`) drops a Go file under
`internal/base/{data,registry,observability}/` depending on the kind.
Each file ships only the typed constructor; the agent registers the
config struct and the constructor into the `samber/do` injector inside
`server.Run` (`registry` / `observability` add-ons are wired directly as
kitex server options, not through `do`). Kitex add-ons use string
`oops.Code` values (`<kind>_<reason>`), unlike Hertz which uses the
numeric errcode registry.

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

#### Etcd Registry / Discovery (`registry/etcd.go`, kitex-only)

- Provides: `kitexregistry.Registry` via `NewEtcdRegistry(cfg)`,
  `discovery.Resolver` via `NewEtcdResolver(cfg)`, and
  `*kitexregistry.Info` via `NewRegistryInfo(cfg)`.
- Config struct `EtcdConfig{ Endpoints, Username, Password,
  DialTimeoutSeconds, ServicePrefix, RegistryRetry{Enabled,
  MaxAttemptTimes, ObserveDelaySeconds, RetryDelaySeconds} }`.
- Dep: `github.com/kitex-contrib/registry-etcd`.
- Failures: `registry_config_invalid` (empty endpoints, negative
  durations, malformed `public_addr`).
- Wiring at bootstrap (in `server.Run`):
  ```go
  r, err := registry.NewEtcdRegistry(cfg.Registry)
  if err != nil { return oops.Wrapf(err, "etcd registry") }
  // pass kitexserver.WithRegistry(r) into kitex server options
  ```

#### LoongSuite Go Agent observability (`observability/otel.go`, common)

- Provides: `LoongSuiteConfig`, `DefaultLoongSuiteConfig(serviceName)`, and
  `LoongSuiteConfig.Env()` helpers for standard `OTEL_*` environment variables.
- No SDK dependency is added to the generated service. LoongSuite instruments at
  compile time via the external `otel` CLI.
- Build and run:
  ```bash
  otel go build ./...
  OTEL_SERVICE_NAME=user-rpc OTEL_EXPORTER_OTLP_ENDPOINT=http://127.0.0.1:4318 ./<your-binary>
  ```

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
| `kitex/kitex-template/makefile.yaml` | Makefile targets (`make dev`, `make sqlc-gen`, ...) |
| `kitex/sqlc.yaml` | sqlc config, structurally identical to the Hertz version |
| `kitex/optional/{redis,kafka,es,clickhouse,registry_etcd}.go`, `optional/observability_otel.go` | `add infra` snippets for the kitex family |

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
`internal/base/registry/` or `internal/base/observability/`.

Constraints for new optional files:

- Must not import project-specific packages.
- Package must match the target package (`data`, `registry`,
  `observability`, etc.).
- Top-of-file comment must list required dependencies and wiring notes.

Currently shipped: `redis`, `kafka`, `es`, `clickhouse`,
`observability_otel` (`otel` alias), and Kitex-only `registry_etcd`.

## 7. Differences from Hertz

| Aspect | Hertz | Kitex |
|---|---|---|
| Variable name for module | `{{.GoModule}}` | `{{.Module}}` |
| Layout container | One `layout.yaml` listing every file | One YAML file per output path |
| Handler template | `--customize_package` (`package.yaml`) | Per-path template `handler.yaml` |
| Variables source | `data.json` (separate) | Inline kitex render context |
| Optional infra | 5 kinds (adds `observability_otel`) | 6 kinds (adds `registry_etcd`) |

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
