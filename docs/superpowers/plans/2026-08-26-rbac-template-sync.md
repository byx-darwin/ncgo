# Sync ncgo-templates RBAC to s-web Alignment Decisions — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将 ncgo-templates 仓库中 RBAC 相关模板同步到 ncgo 已锁定的 s-web 对齐决策（string ID + ValidateToken 载荷），消除设计规格与模板现状之间的差距。

**Architecture:** 自底向上逐层变更：proto → handler → repository → migration → domain → application。先改 rbac-kitex（源头），再同步 admin-bff-hertz / admin-services-kitex / micro-admin（消费方）。

**Tech Stack:** Go, Kitex (RPC), Hertz (HTTP), Protobuf, sqlc, PostgreSQL, Casbin

**Spec:** `docs/specs/2026-08-18-rbac-kitex-design.md`（ncgo 仓库）+ `docs/specs/2026-08-19-rbac-kitex-s-web-alignment.md`（锁定决策）

**Working Directory:** 本计划的修改发生在 `../ncgo-templates` 仓库，而非当前 ncgo 仓库。所有路径相对于 ncgo-templates 根目录。

## Global Constraints

- **string ID 决策**：所有实体 ID 使用 `string` 类型（uuid v4, TEXT PK），不再使用 `int64` / `BIGSERIAL`
- **ValidateToken 载荷**：`ValidateTokenResp { string uid = 1; repeated string role_codes = 2; bool valid = 3; }`
- **Enforce 载荷**：`EnforceReq { string uid = 1; string obj = 2; string act = 3; }`
- **AssignRolesToUser**：`string user_id + repeated string role_ids`
- **GrantPermissionsToRole**：`string role_id + repeated string permission_codes`
- **int32 status**：1=enabled, 0=disabled（已对齐，保持不变）
- **单一 Permission 树**：已对齐，保持不变
- **模板文件为 `.yaml` 格式**（ncgo exported template），修改 `body:` 中的代码内容
- **每完成一个 template 后**，需运行 `ncgo export` 验证模板可导出

---

## File Structure

```
rbac-kitex/
├── idl/auth.proto                              # Task 1: proto ID 类型
├── kitex-template/
│   ├── internal_handler_authservice_handler_go.yaml    # Task 2: ValidateToken 载荷
│   ├── internal_handler_rbacservice_handler_go.yaml    # Task 2: handler ID 参数
│   ├── internal_repository_user_repo_go.yaml           # Task 3: user repo ID
│   ├── internal_repository_role_repo_go.yaml           # Task 3: role repo ID
│   ├── internal_repository_permission_repo_go.yaml     # Task 3: permission repo ID
│   ├── internal_repository_menu_repo_go.yaml           # Task 3: menu repo ID
│   ├── internal_domain_user_repository_go.yaml         # Task 4: domain interface
│   ├── internal_domain_role_repository_go.yaml         # Task 4: domain interface
│   ├── internal_domain_permission_repository_go.yaml   # Task 4: domain interface
│   ├── internal_domain_menu_repository_go.yaml         # Task 4: domain interface
│   ├── internal_application_user_dto_go.yaml           # Task 5: DTO
│   ├── internal_application_rbac_dto_go.yaml           # Task 5: DTO
│   ├── internal_application_menu_dto_go.yaml           # Task 5: DTO
│   ├── migration_init.yaml                             # Task 6: DDL ID 类型
│   └── *_test_go.yaml                                  # Task 7: 测试同步

admin-bff-hertz/
├── idl/auth.proto                              # Task 8: 更新 proto 引用
├── idl/rbac.proto                              # Task 8: 更新 proto 引用
└── hertz-template/
    ├── internal_handler_current_user_go.yaml     # Task 8: 修复 Uid TODO
    ├── internal_handler_permission_go.yaml       # Task 8: 修复模块路径
    └── internal_handler_user_go.yaml             # Task 8: 修复模块路径（如需要）

admin-services-kitex/
├── idl/admin.proto                             # Task 9: string ID
└── kitex-template/                             # Task 9: 同步变更

micro-admin/
├── idl/rbac.proto                              # Task 10: 同步 rbac.proto
└── idl/auth.proto                              # Task 10: 同步 auth.proto
```

---

## Task 1: rbac-kitex proto — string ID 类型

**Files:**
- Modify: `../ncgo-templates/rbac-kitex/idl/auth.proto`

