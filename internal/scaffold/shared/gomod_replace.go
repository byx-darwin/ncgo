package shared

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"golang.org/x/mod/modfile"

	"github.com/byx-darwin/ncgo/internal/manifest"
)

// LocalReplace describes a go.mod replace directive whose target is a local
// filesystem path (no version on the RHS).
type LocalReplace struct {
	Module string // replaced module path (LHS)
	Target string // relative filesystem target as written, e.g. "../authority"
}

// ParseLocalReplaces reads <serviceDir>/go.mod and returns every replace
// directive whose target is a local filesystem path. Returns (nil, nil) if
// go.mod does not exist yet.
func ParseLocalReplaces(serviceDir string) ([]LocalReplace, error) {
	path := filepath.Join(serviceDir, "go.mod")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	f, err := modfile.Parse(path, data, nil)
	if err != nil {
		return nil, err
	}
	var out []LocalReplace
	for _, r := range f.Replace {
		if r.New.Version != "" {
			continue
		}
		if !strings.HasPrefix(r.New.Path, "./") && !strings.HasPrefix(r.New.Path, "../") {
			continue
		}
		out = append(out, LocalReplace{Module: r.Old.Path, Target: r.New.Path})
	}
	return out, nil
}

// SiblingDirs resolves each LocalReplace's Target against serviceRel and
// matches it by resolved filesystem path to a manifest.WorkspaceService.Dir.
// Returns matched sibling Dir values (workspace-root-relative, slash form),
// sorted and de-duplicated. Excludes the service's own Dir.
func SiblingDirs(root, serviceRel string, replaces []LocalReplace, services []manifest.WorkspaceService) []string {
	selfPath := filepath.Clean(filepath.Join(root, filepath.FromSlash(serviceRel)))
	seen := map[string]bool{}
	var out []string
	for _, r := range replaces {
		targetPath := filepath.Clean(filepath.Join(root, filepath.FromSlash(serviceRel), r.Target))
		for _, svc := range services {
			svcPath := filepath.Clean(filepath.Join(root, filepath.FromSlash(svc.Dir)))
			if svcPath == selfPath {
				continue
			}
			if svcPath == targetPath {
				dir := filepath.ToSlash(svc.Dir)
				if !seen[dir] {
					seen[dir] = true
					out = append(out, dir)
				}
			}
		}
	}
	sort.Strings(out)
	return out
}
