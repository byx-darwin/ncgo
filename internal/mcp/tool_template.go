package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/byx-darwin/ncgo/internal/registry"
)

// callTemplateList lists template packages available in the template registry.
// Errors (unreachable registry, missing git) are surfaced as isError=true tool
// results rather than JSON-RPC protocol errors, matching the other ncgo tools.
func callTemplateList(raw json.RawMessage) (map[string]any, error) {
	var args struct {
		Registry string `json:"registry"`
		Output   string `json:"output"`
	}
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, err
	}

	client := registry.NewClient(registry.ResolveURL(args.Registry), nil)
	entries, err := client.List(context.Background())
	if err != nil {
		return textResult(err.Error(), true), nil
	}

	output, err := resolveMCPOutput("template_list", args.Output, mcpOutputText, mcpOutputJSON)
	if err != nil {
		return textResult(err.Error(), true), nil
	}

	templates := make([]map[string]any, 0, len(entries))
	for _, e := range entries {
		templates = append(templates, map[string]any{
			"name":        e.Name,
			"kind":        e.Kind,
			"description": e.Description,
		})
	}
	fields := map[string]any{"templates": templates}

	text, err := formatMCPOutput(output, map[string]outputWriter{
		mcpOutputText: func(w io.Writer) error {
			if len(entries) == 0 {
				fmt.Fprintln(w, "no templates in registry")
				return nil
			}
			for _, e := range entries {
				fmt.Fprintf(w, "%s\t%s\t%s\n", e.Name, e.Kind, e.Description)
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

// callTemplatePull fetches the named template package into the local registry
// cache and reports where it landed. Missing templates and registry failures
// are returned as isError=true tool results.
func callTemplatePull(raw json.RawMessage) (map[string]any, error) {
	var args struct {
		Name     string `json:"name"`
		Registry string `json:"registry"`
		Output   string `json:"output"`
	}
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, err
	}

	client := registry.NewClient(registry.ResolveURL(args.Registry), nil)
	dir, err := client.Pull(context.Background(), args.Name)
	if err != nil {
		return textResult(err.Error(), true), nil
	}

	output, err := resolveMCPOutput("template_pull", args.Output, mcpOutputText, mcpOutputJSON)
	if err != nil {
		return textResult(err.Error(), true), nil
	}

	fields := map[string]any{"name": args.Name, "dir": dir}

	text, err := formatMCPOutput(output, map[string]outputWriter{
		mcpOutputText: func(w io.Writer) error {
			fmt.Fprintf(w, "pulled %s -> %s\n", args.Name, dir)
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
