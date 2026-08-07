# Fix Rule-Center Proto Naming Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Fix rule-center preset template to comply with ncgo protolint rules (naming, validation, pagination, field count).

**Architecture:** Update embedded Kitex templates in `internal/assets/_data/` and corresponding golden test data. Rename all RPC message types from `Request/Response` to `Req/Resp`, refactor `CreateRuleReq` with nested `RateLimitConfig`, add PGV validation, and add pagination to `ListRules`.

**Tech Stack:** Go 1.25+, Protocol Buffers, PGV (protoc-gen-validate), Kitex RPC framework

## Global Constraints

- All RPC input messages must use `<Method>Req` naming (PIO101)
- All RPC output messages must use `<Method>Resp` naming (PIO101)
- String fields: `min_len: 1, max_len: 100` (service), `max_len: 50` (phase), `max_len: 255` (path)
- Numeric fields: `priority` gte:0 lte:10000, `window_seconds` gt:0 lte:86400, `max_requests` gt:0
- Repeated fields: `key_by` max_items:10
- Request field count must be ≤12 (PIO warning threshold)
- Pagination must use offset-based (page/page_size) pattern

---

### Task 1: Update Proto Template with Naming and Validation

**Files:**
- Modify: `internal/assets/_data/kitex/kitex-template/ratelimit_proto.yaml`

**Interfaces:**
- Produces: Updated proto definition with `Req/Resp` naming, `RateLimitConfig` message, PGV validation, pagination fields

- [ ] **Step 1: Read current template**

```bash
cat internal/assets/_data/kitex/kitex-template/ratelimit_proto.yaml
```

- [ ] **Step 2: Rewrite proto definition**

Replace entire file content with:

```yaml
# Kitex rule-center preset — proto definition
path: idl/rule-center.proto
update_behavior:
  type: cover
body: |-
  syntax = "proto3";
  package ratelimit.v1;

  option go_package = "api/ratelimit/v1;ratelimitv1";

  service RuleService {
    rpc GetRule(GetRuleReq) returns (GetRuleResp);
    rpc CreateRule(CreateRuleReq) returns (CreateRuleResp);
    rpc UpdateRule(UpdateRuleReq) returns (UpdateRuleResp);
    rpc DeleteRule(DeleteRuleReq) returns (DeleteRuleResp);
    rpc ListRules(ListRulesReq) returns (ListRulesResp);
  }

  message GetRuleReq {
    string service = 1 [(validate.rules).string = { min_len: 1, max_len: 100 }];
    string phase = 2 [(validate.rules).string = { min_len: 1, max_len: 50 }];
    string method = 3 [(validate.rules).string = { max_len: 100 }];
    string path = 4 [(validate.rules).string = { max_len: 255 }];
    optional string app_key = 5;
  }

  message GetRuleResp {
    bool found = 1;
    RateLimitRule rule = 2;
  }

  message CreateRuleReq {
    string service = 1 [(validate.rules).string = { min_len: 1, max_len: 100 }];
    string phase = 2 [(validate.rules).string = { min_len: 1, max_len: 50 }];
    string method = 3 [(validate.rules).string = { max_len: 100 }];
    string path = 4 [(validate.rules).string = { max_len: 255 }];
    string match_kind = 5 [(validate.rules).string = { in: ["exact", "prefix", "regex"] }];
    string path_pattern = 6;
    optional string app_key = 7;
    RateLimitConfig config = 8;
  }

  message CreateRuleResp {
    int64 id = 1;
  }

  message UpdateRuleReq {
    int64 id = 1;
    optional bool enabled = 2;
    optional int32 priority = 3;
    repeated string key_by = 4 [(validate.rules).repeated = { max_items: 10 }];
    optional string strategy = 5;
    optional int32 window_seconds = 6;
    optional int32 max_requests = 7;
    optional double requests_per_second = 8;
    optional int32 burst = 9;
    optional int32 client_ttl_seconds = 10;
  }

  message UpdateRuleResp {}

  message DeleteRuleReq {
    int64 id = 1;
  }

  message DeleteRuleResp {}

  message ListRulesReq {
    optional string service = 1;
    optional string phase = 2;
    int32 page = 3;
    int32 page_size = 4;
  }

  message ListRulesResp {
    repeated RateLimitRule rules = 1;
    int32 total = 2;
    int32 page = 3;
    int32 page_size = 4;
  }

  message RateLimitConfig {
    int32 priority = 1 [(validate.rules).int32 = { gte: 0, lte: 10000 }];
    bool enabled = 2;
    repeated string key_by = 3 [(validate.rules).repeated = { max_items: 10 }];
    string strategy = 4;
    int32 window_seconds = 5 [(validate.rules).int32 = { gt: 0, lte: 86400 }];
    int32 max_requests = 6 [(validate.rules).int32 = { gt: 0 }];
    double requests_per_second = 7;
    int32 burst = 8;
    int32 client_ttl_seconds = 9;
  }

  message RateLimitRule {
    int64 id = 1;
    string service = 2;
    string phase = 3;
    string method = 4;
    string match_kind = 5;
    string path = 6;
    string path_pattern = 7;
    optional string app_key = 8;
    int32 priority = 9;
    bool enabled = 10;
    repeated string key_by = 11;
    string strategy = 12;
    int32 window_seconds = 13;
    int32 max_requests = 14;
    double requests_per_second = 15;
    int32 burst = 16;
    int32 client_ttl_seconds = 17;
  }
```

