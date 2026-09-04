package shared

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/byx-darwin/ncgo/internal/manifest"
)

func TestParseLocalReplaces(t *testing.T) {
	t.Run("missing go.mod", func(t *testing.T) {
		dir := t.TempDir()
		got, err := ParseLocalReplaces(dir)
		if err != nil {
			t.Fatalf("ParseLocalReplaces: %v", err)
		}
		if got != nil {
			t.Fatalf("want nil, got %#v", got)
		}
	})

	t.Run("no replace", func(t *testing.T) {
		dir := writeGoMod(t, "module example.com/a\n\ngo 1.25\n")
		got, err := ParseLocalReplaces(dir)
		if err != nil {
			t.Fatalf("ParseLocalReplaces: %v", err)
		}
		if len(got) != 0 {
			t.Fatalf("want empty, got %#v", got)
		}
	})

	t.Run("single-line replace", func(t *testing.T) {
		dir := writeGoMod(t, "module example.com/a\n\ngo 1.25\n\nreplace example.com/b => ../b\n")
		got, err := ParseLocalReplaces(dir)
		if err != nil {
			t.Fatalf("ParseLocalReplaces: %v", err)
		}
		want := []LocalReplace{{Module: "example.com/b", Target: "../b"}}
		assertReplaces(t, got, want)
	})

	t.Run("grouped replace block", func(t *testing.T) {
		dir := writeGoMod(t, "module example.com/a\n\ngo 1.25\n\nreplace (\n\texample.com/b => ../b\n\texample.com/c => ../../c\n)\n")
		got, err := ParseLocalReplaces(dir)
		if err != nil {
			t.Fatalf("ParseLocalReplaces: %v", err)
		}
		want := []LocalReplace{{Module: "example.com/b", Target: "../b"}, {Module: "example.com/c", Target: "../../c"}}
		assertReplaces(t, got, want)
	})

	t.Run("versioned replace ignored", func(t *testing.T) {
		dir := writeGoMod(t, "module example.com/a\n\ngo 1.25\n\nreplace example.com/b => example.com/b v1.2.3\n")
		got, err := ParseLocalReplaces(dir)
		if err != nil {
			t.Fatalf("ParseLocalReplaces: %v", err)
		}
		if len(got) != 0 {
			t.Fatalf("want empty, got %#v", got)
		}
	})

	t.Run("module-path replace ignored", func(t *testing.T) {
		dir := writeGoMod(t, "module example.com/a\n\ngo 1.25\n\nreplace example.com/b => example.com/forked-b v0.0.0\n")
		got, err := ParseLocalReplaces(dir)
		if err != nil {
			t.Fatalf("ParseLocalReplaces: %v", err)
		}
		if len(got) != 0 {
			t.Fatalf("want empty, got %#v", got)
		}
	})
}

func TestSiblingDirs(t *testing.T) {
	root := t.TempDir()
	services := []manifest.WorkspaceService{
		{Name: "a", Kind: manifest.KindKitex, Dir: "services/a"},
		{Name: "b", Kind: manifest.KindKitex, Dir: "services/b"},
		{Name: "c", Kind: manifest.KindKitex, Dir: "services/c"},
	}

	t.Run("path-resolved match", func(t *testing.T) {
		replaces := []LocalReplace{{Module: "example.com/b", Target: "../b"}}
		got := SiblingDirs(root, "services/a", replaces, services)
		want := []string{"services/b"}
		assertStrings(t, got, want)
	})

	t.Run("self exclusion", func(t *testing.T) {
		replaces := []LocalReplace{{Module: "example.com/a", Target: "../a"}}
		got := SiblingDirs(root, "services/a", replaces, services)
		if len(got) != 0 {
			t.Fatalf("want empty, got %#v", got)
		}
	})

	t.Run("out of workspace target ignored", func(t *testing.T) {
		replaces := []LocalReplace{{Module: "example.com/x", Target: "../../outside"}}
		got := SiblingDirs(root, "services/a", replaces, services)
		if len(got) != 0 {
			t.Fatalf("want empty, got %#v", got)
		}
	})

	t.Run("dedup and sort", func(t *testing.T) {
		replaces := []LocalReplace{
			{Module: "example.com/c", Target: "../c"},
			{Module: "example.com/b", Target: "../b"},
			{Module: "example.com/b-again", Target: "../b"},
		}
		got := SiblingDirs(root, "services/a", replaces, services)
		want := []string{"services/b", "services/c"}
		assertStrings(t, got, want)
	})
}

func writeGoMod(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(content), 0o644); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}
	return dir
}

func assertReplaces(t *testing.T, got, want []LocalReplace) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("length mismatch: got %#v, want %#v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("index %d: got %#v, want %#v", i, got[i], want[i])
		}
	}
}

func assertStrings(t *testing.T, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("length mismatch: got %#v, want %#v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("index %d: got %q, want %q", i, got[i], want[i])
		}
	}
}
