package compiler

import (
	"slices"

	"github.com/danielgatis/go-ruby-prism/parser"
)

// truthyQuiet is a value's Ruby truthiness as Go, or "" when it is always true.
func (f *fctx) truthyQuiet(e expr) string {
	switch {
	case isClass(e.typ, "Boolean"):
		return "bool(" + e.code + ")"
	case isOpt(e.typ):
		return optTruthy(e.code, e.typ)
	case isAny(e.typ), unionFalsy(e.typ):
		return "rbTruthy(" + e.code + ")"
	}
	return ""
}

// unionFalsy reports a union that may hold nil or false, so that only the
// run time knows its truthiness.
func unionFalsy(t Type) bool {
	u, ok := t.(TUnion)
	return ok && slices.ContainsFunc(u.Members, func(m Type) bool { return isNil(m) || isClass(m, "Boolean") })
}

// branchFit is, for branches whose types do not join, the first of them
// every branch compiles to when it is the expected type (a literal types
// itself from what is expected); none, and the join's error stands.
func (f *fctx) branchFit(types []Type, gen func(t tail), joinErr *compileError) Type {
	for _, cand := range types {
		if isVoid(cand) {
			continue
		}
		var err *compileError
		f.probe(func() { err = f.try(func() { gen(tail{kind: tailAssign, target: "_", typ: cand}) }) })
		if err == nil {
			return cand
		}
	}
	panic(*joinErr)
}

// lift turns a statement-shaped expression (if/case/begin) into a temp.
func (f *fctx) lift(n parser.Node, expected Type, gen func(t tail)) expr {
	var types []Type
	once := f.probing() // output is discarded here, so one pass both types the branches and declares their locals (decision 240)
	probe := tail{kind: tailAssign, target: "_", types: &types, hint: expected}
	if once {
		gen(probe)
	} else {
		f.probe(func() { gen(probe) })
	}
	if f.c.round && slices.ContainsFunc(types, func(t Type) bool { return !holdsAny(t) && !isVoid(t) }) {
		// an inference round's untyped is not yet learned, as a use's is (joinUses): `@root = @root ? meld(@root, n) : n`
		// is the typed branch's, so the next round sees what it holds (decision 307)
		types = slices.DeleteFunc(types, holdsAny)
	}
	var typ Type
	err := catchCompileError(func() { typ = f.joinAll(n, types) })
	if err != nil {
		typ = f.branchFit(types, gen, err) // `c ? [2, 5] : list`: the literal takes list's Array[Integer?]
	}
	if typ == nil || isNil(typ) {
		if expected != nil && !isVoid(expected) {
			typ = expected
		} else if typ == nil {
			f.errorf(n, "cannot infer the type of this expression")
		}
	}
	if isVoid(typ) {
		if !once {
			gen(tail{})
		}
		if isNil(typ) { // every branch is nil (`if c then end`): the value is nil, not void
			return expr{code: "nil", typ: TNil{}, lit: true}
		}
		return expr{code: "", typ: TVoid{}, stmt: true}
	}
	tmp := f.newTmp()
	f.emit("var %s %s", tmp, f.c.goType(typ))
	if !once {
		gen(tail{kind: tailAssign, target: tmp, typ: typ})
	}
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
		if !ok && f.lenient() { // a gem's `dir.directory? ? dir.children : []`: the empty literal's untyped elements take the other's
			if k, ok2 := untypedContainerJoin(out, t); ok2 {
				j, ok = k, true
			}
		}
		if !ok {
			f.errorf(n, "branches have incompatible types %s and %s", out, t)
		}
		out = j
	}
	return out
}

// untypedContainerJoin is a and b when they are the same container class and one's type arguments are all untyped:
// the typed one.
func untypedContainerJoin(a, b Type) (Type, bool) {
	ca, ok1 := a.(TClass)
	cb, ok2 := b.(TClass)
	if !ok1 || !ok2 || ca.C != cb.C || len(ca.Args) == 0 || len(ca.Args) != len(cb.Args) {
		return nil, false
	}
	allAny := func(ts []Type) bool { return !slices.ContainsFunc(ts, func(t Type) bool { return !isAny(t) }) }
	switch { // the typed one, unless it is no more typed (brew's `cond ? [x.soname, x.needed] : [nil, []]`, Array[Array?])
	case allAny(cb.Args):
		if slices.ContainsFunc(ca.Args, holdsAny) {
			return b, true
		}
		return a, true
	case allAny(ca.Args):
		if slices.ContainsFunc(cb.Args, holdsAny) {
			return a, true
		}
		return b, true
	}
	return nil, false
}

// ---- operators, and/or

func (f *fctx) genOp(n parser.Node, left expr, op string, right parser.Node) expr {
	return f.genMethodCall(n, left, op, []parser.Node{right}, nil)
}

