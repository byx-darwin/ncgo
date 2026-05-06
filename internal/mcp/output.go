package mcp

import (
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"slices"
	"strings"
)

const (
	mcpOutputText  = "text"
	mcpOutputJSON  = "json"
	mcpOutputSARIF = "sarif"
)

type outputWriter func(io.Writer) error

type structuredMCPTool[T any] struct {
	name      string
	supported []string
	format    func(T, string) (string, error)
	fields    func(T) map[string]any
	isError   func(T) bool
}

func (t structuredMCPTool[T]) resolveOutput(output string) (string, error) {
	return resolveMCPOutput(t.name, output, t.supported...)
}

func (t structuredMCPTool[T]) buildResult(res T, output string) (map[string]any, error) {
	text, err := t.format(res, output)
	if err != nil {
		return nil, err
	}
	return buildMCPResult(text, t.isError(res), t.fields(res)), nil
}

func resolveMCPOutput(toolName, output string, supported ...string) (string, error) {
	if strings.TrimSpace(output) == "" {
		return mcpOutputText, nil
	}
	if slices.Contains(supported, output) {
		return output, nil
	}
	return "", fmt.Errorf("%s: unsupported output %q; want %s", toolName, output, formatOutputList(supported))
}

func formatMCPOutput(output string, writers map[string]outputWriter) (string, error) {
	write, ok := writers[output]
	if !ok {
		return "", fmt.Errorf("mcp: missing renderer for output %q", output)
	}
	var buf strings.Builder
	if err := write(&buf); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func writeJSONOutput(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func buildMCPResult(text string, isError bool, fields map[string]any) map[string]any {
	out := textResult(text, isError)
	maps.Copy(out, fields)
	return out
}

func textResult(text string, isError bool) map[string]any {
	return map[string]any{"content": []map[string]string{{"type": "text", "text": text}}, "isError": isError}
}

func formatOutputList(items []string) string {
	switch len(items) {
	case 0:
		return ""
	case 1:
		return items[0]
	case 2:
		return items[0] + " or " + items[1]
	default:
		return strings.Join(items[:len(items)-1], ", ") + ", or " + items[len(items)-1]
	}
}
