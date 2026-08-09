# Go 工程师评估报告

> 角色：Go 后端工程师（技术准确性审查）
> 审查对象：`internal/assets/_data/docs/hertz/design-doc.en.md` 的 `#### Structured Logging` 章节、
> `internal/assets/_data/docs/kitex/design-doc.en.md` 的 `#### Structured Logging` 与 `#### Polaris Canary Adapter` 章节。
> 交叉验证源码：`_data/hertz/optional/observability_logging.go`、`_data/kitex/optional/observability_logging.go`、
> `_data/optional/observability_logging.go`、`_data/kitex/optional/polaris_canary_adapter.go`，以及
> conf 模板（`hertz/hertz-template/conf_go.yaml`、`kitex/kitex-template/conf.yaml`）、
> `internal/scaffold/infra/infra.go` 输出映射、`go-common@v0.1.0/log` 实际 API。
> 日期：2026-08-09

## 评分（1-5）

- 函数签名准确性: 4/5
- 依赖声明准确性: 5/5
- 配置字段准确性: 4/5
- 代码示例正确性: 2/5
- 错误码/分类准确性: 4/5

## 准确的部分

### Hertz Structured Logging（`docs/hertz/design-doc.en.md`）

- 文件布局准确：`internal/base/logging/logging.go`（共享 helpers）+ `internal/base/logging/hertz.go`（Hertz 专属）
  与 `internal/scaffold/infra/infra.go:107` 及 golden testdata
  （`internal/scaffold/infra/testdata/infra-logging-hertz/{logging.go,hertz.go}`）一致。
- 别名准确：`ncgo add infra observability_logging`（alias `logging`）与 `infra.go:39` `KindLoggingAlias = "logging"`、`normalizeKind` 一致。
- 四个公开函数签名全部与源码一致：
  - `HertzRequestID() app.HandlerFunc`、`HertzAccessLog() app.HandlerFunc`、`HertzRecovery() app.HandlerFunc`、
    `HertzRequestIDFromContext(c *app.RequestContext) string`。
- 行为描述准确：`HertzRequestID`（X-Request-ID → OTel span trace ID → 16-byte hex、写响应头、传播 X-Traffic-Lane 到 context 与 Hertz local storage）、
  `HertzAccessLog`（`goclog.Access` + `http.method/http.path/http.status_code/latency_ms`）、
  `HertzRecovery`（`goclog.L().WithCategory(CategoryPanic)` + panic 值 + stack + HTTP 500）均与源码逐条吻合。
- `InitFromConf(cfg.Logging, ...)` 的配置入参名正确：hertz 根 `Config` 字段为 `Logging LoggingConfig \`yaml:"logging"\``（`conf_go.yaml:48`），源码签名 `InitFromConf(cfg conf.LoggingConfig, release goclog.ReleaseInfo) error`。
- conf.yaml 字段名全部匹配 `LoggingConfig`/`LoggingFileConfig` 的 yaml tag（`level/format/mode/add_source/file.dir/file.filename/file.max_size_mb/file.max_backups/file.max_age_days/file.compress`，见 `conf_go.yaml:294-317`）。
- 8 个 category 常量（Access/Error/Biz/RPC/DB/Panic/Audit/Security）在 `go-common@v0.1.0/log/category.go` 全部存在，共享 `logging.go` 的 re-export 与 `_data/optional/observability_logging.go` 一致。
- 依赖声明准确：`go-common`（log/error）、`go.opentelemetry.io/otel/trace`。

### Kitex Structured Logging（`docs/kitex/design-doc.en.md`）

- 文件布局准确：`logging.go` + `kitex.go`（testdata `infra-logging-kitex/` 一致）。
- 拦截器函数签名全部与源码一致：`KitexRequestID()/KitexAccessLog()/KitexRecovery() endpoint.Middleware`、`KitexMetaValue(ctx, key) string`。
- 行为描述准确：`KitexRequestID`（metainfo → OTel trace ID → 16-byte hex、`metainfo.WithPersistentValue`、传播 `x-traffic-lane`）、
  `KitexRecovery`（`goclog.L().WithCategory(CategoryPanic)` + panic 值）、`KitexMetaValue`（persistent → transient）均与源码吻合。
- `InitFromConf(cfg.Log, ...)` 入参名正确：kitex 根 `Config` 字段为 `Log LogConfig \`yaml:"log"\``（`conf.yaml:38`），源码签名 `InitFromConf(cfg conf.LogConfig, release goclog.ReleaseInfo) error`。
- kitex conf.yaml（`level/format/mode` 三键）与 `LogConfig` 结构体完全一致。
- 依赖声明准确：`go-common`、`github.com/bytedance/gopkg`（metainfo）、`go.opentelemetry.io/otel`。

