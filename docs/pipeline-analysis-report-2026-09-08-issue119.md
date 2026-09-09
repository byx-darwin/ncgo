# Pipeline Analysis Report — 2026-09-08 (Issue #119)

**Branch:** main
**Window:** last 7 days
**Context:** post-merge check after Issue #119 fix (commit 478d762)

## Summary

| Metric | Value |
|---|---|
| Total runs | 29 |
| Success rate | 86.2% (🟡 watch) |
| Avg duration | 177.6s (~3 min) |
| Top failure patterns | none reported (no `topFailures` entries) |

## Analysis

- Success rate sits in the yellow band (below ~90%) but no specific
  recurring failure pattern was surfaced by the tool (`topFailures` empty),
  so the dip does not point at a single fixable root cause from this data
  alone.
- Duration (~3 min average) is reasonable, no bottleneck signal.
- **Correction:** an earlier version of this report incorrectly stated no
  prior data exists for comparison. Two prior reports do cover `main`:
  `pipeline-analysis-report-2026-09-04-issue100.md` (30-day window,
  95.9%, 🟢) and `pipeline-analysis-report-2026-09-05-pr113.md` (same
  7-day window as this report, 93.3%, 🟢, dated just 3 days before this
  one). This report's 86.2% is a real **tier drop from 🟢 to 🟡** over
  that short interval, not a first observation. This is only 2
  data points at different tiers, so the ≥3-consecutive-same-tier
  escalation rule does not trigger yet — but the trend is worth watching
  on the next report.

## Suggestions

1. If failures recur, re-run this analysis with a longer window
   (`--days 30`) or narrow to `gf pipeline jobs` on a specific failing run
   to identify which job/step is intermittently failing.
2. No action required specifically tied to the Issue #119 change — this
   is a general branch-health snapshot, not implicating the merged commit.
3. **Watch the trend**: main dropped from 🟢 (93.3%, 2026-09-05) to 🟡
   (86.2%, 2026-09-08) in 3 days. One more 🟡-or-worse report in the next
   check would make this a persistent pattern worth a dedicated
   investigation (`gf pipeline jobs` on the failing runs to find which
   step/job is intermittent).

## Scope Note

This is a read-only analysis. No pipelines were triggered, retried, or
modified.
