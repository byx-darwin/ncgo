# Pipeline Analysis Report — PR #113 (feat/111-route-vegeta-flag → main)

Date: 2026-09-05
Scope: `byx-darwin/ncgo`, branch `feat/111-route-vegeta-flag`, base `main`
Context: PR #113 for Issue #111 (small one-line refactor in `internal/scaffold/mono/mono.go`), queued for auto-merge pending required checks.

## 1. Success Rate

`gf pipeline report --branch feat/111-route-vegeta-flag --days 7` reported:

| Metric | Value |
|---|---|
| Total runs | 2 |
| Success rate | 0.0% |
| Avg duration | 4.5s |
| Top failures | `test` |

**Finding (anomaly, not a real CI failure):** At query time both CI runs for this branch were still `running` (`gf pipeline status`), and the `test` job in each was `in_progress` with no `conclusion`. The 4.5s "avg duration" is far below the branch's/repo's typical `test` job duration, confirming the runs were snapshotted mid-flight. Re-checking status ~5 minutes later showed run `33970621724` had settled to `status: success` / `conclusion: success`; the second run (`33970716494`) remained `in_progress` at last check.

Actual, currently-known outcome for this branch: **1/1 settled runs = 100% success**, 1 run still pending completion.

Baseline for comparison — `main` branch, 7-day window: **93.3% success rate** (14/15), avg duration ~59s — healthy 🟢.

## 2. Failure Patterns

No genuine failure observed. The only "failure" signal (`test` job) was an artifact of the `report` command treating in-progress runs (empty `conclusion`) as non-success. This is the **same tooling behavior** previously documented in `docs/pipeline-analysis-report-2026-09-04-pr103.md`.

No flaky-test pattern identified — this is a single-observation snapshot artifact, not an intermittent failure across ≥2 settled runs.

## 3. Duration

Insufficient settled-run data to assess duration distribution for this branch specifically (only 1 run settled at time of writing). `main`'s 7-day baseline avg (~59s) shows no bottleneck.

## Overall Assessment

🟢 **Green** — no real CI regression detected. The 0% success rate figure from `gf pipeline report` is a stale/in-flight snapshot, not evidence of a failing check. PR #113's auto-merge queue is proceeding normally; one required check has already passed, the other is still running.

## Escalation Check

This is the **second consecutive observation** of `gf pipeline report` under-counting success rate due to in-progress runs (first: `pipeline-analysis-report-2026-09-04-pr103.md`). Per the skill's Escalation Rule, 3 consecutive same-tier reports with no remediation triggers an escalation callout — this is only the 2nd occurrence, so no escalation yet, but it is now a recurring tooling gap worth tracking:

- **Recommendation (Low/Medium priority):** If this in-progress-miscount behavior appears a 3rd time, it should be raised as a concrete tooling issue (e.g. "`gf pipeline report` should exclude or separately bucket runs with empty `conclusion` rather than counting them as failures") via `/gf-issue-create` (human-triggered), rather than re-diagnosing it ad hoc in each report.

## Suggestions (priority order)

1. **(Medium, tooling)** Track whether `gf pipeline report`'s in-progress-miscount recurs a 3rd time; if so, file a fix issue for the `gf pipeline` CLI/report aggregation.
2. **(Low)** No action needed on PR #113 itself — the underlying refactor's CI is healthy; let auto-merge proceed once the remaining check settles.
