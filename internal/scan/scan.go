// Package scan inspects the real code of an ncgo service and reports what
// actually exists: domains, usecase methods, method-insertion anchors, and
// consistency between the manifest and the filesystem. It is consumed by the
// `ncgo_ai_context` MCP tool and the `ncgo check` CLI command.
package scan

import (
	"fmt"
	"go/ast"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/byx-darwin/ncgo/internal/manifest"
)

const (
	// GeneratedAtMarker prefixes the timestamp line `ai sync` writes into
	// rendered context files; it ends with " -->". `ncgo check` reads it to
	// detect stale context.
	GeneratedAtMarker = "<!-- ncgo:generated-at: "

	startMarker = "// ncgo:methods:start"
	endMarker   = "// ncgo:methods:end"
)

const (
	IssueMissingUsecase   = "missing_usecase"
	IssueUndeclaredDomain = "undeclared_domain"
	IssueAnchorMissing    = "anchor_missing"
	IssueAnchorUnpaired   = "anchor_unpaired"
)

// Domain captures one domain's manifest vs on-disk state.
type Domain struct {
	Name           string   `json:"name"`
	ManifestListed bool     `json:"manifestListed"`
	UsecaseExists  bool     `json:"usecaseExists"`
	RepoExists     bool     `json:"repoExists"`
	Methods        []Method `json:"methods"`
	AnchorsOK      bool     `json:"anchorsOk"`
}

// Method is one usecase method found in real code.
type Method struct {
	Name string `json:"name"`
	File string `json:"file,omitempty"`
	Line int    `json:"line,omitempty"`
}

// Issue is one manifest-vs-code inconsistency or anchor problem.
type Issue struct {
	Kind    string `json:"kind"`
	Message string `json:"message"`
	File    string `json:"file,omitempty"`
}

// ScanResult is the structured result of inspecting one service root.
type ScanResult struct {
	Root    string   `json:"root"`
	Domains []Domain `json:"domains"`
	Issues  []Issue  `json:"issues"`
}

// Scan inspects the service at root. It returns an error only when the root
// is not an ncgo service (no .ncgo/manifest.yaml). Code-level inconsistencies
// are reported as Issues, not errors.
func Scan(root string) (*ScanResult, error) {
	m, err := manifest.Load(root)
	if err != nil {
		return nil, err
	}
	s := &ScanResult{Root: root}
	seen := map[string]bool{}
	usecaseDir := filepath.Join(root, "internal", "usecase")
	if entries, err := os.ReadDir(usecaseDir); err == nil {
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			name := e.Name()
			seen[name] = true
			listed := domainListed(m, name)
			if !listed {
				s.Issues = append(s.Issues, Issue{
					Kind:    IssueUndeclaredDomain,
					Message: fmt.Sprintf("domain %q exists under internal/usecase but is not listed in manifest", name),
					File:    filepath.Join(usecaseDir, name),
				})
			}
			usecasePath := filepath.Join(usecaseDir, name, name+".go")
			usecaseExists := fileExists(usecasePath)
			methods, anchorsOK := scanUsecase(usecasePath)
			if usecaseExists && !anchorsOK {
				s.Issues = append(s.Issues, Issue{
					Kind:    anchorIssueKind(usecasePath),
					Message: "usecase file is missing or has unpaired // ncgo:methods:start|end anchors",
					File:    usecasePath,
				})
			}
			s.Domains = append(s.Domains, Domain{
				Name:           name,
				ManifestListed: listed,
				UsecaseExists:  usecaseExists,
				RepoExists:     fileExists(filepath.Join(root, "internal", "repository", name, name+".go")),
				Methods:        methods,
				AnchorsOK:      anchorsOK,
			})
		}
	}
	for _, name := range m.Domains {
		if seen[name] {
			continue
		}
		s.Issues = append(s.Issues, Issue{
			Kind:    IssueMissingUsecase,
			Message: fmt.Sprintf("domain %q is listed in manifest but has no internal/usecase/%s/%s.go", name, name, name),
			File:    filepath.Join(usecaseDir, name),
		})
		s.Domains = append(s.Domains, Domain{Name: name, ManifestListed: true, UsecaseExists: false})
	}
	sort.SliceStable(s.Domains, func(i, j int) bool { return s.Domains[i].Name < s.Domains[j].Name })
	return s, nil
}

// scanUsecase parses one usecase file for exported UseCase methods and
// validates the method-insertion anchors. Methods `New`, `Repo`, and other
// known accessors are excluded.
func scanUsecase(path string) ([]Method, bool) {
	var methods []Method
	anchorsOK := false
	body, err := os.ReadFile(path)
	if err != nil {
		return nil, false
	}
	start := strings.Index(string(body), startMarker)
	end := strings.Index(string(body), endMarker)
	anchorsOK = start >= 0 && end >= start
	_ = walkGoFiles(filepath.Dir(path), func(fset *token.FileSet, f *ast.File, file string) {
		if filepath.Base(file) != filepath.Base(path) {
			return
		}
		for _, decl := range f.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Recv == nil || len(fn.Recv.List) == 0 {
				continue
			}
			if receiverTypeName(fn.Recv.List[0].Type) != "UseCase" {
				continue
			}
			if isAccessor(fn.Name.Name) {
				continue
			}
			methods = append(methods, Method{
				Name: fn.Name.Name,
				File: file,
				Line: fset.Position(fn.Pos()).Line,
			})
		}
	})
	sort.SliceStable(methods, func(i, j int) bool { return methods[i].Name < methods[j].Name })
	return methods, anchorsOK
}

func receiverTypeName(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.StarExpr:
		return receiverTypeName(t.X)
	}
	return ""
}

// isAccessor reports whether the method is a constructor or accessor that
// should not count as a domain method.
func isAccessor(name string) bool {
	switch name {
	case "New", "Repo":
		return true
	}
	return false
}

func anchorIssueKind(path string) string {
	body, err := os.ReadFile(path)
	if err != nil {
		return IssueAnchorMissing
	}
	if strings.Contains(string(body), startMarker) && strings.Contains(string(body), endMarker) {
		return IssueAnchorUnpaired
	}
	return IssueAnchorMissing
}

func domainListed(m *manifest.Manifest, name string) bool {
	for _, d := range m.Domains {
		if d == name {
			return true
		}
	}
	return false
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
