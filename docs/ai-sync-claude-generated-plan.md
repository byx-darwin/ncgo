# ai sync → `.claude/generated/project-context.md` Plan

This document defines the first `.claude` integration step for `ncgo ai sync`.
The goal is intentionally narrow: add one generated Claude-facing project-facts file
without changing the ownership model of hand-authored rules, skills, hooks,
agents, or commands.

## 1. Goal

Future `ncgo ai sync` MAY add exactly one new generated file:

- `.claude/generated/project-context.md`

This file should be additive. It should not replace current outputs:

- `AGENTS.md`
- `CLAUDE.md`
- `.cursor/rules/ncgo.mdc`

## 2. Why this is the first step

This is the safest `.claude` integration because it only adds deterministic
project facts. It does not attempt to generate repository policy, workflow
definitions, role boundaries, or personal local notes.

## 3. Scope

### In scope

- one generated file: `.claude/generated/project-context.md`
- deterministic content derived from repository state
- reuse of current `ai sync` ownership rules
- additive relationship to existing AI context files

### Out of scope

- generating `.claude/rules/*`
- generating `.claude/skills/*`, `.claude/hooks/*`, `.claude/agents/*`, or `.claude/commands/*`
- replacing `CLAUDE.md`
- AST-derived architecture scanning beyond current manifest + embedded docs inputs
- personal overlays inside `.claude/local/*`

## 4. Inputs

The first version should use only inputs that current `ai sync` already trusts:

- `.ncgo/manifest.yaml`
- embedded design doc matching the service kind and selected language

The first version SHOULD NOT consume local overlays such as:

- `AGENTS.local.md`
- `.claude/local/*`

Reason: `.claude/generated/project-context.md` should remain deterministic and
tool-owned.

## 5. Output Shape

The file should carry the standard managed marker and stay markdown-readable.

Recommended sections:

1. `# Claude Project Context`
2. `## Project Facts`
   - module
   - service name
   - service kind
   - mode
   - service IDL when present
   - infra list
   - domain list
   - ncgo/assets version
3. `## Architecture & Built-in Features`
   - short embedded design-doc summary or body excerpt
4. `## Repository Rules`
   - links to `.claude/rules/go.md`
   - links to `.claude/rules/agent-engineering.md`
5. `## Notes`
   - generated file disclaimer
   - pointer to `.claude/local/*` for personal overlays

The file should emphasize facts and stable references, not long policy text.

## 6. Ownership and Overwrite Rules

The file should follow the same overwrite model as current `ai sync` targets:

- it MUST carry `<!-- ncgo:managed -->`
- existing files without the marker MUST be skipped unless `--force` is used
- `--dry-run` MUST report intended writes without modifying files

The file belongs to the generated/tool-owned layer:

- `.claude/generated/project-context.md`

It MUST NOT overwrite or merge into hand-authored files under:

- `.claude/rules/*`
- `.claude/skills/*`
- `.claude/hooks/*`
- `.claude/agents/*`
- `.claude/commands/*`
- `.claude/local/*`

## 7. Relationship to Current Files

`AGENTS.md` and `CLAUDE.md` remain the long-form cross-tool context files.

`.claude/generated/project-context.md` should instead be the short,
Claude-facing, generated fact sheet.

Recommended interpretation:

- `AGENTS.md`: broad multi-agent context
- `CLAUDE.md`: long-form Claude context
- `.claude/generated/project-context.md`: short generated project facts
- `.claude/rules/*`: hand-authored repository policy
- `.claude/local/*`: personal local overlays

## 8. CLI / MCP Behavior Expectations

The first implementation SHOULD keep current command and MCP contracts stable.

- `ncgo ai sync` CLI still reports written/skipped files
- `ncgo_ai_sync` MCP can continue returning text-only output
- the new file simply appears in the written/skipped result when applicable

No new flag is required for the first step.

## 9. Suggested Implementation Order

1. add a new render target for `.claude/generated/project-context.md`
2. keep content derived only from current manifest + embedded docs
3. reuse existing managed-marker, `--force`, and `--dry-run` behavior
4. add focused unit tests for the new render target
5. update CLI/MCP tests only as needed for the extra written path

## 10. Future Steps (Deferred)

After the first step is stable, later phases may add:

- `.claude/generated/manifest-summary.md`
- `.claude/generated/architecture.md` from AST-derived project scanning
- explicit scaffold commands for starter `.claude/skills/*` or `.claude/commands/*`

Those should remain separate from `ai sync` unless they are still purely derived
and safely overwriteable.
