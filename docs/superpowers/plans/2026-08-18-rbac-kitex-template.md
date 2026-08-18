# rbac-kitex — RBAC + Auth authority Kitex template Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the official `rbac-kitex` template package in `ncgo-templates` — a DDD Kitex RPC service that is the RBAC + authentication **authority** (users/roles/permissions/menus, Casbin enforcement, JWT login, audit), built by developing a real seed project with `ncgo` and exporting it.

**Architecture:** Develop a real Kitex seed project (`ncgo new --kind kitex --db postgres`) in a gitignored scratch dir, hand-write DDD layers (`internal/domain/**` + `internal/application/**` + `internal/repository/<agg>/**`) + infrastructure (sqlc-backed casbin `persist.Adapter`, JWT/argon2, token store, audit), make it build+test green, then `ncgo export templates --kind kitex` and assemble the `rbac-kitex` package (`template.yaml` + `kitex-template/*.yaml` + `idl/` + README + e2e test). The package is consumed via `ncgo new --template rbac-kitex` / `--template-dir`.

**Tech Stack:** Go 1.26+, Kitex gRPC, protoc/protobuf, sqlc (pgx/v5), postgres, Casbin v2, golang-jwt/jwt/v5 (HS256), x/crypto/argon2 (argon2id via alexedwards/argon2id), samber/do DI, go-tools go-common/go-framework.

**Spec:** `docs/superpowers/specs/2026-08-18-rbac-kitex-design.md` (ncgo repo). Program: `docs/superpowers/specs/2026-08-18-micro-admin-ddd-program-design.md`.

## Global Constraints

- Bar = **reference scaffold**: correct structure + runnable happy path + documented production seams. NOT a full production system.
- DDD layers are **aggregate-organized** (`internal/domain/<agg>/`), NOT proto-service-looped. Export rules: `internal/domain/**` + `internal/application/**` are `skip` + `LoopService:false` (ncgo #72/#73, on main).
- Casbin: **basic model `sub, obj, act`**; `casbin_rule` is the enforcement single-source; management writes (grant/assign) **sync into** the adapter in-tx direction (mgmt tables → casbin). Data-scope/domain is a documented seam.
- **Unified permission code**: `permissions.code` == casbin `obj` == `menus.perm_code` (one code drives frontend menus/buttons + backend API enforce).
- Casbin adapter is built-in **sqlc-based** (no gorm).
- Auth: self-built JWT **HS256** (claims `{uid, roles}`) + **argon2id** passwords; refresh/blacklist via a `TokenStore` (hermetic memory default, Redis seam documented). RS256/JWKS is a seam.
- Hermetic happy path: seed/tests/e2e must build+test WITHOUT postgres/redis up (DB-gated paths skip explicitly). No silent skips.
- Template package merge model: `template.yaml` sets `skip_default_templates: [handler.yaml, usecase.yaml, repository.yaml, server.yaml]` so the package's own DDD + handler + server yamls replace the base per-layer scaffolding (same mechanism the rule-center preset uses).
- **ncgo export does NOT capture** `internal/infrastructure/**/*.go`, `internal/db/schema/*.sql`, `internal/db/query/*.sql`, `internal/db/migrations/*.sql`, or `.conf` non-Go files. Those are **hand-assembled** in the assemble task, wrapping the seed's files deterministically.
- **Export post-processing**: strip `loop_service: true` from any exported yaml whose `path` contains no `{{` (aggregate repos `internal/repository/<agg>/`, literal handlers `internal/handler/authservice/`) — loops are meaningless on non-variabilized paths.
- Registry/README: add a `rbac-kitex` row to `ncgo-templates/README.md`'s Templates table.
- Seed + scratch live in gitignored `ncgo-templates/.cache/`. The package deliverable is `ncgo-templates/rbac-kitex/`.
- gofmt-clean; `go vet ./...` clean. Keep diffs minimal.
- Requires local tools: `go`, `ncgo` (built from ncgo main), `kitex`, `protoc`, `sqlc`. Verify with `command -v` before starting; fail loudly with the missing tool.

## File Structure

Seed project (scratch, gitignored) — `ncgo-templates/.cache/rbac-seed/<name>`:

```
idl/auth.proto                          # rbac.v1: AuthService + RBACService
internal/domain/user/{entity,valueobject,service,repository}.go
internal/domain/role/{entity,service,repository}.go
internal/domain/permission/{entity,valueobject,repository}.go
internal/domain/menu/{entity,service,repository}.go
internal/application/auth/{auth_service,dto}.go
internal/application/user/{user_service,dto}.go
internal/application/role/{role_service,dto}.go
internal/application/permission/{permission_service,dto}.go
internal/application/menu/{menu_service,dto}.go
internal/application/rbac/{enforce_service,dto}.go
internal/repository/user/repo.go
internal/repository/role/repo.go
internal/repository/permission/repo.go
internal/repository/menu/repo.go
internal/infrastructure/casbin/{adapter.go,model.conf,enforcer.go}
internal/infrastructure/auth/{jwt.go,password.go}
internal/infrastructure/token/{store.go,memory.go,redis.go}
internal/infrastructure/audit/{writer.go}
internal/handler/authservice/handler.go
internal/handler/rbacservice/handler.go
internal/base/server/server.go            # custom wiring (cover)
internal/base/conf/conf.go                # custom config: + AuthConfig{JWT,Token}
conf/dev/conf.yaml                        # dev config with dsn + jwt secret
internal/db/schema/000001_rbac.sql
internal/db/query/rbac.sql
internal/db/migrations/000001_init.sql    # goose init (replaces placeholder)
Makefile                                  # base + sqlc + kitex targets (exported)
```

Deliverable — `ncgo-templates/rbac-kitex/`:

```
template.yaml             # name/kind/description/version + skip_default_templates
kitex-template/*.yaml     # exported (base+handler+ddd) + hand-assembled (infra/schema/query/migration/model.conf)
idl/{{ToLower .ServiceName}}.proto  # or idl/auth.proto (literal if no service-name token)
README.md
test/e2e_test.sh
```

## Task 1: Seed scaffold + IDL + sqlc schema/queries + codegen

**Files:**
- Create: `ncgo-templates/.cache/rbac-seed/` (seed project)
- Create: seed `idl/auth.proto`, `internal/db/schema/000001_rbac.sql`, `internal/db/query/rbac.sql`, `internal/db/migrations/000001_init.sql`

**Interfaces:**
- Produces: a compiling base Kitex project at `.cache/rbac-seed/rbac-rpc/` with sqlc `internal/db/gen` generated, kitex_gen generated, and a manifest at `.ncgo/manifest.yaml` (module `github.com/byx-darwin/rbac-rpc`, service `rbac-rpc`, kind kitex) — the input to `ncgo export` in Task 8.

- [ ] **Step 1: Verify toolchain + build ncgo from main**

```bash
command -v go kitex protoc sqlc ncgo   # each must resolve; print any missing
cd /Users/xs/Documents/workspce/github.com/byx-darwin/ncgo && go build -o /tmp/ncgo-rbac .
/tmp/ncgo-rbac version
```
Expected: ncgo binary built from main (has DDD export — check `/tmp/ncgo-rbac export templates --help` prints).

- [ ] **Step 2: Write the IDL**

Write `idl/auth.proto`:

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
  rpc DeletePermission(DeletePermissionReq) returns (EmptyResp);
  rpc ListPermissions(ListPermissionsReq) returns (ListPermissionsResp);

  rpc CreateMenu(CreateMenuReq) returns (MenuResp);
  rpc UpdateMenu(UpdateMenuReq) returns (MenuResp);
  rpc DeleteMenu(DeleteMenuReq) returns (EmptyResp);
  rpc ListMenus(ListMenusReq) returns (ListMenusResp);

  rpc AssignRolesToUser(AssignRolesToUserReq) returns (EmptyResp);
  rpc GrantPermissionsToRole(GrantPermissionsToRoleReq) returns (EmptyResp);
  rpc Enforce(EnforceReq) returns (EnforceResp);
  rpc GetUserMenuTree(GetUserMenuTreeReq) returns (GetUserMenuTreeResp);
  rpc GetUserPermCodes(GetUserPermCodesReq) returns (GetUserPermCodesResp);
}

