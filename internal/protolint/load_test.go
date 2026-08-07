package protolint

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestLoadHertzExampleProto(t *testing.T) {
	root := filepath.Clean(filepath.Join("..", "scaffold", "mono", "testdata", "mono-default", "idl"))
	model, err := Load(context.Background(), LoadOptions{
		Root:  root,
		Files: []string{"app/demo.proto"},
	})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(model.Files) != 1 {
		t.Fatalf("got %d files, want 1", len(model.Files))
	}
	f := model.Files[0]
	if f.Path != "app/demo.proto" {
		t.Fatalf("path = %q, want %q", f.Path, "app/demo.proto")
	}
	if f.Package != "app" {
		t.Fatalf("package = %q, want app", f.Package)
	}
	if len(f.Services) != 1 {
		t.Fatalf("got %d services, want 1", len(f.Services))
	}
	svc := f.Services[0]
	if svc.Name != "DemoService" {
		t.Fatalf("service name = %q, want DemoService", svc.Name)
	}
	if svc.Location.Line <= 0 {
		t.Fatalf("service location = %+v, want positive line", svc.Location)
	}
	if len(svc.RPCs) != 1 {
		t.Fatalf("got %d rpcs, want 1", len(svc.RPCs))
	}
	rpc := svc.RPCs[0]
	if rpc.Name != "Ping" {
		t.Fatalf("rpc name = %q, want Ping", rpc.Name)
	}
	if rpc.InputMessageName != "PingReq" || rpc.OutputMessageName != "PingResp" {
		t.Fatalf("rpc io = %s/%s, want PingReq/PingResp", rpc.InputMessageName, rpc.OutputMessageName)
	}
	if len(rpc.HTTPRules) != 1 {
		t.Fatalf("got %d http rules, want 1", len(rpc.HTTPRules))
	}
	if got := rpc.HTTPRules[0]; got.Method != "GET" || got.Path != "/ping" || got.Annotation != "api.get" {
		t.Fatalf("http rule = %+v, want GET /ping api.get", got)
	}
	if rpc.Location.Line <= 0 {
		t.Fatalf("rpc location = %+v, want positive line", rpc.Location)
	}
	if !rpc.HasOpenAPIOperation {
		t.Fatalf("rpc.HasOpenAPIOperation = false, want true")
	}
	if rpc.InputMessage == nil {
		t.Fatalf("input message is nil")
	}
	if rpc.OutputMessage == nil {
		t.Fatalf("output message is nil")
	}
	if len(rpc.InputMessage.Fields) != 1 {
		t.Fatalf("got %d input fields, want 1", len(rpc.InputMessage.Fields))
	}
	field := rpc.InputMessage.Fields[0]
	if field.Name != "name" {
		t.Fatalf("field name = %q, want name", field.Name)
	}
	if field.HasOpenAPIProperty != true {
		t.Fatalf("input field HasOpenAPIProperty = %v, want true", field.HasOpenAPIProperty)
	}
	if len(field.Bindings) != 1 {
		t.Fatalf("got %d bindings, want 1", len(field.Bindings))
	}
	b := field.Bindings[0]
	if b.Kind != BindingQuery || b.Annotation != "api.query" || b.Value != "name" {
		t.Fatalf("binding = %+v, want query/api.query/name", b)
	}
	if field.Location.Line <= 0 {
		t.Fatalf("field location = %+v, want positive line", field.Location)
	}
	if rpc.OutputMessage.HasOpenAPISchema != true {
		t.Fatalf("output message HasOpenAPISchema = %v, want true", rpc.OutputMessage.HasOpenAPISchema)
	}
	if got := rpc.OutputMessage.Fields[0].HasOpenAPIProperty; got != true {
		t.Fatalf("output field HasOpenAPIProperty = %v, want true", got)
	}
}

func TestLoadKitexExampleProto(t *testing.T) {
	root := filepath.Clean(filepath.Join("..", "scaffold", "mono", "testdata", "mono-kitex-default", "idl"))
	model, err := Load(context.Background(), LoadOptions{
		Root:  root,
		Files: []string{"demo.proto"},
	})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(model.Files) != 1 {
		t.Fatalf("got %d files, want 1", len(model.Files))
	}
	f := model.Files[0]
	if f.Path != "demo.proto" {
		t.Fatalf("path = %q, want demo.proto", f.Path)
	}
	if f.Package != "demo" {
		t.Fatalf("package = %q, want demo", f.Package)
	}
	if len(f.Services) != 1 {
		t.Fatalf("got %d services, want 1", len(f.Services))
	}
	svc := f.Services[0]
	if svc.Name != "Demo" {
		t.Fatalf("service name = %q, want Demo", svc.Name)
	}
	if len(svc.RPCs) != 0 {
		t.Fatalf("got %d rpcs, want 0", len(svc.RPCs))
	}
	if svc.Location.Line <= 0 {
		t.Fatalf("service location = %+v, want positive line", svc.Location)
	}
}

func TestLoadValidatesInputs(t *testing.T) {
	if _, err := Load(context.Background(), LoadOptions{}); err == nil {
		t.Fatalf("expected error for empty options")
	}
	if _, err := Load(context.Background(), LoadOptions{Root: "."}); err == nil {
		t.Fatalf("expected error for empty files")
	}
}

// TestLoadHertzGoldenProtoFromProjectRoot mirrors how doctor and the
// protolint CLI invoke Load: Root is the project root and the entry file is
// the manifest-relative idl path. Scaffold protos import support files
// relative to the idl/ directory (hz convention: compile with -I idl), e.g.
// idl/app/demo.proto imports "api.proto" which lives at idl/api.proto, so
// Load must resolve imports against idl/ in addition to the project root.
func TestLoadHertzGoldenProtoFromProjectRoot(t *testing.T) {
	root := filepath.Clean(filepath.Join("..", "scaffold", "mono", "testdata", "mono-default"))
	model, err := Load(context.Background(), LoadOptions{
		Root:  root,
		Files: []string{"idl/app/demo.proto"},
	})
	if err != nil {
		t.Fatalf("Load from project root: %v", err)
	}
	if len(model.Files) != 1 {
		t.Fatalf("got %d files, want 1", len(model.Files))
	}
	if got := model.Files[0].Path; got != "idl/app/demo.proto" {
		t.Fatalf("path = %q, want idl/app/demo.proto", got)
	}
}

// TestImportRoots locks the three branches of importRoots: root is always
// first, and <root>/idl is appended only when idl/ exists as a directory.
func TestImportRoots(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(root string) error // optional, nil means leave root empty
		wantIDL bool
	}{
		{
			name: "idl directory present",
			setup: func(root string) error {
				return os.Mkdir(filepath.Join(root, "idl"), 0o755)
			},
			wantIDL: true,
		},
		{
			name: "idl absent",
		},
		{
			name: "idl is a regular file",
			setup: func(root string) error {
				return os.WriteFile(filepath.Join(root, "idl"), []byte("not a dir"), 0o644)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			if tt.setup != nil {
				if err := tt.setup(root); err != nil {
					t.Fatalf("setup: %v", err)
				}
			}

			want := []string{root}
			if tt.wantIDL {
				want = append(want, filepath.Join(root, "idl"))
			}

			if got := importRoots(root); !reflect.DeepEqual(got, want) {
				t.Fatalf("importRoots(%q) = %v, want %v", root, got, want)
			}
		})
	}
}
