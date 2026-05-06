package mcp

import (
	"reflect"
	"testing"
)

func TestSchemaObject(t *testing.T) {
	schema := schemaObject([]string{"root"}, rootField("Project root"), outputTextJSONField())
	if schema["type"] != "object" || schema["additionalProperties"] != false {
		t.Fatalf("schema header = %+v", schema)
	}
	if !reflect.DeepEqual(schema["required"], []string{"root"}) {
		t.Fatalf("required = %+v", schema["required"])
	}
	props := schema["properties"].(map[string]any)
	if _, ok := props["root"]; !ok {
		t.Fatalf("missing root property: %+v", props)
	}
	if _, ok := props["output"]; !ok {
		t.Fatalf("missing output property: %+v", props)
	}
}

func TestStringArrayField(t *testing.T) {
	field := stringArrayField("files", "proto files")
	if field.name != "files" {
		t.Fatalf("name = %q, want files", field.name)
	}
	if field.schema["type"] != "array" || field.schema["description"] != "proto files" {
		t.Fatalf("schema = %+v", field.schema)
	}
	items := field.schema["items"].(map[string]any)
	if items["type"] != "string" {
		t.Fatalf("items = %+v", items)
	}
}

func TestOutputFields(t *testing.T) {
	textJSON := outputTextJSONField()
	if textJSON.name != "output" {
		t.Fatalf("name = %q, want output", textJSON.name)
	}
	if !reflect.DeepEqual(textJSON.schema["enum"], []string{mcpOutputText, mcpOutputJSON}) {
		t.Fatalf("enum = %+v", textJSON.schema["enum"])
	}

	textJSONSARIF := outputTextJSONSARIFField()
	if !reflect.DeepEqual(textJSONSARIF.schema["enum"], []string{mcpOutputText, mcpOutputJSON, mcpOutputSARIF}) {
		t.Fatalf("enum = %+v", textJSONSARIF.schema["enum"])
	}
}

func TestPrimitiveFieldHelpers(t *testing.T) {
	if got := rootField("Project root"); got.name != "root" || got.schema["description"] != "Project root" {
		t.Fatalf("rootField = %+v", got)
	}
	if got := boolField("dryRun", "Dry run"); got.schema["type"] != "boolean" {
		t.Fatalf("boolField = %+v", got)
	}
	if got := enumField("mode", []string{"dev", "release"}); !reflect.DeepEqual(got.schema["enum"], []string{"dev", "release"}) {
		t.Fatalf("enumField = %+v", got)
	}
	if got := stringField("spec", "<domain>.<Method>"); got.schema["type"] != "string" {
		t.Fatalf("stringField = %+v", got)
	}
}
