# 内置 hertz layout.yaml：RateLimitRuleRepository/Hook 接入 Resolver

Issue: https://github.com/byx-darwin/ncgo/issues/122

## 背景

`internal/assets/_data/hertz/hertz-template/server_go.yaml` 在 `WithDatabase` 分支里
构造了 `RateLimitRuleRepository` 但丢弃了返回值，从未接入 `ratelimit.Resolver`。
`internal/pkg/ratelimit`（resolver.go）已通过 `{{include: ratelimit/resolver}}`
无条件生成，`Resolver.Resolve` 已按 `cfg.RateLimit.Source.Type == "database"`
消费 `Options.Database`——缺的只是 server.go 里的组装代码。

设计文档 `docs/hertz/rate-limit-dynamic-design.{zh-CN,en}.md` 附录 A.1 已经描述了
这个"应有行为"，本次修复是把模板补齐到与文档一致。

## 改动范围

唯一改动文件：`internal/assets/_data/hertz/hertz-template/server_go.yaml`
（`{{if .WithDatabase}}` 分支内）：

1. import 块加入 `"{{.Module}}/internal/pkg/ratelimit"`（WithDatabase-only）。
2. 包级变量：
   ```go
   // RateLimitResolver is the process-level dynamic rate-limit rule resolver,
   // assembled once at startup and reused across requests.
   var RateLimitResolver *ratelimit.Resolver
   ```
3. `Run()` 内，数据库初始化代码块（`if cfg.Database.Enabled { ... }`）之前声明
   `var rateLimitOpts ratelimit.Options`；块内原先丢弃返回值的
   `repository.NewRateLimitRuleRepository(dbData)` 改为：
   ```go
   if cfg.RateLimit.Source.Type == "database" {
       rateLimitOpts.Database = repository.NewRateLimitRuleHook(repository.NewRateLimitRuleRepository(dbData))
   }
   ```
4. 数据库块之后（`{{end}}` 之前），组装 resolver 并加锚点注释：
   ```go
   // RateLimitResolver assembles the dynamic rate-limit rule source (database
   // hook when cfg.RateLimit.Source.Type == "database") with local config
   // fallback. Wire it into routes with:
   //   middleware.RateLimit(phase, cfg.RateLimit, phaseCfg, RateLimitResolver)
   // ncgo:wire:rate-limit:resolver
   RateLimitResolver = ratelimit.NewResolver(cfg.RateLimit, rateLimitOpts)
   ```

不改动：`internal/pkg/ratelimit/resolver.go`、`internal/repository/rate_limit_rule.go`、
schema/migration/seed、`RateLimit`/`rateLimitWithStore` 中间件本身。

## 测试

- 更新受影响 golden 快照：`mono-with-database`、`mono-with-rulecenter`
  （`go test ./internal/scaffold/mono/... -run TestGenerateGolden -update-golden -count=1`
  后人工检查 diff 只涉及 `template/hertz-template/server_go.yaml`）。
- `mono-default`（`WithDatabase=false`）快照不应变化，作为回归验证。
- 端到端编译验证：用本机 `hz` 生成一个 `--db postgres` 的 hertz 项目，
  `go build ./...` 确认新 `server.go` 真实可编译（此前从未被覆盖到）。
- `go build ./... && go vet ./... && go test ./... -count=1`。

## 文档

`docs/hertz/rate-limit-dynamic-design.{zh-CN,en}.md` 附录 A.1 已描述该行为，
预计只需核对变量名/锚点注释与实现一致，不改变设计结论。
