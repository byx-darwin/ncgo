package ai

import (
	"strings"
	"testing"
)

func TestRenderClaudeRestructured(t *testing.T) {
	inputs := renderInputs{
		SourceRef:    ".ncgo/manifest.yaml",
		LongBody:     "## Quick Facts\n\n- module: `github.com/acme/user-api`\n- service.name: `user-api`\n- service.kind: `hertz`\n- domains: `[user]`\n",
		WorkflowBody: "## Implementing a Feature with ncgo\n\n1. **Add domain**",
		MethodsByDomain: map[string][]string{
			"user": {"Create", "Get", "Delete"},
		},
		ErrorCodes:     "| 10000 | CodeSystem | 500 | System error |\n| 40100+ | Business codes | 200 | Application-defined |",
		EditBoundaries: "## Boundaries\n\n### You may edit\n\n| Path | Purpose |\n|------|---------|\n| `internal/usecase/` | Business logic |\n",
	}
	got := renderClaude(inputs)
	for _, want := range []string{
		"Project Context for Claude Code",
		"Quick Facts",
		"### Methods",
		"Create", "Get", "Delete",
		"## Boundaries",
		"## Layer Rules",
		"## Error Codes",
		"## Workflow",
		"## Verify",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("renderClaude missing %q:\n%s", want, got)
		}
	}
	// Must NOT contain the full design doc
	if strings.Contains(got, "Generated Project Architecture") {
		t.Errorf("renderClaude should not embed full design doc:\n%s", got)
	}
}

func TestRenderAgentsRestructured(t *testing.T) {
	inputs := renderInputs{
		SourceRef:    ".ncgo/manifest.yaml",
		LongBody:     "## Quick Facts\n\n- module: `github.com/acme/user-api`\n- domains: `[user]`\n",
		WorkflowBody: "## Implementing a Feature with ncgo\n\n1. **Add domain**",
		MethodsByDomain: map[string][]string{
			"user": {"Create", "Get"},
		},
		ErrorCodes:     "| 10000 | CodeSystem | 500 | System error |",
		EditBoundaries: "## Boundaries\n\n### You may edit\n\n| Path | Purpose |\n|------|---------|\n",
	}
	got := renderAgents(inputs)
	for _, want := range []string{
		"Project Agent Context",
		"Quick Facts",
		"### Methods",
		"Create", "Get",
		"## Boundaries",
		"## Layer Rules",
		"## Error Codes",
		"## Workflow",
		"## Verify",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("renderAgents missing %q:\n%s", want, got)
		}
	}
}

func TestRenderProjectContextRestructured(t *testing.T) {
	inputs := renderInputs{
		SourceRef:          ".ncgo/manifest.yaml",
		ProjectContextBody: "## Project Facts\n\n- module: `github.com/acme/user-api`\n- domains: `[user]`\n\n## Architecture Summary\n\nThis is a summary.\n\n## Repository Rules\n\n- `.claude/rules/go.md`\n\n## Notes\n\n- Generated file.\n",
		MethodsByDomain: map[string][]string{
			"user": {"Create"},
		},
	}
	got := renderProjectContext(inputs)
	for _, want := range []string{
		"Claude Project Context",
		"Project Facts",
		"### Methods",
		"Create",
		"Architecture Summary",
		"Repository Rules",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("renderProjectContext missing %q:\n%s", want, got)
		}
	}
}