message LoginReq { string username = 1; string password = 2; }
message LoginResp {
  string access_token = 1;
  string refresh_token = 2;
  int32 expires_in = 3;           // access token TTL seconds
}
message RefreshReq { string refresh_token = 1; }
message LogoutReq { string access_token = 1; }
message LogoutResp {}
message ValidateTokenReq { string access_token = 1; }
message ValidateTokenResp {
  int64 uid = 1;
  repeated string roles = 2;
  bool valid = 3;
}

message User {
  int64 id = 1;
  string username = 2;
  string status = 3;
  repeated string roles = 4;
}
message CreateUserReq { string username = 1; string password = 2; }
message UpdateUserReq { int64 id = 1; optional string password = 2; optional string status = 3; }
message DeleteUserReq { int64 id = 1; }
message GetUserReq { int64 id = 1; }
message ListUsersReq { int32 page = 1; int32 page_size = 2; }
message UserResp { User user = 1; }
message ListUsersResp { repeated User users = 1; int32 total = 2; }

message Role { int64 id = 1; string code = 2; string name = 3; }
message CreateRoleReq { string code = 1; string name = 2; }
message UpdateRoleReq { int64 id = 1; string name = 2; }
message DeleteRoleReq { int64 id = 1; }
message ListRolesReq { int32 page = 1; int32 page_size = 2; }
message RoleResp { Role role = 1; }
message ListRolesResp { repeated Role roles = 1; int32 total = 2; }

message Permission {
  int64 id = 1;
  string code = 2;
  string name = 3;
  string kind = 4;   // menu | button | api
  string method = 5; // HTTP method for kind=api; "*" for menu/button
}
message CreatePermissionReq { string code = 1; string name = 2; string kind = 3; string method = 4; }
message DeletePermissionReq { int64 id = 1; }
message ListPermissionsReq { int32 page = 1; int32 page_size = 2; }
message PermissionResp { Permission permission = 1; }
message ListPermissionsResp { repeated Permission permissions = 1; int32 total = 2; }

message Menu {
  int64 id = 1;
  int64 parent_id = 2;
  string type = 3;      // dir | menu | button
  string name = 4;
  string path = 5;
  string component = 6;
  string perm_code = 7;
  int32 sort_order = 8;
}
message CreateMenuReq { int64 parent_id = 1; string type = 2; string name = 3; string path = 4; string component = 5; string perm_code = 6; int32 sort_order = 7; }
message UpdateMenuReq { int64 id = 1; optional int64 parent_id = 2; optional string type = 3; optional string name = 4; optional string path = 5; optional string component = 6; optional string perm_code = 7; optional int32 sort_order = 8; }
message DeleteMenuReq { int64 id = 1; }
message ListMenusReq {}
message MenuResp { Menu menu = 1; }
message ListMenusResp { repeated Menu menus = 1; }

message AssignRolesToUserReq { int64 user_id = 1; repeated int64 role_ids = 2; }
message GrantPermissionsToRoleReq { int64 role_id = 1; repeated int64 permission_ids = 2; }
message EnforceReq { int64 uid = 1; string obj = 2; string act = 3; }
message EnforceResp { bool allowed = 1; }
message GetUserMenuTreeReq { int64 uid = 1; }
message MenuNode { Menu menu = 1; repeated MenuNode children = 2; }
message GetUserMenuTreeResp { repeated MenuNode roots = 1; }
message GetUserPermCodesReq { int64 uid = 1; }
message GetUserPermCodesResp { repeated string codes = 1; }
message EmptyResp {}
```

- [ ] **Step 3: Scaffold the seed with ncgo**

```bash
SEED=/Users/xs/Documents/workspce/github.com/byx-darwin/ncgo-templates/.cache/rbac-seed
mkdir -p "$SEED"
cd "$SEED"
/tmp/ncgo-rbac new rbac-rpc --kind kitex --module github.com/byx-darwin/rbac-rpc \
  --db postgres --no-generate
