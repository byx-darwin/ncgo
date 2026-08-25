# Design: `rbac-kitex` — RBAC + Auth authority (Kitex RPC) template

Part of the **micro-admin** program (sub-project 2 of the 3-template decomposition). Program design: `2026-08-18-micro-admin-ddd-program-design.md`.

- **Type:** new Kitex RPC service **template** for `ncgo-templates`, built via `ncgo` + `ncgo export`.
- **Bar:** reference scaffold (correct structure, runnable happy path, documented production seams).
- **Role:** the RBAC + authentication **authority** — owns the DB and Casbin policy; `admin-bff-hertz` consumes it.

## Locked decisions

| Dimension | Decision |
|---|---|
| Framework | Kitex RPC (base-kitex layered layout + DDD layers) |
| Architecture | DDD: `internal/domain/<agg>` + `internal/application/<agg>` + `internal/repository/<agg>` (sqlc) |
| RBAC | Casbin, basic model `sub, obj, act` (no domain first version); data-scope/domain is a documented seam |
| Permission model | **单一 Permission 树**（合并原 menus）: `permissions` 自带 parent_id/path/icon/route_name/sort/status; kind ∈ catalog\|menu\|button\|api |
| Permission code | **Unified**: `permissions.code` == Casbin `obj` == 前端按钮/接口权限码; **同一 code 可同时为 button 与 api 记录**（api 记录携带 method） |
| Casbin storage | built-in **sqlc-based `persist.Adapter`** over `casbin_rule` (no gorm) |
| Single source | `casbin_rule` is the enforcement source; management writes (grant/assign) **sync into** the adapter (direction: mgmt tables → casbin) |
| ID type | **string**（uuid v4, TEXT PK）: users/roles/permissions/user_roles/role_permissions/audit_log.actor_uid 全为 string |
| Status | **int32 0/1** (1=enabled/0=disabled) on user/role/permission (对齐 s-web) |
| Grant/Assign payload | **GrantPermissionsToRole by permission_codes**; **AssignRolesToUser by role_ids** (对齐 s-web REST) |
| Auth | self-built **JWT (HS256 default)**, claims `{uid, roles}`; argon2id passwords + basic lockout; Redis refresh-token + JWT blacklist |
| Audit | basic `audit_log` (who/what/when) written by RBAC mutations |
| Build | develop real Kitex project with ncgo → hand-write DDD+casbin+JWT → build/test → `ncgo export` → `rbac-kitex` package |

## DDD structure (per aggregate)

Aggregates: **user, role, permission**（menu 树由 permission 聚合的 `BuildTree` 构建，非独立聚合）。

```
internal/domain/<agg>/
    entity.go        # entity/aggregate root (User, Role, Permission)
    valueobject.go   # VOs (e.g. PermissionCode, PasswordHash)
    service.go       # domain service (pure rules; e.g. password policy, role-permission invariants)
    repository.go    # repository PORT (interface) + domain errors
internal/application/<agg>/
    <agg>_service.go # application service: orchestrates domain + repo + tx; the use-case layer
    dto.go           # command/query DTOs
internal/repository/<agg>/
    repository.go    # sqlc-backed implementation of the domain PORT
internal/infrastructure/
    casbin/adapter.go   # sqlc-based persist.Adapter over casbin_rule
    auth/jwt.go         # HS256 sign/verify, claims {uid string, role_codes}
    auth/password.go    # argon2id hash/verify
```

- Cross-aggregate consistency (e.g. assign role → permission) lives in an **application service**, not the domain, to keep aggregates independent.
- `permission.BuildTree(items []*Permission) []*Node` is a **pure domain function** (stable by SortOrder then ID) — no separate Menu aggregate.
- `Enforce` is an application concern delegating to the Casbin enforcer (backed by the adapter).

## Data model (postgres + sqlc)

