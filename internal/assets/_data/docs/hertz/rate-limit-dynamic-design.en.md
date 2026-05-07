# Dynamic Rate-Limit Design for the Hertz Template

Audience: `ncgo` maintainers and AI agents that read or modify the
`internal/assets/_data/hertz/` template tree. This document describes the
goals, constraints, architecture, and implementation boundaries of dynamic
rate limiting in the Hertz template family.

For the broader template design, see [`design-doc.en.md`](./design-doc.en.md).

For the Chinese version of this topic, see
[`rate-limit-dynamic-design.zh-CN.md`](./rate-limit-dynamic-design.zh-CN.md).

## 1. Background

Today the Hertz template primarily relies on static `rate_limit` settings under
`conf/<env>/conf.yaml`. That works for simple cases, but it falls short when:

- Operations platforms must publish rules dynamically.
- Limits must be scoped at the API level.
- Combined dimensions such as `ak + path` are required.
- Local config fallback must remain available when a remote rule source fails.

Because of that, the template needs a dynamic rate-limit model built around
"dynamic rule source + local cache + config fallback".

## 2. Goals and Non-Goals

### 2.1 Goals

- Support dynamic rule lookup from a **gRPC** service.
- Support rule lookup through a **database hook**.
- Cache dynamic rule lookup results in local process memory.
- Fall back to config-file rules when a dynamic lookup returns **not found**.
- Fall back to local rules on dynamic-source **errors** when configured to do so.
- Support API-level dimensions, with `ak_path` as the first-class priority.
- Support the `fixed_window` strategy while keeping `token_bucket` compatibility.
- Keep existing `memory` / `redis` backends for rate-limit state storage.

### 2.2 Non-Goals

- No built-in active push from a centralized rule center.
- No unified subscription bus for rules.
- No built-in business-specific table schema or SQL.
- No complex multi-dimensional priority engine in the template.
- No requirement that every project adopt a dynamic rule source; `config` must
  remain usable by itself.

## 3. Design Overview

Dynamic rate limiting is split into three layers:

1. **Rule source layer**: decides where rules come from; supports `config`,
   `grpc`, and `database`.
2. **Rule cache layer**: caches dynamic rule lookup results so every request does
   not hit a remote service or database.
3. **Enforcement layer**: applies rate limiting based on the final resolved rule.

In this model:

- The `grpc` source is built into the template.
- The `database` source is exposed through a template hook/interface, while the
  actual query logic is implemented by the business project.
- `config` remains the final fallback source.

## 4. Rule Priority and Resolution Order

### 4.1 `source.type = config`

Use config-file rules directly.

### 4.2 `source.type = grpc`

Resolve in this order:

1. Check the local rule cache first.
2. On cache miss, issue a gRPC lookup.
3. If gRPC **finds** a rule, use the remote rule directly.
4. If gRPC returns **not found**, fall back to config-file rules.
5. If gRPC returns an **error**, decide whether to fall back based on
   `fallback_on_error`.

### 4.3 `source.type = database`

Resolve in this order:

1. Check the local rule cache first.
2. On cache miss, call the database hook.
3. If the hook **finds** a rule, use the database rule directly.
4. If the hook returns **not found**, fall back to config-file rules.
5. If the hook returns an **error**, decide whether to fall back based on
   `fallback_on_error`.

## 5. Rule Model

### 5.1 Supported Strategies

- `fixed_window`: a good fit for common operational rules like
  "at most N requests within a fixed time window".
- `token_bucket`: retained for compatibility with the current template and for
  traffic-shaping scenarios.

### 5.2 Supported Dimensions

Recommended `key_by` values:

- `ip`
- `ak`
- `user_uuid`
- `ak_user_uuid`
- `path`
- `method_path`
- `ak_path`
- `ak_method_path`

This design prioritizes `ak_path` because it directly covers the core use case:
"limit a specific API for a specific app key".

## 6. Configuration Design

Recommended additions under `rate_limit`:

- `source.type`: `config` / `grpc` / `database`
- `source.cache_ttl_seconds`: TTL for dynamic-rule cache entries
- `source.fallback_on_error`: whether local config should be used when the
  dynamic source errors
