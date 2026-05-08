# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

**ncgo** is an AI-friendly scaffold CLI for Go microservices. It generates Hertz (HTTP) and Kitex (RPC) service scaffolds, renders AI context files (AGENTS.md, CLAUDE.md, Cursor rules), and exposes operations via both CLI and MCP (Model Context Protocol) stdio server.

## Development Commands

```bash
# Build
go build .
go build ./...

# Test
go test ./... -count=1
go test ./internal/scaffold/mono/... -run TestGenerateGoldenDefault -count=1

# Update golden tests (when templates change)
go test ./internal/scaffold/mono/... -update-golden -count=1

# Lint
gofmt -l $(find . -name '*.go' -not -path './.git/*')
go vet ./...

# Full validation (CI-equivalent)
go build ./... && go build . && go vet ./... && go test ./... -count=1 && ./scripts/smoke.sh

# Pre-commit setup
pre-commit install --install-hooks --hook-type pre-commit --hook-type pre-push
```

**Prerequisites:** Go 1.25+, `hz >= v0.9.7` for Hertz flows, `kitex >= v0.16.1` for Kitex flows.

## Architecture

```
main.go              → cli.Main()
internal/cli/         → Cobra CLI commands: version, new, add, ai, i18n, protolint, doctor, mcp, upgrade, extract
internal/mcp/         → MCP stdio server (JSON-RPC 2.0). Tools in tools.go: ncgo_version, ncgo_doctor, ncgo_ai_*, ncgo_i18n_*, ncgo_protolint, ncgo_add_*
internal/scaffold/    → Scaffold generators
  mono/             → Mono service generation (Hertz/Kitex) with golden tests
  micro/            → Micro workspace generation
  bff/              → BFF (Hertz) service generation
  rpc/              → RPC (Kitex) service generation
  domain/           → Domain usecase/repository generation
  infra/            → Optional infra add-ons (Redis, Kafka, ES, observability, canary, logging)
  method/           → Method stub insertion at ncgo anchors
  shared/           → Shared helpers (container files, docker, precommit)
internal/assets/_data/ → Embedded templates (hertz/, kitex/, optional/, docs/)
internal/manifest/    → Manifest/workspace YAML handling
internal/doctor/      → Diagnostic checks
internal/protolint/   → Proto lint rules
internal/ai/          → AI context sync and init
internal/upgrade/     → Manifest metadata updates
internal/extract/     → Domain extraction (mono-to-micro)
internal/exec/        → External command execution (hz/kitex)
internal/testutil/    → Test helpers (golden tests)
schemas/              → JSON schemas for i18n payloads
```

## Key Contracts

### CLI/MCP/Scaffold surfaces
CLI flags, JSON output, MCP schemas (`content[0].text`, top-level structured fields), scaffold templates, and generated file layouts are **contract-sensitive**. Changes require updating tests and docs together.

### Template handoff ordering
Kitex scaffolds must run `make sqlc` before `go mod tidy`. Hertz needs the same ordering only when `WithDatabase=true`. Changing this sequencing requires updating mono tests and docs.

### Generated files
Do not hand-edit downstream generated project files. Fix templates or generators instead.

## Testing

- **Unit tests**: `*_test.go` alongside code for helpers, pure logic, schema parsing.
- **Integration tests**: CLI commands, MCP tools, multi-package wiring (e.g., `internal/cli/add_test.go`, `internal/mcp/server_test.go`).
- **Golden tests**: `internal/scaffold/mono/golden_test.go` locks scaffold output. Snapshots in `testdata/`. Use `-update-golden` flag when templates change.
- **Smoke tests**: `./scripts/smoke.sh` for end-to-end CLI validation.

## Rules

Hand-authored rules in `.claude/rules/`:
- `go.md` — Go coding style, repository structure, contract-surface guidance.
- `agent-engineering.md` — Execution workflow, validation order, failure handling, risk control.

## Documentation

When commands, install flow, release flow, or generated behavior change, update docs. Keep English and Chinese docs (`README.md` / `README.zh-CN.md`, `docs/examples.md` / `docs/examples.zh-CN.md`) aligned.
