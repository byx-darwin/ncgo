# Issue #19 — Minor Polish Bundle Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Close three small consistency gaps: (1) a `doctor` Go-toolchain version check, (2) `extract domain --plan` showing the real target module from the target manifest, and (3) documenting `import` kind auto-detection limits in help text + both READMEs.

**Architecture:** Three independent, contract-aware edits. Item 1 adds a `tool.go` check to the doctor `Run` pipeline (reusing the existing semver comparison) plus a SARIF rule descriptor. Item 2 makes `PlanDomain` read the target service manifest (when present) to report the true target module, falling back to the derived `sourceModule/to` value when the target does not exist yet. Item 3 is docs-only (help copy + bilingual README), with one small guard test on the help text.

**Tech Stack:** Go 1.25+ (ncgo CLI), Cobra (CLI/help), stdlib `regexp`/`os`, Markdown docs.

## Global Constraints

- Go `1.25+` builds the ncgo CLI. All files must be `gofmt`-clean and pass `go vet ./...`.
- Doctor output is a **contract surface**: adding a check increments `summary.checkCount`. Tests that call `doctor.Run`/`RunDoctor` directly and assert counts MUST be updated; tests that build `doctor.Report` literals are unaffected.
- `extract` CLI text (`target module: …`) and MCP fields (`targetModule`, `toDir`) are contract surfaces — only the *value source* of `targetModule` changes in `--plan`; field names stay stable.
- README English and Chinese MUST stay aligned for the same behavior.
- Do NOT hand-edit downstream generated project files; these are generator/template/docs changes only.
- Validation order: focused test → package tests → `go build ./... && go vet ./... && go test ./... -count=1` → `./scripts/smoke.sh`.

## File Structure

| File | Responsibility | Change |
|------|----------------|--------|
| `internal/exec/exec.go` | Tool min-version constants + install hints | Add `MinGoVersion`, `InstallHint("go")` |
| `internal/doctor/tools.go` | Per-tool probe + semver parsing | Add `goVersionRE`, `checkGo`, `normalizeGoVersion` |
| `internal/doctor/doctor.go` | `Run` pipeline ordering | Append `checkGo` after kitex |
| `internal/doctor/sarif.go` | SARIF rule descriptors (`doctorFixedRuleMetadata`) | Add `tool.go` descriptor |
| `internal/doctor/doctor_test.go` | `Run`-level doctor tests | Update 2 tests, add Go-check tests |
| `internal/orchestrator/doctor_test.go` | Host-scope count assertion | Bump host check count 2→3 |
| `internal/extract/domain.go` | `PlanDomain` target-module resolution | Prefer target manifest module |
| `internal/extract/domain_test.go` | extract plan/apply tests | Add target-manifest plan test |
| `internal/cli/import.go` | `ncgo import` command + help text | Extend `Long` help copy |
| `internal/cli/import_test.go` | import unit tests | Add help-text guard test |
| `README.md` | EN docs | Add `import` to command table + subsection; note Go in doctor line |
| `README.zh-CN.md` | ZH docs | Mirror EN changes |

---

## Task 1: doctor — Go toolchain version check

**Files:**
- Modify: `internal/exec/exec.go` (constants + hint)
- Modify: `internal/doctor/tools.go` (new probe)
- Modify: `internal/doctor/doctor.go:100-101` (pipeline)
- Modify: `internal/doctor/sarif.go` (`doctorFixedRuleMetadata` map, after `tool.kitex`)
- Test: `internal/doctor/doctor_test.go`, `internal/orchestrator/doctor_test.go`

**Interfaces:**
- Consumes: `exec.Runner`, `exec.Cmd`, `exec.NotFoundError`, existing `semverCompare`, `truncate`, `Check`, `Severity*`.
- Produces: `exec.MinGoVersion` (`"v1.25.0"`), `doctor.checkGo(ctx, runner) Check` (Check ID `tool.go`), `normalizeGoVersion(string) string`.

**Why a separate probe:** `go version` prints `go version go1.25.0 darwin/arm64` — the token is `goX.Y.Z`, **not** `vX.Y.Z`, so the existing `versionRE` (`v[0-9]+\.[0-9]+\.[0-9]+`) cannot parse it. `checkGo` uses a Go-specific regex and normalizes to `vX.Y.Z` before reusing `semverCompare`.

