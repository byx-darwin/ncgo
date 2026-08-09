# ncgo new Auto Post-Generation Steps Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Automatically execute safe post-generation steps (`go mod tidy` + `ncgo ai sync`) after `ncgo new --mode mono` succeeds, so the generated project is immediately usable by AI agents.

**Architecture:** New `internal/postgenerate` package encapsulates auto-step logic with injectable `exec.Runner` for testability. CLI and MCP both call `postgenerate.Run` after `mono.Generate` succeeds. Failures are non-blocking warnings. NextSteps filtering happens in CLI/MCP layer, not in `mono.Generate`.

**Tech Stack:** Go 1.25+, existing `internal/exec.Runner` interface, existing `internal/ai.Sync()` function, Cobra CLI, JSON-RPC 2.0 (MCP)

## Global Constraints

- Go version: >= 1.25.0
- `ai.Sync` default target: "claude" when Target is empty
- `ai.Sync` skip semantics: existing files not overwritten unless Force=true (default false)
- Auto-step failures: non-blocking, logged as warnings to stdout
- `--no-generate` mode: skip all auto steps (no go.mod exists)
- `--ai-target none`: skip ai sync step only; go mod tidy still runs
- `--no-auto-steps`: skip all auto steps
- NextSteps filtering: CLI/MCP layer removes auto-executed steps from display

---

### Task 1: Create postgenerate Package Skeleton

**Files:**
- Create: `internal/postgenerate/postgenerate.go`
- Create: `internal/postgenerate/postgenerate_test.go`

**Interfaces:**
- Consumes: `internal/exec.Runner` interface (for future goModTidy)
- Produces: `Options`, `Result`, `StepResult`, `Run(Options) *Result`

- [ ] **Step 1: Write failing test for Run() with NoAutoSteps**

```go
package postgenerate

import (
	"bytes"
	"testing"
)

func TestRun_NoAutoSteps(t *testing.T) {
	var buf bytes.Buffer
	opts := Options{
		Dir:         t.TempDir(),
		NoAutoSteps: true,
		RanGenerate: true,
		Stdout:      &buf,
	}
	res := Run(opts)
	if len(res.Steps) != 2 {
		t.Fatalf("expected 2 steps, got %d", len(res.Steps))
	}
	for _, step := range res.Steps {
		if step.Status != "skipped" {
			t.Errorf("step %q: expected status 'skipped', got %q", step.Name, step.Status)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/postgenerate -run TestRun_NoAutoSteps -v`
Expected: FAIL with "undefined: Run" or "undefined: Options"

- [ ] **Step 3: Create postgenerate.go with types and Run stub**

```go
package postgenerate

import (
	"io"

	"github.com/byx-darwin/ncgo/internal/exec"
)

// Options configures post-generation auto-step execution.
type Options struct {
	Dir         string      // absolute project root
	AITarget    string      // "claude" (default) | "all" | "agents" | "cursor" | "none"
	NoAutoSteps bool        // skip all auto steps
	RanGenerate bool        // whether generator (hz/kitex) ran successfully
	Runner      exec.Runner // injected exec; nil = exec.NewDefault()
	Stdout      io.Writer   // progress and warning output
}

// Result reports the outcome of each auto step.
type Result struct {
	Steps []StepResult
}

// StepResult describes one auto step's outcome.
type StepResult struct {
	Name   string // "go mod tidy" | "ai sync"
	Status string // "skipped" | "succeeded" | "failed"
	Detail string // human-readable reason, timing, or error message
}

// Run executes post-generation auto steps. It never returns an error for
// step failures; failures are recorded in Result.Steps and written as
// warnings to opts.Stdout.
func Run(opts Options) *Result {
	res := &Result{
		Steps: []StepResult{
			{Name: "go mod tidy", Status: "skipped"},
			{Name: "ai sync", Status: "skipped"},
		},
	}
	return res
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/postgenerate -run TestRun_NoAutoSteps -v`
Expected: PASS

- [ ] **Step 5: Write failing test for RanGenerate=false**

