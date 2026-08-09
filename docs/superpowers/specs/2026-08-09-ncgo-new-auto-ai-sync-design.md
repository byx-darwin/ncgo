# Design: ncgo new Auto Post-Generation Steps

**Issue:** #58
**Date:** 2026-08-09
**Status:** Draft

## Summary

After `ncgo new --mode mono` successfully generates a project, automatically execute safe post-generation steps (`go mod tidy` + `ncgo ai sync`) so the generated project is immediately usable by AI agents. Runtime-dependent steps (`make sqlc`, `make migrate-up`, `make dev`) remain as manual NextSteps hints.

## Goals

1. `ncgo new` produces a project with AI context files (`.claude/`) already rendered.
2. `go mod tidy` runs automatically after generator success, so dependencies are resolved.
3. Failures in auto steps are non-blocking warnings — the scaffold is complete regardless.
4. Users can override or disable auto steps via flags.
5. Both CLI and MCP surfaces exhibit identical auto-step behavior.

## Non-Goals

- Auto-executing runtime-dependent commands (`make sqlc`, `make migrate-up`, `make dev`).
- Auto-executing steps in `--no-generate` mode (project not fully generated).
- Changing `ai sync` semantics (skip/force behavior remains unchanged).
- Auto-steps for `ncgo new --mode micro` (micro workspace has no Go module to tidy).

## Architecture

### New Package: `internal/postgenerate`

A new package encapsulates post-generation auto-step logic. Both CLI (`internal/cli/root.go`) and MCP (`internal/mcp/tool_new.go`) call into it after `mono.Generate` succeeds.

```
internal/postgenerate/
├── postgenerate.go       # Run(), Options, Result, StepResult
├── postgenerate_test.go  # Unit tests with fake exec.Runner
└── steps.go              # goModTidy(), aiSync() implementations
```

### Interface

```go
package postgenerate

// Options configures post-generation auto-step execution.
type Options struct {
    Dir         string      // absolute project root
    AITarget    string      // "claude" (default) | "all" | "agents" | "cursor" | "none"
    NoAutoSteps bool        // skip all auto steps
    RanGenerate bool        // whether generator (hz/kitex) ran successfully
    Runner      exec.Runner // injected exec; nil = exec.NewDefault()
    Stdout      io.Writer   // progress and warning output
}

// Result reports the outcome of each auto step.
type Result struct {
    Steps []StepResult
}

// StepResult describes one auto step's outcome.
type StepResult struct {
    Name   string // "go mod tidy" | "ai sync"
    Status string // "skipped" | "succeeded" | "failed"
    Detail string // human-readable reason, timing, or error message
}

// Run executes post-generation auto steps. It never returns an error for
// step failures; failures are recorded in Result.Steps and written as
// warnings to opts.Stdout.
func Run(opts Options) *Result
```

### Execution Rules

| Condition | Behavior |
|-----------|----------|
| `NoAutoSteps == true` | All steps skipped |
| `RanGenerate == false` | All steps skipped (no go.mod exists) |
| `AITarget == "none"` | ai sync step skipped; go mod tidy still runs |
| `AITarget == ""` | Defaults to `"claude"` |
| Step failure | Recorded as `"failed"`, warning to Stdout, next step still runs |

### Steps (executed in order)

1. **`goModTidy`** — Runs `go mod tidy` in `opts.Dir` via `exec.Runner`. Must run first because `ai.Sync` may need resolved module dependencies.
2. **`aiSync`** — Calls `ai.Sync(ai.Options{Root: opts.Dir, Target: opts.AITarget})` directly (pure Go, no exec). Renders AI context files (CLAUDE.md, etc.) using existing skip/force semantics.

### Output Format

```
running auto steps...
  ✓ go mod tidy (0.8s)
  ✓ ai sync --target claude (3 files written)
```

On failure:
```
running auto steps...
  ⚠ go mod tidy failed: exit status 1 (non-blocking)
  ✓ ai sync --target claude (3 files written)
```

## CLI Integration

### New Flags

```
--ai-target string    AI sync target: claude (default) | all | agents | cursor | none
--no-auto-steps       Skip automatic post-generation steps (go mod tidy, ai sync)
```

### Changes to `internal/cli/root.go`

Add fields to `newOptions`:
```go
aiTarget    string
noAutoSteps bool
```

Register flags in `newNewCmd()`:
```go
f.StringVar(&opts.aiTarget, "ai-target", "claude", "AI sync target: claude | all | agents | cursor | none")
f.BoolVar(&opts.noAutoSteps, "no-auto-steps", false, "Skip automatic post-generation steps")
```

After `mono.Generate` succeeds in `runNewMono()`:
```go
if res.RanGenerate {
    pgResult := postgenerate.Run(postgenerate.Options{
        Dir:         res.Dir,
        AITarget:    opts.aiTarget,
        NoAutoSteps: opts.noAutoSteps,
        RanGenerate: res.RanGenerate,
        Stdout:      out,
    })
    _ = pgResult // results already printed to stdout
}
```

### NextSteps Changes

