# `ncgo ai init claude` Minimal Plan

This document defines the minimal bootstrap plan for a future
`ncgo ai init claude` command.

The purpose of this command is different from `ncgo ai sync`:

- `ai init claude` bootstraps hand-authored Claude starter files
- `ai sync` refreshes deterministic generated facts

## 1. Goal

Add an explicit bootstrap command for repositories that want a starter
`.claude/` layout without treating those starter files as generated facts.

Proposed command surface:

- `ncgo ai init claude --root .`

## 2. Why This Should Be a Separate Command

`ncgo ai sync` is intentionally conservative. It should only generate files
that are deterministic, derived from repository state, and safe to overwrite.

Starter files under `.claude/rules/*` are different:

- they define repository policy and workflow
- they are expected to be edited after bootstrap
- they are not purely derivable from `.ncgo/manifest.yaml`

That makes them a better fit for an explicit scaffold/init command than for
`ai sync`.

## 3. Minimal Scope

### In scope

- one new explicit command: `ncgo ai init claude`
- bootstrap a minimal hand-authored `.claude/` starter set
- optionally create a safe local-ownership marker file such as `.claude/local/.gitignore`
- skip existing files by default
- support `--dry-run` and `--force`

### Out of scope

- generating `.claude/generated/*` (handled by `ai sync`)
- default generation of workflow starters under `.claude/skills/*`, `.claude/agents/*`, or `.claude/commands/*`
- default generation of active hook content under `.claude/hooks/*`
- generating user-authored local note content under `.claude/local/*`
- template localization beyond a single default language in v1

## 4. Starter File Sets

### 4.1 Default Minimal Set

The first version should create these files when missing:

- `.claude/README.md`
- `.claude/rules/agent-engineering.md`
- `.claude/rules/go.md`
- `.claude/local/.gitignore` (recommended safe default)

Rationale:

- `.claude/README.md` explains layout, ownership, and precedence
- `agent-engineering.md` defines how agents should work in the repo
- `go.md` defines Go-specific coding constraints for this codebase style
- `.claude/local/.gitignore` makes the user-owned `local/` area explicit without generating personal content

The minimal default SHOULD NOT create workflow starter files under:

- `.claude/skills/*`
- `.claude/agents/*`
- `.claude/commands/*`

### 4.2 Recommended `team` Preset (Supported / Opt-in)

If the command supports an explicit preset such as `--preset team`, the
recommended starter set is:

- `.claude/skills/plan-change.md`
- `.claude/skills/run-validation.md`
- `.claude/skills/doc-sync.md`
- `.claude/agents/implementer.md`
- `.claude/agents/reviewer.md`
- `.claude/commands/plan.md`
- `.claude/commands/fix-failing-test.md`
- `.claude/commands/update-docs.md`
- `.claude/commands/review-diff.md`
- `.claude/hooks/README.md`

Rationale:

- `skills/*` can provide reusable workflow recipes without pretending they are generated facts
- `agents/*` can provide role starter templates that teams may edit after bootstrap
- `commands/*` can provide reusable prompt entry points for planning, review, and validation
- `.claude/hooks/README.md` documents hooks conservatively without enabling behavior by default

### 4.3 Content That SHOULD NOT Be Bootstrapped by Default

The command SHOULD NOT create personal or behaviorful starter content such as:

- `.claude/local/notes.md`
- `.claude/local/prompt.md`
- active hook scripts or hook config under `.claude/hooks/*`

Those are either too workflow-specific, too side-effectful, or explicitly user-owned.

## 5. Command Semantics

Suggested flags:

- `--root`: project root, default `.`
- `--dry-run`: report intended writes without changing files
- `--force`: overwrite existing starter files explicitly
- `--preset`: `minimal|team` (default `minimal`)

Suggested behavior:

1. create the default minimal starter files when they do not exist
2. skip existing files by default
3. overwrite only when `--force` is passed
4. report `wrote ...` / `skipped ...` in the same style as `ai sync`

The `team` preset SHOULD add workflow starter files
for `skills/*`, `agents/*`, and `commands/*`, while still avoiding user-owned
local content and active hooks.

No positional args are needed for the minimal command.

## 6. Ownership and Overwrite Model

Files created by `ai init claude` should become repo-owned after bootstrap.

That means:

- they SHOULD NOT carry the `<!-- ncgo:managed -->` marker
- they SHOULD be reviewed and edited like normal source files
- future `ai sync` runs MUST NOT overwrite them

Recommended overwrite model:

- missing file: write it
- existing file: skip by default
- existing file + `--force`: overwrite explicitly

This keeps the command bootstrap-oriented rather than sync-oriented.

## 7. Relationship to `ai sync`

Recommended flow for repositories that want Claude support:

1. run `ncgo ai init claude` once to get starter `.claude` docs
2. run `ncgo ai sync` repeatedly to refresh generated facts

Recommended responsibility split:

- `ai init claude`: starter policy/layout docs
- `ai sync`: `.claude/generated/project-context.md` and other future generated facts

`ai sync` should remain additive and MUST NOT be upgraded into a command that
rewrites starter policy files.

## 8. Implementation Shape

Suggested command tree addition:

- `ncgo ai init claude`

Reason: `ai init` leaves room for future explicit bootstraps without coupling
them to `ai sync`.

Suggested implementation shape:

- add `newAIInitCmd()` under `internal/cli/ai.go`
- add an `internal/ai` bootstrap helper separate from `Sync`
- store starter templates as embedded assets rather than hard-coded strings

Possible asset layout:

- `internal/assets/_data/claude/README.md`
- `internal/assets/_data/claude/rules/agent-engineering.md`
- `internal/assets/_data/claude/rules/go.md`
- `internal/assets/_data/claude/local/.gitignore`

The `team` preset assets may also include:

- `internal/assets/_data/claude/skills/plan-change.md`
- `internal/assets/_data/claude/skills/run-validation.md`
- `internal/assets/_data/claude/skills/doc-sync.md`
- `internal/assets/_data/claude/agents/implementer.md`
- `internal/assets/_data/claude/agents/reviewer.md`
- `internal/assets/_data/claude/commands/plan.md`
- `internal/assets/_data/claude/commands/fix-failing-test.md`
- `internal/assets/_data/claude/commands/update-docs.md`
- `internal/assets/_data/claude/commands/review-diff.md`
- `internal/assets/_data/claude/hooks/README.md`

## 9. Validation Plan

The first implementation should include focused tests for:

- writing all missing starter files
- skipping existing files by default
- overwriting only with `--force`
- `--dry-run` reporting without writing
- coexistence with `ai sync`

Important compatibility check:

- after `ai init claude`, a later `ai sync` run should still write
  `.claude/generated/project-context.md` without touching starter files

## 10. Deferred Follow-ups

After the minimal command is stable, later phases may consider:

- starter templates for additional agent roles beyond `implementer` and `reviewer`
- optional zh-CN starter variants
- additional opt-in docs for hooks without enabling hook behavior by default

Those should remain opt-in scaffolding features, not part of `ai sync`.
