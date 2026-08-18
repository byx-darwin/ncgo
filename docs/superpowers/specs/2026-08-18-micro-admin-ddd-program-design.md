# Design: `micro-admin` 运营中台模板 —— 跨仓程序设计

- **发起需求:** 做一个内部运营中台的微服务模板:DDD 架构 + RBAC(角色/权限)+ 限流,用 ncgo 开发真实项目后 `ncgo export` 转成模板。
- **分类:** Architectural(跨三仓程序,含 ncgo 能力增强 + 新参考项目 + 新模板包)。
- **目标标线:** **参考脚手架**(结构正确、关键路径可跑通、昂贵生产项留清晰接缝 + TODO),不是开箱即用的完整生产系统。

## 已确认决策(brainstorming 结论)

| 维度 | 决策 |
|------|------|
| 服务形态 | micro 工作区:Hertz BFF(admin-bff) + Kitex RPC(rbac-rpc) |
| RBAC | **Casbin**;权限覆盖**两层**:菜单/按钮权限(前端渲染) + API 权限(后端拦截) |
| pg adapter | **内置**基于 sqlc 的自研 casbin `persist.Adapter`(不引 gorm) |
| 认证 | 自建 **JWT 登录**(login/refresh/logout,claims{uid,roles});HS256(默认)→ RS256/JWKS 留接缝 |
| 限流 | 接 **rule-center** 动态限流(admin 写规则 → rule-center;resolver 拉实时规则),resolver 优雅降级 |
| 架构 | **DDD 分层**(domain/application/infrastructure),需先增强 ncgo 才能 export 存活 |
| 构建方法论 | 用 ncgo **开发真实项目 → `ncgo export` → 模板**(dogfooding) |

### 内置 vs 留接缝(按"参考脚手架"标线)
- **内置(便宜且正确):** casbin_rule 为唯一鉴权源(管理表仅元数据,明确同步方向)、口令 argon2/bcrypt + 基本锁定、限流 resolver 降级、基础审计日志(who/what/when)。
- **留接缝 + TODO(昂贵/全仓缺失):** authz 本地 casbin enforcer + watcher(默认先用 Enforce RPC)、RS256/JWKS、OTel 可观测(`ncgo add infra observability`)、组织树/数据域(casbin domain 占位)。

## 关键约束:DDD 与 `ncgo export` 的张力(本程序存在的原因)

`ncgo export templates` 仅扫描既定目录模式(`export.go` 的 `HertzRules`/`KitexRules`):
`main.go`、`conf`、`internal/base/{conf,data,server,logging}`、`internal/{handler,usecase,repository,router,pkg}/**`、`pkg/client/**`。
**完整 DDD 的 `internal/domain/**` 与 `internal/application/**` 不在其中 → export 时被静默丢弃;** 且业务代码需 `skip`(不覆盖),而 `internal/pkg/**` 是 `cover`。
∴ 必须先增强 ncgo,否则 DDD 项目 export 出来是残的。

## 程序分解(3 子项目,按序)

### 子项目 1 —— ncgo DDD 支持(前置,先做)
让 ncgo 的 export(及后续 generate)认识 DDD 分层。

- **DDD 层布局约定:**
  ```
  internal/domain/<agg>/        entity.go / valueobject.go / <agg>.go(聚合根)/ service.go(领域服务)/ repository.go(仓储 PORT + 领域错误)
  internal/application/<agg>/    <agg>_service.go(应用服务/事务编排边界)/ dto.go
  internal/repository/<agg>/     仓储实现(sqlc 支撑)——沿用现有,已被 export 覆盖
  ```
  映射:`application` ≈ 现 `usecase`(编排),`repository` = 基础设施实现,新增独立 `domain` 层承载纯领域模型。
- **P1a(最小可用,真正的前置):** 给 `HertzRules()` 与 `KitexRules()` 各新增两条 FileRule:
  `internal/domain/**/*.go`(skip, loop_service)、`internal/application/**/*.go`(skip, loop_service)。
  更新 export 相关测试 + golden;文档写入 DDD 分层约定。**做完这个,手写 DDD 项目即可干净 export。**
