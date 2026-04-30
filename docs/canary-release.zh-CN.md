# Hertz / Kitex 金丝雀发布方案

## 1. 观点

金丝雀发布不应该主要写在 Hertz handler、Kitex handler 或 usecase 业务逻辑里。

推荐把金丝雀能力拆成四层：

1. **发布层**：CI/CD、Kubernetes、Argo Rollouts、人工/自动回滚。
2. **流量层**：Ingress/Gateway/Service Mesh、Kitex client selector。
3. **注册、发现与配置层**：Nacos / Polaris 提供服务注册、服务发现、实例元数据和动态灰度规则。
4. **可观测层**：日志、指标、Trace 按版本与流量泳道聚合。

`ncgo` 的定位应该是提供统一约定、模板和 helper，而不是内置完整发布平台。

## 2. 目标

支持 Hertz 与 Kitex 服务以统一方式进行金丝雀发布：

- 支持 stable / canary 两类发布轨道。
- 支持基于权重、Header、用户、租户的流量灰度。
- 支持 Nacos 和 Polaris 作为注册中心。
- 支持 Nacos 和 Polaris 作为服务发现来源。
- 支持 Nacos 和 Polaris 作为配置中心。
- 支持动态调整灰度规则，不重启服务。
- 支持快速回滚。
- 支持按 release metadata 做观测与告警。

## 3. 非目标

第一版不建议做：

- 内置完整发布平台。
- 自动修改用户 Kubernetes YAML。
- 自动创建 Nacos / Polaris 规则。
- 在 usecase 中生成业务灰度分支。
- 多实验平台或复杂 A/B testing DSL。
- 自动回滚执行器。

## 4. 核心概念

### 4.1 发布轨道

| 轨道 | 含义 |
|---|---|
| `stable` | 当前稳定版本，默认承载全部或大部分流量 |
| `canary` | 新版本，小流量验证 |

### 4.2 服务实例元数据

每个 Hertz / Kitex 实例都应该带以下 metadata：

| Key | 示例 | 说明 |
|---|---|---|
| `service.name` | `user-api` | 服务名 |
| `service.kind` | `hertz` / `kitex` | 服务类型 |
| `service.version` | `v1.2.3` | 发布版本 |
| `release.track` | `stable` / `canary` | 发布轨道 |
| `release.git_sha` | `abc1234` | 构建 commit |
| `release.build_time` | `2026-04-30T10:00:00Z` | 构建时间 |
| `release.env` | `prod` / `staging` | 环境 |

这些信息应该进入：

- 注册中心实例 metadata；
- 日志字段；
- Trace attributes；
- Metrics labels；
- `/version` 或等价接口；
- Kubernetes labels / annotations。

## 5. 总体架构

```text
Client
  |
  v
Gateway / Ingress / Mesh
  |              \
  |               \-- config center: Nacos / Polaris canary rules
  v
Hertz BFF / API
  |
  |-- reads X-Traffic-Lane / X-Canary / tenant / user
  |-- writes lane into context
  |-- propagates lane to Kitex metadata
  v
Kitex client selector
  |
  |-- discovers service instances from Nacos / Polaris
  |-- reads registry instance metadata from Nacos / Polaris
  |-- reads dynamic rules from Nacos / Polaris config center
  v
Kitex stable / canary instances
```

## 6. Hertz 金丝雀方案

Hertz 是 HTTP 入口服务，推荐把主要分流放在 Gateway / Ingress / Mesh 层。

### 6.1 权重灰度

典型流程：

1. stable 100%。
2. canary 5%。
3. canary 10%。
4. canary 25%。
5. canary 50%。
6. canary 100%。

适合：

- Nginx Ingress；
- Envoy；
- Istio；
- Argo Rollouts；
- Flagger；
- 云网关。

### 6.2 定向灰度

通过 Header / Cookie / 用户 / 租户决定是否进入 canary：

