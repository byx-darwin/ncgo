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
	fields: func(res *infra.Result) map[string]any {
		return withEffects(infra.AddResultFields(res), effectsFromPlan(res.Plan, res.DryRun))
	},
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
	safeRoot, err := sandboxRoot(args.Root)
	if err != nil {
		return sandboxErrorResult(err), nil
	}
	args.Root = safeRoot
	output, err := addInfraMCPTool.resolveOutput(args.Output)
	if err != nil {
		return invalidArgumentResult(err.Error()), nil
	}
	res, err := infra.Add(infra.Options{Root: args.Root, Kind: args.Kind, Force: args.Force, Wire: args.Wire, DryRun: args.DryRun})
	if err != nil {
		return operationErrorResult("add infra", err), nil
	}
	out, err := addInfraMCPTool.buildResult(res, output)
	if err != nil {
		return operationErrorResult("add infra output", err), nil
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
			"dryRun":    res.DryRun,
			"effects":   []mcpEffect{fileWriteEffect(res.Path, res.DryRun)},
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
		DryRun bool   `json:"dryRun"`
		Output string `json:"output"`
	}
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, err
	}
	safeRoot, err := sandboxRoot(args.Root)
	if err != nil {
		return sandboxErrorResult(err), nil
	}
	args.Root = safeRoot
	output, err := addMethodMCPTool.resolveOutput(args.Output)
	if err != nil {
		return invalidArgumentResult(err.Error()), nil
	}
	res, err := method.Add(method.Options{Root: args.Root, Spec: args.Spec, Layer: args.Layer, DryRun: args.DryRun})
	if err != nil {
		return operationErrorResult("add method", err), nil
	}
	out, err := addMethodMCPTool.buildResult(res, output)
	if err != nil {
		return operationErrorResult("add method output", err), nil
	}
	return out, nil
}

func formatMCPAddMethodOutput(res *method.Result, output string) (string, error) {
	return formatMCPOutput(output, map[string]outputWriter{
		mcpOutputText: func(w io.Writer) error {
			verb := "inserted"
			if res.DryRun {
				verb = "would insert"
			}
			_, err := io.WriteString(w, fmt.Sprintf("%s %s.%s into %s", verb, res.Domain, res.Method, res.Path))
			return err
		},
		mcpOutputJSON: func(w io.Writer) error {
			return json.NewEncoder(w).Encode(struct {
				Path      string   `json:"path"`
				Domain    string   `json:"domain"`
				Method    string   `json:"method"`
				DryRun    bool     `json:"dryRun"`
				NextSteps []string `json:"nextSteps"`
			}{
				Path:      res.Path,
				Domain:    res.Domain,
				Method:    res.Method,
				DryRun:    res.DryRun,
				NextSteps: res.NextSteps,
			})
		},
	})
}

var addRPCMethodMCPTool = structuredMCPTool[*method.RPCResult]{
	name:      "add rpc-method",
	supported: []string{mcpOutputText, mcpOutputJSON},
	format:    formatMCPAddRPCMethodOutput,
	fields: func(res *method.RPCResult) map[string]any {
		return map[string]any{
			"path":      res.Path,
			"service":   res.Service,
			"method":    res.Method,
			"dryRun":    res.DryRun,
			"effects":   []mcpEffect{fileWriteEffect(res.Path, res.DryRun)},
			"nextSteps": res.NextSteps,
		}
	},
	isError: func(*method.RPCResult) bool {
		return false
	},
}

func callAddRPCMethod(raw json.RawMessage) (map[string]any, error) {
	var args struct {
		Root    string `json:"root"`
		Service string `json:"service"`
		RPC     string `json:"rpc"`
		DryRun  bool   `json:"dryRun"`
		Output  string `json:"output"`
	}
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, err
	}
	safeRoot, err := sandboxRoot(args.Root)
	if err != nil {
		return sandboxErrorResult(err), nil
	}
	args.Root = safeRoot
	output, err := addRPCMethodMCPTool.resolveOutput(args.Output)
	if err != nil {
		return invalidArgumentResult(err.Error()), nil
	}
	res, err := method.AddRPC(method.RPCOptions{Root: args.Root, Service: args.Service, RPC: args.RPC, DryRun: args.DryRun})
	if err != nil {
		return operationErrorResult("add rpc-method", err), nil
	}
	out, err := addRPCMethodMCPTool.buildResult(res, output)
	if err != nil {
		return operationErrorResult("add rpc-method output", err), nil
	}
	return out, nil
}

func formatMCPAddRPCMethodOutput(res *method.RPCResult, output string) (string, error) {
	return formatMCPOutput(output, map[string]outputWriter{
		mcpOutputText: func(w io.Writer) error {
			verb := "inserted"
			if res.DryRun {
				verb = "would insert"
			}
			_, err := io.WriteString(w, fmt.Sprintf("%s %s.%s into %s", verb, res.Service, res.Method, res.Path))
			return err
		},
		mcpOutputJSON: func(w io.Writer) error {
			return json.NewEncoder(w).Encode(struct {
				Path      string   `json:"path"`
				Service   string   `json:"service"`
				Method    string   `json:"method"`
				DryRun    bool     `json:"dryRun"`
				NextSteps []string `json:"nextSteps"`
			}{Path: res.Path, Service: res.Service, Method: res.Method, DryRun: res.DryRun, NextSteps: res.NextSteps})
		},
	})
}

func fileWriteEffect(path string, dryRun bool) mcpEffect {
	status := "applied"
	if dryRun {
		status = "planned"
	}
	return mcpEffect{Kind: "file", Action: "update", Path: path, Status: status}
}
