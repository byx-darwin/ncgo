# Issue #62: Add Missing Infrastructure Component Documentation to Design Docs

**Date:** 2026-08-09  
**Issue:** [#62](https://github.com/byx-darwin/ncgo/issues/62)  
**Scope:** docs-only — embedded design docs under `internal/assets/_data/docs/`  
**Mode:** standard (Phase 2 retained per user request, but docs-only)

---

## Problem Statement

`ncgo ai sync` materializes embedded design docs (`docs/hertz/design-doc.*.md`, `docs/kitex/design-doc.*.md`) into generated projects. These design docs document several infra components (Redis, Kafka, ES, ClickHouse) but **omit** `observability_logging` — a commonly-used structured logging add-on.

The `observability_logging` add-on generates 3 files (`logging.go`, `hertz.go`/`kitex.go`) with:
- `InitFromConf()` — global logger initialization from conf
- `HertzRequestID()` / `KitexRequestID()` — request ID middleware
- `HertzAccessLog()` / `KitexAccessLog()` — structured access log middleware
- `HertzRecovery()` / `KitexRecovery()` — panic recovery middleware
- Shared helpers: `WithRequestID()`, `WithTrafficLane()`, `SinceMS()`

AI Agents working on generated projects cannot find usage instructions for these components.

### What Already Exists

| Infra Kind | Hertz EN | Hertz ZH | Kitex EN | Kitex ZH |
|---|---|---|---|---|
| redis | ✅ detailed | ✅ | ✅ brief | ✅ |
| kafka | ✅ detailed | ✅ | ✅ brief | ✅ |
| es | ✅ detailed | ✅ | ✅ brief | ✅ |
| clickhouse | ✅ detailed | ✅ | ✅ brief | ✅ |
| observability_logging | ❌ MISSING | ❌ MISSING | ❌ MISSING | ❌ MISSING |
| release_canary (hertz) | ❌ missing | ❌ missing | N/A | N/A |
| release_canary (kitex) | N/A | N/A | ✅ detailed | ✅ |
| registry_polaris | N/A | N/A | ✅ detailed | ✅ |
| polaris_adapter | N/A | N/A | ❌ missing | ❌ missing |

**Primary gap:** `observability_logging` (high priority in issue)  
**Secondary gap:** `polaris_adapter` (kitex-only, mentioned as kitex-only kind)

## Approach: Update Embedded Design Docs (Issue Option 1)

**Decision:** 采用方案 1，AI Agent 一定会读 design doc，内容写在 design doc 里最直接。

**内容标准（AI Agent 友好）：** 每个组件必须包含：
1. **生成哪些文件** — Agent 需要知道文件在哪
2. **配置格式** — `conf.yaml` 的具体字段和示例
3. **初始化代码** — 在 `server.go` 里怎么注册 middleware/interceptor
4. **使用示例** — handler/usecase 里怎么调用

**Rationale:**
- AI sync 已通过 `listDocSpecs()` → `writeStandaloneDocs()` 读取这些文档
- 与现有 Redis/Kafka/ES/ClickHouse 文档模式一致
- 零代码改动，纯内容添加
- 所有下游项目自动受益

## Files to Change

### 1. `internal/assets/_data/docs/hertz/design-doc.en.md`

**Add section** `#### Structured Logging (observability_logging)` after the ClickHouse section (line 381), before `## 4. Files`:

```markdown
#### Structured Logging (`observability_logging.go` + `hertz.go`)

`ncgo add infra observability_logging` (alias: `logging`) adds category-based
structured logging middleware under `internal/base/logging/`.

- **Files added:**
  - `internal/base/logging/logging.go` — shared helpers: `WithRequestID`,
    `WithTrafficLane`, `SinceMS`, and category constants (`CategoryAccess`,
    `CategoryError`, `CategoryBiz`, `CategoryRPC`, `CategoryDB`,
    `CategoryPanic`, `CategoryAudit`, `CategorySecurity`).
  - `internal/base/logging/hertz.go` — Hertz-specific middleware.

- **Middleware (registered in `server.go`):**
  - `logging.HertzRequestID()` — extracts `X-Request-ID` from the request
    header or OTel span context; generates a 16-byte hex ID when absent;
    sets the response header. Also propagates `X-Traffic-Lane`.
  - `logging.HertzAccessLog()` — emits a structured access log entry
    (`goclog.Access`) with `http.method`, `http.path`, `http.status_code`,
    and `latency_ms`.
  - `logging.HertzRecovery()` — catches panics, logs via
    `goclog.L().WithCategory(CategoryPanic)`, and returns HTTP 500.

- **Initialization:**
  ```go
  // In server.go or main.go, after conf.Init():
  logging.InitFromConf(cfg.Logging, goclog.ReleaseInfo{...})
  ```

- **Configuration (`conf.yaml`):**
  ```yaml
  logging:
    level: info
    format: json        # "json" or "text"
    mode: production    # "production" or "development"
    add_source: false
    file:
      dir: logs
      filename: app.log
      max_size_mb: 100
      max_backups: 3
      max_age_days: 7
      compress: true
  ```

- **Deps:** `github.com/byx-darwin/go-tools/go-common` (log, error packages).
```

**Update section** `## 7. Optional Infra` (line 467):

Change "Currently shipped: `redis`, `kafka`, `es`, `clickhouse`." to:

> Currently shipped: `redis`, `kafka`, `es`, `clickhouse`, `observability_logging` (structured logging middleware), `release_canary` (Hertz adapter). Observability tracing (OTLP) is provided by the Hertz base template and no longer ships as a standalone add-on.

### 2. `internal/assets/_data/docs/hertz/design-doc.zh-CN.md`

**Add section** `#### 结构化日志（`observability_logging.go` + `hertz.go`）` after ClickHouse section (line 391), before `## 4. 文件清单`:

Chinese translation of the English section above, following the same structure.

**Update section** `## 7. 可选基础设施` (line 442): add `observability_logging` and `release_canary` to the shipped list.

### 3. `internal/assets/_data/docs/kitex/design-doc.en.md`

**Add section** `#### Structured Logging (observability_logging.go + kitex.go)` after the ClickHouse section (line 306), before `#### Polaris Registry`:

```markdown
#### Structured Logging (`observability_logging.go` + `kitex.go`)

`ncgo add infra observability_logging` (alias: `logging`) adds category-based
structured logging interceptors under `internal/base/logging/`.

- **Files added:**
  - `internal/base/logging/logging.go` — shared helpers (same as Hertz copy).
  - `internal/base/logging/kitex.go` — Kitex-specific interceptors.

- **Interceptors (registered in `server.go` and client constructors):**
  - `logging.KitexRequestID()` — extracts `x-request-id` from Kitex
    metainfo or OTel span context; generates a 16-byte hex ID when absent;
    persists via `metainfo.WithPersistentValue`. Also propagates
    `x-traffic-lane`.
  - `logging.KitexAccessLog()` — emits a structured RPC log entry
    (`goclog.RPC`) with `rpc.system`, `rpc.service`, `rpc.method`, and
    `latency_ms`. Logs at ERROR level on failure, INFO on success.
  - `logging.KitexRecovery()` — catches panics, logs via
    `goclog.L().WithCategory(CategoryPanic)`.

- **Initialization:**
  ```go
  // In server.go or main.go, after conf.Init():
  logging.InitFromConf(cfg.Log, goclog.ReleaseInfo{...})
  ```

- **Configuration (`conf.yaml`):**
  ```yaml
  log:
    level: info
    format: json
    mode: production
  ```

- **Deps:** `github.com/byx-darwin/go-tools/go-common` (log, error packages),
  `github.com/bytedance/gopkg` (metainfo, for request ID propagation across
  Kitex RPC calls).
```

**Add section** `#### Polaris Adapter (polaris_adapter.go, kitex-only)` after the Polaris Registry section:

Document `polaris_adapter.go` — adapter for Polaris-based canary routing (kitex-only).

**Update section** `## 6. Optional Infra` (line 452):

Change "Currently shipped: `redis`, `kafka`, `es`, `clickhouse`, and Kitex-only `registry_polaris`." to:

> Currently shipped: `redis`, `kafka`, `es`, `clickhouse`, `observability_logging` (structured logging interceptors), and Kitex-only `registry_polaris`, `polaris_adapter` (Polaris canary adapter). Observability tracing is provided by the kitex base template and no longer ships as a standalone add-on.

### 4. `internal/assets/_data/docs/kitex/design-doc.zh-CN.md`

**Add section** `#### 结构化日志（`observability_logging.go` + `kitex.go`）` after ClickHouse section (line 284), before Polaris Registry section.

**Add section** `#### Polaris 适配器（`polaris_adapter.go`, kitex 专属）` after Polaris 注册/发现 section.

**Update section** `## 6. 可选基础设施` (line 425): add `observability_logging` and `polaris_adapter` to the shipped list.

## Validation Plan

Since this is a docs-only change to embedded templates, the relevant validation is:

1. **Unit tests:** `go test ./internal/ai/... -count=1` — ensure `ai sync` still works (docs are embedded and read by sync).
2. **Build check:** `go build ./...` — ensure no compilation issues.
3. **Golden tests:** `go test ./internal/scaffold/... -count=1` — ensure no scaffold output changes (design docs are not part of scaffold output directly, but are embedded assets).
4. **Manual check:** Run `ncgo ai sync` on a test project and verify the generated `docs/ncgo/hertz/design-doc.en.md` includes the new logging section.

## Out of Scope

- Modifying `ai sync` code to read additional sources (issue option 2)
- Creating a separate `infrastructure-guide.md` (issue option 3)
- Adding documentation for `rate_limit` (already documented in separate `rate-limit-dynamic-design.*.md` files)
- Changes to any Go source code

## Acceptance Criteria (from Issue #62)

- [x] observability_logging has detailed usage instructions (in both EN and ZH)
- [x] All `ncgo add infra` supported components have corresponding docs
- [x] `ai sync` generated docs include these infrastructure component instructions (via embedded design docs)
- [x] AI Agents can understand how to configure and use these components from generated docs
