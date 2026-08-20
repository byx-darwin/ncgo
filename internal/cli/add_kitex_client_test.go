package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// newCobraTestCmd creates a cobra command with a background context for testing.
func newCobraTestCmd() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())
	return cmd
}

// seedKitexClientWorkspace creates a temp directory with go.mod and a proto
// file suitable for kitex-client CLI tests.
func seedKitexClientWorkspace(t *testing.T) string {
	t.Helper()
	root := t.TempDir()

	// go.mod so detectModule succeeds.
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/demo\n\ngo 1.25\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// A proto file at idl/rbac.proto that protolint can parse.
	idlDir := filepath.Join(root, "idl")
	if err := os.MkdirAll(idlDir, 0o755); err != nil {
		t.Fatal(err)
	}
	proto := `syntax = "proto3";
package rbac;
option go_package = "example.com/demo/kitex_gen/rbac;rbac";

service RbacService {
  rpc CheckPermission(CheckPermissionReq) returns (CheckPermissionResp) {}
}
message CheckPermissionReq { string user_id = 1; }
message CheckPermissionResp { bool allowed = 1; }
`
	if err := os.WriteFile(filepath.Join(idlDir, "rbac.proto"), []byte(proto), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestRunAddKitexClientDryRun(t *testing.T) {
	root := seedKitexClientWorkspace(t)
	var out bytes.Buffer
	cmd := newCobraTestCmd()
	cmd.SetOut(&out)

	err := runAddKitexClient(cmd, "rbac", &addKitexClientOptions{
		root:    root,
		service: "RbacService",
		idl:     "idl/rbac.proto",
		dryRun:  true,
	})
	if err != nil {
		t.Fatalf("runAddKitexClient dry-run: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "would write") {
		t.Fatalf("dry-run output missing 'would write':\n%s", output)
	}
	if !strings.Contains(output, "(dry-run: no files were written)") {
		t.Fatalf("dry-run output missing dry-run notice:\n%s", output)
	}

	// Verify no files were actually written
	clientDir := filepath.Join(root, "pkg", "client", "rbac")
	if _, err := os.Stat(clientDir); !os.IsNotExist(err) {
		t.Fatalf("dry-run created client directory: %s", clientDir)
	}
}

func TestRunAddKitexClientPlanShorthand(t *testing.T) {
	root := seedKitexClientWorkspace(t)
	var out bytes.Buffer
	cmd := newCobraTestCmd()
	cmd.SetOut(&out)

	err := runAddKitexClient(cmd, "rbac", &addKitexClientOptions{
		root:    root,
		service: "RbacService",
		idl:     "idl/rbac.proto",
		plan:    true,
	})
	if err != nil {
		t.Fatalf("runAddKitexClient --plan: %v", err)
	}

	var got struct {
		DryRun       bool     `json:"dryRun"`
		WrittenPaths []string `json:"writtenPaths"`
	}
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("--plan output is not valid json: %v\n%s", err, out.String())
	}
	if !got.DryRun {
		t.Fatalf("dryRun = false, want true")
	}
	// Expect kitex_gen/ + client.go + config.go = 3 paths
	if len(got.WrittenPaths) != 3 {
		t.Fatalf("writtenPaths = %v, want 3 paths", got.WrittenPaths)
	}

	// Verify no files were written
	clientDir := filepath.Join(root, "pkg", "client", "rbac")
	if _, err := os.Stat(clientDir); !os.IsNotExist(err) {
		t.Fatalf("--plan created directory: %s", clientDir)
	}
}

func TestRunAddKitexClientRejectsInvalidOutput(t *testing.T) {
	root := seedKitexClientWorkspace(t)
	cmd := newCobraTestCmd()
	err := runAddKitexClient(cmd, "rbac", &addKitexClientOptions{
		root:    root,
		service: "RbacService",
		idl:     "idl/rbac.proto",
		output:  "yaml",
	})
	if err == nil || !strings.Contains(err.Error(), "unsupported --output") {
		t.Fatalf("expected unsupported output error, got: %v", err)
	}
}

func TestRunAddKitexClientMissingModuleWithoutGoMod(t *testing.T) {
	root := t.TempDir() // no go.mod
	cmd := newCobraTestCmd()
	err := runAddKitexClient(cmd, "rbac", &addKitexClientOptions{
		root:    root,
		service: "RbacService",
		idl:     "idl/rbac.proto",
	})
	if err == nil || !strings.Contains(err.Error(), "--module is required") {
		t.Fatalf("expected module error, got: %v", err)
	}
}

func TestRunAddKitexClientModuleFlagOverridesGoMod(t *testing.T) {
	root := seedKitexClientWorkspace(t)
	var out bytes.Buffer
	cmd := newCobraTestCmd()
	cmd.SetOut(&out)

	// Pass --module explicitly; dry-run so kitex is not invoked.
	err := runAddKitexClient(cmd, "rbac", &addKitexClientOptions{
		root:    root,
		module:  "example.com/override",
		service: "RbacService",
		idl:     "idl/rbac.proto",
		dryRun:  true,
	})
	if err != nil {
		t.Fatalf("runAddKitexClient with --module: %v", err)
	}
	// Smoke test: dry-run with explicit --module succeeds without reading go.mod.
}
