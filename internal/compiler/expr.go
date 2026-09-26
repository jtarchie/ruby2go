package compiler

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/danielgatis/go-ruby-prism/parser"

	"rb2go/internal/rbs"
)

// expr is a generated Go expression with its Ruby type.
type expr struct {
	code     string
	typ      Type
	lit      bool // untyped Go constant
	stmt     bool // already a complete statement (assignment, panic)
	noreturn bool // panic/exit: terminates the statement list
	done     bool // already emitted; code only names the value
}

// genLiteral handles the leaf expressions.
func (f *fctx) genLiteral(n parser.Node) (expr, bool) {
	switch n := n.(type) {
	case *parser.StringNode:
		return expr{code: strconv.Quote(n.Unescaped.Value), typ: f.cls("String"), lit: true}, true
	case *parser.InterpolatedStringNode:
		return f.genInterp(n), true
	case *parser.IntegerNode:
		return expr{code: strings.ReplaceAll(f.f.text(n.Location), "_", ""), typ: f.cls("Integer"), lit: true}, true
	case *parser.FloatNode:
		return expr{code: f.f.text(n.Location), typ: f.cls("Float"), lit: true}, true
	case *parser.TrueNode:
		return expr{code: "true", typ: f.cls("Boolean"), lit: true}, true
	case *parser.FalseNode:
		return expr{code: "false", typ: f.cls("Boolean"), lit: true}, true
	case *parser.NilNode:
		return expr{code: "nil", typ: TNil{}}, true
	case *parser.SelfNode:
		return expr{code: f.selfCode, typ: f.selfType}, true
	}
	return expr{}, false
}

func (f *fctx) genExpr(n parser.Node, expected Type) expr {
	if e, ok := f.genLiteral(n); ok {
		return e
	}
	switch n := n.(type) {
	case *parser.LocalVariableReadNode:
		v := f.readLocal(n)
		return expr{code: v.goName, typ: v.typ}
	case *parser.ItLocalVariableReadNode:
		v := f.readLocal(&parser.LocalVariableReadNode{Name: "it", Location: n.Location})
		return expr{code: v.goName, typ: v.typ}
	case *parser.LocalVariableWriteNode:
		var ann Type
		if t, ok := f.f.trailing[f.f.line(n.Location.StartOffset)]; ok && !strings.HasPrefix(t, "[") {
			ann = f.parseTypeAnn(n, t)
		}
		exp := ann
		if exp == nil {
			if v := f.scope.lookup(n.Name); v != nil {
				exp = v.typ
			}
		}
		val := f.genExpr(n.Value, exp)
		return f.assignLocal(n, n.Name, val, ann)
	case *parser.LocalVariableOperatorWriteNode:
		cur := f.genExpr(&parser.LocalVariableReadNode{Name: n.Name, Location: n.Location}, nil)
		val := f.genOp(n, cur, n.BinaryOperator, n.Value)
		return f.assignLocal(n, n.Name, val, nil)
	case *parser.InstanceVariableReadNode, *parser.InstanceVariableWriteNode, *parser.InstanceVariableOperatorWriteNode:
		return f.genIvarExpr(n)
	case *parser.CallNode:
		return f.genCall(n, expected)
	case *parser.ArrayNode:
		return f.genArray(n, expected)
	case *parser.HashNode:
		return f.genHash(n, expected)
	case *parser.ParenthesesNode:
		st, ok := n.Body.(*parser.StatementsNode)
		if !ok || len(st.Body) != 1 {
			f.errorf(n, "parenthesised statement lists are not supported")
		}
		e := f.genExpr(st.Body[0], expected)
		e.code = "(" + e.code + ")"
		e.lit = false
		return e
	case *parser.IfNode, *parser.UnlessNode, *parser.CaseNode, *parser.BeginNode:
		return f.lift(n, expected, func(t tail) { f.genStmt(n, t) })
	case *parser.OrNode:
		return f.genOr(n)
	case *parser.AndNode:
		return f.genAnd(n)
	case *parser.YieldNode:
		return f.genYield(n)
	case *parser.SuperNode:
		return f.genSuper(n, n.Arguments, false)
	case *parser.ForwardingSuperNode:
		return f.genSuper(n, nil, true)
	case *parser.ConstantReadNode:
		f.errorf(n, "constant %s used as a value is not supported", n.Name)
	case *parser.RescueModifierNode:
		f.errorf(n, "`expr rescue expr` is not supported; use begin/rescue")
	}
	f.c.unsupported(f.f, n)
	return expr{}
}

// genIvarExpr handles @x reads and writes.
func (f *fctx) genIvarExpr(n parser.Node) expr {
	switch n := n.(type) {
	case *parser.InstanceVariableReadNode:
		iv := f.ivar(n, n.Name, nil)
		return expr{code: f.ivarCode(iv), typ: iv.Type}
	case *parser.InstanceVariableWriteNode:
		var exp Type
		if t, ok := f.f.trailing[f.f.line(n.Location.StartOffset)]; ok && !strings.HasPrefix(t, "[") {
			exp = f.parseTypeAnn(n, t)
		} else if iv := f.c.findIvar(f.owner, n.Name); iv != nil {
			exp = iv.Type
		}
		val := f.genExpr(n.Value, exp)
		if exp != nil {
			val.typ = exp
		}
		iv := f.ivar(n, n.Name, val.typ)
		return expr{code: f.ivarCode(iv) + " = " + f.coerce(n, val, iv.Type), typ: iv.Type, stmt: true}
	case *parser.InstanceVariableOperatorWriteNode:
		iv := f.ivar(n, n.Name, nil)
		cur := expr{code: f.ivarCode(iv), typ: iv.Type}
		val := f.genOp(n, cur, n.BinaryOperator, n.Value)
		return expr{code: f.ivarCode(iv) + " = " + f.coerce(n, val, iv.Type), typ: iv.Type, stmt: true}
	}
	f.c.unsupported(f.f, n)
	return expr{}
}

func (f *fctx) cls(name string) Type { return TClass{C: f.c.classes[name]} }