### Kitex Polaris Canary Adapter（`docs/kitex/design-doc.en.md`）

- 文件名表述准确：add-on kind 为 `polaris_adapter`，输出路径 `internal/base/release/polaris_adapter.go`（`infra.go:109`），模板源码为 `kitex/optional/polaris_canary_adapter.go`（`infra.go:307`）。
- "唯一 import polaris-go 的文件 / ncgo 本身不依赖 SDK" 成立：同伴文件 `polaris_canary_observer_otel.go` 仅 import `go.opentelemetry.io/otel*`，不 import polaris-go。
- 三个构造函数签名与行为准确：`NewPolarisInstanceLister`（`ConsumerAPI.GetAllInstances`）、
  `NewPolarisRuleLoader`（`ConfigAPI.GetConfigFile` 读 YAML RuleSet）、`NewPolarisSelector`（组装 `release.Selector`）。
- `PolarisDiscoveryConfig{Addresses,Namespace,Service}` 与 `PolarisRuleConfig{Addresses,Namespace,Group,FileName}` 字段名与
  `_data/optional/release_canary.go:300-328` 完全一致。
- 环境变量 `POLARIS_TOKEN`（空 = 无鉴权）/ `POLARIS_NAMESPACE`（cfg.Namespace 为空时兜底）准确。
- Enable 命令（`go get polaris-go` + `go get gopkg.in/yaml.v3`）与源码顶部注释一致；v1.7.1 版本号一致。
- Usage 代码示例字段名与语法正确，`err` 返回值与源码 `(Selector, error)` 一致。
- 依赖声明（polaris-go、yaml.v3、go-common）准确。

## 不准确/存疑的部分

### 严重（会导致生成的示例代码编译失败）

1. **[Hertz `design-doc.en.md:423` 与 Kitex `design-doc.en.md:348`] `goclog.ReleaseInfo` 没有 `ServiceKind` 字段。**
   文档示例 `goclog.ReleaseInfo{ ServiceName: "my-api", ServiceKind: "hertz", Version: release.Version }` 引用了不存在的字段。
   实测 `go-common@v0.1.0/log/release.go`（以及本地更新的 go-common checkout）的 `ReleaseInfo` 只有
   `ServiceName/Version/GitSHA/BuildTime/Environment/Extra`。生成项目 `go.mod` 固定 `go-common v0.1.0`（`hertz/layout.yaml:90`），
   因此 `ServiceKind` 在生成的 Hertz/Kitex 项目里都是编译错误。生成模板自身（`main_go.yaml`）只填 `ServiceName` + `Environment`。

2. **[Hertz `design-doc.en.md:448`] `goclog.Biz(ctx)` 不存在。**
   `go-common@v0.1.0/log/layer.go` 只有 `App/DB/Access/RPC/MQ/Cache` 分层 Logger，没有 `Biz` 函数（本地 checkout 同样没有）。
   文档的 handler 用法示例会编译失败。应改为 `goclog.App(ctx)`（或按分类选择其它层）。

3. **[Hertz `design-doc.en.md:424` 与 Kitex `design-doc.en.md:349`] `release.Version` 在生成项目里无定义。**
   hertz/kitex 基础模板中不存在 `release` 包或 `Version` 变量（`release` 包只有在 `ncgo add infra canary`/`polaris_adapter` 之后才出现）。
   初始化示例未给出 `release` 的 import，照抄无法编译。

### 中等问题（行为描述与源码不符 / 配置值非法）

4. **[Kitex `design-doc.en.md:336`] `KitexAccessLog()` 失败日志 "with `rpcerror.FormatBiz(err)`" 不准确。**
   源码（`kitex/optional/observability_logging.go:59` 及 golden testdata `infra-logging-kitex/kitex.go:59`）
   直接把原始 `err` 传给 `goclog.RPC(ctx).ErrorContext(ctx, "kitex rpc failed", err, ...)`，并未调用 `rpcerror.FormatBiz`。
   基础模板 `interceptor.yaml:42` 的 `AccessLog` 委托给 `compat.AccessLog()`，同样不经过 `FormatBiz`。

5. **[Hertz `design-doc.en.md:433` 与 Kitex `design-doc.en.md:358`] `mode: production` / 注释 `# production | development` 非法。**
   `goclog.Config.Mode` 的合法值只有 `"console" | "file" | "both"`（`go-common@v0.1.0/log/config.go`）。
   `production`/`development` 不是合法值，`NewLogger` 会落到 default 分支静默当 `console` 处理。
   且 hertz 默认 `conf_dev_yaml.yaml` 的 logging 块是 `mode: console`，与文档示例自相矛盾。

