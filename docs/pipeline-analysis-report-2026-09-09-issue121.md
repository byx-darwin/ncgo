# Pipeline Analysis Report — 2026-09-09 (Issue #121 post-delivery check)

## Context

Phase 4 post-delivery pipeline health check after merging branch
`feat/121-protect-rulecenter-go-export` into `main` locally (merge commit
`e0aa199294563428334ecabd69b2e2cdf7587a4b`, fixing Issue #121:
`export.go`'s `replaceServiceName` mangling rule-center's fixed Go
identifiers). This is a general branch-health snapshot, not an evaluation
specific to the merged commit.

Read-only analysis via `gf` CLI. No pipelines were triggered, retried, or
modified.

## 1. Success-Rate Trend

| Window | Total Runs | Success Rate | Tier |
|--------|-----------:|--------------:|:----:|
| 7 days | 30 | 86.7% | 🟡 |
| 14 days | 31 | 87.1% | 🟡 |

Breaking down the 14-day window by conclusion (`gf pipeline status --branch main`,
30 runs returned): **26 success, 4 cancelled, 0 failure**. There are no
actual `failure`-conclusion runs in this window — the sub-90% success rate
is entirely attributable to `cancelled` runs, not real CI failures.

The 4 cancelled runs cluster tightly on 2026-09-06 (07:21–08:24 UTC),
consistent with GitHub Actions' default concurrency-group cancellation
when multiple pushes land in quick succession (each new push cancels the
prior in-flight run on the same ref). This is expected behavior for a
branch receiving rapid consecutive pushes, not pipeline instability.

## 2. Failure Patterns

`topFailures` was empty in both report windows, and no run in the raw
status list has `conclusion: "failure"`. **No genuine build/test failures
found in the last 14 days.** The only non-success outcomes are the 4
superseded/cancelled runs described above.

**Flaky tests**: none detected — flakiness requires intermittent
pass/fail on re-runs of the same test, and there are zero failures to
examine for that pattern.

## 3. Duration / Bottlenecks

- 14-day average duration: **176.5s** (~2.9 min)
- 7-day average duration: **180.3s** (~3.0 min)
- Range across successful runs: 32s (fastest) to 329s (slowest,
  run `34138232003`, 2026-09-07T15:25:27Z)

Inspected the slowest run's job breakdown (`gf pipeline jobs --pipeline-id
34138232003`): a single `test` job spanning the full 329s, no parallel
step-level timing exposed by the CLI. No single job stands out as an
outlier bottleneck relative to the rest of the distribution — the spread
(32s–329s) likely reflects different trigger paths (e.g. docs-only pushes
finishing fast vs. full build+test runs) rather than a degrading step.

**No duration bottleneck identified.**

## 4. Tier Trend / Escalation Check

| Report | Date | Window | Success Rate | Tier |
|--------|------|--------|--------------:|:----:|
| pipeline-analysis-report-2026-09-04-issue100.md | 2026-09-04 | 30 days | 95.9% | 🟢 |
| pipeline-analysis-report-2026-09-05-pr113.md | 2026-09-05 | 7 days | 93.3% | 🟢 |
| pipeline-analysis-report-2026-09-08-issue119.md | 2026-09-08 | 7 days | 86.2% | 🟡 |
| **This report** | 2026-09-09 | 7 days | 86.7% | 🟡 |

This is the **2nd consecutive 🟡 report** for `main` (2026-09-08 and
2026-09-09), both in the 86–87% band. Per the skill's escalation rule,
≥3 consecutive same-tier reports with no remediation triggers a mandatory
escalation callout — this streak is at 2, so it does not yet trigger.
However, since both 🟡 readings are driven by concurrency-cancelled runs
rather than real failures, this looks like a **measurement-tier artifact**
(the report's success-rate formula counts `cancelled` as non-success) more
than a genuine health regression. Worth confirming on the next report: if
a 3rd consecutive 🟡 lands and is still all-cancellation-driven, the fix
is arguably in how "success rate" is computed/interpreted, not in the CI
pipeline itself.

## Suggestions

1. **No CI fix needed for Issue #121** — the merge is not implicated in
   any pipeline finding; there are no real failures in the analyzed
   window.
2. **Watch, don't act yet**: if the next report is also 🟡 (3rd
   consecutive), treat that as the escalation trigger and investigate
   whether it's still cancellation-driven vs. a genuine new failure
   pattern — at that point a manual `/gf-issue-create` (human-triggered)
   noting the tier metric conflates "cancelled" with "failed" would be a
   reasonable follow-up, since this repeatedly produces a 🟡 signal for
   which there is no remediable regression.
3. No duration bottleneck to address — average ~3 min per run is fine for
   this workflow size.

## Scope Note

This is a read-only analysis. No pipelines were triggered, retried, or
modified.
