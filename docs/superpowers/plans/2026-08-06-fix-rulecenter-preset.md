# Fix ncgo rule-center preset template bugs - Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Fix 6 critical bugs in the rule-center preset template generation that prevent generated code from compiling.

**Architecture:** Hybrid approach - systematic fix for SQL YAML frontmatter (use ReadSharedFragmentBody), incremental fixes for proto syntax, import paths, and other template issues.

**Tech Stack:** Go 1.26, Kitex v0.16.1, PostgreSQL, sqlc, Protocol Buffers

## Global Constraints

- Go version: 1.26.5
- Kitex version: >= v0.16.1
- Proto syntax: proto3 (no `optional repeated`)
- Import paths: Must use `kitex_gen/` prefix for generated code
- SQL files: Pure SQL, no YAML frontmatter
- Template files: YAML format with `path`, `update_behavior`, `body` fields

---

## File Structure

### Files to Create/Modify:

1. **Scaffold Logic:**
   - Modify: `internal/scaffold/mono/files.go:196-203` (SQL YAML fix)

2. **Template Files:**
   - Create: `internal/assets/_data/kitex/schema/000002_rate_limit_rules.yaml` (renamed from .sql)
   - Delete: `internal/assets/_data/kitex/schema/000002_rate_limit_rules.sql` (after migration)
   - Modify: `internal/assets/_data/kitex/kitex-template/ratelimit_proto.yaml:45` (proto syntax)
   - Modify: `internal/assets/_data/kitex/kitex-template/ratelimit_handler.yaml:14` (import path)
   - Modify: `internal/assets/_data/kitex/kitex-template/ratelimit_usecase.yaml:16` (import path)
   - Modify: `internal/assets/_data/kitex/kitex-template/ratelimit_middleware.yaml:21` (limit package)

3. **Golden Tests:**
   - Update: `internal/scaffold/mono/testdata/mono-with-rulecenter/` (regenerate snapshots)

---

## Task 1: Fix SQL YAML Frontmatter (Systematic Fix)

**Files:**
- Create: `internal/assets/_data/kitex/schema/000002_rate_limit_rules.yaml`
- Delete: `internal/assets/_data/kitex/schema/000002_rate_limit_rules.sql`
- Modify: `internal/scaffold/mono/files.go:196-203`

**Interfaces:**
- Consumes: `shared.ReadSharedFragmentBody(srcFS, name, module)`
- Produces: Pure SQL file at `internal/db/schema/000002_rate_limit_rules.sql` in generated projects

- [ ] **Step 1: Create YAML fragment file**

Create the SQL schema as a proper YAML fragment:

```yaml
# internal/assets/_data/kitex/schema/000002_rate_limit_rules.yaml
path: internal/db/schema/000002_rate_limit_rules.sql
update_behavior:
  type: cover
body: |-
  CREATE TABLE IF NOT EXISTS rate_limit_rules (
      id BIGSERIAL PRIMARY KEY,
      service TEXT NOT NULL,
      phase TEXT NOT NULL,
      method TEXT NOT NULL,
      match_kind TEXT NOT NULL,
      path TEXT NOT NULL,
      path_pattern TEXT NOT NULL,
      app_key TEXT,
      priority INTEGER NOT NULL DEFAULT 0,
      enabled BOOLEAN NOT NULL DEFAULT true,
      key_by TEXT[] NOT NULL DEFAULT ARRAY['ip']::text[],
      strategy TEXT NOT NULL,
      window_seconds INTEGER NOT NULL,
      max_requests INTEGER NOT NULL,
      requests_per_second DOUBLE PRECISION,
      burst INTEGER,
      client_ttl_seconds INTEGER,
      created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
      updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
  );

  CREATE INDEX idx_rate_limit_rules_lookup ON rate_limit_rules (service, phase, method, match_kind, path, app_key);
  CREATE INDEX idx_rate_limit_rules_pattern ON rate_limit_rules (service, phase, method, app_key);

  -- Prevent duplicate rules for the same identity.
  CREATE UNIQUE INDEX idx_rate_limit_rules_unique
      ON rate_limit_rules (service, phase, method, match_kind, path, COALESCE(app_key, ''));
```

- [ ] **Step 2: Delete old SQL file**

```bash
rm internal/assets/_data/kitex/schema/000002_rate_limit_rules.sql
```

- [ ] **Step 3: Update files.go to use ReadSharedFragmentBody**

Modify `internal/scaffold/mono/files.go` in the `writeKitexTemplate` function:

