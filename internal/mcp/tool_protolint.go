package mcp

import (
	"context"
	"encoding/json"
	"io"
	"path/filepath"
	"strings"

	"github.com/byx-darwin/ncgo/internal/protolint"
)

var protolintMCPTool = structuredMCPTool[*protolint.Result]{
	name:      "protolint",
	supported: []string{mcpOutputText, mcpOutputJSON, mcpOutputSARIF},
	format:    formatMCPProtolintOutput,
	fields:    mcpProtolintFields,
	isError: func(res *protolint.Result) bool {
		return !res.OK
	},
}

func callProtolint(ctx context.Context, raw json.RawMessage) (map[string]any, error) {
	var args struct {
		Root        string   `json:"root"`
		Files       []string `json:"files"`
		Rules       []string `json:"rules"`
		IgnoreRules []string `json:"ignoreRules"`
		IgnoreFiles []string `json:"ignoreFiles"`
		Output      string   `json:"output"`
	}
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, err
	}
	if strings.TrimSpace(args.Root) == "" {
		return textResult("protolint: root is required", true), nil
	}
	if _, err := sandboxRoot(args.Root); err != nil {
		return textResult(err.Error(), true), nil
	}
	output, err := protolintMCPTool.resolveOutput(args.Output)
	if err != nil {
		return textResult(err.Error(), true), nil
	}
	root, err := filepath.Abs(args.Root)
	if err != nil {
		return textResult(err.Error(), true), nil
	}
	res, err := protolint.Run(ctx, protolint.RunOptions{Root: root, Files: args.Files, RuleIDs: args.Rules, IgnoreRuleIDs: args.IgnoreRules, IgnoreFiles: args.IgnoreFiles})
	if err != nil {
		return textResult(err.Error(), true), nil
	}
	res.Root = root
	out, err := protolintMCPTool.buildResult(res, output)
	if err != nil {
		return textResult(err.Error(), true), nil
	}
	return out, nil
}

func formatMCPProtolintOutput(res *protolint.Result, output string) (string, error) {
	return formatMCPOutput(output, map[string]outputWriter{
		mcpOutputText:  func(w io.Writer) error { return protolint.WriteText(w, res) },
		mcpOutputJSON:  func(w io.Writer) error { return writeJSONOutput(w, res) },
		mcpOutputSARIF: func(w io.Writer) error { return protolint.WriteSARIF(w, res) },
	})
}

func mcpProtolintFields(res *protolint.Result) map[string]any {
	return map[string]any{
		"root":         res.Root,
		"files":        res.Files,
		"rulesRun":     res.RulesRun,
		"ignoredRules": res.IgnoredRules,
		"ignoredFiles": res.IgnoredFiles,
		"ok":           res.OK,
		"summary":      res.Summary,
		"diagnostics":  res.Diagnostics,
	}
}
