# ncgo Product Requirements (v1.0)

## 1. Positioning

ncgo is an AI-friendly scaffold for Go microservices. It turns the
[`nc-skills-golang`](https://github.com/byx-darwin/nc-skills-golang) conventions
(Hertz / Kitex / pgx / sqlc / samber/do / oops) from agent-readable rules into
an executable tool, and exposes every operation to AI agents through both CLI
and MCP.

- **Rules / conventions**: `nc-skills-golang` (review-mode prompts, layer
  rules, AI invocation guides for ncgo).
- **Templates / executor**: `ncgo` (binary that owns the scaffold templates
  and generates and inspects projects).
- **Contract**: anything ncgo generates must pass `nc-skills-golang` review mode.

## 2. Users

- Engineers using Claude Code / Cursor / Codex to build Go backends.
- Small teams that want a single, opinionated way to start services.
- Existing nc-skills users who want a single command to bootstrap a service.

## 3. Core Decisions

| # | Decision | Value |
|---|---|---|
| 1 | Strategy | A → C: wrap `hz`/`kitex` first, layer AI Runtime later |
| 2 | Current scaffold scope | Hertz/Kitex mono services plus micro workspace with Kitex RPC and Hertz BFF services |
| 3 | CLI | cobra + viper |
| 4 | MCP | Yes, in v0.3 |
| 5 | Assets | scaffold templates owned by ncgo (embedded under `internal/assets/_data/`); nc-skills-golang documents conventions only |
| 6 | `ncgo extract` | v0.4 |
| 7 | Project metadata | `.ncgo/manifest.yaml` |
| 8 | Optional infra | MVP covers Redis/Kafka/ES/ClickHouse/LoongSuite Go Agent observability; ~~NATS~~/~~Mongo~~/~~MinIO~~ deferred |

## 4. Command Surface

```
ncgo new <name>            --module --mode mono|micro --kind hertz|kitex --db postgres|none --idl --dir --no-generate
ncgo add domain <name>     --root --force
ncgo add method <domain.Method> --root --in usecase
ncgo add infra <kind>      --root --force; common: redis|kafka|es|clickhouse|observability_otel|otel; kitex-only: registry_etcd
ncgo add rpc <name>        --root --module --dir --no-generate  # micro workspace; Kitex service
ncgo add bff <name>        --root --module --dir --no-generate  # micro workspace; Hertz service
ncgo doctor                --json --root
ncgo ai sync               --root --lang en|zh-CN --force --dry-run
ncgo mcp serve             expose selected commands as MCP tools (MVP)
ncgo upgrade               --root --dry-run --plan  # metadata-only MVP
ncgo version
# v0.4
ncgo extract domain <name> --root --to services/<name>-rpc --json --apply  # conservative copy apply
```

The CLI is the primary API. Selected commands provide machine-readable output
(`doctor --json`, `extract domain --json`) and selected operations are exposed
through MCP. MCP MVP exposes `ncgo_version`, `ncgo_doctor`, `ncgo_ai_sync`,
`ncgo_add_infra`, and `ncgo_add_method`; broader CLI coverage is incremental.
`ncgo_add_infra` accepts `root`, `kind`, and `force`, supports the CLI infra kind
set, and returns dependency next steps without installing packages.

## 5. Project Metadata: `.ncgo/manifest.yaml`

```yaml
ncgo: {version: 0.1.0, assets_version: 0.1.0}
mode: mono                # mono | micro
module: github.com/acme/user-api
service:
  name: user-api
  kind: hertz             # hertz | kitex
  with_database: false
  idl: idl/app/user.proto  # hertz default; kitex default is idl/<service>.proto
infra: [redis]
domains: [device, theme]
generated_at: 2026-04-29T15:00:00+08:00
```

In `micro` mode the repo root holds `ncgo.workspace` and each service
keeps its own `.ncgo/manifest.yaml`.

```yaml
ncgo: {version: 0.3.0-dev, assets_version: 0.1.0}
mode: micro
name: commerce
module: github.com/acme/commerce
services: []              # add rpc/bff append entries here
generated_at: 2026-04-29T15:00:00+08:00
```

## 6. AI Collaboration Artifacts (`ncgo ai sync`)

| File | Audience | Source |
|---|---|---|
| `AGENTS.md` | any agent | manifest + embedded Hertz/Kitex design doc + optional `AGENTS.local.md` |
| `CLAUDE.md` | Claude Code | same, Claude formatting |
| `.cursor/rules/ncgo.mdc` | Cursor | same, with MDC frontmatter |
| `docs/ai-context/architecture.md` | RAG | future: actual tree + domain port signatures |

- Current generation is idempotent and based on manifest + embedded design docs,
  never on the previous generated file. AST-derived `architecture.md` is deferred.
- Top of every managed file: `<!-- ncgo:managed -->`.
- Custom content lives in `AGENTS.local.md`, included on render.

## 7. Agent-friendly Anchors

Generated Go files carry stable comments so agents can insert code precisely:

```go
// ncgo:domain=device kind=usecase
type UseCase struct { ... }

// ncgo:methods:start
// ncgo:methods:end
```

`ncgo add method device.ListThemes --in usecase` inserts a method stub between
the markers. The MVP supports usecase methods; repository/stub synchronized
method insertion remains future work.

## 8. doctor Checks (MVP)

External tool baseline (single source of truth in `internal/exec`):

| Tool | Minimum | Rationale |
|---|---|---|
| `hz` | `v0.9.7` | latest stable of `github.com/cloudwego/hertz/cmd/hz` |
| `kitex` | `v0.16.1` | latest stable of `github.com/cloudwego/kitex` |

`ncgo new` calls `hz` (hertz) or `kitex` (kitex) directly when present and
surfaces a structured `exec.NotFoundError` with an install hint when it is
missing. `--no-generate` skips the external call and prints the exact command to
run later. When the generator runs successfully it creates `go.mod`; post-
generate next steps start at `go mod tidy`.

Static checks mapped to `nc-skills-golang/docs/checklist.md` §1:

- `template/data.json` GoModule/ServiceName consistency *(auto-fix)*
- `WithDatabase` consistent with presence of `internal/db/` *(report)*
- `internal/handler` does not import `internal/repository` *(report)*
- `internal/handler` does not import `internal/base/data` *(report)*
- `internal/usecase` does not import `github.com/cloudwego/hertz` *(report)*
- `internal/usecase` does not import `github.com/cloudwego/kitex` *(report)*
- No raw SQL strings in `internal/repository` *(report)*
- `internal/repository` does not import `internal/usecase` *(report)*
- No `*app.RequestContext` leaking into usecase *(report)*

`--json` output:

```json
{"checks":[{"id":"layer.usecase.no-hertz","ok":false,"file":"internal/usecase/order/usecase.go","line":12,"rule":"nc-skills-golang/SKILL.md#layer-rules"}]}
```

## 9. Repository Layout (ncgo itself)

```
ncgo/
├── main.go           root install entry
├── internal/
│   ├── cli/          cobra commands and reusable CLI bootstrap
│   ├── scaffold/     new/add logic (mono, micro, domain, infra, rpc, bff)
│   ├── doctor/       static scanner
│   ├── manifest/     .ncgo/manifest.yaml read/write
│   ├── ai/           AGENTS.md / CLAUDE.md rendering
│   ├── mcp/          MCP stdio server (v0.3)
│   ├── upgrade/      metadata-only lifecycle upgrade/plan MVP
│   ├── extract/      mono-to-micro extraction plan/apply MVP
│   ├── exec/         hz/kitex invocation (replaceable)
│   └── assets/_data  scaffold templates (mono / domain / optional infra)
│                     embedded via `//go:embed all:_data`; placed under
│                     `_data/` so `go build ./...` ignores the
│                     `optional/*.go` template files
├── scripts/          smoke and developer utility scripts
├── testdata/         golden tests
└── docs/             PRD and design notes
```

ncgo is a binary; nothing is exposed under `pkg/`.

## 10. Milestones

| Version | Scope | Estimate |
|---|---|---|
| v0.1 | `new mono --kind hertz`, `add domain`, `add infra`, doctor scans, golden tests | done |
| v0.2 | `new mono --kind kitex`, embedded design docs, `ai sync` | done |
| v0.3 | `new micro`, `add rpc`, `add bff`, `mcp serve`, anchor system | done (MVP) |
| v0.4 | `extract`, `upgrade` | done (plan/apply-copy + metadata/plan MVP) |
| v0.5 | ~~NATS~~ / ~~Mongo~~ / ~~MinIO~~ / LoongSuite Go Agent observability optional | done (LoongSuite MVP; others deferred) |

### v0.5 LoongSuite Go Agent Observability Optional Plan

The `observability_otel` optional is implemented as an `add infra` optional for
Alibaba LoongSuite Go Agent, not as generator hard-coded Go strings. The source
of truth is an embedded asset template under:

```
internal/assets/_data/optional/observability_otel.go
```

`ncgo add infra observability_otel --root .` copies that template into the
target project at:

```
internal/base/observability/otel.go
```

The optional is common to Hertz and Kitex projects. `otel` is accepted as an
alias, but the manifest records the canonical kind:

```yaml
infra: [observability_otel]
```

MVP scope:

- Provide `OTEL_*` environment helpers for binaries built by LoongSuite `otel`.
- Print setup next steps; do not install LoongSuite automatically.
- Do not run `go get`; LoongSuite performs compile-time auto-instrumentation.
- Do not rewrite `main.go`, Hertz middleware, or Kitex middleware automatically.

Generated API shape:

```go
type LoongSuiteConfig struct {
    ServiceName string
    Endpoint    string
    // additional OTEL_* fields omitted
}

func DefaultLoongSuiteConfig(serviceName string) LoongSuiteConfig
func (c LoongSuiteConfig) Env() map[string]string
```

CLI next steps should include:

```
curl -fsSL https://cdn.jsdelivr.net/gh/alibaba/loongsuite-go-agent@main/install.sh | sudo bash
otel version
otel go build ./...
OTEL_SERVICE_NAME=<service> OTEL_EXPORTER_OTLP_ENDPOINT=http://127.0.0.1:4318 ./<your-binary>
```

Tests:

- `observability_otel` works for Hertz and Kitex manifests.
- `otel` alias writes the same file and records `observability_otel`.
- Output path is `internal/base/observability/otel.go`.
- `--force` overwrites the file.
- Repeated add deduplicates `manifest.infra`.

## 11. Risks

- `hz`/`kitex` upstream drift → embed snapshot + pinned CI + drift report.
- Template evolution → bump `internal/assets/_data/VERSION` with the change;
  release notes track `assets_version`; `upgrade` shows diff.
- nc-skills rules churn → isolated to `Rule:` anchors in doctor output.
- MCP protocol churn → isolated adapter package, stable subset only.
- Misuse of `extract` → `--apply` only copies into an existing Kitex target, rewrites domain-local imports, and refuses to overwrite target files.

## 12. Non-goals

- Runtime governance (rate limit / circuit breaker live in generated code).
- Embedded LLM or LLM API calls.
- IDE plugins.
- Public Go API under `pkg/`.