**Interfaces:**
- Consumes: —
- Produces: `auth.proto` with all ID fields as `string`

- [ ] **Step 1: 修改 ValidateTokenResp**

将 `int64 uid` → `string uid`，`repeated string roles` → `repeated string role_codes`

```proto
message ValidateTokenResp {
  string uid = 1;
  repeated string role_codes = 2;
  bool valid = 3;
}
```

- [ ] **Step 2: 修改 User 消息**

```proto
message User {
  string id = 1;
  string username = 2;
  int32 status = 3;
  repeated string roles = 4;
  string nickname = 5;
  string avatar = 6;
  string email = 7;
  string phone = 8;
}
```

- [ ] **Step 3: 修改 CreateUserReq/UpdateUserReq/DeleteUserReq/GetUserReq**

```proto
message CreateUserReq {
  string username = 1;
  string password = 2;
  string nickname = 3;
  string avatar = 4;
  string email = 5;
  string phone = 6;
}
message UpdateUserReq {
  string id = 1;
  optional string password = 2;
  optional int32 status = 3;
  optional string nickname = 4;
  optional string avatar = 5;
  optional string email = 6;
  optional string phone = 7;
}
message DeleteUserReq { string id = 1; }
message GetUserReq { string id = 1; }
```

- [ ] **Step 4: 修改 Role 消息**

```proto
message Role {
  string id = 1;
  string code = 2;
  string name = 3;
  int32 status = 4;
  string remark = 5;
  repeated string permissions = 6;
}
message CreateRoleReq { string code = 1; string name = 2; string remark = 3; }
message UpdateRoleReq {
  string id = 1;
  string name = 2;
  optional int32 status = 3;
  optional string remark = 4;
}
message DeleteRoleReq { string id = 1; }
```

- [ ] **Step 5: 修改 Permission 消息**

```proto
message Permission {
  string id = 1;
  string code = 2;
  string type = 3;
  string name = 4;
  string parent_id = 5;
  string path = 6;
  string icon = 7;
  string route_name = 8;
  string redirect = 9;
  bool keep_alive = 10;
  bool hide_in_menu = 11;
  bool is_external = 12;
  string method = 13;
  int32 sort = 14;
  int32 status = 15;
  string description = 16;
}
message CreatePermissionReq {
  string code = 1;
  string type = 2;
  string name = 3;
  string parent_id = 4;
  string path = 5;
  string icon = 6;
  string route_name = 7;
  string redirect = 8;
  bool keep_alive = 9;
  bool hide_in_menu = 10;
  bool is_external = 11;
  string method = 12;
  int32 sort = 13;
  int32 status = 14;
  string description = 15;
}
message UpdatePermissionReq {
  string id = 1;
  optional string code = 2;
  optional string type = 3;
  optional string name = 4;
  optional string parent_id = 5;
  optional string path = 6;
  optional string icon = 7;
  optional string route_name = 8;
  optional string redirect = 9;
  optional bool keep_alive = 10;
  optional bool hide_in_menu = 11;
  optional bool is_external = 12;
  optional string method = 13;
  optional int32 sort = 14;
  optional int32 status = 15;
  optional string description = 16;
}
message GetPermissionReq { string id = 1; }
message DeletePermissionReq { string id = 1; }
message ListPermissionsReq {
  int32 page = 1;
  int32 page_size = 2;
  optional string type = 3;
  optional string parent_id = 4;
  optional int32 status = 5;
}
```

- [ ] **Step 6: 修改 Menu 消息**

```proto
message Menu {
  string id = 1;
  string code = 2;
  string name = 3;
  string parent_id = 4;
  string type = 5;
  string path = 6;
  string icon = 7;
  string route_name = 8;
  string redirect = 9;
  bool keep_alive = 10;
  bool hide_in_menu = 11;
  bool is_external = 12;
  int32 sort = 13;
}
```

- [ ] **Step 7: 修改 Enforce/GetUserMenuTree/GetUserPermCodes/Assign/Grant**

```proto
message AssignRolesToUserReq { string user_id = 1; repeated string role_ids = 2; }
message GrantPermissionsToRoleReq {
  string role_id = 1;
  repeated string permission_codes = 2;
}
message EnforceReq { string uid = 1; string obj = 2; string act = 3; }
message GetUserMenuTreeReq { string uid = 1; }
message GetUserPermCodesReq { string uid = 1; }
```

