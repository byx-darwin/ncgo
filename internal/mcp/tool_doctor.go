package mcp

import (
	"context"
	"encoding/json"
	"io"

	"github.com/byx-darwin/ncgo/internal/doctor"
)

var runDoctorReport = doctor.Run

var doctorMCPTool = structuredMCPTool[*doctor.Report]{
	name:      "doctor",
	supported: []string{mcpOutputText, mcpOutputJSON, mcpOutputSARIF},
	format:    formatMCPDoctorOutput,
	fields:    mcpDoctorFields,
	isError: func(rep *doctor.Report) bool {
		return !rep.OK()
	},
}

func (s *Server) callDoctor(ctx context.Context, raw json.RawMessage) (map[string]any, error) {
	var args struct {
		Root   string `json:"root"`
		Output string `json:"output"`
	}
	_ = json.Unmarshal(raw, &args)
	if _, err := sandboxRoot(args.Root); err != nil {
		return textResult(err.Error(), true), nil
	}
	output, err := doctorMCPTool.resolveOutput(args.Output)
	if err != nil {
		return textResult(err.Error(), true), nil
	}
	rep := runDoctorReport(ctx, doctor.Options{Root: args.Root})
	out, err := doctorMCPTool.buildResult(rep, output)
	if err != nil {
		return textResult(err.Error(), true), nil
	}
	return out, nil
}

func formatMCPDoctorOutput(rep *doctor.Report, output string) (string, error) {
	return formatMCPOutput(output, map[string]outputWriter{
		mcpOutputText:  func(w io.Writer) error { return doctor.WriteText(w, rep) },
		mcpOutputJSON:  func(w io.Writer) error { return doctor.WriteJSON(w, rep) },
		mcpOutputSARIF: func(w io.Writer) error { return doctor.WriteSARIF(w, rep) },
	})
}

func mcpDoctorFields(rep *doctor.Report) map[string]any {
	return map[string]any{
		"root":    rep.Root,
		"scope":   rep.Scope,
		"summary": rep.Summary,
		"checks":  rep.Checks,
		"ok":      rep.OK(),
	}
}
