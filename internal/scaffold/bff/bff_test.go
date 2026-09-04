package bff

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/byx-darwin/ncgo/internal/exec"
	"github.com/byx-darwin/ncgo/internal/manifest"
	planpkg "github.com/byx-darwin/ncgo/internal/scaffold/plan"
)

type fakeRunner struct {
	calls []exec.Cmd
}

func (f *fakeRunner) Run(_ context.Context, c exec.Cmd) (exec.Result, error) {
	f.calls = append(f.calls, c)
	return exec.Result{}, nil
}

func seedWorkspace(t *testing.T, services []manifest.WorkspaceService) string {
	t.Helper()
	root := t.TempDir()
	if err := manifest.SaveWorkspace(root, &manifest.Workspace{
		Ncgo:        manifest.Meta{Version: "0.3.0-test", AssetsVersion: "test-assets"},
		Mode:        manifest.ModeMicro,
		Name:        "commerce",
		Module:      "github.com/acme/commerce",
		Services:    services,
		GeneratedAt: time.Date(2026, 4, 29, 8, 0, 0, 0, time.UTC),
	}); err != nil {
		t.Fatalf("seed workspace: %v", err)
	}
	return root
}

func baseOpts(root string) Options {
	return Options{
		Root:          root,
		Name:          "web-bff",
		AssetsVersion: "test-assets",
		NCGOVersion:   "0.3.0-test",
		NoGenerate:    true,
		Now:           time.Date(2026, 4, 29, 8, 30, 0, 0, time.UTC),
	}
}

func TestAddNoGenerateCreatesHertzServiceAndUpdatesWorkspace(t *testing.T) {
	root := seedWorkspace(t, nil)
	res, err := Add(context.Background(), baseOpts(root))
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	if res.RanGenerate {
		t.Errorf("RanGenerate = true, want false")
	}
	if res.Module != "github.com/acme/commerce/services/web-bff" {
		t.Errorf("Module = %q", res.Module)
	}
	for _, p := range []string{
		".pre-commit-config.yaml",
		".ncgo/manifest.yaml",
		".dockerignore",
		"Dockerfile",
		"conf/docker/conf.yaml",
		"compose.yaml",
		"idl/app/web-bff.proto",
		"scripts/run-go-module-checks.sh",
		"template/data.json",
		"template/layout.yaml",
		"template/package.yaml",
	} {
		if _, err := os.Stat(filepath.Join(res.ServiceDir, p)); err != nil {
			t.Fatalf("service missing %s: %v", p, err)
		}
	}
	svcManifest, err := manifest.Load(res.ServiceDir)
	if err != nil {
		t.Fatalf("load service manifest: %v", err)
	}
	if svcManifest.Service.Kind != manifest.KindHertz || svcManifest.Module != res.Module {
		t.Errorf("service manifest mismatch: %+v", svcManifest)
	}
	w, err := manifest.LoadWorkspace(root)
	if err != nil {
		t.Fatalf("load workspace: %v", err)
	}
	if len(w.Services) != 1 || w.Services[0].Name != "web-bff" || w.Services[0].Kind != manifest.KindHertz || w.Services[0].Dir != "services/web-bff" {
		t.Errorf("workspace services = %+v", w.Services)
	}
	composeBody, err := os.ReadFile(filepath.Join(root, "compose.yaml"))
	if err != nil {
		t.Fatalf("read workspace compose: %v", err)
	}
	for _, want := range []string{"web-bff:", "./services/web-bff", "18080:8080", "GO_ENV: docker"} {
		if !strings.Contains(string(composeBody), want) {
			t.Fatalf("workspace compose missing %q\n---\n%s", want, composeBody)
		}
	}
}

func TestAddDryRunPlansWithoutWriting(t *testing.T) {
	root := seedWorkspace(t, nil)
	opts := baseOpts(root)
	opts.NoGenerate = false
	opts.DryRun = true
	res, err := Add(context.Background(), opts)
	if err != nil {
		t.Fatalf("Add dry-run: %v", err)
	}
	if !res.DryRun || !res.Updated || res.RanGenerate {
		t.Fatalf("DryRun/Updated/RanGenerate = %v/%v/%v, want true/true/false", res.DryRun, res.Updated, res.RanGenerate)
	}
	if _, err := os.Stat(res.ServiceDir); !os.IsNotExist(err) {
		t.Fatalf("dry-run created service dir: stat err = %v", err)
	}
	w, err := manifest.LoadWorkspace(root)
	if err != nil {
		t.Fatalf("reload workspace: %v", err)
	}
	if len(w.Services) != 0 {
		t.Fatalf("dry-run updated workspace services = %+v, want empty", w.Services)
	}
	if !planContains(res.Plan, "directory", "create") || !planContains(res.Plan, "workspace", "add") || !planContains(res.Plan, "generator", "run") || !planContains(res.Plan, "file", "write") {
		t.Fatalf("plan missing expected items: %+v", res.Plan)
	}
}

