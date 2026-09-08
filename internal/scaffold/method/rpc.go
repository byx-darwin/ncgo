package method

import (
	"bytes"
	"errors"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/printer"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/byx-darwin/ncgo/internal/manifest"
)

// RPCOptions configures AddRPC.
type RPCOptions struct {
	Root    string // project root containing .ncgo/manifest.yaml
	Service string // must match manifest.Service.Name
	RPC     string // method name; must already exist in the generated handler
}

// RPCResult describes the outcome of AddRPC.
type RPCResult struct {
	Path      string
	Service   string
	Method    string
	NextSteps []string
}

// importSpec is one import line to ensure is present in the target file.
type importSpec struct {
	Alias string // "" means no explicit alias (import as declared by its own package clause)
	Path  string
}

// methodSignature is the Go signature lifted from an already-generated
// handler file for a single RPC method.
type methodSignature struct {
	ParamsSrc string // rendered parameter list, e.g. "ctx context.Context, req *pb.PingReq"
	RespSrc   string // rendered response type, e.g. "*pb.PingResp"
	Imports   []importSpec
}

// AddRPC appends a method stub to an existing top-level usecase.go, copying
// its signature from the already-generated handler file. See
// docs/superpowers/specs/2026-09-08-add-rpc-method-usecase-append-design.md.
func AddRPC(opts RPCOptions) (*RPCResult, error) {
	if opts.Root == "" {
		return nil, errors.New("method: Root is required")
	}
	if opts.Service == "" {
		return nil, errors.New("method: Service is required")
	}
	if !methodRE.MatchString(opts.RPC) {
		return nil, fmt.Errorf("method: rpc %q must match %s", opts.RPC, methodRE)
	}
	root, err := filepath.Abs(opts.Root)
	if err != nil {
		return nil, fmt.Errorf("method: resolve root: %w", err)
	}
	m, err := manifest.Load(root)
	if err != nil {
		return nil, err
	}
	if m.Service.Name != opts.Service {
		return nil, fmt.Errorf("method: --service %q does not match manifest service %q", opts.Service, m.Service.Name)
	}

	var sig *methodSignature
	switch m.Service.Kind {
	case manifest.KindKitex:
		sig, err = findKitexHandlerSignature(root, m, opts.RPC)
	case manifest.KindHertz:
		sig, err = findHzHandlerSignature(root, opts.RPC)
	default:
		return nil, fmt.Errorf("method: unsupported service kind %q", m.Service.Kind)
	}
	if err != nil {
		return nil, err
	}

	usecasePath := filepath.Join(root, "internal", "usecase", strings.ToLower(m.Service.Name), "usecase.go")
	if err := appendUsecaseMethod(usecasePath, m.Service.Name, opts.RPC, *sig); err != nil {
		return nil, err
	}

	return &RPCResult{
		Path:    usecasePath,
		Service: m.Service.Name,
		Method:  opts.RPC,
		NextSteps: []string{
			"go build ./...",
			"replace the generated not-implemented body with domain logic",
			"ncgo ai sync --root .",
		},
	}, nil
}

func findKitexHandlerSignature(root string, m *manifest.Manifest, rpcName string) (*methodSignature, error) {
	handlerPath := filepath.Join(root, "internal", "handler", strings.ToLower(m.Service.Name), "handler.go")
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, handlerPath, nil, 0)
	if err != nil {
		return nil, fmt.Errorf("method: read kitex handler %s (run `make update` first?): %w", handlerPath, err)
	}
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Recv == nil || fn.Name.Name != rpcName {
			continue
		}
		return buildSignature(fset, file, fn.Type.Params, fn.Type.Results)
	}
	return nil, fmt.Errorf("method: rpc %q not found in %s; run `make update` after adding it to the IDL", rpcName, handlerPath)
}

func findHzHandlerSignature(root string, rpcName string) (*methodSignature, error) {
	handlerDir := filepath.Join(root, "internal", "handler")
	var sig *methodSignature
	walkErr := filepath.WalkDir(handlerDir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(p, ".go") {
			return nil
		}
		fset := token.NewFileSet()
		file, ferr := parser.ParseFile(fset, p, nil, 0)
		if ferr != nil {
			return nil // skip unparsable files rather than fail the whole walk
		}
		for _, decl := range file.Decls {
			gd, ok := decl.(*ast.GenDecl)
			if !ok || gd.Tok != token.TYPE {
				continue
			}
			for _, spec := range gd.Specs {
				ts, ok := spec.(*ast.TypeSpec)
				if !ok || ts.Name.Name != "useCase" {
					continue
				}
				it, ok := ts.Type.(*ast.InterfaceType)
				if !ok {
					continue
				}
				for _, meth := range it.Methods.List {
					ft, ok := meth.Type.(*ast.FuncType)
					if !ok || len(meth.Names) == 0 || meth.Names[0].Name != rpcName {
						continue
					}
					built, berr := buildSignature(fset, file, ft.Params, ft.Results)
					if berr != nil {
						return berr
					}
					sig = built
					return filepath.SkipAll
				}
			}
		}
		return nil
	})
	if walkErr != nil {
		return nil, walkErr
	}
	if sig == nil {
		return nil, fmt.Errorf("method: rpc %q not found in any generated handler under %s; run `hz update` after adding it to the IDL", rpcName, handlerDir)
	}
	return sig, nil
}

