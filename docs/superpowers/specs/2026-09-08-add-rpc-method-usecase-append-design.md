# Design: `ncgo add rpc-method` — append IDL-derived RPC stubs to existing top-level usecase.go

## Context

User report: after adding an RPC to an existing kitex service's proto and
running `make update`, they had to hand-edit `handler.go` ("merge generated
interface") and add 6 placeholder not-implemented methods to a project-owned
`composite.go` just to get `go build` passing. They initially attributed this
to `ncgo add method` generating a stub with a fixed, argument-less signature
(`func (u *UseCase) Method() error`).

## Investigation

- `ncgo add method` (`internal/scaffold/method/method.go`) only ever targets
  the **domain-layer** usecase file
  (`internal/usecase/<domain>/<domain>.go`, created by `ncgo add domain`).
  It has no knowledge of IDL and is unrelated to kitex/hz code generation.
  This command's behavior is unchanged by this design and is intentionally
  documented (`ncgo-dev-workflow.*.md` step 5) as requiring manual handler
  wiring — that part is by design, not a defect.
- The kitex custom templates
  (`internal/assets/_data/kitex/kitex-template/`) drive the **top-level RPC**
  layer:
  - `handler.yaml` has `update_behavior: cover` — `make update` always
    rewrites `handler.go` in full, driven by `.ServiceInfo.Methods`. A newly
    added RPC in the proto **automatically appears** in `handler.go`.
  - `usecase.yaml` has `update_behavior: skip` — once `usecase.go` exists,
    kitex's own generator never touches it again. A newly added RPC's method
    is **not** appended automatically; the user must hand-write it to satisfy
    the interface the freshly-`cover`-regenerated `handler.go` expects.
- Confirmed with the user: the method they hand-added lives in this
  **top-level RPC usecase.go**, not the domain-layer file `ncgo add method`
  manages. `composite.go` is a project-owned aggregation/decorator file not
  generated or tracked by ncgo anywhere in the repo — out of scope here.
- Reusable infrastructure: `internal/protolint/load.go` already compiles
  `.proto` via `bufbuild/protocompile` into a `Model` with `Service`/`RPC`
  types (`Model.RPCs()`), including input/output message names. No
  equivalent exists for thrift.
- Gap: nothing in the repo maps a proto message name to the Go type name and
  import path protoc-gen-go/kitex would generate for it. This is the one
  piece of new logic this design requires.

## Decision

Add a new, independent CLI command `ncgo add rpc-method` (not a flag on
`add method`, to keep domain-layer and top-level-RPC semantics from
blurring) that appends a single IDL-derived method stub to an existing
top-level `usecase.go`.

### Scope

- **kitex + proto only.** Reuses `internal/protolint`'s existing proto
  parsing. Thrift IDL has zero parsing support anywhere in the repo today;
  adding it is out of scope. The command errors clearly ("thrift IDL not yet
  supported for add rpc-method") rather than failing silently.
- **hz projects**: supported for the `usecase.go` append only. ncgo does not
  own a custom hz handler template (hz's own generator/handler behavior is
  outside ncgo's template control), so no handler-side automation is
  attempted for hz — this matches current behavior and is called out in
  docs, not silently promised.
- **`composite.go`-style downstream aggregation files are explicitly out of
  scope** — they are not ncgo-owned artifacts anywhere in the codebase.
- `ncgo add method` (domain layer) is unchanged.

### CLI / orchestrator surface

- `internal/cli/add.go`: new `newAddRPCMethodCmd`, flags `--service <name>
  --rpc <IDLMethodName>` (plus existing `--root`/`--output` conventions).
- `internal/orchestrator/add.go`: new `RunAddRPCMethod`, thin wrapper
  matching `RunAddMethod`'s pattern, reused by CLI and a new MCP tool
  `ncgo_add_rpc_method`.
- Implementation lives in `internal/scaffold/method` (new file, e.g.
  `rpc.go`) — reuses existing manifest-reading and file-writing helpers
  rather than introducing a new package.

### Flow

1. Read `.ncgo/manifest.yaml` to locate the target service and its proto IDL
   path; reject non-kitex-proto services with a targeted error (thrift → "not
   yet supported"; hz → still allowed for the append-only path).
2. Parse the proto via the existing `protolint/load.go` model; look up
   `--rpc` by name. Unknown name → error listing the available RPC names.
3. Resolve the RPC's request/response message names to their generated Go
   type name + import path, using the proto's `go_package` option and
   protoc-gen-go's message-naming convention (new, small piece of logic;
   exact resolution strategy is worked out during planning/implementation,
   including its failure mode when `go_package` is absent or ambiguous).
4. Render `func (u *UseCase) <RPC>(ctx context.Context, req *pb.<Req>) (resp *pb.<Resp>, err error) { return nil, nil }`
   matching the existing kitex `usecase.yaml` template's style, and append it
   as a new top-level function at the end of `usecase.go` (no anchor needed —
   these are standalone functions, not a block requiring markers), adding
   the resolved import if not already present, then `gofmt`.
5. If a method with that name already exists in `usecase.go`, refuse with a
   clear error instead of producing a duplicate/duplicate-import build
   failure.

### Error handling

- Unknown `--rpc` name → error + list of valid RPC names from the proto.
- Non-proto (thrift) service → explicit "not supported yet" error.
- Method already present in `usecase.go` → explicit refusal, no partial
  write.
- Message-name → Go-type resolution failure (e.g. missing `go_package`) →
  explicit error naming the message and the proto file, no silent guess.

## Scope boundaries

- No changes to `ncgo add method`, `add domain`, or any golden-tested mono
  scaffold output.
- No thrift support.
- No automated handling of downstream, non-ncgo-owned files
  (`composite.go` or equivalent).
- No change to `update_behavior` semantics of existing kitex/hz templates.

## Testing

- New unit tests in `internal/scaffold/method` (sibling to
  `method_test.go`), using a minimal kitex-proto project fixture (small
  `.proto` similar to `internal/protolint/testdata/reqresp/*.proto`) to
  cover: successful append (signature + import correctness), unknown RPC
  name, duplicate method name, thrift-service rejection.
- No new mono-level golden test required — this is a single-file append,
  consistent with `method_test.go`'s existing (non-golden) style.
- Full validation: `go build ./... && go vet ./... && go test ./... -count=1`.

## Documentation

- README.md / docs/examples.md (English + Chinese) gain a section on
  `ncgo add rpc-method`, and the existing "after IDL change, run `make
  update`" workflow docs (`ncgo-dev-workflow.*.md`) get a step referencing
  it for the usecase.go gap, with an explicit note that `composite.go`-style
  downstream files remain manual.