```
Expected: manifest + `template/kitex-template/` + `idl/rbac-rpc.proto` placeholder + `internal/db/{sqlc.yaml,schema/000001_placeholder.sql,query/health.sql,migrations/000001_init.sql}`. Confirm `.ncgo/manifest.yaml` has `kind: kitex`, `module: github.com/byx-darwin/rbac-rpc`, `service.name: rbac-rpc`.

- [ ] **Step 4: Replace placeholder IDL with the real proto + drop the placeholder name collision**

```bash
cd "$SEED/rbac-rpc"
rm -f idl/rbac-rpc.proto
cp <plan-path>/idl/auth.proto idl/auth.proto
# keep manifest's idl path aligned: edit .ncgo/manifest.yaml idl -> "idl/auth.proto"
```
Expected: `idl/auth.proto` present, no `idl/rbac-rpc.proto`.

- [ ] **Step 5: Write sqlc schema + queries + migration**

Write `internal/db/schema/000001_rbac.sql`:

```sql
CREATE TABLE users (
    id BIGSERIAL PRIMARY KEY,
    username TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active','disabled')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE roles (
    id BIGSERIAL PRIMARY KEY,
    code TEXT NOT NULL UNIQUE,
    name TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE permissions (
    id BIGSERIAL PRIMARY KEY,
    code TEXT NOT NULL UNIQUE,
    name TEXT NOT NULL,
    kind TEXT NOT NULL CHECK (kind IN ('menu','button','api')),
    method TEXT NOT NULL DEFAULT '*',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE user_roles (
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role_id BIGINT NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
    PRIMARY KEY (user_id, role_id)
);

CREATE TABLE role_permissions (
    role_id BIGINT NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
    permission_id BIGINT NOT NULL REFERENCES permissions(id) ON DELETE CASCADE,
    PRIMARY KEY (role_id, permission_id)
);

CREATE TABLE menus (
    id BIGSERIAL PRIMARY KEY,
    parent_id BIGINT REFERENCES menus(id) ON DELETE CASCADE,
    type TEXT NOT NULL DEFAULT 'menu' CHECK (type IN ('dir','menu','button')),
    name TEXT NOT NULL,
    path TEXT,
    component TEXT,
    perm_code TEXT REFERENCES permissions(code) ON DELETE SET NULL,
    sort_order INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE casbin_rule (
    id BIGSERIAL PRIMARY KEY,
    ptype TEXT NOT NULL,
    v0 TEXT NOT NULL DEFAULT '',
    v1 TEXT NOT NULL DEFAULT '',
    v2 TEXT NOT NULL DEFAULT '',
    v3 TEXT NOT NULL DEFAULT '',
    v4 TEXT NOT NULL DEFAULT '',
    v5 TEXT NOT NULL DEFAULT '',
    UNIQUE (ptype, v0, v1, v2, v3, v4, v5)
);

CREATE TABLE audit_log (
    id BIGSERIAL PRIMARY KEY,
    actor_uid BIGINT,
    action TEXT NOT NULL,
    target TEXT NOT NULL DEFAULT '',
    detail_json TEXT NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_menus_parent ON menus(parent_id);
CREATE INDEX idx_casbin_rule_policy ON casbin_rule(ptype, v0, v1, v2);
```

Write `internal/db/query/rbac.sql`:

```sql
-- name: CreateUser :one
INSERT INTO users (username, password_hash, status) VALUES ($1, $2, $3) RETURNING *;
-- name: GetUserByID :one
SELECT * FROM users WHERE id = $1;
-- name: GetUserByUsername :one
SELECT * FROM users WHERE username = $1;
-- name: ListUsers :many
SELECT * FROM users ORDER BY id LIMIT $1 OFFSET $2;
-- name: UpdateUser :one
UPDATE users SET username = $1, status = $2, updated_at = now() WHERE id = $3 RETURNING *;
-- name: UpdateUserPassword :one
UPDATE users SET password_hash = $1, updated_at = now() WHERE id = $2 RETURNING *;
-- name: DeleteUser :exec
DELETE FROM users WHERE id = $1;
-- name: AddUserRole :exec
INSERT INTO user_roles (user_id, role_id) VALUES ($1, $2) ON CONFLICT DO NOTHING;
-- name: RemoveUserRole :exec
DELETE FROM user_roles WHERE user_id = $1 AND role_id = $2;
-- name: ClearUserRoles :exec
DELETE FROM user_roles WHERE user_id = $1;
-- name: ListRolesByUserID :many
SELECT r.* FROM roles r JOIN user_roles ur ON ur.role_id = r.id WHERE ur.user_id = $1 ORDER BY r.id;
-- name: ListRoleIDsByUserID :many
SELECT role_id FROM user_roles WHERE user_id = $1;

-- name: CreateRole :one
INSERT INTO roles (code, name) VALUES ($1, $2) RETURNING *;
-- name: GetRoleByID :one
SELECT * FROM roles WHERE id = $1;
-- name: GetRoleByCode :one
SELECT * FROM roles WHERE code = $1;
-- name: ListRoles :many
SELECT * FROM roles ORDER BY id LIMIT $1 OFFSET $2;
-- name: UpdateRole :one
UPDATE roles SET name = $1 WHERE id = $2 RETURNING *;
-- name: DeleteRole :exec
DELETE FROM roles WHERE id = $1;
-- name: AddRolePermission :exec
INSERT INTO role_permissions (role_id, permission_id) VALUES ($1, $2) ON CONFLICT DO NOTHING;
-- name: RemoveRolePermission :exec
DELETE FROM role_permissions WHERE role_id = $1 AND permission_id = $2;
-- name: ClearRolePermissions :exec
DELETE FROM role_permissions WHERE role_id = $1;
-- name: ListPermissionIDsByRoleID :many
SELECT permission_id FROM role_permissions WHERE role_id = $1;
-- name: ListPermissionsByRoleIDs :many
SELECT DISTINCT p.* FROM permissions p JOIN role_permissions rp ON rp.permission_id = p.id WHERE rp.role_id = ANY($1) ORDER BY p.id;

-- name: CreatePermission :one
INSERT INTO permissions (code, name, kind, method) VALUES ($1, $2, $3, $4) RETURNING *;
-- name: GetPermissionByID :one
SELECT * FROM permissions WHERE id = $1;
-- name: GetPermissionByCode :one
SELECT * FROM permissions WHERE code = $1;
-- name: ListPermissions :many
SELECT * FROM permissions ORDER BY id LIMIT $1 OFFSET $2;
-- name: ListPermissionsByCodes :many
SELECT * FROM permissions WHERE code = ANY($1) ORDER BY id;
-- name: DeletePermission :exec
DELETE FROM permissions WHERE id = $1;

-- name: CreateMenu :one
INSERT INTO menus (parent_id, type, name, path, component, perm_code, sort_order) VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING *;
-- name: GetMenuByID :one
SELECT * FROM menus WHERE id = $1;
-- name: ListMenus :many
SELECT * FROM menus ORDER BY sort_order, id;
-- name: UpdateMenu :one
UPDATE menus SET parent_id = $1, type = $2, name = $3, path = $4, component = $5, perm_code = $6, sort_order = $7 WHERE id = $8 RETURNING *;
-- name: DeleteMenu :exec
DELETE FROM menus WHERE id = $1;
-- name: ListMenusByPermCodes :many
SELECT * FROM menus WHERE perm_code = ANY($1) ORDER BY sort_order, id;

-- name: ListCasbinRules :many
SELECT ptype, v0, v1, v2, v3, v4, v5 FROM casbin_rule ORDER BY id;
-- name: InsertCasbinRule :one
INSERT INTO casbin_rule (ptype, v0, v1, v2, v3, v4, v5) VALUES ($1, $2, $3, $4, $5, $6, $7) ON CONFLICT (ptype, v0, v1, v2, v3, v4, v5) DO NOTHING RETURNING id;
-- name: DeleteCasbinRule :exec
DELETE FROM casbin_rule WHERE ptype = $1 AND v0 = $2 AND v1 = $3 AND v2 = $4 AND v3 = $5 AND v4 = $6 AND v5 = $7;
-- name: DeleteCasbinRuleFiltered :exec
DELETE FROM casbin_rule WHERE ptype = $1
  AND ($2 = '' OR v0 = $2) AND ($3 = '' OR v1 = $3) AND ($4 = '' OR v2 = $4)
  AND ($5 = '' OR v3 = $5) AND ($6 = '' OR v4 = $6) AND ($7 = '' OR v5 = $7);
-- name: CountCasbinRules :one
SELECT count(*) FROM casbin_rule;

-- name: InsertAuditLog :one
INSERT INTO audit_log (actor_uid, action, target, detail_json) VALUES ($1, $2, $3, $4) RETURNING id;
```

Replace `internal/db/migrations/000001_init.sql` with a goose migration wrapping the same 8 tables (`-- +goose Up` all CREATE TABLEs + indexes, `-- +goose Down` DROP TABLEs in reverse order). Keep `internal/db/schema/000001_placeholder.sql` (it is a no-op comment; harmless).

- [ ] **Step 6: Run codegen**

```bash
cd "$SEED/rbac-rpc"
go mod tidy
make sqlc        # sqlc generate -f internal/db/sqlc.yaml
make update      # kitex -template-dir template/kitex-template -type protobuf idl/auth.proto
```
Expected: `internal/db/gen/` generated (models + queries for all 8 tables), `kitex_gen/api/rbac/v1/` generated (both service stubs), base `internal/handler/{authservice,rbacservice}/handler.go` + `internal/usecase/` + `internal/repository/` stubs written by the base templates.

- [ ] **Step 7: Verify seed compiles with base stubs**

```bash
cd "$SEED/rbac-rpc" && go build ./...
```
Expected: PASS (base scaffold + generated stubs compile; handler/usecase stubs return `not_implemented`).

- [ ] **Step 8: Commit (in the seed's own git repo, scratch — no ncgo-templates commit yet)**

```bash
cd "$SEED" && git init -q 2>/dev/null; git add -A && git commit -qm "seed: rbac-rpc scaffold + proto + sqlc schema/query + codegen"
```

---

## Task 2: DDD domain layer (pure Go + unit tests)

**Files:**
- Create: `internal/domain/user/{entity,valueobject,service,repository}.go`, `internal/domain/role/{entity,service,repository}.go`, `internal/domain/permission/{entity,valueobject,repository}.go`, `internal/domain/menu/{entity,service,repository}.go`
- Test: `internal/domain/.../*_test.go` per aggregate

**Interfaces:**
- Consumes: nothing (pure Go; `context` only).
- Produces: domain types + ports consumed by Task 4 (repos) and Task 5 (app services):
  - `user.User{ID int64, Username, PasswordHash, Status string}`; `user.Status` consts `user.StatusActive`/`user.StatusDisabled`; `user.New(username, passwordHash string) (*User, error)` (validates username non-empty, ≥3 chars); `user.Repository` port: `GetByID(ctx,id)`, `GetByUsername(ctx,name)`, `List(ctx, limit, offset)`, `Save(ctx, *User)`, `Delete(ctx,id)`, `SetStatus(ctx,id,status)`; `user.NotFoundError`, `user.ValidationError`.
  - `role.Role{ID int64, Code, Name string}`; `role.Repository`: `GetByID/GetByCode/List/Save/Delete`; `role.Assign(ctx, roleID, permissionIDs []int64) error` domain rule: code non-empty; `role.NotFoundError`.
  - `permission.Permission{ID int64, Code, Name, Kind, Method string}`; `permission.KindMenu|KindButton|KindAPI` consts; `permission.New(code,name,kind,method) (*Permission, error)` (validates kind ∈ {menu,button,api}); `permission.Repository`: `GetByID/GetByCode/ListByIDs/List/Save/Delete`.
  - `menu.Menu{ID, ParentID int64, Type, Name, Path, Component, PermCode string, SortOrder int32}`; `menu.KindDir|KindMenu|KindButton`; `menu.Node{Menu, Children []*Node}`; `menu.BuildTree(items []*Menu) []*Node` (pure, stable by SortOrder then ID); `menu.Repository`: `GetByID/List/ListByPermCodes/Save/Delete`.
- Domain errors: each aggregate defines `type XNotFoundError` (implements `error`, used by app layer for `rpcerror` mapping). Keep them tiny.

- [ ] **Step 1: Write the failing tests** — per aggregate, pure unit tests:
  - `user`: `New` rejects short username; `SetStatus` validates enum.
  - `role`: `New` rejects empty code.
  - `permission`: `New` rejects unknown kind; normalizes method to `*` for menu/button and default `GET` for api when empty.
  - `menu`: `BuildTree` — given flat list builds nested tree by parent_id, ordered by SortOrder then ID; orphans hang as roots.

- [ ] **Step 2: Run tests to verify they fail** — `go test ./internal/domain/... -count=1` → FAIL (no packages).
- [ ] **Step 3: Implement domain files** (entities + VOs + domain services + repository ports + errors, matching the signatures above). Keep each file small and pure.
- [ ] **Step 4: Run tests to verify they pass** — `go test ./internal/domain/... -count=1` → PASS.
- [ ] **Step 5: Commit** — `git add internal/domain && git commit -qm "feat(seed): DDD domain layer (user/role/permission/menu)"`.

---

## Task 3: Infrastructure — casbin adapter + JWT + password + token store + audit

**Files:**
- Create: `internal/infrastructure/casbin/{adapter.go,enforcer.go,model.conf}` (+ `adapter_test.go`), `internal/infrastructure/auth/{jwt.go,password.go}` (+ tests), `internal/infrastructure/token/{store.go,memory.go,redis.go}` (+ memory test), `internal/infrastructure/audit/writer.go`

**Interfaces:**
- Consumes: `internal/db/gen` (sqlc Queries), domain errors.
- Produces (consumed by Task 4/5/6):
  - `casbin.PolicyStore` — `List(ctx) ([]Policy, error)`, `Insert(ctx, Policy) error`, `Delete(ctx, Policy) error`, `DeleteFiltered(ctx, ptype string, filter []string) error`; `casbin.Policy{Ptype string; V [6]string}`.
  - `casbin.NewSQLPolicyStore(q *gen.Queries) *casbin.SQLPolicyStore` (sqlc-backed) and `casbin.NewMemoryPolicyStore() *casbin.MemoryPolicyStore` (test-only).
  - `casbin.Adapter` implements `persist.Adapter` (from `github.com/casbin/casbin/v2/persist`) over a `PolicyStore`.
  - `casbin.NewEnforcer(store PolicyStore) (*casbin.Enforcer, error)` — loads embedded `model.conf`, builds adapter, `LoadPolicy`.
  - `auth.JWTManager{Sign(uid int64, roles []string, ttl time.Duration) (string, error); Parse(token string) (*Claims, error)}`; `auth.Claims{Uid int64, Roles []string, jwt.RegisteredClaims}`; HS256, secret from config.
  - `auth.HashPassword(plain string) (string, error)` + `auth.VerifyPassword(hash, plain string) (bool, error)` — argon2id (alexedwards/argon2id).
  - `token.Store` — `SetRefresh(ctx, uid, refreshToken string, ttl time.Duration) error`, `GetRefresh(ctx, refreshToken) (uid int64, err error)`, `DeleteRefresh(ctx, refreshToken) error`, `Blacklist(ctx, accessToken string, ttl time.Duration) error`, `IsBlacklisted(ctx, accessToken) (bool, error)`; `token.NewMemoryStore() *token.MemoryStore` (default) and `token.NewRedisStore(redisAddr, keyPrefix string) *token.RedisStore` (seam).
  - `audit.Writer` — `Write(ctx, actorUID int64, action, target, detailJSON string) error`; `audit.NewSQLWriter(q *gen.Queries) *audit.SQLWriter` and `audit.NewMemoryWriter() *audit.MemoryWriter` (test-only).

- [ ] **Step 1: Write the failing tests**
  - `casbin`: adapter round-trip against `MemoryPolicyStore` — build enforcer with model.conf, `AddPolicy("admin","user:create","POST")`, `Enforce("1","user:create","POST")` → allowed (with `g("1","admin")`); `SavePolicy` then a fresh enforcer `LoadPolicy` sees the policy; `RemoveFilteredPolicy` removes only matching subset.
  - `auth`: JWT sign/parse round-trip (claims uid+roles), expired token rejected; password hash/verify round-trip + wrong password rejected.
  - `token`: memory store refresh set/get/delete + blacklist round-trip.
- [ ] **Step 2: Run tests to verify they fail.**
- [ ] **Step 3: Implement**
  - `model.conf`:
    ```
    [request_definition]
    r = sub, obj, act
    [policy_definition]
    p = sub, obj, act
    [role_definition]
    g = _, _
    [policy_effect]
    e = some(where (p.eft == allow))
    [matchers]
    m = g(r.sub, p.sub) && r.obj == p.obj && r.act == p.act
    ```
    Embed via `//go:embed model.conf`.
  - Adapter: `LoadPolicy` → store.List → for each `p`/`g` line call `model.AddPolicy`/`AddRoleForUser`; `SavePolicy` → `store.DeleteFiltered("", nil)` + re-insert all; `AddPolicy`/`RemovePolicy` → store.Insert/Delete; `RemoveFilteredPolicy` → store.DeleteFiltered.
  - JWT: HS256 via `github.com/golang-jwt/jwt/v5`, claims `{uid, roles, exp}`; `Parse` verifies signature + expiry.
  - Password: `github.com/alexedwards/argon2id`.
  - Token store: `MemoryStore` = map + TTL expiry; `RedisStore` = `SET/GET/DEL` on a `refresh:{token}` key + `SETEX` on `blacklist:{token}` (documented seam, no redis client wired in v1 beyond the type).
  - Audit: `SQLWriter` inserts `InsertAuditLog`; `MemoryWriter` appends to a slice (test).
- [ ] **Step 4: Run tests to verify they pass.**
- [ ] **Step 5: `go build ./...` + `go test ./internal/infrastructure/... -count=1` PASS.**
- [ ] **Step 6: Commit** — `git add internal/infrastructure && git commit -qm "feat(seed): infrastructure — casbin adapter, JWT, argon2, token store, audit"`.

---

## Task 4: sqlc-backed repositories (domain port impls)

**Files:**
- Create: `internal/repository/user/repo.go`, `internal/repository/role/repo.go`, `internal/repository/permission/repo.go`, `internal/repository/menu/repo.go`

**Interfaces:**
- Consumes: `internal/db/gen` (Queries + pgx), domain ports (Task 2).
- Produces: concrete `*userrepo.Repo` (implements `user.Repository`), `*rolerepo.Repo`, `*permissionrepo.Repo`, `*menurepo.Repo`. Each `New(q *gen.Queries, pool *pgxpool.Pool) *Repo` + `WithTx(ctx, fn) error` (mirror the base repository.yaml `WithTx` pattern). Repos map domain ↔ sqlc types and translate `pgx.ErrNoRows` → domain `NotFoundError`.

- [ ] **Step 1: Implement the four repos** (map every port method to the matching sqlc query; `user.Repo` exposes `AssignRoles(ctx, uid, roleIDs)`, `ClearRoles`, `ListRoles(ctx, uid)`; `role.Repo` exposes `AssignPermissions`, `ClearPermissions`, `ListPermissions(ctx, roleIDs)`; `menu.Repo` exposes `ListByPermCodes`).
- [ ] **Step 2: Verify compile + gen query names exist**

```bash
cd "$SEED/rbac-rpc" && go build ./...
```
Expected: PASS. If a sqlc query name was mistyped, fix the query name or the call — do NOT skip.

- [ ] **Step 3: Add a postgres-gated integration test** for one repo (user round-trip: Create→GetByUsername→SetStatus→Delete) behind `if _, err := exec.LookPath("pg_isready"); ...` + a reachable `POSTGRES_DSN` env (skip with explicit `skipped:` line when absent — mirrors ratelimit-hertz e2e gating).
- [ ] **Step 4: Commit** — `git add internal/repository && git commit -qm "feat(seed): sqlc repository impls for DDD ports"`.

---

## Task 5: Application services + unit tests

**Files:**
- Create: `internal/application/auth/{auth_service.go,dto.go}`, `internal/application/user/{user_service.go,dto.go}`, `internal/application/role/{role_service.go,dto.go}`, `internal/application/permission/{permission_service.go,dto.go}`, `internal/application/menu/{menu_service.go,dto.go}`, `internal/application/rbac/{enforce_service.go,dto.go}`
- Test: `internal/application/.../*_test.go`

**Interfaces:**
- Consumes: domain ports, infrastructure (Task 3), casbin enforcer, `internal/db/gen` for the casbin/audit writers.
- Produces (consumed by Task 6 handlers):
  - `auth.Service{Login(ctx, username, password) (access, refresh string, expiresIn int32, err error); Refresh(ctx, refreshToken) (access, refresh string, err error); Logout(ctx, accessToken string) error; ValidateToken(ctx, accessToken) (uid int64, roles []string, valid bool)}`. Wires `user.Repository` (GetByUsername) + `auth.VerifyPassword` + `token.Store` + `auth.JWTManager` + `audit.Writer`.
  - `user.Service{Create/Update/Delete/Get/List; AssignRoles(ctx, uid, roleIDs) error}` — after AssignRoles writes `user_roles`, calls `enforcer` to set `g(uid, role_code)` for each role (get roles by IDs → `AddRoleForUser(uid, code)`), then audit.
  - `role.Service{Create/Update/Delete/List; GrantPermissions(ctx, roleID, permissionIDs) error}` — writes `role_permissions`, then for each permission `enforcer.AddPolicy(role_code, perm.code, perm.method)`, then audit.
  - `permission.Service{Create/Delete/List}` (Create validates kind via domain; Delete cascades via `ClearRolePermissions` + `enforcer.RemoveFilteredPolicy("p", 1, code)`), audit.
  - `menu.Service{Create/Update/Delete/List; UserMenuTree(ctx, uid) ([]*menu.Node, error); UserPermCodes(ctx, uid) ([]string, error)}` — UserPermCodes: roles by user → permissions by role IDs → codes; if user has role code `"admin"`, return all permissions. UserMenuTree: same role/perm resolution → `ListMenusByPermCodes(codes)` → `menu.BuildTree`. Audit on Create/Update/Delete.
  - `rbac.EnforceService{Enforce(ctx, uid, obj, act) (bool, error)}` — `enforcer.Enforce(fmt.Sprint(uid), obj, act)`.
  - Each service constructor takes its deps as interfaces (small local interfaces over ports) so tests can fake them. Audit is always `audit.Writer`.

- [ ] **Step 1: Write the failing tests** (fake repos + memory enforcer + memory token store + memory audit + real JWT/argon2):
  - `auth`: Login success returns 3-part tokens + writes audit; Login wrong password → error; ValidateToken ok/invalid; Refresh rotates; Logout blacklists access token (IsBlacklisted true).
  - `user`: Create hashes password, saves; AssignRoles adds `g(uid, role_code)` to a memory casbin enforcer and `Enforce(uid, perm_code, method)` returns true for a permission granted to that role.
  - `role`: GrantPermissions writes `p(role_code, perm_code, method)`; Enforce allows; audit written.
  - `menu`: UserMenuTree returns only nodes whose perm_code ∈ user's permission codes; `admin` sees all menus. UserPermCodes returns the union of granted codes.
  - `permission`: Create rejects unknown kind; Delete removes `p(role_code, code, *)`.
  - `rbac`: Enforce true when policy+role granted, false otherwise.
- [ ] **Step 2: Run tests to verify they fail.**
- [ ] **Step 3: Implement app services** per the interfaces above. Keep each service focused; cross-aggregate consistency (grant/assign sync into casbin) lives HERE, not in the domain.
- [ ] **Step 4: Run tests to verify they pass** — `go test ./internal/application/... -count=1` PASS.
- [ ] **Step 5: `go build ./...` PASS.**
- [ ] **Step 6: Commit** — `git add internal/application && git commit -qm "feat(seed): application services (auth + rbac) with policy-sync"`.

---

## Task 6: Handlers + server wiring

**Files:**
- Create: `internal/handler/authservice/handler.go`, `internal/handler/rbacservice/handler.go` (replace the base-generated stubs)
- Modify: `internal/base/server/server.go` (replace base wiring), `internal/base/conf/conf.go` (add AuthConfig: JWT secret + token TTLs + store mode), `conf/dev/conf.yaml`

**Interfaces:**
- Consumes: Task 5 app services.
- Produces: kitex server interfaces (`rbacv1.AuthService`, `rbacv1.RBACService`) implemented by `authservicehandler.AuthServiceImpl` / `rbacservicehandler.RBACServiceImpl`; `server.Run()` wires everything.

- [ ] **Step 1: Write the handlers** — thin; map kitex req/resp ↔ app service args; errors → `rpcerror.ToBizError(err)` (same as base handler pattern). `AuthServiceImpl` fields: `auth *auth.Service`; `RBACServiceImpl` fields: `user *user.Service`, `role *role.Service`, `permission *permission.Service`, `menu *menu.Service`, `rbac *rbac.EnforceService`.
- [ ] **Step 2: Extend conf** — add to `Config`: `Auth AuthConfig`; define:
  ```go
  type AuthConfig struct {
      JWTSecret        string `json:"jwt_secret" yaml:"jwt_secret"`
      AccessTTLSeconds int    `json:"access_ttl_seconds" yaml:"access_ttl_seconds"`
      RefreshTTLSeconds int   `json:"refresh_ttl_seconds" yaml:"refresh_ttl_seconds"`
      TokenStore       string `json:"token_store" yaml:"token_store"` // memory | redis (seam)
      RedisAddr        string `json:"redis_addr" yaml:"redis_addr"`
  }
  ```
  Wire into `conf.Default()` and `conf/dev/conf.yaml` (`jwt_secret: dev-secret-change-me`, TTLs 3600/604800, `token_store: memory`).
- [ ] **Step 3: Rewrite `internal/base/server/server.go`** — keep the base structure (OTel block, `do` injector, address resolve, observability middleware, kitex server), but wire:
  ```go
  pool, cleanup := provideRepository(cfg.Database)   // existing helper
  q := gen.New(pool)
  userRepo := userrepo.New(q, pool)                  // user.Repository
  roleRepo := rolerepo.New(q, pool)
  permRepo := permissionrepo.New(q, pool)
  menuRepo := menurepo.New(q, pool)
  casbinStore := casbin.NewSQLPolicyStore(q)
  enforcer, _ := casbin.NewEnforcer(casbinStore)
  jwtMgr := auth.NewJWTManager(cfg.Auth.JWTSecret)
  tokenStore := token.NewMemoryStore()               // or RedisStore when cfg.Auth.TokenStore=="redis"
  auditWriter := audit.NewSQLWriter(q)
  userSvc := usersvc.New(userRepo, enforcer, auditWriter)
  roleSvc := rolesvc.New(roleRepo, permRepo, enforcer, auditWriter)
  permSvc := permsvc.New(permRepo, roleRepo, enforcer, auditWriter)
  menuSvc := menusvc.New(menuRepo, userRepo, permRepo, enforcer, auditWriter)
  rbacSvc := rbacsvc.New(enforcer)
  authSvc := authsvc.New(userRepo, jwtMgr, tokenStore, auditWriter)
  // register both services:
  authServer := rbacv1.NewServer(new(authservicehandler.AuthServiceImpl, authSvc))
  rbacServer := rbacv1.NewServer(new(rbacservicehandler.RBACServiceImpl, userSvc, roleSvc, permSvc, menuSvc, rbacSvc))
  // ... serve both on the same addr via two goroutines (or kitex multiple services option)
  ```
  Exact kitex multi-service registration follows the generated `kitex_gen/api/rbac/v1` package API (check the generated `server.go` in kitex_gen for the `NewServer` signature — one `NewServer` per service with its handler; both can share the addr via `kitexserver.WithServiceAddr`).
- [ ] **Step 4: `make update`** (regen kitex_gen against final proto) then `go build ./...` PASS. `go vet ./...` clean.
- [ ] **Step 5: Smoke** — `go run .` starts and binds the configured port (run with a 3s timeout; expect it to start and be killed by timeout — that IS the pass signal; no panic). If it fails because DB is down, confirm the failure is the expected `database is unavailable` and gate the smoke behind `POSTGRES_DSN` (skip otherwise with `skipped:`).
- [ ] **Step 6: Commit** — `git add internal/handler internal/base/server internal/base/conf conf && git commit -qm "feat(seed): RPC handlers + server wiring for AuthService + RBACService"`.

---

## Task 7: Full seed validation

- [ ] **Step 1: `gofmt -l .`** (exclude `internal/db/gen`, `kitex_gen`) → no output.
- [ ] **Step 2: `go vet ./...`** → clean.
- [ ] **Step 3: `go build ./...`** → PASS.
- [ ] **Step 4: `go test ./... -count=1`** → PASS (hermetic: domain/application/infrastructure tests run without DB).
- [ ] **Step 5: `make sqlc`** regenerates clean; `make update` regenerates clean (idempotent).
- [ ] **Step 6: Commit** — `git add -A && git commit -qm "chore(seed): full validation green"`.

---

## Task 8: Export + assemble the `rbac-kitex` package

**Files:**
- Create: `ncgo-templates/rbac-kitex/{template.yaml,README.md}`
- Copy: exported `kitex-template/*.yaml` + `idl/*.proto` from seed → `rbac-kitex/`
- Hand-assemble: `internal/infrastructure/**` (casbin/adapter,enforcer,model.conf + auth/jwt,password + token/store,memory,redis + audit/writer), `internal/db/schema/000001_rbac.sql`, `internal/db/query/rbac.sql`, `internal/db/migrations/000001_init.sql` yamls
- Modify: `ncgo-templates/README.md` (add `rbac-kitex` row)

**Interfaces:**
- Consumes: seed (Tasks 1-7 output), `ncgo export templates`.
- Produces: a self-contained template package consumable by `ncgo new --kind kitex --template-dir <pkg>`.

- [ ] **Step 1: Export from the seed**

```bash
cd "$SEED/rbac-rpc"
/tmp/ncgo-rbac export templates --kind kitex --root .
```
Expected: `template/kitex-template/*.yaml` + `template/idl/auth.proto`. Inspect the yamls: DDD domain/application files present with `skip` + no `loop_service`; handler/repository yamls present.

- [ ] **Step 2: Copy exported files into the package**

```bash
PKG=/Users/xs/Documents/workspce/github.com/byx-darwin/ncgo-templates/rbac-kitex
mkdir -p "$PKG/kitex-template" "$PKG/idl"
cp "$SEED/rbac-rpc/template/kitex-template/"*.yaml "$PKG/kitex-template/"
cp "$SEED/rbac-rpc/template/idl/"*.proto "$PKG/idl/"
```

- [ ] **Step 3: Post-process exported yamls** — a small script that, for each yaml in `kitex-template/`, parses `path:` and removes the `loop_service: true` line when the path contains no `{{`. Verify by example: `internal_repository_user_repo_go.yaml`, `internal_repository_role_repo_go.yaml`, `internal_handler_authservice_handler_go.yaml` must NOT have `loop_service`; `internal_handler_{{ToLower_ServiceName}}_handler_go.yaml` (if any) keeps it. Also confirm no exported body still contains the literal module `github.com/byx-darwin/rbac-rpc` (must be `{{.Module}}`) and no literal `rbac-rpc`/`rbacrpc` tokens remain in `path` fields (rerun `ncgo export` fixes any; do not hand-edit bodies).

```bash
python3 - "$PKG/kitex-template" <<'PY'
import glob,os,sys,re
d=sys.argv[1]
for f in glob.glob(d+"/*.yaml"):
    s=open(f).read()
    m=re.search(r'^path:\s*(.*)$',s,re.M)
    if m and "{{" not in m.group(1):
        s=re.sub(r'(?m)^loop_service: true\s*$','',s)
        open(f,"w").write(s)
print("post-processed")
PY
```

- [ ] **Step 4: Hand-assemble the non-exported files as template yamls** — for each file below, wrap the seed file's exact content into a yaml using `path` + `update_behavior` + `body: |-` (content indented 2 spaces), matching the rule-center yaml style:

| target path | behavior |
|---|---|
| `internal/infrastructure/casbin/adapter.go` | cover |
| `internal/infrastructure/casbin/enforcer.go` | cover |
| `internal/infrastructure/casbin/model.conf` | cover |
| `internal/infrastructure/auth/jwt.go` | cover |
| `internal/infrastructure/auth/password.go` | cover |
| `internal/infrastructure/token/store.go` | cover |
| `internal/infrastructure/token/memory.go` | cover |
| `internal/infrastructure/token/redis.go` | cover |
| `internal/infrastructure/audit/writer.go` | cover |
| `internal/db/schema/000001_rbac.sql` | cover |
| `internal/db/query/rbac.sql` | cover |
| `internal/db/migrations/000001_init.sql` | cover (overrides base placeholder) |

Use a deterministic helper: `make_yaml() { printf '# rbac-kitex template — %s\npath: %s\nupdate_behavior:\n  type: %s\nbody: |-' "$1" "$1" "$2"; sed 's/^/  /' "$3"; }`. YAML filename = path with `/`→`_` and `.`→`_` + `.yaml` (matches `yamlFileName`). Run `gofmt`/`go vet` on the extracted seed bodies implicitly by the seed's green build. Verify no `{{`/`}}` residuals that the escape helper would miss (the seed contains none — model.conf has none; SQL none).

- [ ] **Step 5: Write `template.yaml`**

```yaml
name: rbac-kitex
kind: kitex
description: Official RBAC + auth authority Kitex RPC template (DDD: users/roles/permissions/menus, Casbin enforcement, JWT login, audit)
version: "1"
skip_default_templates:
  - handler.yaml
  - usecase.yaml
  - repository.yaml
  - server.yaml
```

- [ ] **Step 6: Write `README.md`** (EN, matching rule-center/base-kitex tone): Use snippet, Contents (what the DDD layers + infra provide), Variables (`{{.Module}}`, `{{.ServiceName}}`, `{{ToLower .ServiceName}}`), and a **Seams** section (documented TODO): data-scope/org-tree (`dom` column + departments), RS256/JWKS, local casbin enforcer + watcher (BFF uses `Enforce` RPC), Redis token store (memory default), OTel.
- [ ] **Step 7: Add the registry row** to `ncgo-templates/README.md`:

```
| `rbac-kitex` | kitex | RBAC + auth authority service (DDD, Casbin sqlc adapter, JWT login, audit) | ✅ `ncgo new --kind kitex --template rbac-kitex` |
```

- [ ] **Step 8: Verify package loads** — `cd ncgo-templates && /tmp/ncgo-rbac template pull rbac-kitex` (or use the dir directly) then confirm `LoadPackage` path works in Task 9.
- [ ] **Step 9: Commit** — `git add rbac-kitex README.md && git commit -qm "feat(rbac-kitex): RBAC + auth authority Kitex template (#10)"`.

---

## Task 9: e2e verification + PR

**Files:**
- Create: `ncgo-templates/rbac-kitex/test/e2e_test.sh`
- Modify: `ncgo-templates/.github/workflows/*` (if CI exists; otherwise run locally only)

**Interfaces:**
- Consumes: the assembled package.
- Produces: proof the package generates a working project.

- [ ] **Step 1: Write `test/e2e_test.sh`** (mirror `ratelimit-hertz/test/e2e_test.sh`): gate tools (`command -v kitex protoc sqlc`; `skipped:` + exit 0 on missing); `ncgo new rbac-e2e --kind kitex --module example.com/rbac-e2e --db postgres --template-dir <pkg> --no-auto-steps`; static asserts (no residual brace-escapes, no unresolved `{{...}}` in `.go`); `make sqlc`; `go mod tidy; go build ./...`; `go test ./...` (hermetic app/domain/infra tests pass without DB); postgres-gated `go test` with `pg_isready`+`POSTGRES_DSN` skip line; explicit `FAILS` accumulation + non-zero exit on failure.
- [ ] **Step 2: Run the e2e locally** — `bash rbac-kitex/test/e2e_test.sh` → all green (or explicit `skipped:` lines).
- [ ] **Step 3: Verify the registry-consumed path** — `ncgo template pull rbac-kitex` then `ncgo new --kind kitex --template rbac-kitex` in a temp dir → build+test green. (If `template pull` needs registry publishing, verify the local-dir path is green and note the pull step as release-time.)
- [ ] **Step 4: Full repo checks** — `gofmt -l rbac-kitex/`, `cd ncgo-templates && git status` (only `rbac-kitex/` + `README.md` changed).
- [ ] **Step 5: Open the PR** — `gf pr create` with `Closes #10`, squash merge on approval.

---

## Self-Review

**Spec coverage:**
- DDD aggregates user/role/permission/menu → Task 2.
- Casbin sqlc adapter + model `sub,obj,act` + `casbin_rule` single-source → Task 3 (+ Task 5 policy-sync).
- Unified permission code (permissions.code == casbin obj == menus.perm_code) → schema (Task 1), app services (Task 5), menu tree/perm codes (Task 5).
- Auth JWT HS256 + argon2id + refresh/blacklist → Task 3, 5.
- Audit log on RBAC mutations → Task 3, 5.
- RPC AuthService + RBACService → proto (Task 1), handlers + wiring (Task 6).
- Build via ncgo → export → assemble → verify → Tasks 1, 8, 9.
- README documents seams → Task 8 Step 6.

**Placeholder scan:** No TBD/TODO. Every task has concrete files, signatures, code (proto/schema/queries/model.conf/conf structs are inline), commands, and pass criteria. The kitex_gen `NewServer` signature and `POSTGRES_DSN` gating are resolved at implementation with a stated fallback (read the generated package; gate on `pg_isready`).

**Type consistency:** Domain ports (`user.Repository`, `role.Repository`, `permission.Repository`, `menu.Repository`) are consumed by Task 4 impls and Task 5 services; infrastructure interfaces (`casbin.PolicyStore`, `auth.JWTManager`, `token.Store`, `audit.Writer`) are produced in Task 3 and consumed in Tasks 4-6; handler constructors match Task 5 service constructors; sqlc query names in Task 1 match repo call sites in Task 4.
