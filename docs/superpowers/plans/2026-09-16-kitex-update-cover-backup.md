# Kitex `make update` cover-file backup + rpcerror skip fix Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Stop `make update` in generated kitex services from silently and unrecoverably discarding hand-edits to `update_behavior: cover` template files — flip the one confirmed genuine hand-extension point (`rpcerror.go`) to `skip`, and add an unconditional backup safety net in the generated Makefile for every remaining `cover`-type file.

**Architecture:** Two independent, additive changes to ncgo's embedded kitex template assets: (1) `internal/assets/_data/kitex/kitex-template/rpcerror.yaml`'s `update_behavior.type` changes from `cover` to `skip`; (2) `internal/assets/_data/kitex/kitex-template/makefile.yaml`'s `update:` recipe gains a pure-POSIX-shell pre-step that scans `template/kitex-template/*.yaml` for `cover`-type fragments and copies any that exist on disk into `.ncgo-backup/<timestamp>/` before invoking `kitex`. Both changes are template-data-only; no Go runtime logic changes are needed because `make update` invokes the vendored `kitex` binary directly and never goes through ncgo's own code.

**Tech Stack:** Go 1.25+, YAML template assets (`gopkg.in/yaml.v3`), Go `testing` package, POSIX `sh`/`awk` (embedded in the generated Makefile), golden-test fixtures under `internal/scaffold/mono/testdata/`.

**Spec:** `docs/superpowers/specs/2026-09-16-kitex-update-cover-backup-design.md`

## Global Constraints

