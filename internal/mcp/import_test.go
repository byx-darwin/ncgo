package mcp

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/byx-darwin/ncgo/internal/manifest"
)

var errImportFixture = errors.New("go.mod not found at /repo/no-go-mod/go.mod; this command requires an existing Go module")

func TestServeToolCallImport(t *testing.T) {
	old := runImportDetect
	runImportDetect = func(opts importDetectOptions) (*manifest.Manifest, error) {
		return &manifest.Manifest{
			Ncgo:    manifest.Meta{Version: "test-version", AssetsVersion: "test-assets"},
			Mode:    manifest.ModeMono,
			Module:  "github.com/acme/user-api",
			Service: manifest.Service{Name: "user-api", Kind: manifest.KindHertz},
		}, nil
	}
	defer func() { runImportDetect = old }()

	input := EncodeMessage(map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": "tools/call",
		"params": map[string]any{"name": "ncgo_import", "arguments": map[string]any{"root": "/repo/user-api"}},
	})
	var out bytes.Buffer
	if err := New("test-version", "test-assets").Serve(context.Background(), bytes.NewReader(input), &out); err != nil {
		t.Fatalf("Serve: %v", err)
	}
	responses, err := DecodeResponses(out.Bytes())
	if err != nil {
		t.Fatalf("DecodeResponses: %v", err)
	}
	result := responses[0].Result.(map[string]any)
	if result["isError"].(bool) {
		t.Fatalf("import preview unexpectedly failed: %+v", result)
	}
	if result["preview"].(bool) != true {
		t.Fatalf("preview = %v, want true", result["preview"])
	}
	if result["module"].(string) != "github.com/acme/user-api" {
		t.Fatalf("module = %v, want github.com/acme/user-api", result["module"])
	}
	svc, ok := result["service"].(map[string]any)
	if !ok {
		t.Fatalf("result missing service field or wrong type: %+v", result)
	}
	if svc["kind"].(string) != manifest.KindHertz {
		t.Fatalf("service.kind = %v, want %v", svc["kind"], manifest.KindHertz)
	}
	if !strings.Contains(resultText(result), "Preview of generated manifest") {
		t.Fatalf("content missing preview header: %s", resultText(result))
	}
	if !strings.Contains(resultText(result), "github.com/acme/user-api") {
		t.Fatalf("content missing module path: %s", resultText(result))
	}
}

func TestServeToolCallImportError(t *testing.T) {
	old := runImportDetect
	runImportDetect = func(opts importDetectOptions) (*manifest.Manifest, error) {
		return nil, errImportFixture
	}
	defer func() { runImportDetect = old }()

	input := EncodeMessage(map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": "tools/call",
		"params": map[string]any{"name": "ncgo_import", "arguments": map[string]any{"root": "/repo/no-go-mod"}},
	})
	var out bytes.Buffer
	if err := New("test-version", "test-assets").Serve(context.Background(), bytes.NewReader(input), &out); err != nil {
		t.Fatalf("Serve: %v", err)
	}
	responses, err := DecodeResponses(out.Bytes())
	if err != nil {
		t.Fatalf("DecodeResponses: %v", err)
	}
	result := responses[0].Result.(map[string]any)
	if !result["isError"].(bool) {
		t.Fatalf("expected error result, got: %+v", result)
	}
	if !strings.Contains(resultText(result), "ncgo_import:") {
		t.Fatalf("content missing tool prefix: %s", resultText(result))
	}
}
