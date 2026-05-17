# 限流实现审计报告

## 概要

审计了 ncgo 限流实现（单体 Hertz HTTP + 微服务 rule-center Kitex）。发现 **2 个严重** bug、**4 个警告**和 **4 个次要**问题。所有严重 bug 和 3 个警告已修复。

## 架构概览

```
单体 (Hertz) ──► 中间件 ──► 解析器 ──► 来源: config | database | rule_center(gRPC)
                                              │
                                              ├─ config:      本地 conf.yaml 规则
                                              ├─ database:    sqlc 通过 repository hook
                                              └─ rule_center: gRPC 连接到 Kitex 服务

微服务 (Kitex) ──► Rule-Center 服务 ──► sqlc ──► PostgreSQL
                  (gRPC CRUD API)
```

## 已修复的 Bug

### 严重 #1 — rule_center_client.go 未传递 app_key [已修复]

**文件:** `internal/assets/_data/hertz/optional/rule_center_client.go:91-96`

**问题:** `GetRuleRequest` 构造时没有传入 `AppKey`，尽管：
- Proto 定义了 `optional string app_key = 5`
- Usecase 通过 `appKey` 处理应用特定的规则查找
- `Lookup` 结构体携带 `lookup.AppKey`

**影响:** 所有应用特定的限流规则**永远不会匹配**。rule-center 始终回退到全局（app_key IS NULL）规则。

**修复:** 在请求中添加 `AppKey: strPtrOrNil(lookup.AppKey)` 和 `strPtrOrNil` 辅助函数。

---

### 严重 #2 — 生成的 server.go 中从未实例化 RuleCenter [已修复]

**文件:** `internal/assets/_data/hertz/layout.yaml:8258-8272`

**问题:** 生成的 `server.go` 创建了 `rlOpts ratelimit.Options` 但当 `source.type: "rule_center"` 时从未设置 `rlOpts.RuleCenter`。接线示例仅存在于注释中。

**影响:** `source.type: "rule_center"` → 动态来源为 nil → 解析器**始终回退到本地配置规则**。

**修复 (ncgo new):** 在 layout.yaml 中添加条件接线，当 `RuleCenterAddr` 设置时生效。

**修复 (ncgo add rule-center):** 在 `internal/scaffold/rulecenter/rulecenter.go` 中添加 `wireRuleCenterInServer()` 函数，当运行 `ncgo add rule-center` 时自动注入接线代码到已存在的 `server.go`。

---

### 警告 #3 — pickBestPattern 缺少 glob/regex 支持 [已修复]

**文件:** `internal/assets/_data/kitex/kitex-template/ratelimit_usecase.yaml:190-203`

**问题:** 只处理了 `*` 通配符、`prefix` 和 `exact`。Schema 和 proto 还支持 `glob` 和 `regex` 匹配类型。

**影响:** 数据库中的 glob/regex 规则被**静默跳过**。

**修复:** 添加了 `globMatch()` 和 `regexMatch()` 函数。

---

### 警告 #4 — SQL 查询缺少 enabled 过滤 [已修复]

**文件:** `internal/assets/_data/kitex/kitex-template/ratelimit_sqlc_queries.yaml`

**问题:** 所有 4 个读查询都返回了已禁用的规则，没有检查 `enabled = true`。

**影响:** 数据库中被禁用的规则仍然会被返回并可能被中间件执行。

**修复:** 在所有读查询的 WHERE 子句中添加 `AND enabled = true`。

---

### 警告 #5 — Seed 数据 path_pattern 语义不正确 [已修复]

**文件:** `internal/scaffold/test/ratelimit/seed.go:52-56`

**问题:** `/healthz` 精确匹配的种子行 `path_pattern='/healthz'` — 语义错误，因为 `path_pattern` 用于前缀/glob/regex 模式，而非精确匹配。

**修复:** 精确匹配规则改为 `path_pattern=''`。通配符规则（`path='*'`）保留 `path_pattern='*'` 作为全局回退。

---

### 警告 #6 — gRPC 连接泄漏 [由严重 #2 修复附带解决]

**文件:** `internal/assets/_data/hertz/optional/rule_center_client.go:69-74`

**问题:** `Close()` 存在但生成的代码从未调用它。

**修复:** 服务器接线现在包含 `defer func() { _ = rc.Close() }()`。

---

## 未修复的问题

### 次要 #8 — Kitex rule-center 无限流中间件

**文件:** `internal/assets/_data/kitex/kitex-template/ratelimit_middleware.yaml`

**状态:** 设计如此。中间件是透传占位符。rule-center 服务自身零限流保护。未修复是因为需要单独的设计决策来确定如何对 gRPC 服务进行限流（与 Hertz 的中间件模式不同）。

### 次要 #9 — Seed SQL 使用字符串插值

**文件:** `internal/scaffold/test/ratelimit/seed.go:52-56`

**状态:** `buildSeedSQL` 使用 `fmt.Sprintf` + `sanitizeSQLString`（单引号转义）。在 PostgreSQL `standard_conforming_strings=on`（9.1 起默认开启）下是安全的，但使用 `psql --variable` 参数化查询会更好。低优先级，因为这只是测试基础设施。

## 一键测试影响

修复严重 #1 和 #2 后，rule-center 流程现在可用于 e2e 测试：

| 场景 | 状态 | 备注 |
|------|------|------|
| 单体 + config 来源 | ✅ 可用 | 本地 conf.yaml 规则，不需要数据库 |
| 单体 + database 来源 | ✅ 可用 | seed → run 通过 PostgreSQL |
| 微服务 + rule-center | ✅ 现在可用 | 之前不可用（严重 #1 + #2） |
| 单体 + Kitex RPC | ❌ 无中间件 | 警告 #8 — 透传占位符 |

`ncgo test rate-limit e2e` 命令现在可以测试所有三个可用场景。