```go
// BEFORE (lines 196-203):
if preset == "rule-center" {
    extras = append(extras,
        struct{ asset, path string }{
            asset: "kitex/schema/000002_rate_limit_rules.sql",
            path:  "internal/db/schema/000002_rate_limit_rules.sql",
        },
    )
    // ... shared fragments code
}

// AFTER:
if preset == "rule-center" {
    // Use ReadSharedFragmentBody to strip YAML frontmatter
    b, err := shared.ReadSharedFragmentBody(srcFS, "kitex/schema/000002_rate_limit_rules", opts.Module)
    if err != nil {
        return fmt.Errorf("scaffold: read rule-center schema: %w", err)
    }
    full := filepath.Join(dir, "internal", "db", "schema", "000002_rate_limit_rules.sql")
    if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
        return fmt.Errorf("scaffold: mkdir schema: %w", err)
    }
    if err := os.WriteFile(full, b, 0o644); err != nil {
        return fmt.Errorf("scaffold: write schema: %w", err)
    }
    // ... shared fragments code (unchanged)
}
```

- [ ] **Step 4: Run unit test**

```bash
go test ./internal/scaffold/mono/... -run TestGenerateGoldenDefault -count=1
```

Expected: PASS (default test should still work)

- [ ] **Step 5: Commit**

```bash
git add internal/assets/_data/kitex/schema/ internal/scaffold/mono/files.go
git commit -m "fix(scaffold): use ReadSharedFragmentBody for rule-center SQL schema

- Convert SQL template to YAML fragment format
- Use ReadSharedFragmentBody to strip YAML frontmatter
- Fixes sqlc 'syntax error at or near #' error"
```

---

## Task 2: Fix Proto Syntax Error

**Files:**
- Modify: `internal/assets/_data/kitex/kitex-template/ratelimit_proto.yaml:45`

**Interfaces:**
- Consumes: N/A
- Produces: Valid proto3 syntax in generated `idl/rule-center.proto`

- [ ] **Step 1: Fix proto syntax**

Edit `internal/assets/_data/kitex/kitex-template/ratelimit_proto.yaml` line 45:

```yaml
# BEFORE:
message UpdateRuleRequest {
  int64 id = 1;
  optional bool enabled = 2;
  optional int32 priority = 3;
  optional repeated string key_by = 4;  # ❌ Invalid proto3
  optional string strategy = 5;
  # ...
}

# AFTER:
message UpdateRuleRequest {
  int64 id = 1;
  optional bool enabled = 2;
  optional int32 priority = 3;
  repeated string key_by = 4;  # ✅ Removed optional
  optional string strategy = 5;
  # ...
}
```

- [ ] **Step 2: Verify syntax**

```bash
grep "optional repeated" internal/assets/_data/kitex/kitex-template/ratelimit_proto.yaml
```

Expected: No output (should not find "optional repeated")

- [ ] **Step 3: Commit**

```bash
git add internal/assets/_data/kitex/kitex-template/ratelimit_proto.yaml
git commit -m "fix(scaffold): remove invalid 'optional repeated' from proto3

- Proto3 repeated fields are already optional by nature
- Fixes kitex proto parser error"
```

---

## Task 3: Fix Import Path Mismatches

**Files:**
- Modify: `internal/assets/_data/kitex/kitex-template/ratelimit_handler.yaml:14`
- Modify: `internal/assets/_data/kitex/kitex-template/ratelimit_usecase.yaml:16`

**Interfaces:**
- Consumes: N/A
- Produces: Correct import paths in generated handler and usecase files

- [ ] **Step 1: Fix handler import path**

Edit `internal/assets/_data/kitex/kitex-template/ratelimit_handler.yaml` line 14:

```yaml
# BEFORE:
imports: |-
    ratelimitv1 "{{.Module}}/api/ratelimit/v1"

# AFTER:
imports: |-
    ratelimitv1 "{{.Module}}/kitex_gen/api/ratelimit/v1"
```

- [ ] **Step 2: Fix usecase import path**

Edit `internal/assets/_data/kitex/kitex-template/ratelimit_usecase.yaml` line 16:

```yaml
# BEFORE:
imports: |-
    ratelimitv1 "{{.Module}}/api/ratelimit/v1"

# AFTER:
imports: |-
    ratelimitv1 "{{.Module}}/kitex_gen/api/ratelimit/v1"
```

- [ ] **Step 3: Verify changes**

