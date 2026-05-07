package ai

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/byx-darwin/ncgo/internal/manifest"
)

func sampleManifest(kind string) *manifest.Manifest {
	return &manifest.Manifest{
		Ncgo:    manifest.Meta{Version: "0.1.0-dev", AssetsVersion: "test"},
		Mode:    manifest.ModeMono,
		Module:  "github.com/acme/user-api",
		Service: manifest.Service{Name: "user-api", Kind: kind, WithDatabase: true},
		Infra:   []string{"redis"},
		Domains: []string{"device"},
	}
}

func sampleWorkspace() *manifest.Workspace {
	return &manifest.Workspace{
		Ncgo:   manifest.Meta{Version: "0.1.0-dev", AssetsVersion: "test"},
		Mode:   manifest.ModeMicro,
		Name:   "commerce",
		Module: "github.com/acme/commerce",
		Services: []manifest.WorkspaceService{
			{Name: "user-rpc", Kind: manifest.KindKitex, Dir: "services/user-rpc"},
			{Name: "web-bff", Kind: manifest.KindHertz, Dir: "services/web-bff"},
		},
	}
}

func workspaceServiceManifest(module, name, kind, idl string, withDatabase bool, infra, domains []string) *manifest.Manifest {
	return &manifest.Manifest{
		Ncgo:    manifest.Meta{Version: "0.1.0-dev", AssetsVersion: "test"},
		Mode:    manifest.ModeMono,
		Module:  module,
		Service: manifest.Service{Name: name, Kind: kind, WithDatabase: withDatabase, IDL: idl},
		Infra:   infra,
		Domains: domains,
	}
}

func writeManifest(t *testing.T, root, kind string) {
	t.Helper()
	if err := manifest.Save(root, sampleManifest(kind)); err != nil {
		t.Fatalf("manifest.Save: %v", err)
	}
}

func writeWorkspace(t *testing.T, root string) {
	t.Helper()
	if err := manifest.SaveWorkspace(root, sampleWorkspace()); err != nil {
		t.Fatalf("manifest.SaveWorkspace: %v", err)
	}
	if err := manifest.Save(filepath.Join(root, "services", "user-rpc"), workspaceServiceManifest(
		"github.com/acme/commerce/services/user-rpc",
		"user-rpc",
		manifest.KindKitex,
		"idl/userrpc.proto",
		true,
		[]string{"redis"},
		[]string{"user"},
	)); err != nil {
		t.Fatalf("manifest.Save user-rpc: %v", err)
	}
	if err := manifest.Save(filepath.Join(root, "services", "web-bff"), workspaceServiceManifest(
		"github.com/acme/commerce/services/web-bff",
		"web-bff",
		manifest.KindHertz,
		"idl/app/webbff.proto",
		false,
		nil,
		[]string{"web"},
	)); err != nil {
		t.Fatalf("manifest.Save web-bff: %v", err)
	}
}

