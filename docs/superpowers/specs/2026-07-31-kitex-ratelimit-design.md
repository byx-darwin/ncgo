# Kitex RPC 服务限流拦截设计

- **日期**: 2026-07-31
- **状态**: 已批准(brainstorming 完成,待用户复审)
- **工作流**: `wf-2026-07-30-001`(gitflow-workflow, full 模式, Phase 1)
- **关联文档**: `internal/assets/_data/docs/hertz/rate-limit-dynamic-design.zh-CN.md`(Hertz 动态限流设计)

## 1. 背景与动机

ncgo 脚手架已在 Hertz(HTTP)侧完整实现动态限流:

- 中间件 `internal/pkg/middleware/rate_limit.go`:token_bucket + fixed_window 算法,memory(hot LRU)/ redis(Lua 脚本)两种计数后端,超限返回 HTTP 429
- 规则解析 `internal/pkg/ratelimit/resolver.go`:config / grpc / database / rule_center 四种规则源,进程内缓存(TTL),`fallback_on_error` 降级,本地规则评分匹配
- rule-center Kitex 服务(`--preset rule-center`):规则 CRUD + PostgreSQL 存储
- E2E 验证:`ncgo test rate-limit {seed|run|e2e}`(vegeta 压测 + 429 分类)

但 **Kitex RPC 服务自身的限流拦截目前只是 pass-through 占位符**(`internal/assets/_data/kitex/kitex-template/ratelimit_middleware.yaml` 生成的 `RateLimit()` 直接 `return next(ctx, req, resp)`)。占位符注释自述:"Rate-limit enforcement for Kitex services will be added in a follow-up."

做 Kitex 拦截的意义:

1. **服务自保 / 防雪崩**:下游变慢 → 上游 goroutine 堆积 → 级联崩溃。RPC 层限流是熔断前的防洪坝
2. **Hertz 不是唯一调用方**:服务间互调(Kitex→Kitex)、定时任务、内部工具直连 RPC,全部绕过 HTTP 入口限流
3. **扇出放大**:1 次 HTTP 请求可扇出为 N 次 RPC,入口限流看不见放大系数
4. **多调用方公平性**:按 caller 维度限流,防 noisy neighbor 吃光下游容量
5. **边际成本极低**:rule-center、resolver、store、缓存、降级基建全部已实现,Kitex 侧只需中间件适配层

## 2. 目标与非目标

### 2.1 目标

- 为**所有** ncgo 生成的 Kitex 服务提供真实的 RPC 限流拦截能力,经 `ncgo add infra rate-limit` 按需启用
- 复用 Hertz 侧限流基建:resolver 与计数 store **单一事实来源**,hertz 与 kitex 生成同一个 `internal/pkg/ratelimit` 包
- 双轨防护:Kitex 内置 `WithLimit` 静态全局兜底 + 动态规则中间件
- 默认生成即 **shadow 观察模式**(计数真实生效但不拒绝),运营确认后手动切 enforce
- 超限拒绝返回 Kitex BizStatusError **10429** + retry-after 提示
- `ncgo add rule-center` 扩展支持 kitex 服务(本轮纳入)
- 验证:单元测试 + golden 测试 + e2e 压测(e2e 扩展支持 Kitex)

### 2.2 非目标

- 分布式精准协调(redis Lua 单实例语义已足够)
- 规则主动推送 / 缓存主动失效(Hertz 设计文档 §10 方向,hertz/kitex 共同后续)
- 每 RPC 方法独立中间件注册(method 粒度经规则匹配的 `method` 字段实现)
- Hertz 侧行为变化(抽取重构保证 hertz 生成输出逐字节不变)

## 3. 关键决策(已与需求方确认)

| # | 决策点 | 结论 |
|---|--------|------|
| 1 | 拦截范围 | 所有 Kitex 服务,经 `ncgo add infra rate-limit` 交付;rule-center 自身同样适用 |
| 2 | 复用策略 | 从 `hertz/layout.yaml` 抽取 resolver + store 为共享 asset;中间件层框架各异 |
| 3 | 拒绝语义 | BizStatusError 10429(镜像 HTTP 429)+ metainfo retry-after |
| 4 | 验证范围 | 单测 + golden + e2e(e2e 扩展 Kitex RPC 压测) |
| 5 | 默认状态 | 动态轨 enabled + **shadow 模式**(计数不拒绝);静态轨默认关闭 |
| 6 | 实现模型 | **双轨**:Kitex `WithLimit` 静态兜底 + 动态 chain 中间件 |
| 7 | rule-center | 本轮放开 kitex 支持(客户端 asset 框架无关,增量小) |

