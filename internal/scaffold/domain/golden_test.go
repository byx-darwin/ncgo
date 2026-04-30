package domain

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/byx-darwin/ncgo/internal/testutil/golden"
)

// renderForGolden runs Add against a fresh seeded manifest and returns the
// project root so the snapshot lives under testdata/golden/<case>/internal/...
func renderForGolden(t *testing.T, name string) string {
	t.Helper()
	root := seedManifest(t, nil)
	if _, err := Add(Options{Root: root, Name: name}); err != nil {
		t.Fatalf("Add %s: %v", name, err)
	}
	return root
}

// goldenSubtree captures only the files Add wrote, not the seeded manifest,
// so the snapshot stays focused on the rendered domain triplet.
func goldenSubtree(t *testing.T, root, rel string) string {
	t.Helper()
	dir := filepath.Join(root, rel)
	if _, err := os.Stat(dir); err != nil {
		t.Fatalf("expected %s to exist: %v", dir, err)
	}
	return dir
}

func TestAddGoldenDevice(t *testing.T) {
	root := renderForGolden(t, "device")
	for _, sub := range []string{
		filepath.Join("internal", "usecase", "device"),
		filepath.Join("internal", "repository", "device"),
		filepath.Join("internal", "base", "data"),
	} {
		dir := goldenSubtree(t, root, sub)
		golden.Tree(t, filepath.Join("domain-device", sub), dir)
	}
}

func TestAddGoldenUserProfile(t *testing.T) {
	root := renderForGolden(t, "user_profile")
	for _, sub := range []string{
		filepath.Join("internal", "usecase", "user_profile"),
		filepath.Join("internal", "repository", "user_profile"),
		filepath.Join("internal", "base", "data"),
	} {
		dir := goldenSubtree(t, root, sub)
		golden.Tree(t, filepath.Join("domain-user_profile", sub), dir)
	}
}