func (f *fctx) parseTypeAnn(n parser.Node, s string) Type {
	t, err := rbs.ParseType(s)
	if err != nil {
		f.errorf(n, "%v", err)
	}
	var tps []string
	if f.m != nil {
		tps = f.m.TypeParams
	}
	return f.c.resolveType(t, typeScope{class: f.owner, methodTPs: tps, file: f.f, line: f.f.line(n.GetLocation().StartOffset)})
}

// lift turns a statement-shaped expression (if/case/begin) into a temp.
func (f *fctx) lift(n parser.Node, expected Type, gen func(t tail)) expr {
	var types []Type
	f.probe(func() { gen(tail{kind: tailAssign, target: "_", types: &types}) })
	typ := f.joinAll(n, types)
	if typ == nil || isNil(typ) {
		if expected != nil && !isVoid(expected) {
			typ = expected
		} else if typ == nil {
			f.errorf(n, "cannot infer the type of this expression")
		}
	}
	if isVoid(typ) {
		gen(tail{})
		return expr{code: "", typ: TVoid{}, stmt: true}
	}
	tmp := f.newTmp()
	f.emit("var %s %s", tmp, f.c.goType(typ))
	gen(tail{kind: tailAssign, target: tmp, typ: typ})
	return expr{code: tmp, typ: typ}
}

func (f *fctx) joinAll(n parser.Node, types []Type) Type {
	var out Type
	for _, t := range types {
		if isVoid(t) && !isNil(t) {
			continue
		}
		if out == nil {
			out = t
			continue
		}
		j, ok := join(out, t)
		if !ok {
			f.errorf(n, "branches have incompatible types %s and %s", out, t)
		}
		out = j
	}
	return out
}

// ---- operators, and/or

func (f *fctx) genOp(n parser.Node, left expr, op string, right parser.Node) expr {
	return f.genMethodCall(n, left, op, []parser.Node{right}, nil)
}

func (f *fctx) genOr(n *parser.OrNode) expr {
	l := f.genExpr(n.Left, nil)
	if isClass(l.typ, "Boolean") {
		r := f.genExpr(n.Right, l.typ)
		if !isClass(r.typ, "Boolean") {
			f.errorf(n, "`||` with Boolean and %s is not supported", r.typ)
		}
		return expr{code: "(" + l.code + " || " + r.code + ")", typ: l.typ}
	}
	if !isOpt(l.typ) {
		f.errorf(n, "`||` on a non-nilable %s is always the left side", l.typ)
	}
	var r expr
	f.probe(func() { r = f.genExpr(n.Right, stripOpt(l.typ)) })
	typ, ok := join(stripOpt(l.typ), r.typ)
	if !ok {
		f.errorf(n, "`||` with incompatible types %s and %s", l.typ, r.typ)
	}
	tmp := f.newTmp()
	lt := f.newTmp()
	f.emit("var %s %s", tmp, f.c.goType(typ))
	f.emit("if %s := %s; %s != nil {", lt, l.code, lt)
	f.indent++
	f.emit("%s = %s", tmp, f.coerce(n, expr{code: "(*" + lt + ")", typ: stripOpt(l.typ)}, typ))
	f.indent--
	f.emit("} else {")
	f.indent++
	r = f.genExpr(n.Right, stripOpt(l.typ))
	f.emit("%s = %s", tmp, f.coerce(n, r, typ))
	f.indent--
	f.emit("}")
	return expr{code: tmp, typ: typ}
}

func (f *fctx) genAnd(n *parser.AndNode) expr {
	l := f.genExpr(n.Left, nil)
	if isClass(l.typ, "Boolean") {
		r := f.genExpr(n.Right, l.typ)
		if !isClass(r.typ, "Boolean") {
			f.errorf(n, "`&&` with Boolean and %s is not supported", r.typ)
		}
		return expr{code: "(" + l.code + " && " + r.code + ")", typ: l.typ}
	}
	f.errorf(n, "`&&` on %s as a value is not supported; use it in a condition", l.typ)
	return expr{}
}

// ---- strings

func (f *fctx) genInterp(n *parser.InterpolatedStringNode) expr {
	var parts []string
	for _, p := range n.Parts {
		switch p := p.(type) {
		case *parser.StringNode:
			parts = append(parts, strconv.Quote(p.Unescaped.Value))
		case *parser.EmbeddedStatementsNode:
			st := p.Statements
			if st == nil || len(st.Body) != 1 {
				f.errorf(p, "interpolation must contain a single expression")
			}
			e := f.genExpr(st.Body[0], nil)
			parts = append(parts, f.toS(st.Body[0], e))
		default:
			f.c.unsupported(f.f, p)
		}
	}
	if len(parts) == 0 {
		return expr{code: `""`, typ: f.cls("String"), lit: true}
	}
	allLit := true
	for _, p := range parts {
		if !strings.HasPrefix(p, `"`) {
			allLit = false
		}
	}
	if allLit {
		return expr{code: "String(" + strings.Join(parts, " + ") + ")", typ: f.cls("String")}
	}
	return expr{code: strings.Join(parts, " + "), typ: f.cls("String")}
}

func (f *fctx) toS(n parser.Node, e expr) string {
	if isClass(e.typ, "String") {
		return e.code
	}
	return f.genMethodCall(n, e, "to_s", nil, nil).code
}

// ---- ivars

func (f *fctx) ivar(n parser.Node, name string, assigned Type) *Ivar {
	if f.owner == nil || !f.owner.isStruct() || f.owner.universal {
		f.errorf(n, "instance variables are only supported in struct classes")
	}
	iv := f.c.findIvar(f.owner, name)
	if iv == nil {
		if f.discover && assigned != nil && !isNil(assigned) && !isVoid(assigned) {
			return f.c.declareIvar(f.owner, name, assigned, f.f, f.f.line(n.GetLocation().StartOffset))
		}
		f.errorf(n, "instance variable %s has no known type; assign it in initialize or add `# @rbs %s: T`", name, name)
	}
	return iv
}

func (f *fctx) ivarCode(iv *Ivar) string {
	return fmt.Sprintf("%s._%s().%s", f.selfCode, iv.Owner.Name, goFieldName(iv.Name))
}

// ---- literals

