// Package ai renders and bootstraps AI collaboration artifacts described in
// docs/prd.md §6, including long-form context files, generated `.claude`
// facts, and hand-authored `.claude` starter files.
//
// The renderer is idempotent and reads only from project metadata
// (`.ncgo/manifest.yaml` or `ncgo.workspace`) and the ncgo-embedded design
// doc (`internal/assets/_data/docs/<profile>/design-doc.<lang>.md`); it
// never reads back its own previous output.
// Each managed file carries an `<!-- ncgo:managed -->` marker on the
// first content line; existing files without the marker are refused
// unless Force is set.
package ai

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/byx-darwin/ncgo/internal/assets"
	"github.com/byx-darwin/ncgo/internal/manifest"
)

const (
	// ManagedMarker is written as the first line of every rendered file.
	// `ncgo ai sync` refuses to overwrite a file without this marker
	// unless the caller passes Force.
	ManagedMarker = "<!-- ncgo:managed -->"

	// LocalNotesFile is the optional, user-owned companion appended under
	// the "Local Notes" section of the long-form rendered targets when
	// present.
	LocalNotesFile = "AGENTS.local.md"

	LangEN   = "en"
	LangZhCN = "zh-CN"
)

// Target groups for `ncgo ai sync --target`.
const (
	TargetAll    = "all"
	TargetAgents = "agents"
	TargetClaude = "claude"
	TargetCursor = "cursor"
)

// Options controls a Sync invocation.
type Options struct {
	Root   string // service root with .ncgo/manifest.yaml or micro workspace root with ncgo.workspace
	Lang   string // "en" (default) or "zh-CN"
	Target string // target group to render; empty defaults to claude (all = every group)
	Force  bool   // overwrite non-managed files
	DryRun bool   // do not write; only report intended actions
}

type syncScope string

const (
	syncScopeService   syncScope = "service"
	syncScopeWorkspace syncScope = "workspace"
)

type workspaceServiceFacts struct {
	Name         string
	Kind         string
	Dir          string
	Module       string
	IDL          string
	WithDatabase bool
	Infra        []string
	Domains      []string
}

type serviceWorkspaceMembership struct {
	RootRel    string
	Name       string
	Module     string
	ServiceDir string
}

type syncSource struct {
	Scope             syncScope
	SourceRef         string
	DesignDoc         string
	Service           *manifest.Manifest
	ServiceWorkspace  *serviceWorkspaceMembership
	Workspace         *manifest.Workspace
	WorkspaceServices []workspaceServiceFacts
}

// ResultWorkspace summarizes workspace facts attached to an ai sync result.
type ResultWorkspace struct {
	Role         string `json:"role"`
	Name         string `json:"name"`
	Module       string `json:"module"`
	Root         string `json:"root,omitempty"`
	ServiceDir   string `json:"serviceDir,omitempty"`
	ServiceCount int    `json:"serviceCount,omitempty"`
}

// Result is the structured outcome of an AI file generation/bootstrap call.
type Result struct {
	Written   []string         `json:"written"`             // relative paths actually written
	Skipped   []Skip           `json:"skipped"`             // relative paths intentionally not written
	Notes     []string         `json:"notes,omitempty"`     // optional informational lines for CLI summaries
	NextSteps []string         `json:"nextSteps,omitempty"` // optional follow-up commands or actions
	Target    string           `json:"target,omitempty"`    // target group rendered (all|agents|claude|cursor)
	Scope     string           `json:"scope,omitempty"`     // service | workspace
	SourceRef string           `json:"sourceRef,omitempty"` // .ncgo/manifest.yaml | ncgo.workspace
	Workspace *ResultWorkspace `json:"workspace,omitempty"`
}

// Skip describes a file an AI helper chose not to write and why.
type Skip struct {
	Path   string `json:"path"`
	Reason string `json:"reason"`
}