- `grpc.target`: rule-center gRPC address
- `grpc.timeout_milliseconds`: gRPC lookup timeout
- `grpc.auth_header` / `grpc.auth_token`: optional auth parameters
- `grpc.service_name`: current service name
- `database.query_timeout_milliseconds`: database-hook lookup timeout

The phase config should still keep `pre_auth` and `post_auth`, but it should be
expanded to contain:

- `enabled`
- `default_rule`
- `rules`

Where:

- `default_rule` is the phase-level fallback rule.
- `rules` is the set of local fine-grained matching rules.

Recommended local match fields:

- `app_key`
- `method`
- `path`
- `path_prefix`

Recommended matching priority:

1. `app_key + method + path`
2. `app_key + path`
3. `method + path`
4. `path`
5. `path_prefix`
6. `default_rule`

## 7. gRPC Rule Source

The template includes a built-in gRPC rule-source implementation. Its job is to:

- Build query parameters from request context.
- Issue a gRPC lookup with timeout control.
- Map the response into a unified rule shape.
- Distinguish among **found**, **not found**, and **error**.

The minimum recommended query fields are:

- `service`
- `phase`
- `app_key`
- `method`
- `path`
- `user_uuid`
- `client_ip`
- `request_id`

The gRPC response must be able to clearly express:

- **Found rule**: use the remote rule directly.
- **Not found**: fall back to config-file rules.
- **Lookup error**: decide whether to fall back based on `fallback_on_error`.

## 8. Database Hook

Because rule-table schemas vary significantly across projects, the template must
not embed concrete SQL or business table models. For that reason, the database
mode is exposed as a hook.

The template side should only provide:

- The database rule-query interface definition
- The invocation entry point
- The unified return shape
- Shared cache and fallback logic

The business project implements:

- Table design
- Repository / DAO query logic
- Rule mapping
- Hook registration and injection

The database hook must also distinguish:

- Found rule
- Not found
- Query error

## 9. Cache Design

The cache stores **dynamic rule lookup results**, not enforcement counters. Those
two responsibilities must stay separate.

Recommended cache key fields:

- `phase`
- `app_key`
- `method`
- `path`

If phase one only prioritizes `ak + path`, the key can start with
`phase + app_key + path`.

Recommended cached content:

- Matched rules
- Empty results (negative cache)
- Optional metadata (source, version)

Recommended defaults:

- `cache_ttl_seconds = 60`
- Support concurrent miss coalescing
- Support negative caching

## 10. Cache Invalidation and Rule-Change Propagation

The rule cache should not rely on a single mechanism only. The recommended model
is **active invalidation + TTL fallback**.

### 10.1 Behavior with TTL Only

If the system only uses TTL caching, rule updates and deletions take effect like
this:

- **Rule update**: old cached data stays active until the TTL expires; the next
  request reloads the new rule.
- **Rule deletion**: old cached data also stays active until expiry; once it
  expires, a miss falls back to config-file rules.

This mode is the simplest to implement, but rule changes are not immediate.

### 10.2 Recommended Mode: Active Invalidation + TTL Fallback

When a rule is updated or deleted, the recommended behavior is to emit a
"cache invalidation" event. Service instances that receive it delete the local
cache entry. If some instance misses the event, TTL still repairs stale data.

Benefits of this model:

- Rule changes usually take effect quickly.
- Notification failures do not create permanently stale data.
- It balances timeliness and operational robustness.

### 10.3 Update and Delete Scenarios

#### Updating a Rule

For example, an `ak + path` rule changes from "100 requests per 60 seconds" to
"20 requests per 60 seconds":

- **TTL mode**: the old rule remains active until cache expiry.
- **Active invalidation mode**: the cache entry is deleted and the next request
  reloads the new rule.

#### Deleting a Rule

For example, deleting a dedicated `ak + path` rule:

- **TTL mode**: the old cache remains until expiry, then a miss falls back to
  config rules.
- **Active invalidation mode**: the cache entry is deleted immediately and the
  next request falls back to config rules after re-querying.

For explicit remote misses or deleted rules, a short-lived negative cache entry
is recommended to avoid hammering the dynamic source under high concurrency.

