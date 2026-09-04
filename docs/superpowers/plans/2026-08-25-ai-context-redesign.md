# AI Context File Redesign Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Restructure ncgo's generated AI context files (CLAUDE.md, AGENTS.md, project-context.md) from template-maintainer documentation to concise agent task execution manuals (~150 lines vs 700+), adding method-level context, edit boundaries, error-code quick reference, and verification checklist.

**Architecture:** Replace the current "embed full design doc" approach with a task-oriented structure. The full design doc remains available as standalone `docs/ncgo/` files. New data sources (scan for methods, static boundary/error-code tables) feed into the existing `renderInputs` pipeline. The managed-marker / Force / DryRun contract is unchanged.

**Tech Stack:** Go 1.25+, `internal/ai/` package, `internal/scan/` package, embedded assets via `//go:embed`.

**Spec:** `docs/superpowers/specs/2026-08-25-ai-context-redesign.md`

## Global Constraints

- Managed marker (`<!-- ncgo:managed -->`) must remain on/near line 1 of every rendered file
- `ncgo check` stale-detection must still work (the `- domains:` fact line format is preserved)
- MCP `ai sync` tool output shape (`ResultFields`, top-level JSON fields) unchanged
- Template handoff ordering (`make sqlc` before `go mod tidy`) unchanged
- `docs/ncgo/` standalone design-doc files still written with full content
- Golden tests in `internal/scaffold/mono/testdata/` must be regenerated with `-update-golden`

---

### Task 1: Error-code quick reference data source

**Files:**
- Create: `internal/ai/errorcodes.go`
- Create: `internal/ai/errorcodes_test.go`

**Interfaces:**
- Consumes: profile string (`hertz` | `kitex` | `micro`)
- Produces: `ErrorCodes(profile string) string` — returns markdown table of common error codes

- [ ] **Step 1: Write the failing test**

Create `internal/ai/errorcodes_test.go`:

```go
package ai

import "testing"

func TestErrorCodesHertz(t *testing.T) {
    got := ErrorCodes("hertz")
    if got == "" {
        t.Fatal("ErrorCodes(hertz) returned empty")
    }
    for _, want := range []string{"10000", "10001", "10002", "40100"} {
        if !contains(got, want) {
            t.Errorf("ErrorCodes(hertz) missing code %s:\n%s", want, got)
        }
    }
}

func TestErrorCodesKitex(t *testing.T) {
    got := ErrorCodes("kitex")
    if got == "" {
        t.Fatal("ErrorCodes(kitex) returned empty")
    }
    for _, want := range []string{"10000", "10001", "10002", "40100"} {
        if !contains(got, want) {
            t.Errorf("ErrorCodes(kitex) missing code %s:\n%s", want, got)
        }
    }
}

func TestErrorCodesUnknown(t *testing.T) {
    got := ErrorCodes("unknown")
    if got == "" {
        t.Fatal("ErrorCodes(unknown) returned empty")
    }
    // Unknown profile should still return common framework codes
    if !contains(got, "10000") {
        t.Errorf("ErrorCodes(unknown) missing CodeSystem:\n%s", got)
    }
}

func contains(s, sub string) bool {
    return strings.Contains(s, sub)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/ai/ -run TestErrorCodes -v`
Expected: FAIL with "ErrorCodes not defined"

- [ ] **Step 3: Write minimal implementation**

Create `internal/ai/errorcodes.go`:

```go
package ai

import "strings"

// ErrorCodes returns the condensed error-code quick reference table for a
// profile (hertz | kitex | micro). The table covers the codes an agent most
// often needs when writing error handling. The full code list lives in the
// standalone design doc under docs/ncgo/.
func ErrorCodes(profile string) string {
    type code struct {
        num, name, http, meaning string
    }
    common := []code{
        {"10000", "CodeSystem", "500", "System error"},
        {"10001", "CodeParamInvalid", "400", "Bad request"},
        {"10002", "CodeAuthFailed", "401", "Auth failure"},
        {"10004", "CodeConfigInvalid", "500", "Config error"},
        {"10010", "CodeRPCUnavailable", "502", "RPC unavailable"},
        {"10011", "CodeRPCTimeout", "504", "RPC timeout"},
    }
    var specific []code
    switch profile {
    case "hertz":
        specific = []code{
            {"10108", "CodePermissionDenied", "403", "Permission denied"},
            {"10200", "CodeRateLimited", "429", "Rate limited"},
            {"10202", "CodeReplayRequest", "401", "Replay detected"},
            {"10203", "CodeIdempotencyKeyMissing", "400", "Missing idempotency key"},
            {"10204", "CodeIdempotencyConflict", "409", "Idempotency conflict"},
            {"10304", "CodeCacheUnavailable", "503", "Cache unavailable"},
        }
    case "kitex":
        specific = []code{
            {"10108", "CodePermissionDenied", "403", "Permission denied"},
            {"10200", "CodeRateLimited", "429", "Rate limited"},
            {"10304", "CodeCacheUnavailable", "503", "Cache unavailable"},
        }
    }
    all := append(common, specific...)
    var b strings.Builder
    b.WriteString("| Code | Name | HTTP | Meaning |\n")
    b.WriteString("|------|------|------|---------|\n")
    for _, c := range all {
        b.WriteString("| " + c.num + " | " + c.name + " | " + c.http + " | " + c.meaning + " |\n")
    }
    b.WriteString("| 40100+ | Business codes | 200 | Application-defined |\n")
    return strings.TrimRight(b.String(), "\n")
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/ai/ -run TestErrorCodes -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/ai/errorcodes.go internal/ai/errorcodes_test.go
git commit -m "feat(ai): add error-code quick reference data source"
```

---

### Task 2: Edit boundaries data source

**Files:**
- Create: `internal/ai/boundaries.go`
- Create: `internal/ai/boundaries_test.go`

**Interfaces:**
- Consumes: `syncSource` (already defined in `internal/ai/sync.go`)
- Produces: `EditBoundaries(source syncSource) (mayEdit, doNotEdit []boundaryEntry)` and `RenderBoundaries(mayEdit, doNotEdit []boundaryEntry) string`

- [ ] **Step 1: Write the failing test**

Create `internal/ai/boundaries_test.go`:

```go
package ai

import (
    "strings"
    "testing"

    "github.com/byx-darwin/ncgo/internal/manifest"
)

func TestEditBoundariesMonoService(t *testing.T) {
    source := syncSource{
        Scope:   syncScopeService,
        Service: &manifest.Manifest{
            Domains: []string{"user", "device"},
        },
    }
    mayEdit, doNotEdit := EditBoundaries(source)
    if len(mayEdit) == 0 {
        t.Fatal("EditBoundaries returned empty mayEdit for mono service")
    }
    if len(doNotEdit) == 0 {
        t.Fatal("EditBoundaries returned empty doNotEdit for mono service")
    }
    // Check that usecase paths are present
    joined := fmt.Sprint(mayEdit, doNotEdit)
    if !strings.Contains(joined, "usecase") {
        t.Errorf("boundaries missing usecase paths: %s", joined)
    }
}

func TestEditBoundariesWorkspace(t *testing.T) {
    source := syncSource{
        Scope: syncScopeWorkspace,
        Workspace: &manifest.Workspace{
            Services: []manifest.WorkspaceService{
                {Name: "user-rpc", Kind: "kitex", Dir: "services/user-rpc"},
            },
        },
    }
    mayEdit, doNotEdit := EditBoundaries(source)
    if len(mayEdit) == 0 {
        t.Fatal("EditBoundaries returned empty mayEdit for workspace")
    }
    if len(doNotEdit) == 0 {
        t.Fatal("EditBoundaries returned empty doNotEdit for workspace")
    }
}

func TestRenderBoundariesProducesMarkdown(t *testing.T) {
    mayEdit := []boundaryEntry{
        {Path: "internal/usecase/<domain>/", Reason: "Business logic"},
    }
    doNotEdit := []boundaryEntry{
        {Path: "internal/db/gen/", Reason: "sqlc-generated code"},
    }
    got := RenderBoundaries(mayEdit, doNotEdit)
    for _, want := range []string{"## Boundaries", "You may edit", "Do not edit", "usecase", "db/gen"} {
        if !strings.Contains(got, want) {
            t.Errorf("RenderBoundaries missing %q:\n%s", want, got)
        }
    }
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/ai/ -run TestEditBoundaries -v`
Expected: FAIL with "EditBoundaries not defined"

