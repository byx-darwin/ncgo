# ncgo Roadmap

This document records completed capabilities, outstanding tasks, and suggested priorities as a roadmap for future development.

## 1. Completed

### 1.1 Core Capabilities

- Initial repo commit: `feat: initialize ncgo scaffold CLI`.
- Hertz / Kitex mono scaffold, micro workspace, add rpc / bff / domain / method, doctor, upgrade, extract domain, MCP server.

### 1.2 Optional Infrastructure

- Supported: `redis`, `kafka`, `es`, `clickhouse`, `registry_etcd`.
- LoongSuite Go Agent `observability_otel` / `otel`.
- `observability_logging` / `logging`: slog, console/file/both/none, lumberjack rotate + gzip, category routing, `samber/oops` structured logging, request/trace/release/canary fields, Hertz/Kitex adapters.
- `release_canary` / `canary`: release metadata, traffic context, Hertz header adapter, Kitex metadata adapter, canary rules, Nacos/Polaris provider identifiers, `Discoverer`/`RuleProvider`/`Selector` abstractions, stable/canary pools, weighted/sticky selection, `fallback=stable|fail_fast`.

### 1.3 Wiring / Preview / Plan

- `ncgo add infra logging --wire` and `ncgo add infra canary --wire`.
- `--dry-run` support: no optional files, no manifest save, no source code modification.
- `--output json` for machine-readable results.
- `--plan` as shorthand for `--dry-run --output json`.
- `infra.Result.Plan` with detailed operation-level actions.
- Default Hertz/Kitex templates with `// ncgo:wire:*` markers.
- MCP `ncgo_add_infra` returns structured fields: `dryRun`, `updated`, `writtenPaths`, `wiredPaths`, `nextSteps`, `plan`.

## 2. Recommended Priorities

### P0: Refine Wiring Plan Detail

Continue adding `from`/`to`/`insertAfter` to wire operation-level plans for more precise patch previews.

### P0: Improve Template Wiring Markers

- Extend markers to reduce fragile string replacement on source code.
- Make marker helpers more generic for reuse in registry/selector/otel wiring.

### P1: Nacos / Polaris SDK Adapter

- SDK-neutral adapter seam/skeleton completed.
- Subsequent adapters need only provide `ListInstances`/`LoadRules` callbacks.

### P2: Extend Plan to Other Add Subcommands

Completed: `ncgo add domain --plan`, `ncgo add rpc --plan`, `ncgo add bff --plan`. Shared `internal/scaffold/plan.Item` schema extracted.

### P2: CI / Release Engineering

Completed: GitHub Actions CI (gofmt, vet, test, build, smoke), tag-driven Release workflow (cross-platform binaries, checksums.txt, GitHub Release), release process docs.

## 3. Suggested Next Steps

CI/release engineering is complete. Next steps: real SDK adapters, more plan patch detail, or generated project release metadata injection.
