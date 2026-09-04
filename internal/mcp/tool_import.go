package mcp

import (
	"encoding/json"
	"fmt"

	"gopkg.in/yaml.v3"

	"github.com/byx-darwin/ncgo/internal/importer"
	"github.com/byx-darwin/ncgo/internal/manifest"
)

// importDetectOptions mirrors importer.Options so tests can stub the
// detection call without depending on internal/importer's exported type
// name directly (keeps the indirection var's signature stable if
// importer.Options ever gains fields tool_import.go doesn't use).
type importDetectOptions = importer.Options

var runImportDetect = importer.Detect

// callImport previews the manifest `ncgo import` would generate for an
// existing hz/kitex project. It is always preview-only through MCP: it
// never calls manifest.Save, matching ncgo_upgrade's "always plan mode"
// behavior so an agent cannot accidentally write project files.
func callImport(raw json.RawMessage, ncgoVersion, assetsVersion string) (map[string]any, error) {
	var args struct {
		Root string `json:"root"`
		Kind string `json:"kind"`
	}
	_ = json.Unmarshal(raw, &args)
	if args.Root == "" {
		args.Root = "."
	}
	if _, err := sandboxRoot(args.Root); err != nil {
		return textResult(err.Error(), true), nil
	}

	m, err := runImportDetect(importDetectOptions{
		Root:          args.Root,
		Kind:          args.Kind,
		NCGOVersion:   ncgoVersion,
		AssetsVersion: assetsVersion,
	})
	if err != nil {
		return textResult("ncgo_import: "+err.Error(), true), nil
	}

	b, err := yaml.Marshal(m)
	if err != nil {
		return textResult("ncgo_import: marshal preview: "+err.Error(), true), nil
	}
	text := fmt.Sprintf("Preview of generated manifest (MCP is always preview-only; run `ncgo import` locally to write it):\n\n%s", string(b))

	return buildMCPResult(text, false, buildImportPreviewFields(m)), nil
}

func buildImportPreviewFields(m *manifest.Manifest) map[string]any {
	return map[string]any{
		"preview": true,
		"module":  m.Module,
		"mode":    m.Mode,
		"service": map[string]any{
			"name":         m.Service.Name,
			"kind":         m.Service.Kind,
			"withDatabase": m.Service.WithDatabase,
			"idl":          m.Service.IDL,
		},
	}
}
