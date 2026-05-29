# ncgo Proto I/O Validation System Design

- Status: v1
- Scope: `ncgo protolint` CLI command and MCP tool
- Full design (ZH): [005-proto-io-system.zh-CN.md](./005-proto-io-system.zh-CN.md)

## 1. Overview

Proto I/O is ncgo's built-in protobuf file validation system, exposed via the `ncgo protolint` CLI command and the `ncgo_protolint` MCP tool.

Core capabilities:
- Structured lint rules for `.proto` files
- Three output formats: text, json, SARIF
- Rule filtering, file filtering, and ignore patterns
- Proto parsing via `bufbuild/protocompile`

## 2. Rule System

### Rule Naming

Rules use a `PIO` prefix (Proto I/O), organized in two phases:

**Phase 1 (Error level):**
`PIO201` -- Dynamic I/O, `PIO202` -- HTTP Body Binding, `PIO203` -- HTTP Methods,
`PIO204` -- Kitex Envelope, `PIO205` -- Multi-binding, `PIO206` -- OpenAPI Missing,
`PIO207` -- Pagination Missing, `PIO208` -- Path Params, `PIO209` -- PGV Constraints,
`PIO210` -- PGV Pagination, `PIO211` -- Request/Response, `PIO212` -- Response Bindings,
`PIO213` -- Unbound Fields, `PIO214` -- Universal Request, `PIO215` -- Raw Body

**Phase 2 (Warning level):**
Field naming conventions, comment completeness, and other advisory rules.

### Rule Registry

Rules are defined in `internal/protolint/rules.go` using a registry pattern:
```go
var rules = map[string]Rule{...}
```
Each rule implements `Check(ctx, file)` returning `[]Diagnostic`.

## 3. Key Design Decisions

### Parser

Uses `bufbuild/protocompile` for proto parsing with:
- Incremental parsing
- Import path resolution
- Source location preservation (for diagnostic positioning)

### Diagnostic Model

```go
type Diagnostic struct {
    Rule     string
    Message  string
    File     string
    Line     int
    Column   int
    Severity Severity // Error | Warning
}
```

### SARIF Output

Follows SARIF 2.1.0 standard. Serialization in `internal/protolint/sarif.go`.

### Ignore Mechanisms

- `--ignore-rule`: skip specific rules
- `--ignore-file`: skip specific files
- Phase 2 rules (warnings) do not fail by default

### Workspace Support

In micro workspaces, `protolint` recursively scans all service IDL directories.

## 4. CLI / MCP Interface

### CLI Commands

```bash
# Check all proto files
ncgo protolint --root .

# Check specific file
ncgo protolint --root . --file idl/app/user.proto

# Filter by rules
ncgo protolint --root . --rule PIO201 --rule PIO202

# Ignore rules
ncgo protolint --root . --ignore-rule PIO207

# Output formats
ncgo protolint --root . --output json
ncgo protolint --root . --output sarif
```

### MCP Tool

| Tool              | Description                                           |
|-------------------|-------------------------------------------------------|
| `ncgo_protolint`  | Check proto files, supports text/json/sarif output    |

Parameters: `root` (required), `files`, `rules`, `ignoreRules`, `ignoreFiles`, `output`.

## 5. Test Data

Test cases in `internal/protolint/testdata/`, each subdirectory contains:
- Proto files triggering specific rules
- Expected diagnostic output

## 6. Related Files

- `internal/protolint/rules.go` -- Rule registry
- `internal/protolint/sarif.go` -- SARIF serialization
- `internal/protolint/load.go` -- Proto file loading
- `schemas/` -- Related JSON Schemas

## 7. Archived Detailed Docs

Original design documents archived in `specs/archive/`:
- `proto-io-validation.zh-CN.md`
- `proto-io-lint-rules.zh-CN.md`
- `proto-io-implementation.zh-CN.md`
- `proto-io-tech-design.zh-CN.md`

## 8. Reference

Full rule specifications, technical design decisions, and implementation breakdown are in the Chinese document:
[005-proto-io-system.zh-CN.md](./005-proto-io-system.zh-CN.md)