- [ ] **Step 1: Add min-version constant + install hint**

In `internal/exec/exec.go`, extend the const block:

```go
const (
	MinHzVersion    = "v0.9.7"
	MinKitexVersion = "v0.16.1"
	MinGoVersion    = "v1.25.0"
)
```

And add a `case` to `InstallHint`:

```go
	case "go":
		return "download Go (>= " + MinGoVersion + ") from https://go.dev/dl/"
```

- [ ] **Step 2: Write the failing test for the Go check**

Append to `internal/doctor/doctor_test.go`:

```go
func TestRunReportsGoOK(t *testing.T) {
	r := Run(context.Background(), Options{Runner: &scriptedRunner{
		out: map[string]string{
			"hz":    "hz version v0.9.7",
			"kitex": "v0.16.1",
			"go":    "go version go1.25.0 darwin/arm64",
		},
	}})
	g := findCheck(t, r, "tool.go")
	if !g.OK {
		t.Errorf("go not OK: %+v", g)
	}
	if !r.OK() {
		t.Errorf("report not OK: %+v", r.Checks)
	}
}

func TestRunFailsWhenGoTooOld(t *testing.T) {
	r := Run(context.Background(), Options{Runner: &scriptedRunner{
		out: map[string]string{
			"hz":    "hz version v0.9.7",
			"kitex": "v0.16.1",
			"go":    "go version go1.24.3 linux/amd64",
		},
	}})
	g := findCheck(t, r, "tool.go")
	if g.OK {
		t.Errorf("go1.24.3 should fail >= v1.25.0")
	}
	if !strings.Contains(g.Message, "below minimum") {
		t.Errorf("expected 'below minimum' in message: %s", g.Message)
	}
	if r.OK() {
		t.Errorf("report should not be OK when go too old")
	}
}

func TestGoTwoComponentVersionParses(t *testing.T) {
	if got := normalizeGoVersion("go1.25"); got != "v1.25.0" {
		t.Errorf("normalizeGoVersion(go1.25) = %q, want v1.25.0", got)
	}
	if got := normalizeGoVersion("go1.26.5"); got != "v1.26.5" {
		t.Errorf("normalizeGoVersion(go1.26.5) = %q, want v1.26.5", got)
	}
}
```

- [ ] **Step 3: Run test to verify it fails**

Run: `go test ./internal/doctor/ -run 'TestRunReportsGoOK|TestRunFailsWhenGoTooOld|TestGoTwoComponentVersionParses' -count=1 -v`
Expected: FAIL — `checkGo`/`normalizeGoVersion` undefined (compile error), and no `tool.go` check.

- [ ] **Step 4: Implement `checkGo` + `normalizeGoVersion`**

Append to `internal/doctor/tools.go`:

```go
// goVersionRE pulls the goMAJOR.MINOR[.PATCH] token out of `go version`
// output, e.g. "go version go1.25.0 darwin/arm64". Go omits the patch
// component in some releases (go1.25), so it is optional here.
var goVersionRE = regexp.MustCompile(`go[0-9]+\.[0-9]+(?:\.[0-9]+)?`)

// checkGo probes the Go toolchain via `go version`. Unlike hz/kitex, the
// version token is goX.Y[.Z] (no leading v), so it needs its own parser;
// comparison still reuses semverCompare after normalization.
func checkGo(ctx context.Context, r exec.Runner) Check {
	c := Check{ID: "tool.go", Severity: SeverityError}
	res, err := r.Run(ctx, exec.Cmd{Name: "go", Args: []string{"version"}})
	if err != nil {
		var nf *exec.NotFoundError
		if errors.As(err, &nf) {
			c.OK = false
			c.Message = "go not found on PATH"
			c.Hint = exec.InstallHint("go")
			return c
		}
		c.OK = false
		c.Severity = SeverityWarn
		c.Message = fmt.Sprintf("go present but probe failed: %v", err)
		c.Hint = exec.InstallHint("go")
		return c
	}
	out := string(res.Stdout) + "\n" + string(res.Stderr)
	got := goVersionRE.FindString(out)
	if got == "" {
		c.OK = false
		c.Severity = SeverityWarn
		c.Message = "go version unparsable: " + truncate(out, 80)
		c.Hint = "expected output to contain a goX.Y.Z token"
		return c
	}
	cmp, err := semverCompare(normalizeGoVersion(got), exec.MinGoVersion)
	if err != nil {
		c.OK = false
		c.Severity = SeverityWarn
		c.Message = fmt.Sprintf("go: %v", err)
		return c
	}
	if cmp < 0 {
		c.OK = false
		c.Message = fmt.Sprintf("go %s is below minimum %s", got, exec.MinGoVersion)
		c.Hint = exec.InstallHint("go")
		return c
	}
	c.OK = true
	c.Message = fmt.Sprintf("go %s (>= %s)", got, exec.MinGoVersion)
	return c
}

// normalizeGoVersion converts a goX.Y[.Z] token to a strict vX.Y.Z semver,
// defaulting a missing patch component to 0 (e.g. "go1.25" -> "v1.25.0").
func normalizeGoVersion(s string) string {
	parts := strings.Split(strings.TrimPrefix(s, "go"), ".")
	for len(parts) < 3 {
		parts = append(parts, "0")
	}
	return "v" + strings.Join(parts[:3], ".")
}
```