// Sync renders all managed AI artifacts under opts.Root.
func Sync(opts Options) (*Result, error) {
	if opts.Root == "" {
		opts.Root = "."
	}
	if opts.Lang == "" {
		opts.Lang = LangEN
	}
	if opts.Target == "" {
		opts.Target = TargetClaude
	}
	if err := validateTarget(opts.Target); err != nil {
		return nil, err
	}
	if opts.Lang != LangEN && opts.Lang != LangZhCN {
		return nil, fmt.Errorf("ai sync: --lang %q is invalid (en|zh-CN)", opts.Lang)
	}
	source, err := resolveSyncSource(opts.Root, opts.Lang)
	if err != nil {
		return nil, err
	}
	local, err := readLocalNotes(opts.Root)
	if err != nil {
		return nil, err
	}
	inputs := buildInputs(source, local, opts.Lang)
	res := newSyncResult(source)
	res.Target = opts.Target
	for _, t := range targets() {
		if opts.Target != TargetAll && t.Group != opts.Target {
			continue
		}
		if err := writeTarget(opts, t, inputs, res); err != nil {
			return res, err
		}
	}
	profile := resolveProfile(source)
	if err := writeStandaloneDocs(opts, res, profile); err != nil {
		return res, err
	}
	return res, nil
}

// validateTarget reports whether the requested --target group is known.
func validateTarget(target string) error {
	switch target {
	case TargetAll, TargetAgents, TargetClaude, TargetCursor:
		return nil
	default:
		return fmt.Errorf("ai sync: --target %q is invalid (all|agents|claude|cursor)", target)
	}
}

// readDesignDoc fetches the embedded design doc that matches the requested
// profile (`hertz`, `kitex`, or `micro`) and language.
func readDesignDoc(profile, lang string) (string, error) {
	rel := filepath.ToSlash(filepath.Join("docs", profile, "design-doc."+lang+".md"))
	b, err := fs.ReadFile(assets.FS(), rel)
	if err != nil {
		return "", fmt.Errorf("ai sync: read embedded %s: %w", rel, err)
	}
	return string(b), nil
}

func resolveSyncSource(root, lang string) (syncSource, error) {
	manifestPath := manifest.Path(root)
	workspacePath := manifest.WorkspacePath(root)
	if pathExists(manifestPath) {
		m, err := manifest.Load(root)
		if err != nil {
			return syncSource{}, err
		}
		membership, err := findServiceWorkspaceMembership(root)
		if err != nil {
			return syncSource{}, err
		}
		doc, err := readDesignDoc(m.Service.Kind, lang)
		if err != nil {
			return syncSource{}, err
		}
		return syncSource{
			Scope:            syncScopeService,
			SourceRef:        ".ncgo/manifest.yaml",
			DesignDoc:        doc,
			Service:          m,
			ServiceWorkspace: membership,
		}, nil
	}
	if pathExists(workspacePath) {
		w, err := manifest.LoadWorkspace(root)
		if err != nil {
			return syncSource{}, err
		}
		services, err := loadWorkspaceServiceFacts(root, w)
		if err != nil {
			return syncSource{}, err
		}
		doc, err := readDesignDoc(manifest.ModeMicro, lang)
		if err != nil {
			return syncSource{}, err
		}
		return syncSource{
			Scope:             syncScopeWorkspace,
			SourceRef:         manifest.WorkspaceFileName,
			DesignDoc:         doc,
			Workspace:         w,
			WorkspaceServices: services,
		}, nil
	}
	_, err := manifest.Load(root)
	return syncSource{}, err
}

func loadWorkspaceServiceFacts(root string, w *manifest.Workspace) ([]workspaceServiceFacts, error) {
	out := make([]workspaceServiceFacts, 0, len(w.Services))
	for _, svc := range w.Services {
		serviceRoot := filepath.Join(root, filepath.FromSlash(svc.Dir))
		m, err := manifest.Load(serviceRoot)
		if err != nil {
			return nil, fmt.Errorf("ai sync: load workspace service %s: %w", svc.Name, err)
		}
		out = append(out, workspaceServiceFacts{
			Name:         svc.Name,
			Kind:         svc.Kind,
			Dir:          filepath.ToSlash(filepath.Clean(svc.Dir)),
			Module:       m.Module,
			IDL:          m.Service.IDL,
			WithDatabase: m.Service.WithDatabase,
			Infra:        append([]string(nil), m.Infra...),
			Domains:      append([]string(nil), m.Domains...),
		})
	}
	return out, nil
}

