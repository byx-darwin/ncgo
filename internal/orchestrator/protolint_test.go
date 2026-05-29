package orchestrator

import (
	"context"
	"path/filepath"
	"testing"
)

func TestRunProtolint(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", "protolint", "testdata", "kitexenvelope"))
	if err != nil {
		t.Fatalf("abs path: %v", err)
	}

	result, err := RunProtolint(context.Background(), ProtolintOptions{
		Root:  root,
		Files: []string{"invalid.proto"},
		Rules: []string{"PIO301"},
	})
	if err != nil {
		t.Fatalf("RunProtolint: %v", err)
	}
	if result.OK {
		t.Fatalf("ok = true, want false")
	}
	if result.Root != root {
		t.Fatalf("root = %q, want %q", result.Root, root)
	}
	if len(result.RulesRun) != 1 || result.RulesRun[0] != "PIO301" {
		t.Fatalf("rulesRun = %v", result.RulesRun)
	}
	if result.Summary.FilesScanned != 1 || result.Summary.RPCsScanned != 2 || result.Summary.DiagnosticsCount != 1 {
		t.Fatalf("summary = %+v", result.Summary)
	}
	if len(result.Diagnostics) != 1 {
		t.Fatalf("diagnostics len = %d, want 1", len(result.Diagnostics))
	}
	if result.Result == nil {
		t.Fatalf("Result should not be nil")
	}
}

func TestRunProtolintSuccess(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", "scaffold", "mono", "testdata", "mono-default", "idl"))
	if err != nil {
		t.Fatalf("abs path: %v", err)
	}

	result, err := RunProtolint(context.Background(), ProtolintOptions{
		Root:  root,
		Files: []string{"app/demo.proto"},
		Rules: []string{"PIO101", "PIO102"},
	})
	if err != nil {
		t.Fatalf("RunProtolint: %v", err)
	}
	if !result.OK {
		t.Fatalf("ok = false, want true")
	}
	if len(result.Diagnostics) != 0 {
		t.Fatalf("diagnostics len = %d, want 0", len(result.Diagnostics))
	}
	if result.Result == nil {
		t.Fatalf("Result should not be nil")
	}
}

func TestRunProtolintWarningsAreNonBlocking(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", "protolint", "testdata", "phase2warnings"))
	if err != nil {
		t.Fatalf("abs path: %v", err)
	}

	result, err := RunProtolint(context.Background(), ProtolintOptions{
		Root:  root,
		Files: []string{"invalid.proto"},
		Rules: []string{"PIO111", "PIO112", "PIO113"},
	})
	if err != nil {
		t.Fatalf("RunProtolint: %v", err)
	}
	if !result.OK {
		t.Fatalf("ok = false, want true (warnings are non-blocking)")
	}
	if result.Summary.WarningCount == 0 && result.Summary.ErrorCount > 0 {
		t.Fatalf("summary = %+v, unexpected errors", result.Summary)
	}
	if result.Result == nil {
		t.Fatalf("Result should not be nil")
	}
}

func TestRunProtolintIgnoreRule(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", "protolint", "testdata", "kitexenvelope"))
	if err != nil {
		t.Fatalf("abs path: %v", err)
	}

	result, err := RunProtolint(context.Background(), ProtolintOptions{
		Root:        root,
		Files:       []string{"invalid.proto"},
		Rules:       []string{"PIO301"},
		IgnoreRules: []string{"PIO301"},
	})
	if err != nil {
		t.Fatalf("RunProtolint: %v", err)
	}
	if !result.OK {
		t.Fatalf("ok = false, want true (rule was ignored)")
	}
	if len(result.Diagnostics) != 0 {
		t.Fatalf("diagnostics len = %d, want 0", len(result.Diagnostics))
	}
	if result.Summary.SuppressedCount != 1 {
		t.Fatalf("suppressedCount = %d, want 1", result.Summary.SuppressedCount)
	}
	if result.Result == nil {
		t.Fatalf("Result should not be nil")
	}
}
