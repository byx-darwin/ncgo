package manifest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func sample() *Manifest {
	return &Manifest{
		Ncgo:    Meta{Version: "0.1.0-dev", AssetsVersion: "449c1e7+dirty"},
		Mode:    ModeMono,
		Module:  "github.com/acme/user-api",
		Service: Service{Name: "user-api", Kind: KindHertz, WithDatabase: false, IDL: "idl/app/user.proto"},
		Infra:   []string{"redis"},
		Domains: []string{"device", "theme"},
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	root := t.TempDir()
	in := sample()
	if err := Save(root, in); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if _, err := os.Stat(Path(root)); err != nil {
		t.Fatalf("manifest file not created: %v", err)
	}
	out, err := Load(root)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if out.Module != in.Module || out.Service.Name != in.Service.Name {
		t.Errorf("round-trip mismatch: got module=%q name=%q", out.Module, out.Service.Name)
	}
	if out.GeneratedAt.IsZero() {
		t.Errorf("GeneratedAt should be stamped on Save")
	}
	if len(out.Infra) != 1 || out.Infra[0] != "redis" {
		t.Errorf("infra not preserved: %v", out.Infra)
	}
}

func TestSavePreservesExplicitGeneratedAt(t *testing.T) {
	root := t.TempDir()
	want := time.Date(2026, 4, 29, 6, 54, 11, 0, time.UTC)
	in := sample()
	in.GeneratedAt = want
	if err := Save(root, in); err != nil {
		t.Fatalf("Save: %v", err)
	}
	out, err := Load(root)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !out.GeneratedAt.Equal(want) {
		t.Errorf("GeneratedAt = %v, want %v", out.GeneratedAt, want)
	}
}

func TestLoadMissingFile(t *testing.T) {
	if _, err := Load(t.TempDir()); err == nil {
		t.Fatalf("Load on empty dir should error")
	}
}

func TestPathLayout(t *testing.T) {
	got := Path("/x/y")
	want := filepath.Join("/x/y", ".ncgo", "manifest.yaml")
	if got != want {
		t.Errorf("Path = %q, want %q", got, want)
	}
}

func TestValidateRejectsBadInputs(t *testing.T) {
	cases := []struct {
		name string
		mut  func(*Manifest)
		want string
	}{
		{"missing mode", func(m *Manifest) { m.Mode = "" }, "mode is required"},
		{"bad mode", func(m *Manifest) { m.Mode = "soup" }, "mode \"soup\""},
		{"missing module", func(m *Manifest) { m.Module = "" }, "module is required"},
		{"missing service name", func(m *Manifest) { m.Service.Name = "" }, "service.name"},
		{"missing service kind", func(m *Manifest) { m.Service.Kind = "" }, "service.kind is required"},
		{"bad service kind", func(m *Manifest) { m.Service.Kind = "grpc" }, "service.kind \"grpc\""},
		{"missing ncgo version", func(m *Manifest) { m.Ncgo.Version = "" }, "ncgo.version"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := sample()
			tc.mut(m)
			err := m.Validate()
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tc.want)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error %q does not contain %q", err.Error(), tc.want)
			}
		})
	}
}

func TestSaveRejectsInvalid(t *testing.T) {
	root := t.TempDir()
	bad := sample()
	bad.Module = ""
	if err := Save(root, bad); err == nil {
		t.Fatalf("Save should reject invalid manifest")
	}
}

func TestSaveAtomicNoPartialFile(t *testing.T) {
	root := t.TempDir()
	in := sample()
	if err := Save(root, in); err != nil {
		t.Fatalf("Save: %v", err)
	}
	entries, err := os.ReadDir(filepath.Join(root, Dir))
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(entries) != 1 || entries[0].Name() != FileName {
		names := make([]string, 0, len(entries))
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Errorf(".ncgo/ contents = %v, want exactly [%s]", names, FileName)
	}
}
