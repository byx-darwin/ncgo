package mono

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/byx-darwin/ncgo/internal/assets"
	"github.com/byx-darwin/ncgo/internal/manifest"
)

// writeTemplate copies the kind-specific custom-template files from the
// embedded snapshot into <dir>/template/. For hertz it also writes a
// freshly rendered data.json so hz picks up the user's values; kitex
// reads its variables inline so no extra file is needed.
func writeTemplate(dir string, opts Options, idl string) error {
	if defaultKind(opts.Kind) == manifest.KindKitex {
		return writeKitexTemplate(dir)
	}
	return writeHertzTemplate(dir, opts)
}

func writeHertzTemplate(dir string, opts Options) error {
	tplDir := filepath.Join(dir, "template")
	if err := os.MkdirAll(tplDir, 0o755); err != nil {
		return fmt.Errorf("scaffold: mkdir %s: %w", tplDir, err)
	}
	srcFS := assets.FS()
	for _, name := range []string{"layout.yaml", "package.yaml"} {
		b, err := fs.ReadFile(srcFS, "hertz/"+name)
		if err != nil {
			return fmt.Errorf("scaffold: read embedded hertz/%s: %w", name, err)
		}
		if err := os.WriteFile(filepath.Join(tplDir, name), b, 0o644); err != nil {
			return fmt.Errorf("scaffold: write %s: %w", name, err)
		}
	}
	data, err := renderDataJSON(opts)
	if err != nil {
		return fmt.Errorf("scaffold: render data.json: %w", err)
	}
	if err := os.WriteFile(filepath.Join(tplDir, "data.json"), data, 0o644); err != nil {
		return fmt.Errorf("scaffold: write data.json: %w", err)
	}
	return nil
}

// writeKitexTemplate copies every embedded kitex-template/*.yaml verbatim
// into <dir>/template/kitex-template/ so that both `kitex` (during
// scaffold) and the generated Makefile's `update` target can consume
// them at the same path.
func writeKitexTemplate(dir string) error {
	tplDir := filepath.Join(dir, "template", "kitex-template")
	if err := os.MkdirAll(tplDir, 0o755); err != nil {
		return fmt.Errorf("scaffold: mkdir %s: %w", tplDir, err)
	}
	srcFS := assets.FS()
	entries, err := fs.ReadDir(srcFS, "kitex/kitex-template")
	if err != nil {
		return fmt.Errorf("scaffold: read embedded kitex/kitex-template: %w", err)
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		b, err := fs.ReadFile(srcFS, "kitex/kitex-template/"+name)
		if err != nil {
			return fmt.Errorf("scaffold: read embedded kitex/kitex-template/%s: %w", name, err)
		}
		if err := os.WriteFile(filepath.Join(tplDir, name), b, 0o644); err != nil {
			return fmt.Errorf("scaffold: write %s: %w", name, err)
		}
	}
	return nil
}

// writeIDLPlaceholder drops a minimal proto3 file at the IDL path so the
// project compiles after the generator runs and gives the user something
// to edit. Hertz uses the `app` package without api.proto annotations so the
// placeholder is self-contained; Kitex uses a service-named package consumed
// by the kitex tool.
func writeIDLPlaceholder(dir, idl string, opts Options) error {
	full := filepath.Join(dir, filepath.FromSlash(idl))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return fmt.Errorf("scaffold: mkdir %s: %w", filepath.Dir(full), err)
	}
	body := renderIDLPlaceholder(opts)
	if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
		return fmt.Errorf("scaffold: write %s: %w", full, err)
	}
	return nil
}

func renderIDLPlaceholder(opts Options) string {
	if defaultKind(opts.Kind) == manifest.KindKitex {
		base := kitexIDLBase(opts)
		service := exportName(opts.Name)
		return fmt.Sprintf(`syntax = "proto3";

package %s;

option go_package = "%s/kitex_gen/%s;%s";

service %s {
}
`, base, opts.Module, base, base, service)
	}
	return fmt.Sprintf(`syntax = "proto3";

package app;

option go_package = "%s/internal/pb;pb";

service %sService {
}
`, opts.Module, exportName(opts.Name))
}

// writeManifest delegates to internal/manifest.Save for the project-root
// .ncgo/manifest.yaml so the schema and atomic-write semantics stay in one
// place.
func writeManifest(dir string, opts Options, idl string) error {
	now := opts.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}
	m := &manifest.Manifest{
		Ncgo: manifest.Meta{
			Version:       opts.NCGOVersion,
			AssetsVersion: opts.AssetsVersion,
		},
		Mode:   manifest.ModeMono,
		Module: opts.Module,
		Service: manifest.Service{
			Name:         opts.Name,
			Kind:         defaultKind(opts.Kind),
			WithDatabase: opts.WithDatabase,
			IDL:          idl,
		},
		GeneratedAt: now,
	}
	return manifest.Save(dir, m)
}

// nextSteps is the agent-facing handoff: the exact shell sequence to run
// when ncgo did not (or could not) call the generator itself. The hz/kitex
// invocation differs by Kind; everything before and after is shared.
func nextSteps(opts Options, idl string) []string {
	rel, _ := filepath.Rel(mustCwd(), opts.Dir)
	if rel == "" {
		rel = filepath.Base(opts.Dir)
	}
	steps := []string{
		fmt.Sprintf("cd %s", rel),
		fmt.Sprintf("go mod init %s", opts.Module),
		generatorCommand(opts, idl),
		"go mod tidy",
	}
	if opts.WithDatabase {
		steps = append(steps, "make migrate-up", "make sqlc-gen")
	}
	steps = append(steps, "make dev")
	return steps
}

// generatorCommand returns the literal shell line a user can paste to
// invoke the appropriate code generator.
func generatorCommand(opts Options, idl string) string {
	if defaultKind(opts.Kind) == manifest.KindKitex {
		return fmt.Sprintf("kitex -module %s -template-dir template/kitex-template -type protobuf %s", opts.Module, idl)
	}
	return fmt.Sprintf("hz new --mod=%s --idl=%s --handler_dir=internal/handler --model_dir=internal/pb --router_dir=internal/router --customize_layout=template/layout.yaml --customize_layout_data_path=template/data.json --customize_package=template/package.yaml", opts.Module, idl)
}

// postGenerateNextSteps is what we print after hz/kitex already ran
// successfully. The generator has already created go.mod, so only tidy
// and runtime follow-ups remain.
func postGenerateNextSteps(opts Options) []string {
	rel, _ := filepath.Rel(mustCwd(), opts.Dir)
	if rel == "" {
		rel = filepath.Base(opts.Dir)
	}
	steps := []string{
		fmt.Sprintf("cd %s", rel),
		"go mod tidy",
	}
	if opts.WithDatabase {
		steps = append(steps, "make migrate-up", "make sqlc-gen")
	}
	steps = append(steps, "make dev")
	return steps
}

func mustCwd() string {
	wd, err := os.Getwd()
	if err != nil {
		return "."
	}
	return wd
}

// exportName converts "user-api" to "UserApi" for use as a proto service name.
func exportName(s string) string {
	out := make([]byte, 0, len(s))
	upper := true
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == '-' || c == '_' {
			upper = true
			continue
		}
		if upper && c >= 'a' && c <= 'z' {
			c -= 'a' - 'A'
		}
		out = append(out, c)
		upper = false
	}
	return string(out)
}
