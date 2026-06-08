# Hertz / Kitex Unified Logging Design

## 1. Philosophy

Logging should be provided as a unified infrastructure capability, not scattered across Hertz handlers, Kitex handlers, usecases, or repositories.

Recommended model:

```text
Business code / handler / usecase / repository
        |
        |-- return or wrap samber/oops errors
        v
Unified logging helper
        |
        |-- console writer
        |-- typed file writers
        |-- rotated + compressed files
        |-- trace / request / release / canary context enrichment
        v
Log collection system / local files / console
```

Core principles:

- Business code uses `github.com/samber/oops` for error semantics.
- Logging layer parses `oops` and handles categorization, structuring, writing, compression, and trace correlation.
- LoongSuite Go Agent handles trace / metrics auto-instrumentation; logging only reads trace context, does not initialize OTel SDK.
- Container environments default to console output; VM / bare metal can enable file + compress.

## 2. Current State

Default ncgo output (without `ncgo add infra logging`):

| Capability | Status |
|---|---|
| Hertz access log | Yes, via `hlog` |
| Kitex access log | Yes, via `klog` |
| request_id | Yes |
| Console output | Yes |
| File output | No |
| File compression | No |
| Log categorization | Not systematic |
| JSON structured | No |
| trace_id / span_id | Not standardized |
| oops structured parsing | No |
| canary / release metadata | No |

Conclusion: currently **console available, files absent, trace weak, error semantics unstructured**.

## 3. `observability_logging` Optional MVP

`ncgo add infra observability_logging` generates:

### Core module: `internal/base/logging/logging.go`

- `Config`: enabled, mode (console/file/both/none), format (text/json), level, add_source
- `ConsoleConfig`: enabled toggle
- `FileConfig`: enabled, dir, filename, max_size_mb, max_backups, max_age_days, compress (gzip via lumberjack)
- `CategoryConfig`: per-category enabled, file output, level override
- `Init()`: returns `*slog.Logger` during init
- `Handler()`: returns singleton `slog.Handler`
- `ReleaseInfo`: ServiceName, ServiceKind, Version, Track, GitSHA, BuildTime

### Service-specific adapters:

**Hertz (`internal/base/logging/hertz.go`):**
- `HertzRequestID()`: extracts or generates request_id for Hertz context
- `HertzAccessLog()`: access log with request_id, method, path, status
- `HertzRecovery()`: panic recovery with request context in log

**Kitex (`internal/base/logging/kitex.go`):**
- `KitexRequestID()`: extracts or generates request_id for Kitex context
- `KitexAccessLog()`: access log with request_id, method, service
- `KitexRecovery()`: panic recovery with request context in log

### Dependencies

- `log/slog` (stdlib)
- `gopkg.in/natefinch/lumberjack.v2` (file rotation)
- `github.com/samber/oops` (structured error parsing)
- LoongSuite trace context (optional, read-only)

## 4. Log Format

### Console mode (text):

```
2026-06-09T10:30:00.000Z INFO access_log request_id=abc123 method=GET path=/api/users status=200
```

### File mode (JSON):

```json
{"time":"2026-06-09T10:30:00.000Z","level":"INFO","msg":"access_log","request_id":"abc123","method":"GET","path":"/api/users","status":200}
```
