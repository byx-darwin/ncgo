package domain

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/byx-darwin/ncgo/internal/manifest"
	planpkg "github.com/byx-darwin/ncgo/internal/scaffold/plan"
)

const testModule = "github.com/x/demo"

func seedManifest(t *testing.T, domains []string) string {
	t.Helper()
	root := t.TempDir()
	m := &manifest.Manifest{
		Ncgo:   manifest.Meta{Version: "0.0.0-test", AssetsVersion: "test"},
		Mode:   manifest.ModeMono,
		Module: testModule,
		Service: manifest.Service{
			Name: "demo", Kind: manifest.KindHertz,
			IDL: "idl/app/demo.proto",
		},
		Domains:     append([]string(nil), domains...),
		GeneratedAt: time.Date(2026, 4, 29, 0, 0, 0, 0, time.UTC),
	}
	if err := manifest.Save(root, m); err != nil {
		t.Fatalf("seed manifest: %v", err)
	}
	return root
}

func TestAddHappyPath(t *testing.T) {
	root := seedManifest(t, nil)
	res, err := Add(Options{Root: root, Name: "device"})
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	if !res.Updated {
		t.Errorf("expected manifest update on first add")
	}
	wantFiles := []string{
		filepath.Join(root, "internal", "usecase", "device", "device.go"),
		filepath.Join(root, "internal", "repository", "device", "device.go"),
		filepath.Join(root, "internal", "base", "data", "device_register.go"),
	}
	if len(res.WrittenPaths) != len(wantFiles) {
		t.Fatalf("WrittenPaths = %v, want %v", res.WrittenPaths, wantFiles)
	}
	for _, p := range wantFiles {
		if _, err := os.Stat(p); err != nil {
			t.Errorf("file not written: %s: %v", p, err)
		}
	}
	uc, _ := os.ReadFile(wantFiles[0])
	for _, want := range []string{
		"package device",
		"ncgo:domain=device kind=usecase",
		"ncgo:methods:start",
		"\"" + testModule + "/internal/repository/device\"",
		"type UseCase struct {",
	} {
		if !strings.Contains(string(uc), want) {
			t.Errorf("usecase missing %q\n--- file ---\n%s", want, uc)
		}
	}
	repo, _ := os.ReadFile(wantFiles[1])
	for _, want := range []string{
		"package devicerepo",
		"type Repository interface",
		"NewStub()",
		"var _ Repository = (*Stub)(nil)",
	} {
		if !strings.Contains(string(repo), want) {
			t.Errorf("repository missing %q\n--- file ---\n%s", want, repo)
		}
	}
	reg, _ := os.ReadFile(wantFiles[2])
	for _, want := range []string{
		"package data",
		"func RegisterDevice(injector *do.Injector)",
		"\"" + testModule + "/internal/repository/device\"",
		"\"" + testModule + "/internal/usecase/device\"",
	} {
		if !strings.Contains(string(reg), want) {
			t.Errorf("register missing %q\n--- file ---\n%s", want, reg)
		}
	}
	m, err := manifest.Load(root)
	if err != nil {
		t.Fatalf("reload manifest: %v", err)
	}
	if got := m.Domains; len(got) != 1 || got[0] != "device" {
		t.Errorf("manifest.Domains = %v, want [device]", got)
	}
}

func TestAddDryRunPlansWithoutWriting(t *testing.T) {
	root := seedManifest(t, nil)
	res, err := Add(Options{Root: root, Name: "device", DryRun: true})
	if err != nil {
		t.Fatalf("Add dry-run: %v", err)
	}
	if !res.DryRun || !res.Updated {
		t.Fatalf("DryRun/Updated = %v/%v, want true/true", res.DryRun, res.Updated)
	}
	wantPath := filepath.Join(root, "internal", "usecase", "device", "device.go")
	if _, err := os.Stat(wantPath); !os.IsNotExist(err) {
		t.Fatalf("dry-run wrote usecase file: stat err = %v", err)
	}
	m, err := manifest.Load(root)
	if err != nil {
		t.Fatalf("reload manifest: %v", err)
	}
	if len(m.Domains) != 0 {
		t.Fatalf("dry-run updated manifest domains = %v, want empty", m.Domains)
	}
	if !planContains(res.Plan, "file", "create") || !planContains(res.Plan, "manifest", "add") || !planContains(res.Plan, "next_step", "run") {
		t.Fatalf("plan missing expected items: %+v", res.Plan)
	}
}

func TestAddRefusesOverwriteWithoutForce(t *testing.T) {
	root := seedManifest(t, nil)
	if _, err := Add(Options{Root: root, Name: "device"}); err != nil {
		t.Fatalf("first Add: %v", err)
	}
	_, err := Add(Options{Root: root, Name: "device"})
	if err == nil {
		t.Fatalf("expected refusal on second add without --force")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestAddForceOverwrites(t *testing.T) {
	root := seedManifest(t, nil)
	if _, err := Add(Options{Root: root, Name: "device"}); err != nil {
		t.Fatalf("first Add: %v", err)
	}
	res, err := Add(Options{Root: root, Name: "device", Force: true})
	if err != nil {
		t.Fatalf("Add with --force: %v", err)
	}
	if res.Updated {
		t.Errorf("manifest should be unchanged on second add (already listed)")
	}
}

func TestAddDedupsManifest(t *testing.T) {
	root := seedManifest(t, []string{"theme"})
	res, err := Add(Options{Root: root, Name: "device"})
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	if !res.Updated {
		t.Errorf("expected update when adding new domain")
	}
	m, _ := manifest.Load(root)
	want := []string{"device", "theme"}
	if len(m.Domains) != len(want) || m.Domains[0] != want[0] || m.Domains[1] != want[1] {
		t.Errorf("manifest.Domains = %v, want %v (sorted)", m.Domains, want)
	}
}

func TestAddRejectsBadNames(t *testing.T) {
	root := seedManifest(t, nil)
	for _, name := range []string{"", "Device", "device-name", "1device", strings.Repeat("a", 64)} {
		if _, err := Add(Options{Root: root, Name: name}); err == nil {
			t.Errorf("expected error for invalid name %q", name)
		}
	}
}

func TestAddRequiresManifest(t *testing.T) {
	root := t.TempDir()
	if _, err := Add(Options{Root: root, Name: "device"}); err == nil {
		t.Errorf("expected error when manifest is missing")
	}
}

func TestExportName(t *testing.T) {
	cases := map[string]string{
		"device":       "Device",
		"user_profile": "UserProfile",
		"a":            "A",
		"a_b_c":        "ABC",
	}
	for in, want := range cases {
		if got := exportName(in); got != want {
			t.Errorf("exportName(%q) = %q, want %q", in, got, want)
		}
	}
}

func planContains(items []planpkg.Item, kind, action string) bool {
	for _, item := range items {
		if item.Kind == kind && item.Action == action {
			return true
		}
	}
	return false
}
