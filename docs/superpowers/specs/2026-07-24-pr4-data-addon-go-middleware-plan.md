# PR4 Implementation Plan — 数据类 add-on 接入 go-middleware（kafka/es/clickhouse）

- 状态：draft
- 日期：2026-07-24
- 关联：Issue #9；设计文档 `docs/superpowers/specs/2026-07-24-pr4-data-addon-go-middleware-design.md`
- 前置：PR3（logging + redis，commit aa786c5）

## 1. Architecture Overview

PR4 follows the same migration pattern established in PR3 (redis):

```
┌─────────────────────────────────────────────────────────────────────┐
│  Generated Project (downstream)                                     │
│                                                                     │
│  conf/dev/conf.yaml ──► Config struct ──► samber/do DI              │
│                                               │                     │
│                                               ▼                     │
│                                    internal/base/data/{kafka,es,    │
│                                    clickhouse}.go                   │
│                                               │                     │
│                                    ┌──────────┴──────────┐          │
│                                    │  Wrapper struct     │          │
│                                    │  (KafkaWriter, etc) │          │
│                                    └──────────┬──────────┘          │
│                                               │ delegates to        │
│                                               ▼                     │
│                              go-middleware/{kafka,es,clickhouse}     │
│                              (Config + Factory)                     │
└─────────────────────────────────────────────────────────────────────┘
```

**Key difference from PR3 redis**: Redis uses a shared client pattern (`SharedRedisClient`) because multiple middleware reuse one connection. Kafka/ES/ClickHouse are single-consumer — each wrapper owns its connection directly via go-middleware factory.

