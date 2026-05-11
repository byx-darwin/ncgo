# gRPC 规则中心 Rule-Center 端到端测试 Design

**Date:** 2026-05-11
**Status:** Draft

## Problem

Hertz 模板中的限流功能已支持 `source.type: grpc` 作为动态规则来源，客户端（`GRPCClient` 接口、`grpcSource` 包装、`Resolver` 解析器）全链路已通，但缺少一个真实的 gRPC 规则中心服务端。用户在微服务模式下无法端到端验证 gRPC 限流规则获取链路是否正常工作。

## Goal

通过 `ncgo add rpc rule-center --preset rule-center` 在微服务 workspace 中生成 Kitex 规则中心服务，该服务从 PostgreSQL 读取限流规则并通过 gRPC 对外提供 CRUD 接口。配合已有的 `ncgo test rate-limit seed` 和 `ncgo test rate-limit run` 完成端到端验证。

## Design

### 架构总览

```
my-workspace/
├── .ncgo/ncgo.workspace.yaml    # workspace: [order-api(hertz/bff), rule-center(kitex)]
├── compose.yaml                  # order-api + rule-center + postgres + redis + vegeta
├── services/
│   ├── order-api/                # Hertz BFF，限流消费方
│   │   ├── .ncgo/manifest.yaml
│   │   └── conf/docker/conf.yaml   # rate_limit.source.type = "grpc"
│   │                                # rate_limit.grpc.target = "rule-center:8888"
│   └── rule-center/              # Kitex，规则中心服务端
│       ├── .ncgo/manifest.yaml   # with_database=true
│       ├── idl/rule-center.proto  # RuleService CRUD 接口
│       ├── internal/handler/rulecenter/
│       ├── internal/db/query/     # sqlc 生成的 SQL 查询
│       └── conf/docker/conf.yaml
```

**数据流：**

```
请求 → order-api (Hertz)
        ↓ rate-limit middleware (source.type=grpc)
        Lookup{Service, Phase="grpc", AppKey, Method, Path}
        ↓ gRPC 调用
    rule-center:8888 (Kitex RuleService.GetRule)
        ↓ sqlc 查询 PostgreSQL (4 条 SQL + * 通配)
    rate_limit_rules 表 (phase='grpc')
        ↓ 返回 RateLimitRule
    order-api 执行限流 (Redis 计数)
```

### 用户流程

```bash
# 1. 创建 workspace
ncgo new --mode micro --db postgres --infra redis my-workspace
cd my-workspace

# 2. 添加 Hertz 网关（限流消费方）
ncgo add bff order-api

# 3. 添加 Kitex 规则中心
ncgo add rpc rule-center --preset rule-center

# 4. 一键启动
docker compose up -d

# 5. 种子数据（同时写入 HTTP 和 gRPC 规则）
ncgo test rate-limit seed

# 6. 验证规则写入（直接查库）
docker compose exec postgres psql -U postgres -d app -c "SELECT * FROM rate_limit_rules;"

# 7. 验证 gRPC 规则中心（grpcurl 调用）
grpcurl -plaintext localhost:8888 ratelimit.v1.RuleService.GetRule \
  '{"service":"order-api","phase":"grpc","method":"GET","path":"/healthz"}'
# 期望：found=true, rule 包含限流配置

grpcurl -plaintext localhost:8888 ratelimit.v1.RuleService.GetRule \
  '{"service":"order-api","phase":"grpc","method":"GET","path":"/some/other/path"}'
# 期望：found=true, 命中 * 通配规则

# 8. 压测 HTTP 验证限流（vegeta）
ncgo test rate-limit run
```

### Proto 定义

**文件:** `internal/assets/_data/kitex/kitex-template/ratelimit.yaml` → `idl/rule-center.proto`

