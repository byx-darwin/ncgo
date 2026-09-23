package mcp

import (
	"fmt"
)

// ReferenceSafety is the rendered form of the MCP annotations and ncgo
// behavior metadata published by tools/list.
type ReferenceSafety struct {
	ReadOnly        bool
	Destructive     bool
	Idempotent      bool
	Network         bool
	ExternalProcess bool
	SupportsDryRun  bool
}

// ReferenceTool combines live registered schemas with the small amount of
// human documentation that cannot be derived from JSON Schema. The registry
// remains authoritative for names, inputs, descriptions, outputs, and safety.
type ReferenceTool struct {
	Name          string
	Description   string
	DescriptionZH string
	CLI           []string
	InputSchema   map[string]any
	OutputFormats []string
	ResultFields  []string
	SideEffects   string
	SideEffectsZH string
	Safety        ReferenceSafety
}

type referenceDetails struct {
	DescriptionZH string
	CLI           []string
	ResultFields  []string
	SideEffects   string
	SideEffectsZH string
}

func ref(descriptionZH, cli string, resultFields []string, sideEffects, sideEffectsZH string) referenceDetails {
	details := referenceDetails{
		DescriptionZH: descriptionZH,
		ResultFields:  resultFields,
		SideEffects:   sideEffects,
		SideEffectsZH: sideEffectsZH,
	}
	if cli != "" {
		details.CLI = []string{cli}
	}
	return details
}

// ReferenceTools returns documentation records derived from the live tool
// registry. Missing or stale documentation metadata is an error, which makes
// drift fail generation and CI instead of silently omitting a tool.
func ReferenceTools() ([]ReferenceTool, error) {
	registered := New("reference", "reference").tools()
	out := make([]ReferenceTool, 0, len(registered))
	for _, item := range registered {
		details := item.Reference
		if details.DescriptionZH == "" || details.SideEffects == "" || details.SideEffectsZH == "" || len(details.ResultFields) == 0 {
			return nil, fmt.Errorf("MCP reference metadata missing registered tool %q", item.Name)
		}
		behavior, _ := item.Meta["io.github.byx-darwin.ncgo/toolBehavior"].(map[string]bool)
		out = append(out, ReferenceTool{
			Name: item.Name, Description: item.Description, DescriptionZH: details.DescriptionZH,
			CLI: append([]string(nil), details.CLI...), InputSchema: item.InputSchema,
			OutputFormats: outputFormats(item.InputSchema), ResultFields: append([]string(nil), details.ResultFields...),
			SideEffects: details.SideEffects, SideEffectsZH: details.SideEffectsZH,
			Safety: ReferenceSafety{
				ReadOnly: item.Annotations.ReadOnlyHint, Destructive: item.Annotations.DestructiveHint,
				Idempotent: item.Annotations.IdempotentHint, Network: behavior["networkAccess"],
				ExternalProcess: behavior["externalProcess"], SupportsDryRun: behavior["supportsDryRun"],
			},
		})
	}
	return out, nil
}

func outputFormats(schema map[string]any) []string {
	properties, _ := schema["properties"].(map[string]any)
	output, _ := properties["output"].(map[string]any)
	values, _ := output["enum"].([]string)
	if len(values) == 0 {
		if raw, ok := output["enum"].([]any); ok {
			for _, value := range raw {
				values = append(values, fmt.Sprint(value))
			}
		}
	}
	if len(values) == 0 {
		return []string{"text"}
	}
	return append([]string(nil), values...)
}