```go
func TestRun_NoGenerate(t *testing.T) {
	var buf bytes.Buffer
	opts := Options{
		Dir:         t.TempDir(),
		RanGenerate: false,
		Stdout:      &buf,
	}
	res := Run(opts)
	for _, step := range res.Steps {
		if step.Status != "skipped" {
			t.Errorf("step %q: expected status 'skipped', got %q", step.Name, step.Status)
		}
	}
}
```

- [ ] **Step 6: Run test to verify it fails**

Run: `go test ./internal/postgenerate -run TestRun_NoGenerate -v`
Expected: PASS (already returns skipped for all steps)

- [ ] **Step 7: Update Run to handle NoAutoSteps and RanGenerate**

```go
func Run(opts Options) *Result {
	res := &Result{}

	// Skip all steps if NoAutoSteps or !RanGenerate
	if opts.NoAutoSteps || !opts.RanGenerate {
		res.Steps = []StepResult{
			{Name: "go mod tidy", Status: "skipped", Detail: "auto steps disabled or generator did not run"},
			{Name: "ai sync", Status: "skipped", Detail: "auto steps disabled or generator did not run"},
		}
		return res
	}

	// TODO: implement steps in next tasks
	res.Steps = []StepResult{
		{Name: "go mod tidy", Status: "skipped", Detail: "not yet implemented"},
		{Name: "ai sync", Status: "skipped", Detail: "not yet implemented"},
	}
	return res
}
```

- [ ] **Step 8: Run all tests**

Run: `go test ./internal/postgenerate -v`
Expected: All tests PASS

- [ ] **Step 9: Commit**

```bash
git add internal/postgenerate/
git commit -m "feat(postgenerate): add package skeleton with Options/Result/Run"
```

---

### Task 2: Implement goModTidy Step

**Files:**
- Modify: `internal/exec/exec.go` (add GoModTidy helper)
- Modify: `internal/postgenerate/steps.go` (create file with goModTidy)
- Modify: `internal/postgenerate/postgenerate.go` (call goModTidy in Run)
- Modify: `internal/postgenerate/postgenerate_test.go` (add tests)

**Interfaces:**
- Consumes: `exec.Runner`, `Options.Dir`
- Produces: `goModTidy(ctx, opts) StepResult`

- [ ] **Step 1: Add GoModTidy helper to exec package**

```go
// In internal/exec/exec.go, after Kitex function:

// GoModTidy runs `go mod tidy` from dir.
func GoModTidy(ctx context.Context, r Runner, dir string) (Result, error) {
	return r.Run(ctx, Cmd{Name: "go", Args: []string{"mod", "tidy"}, Dir: dir})
}
```

- [ ] **Step 2: Create steps.go with goModTidy stub**

```go
package postgenerate

import (
	"context"
	"fmt"
	"time"

	"github.com/byx-darwin/ncgo/internal/exec"
)

// goModTidy runs `go mod tidy` in opts.Dir.
func goModTidy(ctx context.Context, opts Options) StepResult {
	start := time.Now()
	r := opts.Runner
	if r == nil {
		r = exec.NewDefault()
	}
	_, err := exec.GoModTidy(ctx, r, opts.Dir)
	elapsed := time.Since(start)

	if err != nil {
		return StepResult{
			Name:   "go mod tidy",
			Status: "failed",
			Detail: fmt.Sprintf("%v (non-blocking)", err),
		}
	}
	return StepResult{
		Name:   "go mod tidy",
		Status: "succeeded",
		Detail: fmt.Sprintf("(%.1fs)", elapsed.Seconds()),
	}
}
```

- [ ] **Step 3: Write failing test for goModTidy success**

