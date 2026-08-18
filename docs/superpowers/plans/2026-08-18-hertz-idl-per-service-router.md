# Hertz + IDL per-service router generation fix — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make `ParseAllServices` correctly parse Hertz service protos under `idl/app/` so the `loop_service` templates generate `internal/router/pb/<service>.go` (and the per-service handler), fixing the `pb.Register` undefined build failure.

**Architecture:** The defect is in `internal/scaffold/template/proto.go`: `ParseAllServices` passes an **absolute** proto path to `protocompile.Compiler.Compile()` while `SourceResolver.ImportPaths` is set, causing a path double-join that fails for every proto. Fix by computing protoc-style import roots (the `idl/` ancestor + the proto's own dir) and compiling with a path **relative to the root**. Also stop silently discarding the parse error at `internal/scaffold/mono/mono.go`.

**Tech Stack:** Go 1.25+, `github.com/bufbuild/protocompile`, standard `testing`.

**Spec:** `docs/superpowers/specs/2026-08-18-hertz-idl-per-service-router-design.md`

## Global Constraints

- Keep changes inside the smallest relevant package; no cross-package moves.
- Contract-sensitive surfaces (CLI flags/MCP schemas/scaffold template files/JSON schemas) unchanged — this is an internal generator fix. Intended behavior change: re-activating the `loop_service` templates means the default hertz flow now emits `internal/usecase/<svc>/usecase.go` (always) and `internal/repository/<svc>repo/repo.go` (with `--with-database`), plus the template-package flow's per-service router/handler.
- Golden tests run with `NoGenerate: true`; this path is not exercised there — expect zero golden diff (verify).
- Preserve existing error wording where tests rely on it; the `compile proto: %w` / `no files compiled` messages stay.
- gofmt-clean; `go vet ./...` clean.

## File Structure

- `internal/scaffold/template/proto.go` — modify `ParseAllServices`; add two unexported helpers (`importRootsAndTarget`, `findIDLRoot`). One responsibility: proto → `[]ServiceInfo`.
- `internal/scaffold/template/proto_test.go` — add `TestParseAllServices_ImportResolution` (root-cause regression) covering: hertz `idl/app/<svc>.proto` importing `api.proto`, a no-import case, and a Kitex-style `idl/<svc>.proto`.
- `internal/scaffold/mono/mono.go` — change the discarded-error call site to a non-fatal stderr warning.

---

### Task 1: Fix `ParseAllServices` import-root resolution + regression test

**Files:**
- Modify: `internal/scaffold/template/proto.go:14-65` (`ParseAllServices`) and add helpers
- Test: `internal/scaffold/template/proto_test.go`

**Interfaces:**
- Consumes: `protocompile.Compiler`, `protocompile.SourceResolver`, `protocompile.WithStandardImports` (already imported).
- Produces: `ParseAllServices(ctx context.Context, protoPath string, module string) ([]ServiceInfo, error)` — signature unchanged. New unexported helpers `importRootsAndTarget(abs string) (roots []string, target string)` and `findIDLRoot(abs string) string`.

- [ ] **Step 1: Write the failing test**

Add to `internal/scaffold/template/proto_test.go` (add imports `context`, `os`, `path/filepath`):

```go
func writeProto(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestParseAllServices_ImportResolution(t *testing.T) {
	dir := t.TempDir()
	idl := filepath.Join(dir, "idl")

	// api.proto lives at the idl/ root (matches `hz -I idl`).
	writeProto(t, filepath.Join(idl, "api.proto"), `syntax = "proto3";
package api;
import "google/protobuf/descriptor.proto";
extend google.protobuf.MethodOptions { optional string get = 50001; }
`)
	// Hertz service proto under idl/app/, importing the root-level api.proto.
	writeProto(t, filepath.Join(idl, "app", "demo.proto"), `syntax = "proto3";
package app;
option go_package = "example.com/demo/internal/pb;pb";
import "api.proto";
service DemoService {
  rpc Ping(PingReq) returns (PingResp) {}
}
message PingReq { string message = 1; }
message PingResp { string message = 1; }
`)
	// Kitex-style proto directly under idl/, no imports.
	writeProto(t, filepath.Join(idl, "userrpc.proto"), `syntax = "proto3";
package userrpc;
option go_package = "example.com/demo/kitex_gen/userrpc;userrpc";
service UserRPC {
  rpc Get(GetReq) returns (GetResp) {}
}
message GetReq { string id = 1; }
message GetResp { string id = 1; }
`)

	t.Run("hertz idl/app with import", func(t *testing.T) {
		svcs, err := ParseAllServices(context.Background(), filepath.Join(idl, "app", "demo.proto"), "example.com/demo")
		if err != nil {
			t.Fatalf("ParseAllServices error: %v", err)
		}
		if len(svcs) != 1 || svcs[0].ServiceName != "DemoService" {
			t.Fatalf("got %+v, want 1 service DemoService", svcs)
		}
		if len(svcs[0].Methods) != 1 || svcs[0].Methods[0].Name != "Ping" {
			t.Errorf("methods = %+v, want [Ping]", svcs[0].Methods)
		}
	})

	t.Run("kitex idl root no import", func(t *testing.T) {
		svcs, err := ParseAllServices(context.Background(), filepath.Join(idl, "userrpc.proto"), "example.com/demo")
		if err != nil {
			t.Fatalf("ParseAllServices error: %v", err)
		}
		if len(svcs) != 1 || svcs[0].ServiceName != "UserRPC" {
			t.Fatalf("got %+v, want 1 service UserRPC", svcs)
		}
	})
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/scaffold/template/ -run TestParseAllServices_ImportResolution -v -count=1`
Expected: FAIL — `compile proto: open .../idl/app/.../idl/app/demo.proto: no such file or directory` (path double-join).

- [ ] **Step 3: Write minimal implementation**

In `internal/scaffold/template/proto.go`, replace the resolver/compile section of `ParseAllServices` (currently lines ~15-29) so it uses relative-to-root compilation:

```go
	abs, err := filepath.Abs(protoPath)
	if err != nil {
		return nil, fmt.Errorf("resolve proto path: %w", err)
	}
	roots, target := importRootsAndTarget(abs)

	compiler := protocompile.Compiler{
		Resolver: protocompile.WithStandardImports(&protocompile.SourceResolver{
			ImportPaths: roots,
		}),
	}
	files, err := compiler.Compile(ctx, target)
	if err != nil {
		return nil, fmt.Errorf("compile proto: %w", err)
	}
```

Add the helpers (below `ParseServiceInfo`, before `defaultServiceName`):

```go
// importRootsAndTarget derives protoc-style import roots and the compile
// target (relative to the first root) for a proto file. Hertz service protos
// live at <root>/idl/app/<svc>.proto and import files rooted at idl/
// (matching `hz -I idl`), so the idl/ ancestor must be an import root. The
// proto's own directory is added for sibling imports; Kitex protos at
// <root>/idl/<svc>.proto resolve via the idl/ root too. When there is no idl/
// ancestor, compile relative to the proto's own directory.
func importRootsAndTarget(abs string) (roots []string, target string) {
	protoDir := filepath.Dir(abs)
	if idlRoot := findIDLRoot(abs); idlRoot != "" {
		if rel, err := filepath.Rel(idlRoot, abs); err == nil {
			roots = []string{idlRoot}
			if protoDir != idlRoot {
				roots = append(roots, protoDir)
			}
			return roots, filepath.ToSlash(rel)
		}
	}
	return []string{protoDir}, filepath.Base(abs)
}

// findIDLRoot returns the nearest ancestor directory named "idl", or "" if
// none exists.
func findIDLRoot(abs string) string {
	dir := filepath.Dir(abs)
	for {
		if filepath.Base(dir) == "idl" {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}
```

Note: the `dir := filepath.Dir(abs)` line that previously fed `ImportPaths` is removed (replaced by `importRootsAndTarget`).

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/scaffold/template/ -run TestParseAllServices_ImportResolution -v -count=1`
Expected: PASS (both subtests).

- [ ] **Step 5: Run the package's full test suite**

Run: `go test ./internal/scaffold/template/... -count=1`
Expected: PASS (no regression in export/apply/golden tests).

- [ ] **Step 6: Commit**

```bash
git add internal/scaffold/template/proto.go internal/scaffold/template/proto_test.go
git commit -m "fix(scaffold): resolve idl/ import root in ParseAllServices (#70)"
```

---

### Task 2: Surface parse failures instead of silently discarding them

**Files:**
- Modify: `internal/scaffold/mono/mono.go:198` (the `services, _ :=` call site)

**Interfaces:**
- Consumes: `ParseAllServices` (Task 1). `os` and `fmt` are already imported in `mono.go`.
- Produces: no signature change; adds a non-fatal stderr warning on parse failure.

- [ ] **Step 1: Apply the change**

Replace at `internal/scaffold/mono/mono.go:198`:

```go
		services, _ := scaffoldtemplate.ParseAllServices(ctx, filepath.Join(dir, idl), opts.Module)
```

with:

```go
		services, err := scaffoldtemplate.ParseAllServices(ctx, filepath.Join(dir, idl), opts.Module)
		if err != nil {
			fmt.Fprintf(os.Stderr, "ncgo: warning: parse services from %s: %v\n", idl, err)
		}
```

Rationale: keep the flow tolerant (template packages may legitimately lack a parseable proto) while making silent regressions diagnosable. Do NOT return the error — that would regress those flows.

- [ ] **Step 2: Verify build + vet**

Run: `go build ./... && go vet ./internal/scaffold/...`
Expected: no errors, no shadowed-`err` vet complaint (the new `err` is scoped to the `if` guard's block).

- [ ] **Step 3: Run mono package tests**

Run: `go test ./internal/scaffold/mono/... -count=1`
Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add internal/scaffold/mono/mono.go
git commit -m "fix(scaffold): warn instead of discarding ParseAllServices error (#70)"
```

---

### Task 3: Full validation + design doc/plan inclusion

**Files:** none (validation only); the design doc and this plan are committed with the branch.

- [ ] **Step 1: Confirm golden tests are unchanged**

Run: `go test ./internal/scaffold/mono/... -run TestGenerateGolden -count=1`
Expected: PASS with no golden diff (this path is `NoGenerate`-gated).

- [ ] **Step 2: Repository validation**

Run:
```bash
go build ./... && go build . && go vet ./... && go test ./... -count=1
```
Expected: all PASS.

- [ ] **Step 3: Smoke test**

Run: `./scripts/smoke.sh`
Expected: PASS.

- [ ] **Step 4: Commit docs (if not already on the branch)**

```bash
git add docs/superpowers/specs/2026-08-18-hertz-idl-per-service-router-design.md docs/superpowers/plans/2026-08-18-hertz-idl-per-service-router.md
git commit -m "docs: add design + plan for hertz idl per-service router fix (#70)"
```

---

## Self-Review

**Spec coverage:**
- Root-cause fix (path double-join + idl/ import root) → Task 1.
- Non-fatal warning at mono.go:198 → Task 2.
- Regression test `TestParseAllServices_ImportResolution` (hertz import + kitex no-import) → Task 1.
- Golden/no-CLI-change claims → verified in Task 3 (golden path is `NoGenerate`-gated).
- Acceptance criteria: per-service router/handler generation is unblocked by populating `opts.Services`; end-to-end `go build` of a generated project depends on the external `ratelimit-hertz` package and is out of in-repo scope (asserted at the `ParseAllServices` level per the design).

**Placeholder scan:** none — all steps carry concrete code and exact run commands.

**Type consistency:** `importRootsAndTarget` returns `(roots []string, target string)`; consumed in `ParseAllServices` as `roots, target`. `findIDLRoot(abs string) string` used only inside `importRootsAndTarget`. `ParseAllServices` signature unchanged.