// ResultFields exposes the stable top-level fields that structured transports
// such as MCP can surface directly without asking callers to parse text output.
func ResultFields(res *Result) map[string]any {
	if res == nil {
		return nil
	}
	fields := map[string]any{
		"written": res.Written,
		"skipped": res.Skipped,
	}
	if len(res.Notes) > 0 {
		fields["notes"] = res.Notes
	}
	if len(res.NextSteps) > 0 {
		fields["nextSteps"] = res.NextSteps
	}
	if res.Scope != "" {
		fields["scope"] = res.Scope
	}
	if res.SourceRef != "" {
		fields["sourceRef"] = res.SourceRef
	}
	if res.Workspace != nil {
		fields["workspace"] = res.Workspace
	}
	return fields
}

func syncNotes(source syncSource) []string {
	switch source.Scope {
	case syncScopeWorkspace:
		return []string{
			"detected micro workspace root; rendered workspace-level AI context from `ncgo.workspace`",
			"for service-level context, run `ncgo ai sync --root services/<name>` inside a generated service directory",
		}
	case syncScopeService:
		if source.ServiceWorkspace == nil {
			return nil
		}
		return []string{
			fmt.Sprintf("detected parent micro workspace `%s` for this service root", source.ServiceWorkspace.RootRel),
			fmt.Sprintf("this service is registered in workspace `%s` as `%s`", source.ServiceWorkspace.Name, source.ServiceWorkspace.ServiceDir),
		}
	default:
		return nil
	}
}

func newSyncResult(source syncSource) *Result {
	res := &Result{
		Written:   []string{},
		Skipped:   []Skip{},
		Notes:     syncNotes(source),
		Scope:     string(source.Scope),
		SourceRef: source.SourceRef,
	}
	switch source.Scope {
	case syncScopeWorkspace:
		if source.Workspace != nil {
			res.Workspace = &ResultWorkspace{
				Role:         "root",
				Name:         source.Workspace.Name,
				Module:       source.Workspace.Module,
				ServiceCount: len(source.WorkspaceServices),
			}
		}
	case syncScopeService:
		if source.ServiceWorkspace != nil {
			res.Workspace = &ResultWorkspace{
				Role:       "member",
				Name:       source.ServiceWorkspace.Name,
				Module:     source.ServiceWorkspace.Module,
				Root:       source.ServiceWorkspace.RootRel,
				ServiceDir: source.ServiceWorkspace.ServiceDir,
			}
		}
	}
	return res
}

func pathExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func findServiceWorkspaceMembership(root string) (*serviceWorkspaceMembership, error) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("ai sync: resolve root %s: %w", root, err)
	}
	for dir := filepath.Dir(absRoot); ; {
		workspacePath := manifest.WorkspacePath(dir)
		if pathExists(workspacePath) {
			w, err := manifest.LoadWorkspace(dir)
			if err != nil {
				return nil, fmt.Errorf("ai sync: load parent workspace %s: %w", dir, err)
			}
			serviceRel, err := filepath.Rel(dir, absRoot)
			if err != nil {
				return nil, fmt.Errorf("ai sync: relate %s to %s: %w", dir, absRoot, err)
			}
			serviceRel = filepath.ToSlash(filepath.Clean(serviceRel))
			for _, svc := range w.Services {
				if filepath.ToSlash(filepath.Clean(svc.Dir)) != serviceRel {
					continue
				}
				rootRel, err := filepath.Rel(absRoot, dir)
				if err != nil {
					return nil, fmt.Errorf("ai sync: relate %s to %s: %w", absRoot, dir, err)
				}
				return &serviceWorkspaceMembership{
					RootRel:    filepath.ToSlash(filepath.Clean(rootRel)),
					Name:       w.Name,
					Module:     w.Module,
					ServiceDir: serviceRel,
				}, nil
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return nil, nil
		}
		dir = parent
	}
}

