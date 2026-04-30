package doctor

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// osStat is the indirection point for tests; production code calls os.Stat.
var osStat = os.Stat

// finding records a single rule violation with enough context for AI agents
// to navigate to the offending source.
type finding struct {
	File string
	Line int
	Note string
}

// walkImports parses every .go file under dir (recursively, skipping testdata
// and _underscore directories) and applies match to each imported path.
// Matching paths produce findings at the import line.
func walkImports(dir string, match func(path string) bool) ([]finding, error) {
	var out []finding
	err := walkGoFiles(dir, func(fset *token.FileSet, file *ast.File, path string) {
		for _, imp := range file.Imports {
			val, err := strconv.Unquote(imp.Path.Value)
			if err != nil {
				continue
			}
			if match(val) {
				out = append(out, finding{
					File: path,
					Line: fset.Position(imp.Pos()).Line,
					Note: val,
				})
			}
		}
	}, parser.ImportsOnly)
	return out, err
}

// walkSelectors finds expressions of the form pkg.sel anywhere in dir.
// Match is name-based, not type-based: false positives on unrelated symbols
// are accepted as the cost of avoiding go/types and a full module load.
func walkSelectors(dir, pkg, sel string) ([]finding, error) {
	var out []finding
	err := walkGoFiles(dir, func(fset *token.FileSet, file *ast.File, path string) {
		ast.Inspect(file, func(n ast.Node) bool {
			se, ok := n.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			ident, ok := se.X.(*ast.Ident)
			if !ok {
				return true
			}
			if ident.Name == pkg && se.Sel.Name == sel {
				out = append(out, finding{
					File: path,
					Line: fset.Position(se.Pos()).Line,
					Note: pkg + "." + sel,
				})
			}
			return true
		})
	}, 0)
	return out, err
}

// walkStringLiterals visits every basic STRING literal in dir and applies
// match. Tagged-comment-stripped values are passed in (interpreted strings
// are unquoted; raw strings are passed verbatim minus the backticks).
func walkStringLiterals(dir string, match func(string) bool) ([]finding, error) {
	var out []finding
	err := walkGoFiles(dir, func(fset *token.FileSet, file *ast.File, path string) {
		ast.Inspect(file, func(n ast.Node) bool {
			lit, ok := n.(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				return true
			}
			val := lit.Value
			if len(val) >= 2 && val[0] == '`' && val[len(val)-1] == '`' {
				val = val[1 : len(val)-1]
			} else if uq, err := strconv.Unquote(val); err == nil {
				val = uq
			}
			if match(val) {
				out = append(out, finding{
					File: path,
					Line: fset.Position(lit.Pos()).Line,
					Note: truncateLiteral(val),
				})
			}
			return true
		})
	}, 0)
	return out, err
}

// walkGoFiles is the shared traversal: parse each .go file under dir
// (excluding test files, testdata/, and dot/underscore directories), then
// invoke visit. Parse errors are skipped so that an in-progress file does
// not break the whole report.
func walkGoFiles(dir string, visit func(*token.FileSet, *ast.File, string), mode parser.Mode) error {
	fset := token.NewFileSet()
	return filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		name := d.Name()
		if d.IsDir() {
			if name == "testdata" || name == "vendor" || (len(name) > 1 && (name[0] == '.' || name[0] == '_')) {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			return nil
		}
		f, err := parser.ParseFile(fset, path, nil, mode)
		if err != nil {
			return nil
		}
		visit(fset, f, path)
		return nil
	})
}

func truncateLiteral(s string) string {
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\t", " ")
	if len(s) > 60 {
		return s[:60] + "..."
	}
	return s
}

// okCheck builds an OK warn-level Check for a layer rule.
func okCheck(id, msg string) Check {
	return Check{ID: id, OK: true, Severity: SeverityWarn, Message: msg, Rule: ruleAnchor}
}

// scanErrCheck reports a scanner failure (filesystem trouble) without
// blocking the report.
func scanErrCheck(id string, err error) Check {
	return Check{
		ID: id, OK: false, Severity: SeverityWarn, Rule: ruleAnchor,
		Message: fmt.Sprintf("scan failed: %v", err),
	}
}

// violationChecks turns a list of findings into one Check per finding,
// sorted by file/line for stable output.
func violationChecks(id, msgFmt string, findings []finding) []Check {
	sort.SliceStable(findings, func(i, j int) bool {
		if findings[i].File != findings[j].File {
			return findings[i].File < findings[j].File
		}
		return findings[i].Line < findings[j].Line
	})
	out := make([]Check, 0, len(findings))
	for _, f := range findings {
		out = append(out, Check{
			ID:       id,
			OK:       false,
			Severity: SeverityWarn,
			Message:  fmt.Sprintf(msgFmt, f.Note),
			File:     f.File,
			Line:     f.Line,
			Rule:     ruleAnchor,
		})
	}
	return out
}