func (f *fctx) genArray(n *parser.ArrayNode, expected Type) expr {
	if tt, ok := expected.(TTuple); ok && len(tt.Elems) == len(n.Elements) {
		codes := make([]string, len(n.Elements))
		for i, el := range n.Elements {
			codes[i] = f.coerce(el, f.genExpr(el, tt.Elems[i]), tt.Elems[i])
		}
		return expr{code: f.c.goType(tt) + "{" + strings.Join(codes, ", ") + "}", typ: tt}
	}
	var elemT Type
	if ec, ok := expected.(TClass); ok && ec.C.Name == "Array" {
		elemT = ec.Args[0]
	}
	elems := make([]expr, len(n.Elements))
	for i, el := range n.Elements {
		if _, ok := el.(*parser.SplatNode); ok {
			f.errorf(el, "splat inside array literals is not supported")
		}
		elems[i] = f.genExpr(el, elemT)
	}
	if elemT == nil {
		if t, ok := f.tupleLiteral(n, elems); ok {
			return t
		}
		elemT = f.inferElemType(n, elems)
	}
	codes := make([]string, len(elems))
	for i, e := range elems {
		codes[i] = f.coerce(n.Elements[i], e, elemT)
	}
	t := TClass{C: f.c.classes["Array"], Args: []Type{elemT}}
	return expr{code: "(&Array[" + f.c.goType(elemT) + "]{" + strings.Join(codes, ", ") + "})", typ: t}
}

// inferElemType picks the element type of an unannotated array literal.
// Ruby's `[]` is untyped; so is ours.
func (f *fctx) inferElemType(n parser.Node, elems []expr) Type {
	if len(elems) == 0 {
		return TAny{}
	}
	if isNil(elems[0].typ) {
		f.errorf(n, "cannot infer the element type of [nil]; add `#: Array[T?]`")
	}
	return elems[0].typ
}

// tupleLiteral turns a literal with mixed element types into a tuple.
func (f *fctx) tupleLiteral(n parser.Node, elems []expr) (expr, bool) {
	same := true
	for _, e := range elems {
		if !typeEq(e.typ, elems[0].typ) {
			same = false
		}
	}
	if same {
		return expr{}, false
	}
	if len(elems) < 2 || len(elems) > 3 {
		f.errorf(n, "array literal with mixed element types; add `#: Array[T]`")
	}
	ts := make([]Type, len(elems))
	codes := make([]string, len(elems))
	for i, e := range elems {
		if isNil(e.typ) {
			f.errorf(n, "tuple literal with nil element needs an annotation")
		}
		ts[i] = e.typ
		codes[i] = f.coerce(n, e, e.typ)
	}
	tt := TTuple{Elems: ts}
	return expr{code: f.c.goType(tt) + "{" + strings.Join(codes, ", ") + "}", typ: tt}, true
}

func (f *fctx) genHash(n *parser.HashNode, expected Type) expr {
	var kT, vT Type
	if ec, ok := expected.(TClass); ok && ec.C.Name == "Hash" {
		kT, vT = ec.Args[0], ec.Args[1]
	}
	type kv struct{ k, v expr }
	pairs := make([]kv, 0, len(n.Elements))
	for _, el := range n.Elements {
		a, ok := el.(*parser.AssocNode)
		if !ok {
			f.c.unsupported(f.f, el)
		}
		if _, ok := a.Key.(*parser.SymbolNode); ok {
			f.errorf(a.Key, "symbol keys are not supported; use string keys")
		}
		k := f.genExpr(a.Key, kT)
		v := f.genExpr(a.Value, vT)
		if kT == nil {
			kT, vT = k.typ, v.typ
		}
		pairs = append(pairs, kv{k, v})
	}
	if kT == nil {
		kT, vT = TAny{}, TAny{}
	}
	if isNil(kT) || isNil(vT) {
		f.errorf(n, "cannot infer hash types from nil; add `#: Hash[K, V]`")
	}
	var b strings.Builder
	fmt.Fprintf(&b, "NewHash[%s, %s]()", f.c.goType(kT), f.c.goType(vT))
	for _, p := range pairs {
		fmt.Fprintf(&b, ".__Set(%s, %s)", f.coerce(n, p.k, kT), f.coerce(n, p.v, vT))
	}
	return expr{code: b.String(), typ: TClass{C: f.c.classes["Hash"], Args: []Type{kT, vT}}}
}

// ---- coercion

// coerce converts e to the representation of type `to`.
func (f *fctx) coerce(n parser.Node, e expr, to Type) string {
	if to == nil || isVoid(to) && !isNil(to) {
		return e.code
	}
	if typeEq(e.typ, to) {
		return e.code
	}
	switch to := to.(type) {
	case TAny:
		switch {
		case isOpt(e.typ):
			return "Opt(" + e.code + ")"
		case e.lit:
			return f.c.goType(e.typ) + "(" + e.code + ")"
		}
		return e.code
	case TOpt:
		switch {
		case isNil(e.typ):
			return "nil"
		case isOpt(e.typ):
			return e.code
		case isAny(e.typ):
			return "OptOf[" + f.c.goType(to.Elem) + "](" + e.code + ")"
		}
		return "Ref[" + f.c.goType(to.Elem) + "](" + f.coerce(n, e, to.Elem) + ")"
	case TClass:
		if isAny(e.typ) {
			return e.code + ".(" + f.c.goType(to) + ")"
		}
		if isOpt(e.typ) {
			// Ruby would raise NoMethodError on nil; Go panics on the deref.
			return "(*" + e.code + ")"
		}
		if ec, ok := e.typ.(TClass); ok && to.C.GoType != "" && e.lit {
			_ = ec
			return e.code
		}
		if isNil(e.typ) {
			f.errorf(n, "nil where %s is expected", to)
		}
	case TVar:
		return e.code
	}
	return e.code
}

// ---- calls

