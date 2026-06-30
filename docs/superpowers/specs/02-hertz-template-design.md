# Hertz Template Design — per-file yaml + go-tools Integration

**Date**: 2026-06-30
**Status**: Draft
**Author**: ncgo team
**Depends on**: `01-kitex-template-go-tools-integration.md` (completed)

## 1. Goal

Create `internal/assets/_data/hertz/hertz-template/*.yaml`, aligning Hertz template maintenance with the Kitex model — per-file yaml + Go text/template format, integrated with go-tools.

## 2. Architecture

### 2.1 Current vs Target

```
当前 Hertz scaffold：
  ncgo new
    → writeTemplate(layout.yaml + data.json + package.yaml)
    → hz new（生成所有代码：handler/router/pb + main/conf/server/response）
    → template.Apply()（空操作，hertz-template 不存在）
    → addInfra（可选）
    → 结束

目标 Hertz scaffold：
  ncgo new
    → writeTemplate(layout.yaml + data.json + package.yaml)
    → writeHertzTemplate(hertz-template/*.yaml)  // NEW
    → hz new（生成 handler/router/pb + i18n）     // layout.yaml 精简
    → template.Apply()（用 hertz-template 覆盖/补充 DDD 层）
    → addInfra（可选）
    → 结束
```

### 2.2 分工原则

| 层 | 由谁生成 | 理由 |
|---|---|---|
| handler.go | hz（package.yaml） | 结构随 proto IDL 变化（HTTPMethod/Path/RequestType），ncgo 的 MethodInfo 不包含这些信息 |
| router_gen.go | hz（layout.yaml） | 路由注册依赖 proto 注解（api.get/post/put 等） |
| pb/（protobuf 生成代码） | hz | 框架强耦合 |
| i18n/ | hz（layout.yaml） | hz 已有完整的多语言工具链 |
| main.go | hertz-template | go-tools 日志初始化 |
| conf.go | hertz-template | go-framework/config |
| server.go | hertz-template | go-framework/hertz NewHTTPServer |
| data.go | hertz-template | go-middleware/db |
| usecase.go | hertz-template（loop_service） | 业务逻辑层，skip 策略保护用户修改 |
| repository.go | hertz-template（loop_service） | 数据访问层，skip 策略 |
| response.go | hertz-template | go-framework/hertz Responder |
| errcode.go | hertz-template | go-common/error |
| middleware.go | hertz-template | go-framework/hertz/middleware |
| Makefile | hertz-template | 项目构建 |

### 2.3 覆盖策略

hertz-template 的 `update_behavior: cover` 会覆盖 hz 生成的文件。策略：

- **cover**：go-tools 相关的核心文件（main.go, conf.go, server.go, response.go, errcode.go）
- **skip**：用户会修改的业务文件（usecase.go, repository.go）

layout.yaml 对应的文件需删除 body 内容（只保留空目录或简单占位），避免 hz 生成的代码和 hertz-template 冲突。

## 3. Template File List

| 模板文件 | 目标路径 | loop_service | update_behavior | 条件 |
|---|---|---|---|---|
| `main_go.yaml` | `main.go` | - | cover | 总是 |
| `conf_go.yaml` | `internal/base/conf/conf.go` | - | cover | 总是 |
| `conf_dev_yaml.yaml` | `conf/dev/conf.yaml` | - | skip | 总是 |
| `server_go.yaml` | `internal/base/server/server.go` | - | cover | 总是 |
| `data_go.yaml` | `internal/base/data/data.go` | - | cover | `WithDatabase` |
| `usecase_go.yaml` | `internal/usecase/{{ToLower .ServiceName}}/usecase.go` | ✓ | skip | 总是 |
| `repository_go.yaml` | `internal/repository/{{ToLower .ServiceName}}/repo.go` | ✓ | skip | `WithDatabase` |
| `response_go.yaml` | `internal/pkg/response/response.go` | - | cover | 总是 |
| `errcode_go.yaml` | `internal/pkg/errcode/errcode.go` | - | cover | 总是 |
| `middleware_go.yaml` | `internal/pkg/middleware/middleware.go` | - | cover | 总是 |
| `makefile_yaml.yaml` | `Makefile` | - | cover | 总是 |
| `sqlc_yaml.yaml` | `internal/db/sqlc.yaml` | - | cover | `WithDatabase` |

