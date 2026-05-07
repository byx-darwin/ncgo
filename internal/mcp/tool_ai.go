package mcp

import (
	"encoding/json"
	"io"
	"strings"

	"github.com/byx-darwin/ncgo/internal/ai"
)

var aiSyncMCPTool = structuredMCPTool[*ai.Result]{
	name:      "ai sync",
	supported: []string{mcpOutputText, mcpOutputJSON},
	format:    formatMCPAISyncOutput,
	fields:    ai.ResultFields,
	isError: func(*ai.Result) bool {
		return false
	},
}

var aiInitClaudeMCPTool = structuredMCPTool[*ai.Result]{
	name:      "ai init claude",
	supported: []string{mcpOutputText, mcpOutputJSON},
	format:    formatMCPAIInitClaudeOutput,
	fields:    ai.ResultFields,
	isError: func(*ai.Result) bool {
		return false
	},
}

func callAISync(raw json.RawMessage) (map[string]any, error) {
	var args struct {
		Root   string `json:"root"`
		Lang   string `json:"lang"`
		Force  bool   `json:"force"`
		DryRun bool   `json:"dryRun"`
		Output string `json:"output"`
	}
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, err
	}
	output, err := aiSyncMCPTool.resolveOutput(args.Output)
	if err != nil {
		return textResult(err.Error(), true), nil
	}
	res, err := ai.Sync(ai.Options{Root: args.Root, Lang: args.Lang, Force: args.Force, DryRun: args.DryRun})
	if err != nil {
		return textResult(err.Error(), true), nil
	}
	out, err := aiSyncMCPTool.buildResult(res, output)
	if err != nil {
		return textResult(err.Error(), true), nil
	}
	return out, nil
}

func callAIInitClaude(raw json.RawMessage) (map[string]any, error) {
	var args struct {
		Root   string `json:"root"`
		Preset string `json:"preset"`
		Force  bool   `json:"force"`
		DryRun bool   `json:"dryRun"`
		Output string `json:"output"`
	}
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, err
	}
	output, err := aiInitClaudeMCPTool.resolveOutput(args.Output)
	if err != nil {
		return textResult(err.Error(), true), nil
	}
	res, err := ai.InitClaude(ai.InitOptions{Root: args.Root, Preset: args.Preset, Force: args.Force, DryRun: args.DryRun})
	if err != nil {
		return textResult(err.Error(), true), nil
	}
	out, err := aiInitClaudeMCPTool.buildResult(res, output)
	if err != nil {
		return textResult(err.Error(), true), nil
	}
	return out, nil
}

func formatMCPAISyncOutput(res *ai.Result, output string) (string, error) {
	return formatMCPOutput(output, map[string]outputWriter{
		mcpOutputText: func(w io.Writer) error {
			_, err := io.WriteString(w, formatAIResultSummary(res))
			return err
		},
		mcpOutputJSON: func(w io.Writer) error { return writeJSONOutput(w, res) },
	})
}

func formatMCPAIInitClaudeOutput(res *ai.Result, output string) (string, error) {
	return formatMCPOutput(output, map[string]outputWriter{
		mcpOutputText: func(w io.Writer) error {
			_, err := io.WriteString(w, formatAIResultSummary(res))
			return err
		},
		mcpOutputJSON: func(w io.Writer) error { return writeJSONOutput(w, res) },
	})
}

func formatAIResultSummary(res *ai.Result) string {
	if res == nil {
		return ""
	}
	var b strings.Builder
	for _, note := range res.Notes {
		b.WriteString("info: ")
		b.WriteString(note)
		b.WriteByte('\n')
	}
	for _, p := range res.Written {
		b.WriteString("wrote ")
		b.WriteString(p)
		b.WriteByte('\n')
	}
	for _, s := range res.Skipped {
		b.WriteString("skipped ")
		b.WriteString(s.Path)
		b.WriteString(" (")
		b.WriteString(s.Reason)
		b.WriteString(")\n")
	}
	for _, step := range res.NextSteps {
		b.WriteString("next: ")
		b.WriteString(step)
		b.WriteByte('\n')
	}
	if len(res.Written) == 0 && len(res.Skipped) == 0 {
		b.WriteString("(nothing to do)\n")
	}
	return strings.TrimRight(b.String(), "\n")
}
