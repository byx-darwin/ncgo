// Package assets exposes the scaffold templates ncgo embeds at build time.
//
// The templates (Hertz layout/package YAMLs, Kitex template tree, optional
// infra Go files) are owned and maintained in this repository. Bumping a
// template means editing the file under _data/ and bumping VERSION in the
// same change; nc-skills-golang documents the conventions but no longer
// hosts the template source.
package assets

import (
	"embed"
	"io/fs"
	"strings"
)

// The snapshot lives under _data/ so that `go build ./...` ignores the
// upstream Go template files that ship with optional/ add-ons (Go's tooling
// skips directories whose names start with `_`). The `all:` embed prefix
// keeps them visible to //go:embed.
//
//go:embed all:_data
var raw embed.FS

// FS returns the embedded asset filesystem with the internal `_data/` prefix
// stripped, so callers see paths like "hertz/layout.yaml" and
// "kitex/kitex-template/server.yaml".
func FS() fs.FS {
	sub, err := fs.Sub(raw, "_data")
	if err != nil {
		panic(err) // unreachable: _data is embedded above
	}
	return sub
}

// Version returns the embedded asset version string parsed from the
// _data/VERSION file's `ncgo_assets_version` field. Returns "unknown" when
// the file is missing or the field is absent.
func Version() string {
	b, err := raw.ReadFile("_data/VERSION")
	if err != nil {
		return "unknown"
	}
	for _, line := range strings.Split(string(b), "\n") {
		k, v, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		if strings.TrimSpace(k) == "ncgo_assets_version" {
			if v = strings.TrimSpace(v); v != "" {
				return v
			}
		}
	}
	return "unknown"
}