**共 12 个模板文件**，其中 2 个有条件（`data_go.yaml`, `repository_go.yaml`, `sqlc_yaml.yaml` 随 `WithDatabase`），2 个 loop_service（`usecase_go.yaml`, `repository_go.yaml`）。

### 3.1 和 Kitex-template 的对照

| 功能 | Kitex 模板 | Hertz 模板 | 差异 |
|---|---|---|---|
| 入口 | `main.yaml` | `main_go.yaml` | 相同 |
| 配置 | `conf.yaml` | `conf_go.yaml` | Hertz 用 `hertz.ServerConfig` |
| 服务启动 | `server.yaml` | `server_go.yaml` | Hertz 用 `NewHTTPServer` |
| 数据层 | `data.yaml` | `data_go.yaml` | 相同 |
| 用例层 | `usecase.yaml` | `usecase_go.yaml` | 相同 |
| 仓库层 | `repository.yaml` | `repository_go.yaml` | 相同 |
| 处理器 | `handler.yaml` | ❌ hz package.yaml 负责 | Hertz handler 依赖 proto 注解，hz 原生支持 |
| 路由 | ❌ 无 | ❌ hz layout.yaml 负责 | Hertz 路由依赖 proto 注解 `api.get/post` |
| 错误处理 | `rpcerror.yaml` | `errcode_go.yaml` | Hertz 用 Responder.Error，非 BizStatus |
| 响应 | ❌ 无（RPC 无此概念） | `response_go.yaml` | Hertz 独有（HTTP 统一响应） |
| 中间件 | `interceptor.yaml` | `middleware_go.yaml` | 名称不同，机制不同 |
| 客户端 | `client.yaml` | ❌ 无 | RPC 客户端，Hertz 不需要 |
| Makefile | `makefile.yaml` | `makefile_yaml.yaml` | 相同 |
| SQLC | ❌ 无 | `sqlc_yaml.yaml` | Hertz 的 sqlc 配置独立管理 |

## 4. go-tools 引用表

| 模板文件 | go-tools 包 | 用途 |
|---|---|---|
| `main_go.yaml` | `go-common/log` | 结构化日志初始化 |
| `conf_go.yaml` | `go-framework/config` + `config/hertz` | `LoadYAML[T]()`, `hertz.ServerConfig` |
| `server_go.yaml` | `go-framework/hertz` | `NewHTTPServer()`, CORS/Recovery 中间件 |
| `server_go.yaml` | `go-framework/hertz/observability` | OTel 链路追踪 |
| `data_go.yaml` | `go-middleware/db` | 数据库连接池 |
| `response_go.yaml` | `go-framework/hertz` | `Responder`, 统一 `Success`/`Error` 响应 |
| `errcode_go.yaml` | `go-common/error` | 预定义错误码 |

## 5. Template Details

### 5.1 main_go.yaml

和 kitex `main.yaml` 对称，使用 `go-common/log` 初始化结构化日志。

```yaml
path: main.go
update_behavior:
  type: cover
body: |-
  package main

  import (
      goclog "github.com/byx-darwin/go-tools/go-common/log"

      "{{.Module}}/internal/base/conf"
      "{{.Module}}/internal/base/server"
  )

  func main() {
      if err := conf.Init(); err != nil {
          goclog.L().Fatal("load config", "error", err)
      }

      cfg := conf.Get()
      if err := goclog.Init(goclog.Config{
          Level:  cfg.Log.Level,
          Format: cfg.Log.Format,
          Mode:   cfg.Log.Mode,
      }, goclog.ReleaseInfo{
          ServiceName: cfg.Server.Registry.Name,
          Environment: cfg.Env,
      }); err != nil {
          goclog.L().Fatal("init log", "error", err)
      }
      defer goclog.Close()

      server.Run()
  }
```

### 5.2 conf_go.yaml

使用 `go-framework/config.LoadYAML[T]()` 和 `go-framework/config/hertz.ServerConfig`。

关键 Import：
```go
import (
    "github.com/byx-darwin/go-tools/go-framework/config"
    hertzconfig "github.com/byx-darwin/go-tools/go-framework/config/hertz"
)
```

