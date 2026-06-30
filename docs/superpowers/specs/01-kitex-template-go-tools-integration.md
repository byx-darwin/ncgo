# Kitex Template go-tools Integration Design

**Date**: 2026-06-30
**Status**: Draft
**Author**: ncgo team

## 1. Goal

Update existing kitex-template (`internal/assets/_data/kitex/kitex-template/*.yaml`) to integrate `go-tools` packages where they add value, while preserving existing functionality and avoiding breaking changes.

## 2. Background

Current kitex-template files are "bare" implementations:
- Use standard `log` package (no structured logging)
- Use `samber/oops` directly (no unified error code registry)
- Config loading is custom (no reuse of go-framework/config)
- Interceptors are custom (no reuse of go-framework/kitex/middleware)

go-tools provides layered packages:
```
go-common/log        → Structured logging (zap-based)
go-common/error      → Error codes (10000-59999), oops integration
go-framework/config  → LoadYAML[T](), common config types
go-framework/config/kitex → ServerConfig, ClientConfig types
go-framework/kitex/rpcerror → Error classification, BizStatus adapter
go-framework/kitex/middleware → AccessLog middleware
go-framework/kitex/observability → OTel tracing provider
go-middleware/db     → database/sql wrapper (NOT pgx)
```

## 3. Constraints

### 3.1 go-framework/kitex/option is disabled
`option.go` has `//go:build ignore` due to genproto conflicts between kitex SDK and otel. We **cannot** use `option.NewServerOption()` yet. Server wiring must remain custom.

### 3.2 go-middleware/db uses database/sql, not pgx
Current kitex template uses `pgx/v5/pgxpool` for PostgreSQL-specific features (Batch, CopyFrom, LISTEN/NOTIFY). We **should not** replace it with `go-middleware/db` which wraps `database/sql`.

### 3.3 Config type differences
go-tools uses `time.Duration` fields (D2 decision), current templates use `int` seconds. Migration requires config file format changes.

### 3.4 Preserve ncgo anchors
Server.yaml has `// ncgo:wire:logging:init`, `// ncgo:wire:canary:server-traffic`, etc. These anchors must be preserved for `ncgo add infra` to work.

## 4. Template Changes

### 4.1 main.yaml

**Change**: Use `go-common/log` for structured logging initialization.

**Before**:
```go
import "log"

func main() {
    if err := conf.Init(); err != nil {
        log.Fatalf("load config: %+v", err)
    }
    server.Run()
}
```

**After**:
```go
import (
    "log"

    goclog "github.com/byx-darwin/go-tools/go-common/log"
    "{{.Module}}/internal/base/conf"
    "{{.Module}}/internal/base/server"
)

func main() {
    if err := conf.Init(); err != nil {
        log.Fatalf("load config: %+v", err)
    }

    cfg := conf.Get()
    if err := goclog.Init(goclog.Config{
        Level:  "info",
        Format: "json",
    }, goclog.ReleaseInfo{
        ServiceName: cfg.Server.Name,
        Environment: cfg.Env,
    }); err != nil {
        log.Fatalf("init log: %+v", err)
    }
    defer goclog.Close()

    server.Run()
}
```

**Benefits**: Structured JSON logging, log rotation, masking support.

### 4.2 conf.yaml

**Changes**:
1. Use `go-framework/config.LoadYAML[T]()` for loading.
2. Embed `go-framework/config/kitex.ServerConfig` for the `Server` field.
3. Keep domain-specific types (RateLimitConfig, AuthConfig, DatabaseConfig).

**Before**:
```go
import (
    "os"
    "path/filepath"
    "sync"
    "github.com/samber/oops"
    "gopkg.in/yaml.v3"
)

type Config struct {
    Env    string       `yaml:"env"`
    Server ServerConfig `yaml:"server"`
    // ...
}

type ServerConfig struct {
    Name                    string `yaml:"name"`
    Addr                    string `yaml:"addr"`
    ReadWriteTimeoutSeconds int    `yaml:"read_write_timeout_seconds"`
    ExitWaitTimeSeconds     int    `yaml:"exit_wait_time_seconds"`
}

func Load() (*Config, error) {
    // Custom loading logic
}
```

