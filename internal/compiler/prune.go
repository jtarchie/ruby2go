package compiler

import (
	"go/ast"
	"go/token"
	"slices"
	"strings"
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

// pruner marks what main reaches: a type when an identifier names it, a method when its receiver
// is kept and its name is selected. A method whose name a kept interface declares but no kept code
// selects is kept as a stub (its body a panic, not visited): Go calls methods only through
// selectors, so nothing can run it, but its type still has to satisfy the interface.
type pruner struct {
	byName      map[string][]ast.Node     // funcs, type specs and value specs by declared name
	constBlock  map[ast.Node]*ast.GenDecl // an iota block stays whole: its specs repeat the one before
	methods     []pruneMethod
	roots       []ast.Node
	kept        map[ast.Node]bool
	names       map[string]bool // identifiers used by kept code
	methodNames map[string]bool // selectors used by kept code, and methods the standard library calls
	declared    map[string]bool // methods declared by kept interfaces
	stubs       map[*ast.FuncDecl]bool
	queue       []ast.Node
}

// pruneDecls drops what main cannot reach, like the linker, so a program compiles only the prelude it uses; names match unscoped, which only over-keeps.
func pruneDecls(f *ast.File) {
	p := &pruner{byName: map[string][]ast.Node{}, constBlock: map[ast.Node]*ast.GenDecl{}, kept: map[ast.Node]bool{}, names: map[string]bool{}, methodNames: map[string]bool{}, declared: map[string]bool{}, stubs: map[*ast.FuncDecl]bool{}}
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
			if !p.names[m.recv] {
				continue
			}
			switch name := m.decl.Name.Name; {
			case p.methodNames[name] && p.stubs[m.decl]:
				delete(p.stubs, m.decl)
				p.queue = append(p.queue, m.decl.Body)
			case p.methodNames[name]:
				p.keep(m.decl)
			case p.declared[name] && !p.kept[m.decl]:
				p.kept[m.decl] = true
				p.stubs[m.decl] = true
				p.queue = append(p.queue, m.decl.Recv, m.decl.Type)
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
				p.declared[mn.Name] = true
			}
		}
	}
	return true
}

// sweep deletes unkept declarations and the comments (//line directives included) no longer inside one; a stub's body becomes a panic.
func (p *pruner) sweep(f *ast.File) {
	var dropped [][2]token.Pos // deleted declarations: their comments go
	f.Decls = slices.DeleteFunc(f.Decls, func(d ast.Decl) bool {
		span := [2]token.Pos{declStart(d), d.End()} // before the specs go: a GenDecl's End reads its last spec
		drop := false
		switch d := d.(type) {
		case *ast.FuncDecl:
			drop = !p.kept[d]
		case *ast.GenDecl:
			if d.Tok != token.IMPORT {
				d.Specs = slices.DeleteFunc(d.Specs, func(s ast.Spec) bool {
					if !p.kept[s] {
						dropped = append(dropped, [2]token.Pos{s.Pos(), s.End()})
					}
					return !p.kept[s]
				})
				drop = len(d.Specs) == 0
			}
		}
		if drop {
			dropped = append(dropped, span)
		}
		return drop
	})
	for d := range p.stubs {
		if d.Body != nil { // `{ panic(...) }` on the brace's line; the old body's //line comments now sit between declarations
			at := d.Body.Lbrace + 1
			d.Body = &ast.BlockStmt{Lbrace: d.Body.Lbrace, Rbrace: at + 1, List: []ast.Stmt{&ast.ExprStmt{X: &ast.CallExpr{
				Fun: &ast.Ident{NamePos: at, Name: "panic"}, Lparen: at, Rparen: at,
				Args: []ast.Expr{&ast.BasicLit{ValuePos: at, Kind: token.STRING, Value: `"rb2go: pruned"`}},
			}}}}
		}
	}
	spans := make([][2]token.Pos, 0, len(f.Decls))
	for _, d := range f.Decls {
		spans = append(spans, [2]token.Pos{declStart(d), d.End()})
	}
	within := func(c *ast.CommentGroup, spans [][2]token.Pos) bool {
		for _, s := range spans {
			if c.Pos() >= s[0] && c.End() <= s[1] {
				return true
			}
		}
		return false
	}
	f.Comments = slices.DeleteFunc(f.Comments, func(c *ast.CommentGroup) bool {
		switch {
		case within(c, spans), c.Pos() < f.Name.End(): // inside a kept declaration, or the header above the package clause
			return false
		case within(c, dropped):
			return true
		}
		// a //line directive between declarations maps the code after it, which may be kept
		return !strings.HasPrefix(c.List[0].Text, "//line ")
	})
}

func declStart(d ast.Decl) token.Pos {
	if fd, ok := d.(*ast.FuncDecl); ok && fd.Doc != nil {
		return fd.Doc.Pos()
	}
	if gd, ok := d.(*ast.GenDecl); ok && gd.Doc != nil {
		return gd.Doc.Pos()
	}
	return d.Pos()
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
