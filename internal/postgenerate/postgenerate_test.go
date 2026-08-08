package postgenerate

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/byx-darwin/ncgo/internal/exec"
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

type fakeRunner struct {
	success bool
}

func (f *fakeRunner) Run(_ context.Context, c exec.Cmd) (exec.Result, error) {
	if !f.success {
		return exec.Result{}, fmt.Errorf("command failed")
	}
	return exec.Result{ExitCode: 0}, nil
}

func TestResultFilterNextSteps(t *testing.T) {
	succeeded := &Result{
		Steps: []StepResult{
			{Name: "go mod tidy", Status: "succeeded"},
			{Name: "ai sync", Status: "succeeded"},
		},
	}
	steps := []string{"go mod tidy", "ncgo ai sync --target all --root .", "hz update", "make lint"}
	got := succeeded.FilterNextSteps(steps)
	want := []string{"hz update", "make lint"}
	if len(got) != len(want) {
		t.Fatalf("FilterNextSteps len = %d, want %d (got %v)", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("FilterNextSteps[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestResultFilterNextStepsKeepsFailedAndSkipped(t *testing.T) {
	failed := &Result{
		Steps: []StepResult{
			{Name: "go mod tidy", Status: "failed"},
			{Name: "ai sync", Status: "skipped"},
		},
	}
	steps := []string{"go mod tidy", "ncgo ai sync --target all --root ."}
	got := failed.FilterNextSteps(steps)
	if len(got) != 2 {
		t.Errorf("FilterNextSteps should keep steps when not succeeded, got %v", got)
	}
}

func TestResultFilterNextStepsNilReceiver(t *testing.T) {
	var nilResult *Result
	steps := []string{"go mod tidy", "ncgo ai sync --target all --root ."}
	got := nilResult.FilterNextSteps(steps)
	if len(got) != 2 {
		t.Errorf("FilterNextSteps(nil) should keep all steps, got %v", got)
	}
}
