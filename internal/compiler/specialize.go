package compiler

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"go/types"

	"golang.org/x/tools/go/ast/astutil"
)

// specializeSelf copies a module func per single-parameter container Self (decision 377): behind Self, Each() is a dictionary call whose closure escapes.
func specializeSelf(fset *token.FileSet, f *ast.File, labels map[string]string) error {
	gens := map[string]*ast.FuncDecl{}
	elemCons := map[string]string{} // a single-parameter generic type's constraint: Array -> comparable
	for _, d := range f.Decls {
		switch d := d.(type) {
		case *ast.FuncDecl:
			if selfElem(d) != "" {
				gens[d.Name.Name] = d
			}
		case *ast.GenDecl:
			for _, s := range d.Specs {
				if ts, ok := s.(*ast.TypeSpec); ok && ts.TypeParams != nil && len(ts.TypeParams.List) == 1 && len(ts.TypeParams.List[0].Names) == 1 {
					elemCons[ts.Name.Name] = types.ExprString(ts.TypeParams.List[0].Type)
				}
			}
		}
	}
	type req struct{ gen, name, container string }
	var queue []req
	made := map[string]bool{}
	rewrite := func(root ast.Node) {
		astutil.Apply(root, nil, func(c *astutil.Cursor) bool {
			x, ok := c.Node().(*ast.IndexListExpr)
			if !ok {
				return true
			}
			fn, ok := x.X.(*ast.Ident)
			if !ok || gens[fn.Name] == nil {
				return true
			}
			g := gens[fn.Name]
			star, ok := x.Indices[0].(*ast.StarExpr)
			if !ok {
				return true
			}
			inst, ok := star.X.(*ast.IndexExpr)
			if !ok {
				return true
			}
			cont, ok := inst.X.(*ast.Ident)
			if !ok || types.ExprString(inst.Index) != types.ExprString(x.Indices[1]) {
				return true
			}
			cons, ok := elemCons[cont.Name]
			if !ok || cons != elemConstraint(g) { // the copy's E must satisfy the container's parameter
				return true
			}
			name := fn.Name + "_on" + cont.Name
			if !made[name] {
				made[name] = true
				queue = append(queue, req{fn.Name, name, cont.Name})
			}
			rest := x.Indices[1:]
			if len(rest) == 1 {
				c.Replace(&ast.IndexExpr{X: ast.NewIdent(name), Index: rest[0]})
			} else {
				c.Replace(&ast.IndexListExpr{X: ast.NewIdent(name), Indices: rest})
			}
			return true
		})
	}
	for _, d := range f.Decls {
		rewrite(d)
	}
	for i := 0; i < len(queue); i++ {
		r := queue[i]
		if gens[r.name] != nil { // a func of that name exists: leave the call generic
			continue
		}
		d, err := specCopy(fset, f, gens[r.gen], r.name, r.container, i)
		if err != nil {
			return err
		}
		rewrite(d.Decls[0])
		merge(f, d)
		if l, ok := labels[r.gen]; ok {
			labels[r.name] = l
		}
	}
	return nil
}

// selfElem is E of `M_X[Self M_Self[Self, E], E ...]`; the copy's first type parameter must be it.
func selfElem(d *ast.FuncDecl) string {
	if d.Recv != nil || d.Type.TypeParams == nil || len(d.Type.TypeParams.List) < 2 {
		return ""
	}
	first := d.Type.TypeParams.List[0]
	if len(first.Names) != 1 || first.Names[0].Name != "Self" {
		return ""
	}
	cons, ok := first.Type.(*ast.IndexListExpr)
	if !ok || len(cons.Indices) < 2 {
		return ""
	}
	if s, ok := cons.Indices[0].(*ast.Ident); !ok || s.Name != "Self" {
		return ""
	}
	e, ok := cons.Indices[1].(*ast.Ident)
	if !ok || d.Type.TypeParams.List[1].Names[0].Name != e.Name { // E must be the next parameter: the copy's first
		return ""
	}
	return e.Name
}

func elemConstraint(d *ast.FuncDecl) string { return types.ExprString(d.Type.TypeParams.List[1].Type) }

// specCopy reprints gen with its comments so the copy keeps the //line directives backtraces read.
func specCopy(fset *token.FileSet, f *ast.File, gen *ast.FuncDecl, name, container string, i int) (*ast.File, error) {
	var buf bytes.Buffer
	buf.WriteString("package main\n")
	err := printer.Fprint(&buf, fset, &printer.CommentedNode{Node: gen, Comments: f.Comments})
	if err != nil {
		return nil, fmt.Errorf("specialize %s: %w", name, err)
	}
	t, err := parser.ParseFile(fset, fmt.Sprintf("spec%d.go", i), buf.Bytes(), parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("specialize %s: %w", name, err)
	}
	d := t.Decls[0].(*ast.FuncDecl) //nolint:forcetypeassert // printed from a FuncDecl
	elem := selfElem(d)
	d.Name.Name = name
	d.Type.TypeParams.List = d.Type.TypeParams.List[1:]
	astutil.Apply(d, nil, func(c *astutil.Cursor) bool {
		if id, ok := c.Node().(*ast.Ident); ok && id.Name == "Self" {
			at := id.Pos() // positioned like Self, or the printer breaks the parameter list (`self *Array[E],)`)
			c.Replace(&ast.StarExpr{Star: at, X: &ast.IndexExpr{X: &ast.Ident{NamePos: at, Name: container}, Lbrack: at, Index: &ast.Ident{NamePos: at, Name: elem}, Rbrack: at}})
		}
		return true
	})
	return t, nil
}
