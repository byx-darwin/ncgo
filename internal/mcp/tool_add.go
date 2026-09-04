package mcp

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/byx-darwin/ncgo/internal/scaffold/infra"
	"github.com/byx-darwin/ncgo/internal/scaffold/method"
)

var addInfraMCPTool = structuredMCPTool[*infra.Result]{
	name:      "add infra",
	supported: []string{mcpOutputText, mcpOutputJSON},
	format:    formatMCPAddInfraOutput,
	fields:    infra.AddResultFields,
	isError: func(*infra.Result) bool {
		return false
	},
}

func callAddInfra(raw json.RawMessage) (map[string]any, error) {
	var args struct {
		Root   string `json:"root"`
		Kind   string `json:"kind"`
		Force  bool   `json:"force"`
		Wire   bool   `json:"wire"`
		DryRun bool   `json:"dryRun"`
		Output string `json:"output"`
	}
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, err
	}
	if _, err := sandboxRoot(args.Root); err != nil {
		return textResult(err.Error(), true), nil
	}
	output, err := addInfraMCPTool.resolveOutput(args.Output)
	if err != nil {
		return textResult(err.Error(), true), nil
	}
	res, err := infra.Add(infra.Options{Root: args.Root, Kind: args.Kind, Force: args.Force, Wire: args.Wire, DryRun: args.DryRun})
	if err != nil {
		return textResult(err.Error(), true), nil
	}
	out, err := addInfraMCPTool.buildResult(res, output)
	if err != nil {
		return textResult(err.Error(), true), nil
	}
	return out, nil
}

func formatMCPAddInfraOutput(res *infra.Result, output string) (string, error) {
	return formatMCPOutput(output, map[string]outputWriter{
		mcpOutputText: func(w io.Writer) error {
			_, err := io.WriteString(w, infra.FormatAddResultText(res))
			return err
		},
		mcpOutputJSON: func(w io.Writer) error { return infra.WriteAddResultJSON(w, res) },
	})
}

var addMethodMCPTool = structuredMCPTool[*method.Result]{
	name:      "add method",
	supported: []string{mcpOutputText, mcpOutputJSON},
	format:    formatMCPAddMethodOutput,
	fields: func(res *method.Result) map[string]any {
		return map[string]any{
			"path":      res.Path,
			"domain":    res.Domain,
			"method":    res.Method,
			"nextSteps": res.NextSteps,
		}
	},
	isError: func(*method.Result) bool {
		return false
	},
}

func callAddMethod(raw json.RawMessage) (map[string]any, error) {
	var args struct {
		Root   string `json:"root"`
		Spec   string `json:"spec"`
		Layer  string `json:"in"`
		Output string `json:"output"`
	}
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, err
	}
	if _, err := sandboxRoot(args.Root); err != nil {
		return textResult(err.Error(), true), nil
	}
	output, err := addMethodMCPTool.resolveOutput(args.Output)
	if err != nil {
		return textResult(err.Error(), true), nil
	}
	res, err := method.Add(method.Options{Root: args.Root, Spec: args.Spec, Layer: args.Layer})
	if err != nil {
		return textResult(err.Error(), true), nil
	}
	out, err := addMethodMCPTool.buildResult(res, output)
	if err != nil {
		return textResult(err.Error(), true), nil
	}
	return out, nil
}

func formatMCPAddMethodOutput(res *method.Result, output string) (string, error) {
	return formatMCPOutput(output, map[string]outputWriter{
		mcpOutputText: func(w io.Writer) error {
			_, err := io.WriteString(w, fmt.Sprintf("inserted %s.%s into %s", res.Domain, res.Method, res.Path))
			return err
		},
		mcpOutputJSON: func(w io.Writer) error {
			return json.NewEncoder(w).Encode(struct {
				Path      string   `json:"path"`
				Domain    string   `json:"domain"`
				Method    string   `json:"method"`
				NextSteps []string `json:"nextSteps"`
			}{
				Path:      res.Path,
				Domain:    res.Domain,
				Method:    res.Method,
				NextSteps: res.NextSteps,
			})
		},
	})
}
