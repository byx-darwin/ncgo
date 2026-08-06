# Fix ncgo rule-center preset template bugs

**Date:** 2026-08-06  
**Author:** AI Assistant  
**Status:** Draft  
**Workflow:** fix-rulecenter-preset (fast mode)

## 1. Problem Statement

The `ncgo new --preset rule-center` command generates a rule-center service with multiple critical bugs that prevent the generated code from compiling:

1. **SQL files contain YAML frontmatter** - `internal/db/schema/000002_rate_limit_rules.sql` includes YAML metadata (`# Kitex rule-center preset...`, `path:`, `update_behavior:`, `body:`), causing sqlc to fail with "syntax error at or near '#'"
2. **Invalid proto3 syntax** - `idl/rule-center.proto` contains `optional repeated string key_by = 4;` which is invalid proto3 syntax
3. **Proto file naming confusion** - Both `rulecenter.proto` (empty) and `rule-center.proto` (correct) are generated, but manifest and Makefile point to the empty one
4. **Import path mismatches** - Generated code imports from `api/ratelimit/v1` but kitex generates to `kitex_gen/api/ratelimit/v1`
5. **Missing Kitex package** - `github.com/cloudwego/kitex/server/limit` doesn't exist in Kitex v0.16.3
6. **Duplicate directories** - Both `handler/rulecenter/` and `handler/ruleservice/` are generated, causing confusion

**Impact:** Users cannot successfully generate and build a rule-center service using `ncgo new --preset rule-center`.

## 2. Solution Approach

**Hybrid approach (方案 C):**
- **Systematic fix** for SQL YAML frontmatter: Modify scaffold logic to use `ReadSharedFragmentBody`
- **Incremental fixes** for other issues: Modify specific template files and configurations

**Rationale:**
- SQL file issue is systemic (YAML template processing), warrants unified solution
- Proto naming and import paths are specific configuration issues, safer to fix incrementally
- Balances risk and benefit

## 3. Architecture

### Fix Strategy

```
P0: SQL YAML frontmatter (systematic)
  ↓
P0: Proto syntax error (incremental)
  ↓
P1: Proto file naming (incremental)
  ↓
P1: Import path mismatches (incremental)
  ↓
P2: Kitex limit package missing (incremental)
  ↓
P2: Duplicate directories (incremental)
  ↓
Verification: Golden test + manual generation
```

### Priority Classification

- **P0 (Critical):** Blocks code generation or compilation
- **P1 (High):** Causes runtime errors or incorrect behavior
- **P2 (Medium):** Code quality or maintainability issues

## 4. Detailed Fixes

### Bug 1: SQL File YAML Frontmatter (Systematic Fix)

**Root Cause:** `internal/scaffold/mono/files.go:200-202` copies SQL template file directly using `fs.ReadFile` instead of using `shared.ReadSharedFragmentBody()` to strip YAML frontmatter.

**Fix:**
```go
// File: internal/scaffold/mono/files.go
// Location: writeKitexTemplate function, around line 196-203

// BEFORE:
if preset == "rule-center" {
    extras = append(extras,
        struct{ asset, path string }{
            asset: "kitex/schema/000002_rate_limit_rules.sql",
            path:  "internal/db/schema/000002_rate_limit_rules.sql",
        },
    )
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
}
```

**Files Modified:**
- `internal/scaffold/mono/files.go`

---

### Bug 2: Proto Syntax Error (Incremental Fix)

**Root Cause:** Template file `internal/assets/_data/kitex/kitex-template/ratelimit_proto.yaml` contains `optional repeated string key_by = 4;` which is invalid proto3 syntax. In proto3, `repeated` fields are already optional by nature.

**Fix:**
```yaml
# File: internal/assets/_data/kitex/kitex-template/ratelimit_proto.yaml
# Location: UpdateRuleRequest message

# BEFORE:
message UpdateRuleRequest {
  int64 id = 1;
  optional bool enabled = 2;
  optional int32 priority = 3;
  optional repeated string key_by = 4;  # ❌ Invalid
  optional string strategy = 5;
  ...
}

# AFTER:
message UpdateRuleRequest {
  int64 id = 1;
  optional bool enabled = 2;
  optional int32 priority = 3;
  repeated string key_by = 4;  # ✅ Removed optional
  optional string strategy = 5;
  ...
}
```

**Files Modified:**
- `internal/assets/_data/kitex/kitex-template/ratelimit_proto.yaml`

---

### Bug 3: Proto File Naming Confusion (Incremental Fix)

**Root Cause:** Multiple proto files are generated with inconsistent naming. The manifest and Makefile point to the wrong file.

**Fix:**
1. Ensure only one proto file is generated: `idl/rule-center.proto`
2. Update manifest template to point to `idl/rule-center.proto`
3. Update Makefile template to set `IDL_FILE = idl/rule-center.proto`

**Files Modified:**
- Check `internal/scaffold/mono/files.go` for duplicate proto generation
- Update manifest template (if separate)
- Update Makefile template (if separate)

---

### Bug 4: Import Path Mismatches (Incremental Fix)

**Root Cause:** Generated code imports from `api/ratelimit/v1` but kitex generates to `kitex_gen/api/ratelimit/v1`.

**Fix:** Update import paths in template files:

```go
// BEFORE:
import ratelimitv1 "github.com/.../api/ratelimit/v1"

// AFTER:
import ratelimitv1 "github.com/.../kitex_gen/api/ratelimit/v1"
```