| 条件 | 示例 |
|---|---|
| Header | `X-Traffic-Lane: canary` |
| Cookie | `canary=1` |
| 用户 | `user_id in [1001, 1002]` |
| 租户 | `tenant_id in [acme]` |

### 6.3 Hertz 应承担的职责

Hertz 服务内建议只做轻量支持：

- 暴露 `/version`；
- 日志增加 `release.track`、`service.version`；
- Trace 增加 `release.track`、`service.version`；
- Middleware 读取 `X-Traffic-Lane`；
- 把灰度 lane 写入 request context；
- 调用 Kitex 时透传 lane。

不建议在 handler 中硬编码“如果 canary 就走另一套业务逻辑”。

## 7. Kitex 金丝雀方案

Kitex 是 RPC 服务，推荐优先使用 **注册中心 metadata + client-side selector**。

### 7.1 实例注册

stable 与 canary 可以注册到同一个 Kitex service name，但 metadata 不同：

| 实例 | Service | Metadata |
|---|---|---|
| stable | `user.UserService` | `release.track=stable`, `service.version=v1.2.2` |
| canary | `user.UserService` | `release.track=canary`, `service.version=v1.2.3` |

### 7.2 客户端选路

Kitex client selector 根据请求上下文和动态规则选择实例：

1. 如果请求明确指定 `X-Traffic-Lane=canary`，优先选 canary。
2. 如果用户 / 租户命中 canary 规则，选 canary。
3. 如果命中百分比灰度，按权重选 canary。
4. 否则选 stable。
5. 如果 canary 无可用实例，按策略 fallback stable 或 fail-fast。

### 7.3 灰度上下文透传

HTTP 到 RPC：

```text
X-Traffic-Lane: canary
X-User-ID: 1001
X-Tenant-ID: acme
```

Hertz BFF 应把这些信息写入 Kitex metadata，例如：

```text
traffic.lane=canary
traffic.user_id=1001
traffic.tenant_id=acme
```

Kitex 服务继续调用下游 Kitex 服务时，也应继续透传这些 metadata。

## 8. Nacos 支持方案

Nacos 同时支持注册中心、服务发现和配置中心，适合承担：

- 服务实例注册；
- 服务实例发现；
- 实例 metadata 存储；
- 动态灰度规则配置。

### 8.1 Nacos 注册中心

服务注册时写入 metadata：

```yaml
serviceName: user-rpc
groupName: DEFAULT_GROUP
clusterName: DEFAULT
ephemeral: true
metadata:
  service.kind: kitex
  service.version: v1.2.3
  release.track: canary
  release.git_sha: abc1234
  release.build_time: "2026-04-30T10:00:00Z"
```

Hertz 服务也可以注册 metadata，用于网关、控制面或观测系统识别版本。

### 8.2 Nacos 服务发现

Kitex client selector 应从 Nacos 获取目标服务的实例列表，并读取每个实例的 metadata：

```text
discover user-rpc instances from Nacos
  -> filter healthy / enabled instances
  -> split instances by release.track
  -> stable pool: release.track=stable
  -> canary pool: release.track=canary
  -> route by canary rule
```

服务发现需要支持：

- 按 `serviceName` / `groupName` / `clusterName` 查询实例；
- 过滤不可用实例；
- 读取 `release.track`、`service.version` 等 metadata；
- 监听实例变更并刷新本地 instance cache；
- 配置中心不可用时，仍可基于服务发现 metadata 默认路由到 stable。

生成代码已提供 SDK-neutral seam：真实 Nacos SDK adapter 可把 SDK instance 映射为 `release.NacosInstance`，再由 `release.NacosDiscoverer` / `release.InstancesFromNacos` 转成统一 `release.Instance`。

### 8.3 Nacos 配置中心

建议按环境、服务、灰度主题拆分配置：

| 配置项 | 示例 |
|---|---|
| namespace | `prod` |
| group | `NCGO_CANARY` |
| dataId | `user-rpc.canary.yaml` |

示例配置：