### 10.4 Recommended Propagation for gRPC and Database

#### gRPC

Prefer one of the following:

- **gRPC stream invalidation events**: instances subscribe to a rule-change
  stream and delete cache entries precisely.
- **MQ / event-bus broadcast**: the rule center broadcasts invalidation events
  via Kafka, Redis Pub/Sub, NATS, or similar infrastructure.

#### Database

Databases typically do not push rule-change notifications directly to business
processes. Recommended options:

- **TTL only**: simplest, but less real time.
- **Emit invalidation events after DB writes**: when the operations platform
  updates the database, it also broadcasts invalidation messages.
- **Poll version / updated_at**: services periodically detect rule-version
  changes and clear affected cache entries.

### 10.5 Recommended Invalidation Granularity

Invalidation events should support at least:

- **Precise invalidation**: remove one cache entry by
  `service + phase + app_key + method + path`.
- **Prefix invalidation**: clear a group by `service + phase`.
- **Full flush**: clear the entire rule cache for emergency handling.

### 10.6 Recommended Rollout Path

Recommended phased rollout:

- **V1**: start with TTL cache only; changes take effect on expiry.
- **V2**: add active invalidation so updates/deletions converge faster.
- **Long-term**: active invalidation speeds convergence; TTL guarantees
  eventual consistency.

## 11. Enforcement Design

The rate-limit state store should continue using the existing backends:

- `memory`
- `redis`

Notes:

- Rules may be cached locally.
- In multi-instance deployments, counters usually belong in Redis; otherwise the
  quota is split across replicas.

Recommended `fixed_window` implementation:

- `memory`: track window start time and per-window count.
- `redis`: use a Lua script or equivalent atomic operations for counter + TTL.

`token_bucket` should remain available for compatibility with the current
template behavior.

## 12. Middleware Execution Flow

For each request entering the rate-limit middleware, the recommended flow is:

1. Check the global and phase-level enable flags.
2. Check whether the request hits `skip_paths`.
3. Extract request context: `app_key`, `method`, `path`, `user_uuid`, `ip`.
4. Resolve rules according to `source.type`.
5. If a dynamic rule is found, use it.
6. If no dynamic rule is found, fall back to local config rules.
7. If the dynamic source errors, decide using `fallback_on_error` and
   `fail_open`.
8. Enforce rate limiting using the final rule.
9. Return the result: pass through or `10200 rate_limited`.

## 13. Template Changes

This design mainly affects the following generated template outputs:

- `conf/dev/conf.yaml`
- `internal/base/conf/conf.go`
- `internal/base/server/server.go`
- `internal/pkg/middleware/rate_limit.go`
- `internal/pkg/middleware/rate_limit_test.go`

The following docs should also be kept in sync:

- `design-doc.zh-CN.md`
- `design-doc.en.md`

## 14. Code-Organization Guidance

To avoid putting middleware orchestration, rule resolution, gRPC calls,
database hooks, and cache logic all into `internal/pkg/middleware/rate_limit.go`,
the generated project should add a dedicated rule-resolution package.

### 14.1 Recommended Directory

Add:

- `internal/pkg/ratelimit/`

This package owns the dynamic rate-limit rule-resolution domain.

### 14.2 Recommended Responsibility Split

Recommended file split:

- `internal/pkg/ratelimit/types.go`
  - shared models such as `lookup`, `resolvedRule`, and `resolveResult`
- `internal/pkg/ratelimit/source.go`
  - unified rule-source interfaces and abstractions
- `internal/pkg/ratelimit/cache.go`
  - shared cache layer, for example `CachedRuleSource`
- `internal/pkg/ratelimit/config_source.go`
  - local config matching and fallback logic
- `internal/pkg/ratelimit/grpc_source.go`
  - gRPC dynamic rule lookup
- `internal/pkg/ratelimit/database_source.go`
  - database hook wrapper and entry point
- `internal/pkg/ratelimit/resolver.go`
  - unified resolution entry for "dynamic first + config fallback"

### 14.3 Cache Placement Guidance

The gRPC and database cache layer should not be embedded separately inside each
implementation. It should live as a shared decorator in
`internal/pkg/ratelimit/cache.go`.