func buildSignature(fset *token.FileSet, file *ast.File, params, results *ast.FieldList) (*methodSignature, error) {
	paramsSrc, err := renderParams(fset, params)
	if err != nil {
		return nil, fmt.Errorf("method: render params: %w", err)
	}
	respSrc, err := responseType(fset, results)
	if err != nil {
		return nil, fmt.Errorf("method: render response type: %w", err)
	}
	used := collectPackageIdents(params, results)
	var imports []importSpec
	for _, imp := range file.Imports {
		name := importLocalName(imp)
		if !used[name] {
			continue
		}
		alias := ""
		if imp.Name != nil {
			alias = imp.Name.Name
		}
		imports = append(imports, importSpec{Alias: alias, Path: strings.Trim(imp.Path.Value, `"`)})
	}
	return &methodSignature{ParamsSrc: paramsSrc, RespSrc: respSrc, Imports: imports}, nil
}

func printExpr(fset *token.FileSet, n ast.Expr) (string, error) {
	var buf bytes.Buffer
	if err := printer.Fprint(&buf, fset, n); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func renderParams(fset *token.FileSet, fl *ast.FieldList) (string, error) {
	if fl == nil {
		return "", nil
	}
	var parts []string
	for _, f := range fl.List {
		typ, err := printExpr(fset, f.Type)
		if err != nil {
			return "", err
		}
		if len(f.Names) == 0 {
			parts = append(parts, typ)
			continue
		}
		for _, n := range f.Names {
			parts = append(parts, n.Name+" "+typ)
		}
	}
	return strings.Join(parts, ", "), nil
}

// responseType returns the source text of the response type, requiring the
// method to return exactly (response, error) — the shape both kitex's and
// hz's generated handlers always use.
func responseType(fset *token.FileSet, fl *ast.FieldList) (string, error) {
	if fl == nil {
		return "", errors.New("no results")
	}
	count := 0
	for _, f := range fl.List {
		if len(f.Names) == 0 {
			count++
		} else {
			count += len(f.Names)
		}
	}
	if count != 2 {
		return "", fmt.Errorf("expected exactly 2 results (response, error), found %d", count)
	}
	return printExpr(fset, fl.List[0].Type)
}

func collectPackageIdents(nodes ...ast.Node) map[string]bool {
	idents := map[string]bool{}
	for _, n := range nodes {
		if n == nil {
			continue
		}
		ast.Inspect(n, func(x ast.Node) bool {
			if sel, ok := x.(*ast.SelectorExpr); ok {
				if id, ok := sel.X.(*ast.Ident); ok {
					idents[id.Name] = true
				}
			}
			return true
		})
	}
	return idents
}

func importLocalName(spec *ast.ImportSpec) string {
	if spec.Name != nil {
		return spec.Name.Name
	}
	p := strings.Trim(spec.Path.Value, `"`)
	if i := strings.LastIndex(p, "/"); i >= 0 {
		return p[i+1:]
	}
	return p
}

func appendUsecaseMethod(path, service, methodName string, sig methodSignature) error {
	body, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("method: read %s: %w", path, err)
	}
	src := string(body)
	if strings.Contains(src, "UseCase) "+methodName+"(") {
		return fmt.Errorf("method: %s already exists in %s", methodName, path)
	}
	required := append([]importSpec{{Alias: "goerror", Path: "github.com/byx-darwin/go-tools/go-common/error"}}, sig.Imports...)
	updated := mergeImports(src, required)
	updated = strings.TrimRight(updated, "\n") + "\n" + renderRPCMethod(service, methodName, sig)
	formatted, err := format.Source([]byte(updated))
	if err != nil {
		return fmt.Errorf("method: format %s: %w", path, err)
	}
	return os.WriteFile(path, formatted, 0o644)
}

func renderRPCMethod(service, methodName string, sig methodSignature) string {
	return fmt.Sprintf(`
// %s implements the %s business rule.
func (uc *UseCase) %s(%s) (resp %s, err error) {
	return nil, goerror.
		In(%q).
		Code(10010).
		Public("not_implemented").
		Errorf("%s: not implemented")
}
`, methodName, methodName, methodName, sig.ParamsSrc, sig.RespSrc, strings.ToLower(service)+".usecase", methodName)
}

// mergeImports inserts any required import not already present in src. It
// uses parser.ImportsOnly for cheap existing-import detection, then a plain
// text insertion (final go/format.Source pass fixes styling) — no
// golang.org/x/tools/go/ast/astutil dependency needed for this one-shot,
// append-only edit.
func mergeImports(src string, required []importSpec) string {
	existing := existingImportPaths(src)
	var toAdd []importSpec
	for _, imp := range required {
		if !existing[imp.Path] {
			toAdd = append(toAdd, imp)
			existing[imp.Path] = true
		}
	}
	if len(toAdd) == 0 {
		return src
	}
	var lines []string
	for _, imp := range toAdd {
		if imp.Alias != "" {
			lines = append(lines, fmt.Sprintf("\t%s %q", imp.Alias, imp.Path))
		} else {
			lines = append(lines, fmt.Sprintf("\t%q", imp.Path))
		}
	}
	block := strings.Join(lines, "\n")
	if idx := strings.Index(src, "import (\n"); idx >= 0 {
		insertAt := idx + len("import (\n")
		return src[:insertAt] + block + "\n" + src[insertAt:]
	}
	if idx := strings.Index(src, "\n"); idx >= 0 {
		pkgLineEnd := idx + 1
		return src[:pkgLineEnd] + "\nimport (\n" + block + "\n)\n" + src[pkgLineEnd:]
	}
	return src
}

func existingImportPaths(src string) map[string]bool {
	fset := token.NewFileSet()
	existing := map[string]bool{}
	file, err := parser.ParseFile(fset, "", src, parser.ImportsOnly)
	if err != nil {
		return existing
	}
	for _, imp := range file.Imports {
		existing[strings.Trim(imp.Path.Value, `"`)] = true
	}
	return existing
}
