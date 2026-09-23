package mcp

import (
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"

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
	safeRoot, err := sandboxRoot(args.Root)
	if err != nil {
		return sandboxErrorResult(err), nil
	}
	args.Root = safeRoot

	m, err := manifest.Load(args.Root)
	if err != nil {
		return validationErrorResult("export templates", fmt.Errorf("load manifest: %v", err)), nil
	}

	kind := args.Kind
	if kind == "" {
		kind = m.Service.Kind
	}
	switch kind {
	case manifest.KindHertz, manifest.KindKitex:
	default:
		return invalidArgumentResult(fmt.Sprintf("kind %q is invalid (hertz|kitex)", kind)), nil
	}
	output, err := resolveMCPOutput("export_templates", args.Output, mcpOutputText, mcpOutputJSON)
	if err != nil {
		return invalidArgumentResult(err.Error()), nil
	}

	result, err := template.Export(template.ExportOptions{
		Root:        args.Root,
		Kind:        kind,
		Module:      m.Module,
		ServiceName: m.Service.Name,
	})
	if err != nil {
		return operationErrorResult("export templates", err), nil
	}

	templates := result.Templates
	if templates == nil {
		templates = []string{}
	}
	idls := result.IDLs
	if idls == nil {
		idls = []string{}
	}
	fields := map[string]any{
		"outputDir": result.OutputDir,
		"kind":      kind,
		"templates": templates,
		"idls":      idls,
		"effects": []mcpEffect{{
			Kind: "directory", Action: "write_derived_contents", Status: "applied", Path: filepath.Join(args.Root, "template"),
			Detail: "exported templates and IDL snapshots",
		}},
	}

	text, err := formatMCPOutput(output, map[string]outputWriter{
		mcpOutputText: func(w io.Writer) error {
			if len(result.IDLs) > 0 {
				fmt.Fprintf(w, "exported %d templates and %d IDL files to %s/\n", len(result.Templates), len(result.IDLs), result.OutputDir)
			} else {
				fmt.Fprintf(w, "exported %d templates to %s/\n", len(result.Templates), result.OutputDir)
			}
			for _, t := range result.Templates {
				fmt.Fprintf(w, "  - %s\n", t)
			}
			for _, f := range result.IDLs {
				fmt.Fprintf(w, "  - %s\n", f)
			}
			return nil
		},
		mcpOutputJSON: func(w io.Writer) error {
			return writeJSONOutput(w, fields)
		},
	})
	if err != nil {
		return operationErrorResult("export templates output", err), nil
	}

	return buildMCPResult(text, false, fields), nil
}
