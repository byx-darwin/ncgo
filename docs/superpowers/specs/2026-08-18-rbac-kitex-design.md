# Design: `rbac-kitex` — RBAC + Auth authority (Kitex RPC) template

> **Alignment revision (wf-2026-08-19-003):** 本文档已根据 `2026-08-19-rbac-kitex-alignment-decisions.md` 的 6 项锁定决策修订。修订点：Locked decisions 表（+6 行）、DDD structure（menu 降级）、Data model（单一 Permission 树）、RPC surface（UpdatePermission + Menu CRUD 移除 + codes 载荷）、Open points（已决议）。

Part of the **micro-admin** program (sub-project 2 of the 3-template decomposition). Program design: `2026-08-18-micro-admin-ddd-program-design.md`.

- **Type:** new Kitex RPC service **template** for `ncgo-templates`, built via `ncgo` + `ncgo export`.
- **Bar:** reference scaffold (correct structure, runnable happy path, documented production seams).
- **Role:** the RBAC + authentication **authority** — owns the DB and Casbin policy; `admin-bff-hertz` consumes it.

## Locked decisions

| Dimension | Decision |
|---|---|
| Framework | Kitex RPC (base-kitex layered layout + DDD layers) |
| Architecture | DDD: `internal/domain/<agg>` + `internal/application/<agg>` + `internal/repository/<agg>` (sqlc) |
| RBAC | Casbin, **basic model `sub, obj, act`** (no domain first version); data-scope/domain is a documented seam |
| Permission code | **Unified**: `permissions.code` == Casbin `obj` == menu/button `perm_code` (one code drives frontend menus + backend API enforce) |
| Casbin storage | built-in **sqlc-based `persist.Adapter`** over `casbin_rule` (no gorm) |
| Single source | `casbin_rule` is the enforcement source; management writes (grant/assign) **sync into** the adapter (direction: mgmt tables → casbin) |
| Auth | self-built **JWT (HS256 default)**, claims `{uid, roles}`; argon2id passwords + basic lockout; Redis refresh-token + JWT blacklist |
| Audit | basic `audit_log` (who/what/when) written by RBAC mutations |
| Build | develop real Kitex project with ncgo → hand-write DDD+casbin+JWT → build/test → `ncgo export` → `rbac-kitex` package |
| Permission data model (s-web alignment) | **单一 Permission 树**（`permissions` 表承载 `catalog`/`menu`/`button`/`api` 四种 type，`menus` 作为 `WHERE type IN ('catalog','menu')` 的过滤视图；原 `menus` 表删除） |
| Permission code uniqueness | `UNIQUE(code, type)` 联合唯一 —— 同一 `code` 允许在 `button` 型与 `api` 型各出现一次（s-web 事实：`user:create` 同时驱动按钮渲染与接口鉴权） |
| Authorization payload | `GrantPermissionsToRole` 载荷从 `permission_ids []int64` 改为 `permission_codes []string`；端到端（前端 → BFF → RPC → Casbin）统一使用 `code` 作为外部标识符 |
| status 语义 | `status int`（`1=enabled`, `0=disabled`）统一用于 User / Role / Permission；原 User `status` enum (`active`/`disabled`) 改为 `int` |
| RPC 表面修订 | Permission RPC 拥有所有写路径（新增 `UpdatePermission` + `GetPermission`；移除 `Menu.CreateMenu`/`UpdateMenu`/`DeleteMenu`）；Menu RPC 仅保留只读树查询（`GetUserMenuTree`、`ListMenus`） |
| Scope boundary | 本模板仅对齐 s-web 契约；s-web 前端永不在修改范围；admin-bff-hertz 适配作为下游独立 issue |

## DDD structure (per aggregate)

Aggregates: **user, role, permission**（写聚合）；**menu**（只读查询聚合）。