```yaml
version: 1
enabled: true
service: user-rpc
default_track: stable
fallback: stable
rules:
  - name: force-header
    match:
      headers:
        X-Traffic-Lane: canary
    route:
      track: canary

  - name: tenant-canary
    match:
      tenants: [acme, beta]
    route:
      track: canary

  - name: percent-canary
    match: {}
    route:
      weighted:
        stable: 95
        canary: 5
```

生成代码已提供 `release.NacosRuleProvider` skeleton。真实 SDK adapter 只需实现 `LoadRules(ctx, release.NacosRuleConfig) (release.RuleSet, error)`；未显式配置时默认 group 为 `NCGO_CANARY`，dataId 为 `<service>.canary.yaml`。

### 8.4 Nacos 动态刷新

服务启动时：

1. 读取 Nacos 配置。
2. Watch 配置变更。
3. 原子更新本地 canary rule cache。
4. selector 使用本地 cache 做决策。

配置中心不可用时：

- 保留最近一次有效规则；
- 首次加载失败时默认走 stable；
- 记录告警日志和 metrics。

## 9. Polaris 支持方案

Polaris 同时支持服务治理、服务注册、服务发现、配置管理和流量路由，适合更强治理场景。

### 9.1 Polaris 注册中心

服务实例注册 metadata：

```yaml
namespace: prod
service: user-rpc
metadata:
  service.kind: kitex
  service.version: v1.2.3
  release.track: canary
  release.git_sha: abc1234
  release.build_time: "2026-04-30T10:00:00Z"
```

Polaris 的 namespace / service / metadata 可以作为 Kitex selector 的实例过滤条件。

### 9.2 Polaris 服务发现

Kitex client selector 应从 Polaris 获取目标服务实例，并使用实例 metadata 做 stable / canary 池划分：

```text
discover user-rpc instances from Polaris
  -> filter healthy instances by Polaris health status
  -> read namespace / service / metadata
  -> stable pool: release.track=stable
  -> canary pool: release.track=canary
  -> apply Polaris or ncgo canary rules
```

服务发现需要支持：

- 按 `namespace` / `service` 查询实例；
- 读取实例 metadata；
- 使用 Polaris 健康状态过滤实例；
- 订阅实例变更并刷新本地 instance cache；
- 与 Polaris 路由能力协同，避免本地 selector 和控制面规则冲突。

生成代码已提供 SDK-neutral seam：真实 Polaris SDK adapter 可把 SDK instance 映射为 `release.PolarisInstance`，再由 `release.PolarisDiscoverer` / `release.InstancesFromPolaris` 转成统一 `release.Instance`。

### 9.3 Polaris 配置中心

建议配置组织方式：

| 配置项 | 示例 |
|---|---|
| namespace | `prod` |
| group | `ncgo-canary` |
| file | `user-rpc.yaml` |

配置内容可复用统一规则 schema：

```yaml
version: 1
enabled: true
service: user-rpc
default_track: stable
fallback: stable
rules:
  - name: header-canary
    match:
      headers:
        X-Traffic-Lane: canary
    route:
      track: canary

  - name: user-canary
    match:
      users: [1001, 1002]
    route:
      track: canary

  - name: weighted-canary
    match: {}
    route:
      weighted:
        stable: 90
        canary: 10
```

生成代码已提供 `release.PolarisRuleProvider` skeleton。真实 SDK adapter 只需实现 `LoadRules(ctx, release.PolarisRuleConfig) (release.RuleSet, error)`；未显式配置时默认 group 为 `ncgo-canary`，fileName 为 `<service>.canary.yaml`。

### 9.4 Polaris 路由能力

如果团队已经使用 Polaris 的服务治理路由能力，可以优先使用 Polaris 控制面做路由：

- 按 metadata 路由到 canary 实例；
- 按请求标签路由；
- 按权重路由；
- 服务端动态下发规则。

如果 Kitex 接入层暂时无法完全复用 Polaris 路由能力，则先使用统一 canary config + Kitex client selector 的方式落地。

## 10. 统一灰度规则模型

