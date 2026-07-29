package extract

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/byx-darwin/ncgo/internal/manifest"
	"github.com/byx-darwin/ncgo/internal/scaffold/domain"
)

func seedProject(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := manifest.Save(root, &manifest.Manifest{
		Ncgo:   manifest.Meta{Version: "0.4.0-test", AssetsVersion: "test-assets"},
		Mode:   manifest.ModeMono,
		Module: "github.com/acme/demo",
		Service: manifest.Service{
			Name: "demo", Kind: manifest.KindHertz, IDL: "idl/app/demo.proto",
		},
		GeneratedAt: time.Date(2026, 4, 29, 8, 0, 0, 0, time.UTC),
	}); err != nil {
		t.Fatalf("seed manifest: %v", err)
	}
	if _, err := domain.Add(domain.Options{Root: root, Name: "device"}); err != nil {
		t.Fatalf("seed domain: %v", err)
	}
	return root
}

func TestPlanDomainDefaultTarget(t *testing.T) {
	root := seedProject(t)
	plan, err := PlanDomain(DomainOptions{Root: root, Name: "device"})
	if err != nil {
		t.Fatalf("PlanDomain: %v", err)
	}
	if plan.To != "services/device-rpc" {
		t.Errorf("To = %q", plan.To)
	}
	if plan.TargetModule != "github.com/acme/demo/services/device-rpc" {
		t.Errorf("TargetModule = %q", plan.TargetModule)
	}
	if len(plan.Sources) != 3 {
		t.Fatalf("Sources = %v", plan.Sources)
	}
	roles := strings.Join([]string{plan.Sources[0].Role, plan.Sources[1].Role, plan.Sources[2].Role}, ",")
	if roles != "usecase,repository,register" {
		t.Errorf("roles = %s", roles)
	}
	for _, f := range plan.Sources {
		if _, err := os.Stat(f.From); err != nil {
			t.Errorf("source %s missing: %v", f.From, err)
		}
		if !strings.HasPrefix(f.To, "services/device-rpc/") {
			t.Errorf("target %q does not start with services/device-rpc", f.To)
		}
	}
}

func TestPlanDomainCustomTarget(t *testing.T) {
	root := seedProject(t)
	plan, err := PlanDomain(DomainOptions{Root: root, Name: "device", To: "apps/device-rpc"})
	if err != nil {
		t.Fatalf("PlanDomain: %v", err)
	}
	if plan.To != "apps/device-rpc" || plan.TargetModule != "github.com/acme/demo/apps/device-rpc" {
		t.Fatalf("unexpected plan: %+v", plan)
	}
}

func TestApplyDomainCopiesFilesAndRewritesModule(t *testing.T) {
	root := seedProject(t)
	seedTargetService(t, filepath.Join(root, "services", "device-rpc"), "github.com/acme/device-rpc", manifest.KindKitex)
	plan, err := ApplyDomain(DomainOptions{Root: root, Name: "device"})
	if err != nil {
		t.Fatalf("ApplyDomain: %v", err)
	}
	if !plan.Applied {
		t.Fatalf("Applied = false")
	}
	if plan.TargetModule != "github.com/acme/device-rpc" {
		t.Fatalf("TargetModule = %q", plan.TargetModule)
	}
	if len(plan.Written) != 3 {
		t.Fatalf("Written = %v", plan.Written)
	}
	usecasePath := filepath.Join(root, "services", "device-rpc", "internal", "usecase", "device", "device.go")
	body, err := os.ReadFile(usecasePath)
	if err != nil {
		t.Fatalf("read copied usecase: %v", err)
	}
	content := string(body)
	if !strings.Contains(content, "github.com/acme/device-rpc/internal/repository/device") {
		t.Fatalf("copied usecase did not use target module:\n%s", content)
	}
	if strings.Contains(content, "github.com/acme/demo/internal/repository/device") {
		t.Fatalf("copied usecase still uses source module:\n%s", content)
	}
	if _, err := os.Stat(filepath.Join(root, "internal", "usecase", "device", "device.go")); err != nil {
		t.Fatalf("source usecase should remain in place: %v", err)
	}
}

