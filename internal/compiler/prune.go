package compiler

import (
	"go/ast"
	"go/token"
	"slices"
)

// stdMethodNames are called by the standard library through its interfaces (fmt.Stringer, error), never by a selector here.
var stdMethodNames = map[string]bool{
	"String": true, "Error": true, "Unwrap": true, "Is": true, "As": true,
	"Format": true, "GoString": true, "MarshalJSON": true, "UnmarshalJSON": true,
	"ServeHTTP": true, "Read": true, "Write": true, "Close": true,
	"Len": true, "Less": true, "Swap": true, "Lock": true, "Unlock": true,
}

type pruneMethod struct {
	recv string
	decl *ast.FuncDecl
}

// pruner marks what main reaches: a type when an identifier names it, a method when its receiver is kept and its name is selected or declared by a kept interface.
type pruner struct {
	byName      map[string][]ast.Node     // funcs, type specs and value specs by declared name
	constBlock  map[ast.Node]*ast.GenDecl // an iota block stays whole: its specs repeat the one before
	methods     []pruneMethod
	roots       []ast.Node
	kept        map[ast.Node]bool
	names       map[string]bool // identifiers used by kept code
	methodNames map[string]bool // selectors and interface methods used by kept code
	queue       []ast.Node
}

// pruneDecls drops what main cannot reach, like the linker, so a program compiles only the prelude it uses; names match unscoped, which only over-keeps.
func pruneDecls(f *ast.File) {
	p := &pruner{byName: map[string][]ast.Node{}, constBlock: map[ast.Node]*ast.GenDecl{}, kept: map[ast.Node]bool{}, names: map[string]bool{}, methodNames: map[string]bool{}}
	for n := range stdMethodNames {
		p.methodNames[n] = true
	}
	for _, d := range f.Decls {
		p.index(d)
	}
	for _, r := range p.roots {
		p.keep(r)
	}
	for len(p.queue) > 0 {
		for len(p.queue) > 0 {
			n := p.queue[len(p.queue)-1]
			p.queue = p.queue[:len(p.queue)-1]
			ast.Inspect(n, p.visit)
		}
		for _, m := range p.methods {
			if p.names[m.recv] && p.methodNames[m.decl.Name.Name] {
				p.keep(m.decl)
			}
		}
	}
	p.sweep(f)
}

func (p *pruner) index(d ast.Decl) {
	switch d := d.(type) {
	case *ast.FuncDecl:
		switch {
		case d.Recv != nil:
			p.methods = append(p.methods, pruneMethod{recvName(d.Recv.List[0].Type), d})
		case d.Name.Name == "main" || d.Name.Name == "init":
			p.roots = append(p.roots, d)
		default:
			p.byName[d.Name.Name] = append(p.byName[d.Name.Name], d)
		}
	case *ast.GenDecl:
		for _, s := range d.Specs {
			p.indexSpec(d, s)
		}
	}
}

func (p *pruner) indexSpec(d *ast.GenDecl, s ast.Spec) {
	switch s := s.(type) {
	case *ast.TypeSpec:
		p.byName[s.Name.Name] = append(p.byName[s.Name.Name], s)
	case *ast.ValueSpec:
		if d.Tok == token.CONST {
			p.constBlock[s] = d
		}
		for _, n := range s.Names {
			if n.Name == "_" {
				p.roots = append(p.roots, s)
			}
			p.byName[n.Name] = append(p.byName[n.Name], s)
		}
	}
}

func (p *pruner) keep(n ast.Node) {
	if p.kept[n] {
		return
	}
	p.kept[n] = true
	p.queue = append(p.queue, n)
	if d := p.constBlock[n]; d != nil {
		for _, s := range d.Specs {
			p.keep(s)
		}
	}
}

func (p *pruner) visit(x ast.Node) bool {
	switch x := x.(type) {
	case *ast.Ident:
		if !p.names[x.Name] {
			p.names[x.Name] = true
			for _, d := range p.byName[x.Name] {
				p.keep(d)
			}
		}
	case *ast.SelectorExpr:
		p.methodNames[x.Sel.Name] = true
	case *ast.InterfaceType:
		for _, m := range x.Methods.List {
			for _, mn := range m.Names {
				p.methodNames[mn.Name] = true
			}
		}
	}
	return true
}

// sweep deletes unkept declarations and the comments (//line directives included) no longer inside one.
func (p *pruner) sweep(f *ast.File) {
	f.Decls = slices.DeleteFunc(f.Decls, func(d ast.Decl) bool {
		switch d := d.(type) {
		case *ast.FuncDecl:
			return !p.kept[d]
		case *ast.GenDecl:
			if d.Tok == token.IMPORT {
				return false
			}
			d.Specs = slices.DeleteFunc(d.Specs, func(s ast.Spec) bool { return !p.kept[s] })
			return len(d.Specs) == 0
		}
		return false
	})
	spans := make([][2]token.Pos, 0, len(f.Decls))
	for _, d := range f.Decls {
		start := d.Pos()
		if fd, ok := d.(*ast.FuncDecl); ok && fd.Doc != nil {
			start = fd.Doc.Pos()
		}
		if gd, ok := d.(*ast.GenDecl); ok && gd.Doc != nil {
			start = gd.Doc.Pos()
		}
		spans = append(spans, [2]token.Pos{start, d.End()})
	}
	f.Comments = slices.DeleteFunc(f.Comments, func(c *ast.CommentGroup) bool {
		for _, s := range spans {
			if c.Pos() >= s[0] && c.End() <= s[1] {
				return false
			}
		}
		return c.Pos() > f.Name.End() // keep the header above the package clause
	})
}

// recvName is a method receiver's base type name: `*Foo[E]` → Foo.
func recvName(e ast.Expr) string {
	for {
		switch x := e.(type) {
		case *ast.StarExpr:
			e = x.X
		case *ast.IndexExpr:
			e = x.X
		case *ast.IndexListExpr:
			e = x.X
		case *ast.ParenExpr:
			e = x.X
		case *ast.Ident:
			return x.Name
		default:
			return ""
		}
	}
}