为了避免 Nacos / Polaris 两套规则互不兼容，建议 `ncgo` 定义中立 schema，然后映射到具体配置中心。

```yaml
version: 1
enabled: true
service: user-rpc
default_track: stable
fallback: stable
rules:
  - name: header
    priority: 100
    match:
      headers:
        X-Traffic-Lane: canary
    route:
      track: canary

  - name: tenant
    priority: 90
    match:
      tenants: [acme]
    route:
      track: canary

  - name: percentage
    priority: 10
    match: {}
    route:
      weighted:
        stable: 95
        canary: 5
```

### 10.1 匹配字段

| 字段 | 来源 |
|---|---|
| `headers` | Hertz HTTP header 或 RPC metadata |
| `cookies` | Hertz HTTP cookie |
| `users` | `X-User-ID` 或认证上下文 |
| `tenants` | `X-Tenant-ID` 或租户上下文 |
| `regions` | 请求地域或部署地域 |

### 10.2 路由动作

| 动作 | 说明 |
|---|---|
| `track: stable` | 强制 stable |
| `track: canary` | 强制 canary |
| `weighted` | 按权重分流 |

## 11. 配置中心职责边界

Nacos / Polaris 配置中心只负责保存和下发规则，不直接做业务判断。

服务本地应有一个 canary rule engine：

1. 订阅配置。
2. 校验 schema。
3. 编译规则。
4. 从 Nacos / Polaris 服务发现缓存中读取可用实例。
5. 将请求上下文匹配为目标 track。
6. 把 track 交给 Hertz middleware 或 Kitex selector。

这样做的好处：

- Nacos / Polaris 可替换；
- 规则格式统一；
- 单元测试容易；
- 本地缓存可保证配置中心故障时服务继续运行。

## 12. 发布流程

### 12.1 构建

构建时注入：

```text
service.version=v1.2.3
release.git_sha=abc1234
release.build_time=2026-04-30T10:00:00Z
release.track=canary
```

### 12.2 部署

部署 stable 与 canary 两套实例：

```text
user-api-stable
user-api-canary
user-rpc-stable
user-rpc-canary
```

### 12.3 注册

实例启动后注册到 Nacos / Polaris，并附带 release metadata。调用方通过 Nacos / Polaris 服务发现获取实例列表和 metadata。

### 12.4 配置

在 Nacos / Polaris 配置中心发布 canary rule。

### 12.5 放量

按阶段调整配置：

```text
0% -> 5% -> 10% -> 25% -> 50% -> 100%
```

### 12.6 回滚

回滚优先改配置，而不是先销毁实例：

1. 将 canary 权重调为 0。
2. 或将 `enabled=false`。
3. 观察 stable 指标恢复。
4. 保留 canary 实例用于排查。
5. 必要时回滚部署。

## 13. 可观测要求

必须按以下维度聚合：

- `service.name`
- `service.kind`
- `service.version`
- `release.track`
- `release.git_sha`
- `traffic.lane`

关键指标：

- HTTP 5xx；
- RPC error rate；
- business error rate；
- p95 / p99 latency；
- timeout；
- panic；
- saturation；
- canary 与 stable 差异。

使用 LoongSuite Go Agent 时，建议运行时配置：

```bash
OTEL_SERVICE_NAME=user-rpc \
OTEL_EXPORTER_OTLP_ENDPOINT=http://otel-collector:4318 \
OTEL_TRACES_EXPORTER=otlp \
OTEL_METRICS_EXPORTER=otlp \
./user-rpc
```

## 14. ncgo 产品化状态

已新增 optional：

```bash
ncgo add infra release_canary --root .
ncgo add infra canary --root .  # alias
ncgo add infra canary --root . --wire  # 可选：自动挂载默认 traffic middleware
ncgo add infra canary --root . --wire --dry-run  # 预览，不修改文件
ncgo add infra canary --root . --wire --dry-run --output json  # 输出机器可读 plan
ncgo add infra canary --root . --wire --plan  # --dry-run --output json 简写
```

