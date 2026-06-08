package plan

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestItemJSONSerialization tests JSON marshaling behavior of plan.Item.
func TestItemJSONSerialization(t *testing.T) {
	tests := []struct {
		name     string
		item     Item
		contains []string // JSON output must contain these substrings
		excludes []string // JSON output must NOT contain these substrings
	}{
		{
			name: "required_fields_only",
			item: Item{
				Kind:   "file",
				Action: "add",
			},
			contains: []string{`"kind":"file"`, `"action":"add"`},
			excludes: []string{`"path"`, `"detail"`, `"anchor"`, `"anchorSource"`},
		},
		{
			name: "all_fields_populated",
			item: Item{
				Kind:         "file",
				Action:       "overwrite",
				Path:         "/path/to/file.go",
				Detail:       "insert method stub",
				Anchor:       "ncgo:methods:start",
				AnchorSource: "internal/handler/user.go",
			},
			contains: []string{
				`"kind":"file"`,
				`"action":"overwrite"`,
				`"path":"/path/to/file.go"`,
				`"detail":"insert method stub"`,
				`"anchor":"ncgo:methods:start"`,
				`"anchorSource":"internal/handler/user.go"`,
			},
			excludes: nil,
		},
		{
			name: "manifest_item",
			item: Item{
				Kind:   "manifest",
				Action: "already_present",
				Path:   ".ncgo/manifest.yaml",
			},
			contains: []string{
				`"kind":"manifest"`,
				`"action":"already_present"`,
				`"path":".ncgo/manifest.yaml"`,
			},
			excludes: []string{`"detail"`, `"anchor"`, `"anchorSource"`},
		},
		{
			name: "next_step_item",
			item: Item{
				Kind:   "next_step",
				Action: "run",
				Detail: "go mod tidy",
			},
			contains: []string{
				`"kind":"next_step"`,
				`"action":"run"`,
				`"detail":"go mod tidy"`,
			},
			excludes: []string{`"path"`, `"anchor"`, `"anchorSource"`},
		},
		{
			name: "anchor_item",
			item: Item{
				Kind:         "method",
				Action:       "insert",
				Path:         "internal/handler/user.go",
				Anchor:       "ncgo:methods:start",
				AnchorSource: "idl/user.proto",
			},
			contains: []string{
				`"kind":"method"`,
				`"action":"insert"`,
				`"path":"internal/handler/user.go"`,
				`"anchor":"ncgo:methods:start"`,
				`"anchorSource":"idl/user.proto"`,
			},
			excludes: []string{`"detail"`},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.item)
			if err != nil {
				t.Fatalf("json.Marshal failed: %v", err)
			}
			got := string(data)
			for _, s := range tt.contains {
				if !strings.Contains(got, s) {
					t.Errorf("JSON output missing %q\nGot: %s", s, got)
				}
			}
			for _, s := range tt.excludes {
				if strings.Contains(got, s) {
					t.Errorf("JSON output unexpectedly contains %q\nGot: %s", s, got)
				}
			}
		})
	}
}

// TestItemJSONDeserialization tests JSON unmarshaling behavior of plan.Item.
func TestItemJSONDeserialization(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected Item
	}{
		{
			name:  "required_fields_only",
			input: `{"kind":"file","action":"add"}`,
			expected: Item{
				Kind:   "file",
				Action: "add",
			},
		},
		{
			name:  "all_fields",
			input: `{"kind":"file","action":"overwrite","path":"/path/to/file.go","detail":"insert method stub","anchor":"ncgo:methods:start","anchorSource":"internal/handler/user.go"}`,
			expected: Item{
				Kind:         "file",
				Action:       "overwrite",
				Path:         "/path/to/file.go",
				Detail:       "insert method stub",
				Anchor:       "ncgo:methods:start",
				AnchorSource: "internal/handler/user.go",
			},
		},
		{
			name:  "with_path_only",
			input: `{"kind":"manifest","action":"already_present","path":".ncgo/manifest.yaml"}`,
			expected: Item{
				Kind:   "manifest",
				Action: "already_present",
				Path:   ".ncgo/manifest.yaml",
			},
		},
		{
			name:  "with_detail_only",
			input: `{"kind":"next_step","action":"run","detail":"go mod tidy"}`,
			expected: Item{
				Kind:   "next_step",
				Action: "run",
				Detail: "go mod tidy",
			},
		},
		{
			name:  "empty_optional_fields",
			input: `{"kind":"file","action":"add","path":"","detail":"","anchor":"","anchorSource":""}`,
			expected: Item{
				Kind:   "file",
				Action: "add",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got Item
			if err := json.Unmarshal([]byte(tt.input), &got); err != nil {
				t.Fatalf("json.Unmarshal failed: %v", err)
			}
			if got != tt.expected {
				t.Errorf("got %+v, want %+v", got, tt.expected)
			}
		})
	}
}

// TestItemJSONRoundTrip tests that marshal/unmarshal preserves data.
func TestItemJSONRoundTrip(t *testing.T) {
	original := Item{
		Kind:         "method",
		Action:       "insert",
		Path:         "internal/handler/user.go",
		Detail:       "insert CreateUser method",
		Anchor:       "ncgo:methods:start",
		AnchorSource: "idl/user.proto",
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	var decoded Item
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	if decoded != original {
		t.Errorf("round-trip failed:\n  original: %+v\n  decoded:  %+v", original, decoded)
	}
}

// TestItemSlice tests marshaling a slice of plan items.
func TestItemSlice(t *testing.T) {
	items := []Item{
		{Kind: "file", Action: "add", Path: "internal/usecase/user.go"},
		{Kind: "file", Action: "add", Path: "internal/repository/user.go"},
		{Kind: "manifest", Action: "add", Path: ".ncgo/manifest.yaml"},
		{Kind: "next_step", Action: "run", Detail: "go mod tidy"},
	}

	data, err := json.Marshal(items)
	if err != nil {
		t.Fatalf("json.Marshal slice failed: %v", err)
	}

	var decoded []Item
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal slice failed: %v", err)
	}

	if len(decoded) != len(items) {
		t.Fatalf("slice length mismatch: got %d, want %d", len(decoded), len(items))
	}

	for i := range items {
		if decoded[i] != items[i] {
			t.Errorf("item[%d] mismatch:\n  got:  %+v\n  want: %+v", i, decoded[i], items[i])
		}
	}
}