Tables (schema DDL via migration; queries via sqlc). All IDs are TEXT (uuid v4, app-generated).
- `users` (id TEXT PK, username, password_hash, nickname, avatar, email, phone, **status int 0/1**, created_at, updated_at)
- `roles` (id TEXT PK, code UNIQUE, name, **status int 0/1**, remark, created_at)
- `permissions` (id TEXT PK, **code**, name, **kind ∈ catalog|menu|button|api**, **parent_id**→permissions(id) ON DELETE CASCADE, path, icon, **route_name**, redirect, keep_alive, hide_in_menu, is_external, **method**, **sort_order**, **status int 0/1**, description, created_at, **UNIQUE(code, kind)**)
- `user_roles` (user_id TEXT, role_id TEXT)
- `role_permissions` (role_id TEXT, permission_id TEXT)
- `casbin_rule` (ptype, v0..v5) — the Casbin adapter's storage; **enforcement source of truth**
- `audit_log` (id, **actor_uid TEXT**, action, target, detail_json, created_at)

**Single-source rule:** when management grants a permission to a role or assigns a role to a user, the application service writes the relational tables AND the corresponding Casbin policy via the adapter in the same tx. `casbin_rule` is what `Enforce` reads.

**Permission tree semantics** (对齐 s-web):
- `kind=catalog|menu` rows are the frontend menu tree (`GetUserMenuTree`); `kind=button|api` rows carry the same `code` for button rendering + endpoint enforcement.
- `api`-kind rows carry `path` (endpoint) + `method` (HTTP verb); the casbin policy `p(role_code, code, method)` is generated from api-kind rows during grant.
- `// TODO(data-scope)`: a `dom` column + `departments` tree + role→data-scope binding are the documented seam.

## Casbin

- `model.conf` (embedded): RBAC with resource+action —
  ```
  [request_definition] r = sub, obj, act
  [policy_definition]  p = sub, obj, act
  [role_definition]    g = _, _
  [matchers] m = g(r.sub, p.sub) && r.obj == p.obj && r.act == p.act
  ```
- `p` policies: `(role_code, permission_code, http_method)`; `g`: `(user_id, role_code)` — user_id is a **string** uuid.
- **Policy act source:** when granting `permission_codes` to a role, the application service resolves each code to its `api`-kind record(s) and emits `AddPolicy(role_code, code, method)`; button/menu-kind rows share the same code but do not emit policies (frontend rendering only). Codes with no api-kind record are recorded without a policy (documented: BFF must gate such endpoints).
- Enforcer loaded from the sqlc adapter at startup; management mutations call `AddPolicy`/`RemovePolicy` + `SavePolicy` through the adapter. **Seam:** local enforcer + watcher (redis pub/sub) for the BFF is a documented follow-up; v1 authority answers `Enforce` via RPC.

## RPC surface (proto)

**AuthService**
- `Login(username, password) → {access_token, refresh_token, expires_in}`
- `Refresh(refresh_token) → {access_token, …}`
- `Logout(access_token|uid)`  (blacklist + drop refresh)
- `ValidateToken(access_token) → {uid string, role_codes [], valid}`

**RBACService**
- User: `CreateUser/UpdateUser/DeleteUser/GetUser/ListUsers` — ids are **string**; status int32 0/1
- Role: `CreateRole/UpdateRole/DeleteRole/ListRoles` — 同上; Role carries `permission_codes`
- Permission: `CreatePermission/UpdatePermission/DeletePermission/ListPermissions` — **含 UpdatePermission**
- Grant/Assign: `AssignRolesToUser(user_id, role_ids)`, `GrantPermissionsToRole(role_id, permission_codes)`
- `Enforce(uid string, obj, act) → {allowed}`
- `GetUserMenuTree(uid) → MenuNode 树` + `GetUserPermCodes(uid) → [codes]`（驱动前端菜单 + 按钮渲染）

Full proto (Task 1 of the plan doc, verbatim):

