package compiler

import (
	"fmt"
	"go/ast"
	"go/token"
)

// Decision 402: Go inlines the prelude's count/sum/Array.new closure only under its budget (80), so a bigger block was an indirect call per element; its body moves into an index loop at the call statement instead.

// blockLoopHelpers maps each helper to a call its prelude body makes, so a program's own definition is left alone.
var blockLoopHelpers = map[string]string{
	"Enumerable___CountBlock_onArray": "Each",
	"Enumerable___SumBlock_onArray":   "addFloat",
	"Array_Meta___New1":               "Array_Meta___Zeros",
}

type blockInliner struct {
	own    map[string]bool
	types  map[string]bool // the file's type names: a call of one is a conversion
	n      int
	pos    token.Pos      // the printer places //line comments by position, so new nodes sit where their code came from
	labels map[string]int // per FuncDecl, closures included: a moved closure's label must not meet another
}

// inlineBlockLoops runs after specializeSelf, which names the `_onArray` helpers.
func inlineBlockLoops(f *ast.File) {
	b := &blockInliner{own: map[string]bool{}, types: map[string]bool{}}
	for _, d := range f.Decls {
		switch d := d.(type) {
		case *ast.FuncDecl:
			if marker, ok := blockLoopHelpers[d.Name.Name]; ok && d.Recv == nil && d.Body != nil && callsName(d.Body, marker) {
				b.own[d.Name.Name] = true
			}
		case *ast.GenDecl:
			for _, sp := range d.Specs {
				if ts, ok := sp.(*ast.TypeSpec); ok {
					b.types[ts.Name.Name] = true
				}
			}
		}
	}
	if len(b.own) == 0 {
		return
	}
	for _, d := range f.Decls {
		if fd, ok := d.(*ast.FuncDecl); ok && fd.Body != nil && !b.own[fd.Name.Name] {
			b.rewriteFunc(fd.Body)
		}
	}
}

// rewriteFunc goes innermost first: a nested call's loop is in place before its enclosing closure's body moves.
func (b *blockInliner) rewriteFunc(body *ast.BlockStmt) {
	b.labels = map[string]int{}
	var lists []*[]ast.Stmt
	ast.Inspect(body, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.BlockStmt:
			lists = append(lists, &x.List)
		case *ast.CaseClause:
			lists = append(lists, &x.Body)
		case *ast.CommClause:
			lists = append(lists, &x.Body)
		case *ast.LabeledStmt:
			b.labels[x.Label.Name]++
		}
		return true
	})
	for i := len(lists) - 1; i >= 0; i-- {
		var out []ast.Stmt
		for _, s := range *lists[i] {
			if r := b.rewriteStmt(s); r != nil {
				out = append(out, r...)
				continue
			}
			out = append(out, s)
		}
		*lists[i] = out
	}
}

// rewriteStmt hoists each call it can into loops before s, in Go's evaluation order, replacing it in s with the loop's result.
func (b *blockInliner) rewriteStmt(s ast.Stmt) []ast.Stmt {
	slots := stmtSlots(s)
	var pre []ast.Stmt
	for {
		var loop []ast.Stmt
		for _, p := range slots {
			var stop bool
			if loop, stop = b.scan(p); loop != nil || stop {
				break
			}
		}
		if loop == nil {
			break
		}
		pre = append(pre, loop...)
	}
	if pre == nil {
		return nil
	}
	if es, ok := s.(*ast.ExprStmt); ok { // the call was the whole statement: Go rejects a bare value
		if id, ok := es.X.(*ast.Ident); ok {
			s = &ast.AssignStmt{Lhs: []ast.Expr{&ast.Ident{NamePos: id.NamePos, Name: "_"}}, TokPos: id.NamePos, Tok: token.ASSIGN, Rhs: []ast.Expr{id}}
		}
	}
	return append(pre, s)
}

// stmtSlots are the expressions s evaluates first, before anything else it does; an assignment's left side must be plain names, whose operands Go would otherwise evaluate in the same pass.
func stmtSlots(s ast.Stmt) []*ast.Expr {
	var slots []*ast.Expr
	switch s := s.(type) {
	case *ast.ExprStmt:
		slots = append(slots, &s.X)
	case *ast.ReturnStmt:
		for i := range s.Results {
			slots = append(slots, &s.Results[i])
		}
	case *ast.AssignStmt:
		for _, l := range s.Lhs {
			if _, ok := l.(*ast.Ident); !ok {
				return nil
			}
		}
		for i := range s.Rhs {
			slots = append(slots, &s.Rhs[i])
		}
	case *ast.IfStmt:
		if s.Init == nil {
			slots = append(slots, &s.Cond)
		}
	case *ast.SwitchStmt:
		if s.Init == nil && s.Tag != nil {
			slots = append(slots, &s.Tag)
		}
	case *ast.RangeStmt:
		slots = append(slots, &s.X)
	}
	return slots
}

