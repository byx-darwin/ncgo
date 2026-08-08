package template

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/byx-darwin/ncgo/internal/testutil/golden"
)

// writeFixture writes a deterministic source file under root. Every byte is
// controlled by the test so the exported template tree snapshots are stable
// across machines and runs.
func writeFixture(t *testing.T, root, rel, content string) {
	t.Helper()
	abs := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(abs), err)
	}
	if err := os.WriteFile(abs, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", rel, err)
	}
}

// TestExportGoldenHertz locks the whole <root>/template tree exported from a
// minimal Hertz project so future export changes must explicitly bless the
// diff via -update-golden. The fixture exercises the {{.Module}},
// {{.ServiceName}} and {{ToLower .ServiceName}} substitutions (import paths,
// handler package, IDL file name and go_package).
func TestExportGoldenHertz(t *testing.T) {
	root := t.TempDir()
	writeFixture(t, root, "main.go", `package main

import (
	"github.com/acme/golden/internal/handler/userapi"
)

func main() {
	_ = userapi.Ping()
}
`)
	writeFixture(t, root, "conf/dev/conf.yaml", "server:\n  host: localhost\n  port: 8888\n")
	writeFixture(t, root, "internal/base/conf/conf.go", `package conf

type Config struct {
	Port int
}
`)
	writeFixture(t, root, "internal/base/server/server.go", "package server\n\nfunc Run() {}\n")
	writeFixture(t, root, "internal/handler/userapi/handler.go", `package userapi

type UserApiImpl struct{}

func (u *UserApiImpl) Ping() {}
`)
	writeFixture(t, root, "idl/app/userapi.proto", `syntax = "proto3";
package app;

option go_package = "github.com/acme/golden/kitex_gen/userapi";

service UserApi {
  rpc Ping(PingReq) returns (PingResp);
}

message PingReq {}
message PingResp {}
`)
	if _, err := Export(ExportOptions{Root: root, Kind: "hertz",
		Module: "github.com/acme/golden", ServiceName: "UserApi"}); err != nil {
		t.Fatalf("export: %v", err)
	}
	golden.Tree(t, filepath.Join("golden", "export-hertz"), filepath.Join(root, "template"))
}

// TestExportGoldenKitex is the Kitex twin of TestExportGoldenHertz. It also
// covers the middleware/release base dirs unique to Kitex exports and the
// idl/<service>.proto default path.
func TestExportGoldenKitex(t *testing.T) {
	root := t.TempDir()
	writeFixture(t, root, "main.go", `package main

import (
	"github.com/acme/golden/internal/handler/userrpc"
)

func main() {
	_ = userrpc.Ping()
}
`)
	writeFixture(t, root, "conf/dev/conf.yaml", "server:\n  host: localhost\n  port: 8888\n")
	writeFixture(t, root, "internal/base/conf/conf.go", `package conf

type Config struct {
	Port int
}
`)
	writeFixture(t, root, "internal/base/server/server.go", "package server\n\nfunc Run() {}\n")
	writeFixture(t, root, "internal/handler/userrpc/handler.go", `package userrpc

type UserRpcImpl struct{}

func (u *UserRpcImpl) Ping() {}
`)
	writeFixture(t, root, "internal/base/middleware/recovery.go", "package middleware\n\nfunc Recovery() {}\n")
	writeFixture(t, root, "internal/base/release/canary.go", "package release\n\nfunc Canary() {}\n")
	writeFixture(t, root, "idl/userrpc.proto", `syntax = "proto3";
package userrpc;

option go_package = "github.com/acme/golden/kitex_gen/userrpc";

service UserRpc {
  rpc Ping(PingReq) returns (PingResp);
}

message PingReq {}
message PingResp {}
`)
	if _, err := Export(ExportOptions{Root: root, Kind: "kitex",
		Module: "github.com/acme/golden", ServiceName: "UserRpc"}); err != nil {
		t.Fatalf("export: %v", err)
	}
	golden.Tree(t, filepath.Join("golden", "export-kitex"), filepath.Join(root, "template"))
}