**After**:
```go
import (
    "os"
    "path/filepath"
    "sync"

    "github.com/byx-darwin/go-tools/go-framework/config"
    kitexconfig "github.com/byx-darwin/go-tools/go-framework/config/kitex"
    "github.com/samber/oops"
)

type Config struct {
    Env       string                   `yaml:"env"`
    Debug     bool                     `yaml:"debug"`
    Server    kitexconfig.ServerConfig `yaml:"server"`  // Embedded from go-tools
    Database  DatabaseConfig           `yaml:"database"`
    RateLimit RateLimitConfig          `yaml:"rate_limit"`
    Auth      AuthConfig               `yaml:"auth"`
    Log       LogConfig                `yaml:"log"`
    Jaeger    *config.JaegerOption     `yaml:"jaeger"`  // OTel/Jaeger tracing config
}

type LogConfig struct {
    Level  string `yaml:"level"`
    Format string `yaml:"format"`
}

func Load() (*Config, error) {
    cfg := Default()
    path := os.Getenv("CONFIG_PATH")
    fromEnv := path != ""
    if path == "" {
        path = filepath.Join("conf", Env(), "conf.yaml")
    }
    // Use LoadYAML to parse, then merge onto defaults
    loaded, err := config.LoadYAML[Config](path)
    if err != nil {
        if os.IsNotExist(err) && !fromEnv {
            return cfg, nil
        }
        return nil, oops.In("config").Code(10308).Public("config_invalid").With("path", path).Wrap(err)
    }
    // Overwrite defaults with loaded values (zero values in loaded keep defaults)
    *cfg = *loaded
    if err := cfg.Validate(); err != nil {
        return nil, err
    }
    return cfg, nil
}
```

**Config file changes** (`conf/dev/conf.yaml` template — `conf_dev.yaml`):

```yaml
# Before (int seconds)
env: dev
server:
  name: "myservice"
  addr: ":8888"
  read_write_timeout_seconds: 30
  exit_wait_time_seconds: 10

# After (time.Duration strings, aligns with kitexconfig.ServerConfig)
env: dev
server:
  rpc:
    port: ":8888"
    network: "tcp"
  registry:
    name: "myservice"
  timeout:
    read_write_timeout: "30s"
    exit_wait_timeout: "10s"
log:
  level: "info"
  format: "json"
database:
  enabled: false
```

**Note**: The `Default()` function in `conf.yaml` must be updated to construct `kitexconfig.ServerConfig` with the new field structure (RPC/Timeout/Registry sub-structs instead of flat fields).

**Benefits**: Reuse config loading, align config types with go-tools ecosystem.

### 4.3 server.yaml

