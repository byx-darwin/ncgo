package mcp

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/byx-darwin/ncgo/internal/scan"
)

// callAIContext scans real code and returns structured context for an ncgo
// service: domains (with file existence), methods, anchors, and issues.
func callAIContext(raw json.RawMessage) (map[string]any, error) {
	var args struct {
		Root   string `json:"root"`
		Output string `json:"output"`
	}
	_ = json.Unmarshal(raw, &args)
	s, err := scan.Scan(args.Root)
	if err != nil {
		return textResult("ncgo_ai_context: "+err.Error(), true), nil
	}
	output, err := resolveMCPOutput("ai_context", args.Output, mcpOutputText, mcpOutputJSON)
	if err != nil {
		return textResult(err.Error(), true), nil
	}
	text, err := formatAIContext(s, output)
	if err != nil {
		return textResult(err.Error(), true), nil
	}
	return buildMCPResult(text, false, mcpAIContextFields(s)), nil
}

func formatAIContext(s *scan.ScanResult, output string) (string, error) {
	switch output {
	case mcpOutputJSON:
		var buf strings.Builder
		if err := writeJSONOutput(&buf, s); err != nil {
			return "", err
		}
		return buf.String(), nil
	default:
		return formatAIContextText(s), nil
	}
}

// mcpAIContextFields exposes stable top-level fields so agents can read the
// scan without parsing content[0].text. List fields are coerced to non-nil so
// they always serialize as JSON arrays, never null.
func mcpAIContextFields(s *scan.ScanResult) map[string]any {
	domains := s.Domains
	if domains == nil {
		domains = []scan.Domain{}
	}
	issues := s.Issues
	if issues == nil {
		issues = []scan.Issue{}
	}
	return map[string]any{
		"root":    s.Root,
		"domains": domains,
		"methods": flattenMethods(s),
		"anchors": anchorSummaries(s),
		"issues":  issues,
	}
}

func formatAIContextText(s *scan.ScanResult) string {
	var b strings.Builder
	fmt.Fprintf(&b, "ncgo_ai_context for %s\n\n", s.Root)
	for _, d := range s.Domains {
		state := "missing"
		if d.UsecaseExists {
			state = "ok"
		}
		fmt.Fprintf(&b, "- %s: usecase %s, %d methods, anchors %v\n",
			d.Name, state, len(d.Methods), d.AnchorsOK)
	}
	if len(s.Issues) > 0 {
		fmt.Fprintf(&b, "\nissues (%d):\n", len(s.Issues))
		for _, i := range s.Issues {
			fmt.Fprintf(&b, "  - [%s] %s\n", i.Kind, i.Message)
		}
	}
	return b.String()
}

func flattenMethods(s *scan.ScanResult) []map[string]any {
	out := make([]map[string]any, 0)
	for _, d := range s.Domains {
		for _, m := range d.Methods {
			out = append(out, map[string]any{
				"domain": d.Name,
				"name":   m.Name,
				"file":   m.File,
				"line":   m.Line,
			})
		}
	}
	return out
}

func anchorSummaries(s *scan.ScanResult) []map[string]any {
	out := make([]map[string]any, 0)
	for _, d := range s.Domains {
		out = append(out, map[string]any{"domain": d.Name, "ok": d.AnchorsOK})
	}
	return out
}