func TestSyncWritesAllTargets(t *testing.T) {
	root := t.TempDir()
	writeManifest(t, root, manifest.KindHertz)
	res, err := Sync(Options{Root: root})
	if err != nil {
		t.Fatalf("Sync: %v", err)
	}
	if res.Scope != "service" || res.SourceRef != ".ncgo/manifest.yaml" {
		t.Fatalf("sync metadata = scope=%q sourceRef=%q, want service/.ncgo/manifest.yaml", res.Scope, res.SourceRef)
	}
	if res.Workspace != nil {
		t.Fatalf("workspace metadata = %+v, want nil for standalone service", res.Workspace)
	}
	wantPaths := []string{"AGENTS.md", "CLAUDE.md", ".cursor/rules/ncgo.mdc", ".claude/generated/project-context.md"}
	if len(res.Written) != len(wantPaths) {
		t.Fatalf("Written = %v, want %v", res.Written, wantPaths)
	}
	for _, p := range wantPaths {
		full := filepath.Join(root, p)
		b, err := os.ReadFile(full)
		if err != nil {
			t.Fatalf("read %s: %v", p, err)
		}
		body := string(b)
		if !strings.Contains(body, ManagedMarker) {
			t.Errorf("%s missing managed marker", p)
		}
		if !strings.Contains(body, "module: `github.com/acme/user-api`") {
			t.Errorf("%s missing manifest module summary", p)
		}
		if !strings.Contains(body, "infra: `[redis]`") {
			t.Errorf("%s missing manifest infra summary", p)
		}
		if !strings.Contains(body, "domains: `[device]`") {
			t.Errorf("%s missing manifest domains summary", p)
		}
		if p == ".claude/generated/project-context.md" {
			if !strings.Contains(body, "# Claude Project Context") {
				t.Errorf("%s missing Claude project context title", p)
			}
			if !strings.Contains(body, ".claude/rules/go.md") || !strings.Contains(body, ".claude/rules/agent-engineering.md") {
				t.Errorf("%s missing repository rules links", p)
			}
			if !strings.Contains(body, "The Hertz template family backs") {
				t.Errorf("%s missing design-doc overview summary", p)
			}
			continue
		}
		if !strings.Contains(body, "Hertz Template Design Doc") &&
			!strings.Contains(body, "## 2. Generated Project Architecture") {
			t.Errorf("%s missing embedded design-doc body", p)
		}
	}
	mdc, _ := os.ReadFile(filepath.Join(root, ".cursor/rules/ncgo.mdc"))
	if !strings.HasPrefix(string(mdc), "---\n") {
		t.Errorf(".mdc must start with frontmatter; got: %q", string(mdc[:4]))
	}
}

func TestSyncPicksKitexDoc(t *testing.T) {
	root := t.TempDir()
	writeManifest(t, root, manifest.KindKitex)
	if _, err := Sync(Options{Root: root}); err != nil {
		t.Fatalf("Sync: %v", err)
	}
	body, _ := os.ReadFile(filepath.Join(root, "AGENTS.md"))
	if !strings.Contains(string(body), "Kitex") {
		t.Errorf("AGENTS.md should mention Kitex for kitex-kind manifest")
	}
	if strings.Contains(string(body), "## 6. `hz` Invocation Mapping") {
		t.Errorf("AGENTS.md should not embed hertz-specific section for kitex manifest")
	}
}

func TestSyncServiceUnderWorkspaceAddsMembershipFacts(t *testing.T) {
	root := t.TempDir()
	writeWorkspace(t, root)
	serviceRoot := filepath.Join(root, "services", "user-rpc")
	res, err := Sync(Options{Root: serviceRoot})
	if err != nil {
		t.Fatalf("Sync: %v", err)
	}
	if res.Scope != "service" || res.SourceRef != ".ncgo/manifest.yaml" {
		t.Fatalf("sync metadata = scope=%q sourceRef=%q, want service/.ncgo/manifest.yaml", res.Scope, res.SourceRef)
	}
	if res.Workspace == nil || res.Workspace.Role != "member" {
		t.Fatalf("workspace metadata = %+v, want member metadata", res.Workspace)
	}
	if res.Workspace.Name != "commerce" || res.Workspace.Root != "../.." || res.Workspace.ServiceDir != "services/user-rpc" {
		t.Fatalf("workspace metadata = %+v, want commerce/../../services/user-rpc", res.Workspace)
	}
	if len(res.Notes) != 2 {
		t.Fatalf("Notes = %v, want 2 service workspace notes", res.Notes)
	}
	if !strings.Contains(strings.Join(res.Notes, "\n"), "parent micro workspace `../..`") {
		t.Fatalf("service sync notes missing workspace root hint: %v", res.Notes)
	}
	projectContext, _ := os.ReadFile(filepath.Join(serviceRoot, ".claude/generated/project-context.md"))
	body := string(projectContext)
	if !strings.Contains(body, "workspace.member: `true`") {
		t.Errorf("project-context.md missing workspace membership fact")
	}
	if !strings.Contains(body, "workspace.name: `commerce`") {
		t.Errorf("project-context.md missing workspace name")
	}
	if !strings.Contains(body, "workspace.root: `../..`") {
		t.Errorf("project-context.md missing workspace root relative path")
	}
	if !strings.Contains(body, "workspace.service_dir: `services/user-rpc`") {
		t.Errorf("project-context.md missing workspace service dir")
	}
	if !strings.Contains(body, "run `ncgo ai sync --root ../..` for workspace-level context") {
		t.Errorf("project-context.md missing workspace-level sync note")
	}
	agents, _ := os.ReadFile(filepath.Join(serviceRoot, "AGENTS.md"))
	if !strings.Contains(string(agents), "workspace.member: `true`") {
		t.Errorf("AGENTS.md missing workspace membership fact")
	}
}