// readLocalNotes returns the contents of <root>/AGENTS.local.md when it
// exists, or "" when missing. Any other read error is returned.
func readLocalNotes(root string) (string, error) {
	p := filepath.Join(root, LocalNotesFile)
	b, err := os.ReadFile(p)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return "", nil
		}
		return "", fmt.Errorf("ai sync: read %s: %w", p, err)
	}
	return string(b), nil
}

// safeWritePath validates that writing relPath under root cannot escape root
// through symlinks. It returns the canonical absolute write path and, when the
// path is unsafe, a skip reason instead. Force must not bypass this boundary,
// because a symlink escape can overwrite files outside the user's project.
func safeWritePath(root, relPath string) (full string, skip string, err error) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return "", "", err
	}
	resolvedRoot, rerr := filepath.EvalSymlinks(absRoot)
	if rerr != nil {
		resolvedRoot = filepath.Clean(absRoot)
	}
	full = filepath.Join(resolvedRoot, filepath.Clean(relPath))
	rel, err := filepath.Rel(resolvedRoot, full)
	if err != nil {
		return "", "", err
	}
	if rel == "." {
		return full, "", nil
	}
	// Walk each path component under the resolved root. A component that is a
	// symlink must resolve back inside root; a dangling symlink at any level
	// must not be written through. Missing components are safe to create once
	// every existing ancestor has been verified.
	cur := resolvedRoot
	for part := range strings.SplitSeq(rel, string(filepath.Separator)) {
		cur = filepath.Join(cur, part)
		li, lerr := os.Lstat(cur)
		if errors.Is(lerr, fs.ErrNotExist) {
			continue
		}
		if lerr != nil {
			return "", "", fmt.Errorf("ai sync: lstat %s: %w", cur, lerr)
		}
		if li.Mode()&os.ModeSymlink == 0 {
			continue
		}
		target, terr := filepath.EvalSymlinks(cur)
		if terr != nil {
			return "", fmt.Sprintf("refusing to write through dangling symlink %s", part), nil
		}
		if !pathWithin(resolvedRoot, target) {
			return "", fmt.Sprintf("refusing to write through symlink outside project root: %s", part), nil
		}
		cur = target
	}
	return full, "", nil
}

// pathWithin reports whether p is inside root (or equals root).
func pathWithin(root, p string) bool {
	rel, err := filepath.Rel(root, p)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// writeTarget renders one target file relative to opts.Root, honouring
// the managed-marker / Force / DryRun rules. The inputs argument contains
// the pre-rendered shared bodies produced by buildInputs.
func writeTarget(opts Options, t target, inputs renderInputs, res *Result) error {
	full, skip, err := safeWritePath(opts.Root, t.RelPath)
	if err != nil {
		return fmt.Errorf("ai sync: %w", err)
	}
	if skip != "" {
		res.Skipped = append(res.Skipped, Skip{Path: t.RelPath, Reason: skip})
		return nil
	}
	rendered := t.Render(inputs)
	if existing, err := os.ReadFile(full); err == nil {
		if !isManaged(existing) && !opts.Force {
			res.Skipped = append(res.Skipped, Skip{
				Path:   t.RelPath,
				Reason: "exists without ncgo:managed marker; pass --force to overwrite",
			})
			return nil
		}
	} else if !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("ai sync: stat %s: %w", full, err)
	}
	if opts.DryRun {
		res.Skipped = append(res.Skipped, Skip{Path: t.RelPath, Reason: "dry-run"})
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return fmt.Errorf("ai sync: mkdir %s: %w", filepath.Dir(full), err)
	}
	if err := os.WriteFile(full, []byte(rendered), 0o644); err != nil {
		return fmt.Errorf("ai sync: write %s: %w", full, err)
	}
	res.Written = append(res.Written, t.RelPath)
	return nil
}

// docSpec describes one standalone design-doc file to materialize.
type docSpec struct {
	AssetPath string // path inside embedded assets, e.g. "docs/hertz/design-doc.en.md"
	RelPath   string // relative output path from project root, e.g. "docs/ncgo/hertz/design-doc.en.md"
}

