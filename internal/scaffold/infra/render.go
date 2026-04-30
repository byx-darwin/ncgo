package infra

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// AddResultView is the stable machine-readable shape used by CLI JSON output
// and MCP tool result fields for Add results.
type AddResultView struct {
	DryRun       bool       `json:"dryRun"`
	Updated      bool       `json:"updated"`
	WrittenPath  string     `json:"writtenPath,omitempty"`
	WrittenPaths []string   `json:"writtenPaths"`
	WiredPaths   []string   `json:"wiredPaths"`
	NextSteps    []string   `json:"nextSteps"`
	Plan         []PlanItem `json:"plan"`
}

// NewAddResultView converts Add's internal result into the public output DTO.
func NewAddResultView(res *Result) AddResultView {
	if res == nil {
		return AddResultView{}
	}
	return AddResultView{
		DryRun:       res.DryRun,
		Updated:      res.Updated,
		WrittenPath:  res.WrittenPath,
		WrittenPaths: res.WrittenPaths,
		WiredPaths:   res.WiredPaths,
		NextSteps:    res.NextSteps,
		Plan:         res.Plan,
	}
}

// AddResultFields returns JSON-compatible fields for embedding in protocol
// envelopes such as MCP tool responses.
func AddResultFields(res *Result) map[string]any {
	view := NewAddResultView(res)
	fields := map[string]any{
		"dryRun":       view.DryRun,
		"updated":      view.Updated,
		"writtenPaths": view.WrittenPaths,
		"wiredPaths":   view.WiredPaths,
		"nextSteps":    view.NextSteps,
		"plan":         view.Plan,
	}
	if view.WrittenPath != "" {
		fields["writtenPath"] = view.WrittenPath
	}
	return fields
}

// WriteAddResultJSON writes the indented JSON representation used by the CLI.
func WriteAddResultJSON(out io.Writer, res *Result) error {
	enc := json.NewEncoder(out)
	enc.SetIndent("", "  ")
	return enc.Encode(NewAddResultView(res))
}

// FormatAddResultText renders the human-readable add infra result text without
// a trailing newline. Callers can add transport-specific line endings.
func FormatAddResultText(res *Result) string {
	if res == nil {
		return ""
	}
	var b strings.Builder
	writeVerb, wireVerb := "wrote", "wired"
	if res.DryRun {
		writeVerb, wireVerb = "would write", "would wire"
	}
	for _, p := range addResultWrittenPaths(res) {
		fmt.Fprintf(&b, "%s %s\n", writeVerb, p)
	}
	for _, p := range res.WiredPaths {
		fmt.Fprintf(&b, "%s %s\n", wireVerb, p)
	}
	if res.DryRun && res.Updated {
		b.WriteString("(dry-run: manifest would be updated)\n")
	} else if !res.Updated {
		b.WriteString("(manifest already lists this infra)\n")
	}
	if res.DryRun {
		b.WriteString("(dry-run: no files were written)\n")
	}
	b.WriteString("\nnext steps:\n")
	for _, step := range res.NextSteps {
		fmt.Fprintf(&b, "  $ %s\n", step)
	}
	return strings.TrimRight(b.String(), "\n")
}

func addResultWrittenPaths(res *Result) []string {
	if len(res.WrittenPaths) > 0 {
		return res.WrittenPaths
	}
	if res.WrittenPath == "" {
		return nil
	}
	return []string{res.WrittenPath}
}