func (f *fctx) genOr(n *parser.OrNode) expr {
	l := f.genExpr(n.Left, nil)
	if isNil(l.typ) { // `nil || x` is x
		f.valueOf(l)
		return f.genExpr(n.Right, nil)
	}
	if u, ok := l.typ.(TUnion); ok {
		return f.genUnionOr(n, l, u)
	}
	lElem := stripOpt(l.typ)
	r, once := f.genOperandOnce(func() expr { return f.genExpr(n.Right, lElem) })
	boolL := isClass(l.typ, "Boolean")
	if boolL && isClass(r.e.typ, "Boolean") {
		rc, stmts := r.e, r.stmts
		if !once {
			stmts = f.capture(func() { rc = f.genExpr(n.Right, l.typ) })
		}
		if stmts == "" {
			return expr{code: "(" + l.code + " || " + rc.code + ")", typ: l.typ}
		}
		tmp := f.newTmp()
		f.emit("%s := %s", tmp, f.materialize(l))
		f.emit("if !%s {", tmp)
		f.buf.WriteString(stmts)
		f.emit("\t%s = %s", tmp, rc.code)
		f.emit("}")
		return expr{code: tmp, typ: l.typ}
	}
	untyped := isAny(lElem)
	if iv, ok := n.Left.(*parser.InstanceVariableReadNode); ok && !isOpt(l.typ) && f.scope.lookup(ivarNarrowKey(iv.Name)) != nil {
		return l // narrowed non-nil (decision 148): MRI never runs the right side either
	}
	if !isOpt(l.typ) && !untyped && !boolL {
		if f.lenient() { // a gem's defensive `x or raise(...)`: MRI never runs the right side either (decision 172)
			return l
		}
		f.errorf(n, "`||` on a non-nilable %s is always the left side", l.typ)
	}
	// Ruby: the left when it is truthy, else the right; types that share
	// nothing make an untyped result.
	var typ Type
	switch {
	case r.e.noreturn: // `x || raise(...)`
		typ = lElem
	case untyped:
		typ = TAny{}
	default:
		j, ok := join(lElem, r.e.typ)
		if !ok {
			j = TAny{}
		}
		typ = j
	}
	tmp, lt := f.newTmp(), f.newTmp()
	f.emit("var %s %s", tmp, f.c.goType(typ))
	switch {
	case untyped:
		f.emit("if %s := %s; rbTruthy(%s) {", lt, f.coerce(n, l, TAny{}), lt)
		f.emit("\t%s = %s", tmp, lt)
	case boolL:
		f.emit("if %s := %s; %s {", lt, f.materialize(l), lt)
		f.emit("\t%s = %s", tmp, f.coerce(n, expr{code: lt, typ: l.typ}, typ))
	default:
		f.emit("if %s := %s; %s {", lt, l.code, optTruthy(lt, l.typ))
		f.emit("\t%s = %s", tmp, f.coerce(n, expr{code: "(*" + lt + ")", typ: lElem}, typ))
	}
	f.emit("} else {")
	f.indent++
	re := f.operandAgain(r, once, func() expr { return f.genExpr(n.Right, lElem) })
	if re.noreturn {
		f.emit("%s", re.code)
	} else {
		f.emit("%s = %s", tmp, f.coerce(n, re, typ))
	}
	f.indent--
	f.emit("}")
	return expr{code: tmp, typ: typ}
}

// genUnionOr is `l || r` on a union l (decision 150): l when truthy (its
// members but nil), else r, typed as their join.
func (f *fctx) genUnionOr(n *parser.OrNode, l expr, u TUnion) expr {
	if !unionFalsy(u) {
		if f.lenient() {
			return l
		}
		f.errorf(n, "`||` on %s, which is never nil or false, is always the left side", u)
	}
	lElem := unionOf(slices.DeleteFunc(slices.Clone(u.Members), isNil)...)
	r, once := f.genOperandOnce(func() expr { return f.genExpr(n.Right, lElem) })
	typ := lElem
	if !r.e.noreturn {
		typ = joinUnion(lElem, r.e.typ)
	}
	tmp, lt := f.newTmp(), f.newTmp()
	f.emit("var %s %s", tmp, f.c.goType(typ))
	f.emit("if %s := %s; rbTruthy(%s) {", lt, l.code, lt)
	f.emit("\t%s = %s", tmp, f.coerce(n, expr{code: lt, typ: TAny{}}, typ)) // a member, so the check passes
	f.emit("} else {")
	f.indent++
	re := f.operandAgain(r, once, func() expr { return f.genExpr(n.Right, lElem) })
	if re.noreturn {
		f.emit("%s", re.code)
	} else {
		f.emit("%s = %s", tmp, f.coerce(n, re, typ))
	}
	f.indent--
	f.emit("}")
	return expr{code: tmp, typ: typ}
}

