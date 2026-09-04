package mcp

import (
	"encoding/json"
	"io"

	"github.com/byx-darwin/ncgo/internal/doctor"
)

var runCheckReport = doctor.RunCheck

var checkMCPTool = structuredMCPTool[*doctor.Report]{
	name:      "check",
	supported: []string{mcpOutputText, mcpOutputJSON},
	format:    formatMCPCheckOutput,
	fields:    mcpCheckFields,
	isError: func(rep *doctor.Report) bool {
		return !rep.OK()
	},
}

func callCheck(raw json.RawMessage) (map[string]any, error) {
	var args struct {
		Root   string `json:"root"`
		Output string `json:"output"`
	}
	_ = json.Unmarshal(raw, &args)
	if args.Root == "" {
		args.Root = "."
	}
	output, err := checkMCPTool.resolveOutput(args.Output)
	if err != nil {
		return textResult(err.Error(), true), nil
	}
	rep, err := runCheckReport(args.Root)
	if err != nil {
		return textResult("ncgo_check: "+err.Error(), true), nil
	}
	out, err := checkMCPTool.buildResult(rep, output)
	if err != nil {
		return textResult(err.Error(), true), nil
	}
	return out, nil
}

func formatMCPCheckOutput(rep *doctor.Report, output string) (string, error) {
	return formatMCPOutput(output, map[string]outputWriter{
		mcpOutputText: func(w io.Writer) error { return doctor.WriteText(w, rep) },
		mcpOutputJSON: func(w io.Writer) error { return doctor.WriteJSON(w, rep) },
	})
}

func mcpCheckFields(rep *doctor.Report) map[string]any {
	return map[string]any{
		"root":    rep.Root,
		"scope":   rep.Scope,
		"summary": rep.Summary,
		"checks":  rep.Checks,
		"ok":      rep.OK(),
	}
}