推荐落地流程：

1. 先执行 `--wire --dry-run`，确认会创建的 optional 文件、manifest 更新以及将修改的 server/client 文件；
2. CI / agent / 前端可使用 `--output json` 读取结构化 `plan`，或直接使用 `--plan` 简写；
3. 确认 plan 符合预期后再执行不带 `--dry-run` 的 `--wire`；
4. 如后续引入 Nacos / Polaris SDK adapter，再按 adapter 文档手动补依赖，ncgo 不会自动执行依赖安装。

`--wire` 在真实写入 optional 文件、manifest 和源码前会先做 preflight：如果默认模板 anchor 缺失、import block 找不到或格式化失败，会直接报错，并避免留下“optional 已写但 wiring 失败”的半完成状态。`--dry-run` 保证不写 optional 文件、不保存 manifest、不修改 server/client 源码。

JSON plan 常见条目：

```text
file/create                  # 将创建 canary.go / hertz.go / kitex.go
file/overwrite               # 加 --force 时将覆盖已存在 optional 文件
manifest/add                 # manifest 将记录 release_canary
manifest/already_present     # manifest 已记录该 optional
wire/update                  # 将挂载默认 traffic middleware
wire/already_wired           # 已接线，无需再次修改源码
wire/add_import              # 将补充 internal/base/release import
wire/insert_traffic_middleware # 将在 server 侧挂载 release traffic middleware
wire/insert_client_middleware # Kitex client 将插入 release traffic middleware
next_step/run                # 需要用户手动执行的后续命令
```

其中 wire operation-level 条目会保留原有 `detail`，并在适用时额外返回 `anchorSource` / `anchor`：`anchorSource=marker` 表示命中 `// ncgo:wire:*` marker，`anchorSource=legacy` 表示回退到旧模板源码片段。

### 14.1 故障排查

#### `--wire could not find ... anchor`

说明当前 `server.go` / `client.go` 已偏离 ncgo 默认模板，CLI 找不到安全插入点。处理方式：

1. 先运行 `ncgo add infra canary --root . --wire --plan` 查看失败前的预期文件；
2. 对照本章后续示例手动补 import 和 traffic middleware；
3. 或在自定义模板中保留 `// ncgo:wire:canary:server-traffic`、`// ncgo:wire:kitex-client:middleware` marker；
4. 不建议让 CLI 猜测非默认代码结构，避免误改流量治理 middleware / interceptor 顺序。

#### 已手动改过 server/client

如果已经手动接入 canary，再次执行 `--wire --plan` 可能返回 `wire/already_wired`。这表示 CLI 识别到目标调用已存在，不会重复插入。

- `wire/already_wired` 不是错误，不需要再执行 `--force`；
- 如果只接入了一部分，CLI 不会猜测剩余改动，建议手动补齐缺失部分；
- 如果希望继续由 CLI 自动接线，可恢复到带 marker 的默认模板片段后再执行 `--wire`。

#### 什么时候使用 `--force`

`--force` 只影响 optional 文件覆盖，例如 `internal/base/release/canary.go`，不表示强制改写 server/client wiring。源码 wiring 仍遵守 marker / legacy anchor preflight，找不到安全 anchor 时会失败。

建议只在明确要刷新 ncgo 管理的 optional 文件时使用 `--force`。如果 `server.go` / `client.go` 是手工维护的，优先用 `--wire --plan` 看计划，再决定手动修改，不要把 `--force` 当作“强制接线”。

#### dry-run 与真实执行输出对照

`--dry-run` 只预览，不写 optional 文件、不保存 manifest、不修改 server/client 源码；真实执行会写入文件并保存 manifest，若加 `--wire` 还会修改默认模板中的安全 anchor。

```text
$ ncgo add infra canary --root . --wire --dry-run
would write .../internal/base/release/canary.go
would wire .../internal/base/server/server.go
(dry-run: manifest would be updated)
(dry-run: no files were written)

$ ncgo add infra canary --root . --wire
wrote .../internal/base/release/canary.go
wired .../internal/base/server/server.go
```

