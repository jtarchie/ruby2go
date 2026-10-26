package compiler

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/scanner"
	"go/token"
	"go/types"
)

// specializer is specializeSelf's state: the generic module funcs, one-parameter types' constraints, copies asked for.
type specializer struct {
	gens     map[string]*ast.FuncDecl
	elemCons map[string]string // a single-parameter generic type's constraint: Array -> comparable
	made     map[string]bool
	queue    []specReq
}

type specReq struct{ gen, name, container string }

// specializeSelf copies a module func per single-parameter container Self (decision 377): behind Self, Each() is a dictionary call whose closure escapes.
func specializeSelf(fset *token.FileSet, f *ast.File, labels map[string]string) error {
	s := &specializer{gens: map[string]*ast.FuncDecl{}, elemCons: map[string]string{}, made: map[string]bool{}}
	for _, d := range f.Decls {
		s.index(d)
	}
	for _, d := range f.Decls {
		s.rewrite(d)
	}
	for i := 0; len(s.queue) > 0; i++ { // copies are rewritten too, and may ask for more
		r := s.queue[0]
		s.queue = s.queue[1:]
		if s.gens[r.name] != nil { // a func of that name exists: leave the call generic
			continue
		}
		d, err := specCopy(fset, f, s.gens[r.gen], r.name, r.container, i)
		if err != nil {
			return err
		}
		s.rewrite(d.Decls[0])
		merge(f, d)
		if l, ok := labels[r.gen]; ok {
			labels[r.name] = l
		}
	}
	return nil
}

func (s *specializer) index(d ast.Decl) {
	switch d := d.(type) {
	case *ast.FuncDecl:
		if selfElem(d) != "" {
			s.gens[d.Name.Name] = d
		}
	case *ast.GenDecl:
		for _, sp := range d.Specs {
			if ts, ok := sp.(*ast.TypeSpec); ok && ts.TypeParams != nil && len(ts.TypeParams.List) == 1 && len(ts.TypeParams.List[0].Names) == 1 {
				s.elemCons[ts.Name.Name] = types.ExprString(ts.TypeParams.List[0].Type)
			}
		}
	}
}

// rewrite turns each `M_X[*G[T], T, rest...]` under root into `M_X_onG[T, rest...]`, in place.
func (s *specializer) rewrite(root ast.Node) {
	ast.Inspect(root, func(n ast.Node) bool {
		x, ok := n.(*ast.IndexListExpr)
		if !ok {
			return true
		}
		fn, container, ok := s.target(x)
		if !ok {
			return true
		}
		name := fn + "_on" + container
		if !s.made[name] {
			s.made[name] = true
			s.queue = append(s.queue, specReq{fn, name, container})
		}
		x.X = &ast.Ident{NamePos: x.X.Pos(), Name: name}
		x.Indices = x.Indices[1:] // one index left prints as F[T], as an IndexExpr would
		return true
	})
}

// target is the module func and container of an instantiation the copy applies to.
func (s *specializer) target(x *ast.IndexListExpr) (fn, container string, ok bool) {
	id, ok := x.X.(*ast.Ident)
	if !ok || s.gens[id.Name] == nil || len(x.Indices) < 2 {
		return "", "", false
	}
	star, ok := x.Indices[0].(*ast.StarExpr)
	if !ok {
		return "", "", false
	}
	inst, ok := star.X.(*ast.IndexExpr)
	if !ok {
		return "", "", false
	}
	cont, ok := inst.X.(*ast.Ident)
	if !ok || types.ExprString(inst.Index) != types.ExprString(x.Indices[1]) {
		return "", "", false
	}
	if cons, ok := s.elemCons[cont.Name]; !ok || cons != elemConstraint(s.gens[id.Name]) { // the copy's E must satisfy the container's parameter
		return "", "", false
	}
	return id.Name, cont.Name, true
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
	if !ok || d.Type.TypeParams.List[1].Names[0].Name != e.Name {
		return ""
	}
	return e.Name
}

func elemConstraint(d *ast.FuncDecl) string { return types.ExprString(d.Type.TypeParams.List[1].Type) }

// specCopy reprints gen with its comments, so the copy keeps the //line directives backtraces read; Self is replaced as a token.
func specCopy(fset *token.FileSet, f *ast.File, gen *ast.FuncDecl, name, container string, i int) (*ast.File, error) {
	src, err := printDecl(fset, gen, f.Comments)
	if err != nil {
		return nil, fmt.Errorf("specialize %s: %w", name, err)
	}
	t, err := parser.ParseFile(fset, fmt.Sprintf("spec%d.go", i), src, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("specialize %s: %w", name, err)
	}
	d := t.Decls[0].(*ast.FuncDecl) //nolint:forcetypeassert // printed from a FuncDecl
	elem := selfElem(d)
	d.Name.Name = name
	d.Type.TypeParams.List = d.Type.TypeParams.List[1:]
	src, err = printDecl(fset, d, t.Comments)
	if err != nil {
		return nil, fmt.Errorf("specialize %s: %w", name, err)
	}
	t, err = parser.ParseFile(fset, fmt.Sprintf("spec%d.go", i), replaceIdent(src, "Self", "*"+container+"["+elem+"]"), parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("specialize %s: %w", name, err)
	}
	return t, nil
}

func printDecl(fset *token.FileSet, d ast.Node, comments []*ast.CommentGroup) ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteString("package main\n")
	err := printer.Fprint(&buf, fset, &printer.CommentedNode{Node: d, Comments: comments})
	return buf.Bytes(), err
}

// replaceIdent writes repl for every identifier token old in src; strings and comments are left alone.
func replaceIdent(src []byte, old, repl string) []byte {
	var sc scanner.Scanner
	fs := token.NewFileSet()
	sc.Init(fs.AddFile("", -1, len(src)), src, nil, 0)
	var out bytes.Buffer
	last := 0
	for {
		pos, tok, lit := sc.Scan()
		if tok == token.EOF {
			break
		}
		if tok == token.IDENT && lit == old {
			off := fs.Position(pos).Offset
			out.Write(src[last:off])
			out.WriteString(repl)
			last = off + len(old)
		}
	}
	out.Write(src[last:])
	return out.Bytes()
}
