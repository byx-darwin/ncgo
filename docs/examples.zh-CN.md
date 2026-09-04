# ncgo 示例工作流

这些示例面向低风险起步场景。Mono 示例都使用了 `--no-generate`，这样你可
以先检查生成出来的输入文件，再决定何时执行 `hz` 或 `kitex`。

## 0. MCP contract-first 参考

先用下面的命令启动 stdio MCP server：

```bash
ncgo mcp serve
```

### 通用返回约定

- `content[0].text` 是面向人类阅读或直接转存的载荷。
- 对结构化工具来说，同级顶层字段是给 Agent 稳定消费的 machine-readable payload。
- `output` 默认是 `text`；部分工具还支持 `json` 或 `sarif`。
- `output` 只影响 `content[0].text` 的格式，不会移除顶层字段。
- `isError` 跟随阻断状态。对带 `ok` 的工具来说，它等价于 `!ok`；因此 warning-only 的 lint / doctor 场景会保持 `isError=false`。

### 各工具 contract

- `ncgo_version`
  - 输入：无
  - output：text
  - 稳定结果形态：版本 / build / assets 摘要写在 `content[0].text`
- `ncgo_doctor`
  - 输入：`root`（可选）、`output=text|json|sarif`
  - 稳定顶层字段：`root`、`scope`、`summary`、`checks`、`ok`
- `ncgo_ai_init_claude`
  - 输入：`root`，以及可选的 `preset=minimal|team`、`force`、`dryRun`、`output=text|json`
  - 稳定顶层字段：`written`、`skipped`，以及可选的 `notes`、`nextSteps`
  - `content[0].text` 在 `output=text` 时返回人类可读摘要，在 `output=json` 时返回 JSON
- `ncgo_ai_sync`
  - 输入：`root`、`target=all|agents|claude|cursor`（默认 `claude`）、
    `lang=en|zh-CN`、`force`、`dryRun`、`output=text|json`
  - 稳定顶层字段：`target`、`written`、`skipped`，以及可选的 `notes`、`scope`、`sourceRef`、`workspace`
  - `content[0].text` 在 `output=text` 时返回人类可读摘要，在 `output=json` 时返回 JSON
- `ncgo_ai_context`
  - 输入：`root`、`output=text|json`
  - 稳定顶层字段：`root`、`domains`、`methods`、`anchors`、`issues`
  - `content[0].text` 在 `output=text` 时返回人类可读的扫描摘要，在 `output=json` 时返回 JSON payload
- `ncgo_i18n_report`
  - 输入：`root`、`output=text|json`
  - 稳定顶层字段：`root`、`sourceLocale`、`localesDir`、`statusPath`、`glossaryPath`、`reportPathJSON`、`reportPathMarkdown`、`schema`、`report`、`nextSteps`
- `ncgo_i18n_check`
  - 输入：`root`、`mode=dev|release`（默认 `dev`）、`output=text|json`
  - 稳定顶层字段：`root`、`mode`、`ok`、`sourceLocale`、`schema`、`summary`、`failures`、`warnings`、`nextSteps`
- `ncgo_protolint`
  - 输入：`root`，以及可选的 `files`、`rules`、`ignoreRules`、`ignoreFiles`、`output=text|json|sarif`
  - 稳定顶层字段：`root`、`files`、`rulesRun`、`ignoredRules`、`ignoredFiles`、`ok`、`summary`、`diagnostics`
- `ncgo_add_infra`
  - 输入：`root`、`kind`，以及可选的 `force`、`wire`、`dryRun`、`output=text|json`
  - 稳定顶层字段：`dryRun`、`updated`、`writtenPath`、`writtenPaths`、`wiredPaths`、`nextSteps`、`plan`
- `ncgo_add_method`
  - 输入：`root`、`spec=<domain>.<Method>`、`in=usecase`、`output=text|json`
  - 稳定顶层字段：`path`、`domain`、`method`、`nextSteps`
  - `content[0].text` 在 `output=text` 时返回插入摘要，在 `output=json` 时返回 JSON
- `ncgo_add_rule_center`
  - 输入：`root`、`addr`，以及可选的 `force`、`dryRun`、`output=text|json`
  - 稳定顶层字段：`dryRun`、`writtenPaths`、`nextSteps`
- `ncgo_new`
  - 输入：`name`、`module`，以及可选的 `dir`、`mode`、`kind`、`db`、
    `infra`、`noGenerate`、`preset`、`ruleCenterAddr`、`output=text|json`
  - 稳定顶层字段：`dir`、`mode`、`nextSteps`、`ranGenerate`

后面的 workflow 会直接引用这份 contract，不再在每个场景里重复解释同一套传输约定。

### 最小 `tools/call` 请求骨架

