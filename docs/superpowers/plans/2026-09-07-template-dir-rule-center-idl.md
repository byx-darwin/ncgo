# Fix --template-dir rule-center IDL selection Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make `ncgo new --template-dir <rule-center-package>` select the same IDL (`idl/rule-center.proto`) that `ncgo new --preset rule-center` selects, so `--template-dir` renders build cleanly instead of failing with `undefined: conf.RateLimitRuleConfig` / `cannot find module providing package .../ruleservice`.

**Architecture:** `internal/scaffold/mono/mono.go` currently picks the IDL for a `--template-dir` package by scanning `pkg.IDLDir` and taking the lexically-first file (`pkg.IDLs[0]`). For the real `byx-darwin/ncgo-templates` `rule-center` package this directory holds two generic, unrelated placeholder protos, so the wrong one is selected — even though the package's real proto is already written to the fixed path `idl/rule-center.proto` by a separate mechanism (the `kitex-template/ratelimit_proto.yaml` per-file overlay, whose `path:` field is a literal `idl/rule-center.proto`, not templated). The fix adds a `pkg.Meta.Name == "rule-center"` special case in the `TemplateDir` branch that forces `idl = "idl/rule-center.proto"`, exactly mirroring the existing `opts.Preset == "rule-center"` special case a few lines above it. No change to the generic `pkg.IDLs[0]` path for any other package.

The repo's existing test fixture for this package (`seedRuleCenterTemplatePackage` in `internal/scaffold/mono/template_package_test.go`) does not model the real package's `idl/` directory — it writes a single synthetic `idl/rulecenter.proto` file that doesn't exist upstream, so `pkg.IDLs[0]` trivially "worked" in tests while being broken in the field. This plan also corrects that fixture to model the real package shape (the two generic decoy files, no synthetic file) and updates the two tests whose assertions currently encode the bug's symptom (`idl/rulecenter.proto`) as expected behavior.

**Tech Stack:** Go, existing `internal/scaffold/mono` package and its test helpers (`assets.FS()`, `manifest.Load`, `readTreeFile`, `assertFileEqual`, `sortedTemplateNames`).

**Spec:** `docs/superpowers/specs/2026-09-07-template-dir-rule-center-idl-design.md`

## Global Constraints

- Minimal diff. No unrelated refactor (`.claude/rules/go.md` §1, §10).
- `gofmt`-clean; keep existing naming/style (`.claude/rules/go.md` §2).
- Do not change generic `pkg.IDLs[0]` resolution for non-rule-center packages.
- Scaffold/template output is contract-sensitive — golden tests only need updating if embedded template *content* changes (it does not here; only `mono.go` logic and `internal/scaffold/mono` tests change).

---

### Task 1: Correct the rule-center test fixture to model the real upstream package

**Files:**
- Modify: `internal/scaffold/mono/template_package_test.go:263-349` (`seedRuleCenterTemplatePackage`)

**Interfaces:**
- Consumes: `assets.FS()` (existing embedded-asset reader), `t.TempDir()`.
- Produces: a package directory path (`string`), same signature as before — callers in Task 3/4 are unaffected by the signature.

- [ ] **Step 1: Replace the synthetic `idl/rulecenter.proto` write with the two real decoy files**

In `seedRuleCenterTemplatePackage` (`internal/scaffold/mono/template_package_test.go`), delete this block:

```go
	// idl/rulecenter.proto: the full RuleService proto (identical to the
	// preset's ratelimit_proto.yaml body, written under the dashless filename
	// the default kitex IDL path uses for a service named "rulecenter").
	proto, err := ruleCenterIDLBody(srcFS)
	if err != nil {
		t.Fatalf("seed rule-center pkg: idl body: %v", err)
	}
	write("idl/rulecenter.proto", proto)
```

Replace it with:

