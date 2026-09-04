# Pipeline Analysis Report — PR #103 (feat/95-go-tools-v0.3.0-upgrade)

- **Repo**: byx-darwin/ncgo
- **PR**: #103 — go-tools dependency bump v0.1.0 → v0.3.0, Insecure config field wiring
- **Closes**: Issue #95
- **Branch**: `feat/95-go-tools-v0.3.0-upgrade` (merged-queued, now merged into `main`)
- **Analysis date**: 2026-09-04
- **Tool**: `gf` CLI (GitHub Actions backend)
- **Window analyzed**: `--days 30` (branch history is short-lived; only 2 runs exist)

## 1. Success Rate

| Source | Result |
|---|---|
| `gf pipeline report --branch feat/95-go-tools-v0.3.0-upgrade --days 30` | `totalRuns: 2`, `successRate: 0.5`, `topFailures: ["test"]` |
| `gf pipeline status --branch feat/95-go-tools-v0.3.0-upgrade` (live re-check) | Both runs: `status: success`, `conclusion: success` |
| `gf pipeline jobs` for each run ID | Both `test` jobs: `conclusion: success` |

**Finding (anomaly, not a real CI failure):** The `pipeline report` aggregate showed a 50% success rate and flagged `test` as a top failure. Re-querying live status and per-run jobs immediately after showed **both runs completed successfully** (run `33854549825` had transitioned from `running` → `success` between the two status calls). This strongly indicates the `report` command's snapshot was taken while the second run was still in-flight and counted the in-progress run as a non-success/failure, rather than reflecting an actual test failure.

- Run `33854524998` — job `test`: success, duration ~37s (08:39:58 → 08:40:35)
- Run `33854549825` — job `test`: success, duration ~34s (08:40:17 → 08:40:51)

Actual, settled success rate for this branch: **2/2 = 100%**.

## 2. Failure Patterns

- No genuine job failures found in the settled job-level data for either run.
- No flaky-test signal: both runs of the single `test` job completed cleanly with `conclusion: success`. (Flaky classification per skill policy requires ≥2 intermittent failures — none observed.)
- The `topFailures: ["test"]` entry from the aggregate `report` command is attributable to the in-flight/running-run timing artifact above, not a recorded failure log or test assertion failure.

## 3. Duration

- Both runs: single `test` job, ~23–34s average, well within normal bounds for this repo's CI.
- No bottleneck or slow-job pattern detected — sample size (2 runs, 1 job each) is too small to establish a duration trend, but nothing here suggests a regression from the go-tools v0.1.0→v0.3.0 bump.

## 4. Overall Health

- 🟢 **Green** — both actual CI runs for this branch/PR passed. The dependency bump and `Insecure` config wiring did not introduce a test regression visible in CI.
- ⚠️ **Tooling note** — `gf pipeline report` can under-count success rate when a run is still `running` at query time; treat report-command output as a snapshot, not a settled result, and prefer `gf pipeline status`/`gf pipeline jobs` re-checks before treating a reported failure as real. This is the first observation of this behavior for this branch (no repeat-tier streak to escalate per the skill's Escalation Rule — sample size is 2 runs, both now settled).

## 5. Suggestions (priority order)

1. **(Low, tooling)** When automating on `gf pipeline report` output, add a guard to skip or re-poll runs whose live `status` is `running` rather than counting them toward `successRate`/`topFailures` — avoids false-failure signals like the one seen here.
2. **(Low, informational)** Sample size for this feature branch is small (2 runs). No duration or flakiness trend can be reliably established; if go-tools v0.3.0 needs deeper validation, rely on `main`'s longer-running history rather than this short-lived branch.
3. No action needed on the `Insecure` config wiring or the v0.1.0→v0.3.0 bump itself from a CI-health perspective — both runs are clean.

## Data Sources

```
gf pipeline report --branch feat/95-go-tools-v0.3.0-upgrade --days 30
gf pipeline status --branch feat/95-go-tools-v0.3.0-upgrade   (queried twice)
gf pipeline jobs --pipeline-id 33854524998
gf pipeline jobs --pipeline-id 33854549825
```
