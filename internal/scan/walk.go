package scan

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strings"
)

// walkGoFiles parses every non-test .go file under dir (recursively,
// skipping testdata/ and dot/underscore dirs) and invokes visit. Parse
// errors are skipped so an in-progress file does not break a scan.
func walkGoFiles(dir string, visit func(fset *token.FileSet, f *ast.File, path string)) error {
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
		f, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			return nil
		}
		visit(fset, f, path)
		return nil
	})
}
