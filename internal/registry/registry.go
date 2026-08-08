// Package registry implements the template registry client: it mirrors an
// official template package git repository into a local cache and exposes
// list/pull/local-path operations for `ncgo template` and `ncgo new --template`.
package registry

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	ncgoexec "github.com/byx-darwin/ncgo/internal/exec"
	"github.com/byx-darwin/ncgo/internal/scaffold/template"
)

const (
	// DefaultURL is the official template registry repository.
	DefaultURL = "https://github.com/byx-darwin/ncgo-templates.git"
	// EnvOverride overrides the default registry URL when set.
	EnvOverride = "NCGO_REGISTRY"
)

// ResolveURL returns the registry URL to use, honoring flag > env > default.
func ResolveURL(flagValue string) string {
	if flagValue != "" {
		return flagValue
	}
	if v := os.Getenv(EnvOverride); v != "" {
		return v
	}
	return DefaultURL
}

// CacheDir returns the default template registry cache directory.
func CacheDir() (string, error) {
	base, err := os.UserCacheDir()
	if err != nil {
		return "", fmt.Errorf("registry: user cache dir: %w", err)
	}
	return filepath.Join(base, "ncgo", "template-registry"), nil
}

// Entry describes one template package available in the registry.
type Entry struct {
	Name        string
	Kind        string
	Description string
}

// Client manages a local mirror of the template registry git repository.
type Client struct {
	URL    string
	Runner ncgoexec.Runner // nil -> exec.NewDefault() lazily
	Root   string          // cache root override; empty -> CacheDir()
}

// NewClient returns a Client using url and runner. A nil runner is replaced
// lazily with the default exec runner; an empty Root resolves to CacheDir().
func NewClient(url string, runner ncgoexec.Runner) *Client {
	return &Client{URL: url, Runner: runner}
}

func (c *Client) runner() ncgoexec.Runner {
	if c.Runner == nil {
		c.Runner = ncgoexec.NewDefault()
	}
	return c.Runner
}

func (c *Client) root() (string, error) {
	if c.Root != "" {
		return c.Root, nil
	}
	return CacheDir()
}

// LocalPath returns the on-disk path of a template package in the cache,
// without touching the network or checking existence. A cache root resolution
// failure yields an empty string.
func (c *Client) LocalPath(name string) string {
	root, err := c.root()
	if err != nil {
		return ""
	}
	return filepath.Join(root, name)
}

// List returns the template packages available in the registry, sorted by
// name. Directories without template.yaml are skipped.
func (c *Client) List(ctx context.Context) ([]Entry, error) {
	root, err := c.ensureCache(ctx)
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, fmt.Errorf("registry: read cache: %w", err)
	}
	var out []Entry
	for _, e := range entries {
		if !e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		meta, err := template.ReadPackageMeta(filepath.Join(root, e.Name()))
		if err != nil {
			continue // skip dirs without a readable template.yaml (noise or broken entry)
		}
		out = append(out, Entry{Name: meta.Name, Kind: meta.Kind, Description: meta.Description})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// Pull returns the on-disk directory of the named template package, ensuring
// the registry cache is up to date first. An error is returned when the
// package is not present in the registry.
func (c *Client) Pull(ctx context.Context, name string) (string, error) {
	root, err := c.ensureCache(ctx)
	if err != nil {
		return "", err
	}
	dir := filepath.Join(root, name)
	if _, err := os.Stat(filepath.Join(dir, "template.yaml")); err != nil {
		return "", fmt.Errorf("template %q not found in registry %s (run: ncgo template list)", name, c.URL)
	}
	return dir, nil
}

// ensureCache makes the local registry mirror current: it runs `git pull
// --ff-only` when the cache already exists, otherwise clones the registry
// shallowly into Root.
func (c *Client) ensureCache(ctx context.Context) (string, error) {
	root, err := c.root()
	if err != nil {
		return "", err
	}
	runner := c.runner()
	if _, err := os.Stat(filepath.Join(root, ".git")); err == nil {
		if _, err := runner.Run(ctx, ncgoexec.Cmd{Name: "git", Args: []string{"pull", "--ff-only"}, Dir: root}); err != nil {
			return "", c.wrapGitErr(err)
		}
		return root, nil
	}
	if err := os.MkdirAll(filepath.Dir(root), 0o755); err != nil {
		return "", fmt.Errorf("registry: cache dir: %w", err)
	}
	if _, err := runner.Run(ctx, ncgoexec.Cmd{Name: "git", Args: []string{"clone", "--depth", "1", c.URL, root}}); err != nil {
		return "", c.wrapGitErr(err)
	}
	return root, nil
}

func (c *Client) wrapGitErr(err error) error {
	var nf *ncgoexec.NotFoundError
	if errors.As(err, &nf) {
		return errors.New("git is required for registry access")
	}
	return fmt.Errorf("registry unavailable (%s): %v", c.URL, err)
}
