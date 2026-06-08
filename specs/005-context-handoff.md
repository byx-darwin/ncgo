# ncgo Context Handoff Guide

This document helps new conversations / new Agents rapidly restore ncgo project context.

## Working Directory

Repository path:

```text
/Users/byx/Documents/workspace/github.com/byx-darwin/ncgo
```

## Project Positioning

`ncgo` is an AI-Agent-facing Go microservice scaffold CLI. It includes built-in Hertz / Kitex templates, generates project manifests, invokes `hz` / `kitex`, and renders AI context files for Claude / Cursor / other Agents.

## Current Completion Status

- v0.1: Hertz mono scaffold, `add domain`, `add infra`, `doctor`, golden tests.
- v0.2: Kitex mono scaffold, embedded design docs, `ai sync`.
- v0.3: micro workspace, `add rpc`, `add bff`, anchor system, `mcp serve`.
- v0.4: `upgrade` metadata-only + `--plan`, `extract domain` plan/apply-copy.
- v0.5: LoongSuite Go Agent observability optional MVP.

## Important Implementation Conventions

### Infra optional

The `ncgo add infra` mechanism:

1. Copy Go files from embedded assets.
2. Write to the target project.
3. Update `.ncgo/manifest.yaml` `infra` field.
4. Print dependency or tool setup next steps.
5. Does NOT auto-install dependencies.

### Micro workspace

Root directory: `ncgo.workspace`, `services/`. Service directory: `services/<name>/.ncgo/manifest.yaml`.

### Anchor system

Use case files use:

```go
// ncgo:methods:start
// ncgo:methods:end
```

`ncgo add method` inserts usecase method stubs only within markers.

## Key Files

Recommended for new Agents to review first:

- `README.md`
- `README.zh-CN.md`
- `main.go`
- `internal/cli/add.go`
- `internal/cli/root.go`
- `internal/scaffold/infra/infra.go`
- `internal/mcp/server.go`
- `internal/mcp/tools.go`
- `internal/extract/domain.go`
- `internal/upgrade/upgrade.go`

## Notes

- Do not commit / push unless the user explicitly asks.
- Do not auto-install dependencies unless the user explicitly allows.
- Run tests after code changes.
- Keep NATS / Mongo / MinIO with strikethrough in docs.
- CLI stays thin; core logic in `internal/...`.
- Generated code goes through embedded templates; do not hardcode large business code blocks in generators.
