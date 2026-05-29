// Package astwire provides AST-based source code manipulation for
// `ncgo add infra --wire`. It replaces string-based insertion and
// replacement with go/parser, go/ast, and go/format operations.
package astwire

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"strconv"
	"strings"
)

// WireOp describes a single wiring operation to apply to a Go source file.
// Zero-valued fields are ignored. Multiple non-zero fields in one WireOp
// are applied in order: AddImport, then Insert, then Replace.
type WireOp struct {
	// AddImport adds this import path if not already present.
	AddImport string

	// Marker is a comment prefix like "// ncgo:wire:logging:init".
	// If found in a comment group, statements/expressions are inserted after it.
	Marker string

	// Anchors are fallback source-text patterns. The first anchor found in
	// the source file is used when Marker is not found.
	Anchors []string

	// InsertSrc is the Go source code to insert. Parsed as statements
	// unless IsExpr is true, in which case it is parsed as an expression.
	InsertSrc string

	// IsExpr indicates InsertSrc should be parsed as a single expression.
	IsExpr bool

	// ExistsSentinel skips this op if already present in the source (idempotency).
	ExistsSentinel string

	// ReplacePkg, ReplaceName, NewPkg, NewName define a call replacement:
	// calls to pkg.Name() are replaced with newPkg.newName().
	ReplacePkg  string
	ReplaceName string
	NewPkg      string
	NewName     string
}

// WireFile applies wiring operations to Go source code.
// It parses the source, applies each operation, and formats the result.
// Operations that are already applied (ExistsSentinel found) are skipped.
func WireFile(src []byte, ops ...WireOp) ([]byte, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "source.go", src, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("astwire: parse: %w", err)
	}
	for _, op := range ops {
		if op.ExistsSentinel != "" && strings.Contains(string(src), op.ExistsSentinel) {
			continue
		}
		switch {
		case op.AddImport != "":
			addImport(f, op.AddImport)
		case op.ReplacePkg != "" || op.ReplaceName != "":
			replaceCallExpr(f, op.ReplacePkg, op.ReplaceName, op.NewPkg, op.NewName)
		case op.InsertSrc != "":
			if err := applyInsertOp(f, fset, src, op); err != nil {
				return nil, err
			}
		}
	}
	var buf bytes.Buffer
	if err := format.Node(&buf, fset, f); err != nil {
		return nil, fmt.Errorf("astwire: format: %w", err)
	}
	return buf.Bytes(), nil
}

func applyInsertOp(f *ast.File, fset *token.FileSet, src []byte, op WireOp) error {
	if op.IsExpr {
		expr, err := parseExpr(op.InsertSrc)
		if err != nil {
			return fmt.Errorf("astwire: parse insert expr: %w", err)
		}
		ok := false
		if op.Marker != "" {
			ok = insertExprAfterMarker(f, fset, op.Marker, expr)
		}
		if !ok && len(op.Anchors) > 0 {
			ok = insertExprAfterAnchors(f, fset, src, op.Anchors, expr)
		}
		if !ok {
			return fmt.Errorf("astwire: could not find insertion point for %q", op.InsertSrc)
		}
		return nil
	}
	stmts, err := parseStmts(op.InsertSrc)
	if err != nil {
		return fmt.Errorf("astwire: parse insert stmts: %w", err)
	}
	ok := false
	if op.Marker != "" {
		ok = insertStmtsAfterMarker(f, fset, op.Marker, stmts)
	}
	if !ok && len(op.Anchors) > 0 {
		ok = insertStmtsAfterAnchors(f, fset, src, op.Anchors, stmts)
	}
	if !ok {
		return fmt.Errorf("astwire: could not find insertion point for %q", op.InsertSrc)
	}
	return nil
}

// Parse parses Go source code into an AST file and file set.
func Parse(src []byte) (*ast.File, *token.FileSet, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "source.go", src, parser.ParseComments)
	if err != nil {
		return nil, nil, err
	}
	return f, fset, nil
}

