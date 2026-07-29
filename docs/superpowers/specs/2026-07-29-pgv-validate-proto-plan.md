# PGV validate.proto in Default Hertz Scaffolds — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make freshly generated Hertz scaffolds lint with 0 errors and 0 warnings under `ncgo protolint` by vendoring the canonical PGV `validate.proto`, emitting it with new projects, and adding `(validate.rules)` to `PingReq.name`.

**Architecture:** Add the canonical `protoc-gen-validate` `validate.proto` (proto2) as an embedded asset under `_data/hertz/validate/`, emit it into every Hertz scaffold at `idl/validate/validate.proto` (resolved by both `ncgo protolint`, whose import roots are `[root, root/idl]`, and `hz new … -I idl`). Add `import "validate/validate.proto";` plus a `(validate.rules)` string-length constraint to `PingReq.name` in the mono Hertz proto renderer; BFF inherits the change because `bff.Generate` delegates to `mono.Generate`. Tighten the self-consistency golden test to assert zero warnings across all four Hertz fixtures.

**Tech Stack:** Go 1.25+, `//go:embed`, `protocompile` (protolint), `golden.Tree` snapshot tests, `hz v0.9.7` (code-gen verification only).

## Global Constraints

- Go 1.25+; keep all files `gofmt`-clean; `go vet ./...` must pass.
- Scaffold templates and generated file layouts are **contract-sensitive**: update golden fixtures and tests together with any change.
- Golden tests use **pinned metadata** (`AssetsVersion: "test-assets"`, `NCGOVersion: "0.0.0-test"`, fixed clock) and `NoGenerate: true` — bumping `_data/VERSION` does NOT churn golden fixtures, and golden tests do not invoke `hz`.
- Canonical `validate.proto` source of truth: `validate/validate.proto` from `envoyproxy/protoc-gen-validate` (v1.3.3); proto2, `package validate`, extension `optional FieldRules rules = 1071;`.
- `ncgo protolint` resolves imports from `[root, root/idl]`; the Hertz generator command is `hz new … -I idl …`. So `import "validate/validate.proto";` resolves to `idl/validate/validate.proto`.
- Do NOT add `validate.proto` (or its import) to Kitex/micro/rpc scaffolds — their protos have no `PingReq.name` and emit no PIO402 warning.

## File Structure

| File | Responsibility | Action |
|------|----------------|--------|
| `internal/assets/_data/hertz/validate/validate.proto` | Canonical PGV extension definitions, embedded and emitted into Hertz projects | **Create** |
| `internal/assets/assets_test.go` | Locks presence of key embedded files (`TestEmbeddedFilesPresent`) | Modify: add `hertz/validate/validate.proto` to the `want` list |
| `internal/assets/_data/VERSION` | Assets version; convention = bump on any `_data/` template change | Modify: `0.1.31` → `0.1.32` |
| `internal/scaffold/mono/files.go` | `writeHertzProtoSupportFiles` (emit support protos) + `renderIDLPlaceholder` (render service proto) | Modify both |
| `internal/scaffold/mono/mono_test.go` | `TestGenerateNoGenerateProducesGoldenTree` (exact file-set) + the demo.proto content assertion | Modify: add `idl/validate/validate.proto` to file-set lists; extend content assertion |
| `internal/protolint/self_consistency_test.go` | `TestGoldenScaffoldProtoLintsClean` self-consistency lock | Modify: table-driven over 4 fixtures, assert 0 errors **and** 0 warnings |
| Golden fixtures (regenerate) | `mono/testdata/{mono-default,mono-with-database,mono-with-rulecenter}`, `bff/testdata/bff-default` | Regenerate via `-update-golden` |

---

## Task 1: Vendor canonical `validate.proto` into embedded assets

**Files:**
- Create: `internal/assets/_data/hertz/validate/validate.proto`
- Modify: `internal/assets/assets_test.go` (`TestEmbeddedFilesPresent` `want` slice)
- Modify: `internal/assets/_data/VERSION`

