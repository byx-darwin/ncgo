# Hertz `make update` Fix Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Fix the generated Hertz `Makefile` `update` target so it uses `hz update` (incremental, non-destructive) instead of `hz new` (refuses to regenerate / overwrites custom files), and add `(api.vd)` to the proto template so runtime validation works out of the box.

**Architecture:** Modify two embedded source templates (`makefile_yaml.yaml`, `layout.yaml`) and the proto placeholder renderer (`files.go`). The `update` target drops `--customize_layout*` and `--router_dir` (unsupported by `hz update`, read from `.hz` metadata) and `--protoc-gen-validate` (generates to wrong path, unused by hertz runtime validation). Golden test fixtures are regenerated via `-update-golden`.

**Tech Stack:** Go 1.25+, YAML templates, `gf` CLI, hertz scaffold pipeline.

**Spec:** `.cache/workflows/designs/wf-2026-08-26-002-design.md`

## Global Constraints

- `hz update` does NOT support `--customize_layout`, `--customize_layout_data_path`, or `--router_dir` flags (reads from `.hz` metadata).
- `hz update` DOES support `--handler_dir`, `--model_dir`, `--customize_package`.
- Hertz runtime validation uses `(api.vd)` → `vd` struct tag → `BindAndValidate`, NOT protoc-gen-validate. `(validate.rules)` is for OpenAPI docs only.
- `generatorCommand()` in `mono.go` (initial scaffold) must keep using `hz new` — do NOT change it.
- Golden fixtures are regenerated with `-update-golden`, never hand-edited.
- Keep English and Chinese docs aligned if user-facing behavior changes.

---

## Task 1: Fix the Makefile template (source)

**Files:**
- Modify: `internal/assets/_data/hertz/hertz-template/makefile_yaml.yaml:25`

**Interfaces:**
- Consumes: nothing
- Produces: a `make update` target using `hz update` with correct flags

- [ ] **Step 1: Replace the `update` target line**

In `internal/assets/_data/hertz/hertz-template/makefile_yaml.yaml` line 25, replace:

```makefile
update: ; @echo "Generating Hertz HTTP code from IDL..."; hz new --mod=$(MODULE) --idl=$(IDL_FILE) -I idl --handler_dir=internal/handler --model_dir=internal/pb --router_dir=internal/router --customize_layout=template/layout.yaml --customize_layout_data_path=template/data.json --customize_package=template/package.yaml --protoc-plugins validate:lang=go:./internal/pb; echo "Hertz code generation complete"
```

with:

```makefile
update: ; @echo "Generating Hertz HTTP code from IDL..."; hz update --mod=$(MODULE) --idl=$(IDL_FILE) -I idl --handler_dir=internal/handler --model_dir=internal/pb --customize_package=template/package.yaml; echo "Hertz code generation complete"
```

- [ ] **Step 2: Verify the change**

Run: `grep -n "update:" internal/assets/_data/hertz/hertz-template/makefile_yaml.yaml`
Expected: line 25 contains `hz update --mod=$(MODULE)` and does NOT contain `hz new`, `--customize_layout`, `--router_dir`, or `--protoc-plugins`.

---

## Task 2: Fix the layout template (source)

**Files:**
- Modify: `internal/assets/_data/hertz/layout.yaml:4546`

**Interfaces:**
- Consumes: nothing
- Produces: a `make update` target in the layout template using `hz update`

- [ ] **Step 1: Replace the `update` target line**

In `internal/assets/_data/hertz/layout.yaml` line 4546, replace:

```makefile
update: ; @echo "Generating Hertz HTTP code from IDL..."; hz new --mod=$(MODULE) --idl=$(IDL_FILE) -I idl --handler_dir=internal/handler --model_dir=internal/pb --router_dir=internal/router --customize_layout=template/layout.yaml --customize_layout_data_path=template/data.json --customize_package=template/package.yaml; echo "Hertz code generation complete"
```

with:

```makefile
update: ; @echo "Generating Hertz HTTP code from IDL..."; hz update --mod=$(MODULE) --idl=$(IDL_FILE) -I idl --handler_dir=internal/handler --model_dir=internal/pb --customize_package=template/package.yaml; echo "Hertz code generation complete"
```

- [ ] **Step 2: Verify the change**

Run: `grep -n "update:" internal/assets/_data/hertz/layout.yaml`
Expected: line 4546 contains `hz update --mod=$(MODULE)` and does NOT contain `hz new`, `--customize_layout`, `--router_dir`, or `--protoc-plugins`.

---

## Task 3: Add `(api.vd)` to the proto template

**Files:**
- Modify: `internal/scaffold/mono/files.go` (the `renderIDLPlaceholder` function, `PingReq` message)

**Interfaces:**
- Consumes: nothing
- Produces: generated `.proto` files include `(api.vd)` for runtime validation