// scan walks *p in Go's evaluation order to the first lowerable call, which it replaces by the loop's result; stop is anything before it that has an effect (a call, a possible panic, a short circuit), which the hoisted loop must not overtake.
func (b *blockInliner) scan(p *ast.Expr) (loop []ast.Stmt, stop bool) {
	switch x := (*p).(type) {
	case *ast.Ident, *ast.BasicLit, *ast.FuncLit:
		return nil, false
	case *ast.ParenExpr:
		return b.scan(&x.X)
	case *ast.SelectorExpr:
		return b.scan(&x.X)
	case *ast.KeyValueExpr:
		return b.scan(&x.Value)
	case *ast.UnaryExpr:
		if x.Op == token.ARROW {
			return nil, true
		}
		return b.scan(&x.X)
	case *ast.CompositeLit:
		return b.scanList(x.Elts, false)
	case *ast.StarExpr:
		return b.scanThenStop(&x.X)
	case *ast.TypeAssertExpr:
		return b.scanThenStop(&x.X)
	case *ast.IndexExpr:
		if loop, stop := b.scan(&x.X); loop != nil || stop {
			return loop, stop
		}
		return b.scanThenStop(&x.Index)
	case *ast.BinaryExpr:
		if loop, stop := b.scan(&x.X); loop != nil || stop || x.Op == token.LAND || x.Op == token.LOR {
			return loop, stop || loop == nil
		}
		if x.Op == token.QUO || x.Op == token.REM {
			return b.scanThenStop(&x.Y)
		}
		return b.scan(&x.Y)
	case *ast.CallExpr:
		return b.scanCall(p, x)
	}
	return nil, true
}

func (b *blockInliner) scanThenStop(p *ast.Expr) ([]ast.Stmt, bool) {
	loop, _ := b.scan(p)
	return loop, loop == nil
}

func (b *blockInliner) scanList(xs []ast.Expr, stopAfter bool) ([]ast.Stmt, bool) {
	for i := range xs {
		if loop, stop := b.scan(&xs[i]); loop != nil || stop {
			return loop, stop
		}
	}
	return nil, stopAfter
}

// scanCall: a conversion is pure; any other call runs its receiver and arguments first, then is an effect itself.
func (b *blockInliner) scanCall(p *ast.Expr, c *ast.CallExpr) ([]ast.Stmt, bool) {
	if loop, res := b.lower(c); loop != nil {
		*p = res
		return loop, false
	}
	if b.isType(c.Fun) {
		return b.scanList(c.Args, false)
	}
	if sel, ok := c.Fun.(*ast.SelectorExpr); ok {
		if loop, stop := b.scan(&sel.X); loop != nil || stop {
			return loop, stop
		}
	}
	return b.scanList(c.Args, true)
}

// isType: x names a type (a conversion's function), predeclared or declared in the file.
func (b *blockInliner) isType(x ast.Expr) bool {
	switch t := x.(type) {
	case *ast.Ident:
		return b.types[t.Name] || predeclaredTypes[t.Name]
	case *ast.ParenExpr:
		return b.isType(t.X)
	case *ast.StarExpr:
		return b.isType(t.X)
	case *ast.IndexExpr:
		return b.isType(t.X)
	case *ast.IndexListExpr:
		return b.isType(t.X)
	case *ast.ArrayType, *ast.MapType, *ast.FuncType, *ast.InterfaceType, *ast.ChanType:
		return true
	}
	return false
}

var predeclaredTypes = map[string]bool{
	"any": true, "bool": true, "byte": true, "rune": true, "string": true, "error": true,
	"int": true, "int8": true, "int16": true, "int32": true, "int64": true,
	"uint": true, "uint8": true, "uint16": true, "uint32": true, "uint64": true, "uintptr": true,
	"float32": true, "float64": true, "complex64": true, "complex128": true,
}

