# Pipeline Analysis Report — Issue #100 (infra Plugin registry refactor)

- **Repo**: byx-darwin/ncgo
- **Context**: gf-workflow `wf-2026-09-04-005`, Phase 4 post-delivery check
- **Issue**: #100 — infra Plugin registry refactor
- **Delivery**: locally merged to `main` as commit `2f33e8b` (squash merge), **no PR was created**
- **Branch analyzed**: `main`
- **Analysis date**: 2026-09-04
- **Tool**: `gf` CLI (GitHub Actions backend)
- **Window analyzed**: `--days 30`

## 1. Success Rate

`gf pipeline report --branch main --days 30`:

| Metric | Value |
|---|---|
| Total runs | 49 |
| Success rate | 95.9% (47/49) |
| Avg duration | ~65.6s |
| Top failures | `test` |

Tier: 🟢 **Green** (≥95% success rate).

`gf pipeline status --branch main` (last 29 runs shown, 2026-08-09 → 2026-09-04) confirms: 27 `success`, 1 `cancelled` (run `32826965240`, 2026-08-25T08:30), 1 `failed` (run `32819741196`, 2026-08-25T07:03).

**Important caveat**: Commit `2f33e8b` (the Issue #100 delivery, timestamped 2026-09-04 22:27:32 +0800) is **ahead of the most recent recorded CI run** (`33875556927`, completed 2026-09-04T12:59:35Z / ~20:59 local). Because the merge was local-only with no PR, **no CI run has yet executed against this specific commit**. All success-rate data above reflects pre-delivery history on `main`, not a validation of the Plugin registry refactor itself.

## 2. Failure Patterns

- Single genuine failure in the 30-day window: run `32819741196` (2026-08-25), job `test`, `conclusion: failure`, ~23s duration. Isolated occurrence — not repeated in adjacent runs, so it does not meet the ≥2-intermittent-failure bar for flaky classification, nor the ≥3-consecutive bar for a persistent pattern.
- One `cancelled` run (`32826965240`, same day, ~16s) immediately preceding the successful rerun at `32826985667` — consistent with a manual cancel/retrigger, not a pipeline defect.
- No flaky tests identified: no test name shows repeated intermittent failure across runs in this window.

## 3. Duration

- Single `test` job per run across all sampled pipelines (no separate lint/build/deploy stages observed in `gf pipeline jobs` output).
- Typical duration: 16–90s; average ~65.6s. No bottleneck job or duration regression trend visible in the sampled window.
- No data yet for the Plugin registry refactor commit itself (see caveat above) — duration impact of the refactor on `test` job runtime is unverified.

## 4. Overall Health

- 🟢 **Green** for `main`'s general 30-day trend — high success rate, no flaky tests, no duration bottleneck.
- ⚠️ **Coverage gap for this specific delivery**: commit `2f33e8b` has not been exercised by CI. Since no PR was opened, it also bypassed any PR-gated checks (if configured). This is a process gap for this delivery, not a pipeline health regression.
- No repeat-tier streak to escalate — this is the first `main`-branch pipeline report on record (a prior report, `pipeline-analysis-report-2026-09-04-pr103.md`, covered a different feature branch, not `main`).

## 5. Suggestions (priority order)

1. **(Medium)** Trigger a CI run for `main` at or after commit `2f33e8b` (e.g. via a follow-up push, empty commit, or manual workflow dispatch) to validate the Plugin registry refactor actually passed the `test` job — currently unverified by CI.
2. **(Low, process)** Since no PR was created for this delivery, consider whether future deliveries of this size should go through a PR for CI-gated review, per repo convention (recent history shows Issue-linked PRs, e.g. #109, #110).
3. **(Low)** No action needed on general pipeline health — success rate, duration, and flakiness are all within normal bounds for `main`.

## Data Sources

```
gf pipeline report --branch main --days 30
gf pipeline status --branch main
gf pipeline jobs --pipeline-id 32819741196   (failure)
gf pipeline jobs --pipeline-id 33875556927   (latest success, pre-2f33e8b)
git log --oneline -5
git show -s --format='%H %ci' 2f33e8b
```
