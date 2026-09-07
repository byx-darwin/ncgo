# Rule-Center Usecase update_behavior Fix Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Stop `make update` from silently overwriting hand-written business logic in `internal/usecase/rulecenter/usecase.go` for kitex rule-center-preset projects, by correcting a mismarked `update_behavior` field, and add two narrow regression tests locking the fix and guarding against the related drift pattern found in the hz rate-limit middleware template.

**Architecture:** This is a one-line data fix to an embedded YAML template (`internal/assets/_data/kitex/kitex-template/ratelimit_usecase.yaml`), no Go logic changes. Two new Go unit tests are added under `internal/scaffold/mono` (the existing home for kitex-template/hz-layout asset assertions, see `shared_fragments_test.go`), reusing the already-exported `internal/scaffold/template.TemplateFile` struct to parse template YAML. The golden snapshot for the rule-center `--template-dir` path is regenerated to reflect the corrected field.

**Tech Stack:** Go 1.25+, `gopkg.in/yaml.v3`, existing `internal/testutil/golden` snapshot helper.

**Spec:** `docs/superpowers/specs/2026-09-08-ratelimit-usecase-update-behavior-design.md`

## Global Constraints

- No CLI flag, MCP schema, or documented output-format changes — this is an embedded-template data fix plus tests only.
- Do not touch `internal/scaffold/framework/adapter_kitex.go`, the kitex adapter's generator invocation, or `update_behavior` semantics generally — the mechanism is confirmed correct (verified against vendored `kitex@v0.16.3`).
- `ratelimit_handler.yaml` and `ratelimit_repository.yaml` stay `cover` — do not change them (they are thin wiring/plumbing, not hand-edit targets).
- Golden test snapshots (`internal/scaffold/mono/testdata/...`) are contract-sensitive per `.claude/rules/go.md` — any diff beyond the intended one-line `update_behavior` change must be treated as a bug and investigated before blessing with `-update-golden`.
- Casbin/redis.go/main.go/roles-table items reported in Issue #117 are explicitly out of scope (confirmed not reproducible in ncgo's own templates).

---

### Task 1: Fix `ratelimit_usecase.yaml` update_behavior and lock it with a regression test

**Files:**
- Modify: `internal/assets/_data/kitex/kitex-template/ratelimit_usecase.yaml`
- Create: `internal/scaffold/mono/kitex_template_update_behavior_test.go`

**Interfaces:**
- Consumes: `github.com/byx-darwin/ncgo/internal/assets` (`assets.FS()`, already used elsewhere in this package — see `shared_fragments_test.go`), `github.com/byx-darwin/ncgo/internal/scaffold/template` (`template.TemplateFile`, exported fields `Path`, `Body`, `UpdateBehavior.Type` — see `internal/scaffold/template/types.go:6-17`).
- Produces: nothing consumed by later tasks (Task 2 is independent).

- [ ] **Step 1: Write the failing regression test**

Create `internal/scaffold/mono/kitex_template_update_behavior_test.go`:

```go
package mono

import (
	"io/fs"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/byx-darwin/ncgo/internal/assets"
	scaffoldtemplate "github.com/byx-darwin/ncgo/internal/scaffold/template"
)

// TestRuleCenterHandEditTemplatesUseSkipUpdateBehavior locks Issue #117's
// fix: any kitex-template file in the rule-center preset whose body invites
// hand-editing ("Edit business logic here") must declare
// update_behavior: skip, or `make update` (which invokes the vendored kitex
// -template-dir binary directly, bypassing ncgo) silently overwrites the
// user's business logic on every regeneration.
func TestRuleCenterHandEditTemplatesUseSkipUpdateBehavior(t *testing.T) {
	const handEditMarker = "Edit business logic here"
	srcFS := assets.FS()
	entries, err := fs.ReadDir(srcFS, "kitex/kitex-template")
	if err != nil {
		t.Fatalf("read kitex-template dir: %v", err)
	}
	checked := 0
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".yaml") {
			continue
		}
		b, err := fs.ReadFile(srcFS, "kitex/kitex-template/"+e.Name())
		if err != nil {
			t.Fatalf("read %s: %v", e.Name(), err)
		}
		var tpl scaffoldtemplate.TemplateFile
		if err := yaml.Unmarshal(b, &tpl); err != nil {
			t.Fatalf("parse %s: %v", e.Name(), err)
		}
		if !strings.Contains(tpl.Body, handEditMarker) {
			continue
		}
		checked++
		if tpl.UpdateBehavior.Type != "skip" {
			t.Errorf("%s: body invites hand-editing (%q) but update_behavior.type = %q, want \"skip\" — make update will silently overwrite user edits",
				e.Name(), handEditMarker, tpl.UpdateBehavior.Type)
		}
	}
	if checked == 0 {
		t.Fatal("no kitex-template file matched the hand-edit marker — test is not exercising the intended files; did the marker text change?")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/scaffold/mono/... -run TestRuleCenterHandEditTemplatesUseSkipUpdateBehavior -v`