CI、agent 或前端应优先使用 `--plan` 或 `--wire --dry-run --output json`，读取 `plan` 中的 `file/*`、`manifest/*`、`wire/*` 和 `next_step/run` 后再决定是否真实执行。

#### 为什么不自动安装 SDK

当前 canary optional 是 SDK-neutral MVP，不自动引入 Nacos / Polaris SDK，也不自动执行 `go get`。后续接真实 SDK adapter 时，请先审阅 `--plan`，再按 adapter 文档手动补依赖。

当前生成：

```text
internal/base/release/canary.go
internal/base/release/hertz.go  # Hertz 服务额外生成
internal/base/release/kitex.go  # Kitex 服务额外生成
```

说明：第一版是 SDK-neutral MVP，不直接依赖 Nacos / Polaris SDK。它先固化统一模型、规则引擎、metadata、framework context adapter、selector seam 和 instance pool 选择逻辑；后续可以在不改变业务约定的前提下新增 `nacos.go`、`polaris.go`、Kitex selector adapter 和部署示例。

默认 Hertz / Kitex 模板已经预留安全 wiring 注释：未启用 `release_canary` 时不会 import `internal/base/release`，项目仍可正常编译；执行 `ncgo add infra canary --root .` 后，再按注释或下方示例手动取消注释并补 import。若希望由 CLI 自动接入默认生成代码，可显式加 `--wire`；该选项只支持已知的 ncgo 默认模板片段，失败时会报错并提示无法找到 anchor。建议真实执行前先用 `--wire --dry-run --output json` 审阅 plan。

### 14.1 release info

职责：

- 已支持：统一读取 `SERVICE_NAME`、`SERVICE_KIND`、`SERVICE_VERSION`、`RELEASE_TRACK`、`GIT_SHA`、`BUILD_TIME`、`RELEASE_ENV`。
- 已支持：暴露 `ReleaseInfo`。
- 已支持：生成 registry / discovery 可复用 metadata。

### 14.2 context helper

职责：

- 已支持：定义 `Traffic` 模型，包含 lane、user、tenant、region、headers、cookies、sticky key。
- 已支持：写入 / 读取 Go `context.Context`。
- 已支持：Hertz header adapter，读取 `X-Traffic-Lane`、`X-User-ID`、`X-Tenant-ID`。
- 已支持：Kitex metadata adapter，读取并透传 `traffic.lane`、`traffic.user_id`、`traffic.tenant_id` 及对应 Header key。

### 14.3 rule engine

职责：

- 已支持：统一 canary rule schema。
- 已支持：priority。
- 已支持：header / cookie / user / tenant / region / weighted percentage。
- 已支持：输出目标 `release.track`，并支持 `fallback=stable` / `fallback=fail_fast`。

### 14.4 registry / discovery adapter

职责：

- 已支持：Nacos / Polaris SDK-neutral discovery config 与 provider 标识。
- 已支持：统一 `Instance` 模型承载 Nacos / Polaris 发现结果。
- 已支持：按 `release.track` 分 stable / canary / unknown instance pool。
- 已支持：按权重与 sticky key 选择实例。
- 已支持：`Discoverer` / `RuleProvider` / `Selector` 抽象，便于后续接入 Nacos / Polaris SDK。
- 已支持：SDK-neutral Kitex client load balancer adapter，可复用 `Selector` 对 Kitex discovery instances 做 stable / canary 选路。
- 已支持：Nacos / Polaris SDK-neutral adapter skeleton（instance DTO、discoverer、rule provider、mapper）。
- 待增强：真实 Nacos / Polaris SDK adapter、watch、本地缓存。

### 14.5 config adapter

职责：

- 已支持：Nacos / Polaris rule provider skeleton。
- 待增强：Nacos config watch。
- 待增强：Polaris config watch。
- 待增强：本地缓存和配置校验。