// writeStandaloneDocs generates standalone design-doc files to docs/ncgo/
// in the user project, with cross-link rewriting.
func writeStandaloneDocs(opts Options, res *Result, profile string) error {
	for _, spec := range listDocSpecs(profile, opts.Lang) {
		full, skip, err := safeWritePath(opts.Root, spec.RelPath)
		if err != nil {
			return fmt.Errorf("ai sync: %w", err)
		}
		if skip != "" {
			res.Skipped = append(res.Skipped, Skip{Path: spec.RelPath, Reason: skip})
			continue
		}
		if opts.DryRun {
			res.Skipped = append(res.Skipped, Skip{Path: spec.RelPath, Reason: "dry-run"})
			continue
		}
		b, err := fs.ReadFile(assets.FS(), spec.AssetPath)
		if err != nil {
			return fmt.Errorf("ai sync: read embedded %s: %w", spec.AssetPath, err)
		}
		// Mark the materialized doc as managed so a later sync refreshes it
		// instead of treating it as a user-owned file without the marker.
		content := ManagedMarker + "\n" + rewriteDocLinks(string(b))
		if existing, err := os.ReadFile(full); err == nil {
			if !isManaged(existing) && !opts.Force {
				res.Skipped = append(res.Skipped, Skip{
					Path:   spec.RelPath,
					Reason: "exists without ncgo:managed marker; pass --force to overwrite",
				})
				continue
			}
		} else if !errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("ai sync: check %s: %w", spec.RelPath, err)
		}
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			return fmt.Errorf("ai sync: mkdir %s: %w", filepath.Dir(full), err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			return fmt.Errorf("ai sync: write %s: %w", full, err)
		}
		res.Written = append(res.Written, spec.RelPath)
	}
	return nil
}

// listDocSpecs returns the set of docSpec entries to materialize for a given
// profile and language, including the primary doc and cross-profile docs.
func listDocSpecs(profile, lang string) []docSpec {
	specs := []docSpec{
		{
			AssetPath: "docs/" + profile + "/design-doc." + lang + ".md",
			RelPath:   "docs/ncgo/" + profile + "/design-doc." + lang + ".md",
		},
	}
	if profile == manifest.KindHertz {
		specs = append(specs, docSpec{
			AssetPath: "docs/hertz/rate-limit-dynamic-design." + lang + ".md",
			RelPath:   "docs/ncgo/hertz/rate-limit-dynamic-design." + lang + ".md",
		})
	}
	for _, p := range crossProfiles(profile) {
		specs = append(specs, docSpec{
			AssetPath: "docs/" + p + "/design-doc." + lang + ".md",
			RelPath:   "docs/ncgo/" + p + "/design-doc." + lang + ".md",
		})
	}
	return specs
}

// crossProfiles returns the set of profiles that should get a cross-linked
// design doc for the given primary profile.
func crossProfiles(profile string) []string {
	switch profile {
	case manifest.KindHertz:
		return []string{manifest.KindKitex}
	case manifest.KindKitex:
		return []string{manifest.KindHertz}
	case manifest.ModeMicro:
		return []string{manifest.KindHertz, manifest.KindKitex}
	default:
		return nil
	}
}

// rewriteDocLinks rewrites absolute doc cross-links so that they point to the
// materialized docs/ncgo/<profile>/ layout. Because docs/ncgo/ mirrors the
// source docs/<profile>/ hierarchy, existing ../<profile>/ sibling links stay
// correct and are preserved verbatim; only the bare docs/<profile>/ display
// paths are remapped to ../<profile>/.
func rewriteDocLinks(content string) string {
	for _, origProfile := range []string{"hertz", "kitex", "micro"} {
		oldAbs := "docs/" + origProfile + "/"
		newRel := "../" + origProfile + "/"
		content = strings.ReplaceAll(content, oldAbs, newRel)
	}
	return content
}

// resolveProfile determines the profile string from the sync source.
func resolveProfile(source syncSource) string {
	switch source.Scope {
	case syncScopeService:
		return source.Service.Service.Kind
	default:
		return manifest.ModeMicro
	}
}
