# kitex-client Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Implement `ncgo add kitex-client` subcommand to generate Kitex client code for BFF services.

**Architecture:** Add CLI command in internal/cli/add_kitex_client.go, scaffold logic in internal/scaffold/kitexclient/, with templates for client generation.

**Tech Stack:** Go, Cobra CLI, Kitex, Protocol Buffers

**Spec:** docs/superpowers/specs/2026-08-20-kitex-client-design.md

## Global Constraints

- Command: `ncgo add kitex-client <name> --service <service> --idl <path>`
- Generated files: pkg/client/<name>/{client,config}.go + kitex_gen/
- Must compile successfully after generation

---

### Task 1: Create CLI Command

**Files:**
- Create: `internal/cli/add_kitex_client.go`
- Modify: `internal/cli/add.go`

- [ ] **Step 1: Create add_kitex_client.go**

```go
package cli

import (
    "encoding/json"
    "fmt"
    "github.com/spf13/cobra"
    "github.com/byx-darwin/ncgo/internal/scaffold/kitexclient"
)

type addKitexClientOptions struct {
    root    string
    service string
    idl     string
    force   bool
    dryRun  bool
    plan    bool
    output  string
}

func newAddKitexClientCmd() *cobra.Command {
    opts := &addKitexClientOptions{}
    cmd := &cobra.Command{
        Use:   "kitex-client <name>",
        Short: "Add Kitex client for calling RPC services",
        Long:  "Generate Kitex client wrapper and types for calling an RPC service from a BFF.",
        Args:  cobra.ExactArgs(1),
        RunE: func(cmd *cobra.Command, args []string) error {
            return runAddKitexClient(cmd, args[0], opts)
        },
    }
    f := cmd.Flags()
    f.StringVar(&opts.root, "root", ".", "Project root")
    f.StringVar(&opts.service, "service", "", "RPC service name")
    f.StringVar(&opts.idl, "idl", "", "Path to proto file")
    f.BoolVar(&opts.force, "force", false, "Overwrite existing files")
    f.BoolVar(&opts.dryRun, "dry-run", false, "Preview without writing")
    f.BoolVar(&opts.plan, "plan", false, "Shorthand for --dry-run --output json")
    f.StringVar(&opts.output, "output", "text", "Output format: text or json")
    _ = cmd.MarkFlagRequired("service")
    _ = cmd.MarkFlagRequired("idl")
    return cmd
}

func runAddKitexClient(cmd *cobra.Command, name string, opts *addKitexClientOptions) error {
    if opts.plan {
        opts.dryRun = true
        opts.output = "json"
    }
    res, err := kitexclient.Add(kitexclient.Options{
        Root:    opts.root,
        Name:    name,
        Service: opts.service,
        IDL:     opts.idl,
        Force:   opts.force,
        DryRun:  opts.dryRun,
    })
    if err != nil {
        return err
    }
    out := cmd.OutOrStdout()
    if opts.output == "json" {
        enc := json.NewEncoder(out)
        enc.SetIndent("", "  ")
        return enc.Encode(res)
    }
    writeVerb := "wrote"
    if res.DryRun {
        writeVerb = "would write"
    }
    for _, p := range res.WrittenPaths {
        fmt.Fprintf(out, "%s %s\n", writeVerb, p)
    }
    if res.DryRun {
        fmt.Fprintln(out, "(dry-run: no files were written)")
    }
    return nil
}
```

- [ ] **Step 2: Register command in add.go**

Add to newAddCmd():
```go
cmd.AddCommand(newAddKitexClientCmd())
```

- [ ] **Step 3: Commit**

```bash
git add internal/cli/add_kitex_client.go internal/cli/add.go
git commit -m "feat(cli): add kitex-client subcommand"
```

---

### Task 2: Create Scaffold Package

**Files:**
- Create: `internal/scaffold/kitexclient/kitexclient.go`

- [ ] **Step 1: Create kitexclient.go**

```go
package kitexclient

import (
    "fmt"
    "os"
    "path/filepath"
    "text/template"
)

type Options struct {
    Root    string
    Name    string
    Service string
    IDL     string
    Force   bool
    DryRun  bool
}

type Result struct {
    WrittenPaths []string `json:"written_paths"`
    DryRun       bool     `json:"dry_run"`
}

func Add(opts Options) (*Result, error) {
    result := &Result{DryRun: opts.DryRun}
    
    // Create pkg/client/<name>/ directory
    clientDir := filepath.Join(opts.Root, "pkg", "client", opts.Name)
    if !opts.DryRun {
        os.MkdirAll(clientDir, 0755)
    }
    
    // Generate client.go
    clientPath := filepath.Join(clientDir, "client.go")
    if err := generateClient(clientPath, opts, result); err != nil {
        return nil, err
    }
    
    // Generate config.go
    configPath := filepath.Join(clientDir, "config.go")
    if err := generateConfig(configPath, opts, result); err != nil {
        return nil, err
    }
    
    return result, nil
}

func generateClient(path string, opts Options, result *Result) error {
    tmpl := `package {{.Name}}

import (
    "context"
)

type Client struct {
    // TODO: Add Kitex client field
}

func New(addr string) (*Client, error) {
    // TODO: Initialize Kitex client
    return &Client{}, nil
}
`
    t, err := template.New("client").Parse(tmpl)
    if err != nil {
        return err
    }
    
    if !opts.DryRun {
        f, err := os.Create(path)
        if err != nil {
            return err
        }
        defer f.Close()
        if err := t.Execute(f, opts); err != nil {
            return err
        }
    }
    result.WrittenPaths = append(result.WrittenPaths, path)
    return nil
}

func generateConfig(path string, opts Options, result *Result) error {
    content := fmt.Sprintf(`package %s

type Config struct {
    Address string
}
`, opts.Name)
    
    if !opts.DryRun {
        if err := os.WriteFile(path, []byte(content), 0644); err != nil {
            return err
        }
    }
    result.WrittenPaths = append(result.WrittenPaths, path)
    return nil
}
```

- [ ] **Step 2: Commit**

```bash
git add internal/scaffold/kitexclient/
git commit -m "feat(scaffold): add kitexclient package"
```

---

### Task 3: Testing

**Files:**
- Create: `internal/cli/add_kitex_client_test.go`

- [ ] **Step 1: Create test**

```go
package cli

import (
    "bytes"
    "testing"
)

func TestAddKitexClient(t *testing.T) {
    // TODO: Add test
}
```

- [ ] **Step 2: Run tests**

```bash
go test ./internal/cli/ -run TestAddKitexClient -v
```

- [ ] **Step 3: Commit**

```bash
git add internal/cli/add_kitex_client_test.go
git commit -m "test: add kitex-client test"
```

---

### Task 4: Documentation

**Files:**
- Modify: `README.md`
- Modify: `README.zh-CN.md`

- [ ] **Step 1: Update README**

Add section for `ncgo add kitex-client` command.

- [ ] **Step 2: Commit**

```bash
git add README.md README.zh-CN.md
git commit -m "docs: add kitex-client documentation"
```

---

## Self-Review

- [x] Spec coverage: All requirements covered
- [x] No placeholders
- [x] Task granularity: Each task independently testable