### 14.6 安全 wiring 示例

Hertz 服务建议在 `internal/base/server/server.go` 的 `middleware.RequestID()` 后、`middleware.AccessLog()` 前接入，这样 access log / 后续 adapter 可以读到灰度上下文：

```go
import "<module>/internal/base/release"

h.Use(middleware.RequestID())
h.Use(release.HertzTraffic())
h.Use(middleware.AccessLog())
```

Kitex server 建议在 `endpoint.Chain(...)` 的 `interceptor.RequestID()` 后接入：

```go
import "<module>/internal/base/release"

kitexserver.WithMiddleware(endpoint.Chain(
    interceptor.RequestID(),
    release.KitexTraffic(),
    interceptor.AccessLog(),
))
```

Kitex client wrapper 建议在启用 TTHeader 后追加 middleware，用于出站 RPC 继续透传 `traffic.lane` / `traffic.user_id` / `traffic.tenant_id`：

```go
import "<module>/internal/base/release"

options = append(options, kitexclient.WithMetaHandler(transmeta.ClientTTHeaderHandler))
options = append(options, kitexclient.WithMiddleware(release.KitexTraffic()))
options = append(options, kitexclient.WithLoadBalancer(
    release.NewKitexCanaryLoadBalancer(cfg.ServiceName, ruleProvider, nil),
))
```

`release.NewKitexCanaryLoadBalancer` 当前是 SDK-neutral seam。它只消费 Kitex resolver 已发现的实例及其 metadata；真实实例来源仍应在 Nacos / Polaris adapter 落地后，通过 Kitex resolver 提供带 `release.track` 的实例，并通过 `RuleProvider` 提供动态 canary rule。

## 15. MVP 分阶段

### Phase 1：约定与观测

- 已完成：生成 release metadata helper。
- Hertz `/version`。
- 日志 / Trace 附加 release metadata。
- 已完成：Nacos / Polaris 注册 metadata 模型。
- 已完成：Nacos / Polaris 服务发现 metadata 模型。

### Phase 2：灰度上下文透传

- 已完成：Hertz middleware 读取 Header。
- 已完成：Kitex metadata 透传。
- 已完成：下游继续传播 lane / user / tenant。

### Phase 3：配置中心规则

- 待增强：Nacos config watch。
- 待增强：Polaris config watch。
- 已完成：本地 rule engine。
- 已完成：Nacos / Polaris rule provider skeleton。
- 待增强：动态调整 canary 规则。

### Phase 4：Kitex client selector

- 已完成：按 registry metadata 过滤 stable / canary 实例。
- 已完成：SDK-neutral selector seam，可从任意 `Discoverer` / `RuleProvider` 获取实例与规则。
- 已完成：Kitex client load balancer adapter skeleton，可把 Kitex discovery result 转接到统一 selector。
- 待增强：从 Nacos / Polaris 服务发现缓存获取实例。
- 已完成：支持 fallback stable / fail-fast。
- 已完成：支持权重分流。

### Phase 5：部署模板示例

- Kubernetes labels 示例。
- Argo Rollouts 示例。
- Nacos / Polaris 配置示例。

## 16. 推荐结论

### Hertz

优先采用：

```text
Gateway / Ingress / Argo Rollouts 权重分流 + Header/Cookie 定向灰度
```

Hertz 服务内部只做 metadata、观测和上下文透传。

### Kitex

优先采用：

```text
Nacos / Polaris service discovery + registry metadata + Kitex client selector + Nacos / Polaris config rules
```

Kitex 不建议在业务 handler/usecase 中写灰度分支。

### Nacos / Polaris

统一支持方式：

```text
注册中心：实例 metadata 标识 stable / canary
服务发现：调用方获取实例列表与 metadata
配置中心：动态下发 canary rule
服务本地：rule engine + selector 执行选路
```

这样可以让 Hertz / Kitex 共用同一套金丝雀模型，同时保留 Nacos 与 Polaris 的基础设施选择空间。