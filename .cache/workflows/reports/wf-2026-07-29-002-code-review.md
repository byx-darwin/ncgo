# Code Review Report — wf-2026-07-29-002 / PR #22

- PR: https://github.com/byx-darwin/ncgo/pull/22 (Closes #18)
- Branch: feat/18-pgv-validate-proto → main
- Diff: 6 commits (2 docs + 4 implementation), +4369/-18 across 13 files

## Review method
Subagent-driven development with an independent reviewer per task (spec compliance + code
quality), plus a final whole-branch review on the most-capable model.

## Verdict: READY TO MERGE
No Critical or Important findings.

## Acceptance criteria (Issue #18) — all met
1. Vendor `validate.proto` (PGV) into scaffold assets and emit it with new projects — DONE
   (`internal/assets/_data/hertz/validate/validate.proto`, emitted at `idl/validate/validate.proto`).
2. Add `(validate.rules)` min_len/max_len to `PingReq.name` in `mono/files.go` (BFF via delegation) — DONE.
3. Tighten `TestGoldenScaffoldProtoLintsClean` to assert zero warnings (4 Hertz fixtures) — DONE.
4. Regenerate golden fixtures; verify hz codegen works with PGV — DONE (hz v0.9.7 exit 0).

## Verification evidence
- gofmt / go build / go vet / go test ./... (all packages) / scripts/smoke.sh — all green.
- Five `validate.proto` copies byte-identical to canonical envoyproxy/protoc-gen-validate v1.3.3.
- Fresh generated project lints `errorCount: 0, warningCount: 0`.
- CI on PR #22: 2/2 runs success; mergeStateStatus CLEAN.

## Minor triage (none block merge)
1. `TestEmbeddedFilesNonEmpty` count threshold — SKIP (floor check; cannot break from an added file).
2. Provenance "DO NOT EDIT" header on vendored `validate.proto` — NICE-TO-HAVE.
3. Add `idl/validate/validate.proto` to the Kitex exclusion assertion in `mono_test.go` — NICE-TO-HAVE hardening.

## Note on GitHub verdict
A formal GitHub approve is intentionally NOT submitted: the PR author is the authenticated
user, and self-review is prohibited. Approval is deferred to a human reviewer.
