package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestCompletionBashGeneratesOutput(t *testing.T) {
	cmd := newCompletionCmd()
	// The completion command needs a root to generate completion scripts.
	root := newRootCmd()
	root.AddCommand(cmd)

	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetArgs([]string{"completion", "bash"})
	if err := root.Execute(); err != nil {
		t.Fatalf("completion bash: %v", err)
	}
	if buf.Len() == 0 {
		t.Fatal("expected non-empty output for bash completion")
	}
}

func TestCompletionZshGeneratesOutput(t *testing.T) {
	cmd := newCompletionCmd()
	root := newRootCmd()
	root.AddCommand(cmd)

	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetArgs([]string{"completion", "zsh"})
	if err := root.Execute(); err != nil {
		t.Fatalf("completion zsh: %v", err)
	}
	if buf.Len() == 0 {
		t.Fatal("expected non-empty output for zsh completion")
	}
}

func TestCompletionFishGeneratesOutput(t *testing.T) {
	cmd := newCompletionCmd()
	root := newRootCmd()
	root.AddCommand(cmd)

	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetArgs([]string{"completion", "fish"})
	if err := root.Execute(); err != nil {
		t.Fatalf("completion fish: %v", err)
	}
	if buf.Len() == 0 {
		t.Fatal("expected non-empty output for fish completion")
	}
}

func TestCompletionInvalidShellReturnsError(t *testing.T) {
	cmd := newCompletionCmd()
	root := newRootCmd()
	root.AddCommand(cmd)

	root.SetArgs([]string{"completion", "invalid"})
	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for invalid shell, got nil")
	}
}

func TestCompletionMissingShellReturnsError(t *testing.T) {
	cmd := newCompletionCmd()
	root := newRootCmd()
	root.AddCommand(cmd)

	root.SetArgs([]string{"completion"})
	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for missing shell argument, got nil")
	}
}

func TestCompletionBashContainsKeywords(t *testing.T) {
	cmd := newCompletionCmd()
	root := newRootCmd()
	root.AddCommand(cmd)

	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetArgs([]string{"completion", "bash"})
	if err := root.Execute(); err != nil {
		t.Fatalf("completion bash: %v", err)
	}
	output := buf.String()
	for _, want := range []string{"ncgo", "_ncgo", "bash"} {
		if !strings.Contains(output, want) {
			t.Errorf("bash completion missing %q", want)
		}
	}
}

func TestCompletionZshContainsKeywords(t *testing.T) {
	cmd := newCompletionCmd()
	root := newRootCmd()
	root.AddCommand(cmd)

	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetArgs([]string{"completion", "zsh"})
	if err := root.Execute(); err != nil {
		t.Fatalf("completion zsh: %v", err)
	}
	output := buf.String()
	for _, want := range []string{"compdef", "_ncgo", "ncgo"} {
		if !strings.Contains(output, want) {
			t.Errorf("zsh completion missing %q", want)
		}
	}
}

func TestCompletionFishContainsKeywords(t *testing.T) {
	cmd := newCompletionCmd()
	root := newRootCmd()
	root.AddCommand(cmd)

	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetArgs([]string{"completion", "fish"})
	if err := root.Execute(); err != nil {
		t.Fatalf("completion fish: %v", err)
	}
	output := buf.String()
	if !strings.Contains(output, "ncgo") || !strings.Contains(output, "complete") {
		t.Errorf("fish completion missing expected keywords")
	}
}
