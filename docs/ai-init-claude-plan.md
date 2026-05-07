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
- `.claude/skills/write-tests.md`
- `.claude/agents/planner.md`
- `.claude/agents/implementer.md`
- `.claude/agents/reviewer.md`
- `.claude/agents/debugger.md`
- `.claude/agents/doc-writer.md`
- `.claude/commands/plan.md`
- `.claude/commands/implement-change.md`
- `.claude/commands/fix-failing-test.md`
- `.claude/commands/update-docs.md`
- `.claude/commands/review-diff.md`
- `.claude/hooks/README.md`

Rationale:

- `skills/*` can provide reusable workflow recipes without pretending they are generated facts
- starter workflow files should stay project-generic and rely on `.claude/generated/project-context.md`, `.ncgo/manifest.yaml`, and `ncgo.workspace` for project-specific facts
- `agents/*` can provide role starter templates that teams may edit after bootstrap
- `commands/*` can provide reusable prompt entry points for planning, review, and validation
- the command set should cover the main collaboration chain: `planner -> implementer -> reviewer`, plus debugger/doc-writer side paths
- `.claude/hooks/README.md` documents hooks conservatively without enabling behavior by default

Agent starter files under `.claude/agents/*` SHOULD use the Claude Code custom
subagent format so they are immediately dispatchable after bootstrap. Each file
should begin with YAML frontmatter containing at least:

- `name`
- `description`
- `tools`

`description` SHOULD be written in task language such as `Use when ...` so the
role is discoverable by Claude Code. This requirement applies both to the
current `implementer` / `reviewer` starter roles and to any future optional
roles added to the `team` preset.

Starter workflow files should remain project-generic, but `ai init claude` MAY
detect whether the target root looks like a mono service (`.ncgo/manifest.yaml`)
or a micro workspace (`ncgo.workspace`) and render a small amount of
shape-specific guidance into `.claude/README.md`.

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
- `internal/assets/_data/claude/skills/write-tests.md`
- `internal/assets/_data/claude/agents/planner.md`
- `internal/assets/_data/claude/agents/implementer.md`
- `internal/assets/_data/claude/agents/reviewer.md`
- `internal/assets/_data/claude/agents/debugger.md`
- `internal/assets/_data/claude/agents/doc-writer.md`
- `internal/assets/_data/claude/commands/plan.md`
- `internal/assets/_data/claude/commands/implement-change.md`
- `internal/assets/_data/claude/commands/fix-failing-test.md`
- `internal/assets/_data/claude/commands/update-docs.md`
- `internal/assets/_data/claude/commands/review-diff.md`
- `internal/assets/_data/claude/hooks/README.md`

Any embedded starter asset under `internal/assets/_data/claude/agents/*` SHOULD
already include the required YAML frontmatter so `ncgo ai init claude --preset
team` produces Claude Code-compatible subagents without manual edits.

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

## 11. Mapping `cc-skills-golang` into Starter Files

The `team` preset now has enough surface area (`.claude/rules/*`,
`.claude/skills/*`, `.claude/agents/*`, `.claude/commands/*`) that we should
be explicit about how it relates to external Go skill libraries such as
`cc-skills-golang`.

This section defines the intended integration model.

### 11.1 Goal

The goal is **not** to copy full external `SKILL.md` files into generated
repositories.

The goal is to:

- keep starter files lightweight and repository-generic
- distill a small set of stable, high-frequency Go rules into repo-owned docs
- strengthen the `team` preset's workflow files and subagents
- leave deep, stack-specific knowledge available as external, on-demand skills

In other words:

- `ai init claude` should generate starter policy and workflow files
- `ai sync` should keep generating deterministic project facts
- external Go skills should remain the deep knowledge layer, not the default
  starter content

### 11.2 Selection Criteria

A Go skill is a good candidate for starter-file distillation only when it is:

- common across most generated Go service repositories
- stable enough to become repository policy or workflow guidance
- useful to multiple agents (`implementer`, `reviewer`, `debugger`)
- lightweight enough that it can be reduced to a few bullets or a small
  checklist
- not tightly coupled to a specific framework or optional library

If a skill depends heavily on a particular stack choice, it should stay
external by default and be referenced only when the generated repository
actually uses that stack.

### 11.3 Skills Worth Distilling by Default

The best default candidates are the broad Go service skills:

- `golang-code-style`
- `golang-naming`
- `golang-error-handling`
- `golang-testing`
- `golang-safety`
- `golang-project-layout`
- `golang-context`
- `golang-documentation`

These are the right foundation for repo-owned starter files because they map
cleanly to:

- coding rules
- testing workflow
- review questions
- debug discipline
- doc-sync expectations

### 11.4 Skills Better Applied to Reviewer / Debugger

Some skills are valuable, but should strengthen specific roles rather than the
global Go rules file:

- `golang-database`
- `golang-security`
- `golang-observability`
- `golang-troubleshooting`