- [ ] **Step 3: Commit template change**

```bash
git add internal/assets/_data/kitex/kitex-template/ratelimit_proto.yaml
git commit -m "fix(preset): update rule-center proto with Req/Resp naming and validation"
```

---

### Task 2: Update Handler Template References

**Files:**
- Modify: `internal/assets/_data/kitex/kitex-template/ratelimit_handler.yaml`

**Interfaces:**
- Consumes: Updated proto types from Task 1
- Produces: Handler code using `Req/Resp` types

- [ ] **Step 1: Read handler template**

```bash
cat internal/assets/_data/kitex/kitex-template/ratelimit_handler.yaml
```

- [ ] **Step 2: Replace all Request/Response with Req/Resp**

Find and replace:
- `GetRuleRequest` → `GetRuleReq`
- `GetRuleResponse` → `GetRuleResp`
- `CreateRuleRequest` → `CreateRuleReq`
- `CreateRuleResponse` → `CreateRuleResp`
- `UpdateRuleRequest` → `UpdateRuleReq`
- `UpdateRuleResponse` → `UpdateRuleResp`
- `DeleteRuleRequest` → `DeleteRuleReq`
- `DeleteRuleResponse` → `DeleteRuleResp`
- `ListRulesRequest` → `ListRulesReq`
- `ListRulesResponse` → `ListRulesResp`

- [ ] **Step 3: Update CreateRule handler to use config field**

Change handler to access nested config:
```go
// Before
req.Priority, req.Enabled, req.Strategy, etc.

// After
req.Config.Priority, req.Config.Enabled, req.Config.Strategy, etc.
```

- [ ] **Step 4: Update ListRules handler to support pagination**

Add pagination logic:
```go
page := req.GetPage()
if page < 1 {
    page = 1
}
pageSize := req.GetPageSize()
if pageSize < 1 || pageSize > 100 {
    pageSize = 20
}
```

- [ ] **Step 5: Commit handler changes**

```bash
git add internal/assets/_data/kitex/kitex-template/ratelimit_handler.yaml
git commit -m "fix(preset): update handler to use new proto types and pagination"
```

---

### Task 3: Update Usecase Template References

**Files:**
- Modify: `internal/assets/_data/kitex/kitex-template/ratelimit_usecase.yaml`

**Interfaces:**
- Consumes: Updated proto types from Task 1
- Produces: Usecase code using `Req/Resp` types and `RateLimitConfig`

- [ ] **Step 1: Read usecase template**

```bash
cat internal/assets/_data/kitex/kitex-template/ratelimit_usecase.yaml
```

- [ ] **Step 2: Replace all Request/Response with Req/Resp**

Same replacements as Task 2.

- [ ] **Step 3: Update CreateRule usecase to use RateLimitConfig**

Change database insert to use nested config:
```go
// Before
Priority: req.Priority,
Enabled: req.Enabled,

// After
Priority: req.Config.Priority,
Enabled: req.Config.Enabled,
```

- [ ] **Step 4: Update ListRules usecase to support pagination**

Add offset calculation:
```go
offset := (page - 1) * pageSize
```

- [ ] **Step 5: Commit usecase changes**

```bash
git add internal/assets/_data/kitex/kitex-template/ratelimit_usecase.yaml
git commit -m "fix(preset): update usecase to use new proto types and pagination"
```

---

### Task 4: Update Rule Center Client Template

**Files:**
- Modify: `internal/assets/_data/ratelimit/rule_center_client.yaml`

**Interfaces:**
- Consumes: Updated proto types from Task 1
- Produces: Client code using `Req/Resp` types

- [ ] **Step 1: Read client template**

```bash
cat internal/assets/_data/ratelimit/rule_center_client.yaml
```

- [ ] **Step 2: Replace all Request/Response with Req/Resp**

Same replacements as previous tasks.

- [ ] **Step 3: Commit client changes**

```bash
git add internal/assets/_data/ratelimit/rule_center_client.yaml
git commit -m "fix(preset): update rule center client to use Req/Resp types"
```

---

### Task 5: Update Golden Test Data — Proto File

**Files:**
- Modify: `internal/scaffold/rpc/testdata/rpc-rulecenter/idl/rule-center.proto`

**Interfaces:**
- Consumes: Updated proto from Task 1
- Produces: Updated golden test proto

- [ ] **Step 1: Copy updated proto to test data**

```bash
# Extract proto body from template and write to test file
# Use same content as Task 1 Step 2
```

- [ ] **Step 2: Commit test data update**

```bash
git add internal/scaffold/rpc/testdata/rpc-rulecenter/idl/rule-center.proto
git commit -m "test: update rule-center golden test proto"
```

---

### Task 6: Update Golden Test Data — Template Files

