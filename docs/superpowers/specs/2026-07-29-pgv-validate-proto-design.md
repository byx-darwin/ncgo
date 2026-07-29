# Design: Default protos add PGV `validate.rules` (clears PIO402 advisory)

- **Issue:** [#18](https://github.com/byx-darwin/ncgo/issues/18)
- **Workflow:** `wf-2026-07-29-002` (full mode)
- **Date:** 2026-07-29
- **Status:** Approved (brainstorming Phase 1)

## Context

During full-matrix verification (workflow `wf-2026-07-29-001`, PR #17), a freshly
generated Hertz scaffold still emitted one `PIO402` *warning* from `ncgo protolint`:
the request field `name` in the default `Ping` proto has no PGV string-length
constraint. The fix was deferred from PR #17 because a correct resolution requires
vendoring `validate.proto` (the `protoc-gen-validate` extension definitions, not
currently in the scaffold assets) and wiring it through codegen.

`PIO402` (`internal/protolint/rule_pgv_constraints.go`) warns when a request-message
string field whose name suggests free-text input (`name`, `title`, `keyword`, …)
lacks a PGV length constraint. It is a `LevelWarning`, `Phase2` rule.

## Goal

Default generated protos lint with **0 errors and 0 warnings** under `ncgo protolint`.

## Verified current state

Running `ncgo protolint --root <fixture>` (manifest-driven discovery, the same path
`TestGoldenScaffoldProtoLintsClean` uses) against the four affected Hertz fixtures:

| Fixture | errors | warnings | diagnostics |
|---------|--------|----------|-------------|
| `internal/scaffold/mono/testdata/mono-default` | 0 | 1 | PIO402 on `name` |
| `internal/scaffold/mono/testdata/mono-with-database` | 0 | 1 | PIO402 on `name` |
| `internal/scaffold/mono/testdata/mono-with-rulecenter` | 0 | 1 | PIO402 on `name` |
| `internal/scaffold/bff/testdata/bff-default` | 0 | 1 | PIO402 on `name` |

Each emits **exactly one** warning (PIO402). No other warnings exist, so clearing
PIO402 is sufficient to reach zero warnings across all four fixtures.

## Decisions (approved)

1. **Vendor the full canonical `protoc-gen-validate` `validate.proto`** (proto2,
   supports all rule types: string / int32 / repeated / map / enum / message / …).
   Source of truth: `validate/validate.proto` from `github.com/bufbuild/protoc-gen-validate`
   (the de-facto canonical PGV definition; the `validate.rules` field extension number
   is `1071`, matching the subset already used by `internal/protolint/testdata`).
   Rationale: scaffolds are starting points users extend; the canonical file lets
   them add any PGV rule without editing the vendored file, and it is the file PGV
   tooling expects. (Alternative considered: the 49-line subset already present in
   `internal/protolint/testdata/{pgvconstraints,pgvpagination}/validate/validate.proto`
   — smaller, but users adding other rule types would have to extend it.)
2. **Tighten `TestGoldenScaffoldProtoLintsClean` to assert zero warnings and cover
   all four Hertz fixtures** (table-driven). Rationale: the fix touches all four;
   locking all of them gives the strongest self-consistency guarantee. (Alternative
   considered: keep mono-default only — matches the issue letter but leaves the
   other three unlocked.)

## Design

### Approach

Vendor `validate.proto` as a static embedded asset, emit it into every Hertz scaffold
at `idl/validate/validate.proto`, and add `import "validate/validate.proto";` plus a
`(validate.rules)` string constraint to `PingReq.name` in the rendered service proto.

Rejected alternatives:
- *Conditional emission* of `validate.proto` — every Hertz scaffold ships the Ping
  proto with a `name` field, so it is always needed; conditional logic is unnecessary.
- *Adding the rule in `api.proto` / openapi support protos* — those are the Hertz
  API / OpenAPI extension protos, not the service proto where `PingReq` is declared.

### Change surface

| File | Change |
|------|--------|
| `internal/assets/_data/hertz/validate/validate.proto` | **NEW** — canonical proto2 PGV `validate.proto`. Auto-embedded by the existing `//go:embed all:_data` directive in `internal/assets/assets.go` (no directive change needed). |
| `internal/scaffold/mono/files.go` | (a) `writeHertzProtoSupportFiles`: additionally read embedded `hertz/validate/validate.proto` and write it to `idl/validate/validate.proto`; (b) `renderIDLPlaceholder` (Hertz branch): add `import "validate/validate.proto";` after the existing imports and add `(validate.rules) = { string: { min_len: 1, max_len: 64 } }` to `PingReq.name`. |
| `internal/protolint/self_consistency_test.go` | Tighten `TestGoldenScaffoldProtoLintsClean`: assert `WarningCount == 0` in addition to zero error-level diagnostics; make it table-driven over the four Hertz fixtures. |
| Golden fixtures (regenerate) | `mono-default`, `mono-with-database`, `mono-with-rulecenter`, `bff-default`: updated service proto (import + rule line) and a new `idl/validate/validate.proto`. |

BFF is covered automatically: `internal/scaffold/bff/bff.go` delegates the service
scaffold to `mono.Generate`, so the mono change flows through to BFF.

### Data flow

```
internal/assets/_data/hertz/validate/validate.proto   (embedded)
        │  writeHertzProtoSupportFiles
        ▼
<project>/idl/validate/validate.proto
        ▲  resolved by both consumers:
        │   • protolint: importRoots = [root, root/idl]  →  root/idl/validate/validate.proto ✓
        │   • hz codegen: `hz new … -I idl`              →  idl/validate/validate.proto ✓
<project>/idl/app/<svc>.proto
        import "validate/validate.proto";
        message PingReq { string name = 1 [ … (validate.rules) = { string: { min_len: 1, max_len: 64 } } … ]; }
```

Import resolution is already correct on both sides:
- `internal/protolint/load.go` `importRoots(root)` returns `[root, root/idl]` when an
  `idl/` dir exists, so `import "validate/validate.proto"` resolves to
  `idl/validate/validate.proto`.
- The Hertz generator command (`files.go:generatorCommand`) runs
  `hz new --mod=… --idl=… -I idl …`, so hz/protoc resolves the same path.

The chosen constraint values (`min_len: 1`, `max_len: 64`) mirror the existing
OpenAPI `min_length: 1` / `max_length: 64` on the same field, keeping PGV and
OpenAPI metadata consistent.

### Rendered `PingReq.name` (target)

```proto
import "api.proto";
import "openapi/annotations.proto";
import "validate/validate.proto";

message PingReq {
  string name = 1 [
    (api.query) = "name",
    (openapi.parameter) = { required: true },
    (validate.rules) = { string: { min_len: 1, max_len: 64 } },
    (openapi.property) = {
      title: "Name";
      description: "Ping 请求中的 name 查询参数";
      type: "string";
      min_length: 1;
      max_length: 64;
    }
  ];
}
```

## Edge cases & risks

- **Kitex / micro / rpc unaffected.** The Kitex branch of `renderIDLPlaceholder`
  renders an empty service (no `PingReq.name`), and `writeHertzProtoSupportFiles` is
  only invoked for Hertz (`files.go:writeIDLPlaceholder`). No unused `validate.proto`
  import is added to Kitex protos. micro/rpc golden fixtures contain no Hertz Ping
  proto and do not change.
- **proto2 import inside a proto3 service proto** is standard and legal; PGV's
  `validate.proto` is proto2 and is routinely imported from proto3 files.
- **hz codegen with PGV (the originally deferred risk).** Must be empirically
  verified in Phase 3: confirm `hz new … -I idl` compiles `validate.proto` and still
  generates handlers/models without conflict. If hz trips on the full canonical file,
  the documented fallback is the minimal subset from `internal/protolint/testdata`.
- **No hidden warnings.** Verified all four fixtures emit only PIO402 today, so a
  zero-warning assertion is achievable purely from this change.

## Acceptance criteria (from Issue #18)

- [ ] Vendor `validate.proto` (PGV) into scaffold assets and emit it with new projects
- [ ] Add `(validate.rules)` min_len/max_len to `PingReq.name` in `mono/files.go`
      (and BFF equivalents — covered via `mono.Generate` delegation)
- [ ] Tighten `TestGoldenScaffoldProtoLintsClean` to assert zero warnings too
      (extended to all four Hertz fixtures)
- [ ] Regenerate golden fixtures; verify hz codegen still works with PGV annotations

## Testing strategy

- Regenerate golden fixtures with `-update-golden` for the four Hertz fixtures; the
  review step confirms diffs are exactly: new `idl/validate/validate.proto` + the
  import line + the `(validate.rules)` line.
- Tightened `TestGoldenScaffoldProtoLintsClean` (0 errors **and** 0 warnings, four
  fixtures) is the self-consistency lock between generators and lint rules.
- Full validation: `go build ./... && go build . && go vet ./... &&
  go test ./... -count=1 && ./scripts/smoke.sh`.
- Phase 3 empirical hz check on a throwaway generated project (`hz new … -I idl`).

## Contract-surface note

This change alters generated project file layouts (new `idl/validate/validate.proto`)
and embedded scaffold templates — both contract-sensitive per repo rules. Golden
fixtures and docs are updated together with the code.