Config 结构体嵌入 go-tools 类型：
```go
type Config struct {
    Env       string                   `yaml:"env"`
    Debug     bool                     `yaml:"debug"`
    Server    hertzconfig.ServerConfig `yaml:"server"`   // go-tools Hertz 配置
    Database  DatabaseConfig           `yaml:"database"`
    RateLimit RateLimitConfig          `yaml:"rate_limit"`
    Auth      AuthConfig               `yaml:"auth"`
    Log       LogConfig                `yaml:"log"`
    Jaeger    *config.JaegerOption     `yaml:"jaeger"`
}
```

Config 文件格式（`conf/dev/conf.yaml`）：
```yaml
env: dev
debug: true
server:
  http:
    port: "8888"
    network: "tcp"
    mode: 0          # 0=内网IP+端口, 1=仅端口
  registry:
    name: "{{ToLower .ServiceName}}"
database:
  enabled: false
log:
  level: "info"
  format: "json"
  mode: "both"
```

### 5.3 server_go.yaml

使用 `go-framework/hertz.NewHTTPServer()`，替代当前 layout.yaml 中的自定义 server 创建逻辑。

```go
import (
    hertzframework "github.com/byx-darwin/go-tools/go-framework/hertz"
    hertzconfig "github.com/byx-darwin/go-tools/go-framework/config/hertz"
    "github.com/byx-darwin/go-tools/go-framework/hertz/middleware"
    "github.com/byx-darwin/go-tools/go-framework/hertz/observability"
)

func Run() {
    cfg := conf.Get()

    ctx := context.Background()
    h, err := hertzframework.NewHTTPServer(ctx, &hertzconfig.ServerConfig{
        Registry: cfg.Server.Registry,
        HTTP:     cfg.Server.HTTP,
        Jaeger:   cfg.Jaeger,
    })
    if err != nil {
        goclog.L().Fatal("create server", "error", err)
    }

    // Wire DDD layers
    // ...

    // Register routes
    router.Register(h)
    h.Spin()
}
```

### 5.4 response_go.yaml

使用 `go-framework/hertz.Responder` 替代当前的裸 `response.OK`/`response.Err`。

```go
import hertzframework "github.com/byx-darwin/go-tools/go-framework/hertz"

// OK writes a success response using Responder from context.
func OK(ctx *app.RequestContext, data any) {
    hertzframework.RespondFrom(ctx).Success(ctx, data)
}

// Err writes an error response using Responder from context.
func Err(ctx context.Context, c *app.RequestContext, err error, msg string) {
    hertzframework.RespondFrom(c).Error(ctx, c, err, msg)
}
```

**注意**：`package.yaml` 中的 handler.go 模板使用了 `response.OK`/`response.Err`，需保持包级函数签名兼容。

### 5.5 middleware_go.yaml

覆盖 hz 生成的中间件目录文件，使用 go-framework/hertz/middleware。

```yaml
path: internal/pkg/middleware/middleware.go
update_behavior:
  type: cover
body: |-
  package middleware

  import (
      "github.com/byx-darwin/go-tools/go-framework/hertz/middleware"
  )

  // Re-export from go-tools for generated project use.
  var (
      Cors      = middleware.Cors
      Auth      = middleware.Auth
      AccessLog = middleware.AccessLog
  )
```

### 5.6 errcode_go.yaml

使用 go-common/error 预定义错误码，和 kitex-template 对齐。

```yaml
path: internal/pkg/errcode/errcode.go
update_behavior:
  type: cover
body: |-
  package errcode

  import goerror "github.com/byx-darwin/go-tools/go-common/error"

  // Re-export predefined error codes from go-common/error.
  var (
      CodeSystem         = goerror.CodeSystem
      CodeParamInvalid   = goerror.CodeParamInvalid
      CodeAuthFailed     = goerror.CodeAuthFailed
      CodeConfigInvalid  = goerror.CodeConfigInvalid
      CodeRPCTimeout     = goerror.CodeRPCTimeout
      CodeRPCUnavailable = goerror.CodeRPCUnavailable
  )
```

### 5.7 usecase_go.yaml

和 kitex `usecase.yaml` 对称，skip 策略保护用户修改。

### 5.8 repository_go.yaml

和 kitex `repository.yaml` 对称，条件 `WithDatabase`，skip 策略。

### 5.9 data_go.yaml

条件 `WithDatabase`，使用 `go-middleware/db`。

### 5.10 makefile_yaml.yaml