Why:

- What is cached is the **rule lookup result**, not a concrete gRPC client or
  database connection.
- gRPC and database rule sources need the same caching semantics.
- A shared layer centralizes TTL, negative cache, and concurrent miss merging.

Recommended relationships:

- `CachedRuleSource`
  - wraps `GRPCRuleSource`
  - or wraps `DatabaseRuleSource`
- `Resolver`
  - prefers the dynamic source
  - falls back to `ConfigRuleSource` on miss or error

### 14.4 Boundary Between Middleware and Resolver

`internal/pkg/middleware/rate_limit.go` should only keep these responsibilities:

- Check whether rate limiting is enabled
- Check whether the request matches `skip_paths`
- Extract request context
- Call `ratelimit.Resolver` to get the final rule
- Run the concrete enforcement logic
- Map rejections to the response code

It should not directly own:

- gRPC query implementation
- database-hook implementation
- rule-cache implementation
- complex local rule matching logic

### 14.5 Server Assembly Location

Rule-resolution components should be initialized and assembled in
`internal/base/server/server.go`.

Recommended startup flow:

1. Read `cfg.RateLimit`.
2. Create the dynamic rule source according to `source.type`.
3. Wrap the dynamic source with the shared cache layer.
4. Create the local config rule source.
5. Assemble the unified `Resolver`.
6. Pass the `Resolver` into the rate-limit middleware.

This keeps the cache and source as **process-level singletons** rather than
creating them per request.

### 14.6 Placement Boundary for gRPC and Database

#### gRPC

Split it into two layers:

- **Connection or client construction**: may live in `internal/base/data` or
  startup assembly code.
- **Rule-query semantics**: should live in
  `internal/pkg/ratelimit/grpc_source.go`.

#### Database

Also split it into two layers:

- **Business query implementation**: lives in `internal/repository`,
  `internal/base/data`, or another business-specific package.
- **Hook abstraction and invocation**: lives in
  `internal/pkg/ratelimit/database_source.go`.

### 14.7 Summary

Recommended organization:

- `internal/pkg/ratelimit/`: rule resolution, rule cache, source abstractions,
  resolver
- `internal/pkg/middleware/rate_limit.go`: middleware entry and orchestration
- `internal/base/server/server.go`: startup-time resolver assembly
- `internal/base/data`: optional home for gRPC clients or low-level dependency
  construction
- business repository / data layer: concrete database-hook implementation

This organization lowers coupling and leaves room for additional rule sources or
future invalidation mechanisms.

## 15. Integration Examples

The examples below show how a generated project can connect a real gRPC rule
client or database hook to `ratelimit.Resolver`. These are intentionally
skeleton-style examples; the exact fields and dependencies can be adjusted by
the consuming project.

### 15.1 Server Wiring Example

Assemble the resolver in `internal/base/server/server.go`:

```go
var rlOpts ratelimit.Options

if cfg.RateLimit.Source.Type == "grpc" {
    rlOpts.GRPC = newDynamicRuleGRPCClient(cfg)
}
if cfg.RateLimit.Source.Type == "database" {
    rlOpts.Database = repository.NewRateLimitRuleHook(...)
}

resolver := ratelimit.NewResolver(cfg.RateLimit, rlOpts)
```

Recommended principles:

- Create the `resolver` once at startup and reuse it as a process-level
  singleton.
- When `source.type=config`, `Options` may remain empty.
- When `source.type=grpc` or `database` is configured but no real implementation
  is injected, the current template safely falls back to local config rules.

### 15.2 gRPC Client Adapter Example

The template-level gRPC interface is:

```go
type GRPCClient interface {
    ResolveRateLimitRule(ctx context.Context, lookup Lookup) (*conf.RateLimitRuleConfig, bool, error)
}
```

The business project can adapt a real protobuf client like this:

