# rbac-kitex 模板对齐 s-web 前端契约 — 文档修订实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 修订 rbac-kitex 模板的两份设计文档（`2026-08-18-rbac-kitex-design.md` + `2026-08-18-rbac-kitex-template.md`），使其反映 wf-2026-08-19-003 Phase 1 锁定的 6 项对齐决策。

**Architecture:** 修订是纯文档作业（不触及 ncgo 代码、不触及 s-web 前端、不触及 admin-bff-hertz）。Task 1 修订权威设计文档（~120 行），Task 2 修订详细计划文档（~750 行，IDL/SQL/DDD 任务链需同步调整），Task 3 做交叉一致性校验。Task 1 必须先完成，因为 Task 2 的修订以 Task 1 的目标态为基准。

**Tech Stack:** Markdown；Issue #75；decisions doc: `docs/superpowers/specs/2026-08-19-rbac-kitex-alignment-decisions.md`；前置评估: `docs/superpowers/specs/2026-08-19-rbac-kitex-s-web-alignment.md`。

**Spec:** `docs/superpowers/specs/2026-08-19-rbac-kitex-alignment-decisions.md` (wf-2026-08-19-003 Phase 1 产物)。

## Global Constraints

- **6 项锁定决策**（详见 decisions doc §1-§6）：单一 Permission 树、`UNIQUE(code, type)`、`GrantPermissionsToRole` 用 `permission_codes`、`status int` 统一、Permission RPC 拥有所有写路径 + UpdatePermission、scope = template only。
- **修订边界**：仅触及 ncgo 仓库内两份设计/计划文档。不触及 s-web 前端（永不在范围）、admin-bff-hertz（下游独立 issue）、ncgo Go 代码、模板实现（ncgo-templates/rbac-kitex/ 尚未存在）。
- **AC 映射**：AC#1-#3 已由 decisions doc 锁定；本计划执行 AC#4（更新设计文档 + 计划文档）。
- **一致性约束**：修订后的设计文档与计划文档必须在「表结构 / RPC 表面 / DDD 聚合职责 / 字段清单 / status 语义」五项上完全一致，并与 decisions doc §1-§6 对齐。
- **Casbin 零影响**：decisions doc §5 评估 Casbin 模型/策略/适配器均无变化；文档修订仅做"已确认"标注，不引入新约束。
- **保留计划文档的 TDD 骨架**：计划文档的 Task 结构（9 个 task：seed → domain → infra → repos → app services → handlers → validation → export → e2e）保持不变；只修订受影响任务的内容（proto/SQL/domain/service 接口）。

---

## Task 1: 修订权威设计文档（`2026-08-18-rbac-kitex-design.md`）

**Files:**
- Modify: `docs/superpowers/specs/2026-08-18-rbac-kitex-design.md`

**Interfaces:**
- Consumes: decisions doc §1-§6（目标态来源）；当前设计文档（修订基线）。
- Produces: 修订后的设计文档，章节目标态见下。后续 Task 2 以此文档为修订基准。

### 修订清单（按章节顺序）

#### 1.1 "Locked decisions" 表 — 新增 6 行 s-web 对齐决策

在现有 8 行 locked decisions 表**末尾追加** 6 行（保留原 8 行不变）：

| Dimension | Decision |
|---|---|
| Permission data model (s-web alignment) | **单一 Permission 树**（`permissions` 表承载 `catalog`/`menu`/`button`/`api` 四种 type，`menus` 作为 `WHERE type IN ('catalog','menu')` 的过滤视图；原 `menus` 表删除） |
| Permission code uniqueness | `UNIQUE(code, type)` 联合唯一 —— 同一 `code` 允许在 `button` 型与 `api` 型各出现一次（s-web 事实：`user:create` 同时驱动按钮渲染与接口鉴权） |
| Authorization payload | `GrantPermissionsToRole` 载荷从 `permission_ids []int64` 改为 `permission_codes []string`；端到端（前端 → BFF → RPC → Casbin）统一使用 `code` 作为外部标识符 |
| status 语义 | `status int`（`1=enabled`, `0=disabled`）统一用于 User / Role / Permission；原 User `status` enum (`active`/`disabled`) 改为 `int` |
| RPC 表面修订 | Permission RPC 拥有所有写路径（新增 `UpdatePermission` + `GetPermission`；移除 `Menu.CreateMenu`/`UpdateMenu`/`DeleteMenu`）；Menu RPC 仅保留只读树查询（`GetUserMenuTree`、`ListMenus`） |
| Scope boundary | 本模板仅对齐 s-web 契约；s-web 前端永不在修改范围；admin-bff-hertz 适配作为下游独立 issue |

