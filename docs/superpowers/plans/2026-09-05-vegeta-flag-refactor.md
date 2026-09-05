# Route Vegeta Dockerfile Flag Through framework.ComposeFeatureFlags Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Remove the duplicate "Hertz + WithDatabase enables Vegeta sidecar" rule expression at `internal/scaffold/mono/mono.go:147` by routing it through `framework.Adapter.ComposeFeatures(withDatabase).Vegeta`.

**Architecture:** Single-line condition replacement in an existing generation pipeline function. No new types, no new files, no interface changes. `framework` is already imported in `mono.go`.

**Tech Stack:** Go, existing `internal/scaffold/framework` adapter registry (`framework.MustGet`, `ComposeFeatureFlags`), existing golden-test harness in `internal/scaffold/mono`.

**Spec:** `docs/superpowers/specs/2026-09-05-vegeta-flag-refactor-design.md`

## Global Constraints

- Pure refactor: no change to generated project output (design doc, Acceptance Criteria).
- `go test ./internal/scaffold/mono/... -count=1` must show all 6 `TestGenerateGolden*` tests passing with zero diff (design doc, Acceptance Criteria).
- No new imports required — `framework` package already imported in `mono.go` (verified: `internal/scaffold/mono/mono.go:34` imports `github.com/byx-darwin/ncgo/internal/scaffold/framework`).

---

### Task 1: Replace the duplicate Vegeta condition in mono.go

**Files:**
- Modify: `internal/scaffold/mono/mono.go:147`
- Test: `internal/scaffold/mono/golden_test.go` (existing `TestGenerateGolden*` tests — no new test needed, this is a golden-test-covered pure refactor)

**Interfaces:**
- Consumes: `framework.MustGet(kind string) Adapter` (existing, `internal/scaffold/framework/framework.go`), `Adapter.ComposeFeatures(withDatabase bool) ComposeFeatureFlags` (existing), `ComposeFeatureFlags.Vegeta bool` field (existing, `internal/scaffold/framework/framework.go:70`)
- Produces: no new exported symbols; `mono.go`'s internal generation function behavior is unchanged for all existing callers/tests.

- [ ] **Step 1: Confirm the existing golden tests pass before touching anything (baseline)**

Run: `go test ./internal/scaffold/mono/... -run TestGenerateGolden -count=1 -v`
Expected: PASS (all 6 `TestGenerateGolden*` subtests green) — this is the baseline; if it fails before any change, stop and investigate separately, this plan does not cover fixing pre-existing failures.

- [ ] **Step 2: Replace the inline condition**

In `internal/scaffold/mono/mono.go`, change lines 147-151 from:

```go
	if defaultKind(opts.Kind) == manifest.KindHertz && opts.WithDatabase {
		if err := shared.WriteVegetaDockerfile(dir); err != nil {
			return nil, err
		}
	}
```

to:

```go
	if framework.MustGet(defaultKind(opts.Kind)).ComposeFeatures(opts.WithDatabase).Vegeta {
		if err := shared.WriteVegetaDockerfile(dir); err != nil {
			return nil, err
		}
	}
```

- [ ] **Step 3: Check whether `manifest.KindHertz` is still used elsewhere in mono.go**

Run: `grep -n "manifest\.Kind" internal/scaffold/mono/mono.go`
Expected: at least one remaining reference (e.g. `defaultKind`'s own normalization logic, or other kind checks at lines ~178/191/198 per the design doc). If `manifest` import becomes unused after the edit, remove the unused import; otherwise leave it — do not touch unrelated `manifest.Kind*` checks in this file (out of scope for this issue).

- [ ] **Step 4: Build to confirm no compile errors**

Run: `go build ./...`
Expected: succeeds with no errors (confirms `framework` import is still valid and no unused-import issues were introduced).

- [ ] **Step 5: Run the golden tests to confirm zero diff**

Run: `go test ./internal/scaffold/mono/... -run TestGenerateGolden -count=1 -v`
Expected: PASS — all 6 `TestGenerateGolden*` subtests green, identical output to Step 1's baseline (confirms pure refactor, no generated-output change).

- [ ] **Step 6: Run the full mono package test suite**

Run: `go test ./internal/scaffold/mono/... -count=1`
Expected: PASS, no regressions in any other mono test.

- [ ] **Step 7: Run `go vet` on the changed package**

Run: `go vet ./internal/scaffold/mono/...`
Expected: no issues reported.

- [ ] **Step 8: Commit**

```bash
git add internal/scaffold/mono/mono.go
git commit -m "refactor(mono): route Vegeta dockerfile flag through framework.ComposeFeatureFlags

Removes the duplicate inline expression of the 'Hertz + WithDatabase
enables Vegeta sidecar' rule at mono.go:147 in favor of the adapter's
ComposeFeatures(withDatabase).Vegeta, already established by #101.

Closes #111"
```
