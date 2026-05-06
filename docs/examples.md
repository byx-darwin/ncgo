# ncgo Example Workflows

These examples are designed as low-risk starting points. The mono examples use
`--no-generate`, so you can inspect generated inputs before invoking `hz` or
`kitex` yourself.

## 0. MCP contract-first reference

Start the stdio MCP server with:

```bash
ncgo mcp serve
```

### Common response contract

- `content[0].text` is the human-readable or export-ready payload.
- For structured tools, sibling top-level fields remain the stable machine-
  readable payload for agents.
- `output` defaults to `text`; some tools also support `json` or `sarif`.
- `output` only changes `content[0].text`; it does not remove top-level fields.
- `isError` follows the blocking status. For tools with `ok`, it mirrors
  `!ok`; warning-only lint or doctor runs therefore keep `isError=false`.

### Tool contracts

- `ncgo_version`
  - inputs: none
  - output: text
  - stable result shape: version/build/assets summary in `content[0].text`
- `ncgo_doctor`
  - inputs: `root` (optional), `output=text|json|sarif`
  - stable top-level fields: `root`, `scope`, `summary`, `checks`, `ok`
- `ncgo_ai_sync`
  - inputs: `root`, `lang=en|zh-CN`, `force`, `dryRun`
  - output: text
  - current result shape: pretty JSON summary in `content[0].text` from
    `written` and `skipped`
- `ncgo_i18n_report`
  - inputs: `root`, `output=text|json`
  - stable top-level fields: `root`, `sourceLocale`, `localesDir`,
    `statusPath`, `glossaryPath`, `reportPathJSON`, `reportPathMarkdown`,
    `schema`, `report`, `nextSteps`
- `ncgo_i18n_check`
  - inputs: `root`, `mode=dev|release` (default `dev`), `output=text|json`
  - stable top-level fields: `root`, `mode`, `ok`, `sourceLocale`, `schema`,
    `summary`, `failures`, `warnings`, `nextSteps`
- `ncgo_protolint`
  - inputs: `root`, optional `files`, `rules`, `ignoreRules`, `ignoreFiles`,
    `output=text|json|sarif`
  - stable top-level fields: `root`, `files`, `rulesRun`, `ignoredRules`,
    `ignoredFiles`, `ok`, `summary`, `diagnostics`
- `ncgo_add_infra`
  - inputs: `root`, `kind`, optional `force`, `wire`, `dryRun`,
    `output=text|json`
  - stable top-level fields: `dryRun`, `updated`, `writtenPath`,
    `writtenPaths`, `wiredPaths`, `nextSteps`, `plan`
- `ncgo_add_method`
  - inputs: `root`, `spec=<domain>.<Method>`, `in=usecase`
  - output: text
  - stable result shape: insertion summary in `content[0].text`

The workflow sections below reference these contracts instead of restating the
same transport rules each time.

### Minimal `tools/call` request skeletons

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

Recommended top-level reads: start with `ok`, `summary`, and `checks`; use
`root` and `scope` when you need to route or label the result.

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

Recommended top-level reads: start with `report`, `sourceLocale`, and
`nextSteps`; narrow agent edits to `report.missing_translations`,
`report.stale_translations`, and `report.draft_translations`.

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

Recommended top-level reads: start with `ok`, `failures`, `warnings`, and
`nextSteps`; treat `summary` as the compact status header.

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

Recommended top-level reads: start with `ok`, `summary`, and `diagnostics`; use
`ignoredRules` and `ignoredFiles` when you need to explain suppressed findings.

If `root` already points to an ncgo service root or a micro workspace root, you
can omit `files` and let ncgo auto-discover the entry proto files.

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

Recommended top-level reads: start with `updated`, `plan`, and `nextSteps`; use
`writtenPath` / `writtenPaths` / `wiredPaths` to summarize what changed or would
change.

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
      "lang": "en",
      "dryRun": true
    }
  }
}
```

Text-only result shape: read `content[0].text` for the pretty-JSON-style sync
summary; use `isError` to detect blocking failures.

`ncgo_add_method`

```json
{
  "jsonrpc": "2.0",
  "id": 7,
  "method": "tools/call",
  "params": {
    "name": "ncgo_add_method",
    "arguments": {
      "root": ".",
      "spec": "device.ListThemes",
      "in": "usecase"
    }
  }
}
```

Text-only result shape: read `content[0].text` for the insertion summary; use
`isError` to detect failures.

`ncgo_version`

```json
{
  "jsonrpc": "2.0",
  "id": 8,
  "method": "tools/call",
  "params": {
    "name": "ncgo_version"
  }
}
```

Text-only result shape: no `arguments` payload is required; read
`content[0].text` for the single version/build/assets summary.

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
ncgo doctor --root .
ncgo protolint --root . --output json
```

The `--plan` output lets you preview service paths, module names, and generator
steps before writing files.

When the workspace already contains service manifests, `ncgo doctor --root .`
and `ncgo protolint --root .` automatically walk the services listed in
`ncgo.workspace` and aggregate proto lint results from each
`manifest.service.idl`; you do not need to pass `--file` for each service.

Best for: teams that want a single workspace root and add services gradually.

## 4. Expand an existing ncgo project

```bash
ncgo add domain device --root .
ncgo add method device.ListThemes --root . --in usecase
ncgo add infra logging --root . --wire --dry-run
ncgo ai sync --root .
ncgo doctor --root .
ncgo doctor --root . --output json
ncgo doctor --root . --output sarif > doctor.sarif.json
```