// lower is c as its loop and result when c calls a helper of b.own with a closure that can move.
func (b *blockInliner) lower(c *ast.CallExpr) ([]ast.Stmt, ast.Expr) {
	name, targs := helperCall(c)
	if !b.own[name] || c.Ellipsis.IsValid() {
		return nil, nil
	}
	b.pos = c.Pos()
	switch {
	case name == "Enumerable___CountBlock_onArray" && len(c.Args) == 2:
		return b.countLoop(c)
	case name == "Enumerable___SumBlock_onArray" && len(c.Args) == 2 && len(targs) == 2:
		return b.sumLoop(c, targs[1])
	case name == "Array_Meta___New1" && len(c.Args) == 3:
		return b.newLoop(c)
	}
	return nil, nil
}

func (b *blockInliner) id(name string) *ast.Ident { return &ast.Ident{NamePos: b.pos, Name: name} }

func (b *blockInliner) define(l, r ast.Expr) ast.Stmt {
	return &ast.AssignStmt{Lhs: []ast.Expr{l}, TokPos: b.pos, Tok: token.DEFINE, Rhs: []ast.Expr{r}}
}

func (b *blockInliner) varDecl(name string, typ ast.Expr) ast.Stmt {
	return &ast.DeclStmt{Decl: &ast.GenDecl{TokPos: b.pos, Tok: token.VAR, Specs: []ast.Spec{&ast.ValueSpec{Names: []*ast.Ident{b.id(name)}, Type: typ}}}}
}

func (b *blockInliner) call(fn ast.Expr, args ...ast.Expr) *ast.CallExpr {
	return &ast.CallExpr{Fun: fn, Lparen: b.pos, Args: args, Rparen: b.pos}
}

func (b *blockInliner) sel(x, name string) ast.Expr {
	return &ast.SelectorExpr{X: b.id(x), Sel: b.id(name)}
}

func helperCall(c *ast.CallExpr) (string, []ast.Expr) {
	switch fn := c.Fun.(type) {
	case *ast.Ident:
		return fn.Name, nil
	case *ast.IndexExpr:
		if id, ok := fn.X.(*ast.Ident); ok {
			return id.Name, []ast.Expr{fn.Index}
		}
	case *ast.IndexListExpr:
		if id, ok := fn.X.(*ast.Ident); ok {
			return id.Name, fn.Indices
		}
	}
	return "", nil
}

// inlineBody: param is "" when the body never reads it (a local Go would reject as unused); stmts ends in use(value).
type inlineBody struct {
	param string
	stmts func(use func(ast.Expr) ast.Stmt) []ast.Stmt
}

func (b *blockInliner) closureBody(x ast.Expr) *inlineBody {
	lit, ok := x.(*ast.FuncLit)
	if !ok || lit.Type.Params == nil || len(lit.Type.Params.List) != 1 || len(lit.Type.Params.List[0].Names) > 1 {
		return nil
	}
	if r := lit.Type.Results; r == nil || len(r.List) != 1 || len(r.List[0].Names) != 0 {
		return nil
	}
	var pid *ast.Ident
	if ns := lit.Type.Params.List[0].Names; len(ns) == 1 && ns[0].Name != "_" {
		pid = ns[0]
		if pid.Obj == nil { // without the resolver's object a nested closure's same-named param would count as a read
			return nil
		}
	}
	returns, ok := b.closureShape(lit.Body, pid)
	if !ok {
		return nil
	}
	ib := &inlineBody{}
	if pid != nil && readsIdent(lit.Body, pid) {
		ib.param = pid.Name
	}
	list := lit.Body.List
	if last := len(list) - 1; returns == 1 && last >= 0 {
		if r, ok := list[last].(*ast.ReturnStmt); ok {
			ib.stmts = func(use func(ast.Expr) ast.Stmt) []ast.Stmt {
				b.pos = r.Return
				return append(list[:last:last], use(r.Results[0]))
			}
			return ib
		}
	}
	ib.stmts = func(use func(ast.Expr) ast.Stmt) []ast.Stmt {
		b.n++
		v, label := fmt.Sprintf("rbBlk%dv", b.n), fmt.Sprintf("rbBlk%dnext", b.n)
		b.labels[label] = 1 // unique by construction: the enclosing closure may still move
		decl := b.varDecl(v, lit.Type.Results.List[0].Type)
		jumped := false
		// a block of its own: a goto to the label must not jump over the body's declarations
		inner := &ast.BlockStmt{Lbrace: lit.Body.Lbrace, List: returnsToGoto(list, v, label, &jumped), Rbrace: lit.Body.Rbrace}
		b.pos = lit.Body.Rbrace
		after := use(b.id(v))
		if jumped {
			after = &ast.LabeledStmt{Label: b.id(label), Colon: b.pos, Stmt: after}
		}
		return []ast.Stmt{decl, inner, after}
	}
	return ib
}