```proto
syntax = "proto3";
package ratelimit.v1;
option go_package = "api/ratelimit/v1;ratelimitv1";

service RuleService {
  rpc GetRule(GetRuleRequest) returns (GetRuleResponse);
  rpc CreateRule(CreateRuleRequest) returns (CreateRuleResponse);
  rpc UpdateRule(UpdateRuleRequest) returns (UpdateRuleResponse);
  rpc DeleteRule(DeleteRuleRequest) returns (DeleteRuleResponse);
  rpc ListRules(ListRulesRequest) returns (ListRulesResponse);
}

message GetRuleRequest {
  string service = 1;
  string phase = 2;
  string method = 3;
  string path = 4;
  optional string app_key = 5;
}

message CreateRuleRequest {
  string service = 1;
  string phase = 2;
  string method = 3;
  string match_kind = 4;              // exact | prefix | glob | regex
  string path = 5;
  string path_pattern = 6;
  optional string app_key = 7;
  int32 priority = 8;
  bool enabled = 9;
  repeated string key_by = 10;
  string strategy = 11;               // fixed_window | token_bucket
  int32 window_seconds = 12;
  int32 max_requests = 13;
  double requests_per_second = 14;
  int32 burst = 15;
  int32 client_ttl_seconds = 16;
}

message UpdateRuleRequest {
  int64 id = 1;
  optional bool enabled = 2;
  optional int32 priority = 3;
  optional repeated string key_by = 4;
  optional string strategy = 5;
  optional int32 window_seconds = 6;
  optional int32 max_requests = 7;
  optional double requests_per_second = 8;
  optional int32 burst = 9;
  optional int32 client_ttl_seconds = 10;
}

message DeleteRuleRequest {
  int64 id = 1;
}

message ListRulesRequest {
  optional string service = 1;
  optional string phase = 2;
}

message GetRuleResponse {
  bool found = 1;
  RateLimitRule rule = 2;
}

message CreateRuleResponse {
  int64 id = 1;
}

message UpdateRuleResponse {}

message DeleteRuleResponse {}

message ListRulesResponse {
  repeated RateLimitRule rules = 1;
}

message RateLimitRule {
  int64 id = 1;
  string service = 2;
  string phase = 3;
  string method = 4;
  string match_kind = 5;
  string path = 6;
  string path_pattern = 7;
  optional string app_key = 8;
  int32 priority = 9;
  bool enabled = 10;
  repeated string key_by = 11;
  string strategy = 12;
  int32 window_seconds = 13;
  int32 max_requests = 14;
  double requests_per_second = 15;
  int32 burst = 16;
  int32 client_ttl_seconds = 17;
}
```

### Handler 实现

**文件:** `internal/assets/_data/kitex/kitex-template/ratelimit_handler.yaml` → `internal/handler/rulecenter/rule_service_impl.go`

| 方法 | 行为 |
|------|------|
| `GetRule` | 按 service+phase+method+path+app_key 查询。先查 exact 规则（含 `path='*'` 通配），未命中再查 pattern 规则（含 `path_pattern='*'` 通配）。返回 `found + rule` |
| `CreateRule` | INSERT 一条规则到 `rate_limit_rules`，返回生成的 `id` |
| `UpdateRule` | UPDATE 指定 `id` 的规则字段，支持部分更新 |
| `DeleteRule` | DELETE 指定 `id` 的规则 |
| `ListRules` | SELECT 规则列表，支持按 `service` 和 `phase` 过滤 |

### SQL 查询

**文件:** `internal/assets/_data/kitex/` 下的 sqlc 相关模板

复用 Hertz 已有的 4 条查询（已在 `layout.yaml` 支持 `*` 通配），新增 5 条 CRUD 查询：

| 查询名称 | SQL | 说明 |
|----------|-----|------|
| `GetRateLimitExactRuleByAppKey` | `WHERE service=$1 AND phase=$2 AND method=$3 AND match_kind='exact' AND (path=$4 OR path='*') AND app_key=$5` | exact 查询（复用 Hertz 同名 SQL） |
| `GetRateLimitExactRuleFallback` | `WHERE service=$1 AND phase=$2 AND method=$3 AND match_kind='exact' AND (path=$4 OR path='*') AND app_key IS NULL` | exact 兜底 |
| `GetRateLimitPatternRuleByAppKey` | `WHERE service=$1 AND phase=$2 AND method=$3 AND app_key=$5 AND (path_pattern='*' OR ...)` | pattern 查询 |
| `GetRateLimitPatternRuleFallback` | `WHERE service=$1 AND phase=$2 AND method=$3 AND app_key IS NULL AND (path_pattern='*' OR ...)` | pattern 兜底 |
| `CreateRateLimitRule` | INSERT | 新增规则 |
| `UpdateRateLimitRule` | UPDATE | 更新规则 |
| `DeleteRateLimitRule` | DELETE | 删除规则 |
| `ListRateLimitRules` | SELECT ... ORDER BY | 列出规则 |

### SQL Schema

**文件:** `internal/assets/_data/kitex/schema/000002_rate_limit_rules.sql`

与 Hertz 共用同一张表结构（`rate_limit_rules`），`phase` 字段支持 `grpc` 值。

### Kitex 限流中间件

**文件:** `internal/assets/_data/kitex/kitex-template/ratelimit_middleware.yaml` → `internal/base/middleware/ratelimit.go`

Kitex interceptor 实现 gRPC 服务自身的限流保护：

```go
func RateLimitMiddleware(cfg conf.RateLimitConfig) endpoint.Middleware {
    resolver := ratelimit.NewResolver(cfg, ratelimit.Options{...})
    return func(next endpoint.Endpoint) endpoint.Endpoint {
        return func(ctx context.Context, req, resp interface{}) error {
            // 构造 Lookup: phase="grpc", method/rpc_info提取
            lookup := ratelimit.Lookup{
                Phase:    "grpc",
                Service:  cfg.RateLimit.GRPC.ServiceName,
                Method:   rpcinfo.GetRPCInfo(ctx).Method(),
                Path:     rpcinfo.GetRPCInfo(ctx).Method(), // gRPC method as path
                ClientIP: extractClientIP(ctx),
            }
            rule, err := resolver.Resolve(ctx, lookup)
            if err != nil && !cfg.RateLimit.FailOpen {
                return err
            }
            // 执行限流
            return next(ctx, req, resp)
        }
    }
}
```

