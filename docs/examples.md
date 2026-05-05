# ncgo Example Workflows

These examples are designed as low-risk starting points. The mono examples use
`--no-generate`, so you can inspect generated inputs before invoking `hz` or
`kitex` yourself.

## 1. Mono Hertz service

```bash
ncgo new user-api --module github.com/acme/user-api --kind hertz --no-generate
cd user-api
```

Expected files:

- `.ncgo/manifest.yaml`
- `idl/app/user-api.proto`
- `template/layout.yaml`
- `template/package.yaml`
- `template/data.json`

Typical next steps:

```bash
go mod init github.com/acme/user-api
hz new --mod=github.com/acme/user-api --idl=idl/app/user-api.proto --handler_dir=internal/handler --model_dir=internal/pb --router_dir=internal/router --customize_layout=template/layout.yaml --customize_layout_data_path=template/data.json --customize_package=template/package.yaml
go mod tidy
make dev
```

Best for: HTTP-first services where you want to inspect layout inputs before the
generator runs.

## 2. Mono Kitex service

```bash
ncgo new user-rpc --module github.com/acme/user-rpc --kind kitex --no-generate
cd user-rpc
```

Expected files:

- `.ncgo/manifest.yaml`
- `idl/userrpc.proto`
- `template/kitex-template/...`

Typical next steps:

```bash
go mod init github.com/acme/user-rpc
kitex -module github.com/acme/user-rpc -template-dir template/kitex-template -type protobuf idl/userrpc.proto
go mod tidy
make dev
```

Best for: RPC-first services that want a versioned Kitex template tree.

## 3. Micro workspace

```bash
ncgo new commerce --module github.com/acme/commerce --mode micro
cd commerce
```

Expected files:

- `ncgo.workspace`
- `README.md`
- `services/.gitkeep`

Typical next steps:

```bash
ncgo add rpc user-rpc --root . --plan
ncgo add bff web-bff --root . --plan
```

The `--plan` output lets you preview service paths, module names, and generator
steps before writing files.

Best for: teams that want a single workspace root and add services gradually.

## 4. Expand an existing ncgo project

```bash
ncgo add domain device --root .
ncgo add method device.ListThemes --root . --in usecase
ncgo add infra logging --root . --wire --dry-run
ncgo ai sync --root .
ncgo doctor --root .
```

Best for: incrementally growing an existing project without regenerating the
whole service.