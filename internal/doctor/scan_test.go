package doctor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/byx-darwin/ncgo/internal/manifest"
)

// seedManifestForScan seeds a project root with the test module path the
// scanner tests use for handler→repo import matching.
func seedManifestForScan(t *testing.T) string {
	t.Helper()
	return seedProject(t, false, "")
}

func seedKitexManifestForScan(t *testing.T) string {
	t.Helper()
	root := seedProject(t, false, "")
	m := loadOrFail(t, root)
	m.Service.Kind = manifest.KindKitex
	m.Service.IDL = "idl/demo.proto"
	if err := manifest.Save(root, m); err != nil {
		t.Fatalf("save kitex manifest: %v", err)
	}
	return root
}

func loadOrFail(t *testing.T, root string) *manifest.Manifest {
	t.Helper()
	m, err := manifest.Load(root)
	if err != nil {
		t.Fatalf("load manifest: %v", err)
	}
	return m
}

func failingChecks(in []Check) []Check {
	var out []Check
	for _, c := range in {
		if !c.OK {
			out = append(out, c)
		}
	}
	return out
}

func hasCheckID(in []Check, id string) bool {
	for _, c := range in {
		if c.ID == id {
			return true
		}
	}
	return false
}

func writeGo(t *testing.T, dir, name, body string) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	full := filepath.Join(dir, name)
	if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
		t.Fatalf("write %s: %v", full, err)
	}
	return full
}

func TestScanHandlerNoRepoDetectsImport(t *testing.T) {
	root := seedManifestForScan(t)
	writeGo(t, filepath.Join(root, "internal", "handler", "device"), "handler.go", `
package device

import (
	_ "github.com/x/demo/internal/repository/device"
)

func Hello() {}
`)
	m := loadOrFail(t, root)
	checks := scanHandlerNoRepo(root, m)
	got := failingChecks(checks)
	if len(got) != 1 {
		t.Fatalf("expected 1 violation, got %d: %+v", len(got), got)
	}
	if !strings.Contains(got[0].Message, "internal/repository/device") {
		t.Errorf("message missing import path: %s", got[0].Message)
	}
	if got[0].File == "" || got[0].Line == 0 {
		t.Errorf("expected file/line populated: %+v", got[0])
	}
	if got[0].Rule != ruleAnchor {
		t.Errorf("Rule = %q, want %q", got[0].Rule, ruleAnchor)
	}
}

func TestScanHandlerNoDataDetectsImport(t *testing.T) {
	root := seedKitexManifestForScan(t)
	writeGo(t, filepath.Join(root, "internal", "handler", "demo"), "handler.go", `
	package demohandler

	import (
		_ "github.com/x/demo/internal/base/data"
	)
	`)
	got := failingChecks(scanHandlerNoData(root, loadOrFail(t, root)))
	if len(got) != 1 {
		t.Fatalf("expected 1 base/data violation, got %d: %+v", len(got), got)
	}
	if !strings.Contains(got[0].Message, "internal/base/data") {
		t.Errorf("message missing import path: %s", got[0].Message)
	}
}

func TestScanHandlerNoRepoCleanWhenAbsent(t *testing.T) {
	root := seedManifestForScan(t)
	writeGo(t, filepath.Join(root, "internal", "handler"), "ok.go", `
package handler

import "context"

func H(ctx context.Context) {}
`)
	checks := scanHandlerNoRepo(root, loadOrFail(t, root))
	if len(checks) != 1 || !checks[0].OK {
		t.Errorf("expected single OK check, got %+v", checks)
	}
}

func TestScanUsecaseNoHertzDetectsImport(t *testing.T) {
	root := seedManifestForScan(t)
	writeGo(t, filepath.Join(root, "internal", "usecase", "device"), "uc.go", `
package device

import (
	_ "github.com/cloudwego/hertz/pkg/app"
)
`)
	got := failingChecks(scanUsecaseNoHertz(root))
	if len(got) != 1 {
		t.Fatalf("expected 1 hertz violation, got %d: %+v", len(got), got)
	}
}

func TestScanUsecaseNoKitexDetectsImport(t *testing.T) {
	root := seedKitexManifestForScan(t)
	writeGo(t, filepath.Join(root, "internal", "usecase", "demo"), "uc.go", `
	package demo

	import (
		_ "github.com/cloudwego/kitex/server"
	)
	`)
	got := failingChecks(scanUsecaseNoKitex(root))
	if len(got) != 1 {
		t.Fatalf("expected 1 kitex violation, got %d: %+v", len(got), got)
	}
}

