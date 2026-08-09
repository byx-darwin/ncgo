# Code Review — PR #17 (feat/16-full-matrix-verification)

Reviewer: independent ncgo-reviewer agent · Date: 2026-07-29 · Workflow: wf-2026-07-29-001 (Phase 4)

Scope verified: `git diff main...HEAD` = 2 commits (f91c8dc report docs + cf69b92 fix), 11 files, +333/−7. All findings verified against the worktree.

## 1. Correctness — PASS

- `importRoots()` edge cases (internal/protolint/load.go:49-56): no `idl/` → `os.Stat` errors → skipped; `idl` is a file → `fi.IsDir()` false → skipped. Project root stays first — additive only.
- Workspace mode: `Run` → per-target `runSingleRoot(target.Root, …)` (run.go:63-66) → `Load(Root=serviceRoot)` → `importRoots` resolves each service's own `idl/`. Correct.
- `(api.body)` removal has no orphaned consumers: grep across all testdata/ empty; no non-test .go emits it. BFF golden correctly regenerated via `bff.go:90` delegation to `mono.Generate`.
- Meta-test exercises the real discovery path: `self_consistency_test.go:17` calls `Run(Root, no Files)` → `discoverServiceFiles` (manifest-driven) → `runSingleRoot` → `Load` with `importRoots` — byte-for-byte the path doctor uses (internal/doctor/protolint.go:65). Both fixes genuinely under test.

## 2. Contract Safety — PASS

- Golden delta is exactly 4×1-line deletions (mono-default, mono-with-database, mono-with-rulecenter `demo.proto` + bff-default `web-bff.proto`), each removing the single `(api.body) = "message",` line. No reordering, no collateral churn.
- No CLI flags/help, MCP schema, `content[0].text`, or JSON field changes. `files.go` change is a single line (:469). MCP/CLI/doctor funnel through `Run`, so the fix propagates uniformly.

## 3. Test Quality — PASS (minor gap)

- `load_test.go:142` asserts import resolution from project root — the precise Gap #1 regression.
- `self_consistency_test.go` is a true lint-over-golden meta-test (error-level gate, warnings allowed) — correctly tolerates the known PIO402 advisory warning.
- `mono_test.go:243` PIO206 negative assertion guards the template at the generator level.
- RED→GREEN structurally sound (both tests fail for the right reason without each fix).
- Minor gap (non-blocking): no dedicated unit test for `importRoots` edge branches (idl-is-a-file / no-idl). Logic trivial and guarded; a 5-line table test would lock it. Optional follow-up.

## 4. Docs Alignment — PASS

- No stale `api.body` references in README*/docs (only the new report mentions it). No `protolint --root` semantics docs contradicted (docs/examples.md documents `--root` usage, unchanged).
- Report claims reconcile with code; PIO402 warning deferral honestly flagged with stated reason (PGV vendoring).

## 5. Risk — PASS

- Template change affects only newly generated scaffolds; existing users' protos untouched.
- `importRoots` additive and root-first; projects without `idl/` see identical behavior. Mirrors `hz -I idl` convention.
- gofmt clean; protolint/mono/bff packages all PASS (`go test -count=1`).

## Overall Verdict: APPROVE

Minimal, deliberate, contract-safe diff. Both High gaps fixed at the correct layer (generator + resolver), each backed by a test that fails for the right reason, golden fixtures regenerated exactly, docs honest and aligned. Only optional follow-up: small edge-case table test for `importRoots` — not a blocker.
