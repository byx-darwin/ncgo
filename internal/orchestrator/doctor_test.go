package orchestrator

import (
	"context"
	"testing"
	"time"

	"github.com/byx-darwin/ncgo/internal/exec"
	"github.com/byx-darwin/ncgo/internal/manifest"
)

// scriptedRunner returns canned responses keyed on Cmd.Name. Missing keys
// produce a NotFoundError so tests can mix presence and absence per binary.
type scriptedRunner struct {
	out map[string]string
	err map[string]error
}

func (s *scriptedRunner) Run(_ context.Context, c exec.Cmd) (exec.Result, error) {
	if e, ok := s.err[c.Name]; ok {
		return exec.Result{}, e
	}
	if v, ok := s.out[c.Name]; ok {
		return exec.Result{Stdout: []byte(v)}, nil
	}
	return exec.Result{}, &exec.NotFoundError{Name: c.Name}
}

func TestRunDoctor(t *testing.T) {
	// Seed a minimal manifest so doctor can find a project.
	root := t.TempDir()
	m := &manifest.Manifest{
		Ncgo:        manifest.Meta{Version: "0.0.0-test", AssetsVersion: "test"},
		Mode:        manifest.ModeMono,
		Module:      "github.com/x/demo",
		Service:     manifest.Service{Name: "demo", Kind: manifest.KindHertz, IDL: "idl/app/demo.proto"},
		GeneratedAt: time.Date(2026, 4, 29, 0, 0, 0, 0, time.UTC),
	}
	if err := manifest.Save(root, m); err != nil {
		t.Fatalf("seed manifest: %v", err)
	}

	// Use a runner that reports both hz and kitex as present.
	runner := &scriptedRunner{
		out: map[string]string{"hz": "hz version v0.9.7", "kitex": "v0.16.1"},
	}

	result, err := RunDoctor(context.Background(), DoctorOptions{
		Root:   root,
		Runner: runner,
	})
	if err != nil {
		t.Fatalf("RunDoctor: %v", err)
	}
	if result.Summary.CheckCount == 0 {
		t.Fatalf("expected checks, got none")
	}
	if result.Root != root {
		t.Fatalf("root = %q, want %q", result.Root, root)
	}
	if result.Report == nil {
		t.Fatalf("Report should not be nil")
	}
}

func TestRunDoctorHostOnly(t *testing.T) {
	// When Root is empty, doctor only checks tools (host-level checks).
	runner := &scriptedRunner{
		out: map[string]string{"hz": "hz version v0.9.7", "kitex": "v0.16.1"},
	}

	result, err := RunDoctor(context.Background(), DoctorOptions{
		Root:   "",
		Runner: runner,
	})
	if err != nil {
		t.Fatalf("RunDoctor: %v", err)
	}
	if result.Summary.CheckCount != 2 {
		t.Fatalf("expected 2 host checks, got %d", result.Summary.CheckCount)
	}
	if result.Scope != "host" {
		t.Fatalf("scope = %q, want host", result.Scope)
	}
}
