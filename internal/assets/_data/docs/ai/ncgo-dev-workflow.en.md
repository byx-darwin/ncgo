## Implementing a Feature with ncgo

This project is generated and extended with the `ncgo` CLI. Follow this
workflow to add a new feature end-to-end. Every step has a programmatic
contract, so an AI agent can drive it directly.

### Workflow

1. **Add a domain** — `ncgo add domain <name> --root .`
   Generates `internal/usecase/<name>/`, `internal/repository/<name>/`, and
   `internal/base/data/<name>_register.go`, and records the domain in
   `.ncgo/manifest.yaml`. Domain names match `^[a-z][a-z0-9_]{0,62}$`.

2. **Add a usecase method** — `ncgo add method <domain>.<Method> --root .`
   Inserts a `func (u *UseCase) <Method>() error` stub between the
   `// ncgo:methods:start` and `// ncgo:methods:end` markers in the domain
   usecase file. Method names match `^[A-Z][A-Za-z0-9_]{0,62}$`.

3. **Regenerate database code** — `make sqlc`
   Required when the service uses a database (`cfg.Database.Enabled`). Kitex
   services always need this before `go mod tidy`; Hertz services need it
   only when the database scaffold is enabled.

4. **Verify** — `go build ./... && go vet ./... && go test ./... -count=1`
   The scaffold must stay buildable after each method insertion.

5. **Refresh AI context** — `ncgo ai sync --root .`
   Re-renders this project's AI artifacts (see below) so agent context
   reflects the new domain and methods.

### Anchors

- `// ncgo:methods:start` / `// ncgo:methods:end` — method insertion region
  in `internal/usecase/<domain>/<domain>.go`. Do not hand-edit generated
  methods; use `ncgo add method`.
- `// ncgo:wire:domain` — optional wiring marker for `data.Register<Name>`.
  When present, `ncgo add domain --wire` inserts the register call there.

### Verification Checklist

- [ ] `.ncgo/manifest.yaml` lists the new domain
- [ ] `internal/usecase/<domain>/<domain>.go` contains the new method between anchors
- [ ] `go build ./...` passes
- [ ] `ncgo ai sync --root .` completes and reports the managed files written

### Failure Handling

- `ncgo add domain` fails "already exists" — the domain is already present;
  run `ncgo add method` directly or pass `--force`.
- `ncgo add method` fails "missing markers" — the usecase file was hand-edited
  or never generated; regenerate the domain with `ncgo add domain <name> --force`.
- `make sqlc` fails — confirm `sqlc` is installed and the schema files are
  intact; see the project's design doc at `docs/ncgo/<profile>/design-doc.en.md`.
- `ncgo ai sync` refuses to overwrite — a file lacks the
  `<!-- ncgo:managed -->` marker; pass `--force` only if you own the file.
