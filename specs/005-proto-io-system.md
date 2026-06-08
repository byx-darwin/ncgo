# ncgo Proto I/O Validation System Design

- Status: v1
- Scope: `ncgo protolint` command and MCP tool
- Source: Merged from `docs/proto-io-*.zh-CN.md` (4 docs)

## 1. Overview

Proto I/O is ncgo's built-in protobuf file validation system, exposed through the `ncgo protolint` CLI command and `ncgo_protolint` MCP tool.

Core capabilities:
- Run structured lint rules on `.proto` files
- Support text / json / SARIF output formats
- Support rule filtering, file filtering, and ignore patterns
- Proto parsing powered by `bufbuild/protocompile`

## 2. Rule System

### 2.1 Rule IDs

Rules use the `PIO` prefix (Proto I/O), divided into two phases:

**Phase 1 (Error level):**
- `PIO201` — Dynamic I/O validation (`dynamic_io` annotation)
- `PIO202` — HTTP Body binding validation
- `PIO203` — HTTP Methods validation
- `PIO204` — Kitex Envelope validation
- `PIO205` — Multi-binding validation
- `PIO206` — OpenAPI missing validation
- `PIO207` — Pagination missing validation
- `PIO208` — Path Params validation
- `PIO209` — PGV Constraints validation
- `PIO210` — PGV Pagination validation
- `PIO211` — Request/Response validation
- `PIO212` — Response Bindings validation
- `PIO213` — Unbound Fields validation
- `PIO214` — Universal Request validation
- `PIO215` — Raw Body validation

**Phase 2 (Warning level):**
- Advisory rules: field naming, comment completeness, etc.

### 2.2 Rule Implementation

Rules defined in `internal/protolint/rules.go` using a registry pattern:

```go
var rules = map[string]Rule{...}
```

Each rule implements `Check(ctx, file)` interface, returning `[]Diagnostic`.

## 3. Technical Design

### 3.1 Parser Layer

Uses `bufbuild/protocompile` as proto parser, supporting:
- Incremental parsing
- Import path resolution
- Source location preservation (for diagnostic positioning)

### 3.2 Diagnostic Model

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

### 3.3 SARIF Output

Follows SARIF 2.1.0 standard; `internal/protolint/sarif.go` handles serialization.

## 4. CLI / MCP Interface

### CLI Commands

```bash
# Check all proto files
ncgo protolint --root .

# Check specific files
ncgo protolint --root . --file idl/app/user.proto

# Specify rules
ncgo protolint --root . --rule PIO201 --rule PIO202

# Ignore rules
ncgo protolint --root . --ignore-rule PIO207

# Output format
ncgo protolint --root . --output json
ncgo protolint --root . --output sarif
```

### MCP Tools

| Tool | Description |
|------|-------------|
| `ncgo_protolint` | Check proto files, supports text/json/sarif output |

Parameters: `root` (required), `files`, `rules`, `ignoreRules`, `ignoreFiles`, `output`

## 5. Validation Strategy

### 5.1 Ignore Mechanism

- `--ignore-rule`: Skip specified rules
- `--ignore-file`: Skip specified files
- Warnings are non-blocking by default (Phase 2 rules)

### 5.2 Workspace Support

In micro workspaces, `protolint` recursively scans IDL directories of all services.

## 6. Test Data

Test cases in `internal/protolint/testdata/`, each subdirectory contains:
- Proto files triggering specific rules
- Expected diagnostic output

## 7. Related Files

- `internal/protolint/rules.go` — Rule registry
- `internal/protolint/sarif.go` — SARIF serialization
- `internal/protolint/load.go` — Proto file loading
- `schemas/` — Related JSON Schemas

## 8. Detailed Reference

Original design docs archived in `specs/archive/`:
- `proto-io-validation.zh-CN.md` — Validation strategy
- `proto-io-lint-rules.zh-CN.md` — Detailed rule list
- `proto-io-implementation.zh-CN.md` — Implementation task breakdown
- `proto-io-tech-design.zh-CN.md` — Technical design decisions
