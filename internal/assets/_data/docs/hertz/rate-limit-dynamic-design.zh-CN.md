# Hertz 模板动态限流设计

读者: `ncgo` 维护者及读取或修改 `internal/assets/_data/hertz/` 模板树的
AI Agent。本文描述 Hertz 模板中动态限流能力的目标、约束、架构与落地边界。

相关总设计见
[`design-doc.zh-CN.md`](./design-doc.zh-CN.md)。

英文版专题见
[`rate-limit-dynamic-design.en.md`](./rate-limit-dynamic-design.en.md)。

## 1. 背景

当前 Hertz 模板中的限流能力主要依赖 `conf/<env>/conf.yaml` 中的静态
`rate_limit` 配置。该方案适合简单场景,但在以下需求下存在不足:

- 需要由运营平台动态下发限流规则。
- 需要按接口维度进行精细化限流。
- 需要支持 `ak + path` 这类组合维度。
- 需要在远程规则源异常时保留配置兜底能力。

因此,需要在模板中引入“动态规则源 + 本地缓存 + 配置兜底”的动态限流方案。

## 2. 目标与非目标

### 2.1 目标

- 支持从 **gRPC** 服务动态获取限流规则。
- 支持通过 **database hook** 从数据库查询限流规则。
- 支持将动态规则结果缓存到服务本地内存。
- 支持动态规则**未命中**时回退到配置文件规则。
- 支持动态规则源**异常**时按配置决定是否回退本地规则。
- 支持接口级维度,优先支持 `ak_path`。
- 支持 `fixed_window` 限流策略,同时保留 `token_bucket` 兼容能力。
- 保留现有 `memory` / `redis` 作为限流状态存储后端。

### 2.2 非目标

- 不实现规则中心主动推送。
- 不实现统一规则订阅总线。
- 不在模板中内置具体业务数据库表结构与 SQL。
- 不实现复杂多维规则优先级引擎。
- 不强制所有项目接入动态规则源,`config` 模式仍可独立使用。

## 3. 设计总览

动态限流能力分为三层:

1. **规则来源层**: 决定规则从哪里来,支持 `config`、`grpc`、`database`。
2. **规则缓存层**: 缓存动态规则查询结果,避免每个请求都访问远程服务或数据库。
3. **限流执行层**: 根据最终解析出的规则执行限流。

其中:

- `grpc` 规则源由模板内置实现。
- `database` 规则源由模板定义 hook/interface,具体查询由业务项目实现。
- `config` 始终作为最终兜底规则源。

## 4. 规则优先级与解析顺序

### 4.1 `source.type = config`

直接使用配置文件规则。

### 4.2 `source.type = grpc`

按如下顺序解析:

1. 先查本地规则缓存。
2. 缓存未命中时发起 gRPC 查询。
3. 若 gRPC **命中**规则,直接使用远程规则。
4. 若 gRPC **未命中**规则,回退配置文件规则。
5. 若 gRPC **异常**,根据 `fallback_on_error` 决定是否回退配置文件规则。

### 4.3 `source.type = database`

按如下顺序解析:

1. 先查本地规则缓存。
2. 缓存未命中时调用 database hook。
3. 若 hook **命中**规则,直接使用数据库规则。
4. 若 hook **未命中**规则,回退配置文件规则。
5. 若 hook **异常**,根据 `fallback_on_error` 决定是否回退配置文件规则。

## 5. 规则模型

### 5.1 支持的策略

- `fixed_window`: 适合运营平台常见的“固定时间窗口内最多 N 次”规则。
- `token_bucket`: 保留现有模板能力,用于兼容已有配置和流量整形场景。

### 5.2 支持的维度

建议支持以下 `key_by`:

- `ip`
- `ak`
- `user_uuid`
- `ak_user_uuid`
- `path`
- `method_path`
- `ak_path`
- `ak_method_path`

