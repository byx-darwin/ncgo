package mcp

import "testing"

func TestReferenceMetadataCoversEveryRegisteredTool(t *testing.T) {
	references, err := ReferenceTools()
	if err != nil {
		t.Fatalf("ReferenceTools: %v", err)
	}
	registered := New("test", "test").tools()
	if len(references) != len(registered) {
		t.Fatalf("references = %d, registered tools = %d", len(references), len(registered))
	}
	if len(references) != 23 {
		t.Fatalf("registered MCP tools = %d, want 23", len(references))
	}
	for i, reference := range references {
		if reference.Name != registered[i].Name {
			t.Fatalf("reference[%d] = %q, registered = %q", i, reference.Name, registered[i].Name)
		}
		if reference.Description == "" || reference.DescriptionZH == "" || len(reference.OutputFormats) == 0 || len(reference.ResultFields) == 0 || reference.SideEffects == "" || reference.SideEffectsZH == "" {
			t.Fatalf("incomplete reference metadata for %q: %+v", reference.Name, reference)
		}
		dataSchema := registered[i].OutputSchema["properties"].(map[string]any)["data"].(map[string]any)
		dataProperties := dataSchema["properties"].(map[string]any)
		for _, field := range reference.ResultFields {
			if reservedEnvelopeField(field) || field == "content[0].text" {
				continue
			}
			if _, ok := dataProperties[field]; !ok {
				t.Fatalf("tool %q stable field %q missing from outputSchema.data", reference.Name, field)
			}
		}
	}
}
