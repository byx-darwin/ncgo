// Package ai renders and bootstraps AI collaboration artifacts described in
// docs/prd.md §6, including long-form context files, generated `.claude`
// facts, and hand-authored `.claude` starter files.
//
// The renderer is idempotent and reads only from the project manifest
// and the ncgo-embedded design doc (`internal/assets/_data/docs/<kind>/
// design-doc.<lang>.md`); it never reads back its own previous output.
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

// Options controls a Sync invocation.
type Options struct {
	Root   string // project root containing .ncgo/manifest.yaml
	Lang   string // "en" (default) or "zh-CN"
	Force  bool   // overwrite non-managed files
	DryRun bool   // do not write; only report intended actions
}

// Result is the structured outcome of an AI file generation/bootstrap call.
type Result struct {
	Written []string // relative paths actually written
	Skipped []Skip   // relative paths intentionally not written
	Notes   []string // optional informational lines for CLI summaries
}

// Skip describes a file an AI helper chose not to write and why.
type Skip struct {
	Path   string
	Reason string
}

// Sync renders all managed AI artifacts under opts.Root.
func Sync(opts Options) (*Result, error) {
	if opts.Root == "" {
		opts.Root = "."
	}
	if opts.Lang == "" {
		opts.Lang = LangEN
	}
	if opts.Lang != LangEN && opts.Lang != LangZhCN {
		return nil, fmt.Errorf("ai sync: --lang %q is invalid (en|zh-CN)", opts.Lang)
	}
	m, err := manifest.Load(opts.Root)
	if err != nil {
		return nil, err
	}
	inputs, err := buildInputs(m, opts)
	if err != nil {
		return nil, err
	}
	res := &Result{}
	for _, t := range targets() {
		if err := writeTarget(opts, t, inputs, res); err != nil {
			return res, err
		}
	}
	return res, nil
}

// readDesignDoc fetches the embedded design doc that matches the
// project's service kind and the requested language.
func readDesignDoc(kind, lang string) (string, error) {
	rel := filepath.ToSlash(filepath.Join("docs", kind, "design-doc."+lang+".md"))
	b, err := fs.ReadFile(assets.FS(), rel)
	if err != nil {
		return "", fmt.Errorf("ai sync: read embedded %s: %w", rel, err)
	}
	return string(b), nil
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

// writeTarget renders one target file relative to opts.Root, honouring
// the managed-marker / Force / DryRun rules. The inputs argument contains
// the pre-rendered shared bodies produced by buildInputs.
func writeTarget(opts Options, t target, inputs renderInputs, res *Result) error {
	full := filepath.Join(opts.Root, t.RelPath)
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