`ncgo_doctor`

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "tools/call",
  "params": {
    "name": "ncgo_doctor",
    "arguments": {
      "root": ".",
      "output": "json"
    }
  }
}
```

推荐优先读取的顶层字段：先看 `ok`、`summary`、`checks`；如果需要路由或标注结果，再结合 `root` 和 `scope`。

`ncgo_i18n_report`

```json
{
  "jsonrpc": "2.0",
  "id": 2,
  "method": "tools/call",
  "params": {
    "name": "ncgo_i18n_report",
    "arguments": {
      "root": ".",
      "output": "json"
    }
  }
}
```

推荐优先读取的顶层字段：先看 `report`、`sourceLocale`、`nextSteps`；真正让 Agent 缩小修改范围时，优先用 `report.missing_translations`、`report.stale_translations`、`report.draft_translations`。

`ncgo_i18n_check`

```json
{
  "jsonrpc": "2.0",
  "id": 3,
  "method": "tools/call",
  "params": {
    "name": "ncgo_i18n_check",
    "arguments": {
      "root": ".",
      "mode": "release",
      "output": "json"
    }
  }
}
```

推荐优先读取的顶层字段：先看 `ok`、`failures`、`warnings`、`nextSteps`；`summary` 可作为紧凑状态头使用。

`ncgo_protolint`

```json
{
  "jsonrpc": "2.0",
  "id": 4,
  "method": "tools/call",
  "params": {
    "name": "ncgo_protolint",
    "arguments": {
      "root": ".",
      "files": ["idl/app/demo.proto"],
      "output": "json"
    }
  }
}
```

推荐优先读取的顶层字段：先看 `ok`、`summary`、`diagnostics`；如果需要解释哪些问题被显式抑制，再结合 `ignoredRules` 和 `ignoredFiles`。

如果 `root` 本身已经是 ncgo 服务根目录，或是 micro workspace 根目录，也可以省略 `files`，让 ncgo 自动发现 entry proto。

`ncgo_add_infra`

```json
{
  "jsonrpc": "2.0",
  "id": 5,
  "method": "tools/call",
  "params": {
    "name": "ncgo_add_infra",
    "arguments": {
      "root": ".",
      "kind": "logging",
      "dryRun": true,
      "output": "json"
    }
  }
}
```

推荐优先读取的顶层字段：先看 `updated`、`plan`、`nextSteps`；再用 `writtenPath` / `writtenPaths` / `wiredPaths` 总结已经修改或将要修改的内容。

`ncgo_ai_sync`

```json
{
  "jsonrpc": "2.0",
  "id": 6,
  "method": "tools/call",
  "params": {
    "name": "ncgo_ai_sync",
    "arguments": {
      "root": ".",
      "target": "all",
      "lang": "en",
      "dryRun": true,
      "output": "json"
    }
  }
}
```

推荐优先读取的顶层字段：`target`、`scope`、`sourceRef`、`workspace`、
`written`、`skipped`，然后再看 `notes`。

当 `output=text` 时，`content[0].text` 与 CLI 风格同步摘要一致（`info:` / `wrote` / `skipped`）。
当 `output=json` 时，`content[0].text` 返回完整 sync 结果的 JSON。是否阻断则看 `isError`。

当 `scope=service` 且 `workspace.role=member` 时，表示当前服务登记在上层 micro 工作区中。
当 `scope=workspace` 且 `workspace.role=root` 时，表示工具运行在 micro 工作区根目录，
此时 `workspace.serviceCount` 表示发现到的服务数量。

`ncgo_ai_init_claude`

```json
{
  "jsonrpc": "2.0",
  "id": 7,
  "method": "tools/call",
  "params": {
    "name": "ncgo_ai_init_claude",
    "arguments": {
      "root": ".",
      "preset": "team",
      "output": "json"
    }
  }
}
```

推荐优先读取的顶层字段：先看 `written`、`skipped`、`notes`、`nextSteps`。
当 `output=text` 时，`content[0].text` 与 CLI 风格 starter 摘要一致；
当 `output=json` 时，返回同一份结构化结果的 JSON。

`ncgo_add_method`

```json
{
  "jsonrpc": "2.0",
  "id": 8,
  "method": "tools/call",
  "params": {
    "name": "ncgo_add_method",
    "arguments": {
      "root": ".",
      "spec": "device.ListThemes",
      "in": "usecase",
      "output": "json"
    }
  }
}
```

当 `output=json` 时，工具会把 `path`、`domain`、`method`、`nextSteps` 作为同级
顶层字段返回（同一份载荷也会出现在 `content[0].text`）；是否失败则看 `isError`。

`ncgo_version`

```json
{
  "jsonrpc": "2.0",
  "id": 9,
  "method": "tools/call",
  "params": {
    "name": "ncgo_version"
  }
}
```

text-only 返回形态：不需要 `arguments`；主要读取 `content[0].text` 中的单条版本 / build / assets 摘要。

## 1. Mono Hertz 服务

```bash
ncgo new user-api --module github.com/acme/user-api --kind hertz --no-generate
cd user-api
```

预期会看到这些文件：

- `.ncgo/manifest.yaml`
- `.pre-commit-config.yaml`
- `idl/app/user-api.proto`
- `template/layout.yaml`
- `template/package.yaml`
- `template/data.json`

典型下一步：

```bash
go mod init github.com/acme/user-api
hz new --mod=github.com/acme/user-api --idl=idl/app/user-api.proto --handler_dir=internal/handler --model_dir=internal/pb --router_dir=internal/router --customize_layout=template/layout.yaml --customize_layout_data_path=template/data.json --customize_package=template/package.yaml
go mod tidy
make dev
```

如果需要生成 Swagger / OpenAPI 文档，先确保 `protoc` 已通过系统方式安装，并安装 Go 插件：

```bash
go install github.com/hertz-contrib/swagger-generate/protoc-gen-http-swagger@latest
make swagger
```

生成后的 Hertz 项目也提供 `make install-tools` 安装 Go 侧开发工具；其中会安装 `protoc-gen-http-swagger`，但不会自动安装 `protoc`。Swagger spec 会通过 `go:embed` 编译进二进制，执行 `make swagger` 后需要重新 `go run .` / `make dev` 或重新构建并重启服务。

生成的项目构建在 go-tools v0.3.0 之上：`go.mod` 声明 `go 1.26.5`，并 require `go-common v0.3.0` + `go-framework v0.3.0`（项目使用数据库时由 `go mod tidy` 补上 `go-middleware v0.1.0`）。响应层使用 `go-framework/hertz` 的 `Responder`，配置使用 `go-framework/config`，错误码 re-export `go-framework/error` 的框架码（`CodeSystem=10000` … `CodeRPCTimeout=10011`）。业务自定义错误码须 `>= 40100`，经 `goerror.HTTPStatus` 兜底返回 HTTP 200。完整约定见 README「生成项目构建在 go-tools v0.3.0 之上」一节。

配置中的 duration 字段（例如 `rpc.request_timeout_seconds`、
`database.health_check_period_seconds`、`rate_limit.rule.window_seconds`、
`redis.dial_timeout_seconds`）统一使用 `go-framework/config` 的
`config.Duration`，在 `conf/dev/conf.yaml` 中以 `"30s"` / `"200ms"` 形式的
duration 字符串填写；这些字段不再接受裸整数。

适合：以 HTTP 为主，希望在执行生成器前先检查布局输入的服务。

## 2. Mono Kitex 服务

```bash
ncgo new user-rpc --module github.com/acme/user-rpc --kind kitex --no-generate
cd user-rpc
```

预期会看到这些文件：

- `.ncgo/manifest.yaml`
- `.pre-commit-config.yaml`
- `idl/userrpc.proto`
- `template/kitex-template/...`

典型下一步：

```bash
go mod init github.com/acme/user-rpc
kitex -module github.com/acme/user-rpc -template-dir template/kitex-template -type protobuf idl/userrpc.proto
make sqlc
go mod tidy
make dev
```

之所以要在第一次 `go mod tidy` 前先执行 `make sqlc`，是因为生成出来的 Kitex
starter 已经接入了 `internal/base/data` / repository 占位代码，它们会 import
`internal/db/gen`。

生成的 Kitex 项目同样构建在 go-tools v0.3.0 之上：`go.mod` 声明 `go 1.26.5`，并 require `go-common v0.3.0` + `go-framework v0.3.0`。RPC 错误经 `internal/pkg/rpcerror`，通过 `go-framework/kitex/rpcerror` 把 `goerror` 错误映射为 Kitex `BizStatusError`；框架码来自 `go-framework/error`（`CodeInternalError=CodeSystem=10000`、`CodeConfigInvalid=10004`、`CodeRPCTimeout=10011`、`CodePermissionDenied=CodeAuthFailed=10002`）。业务码须 `>= 40100`。conf 中的 duration 类字段统一使用 `config.Duration`，在 `conf/dev/conf.yaml` 中以 duration 字符串（`"3s"`、`"30s"` 等）填写。

适合：以 RPC 为主，并希望把 Kitex 模板树纳入版本控制的服务。

## 3. Micro 工作区

```bash
ncgo new commerce --module github.com/acme/commerce --mode micro
cd commerce
```

预期会看到这些文件：

- `ncgo.workspace`
- `README.md`
- `.pre-commit-config.yaml`
- `services/.gitkeep`

典型下一步：

```bash
ncgo add rpc user-rpc --root . --plan
ncgo add bff web-bff --root . --plan
ncgo doctor --root .
ncgo protolint --root . --output json
```

`--plan` 可以先预览 service 路径、module 名称和后续生成器步骤，再决定是
否真正写入文件。

### 3.1 Micro 工作区模版消费

`ncgo new --mode micro` 与 `ncgo add rpc/bff` 通过 `--template <name>`（从 registry
缓存）或 `--template-dir <path>`（本地路径）消费模版包：

```bash
# 从 registry 拉取 micro 模版包
ncgo template pull my-micro

