package cli

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	ncgoexec "github.com/byx-darwin/ncgo/internal/exec"
	"github.com/byx-darwin/ncgo/internal/registry"
)

// copyDir recursively copies the contents of src into dst, mirroring what a
// real `git clone` would leave on disk for a fixture repository.
func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, info.Mode())
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, info.Mode())
	})
}

// templateFakeGit simulates the git invocations used by the registry client.
type templateFakeGit struct {
	fixture  string // directory copied on "clone"
	cloneErr error
}

func (f *templateFakeGit) Run(_ context.Context, c ncgoexec.Cmd) (ncgoexec.Result, error) {
	if c.Name != "git" {
		return ncgoexec.Result{}, errors.New("unexpected command " + c.Name)
	}
	switch c.Args[0] {
	case "clone":
		if f.cloneErr != nil {
			return ncgoexec.Result{}, f.cloneErr
		}
		dst := c.Args[len(c.Args)-1]
		if err := copyDir(f.fixture, dst); err != nil {
			return ncgoexec.Result{}, err
		}
		return ncgoexec.Result{}, os.MkdirAll(filepath.Join(dst, ".git"), 0o755)
	case "pull":
		return ncgoexec.Result{}, nil
	}
	return ncgoexec.Result{}, errors.New("unexpected git args " + strings.Join(c.Args, " "))
}

func TestRootCmdIncludesTemplateCommand(t *testing.T) {
	cmd, _, err := newRootCmd().Find([]string{"template"})
	if err != nil {
		t.Fatalf("Find template: %v", err)
	}
	if cmd == nil || cmd.Name() != "template" {
		t.Fatalf("template command not registered")
	}
}

func TestTemplateCmdIncludesListAndPull(t *testing.T) {
	for _, sub := range []string{"list", "pull"} {
		cmd, _, err := newTemplateCmd().Find([]string{sub})
		if err != nil {
			t.Fatalf("Find template %s: %v", sub, err)
		}
		if cmd == nil || cmd.Name() != sub {
			t.Errorf("template %s subcommand not registered", sub)
		}
	}
}

func TestTemplateListOutput(t *testing.T) {
	fixture := t.TempDir()
	os.MkdirAll(filepath.Join(fixture, "base-kitex"), 0o755)
	os.WriteFile(filepath.Join(fixture, "base-kitex", "template.yaml"),
		[]byte("name: base-kitex\nkind: kitex\ndescription: base\n"), 0o644)

	var out strings.Builder
	cmd := &cobra.Command{}
	cmd.SetOut(&out)
	client := registry.NewClient("https://example.invalid/registry.git", &templateFakeGit{fixture: fixture})
	client.Root = t.TempDir()
	if err := runTemplateList(cmd, &templateOptions{}, client); err != nil {
		t.Fatalf("runTemplateList: %v", err)
	}
	if want := "base-kitex\tkitex\tbase\n"; out.String() != want {
		t.Errorf("list output = %q, want %q", out.String(), want)
	}
}

func TestTemplateListEmptyOutput(t *testing.T) {
	fixture := t.TempDir() // empty registry
	var out strings.Builder
	cmd := &cobra.Command{}
	cmd.SetOut(&out)
	client := registry.NewClient("u", &templateFakeGit{fixture: fixture})
	client.Root = t.TempDir()
	if err := runTemplateList(cmd, &templateOptions{}, client); err != nil {
		t.Fatalf("runTemplateList: %v", err)
	}
	if want := "no templates in registry\n"; out.String() != want {
		t.Errorf("list output = %q, want %q", out.String(), want)
	}
}

func TestTemplatePullOutput(t *testing.T) {
	fixture := t.TempDir()
	os.MkdirAll(filepath.Join(fixture, "base-hertz"), 0o755)
	os.WriteFile(filepath.Join(fixture, "base-hertz", "template.yaml"),
		[]byte("name: base-hertz\nkind: hertz\n"), 0o644)

	var out strings.Builder
	cmd := &cobra.Command{}
	cmd.SetOut(&out)
	client := registry.NewClient("https://example.invalid/registry.git", &templateFakeGit{fixture: fixture})
	client.Root = t.TempDir()
	if err := runTemplatePull(cmd, "base-hertz", &templateOptions{}, client); err != nil {
		t.Fatalf("runTemplatePull: %v", err)
	}
	if want := "pulled base-hertz -> " + filepath.Join(client.Root, "base-hertz") + "\n"; out.String() != want {
		t.Errorf("pull output = %q, want %q", out.String(), want)
	}
}

func TestTemplateListErrorPassthrough(t *testing.T) {
	var out strings.Builder
	cmd := &cobra.Command{}
	cmd.SetOut(&out)
	client := registry.NewClient("u", &templateFakeGit{cloneErr: errors.New("dial tcp: timeout")})
	client.Root = filepath.Join(t.TempDir(), "cache")
	err := runTemplateList(cmd, &templateOptions{}, client)
	if err == nil || !strings.Contains(err.Error(), "registry unavailable") {
		t.Errorf("want registry unavailable error, got %v", err)
	}
}
