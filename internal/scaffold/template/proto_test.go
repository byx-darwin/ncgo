package template

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultServiceName(t *testing.T) {
	tests := []struct {
		path, want string
	}{
		{"idl/app/user-api.proto", "UserApi"},
		{"idl/userrpc.proto", "Userrpc"},
		{"idl/svc.proto", "Svc"},
	}
	for _, tt := range tests {
		if got := defaultServiceName(tt.path); got != tt.want {
			t.Errorf("defaultServiceName(%q) = %q, want %q", tt.path, got, tt.want)
		}
	}
}

func TestPkgRefName(t *testing.T) {
	tests := []struct {
		module, want string
	}{
		{"github.com/acme/user-api", "user-api"},
		{"github.com/acme/commerce/services/user-rpc", "user-rpc"},
	}
	for _, tt := range tests {
		if got := pkgRefName(tt.module); got != tt.want {
			t.Errorf("pkgRefName(%q) = %q, want %q", tt.module, got, tt.want)
		}
	}
}

// ParseAllServices tests require the protobuf compiler with standard imports
// available. The integration-level export/apply tests cover the full flow.

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