```bash
grep -n "api/ratelimit/v1" internal/assets/_data/kitex/kitex-template/ratelimit_*.yaml
```

Expected: Only show the `go_package` line in `ratelimit_proto.yaml` (which is correct)

- [ ] **Step 4: Commit**

```bash
git add internal/assets/_data/kitex/kitex-template/ratelimit_handler.yaml internal/assets/_data/kitex/kitex-template/ratelimit_usecase.yaml
git commit -m "fix(scaffold): update import paths to use kitex_gen prefix

- Handler and usecase now import from kitex_gen/api/ratelimit/v1
- Fixes 'module not found' compilation errors"
```

---

## Task 4: Fix Kitex server/limit Package Missing

**Files:**
- Modify: `internal/assets/_data/kitex/kitex-template/ratelimit_middleware.yaml:21`

**Interfaces:**
- Consumes: N/A
- Produces: Commented-out limit package with TODO marker

- [ ] **Step 1: Comment out limit import**

Edit `internal/assets/_data/kitex/kitex-template/ratelimit_middleware.yaml`:

```yaml
# BEFORE (line 21):
  	"github.com/cloudwego/kitex/server/limit"

# AFTER:
  	// TODO: Re-enable when kitex server/limit package is available
  	// "github.com/cloudwego/kitex/server/limit"
```

- [ ] **Step 2: Comment out StaticLimitOption implementation**

Find the `StaticLimitOption` function (around line 99) and update:

```yaml
# BEFORE:
func StaticLimitOption(s conf.StaticLimitConfig) kitexserver.Option {
	if s.MaxQPS <= 0 && s.MaxConnections <= 0 {
		return nil
	}
	return kitexserver.WithLimit(&limit.Option{MaxQPS: s.MaxQPS, MaxConnections: s.MaxConnections})
}

# AFTER:
// TODO: Re-enable when kitex server/limit package is available
func StaticLimitOption(s conf.StaticLimitConfig) kitexserver.Option {
	// if s.MaxQPS <= 0 && s.MaxConnections <= 0 {
	// 	return nil
	// }
	// return kitexserver.WithLimit(&limit.Option{MaxQPS: s.MaxQPS, MaxConnections: s.MaxConnections})
	return nil
}
```

- [ ] **Step 3: Commit**

```bash
git add internal/assets/_data/kitex/kitex-template/ratelimit_middleware.yaml
git commit -m "fix(scaffold): comment out unavailable kitex server/limit package

- Package doesn't exist in Kitex v0.16.3
- StaticLimitOption now returns nil
- Added TODO for future re-enablement"
```

---

## Task 5: Update Golden Tests

**Files:**
- Update: `internal/scaffold/mono/testdata/mono-with-rulecenter/` (regenerate)

**Interfaces:**
- Consumes: All template fixes from Tasks 1-4
- Produces: Updated golden test snapshots

- [ ] **Step 1: Run golden test with update flag**

```bash
go test ./internal/scaffold/mono/... -run TestGenerateGoldenWithRuleCenter -update-golden -count=1
```

Expected: Test passes, snapshots updated

- [ ] **Step 2: Verify updated snapshots**

Check that the updated golden files have:
- Pure SQL (no YAML frontmatter) in schema files
- Correct import paths with `kitex_gen/` prefix
- Valid proto syntax

```bash
# Check SQL file
head -5 internal/scaffold/mono/testdata/mono-with-rulecenter/internal/db/schema/000002_rate_limit_rules.sql
# Should start with "CREATE TABLE", not "# Kitex"

# Check import paths
grep "api/ratelimit/v1" internal/scaffold/mono/testdata/mono-with-rulecenter/internal/handler/*/handler.go
# Should show "kitex_gen/api/ratelimit/v1"
```

- [ ] **Step 3: Run golden test without update flag**

```bash
go test ./internal/scaffold/mono/... -run TestGenerateGoldenWithRuleCenter -count=1
```

Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add internal/scaffold/mono/testdata/
git commit -m "test(scaffold): update golden test snapshots for rule-center preset

- SQL schema now pure SQL (no YAML frontmatter)
- Import paths updated to kitex_gen prefix
- Proto syntax corrected"
```

---

## Task 6: End-to-End Verification

**Files:**
- Create: `/tmp/rule-center-test/` (temporary test project)

**Interfaces:**
- Consumes: All fixes from Tasks 1-5
- Produces: Verified working rule-center service

- [ ] **Step 1: Clean up and regenerate**

```bash
rm -rf /tmp/rule-center-test
cd /tmp
ncgo new rule-center-test \
  --module github.com/test/rule-center-test \
  --kind kitex \
  --db postgres \
  --preset rule-center \
  --dir /tmp/rule-center-test
