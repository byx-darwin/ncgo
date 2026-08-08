package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/byx-darwin/ncgo/internal/registry"
	"github.com/byx-darwin/ncgo/internal/scaffold/rpc"
)

func callAddRPC(ctx context.Context, raw json.RawMessage, ncgoVersion, assetsVersion string) (map[string]any, error) {
	var args struct {
		Name        string `json:"name"`
		Root        string `json:"root"`
		Module      string `json:"module"`
		Dir         string `json:"dir"`
		NoGenerate  bool   `json:"noGenerate"`
		DryRun      bool   `json:"dryRun"`
		Preset      string `json:"preset"`
		Template    string `json:"template"`
		TemplateDir string `json:"templateDir"`
		Output      string `json:"output"`
	}
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, err
	}
	if args.Name == "" {
		return textResult("name is required", true), nil
	}
	if args.Root == "" {
		args.Root = "."
	}
	if _, err := sandboxRoot(args.Root); err != nil {
		return textResult(err.Error(), true), nil
	}

	output, err := resolveMCPOutput("add_rpc", args.Output, mcpOutputText, mcpOutputJSON)
	if err != nil {
		return textResult(err.Error(), true), nil
	}

	templateDir, err := registry.ResolveTemplateDir(args.Template, args.TemplateDir)
	if err != nil {
		return textResult(err.Error(), true), nil
	}
	if args.Preset != "" && templateDir != "" {
		return textResult("--preset and --template/--templateDir are mutually exclusive", true), nil
	}

	res, err := rpc.Add(ctx, rpc.Options{
		Root:          args.Root,
		Name:          args.Name,
		Module:        args.Module,
		Dir:           args.Dir,
		AssetsVersion: assetsVersion,
		NCGOVersion:   ncgoVersion,
		NoGenerate:    args.NoGenerate,
		DryRun:        args.DryRun,
		Preset:        args.Preset,
		TemplateDir:   templateDir,
	})
	if err != nil {
		return textResult(err.Error(), true), nil
	}

	out, err := addRPCMCPTool.buildResult(res, output)
	if err != nil {
		return textResult(err.Error(), true), nil
	}
	return out, nil
}

var addRPCMCPTool = structuredMCPTool[*rpc.Result]{
	name:      "add_rpc",
	supported: []string{mcpOutputText, mcpOutputJSON},
	format:    formatMCPAddRPCOutput,
	fields: func(res *rpc.Result) map[string]any {
		m := map[string]any{
			"serviceDir":  res.ServiceDir,
			"serviceRel":  res.ServiceRel,
			"module":      res.Module,
			"updated":     res.Updated,
			"ranGenerate": res.RanGenerate,
			"dryRun":      res.DryRun,
			"nextSteps":   res.NextSteps,
		}
		if res.DryRun && len(res.Plan) > 0 {
			planItems := make([]map[string]any, len(res.Plan))
			for i, p := range res.Plan {
				planItems[i] = map[string]any{
					"kind":   p.Kind,
					"action": p.Action,
					"path":   p.Path,
					"detail": p.Detail,
				}
			}
			m["plan"] = planItems
		}
		return m
	},
	isError: func(*rpc.Result) bool { return false },
}

func formatMCPAddRPCOutput(res *rpc.Result, output string) (string, error) {
	return formatMCPOutput(output, map[string]outputWriter{
		mcpOutputText: func(w io.Writer) error {
			writeVerb := "wrote"
			if res.DryRun {
				writeVerb = "would write"
			}
			fmt.Fprintf(w, "%s RPC service %s in %s\n", writeVerb, res.ServiceRel, res.ServiceDir)
			fmt.Fprintf(w, "module: %s\n", res.Module)
			if res.DryRun {
				fmt.Fprintln(w, "(dry-run: no files were written)")
			}
			fmt.Fprintln(w, "\nnext steps:")
			for _, s := range res.NextSteps {
				fmt.Fprintf(w, "  - %s\n", s)
			}
			return nil
		},
		mcpOutputJSON: func(w io.Writer) error {
			type planItem struct {
				Kind   string `json:"kind"`
				Action string `json:"action"`
				Path   string `json:"path,omitempty"`
				Detail string `json:"detail,omitempty"`
			}
			var plan []planItem
			if res.DryRun && len(res.Plan) > 0 {
				plan = make([]planItem, len(res.Plan))
				for i, p := range res.Plan {
					plan[i] = planItem{Kind: p.Kind, Action: p.Action, Path: p.Path, Detail: p.Detail}
				}
			}
			return writeJSONOutput(w, map[string]any{
				"serviceDir":  res.ServiceDir,
				"serviceRel":  res.ServiceRel,
				"module":      res.Module,
				"updated":     res.Updated,
				"ranGenerate": res.RanGenerate,
				"dryRun":      res.DryRun,
				"nextSteps":   res.NextSteps,
				"plan":        plan,
			})
		},
	})
}