## 4. 架构总览

### 4.1 双轨拦截模型

```
RPC 请求进入 Kitex server
  │
  ├─ 【静态轨】server.WithLimit(MaxConnections/MaxQPS)     ← 全局粗粒度兜底
  │    conf: rate_limit.static.{max_qps,max_connections}     默认 0 = 不挂载
  │    超限 → kitex 框架层直接拒绝
  │
  └─ 【动态轨】WithMiddleware 链:
       RequestID → AccessLog → Recovery
       → CallerAllowlist            ← 此处之后 caller 身份已就绪
       → middleware.RateLimit(cfg.RateLimit)   ★ 新增
       → RequestTimeout
            │
            ├─ Lookup{Service: registry 名, Method: rpcinfo 方法名,
            │         AppKey: caller 服务名(transmeta), ClientIP: caller 地址,
            │         Phase: "post_auth"}
            ├─ 共享 resolver:动态源(rule_center/grpc/database)→ 缓存(TTL)
            │                 → 失败按 fallback_on_error 回退本地 config 规则
            ├─ 共享 store:memory(hot LRU)| redis(Lua 脚本)
            └─ 判决:
                 允许                    → next(ctx, req, resp)
                 超限 + mode=enforce     → BizStatusError 10429 + metainfo retry-after
                 超限 + mode=shadow      → 计数/日志/打点,放行  ← 默认模式
```

### 4.2 Asset 拓扑(共享抽取)

```
internal/assets/_data/
├── ratelimit/                      ★ 新共享目录(框架无关)
│   ├── resolver.yaml               ← 从 hertz/layout.yaml 抽出
│   ├── resolver_test.yaml
│   ├── store.yaml                  ← 从 hertz rate_limit.go 抽出(memory+redis 计数器)
│   ├── store_test.yaml
│   └── rule_center_client.yaml     ← 从 hertz/optional/rule_center_client.go 抽出
├── hertz/
│   └── layout.yaml                 ← 内联 resolver/store 替换为 include 指令;
│                                     hz 工具永远看不到指令(ncgo 缝合后交付)
└── kitex/
    ├── kitex-template/
    │   ├── ratelimit_middleware.yaml  ← 重写:真实 endpoint.Middleware
    │   ├── server.yaml                ← 新增第二个 wire 标记
    │   ├── conf.yaml / conf_dev.yaml  ← 新增 Mode / Static 字段
    │   └── rpcerror.yaml              ← 新增 RateLimited() 构造器
    └── layout-rulecenter.yaml      ← 引用共享 ratelimit assets
```

**核心原则**:resolver / store / rule-center client 单一事实来源;只有中间件适配层框架各异(hertz `app.HandlerFunc` / kitex `endpoint.Middleware`)。

### 4.3 Hertz layout 的 include 缝合

`hertz/layout.yaml` 是 hz 工具的单文件自定义 layout(`hz new --customize_layout=...`),不支持外部引用。方案:

- 共享片段以与 layout 相同的 `- path:` 条目格式存放于 `internal/assets/_data/ratelimit/*.yaml`
- `layout.yaml` 内原位置放置指令注释,如 `# {{include: ratelimit/resolver}}`
- ncgo 在 scaffold 时(`internal/scaffold/mono/files.go` 写出 `template/layout.yaml` 之前)展开指令为对应片段内容
- hz 工具消费的是已缝合的完整 layout,感知不到指令存在
- hertz golden 测试保证缝合后生成输出**逐字节不变**

kitex 侧无此问题:`layout-rulecenter.yaml` 本就是 `templates:` 列表,直接列出共享片段即可。

## 5. 组件与改动清单

### 5.1 ncgo 仓库(脚手架自身)

