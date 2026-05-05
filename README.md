# ncgo

[![CI](https://github.com/byx-darwin/ncgo/actions/workflows/ci.yml/badge.svg)](https://github.com/byx-darwin/ncgo/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/byx-darwin/ncgo)](https://github.com/byx-darwin/ncgo/releases)
[![Go 1.25+](https://img.shields.io/badge/Go-1.25%2B-00ADD8?logo=go)](https://go.dev/)

AI-friendly scaffold CLI for Go microservices. `ncgo` owns embedded Hertz and
Kitex templates, writes a project manifest, invokes upstream generators, and
renders AI context files for Claude/Cursor/other agents.

Use it when you want reproducible Go service scaffolds, optional infra add-ons,
and agent-friendly project context from a single CLI.

中文文档见 [README.zh-CN.md](README.zh-CN.md)。Product requirements live in
[docs/prd.md](docs/prd.md) and [docs/prd.zh-CN.md](docs/prd.zh-CN.md). For
agent handoff, see [docs/context-handoff.zh-CN.md](docs/context-handoff.zh-CN.md).

**Quick links:** [Install](#install) · [30-Second Tour](#30-second-tour) · [Typical Workflows](#typical-workflows) · [Examples](docs/examples.md) · [Contributing](CONTRIBUTING.md) · [FAQ](#faq)

## Why ncgo

- **Deterministic scaffolding**: keep manifests, IDL placeholders, and templates under version control
- **Generator-aware**: orchestrate `hz` / `kitex`, but still allow `--no-generate` workflows
- **Agent-friendly by default**: render `AGENTS.md`, `CLAUDE.md`, Cursor rules, and expose MCP tools
- **Lifecycle helpers included**: ship `doctor`, `upgrade`, and conservative `extract domain` flows in the same CLI

## Highlights

| Area | What you get |
|---|---|
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
- Optional infra: Redis, Kafka, Elasticsearch, ClickHouse, LoongSuite Go Agent observability, structured logging, canary release helpers, and Kitex-only etcd registry.
- AI/agent workflow: `ai sync`, static `doctor`, and MCP stdio server.
- Lifecycle MVPs: metadata-only `upgrade --plan` and conservative `extract domain --apply`.

Deferred optionals remain documented but intentionally not implemented yet:
~~NATS~~, ~~Mongo~~, and ~~MinIO~~.

## What ncgo Generates

| Area | Output |
|---|---|
| Project metadata | `.ncgo/manifest.yaml` for services; `ncgo.workspace` for micro roots |
| Hertz | IDL placeholder, custom `hz` layout/package inputs, HTTP service skeleton |
| Kitex | IDL placeholder, custom Kitex template tree, RPC service skeleton |
| Domain | `internal/usecase/<name>`, `internal/repository/<name>`, DI register file |
| Infra | Optional drop-in Go files under `internal/base/...` |
| AI context | `AGENTS.md`, `CLAUDE.md`, `.cursor/rules/ncgo.mdc` |

## Requirements

- Go `1.25+`
- `hz >= v0.9.7` when generating Hertz services
- `kitex >= v0.16.1` when generating Kitex services

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
|---|---|
| `ncgo new` | Scaffold a mono service or micro workspace |
| `ncgo add domain` | Generate usecase / repository / DI register files |
| `ncgo add method` | Insert a method stub at ncgo anchor markers |
| `ncgo add infra` | Add optional infra helpers such as Redis / logging / canary |
| `ncgo add rpc` / `ncgo add bff` | Add services inside a micro workspace |
| `ncgo ai sync` | Render `AGENTS.md`, `CLAUDE.md`, and Cursor rules |
| `ncgo doctor` | Diagnose host tools and project metadata |
| `ncgo upgrade` | Update ncgo/assets metadata |
| `ncgo extract domain` | Plan or apply mono-to-micro extraction |
| `ncgo mcp serve` | Expose selected ncgo operations over MCP stdio |

## Typical Workflows

| Scenario | Start with | Best when |
|---|---|---|
| Single Hertz service | `ncgo new <name> --module <module>` | You want the fastest path to an HTTP service scaffold |
| Single Kitex service | `ncgo new <name> --module <module> --kind kitex` | You are building an RPC-first service with Kitex |
| Micro workspace | `ncgo new <name> --module <module> --mode micro` | You need multiple services under one workspace root |
| Prepare first, generate later | `ncgo new ... --no-generate` | You want manifests/templates now and generator execution later |
| Existing project enhancement | `ncgo add domain`, `ncgo add infra`, `ncgo ai sync` | You already have an ncgo project and want to expand it incrementally |

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

### Kitex RPC service

```bash
ncgo new user-api --module github.com/acme/user-api --kind kitex
cd user-api
go mod tidy
make dev
```

Kitex service names are normalised for valid proto/Go identifiers. For example,
`user-api` produces `idl/userapi.proto`, proto package `userapi`, and service
`UserApi`.

### Micro workspace

```bash
ncgo new commerce --module github.com/acme/commerce --mode micro
cd commerce
```

This creates a root `ncgo.workspace`, `README.md`, and an empty `services/`
directory. Add Kitex RPC and Hertz BFF services with:

```bash
ncgo add rpc user-rpc --root .
ncgo add bff web-bff --root .
```

Each generated service keeps its own `.ncgo/manifest.yaml`.

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
at `go mod tidy`.

Generated projects intentionally keep `template/`:

- Hertz: `template/layout.yaml`, `template/package.yaml`, `template/data.json`
- Kitex: `template/kitex-template/*.yaml`

These files make future IDL updates reproducible (`make update` in generated
Kitex projects, equivalent generator command for Hertz).

## AI Context

After generating a project, sync AI-readable context:

```bash
ncgo ai sync --root user-api --lang en
```

This writes managed files:

- `AGENTS.md`
- `CLAUDE.md`
- `.cursor/rules/ncgo.mdc`

Files contain `<!-- ncgo:managed -->`; existing files without the marker are
skipped unless `--force` is passed. Add project-specific notes in
`AGENTS.local.md`; they are appended on every sync.

## Command Reference

### Domain and method anchors

```bash
ncgo add domain <name> --root .
ncgo add method device.ListThemes --root . --in usecase
```

`ncgo add method` currently inserts a no-argument `UseCase` method stub between
`// ncgo:methods:start` and `// ncgo:methods:end` markers.

### Optional infra

```bash
ncgo add infra redis --root .
ncgo add infra otel --root .                 # alias for observability_otel
ncgo add infra logging --root .              # alias for observability_logging
ncgo add infra canary --root .               # alias for release_canary
ncgo add infra logging --root . --wire       # optional: patch generated server/client wiring
ncgo add infra logging --root . --wire --dry-run  # preview writes/wiring without changes
ncgo add infra logging --root . --wire --dry-run --output json  # machine-readable plan
ncgo add infra logging --root . --wire --plan  # shorthand for --dry-run --output json
ncgo add infra registry_etcd --root .        # kitex only
```

Supported common infra add-ons: `redis`, `kafka`, `es`, `clickhouse`,
`observability_otel` (`otel` alias), `observability_logging` (`logging` alias),
and `release_canary` (`canary` alias).
Kitex-only add-ons: `registry_etcd`.

`observability_otel` now targets Alibaba LoongSuite Go Agent. It generates
`internal/base/observability/otel.go` with `OTEL_*` environment helpers and
prints setup steps such as installing the `otel` CLI and using `otel go build`.
It does not install the agent, rewrite startup code, or add SDK dependencies.

`observability_logging` generates `internal/base/logging/logging.go` plus a
framework adapter (`hertz.go` or `kitex.go`). The MVP supports `slog`,
console/file/both/none, `lumberjack` rotation with gzip compression, log
categories, `samber/oops` structured error extraction, and
request/trace/release/canary fields.

The default Hertz/Kitex templates only include commented logging wiring anchors,
so projects that have not enabled the optional `internal/base/logging` package
continue to compile. Use opt-in `--wire` to patch generated server/client wiring
automatically; add `--dry-run` to preview the optional files, manifest update, and
wiring targets without modifying files. See `docs/observability-logging.zh-CN.md`
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
`docs/canary-release.zh-CN.md` for examples.

### Micro services

```bash
ncgo new commerce --module github.com/acme/commerce --mode micro
ncgo add rpc user-rpc --root commerce
ncgo add bff web-bff --root commerce
```

### Diagnostics, lifecycle, and agents

```bash
ncgo doctor --root .
ncgo mcp serve
ncgo upgrade --root . --plan
ncgo upgrade --root . --dry-run
ncgo extract domain device --root . --to services/device-rpc
ncgo extract domain device --root . --to services/device-rpc --apply
ncgo version
```

`ncgo mcp serve` starts a stdio MCP server. The MVP exposes `ncgo_version`,
`ncgo_doctor`, `ncgo_ai_sync`, `ncgo_add_infra`, and `ncgo_add_method` tools.
`ncgo_add_infra` accepts `root`, `kind`, `force`, `wire`, and `dryRun`; it
supports the same infra kinds as the CLI, prints dependency next steps without
running `go get`, and returns structured `plan` fields for agent previews.

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

Install the missing generator and rerun the command:

```bash
go install github.com/cloudwego/hertz/cmd/hz@latest
go install github.com/cloudwego/kitex/tool/cmd/kitex@latest
ncgo doctor
```

If you want to prepare files first and run generators later, use `--no-generate`.

## Development Checks

For contributor-oriented local workflow and PR guidance, see
[`CONTRIBUTING.md`](CONTRIBUTING.md).

```bash
go build .
go test ./... -count=1
./scripts/smoke.sh
```

CI runs a fuller set of checks on GitHub Actions. Release builds are tag-driven;
see [docs/release.zh-CN.md](docs/release.zh-CN.md) for the release flow.

Update scaffold goldens after intentional template/scaffold changes:

```bash
go test ./internal/scaffold/mono/... -update-golden -count=1
```