func TestSyncServiceUnderWorkspaceSkipsUnlistedParentWorkspace(t *testing.T) {
	root := t.TempDir()
	serviceRoot := filepath.Join(root, "services", "user-rpc")
	if err := manifest.Save(serviceRoot, workspaceServiceManifest(
		"github.com/acme/commerce/services/user-rpc",
		"user-rpc",
		manifest.KindKitex,
		"idl/userrpc.proto",
		false,
		nil,
		nil,
	)); err != nil {
		t.Fatalf("manifest.Save service: %v", err)
	}
	if err := manifest.SaveWorkspace(root, &manifest.Workspace{
		Ncgo:   manifest.Meta{Version: "0.1.0-dev", AssetsVersion: "test"},
		Mode:   manifest.ModeMicro,
		Name:   "commerce",
		Module: "github.com/acme/commerce",
		Services: []manifest.WorkspaceService{{
			Name: "other-rpc", Kind: manifest.KindKitex, Dir: "services/other-rpc",
		}},
	}); err != nil {
		t.Fatalf("manifest.SaveWorkspace: %v", err)
	}
	res, err := Sync(Options{Root: serviceRoot})
	if err != nil {
		t.Fatalf("Sync: %v", err)
	}
	if len(res.Notes) != 0 {
		t.Fatalf("Notes = %v, want no workspace-membership notes", res.Notes)
	}
	projectContext, _ := os.ReadFile(filepath.Join(serviceRoot, ".claude/generated/project-context.md"))
	if strings.Contains(string(projectContext), "workspace.member") {
		t.Errorf("project-context.md should not include workspace membership for unlisted service")
	}
}

func TestSyncRefusesUnmanagedFile(t *testing.T) {
	root := t.TempDir()
	writeManifest(t, root, manifest.KindHertz)
	pre := "# user-owned AGENTS\n\nhand-written content\n"
	if err := os.WriteFile(filepath.Join(root, "AGENTS.md"), []byte(pre), 0o644); err != nil {
		t.Fatalf("seed AGENTS.md: %v", err)
	}
	res, err := Sync(Options{Root: root})
	if err != nil {
		t.Fatalf("Sync: %v", err)
	}
	if got, _ := os.ReadFile(filepath.Join(root, "AGENTS.md")); string(got) != pre {
		t.Errorf("AGENTS.md must not be overwritten without --force")
	}
	var skipped bool
	for _, s := range res.Skipped {
		if s.Path == "AGENTS.md" && strings.Contains(s.Reason, "ncgo:managed") {
			skipped = true
		}
	}
	if !skipped {
		t.Errorf("expected AGENTS.md skip in result; got %+v", res.Skipped)
	}
}

func TestSyncForceOverwritesUnmanagedFile(t *testing.T) {
	root := t.TempDir()
	writeManifest(t, root, manifest.KindHertz)
	if err := os.WriteFile(filepath.Join(root, "AGENTS.md"), []byte("hand-written\n"), 0o644); err != nil {
		t.Fatalf("seed AGENTS.md: %v", err)
	}
	if _, err := Sync(Options{Root: root, Force: true}); err != nil {
		t.Fatalf("Sync: %v", err)
	}
	body, _ := os.ReadFile(filepath.Join(root, "AGENTS.md"))
	if !strings.Contains(string(body), ManagedMarker) {
		t.Errorf("--force should overwrite with managed file; got %q", string(body))
	}
}

