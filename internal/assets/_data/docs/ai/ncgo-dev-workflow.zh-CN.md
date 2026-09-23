## 使用 ncgo 开发

先按意图选择 recipe。外部 endpoint 与内部 domain capability 修改的是不同的
usecase 形态，不能把对应命令当作同一种操作。

### 命令边界：endpoint 与 domain

- `ncgo add rpc-method` 从已经生成的 Hertz 或 Kitex handler 复制签名，写入顶层
  `internal/usecase/<service>/usecase.go`。它用于 IDL 生成器运行后的外部 endpoint。
- `ncgo add domain` 配合 `ncgo add method` 创建内部 domain 包，并在
  `internal/usecase/<domain>/` 下生成无参数的
  `func (u *UseCase) <Method>() error` capability。
- 不要把两个命令当成创建同一个 endpoint 方法的两种方式而连续执行。只有当
  endpoint 明确委托给职责不同、通常名称也不同的 domain capability 时，才会同时
  使用两条工作流。

任一 recipe 实际应用后，都要刷新并验证全部已启用 Agent 消费者：

```bash
gofmt -w <changed-go-files>
go build ./...
go vet ./...
go test ./... -count=1
ncgo ai sync --target all --root .
ncgo check --root .
```

### Recipe：Hertz HTTP endpoint

**前置条件**

- 位于具有有效 `.ncgo/manifest.yaml` 的 Hertz 服务根目录。
- 读取 `manifest.service.idl`，不要猜测 IDL 路径。
- `hz` 与 `ncgo` 已安装并位于 `PATH`。

**执行顺序**

1. 编辑 `<manifest.service.idl>`，新增或修改 HTTP/RPC 方法。
2. 运行 `ncgo protolint --root . --file <manifest.service.idl>`。
3. 运行 `make update`（Hertz 项目会调用 `hz update`）。
4. 运行 `ncgo add rpc-method --service <manifest.service.name> --rpc <Method> --root .`。
5. 实现新生成的顶层 usecase 桩，再让生成的 handler 调用它；handler 不导入
   repository 或 data 包。
6. 执行上面的公共 build、test、context sync 与 check 命令。

**预期文件**

- 已编辑的 IDL，以及刷新的 handler/router/model 生成文件；
- 含 endpoint 签名的 `internal/usecase/<service>/usecase.go`；
- sync 后的五个已启用托管 Agent 上下文文件。

**验证**

- `ncgo protolint`、`go build`、`go vet` 与 `go test` 全部通过；
- all-target sync 后 `ncgo check --root .` 退出 `0`。

**失败恢复**

- proto lint 失败时，先修复报告的规则再生成。
- `make update` 失败时，检查 manifest IDL 路径、proto import 与 `hz`。
- `add rpc-method` 找不到方法时，重跑 `make update`，确认 handler 包含完全一致的方法名。
- usecase 已有该方法时直接实现，不要强制生成重复方法。

### Recipe：Kitex RPC endpoint

**前置条件**

- 位于具有有效 manifest 和已生成 handler 的 Kitex 服务根目录。
- 读取 `manifest.service.idl`；`kitex`、`protoc` 与 `ncgo` 位于 `PATH`。

**执行顺序**

1. 编辑 `<manifest.service.idl>`，新增或修改 RPC 方法。
2. 运行 `ncgo protolint --root . --file <manifest.service.idl>`。
3. 运行 `make update`，让 Kitex 刷新 `kitex_gen/` 与 handler 签名。
4. 运行 `ncgo add rpc-method --service <manifest.service.name> --rpc <Method> --root .`。
5. 实现新的顶层 usecase 方法，并保持 handler 只做薄适配；使用生成 DB 代码时，
   在 `go mod tidy` 之前运行 `make sqlc`。
6. 执行公共 build、test、context sync 与 check 命令。

**预期文件**

- 已编辑的 IDL、刷新的 `kitex_gen/` 与生成 handler；
- 含复制 RPC 签名的 `internal/usecase/<service>/usecase.go`；
- 刷新的 Agent 上下文。

**验证**

- proto lint 与生成成功，`go build ./...` 可编译该签名；
- 测试通过，sync 后 `ncgo check --root .` 退出 `0`。

**失败恢复**

- 生成失败时检查 proto include root 与已安装 Kitex 版本。
- `add rpc-method` 报 handler 缺少方法时，说明生成结果过期；先重跑 `make update`。
- 缺少 SQL 包时，在模块解析前运行 `make sqlc`。

