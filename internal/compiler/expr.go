package compiler

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"

	"github.com/danielgatis/go-ruby-prism/parser"

	"rb2go/internal/rbs"
)

// expr is a generated Go expression with its Ruby type.
type expr struct {
	code     string
	typ      Type
	classObj bool // exactly a class constant: its Go type is the concrete metaclass
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
		code := strings.ReplaceAll(f.f.text(n.Location), "_", "")
		if digits := strings.TrimPrefix(code, "-"); len(digits) > 1 && (digits[1] == 'd' || digits[1] == 'D') {
			// Ruby's 0d decimal prefix has no Go spelling; 0x/0o/0b/0 carry over.
			code = code[:len(code)-len(digits)] + strings.TrimLeft(digits[2:], "0")
			if strings.TrimPrefix(code, "-") == "" {
				code += "0"
			}
		}
		return expr{code: code, typ: f.cls("Integer"), lit: true}, true
	case *parser.FloatNode:
		if n.Value == 0 && math.Signbit(n.Value) {
			// Go's constant -0.0 is +0; Ruby's is negative zero.
			return expr{code: "Float(math.Copysign(0, -1))", typ: f.cls("Float")}, true
		}
		return expr{code: f.f.text(n.Location), typ: f.cls("Float"), lit: true}, true
	case *parser.TrueNode:
		return expr{code: "true", typ: f.cls("Boolean"), lit: true}, true
	case *parser.FalseNode:
		return expr{code: "false", typ: f.cls("Boolean"), lit: true}, true
	case *parser.NilNode:
		return expr{code: "nil", typ: TNil{}}, true
	case *parser.SelfNode:
		return expr{code: f.selfCode, typ: f.selfType}, true
	case *parser.SymbolNode:
		return expr{code: "Symbol(" + strconv.Quote(n.Unescaped.Value) + ")", typ: f.cls("Symbol")}, true
	case *withMember:
		return f.genMethodCall(n, n.recv, n.name, nil, nil), true
	case *exprNode:
		return n.e, true
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
		if t := f.f.trailingAnnotation(n); t != "" {
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
		return f.genHash(n, n.Elements, expected)
	case *parser.RegularExpressionNode, *parser.InterpolatedRegularExpressionNode:
		return f.genRegexp(n)
	case *parser.MatchWriteNode:
		f.errorf(n, "named captures assigned to locals (/(?<x>..)/ =~ s) are not supported; use match")
	case *parser.LocalVariableOrWriteNode:
		return f.genOrAssignLocal(n)
	case *parser.InstanceVariableOrWriteNode:
		return f.genOrAssignIvar(n)
	case *parser.MultiWriteNode:
		return f.genMultiWrite(n)
	case *parser.KeywordHashNode:
		// `f(a: 1)` on a method without keyword params passes a Hash.
		return f.genHash(n, n.Elements, expected)
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
	case *parser.ConstantReadNode, *parser.ConstantPathNode:
		return f.genConstRead(n)
	case *parser.RescueModifierNode:
		return f.genRescueModifier(n, expected)
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
		var exp, ann Type
		if t := f.f.trailingAnnotation(n); t != "" {
			ann = f.parseTypeAnn(n, t)
			exp = ann
		} else if iv := f.c.findIvar(f.owner, n.Name); iv != nil {
			exp = iv.Type
		}
		val := f.genExpr(n.Value, exp)
		if ann != nil {
			val.typ = ann // `@x = [] #: Array[T]` declares the ivar's type
		}
		f.unnarrow("attr:" + strings.TrimPrefix(n.Name, "@"))
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
	return f.c.resolveType(t, typeScope{class: f.owner, lex: f.lex, methodTPs: tps, file: f.f, line: f.f.line(n.GetLocation().StartOffset)})
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
	lElem := stripOpt(l.typ)
	var r expr
	f.probe(func() { r = f.genExpr(n.Right, lElem) })
	boolL := isClass(l.typ, "Boolean")
	if boolL && isClass(r.typ, "Boolean") {
		var rc expr
		stmts := f.capture(func() { rc = f.genExpr(n.Right, l.typ) })
		if stmts == "" {
			return expr{code: "(" + l.code + " || " + rc.code + ")", typ: l.typ}
		}
		tmp := f.newTmp()
		f.emit("%s := %s", tmp, l.code)
		f.emit("if !%s {", tmp)
		f.buf.WriteString(stmts)
		f.emit("\t%s = %s", tmp, rc.code)
		f.emit("}")
		return expr{code: tmp, typ: l.typ}
	}
	untyped := isAny(lElem)
	if !isOpt(l.typ) && !untyped && !boolL {
		f.errorf(n, "`||` on a non-nilable %s is always the left side", l.typ)
	}
	// Ruby: the left when it is truthy, else the right; types that share
	// nothing make an untyped result.
	var typ Type
	switch {
	case r.noreturn: // `x || raise(...)`
		typ = lElem
	case untyped:
		typ = TAny{}
	default:
		j, ok := join(lElem, r.typ)
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
		f.emit("if %s := %s; %s {", lt, l.code, lt)
		f.emit("\t%s = %s", tmp, f.coerce(n, expr{code: lt, typ: l.typ}, typ))
	default:
		f.emit("if %s := %s; %s != nil {", lt, l.code, lt)
		f.emit("\t%s = %s", tmp, f.coerce(n, expr{code: "(*" + lt + ")", typ: lElem}, typ))
	}
	f.emit("} else {")
	f.indent++
	r = f.genExpr(n.Right, lElem)
	if r.noreturn {
		f.emit("%s", r.code)
	} else {
		f.emit("%s = %s", tmp, f.coerce(n, r, typ))
	}
	f.indent--
	f.emit("}")
	return expr{code: tmp, typ: typ}
}
func (f *fctx) genAnd(n *parser.AndNode) expr {
	l := f.genExpr(n.Left, nil)
	narrow := f.leftNarrowing(n.Left, l)
	var r expr
	f.probe(func() {
		f.push()
		f.applyNarrow(narrow)
		r = f.genExpr(n.Right, nil)
		f.pop()
	})
	boolL := isClass(l.typ, "Boolean")
	if boolL && isClass(r.typ, "Boolean") {
		var rc expr
		stmts := f.capture(func() { rc = f.genExpr(n.Right, nil) })
		if stmts == "" {
			return expr{code: "(" + l.code + " && " + rc.code + ")", typ: l.typ}
		}
		tmp := f.newTmp()
		f.emit("%s := %s", tmp, l.code)
		f.emit("if %s {", tmp)
		f.buf.WriteString(stmts)
		f.emit("\t%s = %s", tmp, rc.code)
		f.emit("}")
		return expr{code: tmp, typ: l.typ}
	}
	if !boolL && !isOpt(l.typ) && !isAny(l.typ) {
		// the left is never nil or false: the value is the right
		if !isSimpleGo(l.code) {
			f.emit("_ = %s", l.code)
		}
		return f.genExpr(n.Right, nil)
	}
	// Ruby: the left when it is falsy (nil or false), else the right.
	var typ Type = TAny{}
	if isOpt(l.typ) && !isAny(stripOpt(l.typ)) {
		if j, ok := join(TNil{}, r.typ); ok {
			typ = j
		}
	}
	tmp, lt := f.newTmp(), f.newTmp()
	f.emit("var %s %s", tmp, f.c.goType(typ))
	f.emit("if %s := %s; %s {", lt, l.code, f.truthy(n.Left, expr{code: lt, typ: l.typ}))
	f.indent++
	f.push()
	for _, nw := range narrow {
		nw.code = "(*" + lt + ")"
		f.applyNarrow([]narrowInfo{nw})
	}
	r = f.genExpr(n.Right, nil)
	f.pop()
	f.emit("%s = %s", tmp, f.coerce(n, r, typ))
	f.indent--
	if isAny(typ) {
		f.emit("} else {")
		f.emit("\t%s = %s", tmp, f.coerce(n, expr{code: lt, typ: l.typ}, TAny{}))
	}
	f.emit("}")
	return expr{code: tmp, typ: typ}
}

// leftNarrowing is what `x && ...` proves about a nilable local x.
func (f *fctx) leftNarrowing(n parser.Node, l expr) []narrowInfo {
	lv, ok := n.(*parser.LocalVariableReadNode)
	if !ok || !isOpt(l.typ) || isAny(stripOpt(l.typ)) {
		return nil
	}
	v := f.scope.lookup(lv.Name)
	if v == nil {
		return nil
	}
	return []narrowInfo{{local: v, typ: stripOpt(l.typ)}}
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
	if len(parts) == 1 {
		return expr{code: parts[0], typ: f.cls("String")}
	}
	return expr{code: "(" + strings.Join(parts, " + ") + ")", typ: f.cls("String")}
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
		elemT = inferElemType(elems)
		// [Integer, String] with nothing expected is a tuple (a sort key, a
		// multiple-return); anywhere untyped is expected it is an Array.
		if isAny(elemT) && expected == nil && len(elems) > 0 {
			if t, ok := f.tupleLiteral(n, elems); ok {
				return t
			}
		}
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
func inferElemType(elems []expr) Type {
	ts := make([]Type, len(elems))
	for i, e := range elems {
		ts[i] = e.typ
	}
	return joinOrAny(ts)
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
		return expr{}, false
	}
	for _, e := range elems {
		if isNil(e.typ) || isAny(e.typ) {
			return expr{}, false
		}
	}
	ts := make([]Type, len(elems))
	codes := make([]string, len(elems))
	for i, e := range elems {
		ts[i] = e.typ
		codes[i] = f.coerce(n, e, e.typ)
	}
	tt := TTuple{Elems: ts}
	return expr{code: f.c.goType(tt) + "{" + strings.Join(codes, ", ") + "}", typ: tt}, true
}

func (f *fctx) genHash(n parser.Node, elements []parser.Node, expected Type) expr {
	var kT, vT Type
	if ec, ok := expected.(TClass); ok && ec.C.RubyName == "Hash" {
		kT, vT = ec.Args[0], ec.Args[1]
	}
	type kv struct{ k, v expr }
	pairs := make([]kv, 0, len(elements))
	ks, vs := make([]Type, 0, len(elements)), make([]Type, 0, len(elements))
	for _, el := range elements {
		a, ok := el.(*parser.AssocNode)
		if !ok {
			f.c.unsupported(f.f, el)
		}
		k := f.genExpr(a.Key, kT)
		v := f.genExpr(a.Value, vT)
		ks, vs = append(ks, k.typ), append(vs, v.typ)
		pairs = append(pairs, kv{k, v})
	}
	if kT == nil {
		kT, vT = joinOrAny(ks), joinOrAny(vs)
	}
	var b strings.Builder
	fmt.Fprintf(&b, "NewHash[%s, %s]()", f.c.goType(kT), f.c.goType(vT))
	for _, p := range pairs {
		fmt.Fprintf(&b, ".__Set(%s, %s)", f.coerce(n, p.k, kT), f.coerce(n, p.v, vT))
	}
	return expr{code: b.String(), typ: TClass{C: f.c.classes["Hash"], Args: []Type{kT, vT}}}
}

// joinOrAny is the element type of an unannotated literal: the join of its
// parts, or untyped when they have none in common (Ruby's literals are
// heterogeneous; RBS would say `untyped` too).
func joinOrAny(ts []Type) Type {
	var out Type
	for _, t := range ts {
		if out == nil {
			out = t
			continue
		}
		j, ok := join(out, t)
		if !ok {
			return TAny{}
		}
		out = j
	}
	if out == nil || isNil(out) || isVoid(out) {
		return TAny{}
	}
	return out
}

// ---- coercion

// valueOf runs a nil-typed call as a statement: `-> nil` methods have no Go result (isVoid).
func (f *fctx) valueOf(e expr) expr {
	if !isNil(e.typ) || e.done {
		return e
	}
	c := e.code
	for strings.HasPrefix(c, "(") && strings.HasSuffix(c, ")") {
		c = c[1 : len(c)-1]
	}
	if !strings.HasSuffix(c, ")") { // nil itself, or a temp holding it
		return e
	}
	f.emit("%s", c)
	return expr{code: "nil", typ: TNil{}}
}

// coerce converts e to the representation of type `to`.
func (f *fctx) coerce(n parser.Node, e expr, to Type) string {
	if to == nil || isVoid(to) && !isNil(to) {
		return e.code
	}
	e = f.valueOf(e)
	if typeEq(e.typ, to) {
		return e.code
	}
	// `untyped?` is just untyped: its nil is Ruby's nil.
	if o, ok := e.typ.(TOpt); ok && isAny(o.Elem) {
		e = expr{code: "Opt(" + e.code + ")", typ: TAny{}}
		if isAny(to) {
			return e.code
		}
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
		if isAny(e.typ) && to.C.RubyName == "Boolean" {
			return "Boolean(rbTruthy(" + e.code + "))" // Ruby conditions test truthiness
		}
		if isAny(e.typ) {
			return e.code + ".(" + f.c.goType(to) + ")"
		}

		if isOpt(e.typ) {
			f.errorf(n, "possibly-nil %s where %s is expected; check it first (`if x`, `x ||= ...`, `return unless x`)", e.typ, to)
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
	if n.Name == "call" && f.isBlockParam(n.Receiver) && n.Block == nil {
		return f.yieldValues(n, callArgs(n))
	}
	if n.Receiver == nil && n.Arguments == nil && n.Block == nil {
		if v := f.scope.lookup("attr:" + n.Name); v != nil {
			return expr{code: v.goName, typ: v.typ} // narrowed attribute read
		}
	}
	if strings.HasSuffix(n.Name, "=") && (n.Receiver == nil || isSelf(n.Receiver)) {
		f.unnarrow("attr:" + strings.TrimSuffix(n.Name, "="))
	}
	if cls := f.classRef(n.Receiver); cls != nil {
		// Foo.new is a direct constructor call unless Foo defines self.new.
		if n.Name == "new" && (cls.meta == nil || isSynthNew(cls.meta.lookup("new"))) {
			if n.Block != nil {
				f.errorf(n, "%s.new with a block is not supported", cls.RubyName)
			}
			return f.genNew(n, cls, callArgs(n), nil, expected)
		}
		if cls.meta == nil {
			f.errorf(n, "%s has no class methods", cls.RubyName)
		}
		recv := expr{code: classVar(cls), typ: TClass{C: cls.meta}, classObj: true}
		return f.genMethodCall(n, recv, n.Name, callArgs(n), n.Block)
	}
	if n.IsSAFE_NAVIGATION() {
		return f.genSafeNav(n)
	}
	var recv expr
	if n.Receiver == nil {
		recv = expr{code: f.selfCode, typ: f.selfType}
	} else {
		recv = f.valueOf(f.genExpr(n.Receiver, nil))
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
	if e, ok := f.genIntrinsic(n, recv, name, args, block); ok {
		return e
	}
	return f.dispatch(n, recv, name, args, block)
}

// genIntrinsic compiles the methods the transpiler answers itself:
// class, with (Data), respond_to?, is_a?/kind_of?.
func (f *fctx) genIntrinsic(n parser.Node, recv expr, name string, args []parser.Node, block parser.Node) (expr, bool) {
	if name == "class" && len(args) == 0 && block == nil {
		if e, ok := f.genClassOf(recv); ok {
			return e, true
		}
	}
	if name == "with" && block == nil {
		if e, ok := f.genDataWith(n, recv, args); ok {
			return e, true
		}
	}
	if name == "respond_to?" && block == nil && len(args) >= 1 {
		if e, ok := f.genRespondTo(n, recv, args); ok {
			return e, true
		}
		return f.genDynRespondTo(n, recv, args), true
	}
	if (name == "send" || name == "__send__" || name == "public_send") && len(args) >= 1 {
		return f.genSend(n, recv, name, args, block), true
	}
	if name == "is_a?" || name == "kind_of?" {
		if len(args) != 1 || block != nil {
			f.errorf(n, "%s takes one class", name)
		}
		return expr{code: "Boolean(" + f.isACheck(n, recv, args[0]) + ")", typ: f.cls("Boolean")}, true
	}
	return expr{}, false
}

// dispatch resolves a call by the receiver's static type.
func (f *fctx) dispatch(n parser.Node, recv expr, name string, args []parser.Node, block parser.Node) expr {
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
			if mm := t.C.lookup("method_missing"); mm != nil {
				return f.callMissing(n, mm, recv, name, args, block)
			}
			// a Module/Class-typed value is some class object, and a
			// struct-typed value may be a subclass defining the method:
			// either way the method is found at run time
			if (t.C.RubyName == "Module" || t.C.RubyName == "Class" || (t.C.isStruct() && t.C.descendantDefines(name))) && block == nil {
				return f.genDynCall(n, recv, name, args)
			}
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
			if mm := f.owner.lookup("method_missing"); mm != nil {
				return f.callMissing(n, mm, recv, name, args, block)
			}
			if f.owner.isStruct() && f.owner.descendantDefines(name) && block == nil {
				return f.genDynCall(n, recv, name, args)
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
			// The default is a node of the def's file (often the prelude):
			// literal text and error positions must come from there.
			caller := f.f
			f.f = m.File
			d := f.genExpr(p.Default, closed(p.Type, env))
			f.f = caller
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
	if m.Private && recv.code != f.selfCode && !f.implicitCall {
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
	if m.Name == "const_get" && m.Owner.RubyName == "Module" {
		if t := f.constGetType(recv, args); t != nil {
			code, ret = code+".("+f.c.goType(t)+")", t
		}
	}
	// `klass.new` returns the hierarchy's root type; narrow to the class the
	// receiver is statically known to be.
	if m.Kind == kindSynth && m.Name == "new" {
		if meta := f.metaOfType(recv.typ); meta != nil && meta.metaOf != meta.metaOf.root() {
			ret = TClass{C: meta.metaOf}
			code += ".(" + f.c.goType(ret) + ")"
		}
	}
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
	if m.Owner.metaOf != nil && m.Name == "new" && !recv.classObj {
		// `new` on a class object of unknown exact class: assert for it.
		ps, ret := f.c.sig(m, env)
		return "any(" + recv.code + ").(interface{ New(" + ps + ") " + ret + " }).New(" + argList + ")"
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
		if f.isBlockParam(b.Expression) {
			return f.forwardClosure(b, sig, env)
		}
		sym, ok := b.Expression.(*parser.SymbolNode)
		if !ok {
			f.errorf(b, "only &:symbol and a method's own &block are supported as block arguments")
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
			saved, savedRuby := f.enterRubyBlock()
			f.closures++
			f.loops = append(f.loops, loopClosure)
			_, pro := f.bindBlockParams(n, names, params)
			pro()
			f.withNextTail(tail{kind: tailReturn, types: &types}, gen)
			f.loops = f.loops[:len(f.loops)-1]
			f.closures--
			f.leaveRubyBlock(saved, savedRuby)
		})
		got := f.joinAll(n, types)
		if got == nil || isNil(got) {
			got = TNil{}
			if _, ok := sig.Ret.(TVar); ok {
				got = TAny{} // Go has no nil type
			}
		}
		if !unify(sig.Ret, got, env) {
			f.errorf(n, "block returns %s, expected %s", got, subst(sig.Ret, env))
		}
		ret = subst(sig.Ret, env)
	}
	var b strings.Builder
	savedBuf := f.buf
	f.buf = &b
	saved, savedRuby := f.enterRubyBlock()
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
		f.withNextTail(tail{}, gen)
	} else {
		f.withNextTail(tail{kind: tailReturn, typ: ret}, gen)
	}
	f.indent--
	f.emit("}")
	f.loops = f.loops[:len(f.loops)-1]
	f.closures--
	f.leaveRubyBlock(saved, savedRuby)
	f.buf = savedBuf
	code := strings.TrimSpace(b.String())
	// re-indent: the closure is embedded in an expression on the current line
	return code
}

// withNextTail runs gen(t) with t as the tail a bare `next` returns through.
func (f *fctx) withNextTail(t tail, gen func(tail)) {
	saved := f.nextTail
	f.nextTail = t
	gen(t)
	f.nextTail = saved
}

// genIterCall emits `for ... range recv.Each(...) { body }` for iterator
// methods called with a block. Returns false if the call is not one.
func (f *fctx) genIterCall(n *parser.CallNode, t tail) bool {
	if f.classRef(n.Receiver) != nil {
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
	call := f.callCode(e, recv, codes, env)
	blk, ok := n.Block.(*parser.BlockNode)
	if !ok {
		f.forwardIter(n, call, yields, t)
		return true
	}
	names := f.blockParamNames(blk.Parameters)
	saved, savedRuby := f.enterRubyBlock()
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
	f.leaveRubyBlock(saved, savedRuby)
	f.emit("}")
	if t.kind != tailNone {
		f.emptyTail(n, t)
	}
	return true
}

// ---- yield / super / new / raise

func (f *fctx) genYield(n *parser.YieldNode) expr {
	args := []parser.Node{}
	if n.Arguments != nil {
		args = n.Arguments.Arguments
	}
	return f.yieldValues(n, args)
}

// yieldValues is `yield args` and `block.call(args)`.
func (f *fctx) yieldValues(n parser.Node, args []parser.Node) expr {
	if f.blockSig == nil {
		f.errorf(n, "yield in a method whose signature has no block")
	}
	if f.closures > 0 {
		f.errorf(n, "yield inside a non-iterator block is not supported")
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
	if vr := cls.valueRoot(); vr != nil && exprs == nil {
		args = f.keywordMembers(n, vr, args)
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
		if cls := f.classRef(args[0]); cls != nil {
			if !cls.isSubclassOf(exc) {
				f.errorf(args[0], "%s is not an exception class", cls.RubyName)
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
		cls := f.classRef(args[0])
		if cls == nil {
			f.errorf(args[0], "raise Class, message: first argument must be a class")
		}
		if !cls.isSubclassOf(exc) {
			f.errorf(args[0], "%s is not an exception class", cls.RubyName)
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
	case "to_json":
		return expr{code: "rbToJson(Opt(" + recv.code + "))", typ: f.cls("String")}
	case "==", "!=", "equal?":
		if len(args) != 1 {
			f.errorf(n, "%s takes one argument", name)
		}
		a := f.genExpr(args[0], nil)
		code := "rbEq[any](Opt(" + recv.code + "), " + f.coerce(args[0], a, TAny{}) + ")"
		if name == "!=" {
			code = "!" + code
		}
		return expr{code: code, typ: f.cls("Boolean")}
	case "!":
		return expr{code: "Boolean(" + recv.code + " == nil)", typ: f.cls("Boolean")}
	}
	// Ruby raises NoMethodError when the value is nil; so does this.
	f.c.warn(f.f, n, "%s called on possibly-nil %s (raises NoMethodError on nil)", name, elem)
	code := recv.code
	if !isSimpleGo(code) {
		code = f.newTmp()
		f.emit("%s := %s", code, recv.code)
	}
	f.emit("if %s == nil {", code)
	f.emit("\tpanic(rbNoMethod(%q, nil, false))", name)
	f.emit("}")
	return f.genMethodCall(n, expr{code: "(*" + code + ")", typ: elem}, name, args, nil)
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
	case "to_s", "inspect", "to_json":
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
	case "to_json":
		return expr{code: "rbToJson(" + recv.code + ")", typ: f.cls("String")}
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
	return f.genDynCall(n, recv, name, args)
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

// classRef resolves n to a class or module when it is a constant naming one.
func (f *fctx) classRef(n parser.Node) *Class {
	switch n.(type) {
	case *parser.ConstantReadNode, *parser.ConstantPathNode:
		cls, _ := f.c.lookupConst(f.f, n, f.lex)
		return cls
	}
	return nil
}

// genConstRead reads a constant: a Go package variable.
func (f *fctx) genConstRead(n parser.Node) expr {
	cls, k := f.c.lookupConst(f.f, n, f.lex)
	switch {
	case k != nil:
		return expr{code: k.GoName, typ: f.c.constType(k)}
	case cls != nil && cls.meta != nil:
		return expr{code: classVar(cls), typ: TClass{C: cls.meta}, classObj: true}
	case cls != nil:
		f.errorf(n, "class %s used as a value is not supported", cls.RubyName)
	}
	f.errorf(n, "uninitialized constant %s", f.f.text(n.GetLocation()))
	return expr{}
}

// metaOfType returns the metaclass a receiver type denotes, if any.
func (f *fctx) metaOfType(t Type) *Class {
	switch t := t.(type) {
	case TClass:
		if t.C.metaOf != nil {
			return t.C
		}
	case TVar:
		if t.Name == "Self" && f.owner != nil && f.owner.metaOf != nil {
			return f.owner
		}
	}
	return nil
}

// genClassOf renders `x.class` for an instance of a class with a metaclass.
func (f *fctx) genClassOf(recv expr) (expr, bool) {
	var cls *Class
	switch t := recv.typ.(type) {
	case TClass:
		cls = t.C
	case TVar:
		if t.Name == "Self" {
			cls = f.owner
		}
	}
	if cls != nil && cls.metaOf != nil {
		// the class of a class object is Class; of a module, Module
		k := f.c.classes["Class"]
		if cls.metaOf.IsModule {
			k = f.c.classes["Module"]
		}
		return expr{code: classVar(k), typ: TClass{C: k.meta}, classObj: true}, true
	}
	if cls == nil || cls.meta == nil {
		return expr{}, false
	}
	if !cls.isStruct() {
		return expr{code: classVar(cls), typ: TClass{C: cls.meta}}, true
	}
	code := recv.code + "._ClassOf()"
	if cls != cls.root() {
		code += ".(" + f.c.goType(TClass{C: cls.meta}) + ")"
	}
	return expr{code: code, typ: TClass{C: cls.meta}}, true
}

func isIsA(n *parser.CallNode) bool {
	return (n.Name == "is_a?" || n.Name == "kind_of?") && n.Arguments != nil && len(n.Arguments.Arguments) == 1
}

// isACheck renders `recv.is_a?(C)` as a Go bool: a constant when the static
// type decides it, otherwise a type assertion.
func (f *fctx) isACheck(n parser.Node, recv expr, classNode parser.Node) string {
	cls := f.classRef(classNode)
	if cls == nil {
		f.errorf(classNode, "is_a? needs a class name")
	}
	if !isSimpleGo(recv.code) {
		tmp := f.newTmp()
		f.emit("%s := %s", tmp, recv.code)
		recv.code = tmp
	}
	t := recv.typ
	if o, ok := t.(TOpt); ok {
		inner := f.isACheck(n, expr{code: "(*" + recv.code + ")", typ: o.Elem}, classNode)
		return "(" + recv.code + " != nil && " + inner + ")"
	}
	if isNil(t) {
		return strconv.FormatBool(cls.universal)
	}
	if v, ok := t.(TVar); ok && v.Name == "Self" && f.owner != nil {
		t = TClass{C: f.owner}
	}
	switch t := t.(type) {
	case TClass:
		switch {
		case cls.universal || t.C.isSubclassOf(cls):
			return "true"
		case cls.isStruct() && t.C.isStruct() && cls.isSubclassOf(t.C):
			return "rbIsA[" + f.c.goType(TClass{C: cls}) + "](" + recv.code + ")"
		case cls.IsModule && t.C.isStruct():
			f.errorf(n, "is_a?(%s) on %s cannot be checked: rb2go has no runtime record of included modules", cls.RubyName, t)
		}
		return "false"
	case TTuple:
		return strconv.FormatBool(cls.universal || cls.RubyName == "Array")
	case TAny:
		if cls.universal {
			return "true"
		}
		return "rbIsA[" + f.isAGoType(n, cls) + "](" + recv.code + ")"
	}
	f.errorf(n, "is_a? on %s is not supported", t)
	return ""
}

// isAGoType is the Go type an untyped value is asserted to for is_a?(cls).
func (f *fctx) isAGoType(n parser.Node, cls *Class) string {
	switch {
	case cls.IsModule:
		f.errorf(n, "is_a?(%s) on untyped cannot be checked: rb2go has no runtime record of included modules", cls.RubyName)
	case len(cls.TypeParams) > 0:
		return cls.Name + "_Any"
	}
	return f.c.goType(TClass{C: cls})
}

// narrowIsA renders `x.is_a?(C)` in a condition and narrows x to C inside.
func (f *fctx) narrowIsA(call *parser.CallNode, v *local) (string, []narrowInfo) {
	recv := expr{code: v.goName, typ: v.typ}
	cond := f.isACheck(call, recv, call.Arguments.Arguments[0])
	cls := f.classRef(call.Arguments.Arguments[0])
	base := stripOpt(v.typ)
	code := v.goName
	if isOpt(v.typ) {
		code = "(*" + v.goName + ")"
	}
	if cond == "true" || cond == "false" || cls.IsModule || cls.universal {
		return cond, nil
	}
	if bt, ok := base.(TClass); ok && bt.C.isSubclassOf(cls) {
		return cond, nil
	}
	typ := TClass{C: cls}
	if len(cls.TypeParams) > 0 {
		for range cls.TypeParams {
			typ.Args = append(typ.Args, TAny{})
		}
		return cond, []narrowInfo{{local: v, typ: typ, code: code + ".(" + cls.Name + "_Any)._ToAny()"}}
	}
	return cond, []narrowInfo{{local: v, typ: typ, code: code + ".(" + f.c.goType(typ) + ")"}}
}

var simpleGo = regexp.MustCompile(`^(?:[A-Za-z_][A-Za-z0-9_]*|\(\*[A-Za-z_][A-Za-z0-9_]*\)|[0-9.]+|"(?:[^"\\]|\\.)*")$`)

// isSimpleGo reports whether code can be evaluated twice without effects.
func isSimpleGo(code string) bool { return simpleGo.MatchString(code) }

// genOrAssign renders `x ||= value` for a target whose current value is cur.
// The value is evaluated only when x is nil (or falsy, for untyped/bool).
func (f *fctx) genOrAssign(n parser.Node, cur expr, value parser.Node) expr {
	elem := stripOpt(cur.typ)
	var cond string
	var want Type
	switch {
	case isAny(elem):
		cond, want = "!"+f.truthy(n, cur), TAny{}
	case isOpt(cur.typ):
		cond, want = cur.code+" == nil", cur.typ
	case isClass(cur.typ, "Boolean"):
		cond, want = "!"+cur.code, cur.typ
	default:
		// never nil or false: Ruby leaves it alone and skips the value
		return expr{code: cur.code, typ: cur.typ, done: true}
	}
	f.emit("if %s {", cond)
	f.indent++
	saved := f.enterBlock()
	v := f.genExpr(value, elem)
	f.emit("%s = %s", cur.code, f.coerce(value, v, want))
	f.leaveBlock(saved)
	f.indent--
	f.emit("}")
	if isOpt(cur.typ) && !isAny(elem) && !isOpt(v.typ) && !isNil(v.typ) {
		return expr{code: "(*" + cur.code + ")", typ: elem, done: true}
	}
	return expr{code: cur.code, typ: cur.typ, done: true}
}

func (f *fctx) genOrAssignLocal(n *parser.LocalVariableOrWriteNode) expr {
	if f.scope.lookup(n.Name) == nil {
		// a new local starts out nil, so this is a plain assignment
		return f.assignLocal(n, n.Name, f.genExpr(n.Value, nil), nil)
	}
	v := f.readLocal(&parser.LocalVariableReadNode{Name: n.Name, Location: n.Location})
	if v.base != nil {
		return expr{code: v.goName, typ: v.typ, done: true} // narrowed: already set
	}
	e := f.genOrAssign(n, expr{code: v.goName, typ: v.typ}, n.Value)
	if info := f.locals[n.Name]; info != nil {
		info.writes++
		if !isAncestorBlock(info.declBlock, f.block) {
			info.hoist = true
		}
	}
	if isOpt(v.typ) && !isOpt(e.typ) {
		f.applyNarrow([]narrowInfo{{local: v, typ: e.typ}}) // non-nil from here on
	}
	return e
}

func (f *fctx) genOrAssignIvar(n *parser.InstanceVariableOrWriteNode) expr {
	iv := f.c.findIvar(f.owner, n.Name)
	if iv == nil && f.discover {
		// first seen through ||=: it starts out nil
		var v expr
		f.probe(func() { v = f.genExpr(n.Value, nil) })
		t := v.typ
		if !isOpt(t) && !isAny(t) && !isClass(t, "Boolean") && !isNil(t) {
			t = TOpt{Elem: t}
		}
		iv = f.ivar(n, n.Name, t)
	}
	if iv == nil {
		iv = f.ivar(n, n.Name, nil)
	}
	return f.genOrAssign(n, expr{code: f.ivarCode(iv), typ: iv.Type}, n.Value)
}

// genMultiWrite renders `a, b = x, y` and `a, b = tuple`.
func (f *fctx) genMultiWrite(n *parser.MultiWriteNode) expr {
	if n.Rest != nil || len(n.Rights) > 0 {
		f.errorf(n, "splat in multiple assignment is not supported")
	}
	var vals []expr
	if arr, ok := n.Value.(*parser.ArrayNode); ok && len(arr.Elements) == len(n.Lefts) {
		vals = f.multiLiteral(arr, n.Lefts)
	} else {
		vals = f.multiDestructure(n)
	}
	for i, target := range n.Lefts {
		switch t := target.(type) {
		case *parser.LocalVariableTargetNode:
			f.assignLocal(t, t.Name, vals[i], nil)
		case *parser.InstanceVariableTargetNode:
			iv := f.ivar(t, t.Name, vals[i].typ)
			f.emit("%s = %s", f.ivarCode(iv), f.coerce(t, vals[i], iv.Type))
		default:
			f.errorf(target, "unsupported assignment target %s", nodeType(target))
		}
	}
	return expr{code: "", typ: TVoid{}, stmt: true, done: true}
}

// multiLiteral evaluates every right-hand side into a temporary before any
// target is written, so `a, b = b, a` swaps.
func (f *fctx) multiLiteral(arr *parser.ArrayNode, lefts []parser.Node) []expr {
	vals := make([]expr, 0, len(arr.Elements))
	for i, el := range arr.Elements {
		e := f.genExpr(el, f.targetType(lefts[i]))
		if isNil(e.typ) {
			t := f.targetType(lefts[i])
			if t == nil {
				f.errorf(el, "cannot infer a type from nil; annotate the target first")
			}
			e.typ = t
		}
		tmp := f.newTmp()
		code := e.code
		if e.lit {
			code = f.c.goType(e.typ) + "(" + code + ")"
		}
		if isNil(e.typ) || code == "nil" {
			f.emit("var %s %s", tmp, f.c.goType(e.typ))
		} else {
			f.emit("%s := %s", tmp, code)
		}
		vals = append(vals, expr{code: tmp, typ: e.typ})
	}
	return vals
}

// multiDestructure splits a tuple (or an Array, into T? values).
func (f *fctx) multiDestructure(n *parser.MultiWriteNode) []expr {
	var vals []expr
	{
		v := f.genExpr(n.Value, nil)
		tmp := f.newTmp()
		f.emit("%s := %s", tmp, v.code)
		switch t := v.typ.(type) {
		case TTuple:
			if len(t.Elems) != len(n.Lefts) {
				f.errorf(n, "%d targets for a %d-tuple", len(n.Lefts), len(t.Elems))
			}
			for i, et := range t.Elems {
				vals = append(vals, expr{code: fmt.Sprintf("%s.F%d", tmp, i), typ: et})
			}
		case TClass:
			if t.C.RubyName != "Array" {
				f.errorf(n, "cannot destructure %s", v.typ)
			}
			for i := range n.Lefts {
				vals = append(vals, expr{code: fmt.Sprintf("%s.Idx(%d)", tmp, i), typ: TOpt{Elem: t.Args[0]}})
			}
		default:
			f.errorf(n, "cannot destructure %s", v.typ)
		}
	}
	return vals
}

// targetType is the current type of an assignment target, if it has one.
func (f *fctx) targetType(n parser.Node) Type {
	switch t := n.(type) {
	case *parser.LocalVariableTargetNode:
		if v := f.scope.lookup(t.Name); v != nil {
			if v.base != nil {
				return v.base.typ
			}
			return v.typ
		}
	case *parser.InstanceVariableTargetNode:
		if iv := f.c.findIvar(f.owner, t.Name); iv != nil {
			return iv.Type
		}
	}
	return nil
}

// constGetType narrows `M.const_get(name)` when M is statically known: a
// literal name gets that constant's type; any other name the join of the
// constants of M and its subclasses (whose tables M's may stand for).
func (f *fctx) constGetType(recv expr, args []parser.Node) Type {
	meta := f.metaOfType(recv.typ)
	if meta == nil || len(args) == 0 {
		return nil
	}
	mod := meta.metaOf
	if lit := literalName(args[0]); lit != "" {
		cls, k := f.c.lookupConst(f.f, constPath(lit), []*Class{mod})
		if mod.RubyName == "Object" {
			cls, k = f.c.lookupConst(f.f, constPath(lit), nil)
		}
		return f.c.constTypeOf(cls, k)
	}
	var ts []Type
	var walk func(c *Class)
	walk = func(c *Class) {
		for _, full := range f.c.constEntries(c) {
			cls, k := f.c.classes[full], f.c.consts[full]
			if t := f.c.constTypeOf(cls, k); t != nil {
				ts = append(ts, t)
			}
		}
		for _, sub := range c.Subclasses {
			if sub.metaOf == nil {
				walk(sub)
			}
		}
	}
	walk(mod)
	if len(ts) == 0 {
		return nil
	}
	if t := joinOrAny(ts); !isAny(t) {
		return t
	}
	return nil
}

func (c *Compiler) constTypeOf(cls *Class, k *Const) Type {
	switch {
	case cls != nil && cls.meta != nil:
		return TClass{C: cls.meta}
	case k != nil:
		return c.constType(k)
	}
	return nil
}

// literalName returns the text of a Symbol or String literal.
func literalName(n parser.Node) string {
	switch n := n.(type) {
	case *parser.SymbolNode:
		return n.Unescaped.Value
	case *parser.StringNode:
		return n.Unescaped.Value
	}
	return ""
}

// constPath turns "A::B" into a constant node lookupConst understands.
func constPath(path string) parser.Node {
	parts := strings.Split(strings.TrimPrefix(path, "::"), "::")
	var n parser.Node = &parser.ConstantReadNode{Name: parts[0]}
	for _, p := range parts[1:] {
		name := p
		n = &parser.ConstantPathNode{Parent: n, Name: &name}
	}
	return n
}

// isBlockParam reports whether n reads the method's own &block parameter.
func (f *fctx) isBlockParam(n parser.Node) bool {
	lv, ok := n.(*parser.LocalVariableReadNode)
	return ok && f.m != nil && f.m.BlockParam != "" && lv.Name == f.m.BlockParam && f.scope.lookup(lv.Name) == nil
}

// forwardIter passes the method's own block on to an iterator:
// `list.each(&block)` re-yields every value (or calls the closure with it).
func (f *fctx) forwardIter(n parser.Node, call string, yields []Type, t tail) {
	if len(yields) != len(f.blockSig.Params) {
		f.errorf(n, "the forwarded block takes %d values but %d are yielded", len(f.blockSig.Params), len(yields))
	}
	vars := make([]string, len(yields))
	for i := range yields {
		vars[i] = f.newTmp()
	}
	list := strings.Join(vars, ", ")
	f.emit("for %s := range %s {", list, call)
	if f.iterator {
		f.emit("\tif !yield(%s) {", list)
		f.emit("\t\treturn")
		f.emit("\t}")
	} else {
		f.emit("\tblk(%s)", list)
	}
	f.emit("}")
	if t.kind != tailNone {
		f.emptyTail(n, t)
	}
}

// forwardClosure passes the method's own block on to a closure-taking
// method: `list.map(&block)`.
func (f *fctx) forwardClosure(n parser.Node, sig *BlockSig, env map[string]Type) string {
	if f.iterator {
		f.errorf(n, "this method's block is an iterator (it yields); it can only be passed on to another iterator")
	}
	if len(sig.Params) != len(f.blockSig.Params) {
		f.errorf(n, "the forwarded block takes %d values but %d are passed", len(f.blockSig.Params), len(sig.Params))
	}
	for i, p := range sig.Params {
		unify(p, f.blockSig.Params[i], env)
	}
	unify(sig.Ret, f.blockSig.Ret, env)
	return "blk"
}

// keywordMembers turns `Point.new(x: 1, y: 2)` into positional arguments
// in member order. Missing members are an error for Data and nil for a
// Struct.
func (f *fctx) keywordMembers(n parser.Node, vr *Class, args []parser.Node) []parser.Node {
	if len(args) != 1 {
		return args
	}
	kw, ok := args[0].(*parser.KeywordHashNode)
	if !ok {
		return args
	}
	byName := map[string]parser.Node{}
	for _, el := range kw.Elements {
		a, ok := el.(*parser.AssocNode)
		sym, isSym := a.Key.(*parser.SymbolNode)
		if !ok || !isSym {
			return args // a Hash argument, not keywords
		}
		byName[sym.Unescaped.Value] = a.Value
	}
	out := make([]parser.Node, len(vr.valueMembers))
	for i, m := range vr.valueMembers {
		v, ok := byName[m]
		switch {
		case ok:
			delete(byName, m)
		case vr.valueKind == "data":
			f.errorf(n, "missing keyword: :%s", m)
		default:
			v = &parser.NilNode{}
		}
		out[i] = v
	}
	for k := range byName {
		f.errorf(n, "unknown keyword: :%s", k)
	}
	return out
}

// genDataWith compiles `value.with(k: v)`: a copy with some members
// replaced, made by the receiver's own class.
func (f *fctx) genDataWith(n parser.Node, recv expr, args []parser.Node) (expr, bool) {
	var cls *Class
	switch t := recv.typ.(type) {
	case TClass:
		cls = t.C
	case TVar:
		if t.Name == "Self" {
			cls = f.owner
		}
	}
	if cls == nil || cls.valueRoot() == nil || cls.valueRoot().valueKind != "data" {
		return expr{}, false
	}
	vr := cls.valueRoot()
	byName := map[string]parser.Node{}
	if len(args) == 1 {
		kw, ok := args[0].(*parser.KeywordHashNode)
		if !ok {
			f.errorf(n, "with takes keyword arguments")
		}
		for _, el := range kw.Elements {
			a, ok := el.(*parser.AssocNode)
			sym, isSym := a.Key.(*parser.SymbolNode)
			if !ok || !isSym {
				f.errorf(el, "with takes keyword arguments")
			}
			byName[sym.Unescaped.Value] = a.Value
		}
	} else if len(args) > 1 {
		f.errorf(n, "with takes keyword arguments")
	}
	if !isSimpleGo(recv.code) {
		tmp := f.newTmp()
		f.emit("%s := %s", tmp, recv.code)
		recv.code = tmp
	}
	nodes := make([]parser.Node, len(vr.valueMembers))
	for i, m := range vr.valueMembers {
		v, ok := byName[m]
		if !ok {
			v = &withMember{recv: recv, name: m}
		}
		delete(byName, m)
		nodes[i] = v
	}
	for k := range byName {
		f.errorf(n, "unknown keyword: :%s", k)
	}
	return f.genMethodCall(n, recv, "__with", nodes, nil), true
}

// withMember is an argument node for `with` reading the receiver's member.
type withMember struct {
	parser.Node
	recv expr
	name string
}

func (w *withMember) GetLocation() parser.Location { return parser.Location{} }

// callMissing sends an unknown method to the receiver's method_missing,
// with the name as a Symbol; Ruby calls it even when it is private.
func (f *fctx) callMissing(n parser.Node, mm *entry, recv expr, name string, args []parser.Node, block parser.Node) expr {
	if block != nil {
		f.errorf(n, "a block passed to %s (handled by method_missing) is not supported", name)
	}
	nodes := append([]parser.Node{&parser.SymbolNode{Unescaped: parser.RubyString{Value: name}}}, args...)
	f.implicitCall = true
	defer func() { f.implicitCall = false }()
	return f.callEntry(n, mm, recv, nodes, nil)
}

// rubyPrivate names methods Ruby makes private whoever defines them.
var rubyPrivate = map[string]bool{"initialize": true, "method_missing": true, "respond_to_missing?": true, "initialize_copy": true}

// genRespondTo decides `recv.respond_to?(:name)` for a typed receiver and
// a literal name: true for a public method, else respond_to_missing?,
// else false. Returns false when the answer depends on the runtime class.
func (f *fctx) genRespondTo(n parser.Node, recv expr, args []parser.Node) (expr, bool) {
	name := literalName(args[0])
	var cls *Class
	switch t := recv.typ.(type) {
	case TClass:
		cls = t.C
	case TVar:
		if t.Name == "Self" {
			cls = f.owner
		}
	}
	if name == "" || cls == nil {
		return expr{}, false
	}
	if e := cls.lookup(name); e != nil && !e.M.Private && !rubyPrivate[name] {
		if !isSimpleGo(recv.code) {
			f.emit("_ = %s", recv.code)
		}
		return expr{code: "Boolean(true)", typ: f.cls("Boolean")}, true
	}
	if cls.descendantDefines(name) {
		return expr{}, false
	}
	if rm := cls.lookup("respond_to_missing?"); rm != nil {
		f.implicitCall = true
		defer func() { f.implicitCall = false }()
		return f.callEntry(n, rm, recv, []parser.Node{args[0], &parser.FalseNode{}}, nil), true
	}
	if !isSimpleGo(recv.code) {
		f.emit("_ = %s", recv.code)
	}
	return expr{code: "Boolean(false)", typ: f.cls("Boolean")}, true
}

// exprNode carries an already generated expression where a node is
// expected (wrapper arguments).
type exprNode struct {
	parser.Node
	e expr
}

func (x *exprNode) GetLocation() parser.Location { return parser.Location{} }

// genDynCall sends a method to an untyped value: see prelude/dynamic.rb.
func (f *fctx) genDynCall(n parser.Node, recv expr, name string, args []parser.Node) expr {
	f.c.noteDyn(name)
	f.c.warn(f.f, n, "dynamic call: %s on %s", name, recv.typ)
	vcall := false
	if call, ok := n.(*parser.CallNode); ok && call.IsVARIABLE_CALL() {
		vcall = true
	}
	codes := make([]string, 0, 2+len(args))
	codes = append(codes, strconv.FormatBool(vcall), f.coerce(n, recv, TAny{}))
	for _, a := range args {
		if _, ok := a.(*parser.SplatNode); ok {
			f.errorf(a, "splat arguments in a dynamic call are not supported")
		}
		e := f.genExpr(a, nil)
		codes = append(codes, f.coerce(a, e, TAny{}))
	}
	return expr{code: "rbDyn" + goMethodName(name) + "(" + strings.Join(codes, ", ") + ")", typ: TAny{}}
}

// genSend compiles send/public_send/__send__. A literal name is an
// ordinary call (send may reach private methods); a computed one switches
// over every method name at run time.
func (f *fctx) genSend(n parser.Node, recv expr, name string, args []parser.Node, block parser.Node) expr {
	if lit := literalName(args[0]); lit != "" {
		if name != "public_send" {
			f.implicitCall = true
		}
		e := f.genMethodCall(n, recv, lit, args[1:], block)
		f.implicitCall = false
		return e
	}
	if block != nil {
		f.errorf(n, "a block with a computed send is not supported")
	}
	f.c.dynAll = true
	f.c.warn(f.f, n, "dynamic call: %s with a computed name", name)
	nameExpr := f.genExpr(args[0], nil)
	codes := []string{f.coerce(n, recv, TAny{}), "rbConstName(" + f.coerce(args[0], nameExpr, TAny{}) + ")"}
	for _, a := range args[1:] {
		e := f.genExpr(a, nil)
		codes = append(codes, f.coerce(a, e, TAny{}))
	}
	return expr{code: "rbSendByName(" + strings.Join(codes, ", ") + ")", typ: TAny{}}
}

// universalNames answer respond_to? for every object.
var universalNames = map[string]bool{"to_s": true, "inspect": true, "==": true, "!=": true, "!": true,
	"nil?": true, "equal?": true, "class": true, "is_a?": true, "kind_of?": true, "respond_to?": true,
	"send": true, "public_send": true, "hash": true, "then": true, "frozen?": true, "to_json": true}

// genDynRespondTo answers respond_to? when the static type cannot.
func (f *fctx) genDynRespondTo(n parser.Node, recv expr, args []parser.Node) expr {
	r := f.coerce(n, recv, TAny{})
	if lit := literalName(args[0]); lit != "" {
		if universalNames[lit] {
			return expr{code: "Boolean(true)", typ: f.cls("Boolean")}
		}
		f.c.noteRespond(lit)
		return expr{code: "rbResponds" + goMethodName(lit) + "(" + r + ")", typ: f.cls("Boolean")}
	}
	f.c.dynAll = true
	nameExpr := f.genExpr(args[0], nil)
	return expr{code: "rbRespondsByName(" + r + ", rbConstName(" + f.coerce(args[0], nameExpr, TAny{}) + "))", typ: f.cls("Boolean")}
}

func isSelf(n parser.Node) bool { _, ok := n.(*parser.SelfNode); return ok }

// attrLocal treats a receiver-less call of self's attribute reader as a
// local for narrowing: readers are pure, so `if resource.is_a?(Array)`
// may narrow `resource` for the branch, like Ruby programmers expect.
func (f *fctx) attrLocal(n parser.Node) *local {
	call, ok := n.(*parser.CallNode)
	if !ok || call.Receiver != nil || call.Arguments != nil || call.Block != nil || f.owner == nil {
		return nil
	}
	if v := f.scope.lookup("attr:" + call.Name); v != nil {
		return v
	}
	e := f.owner.lookup(call.Name)
	if e == nil || e.M.Kind != kindAttrReader {
		return nil
	}
	x := f.genExpr(call, nil)
	return &local{name: "attr:" + call.Name, goName: x.code, typ: x.typ, declared: true}
}

// genRescueModifier compiles `expr rescue fallback`: the fallback's value
// when expr raises a StandardError.
func (f *fctx) genRescueModifier(n *parser.RescueModifierNode, expected Type) expr {
	var e, r expr
	f.probe(func() {
		e = f.genExpr(n.Expression, expected)
		r = f.genExpr(n.RescueExpression, expected)
	})
	typ, ok := join(e.typ, r.typ)
	if !ok {
		typ = TAny{}
	}
	if isVoid(typ) && !isNil(typ) {
		typ = TAny{}
	}
	tmp := f.newTmp()
	f.emit("var %s %s", tmp, f.c.goType(typ))
	f.emit("func() {")
	f.indent++
	saved := f.enterBlock()
	f.emit("defer func() {")
	f.emit("\tif p := recover(); p != nil {")
	f.emit("\t\tp = rbWrapPanic(p)")
	f.emit("\t\tif !rbIsA[StandardErrorI](p) {")
	f.emit("\t\t\tpanic(p)")
	f.emit("\t\t}")
	f.indent += 2
	r = f.genExpr(n.RescueExpression, typ)
	f.emit("%s = %s", tmp, f.coerce(n, r, typ))
	f.indent -= 2
	f.emit("\t}")
	f.emit("}()")
	e = f.genExpr(n.Expression, typ)
	f.emit("%s = %s", tmp, f.coerce(n, e, typ))
	f.leaveBlock(saved)
	f.indent--
	f.emit("}()")
	return expr{code: tmp, typ: typ}
}
