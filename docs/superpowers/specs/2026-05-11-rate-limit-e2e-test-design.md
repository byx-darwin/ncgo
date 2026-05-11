# 限流端到端测试 + `*` 路径通配 Design

**Date:** 2026-05-11
**Status:** Draft

## Problem

Hertz 单体项目生成的限流功能（rate-limit）支持 `config`/`grpc`/`database` 三种规则来源，但缺少端到端的测试手段。用户在开发环境中难以快速验证限流是否按预期工作。此外，SQL 查询不支持 `path = '*'` 通配所有路径，导致测试时需要为每个路径单独插入规则。

## Goal

通过 `ncgo test rate-limit` 命令对限流功能进行端到端测试：`seed` 子命令往 PostgreSQL 写入测试规则，`run` 子命令用 vegeta 执行压测并输出报告。同时让限流 SQL 查询支持 `path = '*'` 通配所有路径。

## Design

### 架构总览

```
ncgo new --db postgres --infra redis my-service
cd my-service
docker compose up -d           # 启动 app + postgres + redis + vegeta(待命)
ncgo test rate-limit seed      # 往 PostgreSQL 写入限流规则
ncgo test rate-limit run       # vegeta 打流量，验证限流效果
```

```
┌──────────────────────────────────────────┐
│  compose.yaml (4 services)               │
│  ├── my-service  (Hertz app)            │
│  ├── postgres    (规则存储)              │
│  ├── redis       (限流计数存储)          │
│  └── vegeta      (压测工具，待命)        │
└──────────────────────────────────────────┘
```

### Components

#### 1. SQL `*` 通配支持

**文件:** `internal/assets/_data/hertz/layout.yaml` 中 `internal/db/query/rate_limit_rule.sql` 模板

4 条 SQL 查询均需支持 `*` 通配：

| 查询 | 改动 |
|------|------|
| `GetRateLimitExactRuleByAppKey` | `AND path = $4` → `AND (path = $4 OR path = '*')` |
| `GetRateLimitExactRuleFallback` | 同上 |
| `GetRateLimitPatternRuleByAppKey` | `AND (` 条件中追加 `path_pattern = '*' OR` |
| `GetRateLimitPatternRuleFallback` | 同上 |

语义：`path = '*'` 匹配任何请求路径，作为 exact 规则的通配兜底。查找顺序不变：精确路径 → `*` → pattern。

#### 2. compose.yaml 扩展

**文件:** `internal/assets/_data/hertz/compose.yaml`

追加 3 个 service：

- **postgres:** `postgres:17-alpine`，暴露 5432
- **redis:** `redis:7-alpine`，暴露 6379
- **vegeta:** 从 `Dockerfile.vegeta` 构建，`depends_on: demo`，entrypoint 设为 `/bin/sh`（不自动运行，由用户手动 `docker compose run vegeta` 调用）

#### 3. Dockerfile.vegeta

**文件:** `internal/assets/_data/hertz/Dockerfile.vegeta`（新建）

两阶段构建：`golang:1.22` 编译 vegeta → `alpine:3.20` 运行。

#### 4. conf/docker/conf.yaml 扩展

**文件:** `internal/assets/_data/hertz/layout.yaml` 中 `conf/docker/conf.yaml` 模板

追加：
- `redis.addrs: ["redis:6379"]`
- `rate_limit.enabled: true`
- `rate_limit.source.type: database`
- `rate_limit.backend: redis`

#### 5. `ncgo test rate-limit seed`

**文件:**
- `internal/cli/test.go`（CLI 入口）
- `internal/scaffold/test/ratelimit/seed.go`（核心逻辑）

行为：
1. 读取 `.ncgo/manifest.yaml` 获取 service name
2. 连接 PostgreSQL（默认 DSN: `postgres://app:app@localhost:5432/app`）
3. 清理该 service 的旧规则
4. 插入两条测试规则：
   - `('*', 'pre_auth', '*', 'exact', '*')` — 所有路径，60 秒/10 次
   - `('/healthz', 'pre_auth', 'GET', 'exact', '/healthz')` — 同参数

#### 6. `ncgo test rate-limit run`

**文件:** `internal/scaffold/test/ratelimit/run.go`

行为：
1. 检查本地是否有 `vegeta` 命令
2. 有则直接调用 `vegeta attack`
3. 无则回退到 `docker compose run vegeta attack`
4. 默认目标：`/healthz` 和 `/`，200 QPS，持续 10 秒

### Files Changed

| File | Change |
|------|--------|
| `internal/assets/_data/hertz/layout.yaml` | SQL `*` 通配 + conf/docker 追加 redis/rate_limit |
| `internal/assets/_data/hertz/compose.yaml` | 追加 postgres/redis/vegeta services |
| `internal/assets/_data/hertz/Dockerfile.vegeta` | 新建 |
| `internal/cli/test.go` | 新建：`ncgo test` 命令入口 |
| `internal/scaffold/test/ratelimit/seed.go` | 新建 |
| `internal/scaffold/test/ratelimit/run.go` | 新建 |
| `internal/scaffold/mono/testdata/` | 更新 golden 快照 |

### Error Handling

- PostgreSQL 连接失败：返回错误，提示检查 `docker compose up -d` 和 DSN
- `rate_limit_rules` 表不存在：返回错误，提示先运行 `make sqlc` 初始化数据库
- vegeta 不可用且 docker compose 失败：返回错误，提示手动安装 vegeta

### Testing

- **单元测试:** `seed.go` 的 SQL 生成逻辑（mock pgx），`run.go` 的 target 构建逻辑
- **Golden 测试:** 运行 `go test ./internal/scaffold/mono/... -update-golden` 更新快照
- **冒烟测试:** `./scripts/smoke.sh` 验证 CLI 入口正常

### Out of Scope

- `ncgo test` 的其他测试类型（后续可扩展）
- 动态规则的 gRPC 源测试
- 限流规则的运营平台对接