`postGenerateNextSteps` in `internal/scaffold/mono/files.go` currently includes `go mod tidy` and `ncgo ai sync --target all --root .` in its output. Rather than changing `mono.Generate`'s signature to pass an `autoStepsRan` flag, the **CLI/MCP layer filters NextSteps** after `postgenerate.Run` completes:

- If auto steps ran successfully, remove `"go mod tidy"` and `"ncgo ai sync ..."` from `res.NextSteps` before printing.
- If auto steps were skipped or failed, keep the full manual list.

This keeps `mono.Generate` and `postGenerateNextSteps` unchanged — the filtering is a thin CLI/MCP concern.

**When auto steps ran (default, generator succeeded):**
```
cd <dir>
[make sqlc]           ← if requiresSQLCBeforeTidy
[make migrate-up]     ← if WithDatabase
make dev
```

**When auto steps did NOT run (`--no-auto-steps` or `--no-generate`):**
```
cd <dir>
[make sqlc]
go mod tidy           ← manual hint restored
[make migrate-up]
make dev
ncgo ai sync --target all --root .   ← manual hint restored
```

## MCP Integration

### Changes to `internal/mcp/tool_new.go`

Add fields to the args struct in `callNew`:
```go
AITarget    string `json:"aiTarget"`
NoAutoSteps bool   `json:"noAutoSteps"`
```

After `runNewMono` succeeds, call `postgenerate.Run` same as CLI.

Add `AutoSteps` field to `newResult`:
```go
type newResult struct {
    Dir         string
    NextSteps   []string
    Mode        string
    RanGenerate *bool
    AutoSteps   []postgenerate.StepResult `json:",omitempty"`
}
```

## Changes to Existing Files

| File | Change |
|------|--------|
| `internal/postgenerate/postgenerate.go` | New file: `Run`, `Options`, `Result`, `StepResult` |
| `internal/postgenerate/steps.go` | New file: `goModTidy`, `aiSync` step implementations |
| `internal/postgenerate/postgenerate_test.go` | New file: unit tests |
| `internal/cli/root.go` | Add `--ai-target`, `--no-auto-steps` flags; call `postgenerate.Run` after `mono.Generate` |
| `internal/mcp/tool_new.go` | Add `aiTarget`, `noAutoSteps` args; call `postgenerate.Run`; add `AutoSteps` to result |
| `internal/scaffold/mono/files.go` | No changes — NextSteps filtering handled by CLI/MCP layer |
| `internal/scaffold/mono/mono.go` | No changes — `mono.Generate` stays unchanged |
| `internal/exec/` | Add `GoModTidy` helper (or inline command construction) |

## Testing Strategy

### Unit Tests (`internal/postgenerate/postgenerate_test.go`)

| Test | Scenario | Expected |
|------|----------|----------|
| `TestRun_NoAutoSteps` | `NoAutoSteps: true` | All steps skipped |
| `TestRun_NoGenerate` | `RanGenerate: false` | All steps skipped |
| `TestRun_GoModTidySuccess` | Fake Runner succeeds | go mod tidy succeeded |
| `TestRun_GoModTidyFailure` | Fake Runner returns error | go mod tidy failed, ai sync still runs |
| `TestRun_AISyncSuccess` | Valid project dir | ai sync succeeded |
| `TestRun_AISyncNone` | `AITarget: "none"` | ai sync skipped |
| `TestRun_AISyncFailure` | Invalid root dir | ai sync failed, warning written |
| `TestRun_DefaultTarget` | `AITarget: ""` | Defaults to "claude" |
| `TestRun_FullFlow` | Both steps succeed | Both succeeded |

### Integration / Smoke Tests

- Generate project with default flags → `.claude/CLAUDE.md` exists
- Generate with `--no-auto-steps` → `.claude/` NOT created
- Generate with `--ai-target none` → `.claude/` NOT created
- Generate with `--no-generate` → `.claude/` NOT created
- Generate with `--ai-target all` → `AGENTS.md` + `CLAUDE.md` + `.cursor/rules/ncgo.mdc` all exist

### Golden Tests

Existing golden tests in `internal/scaffold/mono/` verify NextSteps output. When `postGenerateNextSteps` changes signature, golden test fixtures may need updating to reflect the removal of `go mod tidy` and `ncgo ai sync` from the default post-generate NextSteps.

## Edge Cases

| Scenario | Behavior |
|----------|----------|
| Existing `.claude/` files | `ai.Sync` skips (force=false default) |
| No `go.mod` after generator | Shouldn't happen (guarded by `RanGenerate`); if it does, go mod tidy skips gracefully |
| Generator failure | `mono.Generate` returns error; `postgenerate.Run` is never called |
| `--template`/`--template-dir` | No special handling; generator ran → auto steps run |
| `go mod tidy` very slow | Runs synchronously; user sees progress output |

## Dependencies

- `internal/ai` — `ai.Sync()` (existing, no changes needed)
- `internal/exec` — `exec.Runner` interface (existing); may add `GoModTidy` helper
- `internal/scaffold/mono` — `mono.Generate` (no changes to Generate itself)

## Relationship to Other Issues

- **#56** (template preset equivalence) — already merged; this builds on template consumption
- **#57** (add rpc/bff template support) — independent; this only affects `ncgo new`
- Together these form the "official template → agent-ready" pipeline