**Files:**
- Modify: `internal/scaffold/rpc/testdata/rpc-rulecenter/template/kitex-template/ratelimit_proto.yaml`
- Modify: `internal/scaffold/rpc/testdata/rpc-rulecenter/template/kitex-template/ratelimit_handler.yaml`
- Modify: `internal/scaffold/rpc/testdata/rpc-rulecenter/template/kitex-template/ratelimit_usecase.yaml`
- Modify: `internal/scaffold/rpc/testdata/rpc-rulecenter/template/kitex-template/ratelimit_shared_rule_center_client.yaml`

**Interfaces:**
- Consumes: Updated templates from Tasks 1-4
- Produces: Updated golden test templates

- [ ] **Step 1: Copy updated templates to test data**

Copy each file from `internal/assets/_data/` to corresponding test data location.

- [ ] **Step 2: Commit test data updates**

```bash
git add internal/scaffold/rpc/testdata/rpc-rulecenter/template/kitex-template/
git commit -m "test: update rule-center golden test templates"
```

---

### Task 7: Update Golden Test Data — Mono Client

**Files:**
- Modify: `internal/scaffold/mono/testdata/mono-with-rulecenter/internal/pkg/middleware/rule_center_client.go`

**Interfaces:**
- Consumes: Updated proto types from Task 1
- Produces: Updated mono test client code

- [ ] **Step 1: Read mono client test data**

```bash
cat internal/scaffold/mono/testdata/mono-with-rulecenter/internal/pkg/middleware/rule_center_client.go
```

- [ ] **Step 2: Replace all Request/Response with Req/Resp**

Same replacements as previous tasks.

- [ ] **Step 3: Commit mono test data update**

```bash
git add internal/scaffold/mono/testdata/mono-with-rulecenter/internal/pkg/middleware/rule_center_client.go
git commit -m "test: update mono rule-center client golden test"
```

---

### Task 8: Run Tests and Update Golden Files

**Files:**
- Test: All test files in `internal/scaffold/`

**Interfaces:**
- Consumes: All updated templates and test data from Tasks 1-7
- Produces: Passing test suite

- [ ] **Step 1: Run scaffold tests**

```bash
go test ./internal/scaffold/... -count=1
```

Expected: Tests should pass. If golden tests fail, review differences.

- [ ] **Step 2: Update golden tests if needed**

```bash
go test ./internal/scaffold/... -update-golden -count=1
```

- [ ] **Step 3: Run full test suite**

```bash
go test ./... -count=1
```

Expected: All tests pass.

- [ ] **Step 4: Commit any golden test updates**

```bash
git add internal/scaffold/
git commit -m "test: update golden test snapshots"
```

---

### Task 9: Validate with ncgo doctor

**Files:**
- None (validation only)

**Interfaces:**
- Consumes: All changes from Tasks 1-8
- Produces: Validation report

- [ ] **Step 1: Create test rule-center project**

```bash
cd /tmp
rm -rf test-rulecenter
ncgo new rule-center \
  --module example.com/test-rulecenter \
  --kind kitex --db postgres --preset rule-center \
  --dir test-rulecenter --no-generate
```

- [ ] **Step 2: Run ncgo doctor**

```bash
cd test-rulecenter
ncgo doctor --root .
```

Expected: No protolint errors related to PIO101/PIO102.

- [ ] **Step 3: Verify proto file**

```bash
cat idl/rule-center.proto | head -20
```

Expected: Uses `Req/Resp` naming.

- [ ] **Step 4: Clean up test project**

```bash
cd /Volumes/SSD/workspace/github.com/byx-darwin/ncgo
rm -rf /tmp/test-rulecenter
```

---

### Task 10: Final Validation and Documentation

**Files:**
- None (final checks)

**Interfaces:**
- Consumes: All changes from Tasks 1-9
- Produces: Complete implementation

- [ ] **Step 1: Run full validation**

```bash
go build ./...
go vet ./...
go test ./... -count=1
```

Expected: All pass.

- [ ] **Step 2: Verify all acceptance criteria**

- [ ] All 21 protolint errors resolved
- [ ] `ncgo doctor` passes on generated rule-center project
- [ ] All existing tests pass
- [ ] New rule-center project builds successfully
- [ ] Template files updated (4 files)
- [ ] Test data updated (6 files)

- [ ] **Step 3: Create summary commit**

```bash
git log --oneline -10
```

Verify all commits are present.

- [ ] **Step 4: Push changes**

```bash
git push origin main
```

---

## Execution Summary

**Total tasks:** 10  
**Estimated time:** 2-3 hours  
**Risk level:** Medium (breaking changes to generated code)

**Key changes:**
1. Proto naming: `Request/Response` → `Req/Resp`
2. CreateRuleReq: 16 fields → 8 fields (nested `RateLimitConfig`)
3. ListRulesReq: added pagination (page/page_size)
4. PGV validation: string min/max, numeric ranges, repeated max_items

**Breaking changes:**
- Existing rule-center projects must regenerate or manually update code
- API contracts change (nested config field, pagination fields)