**Changes**:
1. Use `go-framework/kitex/observability` for OTel tracing.
2. Keep custom wiring (can't use go-framework/kitex/option due to build ignore).
3. Preserve ncgo anchors.
4. **Config field migration**: Update field access from flat `cfg.Server.Addr` to `cfg.Server.RPC.Port`, from `cfg.Server.ReadWriteTimeoutSeconds` to `cfg.Server.Timeout.ReadWriteTimeout`, etc.

**Key code changes in server.yaml body**:

```go
// Before
addr, err := net.ResolveTCPAddr("tcp", cfg.Server.Addr)
kitexserver.WithReadWriteTimeout(durationSeconds(cfg.Server.ReadWriteTimeoutSeconds))
kitexserver.WithExitWaitTime(durationSeconds(cfg.Server.ExitWaitTimeSeconds))
kitexserver.WithServerBasicInfo(&rpcinfo.EndpointBasicInfo{ServiceName: cfg.Server.Name})

// After
addr, err := net.ResolveTCPAddr("tcp", cfg.Server.RPC.Port)
kitexserver.WithReadWriteTimeout(cfg.Server.Timeout.ReadWriteTimeout)   // Already time.Duration
kitexserver.WithExitWaitTime(cfg.Server.Timeout.ExitWaitTimeout)        // Already time.Duration
kitexserver.WithServerBasicInfo(&rpcinfo.EndpointBasicInfo{ServiceName: cfg.Server.Registry.Name})
```

Since `kitexconfig.ServerConfig` already uses `time.Duration` fields, the `durationSeconds()` helper in server.yaml is no longer needed for server config (but keep it for RPC request timeout if that stays as `int`).

**Observability integration** (new code block in server.yaml):
```go
import (
    "github.com/byx-darwin/go-tools/go-framework/kitex/observability"
    "github.com/byx-darwin/go-tools/go-framework/config"
)

func Run(extraOptions ...kitexserver.Option) {
    cfg := conf.Get()

    // OTel tracing (new)
    if cfg.Jaeger != nil && cfg.Jaeger.Enable {
        provider, err := observability.NewProvider(ctx, config.ObservabilityConfig{
            Enabled:     true,
            Endpoint:    cfg.Jaeger.Endpoint,
            ServiceName: cfg.Server.Registry.Name,
        })
        if err != nil {
            log.Fatalf("init observability: %+v", err)
        }
        defer provider.Shutdown()
        // Append OTel middleware to opts
    }

    // ... rest of wiring with ncgo anchors preserved ...
}
```

**Benefits**: OTel tracing integration.

### 4.4 interceptor.yaml

**Changes**:
1. Use `go-framework/kitex/middleware.AccessLog()` for access logging.
2. Keep custom interceptors (RequestID, CallerAllowlist, Recovery, RequestTimeout).

**Before**:
```go
func AccessLog() endpoint.Middleware {
    // Custom implementation
}
```

**After**:
```go
import (
    "github.com/byx-darwin/go-tools/go-framework/kitex/middleware"
)

// AccessLog delegates to go-framework/kitex/middleware
func AccessLog() endpoint.Middleware {
    return middleware.AccessLog()
}
```

**Benefits**: Reuse standardized access log format.

### 4.5 rpcerror.yaml

**Changes**:
1. Use `go-common/error` for predefined error codes and helpers.
2. Use `go-framework/kitex/rpcerror` for BizStatus adapter.
3. Keep ToBizError() but simplify using go-common/error.

**Before**:
```go
import (
    "github.com/cloudwego/kitex/pkg/kerrors"
    "github.com/samber/oops"
)

const (
    CodeInternalError  int32 = 10000
    CodeRPCTimeout     int32 = 10011
)

func ToBizError(err error) error {
    // Custom implementation
}
```

**After**:
```go
import (
    goerror "github.com/byx-darwin/go-tools/go-common/error"
    "github.com/byx-darwin/go-tools/go-framework/kitex/rpcerror"
    "github.com/cloudwego/kitex/pkg/kerrors"
)

// Use predefined codes from go-common/error
var (
    CodeInternalError  = goerror.CodeSystem       // 10000
    CodeRPCTimeout     = goerror.CodeRPCTimeout   // 10011
    CodeConfigInvalid  = goerror.CodeConfigInvalid // 10004
)

func ToBizError(err error) error {
    if err == nil {
        return nil
    }
    if _, ok := kerrors.FromBizStatusError(err); ok {
        return err
    }
    // Use OopsStatusAdapter from go-framework/kitex/rpcerror
    adapter := &rpcerror.OopsStatusAdapter{Err: err}
    return kerrors.NewBizStatusError(adapter.BizStatusCode(), adapter.BizMessage())
}
```

**Benefits**: Unified error codes across projects, reuse BizStatus adapter.

### 4.6 data.yaml

**No changes**. Keep `pgx/v5/pgxpool` for PostgreSQL-specific features.

### 4.7 Other templates

No changes to:
- `handler.yaml`, `usecase.yaml`, `repository.yaml` (business logic)
- `client.yaml`, `client_test.yaml` (RPC client)
- `makefile.yaml` (build targets)
- `migration_*.yaml` (database migrations)
- `ratelimit_*.yaml` (rule-center preset)
- `conf_dev.yaml` (config file format will change due to conf.yaml changes)

## 5. Migration Path

### 5.1 Backward compatibility

The config file format changes (int seconds → time.Duration strings). To ease migration:

1. Update `conf.yaml` template to accept both formats during transition.
2. Document the new format in generated project's README.
3. Existing projects can keep old format until they regenerate.

### 5.2 Testing

1. **Golden tests**: Update `internal/scaffold/mono/testdata/` snapshots.
2. **Integration tests**: Generate a Kitex project, verify it compiles and runs.
3. **Manual validation**: Run `ncgo new --mode mono --kind kitex`, check generated code.

## 6. Risks

1. **go-framework/kitex/option is disabled**: We can't use it yet. Server wiring remains custom.
2. **Config format change**: Existing projects need manual migration.
3. **go-tools version**: Generated projects use `@latest`, which may introduce breaking changes. Consider pinning versions in the future.

## 7. Success Criteria

1. Generated Kitex projects use `go-common/log` for logging.
2. Generated Kitex projects use `go-framework/config.LoadYAML[T]()` for config loading.
3. Generated Kitex projects use `go-framework/config/kitex.ServerConfig` for server config.
4. Generated Kitex projects use `go-framework/kitex/observability` for tracing.
5. Generated Kitex projects use `go-framework/kitex/middleware.AccessLog()` for access logging.
6. Generated Kitex projects use `go-common/error` for error codes.
7. Generated Kitex projects use `go-framework/kitex/rpcerror.OopsStatusAdapter` for BizStatus conversion.
8. All golden tests pass.
9. Generated project compiles and runs without errors.

## 8. Implementation Order

1. Update `main.yaml` (logging init)
2. Update `conf.yaml` (config loading + types)
3. Update `conf_dev.yaml` (new config format)
4. Update `rpcerror.yaml` (error codes + BizStatus)
5. Update `interceptor.yaml` (access log)
6. Update `server.yaml` (observability)
7. Update golden tests
8. Integration test

## 9. Out of Scope

- Updating Hertz templates (separate design doc)
- Using `go-framework/kitex/option` (blocked by build ignore)
- Using `go-middleware/db` (incompatible with pgx)
- Version pinning for go-tools (use @latest for now)
