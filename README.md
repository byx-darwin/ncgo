# ncgo

[![CI](https://github.com/byx-darwin/ncgo/actions/workflows/ci.yml/badge.svg)](https://github.com/byx-darwin/ncgo/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/byx-darwin/ncgo)](https://github.com/byx-darwin/ncgo/releases)
[![Go 1.25+](https://img.shields.io/badge/Go-1.25%2B-00ADD8?logo=go)](https://go.dev/)
[![License](https://img.shields.io/github/license/byx-darwin/ncgo)](LICENSE)

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
- Optional infra: Redis, Kafka, Elasticsearch, ClickHouse, LoongSuite Go Agent observability, structured logging, canary release helpers, and Kitex-only etcd registry.
- AI/agent workflow: `ai init claude`, `ai sync`, static `doctor`, and MCP stdio server.
- Lifecycle MVPs: metadata-only `upgrade --plan` and conservative `extract domain --apply`.

Deferred optionals remain documented but intentionally not implemented yet:
~~NATS~~, ~~Mongo~~, and ~~MinIO~~.

## What ncgo Generates

| Area | Output |
| --- | --- |
| Project metadata | `.ncgo/manifest.yaml` for services; `ncgo.workspace` for micro roots |
| Hertz | IDL placeholder, custom `hz` layout/package inputs, HTTP service skeleton |
| Kitex | IDL placeholder, custom Kitex template tree, RPC service skeleton |
| Domain | `internal/usecase/<name>`, `internal/repository/<name>`, DI register file |
| Infra | Optional drop-in Go files under `internal/base/...` |
| AI context | `AGENTS.md`, `CLAUDE.md`, `.claude/generated/project-context.md`, `.cursor/rules/ncgo.mdc` |

## Requirements

- Go `1.25+`
- `hz >= v0.9.7` when generating Hertz services
- `kitex >= v0.16.1` when generating Kitex services
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
| `ncgo new` | Scaffold a mono service or micro workspace |
| `ncgo add domain` | Generate usecase / repository / DI register files |
| `ncgo add method` | Insert a method stub at ncgo anchor markers |
| `ncgo add infra` | Add optional infra helpers such as Redis / logging / canary |
| `ncgo add rpc` / `ncgo add bff` | Add services inside a micro workspace |
| `ncgo ai init claude` | Bootstrap hand-authored `.claude` starter files (`--preset minimal` or `--preset team`) |
| `ncgo ai sync` | Render `AGENTS.md`, `CLAUDE.md`, `.claude/generated/project-context.md`, and Cursor rules |
| `ncgo protolint` | Lint selected `.proto` files with Proto I/O rules |
| `ncgo doctor` | Diagnose host tools, project metadata, and default proto contract issues |
| `ncgo upgrade` | Update ncgo/assets metadata |
| `ncgo extract domain` | Plan or apply mono-to-micro extraction |
| `ncgo mcp serve` | Expose selected ncgo operations over MCP stdio |

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

This creates a root `ncgo.workspace`, `README.md`, a workspace-level
`.pre-commit-config.yaml`, and an empty `services/` directory. Add Kitex RPC
and Hertz BFF services with:

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

This writes managed files:

- `AGENTS.md`
- `CLAUDE.md`
- `.claude/generated/project-context.md`
- `.cursor/rules/ncgo.mdc`

Files contain `<!-- ncgo:managed -->`; existing files without the marker are
skipped unless `--force` is passed. Add project-specific notes in
`AGENTS.local.md`; they are appended to the long-form generated context files,
while `.claude/generated/project-context.md` stays deterministic.

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

`ncgo doctor` now checks `hz` / `kitex`, the manifest, `template/data.json`, and
runs proto lint automatically when `manifest.service.idl` is present. The CLI
supports `--output text|json|sarif`; `--json` remains as a compatibility alias
for `--output json`. SARIF output is suitable for code scanning, IDE problems,
and CI artifact pipelines.

`ncgo mcp serve` starts a stdio MCP server. It currently exposes
`ncgo_version`, `ncgo_doctor`, `ncgo_ai_init_claude`, `ncgo_ai_sync`,
`ncgo_i18n_report`, `ncgo_i18n_check`, `ncgo_protolint`, `ncgo_add_infra`, and
`ncgo_add_method` tools.
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

Install the missing generator and rerun the command:

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
see [docs/release.zh-CN.md](docs/release.zh-CN.md) for the release flow.

Update scaffold goldens after intentional template/scaffold changes:

```bash
go test ./internal/scaffold/mono/... -update-golden -count=1
```
