# replaceServiceName Boundary Regex Fix Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Fix `replaceServiceName` in `internal/scaffold/template/export.go` so lowercase service-name occurrences immediately followed by Go delimiters like `(` and `{` (not just whitespace/`/`/`.`/quotes/`;`/`,`) are correctly variabilized during `ncgo export templates`.

**Architecture:** Replace the hand-enumerated boundary-character regex with Go RE2's native `\b` word-boundary assertion. `\b` is zero-width (matches between a `\w` char `[0-9A-Za-z_]` and a non-`\w` char, or string start/end), so it naturally covers every Go delimiter (`(`, `)`, `{`, `}`, `:`, `&`, etc.) without a maintained character class, and it doesn't consume the boundary character — eliminating the need for the current double-run-with-capture-group-backfill workaround.

**Tech Stack:** Go standard library `regexp` (RE2 engine, `\b` supported natively).

**Spec:** No separate spec file — this is a bounded fix approved inline during brainstorming for Issue #119 (https://github.com/byx-darwin/ncgo/issues/119). Design: replace the `segRE` boundary character class with `\b`, collapse the double-run loop into a single `ReplaceAllString` call.

## Global Constraints

- Single file behavior change: only `internal/scaffold/template/export.go` (function `replaceServiceName`, lines ~355-369) and its test file `internal/scaffold/template/export_test.go`.
- PascalCase branch (`typeRE`, lines ~343-353) must NOT be touched.
- No CLI/MCP/scaffold-template contract changes — this is an internal regex fix inside an existing exported behavior (`ncgo export templates`).
- Existing tests `TestReplaceServiceName_LowercaseSegments`, `TestReplaceServiceName_LowercaseNoFalsePositive`, `TestReplaceServiceName_ImportPath` must continue to pass unchanged.
- Golden tests `TestExportGoldenHertz` / `TestExportGoldenKitex` (`internal/scaffold/template/testdata/golden/export-*`) must pass without needing `-update-golden` (the fixtures contain no `(`/`{`-adjacent lowercase service-name occurrences today, so output should be byte-identical).

---

### Task 1: Add failing regression test for delimiter-adjacent identifiers

**Files:**
- Modify (test): `internal/scaffold/template/export_test.go`

**Interfaces:**
- Consumes: `replaceServiceName(body, serviceName string) string` (existing signature, unchanged) from `internal/scaffold/template/export.go`.
- Produces: nothing new for later tasks — this is a leaf regression test.

- [ ] **Step 1: Write the failing test**

Add this test function to `internal/scaffold/template/export_test.go` (after `TestReplaceServiceName_LowercaseNoFalsePositive`, i.e. after line 202):

```go
func TestReplaceServiceName_LowercaseDelimiterAdjacent(t *testing.T) {
	body := "cfg := &conf.RateLimitConfig{\n" +
		"\tStrategy:      rule.GetStrategy(),\n" +
		"\tWindowSeconds: config.Duration{Duration: time.Duration(rule.GetWindowSeconds()) * time.Second},\n" +
		"\tMaxRequests:   int(rule.GetMaxRequests()),\n" +
		"}\n"
	got := replaceServiceName(body, "Rule")
	if strings.Contains(got, "rule.") {
		t.Errorf("lowercase service name immediately followed by '(' must be replaced, got:\n%s", got)
	}
	if strings.Count(got, "{{ToLower .ServiceName}}") != 3 {
		t.Errorf("expected 3 lowercase substitutions (Strategy/WindowSeconds/MaxRequests lines), got:\n%s", got)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/scaffold/template/... -run TestReplaceServiceName_LowercaseDelimiterAdjacent -v`

Expected: FAIL — the current boundary character class `[\s/."';,]` does not include `(` or newline-then-tab-`r`, so `rule.GetStrategy()` (preceded by tab, a whitespace char — that one actually matches) — but critically the `Duration(rule.GetWindowSeconds())` case has `rule` preceded by `(` which is NOT in the boundary set, so that occurrence stays as literal `rule` and the count assertion (`!= 3`) fails, or the `strings.Contains(got, "rule.")` check fails because `rule.GetWindowSeconds` inside `Duration(...)` is not replaced.

- [ ] **Step 3: Commit the failing test**

```bash
git add internal/scaffold/template/export_test.go
git commit -m "test(export): add regression case for delimiter-adjacent service name"
```

---

### Task 2: Replace boundary character class with `\b` word-boundary

**Files:**
- Modify: `internal/scaffold/template/export.go:355-369`