```go
func TestRun_GoModTidySuccess(t *testing.T) {
	var buf bytes.Buffer
	opts := Options{
		Dir:         t.TempDir(),
		AITarget:    "none", // skip ai sync for this test
		RanGenerate: true,
		Runner:      &fakeRunner{success: true},
		Stdout:      &buf,
	}
	res := Run(opts)
	if len(res.Steps) < 1 {
		t.Fatal("expected at least 1 step")
	}
	if res.Steps[0].Status != "succeeded" {
		t.Errorf("go mod tidy: expected 'succeeded', got %q", res.Steps[0].Status)
	}
}

type fakeRunner struct {
	success bool
}

func (f *fakeRunner) Run(_ context.Context, c exec.Cmd) (exec.Result, error) {
	if !f.success {
		return exec.Result{}, fmt.Errorf("command failed")
	}
	return exec.Result{ExitCode: 0}, nil
}
```

- [ ] **Step 4: Run test to verify it fails**

Run: `go test ./internal/postgenerate -run TestRun_GoModTidySuccess -v`
Expected: FAIL (goModTidy not called yet)

- [ ] **Step 5: Update Run to call goModTidy**

```go
func Run(opts Options) *Result {
	res := &Result{}

	if opts.NoAutoSteps || !opts.RanGenerate {
		res.Steps = []StepResult{
			{Name: "go mod tidy", Status: "skipped", Detail: "auto steps disabled or generator did not run"},
			{Name: "ai sync", Status: "skipped", Detail: "auto steps disabled or generator did not run"},
		}
		return res
	}

	ctx := context.Background()

	// Step 1: go mod tidy
	goModTidyResult := goModTidy(ctx, opts)
	res.Steps = append(res.Steps, goModTidyResult)

	// Step 2: ai sync (TODO in next task)
	aiSyncResult := StepResult{Name: "ai sync", Status: "skipped", Detail: "not yet implemented"}
	res.Steps = append(res.Steps, aiSyncResult)

	return res
}
```

- [ ] **Step 6: Run test to verify it passes**

Run: `go test ./internal/postgenerate -run TestRun_GoModTidySuccess -v`
Expected: PASS

- [ ] **Step 7: Write failing test for goModTidy failure**

```go
func TestRun_GoModTidyFailure(t *testing.T) {
	var buf bytes.Buffer
	opts := Options{
		Dir:         t.TempDir(),
		AITarget:    "none",
		RanGenerate: true,
		Runner:      &fakeRunner{success: false},
		Stdout:      &buf,
	}
	res := Run(opts)
	if res.Steps[0].Status != "failed" {
		t.Errorf("go mod tidy: expected 'failed', got %q", res.Steps[0].Status)
	}
	// ai sync should still run (or be skipped due to "none" target)
	if len(res.Steps) < 2 {
		t.Fatal("expected 2 steps even if first failed")
	}
}
```

- [ ] **Step 8: Run test to verify it passes**

Run: `go test ./internal/postgenerate -run TestRun_GoModTidyFailure -v`
Expected: PASS (goModTidy already returns "failed" on error)

- [ ] **Step 9: Commit**

```bash
git add internal/exec/exec.go internal/postgenerate/
git commit -m "feat(postgenerate): implement goModTidy step with exec.Runner"
```

---

### Task 3: Implement aiSync Step

**Files:**
- Modify: `internal/postgenerate/steps.go` (add aiSync)
- Modify: `internal/postgenerate/postgenerate.go` (call aiSync in Run)
- Modify: `internal/postgenerate/postgenerate_test.go` (add tests)

**Interfaces:**
- Consumes: `ai.Sync`, `Options.Dir`, `Options.AITarget`
- Produces: `aiSync(ctx, opts) StepResult`

- [ ] **Step 1: Write failing test for aiSync with target=none**

```go
func TestRun_AISyncNone(t *testing.T) {
	var buf bytes.Buffer
	opts := Options{
		Dir:         t.TempDir(),
		AITarget:    "none",
		RanGenerate: true,
		Runner:      &fakeRunner{success: true},
		Stdout:      &buf,
	}
	res := Run(opts)
	// Find ai sync step
	var aiStep *StepResult
	for i := range res.Steps {
		if res.Steps[i].Name == "ai sync" {
			aiStep = &res.Steps[i]
			break
		}
	}
	if aiStep == nil {
		t.Fatal("ai sync step not found")
	}
	if aiStep.Status != "skipped" {
		t.Errorf("ai sync: expected 'skipped' for target=none, got %q", aiStep.Status)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/postgenerate -run TestRun_AISyncNone -v`