和 kitex `makefile.yaml` 对称。

### 5.11 sqlc_yaml.yaml

条件 `WithDatabase`。

## 6. layout.yaml 修改

为配合 hertz-template 的 cover 策略，layout.yaml 需精简：

### 6.1 删除 body 的文件

以下文件的 body 内容从 layout.yaml 删除（改为空目录定义），由 hertz-template 负责：

| 文件 | layout.yaml 变更 |
|---|---|
| `main.go` | 删除 body，只保留目录 |
| `internal/base/conf/conf.go` | 删除 body |
| `internal/base/server/server.go` | 删除 body |
| `internal/pkg/response/response.go` | 删除 body |
| `internal/pkg/errcode/errcode.go` | 删除 body |
| `internal/pkg/middleware/*.go` | 删除 body |
| `internal/base/data/data.go` | 删除 body |
| `conf/dev/conf.yaml` | 删除 body |

### 6.2 保留的 hz 生成文件

| 文件 | 理由 |
|---|---|
| `internal/handler/*/handler.go` | hz package.yaml 模板生成，随着 proto 变化 |
| `internal/router/*.go` | hz 路由生成 |
| `internal/pb/*.go` | hz protobuf 生成 |
| `internal/pkg/i18n/*` | hz i18n 工具链完整 |
| `internal/docs/*` | hz swagger/openapi 生成 |
| `go.mod` | hz 生成 |
| `.gitignore` | hz 生成 |

## 7. scaffold 代码变更

### 7.1 writeHertzTemplate() 扩展

在 `internal/scaffold/mono/files.go` 中：
```go
func writeHertzTemplate(dir string, opts Options) error {
    tplDir := filepath.Join(dir, "template")
    // ... existing layout.yaml + data.json + package.yaml ...

    // NEW: Copy hertz-template/*.yaml from embedded assets
    hertzDir := filepath.Join(tplDir, "hertz-template")
    if err := os.MkdirAll(hertzDir, 0o755); err != nil {
        return err
    }
    srcFS := assets.FS()
    entries, err := fs.ReadDir(srcFS, "hertz/hertz-template")
    if err != nil {
        return nil // No hertz-template yet — skip
    }
    for _, e := range entries {
        if e.IsDir() { continue }
        b, _ := fs.ReadFile(srcFS, "hertz/hertz-template/"+e.Name())
        os.WriteFile(filepath.Join(hertzDir, e.Name()), b, 0o644)
    }
    return nil
}
```

### 7.2 template.Apply() 不再空操作

hz 跑完后，`template.Apply()` 读到 hertz-template 目录并执行覆盖。现有代码已支持，不需要修改。

## 8. 实施顺序

1. 创建 `internal/assets/_data/hertz/hertz-template/` 目录
2. 编写每个 yaml 文件（从 5.1 到 5.13）
3. 修改 layout.yaml（删除 body，保留 hz 特有内容）
4. 修改 `writeHertzTemplate()` 复制 yaml
5. 更新 golden tests
6. 端到端验证

## 9. 风险

1. **handler/router 不覆盖**：handler.go 和 router_gen.go 保持由 hz 生成（package.yaml + layout.yaml），需要确保 hz 生成的 handler 签名和 response 包兼容。
2. **response 包 API 兼容性**：`package.yaml` 的 handler.go 模板使用 `response.OK(c, resp)` / `response.Err(c, err)`。hertz-template 覆盖 response.go 时必须保持这些包级函数的签名不变。
3. **layout.yaml 精简风险**：layout.yaml 350KB+，精简需仔细，避免误删 hz 依赖的条目。
4. **go-framework/hertz.NewHTTPServer 依赖**：需确认 `go-framework/hertz` 和 `go-framework/kitex/rpcerror` 之间没有 genproto 冲突。当前 kitex option 有 build ignore 的问题，但 hertz 的 NewHTTPServer 使用的是 `go.opentelemetry.io/otel` 标准库，应无冲突。

## 10. 成功标准

1. `ncgo new --mode mono --kind hertz --name test-svc` 生成可编译运行的项目
2. 生成的项目使用 go-tools 的 log/config/server/response/errcode/middleware
3. 模板格式和 kitex-template 一致（per-file yaml + Go text/template）
4. golden tests 通过
5. `make dev` 可以启动服务
