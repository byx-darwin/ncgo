// Package mono implements `ncgo new --mode mono` for both kinds of
// monolith service: a Hertz HTTP service (default) and a Kitex RPC
// service (Options.Kind = manifest.KindKitex).
//
// Generate is split into two phases:
//
//  1. Prepare: write the manifest, the IDL placeholder, and the kind's
//     custom template files under <dir>/template/. This phase has no
//     external tool dependency and is fully deterministic.
//  2. Run generator: optionally invoke `hz new` (hertz) or `kitex` (kitex)
//     via the injected Runner. Skipped when Options.NoGenerate is true.
//     Failures here surface as the original *exec.NotFoundError or
//     *exec.ExitError so callers can branch on them.
//
// With NoGenerate=true, the user/agent must run the generator command and
// initialise/tidy the module; those are listed in Result.NextSteps. When
// the generator ran successfully, Result.NextSteps only includes post-
// generation maintenance commands because hz/kitex already created go.mod.
package mono

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/byx-darwin/ncgo/internal/exec"
	"github.com/byx-darwin/ncgo/internal/manifest"
	scaffoldinfra "github.com/byx-darwin/ncgo/internal/scaffold/infra"
	"github.com/byx-darwin/ncgo/internal/scaffold/shared"
)

// Options describes a `ncgo new --mode mono` invocation.
type Options struct {
	Name          string      // service name; also default base directory
	Module        string      // Go module path
	Kind          string      // service kind: manifest.KindHertz (default) | manifest.KindKitex
	Dir           string      // target directory; must be empty or nonexistent
	WithDatabase  bool        // postgres scaffolding flag
	Infra         []string    // creation-time infra add-ons (currently only redis)
	IDL           string      // IDL path relative to project root; default differs by Kind
	AssetsVersion string      // recorded into manifest.ncgo.assets_version
	NCGOVersion   string      // recorded into manifest.ncgo.version
	NoGenerate    bool        // when true, skip the generator (hz/kitex) invocation
	Runner        exec.Runner // injected exec; nil means exec.NewDefault()
	Now           time.Time   // injected clock for golden tests; zero means time.Now().UTC()
}

// Result describes what Generate produced.
type Result struct {
	Dir         string   // resolved absolute target directory
	NextSteps   []string // shell commands the user/agent should run next
	RanGenerate bool     // true when hz was invoked successfully
}

var nameRE = regexp.MustCompile(`^[a-z][a-z0-9-]{0,62}$`)

// Generate prepares the directory and (unless NoGenerate) calls hz.
func Generate(ctx context.Context, opts Options) (*Result, error) {
	normalizedInfra, err := normalizeCreationInfra(opts.Infra)
	if err != nil {
		return nil, err
	}
	opts.Infra = normalizedInfra
	if err := opts.validate(); err != nil {
		return nil, err
	}
	dir, err := filepath.Abs(opts.Dir)
	if err != nil {
		return nil, fmt.Errorf("scaffold: resolve dir: %w", err)
	}
	if err := ensureEmptyDir(dir); err != nil {
		return nil, err
	}
	idl := opts.IDL
	if idl == "" {
		idl = defaultIDL(opts)
	}
	if err := writeTemplate(dir, opts); err != nil {
		return nil, err
	}
	if err := writeIDLPlaceholder(dir, idl, opts); err != nil {
		return nil, err
	}
	if err := writeManifest(dir, opts, idl); err != nil {
		return nil, err
	}
	if err := shared.WriteRepositoryHooks(dir); err != nil {
		return nil, err
	}
	res := &Result{Dir: dir, NextSteps: nextSteps(opts, idl)}
	if opts.NoGenerate {
		return res, nil
	}
	r := opts.Runner
	if r == nil {
		r = exec.NewDefault()
	}
	if _, err := runGenerator(ctx, r, dir, opts, idl); err != nil {
		return res, err
	}
	if err := addSelectedInfra(dir, opts.Infra); err != nil {
		return res, err
	}
	res.RanGenerate = true
	res.NextSteps = postGenerateNextSteps(opts)
	return res, nil
}

// defaultKind normalises an empty Kind to manifest.KindHertz so existing
// callers (and tests) that omit it keep their pre-Kind behaviour.
func defaultKind(k string) string {
	if k == "" {
		return manifest.KindHertz
	}
	return k
}

