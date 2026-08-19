# Design / Assessment: rbac-kitex 模板 vs s-web 前端契约对齐性评估

- **日期:** 2026-08-19
- **工作流:** wf-2026-08-19-001（gf-workflow Phase 1 澄清产物）
- **评估对象:**
  - A. ncgo `rbac-kitex` 模板（`docs/superpowers/specs/2026-08-18-rbac-kitex-design.md` + `docs/superpowers/plans/2026-08-18-rbac-kitex-template.md`，**尚未实现**，`ncgo-templates/rbac-kitex/` 不存在）
  - B. 前端项目 `/Users/xs/Documents/workspce/xiaosuan/iproost/proxy/s-web`（Ant Design Pro，Umi Max v4 / antd v6）
- **性质:** 评估/对齐性分析（spike）。本文件是评估产物，非实现规格。

## 结论

**理念对齐，结构不对齐。** s-web 与 rbac-kitex 在「统一权限码 + admin 通配角色 + JWT 认证 + RBAC CRUD 能力」上设计意图一致；但在**权限/菜单数据模型、授权载荷、RPC 能力缺口、字段覆盖**四个层面存在实质差距。若以 s-web 前端契约为事实标准，rbac-kitex 需要先做设计调整（不能仅靠 BFF 兜底）。

## 一、s-web 前端契约（事实标准）

### 1.1 认证流
- `POST /api/login/account`（body `{username,password,type}`）→ `{status, type, currentAuthority}`
- `GET /api/currentUser` → `{...defaultUser, name, userid, email, roles[], roleCodes[], permissions[], access}`
- `POST /api/login/outLogin`；401 → 跳 `/user/login?redirect=...`（`requestErrorConfig` + `app.tsx getInitialState`）

### 1.2 权限模型（单一扁平 Permission 树）
`API.Permission`（`src/typings.d.ts`）—— **一张表承载菜单+按钮+接口**，`parentId` 建树：

```
{ id, code, name,
  type: 'catalog' | 'menu' | 'button' | 'api',
  parentId?, path?, icon?, routeName?,   // catalog/menu 路由字段
  redirect?, keepAlive?, hideInMenu?, isExternal?,
  method?: 'GET'|'POST'|'PUT'|'DELETE',   // api 型
  sort?, status?, description?, createdAt? }
```

- **同一权限码可出现在 button 型与 api 型两条记录**（如 `user:create` 同时有 p5 button + p16 api，method/path 挂在 api 型上）。
- 权限码命名：`system:view`(catalog)、`system:user`/`system:role`/`system:permission`(menu)、`user:create`/`user:update`/`user:delete`/`user:assignRole`/`role:create`/`role:update`/`role:delete`/`role:assignPerm`/`permission:create`/`permission:update`/`permission:delete`(button+api)。

### 1.3 角色 / 用户
- `API.Role`：`{id, name, code, permissions: string[](codes), status:number, remark?, createdAt?}`
- `API.RbacUser`：`{id, username, nickname?, avatar?, email?, phone?, status:number, roles: string[](roleIds), createdAt?}`
- `API.MenuNode`（`GET /api/menus` 返回）：`{id, code, name, routeName?, path?, icon?, redirect?, hideInMenu?, isExternal?, keepAlive?, children?}` —— **有 `code`，无 `component`**。

### 1.4 REST API 面
| 资源 | 端点 |
|---|---|
| Users | `GET/POST /api/users` · `PUT/DELETE /api/users/:id` · `POST /api/users/:id/roles` `{roleIds}` |
| Roles | `GET/POST /api/roles` · `PUT/DELETE /api/roles/:id` · `POST /api/roles/:id/permissions` `{permissionCodes}` |
| Permissions | `GET/POST /api/permissions` · `PUT/DELETE /api/permissions/:id` |
| Menus | `GET /api/menus` → 当前用户可见菜单树 |
| Auth | `login/account` · `currentUser` · `outLogin` |

授权载荷：**用户用 `roleIds`（id）**，**角色用 `permissionCodes`（code）**。

## 二、rbac-kitex 模板设计（评估基准 = 设计+计划文档）

### 2.1 RPC 面（proto，见 plan Task 1）
- **AuthService:** `Login` `Refresh` `Logout` `ValidateToken`
- **RBACService:** User `Create/Update/Delete/Get/List`；Role `Create/Update/Delete/List`；Permission `Create/Delete/List`（**无 Update**）；Menu `Create/Update/Delete/List`；`AssignRolesToUser` `GrantPermissionsToRole` `Enforce` `GetUserMenuTree` `GetUserPermCodes`

### 2.2 数据模型（分离两表）
- `permissions(id, code UNIQUE, name, kind[menu|button|api], method)` —— 无 parentId/path/icon/routeName/status
- `menus(id, parent_id, type[dir|menu|button], name, path, component, perm_code→permissions.code, sort_order)`
- `users(id, username, password_hash, status[active|disabled])` —— 无 nickname/avatar/email/phone
- `roles(id, code, name)` —— 无 permissions/status/remark
- `user_roles` / `role_permissions` / `casbin_rule` / `audit_log`