- [ ] **Step 8: 验证 proto 语法**

```bash
cd ../ncgo-templates/rbac-kitex && protoc --proto_path=idl --decode_raw < /dev/null idl/auth.proto 2>&1 | head -5
# 或者直接检查文件完整性
grep -c "int64 id\|int64 uid" idl/auth.proto  # 应返回 0
grep -c "string id\|string uid" idl/auth.proto  # 应返回多行
```

- [ ] **Step 9: Commit**

```bash
cd ../ncgo-templates && git add rbac-kitex/idl/auth.proto
git commit -m "refactor(rbac-kitex): change all ID fields from int64 to string (uuid v4)"
```

---

## Task 2: rbac-kitex handler — ValidateToken + ID 参数适配

**Files:**
- Modify: `../ncgo-templates/rbac-kitex/kitex-template/internal_handler_authservice_handler_go.yaml`
- Modify: `../ncgo-templates/rbac-kitex/kitex-template/internal_handler_rbacservice_handler_go.yaml`

**Interfaces:**
- Consumes: `auth.proto` (Task 1 output)
- Produces: handler code compatible with string IDs

- [ ] **Step 1: 修改 auth handler ValidateToken**

在 `internal_handler_authservice_handler_go.yaml` 中，修改 ValidateToken 方法返回值的字段名：

```go
func (s *AuthServiceImpl) ValidateToken(ctx context.Context, req *v1.ValidateTokenReq) (resp *v1.ValidateTokenResp, err error) {
	uid, roleCodes, valid := s.auth.ValidateToken(ctx, req.AccessToken)
	return &v1.ValidateTokenResp{Uid: uid, RoleCodes: roleCodes, Valid: valid}, nil
}
```

- [ ] **Step 2: 修改 rbacservice handler 中的 ID 参数转换**

在 `internal_handler_rbacservice_handler_go.yaml` 中，找到所有 `strconv.ParseInt(idStr, 10, 64)` 调用，改为直接使用 string：

```go
// Before:
id, _ := strconv.ParseInt(idStr, 10, 64)
resp, err := h.rbacCli.GetUser(ctx, &api.GetUserReq{Id: id})

// After:
resp, err := h.rbacCli.GetUser(ctx, &api.GetUserReq{Id: idStr})
```

- [ ] **Step 3: 移除不再需要的 strconv import**

检查 handler 中是否还有其他 `strconv` 使用，如果没有则移除 import。

- [ ] **Step 4: Commit**

```bash
cd ../ncgo-templates && git add rbac-kitex/kitex-template/internal_handler_authservice_handler_go.yaml rbac-kitex/kitex-template/internal_handler_rbacservice_handler_go.yaml
git commit -m "refactor(rbac-kitex): adapt handler to string ID + role_codes"
```

---

## Task 3: rbac-kitex repository — string ID

**Files:**
- Modify: `../ncgo-templates/rbac-kitex/kitex-template/internal_repository_user_repo_go.yaml`
- Modify: `../ncgo-templates/rbac-kitex/kitex-template/internal_repository_role_repo_go.yaml`
- Modify: `../ncgo-templates/rbac-kitex/kitex-template/internal_repository_permission_repo_go.yaml`
- Modify: `../ncgo-templates/rbac-kitex/kitex-template/internal_repository_menu_repo_go.yaml`

**Interfaces:**
- Consumes: `auth.proto` (Task 1)
- Produces: repository methods using string IDs

- [ ] **Step 1: 修改 user repository**

在 `internal_repository_user_repo_go.yaml` 中：
- `GetByID(ctx, id int64)` → `GetByID(ctx, id string)`
- `UpdatePassword(ctx, id int64, ...)` → `UpdatePassword(ctx, id string, ...)`
- `Delete(ctx, id int64)` → `Delete(ctx, id string)`
- `SetStatus(ctx, id int64, ...)` → `SetStatus(ctx, id string, ...)`

- [ ] **Step 2: 修改 role repository**

同上模式，所有 `int64` ID 参数改为 `string`。

- [ ] **Step 3: 修改 permission repository**

同上模式。

- [ ] **Step 4: 修改 menu repository**

同上模式。注意 `parent_id` 也是 `string`。

- [ ] **Step 5: Commit**

```bash
cd ../ncgo-templates && git add rbac-kitex/kitex-template/internal_repository_*_repo_go.yaml
git commit -m "refactor(rbac-kitex): change repository ID params from int64 to string"
```

