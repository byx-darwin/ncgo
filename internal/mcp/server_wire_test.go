package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
)

// TestServeNewlineDelimitedWireFormat proves the stdio transport speaks MCP's
// newline-delimited JSON, not LSP Content-Length framing: a client that sends
// raw '\n'-terminated JSON (as real MCP clients do) must receive
// '\n'-terminated single-line JSON responses with no Content-Length header.
func TestServeNewlineDelimitedWireFormat(t *testing.T) {
	input := []byte(`{"jsonrpc":"2.0","id":1,"method":"initialize"}` + "\n" +
		`{"jsonrpc":"2.0","id":2,"method":"tools/list"}` + "\n")
	var out bytes.Buffer
	if err := New("v", "a").Serve(context.Background(), bytes.NewReader(input), &out); err != nil {
		t.Fatalf("Serve: %v", err)
	}
	raw := out.String()
	if strings.Contains(raw, "Content-Length") {
		t.Fatalf("output must not use Content-Length framing, got:\n%s", raw)
	}
	lines := strings.Split(strings.TrimRight(raw, "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 newline-delimited responses, got %d:\n%s", len(lines), raw)
	}
	for i, line := range lines {
		var resp map[string]any
		if err := json.Unmarshal([]byte(line), &resp); err != nil {
			t.Fatalf("response %d is not single-line JSON: %v\nline=%q", i, err, line)
		}
	}
	var init map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &init); err != nil {
		t.Fatalf("decode initialize response: %v", err)
	}
	result := init["result"].(map[string]any)
	if result["protocolVersion"] != protocolVersion {
		t.Fatalf("protocolVersion = %v, want %s", result["protocolVersion"], protocolVersion)
	}
}

// TestServeSkipsMalformedLine proves a non-JSON line is skipped without
// crashing the serve loop.
func TestServeSkipsMalformedLine(t *testing.T) {
	input := []byte("this is not json\n" + `{"jsonrpc":"2.0","id":7,"method":"initialize"}` + "\n")
	var out bytes.Buffer
	if err := New("v", "a").Serve(context.Background(), bytes.NewReader(input), &out); err != nil {
		t.Fatalf("Serve: %v", err)
	}
	responses, err := DecodeResponses(out.Bytes())
	if err != nil {
		t.Fatalf("DecodeResponses: %v", err)
	}
	if len(responses) != 1 {
		t.Fatalf("expected 1 response (malformed line skipped), got %d", len(responses))
	}
}

// TestServeHandlesMissingTrailingNewline proves a final message without a
// trailing '\n' is still processed.
func TestServeHandlesMissingTrailingNewline(t *testing.T) {
	input := []byte(`{"jsonrpc":"2.0","id":3,"method":"initialize"}`) // no trailing newline
	var out bytes.Buffer
	if err := New("v", "a").Serve(context.Background(), bytes.NewReader(input), &out); err != nil {
		t.Fatalf("Serve: %v", err)
	}
	responses, err := DecodeResponses(out.Bytes())
	if err != nil {
		t.Fatalf("DecodeResponses: %v", err)
	}
	if len(responses) != 1 {
		t.Fatalf("expected 1 response, got %d", len(responses))
	}
}