本方案优先支持 `ak_path`,用于满足“某个 app key 对某个接口限流”的核心诉求。

## 6. 配置设计

建议在 `rate_limit` 下扩展以下配置:

- `source.type`: `config` / `grpc` / `database`
- `source.cache_ttl_seconds`: 动态规则缓存 TTL
- `source.fallback_on_error`: 动态规则源异常时是否回退本地配置
- `grpc.target`: 规则中心 gRPC 地址
- `grpc.timeout_milliseconds`: gRPC 查询超时
- `grpc.auth_header` / `grpc.auth_token`: 可选鉴权参数
- `grpc.service_name`: 当前服务名
- `database.query_timeout_milliseconds`: database hook 查询超时

阶段配置仍保留 `pre_auth` 和 `post_auth`,但建议扩展为:

- `enabled`
- `default_rule`
- `rules`

其中:

- `default_rule` 表示阶段默认兜底规则。
- `rules` 表示本地细粒度匹配规则集合。

本地规则建议支持以下匹配条件:

- `app_key`
- `method`
- `path`
- `path_prefix`

建议匹配优先级如下:

1. `app_key + method + path`
2. `app_key + path`
3. `method + path`
4. `path`
5. `path_prefix`
6. `default_rule`

## 7. gRPC 规则源

模板内置 gRPC 规则源实现,职责包括:

- 根据请求上下文构造查询参数。
- 发起 gRPC 查询并设置超时。
- 将返回结果映射为统一规则结构。
- 区分“命中”“未命中”“异常”三种结果。

建议查询条件至少包含:

- `service`
- `phase`
- `app_key`
- `method`
- `path`
- `user_uuid`
- `client_ip`
- `request_id`

gRPC 返回必须能明确表达:

- **命中规则**: 直接使用远程规则。
- **未命中规则**: 回退配置文件规则。
- **查询异常**: 根据 `fallback_on_error` 决定是否回退配置文件规则。

## 8. Database Hook

不同项目的规则表结构差异较大,模板不应内置具体 SQL 与表模型,因此
database 模式采用 hook 扩展。

模板侧仅提供:

- database 规则查询接口定义
- 查询调用入口
- 统一返回结构
- 缓存与 fallback 相关通用逻辑

业务项目自行实现:

- 数据表设计
- repository / DAO 查询逻辑
- 规则结构映射
- hook 注册与注入

database hook 同样需要区分:

- 命中规则
- 未命中规则
- 查询异常

## 9. 缓存设计

缓存的是**动态规则查询结果**,不是限流计数状态。两者职责不同,不能混用。

建议缓存 key 至少包含:

- `phase`
- `app_key`
- `method`
- `path`

若一期仅优先支持 `ak + path`,可先采用 `phase + app_key + path`。

建议缓存内容包括:

- 命中规则
- 空结果(negative cache)
- 可选元信息(来源、版本号)

建议默认:

- `cache_ttl_seconds = 60`
- 支持并发 miss 合并
- 支持空结果缓存

## 10. 缓存失效与规则变更传播

规则缓存不建议只依赖单一机制,推荐采用 **主动失效 + TTL 兜底** 的组合方案。

### 10.1 仅使用 TTL 的行为

若系统仅配置 TTL 缓存,则规则更新或删除后的生效方式如下:

- **更新规则**: 旧缓存会持续生效,直到 TTL 到期后,下一次请求重新查询并加载新规则。
- **删除规则**: 旧缓存同样会持续到 TTL 到期;到期后再次查询若未命中,则回退到配置文件规则。

该模式实现最简单,但缺点是规则变更不会立即生效。

### 10.2 推荐方案: 主动失效 + TTL 兜底

推荐在规则更新或删除时主动发送“缓存失效事件”,服务实例收到事件后删除对应本地缓存;
若某个实例未收到事件,TTL 仍可在到期后自动修正脏缓存。

该方案具备以下特点:

