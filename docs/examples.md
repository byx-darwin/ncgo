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

To generate Swagger / OpenAPI docs, install `protoc` through your system tooling
and add the Go plugin:

```bash
go install github.com/hertz-contrib/swagger-generate/protoc-gen-http-swagger@latest
make swagger
```

Generated Hertz projects also include `make install-tools` for Go-side development
tools, including `protoc-gen-http-swagger`; it does not install `protoc` itself.
The Swagger spec is embedded with `go:embed`, so rerun `go run .` / `make dev` or
rebuild and restart the service after `make swagger`.

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

`ncgo doctor --root .` now checks:

- host tools such as `hz` / `kitex`
- `.ncgo/manifest.yaml`
- `template/data.json`
- and Proto I/O rules for the entry proto referenced by `manifest.service.idl`

If doctor reports a failing item with `Rule=PIO...`, that means the project's
`.proto` contract already hit a built-in proto lint rule. Follow up with
`ncgo protolint --root . --file ... --output json` when you want the full
structured diagnostics.

Best for: incrementally growing an existing project without regenerating the
whole service.

## 5. i18n translation workflow in a generated project

Assume you already generated a Hertz service and want to add or refresh
translations for `it-IT`.

First sync locale files and translation status:

```bash
make i18n-sync
make i18n-report
```

Then inspect the structured output through the CLI:

```bash
ncgo i18n report --root . --output json
ncgo i18n check --root . --mode dev --output json
```

A typical flow looks like this:

1. Update `internal/pkg/i18n/locales/zh-CN.json`
2. Run `make i18n-sync`
3. Review `internal/pkg/i18n/.meta/report.md`
4. Use `ncgo i18n report --output json` to inspect `missing_translations` /
   `stale_translations`
5. Update `internal/pkg/i18n/locales/it-IT.json` and
   `internal/pkg/i18n/.meta/status.json`
6. Run `ncgo i18n check --root . --mode release --output json`
7. Once `failures` is empty, run `make i18n`

If you want an agent or MCP client to consume the same structured data, start:

```bash
ncgo mcp serve
```

Then let the agent call:

- `ncgo_i18n_report`
- `ncgo_i18n_check`

Agents should keep their working scope limited to items from
`report.missing_translations`, `stale_translations`, and `draft_translations`
for the current target locale. They should not edit business code or
`catalog_gen.go` directly.

Best for: generated Hertz projects that want a stable chain from manual or
agent-assisted translation updates through final `make i18n` validation.

## 6. Proto contract lint workflow in a generated project

Assume you already generated a service and want `.proto` contract checks in your
local or agent-assisted workflow.

First inspect the structured result through the CLI:

```bash
ncgo protolint --root . --file idl/app/demo.proto --output json
ncgo protolint --root . --file idl/app/demo.proto --rule PIO201 --rule PIO202
```

A typical flow looks like this:

1. Update `idl/*.proto`
2. Run `ncgo protolint --root . --file ... --output json`
3. Review `diagnostics` entries such as `ruleId`, `summary`, `message`, and `field`
4. Narrow the scope with `--rule PIO201 --rule PIO202` when you want to focus on one issue class
5. Fix the proto contract and rerun until `ok=true`

If the run only hits `phase2` warning rules (for example `PIO111`, `PIO112`,
`PIO113`, `PIO211`, `PIO212`, `PIO302`, `PIO303`, or `PIO401`), the
`diagnostics` array still contains those entries, but `ok` stays `true`. Treat
this batch as design guidance and gradual contract cleanup rather than a hard
block on local development.

The current minimal coverage of those warnings is:

- `PIO111` / `PIO112` / `PIO113`: top-level `Empty`, generic message names, and oversized requests
- `PIO211` / `PIO212`: Hertz request fields without bindings, plus missing `openapi.operation/schema/property` metadata
- `PIO302` / `PIO303`: Kitex list/search/query methods without pagination, plus request objects that mix too many filtering/sorting/pagination/debug/extension concerns
- `PIO401`: pagination fields such as `page`, `page_size`, `limit`, or `offset` without obvious PGV range bounds

If you want an agent or MCP client to consume the same structured data, start:

```bash
ncgo mcp serve
```

Then let the agent call:

- `ncgo_protolint`

Recommended inputs are:

- `root`: the proto import root
- `files`: the entry proto files to check
- `rules`: optional rule IDs when you want a narrower lint slice

Agents should keep their scope limited to `.proto` contract fixes implied by
`diagnostics`, and should not mix in unrelated business-code or generated-file
changes.

Best for: projects that want Req/Resp naming, Hertz binding, and Kitex response
shape checks in a repeatable automated workflow.