// Format formats an AST file back to Go source code.
// fset must not be nil (use the FileSet returned by Parse).
func Format(fset *token.FileSet, f *ast.File) ([]byte, error) {
	if fset == nil {
		return nil, fmt.Errorf("astwire: Format requires a non-nil FileSet")
	}
	var buf bytes.Buffer
	if err := format.Node(&buf, fset, f); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// AddImport adds an import path to the file's import block.
// Returns true if a new import was added.
func AddImport(f *ast.File, importPath string) bool {
	return addImport(f, importPath)
}

func addImport(f *ast.File, importPath string) bool {
	quoted := strconv.Quote(importPath)
	// Check if already imported
	for _, decl := range f.Decls {
		gd, ok := decl.(*ast.GenDecl)
		if !ok || gd.Tok != token.IMPORT {
			continue
		}
		for _, spec := range gd.Specs {
			is, ok := spec.(*ast.ImportSpec)
			if !ok {
				continue
			}
			if is.Path.Value == quoted {
				return false
			}
		}
	}
	newSpec := &ast.ImportSpec{
		Path: &ast.BasicLit{
			Kind:  token.STRING,
			Value: quoted,
		},
	}
	// Find or create the import declaration
	for _, decl := range f.Decls {
		gd, ok := decl.(*ast.GenDecl)
		if ok && gd.Tok == token.IMPORT {
			gd.Specs = append(gd.Specs, newSpec)
			return true
		}
	}
	// No import block exists - create one at the top
	importDecl := &ast.GenDecl{
		Tok:    token.IMPORT,
		Lparen: f.Pos(), // non-zero forces multi-line "import (" form
		Rparen: f.Pos(),
		Specs:  []ast.Spec{newSpec},
	}
	// Insert after package declaration
	if len(f.Decls) > 0 {
		f.Decls = append([]ast.Decl{f.Decls[0], importDecl}, f.Decls[1:]...)
	} else {
		f.Decls = append([]ast.Decl{importDecl}, f.Decls...)
	}
	return true
}

// parseStmts parses Go source code into AST statement nodes.
// The source can include leading whitespace/tabs used for indentation.
func parseStmts(src string) ([]ast.Stmt, error) {
	wrapper := "package p\nfunc _() {\n" + src + "\n}\n"
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "", wrapper, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("parse: %w", err)
	}
	return f.Decls[0].(*ast.FuncDecl).Body.List, nil
}

// parseExpr parses Go source code into a single AST expression.
// The source can include leading whitespace/tabs and a trailing comma.
func parseExpr(src string) (ast.Expr, error) {
	trimmed := strings.TrimSpace(src)
	// Remove trailing comma (common in expression lists like endpoint.Chain(args))
	trimmed = strings.TrimSuffix(trimmed, ",")
	trimmed = strings.TrimSpace(trimmed)
	if trimmed == "" {
		return nil, fmt.Errorf("parse expr: empty source")
	}
	wrapper := "package p\nfunc _() {\n_ = " + trimmed + "\n}\n"
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "", wrapper, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("parse expr: %w", err)
	}
	assign := f.Decls[0].(*ast.FuncDecl).Body.List[0].(*ast.AssignStmt)
	return assign.Rhs[0], nil
}

// findCommentGroup returns the CommentGroup whose text contains the marker prefix.
// ast.Comment.Text field retains the // (or /* */) markers, so we compare
// the marker as-is after trimming whitespace.
func findCommentGroup(f *ast.File, marker string) *ast.CommentGroup {
	searchText := strings.TrimSpace(marker)
	for _, cg := range f.Comments {
		for _, c := range cg.List {
			if strings.HasPrefix(strings.TrimSpace(c.Text), searchText) {
				return cg
			}
		}
	}
	return nil
}

