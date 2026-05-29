---
title: "Hertz 服务接入 rule-center 限流规则中心"
date: 2026-05-15
status: draft
author: claude
---

# 设计文档：Hertz 服务接入 rule-center 限流规则中心

## 摘要

为 ncgo 新增 `source.type: rule_center` 限流规则源，使 Hertz 服务可通过 gRPC 从独立的 rule-center Kitex 服务查询限流规则，实现多服务共享限流策略。

## 背景

当前 Hertz 服务限流规则源支持两种模式：
- `config` — 规则静态写在配置文件中
- `database` — 直连 PostgreSQL 查询

这两种模式在单服务场景下工作良好，但多服务共享限流策略时需要每个服务各自连接数据库或各自维护配置，缺乏统一管理面。

项目已有 rule-center Kitex 预设模板（proto/usecase/handler/repository/sqlc），但 middleware 层为 TODO 占位符，且 Hertz 侧没有接入 rule-center 的能力。

## 目标

1. Hertz 服务可通过 `source.type: rule_center` 配置项切换到远程规则中心
2. 规则查询采用本地缓存优先 + gRPC fallback 策略，兼顾性能与可靠性
3. 已有 `config` 和 `database` 模式不受影响
4. rule-center Kitex 服务模板从半成品变为可用状态

## 非目标

- rule-center 服务自身的限流拦截（它只是规则查询服务，不需要）
- 其他 Kitex 服务的限流中间件实现（后续工作）
- 规则推送/订阅机制（当前采用轮询模式）

## 架构

```
┌─────────────────┐         gRPC          ┌─────────────────┐
│  Hertz 服务      │ ────────────────────→ │  rule-center     │
│  ┌────────────┐ │                        │  (Kitex gRPC)    │
│  │ 限流中间件  │ │  规则查询              │  ┌────────────┐  │
│  │            │ │ ← ─ ─ ─ ─ ─ ─ ─ ─ ─ ─│  │ Handler    │  │
│  │ ┌────────┐ │ │                        │  └─────┬─────┘  │
│  │ │本地缓存 │ │ │                        │  ┌─────┴─────┐  │
│  │ └───┬────┘ │ │                        │  │ UseCase   │  │
│  └─────┼──────┘ │                        │  └─────┬─────┘  │
│        │        │                        │  ┌─────┴─────┐  │
│   HTTP 请求     │                        │  │ Repository│  │
│        │        │                        │  └─────┬─────┘  │
│   限流计数器   │                        │  ┌─────┴─────┐  │
│   (Redis)      │                        │  │ PostgreSQL│  │
└─────────────────┘                        │  └───────────┘  │
                                           └─────────────────┘
```

### 查询流程（每个请求）

1. 检查本地内存缓存是否命中（TTL 内有效）
2. 命中 → 返回缓存的规则
3. Miss → 发 gRPC 到 rule-center 查询，结果写入缓存
4. gRPC 失败 + `fallback_on_error: true` → 使用缓存中的旧规则
5. gRPC 失败 + 无缓存 → 根据 `fail_open` 决定放行或拒绝

## 配置设计

### 新增 `rule_center` source type

```yaml
rate_limit:
  enabled: true
  source:
    type: rule_center              # 新值：rule_center
    cache_ttl_seconds: 60
    fallback_on_error: true
  rule_center:                     # 新增配置块
    address: "localhost:8888"      # rule-center gRPC 地址
    query_timeout_milliseconds: 200
  backend: redis                   # 限流计数器仍在 Redis
  fail_open: false
  key_prefix: "user-api:rate_limit"
  ...
```

### source.type 完整矩阵

| `source.type` | 规则来源 | 适用场景 | 状态 |
|---|---|---|---|
| `config` | 配置文件静态定义 | 简单服务 | ✅ 已存在 |
| `database` | 直连 PostgreSQL | 单服务自带数据库 | ✅ 已存在 |
| `rule_center` | gRPC 远程规则中心 + 本地缓存 | 多服务共享规则 | 新增 |