**Interfaces:**
- Produces: an embedded asset readable as `hertz/validate/validate.proto` via `assets.FS()` (used by Task 2's `writeHertzProtoSupportFiles`).

- [ ] **Step 1: Add the failing test entry**

In `internal/assets/assets_test.go`, inside `TestEmbeddedFilesPresent`, add this line to the `want` slice immediately after the `"hertz/optional/clickhouse.go",` entry:

```go
		"hertz/validate/validate.proto",
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./internal/assets/ -run TestEmbeddedFilesPresent -count=1`
Expected: FAIL with `missing embedded file "hertz/validate/validate.proto"`.

- [ ] **Step 3: Vendor the canonical file from the module cache**

The canonical PGV `validate.proto` (v1.3.3) is in the Go module cache. Copy it (it is read-only in the cache, so make it writable):

```bash
mkdir -p internal/assets/_data/hertz/validate
cp "$(go env GOMODCACHE)/github.com/envoyproxy/protoc-gen-validate@v1.3.3/validate/validate.proto" \
   internal/assets/_data/hertz/validate/validate.proto
chmod u+w internal/assets/_data/hertz/validate/validate.proto
```

Fallback if the module is not cached:

```bash
curl -fsSL https://raw.githubusercontent.com/envoyproxy/protoc-gen-validate/v1.3.3/validate/validate.proto \
  -o internal/assets/_data/hertz/validate/validate.proto
```

- [ ] **Step 4: Verify the vendored file is the canonical PGV definition**

Run:
```bash
head -3 internal/assets/_data/hertz/validate/validate.proto
grep -n "rules = 1071" internal/assets/_data/hertz/validate/validate.proto
```
Expected: line 1 `syntax = "proto2";`, line 2 `package validate;`, and a match for `optional FieldRules rules = 1071;`.

- [ ] **Step 5: Bump the assets version (repo convention for `_data/` changes)**

In `internal/assets/_data/VERSION`, change:

```
ncgo_assets_version: 0.1.31
```
to:
```
ncgo_assets_version: 0.1.32
```

(Leave the `generated_at` line unchanged; golden fixtures use the pinned `"test-assets"` version so this bump does not affect snapshots.)

- [ ] **Step 6: Run the assets tests to verify they pass**

Run: `go test ./internal/assets/ -count=1`
Expected: PASS (`TestEmbeddedFilesPresent`, `TestEmbeddedFilesNonEmpty`, `TestVersionParsesAssetsVersion`).

- [ ] **Step 7: Commit**

```bash
git add internal/assets/_data/hertz/validate/validate.proto internal/assets/assets_test.go internal/assets/_data/VERSION
git commit -m "feat(assets): vendor canonical PGV validate.proto for Hertz scaffolds"
```

---

## Task 2: Emit `validate.proto` and add the PGV rule in the Hertz generator

**Files:**
- Modify: `internal/scaffold/mono/files.go` (`writeHertzProtoSupportFiles`, `renderIDLPlaceholder`)
- Modify: `internal/scaffold/mono/mono_test.go` (`TestGenerateNoGenerateProducesGoldenTree` file-set lists; demo.proto content assertion)

**Interfaces:**
- Consumes: embedded asset `hertz/validate/validate.proto` (from Task 1) via `assets.FS()`.
- Produces: generated Hertz projects contain `idl/validate/validate.proto`; the rendered service proto imports it and constrains `PingReq.name`. Golden `Tree` snapshots will go red until Task 4 regenerates them — that is expected.

- [ ] **Step 1: Extend the demo.proto content assertion (RED)**

In `internal/scaffold/mono/mono_test.go`, find the `for _, want := range []string{...}` block that asserts `demo.proto` contents (it contains `import "api.proto";` and `import "openapi/annotations.proto";`). Add these two entries inside that slice, right after the `import "openapi/annotations.proto";` line:

```go
		`import "validate/validate.proto";`,
		`(validate.rules) = { string: { min_len: 1, max_len: 64 } },`,
```

- [ ] **Step 2: Add `validate.proto` to the exact file-set assertions (RED)**

In the same file, `TestGenerateNoGenerateProducesGoldenTree` builds a `want := []string{...}` of every generated file. Add this entry in sorted position, immediately after `"idl/openapi/openapi.proto",`:

```go
		"idl/validate/validate.proto",
```

Then find every OTHER exact file-set assertion in `mono_test.go` that lists the Hertz idl support files — search with:

```bash
grep -n '"idl/openapi/openapi.proto"' internal/scaffold/mono/mono_test.go
```

For each Hertz (non-Kitex) occurrence, insert `"idl/validate/validate.proto",` in sorted position right after the `idl/openapi/openapi.proto` line. (Do NOT add it to any Kitex file-set list — Kitex emits no `idl/openapi/` files.)

- [ ] **Step 3: Run the unit tests to verify they fail**

Run: `go test ./internal/scaffold/mono/ -run 'TestGenerateNoGenerateProducesGoldenTree|TestGenerateHertzProto' -count=1`
Expected: FAIL — `demo.proto missing "import \"validate/validate.proto\""` and a file-set mismatch (missing `idl/validate/validate.proto`).

- [ ] **Step 4: Emit `validate.proto` in `writeHertzProtoSupportFiles`**

In `internal/scaffold/mono/files.go`, in `writeHertzProtoSupportFiles`, after the existing `for _, name := range []string{"annotations.proto", "openapi.proto"}` loop (and before the final `return nil`), add:

```go
	// validate.proto (PGV) ships in its own idl/validate/ subdir so the service
	// proto's `import "validate/validate.proto";` resolves under `-I idl` (hz)
	// and protolint's [root, root/idl] import roots.
	validateBody, err := fs.ReadFile(srcFS, filepath.ToSlash(filepath.Join("hertz", "validate", "validate.proto")))
	if err != nil {
		return fmt.Errorf("scaffold: read embedded hertz/validate/validate.proto: %w", err)
	}
	validatePath := filepath.Join(dir, "idl", "validate", "validate.proto")
	if err := os.MkdirAll(filepath.Dir(validatePath), 0o755); err != nil {
		return fmt.Errorf("scaffold: mkdir %s: %w", filepath.Dir(validatePath), err)
	}
	if err := os.WriteFile(validatePath, validateBody, 0o644); err != nil {
		return fmt.Errorf("scaffold: write %s: %w", validatePath, err)
	}
```

- [ ] **Step 5: Add the import and the PGV rule in `renderIDLPlaceholder` (Hertz branch)**

In the same file, in the Hertz branch of `renderIDLPlaceholder`, the `strings.Join([]string{...}, "\n")` list currently has:

```go
		`import "api.proto";`,
		`import "openapi/annotations.proto";`,
```

Change it to:

```go
		`import "api.proto";`,
		`import "openapi/annotations.proto";`,
		`import "validate/validate.proto";`,
```

And in the `PingReq` block, currently:

```go
		`message PingReq {`,
		`  string name = 1 [`,
		`    (api.query) = "name",`,
		`    (openapi.parameter) = { required: true },`,
		`    (openapi.property) = {`,
```

Change it to (insert the `(validate.rules)` line after `(openapi.parameter)`):

```go
		`message PingReq {`,
		`  string name = 1 [`,
		`    (api.query) = "name",`,
		`    (openapi.parameter) = { required: true },`,
		`    (validate.rules) = { string: { min_len: 1, max_len: 64 } },`,
		`    (openapi.property) = {`,
```

- [ ] **Step 6: Run the unit tests to verify they pass**

Run: `go test ./internal/scaffold/mono/ -run 'TestGenerateNoGenerateProducesGoldenTree|TestGenerateHertzProto' -count=1`
Expected: PASS.

(Note: `TestGenerateGolden*` snapshot tests will FAIL now because the on-disk fixtures are stale — that is resolved in Task 4. Do not regenerate yet.)

- [ ] **Step 7: Commit**

```bash
git add internal/scaffold/mono/files.go internal/scaffold/mono/mono_test.go
git commit -m "feat(scaffold): emit validate.proto and add PGV validate.rules to PingReq.name"
```

---

## Task 3: Tighten `TestGoldenScaffoldProtoLintsClean` (RED)

**Files:**
- Modify: `internal/protolint/self_consistency_test.go`

**Interfaces:**
- Consumes: the four Hertz golden fixtures (regenerated in Task 4) and the protolint `Run`/`RunOptions` API (`Run(ctx, RunOptions{Root: root})`).
- Produces: a self-consistency lock asserting 0 errors **and** 0 warnings across all four Hertz fixtures.

- [ ] **Step 1: Rewrite the test to be table-driven and assert zero warnings**

Replace the entire body of `internal/protolint/self_consistency_test.go` with:

```go
package protolint

import (
	"context"
	"path/filepath"
	"testing"
)

// TestGoldenScaffoldProtoLintsClean locks the self-consistency contract
// between the scaffold generators and the lint rules: the default proto a
// fresh `ncgo new` writes must pass ncgo's own protolint through the same
// manifest-driven discovery path doctor uses, with zero error-level AND zero
// warning-level diagnostics. Every Hertz fixture that ships the Ping proto is
// covered so the PIO402 advisory stays cleared on all of them.
func TestGoldenScaffoldProtoLintsClean(t *testing.T) {
	cases := []struct {
		name string
		root string
	}{
		{"mono-default", filepath.Join("..", "scaffold", "mono", "testdata", "mono-default")},
		{"mono-with-database", filepath.Join("..", "scaffold", "mono", "testdata", "mono-with-database")},
		{"mono-with-rulecenter", filepath.Join("..", "scaffold", "mono", "testdata", "mono-with-rulecenter")},
		{"bff-default", filepath.Join("..", "scaffold", "bff", "testdata", "bff-default")},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := filepath.Clean(tc.root)
			res, err := Run(context.Background(), RunOptions{Root: root})
			if err != nil {
				t.Fatalf("Run: %v", err)
			}
			for _, d := range res.Diagnostics {
				if d.Level == LevelError || d.Level == LevelWarning {
					t.Errorf("%s diagnostic %s: %s (%s:%d field=%s)", d.Level, d.RuleID, d.Summary, d.File, d.Line, d.Field)
				}
			}
			if res.Summary.ErrorCount != 0 {
				t.Errorf("fresh scaffold proto produced %d error-level diagnostics, want 0", res.Summary.ErrorCount)
			}
			if res.Summary.WarningCount != 0 {
				t.Errorf("fresh scaffold proto produced %d warning-level diagnostics, want 0", res.Summary.WarningCount)
			}
		})
	}
}
```

- [ ] **Step 2: Run the test to verify it fails against the stale fixtures**

Run: `go test ./internal/protolint/ -run TestGoldenScaffoldProtoLintsClean -count=1`
Expected: FAIL — each subtest reports the PIO402 warning (`request field name looks like a free-text input but has no PGV string length constraint`) and/or a compile error from the missing `validate.proto` import, because the fixtures have not been regenerated yet.

- [ ] **Step 3: Commit the failing test**

```bash
git add internal/protolint/self_consistency_test.go
git commit -m "test(protolint): tighten self-consistency lock to zero warnings across Hertz fixtures"
```

---

## Task 4: Regenerate golden fixtures (GREEN)

**Files:**
- Regenerate: `internal/scaffold/mono/testdata/mono-default/`, `mono-with-database/`, `mono-with-rulecenter/`
- Regenerate: `internal/scaffold/bff/testdata/bff-default/`

**Interfaces:**
- Consumes: the generator changes from Task 2.
- Produces: fixtures whose service protos import `validate/validate.proto`, constrain `PingReq.name`, and ship `idl/validate/validate.proto` — making Task 3's test and all golden `Tree` tests pass.

- [ ] **Step 1: Regenerate the mono fixtures**

Run:
```bash
go test ./internal/scaffold/mono/... -run 'TestGenerateGoldenDefault|TestGenerateGoldenWithDatabase|TestGenerateGoldenWithRuleCenter' -update-golden -count=1
```
Expected: PASS (snapshots rewritten).

- [ ] **Step 2: Regenerate the bff fixture**

Run:
```bash
go test ./internal/scaffold/bff/... -run 'TestGenerateGoldenBFF' -update-golden -count=1
```
Expected: PASS (snapshot rewritten).

- [ ] **Step 3: Inspect the diff is exactly the expected change**

Run:
```bash
git status --short internal/scaffold/mono/testdata internal/scaffold/bff/testdata
git diff --stat internal/scaffold/mono/testdata internal/scaffold/bff/testdata
```
Expected: each of the four fixtures shows a modified `idl/app/*.proto` (or `idl/app/web-bff.proto`) and a NEW `idl/validate/validate.proto`. No other files changed.

Spot-check one fixture's service proto:
```bash
grep -n 'import "validate/validate.proto";\|(validate.rules)' internal/scaffold/mono/testdata/mono-default/idl/app/demo.proto
```
Expected: both lines present.

- [ ] **Step 4: Run the self-consistency test and all golden tests to verify GREEN**

Run:
```bash
go test ./internal/protolint/ -run TestGoldenScaffoldProtoLintsClean -count=1
go test ./internal/scaffold/mono/... ./internal/scaffold/bff/... -count=1
```
Expected: PASS (0 errors, 0 warnings across the four fixtures; all golden snapshots match).

- [ ] **Step 5: Commit**

```bash
git add internal/scaffold/mono/testdata internal/scaffold/bff/testdata
git commit -m "test(scaffold): regenerate golden fixtures with validate.proto + PGV rules"
```

---

## Task 5: Verify `hz` codegen still works with PGV annotations

**Files:** none (throwaway verification in a temp dir)

**Interfaces:**
- Consumes: a real `ncgo new` Hertz project (no `--no-generate`) and `hz v0.9.7`.
- Produces: evidence that `hz new … -I idl …` compiles `validate.proto` and generates handlers/models without conflict (the risk originally deferred from PR #17).

- [ ] **Step 1: Build ncgo and generate a throwaway Hertz project WITH codegen**

```bash
go build -o /tmp/ncgo .
rm -rf /tmp/pgv-verify && mkdir -p /tmp/pgv-verify
/tmp/ncgo new verify-svc --module github.com/acme/verify-svc --kind hertz --mode mono --dir /tmp/pgv-verify/verify-svc
```
Expected: the command runs `hz new … -I idl …` successfully (no proto compile error referencing `validate/validate.proto`), and prints next steps.

- [ ] **Step 2: Confirm generated artifacts exist and validate.proto was emitted**

```bash
ls /tmp/pgv-verify/verify-svc/idl/validate/validate.proto
ls /tmp/pgv-verify/verify-svc/internal/handler /tmp/pgv-verify/verify-svc/internal/pb
```
Expected: `validate.proto` present; hz generated handler and model (`internal/pb`) code.

- [ ] **Step 3: Confirm the generated project lints clean**

```bash
/tmp/ncgo protolint --root /tmp/pgv-verify/verify-svc --output json | jq '.summary'
```
Expected: `errorCount: 0` and `warningCount: 0`.

- [ ] **Step 4: Fallback decision point (only if Step 1 fails on validate.proto)**

If `hz` cannot compile the full canonical `validate.proto`, stop and surface the failure. The documented fallback is to vendor the 49-line subset already used by `internal/protolint/testdata/pgvconstraints/validate/validate.proto` instead, then repeat Tasks 1/4/5. Do NOT silently apply the fallback — report to the orchestrator first.

- [ ] **Step 5: Clean up**

```bash
rm -rf /tmp/pgv-verify
```

(No commit — this task produces verification evidence only, recorded in the PR body.)

---

## Task 6: Full validation, docs check, final commit

**Files:**
- Possibly modify: `README.md` / `README.zh-CN.md` / `docs/examples*.md` ONLY if they enumerate the generated `idl/` file set.

**Interfaces:**
- Consumes: all prior tasks.
- Produces: a green full-repo validation and aligned docs.

- [ ] **Step 1: Run the full validation suite (CI-equivalent)**

Run:
```bash
gofmt -l $(find . -name '*.go' -not -path './.git/*')
go build ./... && go build .
go vet ./...
go test ./... -count=1
./scripts/smoke.sh
```
Expected: `gofmt -l` prints nothing; build/vet/test/smoke all pass.

- [ ] **Step 2: Check whether docs enumerate the generated idl file set**

Run:
```bash
grep -rn "idl/openapi/openapi.proto\|idl/api.proto" README.md README.zh-CN.md docs/ 2>/dev/null
```
Expected: if any doc lists the generated Hertz `idl/` files, add `idl/validate/validate.proto` to that list in BOTH the English and Chinese variants. If no doc enumerates the file set, no doc change is needed (state this explicitly in the PR).

- [ ] **Step 3: Commit any doc updates**

```bash
git add -A README.md README.zh-CN.md docs/ 2>/dev/null
git diff --cached --quiet || git commit -m "docs: note validate.proto in generated Hertz idl layout"
```

---

## Self-Review

**1. Spec coverage:**
- "Vendor `validate.proto` (PGV) into scaffold assets and emit it with new projects" → Task 1 (asset) + Task 2 Step 4 (emit). ✓
- "Add `(validate.rules)` min_len/max_len to `PingReq.name` in `mono/files.go` (and BFF equivalents)" → Task 2 Step 5; BFF covered via `mono.Generate` delegation (Global Constraints). ✓
- "Tighten `TestGoldenScaffoldProtoLintsClean` to assert zero warnings too" → Task 3 (extended to all four Hertz fixtures per the approved decision). ✓
- "Regenerate golden fixtures; verify hz codegen still works with PGV annotations" → Task 4 (regen) + Task 5 (hz verification). ✓
- Contract-surface note (update tests/docs together) → Tasks 2/4/6. ✓

**2. Placeholder scan:** no TBD/TODO; every code step has full code; every command has expected output. ✓

**3. Type/path consistency:** asset path `hertz/validate/validate.proto` (Task 1) matches the `fs.ReadFile` path in Task 2 Step 4 and the emitted `idl/validate/validate.proto`; the `(validate.rules)` string and import string in Task 2 Steps 1/5 match exactly; fixture roots in Task 3 match the golden dirs from Task 4. ✓