// closureShape: a defer (rescue) would run at the enclosing function's exit, a repeated label would clash there, and an assigned param is decision 387's exclusion.
func (b *blockInliner) closureShape(body *ast.BlockStmt, pid *ast.Ident) (int, bool) {
	returns, ok := 0, true
	ast.Inspect(body, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.FuncLit:
			ast.Inspect(x.Body, func(m ast.Node) bool {
				ok = ok && !assignsIdent(m, pid)
				return ok
			})
			return false
		case *ast.DeferStmt:
			ok = false
		case *ast.ReturnStmt:
			returns++
			ok = ok && len(x.Results) == 1
		case *ast.LabeledStmt:
			ok = ok && b.labels[x.Label.Name] == 1
		default:
			ok = ok && !assignsIdent(n, pid)
		}
		return ok
	})
	return returns, ok
}

// assignsIdent counts a nil Obj (a node an earlier rewrite made) as the param: skipping a loop is always safe.
func assignsIdent(n ast.Node, pid *ast.Ident) bool {
	if pid == nil {
		return false
	}
	is := func(x ast.Expr) bool {
		id, ok := x.(*ast.Ident)
		return ok && id.Name == pid.Name && (id.Obj == nil || id.Obj == pid.Obj)
	}
	switch x := n.(type) {
	case *ast.AssignStmt:
		for _, l := range x.Lhs {
			if is(l) {
				return true
			}
		}
	case *ast.IncDecStmt:
		return is(x.X)
	case *ast.UnaryExpr:
		return x.Op == token.AND && is(x.X)
	}
	return false
}

// readsIdent goes by Obj: a nested closure's own param of the same name is not a read.
func readsIdent(body *ast.BlockStmt, pid *ast.Ident) bool {
	found := false
	ast.Inspect(body, func(n ast.Node) bool {
		if id, ok := n.(*ast.Ident); ok && id.Name == pid.Name && id.Obj == pid.Obj {
			found = true
		}
		return !found
	})
	return found
}

// returnsToGoto: the last statement's return falls through to the label, so only earlier ones jump.
func returnsToGoto(list []ast.Stmt, v, label string, jumped *bool) []ast.Stmt {
	out := make([]ast.Stmt, 0, len(list))
	for i, s := range list {
		if i == len(list)-1 {
			switch x := s.(type) {
			case *ast.ReturnStmt:
				out = append(out, &ast.AssignStmt{Lhs: []ast.Expr{&ast.Ident{NamePos: x.Return, Name: v}}, TokPos: x.Return, Tok: token.ASSIGN, Rhs: x.Results})
				continue
			case *ast.IfStmt: // an if/else ending the body: its branches' last returns fall through as well
				tailIf(x, v, label, jumped)
				out = append(out, x)
				continue
			}
		}
		out = append(out, jumpReturns([]ast.Stmt{s}, v, label, jumped)...)
	}
	return out
}

func tailIf(x *ast.IfStmt, v, label string, jumped *bool) {
	x.Body.List = returnsToGoto(x.Body.List, v, label, jumped)
	switch e := x.Else.(type) {
	case *ast.BlockStmt:
		e.List = returnsToGoto(e.List, v, label, jumped)
	case *ast.IfStmt:
		tailIf(e, v, label, jumped)
	}
}