func (f *fctx) genAnd(n *parser.AndNode) expr {
	l := f.genExpr(n.Left, nil)
	narrow := f.leftNarrowing(n.Left, l)
	r, once := f.genOperandOnce(func() expr {
		f.push()
		f.applyNarrow(narrow)
		e := f.genExpr(n.Right, nil)
		f.pop()
		return e
	})
	boolL := isClass(l.typ, "Boolean")
	if boolL && isClass(r.e.typ, "Boolean") {
		rc, stmts := r.e, r.stmts
		if !once || len(narrow) > 0 { // the unnarrowed generation is the one emitted
			stmts = f.capture(func() { rc = f.genExpr(n.Right, nil) })
		}
		if stmts == "" {
			return expr{code: "(" + l.code + " && " + rc.code + ")", typ: l.typ}
		}
		tmp := f.newTmp()
		f.emit("%s := %s", tmp, f.materialize(l))
		f.emit("if %s {", tmp)
		f.buf.WriteString(stmts)
		f.emit("\t%s = %s", tmp, rc.code)
		f.emit("}")
		return expr{code: tmp, typ: l.typ}
	}
	if !boolL && !isOpt(l.typ) && !isAny(l.typ) && !isNil(l.typ) && !unionFalsy(l.typ) {
		// the left is never nil or false: the value is the right
		f.discard(l)
		return f.operandAgain(r, once && len(narrow) == 0, func() expr { return f.genExpr(n.Right, nil) })
	}
	// Ruby: the left when it is falsy (nil or false), else the right.
	// A Boolean? left may be false, not just nil: it is a possible value.
	optBool := isOpt(l.typ) && isClass(stripOpt(l.typ), "Boolean")
	var typ Type = TAny{}
	if (isOpt(l.typ) || isNil(l.typ)) && !isAny(stripOpt(l.typ)) {
		lf := Type(TNil{})
		if optBool {
			lf = l.typ
		}
		if j, ok := join(lf, r.e.typ); ok {
			typ = j
		}
	}
	unionL := isUnion(l.typ)
	if unionL { // the left's falsy members (nil, false) join the right (decision 150)
		lf := unionOf(slices.DeleteFunc(slices.Clone(l.typ.(TUnion).Members), func(m Type) bool { return !isNil(m) && !isClass(m, "Boolean") })...)
		typ = joinUnion(lf, r.e.typ)
	}
	tmp, lt := f.newTmp(), f.newTmp()
	f.emit("var %s %s", tmp, f.c.goType(typ))
	if isNil(l.typ) { // `nil && x`: x is dead, but still compiled (like `if nil`)
		lt = f.valueOf(l).code
		f.emit("if false {")
	} else {
		f.emit("if %s := %s; %s {", lt, f.materialize(l), f.truthy(n.Left, expr{code: lt, typ: l.typ}))
	}
	f.indent++
	f.push()
	for _, nw := range narrow {
		if isOpt(l.typ) {
			nw.code = "(*" + lt + ")"
		}
		f.applyNarrow([]narrowInfo{nw})
	}
	re := f.operandAgain(r, once, func() expr { return f.genExpr(n.Right, nil) })
	f.pop()
	f.emit("%s = %s", tmp, f.coerce(n, re, typ))
	f.indent--
	switch {
	case unionL: // nil or false: checked into the result type, which holds both
		f.emit("} else {")
		f.emit("\t%s = %s", tmp, f.coerce(n, expr{code: lt, typ: TAny{}}, typ))
	case isAny(typ) || optBool:
		f.emit("} else {")
		f.emit("\t%s = %s", tmp, f.coerce(n, expr{code: lt, typ: l.typ}, typ))
	}
	f.emit("}")
	return expr{code: tmp, typ: typ}
}

// operand is an expression generated ahead of its emission: its value and the statements it emitted.
type operand struct {
	e     expr
	stmts string
}

// genOperandOnce generates an operand of `||`, `&&`, `&.` or a nil receiver's call to learn its type. Outside a probe that is a dry run, and
// operandAgain generates it for real. Inside one the output is discarded anyway, so it is generated once, here,
// and operandAgain replays it: a dry run per level doubles the work of every nested level (decision 240).
func (f *fctx) genOperandOnce(gen func() expr) (operand, bool) {
	var o operand
	if f.probing() {
		o.stmts = f.capture(func() { o.e = gen() })
		return o, true
	}
	f.probe(func() { o.e = gen() })
	return o, false
}

// operandAgain is the operand for real: generated now, or the one genOperandOnce kept, its statements emitted here.
func (f *fctx) operandAgain(o operand, once bool, gen func() expr) expr {
	if !once {
		return gen()
	}
	f.buf.WriteString(o.stmts)
	return o.e
}

// leftNarrowing is what `x && ...` proves about a nilable local x.
func (f *fctx) leftNarrowing(n parser.Node, l expr) []narrowInfo {
	lv, ok := n.(*parser.LocalVariableReadNode)
	if ok && isUnion(l.typ) {
		if nw, ok := f.narrowUnion(f.scope.lookup(lv.Name), func(m Type) bool { return !isNil(m) }); ok {
			return []narrowInfo{nw}
		}
		return nil
	}
	if !ok || !isOpt(l.typ) || isAny(stripOpt(l.typ)) {
		return nil
	}
	v := f.scope.lookup(lv.Name)
	if v == nil {
		return nil
	}
	return []narrowInfo{{local: v, typ: stripOpt(l.typ)}}
}