#### 1.2 "DDD structure" 节 — Menu 聚合职责降级

将当前内容：
```
Aggregates: **user, role, permission, menu**.
```

替换为：
```
Aggregates: **user, role, permission**（写聚合）；**menu**（只读查询聚合）。

`permission` 聚合现在承载完整树（`catalog`/`menu`/`button`/`api`），其写路径覆盖原 `menu` 聚合的 CRUD。
`menu` 聚合保留为只读查询聚合，职责为 `GetUserMenuTree(uid)` 与 `ListMenus(filter)` 的树形组装（按 type∈{catalog,menu} 过滤 + 按 sort/id 排序）。
```

其余 DDD 目录结构保持（`internal/domain/permission/`、`internal/domain/menu/` 仍存在），但在 `permission/repository.go` 行追加注释 `(owns all writes; tree CRUD via Permission)`，在 `menu/service.go` 行替换为 `menu/query_service.go (read-only tree queries)`。

#### 1.3 "Data model" 节 — 表结构大幅修订

**完全替换**当前 `Data model (postgres + sqlc)` 节内容。目标态：

```
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
```

#### 1.4 "Casbin" 节 — 增加已确认标注

在当前 Casbin 节末尾追加一段：

```
**s-web 对齐影响评估（wf-2026-08-19-003 §5）：** 零影响。`casbin_rule` 表结构、`p` 策略 `(role_code, permission_code, http_method)`、`g` 映射 `(user_id, role_code)`、`model.conf` 均不变化。Casbin 仅消费 `code`，对 `permissions.code+type` 联合唯一无感。
```

#### 1.5 "RPC surface (proto)" 节 — 修订 RBACService

将当前 RBACService 列表替换为：

```
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
```

#### 1.6 "Open points for review" — 标注已决议

将当前 3 条 open points 替换为：

```
## Open points (resolved by wf-2026-08-19-003)

1. ~~Aggregate set `user/role/permission/menu` — enough for v1?~~ ✅ 确认；`menu` 退化为只读查询聚合，写路径并入 `permission`（decisions doc §1, §6）。
2. ~~`permissions.kind` enum `menu|button|api` vs simpler — keep the kind column for clarity?~~ ✅ 保留并扩展为 `type` 列，新增 `catalog` 取值（decisions doc §1）。
3. ~~Enforce via RPC in v1 vs client-side enforcer~~ ✅ v1 仍走 RPC，enforcer 作为 seam（decisions doc 不影响此决策）。

新增 open point（deferred to downstream issue）:
4. `admin-bff-hertz` 适配到修订后的模板契约 —— 不在本 issue 范围，作为下游独立 issue 跟进（decisions doc §7）。
```

#### 1.7 文档头部 — 增加 wf 引用

在文档首行（`# Design: \`rbac-kitex\` — ...`）下追加：

```
> **Alignment revision (wf-2026-08-19-003):** 本文档已根据 `2026-08-19-rbac-kitex-alignment-decisions.md` 的 6 项锁定决策修订。修订点：Locked decisions 表（+6 行）、DDD structure（menu 降级）、Data model（单一 Permission 树）、RPC surface（UpdatePermission + Menu CRUD 移除 + codes 载荷）、Open points（已决议）。
```

### 验证

- [ ] **Step 1: 修订后 `gofmt` 不适用（纯 MD）** —— 用 markdown 语法检查：无 broken links、无 orphan headers、表格对齐。
- [ ] **Step 2: 章节目标态比对** —— 打开 decisions doc，逐项核对：
  - §1 (single tree) → 设计文档 Data model 节
  - §2 (UNIQUE(code,type)) → Data model 节的 permissions 表描述
  - §3 (codes payload) → RPC surface 节的 GrantPermissionsToRole 行
  - §4 (status int + 字段补齐) → Data model 节的 users/roles/permissions 字段
  - §5 (Casbin 零影响) → Casbin 节的确认标注
  - §6 (RPC 表面) → RPC surface 节全部修订
  - §7 (scope) → Open points 节的 #4 deferred item
- [ ] **Step 3: 内部一致性** —— 设计文档内无自相矛盾（例如 DDD 节说 menu 只读、Data model 节无 menus 表、RPC 节无 Menu CRUD —— 三者一致）。
- [ ] **Step 4: Commit**

