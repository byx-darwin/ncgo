package ai

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
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
	standalonePaths := []string{
		"docs/ncgo/hertz/design-doc.en.md",
		"docs/ncgo/hertz/rate-limit-dynamic-design.en.md",
		"docs/ncgo/kitex/design-doc.en.md",
	}
	allWantPaths := append(wantPaths, standalonePaths...)
	if len(res.Written) != len(allWantPaths) {
		t.Fatalf("Written = %v, want %v", res.Written, allWantPaths)
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

func TestSyncRefusesSymlinkEscape(t *testing.T) {
	root := t.TempDir()
	outside := filepath.Join(t.TempDir(), "outside.txt")
	if err := os.WriteFile(outside, []byte("sensitive\n"), 0o644); err != nil {
		t.Fatalf("seed outside file: %v", err)
	}
	writeManifest(t, root, manifest.KindHertz)
	if err := os.Symlink(outside, filepath.Join(root, "AGENTS.md")); err != nil {
		t.Fatalf("symlink AGENTS.md: %v", err)
	}
	res, err := Sync(Options{Root: root})
	if err != nil {
		t.Fatalf("Sync: %v", err)
	}
	var skipped bool
	for _, s := range res.Skipped {
		if s.Path == "AGENTS.md" && strings.Contains(s.Reason, "symlink") {
			skipped = true
		}
	}
	if !skipped {
		t.Fatalf("expected AGENTS.md skip due to symlink escape; got %+v", res.Skipped)
	}
	b, _ := os.ReadFile(outside)
	if string(b) != "sensitive\n" {
		t.Errorf("outside file must not be overwritten through symlink; got %q", string(b))
	}
}

func TestSyncForceStillRefusesSymlinkEscape(t *testing.T) {
	root := t.TempDir()
	outside := filepath.Join(t.TempDir(), "outside.txt")
	if err := os.WriteFile(outside, []byte("sensitive\n"), 0o644); err != nil {
		t.Fatalf("seed outside file: %v", err)
	}
	writeManifest(t, root, manifest.KindHertz)
	if err := os.Symlink(outside, filepath.Join(root, "AGENTS.md")); err != nil {
		t.Fatalf("symlink AGENTS.md: %v", err)
	}
	res, err := Sync(Options{Root: root, Force: true})
	if err != nil {
		t.Fatalf("Sync: %v", err)
	}
	var skipped bool
	for _, s := range res.Skipped {
		if s.Path == "AGENTS.md" && strings.Contains(s.Reason, "symlink") {
			skipped = true
		}
	}
	if !skipped {
		t.Fatalf("expected AGENTS.md skip even with --force; got %+v", res.Skipped)
	}
	b, _ := os.ReadFile(outside)
	if string(b) != "sensitive\n" {
		t.Errorf("outside file must not be overwritten even with --force; got %q", string(b))
	}
}

func TestSyncRefusesDanglingSymlink(t *testing.T) {
	root := t.TempDir()
	outside := filepath.Join(t.TempDir(), "not-created.txt")
	writeManifest(t, root, manifest.KindHertz)
	if err := os.Symlink(outside, filepath.Join(root, "AGENTS.md")); err != nil {
		t.Fatalf("symlink AGENTS.md: %v", err)
	}
	res, err := Sync(Options{Root: root})
	if err != nil {
		t.Fatalf("Sync: %v", err)
	}
	var skipped bool
	for _, s := range res.Skipped {
		if s.Path == "AGENTS.md" && strings.Contains(s.Reason, "symlink") {
			skipped = true
		}
	}
	if !skipped {
		t.Fatalf("expected AGENTS.md skip due to dangling symlink; got %+v", res.Skipped)
	}
	if _, err := os.Stat(outside); !os.IsNotExist(err) {
		t.Errorf("outside target must not be created through dangling symlink")
	}
}

func TestSyncAllowsSymlinkWithinRoot(t *testing.T) {
	root := t.TempDir()
	writeManifest(t, root, manifest.KindHertz)
	target := filepath.Join(root, "target-agents.md")
	if err := os.WriteFile(target, []byte(ManagedMarker+"\n\n# old\n"), 0o644); err != nil {
		t.Fatalf("seed target: %v", err)
	}
	if err := os.Symlink(target, filepath.Join(root, "AGENTS.md")); err != nil {
		t.Fatalf("symlink AGENTS.md: %v", err)
	}
	res, err := Sync(Options{Root: root})
	if err != nil {
		t.Fatalf("Sync: %v", err)
	}
	for _, s := range res.Skipped {
		if s.Path == "AGENTS.md" {
			t.Fatalf("AGENTS.md should not be skipped for within-root symlink; got %+v", res.Skipped)
		}
	}
	b, _ := os.ReadFile(target)
	if !strings.Contains(string(b), ManagedMarker) {
		t.Errorf("within-root symlink target should receive managed content")
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
	skipped := map[string]bool{}
	for _, s := range res.Skipped {
		skipped[s.Path] = s.Reason == "dry-run"
	}
	for _, tgt := range targets() {
		if skipped[tgt.RelPath] {
			continue
		}
		t.Errorf("dry-run should report %s as skipped; got %+v", tgt.RelPath, res.Skipped)
	}
	if !skipped["docs/ncgo/hertz/design-doc.en.md"] {
		t.Errorf("dry-run should also report standalone docs; got %+v", res.Skipped)
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
	standalonePaths := []string{
		"docs/ncgo/micro/design-doc.en.md",
		"docs/ncgo/hertz/design-doc.en.md",
		"docs/ncgo/kitex/design-doc.en.md",
	}
	allWantPaths := append(wantPaths, standalonePaths...)
	if len(res.Written) != len(allWantPaths) {
		t.Fatalf("Written = %v, want %v", res.Written, allWantPaths)
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
	skipped := map[string]bool{}
	for _, s := range res.Skipped {
		skipped[s.Path] = s.Reason == "dry-run"
	}
	for _, tgt := range targets() {
		if skipped[tgt.RelPath] {
			continue
		}
		t.Errorf("dry-run should report %s as skipped; got %+v", tgt.RelPath, res.Skipped)
	}
	if !skipped["docs/ncgo/micro/design-doc.en.md"] {
		t.Errorf("dry-run should also report standalone docs; got %+v", res.Skipped)
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

func TestSyncWritesStandaloneDocs(t *testing.T) {
	root := t.TempDir()
	writeManifest(t, root, manifest.KindHertz)
	_, err := Sync(Options{Root: root})
	if err != nil {
		t.Fatalf("Sync: %v", err)
	}
	p := filepath.Join(root, "docs", "ncgo", "hertz", "design-doc.en.md")
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("read %s: %v", p, err)
	}
	body := string(b)
	if !strings.Contains(body, "Hertz Template Design Doc") {
		t.Errorf("standalone design-doc missing title; got %d bytes", len(body))
	}
	if strings.Contains(body, "docs/hertz/") {
		t.Errorf("standalone design-doc still contains original absolute doc links")
	}
	kp := filepath.Join(root, "docs", "ncgo", "kitex", "design-doc.en.md")
	if _, err := os.Stat(kp); os.IsNotExist(err) {
		t.Errorf("cross-profile kitex/design-doc.en.md not generated for hertz project")
	}
}

func TestSyncRefreshesStandaloneDocsOnSecondPass(t *testing.T) {
	root := t.TempDir()
	writeManifest(t, root, manifest.KindHertz)
	if _, err := Sync(Options{Root: root}); err != nil {
		t.Fatalf("first Sync: %v", err)
	}
	res, err := Sync(Options{Root: root})
	if err != nil {
		t.Fatalf("second Sync: %v", err)
	}
	p := filepath.Join(root, "docs", "ncgo", "hertz", "design-doc.en.md")
	if _, err := os.Stat(p); err != nil {
		t.Fatalf("standalone doc missing after second sync: %v", err)
	}
	const docRel = "docs/ncgo/hertz/design-doc.en.md"
	var written bool
	for _, w := range res.Written {
		if w == docRel {
			written = true
		}
	}
	if !written {
		t.Fatalf("standalone doc should be rewritten on second sync; Written=%v Skipped=%v", res.Written, res.Skipped)
	}
	b, _ := os.ReadFile(p)
	if !strings.Contains(string(b), ManagedMarker) {
		t.Errorf("standalone doc missing managed marker")
	}
}

func TestSyncWritesStandaloneDocsForKitex(t *testing.T) {
	root := t.TempDir()
	writeManifest(t, root, manifest.KindKitex)
	_, err := Sync(Options{Root: root})
	if err != nil {
		t.Fatalf("Sync: %v", err)
	}
	// Primary kitex design-doc
	p := filepath.Join(root, "docs", "ncgo", "kitex", "design-doc.en.md")
	if _, err := os.Stat(p); os.IsNotExist(err) {
		t.Errorf("kitex design-doc not generated: %s", p)
	}
	// Cross-profile hertz design-doc (for links)
	hp := filepath.Join(root, "docs", "ncgo", "hertz", "design-doc.en.md")
	if _, err := os.Stat(hp); os.IsNotExist(err) {
		t.Errorf("cross-profile hertz design-doc not generated for kitex project")
	}
	// rate-limit-dynamic-design should NOT be generated for kitex
	rl := filepath.Join(root, "docs", "ncgo", "kitex", "rate-limit-dynamic-design.en.md")
	if _, err := os.Stat(rl); !os.IsNotExist(err) {
		t.Errorf("rate-limit-dynamic-design should not be generated for kitex profile")
	}
}

func TestSyncWritesStandaloneDocsZhCN(t *testing.T) {
	root := t.TempDir()
	writeManifest(t, root, manifest.KindHertz)
	_, err := Sync(Options{Root: root, Lang: LangZhCN})
	if err != nil {
		t.Fatalf("Sync: %v", err)
	}
	p := filepath.Join(root, "docs", "ncgo", "hertz", "design-doc.zh-CN.md")
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("read %s: %v", p, err)
	}
	body := string(b)
	if !strings.Contains(body, "Hertz 模板详细设计") {
		t.Errorf("zh-CN standalone doc missing Chinese title")
	}
	// Check links are rewritten to sibling profile paths. Note ../<profile>/
	// contains ./.<profile>/ as a substring, so only exact source-style
	// (docs/<profile>/) paths and the preserved sibling form are asserted;
	// resolvability is covered by TestStandaloneDocHrefsResolve.
	if strings.Contains(body, "docs/kitex/") {
		t.Errorf("zh-CN standalone doc still contains original absolute kitex links")
	}
	if !strings.Contains(body, "../kitex/") {
		t.Errorf("zh-CN standalone doc missing preserved sibling kitex link")
	}
}

func TestSyncDryRunWritesNoStandaloneDocs(t *testing.T) {
	root := t.TempDir()
	writeManifest(t, root, manifest.KindHertz)
	res, err := Sync(Options{Root: root, DryRun: true})
	if err != nil {
		t.Fatalf("Sync: %v", err)
	}
	if len(res.Written) != 0 {
		t.Errorf("DryRun must not write; got %v", res.Written)
	}
	var reported bool
	for _, s := range res.Skipped {
		if s.Path == "docs/ncgo/hertz/design-doc.en.md" && s.Reason == "dry-run" {
			reported = true
		}
	}
	if !reported {
		t.Errorf("dry-run should report standalone docs as skipped; got %+v", res.Skipped)
	}
	p := filepath.Join(root, "docs", "ncgo", "hertz", "design-doc.en.md")
	if _, err := os.Stat(p); !os.IsNotExist(err) {
		t.Errorf("standalone doc should not exist after dry run")
	}
}

// TestStandaloneDocHrefsResolve verifies that every relative markdown href in
// the generated design docs resolves to a real file in the project, guarding
// against link-rewrite regressions (e.g. ../<profile>/ turned into ./<profile>/).
func TestStandaloneDocHrefsResolve(t *testing.T) {
	root := t.TempDir()
	writeManifest(t, root, manifest.KindHertz)
	if _, err := Sync(Options{Root: root}); err != nil {
		t.Fatalf("Sync: %v", err)
	}
	var docPaths []string
	err := filepath.WalkDir(filepath.Join(root, "docs", "ncgo"), func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		// Only design docs participate in cross-profile links; rate-limit docs
		// legitimately reference language variants that a single sync does not
		// materialize.
		if !d.IsDir() && strings.HasPrefix(d.Name(), "design-doc.") {
			docPaths = append(docPaths, path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk docs/ncgo: %v", err)
	}
	if len(docPaths) == 0 {
		t.Fatalf("no design docs generated")
	}
	hrefRE := regexp.MustCompile(`\]\(([^)]+)\)`)
	for _, doc := range docPaths {
		b, err := os.ReadFile(doc)
		if err != nil {
			t.Fatalf("read %s: %v", doc, err)
		}
		for _, m := range hrefRE.FindAllStringSubmatch(string(b), -1) {
			href := m[1]
			// Skip external/anchor targets and Go call syntax like
			// do.MustInvoke[*data.Data](inj, startupCtx) — real doc links
			// are path-shaped (contain a "/").
			if !strings.Contains(href, "/") {
				continue
			}
			if strings.HasPrefix(href, "http://") || strings.HasPrefix(href, "https://") ||
				strings.HasPrefix(href, "#") || strings.HasPrefix(href, "mailto:") {
				continue
			}
			target := filepath.Join(filepath.Dir(doc), filepath.FromSlash(href))
			if _, err := os.Stat(target); err != nil {
				t.Errorf("%s: href %q does not resolve to %s", doc, href, target)
			}
		}
	}
}

func TestRewriteDocLinks(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "absolute hertz link",
			input:    "see `docs/hertz/rate-limit-dynamic-design.en.md`",
			expected: "see `../hertz/rate-limit-dynamic-design.en.md`",
		},
		{
			name:     "relative kitex link stays a sibling",
			input:    "[kitex](../kitex/design-doc.en.md)",
			expected: "[kitex](../kitex/design-doc.en.md)",
		},
		{
			name:     "absolute kitex link in hertz doc",
			input:    "[kitex docs](docs/kitex/design-doc.en.md)",
			expected: "[kitex docs](../kitex/design-doc.en.md)",
		},
		{
			name:     "no links unchanged",
			input:    "plain text with no links",
			expected: "plain text with no links",
		},
		{
			name:     "absolute micro link",
			input:    "see `docs/micro/design-doc.en.md`",
			expected: "see `../micro/design-doc.en.md`",
		},
		{
			name:     "relative micro link stays a sibling",
			input:    "[micro docs](../micro/design-doc.en.md)",
			expected: "[micro docs](../micro/design-doc.en.md)",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := rewriteDocLinks(tt.input)
			if got != tt.expected {
				t.Errorf("rewriteDocLinks() = %q, want %q", got, tt.expected)
			}
		})
	}
}