Expected: FAIL — `ratelimit_usecase.yaml: body invites hand-editing ("Edit business logic here") but update_behavior.type = "cover", want "skip"`

- [ ] **Step 3: Fix the template**

Edit `internal/assets/_data/kitex/kitex-template/ratelimit_usecase.yaml`, changing:

```yaml
update_behavior:
  type: cover
```

to:

```yaml
update_behavior:
  type: skip
```

(This is the only line that changes in the file — the `body:` and `path:` fields stay identical.)

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/scaffold/mono/... -run TestRuleCenterHandEditTemplatesUseSkipUpdateBehavior -v`
Expected: PASS

- [ ] **Step 5: Regenerate golden snapshots and review the diff**

Run: `go test ./internal/scaffold/mono/... -run TestGenerateGoldenTemplateRuleCenter -update-golden -count=1`

Then inspect exactly what changed:

```bash
git diff internal/scaffold/mono/testdata/mono-kitex-template-rulecenter/
```

Expected: the only diff is in
`internal/scaffold/mono/testdata/mono-kitex-template-rulecenter/template/kitex-template/ratelimit_usecase.yaml`,
changing `type: cover` to `type: skip`. If any other file in that testdata
tree changed, stop and investigate before proceeding — that would mean the
fix has a wider blast radius than expected.

- [ ] **Step 6: Run the full golden suite to confirm no other snapshot regressed**

Run: `go test ./internal/scaffold/mono/... -count=1`
Expected: PASS (all golden tests, including `TestGenerateTemplateRuleCenterEquivalentToPreset` and `TestGenerateGoldenWithRuleCenter`, which must still pass unchanged since they don't touch this file).

- [ ] **Step 7: Commit**

```bash
git add internal/assets/_data/kitex/kitex-template/ratelimit_usecase.yaml \
        internal/scaffold/mono/kitex_template_update_behavior_test.go \
        internal/scaffold/mono/testdata/mono-kitex-template-rulecenter/
git commit -m "fix(kitex-template): rule-center usecase.go is skip, not cover, on make update