// findBlockStmtContaining finds the innermost BlockStmt that contains (or is
// right before) the given position, and the index at which to insert.
func findBlockStmtContaining(f ast.Node, pos token.Pos) (*ast.BlockStmt, int) {
	var target *ast.BlockStmt
	var insertIdx int
	ast.Inspect(f, func(n ast.Node) bool {
		block, ok := n.(*ast.BlockStmt)
		if !ok {
			return true
		}
		if block.Lbrace >= pos {
			return true // block starts after pos, skip
		}
		if block.Rbrace < pos {
			return true // block ends before pos, skip
		}
		// This block contains the position
		if target == nil || block.Lbrace > target.Lbrace {
			// Prefer innermost
			target = block
			insertIdx = len(block.List)
			for i, stmt := range block.List {
				if stmt.Pos() > pos {
					insertIdx = i
					break
				}
			}
		}
		return true
	})
	return target, insertIdx
}

// insertStmtsAfterMarker inserts stmts after a comment starting with marker.
func insertStmtsAfterMarker(f *ast.File, fset *token.FileSet, marker string, stmts []ast.Stmt) bool {
	cg := findCommentGroup(f, marker)
	if cg == nil {
		return false
	}
	block, idx := findBlockStmtContaining(f, cg.End())
	if block == nil {
		return false
	}
	// Fix positions: inserted statements came from a different FileSet.
	// Set all positions to the insertion point so format.Node places them correctly.
	for _, stmt := range stmts {
		fixPositions(stmt, cg.End())
	}
	newList := make([]ast.Stmt, 0, len(block.List)+len(stmts))
	newList = append(newList, block.List[:idx]...)
	newList = append(newList, stmts...)
	newList = append(newList, block.List[idx:]...)
	block.List = newList
	return true
}

// fixPositions walks an AST node and sets all token.Pos fields
// to the given position so that format.Node places the inserted nodes correctly.
func fixPositions(node ast.Node, pos token.Pos) {
	if node == nil {
		return
	}
	ast.Inspect(node, func(n ast.Node) bool {
		if n == nil {
			return false
		}
		switch x := n.(type) {
		// Expressions
		case *ast.Ident:
			x.NamePos = pos
		case *ast.BasicLit:
			x.ValuePos = pos
		case *ast.CompositeLit:
			x.Lbrace = pos
			x.Rbrace = pos
		case *ast.FuncLit:
			x.Type.Func = pos
		case *ast.SelectorExpr:
			// no position field
		case *ast.CallExpr:
			x.Lparen, x.Rparen = pos, pos
		case *ast.UnaryExpr:
			x.OpPos = pos
		case *ast.BinaryExpr:
			x.OpPos = pos
		case *ast.KeyValueExpr:
			x.Colon = pos
		case *ast.StarExpr:
			x.Star = pos

		// Statements
		case *ast.AssignStmt:
			x.TokPos = pos
		case *ast.ReturnStmt:
			x.Return = pos
		case *ast.IfStmt:
			x.If = pos
		case *ast.ForStmt:
			x.For = pos
		case *ast.RangeStmt:
			x.For = pos
		case *ast.GoStmt:
			x.Go = pos
		case *ast.DeferStmt:
			x.Defer = pos
		case *ast.ExprStmt:
			// no top-level position, children handled recursively
		case *ast.DeclStmt:
			// no position field

		// Misc
		case *ast.Field:
			// no single position field
		case *ast.FieldList:
			// no position field
		}
		return true
	})
}

// insertStmtsAfterAnchors inserts stmts after the first anchor found in src.
func insertStmtsAfterAnchors(f *ast.File, fset *token.FileSet, src []byte, anchors []string, stmts []ast.Stmt) bool {
	for _, anchor := range anchors {
		offset := findAnchorOffset(src, anchor)
		if offset < 0 {
			continue
		}
		pos := token.Pos(int(f.Pos()) + offset + len(anchor))
		block, idx := findBlockStmtContaining(f, pos)
		if block == nil {
			continue
		}
		for _, stmt := range stmts {
			fixPositions(stmt, pos)
		}
		newList := make([]ast.Stmt, 0, len(block.List)+len(stmts))
		newList = append(newList, block.List[:idx]...)
		newList = append(newList, stmts...)
		newList = append(newList, block.List[idx:]...)
		block.List = newList
		return true
	}
	return false
}

