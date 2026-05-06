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

func callAddMethod(raw json.RawMessage) (map[string]any, error) {
	var args struct {
		Root  string `json:"root"`
		Spec  string `json:"spec"`
		Layer string `json:"in"`
	}
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, err
	}
	res, err := method.Add(method.Options{Root: args.Root, Spec: args.Spec, Layer: args.Layer})
	if err != nil {
		return textResult(err.Error(), true), nil
	}
	return textResult(fmt.Sprintf("inserted %s.%s into %s", res.Domain, res.Method, res.Path), false), nil
}