`permission` 聚合现在承载完整树（`catalog`/`menu`/`button`/`api`），其写路径覆盖原 `menu` 聚合的 CRUD。
`menu` 聚合保留为只读查询聚合，职责为 `GetUserMenuTree(uid)` 与 `ListMenus(filter)` 的树形组装（按 type∈{catalog,menu} 过滤 + 按 sort/id 排序）。

```
internal/domain/<agg>/
    entity.go        # entity/aggregate root (User, Role, Permission, Menu)
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
    auth/jwt.go         # HS256 sign/verify, claims
    auth/password.go    # argon2id hash/verify
```

- `internal/domain/permission/repository.go` `(owns all writes; tree CRUD via Permission)`
- `internal/domain/menu/query_service.go` `(read-only tree queries)`（原 `menu/service.go`）
- Cross-aggregate consistency (e.g. assign role → permission) lives in an **application service**, not the domain, to keep aggregates independent.
- `Enforce` is an application concern delegating to the Casbin enforcer (backed by the adapter).

## Data model (postgres + sqlc)

Tables（schema DDL via migration；queries via sqlc）：

- `users` (id, username, password_hash, **nickname NULL**, **avatar NULL**, **email NULL**, **phone NULL**, **status int NOT NULL DEFAULT 1**, created_at, updated_at)
  - `status` 1=enabled, 0=disabled（取代原 enum `active|disabled`）
- `roles` (id, code, name, **status int NOT NULL DEFAULT 1**, **remark text NULL**, created_at, updated_at)
- `permissions` (id, **code**, **type** varchar, name, **parent_id bigint NULL REFERENCES permissions(id)**, **path NULL**, **icon NULL**, **route_name NULL**, **redirect NULL**, **keep_alive bool NULL**, **hide_in_menu bool NULL**, **is_external bool NULL**, **method NULL** — api 型必填, **sort int NULL**, **status int NOT NULL DEFAULT 1**, **description text NULL**, created_at, updated_at)
  - **`UNIQUE(code, type)`** 联合唯一（取代原 `code UNIQUE`）
  - type 取值：`catalog | menu | button | api`（扩展自原 `kind: menu|button|api`；新增 `catalog` 对应 s-web 顶层分类）
  - 树字段（parent_id / path / icon / route_name / redirect / keep_alive / hide_in_menu / is_external / sort）仅 `catalog`/`menu` 型填充；`button`/`api` 型为 NULL
  - `method` 仅 `api` 型必填（`GET|POST|PUT|DELETE`），其他型 NULL
- `menus` 表已删除 —— 原菜单语义通过 `SELECT ... FROM permissions WHERE type IN ('catalog','menu')` 视图化
- `user_roles` (user_id, role_id) — 不变
- `role_permissions` (role_id, permission_id) — 不变（仍按 `permission_id` 关联，维护 FK 完整性；RPC 表面不暴露 `permission_id`）
- `casbin_rule` (ptype, v0..v5) — **不变**；Casbin 仅消费 `code`，对 `code+type` 联合唯一无感
- `audit_log` — 不变

**Single-source rule 保持**：mgmt writes → `role_permissions` + `casbin_rule` 同事务。`casbin_rule` 仍是 `Enforce` 的读源。

**Status 常量（Go）**：
```go
const (
    StatusEnabled  = 1
    StatusDisabled = 0
)
```
各聚合（User/Role/Permission）统一使用这两个常量，不引入 enum 类型。

## Casbin

- `model.conf` (embedded): RBAC with resource+action —
  ```
  [request_definition] r = sub, obj, act
  [policy_definition]  p = sub, obj, act
  [role_definition]    g = _, _
  [matchers] m = g(r.sub, p.sub) && r.obj == p.obj && r.act == p.act
  ```
- `p` policies: `(role_code, permission_code, http_method)`; `g`: `(user_id, role_code)`.
- Enforcer loaded from the sqlc adapter at startup; management mutations call `AddPolicy`/`RemovePolicy` + `SavePolicy` through the adapter. **Seam:** local enforcer + watcher (redis pub/sub) for the BFF is a documented follow-up; v1 authority answers `Enforce` via RPC.