## CLI 命令

### 1. 创建 rule-center Kitex 服务（已有骨架，需补全）

```bash
ncgo new rule-center --module github.com/acme/rule-center --kind kitex --db postgres --preset rule-center
```

生成内容：
- `idl/rule-center.proto` — 限流规则 CRUD + 查询 gRPC 接口
- `internal/handler/rulecenter/handler.go` — gRPC Handler
- `internal/usecase/rulecenter/usecase.go` — 业务逻辑
- `internal/repository/rulecenter/` — 数据访问层
- `internal/base/middleware/ratelimit.go` — 规则查询中间件（清理 TODO）
- `schema/` + `query/` — sqlc schema 和查询

### 2. 创建 Hertz 服务时指定规则中心地址

```bash
ncgo new user-api --module github.com/acme/user-api --kind hertz --db postgres --rule-center-addr rule-center:8888
```

当 `--rule-center-addr` 提供时：
- `rate_limit.source.type` 设为 `rule_center`
- `rate_limit.rule_center.address` 填入指定地址
- 自动生成 `rule_center_client.go` 和 `rule_cache.go`

### 3. 已有服务接入规则中心（新增）

```bash
ncgo add rule-center --root ./user-api --addr rule-center:8888
```

- 修改现有 `conf/dev/conf.yaml` 中的 `rate_limit.source.type`
- 注入 `rule_center` 配置块
- 生成客户端和缓存文件

## 文件变更清单

### 模板文件（新增）

| 文件 | 说明 |
|---|---|
| `internal/assets/_data/hertz/optional/rule_center_client.go` | gRPC 客户端，连接 rule-center 查询规则 |
| `internal/assets/_data/hertz/optional/rule_cache.go` | 本地内存缓存，定时刷新 + lazy fallback |
| `internal/assets/_data/hertz/ratelint_client_template.yaml` | 客户端模板渲染 |

### 模板文件（修改）

| 文件 | 变更 |
|---|---|
| `internal/assets/_data/hertz/layout.yaml` | 新增 `RuleCenter` 配置 struct、默认值初始化、yaml 解析 |
| `internal/assets/_data/kitex/kitex-template/ratelimit_middleware.yaml` | 清理 TODO 注释，明确说明 rule-center 不需要限流拦截 |

### Go 代码（修改）

| 文件 | 变更 |
|---|---|
| `internal/cli/root.go` | 新增 `--rule-center-addr` flag |
| `internal/cli/add.go` | 新增 `rule-center` 子命令类型处理 |
| `internal/scaffold/mono/files.go` | Hertz 模板渲染时根据 rule-center-addr 生成客户端文件 |
| `internal/scaffold/mono/mono.go` | 传入 rule-center 配置到模板渲染 |
| `internal/scaffold/shared/container.go` | Hertz docker config 支持 `rule_center` 源 |
| `internal/scaffold/infra/wire.go` | （如需）规则中心依赖注入 |

## 实现顺序

1. 清理 `ratelimit_middleware.yaml` TODO 注释
2. 新增 `--rule-center-addr` flag 到 `ncgo new`
3. 新增 `rule-center` 类型到 `ncgo add`
4. 新增 Hertz 模板文件：rule_center_client.go、rule_cache.go
5. 修改 `layout.yaml` 增加 `RuleCenter` 配置
6. 修改 `container.go` 支持 docker compose 中的 rule_center
7. 修改 mono 渲染逻辑传递 rule-center 配置

## 风险与缓解

| 风险 | 缓解措施 |
|---|---|
| rule-center 服务不可用时 Hertz 服务被阻塞 | `query_timeout_milliseconds` 默认 200ms，`fallback_on_error` 回退到缓存 |
| 本地缓存规则过期 | `cache_ttl_seconds` 默认 60s 后标记过期，下次请求刷新 |
| 配置文件结构变更破坏老服务 | 新增字段，不改已有字段，默认值与当前行为一致 |
