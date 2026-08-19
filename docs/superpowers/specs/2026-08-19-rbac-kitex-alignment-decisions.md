# Design Decisions: `rbac-kitex` 模板对齐 s-web 前端契约

- **Workflow:** `wf-2026-08-19-003` (gf-workflow Phase 1 澄清产物)
- **Issue:** [#75](https://github.com/byx-darwin/ncgo/issues/75) — feat(rbac-kitex): 对齐 s-web 前端契约的权限模型与 RPC 面
- **日期:** 2026-08-19
- **性质:** 设计决策记录（decisions record）。前置评估产物：`2026-08-19-rbac-kitex-s-web-alignment.md`（6 项缺口 + 5 项建议）。本文锁定 6 项决策，作为 Phase 2 计划的输入。
- **范围:** ncgo `rbac-kitex` 模板设计（数据模型 + proto + 字段）。s-web 前端与 admin-bff-hertz 不在本 issue 范围（见 §7）。
- **状态:** 决策已锁定，等待用户审阅。Phase 3 将根据本文修订 `2026-08-18-rbac-kitex-design.md`。

## 1. 权限数据模型（主决策）

**决策：单一 Permission 树（s-web 风格）。**

`menus` 表被吸收为 `permissions` 的过滤视图（`WHERE type IN ('catalog','menu')`）。Permission 聚合拥有完整的菜单+按钮+接口树。

### 1.1 表结构（`permissions`，替代原 permissions + menus 两表）

| 列 | 类型 | 约束 | 说明 |
|---|---|---|---|
| `id` | `bigint` | PK | |
| `code` | `varchar` | 与 `type` 联合唯一 | 权限码，如 `user:create` |
| `type` | `varchar` | 与 `code` 联合唯一 | `catalog` \| `menu` \| `button` \| `api` |
| `name` | `varchar` | NOT NULL | 显示名 |
| `parent_id` | `bigint` | NULL, FK → permissions.id | 树父节点（catalog/menu 有，button/api 通常 NULL） |
| `path` | `varchar` | NULL | 路由路径（catalog/menu） |
| `icon` | `varchar` | NULL | 图标（catalog/menu） |
| `route_name` | `varchar` | NULL | 前端路由名（catalog/menu） |
| `redirect` | `varchar` | NULL | 重定向（catalog） |
| `keep_alive` | `bool` | NULL | 路由缓存（menu） |
| `hide_in_menu` | `bool` | NULL | 菜单隐藏（menu） |
| `is_external` | `bool` | NULL | 外链（catalog/menu） |
| `method` | `varchar` | NULL | HTTP 方法（api 型：`GET`/`POST`/`PUT`/`DELETE`） |
| `sort` | `int` | NULL | 排序 |
| `status` | `int` | NOT NULL, default 1 | 1=enabled, 0=disabled（见 §4） |
| `description` | `text` | NULL | 描述 |
| `created_at` | `timestamptz` | NOT NULL | |
| `updated_at` | `timestamptz` | NOT NULL | |

**唯一约束：** `UNIQUE(code, type)` —— 同一 code 可在 button 型与 api 型各出现一次（s-web 事实：`user:create` 同时驱动按钮渲染与接口鉴权）。

### 1.2 菜单视图

```sql
-- 菜单树查询（ListMenus 实现）
SELECT id, code, name, parent_id, path, icon, route_name, redirect, keep_alive, hide_in_menu, is_external, sort
FROM permissions
WHERE type IN ('catalog', 'menu') AND status = 1
ORDER BY sort;
```

### 1.3 决策依据

- s-web 是既有事实标准（前端已落地，契约稳定）；
- 单一树与 s-web `API.Permission` 1:1 对齐，BFF 零映射；
- 消除原两表设计的「一次逻辑权限、两次物理写入」问题；
- Casbin 只消费 `code`，对行级结构变化无感（见 §5）。

### 1.4 替代方案（否决）

**保留 permissions + menus 分离两表 + BFF 映射**：BFF 长期承担映射复杂度；button/api 同 code 冲突无法优雅解决；s-web 管理页需额外适配。否决理由：把一次性模板修订成本转嫁到每个部署实例的 BFF。

## 2. 权限码唯一性

**决策：`UNIQUE(code, type)` 联合唯一。**

- 同一 `code` 在 `button` 型与 `api` 型各允许一条记录（如 `user:create` 有两行：一行驱动前端按钮，一行描述 API 端点 `POST /api/users`）；
- `catalog` / `menu` 型 `code` 独立（如 `system:user` 只出现一次，type=menu）；
- Casbin 策略只绑定 `code`，与行级 `type` 解耦（见 §5）。

### 替代方案（否决）

- **单行 + method 数组**：偏离 s-web 数据形态，BFF 反向映射繁琐；
- **单行 + 可选 method**：无法干净表达「纯按钮无接口」与「纯接口无按钮」。

## 3. 授权载荷：ids → codes

**决策：`GrantPermissionsToRole` 载荷改为 `permission_codes []string`。**

```protobuf
rpc GrantPermissionsToRole(GrantPermissionsToRoleRequest) returns (GrantPermissionsToRoleResponse);

message GrantPermissionsToRoleRequest {
  int64 role_id = 1;
  repeated string permission_codes = 2;  // 替代原 permission_ids
}
```

### 3.1 端到端一致性

- 前端发送 `permissionCodes`（s-web `POST /api/roles/:id/permissions { permissionCodes }`）；
- BFF 直传（无需 code→id 翻译）；
- RPC 接收 `permission_codes`；
- Casbin 策略以 `code` 为 `obj`；
- 单一标识符贯穿全链。

### 3.2 关系表

`role_permissions` 关联表保留（按 `permission_id` 关联），用于：
- 关系完整性约束；
- 派生 `GET /api/roles/:id` 响应中的 `permissions: string[]`（通过 join 取 code）。

RPC 表面不再暴露 `permission_id`。

## 4. status 语义 + 字段补齐

**决策：`status int`（1=enabled, 0=disabled）统一用于 User/Role/Permission。**

### 4.1 字段补齐清单

| 实体 | 新增字段 | 说明 |
|---|---|---|
| **Role** | `status int NOT NULL DEFAULT 1`, `remark text NULL` | `permissions []string` 响应字段由 `role_permissions` join 派生，不落 role 行 |
| **User** | `nickname text NULL`, `avatar text NULL`, `email text NULL`, `phone text NULL`；`status` 由原 enum (`active`/`disabled`) 改为 `int` | `roles []string` 响应字段由 `user_roles` join 派生 |
| **Permission** | 见 §1.1 全表 | 单一树承载原 permissions + menus 全字段 |

### 4.2 Go 常量（避免魔法数字）

```go
const (
    StatusEnabled  = 1
    StatusDisabled = 0
)
```

### 4.3 决策依据

s-web API 发送/接收 `status: number`。统一为 int 消除 BFF enum↔int 翻译；Go 常量保持代码可读性。

## 5. Casbin 影响评估

**结论：零影响。**

- Casbin 模型 `r = sub(role_code), obj(permission_code), act(HTTP method)` 不变；
- `casbin_rule` 表结构不变；
- `p` 策略仍为 `(role_code, permission_code, http_method)` 三元组；
- `g` 仍为 `(user_id, role_code)`；
- `code+type` 联合唯一不影响 Casbin —— Casbin 只消费 `code`，不关心同一 code 有几行。

`casbin_adapter` 加载策略时从 `permissions` 表取 `code`（而非原 permissions + menus 两路），查询略简化。

## 6. RPC 表面（proto 修订）

**决策：Permission RPC 拥有所有写路径；Menu RPC 仅保留只读树查询。**

### 6.1 RBACService 修订清单

| RPC | 状态 | 说明 |
|---|---|---|
| `CreatePermission` | 修订 | 支持 type∈{catalog,menu,button,api}；按钮/api 行 method 必填 |
| **`UpdatePermission`** | **新增** | Issue #75 AC #2；按 id 更新任意 type 的 permission |
| `DeletePermission` | 保持 | 级联删除子节点（树语义） |
| `GetPermission` | 新增 | 按 id 单条查询 |
| `ListPermissions` | 修订 | 增加 type/parent_id/status 过滤 |
| `CreateMenu` / `UpdateMenu` / `DeleteMenu` | **移除** | 写路径统一走 Permission RPC |
| `ListMenus` | 修订 | 仅返回 type∈{catalog,menu} 的树形结构 |
| `GetUserMenuTree(uid)` | 保持 | 当前用户可见菜单树（按权限过滤） |
| `GetUserPermCodes(uid)` | 保持 | 当前用户权限码列表（驱动前端按钮渲染） |
| `GrantPermissionsToRole` | 修订 | 载荷改为 `permission_codes []string`（§3） |
| `AssignRolesToUser` | 保持 | 仍用 `role_ids []int64`（s-web 用户授权用 roleIds） |
| `Enforce` / `Login` / `Refresh` / `Logout` / `ValidateToken` | 保持 | 无变更 |

### 6.2 Menu 聚合的角色

Menu 聚合从「CRUD 聚合」退化为「只读查询聚合」，职责：
- `ListMenus(filter)`：按 type 过滤 + 树形组装；
- `GetUserMenuTree(uid)`：按用户权限过滤 + 树形组装。

这符合 CQRS-lite：写路径单一（Permission），读路径按查询模型分片（Menu）。

## 7. 范围边界

| 层 | 责任 | 本 workflow 范围 |
|---|---|---|
| **s-web 前端** | Ant Design Pro UI，事实标准契约 | ❌ 永不在范围（s-web 是参考基准，不是修改目标） |
| **rbac-kitex 模板** | ncgo 导出的 Kitex RPC 模板 | ✅ 在范围（数据模型 + proto + 字段 + status + 文档） |
| **admin-bff-hertz** | Hertz BFF，桥接 s-web ↔ rbac-kitex | ❌ 本 issue 不在范围（模板契约稳定后作为下游 issue 跟进） |

### 7.1 Issue #75 AC 映射

| AC | 本文对应 | Phase 3 产出 |
|---|---|---|
| 决定权限模型：单一树 vs 分离两表 | §1 | 更新 `2026-08-18-rbac-kitex-design.md` 数据模型章节 |
| 新增 UpdatePermission RPC，授权载荷 code 语义 | §3, §6 | proto 修订；更新设计文档 RPC 章节 |
| Role/User/Permission 补齐字段，统一 status 语义 | §4 | 数据模型 DDL 修订；更新设计文档字段清单 |
| 更新设计文档 + 计划文档，记录对齐决策 | 全文 | 修订 `2026-08-18-rbac-kitex-design.md` + `docs/superpowers/plans/2026-08-18-rbac-kitex-template.md` |

## 8. 已知取舍与后续

- **后续 issue（admin-bff-hertz 适配）**：本 workflow 锁定模板契约后，admin-bff-hertz 需要相应调整（尤其是菜单树/权限列表的响应形态、授权请求字段）。作为独立 issue 跟进。
- **`role_permissions` 关系表**：保留 `permission_id` 关联，但 RPC 表面只暴露 code。后续可评估是否完全改为 code-based 关联（当前选择：保持 id 关联以维持 FK 完整性）。
- **数据迁移**：rbac-kitex 模板尚未实现（无现存部署数据），无迁移成本。
- **`method` 列的 api 行必填约束**：通过 DB check 约束或应用层校验（`IF type = 'api' THEN method IS NOT NULL`）。具体实现由 Phase 2 计划决定。

## 9. 引用

- 前置评估：`docs/superpowers/specs/2026-08-19-rbac-kitex-s-web-alignment.md`
- 当前模板设计（待 Phase 3 修订）：`docs/superpowers/specs/2026-08-18-rbac-kitex-design.md`
- 当前模板计划（待 Phase 3 修订）：`docs/superpowers/plans/2026-08-18-rbac-kitex-template.md`
- micro-admin program：`docs/superpowers/specs/2026-08-18-micro-admin-ddd-program-design.md`
