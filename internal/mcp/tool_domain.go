package mcp

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/byx-darwin/ncgo/internal/scaffold/domain"
	planpkg "github.com/byx-darwin/ncgo/internal/scaffold/plan"
)

var addDomainMCPTool = structuredMCPTool[*domain.Result]{
	name:      "add domain",
	supported: []string{mcpOutputText, mcpOutputJSON},
	format:    formatMCPAddDomainOutput,
	fields: func(res *domain.Result) map[string]any {
		return map[string]any{
			"dryRun":       res.DryRun,
			"updated":      res.Updated,
			"writtenPaths": res.WrittenPaths,
			"nextSteps":    res.NextSteps,
			"plan":         res.Plan,
		}
	},
	isError: func(*domain.Result) bool { return false },
}

func callAddDomain(raw json.RawMessage) (map[string]any, error) {
	var args struct {
		Name   string `json:"name"`
		Root   string `json:"root"`
		Force  bool   `json:"force"`
		DryRun bool   `json:"dryRun"`
		Output string `json:"output"`
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

	output, err := addDomainMCPTool.resolveOutput(args.Output)
	if err != nil {
		return textResult(err.Error(), true), nil
	}
	res, err := domain.Add(domain.Options{
		Root:   args.Root,
		Name:   args.Name,
		Force:  args.Force,
		DryRun: args.DryRun,
	})
	if err != nil {
		return textResult(err.Error(), true), nil
	}
	out, err := addDomainMCPTool.buildResult(res, output)
	if err != nil {
		return textResult(err.Error(), true), nil
	}
	return out, nil
}

func formatMCPAddDomainOutput(res *domain.Result, output string) (string, error) {
	return formatMCPOutput(output, map[string]outputWriter{
		mcpOutputText: func(w io.Writer) error {
			writeVerb := "wrote"
			if res.DryRun {
				writeVerb = "would write"
			}
			for _, p := range res.WrittenPaths {
				if _, err := fmt.Fprintf(w, "%s %s\n", writeVerb, p); err != nil {
					return err
				}
			}
			if res.DryRun && res.Updated {
				fmt.Fprintln(w, "(dry-run: manifest will be updated)")
			} else if !res.Updated {
				fmt.Fprintln(w, "(manifest already lists this domain)")
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
				DryRun       bool           `json:"dryRun"`
				Updated      bool           `json:"updated"`
				WrittenPaths []string       `json:"writtenPaths"`
				NextSteps    []string       `json:"nextSteps"`
				Plan         []planpkg.Item `json:"plan"`
			}{
				DryRun:       res.DryRun,
				Updated:      res.Updated,
				WrittenPaths: res.WrittenPaths,
				NextSteps:    res.NextSteps,
				Plan:         res.Plan,
			})
		},
	})
}