- 正常情况下,规则更新或删除后可快速生效。
- 通知链路异常时,不会造成永久脏数据。
- 兼顾实时性与系统稳健性。

### 10.3 更新与删除场景

#### 更新规则

例如某条 `ak + path` 规则从“60 秒 100 次”调整为“60 秒 20 次”:

- **TTL 模式**: 旧规则继续生效直到缓存过期。
- **主动失效模式**: 收到失效通知后删除缓存,下一次请求立即重新加载新规则。

#### 删除规则

例如删除某个 `ak + path` 的专属规则:

- **TTL 模式**: 旧缓存保留到过期,过期后重新查询,未命中则回退配置规则。
- **主动失效模式**: 收到删除通知后删除缓存,下一次请求重新查询并直接回退配置规则。

对于“远程明确未命中”或“规则已删除”的结果,建议写入短 TTL 的空结果缓存,
以避免高并发场景下重复打远程规则源。

### 10.4 gRPC 与 database 的变更传播建议

#### gRPC

推荐优先支持以下方式之一:

- **gRPC Stream 推送失效事件**: 服务实例订阅规则变更流,按事件精准删除缓存。
- **MQ / 事件总线广播**: 规则中心通过 Kafka、Redis Pub/Sub、NATS 等广播失效事件。

#### database

database 本身通常不直接向业务进程推送变更事件,建议采用以下方式之一:

- **仅使用 TTL**: 简单但实时性较弱。
- **写库后同步发送失效事件**: 运营平台更新数据库后,同时广播缓存失效消息。
- **轮询版本号 / 更新时间**: 服务周期性检查规则版本变化并主动清理缓存。

### 10.5 建议支持的失效粒度

建议缓存失效事件至少支持以下粒度:

- **精准失效**: 按 `service + phase + app_key + method + path` 删除单条缓存。
- **前缀失效**: 按 `service + phase` 清理一组规则缓存。
- **全量清空**: 在紧急场景下清空整个规则缓存。

### 10.6 推荐结论

本方案建议按版本分阶段落地:

- **V1**: 先实现 TTL 缓存,规则变更在缓存过期后生效。
- **V2**: 增加主动失效通知机制,使更新/删除更快生效。
- **长期方案**: 主动失效负责加速收敛,TTL 负责最终一致性兜底。

## 11. 限流执行设计

限流状态存储继续沿用现有后端:

- `memory`
- `redis`

说明:

- 规则可以本地缓存。
- 但多实例部署场景下,限流计数通常应放在 Redis,否则配额会在副本之间分裂。

`fixed_window` 建议实现为:

- `memory`: 维护窗口起点与窗口内计数。
- `redis`: 使用 Lua 脚本或等价原子操作维护窗口计数与过期时间。

`token_bucket` 继续保留当前实现,用于兼容已有配置。

## 12. 中间件执行流程

每个请求进入限流中间件后,建议按如下步骤执行:

1. 判断总开关与阶段开关。
2. 判断是否命中 `skip_paths`。
3. 提取请求上下文: `app_key`、`method`、`path`、`user_uuid`、`ip`。
4. 根据 `source.type` 查询规则。
5. 若动态规则命中,使用动态规则。
6. 若动态规则未命中,回退本地配置规则。
7. 若动态规则源异常,根据 `fallback_on_error` 与 `fail_open` 决策。
8. 按最终规则执行限流。
9. 返回结果: 放行或返回 `10200 rate_limited`。

## 13. 模板改造范围

本方案主要影响以下模板生成内容:

- `conf/dev/conf.yaml`
- `internal/base/conf/conf.go`
- `internal/base/server/server.go`
- `internal/pkg/middleware/rate_limit.go`
- `internal/pkg/middleware/rate_limit_test.go`

建议同步更新文档:

- `design-doc.zh-CN.md`
- `design-doc.en.md`

## 14. 代码组织建议

