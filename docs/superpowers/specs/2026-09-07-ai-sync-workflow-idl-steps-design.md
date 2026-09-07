# Design: restore IDL/handler steps in the ncgo-dev workflow doc

- Issue: #116 — fix(ai-sync): rendered CLAUDE.md drops Workflow steps 3-5
- Classification: bounded (existing doc/render flow, no new abstractions)
- Date: 2026-09-07

## Context

`ncgo ai sync` renders `CLAUDE.md`, `AGENTS.md`, and
`.claude/skills/ncgo-dev/SKILL.md` from a single shared embedded doc,
`internal/assets/_data/docs/ai/ncgo-dev-workflow.{en,zh-CN}.md`
(`internal/ai/render.go:60`, `renderClaude`/`renderAgents`/`renderNcgoDevSkill`).

Issue #116 reports that the rendered "Implementing a Feature with ncgo"
section is missing steps 3-5 (define the proto IDL, regenerate code via
`make update`, implement the handler/usecase) on Kitex-based downstream
services.

## Root cause

Not a rendering bug — `render.go` copies the doc body verbatim. The source
doc itself never described the IDL-driven codegen path. It only covered
`add domain → add method → make sqlc → verify → check → sync` (6 steps),
which is usecase-layer only. Both the Hertz and Kitex Makefile templates
have a `make update` target (`hz update` / `kitex ...`) that regenerates
the handler stub from the IDL — this path was never documented.

## Decision

Insert three new steps between "add usecase method" and "regenerate
database code" in both `ncgo-dev-workflow.en.md` and
`ncgo-dev-workflow.zh-CN.md`:

1. Update the IDL (`idl/<service>.proto`)
2. Regenerate code from IDL (`make update`) — applies to both Hertz
   (`hz update`) and Kitex (`kitex` generator); skippable when a change is
   usecase-only
3. Implement the handler — wire the generated handler to the usecase,
   per the `handler/* → usecase/*` layer rule

Total workflow steps: 9. Verification Checklist and Failure Handling
sections get matching additions (IDL/`make update` checklist items and
failure entry). No change to `internal/ai/render.go` — this is a
doc-content-only fix.

## Testing

`internal/ai/sync_test.go`: extend `TestSync...` assertions on the
rendered `SKILL.md` and `CLAUDE.md`/`AGENTS.md` bodies to require
`"make update"` is present, guarding against a future silent drop of this
step (this is the second time this class of doc-content gap has surfaced,
per the issue-45 follow-up note).

## Out of scope

- No change to `render.go` dispatch logic (doc is still shared across
  Hertz/Kitex; both frameworks have `make update`, so no per-kind branch
  is needed).
- No change to `ncgo-dev-rules.*.md` or other embedded docs.