// defaultIDL returns the per-Kind default IDL path when Options.IDL is empty.
// Hertz uses the api.proto-importing app/ subdir; Kitex points at the
// service-named proto consumed by the kitex template Makefile.
func defaultIDL(opts Options) string {
	if defaultKind(opts.Kind) == manifest.KindKitex {
		return filepath.ToSlash(filepath.Join("idl", kitexIDLBase(opts)+".proto"))
	}
	return filepath.ToSlash(filepath.Join("idl", "app", opts.Name+".proto"))
}

// kitexIDLBase matches the generated Makefile's
// `{{.ServiceInfo.ServiceName | ToLower}}` convention. It also avoids
// invalid proto / Go identifiers for CLI names like "user-api".
func kitexIDLBase(opts Options) string {
	return strings.ToLower(exportName(opts.Name))
}

// runGenerator invokes hz or kitex depending on opts.Kind.
func runGenerator(ctx context.Context, r exec.Runner, dir string, opts Options, idl string) (exec.Result, error) {
	switch defaultKind(opts.Kind) {
	case manifest.KindKitex:
		return exec.Kitex(ctx, r, dir, kitexArgs(opts, idl)...)
	default:
		return exec.HZ(ctx, r, dir, hzArgs(opts, idl)...)
	}
}

func (o Options) validate() error {
	if !nameRE.MatchString(o.Name) {
		return fmt.Errorf("scaffold: name %q must match %s", o.Name, nameRE)
	}
	if o.Module == "" || !strings.Contains(o.Module, "/") {
		return fmt.Errorf("scaffold: module %q is not a valid Go module path", o.Module)
	}
	if o.Dir == "" {
		return errors.New("scaffold: Dir is required")
	}
	if o.NCGOVersion == "" {
		return errors.New("scaffold: NCGOVersion is required")
	}
	switch defaultKind(o.Kind) {
	case manifest.KindHertz, manifest.KindKitex:
	default:
		return fmt.Errorf("scaffold: kind %q is invalid (hertz|kitex)", o.Kind)
	}
	return nil
}

func normalizeCreationInfra(kinds []string) ([]string, error) {
	if len(kinds) == 0 {
		return nil, nil
	}
	seen := map[string]struct{}{}
	normalized := make([]string, 0, len(kinds))
	for _, kind := range kinds {
		switch kind {
		case scaffoldinfra.KindRedis:
		default:
			return nil, fmt.Errorf("scaffold: infra %q is not supported by ncgo new yet (want %q)", kind, scaffoldinfra.KindRedis)
		}
		if _, ok := seen[kind]; ok {
			continue
		}
		seen[kind] = struct{}{}
		normalized = append(normalized, kind)
	}
	return normalized, nil
}

func addSelectedInfra(root string, kinds []string) error {
	for _, kind := range kinds {
		if _, err := scaffoldinfra.Add(scaffoldinfra.Options{Root: root, Kind: kind}); err != nil {
			return fmt.Errorf("scaffold: add infra %s: %w", kind, err)
		}
	}
	return nil
}

func ensureEmptyDir(dir string) error {
	info, err := os.Stat(dir)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return os.MkdirAll(dir, 0o755)
	case err != nil:
		return fmt.Errorf("scaffold: stat %s: %w", dir, err)
	case !info.IsDir():
		return fmt.Errorf("scaffold: %s exists and is not a directory", dir)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("scaffold: read %s: %w", dir, err)
	}
	if len(entries) > 0 {
		return fmt.Errorf("scaffold: %s is not empty", dir)
	}
	return nil
}

func hzArgs(opts Options, idl string) []string {
	return []string{
		"new",
		"--mod=" + opts.Module,
		"--idl=" + idl,
		"-I", "idl",
		"--handler_dir=internal/handler",
		"--model_dir=internal/pb",
		"--router_dir=internal/router",
		"--customize_layout=template/layout.yaml",
		"--customize_layout_data_path=template/data.json",
		"--customize_package=template/package.yaml",
	}
}

// kitexArgs are the flags ncgo passes to `kitex` for a kitex monolith
// scaffold. They mirror the invocation documented in
// internal/assets/_data/kitex/kitex-template/makefile.yaml so that
// `make update` later produces the same files.
func kitexArgs(opts Options, idl string) []string {
	return []string{
		"-module", opts.Module,
		"-template-dir", "template/kitex-template",
		"-type", "protobuf",
		idl,
	}
}

// dataPayload is what hz reads from data.json: a wildcard mapping with the
// rendered template variables.
type dataPayload map[string]map[string]any

func renderDataJSON(opts Options) ([]byte, error) {
	p := dataPayload{"*": {
		"GoModule":     opts.Module,
		"ServiceName":  opts.Name,
		"WithDatabase": opts.WithDatabase,
	}}
	return json.MarshalIndent(p, "", "  ")
}