// findAnchorOffset returns the byte offset of anchor in src, or -1 if not found.
func findAnchorOffset(src []byte, anchor string) int {
	idx := 0
	for {
		i := idx + strings.Index(string(src[idx:]), anchor)
		if i < idx {
			return -1
		}
		// Only match at line start (to avoid matching substrings of other text)
		if i == 0 || src[i-1] == '\n' || src[i-1] == '\t' {
			return i
		}
		idx = i + 1
	}
}

// insertExprAfterMarker inserts expr after a marker comment at expression level.
func insertExprAfterMarker(f *ast.File, fset *token.FileSet, marker string, expr ast.Expr) bool {
	cg := findCommentGroup(f, marker)
	if cg == nil {
		return false
	}
	fixPositions(expr, cg.End())
	return insertExprAtPos(f, cg.End(), expr)
}

// insertExprAfterAnchors inserts expr after the first anchor found at expression level.
func insertExprAfterAnchors(f *ast.File, fset *token.FileSet, src []byte, anchors []string, expr ast.Expr) bool {
	for _, anchor := range anchors {
		offset := findAnchorOffset(src, anchor)
		if offset < 0 {
			continue
		}
		pos := token.Pos(int(f.Pos()) + offset + len(anchor))
		fixPositions(expr, pos)
		if insertExprAtPos(f, pos, expr) {
			return true
		}
	}
	return false
}

// insertExprAtPos inserts expr in the expression list (CallExpr.Args or
// CompositeLit.Elts) that contains (or is right before) pos.
func insertExprAtPos(f ast.Node, pos token.Pos, expr ast.Expr) bool {
	var targetList *[]ast.Expr
	var targetIdx int
	var targetDepth int

	var walk func(n ast.Node, depth int)
	walk = func(n ast.Node, depth int) {
		if n == nil {
			return
		}
		switch n := n.(type) {
		case *ast.CallExpr:
			if n.Lparen < pos && pos < n.Rparen {
				for i, arg := range n.Args {
					if arg.Pos() > pos {
						if targetList == nil || depth > targetDepth {
							targetList = &n.Args
							targetIdx = i
							targetDepth = depth
						}
						return
					}
				}
				if targetList == nil || depth > targetDepth {
					targetList = &n.Args
					targetIdx = len(n.Args)
					targetDepth = depth
				}
				// Recurse into args
				for _, arg := range n.Args {
					walk(arg, depth+1)
				}
				return // Don't walk sub-args again
			}
		case *ast.CompositeLit:
			if n.Lbrace < pos && pos < n.Rbrace {
				for i, elt := range n.Elts {
					if elt.Pos() > pos {
						if targetList == nil || depth > targetDepth {
							targetList = &n.Elts
							targetIdx = i
							targetDepth = depth
						}
						return
					}
				}
				if targetList == nil || depth > targetDepth {
					targetList = &n.Elts
					targetIdx = len(n.Elts)
					targetDepth = depth
				}
				for _, elt := range n.Elts {
					walk(elt, depth+1)
				}
				return
			}
		}
		// Walk children generically
		ast.Inspect(n, func(child ast.Node) bool {
			if child != n && child != nil {
				walk(child, depth+1)
			}
			return true
		})
	}
	walk(f, 0)

	if targetList == nil {
		return false
	}
	newList := make([]ast.Expr, 0, len(*targetList)+1)
	newList = append(newList, (*targetList)[:targetIdx]...)
	newList = append(newList, expr)
	newList = append(newList, (*targetList)[targetIdx:]...)
	*targetList = newList
	return true
}

// replaceCallExpr replaces calls like pkg.Name() with newPkg.newName().
func replaceCallExpr(f ast.Node, pkg, name, newPkg, newName string) {
	ast.Inspect(f, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		xIdent, ok := sel.X.(*ast.Ident)
		if !ok {
			return true
		}
		if xIdent.Name == pkg && sel.Sel.Name == name {
			xIdent.Name = newPkg
			sel.Sel.Name = newName
		}
		return true
	})
}