func (f *fctx) genCall(n *parser.CallNode, expected Type) expr {
	if n.Receiver == nil {
		switch n.Name {
		case "raise":
			return f.genRaise(n)
		case "require", "require_relative":
			return expr{code: "", stmt: true, typ: TVoid{}}
		case "block_given?", "lambda", "proc", "binding", "send", "method_missing", "define_method":
			f.errorf(n, "%s is not supported", n.Name)
		}
	}
	if cr, ok := n.Receiver.(*parser.ConstantReadNode); ok {
		cls := f.c.classes[cr.Name]
		if cls == nil {
			f.errorf(cr, "unknown constant %s", cr.Name)
		}
		if n.Name != "new" {
			f.errorf(n, "class methods (%s.%s) are not supported", cr.Name, n.Name)
		}
		if n.Block != nil {
			f.errorf(n, "%s.new with a block is not supported", cr.Name)
		}
		return f.genNew(n, cls, callArgs(n), nil, expected)
	}
	if n.IsSAFE_NAVIGATION() {
		return f.genSafeNav(n)
	}
	var recv expr
	if n.Receiver == nil {
		recv = expr{code: f.selfCode, typ: f.selfType}
	} else {
		recv = f.genExpr(n.Receiver, nil)
	}
	if n.Block != nil {
		if _, ok := n.Block.(*parser.BlockNode); ok {
			if e := f.resolve(recv.typ, n.Name); e != nil && e.M.Iterator {
				f.errorf(n, "%s with a block is an iterator and can only be used as a statement", n.Name)
			}
		}
	}
	return f.genMethodCall(n, recv, n.Name, callArgs(n), n.Block)
}

func (f *fctx) genSafeNav(n *parser.CallNode) expr {
	recv := f.genExpr(n.Receiver, nil)
	if !isOpt(recv.typ) {
		return f.genMethodCall(n, recv, n.Name, callArgs(n), n.Block)
	}
	elem := recv.typ.(TOpt).Elem
	rt := f.newTmp()
	inner := expr{code: "(*" + rt + ")", typ: elem}
	var probe expr
	f.probe(func() { probe = f.genMethodCall(n, inner, n.Name, callArgs(n), n.Block) })
	if isVoid(probe.typ) {
		f.emit("if %s := %s; %s != nil {", rt, recv.code, rt)
		f.indent++
		e := f.genMethodCall(n, inner, n.Name, callArgs(n), n.Block)
		f.emitExprStmt(n, e)
		f.indent--
		f.emit("}")
		return expr{code: "", typ: TVoid{}, stmt: true}
	}
	resT := probe.typ
	if !isOpt(resT) && !isAny(resT) {
		resT = TOpt{Elem: resT}
	}
	tmp := f.newTmp()
	f.emit("var %s %s", tmp, f.c.goType(resT))
	f.emit("if %s := %s; %s != nil {", rt, recv.code, rt)
	f.indent++
	e := f.genMethodCall(n, inner, n.Name, callArgs(n), n.Block)
	f.emit("%s = %s", tmp, f.coerce(n, e, resT))
	f.indent--
	f.emit("}")
	return expr{code: tmp, typ: resT}
}

// resolve finds the method entry for name on a receiver type, or nil.
// Top-level defs are private methods on Object, so a receiver-less call
// anywhere can reach them.
func (f *fctx) resolve(recvT Type, name string) *entry {
	var e *entry
	switch t := recvT.(type) {
	case TClass:
		e = t.C.lookup(name)
	case TVar:
		if t.Name == "Self" && f.owner != nil {
			e = f.owner.lookup(name)
		}
	}
	if e == nil {
		if td := f.c.topDefs[name]; td != nil {
			return &entry{M: td}
		}
	}
	return e
}

// genMethodCall dispatches a call on an already-generated receiver.
func (f *fctx) genMethodCall(n parser.Node, recv expr, name string, args []parser.Node, block parser.Node) expr {
	switch t := recv.typ.(type) {
	case TOpt:
		return f.optCall(n, recv, name, args)
	case TTuple:
		return f.tupleCall(n, recv, name, args)
	case TClass:
		e := t.C.lookup(name)
		if e == nil && recv.code == f.selfCode {
			if td := f.c.topDefs[name]; td != nil {
				e = &entry{M: td}
			}
		}
		if e == nil {
			f.errorf(n, "undefined method %s for %s", name, recv.typ)
		}
		return f.callEntry(n, e, recv, args, block)
	case TVar:
		if t.Name == "Self" && f.owner != nil {
			if e := f.owner.lookup(name); e != nil {
				return f.callEntry(n, e, recv, args, block)
			}
			if td := f.c.topDefs[name]; td != nil {
				return f.callEntry(n, &entry{M: td}, recv, args, block)
			}
			if e := f.c.classes["Object"].lookup(name); e != nil {
				return f.callEntry(n, e, recv, args, block)
			}
		}
		return f.universalCall(n, recv, name, args, block)
	case TAny, TNil:
		return f.universalCall(n, recv, name, args, block)
	}
	f.errorf(n, "undefined method %s for %s", name, recv.typ)
	return expr{}
}

// genArgs generates and coerces call arguments against m's parameters,
// binding type variables in env. exprs, if non-nil, are pre-generated.
func (f *fctx) genArgs(n parser.Node, m *Method, env map[string]Type, args []parser.Node, exprs []expr) []string {
	var codes []string
	nargs := len(args)
	if exprs != nil {
		nargs = len(exprs)
	}
	ai := 0
	for _, p := range m.Params {
		if p.Rest {
			for ; ai < nargs; ai++ {
				var a expr
				an := n
				if exprs != nil {
					a = exprs[ai]
				} else {
					an = args[ai]
					if sp, ok := an.(*parser.SplatNode); ok {
						a = f.genExpr(sp.Expression, nil)
						ac, ok := a.typ.(TClass)
						if !ok || ac.C.Name != "Array" {
							f.errorf(an, "splat of non-array %s", a.typ)
						}
						unify(p.Type, ac.Args[0], env)
						codes = append(codes, "(*"+a.code+")...")
						continue
					}
					a = f.genExpr(an, closed(p.Type, env))
				}
				unify(p.Type, a.typ, env)
				codes = append(codes, f.coerce(an, a, subst(p.Type, env)))
			}
			continue
		}
		if ai < nargs {
			var a expr
			an := n
			if exprs != nil {
				a = exprs[ai]
			} else {
				an = args[ai]
				if _, ok := an.(*parser.SplatNode); ok {
					f.errorf(an, "splat into a non-rest parameter is not supported")
				}
				a = f.genExpr(an, closed(p.Type, env))
			}
			ai++
			unify(p.Type, a.typ, env)
			codes = append(codes, f.coerce(an, a, subst(p.Type, env)))
			continue
		}
		if p.Default != nil {
			d := f.genExpr(p.Default, closed(p.Type, env))
			unify(p.Type, d.typ, env)
			codes = append(codes, f.coerce(n, d, subst(p.Type, env)))
			continue
		}
		f.errorf(n, "%s: wrong number of arguments (given %d, expected %d)", m, nargs, len(m.Params))
	}
	if ai < nargs {
		f.errorf(n, "%s: wrong number of arguments (given %d, expected %d)", m, nargs, len(m.Params))
	}
	return codes
}

