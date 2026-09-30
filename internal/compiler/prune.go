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
	fwd  bool // a generic type's forwarder to its free func: kept only when a kept interface declares it (decision 86)
}

// pruner marks what main reaches: a type when an identifier names it, a method when its receiver
// is kept and its name is selected. A method whose name a kept interface declares but no kept code
// selects is kept as a stub (its body a panic, not visited): Go calls methods only through
// selectors, so nothing can run it, but its type still has to satisfy the interface.
// A type switch's case of one concrete type names it weakly: a case whose type nothing else keeps can never match
// (no value of such a type can exist), so it is dropped, body and all, rather than keeping them.
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
	pending     map[*ast.CaseClause][]string // type-switch cases waiting for their declared type names to be kept
	queue       []ast.Node
}

// pruneDecls drops what main cannot reach, like the linker, so a program compiles only the prelude it uses; names match unscoped, which only over-keeps.
func pruneDecls(f *ast.File) {
	p := &pruner{byName: map[string][]ast.Node{}, constBlock: map[ast.Node]*ast.GenDecl{}, kept: map[ast.Node]bool{}, names: map[string]bool{}, methodNames: map[string]bool{}, declared: map[string]bool{}, stubs: map[*ast.FuncDecl]bool{}, pending: map[*ast.CaseClause][]string{}}
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
		for cc, names := range p.pending {
			if !slices.ContainsFunc(names, func(n string) bool { return !p.names[n] }) {
				delete(p.pending, cc)
				p.queue = append(p.queue, cc)
			}
		}
		for _, m := range p.methods {
			if !p.names[m.recv] {
				continue
			}
			name := m.decl.Name.Name
			// a selector matches unscoped, and Ruby's names (size, first, ...) are on every class: a generic
			// type's forwarder taken on such a match would instantiate its free func for every type argument
			selected := p.methodNames[name] && (!m.fwd || p.declared[name])
			switch {
			case selected && p.stubs[m.decl]:
				delete(p.stubs, m.decl)
				p.queue = append(p.queue, m.decl.Body)
			case selected:
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
			p.methods = append(p.methods, pruneMethod{recvName(d.Recv.List[0].Type), d, isGenericForwarder(d)})
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
	case *ast.TypeSwitchStmt:
		if x.Init != nil {
			ast.Inspect(x.Init, p.visit)
		}
		ast.Inspect(x.Assign, p.visit)
		for _, st := range x.Body.List {
			cc := st.(*ast.CaseClause)
			// only a single concrete type: narrowing `case A, B:` would change its variable's type,
			// and values implementing an interface exist without anything naming the interface
			if names := p.caseTypeNames(cc); len(cc.List) == 1 && !p.isInterface(cc.List[0]) && len(names) > 0 {
				p.pending[cc] = names // visited once every name is kept
				continue
			}
			ast.Inspect(cc, p.visit)
		}
		return false
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

// isInterface reports whether type expression e is an interface: a literal, or a declared interface type.
func (p *pruner) isInterface(e ast.Expr) bool {
	switch x := e.(type) {
	case *ast.InterfaceType:
		return true
	case *ast.IndexExpr:
		return p.isInterface(x.X)
	case *ast.IndexListExpr:
		return p.isInterface(x.X)
	case *ast.Ident:
		for _, d := range p.byName[x.Name] {
			if ts, ok := d.(*ast.TypeSpec); ok {
				if _, iface := ts.Type.(*ast.InterfaceType); iface {
					return true
				}
			}
		}
	}
	return false
}

// dropDeadCases removes the cases whose type nothing kept. The closing brace
// moves up to the last case left (else the printer keeps the gap as blank
// lines), and a switch left with no case drops its variable (else Go reports
// it unused).
func (p *pruner) dropDeadCases(ts *ast.TypeSwitchStmt) {
	n := len(ts.Body.List)
	ts.Body.List = slices.DeleteFunc(ts.Body.List, func(st ast.Stmt) bool {
		_, dead := p.pending[st.(*ast.CaseClause)]
		return dead
	})
	if len(ts.Body.List) == n {
		return
	}
	if len(ts.Body.List) == 0 {
		ts.Body.Rbrace = ts.Body.Lbrace + 1
		if as, ok := ts.Assign.(*ast.AssignStmt); ok {
			ts.Assign = &ast.ExprStmt{X: as.Rhs[0]}
		}
		return
	}
	ts.Body.Rbrace = ts.Body.List[len(ts.Body.List)-1].End()
}

// caseTypeNames lists the declared names a type-switch case's types use that kept code has not named yet.
func (p *pruner) caseTypeNames(cc *ast.CaseClause) []string {
	var out []string
	for _, e := range cc.List {
		ast.Inspect(e, func(n ast.Node) bool {
			if id, ok := n.(*ast.Ident); ok && len(p.byName[id.Name]) > 0 && !p.names[id.Name] && !slices.Contains(out, id.Name) {
				out = append(out, id.Name)
			}
			return true
		})
	}
	return out
}

// sweep deletes unkept declarations and the comments (//line directives included) no longer inside one; a stub's body becomes a panic.
func (p *pruner) sweep(f *ast.File) {
	for _, d := range f.Decls {
		ast.Inspect(d, func(n ast.Node) bool {
			if ts, ok := n.(*ast.TypeSwitchStmt); ok {
				p.dropDeadCases(ts)
			}
			return true
		})
	}
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

// isGenericForwarder reports whether d is a method on a generic type whose whole body calls the
// free func of the same name (`func (self *Array[E]) Size() Integer { return Array_Size[E](self) }`).
func isGenericForwarder(d *ast.FuncDecl) bool {
	recv := d.Recv.List[0].Type
	if s, ok := recv.(*ast.StarExpr); ok {
		recv = s.X
	}
	switch recv.(type) {
	case *ast.IndexExpr, *ast.IndexListExpr:
	default:
		return false
	}
	if d.Body == nil || len(d.Body.List) != 1 {
		return false
	}
	var call ast.Expr
	switch st := d.Body.List[0].(type) {
	case *ast.ReturnStmt:
		if len(st.Results) != 1 {
			return false
		}
		call = st.Results[0]
	case *ast.ExprStmt:
		call = st.X
	default:
		return false
	}
	ce, ok := call.(*ast.CallExpr)
	if !ok {
		return false
	}
	fun := ce.Fun
	switch x := fun.(type) {
	case *ast.IndexExpr:
		fun = x.X
	case *ast.IndexListExpr:
		fun = x.X
	}
	id, ok := fun.(*ast.Ident)
	return ok && id.Name == recvName(recv)+"_"+d.Name.Name
}