| # | 文件 | 改动 |
|---|------|------|
| 1 | `internal/assets/_data/ratelimit/` ★新 | 共享片段:resolver / resolver_test / store / store_test / rule_center_client |
| 2 | `internal/scaffold/mono/files.go` | include 指令展开逻辑(缝合共享片段进 hertz layout) |
| 3 | `internal/assets/_data/hertz/layout.yaml` | 内联 resolver/store 换为 include 指令;hertz 中间件层瘦身为调用共享 store 的适配层 |
| 4 | `internal/assets/_data/hertz/optional/rule_center_client.go` | 删除,内容由共享片段 `ratelimit/rule_center_client.yaml` 取代;`add rule-center` 的 hertz 分支改写共享片段 |
| 5 | `internal/scaffold/infra/infra.go` | 新 `KindRateLimit = "rate_limit"`(**kitex-only**):写 pkg+middleware、更新 conf、astwire 激活两个标记、goGetDeps、next-steps |
| 6 | `kitex-template/server.yaml` | 新增标记 `// ncgo:wire:ratelimit:static-limit` |
| 7 | `kitex-template/ratelimit_middleware.yaml` | 重写占位符 → 真实中间件 + `StaticLimitOption(cfg.Static)` helper(零值返回 nil) |
| 8 | `kitex-template/conf.yaml` + `conf_dev.yaml` | `RateLimitConfig` 增字段:`Mode`(shadow\|enforce,默认 shadow)、`Static{MaxQPS,MaxConnections}`(默认 0) |
| 9 | `kitex-template/rpcerror.yaml` | 新 `RateLimited(retryAfter time.Duration)` → BizStatusError 10429 |
| 10 | `kitex/layout-rulecenter.yaml` | 引用共享 ratelimit 片段 |
| 11 | `internal/scaffold/rulecenter/rulecenter.go` | 放宽 kind 校验(hertz → hertz\|kitex);kitex 分支:写 client + 改 conf,**无需 wire server.go**(见 §6.2) |
| 12 | `internal/scaffold/test/ratelimit/` | e2e 扩展:RPC attacker + kitex 分支(detectProject 已支持) |
| 13 | golden 测试 | infra golden(新 kind)+ rpc golden(middleware 模板重写)+ rulecenter golden(kitex 分支) |
| 14 | 文档 | rate-limit-dynamic-design 增 Kitex 章节;CLI 命令文档;README 更新 |

MCP 侧零额外工作:`ncgo_add_infra` 工具的 kind 为通用字符串,自动支持新 kind。

### 5.2 用户执行命令后得到什么

```
$ ncgo add infra rate-limit [--root .] [--dry-run] [--plan] [--output json]
wrote   internal/pkg/ratelimit/resolver.go
wrote   internal/pkg/ratelimit/store.go
wrote   internal/base/middleware/ratelimit.go
updated conf/dev/conf.yaml
wired   internal/base/server/server.go (middleware + static-limit)

next steps:
  - 配置规则源(conf/dev/conf.yaml: rate_limit.source.type)
  - 观察 shadow 日志 1-2 周,确认后改 mode: enforce
  - (可选) 设置 static.max_qps / max_connections 添加全局兜底
```

## 6. 关键设计细节

### 6.1 Kitex 中间件结构

```go
// internal/base/middleware/ratelimit.go(生成产物示意)
func RateLimit(cfg conf.RateLimitConfig) endpoint.Middleware {
    resolver := ratelimit.NewResolver(cfg, buildOptions(cfg))  // 内部按 source.type 建 client
    store := ratelimit.NewStore(cfg)                            // memory | redis
    return func(next endpoint.Endpoint) endpoint.Endpoint {
        return func(ctx context.Context, req, resp interface{}) error {
            if !cfg.Enabled { return next(ctx, req, resp) }
            ri := rpcinfo.GetRPCInfo(ctx)
            lookup := ratelimit.Lookup{
                Service:  ri.To().ServiceName(),           // server basic info(= cfg.Server.Registry.Name)
                Phase:    "post_auth",
                Method:   ri.To().RPCMethod(),
                AppKey:   callerServiceFromCtx(ctx),   // transmeta x-caller-service
                ClientIP: callerAddr(ri),
            }
            resolved, err := resolver.Resolve(ctx, lookup)
            if err != nil { /* fail_open 裁决 */ }
            if !resolved.Rule.Enabled { return next(ctx, req, resp) }
            key := ratelimit.BuildKey(lookup, resolved.Rule.KeyBy)
            ok, err := store.Allow(ctx, key, resolved.Rule)
            if err != nil { /* fail_open 裁决 */ }
            if !ok {
                if cfg.Mode == "enforce" {
                    return rpcerror.RateLimited(resolved.Rule.ClientTTLSeconds.Duration)
                }
                // shadow:计数已由 Allow() 完成,只记录不拒绝
                klog.CtxWarnf(ctx, "ratelimit shadow denied: %s/%s", lookup.Service, lookup.Method)
                // metrics: ratelimit_shadow_denied{service,method}
            }
            return next(ctx, req, resp)
        }
    }
}
```

