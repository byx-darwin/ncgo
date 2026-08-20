package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestRunAddKitexClientDryRun(t *testing.T) {
	root := t.TempDir()
	var out bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&out)

	err := runAddKitexClient(cmd, "rbac", &addKitexClientOptions{
		root:    root,
		service: "rbac-rpc",
		idl:     "../../rbac/idl/rbac.proto",
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

func TestRunAddKitexClientTextOutput(t *testing.T) {
	root := t.TempDir()
	var out bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&out)

	err := runAddKitexClient(cmd, "rbac", &addKitexClientOptions{
		root:    root,
		service: "rbac-rpc",
		idl:     "../../rbac/idl/rbac.proto",
	})
	if err != nil {
		t.Fatalf("runAddKitexClient text: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "wrote ") {
		t.Fatalf("text output missing 'wrote':\n%s", output)
	}

	// Verify files were created
	clientPath := filepath.Join(root, "pkg", "client", "rbac", "client.go")
	configPath := filepath.Join(root, "pkg", "client", "rbac", "config.go")
	if _, err := os.Stat(clientPath); err != nil {
		t.Fatalf("client.go not created: %v", err)
	}
	if _, err := os.Stat(configPath); err != nil {
		t.Fatalf("config.go not created: %v", err)
	}

	// Verify client.go content
	clientContent, err := os.ReadFile(clientPath)
	if err != nil {
		t.Fatalf("read client.go: %v", err)
	}
	if !strings.Contains(string(clientContent), "package rbac") {
		t.Fatalf("client.go missing 'package rbac':\n%s", string(clientContent))
	}
	if !strings.Contains(string(clientContent), "type Client struct") {
		t.Fatalf("client.go missing 'type Client struct':\n%s", string(clientContent))
	}

	// Verify config.go content
	configContent, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read config.go: %v", err)
	}
	if !strings.Contains(string(configContent), "package rbac") {
		t.Fatalf("config.go missing 'package rbac':\n%s", string(configContent))
	}
	if !strings.Contains(string(configContent), "type Config struct") {
		t.Fatalf("config.go missing 'type Config struct':\n%s", string(configContent))
	}
}

func TestRunAddKitexClientJSONOutput(t *testing.T) {
	root := t.TempDir()
	var out bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&out)

	err := runAddKitexClient(cmd, "rulecenter", &addKitexClientOptions{
		root:    root,
		service: "rule-rpc",
		idl:     "../../rule/idl/rule_center.proto",
		output:  "json",
	})
	if err != nil {
		t.Fatalf("runAddKitexClient json: %v", err)
	}

	var got struct {
		DryRun       bool     `json:"dryRun"`
		WrittenPaths []string `json:"writtenPaths"`
		NextSteps    []string `json:"nextSteps"`
	}
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("json unmarshal: %v\n%s", err, out.String())
	}
	if got.DryRun {
		t.Fatalf("dryRun = true, want false")
	}
	if len(got.WrittenPaths) != 2 {
		t.Fatalf("writtenPaths = %v, want 2 paths", got.WrittenPaths)
	}
	if len(got.NextSteps) == 0 {
		t.Fatalf("nextSteps is empty")
	}

	// Verify paths contain expected files
	wantClient := filepath.Join(root, "pkg", "client", "rulecenter", "client.go")
	wantConfig := filepath.Join(root, "pkg", "client", "rulecenter", "config.go")
	if !containsPath(got.WrittenPaths, wantClient) {
		t.Fatalf("writtenPaths missing %s: %v", wantClient, got.WrittenPaths)
	}
	if !containsPath(got.WrittenPaths, wantConfig) {
		t.Fatalf("writtenPaths missing %s: %v", wantConfig, got.WrittenPaths)
	}
}

func TestRunAddKitexClientPlanShorthand(t *testing.T) {
	root := t.TempDir()
	var out bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&out)

	err := runAddKitexClient(cmd, "rbac", &addKitexClientOptions{
		root:    root,
		service: "rbac-rpc",
		idl:     "../../rbac/idl/rbac.proto",
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
	if len(got.WrittenPaths) != 2 {
		t.Fatalf("writtenPaths = %v, want 2 paths", got.WrittenPaths)
	}

	// Verify no files were written
	clientDir := filepath.Join(root, "pkg", "client", "rbac")
	if _, err := os.Stat(clientDir); !os.IsNotExist(err) {
		t.Fatalf("--plan created directory: %s", clientDir)
	}
}

func TestRunAddKitexClientForceOverwrite(t *testing.T) {
	root := t.TempDir()

	// Create existing files
	clientDir := filepath.Join(root, "pkg", "client", "rbac")
	if err := os.MkdirAll(clientDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	clientPath := filepath.Join(clientDir, "client.go")
	if err := os.WriteFile(clientPath, []byte("existing"), 0o644); err != nil {
		t.Fatalf("write existing: %v", err)
	}

	// Without --force should fail
	var out bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&out)
	err := runAddKitexClient(cmd, "rbac", &addKitexClientOptions{
		root:    root,
		service: "rbac-rpc",
		idl:     "../../rbac/idl/rbac.proto",
	})
	if err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("expected 'already exists' error, got: %v", err)
	}

	// With --force should succeed
	out.Reset()
	cmd2 := &cobra.Command{}
	cmd2.SetOut(&out)
	err = runAddKitexClient(cmd2, "rbac", &addKitexClientOptions{
		root:    root,
		service: "rbac-rpc",
		idl:     "../../rbac/idl/rbac.proto",
		force:   true,
	})
	if err != nil {
		t.Fatalf("runAddKitexClient --force: %v", err)
	}

	// Verify file was overwritten
	content, err := os.ReadFile(clientPath)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(content) == "existing" {
		t.Fatalf("file was not overwritten")
	}
}

func TestRunAddKitexClientRejectsInvalidOutput(t *testing.T) {
	root := t.TempDir()
	cmd := &cobra.Command{}
	err := runAddKitexClient(cmd, "rbac", &addKitexClientOptions{
		root:    root,
		service: "rbac-rpc",
		idl:     "../../rbac/idl/rbac.proto",
		output:  "yaml",
	})
	if err == nil || !strings.Contains(err.Error(), "unsupported --output") {
		t.Fatalf("expected unsupported output error, got: %v", err)
	}
}
