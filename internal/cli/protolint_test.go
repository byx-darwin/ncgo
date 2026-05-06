package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestRunProtolintJSON(t *testing.T) {
	root := protolintFixtureRoot(t, "..", "protolint", "testdata", "kitexenvelope")
	var out bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&out)

	err := runProtolint(cmd, &protolintOptions{
		root:   root,
		files:  []string{"invalid.proto"},
		rules:  []string{"PIO301"},
		output: "json",
	})
	if !errors.Is(err, errSilentFailure) {
		t.Fatalf("err = %v, want errSilentFailure", err)
	}
	var got struct {
		Root     string   `json:"root"`
		OK       bool     `json:"ok"`
		RulesRun []string `json:"rulesRun"`
		Summary  struct {
			FilesScanned     int `json:"filesScanned"`
			RPCsScanned      int `json:"rpcsScanned"`
			DiagnosticsCount int `json:"diagnosticsCount"`
		} `json:"summary"`
		Diagnostics []struct {
			RuleID  string `json:"ruleId"`
			Message string `json:"message"`
		} `json:"diagnostics"`
	}
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("invalid json: %v\n%s", err, out.String())
	}
	if got.Root != root {
		t.Fatalf("root = %q, want %q", got.Root, root)
	}
	if got.OK {
		t.Fatalf("expected ok=false: %+v", got)
	}
	if len(got.RulesRun) != 1 || got.RulesRun[0] != "PIO301" {
		t.Fatalf("rulesRun = %v", got.RulesRun)
	}
	if got.Summary.FilesScanned != 1 || got.Summary.RPCsScanned != 2 || got.Summary.DiagnosticsCount != 1 {
		t.Fatalf("summary = %+v", got.Summary)
	}
	if len(got.Diagnostics) != 1 || got.Diagnostics[0].RuleID != "PIO301" || got.Diagnostics[0].Message != "GetUserResp" {
		t.Fatalf("diagnostics = %+v", got.Diagnostics)
	}
}

func TestRunProtolintTextSuccess(t *testing.T) {
	root := protolintFixtureRoot(t, "..", "scaffold", "mono", "testdata", "mono-default", "idl")
	var out bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&out)

	err := runProtolint(cmd, &protolintOptions{
		root:   root,
		files:  []string{"app/demo.proto"},
		rules:  []string{"PIO101", "PIO102"},
		output: "text",
	})
	if err != nil {
		t.Fatalf("runProtolint text: %v", err)
	}
	if !strings.Contains(out.String(), "protolint: ok") {
		t.Fatalf("unexpected text output:\n%s", out.String())
	}
	if strings.Contains(out.String(), "✗ [") {
		t.Fatalf("unexpected diagnostics in text output:\n%s", out.String())
	}
}

func TestRunProtolintWarningsAreNonBlocking(t *testing.T) {
	root := protolintFixtureRoot(t, "..", "protolint", "testdata", "phase2warnings")
	var out bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&out)

	err := runProtolint(cmd, &protolintOptions{
		root:   root,
		files:  []string{"invalid.proto"},
		rules:  []string{"PIO111", "PIO112", "PIO113"},
		output: "text",
	})
	if err != nil {
		t.Fatalf("runProtolint warnings: %v", err)
	}
	if !strings.Contains(out.String(), "protolint: ok") {
		t.Fatalf("unexpected text output:\n%s", out.String())
	}
	if !strings.Contains(out.String(), "! [PIO111]") || !strings.Contains(out.String(), "! [PIO112]") || !strings.Contains(out.String(), "! [PIO113]") {
		t.Fatalf("warning output missing expected markers:\n%s", out.String())
	}
}

func TestRunProtolintRejectsInvalidOutput(t *testing.T) {
	root := protolintFixtureRoot(t, "..", "protolint", "testdata", "kitexenvelope")
	cmd := &cobra.Command{}
	err := runProtolint(cmd, &protolintOptions{root: root, files: []string{"invalid.proto"}, output: "yaml"})
	if err == nil || !strings.Contains(err.Error(), "unsupported --output") {
		t.Fatalf("err = %v, want unsupported --output", err)
	}
}

func TestRunProtolintRequiresFiles(t *testing.T) {
	cmd := &cobra.Command{}
	err := runProtolint(cmd, &protolintOptions{root: ".", output: "text"})
	if err == nil || !strings.Contains(err.Error(), "at least one --file is required") {
		t.Fatalf("err = %v, want missing --file error", err)
	}
}

func protolintFixtureRoot(t *testing.T, elems ...string) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join(elems...))
	if err != nil {
		t.Fatalf("fixture abs path: %v", err)
	}
	return root
}
