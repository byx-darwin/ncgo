package mcp

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/byx-darwin/ncgo/internal/extract"
)

func callExtractDomain(raw json.RawMessage) (map[string]any, error) {
	var args struct {
		Name   string `json:"name"`
		Root   string `json:"root"`
		To     string `json:"to"`
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

	// MCP always uses PlanDomain — never ApplyDomain. Extraction that modifies
	// files must go through the CLI with explicit --apply.
	plan, err := extract.PlanDomain(extract.DomainOptions{
		Root: args.Root,
		Name: args.Name,
		To:   args.To,
	})
	if err != nil {
		return textResult(err.Error(), true), nil
	}

	output, err := resolveMCPOutput("extract_domain", args.Output, mcpOutputText, mcpOutputJSON)
	if err != nil {
		return textResult(err.Error(), true), nil
	}

	fields := extractDomainFields(plan)

	text, err := formatMCPOutput(output, map[string]outputWriter{
		mcpOutputText: func(w io.Writer) error {
			fmt.Fprintf(w, "planned extraction for domain %s -> %s\n", plan.Name, plan.To)
			fmt.Fprintf(w, "target module: %s\n\n", plan.TargetModule)
			fmt.Fprintln(w, "files:")
			for _, f := range plan.Sources {
				fmt.Fprintf(w, "  - [%s] %s -> %s\n", f.Role, f.From, f.To)
			}
			fmt.Fprintln(w, "\nnext steps:")
			for _, step := range plan.NextSteps {
				fmt.Fprintf(w, "  - %s\n", step)
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

func extractDomainFields(plan *extract.DomainPlan) map[string]any {
	type sourceItem struct {
		Role string `json:"role"`
		From string `json:"from"`
		To   string `json:"to"`
	}
	sources := make([]sourceItem, len(plan.Sources))
	for i, s := range plan.Sources {
		sources[i] = sourceItem{Role: s.Role, From: s.From, To: s.To}
	}
	return map[string]any{
		"name":         plan.Name,
		"targetModule": plan.TargetModule,
		"toDir":        plan.To,
		"sources":      sources,
		"nextSteps":    plan.NextSteps,
	}
}