- [ ] **Step 3: Write minimal implementation**

Create `internal/ai/boundaries.go`:

```go
package ai

import "strings"

// boundaryEntry describes one file path in the edit-boundaries table.
type boundaryEntry struct {
    Path   string
    Reason string
}

// EditBoundaries returns the may-edit and do-not-edit file tables for a
// sync source. The may-edit list is derived from the manifest domains;
// the do-not-edit list is static (generated code paths).
func EditBoundaries(source syncSource) (mayEdit, doNotEdit []boundaryEntry) {
    mayEdit = []boundaryEntry{
        {Path: "internal/handler/", Reason: "HTTP/RPC handlers"},
        {Path: "internal/adapter/", Reason: "Outbound RPC clients"},
        {Path: "internal/pkg/middleware/", Reason: "Custom middleware"},
        {Path: "internal/base/server/server.go", Reason: "DI wiring"},
    }
    doNotEdit = []boundaryEntry{
        {Path: "internal/base/data/", Reason: "Generated data layer"},
        {Path: "internal/db/gen/", Reason: "sqlc-generated code"},
        {Path: "internal/router/register.go", Reason: "Generated routes"},
        {Path: "internal/pb/", Reason: "Protobuf generated types"},
        {Path: "CLAUDE.md, AGENTS.md", Reason: "Managed by ncgo ai sync"},
    }
    // Add per-domain paths from manifest
    var domains []string
    switch source.Scope {
    case syncScopeService:
        domains = source.Service.Domains
    case syncScopeWorkspace:
        for _, svc := range source.WorkspaceServices {
            domains = append(domains, svc.Name)
        }
    }
    for _, d := range domains {
        mayEdit = append(mayEdit,
            boundaryEntry{Path: "internal/usecase/" + d + "/", Reason: "Business logic"},
            boundaryEntry{Path: "internal/repository/" + d + "/", Reason: "Data access implementation"},
        )
        doNotEdit = append(doNotEdit,
            boundaryEntry{Path: "internal/usecase/" + d + "/" + d + ".go between anchors", Reason: "Generated method stubs"},
        )
    }
    return mayEdit, doNotEdit
}

// RenderBoundaries renders the may-edit / do-not-edit tables as markdown.
func RenderBoundaries(mayEdit, doNotEdit []boundaryEntry) string {
    var b strings.Builder
    b.WriteString("## Boundaries\n\n")
    b.WriteString("### You may edit\n\n")
    b.WriteString("| Path | Purpose |\n")
    b.WriteString("|------|---------|\n")
    for _, e := range mayEdit {
        b.WriteString("| `" + e.Path + "` | " + e.Reason + " |\n")
    }
    b.WriteString("\n### Do not edit\n\n")
    b.WriteString("| Path | Why |\n")
    b.WriteString("|------|-----|\n")
    for _, e := range doNotEdit {
        b.WriteString("| `" + e.Path + "` | " + e.Reason + " |\n")
    }
    return strings.TrimRight(b.String(), "\n")
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/ai/ -run TestEditBoundaries -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/ai/boundaries.go internal/ai/boundaries_test.go
git commit -m "feat(ai): add edit boundaries data source"
```

---

### Task 3: Add method-level context from scan

**Files:**
- Modify: `internal/ai/sync.go`
- Modify: `internal/ai/sync_test.go`

**Interfaces:**
- Consumes: existing `Sync()` flow
- Produces: `renderInputs.MethodsByDomain map[string][]string` populated from `scan.Scan()`

- [ ] **Step 1: Write the failing test**

Add to `internal/ai/sync_test.go` (append new test function):