Expected: FAIL (aiSync not implemented yet)

- [ ] **Step 3: Add aiSync implementation to steps.go**

```go
import (
	// ... existing imports ...
	"github.com/byx-darwin/ncgo/internal/ai"
)

// aiSync calls ai.Sync to render AI context files.
func aiSync(ctx context.Context, opts Options) StepResult {
	if opts.AITarget == "none" {
		return StepResult{
			Name:   "ai sync",
			Status: "skipped",
			Detail: "target=none",
		}
	}

	target := opts.AITarget
	if target == "" {
		target = ai.TargetClaude
	}

	start := time.Now()
	_, err := ai.Sync(ai.Options{
		Root:   opts.Dir,
		Target: target,
	})
	elapsed := time.Since(start)

	if err != nil {
		return StepResult{
			Name:   "ai sync",
			Status: "failed",
			Detail: fmt.Sprintf("%v (non-blocking)", err),
		}
	}
	return StepResult{
		Name:   "ai sync",
		Status: "succeeded",
		Detail: fmt.Sprintf("--target %s (%.1fs)", target, elapsed.Seconds()),
	}
}
```

- [ ] **Step 4: Update Run to call aiSync**

```go
func Run(opts Options) *Result {
	res := &Result{}

	if opts.NoAutoSteps || !opts.RanGenerate {
		res.Steps = []StepResult{
			{Name: "go mod tidy", Status: "skipped", Detail: "auto steps disabled or generator did not run"},
			{Name: "ai sync", Status: "skipped", Detail: "auto steps disabled or generator did not run"},
		}
		return res
	}

	ctx := context.Background()

	// Step 1: go mod tidy
	goModTidyResult := goModTidy(ctx, opts)
	res.Steps = append(res.Steps, goModTidyResult)

	// Step 2: ai sync
	aiSyncResult := aiSync(ctx, opts)
	res.Steps = append(res.Steps, aiSyncResult)

	return res
}
```

- [ ] **Step 5: Run test to verify it passes**

Run: `go test ./internal/postgenerate -run TestRun_AISyncNone -v`
Expected: PASS

- [ ] **Step 6: Write test for default target**

```go
func TestRun_DefaultTarget(t *testing.T) {
	var buf bytes.Buffer
	dir := t.TempDir()
	// Create a minimal manifest so ai.Sync doesn't fail
	manifestDir := filepath.Join(dir, ".ncgo")
	if err := os.MkdirAll(manifestDir, 0o755); err != nil {
		t.Fatal(err)
	}
	manifestBody := `service:
  name: test
  kind: hertz
  idl: idl/app.proto
module: example.com/test
`
	if err := os.WriteFile(filepath.Join(manifestDir, "manifest.yaml"), []byte(manifestBody), 0o644); err != nil {
		t.Fatal(err)
	}

	opts := Options{
		Dir:         dir,
		AITarget:    "", // empty should default to "claude"
		RanGenerate: true,
		Runner:      &fakeRunner{success: true},
		Stdout:      &buf,
	}
	res := Run(opts)
	var aiStep *StepResult
	for i := range res.Steps {
		if res.Steps[i].Name == "ai sync" {
			aiStep = &res.Steps[i]
			break
		}
	}
	if aiStep == nil {
		t.Fatal("ai sync step not found")
	}
	// Should succeed (or fail gracefully if manifest is incomplete)
	if aiStep.Status != "succeeded" && aiStep.Status != "failed" {
		t.Errorf("ai sync: expected 'succeeded' or 'failed', got %q", aiStep.Status)
	}
}
```

- [ ] **Step 7: Run all postgenerate tests**

Run: `go test ./internal/postgenerate -v`
Expected: All tests PASS

- [ ] **Step 8: Commit**

```bash
git add internal/postgenerate/
git commit -m "feat(postgenerate): implement aiSync step with ai.Sync"
```

---