### conf.yaml 扩展

**文件:** `internal/assets/_data/kitex/kitex-template/conf.yaml`

在 Kitex Config struct 中追加 `RateLimit RateLimitConfig`，结构与 Hertz 保持一致（`source.type` / `grpc` / `database` / `backend` / `pre_auth` / `post_auth`）。

### `ncgo add rpc --preset rule-center`

**文件:** `internal/cli/add.go` + `internal/scaffold/rpc/`

- `--preset rule-center` 标志触发特殊模板：
  - 使用 `ratelimit.yaml` 替代默认 handler/usecase 模板
  - 生成 `idl/rule-center.proto` 而非默认 IDL
  - manifest 中 `with_database=true`
  - 生成时优先调用 `make sqlc` 再 `kitex`

### `ncgo test rate-limit seed` 扩展

**文件:** `internal/scaffold/test/ratelimit/seed.go`

`buildSeedSQL` 追加 `phase='grpc'` 规则：
- `('*', 'grpc', '*', 'exact', '*', '*')` — gRPC 所有路径，60 秒/10 次
- `('/ratelimit.v1.RuleService/GetRule', 'grpc', '*', 'exact', '/ratelimit.v1.RuleService/GetRule', ...)` — 精确规则

### `ncgo test rate-limit run` 扩展

**文件:** `internal/scaffold/test/ratelimit/run.go`

- 新增 `--grpc` 标志
- `--grpc` 时使用 `grpcurl` 替代 vegeta：
  ```bash
  grpcurl -plaintext -d '{"service":"order-api","phase":"grpc","path":"/test"}' \
    localhost:8888 ratelimit.v1.RuleService.GetRule
  ```

### Workspace compose 支持

`renderWorkspaceCompose` 通过 `loadWorkspaceComposeApps` 已能正确传播 `Infra`/`WithDatabase`。当 workspace 中有 `with_database=true` 的服务时自动追加 postgres，有 `infra: [redis]` 时自动追加 redis。

需确保 `ncgo add rpc rule-center` 生成的 manifest 中 `with_database=true`，且 workspace 级 manifest 或至少一个服务包含 `infra: [redis]`。

### Files Changed

| File | Change |
|------|--------|
| `internal/assets/_data/kitex/kitex-template/ratelimit.yaml` | 新建：proto 定义 |
| `internal/assets/_data/kitex/kitex-template/ratelimit_handler.yaml` | 新建：handler 实现 |
| `internal/assets/_data/kitex/kitex-template/ratelimit_middleware.yaml` | 新建：Kitex 限流中间件 |
| `internal/assets/_data/kitex/schema/000002_rate_limit_rules.sql` | 新建：限流规则表 |
| `internal/assets/_data/kitex/kitex-template/conf.yaml` | 修改：追加 rate_limit 配置 |
| `internal/assets/_data/kitex/kitex-template/server.yaml` | 修改：server wiring 接入限流 |
| `internal/assets/_data/kitex/sqlc.yaml` | 修改：追加 rate_limit 查询 |
| `internal/assets/_data/kitex/layout.yaml` | 新建：包含 SQL 模板和 handler/wiring 代码 |
| `internal/cli/add.go` | 修改：追加 `--preset` 标志 |
| `internal/scaffold/rpc/rpc.go` | 修改：支持 preset 逻辑 |
| `internal/scaffold/test/ratelimit/seed.go` | 修改：追加 grpc phase 规则 |
| `internal/scaffold/test/ratelimit/run.go` | 修改：追加 --grpc 支持 |
| `internal/scaffold/shared/container.go` | 可能需要微调 vegeta 对 workspace 的支持 |
| `internal/scaffold/mono/testdata/` | 更新 golden 快照 |

### Error Handling

- PostgreSQL 连接失败：提示检查 `docker compose up -d`
- `rate_limit_rules` 表不存在：提示先运行 `make sqlc`
- gRPC 规则中心未启动：seed 命令不依赖 gRPC 服务（直接写库），run 命令的 grpcurl 会报错并提示
- grpcurl 不可用：提示安装或改用 docker compose 内置方式

### Testing

- **单元测试:** handler CRUD 逻辑（mock repository），限流中间件（mock resolver）
- **Golden 测试:** `go test ./internal/scaffold/mono/... -update-golden`
- **冒烟测试:** `./scripts/smoke.sh`
- **E2E 验证:** grpcurl 调用 rule-center 验证规则返回

### Out of Scope

- 规则中心主动推送（pull 模型已满足需求）
- 多实例规则中心高可用
- gRPC 限流中间件的完整生产级实现（本次做骨架）
- Kitex 以外的 RPC 框架适配