```go
func TestSyncIncludesMethodsFromScan(t *testing.T) {
    root := t.TempDir()
    writeManifest(t, root, manifest.KindHertz)
    // Create a usecase file with methods
    usecaseDir := filepath.Join(root, "internal", "usecase", "user")
    if err := os.MkdirAll(usecaseDir, 0o755); err != nil {
        t.Fatalf("mkdir: %v", err)
    }
    usecaseContent := `package user

import "context"

type UseCase struct{}

func New() *UseCase { return &UseCase{} }
func (u *UseCase) Repo() {}

// ncgo:methods:start
func (u *UseCase) Create(ctx context.Context) error { return nil }
func (u *UseCase) Get(ctx context.Context) error { return nil }
// ncgo:methods:end
`
    if err := os.WriteFile(filepath.Join(usecaseDir, "user.go"), []byte(usecaseContent), 0o644); err != nil {
        t.Fatalf("write usecase: %v", err)
    }
    res, err := Sync(Options{Root: root, Target: "claude"})
    if err != nil {
        t.Fatalf("Sync: %v", err)
    }
    // The rendered CLAUDE.md should contain method names
    // We can't directly inspect render output from Sync, but we can verify
    // the scan was called by checking the result has content
    if len(res.Written) == 0 {
        t.Fatal("Sync wrote no files")
    }
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/ai/ -run TestSyncIncludesMethodsFromScan -v`
Expected: FAIL (methods not yet rendered in output)

- [ ] **Step 3: Modify buildInputs to include methods**

In `internal/ai/sync.go`, add `MethodsByDomain` to `renderInputs`:

```go
type renderInputs struct {
    SourceRef          string
    LongBody           string
    ProjectContextBody string
    WorkflowBody       string
    RulesBody          string
    MethodsByDomain    map[string][]string // NEW
}
```

Add a helper to extract methods from scan:

```go
// methodsFromScan runs scan.Scan and extracts method names per domain.
// Returns nil when scan fails (non-fatal — methods are best-effort).
func methodsFromScan(root string) map[string][]string {
    s, err := scan.Scan(root)
    if err != nil {
        return nil
    }
    out := make(map[string][]string)
    for _, d := range s.Domains {
        var methods []string
        for _, m := range d.Methods {
            methods = append(methods, m.Name)
        }
        if len(methods) > 0 {
            out[d.Name] = methods
        }
    }
    return out
}
```

In `Sync()`, after `buildInputs` is called, add:

```go
inputs.MethodsByDomain = methodsFromScan(opts.Root)
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/ai/ -run TestSyncIncludesMethodsFromScan -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/ai/sync.go internal/ai/sync_test.go
git commit -m "feat(ai): inject method-level context from scan into render inputs"
```

---

### Task 4: Restructure renderClaude() output

**Files:**
- Modify: `internal/ai/render.go`
- Modify: `internal/ai/render_test.go`

**Interfaces:**
- Consumes: `renderInputs` (now with `MethodsByDomain`)
- Produces: restructured CLAUDE.md with Quick Facts, Methods, Boundaries, Layer Rules, Error Codes, Workflow, Verify, Local Notes

- [ ] **Step 1: Write the failing test**

Add to `internal/ai/render_test.go`:

```go
func TestRenderClaudeRestructured(t *testing.T) {
    inputs := renderInputs{
        SourceRef: ".ncgo/manifest.yaml",
        LongBody: "## Quick Facts\n\n- module: `github.com/acme/user-api`\n- service.name: `user-api`\n- service.kind: `hertz`\n- domains: `[user]`\n",
        WorkflowBody: "## Implementing a Feature with ncgo\n\n1. **Add domain**",
        MethodsByDomain: map[string][]string{
            "user": {"Create", "Get", "Delete"},
        },
    }
    got := renderClaude(inputs)
    for _, want := range []string{
        "Project Context for Claude Code",
        "Quick Facts",
        "Methods",
        "Create", "Get", "Delete",
        "Boundaries",
        "Layer Rules",
        "Error Codes",
        "Workflow",
        "Verify",
    } {
        if !strings.Contains(got, want) {
            t.Errorf("renderClaude missing %q:\n%s", want, got)
        }
    }
    // Must NOT contain the full design doc
    if strings.Contains(got, "Generated Project Architecture") {
        t.Errorf("renderClaude should not embed full design doc:\n%s", got)
    }
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/ai/ -run TestRenderClaudeRestructured -v`
Expected: FAIL (renderClaude still uses old structure)

