package kitexclient

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/byx-darwin/ncgo/internal/exec"
)

// fakeRunner is an exec.Runner that records invocations and returns canned
// output. It allows tests to verify that kitex and go mod tidy are called
// with the expected arguments without requiring the binaries on PATH.
type fakeRunner struct {
	calls []exec.Cmd
}

func (f *fakeRunner) Run(_ context.Context, c exec.Cmd) (exec.Result, error) {
	f.calls = append(f.calls, c)
	return exec.Result{}, nil
}

// seedWorkspace creates a temp directory with go.mod and a proto file.
func seedWorkspace(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/demo\n\ngo 1.25\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	idlDir := filepath.Join(root, "idl")
	if err := os.MkdirAll(idlDir, 0o755); err != nil {
		t.Fatal(err)
	}
	proto := `syntax = "proto3";
package rbac;
option go_package = "example.com/demo/kitex_gen/rbac;rbac";

service RbacService {
  rpc CheckPermission(CheckPermissionReq) returns (CheckPermissionResp) {}
  rpc ListRoles(ListRolesReq) returns (ListRolesResp) {}
}
message CheckPermissionReq { string user_id = 1; }
message CheckPermissionResp { bool allowed = 1; }
message ListRolesReq {}
message ListRolesResp { repeated string roles = 1; }
`
	if err := os.WriteFile(filepath.Join(idlDir, "rbac.proto"), []byte(proto), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestAddGeneratesCompleteClient(t *testing.T) {
	root := seedWorkspace(t)
	r := &fakeRunner{}

	res, err := Add(context.Background(), Options{
		Root:    root,
		Name:    "rbac",
		Service: "RbacService",
		IDL:     "idl/rbac.proto",
		Module:  "example.com/demo",
		Runner:  r,
	})
	if err != nil {
		t.Fatalf("Add: %v", err)
	}

	// kitex should have been called.
	if !hasCall(r, "kitex") {
		t.Fatalf("kitex not called; calls = %+v", r.calls)
	}
	// go mod tidy should have been called.
	if !hasCall(r, "go") {
		t.Fatalf("go not called; calls = %+v", r.calls)
	}

	// Verify kitex_gen/ is in written paths.
	if !containsStr(res.WrittenPaths, "kitex_gen/") {
		t.Fatalf("WrittenPaths missing kitex_gen/: %v", res.WrittenPaths)
	}

	// Read the generated client.go.
	clientPath := filepath.Join(root, "pkg", "client", "rbac", "client.go")
	content, err := os.ReadFile(clientPath)
	if err != nil {
		t.Fatalf("read client.go: %v", err)
	}
	s := string(content)

	// The generated code must contain actual Kitex client usage, not TODOs.
	if strings.Contains(s, "TODO") {
		t.Fatalf("client.go still contains TODOs:\n%s", s)
	}

	// Must import kitex_gen package.
	wantImport := "example.com/demo/kitex_gen/rbac"
	if !strings.Contains(s, wantImport) {
		t.Fatalf("client.go missing import %q:\n%s", wantImport, s)
	}

	// Must import kitex client.
	if !strings.Contains(s, "github.com/cloudwego/kitex/client") {
		t.Fatalf("client.go missing kitex/client import:\n%s", s)
	}

	// Must have the Client struct backed by the service sub-package.
	if !strings.Contains(s, "rbacservice.Client") {
		t.Fatalf("client.go missing rbacservice.Client field (service sub-package):\n%s", s)
	}
	// Must import the service sub-package.
	if !strings.Contains(s, "example.com/demo/kitex_gen/rbac/rbacservice") {
		t.Fatalf("client.go missing service sub-package import:\n%s", s)
	}

	// Must proxy both RPC methods.
	if !strings.Contains(s, "CheckPermission") {
		t.Fatalf("client.go missing CheckPermission method:\n%s", s)
	}
	if !strings.Contains(s, "ListRoles") {
		t.Fatalf("client.go missing ListRoles method:\n%s", s)
	}

	// Must have proper request/response types from the top-level kitex_gen pkg.
	if !strings.Contains(s, "*rbac.CheckPermissionReq") {
		t.Fatalf("client.go missing *rbac.CheckPermissionReq:\n%s", s)
	}
	if !strings.Contains(s, "*rbac.CheckPermissionResp") {
		t.Fatalf("client.go missing *rbac.CheckPermissionResp:\n%s", s)
	}

	// Verify config.go also exists.
	configPath := filepath.Join(root, "pkg", "client", "rbac", "config.go")
	if _, err := os.Stat(configPath); err != nil {
		t.Fatalf("config.go not created: %v", err)
	}
}

func TestAddAutoDetectsModuleFromGoMod(t *testing.T) {
	root := seedWorkspace(t)
	r := &fakeRunner{}

	_, err := Add(context.Background(), Options{
		Root:    root,
		Name:    "rbac",
		Service: "RbacService",
		IDL:     "idl/rbac.proto",
		// Module intentionally empty — should be detected from go.mod
		Runner: r,
	})
	if err != nil {
		t.Fatalf("Add: %v", err)
	}

	// Verify kitex was called with the detected module.
	for _, c := range r.calls {
		if c.Name == "kitex" {
			if !containsStr(c.Args, "example.com/demo") {
				t.Fatalf("kitex args missing module: %v", c.Args)
			}
			return
		}
	}
	t.Fatalf("kitex not called")
}

func TestAddDryRunSkipsKitexAndGoModTidy(t *testing.T) {
	root := seedWorkspace(t)
	r := &fakeRunner{}

	res, err := Add(context.Background(), Options{
		Root:    root,
		Name:    "rbac",
		Service: "RbacService",
		IDL:     "idl/rbac.proto",
		Module:  "example.com/demo",
		DryRun:  true,
		Runner:  r,
	})
	if err != nil {
		t.Fatalf("Add: %v", err)
	}

	// No commands should have been executed.
	if len(r.calls) != 0 {
		t.Fatalf("dry-run should not execute commands; got %d calls: %+v", len(r.calls), r.calls)
	}

	// But result should still list what would be written.
	if len(res.WrittenPaths) == 0 {
		t.Fatalf("dry-run should list planned paths; got %v", res.WrittenPaths)
	}
}

func TestAddFailsWhenServiceNotFound(t *testing.T) {
	root := seedWorkspace(t)
	r := &fakeRunner{}

	// The proto has only one service (RbacService). When there are multiple
	// services, a mismatch produces an error. We test with a proto that has
	// the service but request a non-matching name in a multi-service proto.
	// With a single-service proto, the fallback always matches — so we can
	// only test the "not found" path indirectly: request a service name
	// that doesn't exist in a two-service proto.
	//
	// Since our seed proto only has RbacService, the fallback will match.
	// We test the error path by directly calling findService.
	_, err := findService([]protoServiceInfo{
		{ServiceName: "SvcA"},
		{ServiceName: "SvcB"},
	}, "SvcC")
	if err == nil || !strings.Contains(err.Error(), "not found in proto") {
		t.Fatalf("expected 'not found in proto' error, got: %v", err)
	}

	// Single-service fallback: even with a different name, the service matches.
	_, err = Add(context.Background(), Options{
		Root:    root,
		Name:    "rbac",
		Service: "NonExistentService",
		IDL:     "idl/rbac.proto",
		Module:  "example.com/demo",
		Runner:  r,
	})
	if err != nil {
		t.Fatalf("single-service proto should fallback-match any service name: %v", err)
	}
}

func TestAddKitexCommandArgs(t *testing.T) {
	root := seedWorkspace(t)
	r := &fakeRunner{}

	_, err := Add(context.Background(), Options{
		Root:    root,
		Name:    "rbac",
		Service: "RbacService",
		IDL:     "idl/rbac.proto",
		Module:  "example.com/demo",
		Runner:  r,
	})
	if err != nil {
		t.Fatalf("Add: %v", err)
	}

	// Verify kitex was called with correct args.
	for _, c := range r.calls {
		if c.Name == "kitex" {
			wantArgs := []string{"-module", "example.com/demo", "-type", "protobuf", "idl/rbac.proto"}
			if len(c.Args) != len(wantArgs) {
				t.Fatalf("kitex args = %v, want %v", c.Args, wantArgs)
			}
			for i, a := range wantArgs {
				if c.Args[i] != a {
					t.Fatalf("kitex args[%d] = %q, want %q (all: %v)", i, c.Args[i], a, c.Args)
				}
			}
			if c.Dir != root {
				t.Fatalf("kitex dir = %q, want %q", c.Dir, root)
			}
			return
		}
	}
	t.Fatalf("kitex not called")
}

func TestAddGoModTidyCalledAfterKitex(t *testing.T) {
	root := seedWorkspace(t)
	r := &fakeRunner{}

	_, err := Add(context.Background(), Options{
		Root:    root,
		Name:    "rbac",
		Service: "RbacService",
		IDL:     "idl/rbac.proto",
		Module:  "example.com/demo",
		Runner:  r,
	})
	if err != nil {
		t.Fatalf("Add: %v", err)
	}

	// Verify go mod tidy was called after kitex.
	kitexIdx, tidyIdx := -1, -1
	for i, c := range r.calls {
		if c.Name == "kitex" {
			kitexIdx = i
		}
		if c.Name == "go" && len(c.Args) >= 2 && c.Args[0] == "mod" && c.Args[1] == "tidy" {
			tidyIdx = i
		}
	}
	if kitexIdx < 0 {
		t.Fatalf("kitex not called")
	}
	if tidyIdx < 0 {
		t.Fatalf("go mod tidy not called")
	}
	if tidyIdx <= kitexIdx {
		t.Fatalf("go mod tidy (idx=%d) should be called after kitex (idx=%d)", tidyIdx, kitexIdx)
	}
}

func TestAddFailsWithoutModuleOrGoMod(t *testing.T) {
	root := t.TempDir() // no go.mod
	_, err := Add(context.Background(), Options{
		Root:    root,
		Name:    "rbac",
		Service: "RbacService",
		IDL:     "idl/rbac.proto",
	})
	if err == nil || !strings.Contains(err.Error(), "--module is required") {
		t.Fatalf("expected module error, got: %v", err)
	}
}

func TestAddSingleServiceMatchesByPackage(t *testing.T) {
	// When proto has one service and opts.Service matches the package,
	// it should still work.
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/demo\n\ngo 1.25\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	idlDir := filepath.Join(root, "idl")
	if err := os.MkdirAll(idlDir, 0o755); err != nil {
		t.Fatal(err)
	}
	proto := `syntax = "proto3";
package userrpc;
service UserRPC {
  rpc Get(GetReq) returns (GetResp) {}
}
message GetReq { string id = 1; }
message GetResp { string id = 1; }
`
	if err := os.WriteFile(filepath.Join(idlDir, "userrpc.proto"), []byte(proto), 0o644); err != nil {
		t.Fatal(err)
	}

	r := &fakeRunner{}
	// When there's only one service, any service name should match it.
	_, err := Add(context.Background(), Options{
		Root:    root,
		Name:    "user",
		Service: "anything", // doesn't match "UserRPC"
		IDL:     "idl/userrpc.proto",
		Module:  "example.com/demo",
		Runner:  r,
	})
	if err != nil {
		t.Fatalf("Add with single service: %v", err)
	}

	// Verify the generated code uses the correct proto package (userrpc).
	clientPath := filepath.Join(root, "pkg", "client", "user", "client.go")
	content, err := os.ReadFile(clientPath)
	if err != nil {
		t.Fatalf("read client.go: %v", err)
	}
	if !strings.Contains(string(content), "userrpc.Client") {
		t.Fatalf("client.go should reference userrpc.Client (from proto package):\n%s", string(content))
	}
	if !strings.Contains(string(content), "kitex_gen/userrpc") {
		t.Fatalf("client.go should import kitex_gen/userrpc:\n%s", string(content))
	}
}

// seedWorkspaceWithGoPackage creates a workspace whose proto uses a long
// go_package path (the micro-workspace scenario from issue #87).
func seedWorkspaceWithGoPackage(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module iproost/proxy/api-src/edge-bff\n\ngo 1.25\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	idlDir := filepath.Join(root, "idl")
	if err := os.MkdirAll(idlDir, 0o755); err != nil {
		t.Fatal(err)
	}
	proto := `syntax = "proto3";
package edgerpc;
option go_package = "iproost/proxy/api-src/edge-rpc/kitex_gen/edgerpc;edgerpc";

service DeviceService {
  rpc GetDevice(GetDeviceReq) returns (GetDeviceResp) {}
}
message GetDeviceReq { string id = 1; }
message GetDeviceResp { string name = 1; }
`
	if err := os.WriteFile(filepath.Join(idlDir, "edgerpc.proto"), []byte(proto), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

// TestAddMicroWorkspaceScenario covers the 4 bugs from issue #87 in one shot:
//   1. kitex_gen path uses last segment of go_package, not the full path
//   2. package name is sanitized (no hyphens)
//   3. import path matches where kitex_gen is actually written
//   4. Client interface is referenced from the service sub-package
func TestAddMicroWorkspaceScenario(t *testing.T) {
	root := seedWorkspaceWithGoPackage(t)
	r := &fakeRunner{}

	res, err := Add(context.Background(), Options{
		Root:    root,
		Name:    "edge-rpc", // hyphenated — must be sanitized
		Service: "DeviceService",
		IDL:     "idl/edgerpc.proto",
		Module:  "iproost/proxy/api-src/edge-bff",
		Runner:  r,
	})
	if err != nil {
		t.Fatalf("Add: %v", err)
	}

	clientPath := filepath.Join(root, "pkg", "client", "edge-rpc", "client.go")
	content, err := os.ReadFile(clientPath)
	if err != nil {
		t.Fatalf("read client.go: %v", err)
	}
	s := string(content)

	// Bug 2: package name must be a valid Go identifier (no hyphens).
	if strings.Contains(s, "package edge-rpc") {
		t.Fatalf("client.go has invalid package name (hyphens):\n%s", s)
	}
	if !strings.Contains(s, "package edgerpc") {
		t.Fatalf("client.go missing sanitized package name:\n%s", s)
	}

	// Bug 1 & 3: import path must use last segment of go_package path.
	// go_package = "iproost/.../kitex_gen/edgerpc;edgerpc" → kitex_gen/edgerpc
	wantImport := "iproost/proxy/api-src/edge-bff/kitex_gen/edgerpc"
	if !strings.Contains(s, wantImport) {
		t.Fatalf("client.go missing correct import %q:\n%s", wantImport, s)
	}
	// Must NOT contain the full go_package path as nested dir.
	badImport := "kitex_gen/iproost/proxy/api-src/edge-rpc/kitex_gen/edgerpc"
	if strings.Contains(s, badImport) {
		t.Fatalf("client.go has nested go_package path (bug #1):\n%s", s)
	}

	// Bug 4: Client interface must come from service sub-package.
	if !strings.Contains(s, "deviceservice.Client") {
		t.Fatalf("client.go should reference deviceservice.Client (service sub-package):\n%s", s)
	}
	if strings.Contains(s, "edgerpc.Client") {
		t.Fatalf("client.go references wrong edgerpc.Client (top-level, not service sub-pkg):\n%s", s)
	}

	// Service sub-package must be imported.
	if !strings.Contains(s, "kitex_gen/edgerpc/deviceservice") {
		t.Fatalf("client.go missing service sub-package import:\n%s", s)
	}

	// Sanity: still wrote the client and config.
	if !containsStr(res.WrittenPaths, clientPath) {
		t.Fatalf("WrittenPaths missing client.go: %v", res.WrittenPaths)
	}
	configPath := filepath.Join(root, "pkg", "client", "edge-rpc", "config.go")
	if _, err := os.Stat(configPath); err != nil {
		t.Fatalf("config.go not created: %v", err)
	}
}

// TestSanitizeGoPackageName verifies the name-sanitization helper.
func TestSanitizeGoPackageName(t *testing.T) {
	cases := map[string]string{
		"rbac":        "rbac",
		"edge-rpc":    "edgerpc",
		"rule_center": "rule_center",
		"foo-bar-baz": "foobarbaz",
		"123abc":      "123abc",
		"a-b-c":       "abc",
	}
	for in, want := range cases {
		if got := sanitizeGoPackageName(in); got != want {
			t.Errorf("sanitizeGoPackageName(%q) = %q, want %q", in, got, want)
		}
	}
}

// --- helpers ---

func hasCall(r *fakeRunner, name string) bool {
	for _, c := range r.calls {
		if c.Name == name {
			return true
		}
	}
	return false
}

func containsStr(ss []string, s string) bool {
	for _, v := range ss {
		if v == s {
			return true
		}
	}
	return false
}