```bash
git add docs/superpowers/specs/2026-08-18-rbac-kitex-design.md
git commit -m "docs(specs): revise rbac-kitex design doc per wf-2026-08-19-003 decisions

- Add 6 s-web alignment locked decisions to the Locked decisions table
- DDD structure: menu aggregate degrades to read-only query aggregate
- Data model: single Permission tree replaces permissions+menus split
- UNIQUE(code, type) composite for button/api code coexistence
- GrantPermissionsToRole payload switches to permission_codes
- status int unified across User/Role/Permission
- RPC surface: Permission owns writes + UpdatePermission added;
  Menu Create/Update/Delete removed; Menu keeps read-only tree queries
- Open points: mark resolved per decisions doc

Refs: #75, wf-2026-08-19-003"
```

---

## Task 2: 修订详细计划文档（`2026-08-18-rbac-kitex-template.md`）

**Files:**
- Modify: `docs/superpowers/plans/2026-08-18-rbac-kitex-template.md`

**Interfaces:**
- Consumes: Task 1 修订后的设计文档（目标态来源）；decisions doc；当前计划文档（修订基线）。
- Produces: 修订后的计划文档，各任务链反映新的 proto/SQL/domain/service 接口。

### 修订策略

计划文档 756 行，修订影响 Task 1/2/4/5/6（proto/SQL/domain/repos/services/handlers）。Task 3（infrastructure）、Task 7（validation）、Task 8（export）、Task 9（e2e）基本不受影响。修订方式：**按 Task 顺序给出每个受影响 Task 的具体变更清单**（添加/删除/替换的精确内容）。

### 2.1 文档头部 — 增加 wf 引用 + Spec 链接

在 `**Spec:**` 行后追加：

```
**Alignment revision (wf-2026-08-19-003):** 本计划已根据 `docs/superpowers/specs/2026-08-19-rbac-kitex-alignment-decisions.md` 的 6 项锁定决策修订。主要修订点：Task 1 proto + SQL schema、Task 2 DDD domain（permission 聚合扩展 + menu 聚合只读化）、Task 4 repos、Task 5 app services（UpdatePermission + GrantPermissionsToRole codes + Menu CRUD 移除）、Task 6 handlers。
```

### 2.2 Global Constraints — 追加 2 条

在现有 Global Constraints 列表末尾追加：

```
- **s-web 对齐约束（wf-2026-08-19-003）：** `permissions` 单一树（type∈{catalog,menu,button,api}），`UNIQUE(code, type)`，`status int` 统一，`GrantPermissionsToRole` 用 `permission_codes []string`，Permission RPC 拥有所有写路径（含 `UpdatePermission`），Menu RPC 仅保留 `ListMenus` + `GetUserMenuTree`。详见 decisions doc §1-§6。
- **`menus` 表已删除：** 原 `menus` 表的语义通过 `SELECT ... FROM permissions WHERE type IN ('catalog','menu')` 视图化。所有 menu 写操作走 Permission RPC。
```

### 2.3 File Structure — 调整聚合目录

将当前：
```
internal/domain/permission/{entity,valueobject,repository}.go
internal/domain/menu/{entity,service,repository}.go
internal/application/permission/{permission_service,dto}.go
internal/application/menu/{menu_service,dto}.go
```

替换为：
```
internal/domain/permission/{entity,valueobject,repository}.go  # owns all writes (tree CRUD + button/api)
internal/domain/menu/{entity,query_service,repository}.go      # read-only tree query aggregate
internal/application/permission/{permission_service,dto}.go    # incl. UpdatePermission
internal/application/menu/{menu_query_service,dto}.go          # read-only (ListMenus, GetUserMenuTree)
```

### 2.4 Task 1 Step 2 (proto) — 修订 `idl/auth.proto`

**删除的 RPC**（在 `service RBACService` 块中）：
```proto
  rpc CreateMenu(CreateMenuReq) returns (MenuResp);
  rpc UpdateMenu(UpdateMenuReq) returns (MenuResp);
  rpc DeleteMenu(DeleteMenuReq) returns (EmptyResp);
```

**新增的 RPC**（在 Permission 块中）：
```proto
  rpc UpdatePermission(UpdatePermissionReq) returns (PermissionResp);
  rpc GetPermission(GetPermissionReq) returns (PermissionResp);
```

**修订 `Permission` message**（替换原定义）：
```proto
message Permission {
  int64 id = 1;
  string code = 2;
  string type = 3;    // catalog | menu | button | api
  string name = 4;
  int64 parent_id = 5;                  // tree parent (catalog/menu only)
  string path = 6;                      // route path (catalog/menu)
  string icon = 7;                      // icon (catalog/menu)
  string route_name = 8;                // frontend route name (catalog/menu)
  string redirect = 9;                  // redirect (catalog)
  bool keep_alive = 10;                 // route cache (menu)
  bool hide_in_menu = 11;               // hide from menu (menu)
  bool is_external = 12;                // external link (catalog/menu)
  string method = 13;                   // HTTP method (api only: GET/POST/PUT/DELETE)
  int32 sort = 14;                      // sort order
  int32 status = 15;                    // 1=enabled, 0=disabled
  string description = 16;
}
```

