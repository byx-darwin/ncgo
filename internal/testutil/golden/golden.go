// Package golden provides snapshot-comparison helpers for tests under the
// scaffold packages. Snapshots live in each package's testdata/golden/ tree
// so they are picked up by `go test` automatically (the testdata/ directory
// is excluded from build) and reviewed in code review the same way as any
// other source file.
//
// Usage:
//
//	golden.File(t, "mono-default/.ncgo/manifest.yaml", got)
//	golden.Tree(t, "mono-default", gotRootDir)
//
// To regenerate snapshots after an intentional change:
//
//	go test ./internal/scaffold/mono/... -run Golden -update-golden
//
// The flag is registered once per test binary; multiple test files in the
// same package share it.
package golden

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

var update = flag.Bool("update-golden", false, "rewrite golden snapshots instead of asserting")

// File compares got to the contents of testdata/<rel>. With -update-golden
// the file (and any missing parents) is created or overwritten. Without the
// flag the test fails on any byte difference, with a unified-style summary.
func File(t *testing.T, rel string, got []byte) {
	t.Helper()
	path := filepath.Join("testdata", rel)
	if *update {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("golden: mkdir %s: %v", filepath.Dir(path), err)
		}
		if err := os.WriteFile(path, got, 0o644); err != nil {
			t.Fatalf("golden: write %s: %v", path, err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("golden: read %s: %v\n(run `go test ... -update-golden` to create)", path, err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("golden mismatch for %s\n%s", path, summarize(got, want))
	}
}

// Tree walks gotRoot and compares every regular file to testdata/<rel>/<file>.
// With -update-golden the testdata/<rel> directory is wiped and rewritten so
// stale entries cannot survive a regeneration. Without the flag, files
// present in only one side are reported as missing/extra.
func Tree(t *testing.T, rel, gotRoot string) {
	t.Helper()
	dst := filepath.Join("testdata", rel)
	gotFiles, err := walkFiles(gotRoot)
	if err != nil {
		t.Fatalf("golden: walk %s: %v", gotRoot, err)
	}
	if *update {
		if err := os.RemoveAll(dst); err != nil {
			t.Fatalf("golden: clean %s: %v", dst, err)
		}
		for _, p := range gotFiles {
			body, err := os.ReadFile(filepath.Join(gotRoot, p))
			if err != nil {
				t.Fatalf("golden: read %s: %v", p, err)
			}
			out := filepath.Join(dst, p)
			if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
				t.Fatalf("golden: mkdir %s: %v", filepath.Dir(out), err)
			}
			if err := os.WriteFile(out, body, 0o644); err != nil {
				t.Fatalf("golden: write %s: %v", out, err)
			}
		}
		return
	}
	wantFiles, err := walkFiles(dst)
	if err != nil {
		t.Fatalf("golden: walk %s: %v\n(run with -update-golden to create)", dst, err)
	}
	if diff := diffSlices(gotFiles, wantFiles); diff != "" {
		t.Errorf("golden tree %s: file set mismatch:\n%s", rel, diff)
		return
	}
	for _, p := range gotFiles {
		got, err := os.ReadFile(filepath.Join(gotRoot, p))
		if err != nil {
			t.Fatalf("golden: read got %s: %v", p, err)
		}
		want, err := os.ReadFile(filepath.Join(dst, p))
		if err != nil {
			t.Fatalf("golden: read want %s: %v", p, err)
		}
		if !bytes.Equal(got, want) {
			t.Errorf("golden mismatch for %s/%s\n%s", rel, p, summarize(got, want))
		}
	}
}

func walkFiles(root string) ([]string, error) {
	var out []string
	err := filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		out = append(out, filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(out)
	return out, nil
}

// diffSlices returns a human-readable description of additions / deletions
// between two sorted file-name slices.
func diffSlices(got, want []string) string {
	g := map[string]bool{}
	for _, s := range got {
		g[s] = true
	}
	w := map[string]bool{}
	for _, s := range want {
		w[s] = true
	}
	var extra, missing []string
	for _, s := range got {
		if !w[s] {
			extra = append(extra, "+ "+s)
		}
	}
	for _, s := range want {
		if !g[s] {
			missing = append(missing, "- "+s)
		}
	}
	if len(extra) == 0 && len(missing) == 0 {
		return ""
	}
	return strings.Join(append(extra, missing...), "\n")
}

func summarize(got, want []byte) string {
	const max = 400
	gs := preview(got, max)
	ws := preview(want, max)
	return fmt.Sprintf("--- want (%d bytes) ---\n%s\n--- got (%d bytes) ---\n%s", len(want), ws, len(got), gs)
}

func preview(b []byte, n int) string {
	if len(b) <= n {
		return string(b)
	}
	return string(b[:n]) + "...<truncated>"
}
