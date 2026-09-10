# Pipeline Analysis Report — 2026-09-10 (Issue #122)

Read-only analysis of `main` branch CI health after merging the fix for
Issue #122 (wire `RateLimitRuleRepository`/`RateLimitRuleHook` into
`ratelimit.Resolver`).

## Summary

| Window | Total Runs | Success Rate | Avg Duration | Tier |
|--------|-----------:|--------------:|-------------:|:----:|
| 7 days  | 31 | 87.1% | ~3m 06s | 🟡 |
| 14 days | 31 | 87.1% | ~3m 06s | 🟡 |

`topFailures` is empty in both windows — no recurring failure signature.

## Failure Pattern Detail

All 4 non-success runs in the 14-day window are `cancelled`, all on
2026-09-06, each immediately followed by a `success` run minutes later
(superseded-by-newer-push pattern, same as prior reports):

- `34021692368` cancelled 08:22:59 → `34021745824` success shortly after
- `34021501790` cancelled 08:18:52
- `34019057441` cancelled 07:24:23
- `34018947455` cancelled 07:21:57

No run tied to the Issue #122 merge failed. The most recent run
(2026-09-10T07:47:30Z, triggered by this merge) is `success`.

## Duration

~3 min average, unchanged from prior reports. No bottleneck to address.

## ⚠️ Escalation — 3rd Consecutive 🟡 Tier, No Remediation

This is the **3rd consecutive report** at 🟡 tier with the same root
cause and no remediation landed since the streak started:

| Report | Date | Success Rate | Tier |
|--------|------|--------------:|:----:|
| `pipeline-analysis-report-2026-09-08-issue119.md` | 2026-09-08 | 86.2% | 🟡 |
| `pipeline-analysis-report-2026-09-09-issue121.md` | 2026-09-09 | 86.7–87.1% | 🟡 |
| `pipeline-analysis-report-2026-09-10-issue122.md` (this report) | 2026-09-10 | 87.1% | 🟡 |

Per the Escalation Rule, this must be called out rather than silently
re-stated. Across all three reports, the metric's classification of
`cancelled` (superseded-push) runs as failures is the entire cause of
the sub-90% tier — there is no genuine CI regression in this window.

**Recommended next step (human-triggered, not done here):** open a
concrete Issue via `/gf-issue-create` proposing the pipeline-report
metric exclude `cancelled` runs from the success-rate denominator (or
report them as a separate category), since the current definition
produces a persistent false-positive 🟡 signal. This skill remains
read-only and will not open that Issue itself.

## Suggestions

1. No CI fix needed for Issue #122 — the merge is not implicated in any
   pipeline finding.
2. **Escalation trigger reached** (see above) — recommend the user
   decide whether to file the metric-definition Issue now.
3. No duration bottleneck to address.

## Scope Note

This is a read-only analysis. No pipelines were triggered, retried, or
modified.