### 次要问题

6. **[Hertz `design-doc.en.md:464` / Kitex `design-doc.en.md:522`] §4 Files 表不完整。**
   hertz 表只列 `{redis,kafka,es,clickhouse}.go`，漏了 `observability_logging.go`、`release_canary.go`、`redis_shared.go`；
   kitex 表只列 `{redis,kafka,es,clickhouse,registry_polaris}.go`，漏了 `observability_logging.go`、
   `polaris_canary_adapter.go`、`polaris_canary_observer_otel.go`、`release_canary.go`。与 §6/§7 "Currently shipped" 的清单不一致。

7. **[Kitex `design-doc.en.md:400-407`] "Provides" 小节省略了 `NewPolarisInstanceLister`/`NewPolarisRuleLoader` 的 `error` 返回值**
   （源码签名 `(PolarisInstanceLister, error)`、`(PolarisRuleLoader, error)`）。不算错误，但建议补全签名以便使用者处理错误。

8. **[Kitex `design-doc.en.md:269` §3.7 开头] "Kitex add-ons 用 string `goerror.Code`，不像 Hertz 用数字 errcode registry" 的概括对 logging add-on 不完全成立。**
   Hertz 的 logging add-on（`HertzRecovery`）同样用 string code `"panic_recovered"`，只有 hertz 的 data add-ons（redis/kafka/es/clickhouse）用数字码。

## 改进建议

- **删除或替换 `ServiceKind`**：`goclog.ReleaseInfo` 无此字段。初始化示例建议改为
  ```go
  logging.InitFromConf(cfg.Logging, goclog.ReleaseInfo{
      ServiceName: cfg.Server.Registry.Name, // 参考生成 main_go.yaml 的用法
      Environment: cfg.Env,
  })
  ```
  （kitex 侧把 `cfg.Logging` 换成 `cfg.Log`）。若文档想表达"服务类型"信息，需先给 `ReleaseInfo` 加字段并 bump go-common 版本，否则不要写。
- **把 `goclog.Biz(ctx)` 改为 `goclog.App(ctx)`**（或注明"按需选择 `App/DB/RPC/...` 分层 Logger"）。
- **`release.Version` 要么补全 import 与前置条件（例如先 `ncgo add infra canary`），要么换成已存在的字段**（如 `cfg.Env`），避免照抄即编译失败。
- **删除 `KitexAccessLog` 描述中的 `rpcerror.FormatBiz(err)`**，如实写"以 ERROR 级别记录原始错误（附带 rpc.system/rpc.service/rpc.method/latency_ms）"。
- **修正 mode 取值**：把 `mode: production` 与注释 `# production | development` 改为 `mode: console`（合法值 `console | file | both`），与 go-common 文档注释及默认 conf 一致。
- **补全 §4 Files 表**，纳入 `observability_logging.go`、`polaris_canary_adapter.go` 等新增 optional 文件，与 §6/§7 对齐；或直接指向 `optional/` 目录并声明"清单以目录为准"。
- 在 `Provides` 小节补全 `(T, error)` 返回，并顺带核对 `NewPolarisSelector` 的 error 分支。

## 验证方法（本次评估实际执行）

- 逐文件 diff：文档函数名/参数 vs `observability_logging.go`（hertz/kitex/shared）与 `polaris_canary_adapter.go`。
- 字段名核对：conf 模板 `conf_go.yaml`（Hertz `LoggingConfig`/`LoggingFileConfig`）与 `kitex/kitex-template/conf.yaml`（`LogConfig`）。
- API 核实：`~/go/pkg/mod/github.com/byx-darwin/go-tools/go-common@v0.1.0/log/` 的 `release.go/config.go/layer.go/category.go/new_logger.go`
  （确认无 `ServiceKind`、无 `Biz`、`Mode` 合法值为 `console|file|both`、category 常量齐全、`Logger` 内嵌 `*slog.Logger` 提供 `InfoContext`）。
- 输出映射核实：`internal/scaffold/infra/infra.go:96-109,473-494` 与 golden testdata `infra-logging-{hertz,kitex}/`、`infra-polaris-adapter/`。
- 生成项目符号核实：hertz `main_go.yaml`（`ReleaseInfo` 只填 `ServiceName`/`Environment`）、kitex `server.yaml`/`main.yaml`（无 `release.Version`）。