These are better expressed as:

- reviewer checklists
- debugger workflow steps
- targeted validation prompts

rather than as always-on repository policy.

### 11.5 Optional, Stack-Specific Skills

The following skills are reasonable optional extensions, but SHOULD only be
distilled when the generated repository clearly uses the relevant stack:

- `golang-swagger`
- `golang-concurrency`
- `golang-dependency-injection`
- `golang-samber-do`
- `golang-stretchr-testify`

Examples:

- If a generated HTTP/BFF repository owns Swagger/OpenAPI docs, distilling a
  small amount of `golang-swagger` guidance is reasonable.
- If a generated repository standardizes on `samber/do`, a small amount of
  `golang-samber-do` guidance is reasonable.
- If the repository uses `testify` heavily, a small amount of
  `golang-stretchr-testify` guidance is reasonable.

These should remain conditional. They are not good global defaults for every
generated repository.

### 11.6 Skills That SHOULD NOT Be Default Starter Content

The following skills are generally too stack-specific or more appropriate for
`ncgo` itself than for all generated repositories:

- `golang-cli`
- `golang-spf13-cobra`
- `golang-spf13-viper`
- `golang-google-wire`
- `golang-uber-dig`
- `golang-uber-fx`
- `golang-graphql`
- `golang-grpc`
- `golang-samber-lo`
- `golang-samber-mo`
- `golang-samber-oops`
- `golang-samber-ro`
- `golang-samber-slog`

Reasons include:

- the skill primarily helps `ncgo`'s own CLI codebase rather than generated
  services
- the skill assumes a specific library or application framework
- the skill is better triggered on demand than embedded in repository policy

### 11.7 File-Level Mapping for `--preset team`

The recommended distillation map is:

| Starter file | Best external skill inputs | Intended result |
| --- | --- | --- |
| `.claude/rules/go.md` | `code-style`, `naming`, `error-handling`, `safety`, `project-layout`, `context`, `documentation` | small, always-on Go repository rules |
| `.claude/skills/write-tests.md` | `testing`, optional `stretchr-testify` | test scope, golden handling, assertion discipline |
| `.claude/skills/run-validation.md` | `testing`, `troubleshooting` | validation order and feedback-loop discipline |
| `.claude/skills/doc-sync.md` | `documentation`, optional `swagger` | user-facing doc sync and worked-example updates |
| `.claude/skills/plan-change.md` | `project-layout`, `testing`, `documentation` | affected-surface planning and test/doc awareness |
| `.claude/agents/implementer.md` | `code-style`, `error-handling`, `context`, `project-layout` | smallest-safe-edit implementation guidance |
| `.claude/agents/reviewer.md` | `database`, `security`, `observability`, `error-handling`, `context`, `project-layout` | stronger review checklist |
| `.claude/agents/debugger.md` | `troubleshooting`, `testing`, `context`, optional `concurrency` | reproduce-first debugging workflow |
| `.claude/agents/doc-writer.md` | `documentation`, optional `swagger` | doc ownership, pairing, and example accuracy |
| `.claude/commands/*.md` | none directly | keep thin; dispatch workflows only |

`commands/*` SHOULD stay intentionally thin. They are not the right place to
copy Go rules. They should keep their role as entrypoints that select an agent,
name the workflow, and define the expected output shape.

### 11.8 What This Means for `minimal` vs `team`

Recommended split:

- `minimal` keeps only a small, conservative rule layer under
  `.claude/rules/*`
- `team` adds workflow recipes and role templates, then distills more guidance
  into `write-tests`, `run-validation`, `reviewer`, and `debugger`

This means the value of `team` is not just “more files”. It is:

- better planning prompts
- stronger test-writing discipline
- safer review checklists
- more reliable debugging steps
- clearer doc-sync expectations

### 11.9 Starter Files Should Stay Lighter Than Full Skills

Even when a starter file draws from an external Go skill, it SHOULD remain much
shorter and more repository-specific than the upstream skill.

Starter files should answer questions like:

- what must an agent do in *this* repo?
- what should be checked before editing or reviewing here?
- which surfaces are contract-sensitive in this generated repository shape?

Full external skills remain the better place for:

- deeper library guidance
- extended examples
- long troubleshooting playbooks
- benchmark and performance methodology
- stack-specific implementation detail

### 11.10 Recommended First Rollout

The first rollout should be conservative and text-only.

Start by strengthening these embedded assets:

- `internal/assets/_data/claude/rules/go.md`
- `internal/assets/_data/claude/agents/reviewer.md`
- `internal/assets/_data/claude/agents/debugger.md`

Then, if the result is useful and still lightweight, follow with:

- `internal/assets/_data/claude/skills/write-tests.md`
- `internal/assets/_data/claude/skills/run-validation.md`
- `internal/assets/_data/claude/skills/plan-change.md`

This keeps the rollout incremental and avoids overloading the initial starter
set.
