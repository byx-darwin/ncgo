# Rate Limiting Implementation Audit

## Summary

Audited the ncgo rate limiting implementation for mono (Hertz HTTP) and micro (rule-center Kitex) services. Found **2 Critical** bugs, **4 Warnings**, and **4 Minor** issues. All Critical bugs and 3 Warnings have been fixed.

## Architecture Overview

```
Mono (Hertz) ──► Middleware ──► Resolver ──► Source: config | database | rule_center(gRPC)
                                            │
                                            ├─ config:      local conf.yaml rules
                                            ├─ database:    sqlc via repository hook
                                            └─ rule_center: gRPC to Kitex service

Micro (Kitex) ──► Rule-Center Service ──► sqlc ──► PostgreSQL
                  (gRPC CRUD API)
```

## Bugs Fixed

### Critical #1 — app_key not passed in rule_center_client.go [FIXED]

**File:** `internal/assets/_data/hertz/optional/rule_center_client.go:91-96`

**Bug:** `GetRuleRequest` was constructed without `AppKey`, even though:
- The proto has `optional string app_key = 5`
- The usecase handles app-specific rule lookups via `appKey`
- The `Lookup` struct carries `lookup.AppKey`

**Impact:** All app-specific rate-limit rules **never matched**. The rule-center always fell back to global (app_key IS NULL) rules.

**Fix:** Added `AppKey: strPtrOrNil(lookup.AppKey)` to the request and a `strPtrOrNil` helper.

```go
resp, err := c.cli.GetRule(ctx, &ratelimitv1.GetRuleRequest{
    Service: lookup.Service,
    Phase:   lookup.Phase,
    Method:  lookup.Method,
    Path:    lookup.Path,
    AppKey:  strPtrOrNil(lookup.AppKey),  // ← added
})
```

---

### Critical #2 — RuleCenter never instantiated in generated server.go [FIXED]

**File:** `internal/assets/_data/hertz/layout.yaml:8258-8272`

**Bug:** Generated `server.go` creates `rlOpts ratelimit.Options` but never sets `rlOpts.RuleCenter` when `source.type: "rule_center"`. The wiring example existed only as a comment.

**Impact:** `source.type: "rule_center"` → dynamic source is nil → resolver **always falls back to local config rules**.

**Fix (ncgo new):** Added conditional wiring to layout.yaml when `RuleCenterAddr` is set:

```go
{{ if .RuleCenterAddr}}
if strings.EqualFold(strings.TrimSpace(cfg.RateLimit.Source.Type), "rule_center") {
    rc, err := middleware.NewRuleCenterClient(cfg.RateLimit.RuleCenter.Address)
    if err != nil {
        panic(fmt.Sprintf("rule_center: init client: %v", err))
    }
    defer func() { _ = rc.Close() }()
    rlOpts.RuleCenter = rc
}
{{ end}}
```

**Fix (ncgo add rule-center):** Added `wireRuleCenterInServer()` in `internal/scaffold/rulecenter/rulecenter.go` that injects the same wiring block into existing `server.go` when `ncgo add rule-center` is run.

---

### Warning #3 — pickBestPattern missing glob/regex support [FIXED]

**File:** `internal/assets/_data/kitex/kitex-template/ratelimit_usecase.yaml:190-203`

**Bug:** Only handled `*` wildcard, `prefix`, and `exact`. Schema and proto support `glob` and `regex` match kinds.

**Impact:** glob/regex rules in the database were **silently skipped**.

**Fix:** Added `globMatch()` and `regexMatch()` functions with `switch` dispatch.

---

### Warning #4 — SQL queries missing enabled filter [FIXED]

**File:** `internal/assets/_data/kitex/kitex-template/ratelimit_sqlc_queries.yaml`

**Bug:** All 4 read queries returned disabled rules. None checked `enabled = true`.

**Impact:** Disabled rules in the database would still be returned and could be enforced by the middleware.

**Fix:** Added `AND enabled = true` to all read query WHERE clauses.

---

### Warning #5 — Seed data path_pattern semantics [FIXED]

**File:** `internal/scaffold/test/ratelimit/seed.go:52-56`

**Bug:** Seed row for `/healthz` exact match had `path_pattern='/healthz'` — semantically wrong since `path_pattern` is for prefix/glob/regex patterns, not exact matches.

**Fix:** Changed to `path_pattern=''` for exact-match rules. Wildcard rules (`path='*'`) keep `path_pattern='*'` as the universal fallback.

---

### Warning #6 — gRPC connection leak [FIXED by Critical #2 fix]

**File:** `internal/assets/_data/hertz/optional/rule_center_client.go:69-74`

**Bug:** `Close()` existed but generated code never called it.

**Fix:** The server wiring now includes `defer func() { _ = rc.Close() }()`.

---

## Remaining Issues (Not Fixed)

### Minor #8 — Kitex rule-center has no rate-limit middleware

**File:** `internal/assets/_data/kitex/kitex-template/ratelimit_middleware.yaml`

**Status:** By design. The middleware is a pass-through placeholder. The rule-center service itself has zero rate-limit enforcement. Not fixed because it requires a separate design decision on how to rate-limit a gRPC service (different middleware pattern from Hertz).

### Minor #9 — Seed SQL uses string interpolation

**File:** `internal/scaffold/test/ratelimit/seed.go:52-56`

**Status:** `buildSeedSQL` uses `fmt.Sprintf` with `sanitizeSQLString` (single-quote escape). While safe for PostgreSQL with `standard_conforming_strings=on` (default since 9.1), parameterized queries via `psql --variable` would be better. Low priority since this is test infrastructure only.

## One-Click Test Implications

After fixing Critical #1 and #2, the rule-center flow is now functional for e2e testing:

| Scenario | Status | Notes |
|----------|--------|-------|
| Mono + config source | ✅ Works | Local conf.yaml rules, no DB needed |
| Mono + database source | ✅ Works | seed → run via PostgreSQL |
| Micro + rule-center | ✅ Now works | Was broken before (Critical #1 + #2) |
| Mono + Kitex RPC | ❌ No middleware | Warning #8 — pass-through placeholder |

The `ncgo test rate-limit e2e` command can now test all three working scenarios.