**新增的 Request messages**（在 Permission 块后）：
```proto
message UpdatePermissionReq {
  int64 id = 1;
  optional string code = 2;
  optional string type = 3;
  optional string name = 4;
  optional int64 parent_id = 5;
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
message GetPermissionReq { int64 id = 1; }
```

**修订 `CreatePermissionReq`**（替换原定义）：
```proto
message CreatePermissionReq {
  string code = 1;
  string type = 2;     // catalog | menu | button | api
  string name = 3;
  int64 parent_id = 4;                  // optional, 0 = root
  string path = 5;
  string icon = 6;
  string route_name = 7;
  string redirect = 8;
  bool keep_alive = 9;
  bool hide_in_menu = 10;
  bool is_external = 11;
  string method = 12;                   // required when type=api
  int32 sort = 13;
  int32 status = 14;                    // 1=enabled, 0=disabled
  string description = 15;
}
```

**修订 `ListPermissionsReq`**（增加过滤字段）：
```proto
message ListPermissionsReq {
  int32 page = 1;
  int32 page_size = 2;
  optional string type = 3;             // filter by type
  optional int64 parent_id = 4;         // filter by parent
  optional int32 status = 5;            // filter by status
}
```

**修订 `User` message**（替换原定义）：
```proto
message User {
  int64 id = 1;
  string username = 2;
  int32 status = 3;                     // 1=enabled, 0=disabled (was string)
  repeated string roles = 4;            // role ids (derived from user_roles)
  string nickname = 5;
  string avatar = 6;
  string email = 7;
  string phone = 8;
}
```

