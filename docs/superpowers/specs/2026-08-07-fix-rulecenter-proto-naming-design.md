# Fix Rule-Center Preset Proto Naming and Validation

**Date**: 2026-08-07  
**Status**: Approved  
**Workflow**: fix-rulecenter-proto-naming (standard mode)

---

## Problem Statement

The ncgo `rule-center` preset template violates ncgo's own protolint conventions:

1. **Naming convention violation**: Uses `Request/Response` suffix instead of `Req/Resp`
2. **Missing PGV validation**: No data validation constraints
3. **Missing pagination**: `ListRules` has no pagination support
4. **Excessive fields**: `CreateRuleRequest` has 16 fields (threshold: 12)

This causes all projects created with `ncgo new --preset rule-center` to fail `ncgo doctor` checks with 21 protolint errors.

---

## Design Goals

1. Fix all protolint violations in the rule-center preset
2. Add PGV validation constraints for data integrity
3. Add pagination support for list operations
4. Refactor large messages using nested structures
5. Maintain backward compatibility where possible (generated code will change, but API semantics remain)

---

## Solution Overview

### 1. Naming Convention Fix

Rename all RPC message types from `Request/Response` to `Req/Resp`:

```
GetRuleRequest → GetRuleReq
GetRuleResponse → GetRuleResp
CreateRuleRequest → CreateRuleReq
CreateRuleResponse → CreateRuleResp
UpdateRuleRequest → UpdateRuleReq
UpdateRuleResponse → UpdateRuleResp
DeleteRuleRequest → DeleteRuleReq
DeleteRuleResponse → DeleteRuleResp
ListRulesRequest → ListRulesReq
ListRulesResponse → ListRulesResp
```

### 2. CreateRuleReq Refactoring (Nested Message)

Extract rate-limit configuration into a reusable `RateLimitConfig` message:

```protobuf
message CreateRuleReq {
  // Base identification (7 fields)
  string service = 1 [(validate.rules).string = { min_len: 1, max_len: 100 }];
  string phase = 2 [(validate.rules).string = { min_len: 1, max_len: 50 }];
  string method = 3 [(validate.rules).string = { max_len: 100 }];
  string path = 4 [(validate.rules).string = { max_len: 255 }];
  string match_kind = 5 [(validate.rules).string = { in: ["exact", "prefix", "regex"] }];
  string path_pattern = 6;
  optional string app_key = 7;

  // Rate-limit configuration (nested message)
  RateLimitConfig config = 8;
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
```

**Benefits**:
- Reduces `CreateRuleReq` from 16 to 8 fields (below threshold)
- `RateLimitConfig` can be reused in other messages
- Clearer separation of concerns

### 3. Pagination Support (Offset-based)

Add pagination to `ListRulesReq` and `ListRulesResp`:

```protobuf
message ListRulesReq {
  optional string service = 1;
  optional string phase = 2;
  int32 page = 3;        // Page number (1-based, default 1)
  int32 page_size = 4;   // Items per page (default 20, max 100)
}

message ListRulesResp {
  repeated RateLimitRule rules = 1;
  int32 total = 2;       // Total record count
  int32 page = 3;
  int32 page_size = 4;
}
```

**Rationale**: Offset-based pagination is simpler to implement and sufficient for typical rule counts (tens to hundreds).

### 4. PGV Validation Rules

Add validation constraints to key fields:

**String fields**:
- `service`: `min_len: 1, max_len: 100`
- `phase`: `min_len: 1, max_len: 50`
- `method`: `max_len: 100`
- `path`: `max_len: 255`
- `match_kind`: `in: ["exact", "prefix", "regex"]`

**Numeric fields**:
- `priority`: `gte: 0, lte: 10000`
- `window_seconds`: `gt: 0, lte: 86400` (1 day max)
- `max_requests`: `gt: 0`

**Repeated fields**:
- `key_by`: `max_items: 10`

**Rationale**: Prevent invalid data at the API boundary without being overly restrictive.

### 5. UpdateRuleReq Adjustments

`UpdateRuleReq` also uses validation for `key_by`:

```protobuf
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
```

**Note**: `UpdateRuleReq` keeps flat structure because it uses optional fields for partial updates.

---

## Implementation Plan

### Phase 1: Update Template Files (4 files)

1. `internal/assets/_data/kitex/kitex-template/ratelimit_proto.yaml`
   - Rename all message types
   - Add `RateLimitConfig` message
   - Add PGV validation rules
   - Add pagination fields

2. `internal/assets/_data/kitex/kitex-template/ratelimit_handler.yaml`
   - Update references to renamed types

3. `internal/assets/_data/kitex/kitex-template/ratelimit_usecase.yaml`
   - Update references to renamed types
   - Adjust code to use `config` nested field

4. `internal/assets/_data/ratelimit/rule_center_client.yaml`
   - Update references to renamed types

### Phase 2: Update Test Data (6 files)

5. `internal/scaffold/rpc/testdata/rpc-rulecenter/idl/rule-center.proto`
6. `internal/scaffold/rpc/testdata/rpc-rulecenter/template/kitex-template/ratelimit_proto.yaml`
7. `internal/scaffold/rpc/testdata/rpc-rulecenter/template/kitex-template/ratelimit_handler.yaml`
8. `internal/scaffold/rpc/testdata/rpc-rulecenter/template/kitex-template/ratelimit_usecase.yaml`
9. `internal/scaffold/rpc/testdata/rpc-rulecenter/template/kitex-template/ratelimit_shared_rule_center_client.yaml`
10. `internal/scaffold/mono/testdata/mono-with-rulecenter/internal/pkg/middleware/rule_center_client.go`

### Phase 3: Validation

- Run `ncgo protolint --root .` on generated rule-center project
- Run `go test ./internal/scaffold/...` to update golden tests
- Run `go test ./... -count=1` to ensure no regressions

---

## Breaking Changes

**This is a breaking change for existing rule-center projects**:

1. Generated Go code will have different type names (`Req/Resp` instead of `Request/Response`)
2. `CreateRule` API changes: fields moved to `config` nested message
3. `ListRules` API changes: added pagination fields

**Migration path**: Users must regenerate their rule-center service or manually update their code.

---

## Testing Strategy

1. **Unit tests**: Existing tests should pass after golden test updates
2. **Integration tests**: Create a new rule-center project and verify:
   - `ncgo doctor` passes with no protolint errors
   - `make sqlc` generates correct code
   - `go build` succeeds
   - Service starts and handles requests

---

## Success Criteria

- [ ] All 21 protolint errors resolved
- [ ] `ncgo doctor` passes on generated rule-center project
- [ ] All existing tests pass
- [ ] New rule-center project builds and runs successfully
- [ ] Documentation updated (if needed)

---

## Appendix: Complete Proto Definition

<details>
<summary>Click to expand full proto definition</summary>

```protobuf
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

</details>