- `make update` invokes the vendored `kitex` binary directly (`kitex -module $(MODULE) -template-dir template/kitex-template -type protobuf $(IDL_FILE)`) — ncgo's own Go code does not run at that point. Any protective logic must live in the Makefile template itself.
- No new dependency on the `ncgo` binary being installed/on PATH at `make update` time.
- No new baseline/hash state file — backups are unconditional (see spec's "Alternatives considered").
- `main.go` and `internal/base/conf/conf.go` stay `update_behavior: cover` (see spec) — do not touch `main.yaml` or `conf.yaml`.
- Contract-sensitive surfaces touched: `internal/assets/_data/kitex/kitex-template/*.yaml` (scaffold templates) and golden fixtures under `internal/scaffold/mono/testdata/**`. Per `.claude/rules/go.md` §7 and §9, golden tests must be updated via `-update-golden`, not hand-edited.

---

### Task 1: Flip `rpcerror.yaml` to `update_behavior: skip`

**Files:**
- Modify: `internal/assets/_data/kitex/kitex-template/rpcerror.yaml`
- Test: `internal/scaffold/mono/kitex_template_update_behavior_test.go`

**Interfaces:**
- Consumes: `scaffoldtemplate.TemplateFile` (from `internal/scaffold/template/types.go`) — already used by the existing test in this file; no new types needed.
- Produces: nothing consumed by later tasks (independent change).

- [ ] **Step 1: Write the failing test**

Add this test function to `internal/scaffold/mono/kitex_template_update_behavior_test.go` (same package, same imports already present — `io/fs`, `testing`, `gopkg.in/yaml.v3`, `github.com/byx-darwin/ncgo/internal/assets`, `scaffoldtemplate "github.com/byx-darwin/ncgo/internal/scaffold/template"`):

```go
// TestRPCErrorTemplateUsesSkipUpdateBehavior locks Issue #123's fix:
// internal/pkg/rpcerror/rpcerror.go is a genuine hand-extension point (new
// business error codes/functions are added over a service's lifetime, with
// no merge mechanism), so update_behavior must be "skip" — otherwise
// `make update` silently discards every hand-added error code on the next
// IDL change, exactly as reported in Issue #123.
func TestRPCErrorTemplateUsesSkipUpdateBehavior(t *testing.T) {
	srcFS := assets.FS()
	b, err := fs.ReadFile(srcFS, "kitex/kitex-template/rpcerror.yaml")
	if err != nil {
		t.Fatalf("read kitex/kitex-template/rpcerror.yaml: %v", err)
	}
	var tpl scaffoldtemplate.TemplateFile
	if err := yaml.Unmarshal(b, &tpl); err != nil {
		t.Fatalf("parse rpcerror.yaml: %v", err)
	}
	if tpl.UpdateBehavior.Type != "skip" {
		t.Errorf("rpcerror.yaml: update_behavior.type = %q, want \"skip\" — make update will silently overwrite hand-added error codes", tpl.UpdateBehavior.Type)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/scaffold/mono/... -run TestRPCErrorTemplateUsesSkipUpdateBehavior -v -count=1`
Expected: FAIL — `rpcerror.yaml: update_behavior.type = "cover", want "skip"`

- [ ] **Step 3: Flip the template's `update_behavior`**

In `internal/assets/_data/kitex/kitex-template/rpcerror.yaml`, change:

```yaml
update_behavior:
  type: cover
```

to:

```yaml
update_behavior:
  type: skip
```

Leave the `path:` and `body:` fields unchanged.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/scaffold/mono/... -run TestRPCErrorTemplateUsesSkipUpdateBehavior -v -count=1`
Expected: PASS

- [ ] **Step 5: Update golden fixtures**

`rpcerror.yaml` is copied verbatim into every generated kitex project's `template/kitex-template/rpcerror.yaml`, so golden fixtures embedding it must be refreshed:

Run: `go test ./internal/scaffold/mono/... -update-golden -count=1`

Then inspect the diff to confirm only `template/kitex-template/rpcerror.yaml`'s `update_behavior` line changed across the golden trees (e.g. `mono-kitex-default`, `mono-kitex-with-database`; `mono-kitex-template-rulecenter` if it includes the default `rpcerror.yaml`):

Run: `git diff --stat internal/scaffold/mono/testdata/`

- [ ] **Step 6: Run the full existing update-behavior test file to confirm no regressions**

Run: `go test ./internal/scaffold/mono/... -run TestRuleCenterHandEditTemplatesUseSkipUpdateBehavior -v -count=1`
Run: `go test ./internal/scaffold/mono/... -run TestGenerateGoldenKitex -v -count=1`
Expected: both PASS

- [ ] **Step 7: Commit**

```bash
git add internal/assets/_data/kitex/kitex-template/rpcerror.yaml \
        internal/scaffold/mono/kitex_template_update_behavior_test.go \
        internal/scaffold/mono/testdata/
git commit -m "fix(kitex-template): rpcerror.go is skip, not cover, on make update

rpcerror.go is a genuine hand-extension point (new business error codes
are added over a service's lifetime, with no merge mechanism). cover
made make update silently discard those additions on every IDL change.
Mirrors the fix already applied to ratelimit_usecase.yaml in #117.

Fixes #123"
```

---

### Task 2: Back up `cover`-type files before `make update` overwrites them

**Files:**
- Modify: `internal/assets/_data/kitex/kitex-template/makefile.yaml`
- Test: `internal/scaffold/mono/kitex_template_update_behavior_test.go`

**Interfaces:**
- Consumes: nothing from Task 1.
- Produces: nothing consumed by later tasks (independent change). The functional test in this task does NOT depend on the real `kitex` binary being installed — it only exercises the backup shell snippet in isolation.

- [ ] **Step 1: Write the failing test**

Add this test function to `internal/scaffold/mono/kitex_template_update_behavior_test.go`. It extracts the backup portion of the `update:` recipe from the embedded `makefile.yaml`, un-escapes Make's `$$` doubling into plain shell `$`, truncates the recipe right before the `kitex` invocation (so the real `kitex` binary is never invoked), and runs the resulting shell snippet in a temp directory seeded with sample `cover`/`skip` template fragments and sample target files.

```go
// TestMakeUpdateBackupsCoverFilesBeforeOverwrite locks Issue #123's
// general safety net: before `make update` lets the vendored kitex binary
// overwrite any update_behavior:cover file, the Makefile's update target
// must back it up to .ncgo-backup/<timestamp>/ so a hand-edit is never
// lost without a recovery path.
func TestMakeUpdateBackupsCoverFilesBeforeOverwrite(t *testing.T) {
	srcFS := assets.FS()
	b, err := fs.ReadFile(srcFS, "kitex/kitex-template/makefile.yaml")
	if err != nil {
		t.Fatalf("read kitex/kitex-template/makefile.yaml: %v", err)
	}
	var tpl scaffoldtemplate.TemplateFile
	if err := yaml.Unmarshal(b, &tpl); err != nil {
		t.Fatalf("parse makefile.yaml: %v", err)
	}

	const marker = "update: ; "
	idx := strings.Index(tpl.Body, marker)
	if idx < 0 {
		t.Fatal("makefile.yaml body has no \"update: ; \" recipe line — did the target name or spacing change?")
	}
	recipeStart := idx + len(marker)
	recipeEnd := strings.Index(tpl.Body[recipeStart:], "\n")
	if recipeEnd < 0 {
		recipeEnd = len(tpl.Body) - recipeStart
	}
	recipe := tpl.Body[recipeStart : recipeStart+recipeEnd]

	kitexIdx := strings.Index(recipe, "kitex -module")
	if kitexIdx < 0 {
		t.Fatal("update recipe has no \"kitex -module\" invocation — did the recipe structure change?")
	}
	backupSnippet := recipe[:kitexIdx]
	if !strings.Contains(backupSnippet, ".ncgo-backup") {
		t.Fatal("update recipe's pre-kitex portion has no .ncgo-backup logic — backup step missing or moved after the kitex call")
	}
	backupSnippet = strings.ReplaceAll(backupSnippet, "$$", "$")
	backupSnippet = strings.TrimPrefix(strings.TrimSpace(backupSnippet), "@")

	dir := t.TempDir()
	tplDir := filepath.Join(dir, "template", "kitex-template")
	if err := os.MkdirAll(tplDir, 0o755); err != nil {
		t.Fatal(err)
	}

	writeFragment := func(yamlName, path, updateType string) {
		content := "path: " + path + "\nupdate_behavior:\n  type: " + updateType + "\nbody: |-\n  package x\n"
		if err := os.WriteFile(filepath.Join(tplDir, yamlName), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	writeFragment("cover_one.yaml", "internal/pkg/rpcerror/rpcerror.go", "cover")
	writeFragment("cover_two.yaml", "main.go", "cover")
	writeFragment("skip_one.yaml", "internal/handler/handler.go", "skip")

	writeTarget := func(relPath, content string) {
		full := filepath.Join(dir, relPath)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	writeTarget("internal/pkg/rpcerror/rpcerror.go", "package rpcerror // hand-edited\n")
	writeTarget("main.go", "package main // hand-edited\n")
	writeTarget("internal/handler/handler.go", "package handler // hand-edited\n")

	cmd := exec.Command("sh", "-c", backupSnippet)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("backup snippet failed: %v\noutput:\n%s\nsnippet:\n%s", err, out, backupSnippet)
	}

	backupRoot := filepath.Join(dir, ".ncgo-backup")
	entries, err := os.ReadDir(backupRoot)
	if err != nil {
		t.Fatalf("read .ncgo-backup: %v (output: %s)", err, out)
	}
	if len(entries) != 1 {
		t.Fatalf(".ncgo-backup: got %d timestamp dirs, want 1", len(entries))
	}
	tsDir := filepath.Join(backupRoot, entries[0].Name())

	for _, want := range []string{"internal/pkg/rpcerror/rpcerror.go", "main.go"} {
		gotPath := filepath.Join(tsDir, want)
		got, err := os.ReadFile(gotPath)
		if err != nil {
			t.Errorf("expected backup of cover file %s at %s: %v", want, gotPath, err)
			continue
		}
		wantContent, _ := os.ReadFile(filepath.Join(dir, want))
		if string(got) != string(wantContent) {
			t.Errorf("backup of %s = %q, want %q", want, got, wantContent)
		}
	}

	if _, err := os.Stat(filepath.Join(tsDir, "internal/handler/handler.go")); err == nil {
		t.Error("skip-type file internal/handler/handler.go was backed up; only cover-type files should be")
	}
}
```

The file already imports `strings` (used by the existing `TestRuleCenterHandEditTemplatesUseSkipUpdateBehavior`). Add the three new imports this test needs — `os`, `os/exec`, `path/filepath` — to the existing import block at the top of `internal/scaffold/mono/kitex_template_update_behavior_test.go`. Do not re-add `strings`.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/scaffold/mono/... -run TestMakeUpdateBackupsCoverFilesBeforeOverwrite -v -count=1`
Expected: FAIL — either the "no .ncgo-backup logic" fatal, or a backup-file-not-found error, since the current `update:` recipe has no backup step yet.

- [ ] **Step 3: Add the backup step to the `update:` recipe**

In `internal/assets/_data/kitex/kitex-template/makefile.yaml`, replace this line:

```yaml
  update: ; @echo "Generating Kitex RPC code from IDL..."; kitex -module $(MODULE) -template-dir template/kitex-template -type protobuf $(IDL_FILE); echo "Kitex code generation complete"
```

with:

```yaml
  update: ; @echo "Generating Kitex RPC code from IDL..."; ts=$$(date +%Y%m%d%H%M%S); n=0; for y in template/kitex-template/*.yaml; do [ -f "$$y" ] || continue; if awk '/^update_behavior:/{f=1;next} f && /type:[[:space:]]*cover/{print "COVER"; exit} /^body:/{exit}' "$$y" | grep -q COVER; then p=$$(awk -F': ' '/^path:/{print $$2; exit}' "$$y"); if [ -n "$$p" ] && [ -f "$$p" ]; then mkdir -p ".ncgo-backup/$$ts/$$(dirname "$$p")"; cp "$$p" ".ncgo-backup/$$ts/$$p"; n=$$((n+1)); fi; fi; done; if [ "$$n" -gt 0 ]; then echo "Backed up $$n cover-managed file(s) to .ncgo-backup/$$ts/ before regeneration — diff them if you made manual edits."; fi; kitex -module $(MODULE) -template-dir template/kitex-template -type protobuf $(IDL_FILE); echo "Kitex code generation complete"
```

Notes on why every `$` inside the shell logic is doubled to `$$`: this line is a Make recipe, and Make itself expands single `$` before handing the line to the shell — `$$` is Make's escape for a literal `$` the shell should see. Only `$(MODULE)` and `$(IDL_FILE)` are genuine Make variables and stay single-`$`.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/scaffold/mono/... -run TestMakeUpdateBackupsCoverFilesBeforeOverwrite -v -count=1`
Expected: PASS

- [ ] **Step 5: Update golden fixtures**

`makefile.yaml` is copied verbatim into every generated kitex project's `template/kitex-template/makefile.yaml`:

Run: `go test ./internal/scaffold/mono/... -update-golden -count=1`
Run: `git diff --stat internal/scaffold/mono/testdata/`

Confirm only `template/kitex-template/makefile.yaml` changed (the `update:` line), across all kitex golden trees (`mono-kitex-default`, `mono-kitex-template-rulecenter`, `mono-kitex-with-database`).

- [ ] **Step 6: Run the full mono package test suite**

Run: `go test ./internal/scaffold/mono/... -count=1`
Expected: PASS (including Task 1's tests and all `TestGenerateGolden*` tests)

- [ ] **Step 7: Commit**

```bash
git add internal/assets/_data/kitex/kitex-template/makefile.yaml \
        internal/scaffold/mono/kitex_template_update_behavior_test.go \
        internal/scaffold/mono/testdata/
git commit -m "fix(kitex-template): back up cover-type files before make update overwrites them

make update invokes the vendored kitex binary directly, bypassing ncgo,
so this can't be enforced from ncgo's Go code. The generated Makefile's
update target now backs up every update_behavior:cover file to
.ncgo-backup/<timestamp>/ before kitex regenerates it, so a hand-edit is
never silently lost without a recovery path.

Fixes #123"
```

---

### Task 3: Document the new backup behavior

**Files:**
- Modify: `README.md`
- Modify: `README.zh-CN.md`
- Modify: `docs/examples.md`
- Modify: `docs/examples.zh-CN.md`

**Interfaces:**
- Consumes: nothing (docs only).
- Produces: nothing.

- [ ] **Step 1: Update `README.md`**

Find this paragraph (around line 373):

```markdown
These files make future IDL updates reproducible (`make update` in generated
Kitex projects, equivalent generator command for Hertz).
```

Replace it with:

```markdown
These files make future IDL updates reproducible (`make update` in generated
Kitex projects, equivalent generator command for Hertz). Before each
`make update` overwrites a `cover`-type template file, the generated
Makefile backs it up to `.ncgo-backup/<timestamp>/` so a hand-edit is never
lost without a recovery path — files whose `update_behavior` is `skip`
(e.g. `internal/pkg/rpcerror/rpcerror.go`) are never touched at all.
```

- [ ] **Step 2: Update `README.zh-CN.md`**

Find the corresponding section describing `template/` retention (search for `template/kitex-template` or the paragraph mirroring README.md's "Generated projects intentionally keep `template/`"). Add a sentence after it:

```markdown
每次 `make update` 覆盖某个 `cover` 类型模板文件之前，生成的 Makefile 会先将其备份到
`.ncgo-backup/<timestamp>/`，避免手工修改被无法恢复地丢弃；`update_behavior` 为
`skip` 的文件（例如 `internal/pkg/rpcerror/rpcerror.go`）则完全不会被覆盖。
```

- [ ] **Step 3: Update `docs/examples.md`**

In the "Mono Kitex service" section (around line 374, right after the `kitex -module ... -template-dir ...` command in "Typical next steps"), add a note:

```markdown
Re-running `kitex -module ... -template-dir template/kitex-template ...`
(via `make update`) backs up every `update_behavior: cover` file to
`.ncgo-backup/<timestamp>/` first, so a hand-edit to a `cover`-type file is
never lost without a recovery path.
```

- [ ] **Step 4: Update `docs/examples.zh-CN.md`**

Add the equivalent Chinese note in the same location as Step 3 (mirroring `docs/examples.md`'s "Mono Kitex service" / 对应中文小节):

```markdown
重新运行 `kitex -module ... -template-dir template/kitex-template ...`
（即 `make update`）之前，会先把所有 `update_behavior: cover` 的文件备份到
`.ncgo-backup/<timestamp>/`，避免手工修改被无法恢复地丢弃。
```

- [ ] **Step 5: Run markdown diagnostics**

Run: `gofmt -l $(find . -name '*.go' -not -path './.git/*')` (confirm no unrelated Go changes)
Verify the four docs render correctly by reading them back (no broken markdown tables/links introduced).

- [ ] **Step 6: Commit**

```bash
git add README.md README.zh-CN.md docs/examples.md docs/examples.zh-CN.md
git commit -m "docs: document make update's new cover-file backup safety net

Fixes #123"
```

---

### Task 4: Full repository validation

**Files:** none (validation only)

**Interfaces:** none

- [ ] **Step 1: Build**

Run: `go build ./... && go build .`
Expected: no errors

- [ ] **Step 2: Vet**

Run: `go vet ./...`
Expected: no errors

- [ ] **Step 3: Full test suite**

Run: `go test ./... -count=1`
Expected: all PASS

- [ ] **Step 4: Smoke test**

Run: `./scripts/smoke.sh`
Expected: PASS

- [ ] **Step 5: gofmt check**

Run: `gofmt -l $(find . -name '*.go' -not -path './.git/*')`
Expected: empty output

No commit for this task — it's pure validation of Tasks 1–3's commits.
