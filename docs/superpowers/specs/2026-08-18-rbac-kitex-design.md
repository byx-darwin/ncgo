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
| RBAC | Casbin, **basic model `sub, obj, act`** (no domain first version); data-scope/domain is a documented seam |
| Permission code | **Unified**: `permissions.code` == Casbin `obj` == menu/button `perm_code` (one code drives frontend menus + backend API enforce) |
| Casbin storage | built-in **sqlc-based `persist.Adapter`** over `casbin_rule` (no gorm) |
| Single source | `casbin_rule` is the enforcement source; management writes (grant/assign) **sync into** the adapter (direction: mgmt tables → casbin) |
| Auth | self-built **JWT (HS256 default)**, claims `{uid, roles}`; argon2id passwords + basic lockout; Redis refresh-token + JWT blacklist |
| Audit | basic `audit_log` (who/what/when) written by RBAC mutations |
| Build | develop real Kitex project with ncgo → hand-write DDD+casbin+JWT → build/test → `ncgo export` → `rbac-kitex` package |

## DDD structure (per aggregate)

Aggregates: **user, role, permission, menu**.

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

- Cross-aggregate consistency (e.g. assign role → permission) lives in an **application service**, not the domain, to keep aggregates independent.
- `Enforce` is an application concern delegating to the Casbin enforcer (backed by the adapter).

## Data model (postgres + sqlc)

Tables (schema DDL via migration; queries via sqlc):
- `users` (id, username, password_hash, status, created_at, …)
- `roles` (id, code, name, …)
- `permissions` (id, **code**, name, kind[menu|button|api], …) — `code` is the unified permission code
- `user_roles` (user_id, role_id)
- `role_permissions` (role_id, permission_id)
- `menus` (id, parent_id, type[dir|menu|button], name, path/component, **perm_code**, order) — tree; button rows carry `perm_code` referencing `permissions.code`
- `casbin_rule` (ptype, v0..v5) — the Casbin adapter's storage; **enforcement source of truth**
- `audit_log` (id, actor_uid, action, target, detail_json, created_at)

**Single-source rule:** when management grants a permission to a role or assigns a role to a user, the application service writes the relational tables AND the corresponding Casbin policy via the adapter in the same tx. `casbin_rule` is what `Enforce` reads. `// TODO(data-scope)`: a `dom` column + `departments` tree + role→data-scope binding are the documented seam.

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

## RPC surface (proto)

**AuthService**
- `Login(username, password) → {access_token, refresh_token, expires_in}`
- `Refresh(refresh_token) → {access_token, …}`
- `Logout(access_token|uid)`  (blacklist + drop refresh)
- `ValidateToken(access_token) → {uid, roles, valid}`

**RBACService**
- User: `CreateUser/UpdateUser/DeleteUser/GetUser/ListUsers`
- Role: `CreateRole/UpdateRole/DeleteRole/ListRoles`
- Permission: `CreatePermission/DeletePermission/ListPermissions`
- Menu: `CreateMenu/UpdateMenu/DeleteMenu/ListMenus`
- Grant/Assign: `AssignRolesToUser`, `GrantPermissionsToRole`
- `Enforce(uid, obj, act) → {allowed}`
- `GetUserMenuTree(uid) → menu tree` + `GetUserPermCodes(uid) → [codes]` (drives frontend menu + button rendering)

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

## Open points for review

1. Aggregate set `user/role/permission/menu` — enough for v1? (recommend yes)
2. `permissions.kind` enum `menu|button|api` vs simpler — keep the kind column for clarity? (recommend keep)
3. Enforce via RPC in v1 (BFF calls it) vs shipping a client-side enforcer now — recommend RPC in v1, enforcer as seam.