func TestAddSupportsModuleAndDirOverride(t *testing.T) {
	root := seedWorkspace(t, nil)
	opts := baseOpts(root)
	opts.Module = "github.com/acme/web-bff"
	opts.Dir = "apps/web-bff"
	res, err := Add(context.Background(), opts)
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	if res.ServiceRel != "apps/web-bff" || res.Module != opts.Module {
		t.Errorf("result = %+v", res)
	}
	m, _ := manifest.Load(res.ServiceDir)
	if m.Module != opts.Module {
		t.Errorf("service module = %q, want %q", m.Module, opts.Module)
	}
}

func TestAddRejectsDuplicateWorkspaceService(t *testing.T) {
	root := seedWorkspace(t, []manifest.WorkspaceService{{Name: "web-bff", Kind: manifest.KindHertz, Dir: "services/web-bff"}})
	_, err := Add(context.Background(), baseOpts(root))
	if err == nil || !strings.Contains(err.Error(), "already listed") {
		t.Fatalf("Add error = %v, want duplicate error", err)
	}
}

func TestAddRequiresWorkspace(t *testing.T) {
	opts := baseOpts(t.TempDir())
	_, err := Add(context.Background(), opts)
	if err == nil || !strings.Contains(err.Error(), "ncgo.workspace") {
		t.Fatalf("Add error = %v, want missing workspace", err)
	}
}

func TestAddInvokesHZViaRunner(t *testing.T) {
	root := seedWorkspace(t, nil)
	opts := baseOpts(root)
	opts.NoGenerate = false
	r := &fakeRunner{}
	opts.Runner = r
	res, err := Add(context.Background(), opts)
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	if !res.RanGenerate {
		t.Errorf("RanGenerate = false, want true")
	}
	if len(r.calls) != 1 || r.calls[0].Name != "hz" {
		t.Fatalf("expected one hz call, got %+v", r.calls)
	}
	args := strings.Join(r.calls[0].Args, " ")
	for _, want := range []string{"new", "--mod=" + res.Module, "--idl=idl/app/web-bff.proto"} {
		if !strings.Contains(args, want) {
			t.Errorf("hz args missing %q in %q", want, args)
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

func TestRewriteDockerfileForLocalReplacesRewritesWhenReplacePresent(t *testing.T) {
	root := t.TempDir()
	w := &manifest.Workspace{
		Services: []manifest.WorkspaceService{
			{Name: "web-bff", Kind: manifest.KindHertz, Dir: "services/web-bff"},
			{Name: "shared-types", Kind: manifest.KindHertz, Dir: "services/shared-types"},
		},
	}
	webDir := filepath.Join(root, "services", "web-bff")
	if err := os.MkdirAll(webDir, 0o755); err != nil {
		t.Fatalf("mkdir web-bff: %v", err)
	}
	sharedDir := filepath.Join(root, "services", "shared-types")
	if err := os.MkdirAll(sharedDir, 0o755); err != nil {
		t.Fatalf("mkdir shared-types: %v", err)
	}
	goMod := "module github.com/acme/commerce/services/web-bff\n\ngo 1.25\n\nreplace github.com/acme/commerce/services/shared-types => ../shared-types\n"
	if err := os.WriteFile(filepath.Join(webDir, "go.mod"), []byte(goMod), 0o644); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}
	if err := os.WriteFile(filepath.Join(webDir, "Dockerfile"), []byte("COPY . .\n"), 0o644); err != nil {
		t.Fatalf("write Dockerfile: %v", err)
	}
	if err := rewriteDockerfileForLocalReplaces(root, webDir, "services/web-bff", w); err != nil {
		t.Fatalf("rewriteDockerfileForLocalReplaces: %v", err)
	}
	body, err := os.ReadFile(filepath.Join(webDir, "Dockerfile"))
	if err != nil {
		t.Fatalf("read Dockerfile: %v", err)
	}
	if !strings.Contains(string(body), "COPY services/shared-types/ services/shared-types/\n") {
		t.Fatalf("Dockerfile not rewritten for sibling:\n%s", body)
	}
}

func TestRewriteDockerfileForLocalReplacesNoopWithoutReplace(t *testing.T) {
	root := t.TempDir()
	w := &manifest.Workspace{
		Services: []manifest.WorkspaceService{{Name: "web-bff", Kind: manifest.KindHertz, Dir: "services/web-bff"}},
	}
	webDir := filepath.Join(root, "services", "web-bff")
	if err := os.MkdirAll(webDir, 0o755); err != nil {
		t.Fatalf("mkdir web-bff: %v", err)
	}
	if err := os.WriteFile(filepath.Join(webDir, "Dockerfile"), []byte("COPY . .\n"), 0o644); err != nil {
		t.Fatalf("write Dockerfile: %v", err)
	}
	if err := rewriteDockerfileForLocalReplaces(root, webDir, "services/web-bff", w); err != nil {
		t.Fatalf("rewriteDockerfileForLocalReplaces: %v", err)
	}
	body, err := os.ReadFile(filepath.Join(webDir, "Dockerfile"))
	if err != nil {
		t.Fatalf("read Dockerfile: %v", err)
	}
	if string(body) != "COPY . .\n" {
		t.Fatalf("Dockerfile should be unchanged, got:\n%s", body)
	}
}
