# ncgo

[![CI](https://github.com/byx-darwin/ncgo/actions/workflows/ci.yml/badge.svg)](https://github.com/byx-darwin/ncgo/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/byx-darwin/ncgo)](https://github.com/byx-darwin/ncgo/releases)
[![Go 1.25+](https://img.shields.io/badge/Go-1.25%2B-00ADD8?logo=go)](https://go.dev/)
[![License](https://img.shields.io/github/license/byx-darwin/ncgo)](LICENSE)

**Website:** [byx-darwin.github.io/ncgo](https://byx-darwin.github.io/ncgo/)

AI-friendly scaffold CLI for Go microservices. `ncgo` owns embedded Hertz and
Kitex templates, writes a project manifest, invokes upstream generators, and
renders AI context files for Claude/Cursor/other agents.

Use it when you want reproducible Go service scaffolds, optional infra add-ons,
and agent-friendly project context from a single CLI.

中文文档见 [README.zh-CN.md](README.zh-CN.md)。Product requirements live in
[specs/prd.md](specs/prd.md) and [specs/prd.zh-CN.md](specs/prd.zh-CN.md). For
agent handoff, see [specs/005-context-handoff.zh-CN.md](specs/005-context-handoff.zh-CN.md).

**Quick links:** [Install](#install) · [30-Second Tour](#30-second-tour) · [Typical Workflows](#typical-workflows) · [Examples](docs/examples.md) · [Contributing](CONTRIBUTING.md) · [FAQ](#faq)

## Why ncgo

- **Deterministic scaffolding**: keep manifests, IDL placeholders, and templates under version control
- **Generator-aware**: orchestrate `hz` / `kitex`, but still allow `--no-generate` workflows
- **Agent-friendly by default**: render `AGENTS.md`, `CLAUDE.md`, `.claude/generated/project-context.md`, Cursor rules, and expose MCP tools
- **Lifecycle helpers included**: ship `doctor`, `upgrade`, and conservative `extract domain` flows in the same CLI

## Highlights

| Area | What you get |
| --- | --- |
| Service scaffolding | Mono Hertz/Kitex scaffolds plus micro workspaces with `add rpc` / `add bff` |
| Project context | Versioned manifests, template inputs, and AI collaboration files |
| Optional infra | Redis, Kafka, Elasticsearch, ClickHouse, logging, canary, and more |
| Lifecycle tooling | Built-in `doctor`, `upgrade`, `extract domain`, and MCP exposure |

## Best fit

Use `ncgo` when you:

- build Go services around Hertz or Kitex
- want reproducible scaffolds instead of one-off generator output
- want AI agents to understand and operate on generated projects more reliably

`ncgo` is probably not the right tool if you:

- need a framework-agnostic or non-Go project generator
- do not want any Hertz / Kitex generator dependency in your workflow
- expect the CLI to install dependencies or refactor arbitrary existing services automatically

## Current Status

The v0.5 MVP is complete:

- Mono scaffolds: Hertz HTTP service and Kitex RPC service.
- Micro workspace: root `ncgo.workspace` plus `add rpc` / `add bff` services.
- Domain workflow: `add domain` and anchor-based `add method`.
- Optional infra: Redis, Kafka, Elasticsearch, ClickHouse, structured logging, canary release helpers, and Kitex-only Polaris registry.
- AI/agent workflow: `ai init claude`, `ai sync`, static `doctor`, and MCP stdio server.
- Lifecycle MVPs: metadata-only `upgrade --plan` and conservative `extract domain --apply`.

Deferred optionals remain documented but intentionally not implemented yet:
~~NATS~~, ~~Mongo~~, and ~~MinIO~~ (formally removed per P0-5 decision — Kafka, Postgres, and ClickHouse cover equivalent use-cases).

## What ncgo Generates

| Area | Output |
| --- | --- |
| Project metadata | `.ncgo/manifest.yaml` for services; `ncgo.workspace` for micro roots |
| Hertz | IDL placeholder, custom `hz` layout/package inputs, HTTP service skeleton |
| Kitex | IDL placeholder, custom Kitex template tree, RPC service skeleton |
| Containerization | Service-level `Dockerfile` / `.dockerignore`; `compose.yaml` for mono and micro roots |
| Domain | `internal/usecase/<name>`, `internal/repository/<name>`, DI register file |
| Infra | Optional drop-in Go files under `internal/base/...` |
| AI context | `AGENTS.md`, `CLAUDE.md`, `.claude/skills/ncgo-dev/SKILL.md`, `.claude/generated/project-context.md`, `.cursor/rules/ncgo.mdc` |

### What generated projects build on (go-tools v0.1.0)

Generated Hertz/Kitex projects are a thin business layer on top of
[go-tools](https://github.com/byx-darwin/go-tools) v0.1.0. The generated
`go.mod` declares `go 1.26.5` and requires `go-common v0.1.0` +
`go-framework v0.1.0` (`go-middleware v0.1.0` is added by `go mod tidy` when
`WithDatabase=true`).

| Concern | go-tools module |
| --- | --- |
| HTTP responses | `go-framework/hertz` `Responder` (`RespondFrom(c).Success` / `.Error`) |
| Configuration | `go-framework/config` (+ `config/hertz`, `config/kitex`) |
| Logging | `go-common/log` |
| Error codes | re-exports the framework codes from `go-framework/error` |
| Database | `go-middleware/db` (when `WithDatabase=true`) |
| Kitex RPC errors | `go-framework/kitex/rpcerror` |

**Configuration duration fields.** Duration-typed fields in the generated
project's `conf.Config` (for example `rpc.request_timeout_seconds`,
`database.health_check_period_seconds`, `rate_limit.rule.window_seconds`,
`redis.dial_timeout`) use `config.Duration` from `go-framework/config`,
which wraps `time.Duration`. In `conf/dev/conf.yaml` these fields are written
as duration strings such as `"30s"`, `"5m"`, or `"8ms"` and parsed by
`time.ParseDuration`. Bare integers are no longer accepted for these fields.
Redis config fields align with `go-middleware/redis.Config` (for example
`dial_timeout`, `read_timeout`, `conn_max_lifetime`); a `ToMiddlewareConfig()`
method converts `config.Duration` fields to `time.Duration` for the middleware.

**Error codes.** Errors are built with `goerror.In/Code` from `go-common/error`
(it wraps `samber/oops`; do not construct errors with `samber/oops` directly).
Framework code constants come from `go-framework/error` and are re-exported by
the generated `internal/pkg/errcode` / `internal/pkg/rpcerror` packages:
`CodeSystem=10000`, `CodeParamInvalid=10001`, `CodeAuthFailed=10002`,
`CodeConfigInvalid=10004`, `CodeRPCUnavailable=10010`, `CodeRPCTimeout=10011`.

Code segments: Framework `10000–10499`, Middleware `20000–20699`, Auth
`40000–40099`, Project `40100–59999`. **Business-defined codes must be
`>= 40100`** (`goerror.ProjectCodeMin`).

> **Behavior note:** business codes (`>= 40100`) fall back to **HTTP 200** via
> `goerror.HTTPStatus` — go-tools treats them as "business error, RPC call
> succeeded". Register fine-grained HTTP statuses with
> `goerror.RegisterHTTPStatuses` when you need non-200 responses.

## Requirements

- Go `1.25+` to build and run the `ncgo` CLI itself. **Generated projects
  require Go `1.26.5`** because they build on go-tools v0.1.0 (their `go.mod`
  declares `go 1.26.5` and the service `Dockerfile` uses `golang:1.26.5`).
- `hz >= v0.9.7` when generating Hertz services (auto-installed on demand)
- `kitex >= v0.16.1` when generating Kitex services (auto-installed on demand)
- Hertz templates' `make swagger` target requires `protoc` and
  `protoc-gen-http-swagger` on `PATH`

If you only want manifests, IDL placeholders, and template inputs, use
`--no-generate` and install the generators later.

## Install

```bash
go install github.com/byx-darwin/ncgo@latest
ncgo version
```

If `ncgo` is not found after installation, make sure your `GOBIN` (or
`$(go env GOPATH)/bin`) is on `PATH`.

From a local checkout, the repository root is also installable:

```bash
go install .
ncgo version
```

## 30-Second Tour

Assuming `hz` is already on `PATH`, the shortest happy path is:

```bash
go install github.com/byx-darwin/ncgo@latest
ncgo new user-api --module github.com/acme/user-api
cd user-api
go mod tidy
make dev
```

For Kitex, micro workspaces, and generator-free flows, see the detailed examples
below.

## Common Commands

| Command | Purpose |
| --- | --- |
| `ncgo new` | Scaffold a mono service or micro workspace; `--template-dir <dir>` / `--template <name>` generate from a template package (`--template`/`--template-dir` are mutually exclusive and neither combines with `--preset`) |
| `ncgo import` | Generate `.ncgo/manifest.yaml` for an existing Hertz/Kitex project |
| `ncgo add domain` | Generate usecase / repository / DI register files |
| `ncgo add method` | Insert a method stub at ncgo anchor markers |
| `ncgo add infra` | Add optional infra helpers such as Redis / logging / canary / polaris_adapter |
| `ncgo add rpc` / `ncgo add bff` | Add services inside a micro workspace |
| `ncgo ai init claude` | Bootstrap hand-authored `.claude` starter files (`--preset minimal` or `--preset team`) |
| `ncgo ai sync` | Render AI context files — `claude` target by default; `--target all\|agents\|claude\|cursor` selects a group |
| `ncgo protolint` | Lint selected `.proto` files with Proto I/O rules |
| `ncgo doctor` | Diagnose host tools, project metadata, and default proto contract issues |
| `ncgo check` | Validate AI context integrity: method anchors, manifest↔usecase consistency, and stale context files. Exits 0 pass / 1 check failed / 2 command error (`--output text\|json`) |
| `ncgo upgrade` | Update ncgo/assets metadata |
| `ncgo extract domain` | Plan or apply mono-to-micro extraction |
| `ncgo export templates` | Export code templates from an existing ncgo project |
| `ncgo template list` | List template packages in the official template registry |
| `ncgo template pull <name>` | Pull a template package from the registry into the local cache |
| `ncgo mcp serve` | Expose selected ncgo operations over MCP stdio |

`ncgo new --template <name>` generates from a template package already cached by
`ncgo template pull <name>`; `--template-dir <dir>` points at any local template
package (the layout `ncgo export templates` produces). The two flags are mutually
exclusive, and neither is combined with `--preset`. The registry URL defaults to
the official repository; override it with `--registry <url>` on `ncgo template
list`/`pull`, or set `NCGO_REGISTRY`. See the full registry workflow in
[docs/examples.md](docs/examples.md#9-generate-base-projects-from-official-templates).

## Typical Workflows

| Scenario | Start with | Best when |
| --- | --- | --- |
| Single Hertz service | `ncgo new <name> --module <module>` | You want the fastest path to an HTTP service scaffold |
| Single Kitex service | `ncgo new <name> --module <module> --kind kitex` | You are building an RPC-first service with Kitex |
| Micro workspace | `ncgo new <name> --module <module> --mode micro` | You need multiple services under one workspace root |
| Prepare first, generate later | `ncgo new ... --no-generate` | You want manifests/templates now and generator execution later |
| Existing project enhancement | `ncgo add domain`, `ncgo add infra`, `ncgo ai sync` | You already have an ncgo project and want to expand it incrementally |
| i18n translation in a generated project | `make i18n-report`, `ncgo i18n check --mode release --output json` | You want a stable workflow from locale/status updates to agent-assisted translation review and final validation |
| Proto contract lint in a generated project | `ncgo protolint --root . --file idl/app/demo.proto --output json` | You want Req/Resp naming, Hertz binding, and Kitex response shape checks in a repeatable workflow |
| Rule-center Kitex service | `ncgo new rule-center --module github.com/acme/rule-center --kind kitex --db postgres --preset rule-center` | You need a centralized rate-limit rule management service |
| Hertz with rule-center | `ncgo new user-api --module github.com/acme/user-api --kind hertz --db postgres --rule-center-addr rule-center:8888` | Your Hertz service queries rate-limit rules from a remote rule-center |

If you are starting fresh, pick one row above and then follow the matching
Quick Start below.

For longer, worked examples, see [docs/examples.md](docs/examples.md) and
[docs/examples.zh-CN.md](docs/examples.zh-CN.md).

## Quick Start

### Hertz HTTP service

```bash
ncgo new user-api --module github.com/acme/user-api --kind hertz
cd user-api
go mod tidy
make dev
```

`--kind hertz` is the default, so the first command can be shortened to:

```bash
ncgo new user-api --module github.com/acme/user-api
```

The Hertz template ships with `zh-CN`, `zh-TW`, `ja-JP`, `ko-KR`, `fr-FR`,
`de-DE`, and `es-ES` by default. Both the default locales and any additional
locales are maintained under `internal/pkg/i18n/locales/*.json` in the
generated project, then compiled into `internal/pkg/i18n/catalog_gen.go` via
`make i18n`.

New scaffolds also include a service-level `Dockerfile`, `.dockerignore`, and
`compose.yaml` for local container builds.

### Kitex RPC service

```bash
ncgo new user-api --module github.com/acme/user-api --kind kitex
cd user-api
make sqlc
go mod tidy
make dev
```

Kitex service names are normalised for valid proto/Go identifiers. For example,
`user-api` produces `idl/userapi.proto`, proto package `userapi`, and service
`UserApi`.

Kitex scaffolds also ship with a service-level `Dockerfile`, `.dockerignore`,
and `compose.yaml`.

### Micro workspace

```bash
ncgo new commerce --module github.com/acme/commerce --mode micro
cd commerce
```

This creates a root `ncgo.workspace`, `README.md`, `compose.yaml`, a
workspace-level `.pre-commit-config.yaml`, and an empty `services/` directory.
Add Kitex RPC and Hertz BFF services with:

```bash
ncgo add rpc user-rpc --root .
ncgo add bff web-bff --root .
```

Each generated service keeps its own `.ncgo/manifest.yaml`, `Dockerfile`, and
service-local `compose.yaml`; the workspace root `compose.yaml` is refreshed as
services are added.

## Prepare vs Generate

By default, `ncgo new` has two phases:

1. Prepare deterministic inputs: `.ncgo/manifest.yaml`, IDL placeholder, and
   custom template files under `template/`.
2. Invoke the upstream generator: `hz` for Hertz, `kitex` for Kitex.

Use `--no-generate` to stop after phase 1:

```bash
ncgo new user-api --module github.com/acme/user-api --kind kitex --no-generate
```

In that mode ncgo prints the exact generator command to run later. When the
generator runs successfully, it creates `go.mod`; the remaining next steps start
at `make sqlc` when the generated scaffold already imports `internal/db/gen`
(all Kitex services, plus Hertz services with `WithDatabase=true`), otherwise at
`go mod tidy`.

Generated projects intentionally keep `template/`:

- Hertz: `template/layout.yaml`, `template/package.yaml`, `template/data.json`
- Kitex: `template/kitex-template/*.yaml`

Generated mono services and micro workspaces also include a root
`.pre-commit-config.yaml` plus `scripts/run-go-module-checks.sh` so
contributors can enable `pre-commit` / `pre-push` checks across one or more Go
modules.

These files make future IDL updates reproducible (`make update` in generated
Kitex projects, equivalent generator command for Hertz).

## AI Context

Optionally bootstrap hand-authored `.claude` starter files once:

```bash
ncgo ai init claude --root user-api
```

For machine-readable CLI output during init, add `--output json`.

For the workflow-oriented starter set, use:

```bash
ncgo ai init claude --root user-api --preset team
```

The command reports whether the target root was detected as a service root, a micro workspace root, or still unknown.

On a successful non-dry-run init, it also suggests the next sync command: `ncgo ai sync --root <root> --lang en`.

After generating a project, sync AI-readable context:

```bash
ncgo ai sync --root user-api --lang en
```

For machine-readable CLI output, add `--output json`.

For a micro workspace root, run the same command at the workspace root:

```bash
ncgo ai sync --root commerce --lang en
```

By default `ncgo ai sync` renders the `claude` target group, which writes:

- `CLAUDE.md`
- `.claude/skills/ncgo-dev/SKILL.md`
- `.claude/generated/project-context.md`

Select a different group with `--target`:

- `agents` — `AGENTS.md`
- `cursor` — `.cursor/rules/ncgo.mdc`
- `all` — all five files above

```bash
ncgo ai sync --root user-api --target all
```

> **Migration note:** earlier versions of `ncgo ai sync` wrote every context
> file by default. The default is now `claude`; pass `--target all` to keep
> the previous full behavior.

Files contain `<!-- ncgo:managed -->`; existing files without the marker are
skipped unless `--force` is passed. Add project-specific notes in
`AGENTS.local.md`; they are appended to the long-form generated context files,
while `.claude/generated/project-context.md` stays deterministic.

Rendered files carry a `<!-- ncgo:generated-at: ... -->` marker with the
manifest timestamp that produced them. `ncgo check` compares the domains
declared in a rendered context file against the current `manifest.Domains`;
a mismatch means the context is stale.

When `--root` is a micro workspace root, the generated files describe
workspace-level facts from `ncgo.workspace` and list the registered services.
For service-level context, run `ncgo ai sync --root services/<name> --lang en`
inside the generated service directory.

When `--root` is a service directory that is also registered in a parent micro
workspace, `ncgo ai sync` keeps generating service-level context from the local
`.ncgo/manifest.yaml`, but it also annotates the generated facts with workspace
membership details such as the parent workspace name, module, relative root,
and registered service directory. The CLI summary prints matching `info:` lines
so agents and humans can tell that the service belongs to a larger workspace.

Starter files created by `ncgo ai init claude` are hand-authored and are not
overwritten by `ncgo ai sync`.

## Command Reference

### Domain and method anchors

```bash
ncgo add domain <name> --root .
ncgo add method device.ListThemes --root . --in usecase
```

`ncgo add method` inserts a no-argument `UseCase` method stub between
`// ncgo:methods:start` and `// ncgo:methods:end` markers. Its text output then
lists the next steps: `go build ./...`, replace the generated stub body with
domain logic, and `ncgo ai sync --root .`.

For machine-readable output, add `--output json`; the result carries `path`,
`domain`, `method`, and `nextSteps`:

```bash
ncgo add method device.ListThemes --root . --in usecase --output json
```

```json
{
  "path": "internal/usecase/device/device.go",
  "domain": "device",
  "method": "ListThemes",
  "nextSteps": [
    "go build ./...",
    "replace the generated stub body with domain logic",
    "ncgo ai sync --root ."
  ]
}
```

### Optional infra

```bash
ncgo add infra redis --root .
ncgo add infra logging --root .              # alias for observability_logging
ncgo add infra canary --root .               # alias for release_canary
ncgo add infra logging --root . --wire       # optional: patch generated server/client wiring
ncgo add infra logging --root . --wire --dry-run  # preview writes/wiring without changes
ncgo add infra logging --root . --wire --dry-run --output json  # machine-readable plan
ncgo add infra logging --root . --wire --plan  # shorthand for --dry-run --output json
ncgo add infra registry_polaris --root . --wire  # kitex only: Polaris registry + wire
ncgo add infra rate-limit --root . --wire         # kitex only: shared ratelimit pkg + real middleware (shadow-first)
ncgo add infra polaris_adapter --root .           # kitex only: opt-in real Polaris canary adapter
```

Supported common infra add-ons: `redis`, `kafka`, `es`, `clickhouse`,
`observability_logging` (`logging` alias),
and `release_canary` (`canary` alias).
Kitex-only add-ons: `registry_polaris`, `rate-limit`, `polaris_adapter`.

`rate-limit` is **kitex-only** (Hertz uses a different rate-limit design). It
rewrites the Kitex template's pass-through `RateLimit()` placeholder into a real
`endpoint.Middleware` backed by the shared `internal/pkg/ratelimit` package
(resolver + store, single source of truth with the Hertz side). The middleware is
wired into the server chain after `CallerAllowlist`, and a static
`server.WithLimit` safety net is conditionally mounted via the
`// ncgo:wire:ratelimit:static-limit` marker. The generated conf defaults to
`mode = shadow` (counted but never rejected); switch to `mode = enforce` after
observing shadow logs. See
[rate-limit-dynamic-design.zh-CN.md](internal/assets/_data/docs/hertz/rate-limit-dynamic-design.zh-CN.md)
(§19) for the dual-track model, shadow-first operations flow, and 10429
rejection semantics.

`ncgo test rate-limit e2e` verifies a generated project's rate limiting against a
running instance. For Kitex services it drives the attack over gRPC via grpcurl:

```bash
ncgo test rate-limit e2e --rpc-method MyService.Ping --rpc-payload '{"user":"alice"}'
```

The two-stage run first confirms shadow mode produces zero rejections (with
`ratelimit shadow denied` log lines), then confirms enforce mode returns the
expected ratio of 10429 responses.

`polaris_adapter` is an **opt-in, kitex-only** add-on that wires the
SDK-neutral `internal/base/release` seams (from `release_canary`) to a real
Polaris backend via `polaris-go`. It writes
`internal/base/release/polaris_adapter.go` (package `release`) and prints the
next-step `go get` commands on stdout. ncgo itself stays SDK-free — the Polaris
SDK dependency lives in the user's project, not in ncgo.

Enable it after `release_canary` is already in place:

```bash
ncgo add infra polaris_adapter --root .
# then follow the printed go get next-steps, e.g.:
#   go get github.com/polarismesh/polaris-go
#   go get gopkg.in/yaml.v3
#   go get github.com/byx-darwin/go-tools/go-common
```

Provide Polaris credentials via environment variables
(`POLARIS_TOKEN`, `POLARIS_NAMESPACE`); never hardcode them. Wire the adapter
into the Kitex canary load balancer by constructing
`release.NewPolarisSelector(discoveryCfg, ruleCfg)` and feeding its
`RuleProvider` into `KitexCanaryLoadBalancer.RuleProvider`. The adapter was
tested with `polaris-go v1.7.1`.

**Troubleshooting**

- `addresses is empty` / missing token → construction fails fast. Fix the
  config / env vars before retrying.
- Discovery or rule-load failure at runtime → canary routing **fails OPEN** to
  the Kitex default weighted LB (availability-first). Observe metrics before
  switching expectations.
- **Kitex resolver instance-visibility assumption** — if the canary pool is
  empty, confirm the Kitex resolver (e.g. `registry_polaris`) returns the FULL
  stable+canary instance set. If the resolver filters by routing, the adapter
  would need to sit at the LB layer (future work).
- `release.track` metadata not taking effect → verify the registering side
  sets `release.track` metadata on instances.

GA hardening (metrics / cache+TTL / dry-run / runtime harness) is future work.

Observability (tracing / OTel) is now built into the Hertz and Kitex base
templates via go-framework OTLP (Hertz: `cfg.Server.Jaeger` →
`hertz/observability.NewProvider`; Kitex: `cfg.Jaeger` →
`kitex/observability.NewProvider`). The legacy `observability_otel` / `otel`
add-on has been removed; `ncgo add infra otel` now returns invalid kind.

For Hertz projects, `redis` now defaults to a single shared
`redis.UniversalClient` derived from top-level `cfg.Redis` via
`go-middleware/redis.NewUniversalClient`: signature nonce,
rate-limit, idempotency, and optional `internal/base/data/redis.go` reuse the
same client unless you set a module-specific Redis override.

The `kafka`, `es`, and `clickhouse` add-ons now delegate connection
construction to `go-middleware` factory methods (`go-middleware/kafka`,
`go-middleware/es`, `go-middleware/clickhouse`). Generated wrapper structs
(`KafkaWriter`, `KafkaReader`, `ES`, `ClickHouse`) accept go-middleware
`Config` types via samber/do instead of raw third-party library structs.
Error codes use `go-framework/error.CodeConfigInvalid` for config validation
and go-middleware predefined codes (ClickHouse) or project-segment codes
(ES: `40506`) for connection failures.

`observability_logging` generates `internal/base/logging/logging.go` plus a
framework adapter (`hertz.go` or `kitex.go`). It uses `go-common/log` for
structured logging with `WithCategory` sub-loggers, masking, OTel trace context
injection, and `go-common/error` (`goerror`) structured error extraction.

The default Hertz/Kitex templates only include commented logging wiring anchors,
so projects that have not enabled the optional `internal/base/logging` package
continue to compile. Use opt-in `--wire` to patch generated server/client wiring
automatically; add `--dry-run` to preview the optional files, manifest update, and
wiring targets without modifying files. See `specs/007-observability-logging.zh-CN.md`
for examples.

`release_canary` generates `internal/base/release/canary.go` plus a framework
adapter (`hertz.go` or `kitex.go`). The MVP is an SDK-neutral helper for release
metadata, traffic context, Hertz header extraction, Kitex metadata propagation,
unified canary rules, Nacos/Polaris discovery instance models,
`Discoverer`/`RuleProvider`/`Selector` abstractions, stable/canary pool
splitting, weighted selection, `fallback=stable|fail_fast`, SDK-neutral Kitex
load balancing, and Nacos/Polaris discoverer/rule-provider skeletons. Concrete
Nacos/Polaris SDK adapters can be layered on later.

The default Hertz/Kitex templates only include commented canary wiring anchors,
so projects that have not enabled the optional `internal/base/release` package
continue to compile. Use opt-in `--wire` to mount traffic middleware automatically.
Use `--dry-run` with `--wire` to preview the source files that would change. See
`specs/006-canary-release.zh-CN.md` for examples.

The Hertz template includes lightweight localized response messages. `internal/pkg/i18n`
selects `en`, `zh-CN`, `zh-TW`, `ja-JP`, `ko-KR`, `fr-FR`, `de-DE`, or `es-ES`
from `Accept-Language`, and `internal/pkg/response` translates JSON response
`msg` values while setting `Content-Language`. These default locales are also
loaded from `internal/pkg/i18n/locales/*.json` and registered by the generated
`catalog_gen.go`. Without an `Accept-Language` header, responses keep the
existing English error keys. Business messages can be extended at startup with
`i18n.Register`.

### Micro services

```bash
ncgo new commerce --module github.com/acme/commerce --mode micro
ncgo add rpc user-rpc --root commerce
ncgo add bff web-bff --root commerce
```

### Diagnostics, lifecycle, and agents

```bash
ncgo doctor --root .
ncgo doctor --root . --output json
ncgo doctor --root . --output sarif > doctor.sarif.json
ncgo protolint --root . --file idl/app/demo.proto --output json
ncgo mcp serve
ncgo upgrade --root . --plan
ncgo upgrade --root . --dry-run
ncgo extract domain device --root . --to services/device-rpc
ncgo extract domain device --root . --to services/device-rpc --apply
ncgo version
```

`ncgo doctor` now checks the Go toolchain, `hz` / `kitex`, the manifest, `template/data.json`, and
runs proto lint automatically when `manifest.service.idl` is present. The CLI
supports `--output text|json|sarif`; `--json` remains as a compatibility alias
for `--output json`. SARIF output is suitable for code scanning, IDE problems,
and CI artifact pipelines.

`ncgo import` reverse-generates `.ncgo/manifest.yaml` for an existing project.
Kind auto-detection relies on generator marker files: `router.go` containing
`// Code generated by hz.` (Hertz) or `handler.go` containing
`// Code generated by kitex.` (Kitex). Projects scaffolded with
`ncgo new --no-generate` have no marker files yet, so import them with an
explicit `--kind` flag (e.g. `ncgo import --root . --kind kitex`).

`ncgo mcp serve` starts a stdio MCP server. It currently exposes
`ncgo_version`, `ncgo_doctor`, `ncgo_ai_init_claude`, `ncgo_ai_sync`,
`ncgo_i18n_report`, `ncgo_i18n_check`, `ncgo_protolint`, `ncgo_add_infra`,
`ncgo_add_method`, and `ncgo_ai_context` tools. `ncgo_ai_context` scans real
code and returns structured domains/methods/anchors/consistency for agents.
The `ncgo_ai_sync` tool accepts the same `target` values as the CLI
(`all|agents|claude|cursor`, default `claude`), and
`ncgo_add_method` supports `output: json`.
The MCP interface is now documented in a contract-first layout in
[`docs/examples.md#0-mcp-contract-first-reference`](docs/examples.md#0-mcp-contract-first-reference): see `0. MCP contract-first reference`
for each tool's inputs, supported `output` values, and stable top-level result
fields before the workflow examples. In short, structured MCP tools keep
`content[0].text` as the display/export payload and expose sibling top-level
fields for agent use; `output` only changes the text payload format.

If you use the built-in i18n workflow in a generated Hertz project, you can now
consume structured results via `ncgo i18n report` / `ncgo i18n check` or MCP via
`ncgo_i18n_report` / `ncgo_i18n_check`. See the worked example in
[`docs/examples.md#5-i18n-translation-workflow-in-a-generated-project`](docs/examples.md#5-i18n-translation-workflow-in-a-generated-project).

If you want the same kind of structured workflow for `.proto` contracts, use
`ncgo protolint --root . --file ...` or the MCP `ncgo_protolint` tool. The CLI
supports `--output text|json|sarif`; SARIF can be fed directly into code
scanning and IDE tooling. You can also suppress known legacy findings with
`--ignore-rule` / `--ignore-file` (MCP: `ignoreRules` / `ignoreFiles`). In mono
service roots or micro workspace roots, omitting `--file` lets ncgo auto-
discover entry proto files from the manifest or workspace. See the worked
example in [`docs/examples.md#6-proto-contract-lint-workflow-in-a-generated-project`](docs/examples.md#6-proto-contract-lint-workflow-in-a-generated-project).

The built-in Proto I/O rules now include a first batch of default `phase2`
warnings in addition to the existing `error` rules: `PIO111`, `PIO112`,
`PIO113`, `PIO211`, `PIO212`, `PIO302`, `PIO303`, `PIO401`, `PIO402`, `PIO403`,
and `PIO404`. These warnings still appear in `diagnostics` and doctor reports,
but **warning-only runs keep `ok=true`**; CLI / MCP / doctor only fail when an
`error` rule is hit.

`ncgo upgrade` updates ncgo/assets version metadata in `.ncgo/manifest.yaml` or
`ncgo.workspace` (and listed micro service manifests). `--plan` prints a detailed
read-only metadata plan for the root/workspace and service manifests; `--dry-run`
keeps the older concise no-write output. The MVP does not rewrite generated
source files.

`ncgo extract domain` emits a migration plan for mono-to-micro domain extraction.
With `--apply`, it copies the planned domain files into an existing Kitex target
service and rewrites domain-local imports to the target module. It does not
delete source files, overwrite target files, or wire cross-service clients.

## FAQ

### `ncgo: command not found`

Make sure your `GOBIN` or `$(go env GOPATH)/bin` is on `PATH`, then rerun:

```bash
ncgo version
```

### `hz` or `kitex` not found on PATH

`ncgo new` will automatically detect missing generators and offer to
install them for you. Answer `Y` at the prompt to auto-install, or type
`n` to abort and install manually:

```bash
go install github.com/cloudwego/hertz/cmd/hz@latest
go install github.com/cloudwego/kitex/tool/cmd/kitex@latest
ncgo doctor
```

If you want to prepare files first and run generators later, use `--no-generate`.

### Hertz `make swagger` cannot find `protoc` or the plugin

Hertz templates' `make swagger` runs `protoc --http-swagger_out=...`, so both of
these tools must be available:

- `protoc`: the Protocol Buffers compiler, installed via your system package
  manager or an official release.
- `protoc-gen-http-swagger`: the Go plugin, installed into `GOBIN` or
  `$(go env GOPATH)/bin`; that directory must be on `PATH`.

Common setup:

```bash
# macOS / Homebrew
brew install protobuf

# Go plugin
go install github.com/hertz-contrib/swagger-generate/protoc-gen-http-swagger@latest

# Verify PATH
protoc --version
protoc-gen-http-swagger --help
```

Generated Hertz projects also include `make install-tools`, which installs Go-side
development tools including `protoc-gen-http-swagger`; `protoc` itself still needs
to be installed through your system tooling.

The Swagger spec is embedded into the service binary with `go:embed`. After
`make swagger` updates `internal/docs/swagger/openapi.yaml`, rerun `go run .` /
`make dev` or rebuild and restart the service so `/swagger/openapi.yaml` serves
the updated spec.

## Development Checks

For contributor-oriented local workflow and PR guidance, see
[`CONTRIBUTING.md`](CONTRIBUTING.md).

```bash
go build .
go test ./... -count=1
./scripts/smoke.sh
```

CI runs a fuller set of checks on GitHub Actions. Release builds are tag-driven;
see [specs/008-release-process.zh-CN.md](specs/008-release-process.zh-CN.md) for the release flow.

Update scaffold goldens after intentional template/scaffold changes:

```bash
go test ./internal/scaffold/mono/... -update-golden -count=1
```