- [ ] **Step 5: Wire `checkGo` into the `Run` pipeline**

In `internal/doctor/doctor.go`, after the kitex line (line 101):

```go
	r.Checks = append(r.Checks, checkTool(ctx, runner, "hz", []string{"--version"}, exec.MinHzVersion))
	r.Checks = append(r.Checks, checkTool(ctx, runner, "kitex", []string{"-version"}, exec.MinKitexVersion))
	r.Checks = append(r.Checks, checkGo(ctx, runner))
```

- [ ] **Step 6: Add a SARIF rule descriptor for `tool.go`**

In `internal/doctor/sarif.go`, inside the `doctorFixedRuleMetadata` map, after the `"tool.kitex"` entry (line ~305), add:

```go
		"tool.go": {
			ID:           "tool.go",
			Name:         "go toolchain available",
			Short:        "Checks that the Go toolchain is on PATH and new enough for ncgo.",
			Full:         "Doctor verifies that the Go toolchain is installed and meets the minimum supported version required to build ncgo and generated projects.",
			Help:         "Install or upgrade Go, then rerun ncgo doctor to verify the toolchain.",
			HelpURI:      doctorLifecycleHelpURI,
			DefaultLevel: "error",
			Tags:         []string{"doctor", "tooling", "go", "blocking"},
			Taxa:         []string{doctorTaxonTooling},
		},
```

- [ ] **Step 7: Run the new Go-check tests to verify they pass**

Run: `go test ./internal/doctor/ -run 'TestRunReportsGoOK|TestRunFailsWhenGoTooOld|TestGoTwoComponentVersionParses' -count=1 -v`
Expected: PASS.

- [ ] **Step 8: Update pre-existing `Run`-level tests broken by the extra check**

Adding `tool.go` changes host-scope `checkCount` from 2 to 3, and an unscripted `go` probe fails (NotFoundError, error severity), which flips `Report.OK()`.

In `internal/doctor/doctor_test.go`, `TestRunReportsHzKitexOK` (≈line 57): add `"go"` to the scripted output, assert the go check, and bump counts:

```go
func TestRunReportsHzKitexOK(t *testing.T) {
	r := Run(context.Background(), Options{Runner: &scriptedRunner{
		out: map[string]string{
			"hz":    "hz version v0.9.7",
			"kitex": "v0.16.1",
			"go":    "go version go1.25.0 darwin/arm64",
		},
	}})
	if r.Scope != ScopeHost {
		t.Fatalf("scope = %q, want %q", r.Scope, ScopeHost)
	}
	hz := findCheck(t, r, "tool.hz")
	if !hz.OK {
		t.Errorf("hz not OK: %+v", hz)
	}
	kx := findCheck(t, r, "tool.kitex")
	if !kx.OK {
		t.Errorf("kitex not OK: %+v", kx)
	}
	g := findCheck(t, r, "tool.go")
	if !g.OK {
		t.Errorf("go not OK: %+v", g)
	}
	if !r.OK() {
		t.Errorf("report not OK: %+v", r.Checks)
	}
	if r.Summary.CheckCount != 3 || r.Summary.PassedCount != 3 || r.Summary.FailedCount != 0 {
		t.Fatalf("summary = %+v", r.Summary)
	}
}
```