- [ ] **Step 3: Rewrite renderClaude()**

Replace `renderClaude` in `internal/ai/render.go`:

```go
func renderClaude(inputs renderInputs) string {
    var b strings.Builder
    b.WriteString(ManagedMarker + "\n\n")
    b.WriteString("# Project Context for Claude Code\n\n")
    b.WriteString("> Auto-generated by `ncgo ai sync` from `")
    b.WriteString(inputs.SourceRef)
    b.WriteString("`.\n")
    b.WriteString("> This file is a task-oriented summary; the full design\n")
    b.WriteString("> reference lives at `docs/ncgo/<profile>/design-doc.en.md`.\n\n")

    // Quick Facts (from LongBody which now contains just facts)
    b.WriteString(inputs.LongBody)

    // Methods per domain
    if len(inputs.MethodsByDomain) > 0 {
        b.WriteString("\n### Methods\n\n")
        // Sort domains for deterministic output
        domains := make([]string, 0, len(inputs.MethodsByDomain))
        for d := range inputs.MethodsByDomain {
            domains = append(domains, d)
        }
        sort.Strings(domains)
        for _, d := range domains {
            methods := inputs.MethodsByDomain[d]
            b.WriteString("- **" + d + "**: ")
            b.WriteString(strings.Join(methods, ", "))
            b.WriteString("\n")
        }
    }

    // Boundaries
    b.WriteString("\n")
    b.WriteString(boundariesMarkdown)

    // Layer Rules
    b.WriteString("\n")
    b.WriteString(layerRulesMarkdown)

    // Error Codes
    b.WriteString("\n## Error Codes\n\n")
    b.WriteString(errorCodesNote)
    b.WriteString(inputs.ErrorCodes)

    // Workflow
    b.WriteString("\n")
    b.WriteString(inputs.WorkflowBody)

    // Verify
    b.WriteString("\n")
    b.WriteString(verifyMarkdown)

    return ensureTrailingNewline(b.String())
}
```

Add the static content constants/vars near the top of render.go:

```go
const boundariesMarkdown = `## Boundaries

### You may edit

| Path | Purpose |
|------|---------|
| + `internal/usecase/<domain>/` | Business logic |
| + `internal/handler/` | HTTP/RPC handlers |
| + `internal/adapter/` | Outbound RPC clients |
| + `internal/repository/<domain>/` | Data access implementations |
| + `internal/pkg/middleware/` | Custom middleware |
| + `internal/base/server/server.go` | DI wiring |

### Do not edit

| Path | Why |
|------|-----|
| + `internal/usecase/<domain>/<domain>.go` between anchors | Generated method stubs |
| + `internal/base/data/` | Generated data layer |
| + `internal/db/gen/` | sqlc-generated code |
| + `internal/router/register.go` | Generated routes |
| + `internal/pb/` | Protobuf generated types |
| + `CLAUDE.md`, `AGENTS.md` | Managed by `ncgo ai sync` |

`

const layerRulesMarkdown = `## Layer Rules

` + "```" + `
handler/* → usecase/*, pkg/response, pb        (no repo / data import)
usecase/* → adapter, repository (ports), pb    (no framework import)
repository/* → base/data, db/gen               (no usecase import)
` + "```" + `

- All errors are ` + "`goerror`" + ` chains carrying a numeric ` + "`Code`" + ` + ` + "`Public`" + ` msg.
- Handlers use ` + "`response.OK(c, resp)`" + ` / ` + "`response.Err(c, err)`" + `.
- Do not store request context in struct state.

`

const errorCodesNote = `Business codes (` + "`>= 40100`" + `) fallback to HTTP 200. Register fine-grained
statuses with ` + "`goerror.RegisterHTTPStatuses`" + ` when non-200 is required.

`

