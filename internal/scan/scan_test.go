package scan

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/byx-darwin/ncgo/internal/manifest"
)

// seedScanProject writes a manifest plus one domain's usecase/repository dirs.
func seedScanProject(t *testing.T, domains []string) string {
	t.Helper()
	root := t.TempDir()
	if err := manifest.Save(root, &manifest.Manifest{
		Ncgo:    manifest.Meta{Version: "0.1.0-test", AssetsVersion: "test"},
		Mode:    manifest.ModeMono,
		Module:  "github.com/x/demo",
		Service: manifest.Service{Name: "demo", Kind: manifest.KindHertz},
		Domains: domains,
	}); err != nil {
		t.Fatalf("seed manifest: %v", err)
	}
	for _, d := range domains {
		usecase := filepath.Join(root, "internal", "usecase", d, d+".go")
		if err := os.MkdirAll(filepath.Dir(usecase), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		body := `package ` + d + `

type UseCase struct{}

func (u *UseCase) List() error { return nil }
func (u *UseCase) Repo() {}
// ncgo:methods:start
// ncgo:methods:end
`
		if err := os.WriteFile(usecase, []byte(body), 0o644); err != nil {
			t.Fatalf("write usecase: %v", err)
		}
	}
	return root
}

func TestScanReportsDomainsMethodsAnchors(t *testing.T) {
	root := seedScanProject(t, []string{"device", "order"})
	s, err := Scan(root)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(s.Domains) != 2 {
		t.Fatalf("domains = %d, want 2", len(s.Domains))
	}
	byName := map[string]Domain{}
	for _, d := range s.Domains {
		byName[d.Name] = d
	}
	dev := byName["device"]
	if !dev.ManifestListed || !dev.UsecaseExists || !dev.AnchorsOK {
		t.Fatalf("device domain = %+v, want all true", dev)
	}
	if len(dev.Methods) != 1 || dev.Methods[0].Name != "List" {
		t.Fatalf("device methods = %+v, want [List] only (Repo excluded)", dev.Methods)
	}
}

func TestScanFlagsMissingUsecase(t *testing.T) {
	root := seedScanProject(t, []string{"device", "ghost"})
	_ = os.RemoveAll(filepath.Join(root, "internal", "usecase", "ghost"))
	s, err := Scan(root)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if !hasIssue(s.Issues, IssueMissingUsecase) {
		t.Fatalf("issues = %+v, want missing_usecase", s.Issues)
	}
}

func TestScanFlagsUndeclaredDomain(t *testing.T) {
	root := seedScanProject(t, []string{"device"})
	if err := os.MkdirAll(filepath.Join(root, "internal", "usecase", "rogue"), 0o755); err != nil {
		t.Fatalf("mkdir rogue: %v", err)
	}
	s, err := Scan(root)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if !hasIssue(s.Issues, IssueUndeclaredDomain) {
		t.Fatalf("issues = %+v, want undeclared_domain", s.Issues)
	}
}

func TestScanFlagsBrokenAnchors(t *testing.T) {
	root := seedScanProject(t, []string{"device"})
	p := filepath.Join(root, "internal", "usecase", "device", "device.go")
	b, _ := os.ReadFile(p)
	_ = os.WriteFile(p, []byte(strings.ReplaceAll(string(b), "// ncgo:methods:start\n", "")), 0o644)
	s, err := Scan(root)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if !hasIssue(s.Issues, IssueAnchorMissing) {
		t.Fatalf("issues = %+v, want anchor_missing", s.Issues)
	}
}

func TestScanErrorsOnMissingManifest(t *testing.T) {
	root := t.TempDir()
	if _, err := Scan(root); err == nil {
		t.Fatal("Scan on non-project root should error")
	}
}

func hasIssue(issues []Issue, kind string) bool {
	for _, i := range issues {
		if i.Kind == kind {
			return true
		}
	}
	return false
}