// jumpReturns rewrites every return of list, nested lists included, stopping at closures (their returns are their own).
func jumpReturns(list []ast.Stmt, v, label string, jumped *bool) []ast.Stmt {
	out := make([]ast.Stmt, 0, len(list))
	for _, s := range list {
		r, ok := s.(*ast.ReturnStmt)
		if !ok {
			ast.Inspect(s, func(n ast.Node) bool {
				switch x := n.(type) {
				case *ast.FuncLit:
					return false
				case *ast.BlockStmt:
					x.List = jumpReturns(x.List, v, label, jumped)
				case *ast.CaseClause:
					x.Body = jumpReturns(x.Body, v, label, jumped)
				case *ast.CommClause:
					x.Body = jumpReturns(x.Body, v, label, jumped)
				case *ast.LabeledStmt:
					if r, ok := x.Stmt.(*ast.ReturnStmt); ok {
						x.Stmt = &ast.BlockStmt{Lbrace: r.Return, List: jumpReturns([]ast.Stmt{r}, v, label, jumped), Rbrace: r.End()}
					}
				}
				return true
			})
			out = append(out, s)
			continue
		}
		*jumped = true
		out = append(out,
			&ast.AssignStmt{Lhs: []ast.Expr{&ast.Ident{NamePos: r.Return, Name: v}}, TokPos: r.Return, Tok: token.ASSIGN, Rhs: r.Results},
			&ast.BranchStmt{TokPos: r.Return, Tok: token.GOTO, Label: &ast.Ident{NamePos: r.Return, Name: label}})
	}
	return out
}

// arrayLoop re-reads the length each step, as Array#each: a block that grows or shrinks the receiver sees what MRI's does.
func (b *blockInliner) arrayLoop(recv ast.Expr, lit *ast.FuncLit, ib *inlineBody, use func(ast.Expr) ast.Stmt) []ast.Stmt {
	b.n++
	a := fmt.Sprintf("rbBlk%da", b.n)
	head := b.define(b.id(a), recv)
	b.pos = lit.Pos()
	i := fmt.Sprintf("rbBlk%di", b.n)
	body := []ast.Stmt{&ast.IfStmt{
		If:   b.pos,
		Cond: &ast.BinaryExpr{X: b.id(i), OpPos: b.pos, Op: token.GEQ, Y: b.call(b.id("len"), b.sel(a, "s"))},
		Body: &ast.BlockStmt{Lbrace: b.pos, List: []ast.Stmt{&ast.BranchStmt{TokPos: b.pos, Tok: token.BREAK}}, Rbrace: b.pos},
	}}
	if ib.param != "" {
		body = append(body, b.define(b.id(ib.param), &ast.IndexExpr{X: b.sel(a, "s"), Lbrack: b.pos, Index: b.id(i), Rbrack: b.pos}))
	}
	loop := &ast.ForStmt{
		For:  b.pos,
		Init: b.define(b.id(i), &ast.BasicLit{ValuePos: b.pos, Kind: token.INT, Value: "0"}),
		Post: &ast.IncDecStmt{X: b.id(i), TokPos: b.pos, Tok: token.INC},
	}
	body = append(body, ib.stmts(use)...)
	loop.Body = &ast.BlockStmt{Lbrace: lit.Pos(), List: body, Rbrace: lit.Body.Rbrace} // the closure's closing //line stays inside the loop
	return []ast.Stmt{head, loop}
}

// countLoop: the block's type is `(E) -> bool`, so its value is a Boolean the codegen already made truthy.
func (b *blockInliner) countLoop(c *ast.CallExpr) ([]ast.Stmt, ast.Expr) {
	ib := b.closureBody(c.Args[1])
	if ib == nil || !isIdent(c.Args[1].(*ast.FuncLit).Type.Results.List[0].Type, "Boolean") {
		return nil, nil
	}
	b.n++
	n := fmt.Sprintf("rbBlk%dn", b.n)
	head := b.varDecl(n, b.id("Integer"))
	use := func(v ast.Expr) ast.Stmt {
		inc := &ast.IncDecStmt{X: b.id(n), TokPos: b.pos, Tok: token.INC}
		return &ast.IfStmt{If: b.pos, Cond: v, Body: &ast.BlockStmt{Lbrace: b.pos, List: []ast.Stmt{inc}, Rbrace: b.pos}}
	}
	loop := b.arrayLoop(c.Args[0], c.Args[1].(*ast.FuncLit), ib, use)
	b.pos = c.Pos() // the result stands where the call stood
	return append([]ast.Stmt{head}, loop...), b.id(n)
}

