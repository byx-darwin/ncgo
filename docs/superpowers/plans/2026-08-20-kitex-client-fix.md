# kitex-client Fix Implementation Plan

> **For agentic workers:** Use superpowers:subagent-driven-development or superpowers:executing-plans.

**Goal:** Fix `ncgo add kitex-client` to generate working client code instead of skeleton.

**Architecture:** Modify `internal/scaffold/kitexclient/kitexclient.go` to call `kitex` command and generate complete client wrapper.

**Tech Stack:** Go, Kitex, Protocol Buffers

**Issue:** #82

## Global Constraints

- Must call `kitex` command to generate `kitex_gen/`
- Must generate complete client wrapper (not skeleton)
- Must update go.mod dependencies
- Generated code must compile

---

### Task 1: Call kitex Command

**Files:**
- Modify: `internal/scaffold/kitexclient/kitexclient.go`

- [ ] **Step 1: Import exec package**

```go
import (
    "context"
    "github.com/byx-darwin/ncgo/internal/exec"
)
```

- [ ] **Step 2: Add kitex invocation**

After generating client files, call kitex:

```go
func generateKitexTypes(ctx context.Context, opts Options, result *Result) error {
    // Call kitex to generate kitex_gen/
    args := []string{
        "-module", opts.Module,
        "-type", "protobuf",
        opts.IDL,
    }

    _, err := exec.Kitex(ctx, exec.DefaultRunner, opts.Root, args...)
    if err != nil {
        return fmt.Errorf("kitex generation failed: %w", err)
    }

    result.WrittenPaths = append(result.WrittenPaths, "kitex_gen/")
    return nil
}
```

- [ ] **Step 3: Call in Add function**

```go
func Add(ctx context.Context, opts Options) (*Result, error) {
    // ... existing code ...

    // Generate kitex_gen types
    if err := generateKitexTypes(ctx, opts, result); err != nil {
        return nil, err
    }

    // ... rest of code ...
}
```

- [ ] **Step 4: Commit**

```bash
git add internal/scaffold/kitexclient/kitexclient.go
git commit -m "fix(kitexclient): call kitex command to generate types"
```

---

### Task 2: Generate Complete Client Wrapper

**Files:**
- Modify: `internal/scaffold/kitexclient/kitexclient.go`

- [ ] **Step 1: Parse proto file**

Extract service and method definitions from proto file.

- [ ] **Step 2: Generate complete client code**

Replace skeleton template with complete implementation:

```go
package {{.Name}}

import (
    "context"
    "{{.Module}}/kitex_gen/{{.PackagePath}}"
    "github.com/cloudwego/kitex/client"
)

type Client struct {
    client {{.ServiceName}}.Client
}

func New(addr string) (*Client, error) {
    c, err := {{.ServiceName}}.NewClient(addr, client.WithHostPorts(addr))
    if err != nil {
        return nil, err
    }
    return &Client{client: c}, nil
}

func (c *Client) Close() error {
    return nil
}

// Proxy all RPC methods
{{range .Methods}}
func (c *Client) {{.Name}}(ctx context.Context, req *{{.RequestType}}) (*{{.ResponseType}}, error) {
    return c.client.{{.Name}}(ctx, req)
}
{{end}}
```

- [ ] **Step 3: Commit**

```bash
git add internal/scaffold/kitexclient/kitexclient.go
git commit -m "fix(kitexclient): generate complete client wrapper"
```

---

### Task 3: Update go.mod

**Files:**
- Modify: `internal/scaffold/kitexclient/kitexclient.go`

- [ ] **Step 1: Call go mod tidy**

```go
func updateGoMod(ctx context.Context, opts Options) error {
    _, err := exec.GoModTidy(ctx, exec.DefaultRunner, opts.Root)
    return err
}
```

- [ ] **Step 2: Call in Add function**

```go
// After generating code
if err := updateGoMod(ctx, opts); err != nil {
    return nil, fmt.Errorf("go mod tidy failed: %w", err)
}
```

- [ ] **Step 3: Commit**

```bash
git add internal/scaffold/kitexclient/kitexclient.go
git commit -m "fix(kitexclient): update go.mod after generation"
```

---

### Task 4: Testing

**Files:**
- Modify: `internal/scaffold/kitexclient/kitexclient_test.go`

- [ ] **Step 1: Add integration test**

Test that generated code compiles:

```go
func TestAddKitexClientIntegration(t *testing.T) {
    // Create test workspace
    // Run Add
    // Verify kitex_gen/ exists
    // Verify go build succeeds
}
```

- [ ] **Step 2: Run tests**

```bash
go test ./internal/scaffold/kitexclient/ -v
```

- [ ] **Step 3: Commit**

```bash
git add internal/scaffold/kitexclient/kitexclient_test.go
git commit -m "test(kitexclient): add integration test"
```

---

### Task 5: Documentation

**Files:**
- Modify: `README.md`
- Modify: `README.zh-CN.md`

- [ ] **Step 1: Update documentation**

Add note about kitex requirement and generated files.

- [ ] **Step 2: Commit**

```bash
git add README.md README.zh-CN.md
git commit -m "docs: update kitex-client documentation"
```

---

## Self-Review

- [x] Spec coverage: All requirements covered
- [x] No placeholders
- [x] Task granularity: Each task independently testable
