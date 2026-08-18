# ncgo DDD export support (子项目1 / P1a) — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make `ncgo export templates` preserve DDD `internal/domain/**` and `internal/application/**` layers so a hand-written DDD project exports cleanly into a template package.

**Architecture:** Add two `FileRule`s to both `HertzRules()` and `KitexRules()` in `internal/scaffold/template/export.go`. The layers are **aggregate-organized** (e.g. `internal/domain/user/`), NOT proto-service-organized, so the rules use `UpdateBehavior: "skip"` and **`LoopService: false`** (the zero value) — export keeps the literal aggregate path and variablizes only module/service in the body. Using `LoopService: true` would wrongly rewrite the aggregate dir segment via `templatePath` (which parameterizes the *service*-name segment).

**Tech Stack:** Go 1.25+, existing `internal/scaffold/template` export machinery, `internal/testutil/golden`.

**Spec:** `docs/superpowers/specs/2026-08-18-micro-admin-ddd-program-design.md`

## Global Constraints

- Contract-sensitive surface: `export.go` FileRule set + `ncgo export` output layout + export golden. Changes require updating tests + docs together.
- Additive only: do NOT change existing rules (`handler/usecase/repository/router/pkg`), their behavior, or `ExcludedPaths`.
- **Design decision (resolves issue #72 review):** domain/application rules are `skip` + `loop_service:false` (aggregate-organized concrete export), NOT `loop_service:true`.
- gofmt-clean; `go vet ./...` clean. Keep diffs minimal.
- DDD layer convention (documented): `internal/domain/<agg>/{entity,valueobject,<agg>,service,repository}.go`; `internal/application/<agg>/{<agg>_service,dto}.go`; `internal/repository/<agg>/` = infra impl (existing rule).

## File Structure

- `internal/scaffold/template/export.go` — add 2 FileRules to `HertzRules()` (after line 31) and `KitexRules()` (after line 48).
- `internal/scaffold/template/export_test.go` — add `TestExport_DDDLayers_Hertz` and `TestExport_DDDLayers_Kitex`.
- `internal/scaffold/template/golden_test.go` + `testdata/golden/export-{hertz,kitex}/` — extend fixtures with a domain+application aggregate; regenerate golden.
- Docs: nearest `ncgo export` doc + EN/ZH examples — document the DDD convention + export behavior.

---

### Task 1: Add domain/application FileRules + unit tests

**Files:**
- Modify: `internal/scaffold/template/export.go` (`HertzRules()` ~line 22-36, `KitexRules()` ~line 39-55)
- Test: `internal/scaffold/template/export_test.go`

**Interfaces:**
- Consumes: `Export(ExportOptions) (*ExportResult, error)`, `FileRule{Pattern, UpdateBehavior, LoopService}`, `writeFileExport(t, root, rel, content)` (existing test helper).
- Produces: no signature change; two new `FileRule` entries in each rule set.

- [ ] **Step 1: Write the failing test**

Add to `internal/scaffold/template/export_test.go` (imports `os`, `path/filepath`, `strings`, `gopkg.in/yaml.v3` already used in pkg; use a small local helper to load exported yamls):

```go
// loadExportedTemplateByPath walks template/<kind>-template/*.yaml and returns
// the TemplateFile whose Path matches want, or fails.
func loadExportedTemplateByPath(t *testing.T, root, kind, want string) *TemplateFile {
	t.Helper()
	dir := filepath.Join(root, "template", kind+"-template")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read template dir: %v", err)
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".yaml") {
			continue
		}
		tpl, err := readTemplateYAML(filepath.Join(dir, e.Name()))
		if err != nil {
			t.Fatalf("read yaml %s: %v", e.Name(), err)
		}
		if tpl.Path == want {
			return tpl
		}
	}
	t.Fatalf("no exported template with path %q in %s", want, dir)
	return nil
}

func TestExport_DDDLayers_Hertz(t *testing.T) {
	dir := t.TempDir()
	writeFileExport(t, dir, "main.go", "package main\n\nfunc main() {}\n")
	// aggregate "user" — name deliberately != service name "UserApi"
	writeFileExport(t, dir, "internal/domain/user/entity.go", `package user

// User is an aggregate root in module github.com/acme/test.
type User struct{ ID string }
`)
	writeFileExport(t, dir, "internal/domain/user/repository.go", `package user

import "context"

type Repository interface {
	Save(ctx context.Context, u *User) error
}
`)
	writeFileExport(t, dir, "internal/application/user/user_service.go", `package user

import "github.com/acme/test/internal/domain/user"

type Service struct{ repo user.Repository }
`)

	if _, err := Export(ExportOptions{Root: dir, Kind: "hertz",
		Module: "github.com/acme/test", ServiceName: "UserApi"}); err != nil {
		t.Fatalf("export: %v", err)
	}

	// domain entity: aggregate path preserved literally, module variabilized, skip.
	ent := loadExportedTemplateByPath(t, dir, "hertz", "internal/domain/user/entity.go")
	if ent.UpdateBehavior.Type != "skip" {
		t.Errorf("domain entity UpdateBehavior = %q, want skip", ent.UpdateBehavior.Type)
	}
	if ent.LoopService {
		t.Errorf("domain entity LoopService = true, want false (aggregate-organized)")
	}
	if !strings.Contains(ent.Body, "module {{.Module}}") {
		t.Errorf("module not variabilized in domain entity:\n%s", ent.Body)
	}

	// application service: path preserved, import module variabilized.
	app := loadExportedTemplateByPath(t, dir, "hertz", "internal/application/user/user_service.go")
	if app.UpdateBehavior.Type != "skip" {
		t.Errorf("application service UpdateBehavior = %q, want skip", app.UpdateBehavior.Type)
	}
	if !strings.Contains(app.Body, "{{.Module}}/internal/domain/user") {
		t.Errorf("import module not variabilized in application service:\n%s", app.Body)
	}
}

func TestExport_DDDLayers_Kitex(t *testing.T) {
	dir := t.TempDir()
	writeFileExport(t, dir, "main.go", "package main\n\nfunc main() {}\n")
	writeFileExport(t, dir, "internal/domain/role/entity.go", `package role

// Role is an aggregate root in module github.com/acme/test.
type Role struct{ ID string }
`)
	writeFileExport(t, dir, "internal/application/role/role_service.go", `package role

import "github.com/acme/test/internal/domain/role"

type Service struct{ _ role.Role }
`)

	if _, err := Export(ExportOptions{Root: dir, Kind: "kitex",
		Module: "github.com/acme/test", ServiceName: "UserRpc"}); err != nil {
		t.Fatalf("export: %v", err)
	}

	ent := loadExportedTemplateByPath(t, dir, "kitex", "internal/domain/role/entity.go")
	if ent.UpdateBehavior.Type != "skip" || ent.LoopService {
		t.Errorf("kitex domain entity behavior wrong: skip=%q loop=%v", ent.UpdateBehavior.Type, ent.LoopService)
	}
	if !strings.Contains(ent.Body, "module {{.Module}}") {
		t.Errorf("module not variabilized in kitex domain entity:\n%s", ent.Body)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/scaffold/template/ -run 'TestExport_DDDLayers' -v -count=1`
Expected: FAIL — `no exported template with path "internal/domain/user/entity.go"` (rules absent, files not exported).

- [ ] **Step 3: Write minimal implementation**

In `internal/scaffold/template/export.go`, in `HertzRules()` immediately after the `internal/repository/**/*.go` rule (line 31):

```go
		{Pattern: "internal/domain/**/*.go", UpdateBehavior: "skip"},
		{Pattern: "internal/application/**/*.go", UpdateBehavior: "skip"},
```

In `KitexRules()` immediately after its `internal/repository/**/*.go` rule (line 48):

```go
		{Pattern: "internal/domain/**/*.go", UpdateBehavior: "skip"},
		{Pattern: "internal/application/**/*.go", UpdateBehavior: "skip"},
```

(`LoopService` is omitted → defaults to `false`, the intended behavior.)

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/scaffold/template/ -run 'TestExport_DDDLayers' -v -count=1`
Expected: PASS (both hertz + kitex).

- [ ] **Step 5: Run the package suite (catch count-test / regression)**

Run: `go test ./internal/scaffold/template/... -count=1`
Expected: PASS. (`TestHertzRules_Count`/`TestKitexRules_Count` use `>=` bounds — still pass.)

- [ ] **Step 6: Commit**

```bash
git add internal/scaffold/template/export.go internal/scaffold/template/export_test.go
git commit -m "feat(export): export DDD domain/application layers (#72)"
```

---

### Task 2: Extend export golden fixtures with a DDD aggregate

**Files:**
- Modify: `internal/scaffold/template/golden_test.go` (add domain/application fixture files to `TestExportGoldenHertz` and `TestExportGoldenKitex`)
- Regenerate: `internal/scaffold/template/testdata/golden/export-hertz/` and `export-kitex/`

- [ ] **Step 1: Add fixture files to the golden tests**

In `TestExportGoldenHertz` (before the `Export(...)` call), add:

```go
	writeFixture(t, root, "internal/domain/account/entity.go", `package account

// Account is an aggregate root under github.com/acme/golden.
type Account struct{ ID string }
`)
	writeFixture(t, root, "internal/application/account/account_service.go", `package account

import "github.com/acme/golden/internal/domain/account"

type Service struct{ _ account.Account }
`)
```

Add the equivalent two files to `TestExportGoldenKitex` (using its module `github.com/acme/golden` and an aggregate name, e.g. `account`).

- [ ] **Step 2: Confirm the golden assertion currently fails (new files not in snapshot)**

Run: `go test ./internal/scaffold/template/ -run 'TestExportGolden' -count=1`
Expected: FAIL — golden tree missing the new `internal_domain_account_*.yaml` / `internal_application_account_*.yaml` templates.

- [ ] **Step 3: Regenerate golden**

Run: `go test ./internal/scaffold/template/ -run 'TestExportGolden' -count=1 -update-golden`
Expected: snapshots rewritten under `testdata/golden/export-{hertz,kitex}/template/`.

- [ ] **Step 4: Verify golden now passes + inspect the new templates**

Run: `go test ./internal/scaffold/template/ -run 'TestExportGolden' -count=1`
Expected: PASS.
Then confirm the regenerated yaml for `internal/domain/account/entity.go` has `update_behavior: {type: skip}`, no `loop_service: true`, and `{{.Module}}` in the body (spot-check the file).

- [ ] **Step 5: Commit**

```bash
git add internal/scaffold/template/golden_test.go internal/scaffold/template/testdata/golden
git commit -m "test(export): golden coverage for DDD domain/application layers (#72)"
```

---

### Task 3: Document the DDD layer convention + export behavior (EN + ZH)

**Files:**
- Modify: the nearest `ncgo export` doc and its Chinese twin. Locate first:

- [ ] **Step 1: Locate export docs**

Run: `grep -rln "export templates\|ncgo export" README.md README.zh-CN.md docs/ 2>/dev/null`
Pick the doc(s) that describe `ncgo export`; identify the EN/ZH pair.

- [ ] **Step 2: Add a DDD-layers subsection to the EN doc**

Add prose stating: `ncgo export templates` now also captures `internal/domain/**` and `internal/application/**` (business layers, `skip` update-behavior, exported as concrete per-aggregate files — not looped per proto service). Include the layer convention:
```
internal/domain/<agg>/        entity.go valueobject.go <agg>.go service.go repository.go   (domain model + repo port)
internal/application/<agg>/    <agg>_service.go dto.go                                       (application service)
internal/repository/<agg>/     repo implementation (sqlc-backed)
```

- [ ] **Step 3: Mirror the change in the ZH doc**

Add the equivalent Chinese subsection, keeping wording aligned with the EN version.

- [ ] **Step 4: Markdown diagnostics**

Run: `grep -rn "internal/domain\|internal/application" README*.md docs/ | head` to confirm the additions landed; visually confirm headings/tables are well-formed.

- [ ] **Step 5: Commit**

```bash
git add README.md README.zh-CN.md docs/
git commit -m "docs(export): document DDD domain/application export layers (#72)"
```

---

### Task 4: Full validation

- [ ] **Step 1: Repository validation**

Run:
```bash
go build ./... && go build . && go vet ./... && go test ./... -count=1
```
Expected: all PASS (28+ packages).

- [ ] **Step 2: Smoke**

Run: `./scripts/smoke.sh`
Expected: `smoke OK`.

- [ ] **Step 3: gofmt**

Run: `gofmt -l internal/scaffold/template/export.go internal/scaffold/template/export_test.go internal/scaffold/template/golden_test.go`
Expected: no output.

---

## Self-Review

**Spec coverage:**
- Add domain/application FileRules (hertz+kitex) → Task 1.
- Correct behavior `skip`+`loop_service:false` (resolves #72 review) → Task 1 asserted, Task 2 golden-locked.
- Export unit tests (both kinds) → Task 1.
- Golden update → Task 2.
- Docs EN+ZH → Task 3.
- Full validation → Task 4.

**Placeholder scan:** none — all steps carry concrete code/commands. Task 3's exact doc file is resolved at Step 1 by grep (doc location not assumable), then edited with the concrete content given.

**Type consistency:** `loadExportedTemplateByPath` returns `*TemplateFile`; asserts `.Path`, `.UpdateBehavior.Type`, `.LoopService`, `.Body` — all existing fields of `TemplateFile`. `FileRule{Pattern, UpdateBehavior, LoopService}` matches the struct. `Export`/`ExportOptions`/`readTemplateYAML`/`writeFileExport`/`writeFixture` are all existing symbols.
