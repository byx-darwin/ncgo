## ncgo Project Rules

> Paths in the embedded design doc (`docs/ncgo/<profile>/design-doc.*.md`)
> are **template-internal** paths (e.g. `kitex/kitex-template/main.yaml`);
> the generated project's actual paths differ. Read the design doc to see
> the mapping for this project.

- Do not hand-edit generated files. Fix the template or generator instead.
- Respect layer boundaries: handler → usecase → repository.
- Run `make sqlc` before `go mod tidy` (Kitex always; Hertz only with database).
- Add usecase methods via `ncgo add method <domain>.<Method>`, not by hand.
- After changing manifest or generated code, run `ncgo ai sync --root .`.
- Full workflow: see "Implementing a Feature with ncgo" in `AGENTS.md`.
- Architecture reference: `docs/ncgo/<profile>/design-doc.en.md`.