// closed returns subst(t, env) if it has no unbound method type vars.
func closed(t Type, env map[string]Type) Type {
	s := subst(t, env)
	var vars []string
	freeVars(s, &vars)
	for _, v := range vars {
		if _, ok := env[v]; !ok {
			return nil
		}
	}
	return s
}

func (f *fctx) callEntry(n parser.Node, e *entry, recv expr, args []parser.Node, block parser.Node) expr {
	m := e.M
	env := map[string]Type{}
	if rt, ok := recv.typ.(TClass); ok {
		classEnv := map[string]Type{}
		for i, p := range rt.C.TypeParams {
			if i < len(rt.Args) {
				classEnv[p] = rt.Args[i]
			}
		}
		env = composeEnv(e.Env, classEnv)
	} else {
		for k, v := range e.Env {
			env[k] = v
		}
	}
	env["Self"] = recv.typ
	// bind vars visible in the current generic context so they count as bound
	if m.Private && recv.code != f.selfCode {
		f.errorf(n, "private method %s called on %s", m.Name, recv.typ)
	}
	codes := f.genArgs(n, m, env, args, nil)
	if m.Block != nil {
		if m.Iterator {
			f.errorf(n, "%s is an iterator (its block returns void); call it as a statement with a block", m.Name)
		}
		if block == nil {
			f.errorf(n, "%s requires a block", m.Name)
		}
		codes = append(codes, f.genClosure(n, block, m.Block, env))
	} else if block != nil {
		f.errorf(n, "%s does not take a block", m.Name)
	}
	for _, tp := range m.TypeParams {
		if _, ok := env[tp]; !ok {
			f.errorf(n, "cannot infer type parameter %s of %s", tp, m)
		}
	}
	ret := subst(m.Ret, env)
	code := f.callCode(e, recv, codes, env)
	if v, ok := m.Ret.(TVar); ok && v.Name == "Self" && e.Owner != nil {
		if rc, ok := recv.typ.(TClass); ok && rc.C.isStruct() && e.Entry != rc.C {
			code += ".(" + f.c.goType(recv.typ) + ")"
		}
	}
	return expr{code: code, typ: ret}
}

// callCode renders the call for entry e.
func (f *fctx) callCode(e *entry, recv expr, args []string, env map[string]Type) string {
	m := e.M
	argList := strings.Join(args, ", ")
	if recv.lit {
		recv.code = f.c.goType(recv.typ) + "(" + recv.code + ")"
	}
	if m.Owner == nil {
		return m.GoName + "(" + argList + ")"
	}
	free := m.generic() || (m.Private && !f.c.isDirectMethod(m)) || (m.Owner.GoType == "" && !f.hasForwarder(recv.typ, e))
	if !free {
		return recv.code + "." + m.GoName + "(" + argList + ")"
	}
	targs := ""
	if f.needsExplicitTypeArgs(m) {
		var ts []string
		if m.Owner.GoType == "" {
			ts = append(ts, f.c.goType(recv.typ))
		}
		for _, p := range m.Owner.TypeParams {
			ts = append(ts, f.c.goType(env[p]))
		}
		for _, p := range m.TypeParams {
			ts = append(ts, f.c.goType(env[p]))
		}
		targs = "[" + strings.Join(ts, ", ") + "]"
	}
	return freeFuncName(m) + targs + "(" + recv.code + comma(argList) + ")"
}

// hasForwarder reports whether the receiver's Go type carries a method for e.
func (f *fctx) hasForwarder(recvT Type, e *entry) bool {
	if e.Owner == nil || e.M.Private || e.M.generic() {
		return false
	}
	switch t := recvT.(type) {
	case TClass:
		if t.C.universal || t.C.IsModule {
			return false
		}
		return f.c.wantsForwarder(t.C, *e)
	case TVar:
		if t.Name != "Self" || f.owner == nil || f.owner.universal {
			return false
		}
		if f.owner.isStruct() {
			return true
		}
		if f.owner.IsModule {
			return f.c.selfCalls(f.owner)[e.M.Name]
		}
		return e.Owner == f.owner
	}
	return false
}

func (f *fctx) needsExplicitTypeArgs(m *Method) bool {
	var mentioned []string
	for _, p := range m.Params {
		freeVars(p.Type, &mentioned)
	}
	if m.Block != nil {
		for _, p := range m.Block.Params {
			freeVars(p, &mentioned)
		}
		freeVars(m.Block.Ret, &mentioned)
	}
	for _, tp := range m.TypeParams {
		found := false
		for _, v := range mentioned {
			if v == tp {
				found = true
			}
		}
		if !found {
			return true
		}
	}
	return false
}

// ---- blocks

func (f *fctx) blockParamNames(b parser.Node) []string {
	switch p := b.(type) {
	case nil:
		return nil
	case *parser.BlockParametersNode:
		if p.Parameters == nil {
			return nil
		}
		ps := p.Parameters
		if len(ps.Optionals) > 0 || ps.Rest != nil || len(ps.Posts) > 0 || len(ps.Keywords) > 0 || ps.KeywordRest != nil || ps.Block != nil {
			f.errorf(b, "unsupported block parameter form")
		}
		var names []string
		for _, r := range ps.Requireds {
			rp, ok := r.(*parser.RequiredParameterNode)
			if !ok {
				f.errorf(r, "unsupported block parameter form")
			}
			names = append(names, rp.Name)
		}
		return names
	case *parser.NumberedParametersNode:
		var names []string
		for i := 1; i <= int(p.Maximum); i++ {
			names = append(names, fmt.Sprintf("_%d", i))
		}
		return names
	case *parser.ItParametersNode:
		return []string{"it"}
	}
	f.c.unsupported(f.f, b)
	return nil
}

