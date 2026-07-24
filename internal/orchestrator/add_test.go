package orchestrator

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/byx-darwin/ncgo/internal/manifest"
	"github.com/byx-darwin/ncgo/internal/scaffold/infra"
)

func seedOrchAddProject(t *testing.T, kind string) string {
	t.Helper()
	root := t.TempDir()
	m := &manifest.Manifest{
		Ncgo:   manifest.Meta{Version: "0.0.0-test", AssetsVersion: "test"},
		Mode:   manifest.ModeMono,
		Module: "github.com/x/demo",
		Service: manifest.Service{
			Name: "demo", Kind: kind, IDL: "idl/app/demo.proto",
		},
		GeneratedAt: time.Date(2026, 4, 29, 0, 0, 0, 0, time.UTC),
	}
	if err := manifest.Save(root, m); err != nil {
		t.Fatalf("seed manifest: %v", err)
	}
	return root
}

func TestRunAddInfra(t *testing.T) {
	root := seedOrchAddProject(t, manifest.KindHertz)
	res, err := RunAddInfra(AddInfraOptions{Root: root, Kind: infra.KindRedis})
	if err != nil {
		t.Fatalf("RunAddInfra: %v", err)
	}
	if res.Raw == nil || res.Raw.DryRun {
		t.Fatalf("Raw=%v DryRun=%v, want non-nil and false", res.Raw, res.Raw != nil && res.Raw.DryRun)
	}
	if !res.Raw.Updated {
		t.Fatalf("updated = false, want true")
	}
	if len(res.Raw.WrittenPaths) != 3 {
		t.Fatalf("writtenPaths = %v, want 3", res.Raw.WrittenPaths)
	}
	for _, suffix := range []string{
		filepath.Join("internal", "base", "data", "redis.go"),
		filepath.Join("internal", "base", "data", "redis_shared.go"),
		filepath.Join("conf", "dev", "conf.yaml"),
	} {
		if _, err := os.Stat(filepath.Join(root, suffix)); err != nil {
			t.Fatalf("file %s not written: %v", suffix, err)
		}
	}
	m, err := manifest.Load(root)
	if err != nil {
		t.Fatalf("reload manifest: %v", err)
	}
	if len(m.Infra) != 1 || m.Infra[0] != infra.KindRedis {
		t.Fatalf("manifest.Infra = %v, want [redis]", m.Infra)
	}
}

func TestRunAddInfraDryRun(t *testing.T) {
	root := seedOrchAddProject(t, manifest.KindHertz)
	res, err := RunAddInfra(AddInfraOptions{Root: root, Kind: infra.KindRedis, DryRun: true})
	if err != nil {
		t.Fatalf("RunAddInfra dry-run: %v", err)
	}
	if !res.Raw.DryRun || !res.Raw.Updated {
		t.Fatalf("dryRun/updated = %v/%v, want true/true", res.Raw.DryRun, res.Raw.Updated)
	}
	if _, err := os.Stat(filepath.Join(root, "internal", "base", "data", "redis.go")); !os.IsNotExist(err) {
		t.Fatalf("dry-run wrote redis file: stat err = %v", err)
	}
}

func TestRunAddInfraInvalidKind(t *testing.T) {
	root := seedOrchAddProject(t, manifest.KindHertz)
	_, err := RunAddInfra(AddInfraOptions{Root: root, Kind: "bogus"})
	if err == nil {
		t.Fatal("expected error for invalid kind")
	}
}

func TestRunAddDomain(t *testing.T) {
	root := seedOrchAddProject(t, manifest.KindHertz)
	res, err := RunAddDomain(AddDomainOptions{Root: root, Name: "device"})
	if err != nil {
		t.Fatalf("RunAddDomain: %v", err)
	}
	if res.Raw == nil || res.Raw.DryRun {
		t.Fatalf("Raw=%v DryRun=%v", res.Raw, res.Raw != nil && res.Raw.DryRun)
	}
	if !res.Raw.Updated {
		t.Fatalf("updated = false, want true")
	}
	if len(res.Raw.WrittenPaths) != 3 {
		t.Fatalf("writtenPaths = %v, want 3", res.Raw.WrittenPaths)
	}
	for _, suffix := range []string{
		filepath.Join("internal", "usecase", "device", "device.go"),
		filepath.Join("internal", "repository", "device", "device.go"),
		filepath.Join("internal", "base", "data", "device_register.go"),
	} {
		if _, err := os.Stat(filepath.Join(root, suffix)); err != nil {
			t.Fatalf("file %s not written: %v", suffix, err)
		}
	}
}