`ncgo doctor --root .` now checks:

- host tools such as `hz` / `kitex`
- `.ncgo/manifest.yaml`
- `template/data.json`
- and Proto I/O rules for the entry proto referenced by `manifest.service.idl`

If doctor reports a failing item with `Rule=PIO...`, that means the project's
`.proto` contract already hit a built-in proto lint rule. Follow up with
`ncgo protolint --root . --file ... --output json` when you want the full
structured diagnostics. Doctor itself now also supports `--output json` and
`--output sarif`; `--json` remains available as a compatibility alias.

If you want to feed doctor into code scanning, IDE diagnostics, or CI artifact
pipelines, you can emit SARIF directly:

`ncgo doctor --root . --output sarif > doctor.sarif.json`

Over MCP, call `ncgo_doctor`; see §0 for the shared response contract and the
stable `root` / `scope` / `summary` / `checks` / `ok` fields.

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

Both MCP tools follow the contract in §0: use `output=text|json` when you want
either a short summary or pretty JSON in `content[0].text`, and consume the
stable top-level `report`, `summary`, `failures`, `warnings`, and `nextSteps`
fields directly when you need machine-readable state.

Agents should keep their working scope limited to items from
`report.missing_translations`, `stale_translations`, and `draft_translations`
for the current target locale. They should not edit business code or
`catalog_gen.go` directly.

Best for: generated Hertz projects that want a stable chain from manual or
agent-assisted translation updates through final `make i18n` validation.

## 6. Proto contract lint workflow in a generated project

Assume you already generated a service and want `.proto` contract checks in your
local or agent-assisted workflow.

If the current directory is a mono service root or a micro workspace root with
registered services, you can omit `--file` and let ncgo auto-discover entry
proto files from the manifest or workspace.

First inspect the structured result through the CLI:

```bash
ncgo protolint --root . --file idl/app/demo.proto --output json
ncgo protolint --root . --output json
ncgo protolint --root . --file idl/app/demo.proto --output sarif > protolint.sarif.json
ncgo protolint --root . --file idl/app/demo.proto --rule PIO201 --rule PIO202
ncgo protolint --root . --file idl/app/demo.proto --ignore-rule PIO212 --ignore-file idl/app/legacy.proto
```

A typical flow looks like this:

1. Update `idl/*.proto`
2. Run `ncgo protolint --root . --file ... --output json`
3. Review `diagnostics` entries such as `ruleId`, `summary`, `message`, and `field`
4. Narrow the scope with `--rule PIO201 --rule PIO202` when you want to focus on one issue class
5. Suppress known legacy warnings explicitly with `--ignore-rule PIO212` or `--ignore-file idl/app/legacy.proto` when they are intentionally deferred
6. Fix the proto contract and rerun until `ok=true`

If you want to feed the result into GitHub code scanning, IDE plugins, or other
static-analysis platforms, you can also emit SARIF directly:

`ncgo protolint --root . --file idl/app/demo.proto --output sarif > protolint.sarif.json`

If the run only hits `phase2` warning rules (for example `PIO111`, `PIO112`,
`PIO113`, `PIO211`, `PIO212`, `PIO302`, `PIO303`, `PIO401`, `PIO402`, `PIO403`,
or `PIO404`), the `diagnostics` array still contains those entries, but `ok`
stays `true`. Treat this batch as design guidance and gradual contract cleanup
rather than a hard block on local development.

The current minimal coverage of those warnings is:

- `PIO111` / `PIO112` / `PIO113`: top-level `Empty`, generic message names, and oversized requests
- `PIO211` / `PIO212`: Hertz request fields without bindings, plus missing `openapi.operation/schema/property` metadata
- `PIO302` / `PIO303`: Kitex list/search/query methods without pagination, plus request objects that mix too many filtering/sorting/pagination/debug/extension concerns
- `PIO401` / `PIO402` / `PIO403` / `PIO404`: pagination fields without range bounds, free-text strings without length bounds, repeated/map fields without count bounds, and enums without `defined_only`

If you want an agent or MCP client to consume the same structured data, start:

```bash
ncgo mcp serve
```

Then let the agent call:

- `ncgo_protolint`

The MCP `ncgo_protolint` tool also accepts `ignoreRules` and `ignoreFiles`, with
the same semantics as CLI `--ignore-rule` and `--ignore-file`.

Recommended inputs are also summarized in §0. For this workflow, the most common
ones are:

- `root`: the proto import root
- `files`: the entry proto files to check
- `rules`: optional rule IDs when you want a narrower lint slice

Agents should keep their scope limited to `.proto` contract fixes implied by
`diagnostics`, and should not mix in unrelated business-code or generated-file
changes.

Best for: projects that want Req/Resp naming, Hertz binding, and Kitex response
shape checks in a repeatable automated workflow.

### Output examples (warning-only run)

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

**`--output text` (default)**

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

Lines prefixed with `!` are warnings; `✗` marks errors. The trailing summary line
reads `ok` here because all five findings are warnings — `ok` only becomes `false`
when at least one error-level diagnostic fires.

**MCP `ncgo_protolint` tool result (warning-only)**

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

This example follows the common MCP contract from §0: `content[0].text` carries
the output-selected text payload, while `ok`, `summary`, and `diagnostics`
remain available as sibling fields. `isError` is `false` here because warnings
never block the run.