# 使用 micro 模版包创建工作区
ncgo new commerce --module github.com/acme/commerce --mode micro --template my-micro

# 使用同一个 micro 模版包添加服务
ncgo add rpc user-rpc --template my-micro
ncgo add bff web-bff --template my-micro

# 或使用独立的 mono 模版包
ncgo add rpc user-rpc --template base-kitex
ncgo add bff web-bff --template base-hertz

# 本地模版目录同样生效
ncgo new commerce --module github.com/acme/commerce --mode micro \
  --template-dir ./my-micro-pkg
```

**Micro 模版包结构**（`kind: micro`）：

```
my-micro/
├── template.yaml              # name, kind: micro, description, version
├── workspace/                 # 工作区骨架模板
│   ├── compose.yaml.tpl       # .tpl 后缀 → Go 模板变量替换
│   └── scripts/build.sh.tpl
├── kitex-template/            # Kitex 服务模板（add rpc 使用）
│   └── *.yaml
├── hertz-template/            # Hertz 服务模板（add bff 使用）
│   └── *.yaml
└── idl/
    ├── kitex/*.proto          # Kitex IDL 模板
    └── hertz/app/*.proto      # Hertz IDL 模板
```

- `--preset`、`--template`、`--template-dir` 三者互斥。
- `add rpc` 接受 `kind: kitex` 或 `kind: micro` 包（从 micro 包中提取 kitex 模板）。
- `add bff` 接受 `kind: hertz` 或 `kind: micro` 包（从 micro 包中提取 hertz 模板）。

当工作区里已经有服务 manifest 时，`ncgo doctor --root .` 与 `ncgo protolint --root .`
会自动聚合 `ncgo.workspace` 中列出的各个服务 `manifest.service.idl`，不必手动逐个
传 `--file`。

适合：希望在一个工作区根目录下逐步增加多个服务的团队。

## 4. 在已有 ncgo 项目上继续扩展

```bash
ncgo add domain device --root .
ncgo add method device.ListThemes --root . --in usecase
ncgo add infra logging --root . --wire --dry-run
ncgo ai sync --target all --root .
ncgo doctor --root .
ncgo doctor --root . --output json
ncgo doctor --root . --output sarif > doctor.sarif.json
```

如果希望在 CLI 里直接拿到 machine-readable 的 AI helper 输出，可使用：

```bash
ncgo ai init claude --root . --output json
ncgo ai sync --target all --root . --output json
```

`ai init claude --output json` 会返回 starter files 的结构化结果以及 `nextSteps`。
`ai sync --output json` 会返回与 MCP 对齐的结构化 sync payload，其中包含
`target`、`scope`、`sourceRef` 以及可选的 `workspace` 元数据。

现在 `ncgo doctor --root .` 会默认检查：

- 宿主机上的 `hz` / `kitex`
- `.ncgo/manifest.yaml`
- `template/data.json`
- 以及 `manifest.service.idl` 指向的 entry proto 的 Proto I/O 规则

如果 doctor 输出了 `Rule=PIO...` 的失败项，就表示该项目的 `.proto` 契约已经触发了内置的 proto lint 规则；此时可以继续用 `ncgo protolint --root . --file ... --output json` 深入看结构化 diagnostics。现在 `doctor` 自身也支持 `--output json` 与 `--output sarif`；其中 `--json` 仍然保留为兼容别名。

如果你要把 `doctor` 结果接到 code scanning、IDE 问题面板或 CI 工件中，可以直接输出：

`ncgo doctor --root . --output sarif > doctor.sarif.json`

如果你通过 MCP 调用这里的检查流程，直接调用 `ncgo_doctor` 即可；通用返回约定与 `root` / `scope` / `summary` / `checks` / `ok` 字段可直接参考上面的 §0。

适合：已经有 ncgo 项目，只想按需逐步增强，而不是重新生成整个服务。

### 用 ncgo 实现一个功能

一个聚焦的端到端流程——新增领域与用例方法，然后刷新 AI 上下文——如下：

```bash
ncgo add domain device --root .
ncgo add method device.ListThemes --root . --in usecase --output json
make sqlc
go build ./...
ncgo ai sync --target all --root .
```

`add method --output json` 会返回 `path`、`domain`、`method` 和 `nextSteps`，
Agent 可以直接据此驱动后续步骤：

```json
{
  "path": "internal/usecase/device/device.go",
  "domain": "device",
  "method": "ListThemes",
  "nextSteps": [
    "go build ./...",
    "replace the generated stub body with domain logic",
    "ncgo ai sync --root ."
  ]
}
```

用功能的业务逻辑替换生成的桩代码。当服务使用数据库时执行 `make sqlc`
（Kitex 在 `go mod tidy` 之前始终需要；Hertz 仅在启用数据库脚手架时需要）。
最后执行 `ncgo ai sync --target all --root .`，让所有 AI 上下文文件——
`AGENTS.md`、`CLAUDE.md`、ncgo-dev skill、project context 与 Cursor 规则——
都反映新增的领域与方法。

### 校验 Agent 的改动

在用 `ncgo add domain` / `ncgo add method` 实现功能之后，运行：

```bash
ncgo check --root .
```

如果某个 usecase 丢失了 `// ncgo:methods` anchor、manifest 与
`internal/usecase/*/` 目录不一致，或 AI 上下文文件已过期，命令会以退出码 1
结束并输出结构化报告（`--output json`）。刷新上下文：

```bash
ncgo ai sync --root .
```

### 独立参考文档

`ncgo ai sync` 同时生成独立文档到 `docs/ncgo/` 目录：

```bash
# 英文（默认）
ncgo ai sync --root ./user-api