In `TestRunVersionUnparsableIsWarn` (≈line 110): add `"go": "go version go1.25.0 darwin/arm64"` to the scripted `out` map so the unparsable-hz warn case keeps `r.OK()` true (an absent go would otherwise add an error-severity failure).

For determinism, also add the same `"go"` entry to the scripted outputs in `TestRunFailsWhenHzAbsent` and `TestRunFailsWhenHzTooOld` (those tests still pass without it, but scripting go isolates the assertion to hz).

- [ ] **Step 9: Update the orchestrator host-scope count assertion**

In `internal/orchestrator/doctor_test.go`, `TestRunDoctorHostOnly` (line 79): change the expected count and script go:

```go
	runner := &scriptedRunner{
		out: map[string]string{
			"hz":    "hz version v0.9.7",
			"kitex": "v0.16.1",
			"go":    "go version go1.25.0 darwin/arm64",
		},
	}
```

```go
	if result.Summary.CheckCount != 3 {
		t.Fatalf("expected 3 host checks, got %d", result.Summary.CheckCount)
	}
```

(Also add the `"go"` entry to `TestRunDoctor`'s scripted runner for a clean host pass; its `CheckCount == 0` guard is otherwise unaffected.)

- [ ] **Step 10: Run doctor + orchestrator package tests**

Run: `go test ./internal/doctor/... ./internal/orchestrator/... -count=1`
Expected: PASS.

- [ ] **Step 11: Commit**

```bash
git add internal/exec/exec.go internal/doctor/tools.go internal/doctor/doctor.go internal/doctor/sarif.go internal/doctor/doctor_test.go internal/orchestrator/doctor_test.go
git commit -m "feat(doctor): add Go toolchain version check (tool.go)"
```

---

## Task 2: extract — show real target module in `--plan`

**Files:**
- Modify: `internal/extract/domain.go` (`PlanDomain`, lines 77-88)
- Test: `internal/extract/domain_test.go`

**Interfaces:**
- Consumes: `manifest.Load`, existing `DomainPlan`, `seedProject`/`seedTargetService` test helpers.
- Produces: `PlanDomain` sets `TargetModule` to the target manifest's `Module` when the target service exists; otherwise the derived `sourceModule/to` value (unchanged behavior).

**Why:** `--plan` currently computes `TargetModule = sourceModule + "/" + to` (a filesystem-subpath guess). `--apply` overwrites it with the real `targetManifest.Module`. When the target RPC service already exists, `--plan` should display that same real module so the preview matches what `--apply` will do.

- [ ] **Step 1: Write the failing test**

Append to `internal/extract/domain_test.go`:

```go
func TestPlanDomainUsesTargetManifestModule(t *testing.T) {
	root := seedProject(t)
	seedTargetService(t, filepath.Join(root, "services", "device-rpc"), "github.com/acme/device-rpc", manifest.KindKitex)
	plan, err := PlanDomain(DomainOptions{Root: root, Name: "device"})
	if err != nil {
		t.Fatalf("PlanDomain: %v", err)
	}
	if plan.TargetModule != "github.com/acme/device-rpc" {
		t.Errorf("TargetModule = %q, want target manifest module github.com/acme/device-rpc", plan.TargetModule)
	}
	if plan.Applied {
		t.Errorf("plan-only run must not set Applied")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/extract/ -run TestPlanDomainUsesTargetManifestModule -count=1 -v`
Expected: FAIL — `TargetModule = "github.com/acme/demo/services/device-rpc"` (derived), not the target manifest module.

- [ ] **Step 3: Implement target-manifest resolution in `PlanDomain`**

In `internal/extract/domain.go`, replace the `TargetModule` assignment in the returned `&DomainPlan{...}` (line 81). Compute it before the return:

```go
	targetModule := strings.TrimRight(m.Module, "/") + "/" + filepath.ToSlash(to)
	if tm, err := manifest.Load(filepath.Join(root, filepath.FromSlash(to))); err == nil && tm.Module != "" {
		targetModule = tm.Module
	}
	return &DomainPlan{
		Root:         root,
		Name:         opts.Name,
		To:           filepath.ToSlash(to),
		TargetModule: targetModule,
		Sources:      sources,
		NextSteps: []string{
			"create target RPC service with `ncgo add rpc " + opts.Name + "-rpc --root <workspace>`",
			"run `ncgo extract domain " + opts.Name + " --root <mono> --to " + filepath.ToSlash(to) + " --apply` to copy planned files",
			"wire clients and update imports manually; automatic migration is future work",
		},
	}, nil
```

(The fallback keeps existing behavior — and existing tests — intact when the target service does not exist yet. `--plan` stays preview-only: it does not validate target kind or fail when the target is absent.)

- [ ] **Step 4: Run the new test to verify it passes**

Run: `go test ./internal/extract/ -run TestPlanDomainUsesTargetManifestModule -count=1 -v`
Expected: PASS.

- [ ] **Step 5: Run the whole extract package to confirm no regression**

Run: `go test ./internal/extract/... -count=1`
Expected: PASS (the derived-value tests `TestPlanDomainDefaultTarget`, `TestPlanDomainCustomTarget` still pass via the fallback).

- [ ] **Step 6: Commit**

```bash
git add internal/extract/domain.go internal/extract/domain_test.go
git commit -m "fix(extract): show target manifest module in extract domain --plan"
```

---

## Task 3: import — document kind auto-detection limits (help + READMEs)

**Files:**
- Modify: `internal/cli/import.go` (`Long` help string)
- Modify: `internal/cli/import_test.go` (guard test)
- Modify: `README.md` (command table + new subsection + doctor note)
- Modify: `README.zh-CN.md` (mirror)

**Interfaces:**
- Consumes: existing `newImportCmd`, `detectKind` markers (`// Code generated by hz.` in `router.go`, `// Code generated by kitex.` in `handler.go`).
- Produces: help text + docs stating that auto-detection relies on generator marker files, and that `--no-generate` scaffolds (which lack markers) require explicit `--kind`.

- [ ] **Step 1: Write the guard test for the help text**

Append to `internal/cli/import_test.go`:

```go
func TestImportHelpDocumentsKindDetection(t *testing.T) {
	cmd := newImportCmd()
	if !strings.Contains(cmd.Long, "marker") {
		t.Errorf("import Long help should mention generator marker files:\n%s", cmd.Long)
	}
	if !strings.Contains(cmd.Long, "--kind") {
		t.Errorf("import Long help should mention explicit --kind for marker-less scaffolds:\n%s", cmd.Long)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/cli/ -run TestImportHelpDocumentsKindDetection -count=1 -v`
Expected: FAIL — current `Long` text contains neither "marker" nor "--kind".

- [ ] **Step 3: Extend the `Long` help string**

In `internal/cli/import.go`, replace the `Long:` value in `newImportCmd`:

```go
		Long: `Detect an existing Hertz or Kitex project and generate .ncgo/manifest.yaml.

ncgo import detects project type from generated markers in the source code,
extracts the Go module path from go.mod, and writes a minimal manifest.
This allows existing projects to use ncgo doctor, ncgo ai sync, and MCP tools.

Kind auto-detection looks for generator marker files: router.go containing
"// Code generated by hz." (Hertz) or handler.go containing
"// Code generated by kitex." (Kitex). Projects scaffolded with
` + "`ncgo new --no-generate`" + ` have not run the generators yet, so they have
no marker files; import such projects with an explicit --kind flag
(for example: ncgo import --root . --kind kitex).`,
```

- [ ] **Step 4: Run the guard test to verify it passes**

Run: `go test ./internal/cli/ -run TestImportHelpDocumentsKindDetection -count=1 -v`
Expected: PASS.

- [ ] **Step 5: Add `ncgo import` to the EN README command table**

In `README.md`, in the "Common Commands" table (≈line 169-183), add a row after `ncgo new`:

```markdown
| `ncgo import` | Generate `.ncgo/manifest.yaml` for an existing Hertz/Kitex project |
```

- [ ] **Step 6: Add an EN README subsection on import detection**

In `README.md`, in the "Diagnostics, lifecycle, and agents" section, immediately after the `ncgo doctor …` explanatory paragraph (≈line 471), update the doctor sentence and add an import note:

Replace:
```markdown
`ncgo doctor` now checks `hz` / `kitex`, the manifest, `template/data.json`, and
```
with:
```markdown
`ncgo doctor` now checks the Go toolchain, `hz` / `kitex`, the manifest, `template/data.json`, and
```

Then append a new paragraph after that block:

```markdown
`ncgo import` reverse-generates `.ncgo/manifest.yaml` for an existing project.
Kind auto-detection relies on generator marker files: `router.go` containing
`// Code generated by hz.` (Hertz) or `handler.go` containing
`// Code generated by kitex.` (Kitex). Projects scaffolded with
`ncgo new --no-generate` have no marker files yet, so import them with an
explicit `--kind` flag (e.g. `ncgo import --root . --kind kitex`).
```

- [ ] **Step 7: Mirror both changes in README.zh-CN.md**

In `README.zh-CN.md`: add the command-table row (after `ncgo new`, ≈line 162-176):

```markdown
| `ncgo import` | 为已有的 Hertz/Kitex 项目反向生成 `.ncgo/manifest.yaml` |
```

Update the doctor summary sentence (the ZH equivalent of "checks hz / kitex, the manifest…") to include the Go toolchain ("Go 工具链"), and add the mirrored import paragraph:

```markdown
`ncgo import` 会为已有项目反向生成 `.ncgo/manifest.yaml`。类型自动检测依赖生成器标记文件：`router.go` 含 `// Code generated by hz.`（Hertz）或 `handler.go` 含 `// Code generated by kitex.`（Kitex）。用 `ncgo new --no-generate` 生成的脚手架还没有标记文件，导入时需要显式指定 `--kind`（例如 `ncgo import --root . --kind kitex`）。
```

- [ ] **Step 8: Run markdown diagnostics on both READMEs**

Run: `npx --yes markdownlint-cli README.md README.zh-CN.md 2>&1 | head -40 || true`
Expected: no new errors introduced by the added rows/paragraphs (tables and fenced code balanced). If the repo has a different markdown linter configured via pre-commit, run that instead.

- [ ] **Step 9: Commit**

```bash
git add internal/cli/import.go internal/cli/import_test.go README.md README.zh-CN.md
git commit -m "docs(import): document kind auto-detection markers and --no-generate --kind"
```

---

## Task 4: Full-repo validation

**Files:** none (validation only)

- [ ] **Step 1: gofmt + vet + build**

Run: `gofmt -l $(find . -name '*.go' -not -path './.git/*') && go vet ./... && go build ./... && go build .`
Expected: no files listed by gofmt; vet and build succeed.

- [ ] **Step 2: Full test suite**

Run: `go test ./... -count=1`
Expected: PASS.

- [ ] **Step 3: Smoke test**

Run: `./scripts/smoke.sh`
Expected: PASS.

- [ ] **Step 4 (if any golden/scaffold output changed): regenerate goldens**

Only if `go test ./...` reports golden mismatches (not expected for this bundle — no scaffold template changes): `go test ./internal/scaffold/mono/... -update-golden -count=1`, then re-run Step 2.

---

## Self-Review

**Spec coverage (Issue #19 acceptance criteria):**
- [x] doctor: Go version check → Task 1 (`tool.go`, `MinGoVersion`, pipeline, SARIF, tests).
- [x] `extract domain --plan`: target module from target manifest → Task 2 (fallback preserves no-target behavior; display now matches `--apply`).
- [x] `import`: document marker-file auto-detection + `--no-generate` needs `--kind` in help + README/README.zh-CN → Task 3.

**Placeholder scan:** none — every code step has concrete code; doc steps have exact markdown.

**Type consistency:** `checkGo`/`normalizeGoVersion`/`MinGoVersion` named identically across Task 1 steps and tests; `TargetModule` field unchanged; `newImportCmd`/`cmd.Long` consistent in Task 3.

**Risk notes for implementer:**
- Tests building `doctor.Report` literals (`internal/mcp/server_test.go`, `internal/cli/doctor_test.go`, and the SARIF/JSON cases in `doctor_test.go` ≈line 400+) are NOT affected — do not change their counts.
- No `sarif_test.go` exists, so the `tool.go` descriptor addition has no test coupling.
- Local toolchain is go1.26.5 (≥ v1.25.0), so the live `tool.go` check passes on dev machines.