```proto
syntax = "proto3";
package rbac.v1;
option go_package = "api/rbac/v1;rbacv1";

service AuthService {
  rpc Login(LoginReq) returns (LoginResp);
  rpc Refresh(RefreshReq) returns (LoginResp);
  rpc Logout(LogoutReq) returns (LogoutResp);
  rpc ValidateToken(ValidateTokenReq) returns (ValidateTokenResp);
}

service RBACService {
  rpc CreateUser(CreateUserReq) returns (UserResp);
  rpc UpdateUser(UpdateUserReq) returns (UserResp);
  rpc DeleteUser(DeleteUserReq) returns (EmptyResp);
  rpc GetUser(GetUserReq) returns (UserResp);
  rpc ListUsers(ListUsersReq) returns (ListUsersResp);

  rpc CreateRole(CreateRoleReq) returns (RoleResp);
  rpc UpdateRole(UpdateRoleReq) returns (RoleResp);
  rpc DeleteRole(DeleteRoleReq) returns (EmptyResp);
  rpc ListRoles(ListRolesReq) returns (ListRolesResp);

  rpc CreatePermission(CreatePermissionReq) returns (PermissionResp);
  rpc UpdatePermission(UpdatePermissionReq) returns (PermissionResp);
  rpc DeletePermission(DeletePermissionReq) returns (EmptyResp);
  rpc ListPermissions(ListPermissionsReq) returns (ListPermissionsResp);

  rpc AssignRolesToUser(AssignRolesToUserReq) returns (EmptyResp);
  rpc GrantPermissionsToRole(GrantPermissionsToRoleReq) returns (EmptyResp);
  rpc Enforce(EnforceReq) returns (EnforceResp);
  rpc GetUserMenuTree(GetUserMenuTreeReq) returns (GetUserMenuTreeResp);
  rpc GetUserPermCodes(GetUserPermCodesReq) returns (GetUserPermCodesResp);
}

message LoginReq { string username = 1; string password = 2; }
message LoginResp { string access_token = 1; string refresh_token = 2; int32 expires_in = 3; }
message RefreshReq { string refresh_token = 1; }
message LogoutReq { string access_token = 1; }
message LogoutResp {}
message ValidateTokenReq { string access_token = 1; }
message ValidateTokenResp { string uid = 1; repeated string role_codes = 2; bool valid = 3; }

message User {
  string id = 1;
  string username = 2;
  string nickname = 3;
  string avatar = 4;
  string email = 5;
  string phone = 6;
  int32 status = 7;                 // 1 enabled / 0 disabled
  repeated string role_ids = 8;
}
message CreateUserReq { string username = 1; string password = 2; string nickname = 3; string email = 4; string phone = 5; }
message UpdateUserReq { string id = 1; optional string password = 2; optional string nickname = 3; optional string email = 4; optional string phone = 5; optional int32 status = 6; }
message DeleteUserReq { string id = 1; }
message GetUserReq { string id = 1; }
message ListUsersReq { int32 page = 1; int32 page_size = 2; string keyword = 3; }
message UserResp { User user = 1; }
message ListUsersResp { repeated User users = 1; int32 total = 2; }

message Role {
  string id = 1;
  string code = 2;
  string name = 3;
  int32 status = 4;
  string remark = 5;
  repeated string permission_codes = 6;
}
message CreateRoleReq { string code = 1; string name = 2; string remark = 3; }
message UpdateRoleReq { string id = 1; optional string name = 2; optional string remark = 3; optional int32 status = 4; }
message DeleteRoleReq { string id = 1; }
message ListRolesReq { int32 page = 1; int32 page_size = 2; }
message RoleResp { Role role = 1; }
message ListRolesResp { repeated Role roles = 1; int32 total = 2; }

message Permission {
  string id = 1;
  string code = 2;
  string name = 3;
  string kind = 4;              // catalog | menu | button | api
  string parent_id = 5;
  string path = 6;              // catalog/menu 路由；api 接口路径
  string icon = 7;
  string route_name = 8;        // 路由名 = 多语言标识
  string redirect = 9;
  bool keep_alive = 10;
  bool hide_in_menu = 11;
  bool is_external = 12;
  string method = 13;           // api: GET/POST/PUT/DELETE；其他 ""
  int32 sort_order = 14;
  int32 status = 15;            // 1 enabled / 0 disabled
  string description = 16;
}
message CreatePermissionReq { string code = 1; string name = 2; string kind = 3; string parent_id = 4; string path = 5; string icon = 6; string route_name = 7; string redirect = 8; bool keep_alive = 9; bool hide_in_menu = 10; bool is_external = 11; string method = 12; int32 sort_order = 13; string description = 14; }
message UpdatePermissionReq { string id = 1; optional string code = 2; optional string name = 3; optional string kind = 4; optional string parent_id = 5; optional string path = 6; optional string icon = 7; optional string route_name = 8; optional string redirect = 9; optional bool keep_alive = 10; optional bool hide_in_menu = 11; optional bool is_external = 12; optional string method = 13; optional int32 sort_order = 14; optional int32 status = 15; optional string description = 16; }
message DeletePermissionReq { string id = 1; }
message ListPermissionsReq {}
message PermissionResp { Permission permission = 1; }
message ListPermissionsResp { repeated Permission permissions = 1; }

message AssignRolesToUserReq { string user_id = 1; repeated string role_ids = 2; }
message GrantPermissionsToRoleReq { string role_id = 1; repeated string permission_codes = 2; }
message EnforceReq { string uid = 1; string obj = 2; string act = 3; }
message EnforceResp { bool allowed = 1; }
message GetUserMenuTreeReq { string uid = 1; }
message MenuNode {
  string id = 1;
  string code = 2;
  string name = 3;
  string route_name = 4;
  string path = 5;
  string icon = 6;
  string redirect = 7;
  bool hide_in_menu = 8;
  bool is_external = 9;
  bool keep_alive = 10;
  repeated MenuNode children = 11;
}
message GetUserMenuTreeResp { repeated MenuNode roots = 1; }
message GetUserPermCodesReq { string uid = 1; }
message GetUserPermCodesResp { repeated string codes = 1; }
message EmptyResp {}
```