---

## Task 4: rbac-kitex domain repository interface

**Files:**
- Modify: `../ncgo-templates/rbac-kitex/kitex-template/internal_domain_user_repository_go.yaml`
- Modify: `../ncgo-templates/rbac-kitex/kitex-template/internal_domain_role_repository_go.yaml`
- Modify: `../ncgo-templates/rbac-kitex/kitex-template/internal_domain_permission_repository_go.yaml`
- Modify: `../ncgo-templates/rbac-kitex/kitex-template/internal_domain_menu_repository_go.yaml`

**Interfaces:**
- Consumes: Task 3 output
- Produces: domain repository interfaces with string IDs

- [ ] **Step 1: 修改各 domain repository 接口**

在每个 domain repository interface 文件中，将方法签名中的 `int64` ID 改为 `string`：

```go
// Before:
GetByID(ctx context.Context, id int64) (*user.User, error)
// After:
GetByID(ctx context.Context, id string) (*user.User, error)
```

- [ ] **Step 2: Commit**

```bash
cd ../ncgo-templates && git add rbac-kitex/kitex-template/internal_domain_*_repository_go.yaml
git commit -m "refactor(rbac-kitex): change domain repository interfaces to string ID"
```

---

## Task 5: rbac-kitex application DTO + service

**Files:**
- Modify: `../ncgo-templates/rbac-kitex/kitex-template/internal_application_user_dto_go.yaml`
- Modify: `../ncgo-templates/rbac-kitex/kitex-template/internal_application_rbac_dto_go.yaml`
- Modify: `../ncgo-templates/rbac-kitex/kitex-template/internal_application_menu_dto_go.yaml`

**Interfaces:**
- Consumes: Task 3, Task 4
- Produces: application layer DTOs with string IDs

- [ ] **Step 1: 修改 DTO 中的 ID 字段类型**

在每个 DTO 文件中，将 `int64` ID 字段改为 `string`。

- [ ] **Step 2: 修改 service 层的方法签名**

在 application service 文件中，将涉及 ID 的方法参数从 `int64` 改为 `string`。

- [ ] **Step 3: Commit**

```bash
cd ../ncgo-templates && git add rbac-kitex/kitex-template/internal_application_*_go.yaml
git commit -m "refactor(rbac-kitex): change application DTOs and services to string ID"
```

---

## Task 6: rbac-kitex migration — BIGSERIAL → TEXT

**Files:**
- Modify: `../ncgo-templates/rbac-kitex/kitex-template/migration_init.yaml`

**Interfaces:**
- Consumes: Task 1 (string ID decision)
- Produces: DDL using TEXT PK for all entity IDs

- [ ] **Step 1: 修改 users 表**

```sql
-- Before:
id BIGSERIAL PRIMARY KEY,
-- After:
id TEXT PRIMARY KEY,  -- uuid v4, app-generated
```

- [ ] **Step 2: 修改 roles 表**

```sql
id TEXT PRIMARY KEY,  -- uuid v4
```

- [ ] **Step 3: 修改 permissions 表**

```sql
id TEXT PRIMARY KEY,  -- uuid v4
parent_id TEXT REFERENCES permissions(id),  -- was BIGINT
```

- [ ] **Step 4: 修改 user_roles 表**

```sql
user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,  -- was BIGINT
role_id TEXT NOT NULL REFERENCES roles(id) ON DELETE CASCADE,  -- was BIGINT
```

- [ ] **Step 5: 修改 role_permissions 表**

```sql
role_id TEXT NOT NULL REFERENCES roles(id) ON DELETE CASCADE,  -- was BIGINT
permission_id TEXT NOT NULL REFERENCES permissions(id) ON DELETE CASCADE,  -- was BIGINT
```

- [ ] **Step 6: 修改 audit_log 表**

```sql
actor_uid TEXT,  -- was BIGINT
```

- [ ] **Step 7: casbin_rule 保持不变**（v0..v5 已是 TEXT）

- [ ] **Step 8: Commit**

```bash
cd ../ncgo-templates && git add rbac-kitex/kitex-template/migration_init.yaml
git commit -m "refactor(rbac-kitex): change DDL PK from BIGSERIAL to TEXT (uuid v4)"
```

---

## Task 7: rbac-kitex 测试同步