func TestSyncAppendsLocalNotes(t *testing.T) {
	root := t.TempDir()
	writeManifest(t, root, manifest.KindHertz)
	notes := "extra rule: avoid global variables.\n"
	if err := os.WriteFile(filepath.Join(root, LocalNotesFile), []byte(notes), 0o644); err != nil {
		t.Fatalf("seed local notes: %v", err)
	}
	if _, err := Sync(Options{Root: root}); err != nil {
		t.Fatalf("Sync: %v", err)
	}
	body, _ := os.ReadFile(filepath.Join(root, "AGENTS.md"))
	if !strings.Contains(string(body), "## Local Notes") || !strings.Contains(string(body), "avoid global variables") {
		t.Errorf("Local Notes section missing from AGENTS.md")
	}
	projectContext, _ := os.ReadFile(filepath.Join(root, ".claude/generated/project-context.md"))
	if strings.Contains(string(projectContext), "Local Notes") || strings.Contains(string(projectContext), "avoid global variables") {
		t.Errorf("project-context.md must not include AGENTS.local.md content")
	}
}

func TestSyncDryRunWritesNothing(t *testing.T) {
	root := t.TempDir()
	writeManifest(t, root, manifest.KindHertz)
	res, err := Sync(Options{Root: root, DryRun: true})
	if err != nil {
		t.Fatalf("Sync: %v", err)
	}
	if len(res.Written) != 0 {
		t.Errorf("DryRun must not write; got %v", res.Written)
	}
	if len(res.Skipped) != len(targets()) {
		t.Errorf("DryRun should skip all targets; got %v", res.Skipped)
	}
	if _, err := os.Stat(filepath.Join(root, "AGENTS.md")); !os.IsNotExist(err) {
		t.Errorf("AGENTS.md should not exist after dry run")
	}
}

func TestSyncRejectsBadLang(t *testing.T) {
	root := t.TempDir()
	writeManifest(t, root, manifest.KindHertz)
	if _, err := Sync(Options{Root: root, Lang: "fr"}); err == nil {
		t.Fatalf("expected error for --lang fr")
	}
}

func TestSyncZhLangPicksZhDoc(t *testing.T) {
	root := t.TempDir()
	writeManifest(t, root, manifest.KindHertz)
	if _, err := Sync(Options{Root: root, Lang: LangZhCN}); err != nil {
		t.Fatalf("Sync: %v", err)
	}
	body, _ := os.ReadFile(filepath.Join(root, "AGENTS.md"))
	if !strings.Contains(string(body), "生成项目架构") {
		t.Errorf("zh-CN sync should embed Chinese design doc")
	}
	projectContext, _ := os.ReadFile(filepath.Join(root, ".claude/generated/project-context.md"))
	if !strings.Contains(string(projectContext), "Hertz 模板族支撑") {
		t.Errorf("zh-CN sync should render Chinese overview in project-context.md")
	}
}

func TestSyncWorkspaceWritesAllTargets(t *testing.T) {
	root := t.TempDir()
	writeWorkspace(t, root)
	res, err := Sync(Options{Root: root})
	if err != nil {
		t.Fatalf("Sync: %v", err)
	}
	if res.Scope != "workspace" || res.SourceRef != manifest.WorkspaceFileName {
		t.Fatalf("sync metadata = scope=%q sourceRef=%q, want workspace/%s", res.Scope, res.SourceRef, manifest.WorkspaceFileName)
	}
	if res.Workspace == nil || res.Workspace.Role != "root" || res.Workspace.ServiceCount != 2 {
		t.Fatalf("workspace metadata = %+v, want root metadata with serviceCount=2", res.Workspace)
	}
	wantPaths := []string{"AGENTS.md", "CLAUDE.md", ".cursor/rules/ncgo.mdc", ".claude/generated/project-context.md"}
	if len(res.Written) != len(wantPaths) {
		t.Fatalf("Written = %v, want %v", res.Written, wantPaths)
	}
	if len(res.Notes) != 2 {
		t.Fatalf("Notes = %v, want 2 workspace notes", res.Notes)
	}
	if !strings.Contains(strings.Join(res.Notes, "\n"), "workspace-level AI context") {
		t.Fatalf("workspace notes missing scope hint: %v", res.Notes)
	}
	agents, _ := os.ReadFile(filepath.Join(root, "AGENTS.md"))
	if !strings.Contains(string(agents), "`ncgo.workspace`") {
		t.Errorf("AGENTS.md should mention workspace metadata source")
	}
	if !strings.Contains(string(agents), "workspace.name: `commerce`") {
		t.Errorf("AGENTS.md missing workspace facts")
	}
	if !strings.Contains(string(agents), "The micro workspace profile backs repository roots created by `ncgo new --mode micro`") {
		t.Errorf("AGENTS.md missing embedded micro design doc overview")
	}
	projectContext, _ := os.ReadFile(filepath.Join(root, ".claude/generated/project-context.md"))
	body := string(projectContext)
	if !strings.Contains(body, "services.count: `2`") {
		t.Errorf("project-context.md missing workspace service count")
	}
	if !strings.Contains(body, "dir `services/user-rpc`") || !strings.Contains(body, "dir `services/web-bff`") {
		t.Errorf("project-context.md missing workspace service inventory")
	}
	if !strings.Contains(body, "run `ncgo ai sync --root services/<name>`") {
		t.Errorf("project-context.md missing workspace-specific note")
	}
}