// bindBlockParams declares block params for the yielded types, returning
// the Go loop/closure parameter names and a destructuring prologue.
func (f *fctx) bindBlockParams(n parser.Node, names []string, yields []Type) (goParams []string, prologue func()) {
	if len(yields) == 1 {
		if tt, ok := yields[0].(TTuple); ok && len(names) > 1 {
			if len(names) != len(tt.Elems) {
				f.errorf(n, "block takes %d params but the tuple has %d elements", len(names), len(tt.Elems))
			}
			p := f.newTmp()
			return []string{p}, func() {
				lhs := make([]string, 0, len(names))
				rhs := make([]string, 0, len(names))
				vars := make([]*local, 0, len(names))
				for i, nm := range names {
					v := f.blockParam(nm, tt.Elems[i])
					lhs = append(lhs, v.goName)
					rhs = append(rhs, fmt.Sprintf("%s.F%d", p, i))
					vars = append(vars, v)
				}
				f.emit("%s := %s", strings.Join(lhs, ", "), strings.Join(rhs, ", "))
				for _, v := range vars {
					f.noteUnused(v)
				}
			}
		}
	}
	if len(names) > len(yields) {
		f.errorf(n, "block takes %d params but only %d values are yielded", len(names), len(yields))
	}
	for i := range yields {
		if i < len(names) {
			v := f.blockParam(names[i], yields[i])
			goParams = append(goParams, v.goName)
		} else {
			goParams = append(goParams, "_")
		}
	}
	return goParams, func() {
		for i := range yields {
			if i < len(names) {
				if v := f.scope.lookup(names[i]); v != nil {
					f.noteUnused(v)
				}
			}
		}
	}
}

// genClosure renders a block as a Go func literal.
func (f *fctx) genClosure(n parser.Node, block parser.Node, sig *BlockSig, env map[string]Type) string {
	params := substAll(sig.Params, env)
	var body parser.Node
	var names []string
	var symbolCall string
	switch b := block.(type) {
	case *parser.BlockNode:
		names = f.blockParamNames(b.Parameters)
		body = b.Body
	case *parser.BlockArgumentNode:
		sym, ok := b.Expression.(*parser.SymbolNode)
		if !ok {
			f.errorf(b, "only &:symbol block arguments are supported")
		}
		symbolCall = sym.Unescaped.Value
		if len(params) != 1 {
			f.errorf(b, "&:%s needs a single-argument block", symbolCall)
		}
		names = []string{"x_"}
	default:
		f.c.unsupported(f.f, block)
	}
	gen := func(t tail) {
		if symbolCall != "" {
			v := f.scope.lookup("x_")
			e := f.genMethodCall(n, expr{code: v.goName, typ: v.typ}, symbolCall, nil, nil)
			f.applyTail(n, e, t)
			return
		}
		f.genStmts(body, t)
	}
	// probe the body's type if the block return has unbound vars
	ret := closed(sig.Ret, env)
	if ret == nil {
		var types []Type
		f.probe(func() {
			saved := f.enterBlock()
			f.closures++
			f.loops = append(f.loops, loopClosure)
			_, pro := f.bindBlockParams(n, names, params)
			pro()
			gen(tail{kind: tailReturn, types: &types})
			f.loops = f.loops[:len(f.loops)-1]
			f.closures--
			f.leaveBlock(saved)
		})
		got := f.joinAll(n, types)
		if got == nil {
			got = TNil{}
		}
		if !unify(sig.Ret, got, env) {
			f.errorf(n, "block returns %s, expected %s", got, subst(sig.Ret, env))
		}
		ret = subst(sig.Ret, env)
	}
	var b strings.Builder
	savedBuf := f.buf
	f.buf = &b
	saved := f.enterBlock()
	f.closures++
	f.loops = append(f.loops, loopClosure)
	goParams, pro := f.bindBlockParams(n, names, params)
	ps := make([]string, 0, len(goParams))
	for i, gp := range goParams {
		ps = append(ps, gp+" "+f.c.goType(params[i]))
	}
	retS := ""
	if !isVoid(ret) {
		retS = " " + f.c.goType(ret)
	}
	f.emit("func(%s)%s {", strings.Join(ps, ", "), retS)
	f.indent++
	pro()
	if isVoid(ret) {
		gen(tail{})
	} else {
		gen(tail{kind: tailReturn, typ: ret})
	}
	f.indent--
	f.emit("}")
	f.loops = f.loops[:len(f.loops)-1]
	f.closures--
	f.leaveBlock(saved)
	f.buf = savedBuf
	code := strings.TrimSpace(b.String())
	// re-indent: the closure is embedded in an expression on the current line
	return code
}

