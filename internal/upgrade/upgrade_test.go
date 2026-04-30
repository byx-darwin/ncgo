package upgrade

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/byx-darwin/ncgo/internal/manifest"
)

const (
	oldVersion = "0.1.0"
	oldAssets  = "0.1.0"
	newVersion = "0.4.0-test"
	newAssets  = "0.4.0-assets"
)

func seedMono(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := manifest.Save(root, &manifest.Manifest{
		Ncgo:   manifest.Meta{Version: oldVersion, AssetsVersion: oldAssets},
		Mode:   manifest.ModeMono,
		Module: "github.com/acme/demo",
		Service: manifest.Service{
			Name: "demo", Kind: manifest.KindHertz, IDL: "idl/app/demo.proto",
		},
		GeneratedAt: time.Date(2026, 4, 29, 8, 0, 0, 0, time.UTC),
	}); err != nil {
		t.Fatalf("seed mono: %v", err)
	}
	return root
}

func TestRunUpgradesMonoManifest(t *testing.T) {
	root := seedMono(t)
	res, err := Run(Options{Root: root, NCGOVersion: newVersion, AssetsVersion: newAssets})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !res.Changed || res.Mode != manifest.ModeMono || res.OldVersion != oldVersion || res.NewVersion != newVersion {
		t.Fatalf("unexpected result: %+v", res)
	}
	m, err := manifest.Load(root)
	if err != nil {
		t.Fatalf("reload manifest: %v", err)
	}
	if m.Ncgo.Version != newVersion || m.Ncgo.AssetsVersion != newAssets {
		t.Fatalf("manifest meta = %+v", m.Ncgo)
	}
}

func TestRunDryRunDoesNotWrite(t *testing.T) {
	root := seedMono(t)
	res, err := Run(Options{Root: root, NCGOVersion: newVersion, AssetsVersion: newAssets, DryRun: true})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !res.Changed || !res.DryRun {
		t.Fatalf("unexpected result: %+v", res)
	}
	m, err := manifest.Load(root)
	if err != nil {
		t.Fatalf("reload manifest: %v", err)
	}
	if m.Ncgo.Version != oldVersion || m.Ncgo.AssetsVersion != oldAssets {
		t.Fatalf("dry-run wrote manifest meta = %+v", m.Ncgo)
	}
}

func TestRunPlanDoesNotWrite(t *testing.T) {
	root := seedMono(t)
	res, err := Run(Options{Root: root, NCGOVersion: newVersion, AssetsVersion: newAssets, Plan: true})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !res.Changed || !res.Plan || res.DryRun {
		t.Fatalf("unexpected result: %+v", res)
	}
	m, err := manifest.Load(root)
	if err != nil {
		t.Fatalf("reload manifest: %v", err)
	}
	if m.Ncgo.Version != oldVersion || m.Ncgo.AssetsVersion != oldAssets {
		t.Fatalf("plan wrote manifest meta = %+v", m.Ncgo)
	}
}

func TestRunUpgradesMicroWorkspaceAndServices(t *testing.T) {
	root := t.TempDir()
	serviceDir := filepath.Join(root, "services", "user-rpc")
	if err := manifest.SaveWorkspace(root, &manifest.Workspace{
		Ncgo:   manifest.Meta{Version: oldVersion, AssetsVersion: oldAssets},
		Mode:   manifest.ModeMicro,
		Name:   "commerce",
		Module: "github.com/acme/commerce",
		Services: []manifest.WorkspaceService{{
			Name: "user-rpc", Kind: manifest.KindKitex, Dir: "services/user-rpc",
		}},
	}); err != nil {
		t.Fatalf("seed workspace: %v", err)
	}
	if err := manifest.Save(serviceDir, &manifest.Manifest{
		Ncgo:   manifest.Meta{Version: oldVersion, AssetsVersion: oldAssets},
		Mode:   manifest.ModeMono,
		Module: "github.com/acme/commerce/services/user-rpc",
		Service: manifest.Service{
			Name: "user-rpc", Kind: manifest.KindKitex, IDL: "idl/userrpc.proto",
		},
	}); err != nil {
		t.Fatalf("seed service manifest: %v", err)
	}
	res, err := Run(Options{Root: root, NCGOVersion: newVersion, AssetsVersion: newAssets})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !res.Changed || res.Mode != manifest.ModeMicro || len(res.ServiceUpdates) != 1 || !res.ServiceUpdates[0].Changed {
		t.Fatalf("unexpected result: %+v", res)
	}
	w, err := manifest.LoadWorkspace(root)
	if err != nil {
		t.Fatalf("reload workspace: %v", err)
	}
	m, err := manifest.Load(serviceDir)
	if err != nil {
		t.Fatalf("reload service manifest: %v", err)
	}
	if w.Ncgo.Version != newVersion || m.Ncgo.Version != newVersion {
		t.Fatalf("meta not upgraded: workspace=%+v service=%+v", w.Ncgo, m.Ncgo)
	}
}