func TestSyncWorkspaceAppendsLocalNotesOnlyToLongFormFiles(t *testing.T) {
	root := t.TempDir()
	writeWorkspace(t, root)
	notes := "workspace note: review service boundaries.\n"
	if err := os.WriteFile(filepath.Join(root, LocalNotesFile), []byte(notes), 0o644); err != nil {
		t.Fatalf("seed workspace local notes: %v", err)
	}
	if _, err := Sync(Options{Root: root}); err != nil {
		t.Fatalf("Sync: %v", err)
	}
	agents, _ := os.ReadFile(filepath.Join(root, "AGENTS.md"))
	if !strings.Contains(string(agents), "## Local Notes") || !strings.Contains(string(agents), "review service boundaries") {
		t.Errorf("AGENTS.md missing workspace Local Notes section")
	}
	projectContext, _ := os.ReadFile(filepath.Join(root, ".claude/generated/project-context.md"))
	if strings.Contains(string(projectContext), "Local Notes") || strings.Contains(string(projectContext), "review service boundaries") {
		t.Errorf("workspace project-context.md must not include AGENTS.local.md content")
	}
}

func TestSyncWorkspaceDryRunWritesNothing(t *testing.T) {
	root := t.TempDir()
	writeWorkspace(t, root)
	res, err := Sync(Options{Root: root, DryRun: true})
	if err != nil {
		t.Fatalf("Sync: %v", err)
	}
	if len(res.Written) != 0 {
		t.Errorf("DryRun must not write; got %v", res.Written)
	}
	if len(res.Skipped) != len(targets()) {
		t.Errorf("DryRun should skip all targets; got %v", res.Skipped)
	}
	if _, err := os.Stat(filepath.Join(root, "AGENTS.md")); !os.IsNotExist(err) {
		t.Errorf("AGENTS.md should not exist after dry run")
	}
}

func TestSyncWorkspaceZhLangPicksZhDoc(t *testing.T) {
	root := t.TempDir()
	writeWorkspace(t, root)
	if _, err := Sync(Options{Root: root, Lang: LangZhCN}); err != nil {
		t.Fatalf("Sync: %v", err)
	}
	projectContext, _ := os.ReadFile(filepath.Join(root, ".claude/generated/project-context.md"))
	if !strings.Contains(string(projectContext), "micro 工作区 profile 对应 `ncgo new --mode micro` 创建的仓库根目录") {
		t.Errorf("zh-CN workspace sync should render Chinese micro overview in project-context.md")
	}
}

func TestSyncWorkspaceFailsWhenListedServiceManifestIsMissing(t *testing.T) {
	root := t.TempDir()
	if err := manifest.SaveWorkspace(root, &manifest.Workspace{
		Ncgo:   manifest.Meta{Version: "0.1.0-dev", AssetsVersion: "test"},
		Mode:   manifest.ModeMicro,
		Name:   "commerce",
		Module: "github.com/acme/commerce",
		Services: []manifest.WorkspaceService{
			{Name: "user-rpc", Kind: manifest.KindKitex, Dir: "services/user-rpc"},
		},
	}); err != nil {
		t.Fatalf("manifest.SaveWorkspace: %v", err)
	}
	_, err := Sync(Options{Root: root})
	if err == nil {
		t.Fatalf("expected workspace sync to fail when a listed service manifest is missing")
	}
	if !strings.Contains(err.Error(), "load workspace service user-rpc") {
		t.Fatalf("unexpected error: %v", err)
	}
}