// genIterCall emits `for ... range recv.Each(...) { body }` for iterator
// methods called with a block. Returns false if the call is not one.
func (f *fctx) genIterCall(n *parser.CallNode, t tail) bool {
	if _, ok := n.Receiver.(*parser.ConstantReadNode); ok {
		return false
	}
	var recvT Type
	if n.Receiver == nil {
		recvT = f.selfType
	} else {
		f.probe(func() { recvT = f.genExpr(n.Receiver, nil).typ })
	}
	if n.IsSAFE_NAVIGATION() {
		return false
	}
	e := f.resolve(recvT, n.Name)
	if e == nil || !e.M.Iterator {
		return false
	}
	if t.kind != tailNone && t.typ != nil && !isVoid(t.typ) {
		f.errorf(n, "the value of an iterator call (%s) cannot be used", n.Name)
	}
	var recv expr
	if n.Receiver == nil {
		recv = expr{code: f.selfCode, typ: f.selfType}
	} else {
		recv = f.genExpr(n.Receiver, nil)
	}
	m := e.M
	env := map[string]Type{}
	if rt, ok := recv.typ.(TClass); ok {
		classEnv := map[string]Type{}
		for i, p := range rt.C.TypeParams {
			classEnv[p] = rt.Args[i]
		}
		env = composeEnv(e.Env, classEnv)
	} else {
		for k, v := range e.Env {
			env[k] = v
		}
	}
	env["Self"] = recv.typ
	codes := f.genArgs(n, m, env, callArgs(n), nil)
	yields := substAll(m.Block.Params, env)
	blk := n.Block.(*parser.BlockNode)
	names := f.blockParamNames(blk.Parameters)
	call := f.callCode(e, recv, codes, env)
	saved := f.enterBlock()
	goParams, pro := f.bindBlockParams(n, names, yields)
	allBlank := true
	for _, gp := range goParams {
		if gp != "_" {
			allBlank = false
		}
	}
	if allBlank {
		f.emit("for range %s {", call)
	} else {
		f.emit("for %s := range %s {", strings.Join(goParams, ", "), call)
	}
	f.indent++
	f.loops = append(f.loops, loopIter)
	pro()
	f.genStmts(blk.Body, tail{})
	f.loops = f.loops[:len(f.loops)-1]
	f.indent--
	f.leaveBlock(saved)
	f.emit("}")
	if t.kind != tailNone {
		f.emptyTail(n, t)
	}
	return true
}

// ---- yield / super / new / raise

func (f *fctx) genYield(n *parser.YieldNode) expr {
	if f.blockSig == nil {
		f.errorf(n, "yield in a method whose signature has no block")
	}
	if f.closures > 0 {
		f.errorf(n, "yield inside a non-iterator block is not supported")
	}
	args := []parser.Node{}
	if n.Arguments != nil {
		args = n.Arguments.Arguments
	}
	codes := make([]string, 0, len(args))
	if len(args) != len(f.blockSig.Params) {
		f.errorf(n, "yield passes %d values but the block takes %d", len(args), len(f.blockSig.Params))
	}
	for i, a := range args {
		e := f.genExpr(a, f.blockSig.Params[i])
		codes = append(codes, f.coerce(a, e, f.blockSig.Params[i]))
	}
	if f.iterator {
		f.emit("if !yield(%s) {", strings.Join(codes, ", "))
		f.emit("\treturn")
		f.emit("}")
		return expr{code: "", typ: TVoid{}, stmt: true}
	}
	return expr{code: "blk(" + strings.Join(codes, ", ") + ")", typ: f.blockSig.Ret}
}

func (f *fctx) genSuper(n parser.Node, args *parser.ArgumentsNode, forwarding bool) expr {
	if f.m == nil || f.owner == nil {
		f.errorf(n, "super outside a method")
	}
	e := f.c.inheritedSig(f.m)
	if e == nil {
		if f.m.Name == "initialize" {
			return expr{code: "", typ: TVoid{}, stmt: true}
		}
		f.errorf(n, "super: no parent method %s", f.m.Name)
	}
	if e.Owner.GoType != "" {
		f.errorf(n, "super into a primitive class method is not supported")
	}
	env := map[string]Type{}
	for k, v := range e.Env {
		env[k] = v
	}
	env["Self"] = f.selfType
	var codes []string
	if forwarding {
		for _, p := range f.m.Params {
			if p.Rest {
				codes = append(codes, "(*"+goLocalName(p.Name)+")...")
			} else {
				codes = append(codes, goLocalName(p.Name))
			}
		}
	} else {
		var an []parser.Node
		if args != nil {
			an = args.Arguments
		}
		codes = f.genArgs(n, e.M, env, an, nil)
	}
	if e.M.Block != nil {
		f.errorf(n, "super to a block-taking method is not supported")
	}
	code := freeFuncName(e.M) + "(" + f.selfCode + comma(strings.Join(codes, ", ")) + ")"
	return expr{code: code, typ: subst(e.M.Ret, env)}
}

func (f *fctx) genNew(n parser.Node, cls *Class, args []parser.Node, exprs []expr, expected Type) expr {
	switch {
	case cls.IsModule:
		f.errorf(n, "cannot instantiate module %s", cls.Name)
	case cls.universal:
		f.errorf(n, "%s.new is not supported", cls.Name)
	case cls.GoType != "":
		if len(args) > 0 || len(exprs) > 0 {
			f.errorf(n, "%s.new with arguments is not supported", cls.Name)
		}
		var t TClass
		if et, ok := expected.(TClass); ok && et.C == cls {
			t = et
		} else if len(cls.TypeParams) == 0 {
			t = TClass{C: cls}
		} else {
			f.errorf(n, "%s.new needs a type annotation (`#: %s[...]`)", cls.Name, cls.Name)
		}
		targs := ""
		if len(t.Args) > 0 {
			targs = "[" + f.c.goTypes(t.Args) + "]"
		}
		return expr{code: "New" + cls.Name + targs + "()", typ: t}
	}
	init := cls.lookup("initialize")
	var codes []string
	if init != nil {
		env := map[string]Type{}
		for k, v := range init.Env {
			env[k] = v
		}
		env["Self"] = TClass{C: cls}
		codes = f.genArgs(n, init.M, env, args, exprs)
	} else if len(args) > 0 || len(exprs) > 0 {
		f.errorf(n, "%s.new takes no arguments", cls.Name)
	}
	return expr{code: "New" + cls.Name + "(" + strings.Join(codes, ", ") + ")", typ: TClass{C: cls}}
}