**修订 `CreateUserReq` / `UpdateUserReq`**（增加新字段）：
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
  int64 id = 1;
  optional string password = 2;
  optional int32 status = 3;
  optional string nickname = 4;
  optional string avatar = 5;
  optional string email = 6;
  optional string phone = 7;
}
```

**修订 `Role` message**（替换原定义）：
```proto
message Role {
  int64 id = 1;
  string code = 2;
  string name = 3;
  int32 status = 4;                     // 1=enabled, 0=disabled (new)
  string remark = 5;                    // (new)
  repeated string permissions = 6;      // permission codes (derived from role_permissions)
}
```

**修订 `GrantPermissionsToRoleReq`**（替换原定义）：
```proto
message GrantPermissionsToRoleReq {
  int64 role_id = 1;
  repeated string permission_codes = 2; // was: repeated int64 permission_ids
}
```

**修订 `Menu` message（现为只读视图）**（替换原定义）：
```proto
message Menu {
  int64 id = 1;
  string code = 2;                      // permission code (new)
  string name = 3;
  int64 parent_id = 4;
  string type = 5;                      // catalog | menu (was: dir | menu | button)
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

**删除的 Menu request/response messages**：
```proto
message CreateMenuReq { ... }
message UpdateMenuReq { ... }
message DeleteMenuReq { ... }
```

**修订 `MenuNode`**：
```proto
message MenuNode { Menu menu = 1; repeated MenuNode children = 2; }
```

### 2.5 Task 1 Step 5 (SQL schema) — 修订表结构

**替换 `users` 表**：
```sql
CREATE TABLE users (
    id BIGSERIAL PRIMARY KEY,
    username TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    nickname TEXT,
    avatar TEXT,
    email TEXT,
    phone TEXT,
    status INTEGER NOT NULL DEFAULT 1,  -- 1=enabled, 0=disabled
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

**替换 `roles` 表**：
```sql
CREATE TABLE roles (
    id BIGSERIAL PRIMARY KEY,
    code TEXT NOT NULL UNIQUE,
    name TEXT NOT NULL,
    status INTEGER NOT NULL DEFAULT 1,
    remark TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

**替换 `permissions` 表**（单一树，吸收原 menus）：
```sql
CREATE TABLE permissions (
    id BIGSERIAL PRIMARY KEY,
    code TEXT NOT NULL,
    type TEXT NOT NULL CHECK (type IN ('catalog','menu','button','api')),
    name TEXT NOT NULL,
    parent_id BIGINT REFERENCES permissions(id),  -- no ON DELETE CASCADE; app-layer cascade
    path TEXT,
    icon TEXT,
    route_name TEXT,
    redirect TEXT,
    keep_alive BOOLEAN,
    hide_in_menu BOOLEAN,
    is_external BOOLEAN,
    method TEXT CHECK (method IS NULL OR method IN ('GET','POST','PUT','DELETE')),
    sort INTEGER,
    status INTEGER NOT NULL DEFAULT 1,
    description TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (code, type)
);

CREATE INDEX idx_permissions_parent ON permissions(parent_id);
CREATE INDEX idx_permissions_type ON permissions(type);
```

**删除 `menus` 表**（整段 `CREATE TABLE menus ...` 移除）。

**保留不变**：`user_roles`、`role_permissions`、`casbin_rule`、`audit_log`。

**修订 `internal/db/query/rbac.sql`**：

- **删除**所有 menu CRUD queries：`CreateMenu`、`GetMenuByID`、`UpdateMenu`、`DeleteMenu`、`ListMenusByPermCodes`
- **新增/替换**：

```sql
-- Permission queries (revised for single tree)

-- name: CreatePermission :one
INSERT INTO permissions (code, type, name, parent_id, path, icon, route_name, redirect, keep_alive, hide_in_menu, is_external, method, sort, status, description)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15) RETURNING *;

-- name: GetPermissionByID :one
SELECT * FROM permissions WHERE id = $1;

-- name: GetPermissionByCode :many  -- returns multiple rows (same code, different types)
SELECT * FROM permissions WHERE code = $1 ORDER BY type;

-- name: GetPermissionByCodeAndType :one
SELECT * FROM permissions WHERE code = $1 AND type = $2;

-- name: ListPermissions :many
SELECT * FROM permissions ORDER BY sort NULLS LAST, id LIMIT $1 OFFSET $2;

-- name: ListPermissionsFiltered :many
SELECT * FROM permissions
WHERE ($1 = '' OR type = $1)
  AND ($2 < 0 OR parent_id = $2)
  AND ($3 < 0 OR status = $3)
ORDER BY sort NULLS LAST, id
LIMIT $4 OFFSET $5;

-- name: ListPermissionsByCodes :many
SELECT * FROM permissions WHERE code = ANY($1) ORDER BY code, type;

-- name: UpdatePermission :one
UPDATE permissions SET
    code = COALESCE($2, code),
    type = COALESCE($3, type),
    name = COALESCE($4, name),
    parent_id = COALESCE($5, parent_id),
    path = COALESCE($6, path),
    icon = COALESCE($7, icon),
    route_name = COALESCE($8, route_name),
    redirect = COALESCE($9, redirect),
    keep_alive = COALESCE($10, keep_alive),
    hide_in_menu = COALESCE($11, hide_in_menu),
    is_external = COALESCE($12, is_external),
    method = COALESCE($13, method),
    sort = COALESCE($14, sort),
    status = COALESCE($15, status),
    description = COALESCE($16, description),
    updated_at = now()
WHERE id = $1 RETURNING *;

-- name: DeletePermission :exec
DELETE FROM permissions WHERE id = $1;

-- name: ListChildPermissionIDs :many  -- for app-layer cascade delete
SELECT id FROM permissions WHERE parent_id = $1;

-- Menu read-only queries (view over permissions WHERE type IN ('catalog','menu'))

-- name: ListMenusAsTree :many
SELECT id, code, name, parent_id, type, path, icon, route_name, redirect, keep_alive, hide_in_menu, is_external, sort
FROM permissions
WHERE type IN ('catalog', 'menu') AND status = 1
ORDER BY sort NULLS LAST, id;

-- name: ListMenusByParentID :many
SELECT id, code, name, parent_id, type, path, icon, route_name, redirect, keep_alive, hide_in_menu, is_external, sort
FROM permissions
WHERE type IN ('catalog', 'menu') AND status = 1 AND parent_id = $1
ORDER BY sort NULLS LAST, id;

-- User queries (revised for new fields)

-- name: CreateUser :one
INSERT INTO users (username, password_hash, nickname, avatar, email, phone, status)
VALUES ($1, $2, $3, $4, $5, $6, 1) RETURNING *;

-- name: UpdateUser :one
UPDATE users SET
    username = COALESCE($2, username),
    nickname = COALESCE($3, nickname),
    avatar = COALESCE($4, avatar),
    email = COALESCE($5, email),
    phone = COALESCE($6, phone),
    status = COALESCE($7, status),
    updated_at = now()
WHERE id = $1 RETURNING *;

-- name: UpdateUserPassword :one  -- unchanged

-- Role queries (revised for new fields)

-- name: CreateRole :one
INSERT INTO roles (code, name, status, remark) VALUES ($1, $2, 1, $3) RETURNING *;

-- name: UpdateRole :one
UPDATE roles SET
    name = COALESCE($2, name),
    status = COALESCE($3, status),
    remark = COALESCE($4, remark),
    updated_at = now()
WHERE id = $1 RETURNING *;

-- name: ListPermissionCodesByRoleID :many  -- replaces ListPermissionIDsByRoleID
SELECT p.code FROM permissions p
JOIN role_permissions rp ON rp.permission_id = p.id
WHERE rp.role_id = $1
ORDER BY p.code;

-- name: ListPermissionIDsByCodes :many  -- for GrantPermissionsToRole resolution
SELECT id, code FROM permissions WHERE code = ANY($1);
```

- **保留不变**：`GetUserByID`、`GetUserByUsername`、`ListUsers`、`UpdateUserPassword`、`DeleteUser`、`AddUserRole`、`RemoveUserRole`、`ClearUserRoles`、`ListRolesByUserID`、`ListRoleIDsByUserID`、`GetRoleByID`、`GetRoleByCode`、`ListRoles`、`DeleteRole`、`AddRolePermission`、`RemoveRolePermission`、`ClearRolePermissions`、`ListPermissionsByRoleIDs`（仍按 id 关联）、Casbin queries、audit query。

### 2.6 Task 2 (DDD domain layer) — 修订聚合职责

**修订 `permission` 聚合**（扩展）：

```
permission.Permission {
    ID int64
    Code string
    Type string       // catalog | menu | button | api
    Name string
    ParentID int64    // 0 = root
    Path, Icon, RouteName, Redirect string   // nullable (tree metadata)
    KeepAlive, HideInMenu, IsExternal *bool  // nullable
    Method string                          // nullable (api only)
    Sort int32
    Status int                             // 1=enabled, 0=disabled
    Description string
}

permission.TypeCatalog / TypeMenu / TypeButton / TypeAPI 常量
permission.New(code, typ, name string, parentID int64, ...) (*Permission, error)
  - 验证 type ∈ {catalog, menu, button, api}
  - 当 type = api 时，method 必填且 ∈ {GET, POST, PUT, DELETE}
  - 当 type ∈ {catalog, menu} 时，tree 字段可填
  - 当 type = button 时，tree 字段应空
permission.StatusEnabled / StatusDisabled 常量
permission.Repository 扩展:
  - GetByCodeAndType(ctx, code, type) (*Permission, error)
  - ListChildren(ctx, parentID) ([]*Permission, error)  -- for cascade delete
  - ListByCodes(ctx, codes []string) ([]*Permission, error)  -- for GrantPermissionsToRole resolution
```

**修订 `menu` 聚合**（降级为只读查询）：

```
// internal/domain/menu/query_service.go (原 service.go → query_service.go)
menu.QueryService {
    ListMenusAsTree(ctx) ([]*Node, error)           -- 从 permissions 视图读取 + 组装
    GetUserMenuTree(ctx, uid, permCodes []string) ([]*Node, error)
}
menu.Node { Menu, Children []*Node }  -- Menu 字段集简化为只读视图字段
menu.BuildTree(items []*Menu) []*Node  -- 保留，纯函数
```

删除原 `menu.Repository` 的写方法（`Save`/`Delete`）；menu 不再直接写 `permissions` 表，写路径由 `permission.Repository` 承担。

### 2.7 Task 4 (repositories) — 修订以匹配新 queries

- `permission/repo.go` 增加 `GetByCodeAndType`、`ListChildren`、`ListByCodes`、`Update` 方法（对应新 SQL queries）
- `menu/repo.go` 简化为只读：仅 `ListMenusAsTree`、`ListMenusByParentID`
- `user/repo.go` 扩展 `Create`/`Update` 以支持新字段（nickname/avatar/email/phone/status int）
- `role/repo.go` 扩展 `Create`/`Update` 以支持 status int + remark；新增 `ListPermissionCodesByRoleID` 方法

### 2.8 Task 5 (application services) — 修订核心逻辑

**`permission.Service`**（扩展）：
```go
func (s *Service) Create(ctx, cmd CreatePermissionCmd) (*permission.Permission, error)
func (s *Service) Update(ctx, cmd UpdatePermissionCmd) (*permission.Permission, error)  // NEW
func (s *Service) Delete(ctx, id int64) error  -- 应用层级联删除子节点 (ListChildren → recurse)
func (s *Service) Get(ctx, id int64) (*permission.Permission, error)  // NEW
func (s *Service) List(ctx, filter ListFilter) ([]*permission.Permission, int, error)
```

**`menu.Service` → `menu.QueryService`**（降级）：
```go
type QueryService struct { permRepo permission.Repository; ... }
func (qs *QueryService) ListMenus(ctx) ([]*menu.Node, error)
func (qs *QueryService) GetUserMenuTree(ctx, uid int64) ([]*menu.Node, error)
// 移除 Create/Update/Delete
```

**`role.Service.GrantPermissions`**（载荷变更）：
```go
// 原签名
func (s *Service) GrantPermissions(ctx, roleID int64, permissionIDs []int64) error
// 新签名
func (s *Service) GrantPermissions(ctx, roleID int64, permissionCodes []string) error
```

实现变更：
- 接收 `permissionCodes` → 调用 `permission.Repository.ListByCodes(codes)` 解析为 ids
- 写 `role_permissions`（按 id）+ `enforcer.AddPolicy(role_code, code, method)`（按 code）
- 如 codes 中有未知 code → 返回 domain error

**`user.Service`**（扩展）：`Create`/`Update` 接受 nickname/avatar/email/phone 字段；`status` 为 int（`permission.StatusEnabled`/`StatusDisabled` 常量复用或独立 `user.StatusEnabled`/`StatusDisabled`）。

**`role.Service`**（扩展）：`Create`/`Update` 接受 remark 字段；status 为 int。

### 2.9 Task 6 (handlers) — 修订以匹配新 proto

- `rbacservice/handler.go` 增加 `UpdatePermission`、`GetPermission` handler 方法
- 移除 `CreateMenu`、`UpdateMenu`、`DeleteMenu` handler 方法
- `GrantPermissionsToRole` handler 从 `req.PermissionIds` 改为 `req.PermissionCodes`
- `RBACServiceImpl` 构造器：移除 `menu *menu.Service`（写服务）；改为 `menuQuery *menu.QueryService`

### 2.10 Task 7/8/9 — 无实质性修订

- Task 7 (full validation): 命令不变，但 proto/SQL 修订后 regen 内容不同
- Task 8 (export + assemble): 修订 `README.md` 的 seams 段说明新 shape（单一 Permission 树）
- Task 9 (e2e): 测试断言更新以反映新 proto/SQL

### 验证

- [ ] **Step 1: 内部一致性检查** —— 计划文档内：
  - proto message 字段 → SQL 列 → domain entity 字段 → repo 方法签名 → app service 方法签名 → handler 调用 → 全部对齐
  - `UNIQUE(code, type)` 在 SQL 有、proto 有、domain 验证有
  - `GrantPermissionsToRole` 载荷 codes 在 proto 有、app service 签名有、handler 调用有
- [ ] **Step 2: 与 Task 1 修订后的设计文档对齐** —— 计划文档的 proto/SQL/DDD/RPC 表面与设计文档 Locked decisions 表、Data model 节、RPC surface 节完全一致
- [ ] **Step 3: 与 decisions doc §1-§6 对齐** —— 6 项决策在计划文档中都有对应体现
- [ ] **Step 4: Commit**

```bash
git add docs/superpowers/plans/2026-08-18-rbac-kitex-template.md
git commit -m "docs(plans): revise rbac-kitex template plan per wf-2026-08-19-003 decisions

- Proto: add UpdatePermission + GetPermission; remove Menu Create/Update/Delete;
  expand Permission message for single tree; add User/Role fields; status int
- SQL: single permissions table (UNIQUE(code,type)); drop menus table;
  revise user/role schemas; new queries for tree view and code-based lookup
- DDD: permission aggregate owns writes; menu degrades to read-only QueryService
- App services: UpdatePermission, GrantPermissionsToRole uses codes,
  menu.Service → menu.QueryService
- Handlers: follow new proto surface
- Global constraints: add s-web alignment constraints

Refs: #75, wf-2026-08-19-003"
```

---

## Task 3: 交叉一致性校验

**Files:**
- Read: `docs/superpowers/specs/2026-08-18-rbac-kitex-design.md` (Task 1 输出)
- Read: `docs/superpowers/plans/2026-08-18-rbac-kitex-template.md` (Task 2 输出)
- Read: `docs/superpowers/specs/2026-08-19-rbac-kitex-alignment-decisions.md` (spec)

**Interfaces:**
- Consumes: Task 1 + Task 2 修订后的两份文档。
- Produces: 校验报告（通过/不通过 + 修订建议）；如不通过则回到 Task 1/2 修订。

### 校验清单

对以下 7 个维度逐项比对三份文档，确保完全一致：

| # | 维度 | 设计文档章节 | 计划文档章节 | decisions doc 章节 |
|---|---|---|---|---|
| 1 | 表结构（permissions 单一树、字段集、UNIQUE(code,type)） | Data model | Task 1 Step 5 SQL | §1, §2 |
| 2 | users/roles 字段补齐 + status int | Data model | Task 1 Step 5 SQL + Task 2 domain | §4 |
| 3 | Casbin 零影响 | Casbin | Task 3 infrastructure | §5 |
| 4 | RPC 表面（UpdatePermission、Menu CRUD 移除、codes 载荷） | RPC surface | Task 1 Step 2 proto + Task 6 handlers | §6 |
| 5 | DDD 聚合职责（permission 写、menu 只读） | DDD structure | Task 2 domain + Task 5 services | §1, §6 |
| 6 | 字段清单对齐（Permission 18 列、User 8 列、Role 5 列） | Data model | Task 1 proto/SQL + Task 2 domain | §1, §4 |
| 7 | Scope boundary（s-web/BFF 不在范围） | Open points | Global Constraints | §7 |

### 步骤

- [ ] **Step 1: 逐项比对** —— 打开三份文档，按上表 7 个维度交叉核对。每个维度记录「一致 / 不一致 + 具体位置」。
- [ ] **Step 2: 标记不一致** —— 如发现不一致，按「最小修订」原则定位应该修订哪份文档（通常以 decisions doc 为真相源，设计文档次之，计划文档再次之）。
- [ ] **Step 3: 应用修订**（如有）—— 回到 Task 1 或 Task 2 文档应用最小修订，再次 commit。
- [ ] **Step 4: 最终确认** —— 7 个维度全部「一致」→ 通过。
- [ ] **Step 5: Commit final state**（如有 Step 3 修订）

```bash
git add docs/superpowers/specs/2026-08-18-rbac-kitex-design.md docs/superpowers/plans/2026-08-18-rbac-kitex-template.md
git commit -m "docs(specs/plans): cross-doc consistency fix per wf-2026-08-19-003 verification

[list specific fixes if any]"
```

### 完成标准

- 三份文档在 7 个维度上完全一致
- decisions doc 中 6 项锁定决策在设计文档与计划文档中都有对应体现
- Issue #75 的 AC#4 已完成（设计文档 + 计划文档已更新并记录对齐决策）

---

## Self-Review

**Spec coverage:**
- 6 项决策（decisions doc §1-§6）→ Task 1 全部章节 + Task 2 proto/SQL/DDD/service 修订
- Issue #75 AC#1-#3 → 已由 decisions doc 锁定（不在本计划执行范围）
- Issue #75 AC#4 → Task 1 + Task 2 + Task 3 全计划
- Casbin 零影响（decisions doc §5）→ Task 1 Casbin 节标注 + Task 2 Global Constraints 声明
- Scope boundary（decisions doc §7）→ Task 1 Open points + Task 2 Global Constraints

**Placeholder scan:**
- 无 TBD/TODO。所有修订都有具体的目标态（proto message、SQL schema、domain 接口、service 签名）。
- Task 7/8/9 标注为「无实质性修订」是因为它们的内容（validation/export/e2e 流程）不依赖具体 proto/SQL shape，修订影响已在 Task 1/2 中消化。

**Type consistency:**
- proto message 字段 → SQL 列 → domain entity 字段 → repo 方法签名 → app service 方法签名 → handler 调用，全链路一致：
  - `Permission` 18 字段 → `permissions` 18 列 → `permission.Permission` 实体 → `permission.Repository` 全套方法 → `permission.Service` CRUD+Update+Get → handler
  - `GrantPermissionsToRoleReq.permission_codes []string` → `role.Service.GrantPermissions(ctx, roleID, permissionCodes []string)` → handler 调用 `req.PermissionCodes`
  - `User` 8 字段 → `users` 8 列 → `user.User` 实体 → `user.Service.Create/Update` 接受全字段 → handler
  - `Role` 5 字段 → `roles` 5 列 → `role.Role` 实体 → `role.Service.Create/Update` 接受 status/remark → handler
- menu 聚合：domain `menu.QueryService`（只读） → repo `ListMenusAsTree/ListMenusByParentID`（只读 SQL） → app `menu.QueryService` → handler 仅保留 `ListMenus`/`GetUserMenuTree`

**Plan doc location:** `docs/superpowers/plans/2026-08-19-rbac-kitex-alignment-revision.md`

**Spec path:** `docs/superpowers/specs/2026-08-19-rbac-kitex-alignment-decisions.md`