## Auth / security (v1 + seams)

- **JWT HS256** (secret from config); claims `{uid string, roles []string, exp}`. **Seam:** RS256/JWKS.
- **Passwords:** argon2id; basic failed-attempt lockout counter (Redis). 
- **Refresh/blacklist:** Redis stores refresh tokens + a short-TTL access-token blacklist for logout.
- **Audit:** every RBAC mutation writes `audit_log`.
- **Seams (documented TODO):** data-scope/domain, RS256/JWKS, local casbin enforcer+watcher, OTel observability.

## Build plan (how it becomes a template)

1. `ncgo new rbac-rpc --kind kitex --module <m> --db postgres` (base-kitex layered project with sqlc).
2. Hand-write DDD layers (domain/application/infrastructure) + casbin adapter + JWT/argon2 + proto (Auth/RBAC) + sqlc schema/queries + audit + unit tests. Make `make sqlc` + `go build ./...` + `go test ./...` pass.
3. `ncgo export templates --kind kitex` → captures base + DDD `internal/domain/**` + `internal/application/**` (now supported, #72) + repository + infra.
4. Assemble `ncgo-templates/rbac-kitex` (kitex-template/* + idl + README + template.yaml). Verify `ncgo new --template rbac-kitex` → `go build`.

## Testing

- Domain: pure unit tests (password policy, role-permission invariants, permission tree build).
- Application: service tests with a fake repo (Enforce sync, grant/assign tx).
- Infrastructure: casbin adapter round-trip (LoadPolicy/SavePolicy) against a temp DB or a fake; jwt sign/verify; argon2.
- Golden/export: the exported template package builds (`ncgo new --template` smoke).

## Out of scope (this template)

Rate limiting (admin-bff concern via ratelimit-hertz + rule-center), the HTTP admin UI (admin-bff-hertz), org tree/data-scope (seam), SSO/OIDC.
- 前端 s-web 契约适配由 BFF（admin-bff-hertz）承担，本模板不实现 REST 层。

## Resolved points

1. Permission model: **single Permission tree**（permissions 自带树字段，menus 合并）— 对齐 s-web 事实标准。
2. `kind` enum: **catalog|menu|button|api** — 保留 kind 列，按钮与接口共用同一 `code`（`UNIQUE(code, kind)`）。
3. Enforce via RPC in v1（BFF calls it）— 客户端 enforcer 作为 seam。
