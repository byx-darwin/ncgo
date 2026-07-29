# Design: ncgo Full-Matrix Solution Verification

- **Workflow:** `wf-2026-07-29-001` (fast mode)
- **Date:** 2026-07-29
- **Goal:** Actually run `ncgo` to generate projects across every supported surface and verify whether the current repository delivers its claimed "complete solution". Produce a claim-vs-actual matrix report with evidence; flag gaps as follow-up Issues. This is a verification task, not a behavior change.

## 1. Scope

Everything the CLI exposes (from `ncgo --help` + `add infra` kinds + embedded assets):

### A. Service generation — `ncgo new`
| # | Case | Flags |
|---|------|-------|
| A1 | mono hertz, default | `--mode mono --kind hertz` |
| A2 | mono hertz + database | `--db postgres` |
| A3 | mono hertz + infra at creation | `--infra redis` |
| A4 | mono kitex | `--kind kitex` |
| A5 | mono kitex rule-center preset | `--kind kitex --preset rule-center --rule-center-addr ...` |
| A6 | mono `--no-generate` | writes manifest + template/ + idl only |
| A7 | micro workspace | `--mode micro` |

### B. Micro/mono add-ons — `ncgo add ...`
| # | Case |
|---|------|
| B1 | `add bff` inside micro workspace |
| B2 | `add rpc` inside micro workspace |
| B3 | `add domain <name>` |
| B4 | `add method <domain.Method>` at anchors |
| B5 | `add rule-center` |

### C. Infra add-ons — `ncgo add infra <kind>` (9 kinds)
`redis, kafka, es, clickhouse, observability_logging, logging, release_canary, canary, registry_polaris`
- C1: each kind against the matching service kind (hertz/kitex) → files written + manifest recorded
- C2: `--dry-run` / `--plan` (JSON) leave tree untouched
- C3: `--wire` on logging (hertz server wiring, incl. dry-run no-write)
- C4: `--output json` shape stability

### D. Lifecycle & agent surfaces
| # | Case |
|---|------|
| D1 | `doctor` (host tools + project) |
| D2 | `ai sync` idempotency + `ai init/claude` bootstrap |
| D3 | `mcp serve`: `tools/list` contains required tools; invoke `ncgo_version`, `ncgo_doctor`, `ncgo_ai_sync`, `ncgo_add_infra`, `ncgo_add_method` |
| D4 | `protolint` on good + bad proto |
| D5 | `i18n` structured output |
| D6 | `export` / `import` round-trip |
| D7 | `upgrade --plan` read-only; `--apply` updates metadata |
| D8 | `extract domain --plan` / `--apply` import rewrite |
| D9 | `test` command on generated project |
| D10 | `version` build metadata |

### E. Generated-project correctness (best-effort tier)
- E1: `go mod tidy && go build ./...` inside generated mono hertz project (network-dependent)
- E2: same for mono kitex (respects `make sqlc` before tidy when db=true)

### T0. Repository self-checks (baseline of "claimed = locked by tests")
- `go build ./... && go build .`
- `go vet ./...`
- `go test ./... -count=1` (includes golden tests)
- `./scripts/smoke.sh`

## 2. Verification tiers & pass criteria

| Tier | What | Criterion |
|------|------|-----------|
| **T0** repo self-check | build/vet/test/smoke | MUST all pass — otherwise the "complete solution" claim is already broken |
| **T1** command + artifact | every A/B/C/D case runs, exits 0, produces documented files/output | MUST all pass |
| **T2** generated project builds | E1/E2 compile | Best-effort (network/toolchain); report status, not a hard gate |

**Verdict rule:** solution is "complete" iff T0 ✅ and T1 ✅ for all cases. Any T1 failure = gap → listed in report with severity + follow-up Issue recommendation. T2 failures reported as environment caveats or real bugs (triaged case-by-case).

## 3. Method

1. Work in a throwaway sandbox dir (`/tmp` or `.cache/verify`), never inside the repo working tree.
2. Build `ncgo` once from current HEAD (`go build -o <sandbox>/ncgo .`).
3. Drive every matrix case via shell; capture exit codes + key artifact greps as evidence.
4. Record results row-by-row in the report with PASS/FAIL/SKIP + evidence snippet.
5. No repo behavior changes. The only committed artifact is the verification report (Phase 3 PR).

## 4. Deliverables

- `docs/superpowers/reviews/full-matrix-verification-2026-07-29.md` — claim-vs-actual matrix, verdict, gap list, follow-up recommendations.
- Phase 3 PR containing the report (+ verification notes).
- Follow-up Issues for any confirmed gaps (created in Phase 4 triage, not this workflow's code scope).

## 5. Risks / constraints

- **Network dependency:** T2 builds fetch hertz/kitex modules; if offline, mark SKIP not FAIL.
- **External tool versions:** `hz`/`kitex` installed (verified: both on PATH). `doctor` output pins expectations.
- **Non-destructive:** all generation happens in sandbox dirs; no edits to repo-owned generated files.
- **No behavior change:** if bugs are found, report + propose Issues; do not fix inline (out of scope for a verification task).