### 2.3 授权载荷
- `AssignRolesToUser(user_id, role_ids int64[])`
- `GrantPermissionsToRole(role_id, permission_ids int64[])` —— **用 permission id，不是 code**

### 2.4 Casbin
`r = sub(role_code), obj(permission_code), act(HTTP method)`；`g = user_id, role_code`。`casbin_rule` 为唯一鉴权源。

## 三、对齐矩阵

### ✅ 对齐（设计意图一致）
| 维度 | s-web | rbac-kitex | 说明 |
|---|---|---|---|
| 统一权限码 | `permissions.includes(code)`、`withPerm(code)` | `permissions.code`==casbin obj==`menus.perm_code` | 同源理念 |
| admin 通配 | `roleCodes.includes('admin')` / `'*'` | `"admin"` 角色返回全部权限 | 一致 |
| JWT 认证 | 登录→token→401 跳转 | `Login/ValidateToken` | 一致（需 BFF 转 REST） |
| 用户↔角色 / 角色↔权限分配 | 有 | `AssignRolesToUser`/`GrantPermissionsToRole` | 方向一致 |
| 菜单树按权限过滤 | `GET /api/menus` | `GetUserMenuTree` | 概念一致 |

### ❌ 不对齐（核心缺口）
| # | 缺口 | s-web | rbac-kitex | 影响 |
|---|---|---|---|---|
| 1 | **权限/菜单数据模型** | 单一 `Permission` 树，菜单字段挂在权限上 | `permissions` + `menus` 分离两表，`catalog`→`dir` | 前端管理页/菜单树需大量映射或改模板模型 |
| 2 | **`permissions.code` 唯一性** | 同一 code 可同时是 button 与 api | `code UNIQUE`，一条记录一个 kind | 前端「一个码驱动按钮+接口」设计在模板中冲突 |
| 3 | **授权载荷 code vs id** | 角色授权传 `permissionCodes` | 传 `permission_ids` | BFF 需 code↔id 转换，或改模板 |
| 4 | **缺 `UpdatePermission`** | `PUT /api/permissions/:id` + `permission:update` | 只有 Create/Delete/List | 前端权限管理无法编辑 |
| 5 | **字段缺口** | Role 带 `permissions/status/remark`；User 带 `nickname/avatar/email/phone`、status 为 number；Permission 带 `parentId/path/icon/routeName/sort/status`；MenuNode 带 `code` 无 `component` | Role/User/Permission 为精简字段，`status` 语义不同 | 前端页面字段无法直接映射 |
| 6 | **RPC vs REST** | HTTP REST `/api/*` | Kitex RPC | 必须有 BFF（admin-bff-hertz）转换，属 program 既定，但 BFF 需按缺口 1-5 兜底 |

## 四、建议

1. **权限模型取舍（最大决策）**：rbac-kitex 改为 s-web 式**单一 Permission 树**（`permissions` 自带 parentId/path/icon/routeName/sort/status，`method` 覆盖 api 型；`menus` 作为视图查询），**还是**保留分离两表、由 BFF 承担映射（前端管理页需适配，改动小但耦合深）？**推荐前者** —— s-web 前端是既有事实标准，改模板更合算。
2. **补 RPC**：新增 `UpdatePermission`；`GrantPermissionsToRole` 载荷改为 **permission codes**（或新增 code 语义参数）。
3. **字段补齐**：Role 补 `permissions/status/remark`，User 补 `nickname/avatar/email/phone`，统一 `status` 语义（number vs enum）为前端可用形态。
4. **同一 code 多 kind 处理**：明确 button 型与 api 型是否复用同一 code（推荐：复用，api 型 `method` 记录端点），调整 schema 唯一性约束（code+kind 联合唯一 或 保留单条记录带多个 method）。
5. **认证流对齐**：BFF 需将 `ValidateToken`+`GetUserPermCodes` 组装为 s-web `currentUser` 的 `{roles, roleCodes, permissions, access}` 形态。

## 五、下一步（workflow 视角）

本评估为 Phase 1 澄清产物。后续：
- 创建 Issue 记录「rbac-kitex 与 s-web 对齐」评估结论与修复建议。
- Phase 2 计划：以 s-web 契约为基准，修订 rbac-kitex 设计（数据模型 + proto + 字段）→ Phase 3 执行 → Phase 4 交付。

## 六、决策锁定（2026-08-19，Issue #75）

1. 权限模型：**单一 Permission 树**（permissions 自带树字段，menus 合并，kind ∈ catalog|menu|button|api）。
2. ID/status：**string ID（uuid v4）+ int32 status 0/1**，完全对齐 s-web 契约形态。
3. 授权载荷：**GrantPermissionsToRole by permission_codes**；**AssignRolesToUser by role_ids**。
4. RPC：新增 **UpdatePermission**；ValidateToken 返回 uid string + role_codes。
5. MenuNode 无 component，含 code；`permissions.code` 允许 button/api 复用（UNIQUE(code,kind)）。