func (f *fctx) genRaise(n *parser.CallNode) expr {
	args := callArgs(n)
	exc := f.c.classes["Exception"]
	var val expr
	switch len(args) {
	case 0:
		f.errorf(n, "bare `raise` (re-raise) is not supported")
	case 1:
		if cr, ok := args[0].(*parser.ConstantReadNode); ok {
			cls := f.c.classes[cr.Name]
			if cls == nil || !cls.isSubclassOf(exc) {
				f.errorf(cr, "%s is not an exception class", cr.Name)
			}
			val = f.genNew(n, cls, nil, []expr{}, nil)
			break
		}
		a := f.genExpr(args[0], nil)
		if isClass(a.typ, "String") {
			val = f.genNew(n, f.c.classes["RuntimeError"], nil, []expr{a}, nil)
			break
		}
		if c := classOf(a.typ); c != nil && c.isSubclassOf(exc) {
			val = a
			break
		}
		f.errorf(args[0], "raise needs an exception class, a String, or an exception object (got %s)", a.typ)
	case 2:
		cr, ok := args[0].(*parser.ConstantReadNode)
		if !ok {
			f.errorf(args[0], "raise Class, message: first argument must be a class")
		}
		cls := f.c.classes[cr.Name]
		if cls == nil || !cls.isSubclassOf(exc) {
			f.errorf(cr, "%s is not an exception class", cr.Name)
		}
		msg := f.genExpr(args[1], f.cls("String"))
		val = f.genNew(n, cls, nil, []expr{msg}, nil)
	default:
		f.errorf(n, "raise with %d arguments is not supported", len(args))
	}
	return expr{code: "panic(" + val.code + ")", typ: TVoid{}, stmt: true, noreturn: true}
}

// ---- calls on nilable, tuple and untyped receivers

func (f *fctx) optCall(n parser.Node, recv expr, name string, args []parser.Node) expr {
	elem := recv.typ.(TOpt).Elem
	switch name {
	case "nil?":
		return expr{code: "Boolean(" + recv.code + " == nil)", typ: f.cls("Boolean")}
	case "to_s":
		return expr{code: "rbToS(Opt(" + recv.code + "))", typ: f.cls("String")}
	case "inspect":
		return expr{code: "rbInspect(Opt(" + recv.code + "))", typ: f.cls("String")}
	case "==", "!=", "equal?":
		if len(args) != 1 {
			f.errorf(n, "%s takes one argument", name)
		}
		a := f.genExpr(args[0], nil)
		code := "rbEq(Opt(" + recv.code + "), " + f.coerce(args[0], a, TAny{}) + ")"
		if name == "!=" {
			code = "!" + code
		}
		return expr{code: code, typ: f.cls("Boolean")}
	case "!":
		return expr{code: "Boolean(" + recv.code + " == nil)", typ: f.cls("Boolean")}
	}
	f.errorf(n, "method %s called on possibly-nil %s; narrow with `if x` or use `&.`", name, elem)
	return expr{}
}

func (f *fctx) tupleCall(n parser.Node, recv expr, name string, args []parser.Node) expr {
	tt := recv.typ.(TTuple)
	switch name {
	case "[]":
		if len(args) == 1 {
			if in, ok := args[0].(*parser.IntegerNode); ok && in.Value >= 0 && int(in.Value) < len(tt.Elems) {
				return expr{code: fmt.Sprintf("%s.F%d", recv.code, in.Value), typ: tt.Elems[in.Value]}
			}
		}
		f.errorf(n, "tuple index must be a literal in range")
	case "first":
		return expr{code: recv.code + ".F0", typ: tt.Elems[0]}
	case "last":
		return expr{code: fmt.Sprintf("%s.F%d", recv.code, len(tt.Elems)-1), typ: tt.Elems[len(tt.Elems)-1]}
	case "to_s", "inspect":
		return expr{code: recv.code + "." + goMethodName(name) + "()", typ: f.cls("String")}
	case "<=>":
		a := f.genExpr(args[0], recv.typ)
		return expr{code: recv.code + ".Cmp(" + f.coerce(args[0], a, recv.typ) + ")", typ: f.cls("Integer")}
	case "==", "!=":
		a := f.genExpr(args[0], nil)
		code := recv.code + ".Eq(" + f.coerce(args[0], a, TAny{}) + ")"
		if name == "!=" {
			code = "!" + code
		}
		return expr{code: code, typ: f.cls("Boolean")}
	}
	f.errorf(n, "undefined method %s for tuple %s", name, tt)
	return expr{}
}

// universalCall handles Kernel-level methods on values of unknown type.
func (f *fctx) universalCall(n parser.Node, recv expr, name string, args []parser.Node, block parser.Node) expr {
	if block != nil {
		f.errorf(n, "blocks on untyped receivers are not supported")
	}
	one := func(exp Type) expr {
		if len(args) != 1 {
			f.errorf(n, "%s takes one argument", name)
		}
		return f.genExpr(args[0], exp)
	}
	switch name {
	case "to_s":
		return expr{code: "rbToS(" + recv.code + ")", typ: f.cls("String")}
	case "inspect":
		return expr{code: "rbInspect(" + recv.code + ")", typ: f.cls("String")}
	case "nil?":
		return expr{code: "Boolean(any(" + recv.code + ") == nil)", typ: f.cls("Boolean")}
	case "!":
		return expr{code: "Boolean(!rbTruthy(" + recv.code + "))", typ: f.cls("Boolean")}
	case "==", "!=":
		a := one(recv.typ)
		code := "rbEq(" + recv.code + ", " + f.coerce(args[0], a, recv.typ) + ")"
		if isAny(recv.typ) || isNil(recv.typ) {
			code = "rbEq[any](" + recv.code + ", " + f.coerce(args[0], a, TAny{}) + ")"
		}
		if name == "!=" {
			code = "!" + code
		}
		return expr{code: code, typ: f.cls("Boolean")}
	case "equal?":
		a := one(nil)
		return expr{code: "Boolean(rbIdentical(" + recv.code + ", " + f.coerce(args[0], a, TAny{}) + "))", typ: f.cls("Boolean")}
	case "<=>":
		a := one(recv.typ)
		return expr{code: "rbCmp(" + recv.code + ", " + f.coerce(args[0], a, recv.typ) + ")", typ: f.cls("Integer")}
	case "hash":
		return expr{code: "rbHash(" + recv.code + ")", typ: f.cls("Integer")}
	}
	f.errorf(n, "undefined method %s for %s", name, recv.typ)
	return expr{}
}

// blockParam declares a block parameter: a fresh local that Go syntax
// declares, so it is never hoisted.
func (f *fctx) blockParam(name string, typ Type) *local {
	info := f.locals[name]
	if info == nil {
		info = &localInfo{}
		f.locals[name] = info
	}
	info.noHoist = true
	v := f.declareLocal(name, typ)
	v.declared = true
	return v
}