// sumLoop is the prelude's two typed sums, the same calls (Integer's checked `+`, rbSummer's Kahan-Babuska addFloat); any other N keeps the helper.
func (b *blockInliner) sumLoop(c *ast.CallExpr, nt ast.Expr) ([]ast.Stmt, ast.Expr) {
	isInt, isFloat := isIdent(nt, "Integer"), isIdent(nt, "Float")
	if !isInt && !isFloat {
		return nil, nil
	}
	ib := b.closureBody(c.Args[1])
	if ib == nil {
		return nil, nil
	}
	b.n++
	t := fmt.Sprintf("rbBlk%dt", b.n)
	var head ast.Stmt
	var use func(ast.Expr) ast.Stmt
	if isInt {
		head = b.varDecl(t, b.id("Integer"))
		use = func(v ast.Expr) ast.Stmt {
			return &ast.AssignStmt{Lhs: []ast.Expr{b.id(t)}, TokPos: b.pos, Tok: token.ASSIGN, Rhs: []ast.Expr{b.call(b.sel(t, "Op_plus"), v)}}
		}
	} else {
		head = b.define(b.id(t), &ast.CompositeLit{Type: b.id("rbSummer"), Lbrace: b.pos, Elts: []ast.Expr{&ast.KeyValueExpr{Key: b.id("floating"), Colon: b.pos, Value: b.id("true")}}, Rbrace: b.pos})
		use = func(v ast.Expr) ast.Stmt {
			if !untypedLit(v) {
				v = b.call(b.id("float64"), v)
			}
			return &ast.ExprStmt{X: b.call(b.sel(t, "addFloat"), v)}
		}
	}
	loop := b.arrayLoop(c.Args[0], c.Args[1].(*ast.FuncLit), ib, use)
	b.pos = c.Pos() // the result stands where the call stood
	var res ast.Expr = b.id(t)
	if isFloat {
		res = b.call(b.id("Float"), &ast.BinaryExpr{X: b.sel(t, "f"), OpPos: b.pos, Op: token.ADD, Y: b.sel(t, "c")})
	}
	return append([]ast.Stmt{head}, loop...), res
}

// untypedLit converts implicitly; float64() of one would be a lint finding.
func untypedLit(x ast.Expr) bool {
	if u, ok := x.(*ast.UnaryExpr); ok {
		x = u.X
	}
	_, ok := x.(*ast.BasicLit)
	return ok
}

// newLoop calls the helper without a block for its size checks and n zero slots, then sets each slot: the block cannot see the Array, so filling equals MRI's growing.
func (b *blockInliner) newLoop(c *ast.CallExpr) ([]ast.Stmt, ast.Expr) {
	ib := b.closureBody(c.Args[2])
	if ib == nil {
		return nil, nil
	}
	b.n++
	out := fmt.Sprintf("rbBlk%dout", b.n)
	idx := ib.param
	if idx == "" {
		idx = fmt.Sprintf("rbBlk%di", b.n)
	}
	end := c.Args[1].End()
	alloc := &ast.CallExpr{Fun: c.Fun, Lparen: c.Lparen, Args: []ast.Expr{c.Args[0], c.Args[1], &ast.Ident{NamePos: end, Name: "nil"}}, Rparen: end}
	head := b.define(b.id(out), alloc)
	lit := c.Args[2].Pos()
	b.pos = lit
	loop := &ast.RangeStmt{For: b.pos, Key: b.id(idx), TokPos: b.pos, Tok: token.DEFINE, X: b.call(b.id("Integer"), b.call(b.id("len"), b.sel(out, "s")))}
	use := func(v ast.Expr) ast.Stmt {
		slot := &ast.IndexExpr{X: b.sel(out, "s"), Lbrack: b.pos, Index: b.id(idx), Rbrack: b.pos}
		return &ast.AssignStmt{Lhs: []ast.Expr{slot}, TokPos: b.pos, Tok: token.ASSIGN, Rhs: []ast.Expr{v}}
	}
	loop.Body = &ast.BlockStmt{Lbrace: lit, List: ib.stmts(use)}
	loop.Body.Rbrace = c.Args[2].(*ast.FuncLit).Body.Rbrace
	b.pos = c.Pos() // the result stands where the call stood
	return []ast.Stmt{head, loop}, b.id(out)
}

func callsName(body *ast.BlockStmt, name string) bool {
	found := false
	ast.Inspect(body, func(n ast.Node) bool {
		if c, ok := n.(*ast.CallExpr); ok {
			switch fn := c.Fun.(type) {
			case *ast.Ident:
				found = found || fn.Name == name
			case *ast.SelectorExpr:
				found = found || fn.Sel.Name == name
			case *ast.IndexListExpr:
				found = found || isIdent(fn.X, name)
			case *ast.IndexExpr:
				found = found || isIdent(fn.X, name)
			}
		}
		return !found
	})
	return found
}