- [ ] **Step 1: Add the `(api.vd)` annotation**

In `internal/scaffold/mono/files.go`, in the `renderIDLPlaceholder` function, the `PingReq` message currently has:

```go
		`message PingReq {`,
		`  string name = 1 [`,
		`    (api.query) = "name",`,
		`    (openapi.parameter) = { required: true },`,
		`    (validate.rules) = { string: { min_len: 1, max_len: 64 } },`,
```

Insert `(api.vd)` after the `(api.query)` line so it becomes:

```go
		`message PingReq {`,
		`  string name = 1 [`,
		`    (api.query) = "name",`,
		`    (api.vd) = "len($) > 0 && len($) < 65",`,
		`    (openapi.parameter) = { required: true },`,
		`    (validate.rules) = { string: { min_len: 1, max_len: 64 } },`,
```

- [ ] **Step 2: Verify the change**

Run: `grep -n "api.vd" internal/scaffold/mono/files.go`
Expected: line containing `(api.vd) = "len($) > 0 && len($) < 65"` appears in `renderIDLPlaceholder`.

---

## Task 4: Regenerate golden fixtures and verify

**Files:**
- Modify (regenerated): all `Makefile` and `template/*` golden fixtures under:
  - `internal/scaffold/mono/testdata/mono-default/`
  - `internal/scaffold/mono/testdata/mono-with-database/`
  - `internal/scaffold/bff/testdata/bff-default/`

**Interfaces:**
- Consumes: the changed templates from Tasks 1–3
- Produces: updated golden fixtures that reflect the new `hz update` target and `(api.vd)` annotation

- [ ] **Step 1: Run golden tests to confirm they fail (red)**

Run: `go test ./internal/scaffold/mono/... -run 'TestGenerateGoldenDefault|TestGenerateGoldenWithDatabase' -count=1 2>&1 | tail -20`
Expected: FAIL — fixtures still contain `hz new`, new output has `hz update`.

- [ ] **Step 2: Regenerate golden fixtures**

Run: `go test ./internal/scaffold/mono/... -run 'TestGenerateGoldenDefault|TestGenerateGoldenWithDatabase' -update-golden -count=1 2>&1 | tail -10`
Expected: PASS — fixtures regenerated.

- [ ] **Step 3: Regenerate BFF golden fixtures**

Run: `go test ./internal/scaffold/bff/... -run 'TestGenerateGolden' -update-golden -count=1 2>&1 | tail -10`
Expected: PASS — BFF fixtures regenerated. (If no BFF golden test exists, skip this step.)

- [ ] **Step 4: Verify golden tests pass (green)**

Run: `go test ./internal/scaffold/mono/... -run 'TestGenerateGoldenDefault|TestGenerateGoldenWithDatabase' -count=1 2>&1 | tail -10`
Expected: PASS.

- [ ] **Step 5: Verify the regenerated fixtures contain `hz update`**

Run: `grep -rn "hz new\|hz update" internal/scaffold/mono/testdata/ internal/scaffold/bff/testdata/ 2>/dev/null | grep -v "rulecenter" | grep -v "package.yaml" | grep -v "layout.yaml:3:"`
Expected: All `update:` targets show `hz update`; no `hz new` in any `update:` target. (The `layout.yaml:3:` comment and `package.yaml` comment references are expected remnants showing the initial `hz new` scaffold command, which is correct.)

---

## Task 5: Full validation

**Files:** none (validation only)

**Interfaces:**
- Consumes: all changes from Tasks 1–4
- Produces: passing CI-equivalent checks

- [ ] **Step 1: Build**

Run: `go build ./...`
Expected: no errors.

- [ ] **Step 2: Vet**

Run: `go vet ./...`
Expected: no errors.

- [ ] **Step 3: Format check**

Run: `gofmt -l $(find . -name '*.go' -not -path './.git/*')`
Expected: no output (all files formatted).

- [ ] **Step 4: Full test suite**

Run: `go test ./... -count=1 2>&1 | tail -20`
Expected: PASS (all packages).

- [ ] **Step 5: Commit**

```bash
git add internal/assets/_data/hertz/hertz-template/makefile_yaml.yaml \
        internal/assets/_data/hertz/layout.yaml \
        internal/scaffold/mono/files.go \
        internal/scaffold/mono/testdata/ \
        internal/scaffold/bff/testdata/
git commit -m "fix: use hz update in Makefile template and add (api.vd) to proto

The generated Hertz Makefile update target now uses hz update (incremental,
non-destructive) instead of hz new (refuses to regenerate / overwrites
custom files). Drops --customize_layout*, --router_dir (unsupported by
hz update), and --protoc-plugins validate (generates to wrong path,
unused by hertz runtime validation). Adds (api.vd) to the proto template
so runtime validation works out of the box.

Closes #92"
```

Expected: commit succeeds.
