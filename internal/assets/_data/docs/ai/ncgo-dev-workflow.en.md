## Developing with ncgo

Choose a recipe by intent. Endpoint commands and internal-domain commands
modify different usecase shapes; do not treat them as interchangeable.

### Command boundary: endpoint versus domain

- `ncgo add rpc-method` copies an already-generated Hertz or Kitex handler
  signature into the top-level `internal/usecase/<service>/usecase.go`. Use it
  for an external endpoint after the IDL generator has run.
- `ncgo add domain` plus `ncgo add method` creates an internal domain package
  and a no-argument `func (u *UseCase) <Method>() error` capability under
  `internal/usecase/<domain>/`.
- Do **not** run both commands as two ways to create the same endpoint method.
  Use both only when an endpoint method deliberately delegates to a separate
  domain capability with a different responsibility and usually a different
  method name.

After any applied recipe, refresh and validate all enabled Agent consumers:

```bash
gofmt -w <changed-go-files>
go build ./...
go vet ./...
go test ./... -count=1
ncgo ai sync --target all --root .
ncgo check --root .
```

### Recipe: Hertz HTTP endpoint

**Prerequisites**

- Run from a Hertz service root with a valid `.ncgo/manifest.yaml`.
- Read `manifest.service.idl`; use that file instead of guessing an IDL path.
- Ensure `hz` and `ncgo` are on `PATH`.

**Sequence**

1. Edit `<manifest.service.idl>` and add or change the HTTP/RPC method.
2. Run `ncgo protolint --root . --file <manifest.service.idl>`.
3. Run `make update` (the Hertz project invokes `hz update`).
4. Run `ncgo add rpc-method --service <manifest.service.name> --rpc <Method> --root .`.
5. Implement the generated top-level usecase stub, then make the generated
   handler call it without importing repository or data packages.
6. Run the common build, test, context-sync, and check commands above.

**Expected files**

- the edited IDL and refreshed generated handler/router/model files;
- `internal/usecase/<service>/usecase.go` with the endpoint signature;
- all five enabled managed Agent context files after sync.

**Validation**

- `ncgo protolint`, `go build`, `go vet`, and `go test` pass;
- `ncgo check --root .` exits `0` after the all-target sync.

**Failure recovery**

- If proto lint fails, fix the reported rule before generation.
- If `make update` fails, verify the manifest IDL path, proto imports, and `hz`.
- If `add rpc-method` cannot find the method, rerun `make update` and confirm
  the generated handler contains the exact method name.
- If the usecase already owns the method, implement it; do not force a duplicate.

### Recipe: Kitex RPC endpoint

**Prerequisites**

- Run from a Kitex service root with a valid manifest and generated handler.
- Read `manifest.service.idl`; ensure `kitex`, `protoc`, and `ncgo` are on `PATH`.

**Sequence**

1. Edit `<manifest.service.idl>` and add or change the RPC method.
2. Run `ncgo protolint --root . --file <manifest.service.idl>`.
3. Run `make update` so Kitex refreshes `kitex_gen/` and the handler signature.
4. Run `ncgo add rpc-method --service <manifest.service.name> --rpc <Method> --root .`.
5. Implement the new top-level usecase method and keep the handler as a thin
   adapter. Run `make sqlc` before `go mod tidy` when generated DB code is used.
6. Run the common build, test, context-sync, and check commands above.

**Expected files**

- the edited IDL, refreshed `kitex_gen/`, and generated handler;
- `internal/usecase/<service>/usecase.go` with the copied RPC signature;
- refreshed Agent contexts.

**Validation**

- proto lint and generation succeed; `go build ./...` compiles the signature;
- tests pass and `ncgo check --root .` exits `0` after sync.

**Failure recovery**

- If generation fails, check proto import roots and the installed Kitex version.
- If `add rpc-method` reports a missing handler method, generation is stale;
  rerun `make update` before retrying.