为避免把“中间件编排”“规则解析”“gRPC 调用”“database hook”“缓存逻辑”全部堆在
`internal/pkg/middleware/rate_limit.go` 中,建议在生成项目中增加独立的限流规则解析包。

### 14.1 推荐目录

建议新增:

- `internal/pkg/ratelimit/`

用于承载动态限流规则解析域相关代码。

### 14.2 推荐职责拆分

建议按下述方式组织代码:

- `internal/pkg/ratelimit/types.go`
  - 定义 `lookup`、`resolvedRule`、`resolveResult` 等通用模型。
- `internal/pkg/ratelimit/source.go`
  - 定义统一规则源接口与通用抽象。
- `internal/pkg/ratelimit/cache.go`
  - 定义共享缓存层,例如 `CachedRuleSource`。
- `internal/pkg/ratelimit/config_source.go`
  - 本地配置规则匹配与兜底逻辑。
- `internal/pkg/ratelimit/grpc_source.go`
  - gRPC 动态规则查询实现。
- `internal/pkg/ratelimit/database_source.go`
  - database hook 包装与调用入口。
- `internal/pkg/ratelimit/resolver.go`
  - 统一解析入口,负责“动态优先 + 配置兜底”。

### 14.3 缓存层放置建议

gRPC 与 database 的缓存层不建议分别内嵌在各自实现内部,而应作为共享装饰器放在
`internal/pkg/ratelimit/cache.go` 中。

原因如下:

- 缓存的对象是“规则查询结果”,而不是某个具体 gRPC client 或 database 连接。
- gRPC 与 database 规则源都需要相同的缓存语义,适合复用同一层实现。
- 共享缓存层便于统一实现 TTL、空结果缓存与并发 miss 合并。

推荐关系如下:

- `CachedRuleSource`
  - 包装 `GRPCRuleSource`
  - 或包装 `DatabaseRuleSource`
- `Resolver`
  - 优先调用动态规则源
  - 未命中或异常时回退 `ConfigRuleSource`

### 14.4 中间件与 resolver 的边界

建议 `internal/pkg/middleware/rate_limit.go` 仅保留以下职责:

- 判断是否启用限流
- 判断是否命中 `skip_paths`
- 提取请求上下文
- 调用 `ratelimit.Resolver` 获取最终规则
- 调用具体限流执行逻辑
- 将拒绝结果映射为响应码

不建议在 middleware 文件中直接承载:

- gRPC 查询实现
- database hook 实现
- 规则缓存实现
- 复杂本地规则匹配逻辑

### 14.5 server 组装位置

建议在 `internal/base/server/server.go` 中完成规则解析组件的初始化与装配。

启动阶段建议流程如下:

1. 读取 `cfg.RateLimit`。
2. 根据 `source.type` 创建动态规则源。
3. 为动态规则源包裹共享缓存层。
4. 创建本地配置规则源。
5. 组装统一 `Resolver`。
6. 将 `Resolver` 传给限流中间件使用。

这样可以保证缓存与规则源是**进程级单例**,而不是在每个请求里重复创建。

### 14.6 gRPC 与 database 的放置边界

#### gRPC

建议区分两层:

- **连接或 client 构造**: 可放在 `internal/base/data` 或启动装配逻辑中。
- **规则查询语义**: 放在 `internal/pkg/ratelimit/grpc_source.go`。

#### database

建议区分两层:

- **业务查询实现**: 由业务项目放在 `internal/repository`、`internal/base/data` 或其他合适位置。
- **hook 抽象与调用**: 放在 `internal/pkg/ratelimit/database_source.go`。

### 14.7 总结

推荐的代码组织方式为:

- `internal/pkg/ratelimit/`: 规则解析、规则缓存、source 抽象、resolver
- `internal/pkg/middleware/rate_limit.go`: 中间件入口与执行编排
- `internal/base/server/server.go`: 启动时装配 resolver
- `internal/base/data`: 可选承载 gRPC client 或底层依赖构造
- 业务 repository / data 层: 实现 database hook 的具体查询逻辑