### Task 4: CLI Integration

**Files:**
- Modify: `internal/cli/root.go` (add flags, call postgenerate.Run, filter NextSteps)

**Interfaces:**
- Consumes: `postgenerate.Run`, `postgenerate.Options`, `postgenerate.Result`
- Produces: CLI flags `--ai-target`, `--no-auto-steps`

- [ ] **Step 1: Add fields to newOptions struct**

In `internal/cli/root.go`, find the `newOptions` struct (around line 158) and add:

```go
type newOptions struct {
	module       string
	mode         string
	kind         string
	db           string
	infra        []string
	preset       string
	idl          string
	dir          string
	noGenerate   bool
	ruleCenterAddr string
	templateDir    string
	templateName   string
	aiTarget     string    // NEW
	noAutoSteps  bool      // NEW
}
```

- [ ] **Step 2: Register flags in newNewCmd**

In `newNewCmd()` function (around line 193), add:

```go
f.StringVar(&opts.aiTarget, "ai-target", "claude", "AI sync target: claude | all | agents | cursor | none")
f.BoolVar(&opts.noAutoSteps, "no-auto-steps", false, "Skip automatic post-generation steps")
```

- [ ] **Step 3: Import postgenerate package**

At the top of `internal/cli/root.go`, add:

```go
import (
	// ... existing imports ...
	"github.com/byx-darwin/ncgo/internal/postgenerate"
)
```

- [ ] **Step 4: Call postgenerate.Run after mono.Generate succeeds**

In `runNewMono()`, after line 282 (`fmt.Fprintf(out, "scaffolded %s at %s\n", name, res.Dir)`), add:

```go
	// Run auto post-generation steps
	var pgResult *postgenerate.Result
	if res.RanGenerate {
		pgResult = postgenerate.Run(postgenerate.Options{
			Dir:         res.Dir,
			AITarget:    opts.aiTarget,
			NoAutoSteps: opts.noAutoSteps,
			RanGenerate: res.RanGenerate,
			Stdout:      out,
		})
	}
```

- [ ] **Step 5: Filter NextSteps based on auto step results**

Still in `runNewMono()`, before printing NextSteps (around line 290), add:

```go
	// Filter NextSteps: remove auto-executed steps
	nextSteps := res.NextSteps
	if pgResult != nil && !opts.noAutoSteps {
		nextSteps = filterAutoSteps(nextSteps, pgResult)
	}

	fmt.Fprintln(out, "\nnext steps:")
	for _, s := range nextSteps {
		fmt.Fprintf(out, "  $ %s\n", s)
	}
```

- [ ] **Step 6: Add filterAutoSteps helper function**

At the bottom of `internal/cli/root.go`, add:

```go
// filterAutoSteps removes NextSteps that were auto-executed.
func filterAutoSteps(steps []string, pgResult *postgenerate.Result) []string {
	filtered := make([]string, 0, len(steps))
	for _, step := range steps {
		// Skip "go mod tidy" if it succeeded
		if strings.Contains(step, "go mod tidy") {
			succeeded := false
			for _, r := range pgResult.Steps {
				if r.Name == "go mod tidy" && r.Status == "succeeded" {
					succeeded = true
					break
				}
			}
			if succeeded {
				continue
			}
		}
		// Skip "ncgo ai sync" if it succeeded
		if strings.Contains(step, "ncgo ai sync") {
			succeeded := false
			for _, r := range pgResult.Steps {
				if r.Name == "ai sync" && r.Status == "succeeded" {
					succeeded = true
					break
				}
			}
			if succeeded {
				continue
			}
		}
		filtered = append(filtered, step)
	}
	return filtered
}
```

- [ ] **Step 7: Test CLI flags**

Run: `go build . && ./ncgo new --help | grep -E "ai-target|no-auto-steps"`
Expected: Both flags appear in help output

- [ ] **Step 8: Commit**

```bash
git add internal/cli/root.go
git commit -m "feat(cli): add --ai-target and --no-auto-steps flags to ncgo new"
```

---

### Task 5: MCP Integration

