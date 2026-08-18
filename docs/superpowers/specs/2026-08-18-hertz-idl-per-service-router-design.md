# Design: Fix per-service router generation for Hertz + IDL (#70)

- **Workflow:** wf-2026-08-18-001 (standard mode)
- **Issue:** #70 — `fix: ncgo new (hertz + IDL) 不生成 per-service 路由文件导致 pb.Register undefined 编译失败`
- **Classification:** Bounded bug fix in an existing flow.

## Problem

Generating a Hertz project from a template package that carries `idl/app/<service>.proto`
(e.g. `ncgo-templates/ratelimit-hertz`) via `ncgo new --kind hertz --template-dir <pkg>`
does **not** produce the per-service router file `internal/router/pb/<service>.go` (which
defines `pb.Register`). The generated `internal/router/register.go` still calls
`pb.Register(r)`, so the project fails to compile with `undefined: pb.Register`.
`base-hertz` is unaffected only because it ships no `idl/` directory.

## Root cause (confirmed by reproduction)

`internal/scaffold/template/proto.go` → `ParseAllServices` is broken for **every**
invocation:

1. **Path double-join.** It passes the **absolute** proto path to
   `protocompile.Compiler.Compile()` while `SourceResolver.ImportPaths` is set to the
   proto's own directory. protocompile joins the import path with the target, producing a
   doubled path such as `…/idl/app/…/idl/app/demo.proto`, so compilation always errors —
   even for a proto with no imports.
2. **Import root.** Even without the double-join, the service proto's imports
   (`api.proto`, `openapi/annotations.proto`, `validate/validate.proto`) are rooted at
   `idl/` (matching `hz -I idl`), but only `idl/app` was on the import path.

The resulting error is silently discarded at `internal/scaffold/mono/mono.go:198`
(`services, _ := scaffoldtemplate.ParseAllServices(...)`), so `opts.Services` is empty.
The template package's `loop_service: true` templates
(`internal/router/pb/<service>.go`, `internal/handler/pb/<service>_service.go`) therefore
emit nothing → `pb.Register` is undefined → build fails.

## Fix

### `ParseAllServices` (`internal/scaffold/template/proto.go`)

1. Compute the IDL import root: walk up from the proto file for an ancestor directory named
   `idl`; set `ImportPaths = [idlRoot, protoDir]` (deduped). Mirrors `hz -I idl` and
   protolint's `[root, root/idl]` import roots.
2. Compile with the target path **relative to the chosen import root**
   (e.g. `app/demo.proto`), not the absolute path.
3. Fallback to the proto's own directory when there is no `idl` ancestor (keeps the Kitex
   `idl/<name>.proto` convention working).

Prototyped against `idl/app/demo.proto` (with `import "api.proto"`) and a no-import proto —
both parse correctly after the change.

### `mono.go:198` error handling

Keep the flow tolerant but surface failures: on a parse error, print a non-fatal
`fmt.Fprintf(os.Stderr, "ncgo: warning: parse services from %s: %v\n", idl, err)` and
continue with the (possibly empty) service list. This makes silent regressions
diagnosable without breaking flows that legitimately lack a parseable proto.

## Tests (regression)

- `internal/scaffold/template/proto_test.go`: `TestParseAllServices_ImportResolution` —
  temp `idl/app/<svc>.proto` importing `api.proto` (+ a minimal `idl/api.proto`); assert the
  service name and methods parse. Add a no-import case and a Kitex-style `idl/<name>.proto`
  case. Locks the root cause directly.
- Optional `apply_test.go` check that a `loop_service: true` template emits one file per
  parsed service (no such test currently exists).

## Out of scope / unchanged

- **Golden tests:** unaffected — the mono golden tests run with `NoGenerate: true`, so
  `template.Apply` / `ParseAllServices` never execute there. Will run them to confirm zero
  diff.
- **Docs / CLI / MCP / schemas:** no CLI flag, MCP schema, scaffold template, or JSON schema
  changes; `ParseAllServices` signature unchanged.
- End-to-end "`go build ./...` passes out of the box" depends on the external
  `ncgo-templates/ratelimit-hertz` package; the in-repo regression is asserted at the
  `ParseAllServices` level, which is the actual defect.

## Behavior change (intended)

Fixing `ParseAllServices` (so `opts.Services` is populated) re-activates the previously
silently-inert `loop_service: true` templates. The default `ncgo new --kind hertz` flow now
emits per-service files in addition to the template-package flow's per-service router/handler
(`internal/router/pb/<service>.go`, `internal/handler/pb/<service>_service.go`):

- `internal/usecase/<svc>/usecase.go` — always emitted.
- `internal/repository/<svc>repo/repo.go` — emitted when `--with-database` is set.

This is the intended fix and is verified safe by the existing integration tests
`TestGenerateHertzCompiles` and `TestGenerateHertzWithDatabaseCompiles`, which run real
`hz` + `go build` + `go test` on the generated project. Golden output is genuinely unchanged —
the mono golden path is `NoGenerate`-gated and returns before the `ParseAllServices` call.

## Acceptance criteria mapping (#70)

- [x] Per-service router `internal/router/pb/<service>.go` generated & defines `Register`
  → enabled by populating `opts.Services` (fixed parse).
- [x] Generated project `go build ./...` passes out of the box (no template fallback)
  → per-service router + handler now emitted.
- [x] Per-service handler `internal/handler/pb/<service>_service.go` generated
  → same `loop_service` path, same fix.
- [x] proto import-path resolution for `idl/app` + regression test
  → import-root computation + `TestParseAllServices_ImportResolution`.
