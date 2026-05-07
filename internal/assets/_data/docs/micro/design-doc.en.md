# Micro Workspace Design Doc

Audience: ncgo maintainers and AI agents that read or refresh AI context for a
repository root created by `ncgo new --mode micro`.

For service-level architecture details see the embedded Hertz and Kitex design
docs under `docs/hertz/` and `docs/kitex/`.

Dynamic rate-limit behavior is defined by each service template rather than by
the micro workspace profile itself. The current dedicated topic exists only for
Hertz HTTP services:
[`docs/hertz/rate-limit-dynamic-design.en.md`](../hertz/rate-limit-dynamic-design.en.md).

## 1. Overview

The micro workspace profile backs repository roots created by `ncgo new --mode micro`.
The root centers on `ncgo.workspace`, shared `.claude` / `.cursor` context,
repository hooks, and aggregate commands such as `ncgo doctor --root .` and
`ncgo protolint --root .`.

Deployable units live under `services/`. Each generated BFF or RPC service keeps
its own `.ncgo/manifest.yaml`, module path, IDL, and generator-owned template
tree. Workspace-level AI context should describe the repository shape and
service inventory, not replace per-service context.

## 2. Workspace Responsibilities

- root metadata lives in `ncgo.workspace`
- services are registered by `name`, `kind`, and `dir`
- hand-authored `.claude/*` rules belong at the workspace root
- `AGENTS.local.md` may hold local notes for the workspace root only

## 3. Service Responsibilities

- add services with `ncgo add rpc <name>` or `ncgo add bff <name>`
- each service keeps its own `.ncgo/manifest.yaml`
- run `ncgo ai sync --root services/<name>` when you need service-level AI context
- keep most edits scoped to one service unless the task explicitly crosses service boundaries

## 4. Validation and Agent Workflow

- run aggregate discovery commands from the workspace root when checking the service inventory or proto contracts
- run build, test, generator, and service-specific validation commands inside the target service directory unless the task is repository-wide
- when a change touches multiple services, call out which service owns each edit and validate the touched services separately when possible