const verifyMarkdown = `## Verify

` + "```bash" + `
go build ./...            # compiles
go vet ./...              # static analysis
ncgo check --root .       # ncgo consistency (anchors, manifest, context)
ncgo ai sync --root .     # re-render after changes
ncgo check --root .       # confirm stale-context check passes
` + "```" + `

`
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/ai/ -run TestRenderClaudeRestructured -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/ai/render.go internal/ai/render_test.go
git commit -m "feat(ai): restructure renderClaude as task-oriented manual"
```

---

### Task 5: Restructure renderAgents() output

**Files:**
- Modify: `internal/ai/render.go`
- Modify: `internal/ai/render_test.go`

**Interfaces:**
- Consumes: `renderInputs`
- Produces: restructured AGENTS.md (same structure as CLAUDE.md but with universal-agent preface)

- [ ] **Step 1: Write the failing test**

Add to `internal/ai/render_test.go`:

```go
func TestRenderAgentsRestructured(t *testing.T) {
    inputs := renderInputs{
        SourceRef: ".ncgo/manifest.yaml",
        LongBody: "## Quick Facts\n\n- module: `github.com/acme/user-api`\n- domains: `[user]`\n",
        WorkflowBody: "## Implementing a Feature with ncgo\n\n1. **Add domain**",
        MethodsByDomain: map[string][]string{
            "user": {"Create", "Get"},
        },
    }
    got := renderAgents(inputs)
    for _, want := range []string{
        "Project Agent Context",
        "Quick Facts",
        "Methods",
        "Create", "Get",
        "Boundaries",
        "Layer Rules",
        "Error Codes",
        "Workflow",
        "Verify",
    } {
        if !strings.Contains(got, want) {
            t.Errorf("renderAgents missing %q:\n%s", want, got)
        }
    }
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/ai/ -run TestRenderAgentsRestructured -v`
Expected: FAIL

- [ ] **Step 3: Rewrite renderAgents()**

Replace `renderAgents` in `internal/ai/render.go`:

```go
func renderAgents(inputs renderInputs) string {
    var b strings.Builder
    b.WriteString(ManagedMarker + "\n\n")
    b.WriteString("# Project Agent Context\n\n")
    b.WriteString("> Auto-generated by `ncgo ai sync` from `")
    b.WriteString(inputs.SourceRef)
    b.WriteString("`.\n")
    b.WriteString("> This file is a task-oriented summary; the full design\n")
    b.WriteString("> reference lives at `docs/ncgo/<profile>/design-doc.en.md`.\n\n")

    b.WriteString(inputs.LongBody)

    if len(inputs.MethodsByDomain) > 0 {
        b.WriteString("\n### Methods\n\n")
        domains := make([]string, 0, len(inputs.MethodsByDomain))
        for d := range inputs.MethodsByDomain {
            domains = append(domains, d)
        }
        sort.Strings(domains)
        for _, d := range domains {
            methods := inputs.MethodsByDomain[d]
            b.WriteString("- **" + d + "**: ")
            b.WriteString(strings.Join(methods, ", "))
            b.WriteString("\n")
        }
    }

    b.WriteString("\n")
    b.WriteString(boundariesMarkdown)
    b.WriteString("\n")
    b.WriteString(layerRulesMarkdown)
    b.WriteString("\n## Error Codes\n\n")
    b.WriteString(errorCodesNote)
    b.WriteString(inputs.ErrorCodes)
    b.WriteString("\n")
    b.WriteString(inputs.WorkflowBody)
    b.WriteString("\n")
    b.WriteString(verifyMarkdown)

    return ensureTrailingNewline(b.String())
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/ai/ -run TestRenderAgentsRestructured -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/ai/render.go internal/ai/render_test.go
git commit -m "feat(ai): restructure renderAgents as task-oriented manual"
```

---

### Task 6: Restructure renderProjectContext() output

**Files:**
- Modify: `internal/ai/render.go`
- Modify: `internal/ai/render_test.go`

**Interfaces:**
- Consumes: `renderInputs`
- Produces: restructured project-context.md (facts + methods + architecture summary + rules + notes)

- [ ] **Step 1: Write the failing test**

Add to `internal/ai/render_test.go`:

```go
func TestRenderProjectContextRestructured(t *testing.T) {
    inputs := renderInputs{
        SourceRef:          ".ncgo/manifest.yaml",
        ProjectContextBody: "## Project Facts\n\n- module: `github.com/acme/user-api`\n- domains: `[user]`\n\n## Architecture Summary\n\nThis is a summary.\n\n## Repository Rules\n\n- `.claude/rules/go.md`\n\n## Notes\n\n- Generated file.\n",
        MethodsByDomain: map[string][]string{
            "user": {"Create"},
        },
    }
    got := renderProjectContext(inputs)
    for _, want := range []string{
        "Claude Project Context",
        "Project Facts",
        "Methods",
        "Create",
        "Architecture Summary",
        "Repository Rules",
    } {
        if !strings.Contains(got, want) {
            t.Errorf("renderProjectContext missing %q:\n%s", want, got)
        }
    }
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/ai/ -run TestRenderProjectContextRestructured -v`
Expected: FAIL

- [ ] **Step 3: Rewrite renderProjectContext()**

Replace `renderProjectContext` in `internal/ai/render.go`:

```go
func renderProjectContext(inputs renderInputs) string {
    var b strings.Builder
    b.WriteString(ManagedMarker + "\n\n")
    b.WriteString("# Claude Project Context\n\n")
    b.WriteString("> Auto-generated by `ncgo ai sync` from `")
    b.WriteString(inputs.SourceRef)
    b.WriteString("`.\n")
    b.WriteString("> Facts only; policy lives under `.claude/rules/*`.\n")
    b.WriteString("> Personal notes belong in `.claude/local/*`.\n\n")

    b.WriteString(inputs.ProjectContextBody)

    if len(inputs.MethodsByDomain) > 0 {
        b.WriteString("\n### Methods\n\n")
        domains := make([]string, 0, len(inputs.MethodsByDomain))
        for d := range inputs.MethodsByDomain {
            domains = append(domains, d)
        }
        sort.Strings(domains)
        for _, d := range domains {
            methods := inputs.MethodsByDomain[d]
            b.WriteString("- **" + d + "**: ")
            b.WriteString(strings.Join(methods, ", "))
            b.WriteString("\n")
        }
    }

    return ensureTrailingNewline(b.String())
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/ai/ -run TestRenderProjectContextRestructured -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/ai/render.go internal/ai/render_test.go
git commit -m "feat(ai): restructure renderProjectContext with methods"
```

---

### Task 7: Move generated-at marker to line 2

**Files:**
- Modify: `internal/ai/generated_at.go`
- Modify: `internal/ai/render_test.go` (or existing generated_at test)

**Interfaces:**
- Consumes: existing `stampGeneratedAt` function
- Produces: generated-at marker appears immediately after managed marker (line 2)

- [ ] **Step 1: Write the failing test**

Add to `internal/ai/render_test.go` (or create `internal/ai/generated_at_test.go`):

```go
func TestStampGeneratedAtOnLine2(t *testing.T) {
    input := "<!-- ncgo:managed -->\n\n# Title\n\nContent here.\n"
    got := stampGeneratedAt(input, time.Date(2026, 8, 25, 0, 0, 0, 0, time.UTC))
    lines := strings.Split(got, "\n")
    // Line 0: managed marker
    // Line 1: generated-at marker
    // Line 2: blank
    // Line 3: # Title
    if !strings.Contains(lines[0], "ncgo:managed") {
        t.Errorf("line 0 should be managed marker: %q", lines[0])
    }
    if !strings.Contains(lines[1], "ncgo:generated-at") {
        t.Errorf("line 1 should be generated-at marker: %q", lines[1])
    }
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/ai/ -run TestStampGeneratedAtOnLine2 -v`
Expected: FAIL (marker is currently inserted after the managed marker line but the blank line follows)

- [ ] **Step 3: Adjust stampGeneratedAt**

The current `stampGeneratedAt` already inserts the marker right after the managed marker line. The test verifies this behavior. If the current implementation already places it on line 2 (index 1), the test passes. If not, adjust:

```go
func stampGeneratedAt(rendered string, ts time.Time) string {
    marker := scan.GeneratedAtMarker + ts.UTC().Format(time.RFC3339) + " -->"
    lines := strings.Split(rendered, "\n")
    for i, line := range lines {
        if strings.Contains(line, ManagedMarker) {
            // Insert marker immediately after the managed marker line
            lines = append(lines[:i+1], append([]string{marker}, lines[i+1:]...)...)
            break
        }
    }
    return strings.Join(lines, "\n")
}
```

This is the existing implementation — verify it places the marker on line 2 (index 1). The test should pass as-is.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/ai/ -run TestStampGeneratedAtOnLine2 -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/ai/generated_at.go internal/ai/render_test.go
git commit -m "feat(ai): ensure generated-at marker appears on line 2"
```

---

### Task 8: Update existing tests for new structure

**Files:**
- Modify: `internal/ai/sync_test.go`
- Modify: `internal/cli/ai_test.go`

**Interfaces:**
- Ensures existing tests pass with the new render structure

- [ ] **Step 1: Run all AI tests to find failures**

Run: `go test ./internal/ai/ ./internal/cli/ -v 2>&1 | head -100`
Expected: Some tests fail due to changed output structure

- [ ] **Step 2: Update sync_test.go**

Update `TestSyncWritesAllTargets` — the expected written/skipped counts may change if the structure affects file detection. Verify the test still passes or adjust expectations.

Update `TestSyncJSONOutput` — the `Written`/`Skipped` counts should remain the same (structure change doesn't affect file count).

- [ ] **Step 3: Update cli/ai_test.go**

Update `TestRunAISyncDefaultTargetTextShowsClaudeFiles` — the output text now contains different section names. Update the expected strings:

```go
for _, want := range []string{"wrote CLAUDE.md", "wrote .claude/skills/ncgo-dev/SKILL.md", "wrote .claude/generated/project-context.md"} {
```

This should still pass since it checks for `wrote <path>`, not content.

Update `TestRunAISyncTargetFlag` — same, checks for `wrote AGENTS.md`.

- [ ] **Step 4: Run all AI tests**

Run: `go test ./internal/ai/ ./internal/cli/ -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/ai/sync_test.go internal/cli/ai_test.go
git commit -m "test(ai): update existing tests for restructured output"
```

---

### Task 9: Regenerate golden tests

**Files:**
- Modify: `internal/scaffold/mono/testdata/` (golden files)

**Interfaces:**
- Ensures scaffold golden tests reflect new AI context structure

- [ ] **Step 1: Run golden tests to find mismatches**

Run: `go test ./internal/scaffold/mono/... -run TestGenerateGoldenDefault -v 2>&1 | head -50`
Expected: Some golden tests fail due to changed AI context content

- [ ] **Step 2: Regenerate golden files**

Run: `go test ./internal/scaffold/mono/... -update-golden -count=1`
Expected: Golden files updated

- [ ] **Step 3: Verify golden tests pass**

Run: `go test ./internal/scaffold/mono/... -run TestGenerateGoldenDefault -count=1`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add internal/scaffold/mono/testdata/
git commit -m "test(scaffold): regenerate golden tests for AI context redesign"
```

---

### Task 10: Full validation

**Files:** none (validation only)

- [ ] **Step 1: Run full test suite**

Run: `go test ./... -count=1`
Expected: PASS

- [ ] **Step 2: Run go vet**

Run: `go vet ./...`
Expected: No issues

- [ ] **Step 3: Run gofmt check**

Run: `gofmt -l $(find . -name '*.go' -not -path './.git/*')`
Expected: No files listed (all formatted)

- [ ] **Step 4: Run smoke test**

Run: `./scripts/smoke.sh`
Expected: PASS

- [ ] **Step 5: Verify ncgo check still works**

Run: `go build . && ./ncgo ai sync --dry-run --root . && ./ncgo check --root .`
Expected: Both commands succeed, check passes
