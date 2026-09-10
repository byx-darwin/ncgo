# Code Review Report — Issue #122

**Issue:** https://github.com/byx-darwin/ncgo/issues/122
**Delivery:** local merge (no PR) — merge commit `2bc2042` on `main`, from `feat/122-wire-ratelimit-database-hook`
**Scope:** 1 scaffold template, 4 golden fixtures, 2 design docs, +96/-7 lines

> No open PR exists for this delivery (local-merge mode was chosen), so
> there is nothing to submit a `gf review` verdict against. This report
> stands in as the Phase 4 code-review record; findings below were
> formed by direct diff inspection, not `gf review` CLI submission.

## Verdict: PASS

### Changes Reviewed

| File | Notes |
|---|---|
| `internal/assets/_data/hertz/hertz-template/server_go.yaml` | Adds `ratelimit` import, package-level `RateLimitResolver` var, and assembles `ratelimit.Options.Database` from the repository/hook before constructing the resolver |
| `internal/scaffold/{mono,bff}/testdata/**/server_go.yaml` | Golden fixtures regenerated via `-update-golden`; identical diff across all 4, as expected (unrendered template source) |
| `rate-limit-dynamic-design.{zh-CN,en}.md` | Appendix A.1 now names the concrete `RateLimitResolver` variable and its wiring call, replacing generic pseudocode |

### Correctness Check

- Root cause confirmed: `repository.NewRateLimitRuleRepository(dbData)` return value was previously discarded; `ratelimit.NewResolver` was always called with a zero-value `Options`, so the `"database"` source branch in `internal/pkg/ratelimit/resolver.go` was unreachable dead code.
- Fix gates construction on `cfg.RateLimit.Source.Type == "database"` (not just `cfg.Database.Enabled`), matching `resolver.go`'s own switch — no double-gating bug.
- `RateLimitResolver = ratelimit.NewResolver(...)` runs unconditionally inside the `WithDatabase` block, independent of `cfg.Database.Enabled` — correct, since the resolver must still exist (with local-config fallback) when the DB is present in the template but disabled at runtime.
- No change to `internal/pkg/ratelimit/resolver.go`, repository template, schema, or migrations — consistent with the approved "wire up, don't touch resolver" design.

### Consistency Check

- `mono` and `bff` golden fixtures both updated (bff shares the same `server_go.yaml` asset) — confirmed by full `go test ./... -count=1` passing after both updates.
- EN/ZH doc edits are structurally parallel (same sentence added to the same bullet in Appendix A.1).

### Risk Assessment

- Change is confined to a `{{if .WithDatabase}}` block already gated in both the import list and generated body — no impact on non-database hertz scaffolds.
- End-to-end validation (Phase 3) confirmed via the real `hz` binary: scaffolded a `--db postgres` project, rendered `server.go` compiles (`go build`, `go vet` clean), and the `RateLimitResolver`/`rateLimitOpts` code matches the template exactly.
- Full repo suite (`go build ./...`, `go build .`, `go vet ./...`, `go test ./... -count=1`) passes on `main` post-merge.
- Unrelated pre-existing i18n test flake (`TestTranslateBuiltInLanguages`) was observed in the generated project and confirmed present on unmodified `main` too — correctly not touched, out of scope.

### Recommendation

Safe as merged. No follow-up required for this change.
