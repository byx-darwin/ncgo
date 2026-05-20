# ncgo Architecture Design

## Overview

ncgo is a Go microservice scaffold CLI that generates Hertz (HTTP) and Kitex (RPC) service scaffolds, renders AI context files (AGENTS.md, CLAUDE.md, Cursor rules), and exposes operations via both CLI and MCP (Model Context Protocol) stdio server.

## System Architecture

```
┌─────────────────────────────────────────────────────────┐
│                    Entry Point                          │
│                      main.go                            │
│                     cli.Main()                          │
└────────────────────────┬────────────────────────────────┘
                         │
┌────────────────────────▼────────────────────────────────┐
│                   CLI Layer                             │
│              internal/cli/ (Cobra)                      │
│                                                         │
│  Commands: version, new, add, ai, i18n, protolint,     │
│            doctor, mcp, upgrade, extract, export, test  │
│                                                         │
│  Surfaces: flags, JSON output, help text, exit codes    │
└────────┬────────────┬──────────────┬────────────────────┘
         │            │              │
┌────────▼──┐  ┌──────▼──────┐ ┌─────▼───────────┐
│  MCP       │  │  Scaffold    │ │  AI / i18n /    │
│  Server    │  │  Generator   │ │  Other Tools    │
│            │  │              │ │                  │
│ server.go  │  │ mono/        │ │ ai/ sync.go     │
│ tools.go   │  │ micro/       │ │ i18n/           │
│            │  │ bff/         │ │ doctor/         │
│ 12 tools   │  │ rpc/         │ │ protolint/      │
│            │  │ domain/      │ │ upgrade/        │
│            │  │ infra/       │ │ extract/        │
│            │  │ method/      │ │ exec/           │
│            │  │ shared/      │ │ testutil/       │
└─────┬──────┘  └──────┬───────┘ └──────┬───────────┘
      │                │                 │
      │         ┌──────▼──────────┐      │
      │         │  Templates       │      │
      │         │  embedded        │      │
      │         │                  │      │
      │         │  assets/_data/   │      │
      │         │    hertz/        │      │
      │         │    kitex/        │      │
      │         │    optional/     │      │
      │         │    docs/         │      │
      │         └──────────────────┘      │
      │                                   │
      │         ┌──────────────────┐      │
      │         │  Manifest        │      │
      │         │  .ncgo/          │      │
      │         │  manifest.yaml   │      │
      │         │  Single source   │      │
      │         │  of truth        │      │
      │         └──────────────────┘      │
      │                                   │
      └───────────┬───────────────────────┘
                  │
┌─────────────────▼───────────────────────────────────────┐
│                  Generated Output                        │
│                                                          │
│  Hertz services (HTTP)                                   │
│  Kitex services (RPC)                                    │
│  AI context files (AGENTS.md, CLAUDE.md, .cursorrules)  │
│  Docker, pre-commit, CI configs                          │
└──────────────────────────────────────────────────────────┘
```

## Layer Details

### 1. CLI Layer (`internal/cli/`)

- **Framework**: Cobra
- **Root command**: `ncgo`
- **Entry point**: `main.go → cli.Main()`
- **Key constructs**:
  - `newRootCmd()` - registers all subcommands
  - `newOptions` struct - shared flags: `--module`, `--mode`, `--kind`, `--db`, `--infra`, `--preset`, `--idl`, `--dir`, `--no-generate`, `--rule-center-addr`
  - `runNewMono()` / `runNewMicro()` - orchestrates scaffold generation

### 2. MCP Layer (`internal/mcp/`)

- **Protocol**: JSON-RPC 2.0 with Content-Length framing
- **Server**: `server.go` handles `initialize`, `tools/list`, `tools/call`
- **Tools** (12 total, `tools.go`):
  - `ncgo_version`, `ncgo_doctor`, `ncgo_new`, `ncgo_add_domain`
  - `ncgo_ai_init_claude`, `ncgo_ai_sync`
  - `ncgo_i18n_report`, `ncgo_i18n_check`
  - `ncgo_protolint`, `ncgo_add_infra`, `ncgo_add_method`, `ncgo_add_rule_center`
- **Contract**: `content[0].text` for human-readable output, top-level JSON fields for agent consumption

### 3. Manifest (`internal/manifest/`)

- **Schema**: `.ncgo/manifest.yaml`
- **Struct**: `Ncgo` (Meta) → `Mode`, `Module`, `Service`, `Infra`, `Domains`, `GeneratedAt`
- **Validation**: mode (`mono`|`micro`), module (valid Go path), service.name required, service.kind (`hertz`|`kitex`)
- **Write pattern**: atomic via tmp file + rename

### 4. Scaffold Generators (`internal/scaffold/`)

| Package | Purpose |
|---------|---------|
| `mono/` | Single-service generation (Hertz/Kitex) with golden tests |
| `micro/` | Multi-service workspace generation |
| `bff/` | BFF (Hertz) service generation |
| `rpc/` | RPC (Kitex) service generation |
| `domain/` | Domain usecase/repository generation |
| `infra/` | Optional infra add-ons (Redis, Kafka, ES, observability, canary, logging) |
| `method/` | Method stub insertion at ncgo anchors |
| `shared/` | Shared helpers (container files, docker, precommit) |

### 5. Templates (`internal/assets/_data/`)

- **Embedding**: `//go:embed all:_data` in `assets.go`
- **Categories**:
  - `hertz/` - HTTP service templates
  - `kitex/` - RPC service templates
  - `optional/` - Infra add-on templates
  - `docs/` - Design docs for generated projects
- **Rendering**: Go `text/template` with FuncMap (`ToLower`, `ToUpper`, `LowerFirst`, `exportName`)
- **Versioning**: `VERSION` file with `ncgo_assets_version` marker

### 6. AI Context (`internal/ai/`)

- **Managed marker**: `<!-- ncgo:managed -->` for file ownership tracking
- **Sync**: Renders all managed AI artifacts from manifest + design docs
- **Source resolution**: Detects `.ncgo/manifest.yaml` or workspace, loads design doc
- **Write strategy**: Honors managed-marker / Force / DryRun rules
- **Result**: Tracks `Written`, `Skipped`, `Notes`, `NextSteps`, `Scope`, `SourceRef`

## Key Contracts

### CLI/MCP/Scaffold Surfaces
CLI flags, JSON output, MCP schemas (`content[0].text`, top-level structured fields), scaffold templates, and generated file layouts are contract-sensitive. Changes require updating tests and docs together.

### Template Handoff Ordering
Kitex scaffolds must run `make sqlc` before `go mod tidy`. Hertz needs the same ordering only when `WithDatabase=true`.

### Generated Files
Do not hand-edit downstream generated project files. Fix templates or generators instead.

## Testing Strategy

| Layer | Location | Pattern |
|-------|----------|---------|
| Unit | `*_test.go` alongside code | Helpers, pure logic, schema parsing |
| Integration | `internal/cli/add_test.go`, `internal/mcp/server_test.go` | CLI commands, MCP tools, wiring |
| Golden | `internal/scaffold/mono/golden_test.go` | Snapshot-based, `testdata/`, `-update-golden` |
| Smoke | `./scripts/smoke.sh` | End-to-end CLI validation |

## Prerequisites

- Go 1.25+
- `hz >= v0.9.7` for Hertz flows
- `kitex >= v0.16.1` for Kitex flows
- `pre-commit` for git hooks