func TestScanUsecaseNoRequestContext(t *testing.T) {
	root := seedManifestForScan(t)
	writeGo(t, filepath.Join(root, "internal", "usecase", "device"), "uc.go", `
package device

var app = struct{ RequestContext int }{}

type UseCase struct{}
func (u *UseCase) Do() { _ = app.RequestContext }
`)
	got := failingChecks(scanUsecaseNoRequestContext(root))
	if len(got) == 0 {
		t.Fatalf("expected at least one selector violation")
	}
	for _, c := range got {
		if !strings.Contains(c.Message, "app.RequestContext") {
			t.Errorf("unexpected message: %s", c.Message)
		}
	}
}

func TestScanRepoNoRawSQL(t *testing.T) {
	root := seedManifestForScan(t)
	writeGo(t, filepath.Join(root, "internal", "repository", "device"), "repo.go", `
package device

const q = `+"`SELECT id FROM devices WHERE tenant_id = $1`"+`

const harmless = "this is just a message"
`)
	got := failingChecks(scanRepoNoRawSQL(root))
	if len(got) != 1 {
		t.Fatalf("expected exactly 1 SQL violation, got %d: %+v", len(got), got)
	}
	if !strings.Contains(got[0].Message, "SELECT") {
		t.Errorf("expected SELECT in message: %s", got[0].Message)
	}
}

func TestScanRepoNoUsecaseDetectsImport(t *testing.T) {
	root := seedKitexManifestForScan(t)
	writeGo(t, filepath.Join(root, "internal", "repository", "demo"), "repo.go", `
	package demorepo

	import (
		_ "github.com/x/demo/internal/usecase/demo"
	)
	`)
	got := failingChecks(scanRepoNoUsecase(root, loadOrFail(t, root)))
	if len(got) != 1 {
		t.Fatalf("expected 1 repo→usecase violation, got %d: %+v", len(got), got)
	}
	if !strings.Contains(got[0].Message, "internal/usecase/demo") {
		t.Errorf("message missing import path: %s", got[0].Message)
	}
}

func TestScanLayersKitexCleanProject(t *testing.T) {
	root := seedKitexManifestForScan(t)
	writeGo(t, filepath.Join(root, "internal", "handler", "demo"), "handler.go", `
	package demohandler

	import usecase "github.com/x/demo/internal/usecase/demo"

	type Handler struct{ uc *usecase.UseCase }
	`)
	writeGo(t, filepath.Join(root, "internal", "usecase", "demo"), "uc.go", `
	package demo

	type UseCase struct{}
	`)
	writeGo(t, filepath.Join(root, "internal", "repository", "demo"), "repo.go", `
	package demorepo

	import _ "github.com/x/demo/internal/db/gen"
	`)
	for _, c := range scanLayers(root, loadOrFail(t, root)) {
		if !c.OK {
			t.Errorf("expected clean kitex project, got violation: %+v", c)
		}
	}
}

func TestLooksLikeSQL(t *testing.T) {
	cases := map[string]bool{
		"SELECT id FROM users":            true,
		"  insert into users values ($1)": true,
		"UPDATE foo SET x=1":              true,
		"DELETE FROM foo":                 true,
		"WITH cte AS (SELECT 1) SELECT *": true,
		"hello world":                     false,
		"selected":                        false, // word-boundary-ish: needs trailing space/newline/tab
		"":                                false,
		"this is a regular error message": false,
	}
	for in, want := range cases {
		if got := looksLikeSQL(in); got != want {
			t.Errorf("looksLikeSQL(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestScanLayersAllAbsent(t *testing.T) {
	root := seedManifestForScan(t)
	checks := scanLayers(root, loadOrFail(t, root))
	for _, c := range checks {
		if !c.OK {
			t.Errorf("expected all skip-OK when no layers exist, got: %+v", c)
		}
	}
	wantIDs := []string{
		"layer.handler.no-repo",
		"layer.handler.no-data",
		"layer.usecase.no-hertz",
		"layer.usecase.no-kitex",
		"layer.usecase.no-request-context",
		"layer.repo.no-sql-string",
		"layer.repo.no-usecase",
	}
	for _, id := range wantIDs {
		if !hasCheckID(checks, id) {
			t.Errorf("missing rule %q in scan output", id)
		}
	}
}