**Interfaces:**
- Consumes: nothing new.
- Produces: `replaceServiceName` behavior change consumed by Task 1's test (now should pass) and all pre-existing callers/tests.

- [ ] **Step 1: Implement the fix**

In `internal/scaffold/template/export.go`, replace lines 355-369 (comment block + `segRE` + double-run loop):

```go
	// Replace lowercase service name occurrences that appear as bounded
	// tokens: path segments, package declarations, package qualifiers,
	// and quoted import paths. Non-boundary occurrences (userrpc2,
	// myuserrpc) are left untouched.
	lower := serviceNameLower(serviceName)
	if lower != "" {
		// Boundary chars include common delimiters in proto/Go files: whitespace,
		// slash, dot, quotes, semicolon (proto statement terminator), comma.
		// We run the replacement twice to handle adjacent tokens like ";lower;lower"
		// where the shared boundary would otherwise be consumed by the first match.
		segRE := regexp.MustCompile(`(^|[\s/."';,])` + regexp.QuoteMeta(lower) + `($|[\s/."';,])`)
		for i := 0; i < 2; i++ {
			body = segRE.ReplaceAllString(body, "${1}{{ToLower .ServiceName}}${2}")
		}
	}
```

with:

```go
	// Replace lowercase service name occurrences that appear as bounded
	// tokens: path segments, package declarations, package qualifiers,
	// quoted import paths, and any other position bordered by a non-word
	// character (parens, braces, colons, etc.). Non-boundary occurrences
	// (userrpc2, myuserrpc) are left untouched. \b is zero-width so it
	// doesn't consume the boundary character, avoiding the double-run
	// workaround previously needed for adjacent tokens.
	lower := serviceNameLower(serviceName)
	if lower != "" {
		segRE := regexp.MustCompile(`\b` + regexp.QuoteMeta(lower) + `\b`)
		body = segRE.ReplaceAllString(body, "{{ToLower .ServiceName}}")
	}
```

- [ ] **Step 2: Run the regression test to verify it passes**

Run: `go test ./internal/scaffold/template/... -run TestReplaceServiceName_LowercaseDelimiterAdjacent -v`

Expected: PASS

- [ ] **Step 3: Run all `replaceServiceName`-related tests**

Run: `go test ./internal/scaffold/template/... -run TestReplaceServiceName -v`

Expected: PASS for `TestReplaceServiceName`, `TestReplaceServiceName_ImportPath`, `TestReplaceServiceName_LowercaseSegments`, `TestReplaceServiceName_LowercaseNoFalsePositive`, `TestReplaceServiceName_LowercaseDelimiterAdjacent`.

- [ ] **Step 4: Run golden tests**

Run: `go test ./internal/scaffold/template/... -run TestExportGolden -v`

Expected: PASS without needing `-update-golden` (fixtures in `golden_test.go` have no `(`/`{`-adjacent lowercase service-name occurrence, so output is unchanged).

- [ ] **Step 5: Run the full package test suite**

Run: `go test ./internal/scaffold/template/... -count=1`

Expected: PASS, no regressions.

- [ ] **Step 6: Commit**

```bash
git add internal/scaffold/template/export.go
git commit -m "fix(export): use \\b word boundary in replaceServiceName to cover all Go delimiters"
```

---

### Task 3: Repository-wide validation

**Files:** none (validation only)

**Interfaces:** none

- [ ] **Step 1: Build**

Run: `go build ./... && go build .`

Expected: success, no errors.

- [ ] **Step 2: Vet**

Run: `go vet ./...`

Expected: no issues.

- [ ] **Step 3: Full test suite**

Run: `go test ./... -count=1`

Expected: PASS.

- [ ] **Step 4: gofmt check**

Run: `gofmt -l internal/scaffold/template/export.go internal/scaffold/template/export_test.go`

Expected: empty output (no formatting issues).

- [ ] **Step 5: Manual smoke — re-export against a real generated project (Acceptance Criteria item 4)**

If a locally scaffolded ncgo project is available (e.g. one produced by `ncgo new`), run `ncgo export templates` against it and manually grep the exported `template/` tree for any remaining literal (non-templated) occurrences of the service name adjacent to `(` or `{`. This step satisfies Issue #119's fourth acceptance-criteria checkbox. If no real project is at hand, note this as a follow-up manual verification step for the PR description rather than skipping the checkbox silently.

- [ ] **Step 6: Commit if any fixups were needed**

Only if Steps 1-4 surfaced something to fix; otherwise no commit here (Task 2 already committed the fix).