func TestRunAddDomainDryRun(t *testing.T) {
	root := seedOrchAddProject(t, manifest.KindHertz)
	res, err := RunAddDomain(AddDomainOptions{Root: root, Name: "device", DryRun: true})
	if err != nil {
		t.Fatalf("RunAddDomain dry-run: %v", err)
	}
	if !res.Raw.DryRun || !res.Raw.Updated {
		t.Fatalf("dryRun/updated = %v/%v, want true/true", res.Raw.DryRun, res.Raw.Updated)
	}
	if _, err := os.Stat(filepath.Join(root, "internal", "usecase", "device", "device.go")); !os.IsNotExist(err) {
		t.Fatalf("dry-run wrote file: stat err = %v", err)
	}
}

func TestRunAddDomainInvalidName(t *testing.T) {
	root := seedOrchAddProject(t, manifest.KindHertz)
	_, err := RunAddDomain(AddDomainOptions{Root: root, Name: "INVALID"})
	if err == nil {
		t.Fatal("expected error for invalid name")
	}
}

func TestRunAddMethod(t *testing.T) {
	root := seedOrchAddProject(t, manifest.KindHertz)
	// First add the domain so the method can find it.
	domRes, err := RunAddDomain(AddDomainOptions{Root: root, Name: "device"})
	if err != nil {
		t.Fatalf("seed domain: %v", err)
	}
	if len(domRes.Raw.WrittenPaths) < 1 {
		t.Fatalf("no domain files written")
	}

	methodRes, err := RunAddMethod(AddMethodOptions{Root: root, Spec: "device.CreateDevice"})
	if err != nil {
		t.Fatalf("RunAddMethod: %v", err)
	}
	if methodRes.Raw == nil || methodRes.Raw.Domain != "device" || methodRes.Raw.Method != "CreateDevice" {
		t.Fatalf("result = %+v", methodRes.Raw)
	}
}

func TestRunAddMethodDomainNotListed(t *testing.T) {
	root := seedOrchAddProject(t, manifest.KindHertz)
	_, err := RunAddMethod(AddMethodOptions{Root: root, Spec: "missing.SomeMethod"})
	if err == nil {
		t.Fatal("expected error for domain not listed in manifest")
	}
}

func TestRunAddRuleCenter(t *testing.T) {
	root := seedOrchAddProject(t, manifest.KindHertz)
	// rulecenter.Add needs conf/dev/conf.yaml to exist for config update.
	confDir := filepath.Join(root, "conf", "dev")
	if err := os.MkdirAll(confDir, 0o755); err != nil {
		t.Fatalf("mkdir conf: %v", err)
	}
	if err := os.WriteFile(filepath.Join(confDir, "conf.yaml"), []byte("server:\n  host: localhost\n"), 0o644); err != nil {
		t.Fatalf("write conf.yaml: %v", err)
	}
	res, err := RunAddRuleCenter(AddRuleCenterOptions{Root: root, Addr: "localhost:8888"})
	if err != nil {
		t.Fatalf("RunAddRuleCenter: %v", err)
	}
	if res.Raw == nil || res.Raw.DryRun {
		t.Fatalf("Raw=%v DryRun=%v", res.Raw, res.Raw != nil && res.Raw.DryRun)
	}
	if len(res.Raw.WrittenPaths) == 0 {
		t.Fatalf("writtenPaths = %v, want non-empty", res.Raw.WrittenPaths)
	}
	if len(res.Raw.NextSteps) == 0 {
		t.Fatalf("nextSteps = %v, want non-empty", res.Raw.NextSteps)
	}
}

func TestRunAddRuleCenterRequiresAddr(t *testing.T) {
	root := seedOrchAddProject(t, manifest.KindHertz)
	_, err := RunAddRuleCenter(AddRuleCenterOptions{Root: root})
	if err == nil {
		t.Fatal("expected error for missing addr")
	}
}

func TestRunAddRuleCenterKitexRejected(t *testing.T) {
	root := seedOrchAddProject(t, manifest.KindKitex)
	_, err := RunAddRuleCenter(AddRuleCenterOptions{Root: root, Addr: "localhost:8888"})
	if err == nil {
		t.Fatal("expected error for kitex service")
	}
}