```

Expected: Successfully generates project

- [ ] **Step 2: Run sqlc generation**

```bash
cd /tmp/rule-center-test
make sqlc
```

Expected: Success, no "syntax error at or near '#'" errors

- [ ] **Step 3: Run kitex code generation**

```bash
make update
```

Expected: Success, no proto parser errors

- [ ] **Step 4: Run go mod tidy**

```bash
go mod tidy
```

Expected: Success, no "module not found" errors

- [ ] **Step 5: Build the project**

```bash
go build ./...
```

Expected: Success, no compilation errors

- [ ] **Step 6: Verify directory structure**

```bash
tree -L 3 -I 'kitex_gen|go.sum|.git' | grep -E "(handler|usecase|repository)"
```

Expected: Only one set of directories (either `rulecenter/` or `ruleservice/`, not both)

- [ ] **Step 7: Document results**

Create a summary of the verification:

```markdown
## E2E Verification Results

- ✅ `make sqlc`: Success
- ✅ `make update`: Success  
- ✅ `go mod tidy`: Success
- ✅ `go build ./...`: Success
- ✅ Directory structure: No duplicates
- ✅ Import paths: Correct
- ✅ Proto syntax: Valid

All acceptance criteria met.
```

---

## Task 7: Final Review and Documentation

**Files:**
- Modify: `CHANGELOG.md` (if exists)
- Modify: Issue #37 (add completion comment)

**Interfaces:**
- Consumes: All verification results
- Produces: Updated documentation

- [ ] **Step 1: Update CHANGELOG (if exists)**

If `CHANGELOG.md` exists, add entry:

```markdown
## [Unreleased]

### Fixed
- rule-center preset: SQL schema files now generated without YAML frontmatter
- rule-center preset: Proto syntax corrected (removed invalid `optional repeated`)
- rule-center preset: Import paths updated to use `kitex_gen/` prefix
- rule-center preset: Commented out unavailable `kitex/server/limit` package
```

- [ ] **Step 2: Add completion comment to Issue #37**

```bash
gh issue comment 37 --body "## ✅ Implementation Complete

All 6 bugs fixed and verified:

1. ✅ SQL YAML frontmatter - Fixed using ReadSharedFragmentBody
2. ✅ Proto syntax error - Removed invalid 'optional repeated'
3. ✅ Import path mismatches - Updated to kitex_gen prefix
4. ✅ Kitex limit package - Commented out with TODO
5. ✅ Proto file naming - Verified correct
6. ✅ Duplicate directories - No duplicates found

**Verification:**
- Golden tests updated and passing
- E2E generation successful
- All build commands pass
- No compilation errors

**Files modified:**
- \`internal/scaffold/mono/files.go\`
- \`internal/assets/_data/kitex/schema/000002_rate_limit_rules.yaml\` (renamed from .sql)
- \`internal/assets/_data/kitex/kitex-template/ratelimit_proto.yaml\`
- \`internal/assets/_data/kitex/kitex-template/ratelimit_handler.yaml\`
- \`internal/assets/_data/kitex/kitex-template/ratelimit_usecase.yaml\`
- \`internal/assets/_data/kitex/kitex-template/ratelimit_middleware.yaml\`
- Golden test snapshots updated

Ready for review and merge."
```

- [ ] **Step 3: Final commit (if CHANGELOG updated)**

```bash
git add CHANGELOG.md
git commit -m "docs: add changelog entry for rule-center preset fixes"
```

---

## Success Criteria Checklist

After completing all tasks, verify:

- [ ] `make sqlc` succeeds without errors
- [ ] `make update` succeeds without errors
- [ ] `go build ./...` succeeds
- [ ] Golden tests pass
- [ ] No duplicate directories in generated code
- [ ] Generated code compiles and runs
- [ ] Issue #37 updated with completion status

---

## Risk Mitigation

**If golden tests fail:**
- Re-run with `-update-golden` flag
- Manually inspect updated snapshots
- Ensure all template changes are reflected

**If E2E verification fails:**
- Check each step individually
- Verify all template files were committed
- Ensure ncgo binary is rebuilt: `go build .`

**If import paths still wrong:**
- Double-check template files use `kitex_gen/` prefix
- Verify `go_package` in proto file is correct