**规则源客户端由中间件内部按 conf 构建**(`buildOptions`):source.type=rule_center/grpc 时以 `cfg.RuleCenter.Address` 建立共享 rule-center client(懒初始化 + sync.Once;建连失败不 panic,落入 resolver 的 fallback 语义)。由此 `add infra rate-limit` 对 server.go 的 astwire 注入保持最小(仅两条调用语句),也避免每个调用点手工传 client。保留 `RateLimitWithOptions(cfg, opts)` 供测试注入 fake。

### 6.2 rule-center 放开 kitex

`rule_center_client.go` 是纯 grpc + conf 代码,框架无关。改动:

- `rulecenter.Add`:kind 校验 `hertz|kitex`;client 写入路径两侧一致(`internal/pkg/middleware/rule_center_client.go`)
- `updateConfForRuleCenter`(yaml 文本改写)框架无关,直接复用
- **kitex 分支跳过 server.go wire**:hertz 需要 wire 是因为 hertz 中间件经 `Options` 接 client;kitex 中间件按 §6.1 自建 client,无需注入。kitex 侧 `add rule-center` = 写 client 文件 + 改 conf source.type=rule_center
- next-steps 与 hertz 同构(go get grpc、go mod tidy)

### 6.3 conf 新增字段

```yaml
# conf/dev/conf.yaml(生成默认)
rate_limit:
  enabled: true
  mode: shadow          # shadow | enforce —— 默认 shadow(计数不拒绝)
  backend: memory       # memory | redis
  source:
    type: config        # config | database | rule_center | grpc
    cache_ttl_seconds: 60
    fallback_on_error: true
  fail_open: true
  static:               # 静态兜底(Kitex WithLimit)
    max_qps: 0          # 0 = 不挂载 WithLimit
    max_connections: 0
  # ... phases / grpc / database / rule_center 子段沿用现有结构
```

`Mode` 与 `Static` 同步加入 hertz 侧 `RateLimitConfig`:hertz 忽略 `Static`;`Mode` 对 hertz 中间件同样生效(hertz 顺带获得 shadow 能力)。默认值两侧不同:**hertz `Mode` 默认 `enforce`(现状行为不变);kitex `Mode` 默认 `shadow`(新能力安全上线)**。

### 6.4 拒绝语义

- `rpcerror.RateLimited(retryAfter)` → Kitex BizStatusError,biz code **10429**,msg `rate limited`
- metainfo transient value 携带 `rl-retry-after`(秒),供 caller 做退避重试
- 框架将 BizStatusError 计为业务错误而非调用失败 → 不误触发 caller 的失败率熔断

### 6.5 静态轨

- `StaticLimitOption(cfg.Static) kitexserver.Option`:max_qps/max_connections 均 >0 时返回 `kitexserver.WithLimit(&limit.Option{...})`,否则 nil
- astwire 在 `// ncgo:wire:ratelimit:static-limit` 标记后注入条件挂载语句
- 默认 0 = 不挂载 → 与"默认生成不误伤"一致;next-steps 引导压测后设值

## 7. 每请求数据流

```
RPC call
 ├─ [静态轨] WithLimit(若配置):连接数/QPS 超限 → 框架层拒绝
 └─ [动态轨] RateLimit 中间件:
      1. !cfg.Enabled → next
      2. 构建 Lookup(rpcinfo + conf service name)
      3. resolver.Resolve:
           动态源命中(含进程缓存)→ 远程规则
           未命中 / 失败 → fallback_on_error → 旧缓存 / 本地 config 规则
      4. !rule.Enabled → next
      5. store.Allow(key = KeyBy 维度拼接, rule)   ← shadow/enforce 均调用,计数真实
           ├─ 允许 → next
           ├─ store 错误 → fail_open 裁决(默认放行)
           └─ 超限:
                enforce → BizStatusError 10429(+ metainfo)
                shadow  → klog warn + metrics → next    ★ 默认
```

KeyBy 维度降级:caller 未知 → 退化为 caller-ip(与 hertz normalize 逻辑共享)。

## 8. 错误处理矩阵