**Files:**
- Modify: `../ncgo-templates/rbac-kitex/kitex-template/internal_application_auth_auth_service_test_go.yaml`
- Modify: `../ncgo-templates/rbac-kitex/kitex-template/internal_repository_user_repo_test_go.yaml`
- Modify: `../ncgo-templates/rbac-kitex/kitex-template/internal_domain_user_entity_test_go.yaml`
- Modify: 其他涉及 ID 的测试文件

**Interfaces:**
- Consumes: Task 1-6
- Produces: tests using string IDs

- [ ] **Step 1: 更新测试中的 ID 字面量**

在所有测试文件中，将 `int64` 类型的 ID 断言/参数改为 `string`：

```go
// Before:
user, err := repo.GetByID(ctx, 1)
// After:
user, err := repo.GetByID(ctx, "550e8400-e29b-41d4-a716-446655440000")
```

- [ ] **Step 2: 更新 ValidateToken 测试断言**

```go
// Before:
assert.Equal(t, int64(123), resp.Uid)
// After:
assert.Equal(t, "550e8400-e29b-41d4-a716-446655440000", resp.Uid)
```

- [ ] **Step 3: 运行 e2e 验证（如 ncgo-templates 支持）**

```bash
cd ../ncgo-templates/rbac-kitex && make test 2>&1 | tail -20
```

- [ ] **Step 4: Commit**

```bash
cd ../ncgo-templates && git add rbac-kitex/kitex-template/
git commit -m "test(rbac-kitex): update tests for string ID + role_codes"
```

---

## Task 8: admin-bff-hertz — 修复硬编码路径 + proto 引用 + Uid TODO

**Files:**
- Modify: `../ncgo-templates/admin-bff-hertz/hertz-template/internal_handler_current_user_go.yaml`
- Modify: `../ncgo-templates/admin-bff-hertz/hertz-template/internal_handler_permission_go.yaml`
- Modify: `../ncgo-templates/admin-bff-hertz/hertz-template/internal_handler_user_go.yaml`（如存在）
- Modify: `../ncgo-templates/admin-bff-hertz/idl/auth.proto`（更新引用）
- Modify: `../ncgo-templates/admin-bff-hertz/idl/rbac.proto`（更新引用）

**Interfaces:**
- Consumes: Task 1 (new auth.proto)
- Produces: BFF handlers compatible with rbac-kitex string IDs

- [ ] **Step 1: 修复 currentUser handler 中的 Uid TODO**

在 `internal_handler_current_user_go.yaml` 中：

```go
// Before:
resp, err := h.rbacCli.GetUserMenuTree(ctx, &api.GetUserMenuTreeReq{
    Uid: 0, // TODO: convert UUID to int64
})

// After:
resp, err := h.rbacCli.GetUserMenuTree(ctx, &api.GetUserMenuTreeReq{
    Uid: uid,  // uid from JWT claims (string uuid)
})
```

- [ ] **Step 2: 修复 permission handler 中的硬编码模块路径**

在 `internal_handler_permission_go.yaml` 中：

```go
// Before:
api "github.com/test/{{ToLower .ServiceName}}/services/authority/kitex_gen/api/admin/v1"
"github.com/test/{{ToLower .ServiceName}}/services/authority/kitex_gen/api/admin/v1/rbacservice"

// After:
api "{{.Module}}/kitex_gen/api/rbac/v1"
rbacservice "{{.Module}}/kitex_gen/api/rbac/v1/rbacservice"
```

- [ ] **Step 3: 修复 user handler 中的硬编码模块路径**（如存在）

同上模式。

- [ ] **Step 4: 更新 idl/auth.proto 和 idl/rbac.proto 引用**

确认 BFF 的 IDL 文件引用与 rbac-kitex 模板对齐（api/rbac.v1 包名）。

- [ ] **Step 5: 修复 currentUser handler 中的 ValidateToken 调用**

```go
// Before:
_, ok := middleware.GetClaims(c)

// After (如需要 ValidateToken):
claims, ok := middleware.GetClaims(c)
if !ok {
    response.ErrorCode(c, response.CodeUnauthorized)
    return
}
uid := claims.Uid  // string uuid from JWT
```

- [ ] **Step 6: Commit**

```bash
cd ../ncgo-templates && git add admin-bff-hertz/
git commit -m "fix(admin-bff-hertz): remove hardcoded paths, fix Uid TODO, update proto refs"
```

---

## Task 9: admin-services-kitex — 同步 string ID