```go
type dynamicRuleGRPCClient struct {
    cli pb.RuleServiceClient
}

func (c *dynamicRuleGRPCClient) ResolveRateLimitRule(ctx context.Context, lookup ratelimit.Lookup) (*conf.RateLimitRuleConfig, bool, error) {
    resp, err := c.cli.GetRule(ctx, &pb.GetRuleRequest{
        Service:  "order-api",
        Phase:    lookup.Phase,
        AppKey:   lookup.AppKey,
        Method:   lookup.Method,
        Path:     lookup.Path,
        UserUuid: lookup.UserUUID,
        ClientIp: lookup.ClientIP,
    })
    if err != nil {
        return nil, false, err
    }
    if !resp.Found {
        return nil, false, nil
    }
    return &conf.RateLimitRuleConfig{
        Enabled:       resp.Rule.Enabled,
        KeyBy:         resp.Rule.KeyBy,
        Strategy:      resp.Rule.Strategy,
        WindowSeconds: int(resp.Rule.WindowSeconds),
        MaxRequests:   int(resp.Rule.MaxRequests),
    }, true, nil
}
```

Recommended return contract:

- remote rule **found** → `rule, true, nil`
- remote rule **not found** → `nil, false, nil`
- remote query **error** → `nil, false, err`

That contract allows the `Resolver` to reliably distinguish between fallback and
hard failure.

### 15.3 Database Hook Adapter Example

The template-level database hook interface is:

```go
type DatabaseHook interface {
    ResolveRateLimitRule(ctx context.Context, lookup Lookup) (*conf.RateLimitRuleConfig, bool, error)
}
```

The business project can implement it under `internal/repository` or another
appropriate package:

```go
type RateLimitRuleHook struct {
    repo *RuleRepository
}

func (h *RateLimitRuleHook) ResolveRateLimitRule(ctx context.Context, lookup ratelimit.Lookup) (*conf.RateLimitRuleConfig, bool, error) {
    rule, err := h.repo.FindRule(ctx, lookup.Phase, lookup.AppKey, lookup.Method, lookup.Path)
    if err != nil {
        return nil, false, err
    }
    if rule == nil {
        return nil, false, nil
    }
    return &conf.RateLimitRuleConfig{
        Enabled:       true,
        KeyBy:         []string{"ak_path"},
        Strategy:      "fixed_window",
        WindowSeconds: rule.WindowSeconds,
        MaxRequests:   rule.MaxRequests,
    }, true, nil
}
```

The mapping from business tables into the unified rule structure should stay
inside the hook rather than leaking repository details into middleware or the
resolver.

### 15.4 Recommended Startup Strategy

Recommended integration path:

- **V1**: start with `config` + local fallback so the feature works end to end.
- **V1.1**: inject a real `grpc client` or `database hook` implementation.
- **V2**: add active invalidation based on the cache-invalidation section above.

If the project does not yet have a real gRPC or database rule source, it can
stay on `source.type=config` without losing the rest of the rate-limit feature.

## 16. Testing Recommendations

Recommended additional or updated tests:

- Config tests: `source.type`, gRPC/database config validity, strategy parameter
  validity.
- Rule-resolution tests: gRPC/database hit, miss, error, and fallback behavior.
- Cache tests: cache hit, negative cache, TTL expiry, concurrent miss merging.
- Enforcement tests: `ak_path` key generation, fixed-window behavior, dynamic
  rules taking priority over local rules.

## 17. Recommended Defaults

Recommended default configuration:

- `source.type = grpc`
- `source.cache_ttl_seconds = 60`
- `source.fallback_on_error = true`
- `strategy = fixed_window`
- `key_by = ["ak_path", "ip"]`

These defaults balance dynamic-rule capability, runtime stability, and local
fallback behavior.

## 18. Risks and Notes

- **Cache freshness**: with TTL-based caching, rule changes do not take effect
  instantly.
- **Multi-instance deployment**: production systems should usually store
  counters in Redis.
- **Path normalization**: prefer router template paths when available; fall back
  to raw request paths otherwise.

## 19. Conclusion

This design adopts:

- built-in gRPC rule-source support
- database rule-source extensibility through a hook
- config-file rules as the final fallback source
- local in-memory caching for dynamic rule results
- `fixed_window` support while retaining `token_bucket`
- continued use of `memory` / `redis` for enforcement state storage

The result balances dynamic-rule capability, template generality, runtime
stability, and compatibility with the existing Hertz template family.