**Files Modified:**
- `internal/assets/_data/kitex/kitex-template/ratelimit_handler.yaml`
- `internal/assets/_data/kitex/kitex-template/ratelimit_usecase.yaml`
- `internal/assets/_data/kitex/kitex-template/ratelimit_shared_rule_center_client.yaml`

---

### Bug 5: Kitex server/limit Package Missing (Incremental Fix)

**Root Cause:** `github.com/cloudwego/kitex/server/limit` package doesn't exist in Kitex v0.16.3.

**Fix:** Comment out the code with TODO marker:

```go
// File: internal/assets/_data/kitex/kitex-template/ratelimit_middleware.yaml

// BEFORE:
import (
    ...
    "github.com/cloudwego/kitex/server/limit"
    ...
)

func StaticLimitOption(s conf.StaticLimitConfig) kitexserver.Option {
    if s.MaxQPS <= 0 && s.MaxConnections <= 0 {
        return nil
    }
    return kitexserver.WithLimit(&limit.Option{MaxQPS: s.MaxQPS, MaxConnections: s.MaxConnections})
}

// AFTER:
// TODO: Re-enable when kitex server/limit package is available
// import "github.com/cloudwego/kitex/server/limit"

func StaticLimitOption(s conf.StaticLimitConfig) kitexserver.Option {
    // if s.MaxQPS <= 0 && s.MaxConnections <= 0 {
    //     return nil
    // }
    // return kitexserver.WithLimit(&limit.Option{...})
    return nil
}
```

**Files Modified:**
- `internal/assets/_data/kitex/kitex-template/ratelimit_middleware.yaml`

---

### Bug 6: Duplicate Directories (Incremental Fix)

**Root Cause:** Both `handler/rulecenter/` and `handler/ruleservice/` directories are generated.

**Fix:**
1. Review `layout-rulecenter.yaml` template list
2. Remove duplicate directory generation logic
3. Standardize on `ruleservice` (matches proto service name)

**Files Modified:**
- `internal/assets/_data/kitex/layout-rulecenter.yaml`
- Potentially `internal/scaffold/mono/files.go` if it creates directories

---

## 5. Testing Strategy

### Golden Test Updates

```bash
# 1. Fix templates, then update golden snapshots
go test ./internal/scaffold/mono/... -run TestGenerateGoldenRuleCenter -update-golden -count=1

# 2. Verify golden test passes
go test ./internal/scaffold/mono/... -run TestGenerateGoldenRuleCenter -count=1
```

**Checkpoints:**
- SQL file has no YAML frontmatter
- Proto file has correct syntax
- Import paths include `kitex_gen/`
- No duplicate directories

### End-to-End Verification

```bash
# 1. Clean old project
rm -rf /tmp/rule-center-test

# 2. Regenerate
ncgo new rule-center-test \
  --module github.com/test/rule-center-test \
  --kind kitex \
  --db postgres \
  --preset rule-center \
  --dir /tmp/rule-center-test

# 3. Generate code
cd /tmp/rule-center-test
make sqlc          # Should succeed, no YAML errors
make update        # Should succeed, no proto syntax errors

# 4. Compile verification
go mod tidy        # Should succeed, no import errors
go build ./...     # Should succeed, no compilation errors

# 5. Check generated files
tree -L 3 -I 'kitex_gen|go.sum' | grep -E "(handler|usecase|repository)"
# Should only have ruleservice/, no rulecenter/
```

**Success Criteria:**
- ✅ All commands complete without errors
- ✅ SQL file is pure SQL (no YAML)
- ✅ Proto file has correct syntax
- ✅ Code compiles successfully
- ✅ Directory structure has no duplicates

## 6. Implementation Plan

### Phase 1: Fix Templates (30 min)
1. Fix SQL YAML frontmatter in `files.go`
2. Fix proto syntax error in `ratelimit_proto.yaml`
3. Fix proto file naming
4. Fix import paths in template files
5. Fix Kitex limit package issue
6. Fix duplicate directories

### Phase 2: Update Golden Tests (15 min)
1. Run golden tests with `-update-golden`
2. Verify updated snapshots
3. Run tests without `-update-golden` to confirm

### Phase 3: End-to-End Verification (15 min)
1. Generate new rule-center project
2. Run all build commands
3. Verify successful compilation
4. Check directory structure

### Phase 4: Documentation (10 min)
1. Update CHANGELOG if needed
2. Document any breaking changes

**Total estimated time:** 1-1.5 hours

## 7. Risk Assessment

### Low Risk
- SQL YAML frontmatter fix: Isolated to one function
- Proto syntax fix: Simple template change
- Import path fix: Mechanical search and replace

### Medium Risk
- Proto file naming: May affect existing projects
- Duplicate directories: Need to ensure no references to old paths

### Mitigation
- Golden tests will catch regressions
- End-to-end verification ensures real-world correctness
- Fast mode workflow allows quick iteration

## 8. Success Metrics

- ✅ `make sqlc` succeeds without errors
- ✅ `make update` succeeds without errors
- ✅ `go build ./...` succeeds
- ✅ Golden tests pass
- ✅ No duplicate directories in generated code
- ✅ Generated code compiles and runs

## 9. Future Improvements

1. **Add integration test** for rule-center preset
2. **Automated validation** of generated code compilation
3. **Template linting** to catch YAML frontmatter issues
4. **Kitex version compatibility matrix** documentation

## 10. References

- Issue: (to be created)
- Design doc: `docs/superpowers/specs/2026-08-06-fix-rulecenter-preset-design.md`
- Workflow: `fix-rulecenter-preset` (fast mode)
- Related code: `internal/scaffold/mono/files.go`, `internal/assets/_data/kitex/`
