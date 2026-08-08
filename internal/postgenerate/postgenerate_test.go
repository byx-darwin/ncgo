package postgenerate

import (
	"bytes"
	"context"
	"fmt"
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

type fakeRunner struct {
	success bool
}

func (f *fakeRunner) Run(_ context.Context, c exec.Cmd) (exec.Result, error) {
	if !f.success {
		return exec.Result{}, fmt.Errorf("command failed")
	}
	return exec.Result{ExitCode: 0}, nil
}