# 中文
ncgo ai sync --root ./user-api --lang zh-CN
```

产出文件：
- `docs/ncgo/hertz/design-doc.en.md` — Hertz 架构设计文档
- `docs/ncgo/hertz/rate-limit-dynamic-design.en.md` — 动态限流设计文档
- `docs/ncgo/kitex/design-doc.en.md` — Kitex 对应文档（用于交叉引用）

跨 profile 的链接会自动改写为本地相对路径。

## 5. 生成项目中的 i18n 补译工作流

假设你已经生成了一个 Hertz 服务，并希望把新文案补到 `it-IT`。

先同步 locale / status：

```bash
make i18n-sync
make i18n-report
```

然后用 CLI 查看结构化结果：

```bash
ncgo i18n report --root . --output json
ncgo i18n check --root . --mode dev --output json
```

典型流程是：

1. 修改 `internal/pkg/i18n/locales/zh-CN.json`
2. 执行 `make i18n-sync`
3. 查看 `internal/pkg/i18n/.meta/report.md`
4. 用 `ncgo i18n report --output json` 读取 `missing_translations` / `stale_translations`
5. 补写 `internal/pkg/i18n/locales/it-IT.json` 与 `internal/pkg/i18n/.meta/status.json`
6. 执行 `ncgo i18n check --root . --mode release --output json`
7. 当 `failures` 为空后执行 `make i18n`

如果你希望由 Agent / MCP 来消费同一份结构化结果，可在另一个终端启动：

```bash
ncgo mcp serve
```

然后让 Agent 调用：

- `ncgo_i18n_report`
- `ncgo_i18n_check`

这两个 MCP 工具遵循上面 §0 的 contract：你可以用 `output=text|json` 控制 `content[0].text` 返回摘要文本或 pretty JSON，同时继续直接消费顶层的 `report` / `summary` / `failures` / `warnings` / `nextSteps`。

建议 Agent 只把 `report.missing_translations`、`stale_translations`、`draft_translations` 中属于当前 target locale 的条目作为输入范围，不直接修改业务代码或 `catalog_gen.go`。

适合：已经启用了 Hertz 模板内置 i18n，希望把人工补译、Agent 辅助补译与最终 `make i18n` 串成一条稳定链路的项目。

## 6. 生成项目中的 Proto 契约校验工作流

假设你已经生成了一个服务，并希望把 `.proto` 契约检查纳入本地开发或 Agent 流程。

如果当前目录是 mono 服务根，或是已经登记了多个服务的 micro workspace 根，也可以省略
`--file`，让 ncgo 从 manifest / workspace 自动发现要检查的 entry proto。

先用 CLI 读取结构化结果：

```bash
ncgo protolint --root . --file idl/app/demo.proto --output json
ncgo protolint --root . --output json
ncgo protolint --root . --file idl/app/demo.proto --output sarif > protolint.sarif.json
ncgo protolint --root . --file idl/app/demo.proto --rule PIO201 --rule PIO202
ncgo protolint --root . --file idl/app/demo.proto --ignore-rule PIO212 --ignore-file idl/app/legacy.proto
```

典型流程是：

1. 修改 `idl/*.proto`
2. 执行 `ncgo protolint --root . --file ... --output json`
3. 查看 `diagnostics` 中的 `ruleId`、`summary`、`message`、`field`
4. 若只想聚焦某一类问题，用 `--rule PIO201 --rule PIO202` 缩小范围
5. 若某些历史 warning 已知暂不治理，可用 `--ignore-rule PIO212` 或 `--ignore-file idl/app/legacy.proto` 做显式抑制
6. 修正 proto 后重新执行，直到 `ok=true`

如果你要把结果接入 GitHub code scanning、IDE 插件或其他安全/静态分析平台，也可以直接输出 SARIF：

`ncgo protolint --root . --file idl/app/demo.proto --output sarif > protolint.sarif.json`

当前如果只命中了 `phase2` 的 warning 规则（例如 `PIO111`、`PIO112`、`PIO113`、`PIO211`、`PIO212`、`PIO302`、`PIO303`、`PIO401`、`PIO402`、`PIO403`、`PIO404`），输出里仍会保留对应 `diagnostics`，但 `ok` 仍然会保持为 `true`。也就是说，这批规则更适合做设计提示与渐进治理，而不是直接阻断本地开发流。

这批 warning 的当前最小实现边界大致是：

- `PIO111` / `PIO112` / `PIO113`：顶层 `Empty`、泛化 message 名称、request 字段过多
- `PIO211` / `PIO212`：Hertz request 未绑定字段、缺 `openapi.operation/schema/property`
- `PIO302` / `PIO303`：Kitex 列表接口缺分页结构、request 同时混入过多筛选/排序/分页/调试/扩展职责
- `PIO401` / `PIO402` / `PIO403` / `PIO404`：分页字段缺范围约束、自由文本 string 缺长度约束、repeated/map 缺数量约束、enum 缺 `defined_only`

如果你希望由 Agent / MCP 来消费同一份结构化结果，可在另一个终端启动：

```bash
ncgo mcp serve
```

此时 MCP 的 `ncgo_protolint` 也支持传入 `ignoreRules` / `ignoreFiles`，与 CLI 的 `--ignore-rule` / `--ignore-file` 语义保持一致。

然后让 Agent 调用：

- `ncgo_protolint`

推荐输入在上面的 §0 里也有汇总；在这个 workflow 里最常用的是：

- `root`：项目 proto import 根目录
- `files`：需要检查的 entry proto 文件数组
- `rules`：可选，聚焦某几个规则 ID

建议 Agent 只根据 `diagnostics` 修改 `.proto` 契约本身，不顺带改业务代码、生成物或其他派生文件。

适合：希望把 Req/Resp 命名、Hertz binding、Kitex response 结构等 Proto 契约规则纳入稳定自动检查链路的项目。

### 输出示例（warning-only 场景）

**`--output json`**

```json
{
  "root": "/path/to/project",
  "files": ["invalid.proto"],
  "rulesRun": ["PIO111", "PIO112", "PIO113"],
  "ok": true,
  "summary": {
    "filesScanned": 1,
    "rpcsScanned": 4,
    "diagnosticsCount": 5,
    "errorCount": 0,
    "warningCount": 5
  },
  "diagnostics": [
    {
      "ruleId": "PIO111",
      "level": "warning",
      "phase": "phase2",
      "file": "invalid.proto",
      "line": 40,
      "column": 3,
      "service": "Demo",
      "rpc": "Health",
      "message": "google.protobuf.Empty",
      "summary": "rpc Health uses google.protobuf.Empty as input",
      "hint": "prefer an explicit empty <Method>Req message when the RPC is part of your public business contract"
    },
    {
      "ruleId": "PIO111",
      "level": "warning",
      "phase": "phase2",
      "file": "invalid.proto",
      "line": 41,
      "column": 3,
      "service": "Demo",
      "rpc": "Ping",
      "message": "google.protobuf.Empty",
      "summary": "rpc Ping uses google.protobuf.Empty as output",
      "hint": "prefer an explicit empty <Method>Resp message when the RPC is part of your public business contract"
    },
    {
      "ruleId": "PIO112",
      "level": "warning",
      "phase": "phase2",
      "file": "invalid.proto",
      "line": 42,
      "column": 3,
      "service": "Demo",
      "rpc": "GetUser",
      "message": "CommonReq",
      "summary": "rpc GetUser input CommonReq looks too generic for a top-level request",
      "hint": "use a method-specific request message instead of a reusable generic top-level input"
    },
    {
      "ruleId": "PIO112",
      "level": "warning",
      "phase": "phase2",
      "file": "invalid.proto",
      "line": 43,
      "column": 3,
      "service": "Demo",
      "rpc": "Search",
      "message": "Result",
      "summary": "rpc Search output Result looks too generic for a top-level response",
      "hint": "use a method-specific response message instead of a reusable generic top-level output"
    },
    {
      "ruleId": "PIO113",
      "level": "warning",
      "phase": "phase2",
      "file": "invalid.proto",
      "line": 43,
      "column": 3,
      "service": "Demo",
      "rpc": "Search",
      "message": "SearchReq",
      "summary": "request SearchReq declares 13 fields, which exceeds the warning threshold 12",
      "hint": "consider splitting the request or grouping related inputs so the RPC contract stays focused"
    }
  ]
}
```

**`--output text`（默认）**

```text
! [PIO111] invalid.proto:40:3 Demo/Health rpc Health uses google.protobuf.Empty as input
    hint: prefer an explicit empty <Method>Req message when the RPC is part of your public business contract
! [PIO111] invalid.proto:41:3 Demo/Ping rpc Ping uses google.protobuf.Empty as output
    hint: prefer an explicit empty <Method>Resp message when the RPC is part of your public business contract
! [PIO112] invalid.proto:42:3 Demo/GetUser rpc GetUser input CommonReq looks too generic for a top-level request
    hint: use a method-specific request message instead of a reusable generic top-level input
! [PIO112] invalid.proto:43:3 Demo/Search rpc Search output Result looks too generic for a top-level response
    hint: use a method-specific response message instead of a reusable generic top-level output
! [PIO113] invalid.proto:43:3 Demo/Search request SearchReq declares 13 fields, which exceeds the warning threshold 12
    hint: consider splitting the request or grouping related inputs so the RPC contract stays focused
protolint: ok (files=1 rpcs=4 diagnostics=5 errors=0 warnings=5 rules=3)
```

`!` 前缀表示 warning，`✗` 前缀表示 error。最后一行汇总显示 `ok`，是因为本次运行 5 条均为 warning——只有出现至少一条 error 级别 diagnostic 时，`ok` 才会变为 `false`。

**MCP `ncgo_protolint` 工具返回示例（warning-only）**

```json
{
  "content": [
    {
      "type": "text",
      "text": "! [PIO111] invalid.proto:40:3 Demo/Health rpc Health uses google.protobuf.Empty as input\n    hint: prefer an explicit empty <Method>Req message when the RPC is part of your public business contract\n! [PIO111] invalid.proto:41:3 Demo/Ping rpc Ping uses google.protobuf.Empty as output\n    hint: prefer an explicit empty <Method>Resp message when the RPC is part of your public business contract\n! [PIO112] invalid.proto:42:3 Demo/GetUser rpc GetUser input CommonReq looks too generic for a top-level request\n    hint: use a method-specific request message instead of a reusable generic top-level input\n! [PIO112] invalid.proto:43:3 Demo/Search rpc Search output Result looks too generic for a top-level response\n    hint: use a method-specific response message instead of a reusable generic top-level output\n! [PIO113] invalid.proto:43:3 Demo/Search request SearchReq declares 13 fields, which exceeds the warning threshold 12\n    hint: consider splitting the request or grouping related inputs so the RPC contract stays focused\nprotolint: ok (files=1 rpcs=4 diagnostics=5 errors=0 warnings=5 rules=3)\n"
    }
  ],
  "isError": false,
  "root": "/path/to/project",
  "files": ["invalid.proto"],
  "rulesRun": ["PIO111", "PIO112", "PIO113"],
  "ok": true,
  "summary": {
    "filesScanned": 1,
    "rpcsScanned": 4,
    "diagnosticsCount": 5,
    "errorCount": 0,
    "warningCount": 5
  },
  "diagnostics": [
    { "ruleId": "PIO111", "level": "warning", "phase": "phase2", "file": "invalid.proto", "line": 40, "column": 3, "service": "Demo", "rpc": "Health", "message": "google.protobuf.Empty", "summary": "rpc Health uses google.protobuf.Empty as input", "hint": "prefer an explicit empty <Method>Req message when the RPC is part of your public business contract" },
    { "ruleId": "PIO111", "level": "warning", "phase": "phase2", "file": "invalid.proto", "line": 41, "column": 3, "service": "Demo", "rpc": "Ping",   "message": "google.protobuf.Empty", "summary": "rpc Ping uses google.protobuf.Empty as output",  "hint": "prefer an explicit empty <Method>Resp message when the RPC is part of your public business contract" },
    { "ruleId": "PIO112", "level": "warning", "phase": "phase2", "file": "invalid.proto", "line": 42, "column": 3, "service": "Demo", "rpc": "GetUser","message": "CommonReq",            "summary": "rpc GetUser input CommonReq looks too generic for a top-level request", "hint": "use a method-specific request message instead of a reusable generic top-level input" },
    { "ruleId": "PIO112", "level": "warning", "phase": "phase2", "file": "invalid.proto", "line": 43, "column": 3, "service": "Demo", "rpc": "Search", "message": "Result",               "summary": "rpc Search output Result looks too generic for a top-level response",   "hint": "use a method-specific response message instead of a reusable generic top-level output" },
    { "ruleId": "PIO113", "level": "warning", "phase": "phase2", "file": "invalid.proto", "line": 43, "column": 3, "service": "Demo", "rpc": "Search", "message": "SearchReq",            "summary": "request SearchReq declares 13 fields, which exceeds the warning threshold 12", "hint": "consider splitting the request or grouping related inputs so the RPC contract stays focused" }
  ]
}
```

这个示例遵循上面 §0 的通用 MCP contract：`content[0].text` 承载由 `output` 选择的文本格式，`ok` / `summary` / `diagnostics` 作为同级字段保留给 Agent 直接消费。这里 `isError=false`，因为 warning-only 不会阻断运行。

## 6. 从成熟项目导出代码模板

当你的 Hertz 或 Kitex 项目稳定后——中间件、配置结构、分层约定——可以将这些文件导出为可复用的 `.yaml` 模板：

```bash
# 从 Hertz 项目导出
ncgo export templates

# 从 Kitex 项目导出（指定 kind）
ncgo export templates --kind kitex
```

这会扫描 `internal/` 下的 ncgo 托管 `.go` 文件，将模块路径替换为 `{{.Module}}`，服务名替换为 `{{.ServiceName}}`，并写入 `template/<kind>-template/` 目录。

导出的模板：

- **Kitex**：可直接用于 `kitex -template-dir template/kitex-template`
- **Hertz**：由 `ncgo new` 在 `hz new` 之后自动作为 overlay 应用

排除路径：`internal/pb/`（hz 生成的 protobuf 代码）和 `kitex_gen/`（kitex 生成的 RPC stub）。

导出还会把项目的服务 IDL 写入 `template/idl/`，文件名中的服务名会被参数化
（例如 `template/idl/app/{{ToLower .ServiceName}}.proto`），这样使用者可以把它
渲染到自己的默认 IDL 路径上。

### DDD 领域层/应用层

`ncgo export templates` 还会捕获 `internal/domain/**` 与 `internal/application/**`
下的 DDD 业务分层。与 `internal/repository/**` 一样，这些文件以 `skip` 更新行为
导出，但**不会**按 proto service 循环：每个聚合以具体的按聚合文件导出，因此聚合
目录段（`internal/domain/<agg>/`、`internal/application/<agg>/`）在模板路径中
原样保留。

```
internal/domain/<agg>/        entity.go valueobject.go <agg>.go service.go repository.go   （领域模型 + repo 端口）
internal/application/<agg>/    <agg>_service.go dto.go                                       （应用服务）
internal/repository/<agg>/     repo 实现（sqlc-backed）
```

模板包内容就是基础项目的起始代码。`ncgo new --template-dir`（或 `--template`）
把它当作“换名复制”来消费：生成时只替换导出文件里的 module 与服务名，并用你传入
的 `--module` / 服务名生成一个全新的脚手架，而不是逐字复制源项目。

## 7. 规则中心限流集成

当多个 Hertz 服务需要共享限流规则时，可以创建一个独立的 Kitex gRPC
规则中心服务，然后将每个 Hertz 服务接入查询。

### 第一步：创建 rule-center Kitex 服务

```bash
ncgo new rule-center \
  --module github.com/acme/rule-center \
  --kind kitex --db postgres --preset rule-center
cd rule-center
make sqlc
go mod tidy
make dev
```

rule-center 服务包含：

- `idl/rule-center.proto` — `GetRule` RPC，用于查询限流规则
- `internal/handler/rulecenter/` — gRPC Handler
- `internal/usecase/rulecenter/` — 业务逻辑
- `internal/repository/rulecenter/` — PostgreSQL 数据访问
- `schema/` + `query/` — sqlc schema 和查询

### 第二步：创建启用规则中心的 Hertz 服务

```bash
ncgo new user-api \
  --module github.com/acme/user-api \
  --kind hertz --db postgres \
  --rule-center-addr rule-center:8888
cd user-api
```

当提供了 `--rule-center-addr` 时，ncgo 会：

- 在 `conf/dev/conf.yaml` 中将 `rate_limit.source.type` 设为 `rule_center`
- 生成 `internal/pkg/middleware/rule_center_client.go`
- 添加 `rule_center` 配置块，填入指定地址

### 第三步：在已有 Hertz 服务上接入规则中心

如果已经有 Hertz 服务，后续想接入规则中心：

```bash
ncgo add rule-center --root ./user-api --addr rule-center:8888
```

这会修改现有的 `conf/dev/conf.yaml` 并生成客户端文件。
使用 `--force` 可覆盖已存在的客户端文件，使用 `--dry-run` 可预览不写入。

### 配置参考

生成项目的 `conf/dev/conf.yaml` 中，duration 类字段统一使用 `config.Duration`
（来自 `go-framework/config`），以 `"60s"` / `"200ms"` 形式的 duration 字符串
填写（由 `time.ParseDuration` 解析）；这些字段不再接受裸整数，但字段名 /
YAML key 保持不变。

```yaml
rate_limit:
  enabled: true
  source:
    type: rule_center              # 切换到远程规则中心
    cache_ttl_seconds: "60s"       # 本地缓存 TTL（config.Duration）
    fallback_on_error: true        # gRPC 失败时使用缓存规则
  rule_center:                     # 规则中心连接配置
    address: "rule-center:8888"    # gRPC 地址
    query_timeout_milliseconds: "200ms"
  backend: redis                   # 限流计数器仍使用 Redis
  fail_open: false
```

### 查询流程

1. 检查本地内存缓存（`cache_ttl_seconds` 内有效）
2. 缓存命中 → 返回缓存规则
3. 缓存未命中 → gRPC `GetRule` 查询规则中心，结果写入缓存
4. gRPC 失败 + `fallback_on_error: true` → 使用缓存中的旧规则
5. gRPC 失败 + 无缓存 → 根据 `fail_open` 决定放行或拒绝

### 通过 MCP 调用

启动 `ncgo mcp serve` 后，Agent 可以调用：

- `ncgo_new` 配合 `preset: "rule-center"` 和 `kind: "kitex"` 创建 rule-center 服务
- `ncgo_new` 配合 `ruleCenterAddr: "rule-center:8888"` 创建接入规则中心的 Hertz 服务
- `ncgo_add_rule_center` 配合 `addr: "rule-center:8888"` 为已有 Hertz 服务接入规则中心

适合：多服务环境，需要集中管理限流规则、无需重启各服务即可更新规则的场景。

## 8. Polaris canary adapter（opt-in，Kitex only）

在已启用 `release_canary`（在 `internal/base/release` 中提供 SDK 中立的 canary seams）之后，可通过以下命令 opt-in 真实 Polaris 后端：

```bash
ncgo add infra polaris_adapter --root .
```

这会写入 `internal/base/release/polaris_adapter.go`（package `release`），并
打印下一步的 `go get` 命令。ncgo 本身保持无 SDK 依赖 —— Polaris SDK 依赖落在
你的项目中：

```bash
go get github.com/polarismesh/polaris-go
go get gopkg.in/yaml.v3
go get github.com/byx-darwin/go-tools/go-common
```

通过环境变量提供 Polaris 凭证（`POLARIS_TOKEN`、`POLARIS_NAMESPACE`），禁止
硬编码。构造 `release.NewPolarisSelector(discoveryCfg, ruleCfg)`，把它的
`RuleProvider` 喂给 `KitexCanaryLoadBalancer.RuleProvider`。基于 `polaris-go
v1.7.1` 测试通过。

**Troubleshooting**

- `addresses is empty` / 缺 token → 构造期直接失败。先修 config / env 再重试。
- 运行时发现/规则加载失败 → canary 路由 **fail OPEN** 到 Kitex 默认加权 LB
  （可用性优先）。先观察指标，再调整预期。
- **Kitex resolver 实例可见性假设** —— 如果 canary pool 为空，确认 Kitex
  resolver（如 `registry_polaris`）返回的是全量 stable+canary 实例集合。
  如果 resolver 按路由做了过滤，adapter 需要直接坐到 LB 层（后续工作）。
- `release.track` metadata 未生效 → 检查注册端是否在实例上设置了
  `release.track` metadata。

GA 加固（metrics / cache+TTL / dry-run / runtime harness）属于后续工作。

## 9. 从官方模板生成基础项目

当某个模板经过评审并合入官方 registry 之后：

```bash
ncgo template list                 # 浏览官方模板
ncgo template pull base-kitex      # 拉取到本地缓存
ncgo new my-svc --module github.com/acme/my-svc --kind kitex --template base-kitex
```

也可以直接指向任意本地模板包（即 `ncgo export templates` 生成的目录结构）：

```bash
ncgo new my-svc --module github.com/acme/my-svc --kind kitex \
  --template-dir path/to/base-kitex
```

模板包是一个目录，包含 `<kind>-template/*.yaml`，可选 `idl/*.proto`，以及可选的
`template.yaml` 元数据文件（`name`、`kind`、`description`、`version`）。预设类模板包
还可以携带 `schema/*.sql` 文件（复制到 `internal/db/schema/`，使用
`{{.Module}}`/`{{.ServiceName}}` 渲染）、根目录 `layout.yaml`（替换默认 layout），
并在 `template.yaml` 中声明 `skip_default_templates`（要跳过的默认分层模板）。要贡献
一个模板：先从成熟项目导出，补上 `template.yaml` + `README.md`，然后向 registry
仓库提 PR，等待官方评审。

### 预设等价模板包

一个镜像了内置预设的模板包即「预设等价」（preset-equivalent）包。例如，官方
`rule-center` 包声明了 `skip_default_templates: [handler.yaml, server.yaml,
usecase.yaml, repository.yaml]`，携带 `schema/000002_rate_limit_rules.sql`、
根目录 `layout.yaml` 和 `idl/rulecenter.proto`，因此
`ncgo new my-svc --kind kitex --template rule-center` 会生成与
`--preset rule-center` 相同的目录树（合并语义：除被跳过的部分外保留内置默认模板，
并叠加模板包自身的模板）。仅剩一处差异：预设把 IDL 写为 `idl/rule-center.proto`，
而模板包路径写入 `idl/rulecenter.proto`（proto 内容一致、文件名不同；Makefile 的
`IDL_FILE` 也随之反映）。

registry URL 默认指向官方仓库；可通过 `--registry <url>` 或 `NCGO_REGISTRY` 覆盖。