该组织方式能够降低耦合,并为后续扩展更多规则源或更多缓存失效机制预留空间。

## 15. 接入示例

以下示例用于说明生成项目后如何把真实的 gRPC 规则客户端或 database hook 接到
`ratelimit.Resolver` 上。示例偏骨架性质,具体字段与依赖可按业务项目调整。

### 15.1 server wiring 示例

建议在 `internal/base/server/server.go` 中集中组装 resolver:

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

建议保持以下原则:

- `resolver` 在服务启动时创建一次,作为进程级单例复用。
- 当 `source.type=config` 时,`Options` 可为空。
- 当 `source.type=grpc` 或 `database` 但未注入真实实现时,当前模板会自动回退到本地配置规则。

### 15.2 gRPC client 适配示例

模板约定的 gRPC 接口为:

```go
type GRPCClient interface {
    ResolveRateLimitRule(ctx context.Context, lookup Lookup) (*conf.RateLimitRuleConfig, bool, error)
}
```

业务项目可以写一个适配器,把真实的 pb client 映射到该接口:

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

推荐约定如下:

- 远程规则**命中**时返回 `rule, true, nil`
- 远程规则**未命中**时返回 `nil, false, nil`
- 远程查询**异常**时返回 `nil, false, err`

这样 `Resolver` 才能正确区分“回退配置”和“直接报错/按 fail_open 处理”。

### 15.3 database hook 适配示例

模板约定的 database hook 接口为:

```go
type DatabaseHook interface {
    ResolveRateLimitRule(ctx context.Context, lookup Lookup) (*conf.RateLimitRuleConfig, bool, error)
}
```

业务项目可在 `internal/repository` 或其他合适目录实现:

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

建议将数据库表到统一规则结构的映射收敛在 hook 内部,而不是把 repository 细节暴露给
middleware 或 resolver。

### 15.4 推荐的启动期策略

建议按以下策略接入:

- **V1**: 优先接 `config` + 本地 fallback 路径,确保功能先可用。
- **V1.1**: 接入真实 `grpc client` 或 `database hook`。
- **V2**: 结合前文“缓存失效与规则变更传播”机制,补主动失效链路。

若业务项目暂时还没有真实的 gRPC 或 database 规则源,可先保持 `source.type=config`,
不影响模板其余限流能力。

## 16. 测试建议

建议新增或补齐以下测试:

- 配置测试: `source.type`、gRPC/database 配置合法性、策略参数合法性。
- 规则解析测试: gRPC/database 命中、未命中、异常与 fallback 行为。
- 缓存测试: 命中缓存、空结果缓存、TTL 过期、并发 miss 合并。
- 限流测试: `ak_path` key 生成、fixed window 行为、动态规则优先于本地规则。

## 17. 默认建议

建议默认配置如下:

- `source.type = grpc`
- `source.cache_ttl_seconds = 60`
- `source.fallback_on_error = true`
- `strategy = fixed_window`
- `key_by = ["ak_path", "ip"]`

该默认值兼顾了动态规则能力、运行时稳定性与本地兜底能力。

## 18. 风险与注意事项

- **缓存实时性**: 启用 TTL 缓存后,规则变更不会瞬时生效。
- **多实例部署**: 生产环境建议使用 Redis 作为限流状态存储。
- **路径标准化**: 建议优先使用路由模板路径,拿不到时再退回原始路径。

## 19. 结论

本方案采用:

- gRPC 规则源内置实现
- database 规则源通过 hook 扩展
- 配置文件始终作为兜底规则源
- 动态规则结果使用本地内存缓存
- 限流执行支持 `fixed_window`,并保留 `token_bucket`
- 限流状态存储继续沿用 `memory` / `redis`

该方案兼顾了动态规则能力、模板通用性、运行时稳定性以及与现有 Hertz 模板
的兼容性。