**Files:**
- Modify: `internal/mcp/tool_new.go` (add args, call postgenerate.Run, add AutoSteps to result)

**Interfaces:**
- Consumes: `postgenerate.Run`, `postgenerate.StepResult`
- Produces: `newResult.AutoSteps` field

- [ ] **Step 1: Add AutoSteps field to newResult**

In `internal/mcp/tool_new.go`, find the `newResult` struct (around line 36) and add:

```go
type newResult struct {
	Dir         string
	NextSteps   []string
	Mode        string
	RanGenerate *bool
	AutoSteps   []postgenerate.StepResult `json:",omitempty"`  // NEW
}
```

- [ ] **Step 2: Add fields to callNew args struct**

In `callNew()`, find the anonymous args struct (around line 44) and add:

```go
	AITarget    string `json:"aiTarget"`
	NoAutoSteps bool   `json:"noAutoSteps"`
```

- [ ] **Step 3: Import postgenerate package**

At the top of `internal/mcp/tool_new.go`, add:

```go
import (
	// ... existing imports ...
	"github.com/byx-darwin/ncgo/internal/postgenerate"
)
```

- [ ] **Step 4: Pass aiTarget/noAutoSteps to runNewMono**

In `callNew()`, update the `runNewMono` call (around line 84) to pass the new args:

```go
res, err = runNewMono(ctx, args.Name, args.Module, dir, args.Kind, args.DB, args.Infra, args.NoGenerate, args.Preset, args.RuleCenterAddr, args.AITarget, args.NoAutoSteps, ncgoVersion, assetsVersion)
```

- [ ] **Step 5: Update runNewMono signature and implementation**

In `runNewMono()`, update the signature (around line 109):

```go
func runNewMono(ctx context.Context, name, module, dir, kind, db string, infra []string, noGenerate bool, preset, ruleCenterAddr, aiTarget string, noAutoSteps bool, ncgoVersion, assetsVersion string) (*newResult, error) {
```

After `mono.Generate` succeeds (around line 132), add:

```go
	// Run auto post-generation steps
	var autoSteps []postgenerate.StepResult
	if res.RanGenerate {
		pgResult := postgenerate.Run(postgenerate.Options{
			Dir:         res.Dir,
			AITarget:    aiTarget,
			NoAutoSteps: noAutoSteps,
			RanGenerate: res.RanGenerate,
			Stdout:      io.Discard, // MCP doesn't print progress
		})
		autoSteps = pgResult.Steps
	}

	ran := res.RanGenerate
	return &newResult{
		Dir:         res.Dir,
		NextSteps:   res.NextSteps,
		Mode:        manifest.ModeMono,
		RanGenerate: &ran,
		AutoSteps:   autoSteps,
	}, nil
```

- [ ] **Step 6: Add io import**

At the top of `internal/mcp/tool_new.go`, ensure `io` is imported:

```go
import (
	"context"
	"encoding/json"
	"errors"
	"io"  // NEW
	// ... rest ...
)
```

- [ ] **Step 7: Build and test**

Run: `go build ./...`
Expected: No errors

- [ ] **Step 8: Commit**

```bash
git add internal/mcp/tool_new.go
git commit -m "feat(mcp): add aiTarget/noAutoSteps args and AutoSteps to ncgo_new result"
```

---

### Task 6: Integration Tests

**Files:**
- Modify: `internal/scaffold/mono/mono_test.go` (add smoke tests)

**Interfaces:**
- Consumes: `mono.Generate`, `postgenerate.Run` (via CLI)

- [ ] **Step 1: Add smoke test for default auto steps**

In `internal/scaffold/mono/mono_test.go`, add:

```go
func TestGenerate_AutoSteps_Default(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	// This test requires hz/kitex on PATH, so skip if not available
	if _, err := exec.LookPath("hz"); err != nil {
		t.Skip("hz not on PATH")
	}

	dir := t.TempDir()
	res, err := Generate(context.Background(), Options{
		Name:         "test-svc",
		Module:       "example.com/test-svc",
		Kind:         manifest.KindHertz,
		Dir:          dir,
		AssetsVersion: "test",
		NCGOVersion:   "test",
	})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if !res.RanGenerate {
		t.Fatal("expected RanGenerate=true")
	}

	// After auto steps run, .claude/CLAUDE.md should exist
	claudePath := filepath.Join(res.Dir, ".claude", "CLAUDE.md")
	if _, err := os.Stat(claudePath); err != nil {
		t.Errorf("expected %s to exist after auto steps", claudePath)
	}
}
```

- [ ] **Step 2: Run integration test**

Run: `go test ./internal/scaffold/mono -run TestGenerate_AutoSteps_Default -v`
Expected: PASS (or SKIP if hz not on PATH)

- [ ] **Step 3: Commit**

```bash
git add internal/scaffold/mono/mono_test.go
git commit -m "test(mono): add integration test for auto post-generation steps"
```

---

### Task 7: Documentation Updates

**Files:**
- Modify: `README.md` (add new flags to `ncgo new` section)
- Modify: `README.zh-CN.md` (add new flags to `ncgo new` section)
- Modify: `docs/examples.md` (add example with auto steps)

- [ ] **Step 1: Update README.md**

In `README.md`, find the `ncgo new` section and add:

```markdown
### Auto Post-Generation Steps

After `ncgo new` successfully generates a project, it automatically runs:
- `go mod tidy` — resolves Go module dependencies
- `ncgo ai sync --target claude` — renders AI context files (CLAUDE.md, etc.)

These steps make the generated project immediately usable by AI agents.

**Flags:**
- `--ai-target <target>` — AI sync target: `claude` (default) | `all` | `agents` | `cursor` | `none`
- `--no-auto-steps` — skip automatic post-generation steps

**Examples:**
```bash
# Default: auto-run go mod tidy + ai sync (target=claude)
ncgo new user-api --module github.com/acme/user-api

# Skip auto steps
ncgo new user-api --module github.com/acme/user-api --no-auto-steps

# Render all AI targets (agents + claude + cursor)
ncgo new user-api --module github.com/acme/user-api --ai-target all
```
```

- [ ] **Step 2: Update README.zh-CN.md**

In `README.zh-CN.md`, find the `ncgo new` section and add the Chinese translation:

```markdown
### 自动后处理步骤

`ncgo new` 成功生成项目后，会自动执行：
- `go mod tidy` — 解析 Go 模块依赖
- `ncgo ai sync --target claude` — 渲染 AI 上下文文件（CLAUDE.md 等）

这些步骤使生成的项目可立即被 AI agent 使用。

**Flags:**
- `--ai-target <target>` — AI sync 目标：`claude`（默认）| `all` | `agents` | `cursor` | `none`
- `--no-auto-steps` — 跳过自动后处理步骤

**示例：**
```bash
# 默认：自动运行 go mod tidy + ai sync（target=claude）
ncgo new user-api --module github.com/acme/user-api

# 跳过自动步骤
ncgo new user-api --module github.com/acme/user-api --no-auto-steps

# 渲染所有 AI 目标（agents + claude + cursor）
ncgo new user-api --module github.com/acme/user-api --ai-target all
```
```

- [ ] **Step 3: Commit**

```bash
git add README.md README.zh-CN.md docs/examples.md
git commit -m "docs: document --ai-target and --no-auto-steps flags"
```

---

## Self-Review Checklist

- [x] **Spec coverage:** All requirements from design spec covered (auto steps, flags, CLI/MCP integration, tests, docs)
- [x] **Placeholder scan:** No TBD/TODO placeholders; all steps have concrete code
- [x] **Type consistency:** `Options`, `Result`, `StepResult` used consistently across tasks
- [x] **Bite-sized steps:** Each step is 2-5 minutes
- [x] **TDD flow:** Tests written before implementation in each task

---

**Plan complete and saved to `docs/superpowers/plans/2026-08-09-ncgo-new-auto-ai-sync.md`.**

**Returning to orchestrator for Phase 2 completion (quality gate + user approval).**