- If SQL packages are missing, run `make sqlc` before module resolution.

### Recipe: internal domain capability

**Prerequisites**

- The change is internal business behavior, not a new external endpoint.
- Choose a domain name matching `^[a-z][a-z0-9_]{0,62}$` and a method name
  matching `^[A-Z][A-Za-z0-9_]{0,62}$`.

**Sequence**

1. Run `ncgo add domain <domain> --root . --dry-run` and review the plan.
2. Run `ncgo add domain <domain> --root .` if the domain does not exist.
3. Run `ncgo add method <domain>.<Method> --root .`.
4. Replace the no-argument stub body with domain logic and add focused tests.
5. Run `make sqlc` first when the capability changes database queries.
6. Run the common build, test, context-sync, and check commands above.

**Expected files**

- `.ncgo/manifest.yaml` lists the domain;
- `internal/usecase/<domain>/<domain>.go` contains paired method anchors and
  the new method;
- `internal/repository/<domain>/` and `internal/base/data/<domain>_register.go`
  exist when newly generated.

**Validation**

- domain tests and `go test ./... -count=1` pass;
- `check.anchor`, `check.manifest.consistency`, and context checks pass.

**Failure recovery**

- “already exists” means skip domain creation and continue with `add method`.
- “missing markers” means the usecase ownership markers were removed; restore
  them deliberately or regenerate with `add domain --force` only after review.
- Do not use `add rpc-method` to repair an internal domain method.

### Recipe: BFF to RPC client

**Prerequisites**

- Run from the Hertz BFF module that will own the client.
- Locate the RPC service proto and its exact service name; ensure `kitex` is on
  `PATH` and the proto's `go_package` belongs to the current module layout.

**Sequence**

1. Preview with `ncgo add kitex-client <client> --service <rpc-service> --idl <proto> --dry-run`.
2. Apply with `ncgo add kitex-client <client> --service <rpc-service> --idl <proto>`.
3. Wire `pkg/client/<client>` into the BFF usecase/DI layer and supply the RPC
   address through configuration; handlers must not call the client directly.
4. Add client and BFF behavior tests, then run the common validation commands.

**Expected files**

- `pkg/client/<client>/client.go` and `pkg/client/<client>/config.go`;
- generated `kitex_gen/` types and module dependency updates;
- project-owned DI/config edits made during wiring.

**Validation**

- `go mod tidy`, `go build ./...`, and client tests pass;
- the BFF starts with a configured RPC address; `ncgo check` exits `0`.

**Failure recovery**

- If proto imports fail, pass the owning proto path and fix its include roots.
- If module ownership fails, use the RPC module's supported client contract
  rather than generating imports that cannot resolve in the BFF.
- If files already exist, inspect them and use `--force` only when replacement
  is intentional.

### Recipe: infrastructure add-on and wiring

**Prerequisites**

- Confirm the add-on supports this service kind with `ncgo add infra --help`.
- Commit or stash unrelated edits before automatic wiring.

**Sequence**

1. Preview files and wiring with
   `ncgo add infra <kind> --root . --wire --dry-run --output json`.
2. Review `writtenPaths`, the manifest change, wiring targets, and `nextSteps`.
3. Apply with `ncgo add infra <kind> --root . --wire`.
4. Complete the generated configuration section and run any returned dependency
   command before the common validation commands.

**Expected files**

- add-on source/config files reported by the plan;
- `.ncgo/manifest.yaml` records the add-on;
- only marker-owned server/client wiring sites change.

**Validation**

- rerun the dry-run and confirm it is idempotent;
- run focused add-on tests, `go build ./...`, and `ncgo check --root .`.

**Failure recovery**

- If `--wire` cannot find a marker, keep generated files and wire the documented
  constructor manually; do not rewrite unrelated server code.
- If the service kind is unsupported, stop rather than copying another
  framework's adapter.
- If dependency resolution fails, run the returned `go get` command, then
  `go mod tidy`, and retry validation.
