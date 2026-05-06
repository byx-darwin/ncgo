package mcp

type schemaField struct {
	name   string
	schema map[string]any
}

func schemaObject(required []string, fields ...schemaField) map[string]any {
	props := make(map[string]any, len(fields))
	for _, field := range fields {
		props[field.name] = field.schema
	}
	return map[string]any{"type": "object", "properties": props, "required": required, "additionalProperties": false}
}

func field(name string, schema map[string]any) schemaField {
	return schemaField{name: name, schema: schema}
}

func stringField(name, desc string) schemaField {
	return field(name, stringProp(desc))
}

func boolField(name, desc string) schemaField {
	return field(name, boolProp(desc))
}

func enumField(name string, vals []string) schemaField {
	return field(name, enumProp(vals))
}

func stringArrayField(name, desc string) schemaField {
	return field(name, map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": desc})
}

func rootField(desc string) schemaField {
	return stringField("root", desc)
}

func outputTextJSONField() schemaField {
	return enumField("output", []string{mcpOutputText, mcpOutputJSON})
}

func outputTextJSONSARIFField() schemaField {
	return enumField("output", []string{mcpOutputText, mcpOutputJSON, mcpOutputSARIF})
}

func stringProp(desc string) map[string]any {
	return map[string]any{"type": "string", "description": desc}
}

func boolProp(desc string) map[string]any {
	return map[string]any{"type": "boolean", "description": desc}
}

func enumProp(vals []string) map[string]any {
	return map[string]any{"type": "string", "enum": vals}
}
