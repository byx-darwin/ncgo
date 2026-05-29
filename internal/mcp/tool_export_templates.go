package mcp

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/byx-darwin/ncgo/internal/manifest"
	"github.com/byx-darwin/ncgo/internal/scaffold/template"
)

func callExportTemplates(raw json.RawMessage) (map[string]any, error) {
	var args struct {
		Root   string `json:"root"`
		Kind   string `json:"kind"`
		Output string `json:"output"`
	}
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, err
	}
	if args.Root == "" {
		args.Root = "."
	}
	if _, err := sandboxRoot(args.Root); err != nil {
		return textResult(err.Error(), true), nil
	}

	m, err := manifest.Load(args.Root)
	if err != nil {
		return textResult(fmt.Sprintf("load manifest: %v", err), true), nil
	}

	kind := args.Kind
	if kind == "" {
		kind = m.Service.Kind
	}
	switch kind {
	case manifest.KindHertz, manifest.KindKitex:
	default:
		return textResult(fmt.Sprintf("kind %q is invalid (hertz|kitex)", kind), true), nil
	}

	result, err := template.Export(template.ExportOptions{
		Root:        args.Root,
		Kind:        kind,
		Module:      m.Module,
		ServiceName: m.Service.Name,
	})
	if err != nil {
		return textResult(err.Error(), true), nil
	}

	output, err := resolveMCPOutput("export_templates", args.Output, mcpOutputText, mcpOutputJSON)
	if err != nil {
		return textResult(err.Error(), true), nil
	}

	templates := result.Templates
	if templates == nil {
		templates = []string{}
	}
	fields := map[string]any{
		"outputDir": result.OutputDir,
		"kind":      kind,
		"templates": templates,
	}

	text, err := formatMCPOutput(output, map[string]outputWriter{
		mcpOutputText: func(w io.Writer) error {
			fmt.Fprintf(w, "exported %d templates to %s/\n", len(result.Templates), result.OutputDir)
			for _, t := range result.Templates {
				fmt.Fprintf(w, "  - %s\n", t)
			}
			return nil
		},
		mcpOutputJSON: func(w io.Writer) error {
			return writeJSONOutput(w, fields)
		},
	})
	if err != nil {
		return textResult(err.Error(), true), nil
	}

	return buildMCPResult(text, false, fields), nil
}