### Recipe：内部 domain capability

**前置条件**

- 变更属于内部业务行为，而不是新的外部 endpoint。
- domain 名匹配 `^[a-z][a-z0-9_]{0,62}$`，方法名匹配
  `^[A-Z][A-Za-z0-9_]{0,62}$`。

**执行顺序**

1. 运行 `ncgo add domain <domain> --root . --dry-run` 并审阅计划。
2. domain 不存在时运行 `ncgo add domain <domain> --root .`。
3. 运行 `ncgo add method <domain>.<Method> --root .`。
4. 用 domain 逻辑替换无参数桩，并添加聚焦测试。
5. capability 修改数据库查询时，先运行 `make sqlc`。
6. 执行公共 build、test、context sync 与 check 命令。

**预期文件**

- `.ncgo/manifest.yaml` 列出该 domain；
- `internal/usecase/<domain>/<domain>.go` 含成对 method anchors 与新方法；
- 新建 domain 时存在 `internal/repository/<domain>/` 与
  `internal/base/data/<domain>_register.go`。

**验证**

- domain 测试与 `go test ./... -count=1` 通过；
- `check.anchor`、`check.manifest.consistency` 与上下文检查通过。

**失败恢复**

- “already exists”表示跳过 domain 创建，继续执行 `add method`。
- “missing markers”表示 usecase 所有权标记被删除；应审慎恢复标记，或在审阅后
  使用 `add domain --force` 重新生成。
- 不要使用 `add rpc-method` 修复内部 domain 方法。

### Recipe：BFF 调用 RPC client

**前置条件**

- 位于将拥有 client 的 Hertz BFF module。
- 找到 RPC 服务 proto 与准确 service 名；`kitex` 位于 `PATH`，proto 的
  `go_package` 与当前 module 布局兼容。

**执行顺序**

1. 用 `ncgo add kitex-client <client> --service <rpc-service> --idl <proto> --dry-run` 预览。
2. 用 `ncgo add kitex-client <client> --service <rpc-service> --idl <proto>` 应用。
3. 将 `pkg/client/<client>` 接入 BFF usecase/DI 层，并通过配置提供 RPC 地址；
   handler 不得直接调用 client。
4. 添加 client 与 BFF 行为测试，再执行公共验证命令。

**预期文件**

- `pkg/client/<client>/client.go` 与 `pkg/client/<client>/config.go`；
- 生成的 `kitex_gen/` 类型与模块依赖更新；
- 接线时由项目持有的 DI/config 修改。

**验证**

- `go mod tidy`、`go build ./...` 与 client 测试通过；
- BFF 可使用配置的 RPC 地址启动，`ncgo check` 退出 `0`。

**失败恢复**

- proto import 失败时，传入归属 proto 路径并修复 include root。
- module ownership 失败时，应使用 RPC module 支持的 client contract，不要生成
  在 BFF 中无法解析的 import。
- 文件已存在时先审阅；只有明确需要替换时才使用 `--force`。

### Recipe：基础设施 add-on 与 wiring

**前置条件**

- 用 `ncgo add infra --help` 确认 add-on 支持当前服务类型。
- 自动 wiring 前提交或暂存无关修改。

**执行顺序**

1. 用 `ncgo add infra <kind> --root . --wire --dry-run --output json` 预览文件与接线。
2. 审阅 `writtenPaths`、manifest 变化、wiring 目标与 `nextSteps`。
3. 用 `ncgo add infra <kind> --root . --wire` 应用。
4. 补全生成的配置段，并在公共验证命令前执行返回的依赖命令。

**预期文件**

- 计划报告的 add-on 源码/config 文件；
- `.ncgo/manifest.yaml` 记录该 add-on；
- 只有 marker 所有的 server/client wiring 位置发生变化。

**验证**

- 重跑 dry-run 并确认幂等；
- 运行聚焦 add-on 测试、`go build ./...` 与 `ncgo check --root .`。

**失败恢复**

- `--wire` 找不到 marker 时保留生成文件，按文档手工接入 constructor；不要重写
  无关 server 代码。
- 当前服务类型不支持时停止，不要复制另一框架的 adapter。
- 依赖解析失败时，执行返回的 `go get`，再运行 `go mod tidy` 后重试验证。