```go
	// idl/{{ToLower .ServiceName}}.proto and idl/{{ToLower .ServiceName}}-center.proto:
	// generic, unrelated placeholder protos that the real byx-darwin/ncgo-templates
	// rule-center package ships alongside its actual proto. They exist so
	// pkg.IDLs[0]'s lexical-first selection in mono.go picks the WRONG file
	// without the Task 2 fix (Issue #115) — "-center.proto" sorts before
	// ".proto" ('-' < '.'), so {{ToLower .ServiceName}}-center.proto wins.
	// Neither declares "service RuleService"; the real proto is written to
	// the fixed path idl/rule-center.proto by the kitex-template/ratelimit_proto.yaml
	// overlay below (its `path:` field is a literal, non-templated
	// idl/rule-center.proto), a mechanism this decoy directory has no part in.
	decoyProto := []byte("syntax = \"proto3\";\npackage ratelimit.v1;\n\nservice {{.ServiceName}}Service {\n  rpc Get{{.ServiceName}}(Get{{.ServiceName}}Req) returns (Get{{.ServiceName}}Resp);\n}\n\nmessage Get{{.ServiceName}}Req {}\nmessage Get{{.ServiceName}}Resp {}\n")
	write("idl/{{ToLower .ServiceName}}.proto", decoyProto)
	write("idl/{{ToLower .ServiceName}}-center.proto", decoyProto)
```

Note `ruleCenterIDLBody` is still used elsewhere in the file (by `TestGenerateTemplateRuleCenterEquivalentToPreset` in Task 3) — do not delete the function, only this call site.

- [ ] **Step 2: Confirm it still compiles**

