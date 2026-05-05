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

适合：已经有 ncgo 项目，只想按需逐步增强，而不是重新生成整个服务。