**Files:**
- Modify: `../ncgo-templates/admin-services-kitex/idl/admin.proto`
- Modify: `../ncgo-templates/admin-services-kitex/kitex-template/` (所有涉及 ID 的文件)

**Interfaces:**
- Consumes: Task 1
- Produces: admin-services-kitex template with string IDs

- [ ] **Step 1: 修改 admin.proto 中的 ID 字段**

与 Task 1 相同的模式，将所有 `int64 id/uid/user_id/role_id` 改为 `string`。

- [ ] **Step 2: 同步 handler/repository/domain/application 层**

与 Task 2-5 相同的模式。

- [ ] **Step 3: 同步 migration**

与 Task 6 相同的模式。

- [ ] **Step 4: Commit**

```bash
cd ../ncgo-templates && git add admin-services-kitex/
git commit -m "refactor(admin-services-kitex): sync string ID from rbac-kitex alignment"
```

---

## Task 10: micro-admin — 同步 rbac.proto 和 auth.proto

**Files:**
- Modify: `../ncgo-templates/micro-admin/idl/rbac.proto`
- Modify: `../ncgo-templates/micro-admin/idl/auth.proto`

**Interfaces:**
- Consumes: Task 1
- Produces: micro-admin IDL files matching rbac-kitex

- [ ] **Step 1: 更新 rbac.proto**

将 micro-admin 的 `idl/rbac.proto` 更新为与 rbac-kitex `idl/auth.proto` 中的 RBACService 对齐：
- 包名统一为 `api.rbac.v1`
- 添加 ValidateToken 相关消息
- 添加 AssignRolesToUser / GrantPermissionsToRole
- ID 类型改为 string

- [ ] **Step 2: 更新 auth.proto**

添加 ValidateToken 方法：

```proto
service AuthService {
  rpc Login(LoginRequest) returns (LoginResponse);
  rpc Refresh(RefreshRequest) returns (RefreshResponse);
  rpc Logout(LogoutRequest) returns (LogoutResponse);
  rpc ValidateToken(ValidateTokenRequest) returns (ValidateTokenResponse);
}

message ValidateTokenRequest { string access_token = 1; }
message ValidateTokenResponse {
  string uid = 1;
  repeated string role_codes = 2;
  bool valid = 3;
}
```

- [ ] **Step 3: Commit**

```bash
cd ../ncgo-templates && git add micro-admin/idl/
git commit -m "refactor(micro-admin): sync rbac.proto/auth.proto to rbac-kitex string ID"
```

---

## Task 11: 全量验证

**Files:**
- 无修改，仅验证

**Interfaces:**
- Consumes: Task 1-10
- Produces: 验证报告

- [ ] **Step 1: 验证 rbac-kitex proto 无 int64 ID**

```bash
cd ../ncgo-templates && grep -rn "int64 id\|int64 uid\|int64 user_id\|int64 role_id\|int64 permission_id" rbac-kitex/idl/ rbac-kitex/kitex-template/
# 应返回空
```

- [ ] **Step 2: 验证 admin-bff 无硬编码路径**

```bash
grep -rn "github.com/test" admin-bff-hertz/
# 应返回空
```

- [ ] **Step 3: 验证所有模板可导出**

```bash
cd ../ncgo-templates/rbac-kitex && ncgo export templates --kind kitex 2>&1 | tail -5
cd ../ncgo-templates/admin-bff-hertz && ncgo export templates --kind hertz 2>&1 | tail -5
```

- [ ] **Step 4: 创建验证报告**

输出每个模板的验证结果，确认所有 AC 满足。

---

## Dependencies (Task Execution Order)

```
Task 1 (proto) ─┬─ Task 2 (handler) ─ Task 3 (repo) ─ Task 4 (domain) ─ Task 5 (app) ─ Task 6 (migration) ─ Task 7 (test)
                 │
                 ├─ Task 8 (admin-bff) — 可独立并行
                 ├─ Task 9 (admin-services) — 可独立并行
                 └─ Task 10 (micro-admin) — 可独立并行

Task 11 (验证) 依赖所有前置 Task
```

**推荐执行顺序：**
1. Task 1（必须最先）
2. Task 2, 3, 4, 5, 6, 7（串行，rbac-kitex 完整链路）
3. Task 8, 9, 10（可与 Task 2-7 部分并行，但需在 Task 1 之后）
4. Task 11（最后验证）
