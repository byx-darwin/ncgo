package mcp

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/byx-darwin/ncgo/internal/scaffold/rulecenter"
)

func callAddRuleCenter(raw json.RawMessage) (map[string]any, error) {
	var args struct {
		Root   string `json:"root"`
		Addr   string `json:"addr"`
		Force  bool   `json:"force"`
		DryRun bool   `json:"dryRun"`
		Output string `json:"output"`
	}
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, err
	}
	if args.Addr == "" {
		return invalidArgumentResult("addr is required"), nil
	}
	safeRoot, err := sandboxRoot(args.Root)
	if err != nil {
		return sandboxErrorResult(err), nil
	}
	args.Root = safeRoot

	output, err := addRuleCenterMCPTool.resolveOutput(args.Output)
	if err != nil {
		return invalidArgumentResult(err.Error()), nil
	}

	res, err := rulecenter.Add(rulecenter.Options{
		Root:   args.Root,
		Addr:   args.Addr,
		Force:  args.Force,
		DryRun: args.DryRun,
	})
	if err != nil {
		result := operationErrorResult("add rule center", err)
		if res != nil {
			setResultEffects(result, fileEffects(res.WrittenPaths, false, "write"))
		}
		return result, nil
	}

	out, err := addRuleCenterMCPTool.buildResult(res, output)
	if err != nil {
		return operationErrorResult("add rule center output", err), nil
	}
	paths := res.WrittenPaths
	if res.DryRun {
		paths = res.PlannedPaths
	}
	setResultEffects(out, fileEffects(paths, res.DryRun, "update"))
	return out, nil
}

var addRuleCenterMCPTool = structuredMCPTool[*rulecenter.Result]{
	name:      "add_rule_center",
	supported: []string{mcpOutputText, mcpOutputJSON},
	format:    formatMCPAddRuleCenterOutput,
	fields: func(res *rulecenter.Result) map[string]any {
		return map[string]any{
			"dryRun":       res.DryRun,
			"writtenPaths": res.WrittenPaths,
			"plannedPaths": res.PlannedPaths,
			"nextSteps":    res.NextSteps,
		}
	},
	isError: func(*rulecenter.Result) bool { return false },
}

func formatMCPAddRuleCenterOutput(res *rulecenter.Result, output string) (string, error) {
	return formatMCPOutput(output, map[string]outputWriter{
		mcpOutputText: func(w io.Writer) error {
			writeVerb := "wrote"
			if res.DryRun {
				writeVerb = "would write"
			}
			paths := res.WrittenPaths
			if res.DryRun {
				paths = res.PlannedPaths
			}
			for _, p := range paths {
				if _, err := fmt.Fprintf(w, "%s %s\n", writeVerb, p); err != nil {
					return err
				}
			}
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
			return json.NewEncoder(w).Encode(struct {
				DryRun       bool     `json:"dryRun"`
				WrittenPaths []string `json:"writtenPaths"`
				PlannedPaths []string `json:"plannedPaths,omitempty"`
				NextSteps    []string `json:"nextSteps"`
			}{
				DryRun:       res.DryRun,
				WrittenPaths: res.WrittenPaths,
				PlannedPaths: res.PlannedPaths,
				NextSteps:    res.NextSteps,
			})
		},
	})
}