func TestApplyDomainRequiresExistingKitexTarget(t *testing.T) {
	root := seedProject(t)
	_, err := ApplyDomain(DomainOptions{Root: root, Name: "device"})
	if err == nil || !strings.Contains(err.Error(), "target service manifest") {
		t.Fatalf("ApplyDomain error = %v, want target service manifest", err)
	}
	seedTargetService(t, filepath.Join(root, "services", "device-rpc"), "github.com/acme/device-rpc", manifest.KindHertz)
	_, err = ApplyDomain(DomainOptions{Root: root, Name: "device"})
	if err == nil || !strings.Contains(err.Error(), "not kitex") {
		t.Fatalf("ApplyDomain error = %v, want not kitex", err)
	}
}

func TestApplyDomainRefusesExistingTargetFiles(t *testing.T) {
	root := seedProject(t)
	seedTargetService(t, filepath.Join(root, "services", "device-rpc"), "github.com/acme/device-rpc", manifest.KindKitex)
	targetUsecase := filepath.Join(root, "services", "device-rpc", "internal", "usecase", "device", "device.go")
	if err := os.MkdirAll(filepath.Dir(targetUsecase), 0o755); err != nil {
		t.Fatalf("mkdir target: %v", err)
	}
	if err := os.WriteFile(targetUsecase, []byte("package device\n"), 0o644); err != nil {
		t.Fatalf("write target: %v", err)
	}
	_, err := ApplyDomain(DomainOptions{Root: root, Name: "device"})
	if err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("ApplyDomain error = %v, want already exists", err)
	}
}

func TestPlanDomainRejectsInvalidInputs(t *testing.T) {
	root := seedProject(t)
	cases := []struct {
		name string
		opts DomainOptions
		want string
	}{
		{"bad domain", DomainOptions{Root: root, Name: "bad-domain"}, "domain"},
		{"unlisted", DomainOptions{Root: root, Name: "theme"}, "not listed"},
		{"absolute target", DomainOptions{Root: root, Name: "device", To: "/tmp/device"}, "relative"},
		{"escaping target", DomainOptions{Root: root, Name: "device", To: "../device"}, "under root"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := PlanDomain(tc.opts)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("PlanDomain error = %v, want containing %q", err, tc.want)
			}
		})
	}
}

func seedTargetService(t *testing.T, root, module, kind string) {
	t.Helper()
	if err := manifest.Save(root, &manifest.Manifest{
		Ncgo:   manifest.Meta{Version: "0.4.0-test", AssetsVersion: "test-assets"},
		Mode:   manifest.ModeMono,
		Module: module,
		Service: manifest.Service{
			Name: "device-rpc", Kind: kind, IDL: "idl/device.proto",
		},
		GeneratedAt: time.Date(2026, 4, 29, 8, 0, 0, 0, time.UTC),
	}); err != nil {
		t.Fatalf("seed target manifest: %v", err)
	}
}

func TestPlanDomainUsesTargetManifestModule(t *testing.T) {
	root := seedProject(t)
	seedTargetService(t, filepath.Join(root, "services", "device-rpc"), "github.com/acme/device-rpc", manifest.KindKitex)
	plan, err := PlanDomain(DomainOptions{Root: root, Name: "device"})
	if err != nil {
		t.Fatalf("PlanDomain: %v", err)
	}
	if plan.TargetModule != "github.com/acme/device-rpc" {
		t.Errorf("TargetModule = %q, want target manifest module github.com/acme/device-rpc", plan.TargetModule)
	}
	if plan.Applied {
		t.Errorf("plan-only run must not set Applied")
	}
}

func TestPlanDomainRequiresSourceFiles(t *testing.T) {
	root := seedProject(t)
	missing := filepath.Join(root, "internal", "usecase", "device", "device.go")
	if err := os.Remove(missing); err != nil {
		t.Fatalf("remove source: %v", err)
	}
	_, err := PlanDomain(DomainOptions{Root: root, Name: "device"})
	if err == nil || !strings.Contains(err.Error(), "missing") {
		t.Fatalf("PlanDomain error = %v, want missing source", err)
	}
}