- **P1b(后续,可选):** `ncgo add aggregate <name>` 生成器,scaffold domain/application 骨架并带 `// ncgo:methods:start|end` 锚点;补 base 模板 method 锚点(评估已标缺)。
- **契约敏感面:** export FileRule 集合、`ncgo export` 输出布局、golden。改动需同步测试 + docs(英/中)。

### 子项目 2 —— `micro-admin` 参考项目(用增强后的 ncgo 开发)
真实可跑的 micro 工作区,DDD + Casbin(两层)+ pg adapter + JWT + 审计 + rule-center 限流。

- **services/rpc/rbac-rpc(Kitex,拥有 DB):**
  - domain:`user`、`role`、`permission`、`menu` 聚合(实体/VO/领域服务/仓储 PORT)。
  - application:登录/令牌、RBAC 管理、Enforce 编排。
  - infrastructure:postgres + sqlc(`users/roles/permissions/user_roles/role_permissions/menus/casbin_rule/audit_log`);内置 sqlc-backed casbin `persist.Adapter`;redis(refresh/黑名单)。
  - RPC:`AuthService`(Login/Refresh/Logout/ValidateToken)、`RBACService`(User/Role/Permission/Menu CRUD + AssignRole/Grant + Enforce + 取用户菜单/按钮权限树)。
- **services/bff/admin-bff(Hertz,薄层):**
  - 中间件:JWT 解析(HS256,接缝 RS256)、Authz(`RequirePermission("user:create")` → 调 rbac-rpc.Enforce)、审计日志、限流(rule-center resolver)。
  - handlers:`/login /refresh /logout`、用户/角色/权限/菜单 管理、**取当前用户菜单+按钮权限**(前端渲染)、限流规则管理(写 rule-center)。
  - pkg/client:rbac-rpc + rule-center。
- **依赖:** rule-center 服务(另行部署 / 用 rule-center 模板生成)。

### 子项目 3 —— `micro-admin` 模板包(export → 组装)
对子项目 2 每个服务跑 `ncgo export templates` → 组装 `ncgo-templates/micro-admin`(workspace 壳 + rpc/bff 两个 `*-template/` + idl/{kitex,hertz} + README),端到端 `ncgo new --mode micro --template micro-admin` → `go build` 验证。

## Casbin 两层权限模型(子项目 2 细节,先记录)

- **API 权限(后端):** casbin RBAC model `sub(role/user), obj(权限码/资源), act(HTTP method)`;BFF authz 中间件对受保护路由 Enforce。
- **菜单/按钮权限(前端):** `menus` 表(树:目录/菜单/按钮,button 记权限码)+ `role_permissions` 关联;登录后 BFF 返回「当前用户菜单树 + 按钮权限码集合」驱动前端渲染。
- **单一权威:** `casbin_rule` 为鉴权唯一源;管理接口写 role/permission/menu 元数据时**同步**进 casbin adapter(方向:管理表 → casbin),避免双源漂移。

## 交付方式建议

- 子项目 1 独立走一个 gf-workflow 交付(ncgo 仓库,契约敏感,需 golden/docs)。
- 子项目 2、3 各自 spec → plan → 实现;子项目 3 依赖 1 完成、2 可跑。

## 待你 review 的开放点

1. 子项目 1 是否先只做 **P1a(export 支持)**、P1b(生成器)延后?(推荐:是)
2. DDD 层命名:`application` vs 沿用 `usecase` 命名?(推荐:新增 `domain`+`application`,`usecase` 逐步等价于 `application`)
3. `infrastructure` 是否新目录,还是沿用 `internal/repository/<agg>/`?(推荐:沿用 repository,减少 churn)
4. 审计日志、menus 表是否纳入首版参考项目?(推荐:纳入,属中台核心)