| 故障场景 | 行为 |
|---|---|
| 规则源(rule-center/db)查询失败 | `fallback_on_error: true` → 旧缓存/本地规则;`false` → `fail_open` 裁决 |
| store(redis)不可用 | `fail_open: true`(默认)→ 放行;`false` → 拒绝 |
| rule-center client 建连失败 | 不 panic;动态源视为失败,走上一行语义 |
| Lookup 维度缺失 | KeyBy 降级(caller→ip) |
| 静态轨超限 | kitex 框架层拒绝(连接关闭 / QPS 错误),与动态轨独立 |
| shadow 模式任何拒绝判决 | 永不中断:warn 日志 + `ratelimit_shadow_denied{service,method}` |

## 9. 测试策略

### 9.1 单元测试

共享 asset 自带测试文件(随生成落地),ncgo 仓内 testdata 等价覆盖:

- **resolver**:源优先级(远程 > 本地)、缓存命中/过期、fallback 分支、method 级匹配、Normalize 默认值
- **store**:token_bucket / fixed_window × memory / redis-Lua(redis 测试沿用 hertz 现有方案)
- **kitex middleware**:enforce 返回 10429 + metainfo 值;shadow 放行**且计数生效**(再切 enforce 立即拒绝);fail_open 两分支;`StaticLimitOption` 零值返回 nil;client 建连失败不 panic
- **rulecenter(脚手架)**:kitex 分支写文件 + conf 改写 + 跳过 wire

### 9.2 Golden 测试(ncgo 契约面)

- `internal/scaffold/infra` golden:锁定 `add infra rate-limit` 全部输出(文件内容 + conf diff + wire 结果)
- `internal/scaffold/rpc` golden:`--preset rule-center` 输出随 middleware 模板重写更新
- `internal/scaffold/mono`(hertz)golden:include 缝合后输出与重构前**逐字节一致**
- `internal/scaffold/rulecenter` golden:kitex 分支产物

### 9.3 E2E(`ncgo test rate-limit e2e` 扩展)

- 新增 **RPC attacker**:对目标 kitex 服务发压,统计 biz status 10429 比例;方案(generic gRPC 泛化调用 vs 复用生成项目自带 client)在计划阶段确定 —— **本项为最高风险项**
- 结果分类沿用 PASS / FAIL / WARN
- 两段式验证:shadow 段断言"0 拒绝 + 日志含 denied 记录";enforce 段断言 10429 比例
- `detectProject` 已支持 mono/kitex;micro 模式(rule-center + kitex 业务服务,source=rule_center)依赖 §6.2 的 rule-center kitex 支持

**降级预案**:若 generic 泛化调用受阻,e2e kitex 先达成 mono+database 路径;micro+rule_center 路径以集成测试(起真实进程 + 少量真实 RPC 调用验证 10429)过渡。

## 10. 迁移与兼容性

- **Hertz 零感知**:抽取是零行为变化的搬移 + include 缝合;hertz golden 逐字节保护。hertz 额外获得 `Mode` 字段但默认 enforce,行为不变
- **存量 kitex 项目**:占位符 `internal/base/middleware/ratelimit.go` 经 `add infra rate-limit` 覆盖(update_behavior: cover);不执行命令则无任何变化
- **rule-center preset 存量项目**:重新生成或执行 add 命令后获得真实中间件;conf 默认 shadow,不会因升级而开始拒绝流量

## 11. 风险

| 风险 | 等级 | 缓解 |
|---|---|---|
| e2e RPC attacker 方案(泛化调用复杂度) | 高 | 计划阶段先做技术验证(spikes);降级预案见 §9.3 |
| hertz layout include 缝合引入回归 | 中 | golden 逐字节比对;缝合逻辑独立单测 |
| 抽取过程破坏 hertz 中间件对 store 的内部引用 | 中 | 先抽后改,golden + 现有 hertz 测试双重保护 |
| Kitex API(rpcinfo/metainfo/limit)兼容 | 低 | 模板已在使用同款 API |

## 12. 交付顺序建议(供计划阶段参考)

1. 共享 asset 抽取 + include 缝合(hertz golden 保护,行为不变)
2. kitex 模板改造(conf 字段 / rpcerror / middleware 重写 / server 标记)
3. `add infra rate-limit` 命令(infra.go + astwire + golden)
4. `add rule-center` kitex 支持
5. e2e RPC attacker(先 spike 定方案)
6. 文档与 CLI 帮助
