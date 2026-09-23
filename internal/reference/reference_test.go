package reference

import (
	"os"
	"strings"
	"testing"
)

func TestCapabilityCatalogCoversEveryCLILeafAndMCPTool(t *testing.T) {
	tools, commands, err := catalog()
	if err != nil {
		t.Fatalf("catalog: %v", err)
	}
	if len(tools) != 23 {
		t.Fatalf("MCP tools = %d, want 23", len(tools))
	}
	if len(commands) != 29 {
		t.Fatalf("CLI leaf commands = %d, want 29", len(commands))
	}
}

func TestGeneratedReferencesCurrentAndLocalizedStructureAligned(t *testing.T) {
	root, err := FindRoot(".")
	if err != nil {
		t.Fatalf("FindRoot: %v", err)
	}
	if err := Generate(root, true); err != nil {
		t.Fatal(err)
	}
	en, err := os.ReadFile(root + "/docs/mcp-reference.md")
	if err != nil {
		t.Fatal(err)
	}
	zh, err := os.ReadFile(root + "/docs/mcp-reference.zh-CN.md")
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Count(string(en), "### `ncgo_"); got != 23 {
		t.Fatalf("English tool sections = %d, want 23", got)
	}
	if got := strings.Count(string(zh), "### `ncgo_"); got != 23 {
		t.Fatalf("Chinese tool sections = %d, want 23", got)
	}
	for _, tool := range mustTools(t) {
		heading := "### `" + tool.Name + "`"
		if strings.Count(string(en), heading) != 1 || strings.Count(string(zh), heading) != 1 {
			t.Fatalf("localized references do not contain exactly one %s", heading)
		}
	}
}

func mustTools(t *testing.T) []struct{ Name string } {
	t.Helper()
	tools, _, err := catalog()
	if err != nil {
		t.Fatal(err)
	}
	out := make([]struct{ Name string }, len(tools))
	for i, tool := range tools {
		out[i].Name = tool.Name
	}
	return out
}