Run: `go build ./internal/scaffold/mono/...`
Expected: success (test files build as part of `go vet`/`go test`, but a plain package build won't catch unused-import issues in `_test.go` — use Step below instead).

Run: `go vet ./internal/scaffold/mono/...`
Expected: no errors. (This is expected to still compile even though Task 3/4 tests aren't updated yet — `srcFS` and `ruleCenterIDLBody` remain used by other tests in the same file.)

- [ ] **Step 3: Commit**

```bash
git add internal/scaffold/mono/template_package_test.go
git commit -m "test(scaffold): model rule-center package's real decoy idl files"
```

---

### Task 2: Fix IDL selection for the rule-center package under --template-dir

**Files:**
- Modify: `internal/scaffold/mono/mono.go:99-125`

**Interfaces:**
- Consumes: `pkg.Meta.Name` (`string`, from `scaffoldtemplate.Package.Meta.Name`, already loaded by the existing `scaffoldtemplate.LoadPackage` call at line 104).
- Produces: `idl` (`string`, local var already used by the rest of `Generate`) — no signature change, no new exported symbol.

- [ ] **Step 1: Write the failing test**

Add to `internal/scaffold/mono/template_package_test.go` (near `TestGenerateTemplatePackageIDLNameCoupling`):

```go
// TestGenerateTemplatePackageRuleCenterIDLSelection proves that --template-dir
// against the rule-center package selects the package's real, fixed-path proto
// (idl/rule-center.proto) rather than one of the package's generic decoy idl/
// files — regression test for Issue #115.
func TestGenerateTemplatePackageRuleCenterIDLSelection(t *testing.T) {
	opts := templatePkgOptions(t, seedRuleCenterTemplatePackage(t))
	res, err := Generate(context.Background(), opts)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	// 1. The manifest records the package's real proto path, matching --preset.
	m, err := manifest.Load(opts.Dir)
	if err != nil {
		t.Fatalf("load manifest: %v", err)
	}
	if want, got := "idl/rule-center.proto", m.Service.IDL; got != want {
		t.Errorf("manifest Service.IDL = %q, want %q", got, want)
	}

	// 2. That file carries the real RuleService RPC surface, not a decoy.
	idl := readTreeFile(t, opts.Dir, "idl/rule-center.proto")
	if !strings.Contains(string(idl), "service RuleService") {
		t.Errorf("idl/rule-center.proto missing service RuleService:\n%s", idl)
	}

	// 3. nextSteps target the real proto, not a decoy path.
	found := false
	for _, step := range res.NextSteps {
		if strings.Contains(step, "idl/rule-center.proto") {
			found = true
		}
		if strings.Contains(step, "-center.proto") && !strings.Contains(step, "idl/rule-center.proto") {
			t.Errorf("nextSteps references a decoy idl path: %v", res.NextSteps)
		}
	}
	if !found {
		t.Errorf("nextSteps missing idl/rule-center.proto: %v", res.NextSteps)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/scaffold/mono/... -run TestGenerateTemplatePackageRuleCenterIDLSelection -v`
Expected: FAIL — `manifest Service.IDL = "idl/demo-center.proto", want "idl/rule-center.proto"` (the decoy `{{ToLower .ServiceName}}-center.proto` is selected, `opts.Name` is `"demo"` per `templatePkgOptions`).

- [ ] **Step 3: Write minimal implementation**

In `internal/scaffold/mono/mono.go`, replace the `if opts.TemplateDir != ""` block (starting at line 103) so the IDL section reads:

```go
	var pkg *scaffoldtemplate.Package
	if opts.TemplateDir != "" {
		pkg, err = scaffoldtemplate.LoadPackage(opts.TemplateDir, defaultKind(opts.Kind))
		if err != nil {
			return nil, err
		}
		opts.SkipDefaultTemplates = pkg.Meta.SkipDefaultTemplates
		// The rule-center package's real proto is written to the fixed path
		// idl/rule-center.proto by the kitex-template/ratelimit_proto.yaml
		// per-file overlay (its `path:` field is literal, not templated) —
		// exactly mirroring the opts.Preset == "rule-center" branch above.
		// The package's own idl/ directory additionally ships generic decoy
		// protos unrelated to the service, so the pkg.IDLs[0] scan below must
		// not be trusted for this package (Issue #115).
		if pkg.Meta.Name == "rule-center" {
			idl = filepath.ToSlash(filepath.Join("idl", "rule-center.proto"))
		} else if len(pkg.IDLs) > 0 {
			// A template package's IDL defines the project IDL path: the manifest,
			// generator command, and placeholder all target the package's real proto.
			// This matches defaultIDL for variable-named packages
			// ({{ToLower .ServiceName}}.proto) and fixes fixed-named packages whose
			// filename would otherwise diverge from the default <name>.proto path,
			// leaving a stale empty placeholder. writeIDLPlaceholder never clobbers
			// an existing IDL, so the overlay-written real proto wins.
			rel, err := filepath.Rel(pkg.IDLDir, pkg.IDLs[0])
			if err == nil {
				rel = filepath.ToSlash(rel)
				rel = strings.ReplaceAll(rel, "{{ToLower .ServiceName}}", idlNameToken(opts))
				idl = filepath.ToSlash(filepath.Join("idl", rel))
			}
		}
	}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/scaffold/mono/... -run TestGenerateTemplatePackageRuleCenterIDLSelection -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/scaffold/mono/mono.go internal/scaffold/mono/template_package_test.go
git commit -m "fix(scaffold): select rule-center's fixed idl under --template-dir"
```

---

### Task 3: Update the preset-equivalence test to expect identical IDL paths

**Files:**
- Modify: `internal/scaffold/mono/template_package_test.go:369-495` (`TestGenerateTemplateRuleCenterEquivalentToPreset`)

**Interfaces:**
- Consumes: `seedRuleCenterTemplatePackage` (Task 1's corrected version), `Generate` (Task 2's fixed behavior).
- Produces: nothing new — assertions only.

- [ ] **Step 1: Update the doc comment and IDL-content assertion (step 5 in the test body)**

Replace the function doc comment:

```go
// TestGenerateTemplateRuleCenterEquivalentToPreset proves that generating with
// `--template <rule-center package>` (B) produces a scaffold equivalent to the
// `--preset rule-center` scaffold (A), modulo the documented IDL filename
// difference: preset writes idl/rule-center.proto, template writes
// idl/rulecenter.proto (same content). The manifest's Service.IDL field
// records the differing path; everything else must match.
```

with:

```go
// TestGenerateTemplateRuleCenterEquivalentToPreset proves that generating with
// `--template <rule-center package>` (B) produces a scaffold equivalent to the
// `--preset rule-center` scaffold (A) — including an IDENTICAL IDL path
// (idl/rule-center.proto), fixed by Issue #115. Before that fix, B selected a
// generic decoy proto from the package's idl/ directory instead.
```

Replace the "5. IDL content..." block:

```go
	// 5. IDL content is identical under the two different filenames, and both
	// carry the RuleService RPC surface.
	idlA := readTreeFile(t, dirA, "idl/rule-center.proto")
	idlB := readTreeFile(t, dirB, "idl/rulecenter.proto")
	if !bytes.Equal(idlA, idlB) {
		t.Errorf("IDL content differs (preset idl/rule-center.proto vs template idl/rulecenter.proto)\n--- preset ---\n%s\n--- template ---\n%s", idlA, idlB)
	}
```

with:

```go
	// 5. IDL content is identical at the SAME path, and both carry the
	// RuleService RPC surface.
	assertFileEqual(t, dirA, dirB, "idl/rule-center.proto")
	idlA := readTreeFile(t, dirA, "idl/rule-center.proto")
	idlB := readTreeFile(t, dirB, "idl/rule-center.proto")
```

- [ ] **Step 2: Update the manifest assertions (step 6 in the test body)**

Replace:

```go
	if want, got := "idl/rule-center.proto", mA.Service.IDL; got != want {
		t.Errorf("preset manifest idl = %q, want %q", got, want)
	}
	if want, got := "idl/rulecenter.proto", mB.Service.IDL; got != want {
		t.Errorf("template manifest idl = %q, want %q", got, want)
	}
```

with:

```go
	if want, got := "idl/rule-center.proto", mA.Service.IDL; got != want {
		t.Errorf("preset manifest idl = %q, want %q", got, want)
	}
	if want, got := "idl/rule-center.proto", mB.Service.IDL; got != want {
		t.Errorf("template manifest idl = %q, want %q", got, want)
	}
	if mA.Service.IDL != mB.Service.IDL {
		t.Errorf("manifest idl differs: preset=%q template=%q", mA.Service.IDL, mB.Service.IDL)
	}
```

- [ ] **Step 3: Run the test**

Run: `go test ./internal/scaffold/mono/... -run TestGenerateTemplateRuleCenterEquivalentToPreset -v`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add internal/scaffold/mono/template_package_test.go
git commit -m "test(scaffold): assert rule-center preset/template idl paths match"
```

---

### Task 4: Update the IDL-name-coupling test to the corrected fixed path

**Files:**
- Modify: `internal/scaffold/mono/template_package_test.go:497-551` (`TestGenerateTemplatePackageIDLNameCoupling`)

**Interfaces:**
- Consumes: `seedRuleCenterTemplatePackage` (Task 1), `Generate` (Task 2).
- Produces: nothing new — assertions only.

- [ ] **Step 1: Update the doc comment**

Replace:

```go
// TestGenerateTemplatePackageIDLNameCoupling proves that a template package whose
// IDL has a FIXED filename (rule-center ships idl/rulecenter.proto) defines the
// project IDL path even when the service name lowercases to something else. Without
// the coupling, Generate would set the project IDL to the default <name>.proto path
// (idl/mysvc.proto for a service named my-svc), write a stale empty placeholder there,
// and point the manifest and kitex command at it while the overlay writes the real
// proto to idl/rulecenter.proto — a broken, miswired scaffold.
```

with:

```go
// TestGenerateTemplatePackageIDLNameCoupling proves that the rule-center
// package's FIXED idl/rule-center.proto path defines the project IDL path
// even when the service name lowercases to something else. Without the
// pkg.Meta.Name == "rule-center" coupling (Issue #115), Generate would set the
// project IDL to a decoy path derived from the package's generic idl/
// placeholder files instead.
```

- [ ] **Step 2: Replace every `idl/rulecenter.proto` occurrence with `idl/rule-center.proto`**

The body has four occurrences (comment inside step 1, the `readTreeFile` call, the manifest assertion, the nextSteps `strings.Contains` check). Replace all of them:

```go
	// 1. The package's real proto lands at idl/rule-center.proto (overlay write).
	idl := readTreeFile(t, opts.Dir, "idl/rule-center.proto")
	for _, marker := range []string{"service RuleService", "rpc GetRule"} {
		if !strings.Contains(string(idl), marker) {
			t.Errorf("idl/rule-center.proto missing %q:\n%s", marker, idl)
		}
	}

	// 2. No stale empty placeholder at the default <name>.proto path.
	if _, err := os.Stat(filepath.Join(opts.Dir, "idl", "mysvc.proto")); err == nil {
		t.Error("idl/mysvc.proto placeholder should not exist")
	}

	// 3. The manifest records the package IDL path, not the default <name>.proto.
	m, err := manifest.Load(opts.Dir)
	if err != nil {
		t.Fatalf("load manifest: %v", err)
	}
	if want, got := "idl/rule-center.proto", m.Service.IDL; got != want {
		t.Errorf("manifest Service.IDL = %q, want %q", got, want)
	}

	// 4. nextSteps (the user-facing kitex command) targets the package IDL only.
	foundReal, foundStale := false, false
	for _, step := range res.NextSteps {
		if strings.Contains(step, "idl/rule-center.proto") {
			foundReal = true
		}
		if strings.Contains(step, "idl/mysvc.proto") {
			foundStale = true
		}
	}
	if !foundReal {
		t.Errorf("nextSteps missing idl/rule-center.proto: %v", res.NextSteps)
	}
	if foundStale {
		t.Errorf("nextSteps references stale idl/mysvc.proto: %v", res.NextSteps)
	}
```

- [ ] **Step 3: Run the test**

Run: `go test ./internal/scaffold/mono/... -run TestGenerateTemplatePackageIDLNameCoupling -v`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add internal/scaffold/mono/template_package_test.go
git commit -m "test(scaffold): fix idl-name-coupling test to rule-center's real path"
```

---

### Task 5: Full package validation

**Files:** none (validation only)

- [ ] **Step 1: Run the full mono package test suite**

Run: `go test ./internal/scaffold/mono/... -count=1 -v`
Expected: all tests PASS, including `TestGenerateGoldenDefault` and every `TestGenerateTemplatePackage*` test (golden tests are unaffected since no embedded template content changed).

- [ ] **Step 2: Repo-wide build/vet/test**

Run: `go build ./... && go build . && go vet ./... && go test ./... -count=1`
Expected: all green.

- [ ] **Step 3: gofmt check**

Run: `gofmt -l internal/scaffold/mono/mono.go internal/scaffold/mono/template_package_test.go`
Expected: no output (clean).

- [ ] **Step 4: Manual repro against the real ncgo-templates package (optional but recommended given this bug was only visible against the real upstream package, not the old fixture)**

```bash
go build -o /tmp/ncgo-bin .
rm -rf /tmp/ncgo-templates && git clone --depth 1 https://github.com/byx-darwin/ncgo-templates /tmp/ncgo-templates
/tmp/ncgo-bin new scratch --module github.com/acme/scratch --kind kitex --dir /tmp/scratch-tdir --template-dir /tmp/ncgo-templates/rule-center
grep idl /tmp/scratch-tdir/.ncgo/manifest.yaml
# Expected: idl: idl/rule-center.proto
cd /tmp/scratch-tdir && go mod tidy && go build ./...
# Expected: success (no undefined: conf.RateLimitRuleConfig, no missing ruleservice package)
rm -rf /tmp/scratch-tdir /tmp/ncgo-templates /tmp/ncgo-bin
```

- [ ] **Step 5: No commit needed** — Task 5 is validation-only; nothing to add.
