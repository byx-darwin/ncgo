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