**Error code strategy** (from design doc):
- Hertz: `frameworkerror.CodeConfigInvalid` for config validation; project-segment or go-middleware codes for connection errors
- Kitex: **align from string codes to numeric codes** (same as hertz)
- ClickHouse: use go-middleware predefined `mwclickhouse.CodeConnect` (20401); delete project-segment `CodeDatabaseUnavailable` (40503)
- ES: keep project-segment `CodeSearchUnavailable` (40506) since go-middleware/es has no predefined codes
- Kafka: `frameworkerror.CodeConfigInvalid` for config; no connection-time error (factory doesn't fail)

## 2. Component Changes

### 2.1 Hertz Kafka Template (`internal/assets/_data/hertz/optional/kafka.go`)

**Before**: Receives raw `*kafka.Writer` / `kafka.ReaderConfig`, wraps directly.
**After**: Receives `mwkafka.WriterConfig` / `mwkafka.ReaderConfig`, delegates to `mwkafka.NewWriter` / `mwkafka.NewConsumer`.

Changes:
1. Import: replace `"github.com/segmentio/kafka-go"` with `mwkafka "github.com/byx-darwin/go-tools/go-middleware/kafka"`
2. `KafkaWriter.W` field type: `*kafka.Writer` → `*mwkafka.Writer`
3. `KafkaReader.R` field type: `*kafka.Reader` → `*mwkafka.Consumer`
4. `NewKafkaWriter(cfg mwkafka.WriterConfig)`:
   - Validate `len(cfg.Broker) == 0` → error
   - Validate `cfg.Topic == ""` → error
   - Call `mwkafka.NewWriter(cfg)` → assign to `W`
   - Cleanup: `w.Close()`
5. `NewKafkaReader(cfg mwkafka.ReaderConfig)`:
   - Validate `len(cfg.Broker) == 0` → error
   - Validate `cfg.Topic == ""` → error
   - Call `mwkafka.NewConsumer(cfg)` → assign to `R`
   - Cleanup: `c.Close()`
6. Error codes: keep `frameworkerror.CodeConfigInvalid` (already correct)
7. Header comment: update usage examples to show `mwkafka.WriterConfig`/`mwkafka.ReaderConfig` + `do.ProvideValue`; update dependency to `go get github.com/byx-darwin/go-tools/go-middleware`

### 2.2 Hertz ES Template (`internal/assets/_data/hertz/optional/es.go`)

**Before**: Receives `elasticsearch.Config`, calls `elasticsearch.NewClient(cfg)`.
**After**: Receives `mwes.Config`, calls `mwes.NewClient(cfg)`.

Changes:
1. Import: add `mwes "github.com/byx-darwin/go-tools/go-middleware/es"`; keep `"github.com/elastic/go-elasticsearch/v8"` (for `ES.Client` field type)
2. `ES.Client` field type: stays `*elasticsearch.Client` (go-middleware returns this type)
3. `NewES(ctx context.Context, cfg mwes.Config)`:
   - Validate `len(cfg.Addresses) == 0` → error (CodeConfigInvalid)
   - Call `mwes.NewClient(cfg)` → returns `(*elasticsearch.Client, error)`
   - On error → wrap with `CodeSearchUnavailable`
   - Ping: `cli.Ping(cli.Ping.WithContext(ctx))` → on error wrap with `CodeSearchUnavailable`
   - Cleanup: no-op (ES client has no Close)
4. Keep `CodeSearchUnavailable = 40506` + `init()` registration
5. Header comment: update usage to show `mwes.Config`; update dependency

### 2.3 Hertz ClickHouse Template (`internal/assets/_data/hertz/optional/clickhouse.go`)

**Before**: Receives `*clickhouse.Options`, calls `clickhouse.Open(opts)`. Uses `CodeDatabaseUnavailable = 40503`.
**After**: Receives `mwclickhouse.Config`, calls `mwclickhouse.NewClient(cfg)`. Uses `mwclickhouse.CodeConnect` (20401).

Changes:
1. Import: replace `"github.com/ClickHouse/clickhouse-go/v2/lib/driver"` with `mwclickhouse "github.com/byx-darwin/go-tools/go-middleware/clickhouse"`; keep `"github.com/ClickHouse/clickhouse-go/v2"` (for `clickhouse.Conn` interface type)
2. `ClickHouse.Conn` field type: `driver.Conn` → `clickhouse.Conn` (same interface, different import path)
3. Delete `CodeDatabaseUnavailable = 40503` and its `init()` registration
4. `NewClickHouse(ctx context.Context, cfg mwclickhouse.Config)`:
   - Validate: `cfg.DSN == "" && len(cfg.Addrs) == 0` → error (CodeConfigInvalid)
   - Call `mwclickhouse.NewClient(cfg)` → returns `(clickhouse.Conn, error)`
   - On error → wrap with `mwclickhouse.CodeConnect`
   - Ping: `conn.Ping(ctx)` → on error close conn, wrap with `mwclickhouse.CodeConnect`
   - Cleanup: `conn.Close()`
5. Header comment: update usage to show `mwclickhouse.Config`; update dependency

### 2.4 Kitex Kafka Template (`internal/assets/_data/kitex/optional/kafka.go`)

Same structural changes as Hertz kafka (§2.1), plus:
- **Error code alignment**: string codes (`"kafka_writer_missing"`, etc.) → `frameworkerror.CodeConfigInvalid`
- Add `frameworkerror` import
- Header comment: Kitex-specific (no `go-framework` dependency line — kitex uses `go-common` only; but now needs `go-framework` for `frameworkerror.CodeConfigInvalid`)

**Decision**: Kitex templates will import `frameworkerror "github.com/byx-darwin/go-tools/go-framework/error"` for `CodeConfigInvalid`. This aligns with the design doc's "kitex 从字符串码对齐为 frameworkerror.CodeConfigInvalid" requirement. The `go get` next-steps must include `go-framework`.

### 2.5 Kitex ES Template (`internal/assets/_data/kitex/optional/es.go`)

Same structural changes as Hertz ES (§2.2), plus:
- **Error code alignment**: string codes → numeric codes
  - Config: `frameworkerror.CodeConfigInvalid`
  - Connection: `CodeSearchUnavailable = 40506` (add const + init, same as hertz)
- Add `frameworkerror` import
- Add `CodeSearchUnavailable` const + `init()` with `goerror.RegisterHTTPStatuses`

### 2.6 Kitex ClickHouse Template (`internal/assets/_data/kitex/optional/clickhouse.go`)

Same structural changes as Hertz ClickHouse (§2.3), plus:
- **Error code alignment**: string codes → numeric codes
  - Config: `frameworkerror.CodeConfigInvalid`
  - Connection: `mwclickhouse.CodeConnect` (20401)
- Add `frameworkerror` import
- No project-segment code needed (uses go-middleware predefined)

### 2.7 optional-config YAML Snippets (Hertz only — kitex has no optional-config dir)

**`internal/assets/_data/hertz/optional-config/kafka.yaml`**:
- `brokers` → `broker`
- Remove: `balancer`, `required_acks`, `async`, `batch_size`, `batch_bytes`, `batch_timeout_milliseconds`, `queue_capacity`, `start_offset`, `max_wait_milliseconds`
- Add: `allow_auto_topic_creation`, `max_wait` (duration string), `read_batch_timeout`, `tls`, `sasl` blocks

**`internal/assets/_data/hertz/optional-config/es.yaml`**:
- Remove: `service_token`, `compress_request_body`, `discover_nodes_on_start`, `enable_metrics`, `enable_debug_logger`
- Add: `cloud_id`, `max_idle_conns_per_host`, `tls` block

**`internal/assets/_data/hertz/optional-config/clickhouse.yaml`**:
- `addr` → `addrs`
- `dial_timeout_seconds` → `dial_timeout`
- `conn_max_lifetime_seconds` → `conn_max_lifetime`
- `compress: lz4` → `compress: true`
- Remove: `protocol`, `block_buffer_size`
- Add: `dsn`, `tls` block

### 2.8 infra.go goGetDeps Update

```go
// before
KindKafka:      {"github.com/segmentio/kafka-go", "github.com/byx-darwin/go-tools/go-common"},
KindES:         {"github.com/elastic/go-elasticsearch/v8", "github.com/byx-darwin/go-tools/go-common"},
KindClickHouse: {"github.com/ClickHouse/clickhouse-go/v2", "github.com/byx-darwin/go-tools/go-common"},

// after
KindKafka:      {"github.com/byx-darwin/go-tools/go-middleware", "github.com/byx-darwin/go-tools/go-common", "github.com/byx-darwin/go-tools/go-framework"},
KindES:         {"github.com/byx-darwin/go-tools/go-middleware", "github.com/byx-darwin/go-tools/go-common", "github.com/byx-darwin/go-tools/go-framework"},
KindClickHouse: {"github.com/byx-darwin/go-tools/go-middleware", "github.com/byx-darwin/go-tools/go-common", "github.com/byx-darwin/go-tools/go-framework"},
```

**Note**: `go-framework` is added because kitex templates now use `frameworkerror.CodeConfigInvalid`. Hertz templates already had `go-framework` in their header comments. The goGetDeps is the single source of truth for next-steps output.

### 2.9 infra_test.go Updates

- Line ~283: kafka Add test checks next-steps output — update expected `go get` strings
- Line ~367: `TestGoGetDepsAllKinds` iterates all kinds — will pass automatically after goGetDeps change
- Any assertion checking for `segmentio/kafka-go`, `elastic/go-elasticsearch`, or `ClickHouse/clickhouse-go` in next-steps must be updated

### 2.10 Documentation

- `README.md` (line ~392): Add a paragraph noting kafka/es/clickhouse add-ons now use `go-middleware` factories (similar to the existing redis paragraph)
- `README.zh-CN.md` (line ~326): Same in Chinese
- `docs/examples.md` / `docs/examples.zh-CN.md`: No kafka/es/clickhouse-specific examples found — **no changes needed**

## 3. Execution Order

The implementation follows a dependency-safe order:

| Step | File(s) | Rationale |
|------|---------|-----------|
| 1 | `internal/assets/_data/hertz/optional/kafka.go` | Template rewrite, no compile deps |
| 2 | `internal/assets/_data/hertz/optional/es.go` | Template rewrite |
| 3 | `internal/assets/_data/hertz/optional/clickhouse.go` | Template rewrite |
| 4 | `internal/assets/_data/kitex/optional/kafka.go` | Template rewrite |
| 5 | `internal/assets/_data/kitex/optional/es.go` | Template rewrite |
| 6 | `internal/assets/_data/kitex/optional/clickhouse.go` | Template rewrite |
| 7 | `internal/assets/_data/hertz/optional-config/kafka.yaml` | Config alignment |
| 8 | `internal/assets/_data/hertz/optional-config/es.yaml` | Config alignment |
| 9 | `internal/assets/_data/hertz/optional-config/clickhouse.yaml` | Config alignment |
| 10 | `internal/scaffold/infra/infra.go` | goGetDeps update |
| 11 | `internal/scaffold/infra/infra_test.go` | Test assertion updates |
| 12 | `README.md` + `README.zh-CN.md` | Documentation |

## 4. Validation Strategy

```
1. go build ./...                              — compile check (ncgo itself)
2. go vet ./...                                — static analysis
3. go test ./internal/scaffold/infra/... -count=1  — infra unit tests (focused)
4. go test ./... -count=1                      — full test suite
5. ./scripts/smoke.sh                          — end-to-end smoke
```

Templates are embedded (`go:embed`) and not compiled as Go code by ncgo itself — they are text assets rendered into downstream projects. Therefore:
- Template Go syntax correctness is validated by golden tests and smoke (which generates a project and runs `go build` on it if hz/kitex are available)
- The `go build ./...` on ncgo validates infra.go changes compile

## 5. Risk Assessment

| Risk | Mitigation |
|------|-----------|
| Template syntax error in generated Go | Golden tests + smoke.sh generate & compile |
| goGetDeps missing `go-framework` for kitex | Explicit addition in step 10; test validates |
| YAML snippet field mismatch with go-middleware tags | Manual cross-reference with go-middleware source (done above) |
| ES/ClickHouse still import original lib for types | Documented in header comments; `go get go-middleware` transitively pulls them |
| Breaking change for existing generated projects | Accepted per design doc — ncgo tracks go-tools (clean cut) |

## 6. Out of Scope

- Mono/micro/bff/rpc golden tests (kafka/es/clickhouse are optional add-ons, not in default generation)
- MCP tool changes (no schema/output changes)
- CLI flag changes (none)
- `internal/assets/_data/kitex/optional-config/` (directory does not exist; kitex projects share the same config pattern via hertz snippets or manual config)
