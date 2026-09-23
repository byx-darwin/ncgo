package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/byx-darwin/ncgo/internal/registry"
)

// callTemplateList lists template packages available in the template registry.
// Errors (unreachable registry, missing git) are surfaced as isError=true tool
// results rather than JSON-RPC protocol errors, matching the other ncgo tools.
func callTemplateList(ctx context.Context, raw json.RawMessage) (map[string]any, error) {
	var args struct {
		Registry string `json:"registry"`
		Output   string `json:"output"`
	}
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, err
	}
	output, err := resolveMCPOutput("template_list", args.Output, mcpOutputText, mcpOutputJSON)
	if err != nil {
		return invalidArgumentResult(err.Error()), nil
	}

	client := registry.NewClient(registry.ResolveURL(args.Registry), nil)
	entries, err := client.List(ctx)
	if err != nil {
		return registryErrorResult("template list", err), nil
	}

	templates := make([]map[string]any, 0, len(entries))
	for _, e := range entries {
		templates = append(templates, map[string]any{
			"name":        e.Name,
			"kind":        e.Kind,
			"description": e.Description,
		})
	}
	fields := map[string]any{
		"templates": templates,
		"effects": []mcpEffect{
			{Kind: "network", Action: "fetch_registry", Status: "applied"},
			{Kind: "cache", Action: "refresh", Status: "applied"},
		},
	}

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
		return operationErrorResult("template list output", err), nil
	}

	return buildMCPResult(text, false, fields), nil
}

// callTemplatePull fetches the named template package into the local registry
// cache and reports where it landed. Missing templates and registry failures
// are returned as isError=true tool results.
func callTemplatePull(ctx context.Context, raw json.RawMessage) (map[string]any, error) {
	var args struct {
		Name     string `json:"name"`
		Registry string `json:"registry"`
		Output   string `json:"output"`
	}
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, err
	}
	if args.Name == "" {
		return invalidArgumentResult("name is required"), nil
	}
	output, err := resolveMCPOutput("template_pull", args.Output, mcpOutputText, mcpOutputJSON)
	if err != nil {
		return invalidArgumentResult(err.Error()), nil
	}

	client := registry.NewClient(registry.ResolveURL(args.Registry), nil)
	dir, err := client.Pull(ctx, args.Name)
	if err != nil {
		result := registryErrorResult("template pull", err)
		var notFound *registry.TemplateNotFoundError
		if errors.As(err, &notFound) {
			setResultEffects(result, []mcpEffect{
				{Kind: "network", Action: "fetch_template", Status: "applied", Detail: args.Name},
				{Kind: "cache", Action: "refresh", Status: "applied"},
			})
		}
		return result, nil
	}

	fields := map[string]any{
		"name": args.Name,
		"dir":  dir,
		"effects": []mcpEffect{
			{Kind: "network", Action: "fetch_template", Status: "applied", Detail: args.Name},
			{Kind: "cache", Action: "update", Status: "applied", Path: dir},
		},
	}

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
		return operationErrorResult("template pull output", err), nil
	}

	return buildMCPResult(text, false, fields), nil
}

func registryErrorResult(operation string, err error) map[string]any {
	var dependency *registry.DependencyError
	if errors.As(err, &dependency) {
		return dependencyErrorResult(err.Error())
	}
	var notFound *registry.TemplateNotFoundError
	if errors.As(err, &notFound) {
		return resourceNotFoundResult(err.Error())
	}
	var unavailable *registry.UnavailableError
	if errors.As(err, &unavailable) {
		return networkErrorResult(operation, err)
	}
	return operationErrorResult(operation, err)
}
