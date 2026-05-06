# ncgo 示例工作流

这些示例面向低风险起步场景。Mono 示例都使用了 `--no-generate`，这样你可
以先检查生成出来的输入文件，再决定何时执行 `hz` 或 `kitex`。

## 1. Mono Hertz 服务

```bash
ncgo new user-api --module github.com/acme/user-api --kind hertz --no-generate
cd user-api
```

预期会看到这些文件：

- `.ncgo/manifest.yaml`
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

适合：以 HTTP 为主，希望在执行生成器前先检查布局输入的服务。

## 2. Mono Kitex 服务

```bash
ncgo new user-rpc --module github.com/acme/user-rpc --kind kitex --no-generate
cd user-rpc
```

预期会看到这些文件：

- `.ncgo/manifest.yaml`
- `idl/userrpc.proto`
- `template/kitex-template/...`

典型下一步：

```bash
go mod init github.com/acme/user-rpc
kitex -module github.com/acme/user-rpc -template-dir template/kitex-template -type protobuf idl/userrpc.proto
go mod tidy
make dev
```

适合：以 RPC 为主，并希望把 Kitex 模板树纳入版本控制的服务。

## 3. Micro 工作区

```bash
ncgo new commerce --module github.com/acme/commerce --mode micro
cd commerce
```

预期会看到这些文件：

- `ncgo.workspace`
- `README.md`
- `services/.gitkeep`

典型下一步：

```bash
ncgo add rpc user-rpc --root . --plan
ncgo add bff web-bff --root . --plan
```

`--plan` 可以先预览 service 路径、module 名称和后续生成器步骤，再决定是
否真正写入文件。

适合：希望在一个工作区根目录下逐步增加多个服务的团队。

## 4. 在已有 ncgo 项目上继续扩展

```bash
ncgo add domain device --root .
ncgo add method device.ListThemes --root . --in usecase
ncgo add infra logging --root . --wire --dry-run
ncgo ai sync --root .
ncgo doctor --root .
```

现在 `ncgo doctor --root .` 会默认检查：

- 宿主机上的 `hz` / `kitex`
- `.ncgo/manifest.yaml`
- `template/data.json`
- 以及 `manifest.service.idl` 指向的 entry proto 的 Proto I/O 规则

如果 doctor 输出了 `Rule=PIO...` 的失败项，就表示该项目的 `.proto` 契约已经触发了内置的 proto lint 规则；此时可以继续用 `ncgo protolint --root . --file ... --output json` 深入看结构化 diagnostics。

适合：已经有 ncgo 项目，只想按需逐步增强，而不是重新生成整个服务。

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

建议 Agent 只把 `report.missing_translations`、`stale_translations`、`draft_translations` 中属于当前 target locale 的条目作为输入范围，不直接修改业务代码或 `catalog_gen.go`。

适合：已经启用了 Hertz 模板内置 i18n，希望把人工补译、Agent 辅助补译与最终 `make i18n` 串成一条稳定链路的项目。

## 6. 生成项目中的 Proto 契约校验工作流

假设你已经生成了一个服务，并希望把 `.proto` 契约检查纳入本地开发或 Agent 流程。

先用 CLI 读取结构化结果：

```bash
ncgo protolint --root . --file idl/app/demo.proto --output json
ncgo protolint --root . --file idl/app/demo.proto --rule PIO201 --rule PIO202
```

典型流程是：

1. 修改 `idl/*.proto`
2. 执行 `ncgo protolint --root . --file ... --output json`
3. 查看 `diagnostics` 中的 `ruleId`、`summary`、`message`、`field`
4. 若只想聚焦某一类问题，用 `--rule PIO201 --rule PIO202` 缩小范围
5. 修正 proto 后重新执行，直到 `ok=true`

当前如果只命中了 `phase2` 的 warning 规则（例如 `PIO111`、`PIO112`、`PIO113`、`PIO211`、`PIO212`、`PIO302`、`PIO303`、`PIO401`），输出里仍会保留对应 `diagnostics`，但 `ok` 仍然会保持为 `true`。也就是说，这批规则更适合做设计提示与渐进治理，而不是直接阻断本地开发流。

这批 warning 的当前最小实现边界大致是：

- `PIO111` / `PIO112` / `PIO113`：顶层 `Empty`、泛化 message 名称、request 字段过多
- `PIO211` / `PIO212`：Hertz request 未绑定字段、缺 `openapi.operation/schema/property`
- `PIO302` / `PIO303`：Kitex 列表接口缺分页结构、request 同时混入过多筛选/排序/分页/调试/扩展职责
- `PIO401`：`page` / `page_size` / `limit` / `offset` 等字段缺少明显 PGV 范围约束

如果你希望由 Agent / MCP 来消费同一份结构化结果，可在另一个终端启动：

```bash
ncgo mcp serve
```

然后让 Agent 调用：

- `ncgo_protolint`

推荐输入：

- `root`：项目 proto import 根目录
- `files`：需要检查的 entry proto 文件数组
- `rules`：可选，聚焦某几个规则 ID

建议 Agent 只根据 `diagnostics` 修改 `.proto` 契约本身，不顺带改业务代码、生成物或其他派生文件。

适合：希望把 Req/Resp 命名、Hertz binding、Kitex response 结构等 Proto 契约规则纳入稳定自动检查链路的项目。