func TestRunPlanReportsMicroWorkspaceAndServices(t *testing.T) {
	root, serviceDir := seedMicro(t)
	res, err := Run(Options{Root: root, NCGOVersion: newVersion, AssetsVersion: newAssets, Plan: true})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !res.Plan || !res.Changed || res.Mode != manifest.ModeMicro || res.ServiceCount != 1 {
		t.Fatalf("unexpected result: %+v", res)
	}
	if len(res.ServiceUpdates) != 1 {
		t.Fatalf("ServiceUpdates = %v", res.ServiceUpdates)
	}
	update := res.ServiceUpdates[0]
	if !update.Changed || update.NewVersion != newVersion || update.NewAssets != newAssets {
		t.Fatalf("unexpected service update: %+v", update)
	}
	w, err := manifest.LoadWorkspace(root)
	if err != nil {
		t.Fatalf("reload workspace: %v", err)
	}
	m, err := manifest.Load(serviceDir)
	if err != nil {
		t.Fatalf("reload service manifest: %v", err)
	}
	if w.Ncgo.Version != oldVersion || m.Ncgo.Version != oldVersion {
		t.Fatalf("plan wrote metadata: workspace=%+v service=%+v", w.Ncgo, m.Ncgo)
	}
}

func TestRunAlreadyCurrent(t *testing.T) {
	root := seedMono(t)
	if _, err := Run(Options{Root: root, NCGOVersion: newVersion, AssetsVersion: newAssets}); err != nil {
		t.Fatalf("first Run: %v", err)
	}
	res, err := Run(Options{Root: root, NCGOVersion: newVersion, AssetsVersion: newAssets})
	if err != nil {
		t.Fatalf("second Run: %v", err)
	}
	if res.Changed {
		t.Fatalf("Changed = true, want false: %+v", res)
	}
}

func TestRunRequiresMetadata(t *testing.T) {
	_, err := Run(Options{Root: t.TempDir(), NCGOVersion: newVersion, AssetsVersion: newAssets})
	if err == nil {
		t.Fatalf("expected missing metadata error")
	}
}

func seedMicro(t *testing.T) (root, serviceDir string) {
	t.Helper()
	root = t.TempDir()
	serviceDir = filepath.Join(root, "services", "user-rpc")
	if err := manifest.SaveWorkspace(root, &manifest.Workspace{
		Ncgo:   manifest.Meta{Version: oldVersion, AssetsVersion: oldAssets},
		Mode:   manifest.ModeMicro,
		Name:   "commerce",
		Module: "github.com/acme/commerce",
		Services: []manifest.WorkspaceService{{
			Name: "user-rpc", Kind: manifest.KindKitex, Dir: "services/user-rpc",
		}},
	}); err != nil {
		t.Fatalf("seed workspace: %v", err)
	}
	if err := manifest.Save(serviceDir, &manifest.Manifest{
		Ncgo:   manifest.Meta{Version: oldVersion, AssetsVersion: oldAssets},
		Mode:   manifest.ModeMono,
		Module: "github.com/acme/commerce/services/user-rpc",
		Service: manifest.Service{
			Name: "user-rpc", Kind: manifest.KindKitex, IDL: "idl/userrpc.proto",
		},
	}); err != nil {
		t.Fatalf("seed service manifest: %v", err)
	}
	return root, serviceDir
}