**s-web 对齐影响评估（wf-2026-08-19-003 §5）：** 零影响。`casbin_rule` 表结构、`p` 策略 `(role_code, permission_code, http_method)`、`g` 映射 `(user_id, role_code)`、`model.conf` 均不变化。Casbin 仅消费 `code`，对 `permissions.code+type` 联合唯一无感。

## RPC surface (proto)

**AuthService**
- `Login(username, password) → {access_token, refresh_token, expires_in}`
- `Refresh(refresh_token) → {access_token, …}`
- `Logout(access_token|uid)`  (blacklist + drop refresh)
- `ValidateToken(access_token) → {uid, roles, valid}`

**RBACService**（wf-2026-08-19-003 修订）
- User: `CreateUser` / `UpdateUser` / `DeleteUser` / `GetUser` / `ListUsers`
- Role: `CreateRole` / `UpdateRole` / `DeleteRole` / `ListRoles`
- Permission（写路径统一）: `CreatePermission` / **`UpdatePermission`（新增）** / `DeletePermission` / **`GetPermission`（新增）** / `ListPermissions`
  - `CreatePermission` / `UpdatePermission` 支持 type∈{catalog, menu, button, api}；`api` 型 `method` 必填
  - `DeletePermission` 级联删除子节点（树语义，应用层实现）
- Menu（只读查询）: **`ListMenus`（保留；返回 tree，过滤 type∈{catalog,menu}）** / `GetUserMenuTree(uid)`
  - **已移除**: `CreateMenu` / `UpdateMenu` / `DeleteMenu`（写路径统一到 Permission RPC）
- Grant/Assign:
  - `AssignRolesToUser(user_id, role_ids []int64)` — 不变（s-web 用户授权用 roleIds）
  - **`GrantPermissionsToRole(role_id, permission_codes []string)`** — 载荷从 `permission_ids` 改为 `permission_codes`
- `Enforce(uid, obj, act) → {allowed}` — 不变
- `GetUserMenuTree(uid) → menu tree` + `GetUserPermCodes(uid) → [codes]` — 不变（驱动前端菜单 + 按钮渲染）

Proto messages follow base-kitex conventions (`idl/<service>.proto`, `go_package … kitex_gen/…`).

## Auth / security (v1 + seams)

- **JWT HS256** (secret from config); claims `{uid, roles, exp}`. **Seam:** RS256/JWKS.
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

- Domain: pure unit tests (password policy, role-permission invariants, menu tree build).
- Application: service tests with a fake repo (Enforce sync, grant/assign tx).
- Infrastructure: casbin adapter round-trip (LoadPolicy/SavePolicy) against a temp DB or a fake; jwt sign/verify; argon2.
- Golden/export: the exported template package builds (`ncgo new --template` smoke).

## Out of scope (this template)

Rate limiting (admin-bff concern via ratelimit-hertz + rule-center), the HTTP admin UI (admin-bff-hertz), org tree/data-scope (seam), SSO/OIDC.

## Open points (resolved by wf-2026-08-19-003)

1. ~~Aggregate set `user/role/permission/menu` — enough for v1?~~ ✅ 确认；`menu` 退化为只读查询聚合，写路径并入 `permission`（decisions doc §1, §6）。
2. ~~`permissions.kind` enum `menu|button|api` vs simpler — keep the kind column for clarity?~~ ✅ 保留并扩展为 `type` 列，新增 `catalog` 取值（decisions doc §1）。
3. ~~Enforce via RPC in v1 vs client-side enforcer~~ ✅ v1 仍走 RPC，enforcer 作为 seam（decisions doc 不影响此决策）。

新增 open point（deferred to downstream issue）:
4. `admin-bff-hertz` 适配到修订后的模板契约 —— 不在本 issue 范围，作为下游独立 issue 跟进（decisions doc §7）。