The ratelimit_usecase.yaml template's body invites hand-written business
logic (\"Edit business logic here\") but was marked update_behavior: cover.
Since 'make update' calls the vendored kitex -template-dir binary directly
(bypassing ncgo), cover meant every regeneration silently overwrote the
user's business logic. Matches the documented skip contract in
docs/kitex/design-doc.zh-CN.md and the existing generic usecase.yaml.

Fixes #117"
```

---

### Task 2: Add a preventive call-site check for hz rate-limit template helpers

**Files:**
- Create: `internal/scaffold/mono/hz_layout_helper_callsite_test.go`

**Interfaces:**
- Consumes: `assets.FS()`, `gopkg.in/yaml.v3` (no shared types with Task 1 — this parses the hz `layouts:` list shape, which is structurally different from kitex's `TemplateFile`).
- Produces: nothing consumed elsewhere.

- [ ] **Step 1: Write the failing preventive test**

Create `internal/scaffold/mono/hz_layout_helper_callsite_test.go`:

```go
package mono

import (
	"fmt"
	"io/fs"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/byx-darwin/ncgo/internal/assets"
)

// TestHzRateLimitMiddlewareCallsJoinRateLimitKey guards against the class of
// drift reported alongside Issue #117 (Bug 2): a helper function defined in
// the hz layout template but never called from its own cache-key
// construction site, which lets an un-sanitized key slip through
// unnoticed. ncgo's own template is confirmed correct today; this test
// pins that so a future edit to layout.yaml can't silently reintroduce the
// drift without failing CI.
func TestHzRateLimitMiddlewareCallsJoinRateLimitKey(t *testing.T) {
	srcFS := assets.FS()
	b, err := fs.ReadFile(srcFS, "hertz/layout.yaml")
	if err != nil {
		t.Fatalf("read hertz/layout.yaml: %v", err)
	}
	var doc struct {
		Layouts []struct {
			Path string `yaml:"path"`
			Body string `yaml:"body"`
		} `yaml:"layouts"`
	}
	if err := yaml.Unmarshal(b, &doc); err != nil {
		t.Fatalf("parse hertz/layout.yaml: %v", err)
	}
	const wantPath = "internal/pkg/middleware/rate_limit.go"
	var body string
	found := false
	for _, l := range doc.Layouts {
		if l.Path == wantPath {
			body = l.Body
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("layout entry %q not found in hertz/layout.yaml", wantPath)
	}
	const helper = "joinRateLimitKey"
	if !strings.Contains(body, fmt.Sprintf("func %s(", helper)) {
		t.Errorf("%s: %s is not defined in this layout entry's body", wantPath, helper)
	}
	if strings.Count(body, helper+"(") < 2 {
		t.Errorf("%s: %s is defined but never called (want >= 2 occurrences: 1 definition + >= 1 call site, got %d) — this is the exact drift pattern flagged in Issue #117 (Bug 2)",
			wantPath, helper, strings.Count(body, helper+"("))
	}
}
```

- [ ] **Step 2: Run test to verify it currently passes (this is a preventive/pinning test, not a bugfix)**

Run: `go test ./internal/scaffold/mono/... -run TestHzRateLimitMiddlewareCallsJoinRateLimitKey -v`
Expected: PASS — ncgo's own template already calls `joinRateLimitKey` correctly (confirmed in the design doc's investigation). This step confirms the test is correctly written against the current, correct state before it starts guarding against regressions.

- [ ] **Step 3: Verify the test actually catches the drift (temporarily break it)**

Temporarily comment out the call site in `internal/assets/_data/hertz/layout.yaml` (the line containing `return joinRateLimitKey(cfg.KeyPrefix, ...)`) and rerun:

Run: `go test ./internal/scaffold/mono/... -run TestHzRateLimitMiddlewareCallsJoinRateLimitKey -v`
Expected: FAIL (confirms the test has teeth). Then revert the temporary edit:

```bash
git checkout -- internal/assets/_data/hertz/layout.yaml
```

Run: `go test ./internal/scaffold/mono/... -run TestHzRateLimitMiddlewareCallsJoinRateLimitKey -v`
Expected: PASS again.

- [ ] **Step 4: Commit**

```bash
git add internal/scaffold/mono/hz_layout_helper_callsite_test.go
git commit -m "test(hz-template): pin joinRateLimitKey def+call-site in rate_limit.go layout

Preventive regression test for the drift pattern flagged in Issue #117
(Bug 2): the hz rate-limit middleware template already correctly defines
and calls joinRateLimitKey, but nothing pinned that. Guards against a
future layout.yaml edit reintroducing a defined-but-uncalled sanitizer."
```

---

### Task 3: Full validation

**Files:** none (validation only)

- [ ] **Step 1: Full build**

Run: `go build ./... && go build .`
Expected: no errors.

- [ ] **Step 2: Vet**

Run: `go vet ./...`
Expected: no findings.

- [ ] **Step 3: Full test suite**

Run: `go test ./... -count=1`
Expected: all tests pass, including both new tests and all golden tests.

- [ ] **Step 4: Format check**

Run: `gofmt -l $(find . -name '*.go' -not -path './.git/*')`
Expected: empty output.

- [ ] **Step 5: Smoke test**

Run: `./scripts/smoke.sh`
Expected: passes (no CLI-surface changes were made, so this should be unaffected, but the repo's own validation order requires it before calling the work done).

No commit for this task — it's pure validation of Tasks 1 and 2's commits.
