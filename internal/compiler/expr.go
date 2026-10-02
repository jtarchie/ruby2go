package compiler

import (
	"errors"
	"fmt"
	"math"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"unicode"

	"github.com/danielgatis/go-ruby-prism/parser"

	"github.com/jtarchie/ruby2go/internal/rbs"
)

// expr is a generated Go expression with its Ruby type.
type expr struct {
	code     string
	typ      Type
	classObj bool // exactly a class constant: its Go type is the concrete metaclass
	ctor     bool // a struct class's NewX(...): its Go type is *X, not XI
	lit      bool // untyped Go constant
	stmt     bool // already a complete statement (assignment, panic)
	noreturn bool // panic/exit: terminates the statement list
	done     bool // already emitted; code only names the value
	assert   bool // ends in a Go type assertion, which is not a statement
	// view is the untyped Go value that code converts, for an Array or Hash
	// seen as another instantiation (decision 21): a narrowed local, or a
	// call sent through it. Calls and untyped uses take the value itself.
	view    string
	nilable bool // untyped collapsed from untyped? (Hash#[]): calls warn like T?'s
}

// genMatchWrite is `/(?<year>\d+)/ =~ s`: =~ through Regexp#match, then
// each named group into the local of its name (nil when nothing matched).
func (f *fctx) genMatchWrite(n *parser.MatchWriteNode) expr {
	call := n.Call
	re := f.genExpr(call.Receiver, nil)
	m := f.genMethodCall(call, re, "match", callArgs(call), nil)
	tmp := f.newTmp()
	f.emit("%s := %s", tmp, f.materialize(m))
	for _, t := range n.Targets {
		lt, ok := t.(*parser.LocalVariableTargetNode)
		if !ok {
			f.errorf(t, "unsupported named capture target %s", nodeType(t))
		}
		f.assignLocal(lt, lt.Name, expr{code: fmt.Sprintf("rbMatchCapture(%s, %q)", tmp, lt.Name), typ: TOpt{Elem: f.cls("String")}}, nil)
	}
	return expr{code: "rbMatchPos(" + tmp + ")", typ: TOpt{Elem: f.cls("Integer")}}
}

// genSugar renders syntax that reduces to a literal or a plain read:
// `defined?` (answered at compile time), `__LINE__`, `{ a: }`, `:"#{x}"`.
func (f *fctx) genSugar(n parser.Node) expr {
	switch n := n.(type) {
	case *parser.DefinedNode:
		kind := f.definedKind(n.Value)
		if kind == "" {
			return expr{code: "nil", typ: TNil{}}
		}
		f.c.strLits[kind] = true // MRI's answers are frozen literals
		return expr{code: strconv.Quote(kind), typ: f.cls("String"), lit: true}
	case *parser.SourceLineNode:
		return expr{code: strconv.Itoa(f.f.line(n.Location.StartOffset)), typ: f.cls("Integer"), lit: true}
	case *parser.ImplicitNode: // `{ a: }` reads the local or method a
		return f.genExpr(n.Value, nil)
	case *parser.InterpolatedSymbolNode:
		s := f.genInterp(&parser.InterpolatedStringNode{Location: n.Location, Parts: n.Parts})
		return expr{code: "Symbol(" + s.code + ")", typ: f.cls("Symbol")}
	}
	f.c.unsupported(f.f, n)
	return expr{}
}

// genLiteral handles the leaf expressions.
func (f *fctx) genLiteral(n parser.Node) (expr, bool) {
	switch n := n.(type) {
	case *parser.StringNode:
		f.c.strLits[n.Unescaped.Value] = true
		return expr{code: strconv.Quote(n.Unescaped.Value), typ: f.cls("String"), lit: true}, true
	case *parser.InterpolatedStringNode:
		return f.genInterp(n), true
	case *parser.XStringNode:
		if f.f.prelude {
			f.errorf(n, "%%x{} is only allowed as the whole body of a prelude method")
		}
		f.c.strLits[n.Unescaped.Value] = true
		cmd := expr{code: strconv.Quote(n.Unescaped.Value), typ: f.cls("String"), lit: true}
		return f.kernelCall(n, "__backtick", []parser.Node{&exprNode{Node: n, e: cmd}}), true
	case *parser.InterpolatedXStringNode:
		cmd := f.genInterp(&parser.InterpolatedStringNode{Location: n.Location, Parts: n.Parts})
		return f.kernelCall(n, "__backtick", []parser.Node{&exprNode{Node: n, e: cmd}}), true
	case *parser.SourceFileNode:
		f.c.strLits[f.f.Name] = true
		return expr{code: strconv.Quote(f.f.Name), typ: f.cls("String"), lit: true}, true
	case *parser.DefinedNode, *parser.SourceLineNode, *parser.ImplicitNode, *parser.InterpolatedSymbolNode:
		return f.genSugar(n), true
	case *parser.GlobalVariableReadNode:
		return f.genGlobalRead(n), true
	case *parser.GlobalVariableWriteNode:
		return f.genGlobalWrite(n), true
	case *parser.IntegerNode:
		code := strings.ReplaceAll(f.f.text(n.Location), "_", "")
		if digits := strings.TrimPrefix(code, "-"); len(digits) > 1 && (digits[1] == 'd' || digits[1] == 'D') {
			// Ruby's 0d decimal prefix has no Go spelling; 0x/0o/0b/0 carry over.
			code = code[:len(code)-len(digits)] + strings.TrimLeft(digits[2:], "0")
			if strings.TrimPrefix(code, "-") == "" {
				code += "0"
			}
		}
		_, err := strconv.ParseInt(code, 0, 64)
		if errors.Is(err, strconv.ErrRange) {
			f.errorf(n, "Integer literal %s does not fit in 64 bits: there is no Bignum (docs/design.md decision 35)", code)
		}
		return expr{code: code, typ: f.cls("Integer"), lit: true}, true
	case *parser.FloatNode:
		if n.Value == 0 && math.Signbit(n.Value) {
			// Go's constant -0.0 is +0; Ruby's is negative zero.
			return expr{code: "Float(math.Copysign(0, -1))", typ: f.cls("Float")}, true
		}
		return expr{code: f.f.text(n.Location), typ: f.cls("Float"), lit: true}, true
	case *parser.RationalNode:
		return expr{code: fmt.Sprintf("rbRat(%d, %d)", n.Numerator, n.Denominator), typ: f.cls("Rational")}, true
	case *parser.ImaginaryNode:
		im := f.genExpr(n.Numeric, nil)
		return expr{code: "(&Complex{Integer(0), " + f.coerce(n, im, TAny{}) + "})", typ: f.cls("Complex")}, true
	case *parser.TrueNode:
		return expr{code: "true", typ: f.cls("Boolean"), lit: true}, true
	case *parser.FalseNode:
		return expr{code: "false", typ: f.cls("Boolean"), lit: true}, true
	case *parser.NilNode:
		return expr{code: "nil", typ: TNil{}}, true
	case *parser.SelfNode:
		return expr{code: f.selfCode, typ: f.selfType, classObj: f.selfClassObj}, true
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
		return expr{code: v.goName, typ: v.typ, view: v.view}
	case *parser.ItLocalVariableReadNode:
		v := f.readLocal(&parser.LocalVariableReadNode{Name: "it", Location: n.Location})
		return expr{code: v.goName, typ: v.typ, view: v.view}
	case *parser.LocalVariableWriteNode:
		return f.genLocalWrite(n)
	case *parser.LocalVariableOperatorWriteNode:
		cur := f.genExpr(&parser.LocalVariableReadNode{Name: n.Name, Location: n.Location}, nil)
		val := f.genOp(n, cur, n.BinaryOperator, n.Value)
		return f.assignLocal(n, n.Name, val, nil)
	case *parser.InstanceVariableReadNode, *parser.InstanceVariableWriteNode, *parser.InstanceVariableOperatorWriteNode,
		*parser.ClassVariableReadNode, *parser.ClassVariableWriteNode, *parser.ClassVariableOperatorWriteNode:
		return f.genIvarExpr(n)
	case *parser.CallNode:
		return f.genCallValue(n, expected)
	case *assignedArg:
		return f.genAssignedArg(n, expected)
	case *parser.ArrayNode:
		return f.genArray(n, expected)
	case *parser.HashNode:
		return f.genHash(n, n.Elements, expected)
	case *parser.RegularExpressionNode, *parser.InterpolatedRegularExpressionNode:
		return f.genRegexp(n)
	case *parser.MatchWriteNode:
		return f.genMatchWrite(n)
	case *parser.LocalVariableOrWriteNode, *parser.InstanceVariableOrWriteNode, *parser.CallOrWriteNode, *parser.IndexOrWriteNode:
		return f.genOrWrite(n)
	case *parser.CallOperatorWriteNode, *parser.IndexOperatorWriteNode:
		return f.genOpWrite(n)
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
	case *parser.RangeNode:
		return f.genRange(n, expected)
	case *parser.LambdaNode:
		return f.genLambda(n, n, n.Parameters, expected)
	}
	f.c.unsupported(f.f, n)
	return expr{}
}

func (f *fctx) genLocalWrite(n *parser.LocalVariableWriteNode) expr {
	var ann Type
	if t := f.f.trailingAnnotation(n); t != "" {
		ann = f.parseTypeAnn(n, t)
	}
	exp := ann
	if exp == nil {
		if v := f.visibleLocal(n.Name); v != nil {
			exp = v.typ
		}
	}
	empty := ann == nil && isEmptyLit(n.Value)
	if exp == nil && empty {
		exp = f.refined[f.localKey(n.Name)]
	}
	if r, ok := n.Value.(*parser.LocalVariableReadNode); ok && r.Name == n.Name && f.erbDepth > 0 { // `x = x` declares x for the template's parse: a no-op
		if v := f.visibleLocal(n.Name); v != nil {
			return expr{code: v.goName, typ: v.typ, view: v.view, done: true}
		}
		return expr{code: "", typ: TNil{}, done: true} // declared later in the def: not in scope here
	}
	val := f.genExpr(n.Value, exp)
	e := f.assignLocal(n, n.Name, val, ann)
	if info := f.localInfo(n.Name); empty && f.pass == 1 && info != nil {
		info.open = true
	}
	return e
}

func isEmptyLit(n parser.Node) bool {
	switch n := n.(type) {
	case *parser.ArrayNode:
		return len(n.Elements) == 0
	case *parser.HashNode:
		return len(n.Elements) == 0
	}
	return false
}

// noteElems records what `x << v`, `x.push(v)`, `x[i] = v` and
// `h[k] = v` put in an open `[]`/`{}`: a local in pass 1 (analyze), an
// ivar during discovery (refineIvars).
func (f *fctx) noteElems(n *parser.CallNode) {
	if n.Block != nil {
		return
	}
	var t Type
	var open *bool
	var elems *[2][]Type
	switch r := n.Receiver.(type) {
	case *parser.LocalVariableReadNode:
		if info := f.localInfo(r.Name); f.pass == 1 && info != nil && info.open {
			t, open, elems = info.typ, &info.open, &info.elems
		}
	case *parser.InstanceVariableReadNode:
		if f.discover && f.owner != nil {
			if iv := f.c.findIvar(f.owner, r.Name); iv != nil && iv.open {
				t, open, elems = iv.Type, &iv.open, &iv.elems
			}
		}
	}
	if elems == nil {
		return
	}
	args := callArgs(n)
	switch hash := isClass(stripOpt(t), "Hash"); {
	case !hash && slices.Contains([]string{"<<", "push", "append", "unshift", "prepend"}, n.Name):
		f.noteElem(open, &elems[0], args...)
	case !hash && n.Name == "[]=" && len(args) == 2:
		f.noteElem(open, &elems[0], args[1])
		elems[0] = append(elems[0], TNil{}) // a gap before the index reads nil
	case hash && (n.Name == "[]=" || n.Name == "store") && len(args) == 2:
		f.noteElem(open, &elems[0], args[0])
		f.noteElem(open, &elems[1], args[1])
	}
}

func (f *fctx) noteElem(open *bool, elems *[]Type, args ...parser.Node) {
	for _, a := range args {
		if _, ok := a.(*parser.SplatNode); ok {
			*open = false
			return
		}
		var t Type
		f.probe(func() { t = f.genExpr(a, nil).typ })
		*elems = append(*elems, t)
	}
}

// noteConv records a conversion site for analyze.
func (f *fctx) noteConv(n parser.Node, code string) string {
	if f.pass == 1 && f.convs != nil {
		f.convs[n.GetLocation().StartOffset] = true
	}
	if f.pass == 2 && f.c.convs != nil {
		f.c.convs[fmt.Sprintf("%s:%d", f.f.Name, n.GetLocation().StartOffset)] = true
	}
	return code
}

// genIvarExpr handles @x reads and writes (and hands @@x to genClassVarExpr).
func (f *fctx) genIvarExpr(n parser.Node) expr {
	switch n := n.(type) {
	case *parser.ClassVariableReadNode, *parser.ClassVariableWriteNode, *parser.ClassVariableOperatorWriteNode:
		return f.genClassVarExpr(n)
	case *parser.InstanceVariableReadNode:
		iv := f.ivar(n, n.Name, nil)
		return expr{code: f.ivarCode(iv), typ: iv.Type}
	case *parser.InstanceVariableWriteNode:
		var exp, ann Type
		if t := f.f.trailingAnnotation(n); t != "" && !isLetFlag(n.Name) {
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
		if f.discover && ann == nil && isEmptyLit(n.Value) {
			iv.open = true
		}
		if !fitsValue(val, iv.Type) {
			f.errorf(n, "cannot assign %s to %s, which is %s", val.typ, n.Name, iv.Type)
		}
		code := f.ivarCode(iv)
		f.emit("%s = %s", code, f.coerce(n, val, iv.Type))
		// the value is what was assigned: `@x = 1` is an Integer even when @x is Integer?
		if isOpt(iv.Type) && !isAny(stripOpt(iv.Type)) && !isOpt(val.typ) && !isNil(val.typ) && !isAny(val.typ) {
			return expr{code: "(*" + code + ")", typ: stripOpt(iv.Type), done: true}
		}
		return expr{code: code, typ: iv.Type, done: true}
	case *parser.InstanceVariableOperatorWriteNode:
		iv := f.ivar(n, n.Name, nil)
		code := f.ivarCode(iv)
		val := f.genOp(n, expr{code: code, typ: iv.Type}, n.BinaryOperator, n.Value)
		f.emit("%s = %s", code, f.coerce(n, val, iv.Type))
		return expr{code: code, typ: iv.Type, done: true}
	}
	f.c.unsupported(f.f, n)
	return expr{}
}

// genClassVarExpr handles @@x: a package variable of the class whose body
// first assigned it, shared by its subclasses (and a module's includers).
func (f *fctx) genClassVarExpr(n parser.Node) expr {
	switch n := n.(type) {
	case *parser.ClassVariableReadNode:
		k := f.classVar(n, n.Name)
		return expr{code: k.GoName, typ: f.c.constType(k)}
	case *parser.ClassVariableWriteNode:
		k := f.classVar(n, n.Name)
		t := f.c.constType(k)
		val := f.genExpr(n.Value, t)
		f.emit("%s = %s", k.GoName, f.coerce(n, val, t))
		return expr{code: k.GoName, typ: t, done: true}
	case *parser.ClassVariableOperatorWriteNode:
		k := f.classVar(n, n.Name)
		t := f.c.constType(k)
		val := f.genOp(n, expr{code: k.GoName, typ: t}, n.BinaryOperator, n.Value)
		f.emit("%s = %s", k.GoName, f.coerce(n, val, t))
		return expr{code: k.GoName, typ: t, done: true}
	}
	f.c.unsupported(f.f, n)
	return expr{}
}

// classVar finds @@name from the code's class: its own, an ancestor's, or an included module's.
func (f *fctx) classVar(n parser.Node, name string) *Const {
	if k := f.lookupClassVar(name); k != nil {
		return k
	}
	f.errorf(n, "uninitialized class variable %s (rb2go needs its first assignment in a class or module body)", name)
	return nil
}

func (f *fctx) lookupClassVar(name string) *Const {
	cls := f.owner
	if cls != nil && cls.metaOf != nil {
		cls = cls.metaOf
	}
	if cls == nil {
		return nil
	}
	for _, a := range cls.ancestors() {
		if k := a.cvars[name]; k != nil {
			return k
		}
	}
	return nil
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
		if isNil(typ) { // every branch is nil (`if c then end`): the value is nil, not void
			return expr{code: "nil", typ: TNil{}, lit: true}
		}
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
	if isNil(l.typ) { // `nil || x` is x
		f.valueOf(l)
		return f.genExpr(n.Right, nil)
	}
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
		f.emit("%s := %s", tmp, f.materialize(l))
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
		f.emit("if %s := %s; %s {", lt, f.materialize(l), lt)
		f.emit("\t%s = %s", tmp, f.coerce(n, expr{code: lt, typ: l.typ}, typ))
	default:
		f.emit("if %s := %s; %s {", lt, l.code, optTruthy(lt, l.typ))
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
		f.emit("%s := %s", tmp, f.materialize(l))
		f.emit("if %s {", tmp)
		f.buf.WriteString(stmts)
		f.emit("\t%s = %s", tmp, rc.code)
		f.emit("}")
		return expr{code: tmp, typ: l.typ}
	}
	if !boolL && !isOpt(l.typ) && !isAny(l.typ) && !isNil(l.typ) {
		// the left is never nil or false: the value is the right
		f.discard(l)
		return f.genExpr(n.Right, nil)
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
		if j, ok := join(lf, r.typ); ok {
			typ = j
		}
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
		nw.code = "(*" + lt + ")"
		f.applyNarrow([]narrowInfo{nw})
	}
	r = f.genExpr(n.Right, nil)
	f.pop()
	f.emit("%s = %s", tmp, f.coerce(n, r, typ))
	f.indent--
	if isAny(typ) || optBool {
		f.emit("} else {")
		f.emit("\t%s = %s", tmp, f.coerce(n, expr{code: lt, typ: l.typ}, typ))
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
	parts := f.interpParts(n.Parts, nil)
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
	if len(parts) == 1 { // "#{x}" is a new String, never x or x.to_s itself
		return expr{code: "rbStrClone(" + parts[0] + ")", typ: f.cls("String")}
	}
	return expr{code: "(" + strings.Join(parts, " + ") + ")", typ: f.cls("String")}
}

// interpParts appends the Go string pieces of an interpolation's parts.
// Adjacent literals (`"a" "#{b}"`) nest an InterpolatedStringNode as a part.
func (f *fctx) interpParts(ps []parser.Node, parts []string) []string {
	for _, p := range ps {
		switch p := p.(type) {
		case *parser.StringNode:
			parts = append(parts, strconv.Quote(p.Unescaped.Value))
		case *parser.InterpolatedStringNode:
			parts = f.interpParts(p.Parts, parts)
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
	return parts
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
	// A literal is never nil, so an expected T? means T.
	expected = stripOpt(expected)
	if tt, ok := expected.(TTuple); ok && len(tt.Elems) == len(n.Elements) {
		codes := make([]string, len(n.Elements))
		for i, el := range n.Elements {
			mark := f.buf.Len()
			e := f.genExpr(el, tt.Elems[i])
			f.pinBefore(mark, codes[:i])
			codes[i] = f.coerce(el, e, tt.Elems[i])
		}
		return expr{code: f.c.goType(tt) + "{" + strings.Join(codes, ", ") + "}", typ: tt}
	}
	var elemT Type
	if ec, ok := expected.(TClass); ok && ec.C.Name == "Array" {
		elemT = ec.Args[0]
	}
	hint := elemT
	if isAny(expected) {
		hint = TAny{} // a nested literal is untyped-expected too: an Array, not a tuple
	}
	elems := make([]expr, len(n.Elements))
	for i, el := range n.Elements {
		if _, ok := el.(*parser.SplatNode); ok {
			f.errorf(el, "splat inside array literals is not supported")
		}
		mark := f.buf.Len()
		elems[i] = f.genExpr(el, hint)
		f.pinExprs(mark, elems[:i])
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

// pinBefore keeps Ruby's left-to-right evaluation when generating an
// operand emitted statements (a hoisted temporary, `x&.y`) after earlier
// operands were generated as plain Go expressions: those earlier codes
// are evaluated into temporaries placed before the new statements.
// Constants need no pinning.
func (f *fctx) pinBefore(mark int, codes []string) {
	if !f.emittedSince(mark) {
		return
	}
	f.insertAt(mark, func() {
		for i, c := range codes {
			if pinnable(c) {
				t := f.newTmp()
				f.emit("%s := %s", t, c)
				codes[i] = t
			}
		}
	})
}

// pinExprs is pinBefore for operands not yet coerced.
func (f *fctx) pinExprs(mark int, es []expr) {
	if !f.emittedSince(mark) {
		return
	}
	f.insertAt(mark, func() {
		for i, e := range es {
			if !e.lit && !isVoid(e.typ) && pinnable(e.code) {
				t := f.newTmp()
				f.emit("%s := %s", t, e.code)
				es[i].code, es[i].view = t, ""
			}
		}
	})
}

// emittedSince reports whether statements (not just //line directives)
// were written to the buffer after mark.
func (f *fctx) emittedSince(mark int) bool {
	for l := range strings.SplitSeq(f.buf.String()[mark:], "\n") {
		if l = strings.TrimSpace(l); l != "" && !strings.HasPrefix(l, "//line ") {
			return true
		}
	}
	return false
}

// insertAt runs gen with its output placed at mark instead of the end.
func (f *fctx) insertAt(mark int, gen func()) {
	all := f.buf.String()
	var ins strings.Builder
	saved := f.buf
	f.buf = &ins
	gen()
	f.buf = saved
	f.buf.Reset()
	f.buf.WriteString(all[:mark])
	f.buf.WriteString(ins.String())
	f.buf.WriteString(all[mark:])
}

// pinnable is false for Go code whose value can't change: literals and nil.
func pinnable(code string) bool {
	switch {
	case code == "" || code == "nil" || code == "true" || code == "false":
		return false
	case code[0] == '"' || code[0] == '`' || code[0] >= '0' && code[0] <= '9':
		return false
	case code[0] == '-' && len(code) > 1 && code[1] >= '0' && code[1] <= '9':
		return false
	}
	return true
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
	if ec, ok := stripOpt(expected).(TClass); ok && ec.C.RubyName == "Hash" {
		kT, vT = ec.Args[0], ec.Args[1]
	}
	type kv struct {
		k, v  expr
		splat parser.Node // `**h`: v is the Hash merged in
	}
	pairs := make([]kv, 0, len(elements))
	ks, vs := make([]Type, 0, len(elements)), make([]Type, 0, len(elements))
	for _, el := range elements {
		if sp, ok := el.(*parser.AssocSplatNode); ok {
			var want Type
			if kT != nil {
				want = TClass{C: f.c.classes["Hash"], Args: []Type{kT, vT}}
			}
			h := f.genExpr(sp.Value, want)
			ht, ok := h.typ.(TClass)
			if !ok || ht.C.RubyName != "Hash" {
				f.errorf(sp, "**%s must be a Hash, not %s", f.f.text(sp.Value.GetLocation()), h.typ)
			}
			ks, vs = append(ks, ht.Args[0]), append(vs, ht.Args[1])
			pairs = append(pairs, kv{v: h, splat: sp})
			continue
		}
		a, ok := el.(*parser.AssocNode)
		if !ok {
			f.c.unsupported(f.f, el)
		}
		k := f.genExpr(a.Key, kT)
		v := f.genExpr(a.Value, vT)
		ks, vs = append(ks, k.typ), append(vs, v.typ)
		pairs = append(pairs, kv{k: k, v: v})
	}
	if kT == nil {
		kT, vT = joinOrAny(ks), joinOrAny(vs)
	}
	targs := "[" + f.c.goType(kT) + ", " + f.c.goType(vT) + "]"
	code := "NewHash" + targs + "()"
	for _, p := range pairs { // Hash's methods are free funcs (decision 86)
		if p.splat != nil {
			code = "rbHashSplat(" + code + ", " + f.coerce(p.splat, p.v, TClass{C: f.c.classes["Hash"], Args: []Type{kT, vT}}) + ")"
			continue
		}
		code = "Hash___Set" + targs + "(" + code + ", " + f.coerce(n, p.k, kT) + ", " + f.coerce(n, p.v, vT) + ")"
	}
	return expr{code: code, typ: TClass{C: f.c.classes["Hash"], Args: []Type{kT, vT}}}
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
	for wrapped(c) {
		c = c[1 : len(c)-1]
	}
	if !strings.HasSuffix(c, ")") { // nil itself, or a temp holding it
		return e
	}
	f.emit("%s", c)
	return expr{code: "nil", typ: TNil{}}
}

// wrapped reports whether c's first "(" closes at its last ")", so the
// pair can go: `(f(x))` is, `(*File).Close(f)` is not.
func wrapped(c string) bool {
	if !strings.HasPrefix(c, "(") || !strings.HasSuffix(c, ")") {
		return false
	}
	depth := 0
	for i := 0; i < len(c); i++ {
		switch c[i] {
		case '"', '`', '\'': // skip a literal: its parens don't count
			q := c[i]
			for i++; i < len(c) && c[i] != q; i++ {
				if c[i] == '\\' && q != '`' {
					i++
				}
			}
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return i == len(c)-1
			}
		}
	}
	return false
}

// fitsValue is fits plus the one literal conversion Go makes exactly: an
// Integer literal where a Float is expected (Float's operators take
// Integers in Ruby). Any other literal of the wrong class, such as "a" for
// a Symbol, would convert silently too, so it is rejected.
func fitsValue(e expr, to Type) bool {
	return fits(e.typ, to) || e.lit && isClass(e.typ, "Integer") && isClass(stripOpt(to), "Float")
}

// materialize is e's code as a typed Go value: an untyped constant bound
// with := or used as a receiver would otherwise become int/string/bool.
func (f *fctx) materialize(e expr) string {
	if e.lit {
		return f.c.goType(e.typ) + "(" + e.code + ")"
	}
	return e.code
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
	if e.view != "" && isAny(to) {
		return e.view // the value itself, not a converted copy
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
			switch e.typ.(TOpt).Elem.(type) {
			case TVar, TOpt: // E? with E = T?: Opt leaves the inner box
				return "rbUnbox(Opt(" + e.code + "))"
			case TAny, TClass, TFunc, TNil, TTuple, TVoid: // a plain T?: Opt boxes it below
			}
			return "Opt(" + e.code + ")"
		case e.lit:
			return f.c.goType(e.typ) + "(" + e.code + ")"
		}
		if _, ok := e.typ.(TTuple); ok {
			return e.code + "._ToAny()" // untyped code sees an Array (decision 22)
		}
		return e.code
	case TOpt:
		switch {
		case isNil(e.typ):
			return "nil"
		case isOpt(e.typ):
			return e.code
		case isAny(e.typ):
			return f.noteConv(n, fmt.Sprintf("OptOf[%s](%s, %q)", f.c.goType(to.Elem), e.code, to.Elem.String()))
		}
		return "Ref[" + f.c.goType(to.Elem) + "](" + f.coerce(n, e, to.Elem) + ")"
	case TClass:
		return f.coerceClass(n, e, to)
	case TVar:
		return e.code
	case TFunc, TNil, TTuple, TVoid: // converted by the general rules below
	}
	return e.code
}

// coerceClass is coerce to a class type.
func (f *fctx) coerceClass(n parser.Node, e expr, to TClass) string {
	if to.C.universal && e.lit {
		return f.coerce(n, e, TAny{}) // Object is Go any: "a" must box as String, not string
	}
	if isAny(e.typ) && to.C.RubyName == "Boolean" {
		return "Boolean(rbTruthy(" + e.code + "))" // Ruby conditions test truthiness
	}
	if isOpt(e.typ) && to.C.RubyName == "Boolean" {
		return "Boolean(" + optTruthy(e.code, e.typ) + ")" // a predicate block's `seen.add?(x)`
	}
	// rbAs also converts an Array/Hash of another instantiation (rbConv).
	if isAny(e.typ) {
		return f.noteConv(n, fmt.Sprintf("rbAs[%s](%s, %q)", f.c.goType(to), e.code, to.String()))
	}
	// Go instantiations are invariant: Array[Integer] where
	// Array[untyped] is expected (or back) is a converted copy.
	if converts(to) && sameButUntyped(e.typ, to) && f.c.goType(e.typ) != f.c.goType(to) {
		return f.noteConv(n, fmt.Sprintf("rbAs[%s](%s, %q)", f.c.goType(to), e.code, to.String()))
	}

	if isOpt(e.typ) {
		f.errorf(n, "possibly-nil %s where %s is expected; check it first (`if x`, `x ||= ...`, `return unless x`)", e.typ, to)
	}
	if isNil(e.typ) {
		f.errorf(n, "nil where %s is expected", to)
	}
	if isClass(to, "Integer") && isClass(e.typ, "Float") {
		if f.pass == 2 { // earlier passes may still widen the target
			f.errorf(n, "Float where Integer is expected; convert it (to_i, round, floor)")
		}
	} else if !fitsValue(e, to) {
		f.errorf(n, "%s where %s is expected", e.typ, to)
	}
	if isAbstract(to) { // Go any: literals need wrapping, as for untyped
		return f.coerce(n, e, TAny{})
	}
	return e.code
}

// coerceArg coerces a call argument to its parameter. A `T | untyped`
// parameter holds typed arguments to T and passes untyped ones as they are.
func (f *fctx) coerceArg(n parser.Node, a expr, p Param, env map[string]Type) string {
	if p.Want != nil && !isAny(a.typ) {
		f.coerce(n, a, subst(p.Want, env)) // for its compile errors only
	}
	t := subst(p.Type, env)
	code := f.coerce(n, a, t)
	// A type variable bound to untyped gets an explicit any(...), or Go infers
	// it from the argument's own type (E = String for Array[untyped]#include?("a")).
	if _, ok := p.Type.(TVar); ok && isAny(t) && !isOpt(a.typ) && !isVoid(a.typ) && f.c.goType(a.typ) != "any" {
		return "any(" + code + ")"
	}
	// A literal for a type variable is an untyped Go constant, from which Go
	// would infer string or int, not String or Integer: name its type.
	if _, ok := p.Type.(TVar); ok && a.lit && typeEq(a.typ, t) && code == a.code {
		return f.c.goType(t) + "(" + code + ")"
	}
	return code
}

// converts reports whether values of t convert between instantiations
// (rbAs): Array and Hash, which every instantiation of implements
// _to_any and rbFrom.
func converts(t TClass) bool {
	return len(t.Args) > 0 && t.C.lookup("_to_any") != nil
}

// sameButUntyped reports whether a and b differ only where one of them is
// untyped (Hash[Symbol, Integer] and Hash[Symbol, untyped]).
func sameButUntyped(a, b Type) bool {
	if isAny(a) || isAny(b) || typeEq(a, b) {
		return true
	}
	switch a := a.(type) {
	case TClass:
		b, ok := b.(TClass)
		if !ok || a.C != b.C || len(a.Args) != len(b.Args) {
			return false
		}
		for i := range a.Args {
			if !sameButUntyped(a.Args[i], b.Args[i]) {
				return false
			}
		}
		return true
	case TOpt:
		b, ok := b.(TOpt)
		return ok && sameButUntyped(a.Elem, b.Elem)
	case TAny, TFunc, TNil, TTuple, TVar, TVoid: // the same only when typeEq, checked above
	}
	return false
}

// ---- calls

// genKernelIntrinsic handles the receiverless calls the compiler builds itself.
func (f *fctx) genKernelIntrinsic(n *parser.CallNode, expected Type) (expr, bool) {
	switch n.Name {
	case "raise", "fail":
		return f.genRaise(n), true
	case "require", "require_relative":
		return expr{code: "", stmt: true, typ: TVoid{}}, true
	case "__method__":
		if n.Arguments != nil || n.Block != nil {
			return expr{}, false
		}
		if f.m == nil {
			return expr{code: "nil", typ: TNil{}}, true
		}
		return expr{code: "Symbol(" + strconv.Quote(f.m.Name) + ")", typ: f.cls("Symbol")}, true
	case "__dir__":
		if n.Arguments != nil || n.Block != nil {
			return expr{}, false
		}
		return expr{code: "rbSourceDir(" + strconv.Quote(f.f.Name) + ")", typ: f.cls("String")}, true
	case "lambda", "proc":
		if bn, ok := n.Block.(*parser.BlockNode); ok && n.Arguments == nil {
			return f.genLambda(n, bn, bn.Parameters, expected), true
		}
		f.errorf(n, "%s needs a literal block", n.Name)
	case "block_given?", "binding", "send", "method_missing", "define_method":
		f.errorf(n, "%s is not supported", n.Name)
	}
	return expr{}, false
}

func (f *fctx) genCall(n *parser.CallNode, expected Type) expr {
	if n.Receiver == nil {
		if e, ok := f.genKernelIntrinsic(n, expected); ok {
			return e
		}
	}
	if n.Name == "call" && f.isBlockParam(n.Receiver) && n.Block == nil {
		return f.yieldValues(n, callArgs(n))
	}
	if e, ok := f.genBareName(n, expected); ok {
		return e
	}
	if strings.HasSuffix(n.Name, "=") && (n.Receiver == nil || isSelf(n.Receiver)) {
		f.unnarrow("attr:" + strings.TrimSuffix(n.Name, "="))
	}
	if e, ok := f.genERBCall(n); ok {
		return e
	}
	if cls := f.classRef(n.Receiver); cls != nil {
		if e, ok := f.genSpecialClassCall(n, cls); ok {
			return e
		}
		// Direct constructor unless Foo defines self.new; Hash is the one @go_type class with a Go constructor (NewHash).
		// A generic @go_type class's own self.new takes arguments; bare `.new` is its annotated zero value.
		if n.Name == "new" && (cls.meta == nil || isSynthNew(cls.meta.lookup("new")) || cls == f.c.classes["Hash"] || len(cls.TypeParams) > 0 && n.Arguments == nil) {
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
		recv = expr{code: f.selfCode, typ: f.selfType, classObj: f.selfClassObj}
	} else {
		recv = f.valueOf(f.genExpr(n.Receiver, nil))
	}
	f.noteElems(n)
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
	if !isOpt(recv.typ) && !isAny(recv.typ) {
		return f.genMethodCall(n, recv, n.Name, callArgs(n), n.Block)
	}
	rt := f.newTmp()
	inner := expr{code: rt, typ: recv.typ} // an untyped nil is nil too
	if o, ok := recv.typ.(TOpt); ok {
		inner = expr{code: "(*" + rt + ")", typ: o.Elem}
	}
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
	resT := optOf(probe.typ)
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
	case TAny, TFunc, TNil, TOpt, TTuple, TVoid: // no class here to look in: only a top-level def can answer
	}
	if e == nil {
		if td := f.c.topDefs[name]; td != nil {
			e = &entry{M: td}
		}
	}
	if e != nil {
		f.c.inferRet(e.M)
	}
	return e
}

// genMethodCall dispatches a call on an already-generated receiver.
func (f *fctx) genMethodCall(n parser.Node, recv expr, name string, args []parser.Node, block parser.Node) expr {
	if _, ok := recv.typ.(TVoid); ok { // a void call's value is nil, as in Ruby: run it, then call on nil (decision 119)
		recv = f.voidAsNil(recv)
	}
	if e, ok := f.genIntrinsic(n, recv, name, args, block); ok {
		return e
	}
	if recv.view != "" && block == nil {
		if e, ok := f.viewCall(n, recv, name, args); ok {
			return e
		}
	}
	return f.narrowRaises(name, args, f.dispatch(n, recv, name, args, block))
}

// narrowRaises types `assert_raises(K) { }` and `_ { }.must_raise(K)` with
// one class literal as K, not Exception (decision 116, #37): the result is
// an exception the assertion checked is a K, so the Go type assertion
// cannot fail.
func (f *fctx) narrowRaises(name string, args []parser.Node, e expr) expr {
	if name != "assert_raises" && name != "must_raise" || len(args) != 1 {
		return e
	}
	exc := f.c.classes["Exception"]
	got, ok := e.typ.(TClass)
	if !ok || got.C != exc {
		return e
	}
	cls := f.classRef(args[0])
	if cls == nil || cls == exc || !cls.isSubclassOf(exc) {
		return e
	}
	want := TClass{C: cls}
	return expr{code: "(" + e.code + ").(" + f.c.goType(want) + ")", typ: want, assert: true}
}

// genBareName is a receiverless, argumentless call that names a narrowed
// attribute, or a caller's local inside a template parsed on its own
// (decision 111).
func (f *fctx) genBareName(n *parser.CallNode, expected Type) (expr, bool) {
	if n.Receiver != nil || n.Arguments != nil || n.Block != nil {
		return expr{}, false
	}
	if v := f.scope.lookup("attr:" + n.Name); v != nil {
		return expr{code: v.goName, typ: v.typ}, true
	}
	if f.erbDepth > 0 && f.visibleLocal(n.Name) != nil {
		return f.genExpr(&parser.LocalVariableReadNode{Name: n.Name, Location: n.Location}, expected), true
	}
	return expr{}, false
}

// genSpecialClassCall is the class-method calls the compiler answers itself: Ractor.new (decision 103) and ERB.new (decision 111).
func (f *fctx) genSpecialClassCall(n *parser.CallNode, cls *Class) (expr, bool) {
	switch {
	case cls.RubyName == "Ractor":
		return f.genRactorCall(n, cls)
	case cls.RubyName == "ERB" && n.Name == "new":
		f.c.erbNewTemplate(f.f, n) // checked here; the object only marks the template
		return expr{code: "NewERB()", typ: TClass{C: cls}, ctor: true}, true
	}
	return expr{}, false
}

// viewCall sends a block-less call on a converted Array/Hash (expr.view)
// to the value itself, dynamically: the conversion is a copy unless the
// value already is Array[untyped], and Ruby's call would mutate the value.
// The result is typed as the typed call's. Calls that cannot be sent
// dynamically (blocks, generic methods) use the copy.
func (f *fctx) viewCall(n parser.Node, recv expr, name string, args []parser.Node) (expr, bool) {
	t, ok := recv.typ.(TClass)
	if !ok || !converts(t) {
		return expr{}, false
	}
	if e, private := f.c.dynEntry(t.C, name); e == nil || private {
		return expr{}, false
	}
	for _, a := range args {
		if _, ok := a.(*parser.SplatNode); ok {
			return expr{}, false
		}
	}
	var typed expr
	f.probe(func() { typed = f.dispatch(n, recv, name, args, nil) })
	f.c.noteDyn(name)
	codes := []string{"rbCall", recv.view}
	for _, a := range args {
		codes = append(codes, f.coerce(a, f.genExpr(a, nil), TAny{}))
	}
	raw := expr{code: "rbDyn" + goMethodName(name) + "(" + strings.Join(codes, ", ") + ")", typ: TAny{}}
	// The raw result stays the view: `x << 1 << 2` sends both to the value.
	return expr{code: f.coerce(n, raw, typed.typ), typ: typed.typ, view: raw.code}, true
}

// genIntrinsic compiles the methods the transpiler answers itself:
// class, with (Data), respond_to?, is_a?/kind_of?.
func (f *fctx) genIntrinsic(n parser.Node, recv expr, name string, args []parser.Node, block parser.Node) (expr, bool) {
	if name == "class" && len(args) == 0 && block == nil {
		if e, ok := f.genClassOf(n, recv); ok {
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
	if (name == "send" || name == "__send__" || name == "public_send") && len(args) >= 1 && !isRactorRecv(recv.typ) {
		return f.genSend(n, recv, name, args, block), true
	}
	if name == "is_a?" || name == "kind_of?" {
		if len(args) != 1 || block != nil {
			f.errorf(n, "%s takes one class", name)
		}
		return expr{code: "Boolean(" + f.isACheck(n, recv, args[0]) + ")", typ: f.cls("Boolean")}, true
	}
	// `!=` is `!(==)` unless overridden: BasicObject#!= as a free func over any would bind == to identity.
	if name == "!=" {
		if e := f.resolve(recv.typ, name); e == nil || e.Owner == f.c.classes["BasicObject"] {
			eq := f.genMethodCall(n, recv, "==", args, block)
			return expr{code: "Boolean(!(" + f.truthy(n, eq) + "))", typ: f.cls("Boolean")}, true
		}
	}
	return expr{}, false
}

// dispatch resolves a call by the receiver's static type.
func (f *fctx) dispatch(n parser.Node, recv expr, name string, args []parser.Node, block parser.Node) expr {
	switch t := recv.typ.(type) {
	case TOpt:
		return f.optCall(n, recv, name, args, block)
	case TTuple:
		return f.tupleCall(n, recv, name, args)
	case TFunc:
		if t.Proc {
			return f.procCall(n, recv, t, name, args, block)
		}
	case TClass:
		return f.classCall(n, t, recv, name, args, block)
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
			if f.c.isDelegator(f.owner) && f.c.classes["Object"].lookup(name) == nil {
				return f.delegateCall(n, recv, name, args, block)
			}
			if f.owner.isStruct() && f.owner.descendantDefines(name, false) && block == nil {
				return f.genDynCall(n, recv, name, args)
			}
			if e := f.c.classes["Object"].lookup(name); e != nil {
				return f.callEntry(n, e, recv, args, block)
			}
		}
		return f.universalCall(n, recv, name, args, block)
	case TAny, TNil:
		return f.universalCall(n, recv, name, args, block)
	case TVoid: // a void call's value is nil, as in Ruby: run it, then call on nil (`log(x).nil?`)
		return f.universalCall(n, f.voidAsNil(recv), name, args, block)
	}
	f.errorf(n, "undefined method %s for %s", name, recv.typ)
	return expr{}
}

// voidAsNil runs a void call as a statement and stands nil in for its value (decision 119).
func (f *fctx) voidAsNil(recv expr) expr {
	if recv.code != "" && !recv.done {
		f.emit("%s", recv.code)
	}
	return expr{code: "nil", typ: TNil{}}
}

// isDelegator reports whether cls is a Delegator (SimpleDelegator, a DelegateClass) or a subclass of one.
func (c *Compiler) isDelegator(cls *Class) bool {
	d := c.classes["Delegator"]
	return d != nil && cls != nil && cls != d && cls.isSubclassOf(d)
}

// delegateCall is a method a delegator doesn't define, compiled to the same
// call on its __getobj__ (decision 118): untyped, so dispatched at run time,
// for SimpleDelegator; typed for DelegateClass(Foo).
func (f *fctx) delegateCall(n parser.Node, recv expr, name string, args []parser.Node, block parser.Node) expr {
	obj := f.genMethodCall(n, recv, "__getobj__", nil, nil)
	return f.genMethodCall(n, obj, name, args, block)
}

// classCall dispatches on a class-typed receiver.
func (f *fctx) classCall(n parser.Node, t TClass, recv expr, name string, args []parser.Node, block parser.Node) expr {
	if isAbstract(t) && recv.code != f.selfCode {
		return f.abstractCall(n, t, recv, name, args, block)
	}
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
		if f.c.isDelegator(t.C) {
			return f.delegateCall(n, recv, name, args, block)
		}
		// a Module/Class-typed value is some class object, and a
		// struct-typed value may be a subclass defining the method:
		// either way the method is found at run time
		if (t.C.RubyName == "Module" || t.C.RubyName == "Class" || (t.C.isStruct() && t.C.descendantDefines(name, false))) && block == nil {
			return f.genDynCall(n, recv, name, args)
		}
		f.errorf(n, "undefined method %s for %s", name, recv.typ)
	}
	return f.numericMix(n, recv, e, args, block)
}

// abstractCall: an Object or module value is Go any, so it dispatches as untyped.
func (f *fctx) abstractCall(n parser.Node, t TClass, recv expr, name string, args []parser.Node, block parser.Node) expr {
	d := f.universalCall(n, recv, name, args, block)
	// the declared result keeps the caller typed instead of cascading dynamic calls
	if e := t.C.lookup(name); e != nil && isAny(d.typ) {
		f.c.inferRet(e.M)
		if ret := subst(e.M.Ret, e.Env); !isVoid(ret) && !mentionsVar(ret) {
			d = expr{code: f.coerce(n, d, ret), typ: ret}
		}
	}
	return d
}

// numericMix compiles a call on an Integer or Float whose numeric
// parameter gets the other class, as MRI's coerce does: the classes' own
// operators widen the Integer side to Float (still typed and unboxed), and
// Comparable's methods run on rbNum, since clamp hands back the winning
// argument itself. Arguments are generated once and passed on as exprNodes.
// Any other call is plain callEntry.
func (f *fctx) numericMix(n parser.Node, recv expr, e *entry, args []parser.Node, block parser.Node) expr {
	m := e.M
	if !isNumeric(recv.typ) || block != nil || len(args) == 0 || len(args) != len(m.Params) {
		return f.callEntry(n, e, recv, args, block)
	}
	self := map[string]Type{"Self": recv.typ}
	for i, p := range m.Params {
		if _, splat := args[i].(*parser.SplatNode); splat || p.Rest || !isNumeric(subst(p.Type, self)) {
			return f.callEntry(n, e, recv, args, block)
		}
	}
	xs := make([]expr, len(args))
	nodes := make([]parser.Node, len(args))
	mixed := false
	for i, a := range args {
		pt := subst(m.Params[i].Type, self)
		xs[i] = f.genExpr(a, pt)
		mixed = mixed || isNumeric(xs[i].typ) && !typeEq(xs[i].typ, pt)
		nodes[i] = &exprNode{Node: a, e: xs[i]}
	}
	if !mixed {
		return f.callEntry(n, e, recv, nodes, nil)
	}
	if m.Owner == f.c.classes["Comparable"] {
		codes := make([]string, len(xs))
		for i, x := range xs {
			codes[i] = f.coerce(args[i], x, TAny{})
		}
		return f.rbNumCall(n, m, f.coerce(n, recv, TAny{}), codes)
	}
	fe := f.c.classes["Float"].lookup(m.Name)
	if fe == nil || len(fe.M.Params) != len(args) {
		f.errorf(n, "%s#%s with an Integer and a Float is not supported", classOf(recv.typ).RubyName, m.Name)
	}
	widen := func(x expr) expr {
		if isClass(x.typ, "Integer") {
			return expr{code: "Float(" + x.code + ")", typ: f.cls("Float")}
		}
		return x
	}
	for i, x := range xs {
		nodes[i] = &exprNode{Node: args[i], e: widen(x)}
	}
	return f.callEntry(n, fe, widen(recv), nodes, nil)
}

// rbNumCall calls Comparable method m with Self = rbNum, one Go type for
// Integer and Float values (recv and args are untyped Go code).
func (f *fctx) rbNumCall(n parser.Node, m *Method, recv string, args []string) expr {
	boxed := make([]string, 0, 1+len(args))
	for _, a := range append([]string{recv}, args...) {
		boxed = append(boxed, "rbNum{"+a+"}")
	}
	code := freeFuncName(m) + "[rbNum](" + strings.Join(boxed, ", ") + ")"
	if v, ok := m.Ret.(TVar); ok && v.Name == "Self" {
		return expr{code: code + ".v", typ: TAny{}}
	}
	if m.generic() || mentionsVar(m.Ret) {
		f.errorf(n, "%s with an Integer and a Float is not supported", m.Name)
	}
	return expr{code: code, typ: m.Ret}
}

// genArgs generates and coerces call arguments against m's parameters,
// binding type variables in env. exprs, if non-nil, are pre-generated.
// genArgs also returns where *rest's codes start (-1 if none), so a block arg can splice in before them, not after.
func (f *fctx) genArgs(n parser.Node, m *Method, env map[string]Type, args []parser.Node, exprs []expr) ([]string, int) {
	var codes []string
	restIdx := -1
	var kw *kwArgs
	if exprs == nil && m.hasKeywords() {
		args, kw = f.splitKeywordArgs(m, args)
	}
	nargs := len(args)
	if exprs != nil {
		nargs = len(exprs)
	}
	npos := m.positionalCount()
	hasRest := slices.ContainsFunc(m.Params, func(p Param) bool { return p.Rest })
	expected := npos // what the messages have always counted: positional params, *rest as one
	if hasRest {
		expected++
	}
	// Too many arguments is checked first: it is the error Ruby raises, and
	// the surplus would otherwise be coerced to the wrong parameter's type.
	if nargs > npos && !hasRest {
		f.errorf(n, "%s: wrong number of arguments (given %d, expected %d)", m, nargs, expected)
	}
	if exprs == nil {
		f.optJoin(m, env, args)
	}
	ai := 0
	kwMask, kwBit := 0, 0
	for _, p := range m.Params {
		if p.Keyword {
			mark := f.buf.Len()
			code, given := f.keywordArg(n, m, p, env, kw)
			f.pinBefore(mark, codes)
			if given {
				kwMask |= 1 << kwBit
			}
			kwBit++
			codes = append(codes, code)
			continue
		}
		if p.Rest {
			restIdx = len(codes)
			mark := f.buf.Len()
			rest := f.genRestArgs(n, p, env, args, exprs, ai, nargs)
			f.pinBefore(mark, codes)
			codes = append(codes, rest...)
			ai = nargs
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
				mark := f.buf.Len()
				a = f.genExpr(an, closed(p.Type, env))
				f.pinBefore(mark, codes)
			}
			ai++
			unify(p.Type, a.typ, env)
			codes = append(codes, f.coerceArg(an, a, p, env))
			continue
		}
		if p.Default != nil && m.calleeDefaults {
			codes = append(codes, "rbZero["+f.c.goType(subst(p.Type, env))+"]()")
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
			codes = append(codes, f.coerceArg(n, d, p, env))
			continue
		}
		f.errorf(n, "%s: wrong number of arguments (given %d, expected %d)", m, nargs, expected)
	}
	kw.checkUnknown(f, m)
	if m.kwMask() {
		codes = append([]string{strconv.Itoa(kwMask)}, codes...)
		if restIdx >= 0 {
			restIdx++
		}
	}
	if m.calleeDefaults {
		codes = append([]string{strconv.Itoa(min(nargs, npos))}, codes...)
		if restIdx >= 0 {
			restIdx++
		}
	}
	return codes, restIdx
}

// kwArgs is a call's `name: value` arguments, matched to keyword parameters by name.
type kwArgs struct {
	named  map[string]*parser.AssocNode
	order  []*parser.AssocNode
	splats []parser.Node // `**h`
	all    []parser.Node // both, in source order: `**opts` keeps it
	used   map[string]bool
}

// splitKeywordArgs takes a trailing `k: v, **h` off args when m has keyword parameters (a braced Hash stays positional, as in Ruby 3).
func (f *fctx) splitKeywordArgs(m *Method, args []parser.Node) ([]parser.Node, *kwArgs) {
	kw := &kwArgs{named: map[string]*parser.AssocNode{}, used: map[string]bool{}}
	if len(args) == 0 {
		return args, kw
	}
	kh, ok := args[len(args)-1].(*parser.KeywordHashNode)
	if !ok {
		return args, kw
	}
	kw.all = kh.Elements
	for _, el := range kh.Elements {
		switch el := el.(type) {
		case *parser.AssocNode:
			sym, ok := el.Key.(*parser.SymbolNode)
			if !ok {
				f.errorf(el.Key, "%s takes keyword arguments: keys must be symbols", m.Name)
			}
			name := sym.Unescaped.Value
			if kw.named[name] != nil {
				f.errorf(el, "duplicated keyword %s:", name)
			}
			kw.named[name] = el
			kw.order = append(kw.order, el)
		case *parser.AssocSplatNode:
			kw.splats = append(kw.splats, el)
		default:
			f.errorf(el, "unsupported keyword argument")
		}
	}
	return args[:len(args)-1], kw
}

// keywordArg renders keyword parameter p's argument: the call's value, a
// default (callee-side ones get a zero value and a clear bit in rbKw), or
// for `**opts` the call's other keywords as a Hash.
func (f *fctx) keywordArg(n parser.Node, m *Method, p Param, env map[string]Type, kw *kwArgs) (string, bool) {
	if p.KwRest {
		var els []parser.Node
		for _, el := range kw.all {
			if a, ok := el.(*parser.AssocNode); !ok || !kw.used[a.Key.(*parser.SymbolNode).Unescaped.Value] {
				els = append(els, el)
			}
		}
		kw.splats = nil
		for _, a := range kw.order {
			kw.used[a.Key.(*parser.SymbolNode).Unescaped.Value] = true
		}
		h := f.genHash(n, els, closed(p.Type, env))
		unify(p.Type, h.typ, env)
		return f.coerceArg(n, h, p, env), true
	}
	if len(kw.splats) > 0 {
		f.errorf(kw.splats[0], "**splat into %s's named keyword parameters is not supported; pass them by name", m.Name)
	}
	if a := kw.named[p.Name]; a != nil {
		kw.used[p.Name] = true
		v := f.genExpr(a.Value, closed(p.Type, env))
		unify(p.Type, v.typ, env)
		return f.coerceArg(a.Value, v, p, env), true
	}
	switch {
	case p.Default != nil && m.calleeDefaults:
		return "rbZero[" + f.c.goType(subst(p.Type, env)) + "]()", false
	case p.Default != nil:
		caller := f.f
		f.f = m.File
		d := f.genExpr(p.Default, closed(p.Type, env))
		f.f = caller
		unify(p.Type, d.typ, env)
		return f.coerceArg(n, d, p, env), false
	}
	f.errorf(n, "%s: missing keyword: :%s", m, p.Name)
	return "", false
}

// checkUnknown rejects keywords the method has no parameter for (MRI's ArgumentError, at compile time).
func (kw *kwArgs) checkUnknown(f *fctx, m *Method) {
	if kw == nil {
		return
	}
	for _, a := range kw.order {
		if name := a.Key.(*parser.SymbolNode).Unescaped.Value; !kw.used[name] {
			f.errorf(a, "%s: unknown keyword: :%s", m, name)
		}
	}
	if len(kw.splats) > 0 {
		f.errorf(kw.splats[0], "%s takes no **keywords", m)
	}
}

// optJoin binds a method type variable that several arguments share as
// X? when they are X and X? (or nil): `assert_equal 3, h[:a]` takes T as
// Integer?, where binding T from the first argument would reject the
// second (decision 93).
func (f *fctx) optJoin(m *Method, env map[string]Type, args []parser.Node) {
	if len(m.TypeParams) == 0 {
		return
	}
	at := map[string][]int{}
	for i, p := range m.Params {
		if p.Rest || i >= len(args) {
			break
		}
		if v, ok := p.Type.(TVar); ok && slices.Contains(m.TypeParams, v.Name) {
			if _, bound := env[v.Name]; !bound {
				at[v.Name] = append(at[v.Name], i)
			}
		}
	}
	for name, idx := range at {
		if len(idx) < 2 {
			continue
		}
		var j Type
		for k, i := range idx {
			if _, ok := args[i].(*parser.SplatNode); ok {
				j = nil
				break
			}
			var a expr
			f.probe(func() { a = f.genExpr(args[i], nil) })
			if k == 0 {
				j = a.typ
				continue
			}
			var ok bool
			if j, ok = join(j, a.typ); !ok {
				j = nil
				break
			}
		}
		if o, ok := j.(TOpt); ok && !isAny(o.Elem) {
			env[name] = o
		}
	}
}

// genRestArgs: with a splat, one fresh slice, since Go spreads only a lone slice and Ruby's rest param never aliases the caller's array.
func (f *fctx) genRestArgs(n parser.Node, p Param, env map[string]Type, args []parser.Node, exprs []expr, ai, nargs int) []string {
	var codes []string
	var splats map[int]bool
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
				if splats == nil {
					splats = map[int]bool{}
				}
				splats[len(codes)] = true
				codes = append(codes, f.splatSlice(an, a.code, ac.Args[0], subst(p.Type, env)))
				continue
			}
			mark := f.buf.Len()
			a = f.genExpr(an, closed(p.Type, env))
			f.pinBefore(mark, codes)
		}
		unify(p.Type, a.typ, env)
		codes = append(codes, f.coerceArg(an, a, p, env))
	}
	if splats == nil {
		return codes
	}
	et := f.c.goType(subst(p.Type, env))
	var parts, lit []string
	flush := func() {
		if len(lit) > 0 {
			parts = append(parts, "[]"+et+"{"+strings.Join(lit, ", ")+"}")
			lit = nil
		}
	}
	for i, c := range codes {
		if splats[i] {
			flush()
			parts = append(parts, c)
			continue
		}
		lit = append(lit, c)
	}
	flush()
	return []string{"slices.Concat[[]" + et + "](" + strings.Join(parts, ", ") + ")..."}
}

// splatSlice converts elements only when the Go types differ ([]Integer is not []any).
func (f *fctx) splatSlice(n parser.Node, code string, from, to Type) string {
	ft, tt := f.c.goType(from), f.c.goType(to)
	if ft == tt {
		return "*" + code
	}
	conv := f.coerce(n, expr{code: "x", typ: from}, to)
	return "rbSplat(*" + code + ", func(x " + ft + ") " + tt + " { return " + conv + " })"
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

// nilableFetch reroutes Hash#fetch whose default may be nil: MRI returns that nil, so the result is V? (decision 12).
func (f *fctx) nilableFetch(m *Method, args []parser.Node, block parser.Node) *entry {
	if m.Name != "fetch" || m.Owner == nil || m.Owner.RubyName != "Hash" || len(args) != 2 || block != nil {
		return nil
	}
	var d expr
	f.probe(func() { d = f.genExpr(args[1], nil) })
	if !isNil(d.typ) && !isOpt(d.typ) && !isAny(d.typ) {
		return nil
	}
	return m.Owner.lookup("__fetch_opt")
}

// overload stands in for RBS overloads (decision 12): a call whose argument
// count m cannot take goes to the receiver class's `__<name>_<count>`, and
// one whose first argument is of class C to `__<name>_<c>` when that takes
// the call's argument count (C snake-cased:
// `__idx_range`, `__minus_time`), if defined. Operators use their Go name
// minus `Op_`.
func (f *fctx) overload(e *entry, recvT Type, args []parser.Node) *entry {
	m := e.M
	owner := m.Owner
	if rc, ok := recvT.(TClass); ok {
		owner = rc.C
	}
	if owner == nil || strings.HasPrefix(m.Name, "__") {
		return nil
	}
	base := strings.NewReplacer("?", "_q", "!", "_bang").Replace(m.Name)
	if op, ok := opNames[m.Name]; ok {
		base = strings.ToLower(strings.TrimPrefix(op, "Op_"))
	}
	name := "__" + base + "_"
	if r := f.literalOverload(m, owner, args); r != nil {
		return r
	}
	if r := f.selfOverload(m, owner, name, recvT, args); r != nil {
		return r
	}
	if f.curBlock != nil && m.Block == nil {
		return owner.lookup(name + "block") // `xs.sum { |x| x.price }`
	}
	if f.curBlock == nil && m.Block != nil {
		if r := owner.lookup(name + "enum"); r != nil {
			return r // `xs.each_slice(2)` without a block: an Array standing in for the Enumerator
		}
	}
	if r := owner.lookup(name + "same"); r != nil && len(args) >= 2 && f.sameArgs(r.M, args) {
		return r // `assert_equal 3, h[:a]`: both sides one static type
	}
	if len(args) >= 1 && slices.ContainsFunc(owner.methodSet(), func(x entry) bool { return strings.HasPrefix(x.M.Name, name) }) {
		var a expr
		f.probe(func() { a = f.genExpr(args[0], nil) })
		if c, ok := a.typ.(TClass); ok {
			if r := owner.lookup(name + snake(c.C.RubyName)); r != nil && len(args) >= requiredArgs(r.M) && len(args) <= len(r.M.Params) {
				return r
			}
		}
	}
	rest := slices.ContainsFunc(m.Params, func(p Param) bool { return p.Rest })
	if len(args) >= requiredArgs(m) && (rest || len(args) <= len(m.Params)) {
		return nil
	}
	return owner.lookup(name + strconv.Itoa(len(args)))
}

// literalOverload picks an overload from literal arguments: String#scan
// with a pattern that has groups yields each match's groups; OptionParser#on
// is typed by its switch strings (decision 101).
func (f *fctx) literalOverload(m *Method, owner *Class, args []parser.Node) *entry {
	switch {
	case m.Name == "scan" && owner.RubyName == "String" && len(args) == 1:
		if g, ok := f.literalGroups(args[0]); ok && g > 0 {
			return owner.lookup("__scan_groups")
		}
	case owner.RubyName == "OptionParser" && (m.Name == "on" || m.Name == "on_tail" || m.Name == "on_head"):
		if k := f.optKind(args); k != "" {
			return owner.lookup("__" + m.Name + "_" + k)
		}
	case owner.RubyName == "CSV" && owner.metaOf != nil:
		return f.csvOverload(m, owner, args)
	case owner.RubyName == "BigDecimal" && m.Name == "round" && len(args) == 1:
		// round(n) is a BigDecimal for n >= 1 and an Integer otherwise (decision 112): a literal n decides the type
		if lit, ok := args[0].(*parser.IntegerNode); ok {
			n, err := strconv.Atoi(f.f.text(lit.Location))
			if err == nil && n >= 1 {
				return owner.lookup("__round_digits")
			}
		}
	}
	return nil
}

// selfOverload picks, for a method whose own `@self` the receiver doesn't
// fit, the first `__<name>_*` sibling whose `@self` it does and that takes
// the call's arguments and block (`[[1], [2]].flatten` is
// `__flatten_nested`, decision 92).
func (f *fctx) selfOverload(m *Method, owner *Class, name string, recvT Type, args []parser.Node) *entry {
	if !owner.selfDefs {
		return nil
	}
	fits := func(x *Method) bool {
		f.c.resolveMethod(x)
		return x.SelfType != nil && (x.Block != nil) == (f.curBlock != nil) &&
			len(args) >= requiredArgs(x) && len(args) <= len(x.Params) &&
			unify(x.SelfType, recvT, map[string]Type{})
	}
	if m.SelfType != nil && fits(m) {
		return nil
	}
	for _, x := range owner.methodSet() {
		if strings.HasPrefix(x.M.Name, name) && fits(x.M) {
			return owner.lookup(x.M.Name)
		}
	}
	return nil
}

// sameArgs reports whether a call's first two arguments have one static
// type (T and T? or nil counting as T?) that m's first parameter takes
// and m takes the argument count: then its `__<name>_same` overload
// applies (decision 93).
func (f *fctx) sameArgs(m *Method, args []parser.Node) bool {
	f.c.resolveMethod(m)
	if len(args) < requiredArgs(m) || len(args) > len(m.Params) {
		return false
	}
	var a, b expr
	f.probe(func() {
		a = f.genExpr(args[0], nil)
		b = f.genExpr(args[1], nil)
	})
	ea, eb := a.typ, b.typ
	if o, ok := ea.(TOpt); ok {
		ea = o.Elem
	}
	if o, ok := eb.(TOpt); ok {
		eb = o.Elem
	}
	switch {
	case isNil(ea) && isNil(eb), isAny(ea), isAny(eb):
		return false
	case !isNil(ea) && !isNil(eb) && !typeEq(ea, eb):
		return false
	}
	j, ok := join(a.typ, b.typ)
	return ok && unify(m.Params[0].Type, j, map[string]Type{})
}

// mtLitAsserts are the assertions that send a Symbol, and their typed
// twin for a literal one (decision 93).
var mtLitAsserts = map[string]string{
	"assert_operator": "__assert_operator_lit", "refute_operator": "__assert_operator_lit",
	"assert_predicate": "__assert_predicate_lit", "refute_predicate": "__assert_predicate_lit",
	"assert_respond_to": "__assert_respond_to_lit", "refute_respond_to": "__assert_respond_to_lit",
}

// mtLiteral compiles minitest's `assert_operator a, :<, b` (and the
// predicate and respond_to forms) with a literal Symbol on a typed value
// into the call itself, `a < b`, handed to a typed twin that builds the
// message: no send by name at run time (decision 93).
func (f *fctx) mtLiteral(n parser.Node, e *entry, recv expr, args []parser.Node) (expr, bool) {
	lit, ok := mtLitAsserts[e.M.Name]
	if !ok || e.M.Owner == nil || e.M.Owner.RubyName != "Minitest::Test" || len(args) < 2 || len(args) > 4 {
		return expr{}, false
	}
	sym, ok := args[1].(*parser.SymbolNode)
	if !ok {
		return expr{}, false
	}
	var o1 expr
	f.probe(func() { o1 = f.genExpr(args[0], nil) })
	if isAny(o1.typ) || isVoid(o1.typ) {
		return expr{}, false
	}
	op := sym.Unescaped.Value
	refute := strings.HasPrefix(e.M.Name, "refute_")
	msgAt := 2
	if strings.HasSuffix(e.M.Name, "_operator") {
		if len(args) == 2 {
			lit = "__assert_predicate_lit" // assert_operator(o1, :even?) is assert_predicate
		} else {
			msgAt = 3
		}
	}
	if len(args) > msgAt+1 {
		return expr{}, false
	}
	target := e.M.Owner.lookup(lit)
	if target == nil {
		return expr{}, false
	}
	hold := func(x parser.Node) expr {
		v := f.genExpr(x, nil)
		t := f.newTmp()
		f.emit("%s := %s", t, f.materialize(v))
		return expr{code: t, typ: v.typ}
	}
	a := hold(args[0])
	opStr := expr{code: "String(" + strconv.Quote(op) + ")", typ: f.cls("String")}
	litArgs := make([]parser.Node, 0, 6)
	switch lit {
	case "__assert_operator_lit":
		b := hold(args[2])
		test := f.genMethodCall(n, a, op, []parser.Node{&exprNode{Node: args[2], e: b}}, nil)
		litArgs = append(litArgs, &exprNode{Node: args[0], e: a}, &exprNode{Node: args[1], e: opStr}, &exprNode{Node: args[2], e: b}, &exprNode{Node: n, e: test})
	case "__assert_predicate_lit":
		test := f.genMethodCall(n, a, op, nil, nil)
		litArgs = append(litArgs, &exprNode{Node: args[0], e: a}, &exprNode{Node: args[1], e: opStr}, &exprNode{Node: n, e: test})
	default:
		test := f.genMethodCall(n, a, "respond_to?", []parser.Node{sym}, nil)
		litArgs = append(litArgs, &exprNode{Node: args[0], e: a}, &exprNode{Node: args[1], e: opStr}, &exprNode{Node: n, e: test})
	}
	msg := parser.Node(&exprNode{Node: n, e: expr{code: "nil", typ: TNil{}}})
	if len(args) > msgAt {
		msg = args[msgAt]
	}
	litArgs = append(litArgs, msg, &exprNode{Node: n, e: expr{code: "Boolean(" + strconv.FormatBool(refute) + ")", typ: f.cls("Boolean")}})
	return f.callMethod(n, target, recv, litArgs, nil), true
}

// snake is a class name as a method-name part: `DateTime` → `date_time`.
func snake(s string) string {
	var b strings.Builder
	for i, r := range s {
		if unicode.IsUpper(r) {
			if i > 0 {
				b.WriteByte('_')
			}
			r = unicode.ToLower(r)
		}
		if r == ':' {
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

// bindTypeParams checks every type parameter of m is bound, first
// unifying m's return type with the call's expected type, if any
// (`SizedQueue.new(2) #: SizedQueue[Integer]`).
func (f *fctx) bindTypeParams(n parser.Node, m *Method, env map[string]Type) {
	if n == f.retHintNode && f.retHint != nil {
		unify(m.Ret, f.retHint, env)
	}
	for _, tp := range m.TypeParams {
		if _, ok := env[tp]; !ok {
			f.errorf(n, "cannot infer type parameter %s of %s", tp, m)
		}
	}
}

// callEntry calls e; a call to a `bot` method is noreturn, and Go must
// see its panic to know the statement list ends there.
func (f *fctx) callEntry(n parser.Node, e *entry, recv expr, args []parser.Node, block parser.Node) expr {
	if block == nil {
		if r, ok := f.mtLiteral(n, e, recv, args); ok {
			return r
		}
	}
	r := f.callMethod(n, e, recv, args, block)
	if e.M.noReturn && !r.noreturn {
		r = expr{code: r.code + "\npanic(\"rb2go: unreachable\")", typ: TVoid{}, stmt: true, noreturn: true}
	}
	return r
}

// callEnv binds the type variables a call to e on recv starts from: the
// receiver's class arguments, Self, and what its `@self` binds (decision 92).
func (f *fctx) callEnv(n parser.Node, e *entry, recv expr) map[string]Type {
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
	if m.SelfType != nil && !unify(m.SelfType, recv.typ, env) {
		f.errorf(n, "%s needs a receiver of type %s, not %s", m.Name, m.SelfType, recv.typ)
	}
	return env
}

// checkVisibility rejects at compile time what MRI's NoMethodError would: a private method with a receiver, a protected one from outside its owner's family.
func (f *fctx) checkVisibility(n parser.Node, m *Method, recv expr) {
	if recv.code == f.selfCode || f.implicitCall {
		return
	}
	if m.Private {
		f.errorf(n, "private method %s called on %s", m.Name, recv.typ)
	}
	if m.Protected && (f.owner == nil || !f.owner.isSubclassOf(m.Owner)) {
		f.errorf(n, "protected method %s called on %s", m.Name, recv.typ)
	}
}

func (f *fctx) callMethod(n parser.Node, e *entry, recv expr, args []parser.Node, block parser.Node) expr {
	m := e.M
	f.c.inferRet(m)
	if o := f.nilableFetch(m, args, block); o != nil {
		return f.callEntry(n, o, recv, args, block)
	}
	f.curBlock = block
	o := f.overload(e, recv.typ, args)
	f.curBlock = nil
	if o != nil {
		return f.callEntry(n, o, recv, args, block)
	}
	env := f.callEnv(n, e, recv)
	// bind vars visible in the current generic context so they count as bound
	f.checkVisibility(n, m, recv)
	codes, restIdx := f.genArgs(n, m, env, args, nil)
	if m.Block != nil {
		if m.Iterator {
			f.errorf(n, "%s is an iterator (its block returns void); call it as a statement with a block", m.Name)
		}
		blkCode := "nil"
		switch {
		case block != nil:
			blkCode = f.genClosure(n, block, m.Block, env)
		case !m.Block.Optional:
			f.errorf(n, "%s requires a block", m.Name)
		}
		if restIdx >= 0 {
			codes = slices.Insert(codes, restIdx, blkCode)
		} else {
			codes = append(codes, blkCode)
		}
	} else if block != nil {
		f.errorf(n, "%s does not take a block", m.Name)
	}
	f.bindTypeParams(n, m, env)
	ret := subst(m.Ret, env)
	out := expr{code: f.callCode(e, recv, codes, env), typ: ret}
	if m.Name == "const_get" && m.Owner.RubyName == "Module" {
		if t := f.constGetType(recv, args); t != nil {
			if isOpt(t) {
				// the constant table holds T? as T or nil (see Opt)
				out.code = f.coerce(n, expr{code: out.code, typ: TAny{}}, t)
			} else {
				out.code, out.assert = out.code+".("+f.c.goType(t)+")", true
			}
			out.typ = t
		}
	}
	// `klass.new` returns the hierarchy's root type; narrow to the class the
	// receiver is statically known to be.
	if m.Kind == kindSynth && m.Name == "new" {
		if meta := f.metaOfType(recv.typ); meta != nil && meta.metaOf != meta.metaOf.root() {
			out.typ = TClass{C: meta.metaOf}
			out.code, out.assert = out.code+".("+f.c.goType(out.typ)+")", true
		}
	}
	if v, ok := m.Ret.(TVar); ok && v.Name == "Self" && e.Owner != nil {
		if rc, ok := recv.typ.(TClass); ok && rc.C.isStruct() && e.Entry != rc.C {
			out.code, out.assert = out.code+".("+f.c.goType(recv.typ)+")", true
		}
	}
	return flatOpt(out)
}

// flatOpt collapses a nested optional result: a generic E? instantiated
// with E = T? is a Go **T, and with E = untyped a *any, but Ruby has one nil.
func flatOpt(e expr) expr {
	o, ok := e.typ.(TOpt)
	switch {
	case !ok:
	case isAny(o.Elem):
		return expr{code: "Opt(" + e.code + ")", typ: o.Elem, nilable: true}
	case isOpt(o.Elem):
		return expr{code: "rbFlat(" + e.code + ")", typ: o.Elem}
	}
	return e
}

// callCode renders the call for entry e.
func (f *fctx) callCode(e *entry, recv expr, args []string, env map[string]Type) string {
	m := e.M
	argList := strings.Join(args, ", ")
	recv.code = f.materialize(recv)
	if m.Owner == nil {
		return m.GoName + "(" + argList + ")"
	}
	if m.Owner.metaOf != nil && m.Name == "new" && !recv.classObj {
		// `new` on a class object of unknown exact class: assert for it.
		ps, ret := f.c.sig(m, env)
		return "any(" + recv.code + ").(interface{ New(" + ps + ") " + ret + " }).New(" + argList + ")"
	}
	// A primitive's non-direct method is called by its free func, never its forwarder, so the pruner drops unused forwarders (decision 86).
	direct := f.c.isDirectMethod(m)
	free := m.generic() || (m.Private && !direct) || (m.Owner.GoType == "" && !f.hasForwarder(recv.typ, e)) || (m.Owner.GoType != "" && !direct)
	if !free {
		if t := f.methodExprType(m, recv); t != "" {
			return t + "." + m.GoName + "(" + recv.code + comma(argList) + ")"
		}
		return recv.code + "." + m.GoName + "(" + argList + ")"
	}
	return staticCallCode(m, f.typeArgs(m, recv.typ, env), recv.code, argList)
}

// methodExprType is the Go type a call on a receiver of static type t is written as a method expression of
// (`String.Upcase(s)`, `FooI.Bar(x)`), so the pruner keeps the method on that type and its implementers alone
// instead of on every class with the name (decision 122); "" keeps the plain `x.M()` form: a generic class, a
// module constraint (no method expressions on type parameters), or a struct's Dyn wrapper body, which shared
// arms compare textually.
func (f *fctx) methodExprType(m *Method, recv expr) string {
	if f.plainCalls {
		return ""
	}
	switch t := recv.typ.(type) {
	case TClass:
		if t.C.universal || t.C.IsModule || len(t.C.TypeParams) > 0 {
			return ""
		}
		if t.C.metaOf != nil && m.Name == "new" { // `new` is not in a metaclass's interface (its signature is the class's own); a class object is the concrete *Foo_Meta
			return "(*" + t.C.Name + ")"
		}
		if gt := f.c.goType(t); strings.HasPrefix(gt, "*") {
			return "(" + gt + ")"
		} else {
			return gt
		}
	case TVar:
		if t.Name == "Self" && f.owner != nil && f.owner.isStruct() && !f.owner.universal && !f.owner.IsModule && len(f.owner.TypeParams) == 0 {
			return f.owner.Name + "I"
		}
	case TAny, TFunc, TNil, TOpt, TTuple, TVoid:
	}
	return ""
}

// typeArgs are explicit because Go would infer from argument Go types (untyped const, *Foo, *Foo_Meta), not the Ruby binding.
func (f *fctx) typeArgs(m *Method, recvT Type, env map[string]Type) string {
	ts := []Type{}
	if m.Owner.GoType == "" {
		ts = append(ts, recvT)
	}
	for _, p := range m.Owner.TypeParams {
		ts = append(ts, env[p])
	}
	for _, p := range m.TypeParams {
		ts = append(ts, env[p])
	}
	if len(ts) == 0 {
		return ""
	}
	gs := make([]string, len(ts))
	for i, t := range ts {
		if t == nil {
			return "" // unbound: leave it to Go's inference
		}
		if gs[i] = f.c.goType(t); gs[i] == "" {
			return ""
		}
	}
	return "[" + strings.Join(gs, ", ") + "]"
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
	case TAny, TFunc, TNil, TOpt, TTuple, TVoid: // no Go method set to forward to
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
		if len(ps.Optionals) > 0 || len(ps.Posts) > 0 || len(ps.Keywords) > 0 || ps.KeywordRest != nil || ps.Block != nil {
			f.errorf(b, "unsupported block parameter form")
		}
		var names []string
		for _, r := range ps.Requireds {
			switch rp := r.(type) {
			case *parser.RequiredParameterNode:
				names = append(names, rp.Name)
			case *parser.MultiTargetNode:
				// `|(k, v), acc|`: spelt "(k,v)", which no Ruby name can be
				if rp.Rest != nil || len(rp.Rights) > 0 {
					f.errorf(r, "unsupported block parameter form")
				}
				var sub []string
				for _, l := range rp.Lefts {
					lp, ok := l.(*parser.RequiredParameterNode)
					if !ok {
						f.errorf(l, "unsupported block parameter form")
					}
					sub = append(sub, lp.Name)
				}
				names = append(names, "("+strings.Join(sub, ",")+")")
			default:
				f.errorf(r, "unsupported block parameter form")
			}
		}
		switch r := ps.Rest.(type) {
		case nil, *parser.ImplicitRestNode: // `|a, |` takes the first element, as `|a|` would of a splat
		case *parser.RestParameterNode:
			name := ""
			if r.Name != nil {
				name = *r.Name
			}
			names = append(names, "*"+name) // "*" alone: an anonymous rest
		default:
			f.errorf(r, "unsupported block parameter form")
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

// bindRestParams binds `|a, *rest|`: one yielded tuple or Array is split
// across the params when there are leading ones (Ruby's auto-splat);
// otherwise rest collects the yielded values past the leading params.
func (f *fctx) bindRestParams(n parser.Node, names []string, yields []Type) ([]string, func()) {
	lead, rest := names[:len(names)-1], strings.TrimPrefix(names[len(names)-1], "*")
	bind := func(name string, e expr) {
		if name == "" {
			return
		}
		v := f.blockParam(name, e.typ)
		f.emit("%s := %s", v.goName, e.code)
		f.noteUnused(v)
	}
	arrayOf := func(parts []expr) expr {
		ts := make([]Type, len(parts))
		for i, p := range parts {
			ts[i] = p.typ
		}
		et := joinOrAny(ts)
		codes := make([]string, len(parts))
		for i, p := range parts {
			codes[i] = f.coerce(n, p, et)
		}
		return expr{code: fmt.Sprintf("&Array[%s]{%s}", f.c.goType(et), strings.Join(codes, ", ")), typ: TClass{C: f.c.classes["Array"], Args: []Type{et}}}
	}
	if len(yields) == 1 && len(lead) > 0 {
		p := f.newTmp()
		switch t := yields[0].(type) {
		case TTuple:
			if len(t.Elems) < len(lead) {
				f.errorf(n, "block takes %d params but the tuple has %d elements", len(lead), len(t.Elems))
			}
			return []string{p}, func() {
				parts := make([]expr, len(t.Elems))
				for i, et := range t.Elems {
					parts[i] = expr{code: fmt.Sprintf("%s.F%d", p, i), typ: et}
				}
				for i, nm := range lead {
					bind(nm, parts[i])
				}
				bind(rest, arrayOf(parts[len(lead):]))
			}
		case TClass:
			if t.C.RubyName == "Array" {
				return []string{p}, func() {
					for i, nm := range lead {
						bind(nm, flatOpt(expr{code: fmt.Sprintf("rbSplatAt(%s, %d)", p, i), typ: TOpt{Elem: t.Args[0]}}))
					}
					bind(rest, expr{code: fmt.Sprintf("rbMidSplat(%s, %d, 0)", p, len(lead)), typ: t})
				}
			}
		case TAny, TFunc, TNil, TOpt, TVar, TVoid: // a lone value: no splat
		}
	}
	if len(lead) > len(yields) {
		f.errorf(n, "block takes %d params but only %d values are yielded", len(lead), len(yields))
	}
	tmps := make([]string, len(yields))
	for i := range tmps {
		tmps[i] = f.newTmp()
	}
	return tmps, func() {
		parts := make([]expr, len(yields))
		for i, y := range yields {
			parts[i] = expr{code: tmps[i], typ: y}
		}
		for i, nm := range lead {
			bind(nm, parts[i])
		}
		bind(rest, arrayOf(parts[len(lead):]))
		for i := len(lead); i < len(tmps) && rest == ""; i++ {
			f.emit("_ = %s", tmps[i])
		}
	}
}

// bindBlockParams declares block params for the yielded types, returning
// the Go loop/closure parameter names and a destructuring prologue.
// bindTupleParams destructures one yielded tuple across several block params.
func (f *fctx) bindTupleParams(n parser.Node, names []string, tt TTuple) ([]string, func()) {
	if len(names) != len(tt.Elems) {
		f.errorf(n, "block takes %d params but the tuple has %d elements", len(names), len(tt.Elems))
	}
	p := f.newTmp()
	return []string{p}, func() {
		lhs := make([]string, 0, len(names))
		rhs := make([]string, 0, len(names))
		vars := make([]*local, 0, len(names))
		var nested []func()
		for i, nm := range names {
			if strings.HasPrefix(nm, "(") { // `|(k, v), i|`
				sub, t, src := strings.Split(strings.Trim(nm, "()"), ","), tt.Elems[i], fmt.Sprintf("%s.F%d", p, i)
				nested = append(nested, func() {
					gp, pro := f.bindBlockParams(n, sub, []Type{t})
					f.emit("%s := %s", gp[0], src)
					pro()
				})
				continue
			}
			v := f.blockParam(nm, tt.Elems[i])
			lhs = append(lhs, v.goName)
			rhs = append(rhs, fmt.Sprintf("%s.F%d", p, i))
			vars = append(vars, v)
		}
		if len(lhs) > 0 {
			f.emit("%s := %s", strings.Join(lhs, ", "), strings.Join(rhs, ", "))
		}
		for _, d := range nested {
			d()
		}
		for _, v := range vars {
			f.noteUnused(v)
		}
	}
}

// bindArraySplat splats a yielded Array[elem] across several block params.
func (f *fctx) bindArraySplat(names []string, elem Type) ([]string, func()) {
	p := f.newTmp()
	return []string{p}, func() {
		for i, nm := range names {
			e := flatOpt(expr{code: fmt.Sprintf("rbSplatAt(%s, %d)", p, i), typ: TOpt{Elem: elem}})
			v := f.blockParam(nm, e.typ)
			f.emit("%s := %s", v.goName, e.code)
			f.noteUnused(v)
		}
	}
}

func (f *fctx) bindBlockParams(n parser.Node, names []string, yields []Type) (goParams []string, prologue func()) {
	if len(names) > 0 && strings.HasPrefix(names[len(names)-1], "*") {
		return f.bindRestParams(n, names, yields)
	}
	if tt, ok := firstType(yields).(TTuple); ok && len(yields) == 1 && len(names) > 1 {
		return f.bindTupleParams(n, names, tt)
	}
	// Ruby splats a yielded Array across several block params; each gets
	// its element, or nil past the end.
	if ac, ok := firstType(yields).(TClass); len(yields) == 1 && len(names) > 1 && ok && ac.C.RubyName == "Array" && len(ac.Args) == 1 {
		return f.bindArraySplat(names, ac.Args[0])
	}
	if len(names) > len(yields) {
		f.errorf(n, "block takes %d params but only %d values are yielded", len(names), len(yields))
	}
	var destructure []func()
	for i := range yields {
		switch {
		case i < len(names) && strings.HasPrefix(names[i], "("):
			p := f.newTmp()
			goParams = append(goParams, p)
			sub, t := strings.Split(strings.Trim(names[i], "()"), ","), yields[i]
			destructure = append(destructure, func() {
				gp, pro := f.bindBlockParams(n, sub, []Type{t})
				f.emit("%s := %s", gp[0], p)
				pro()
			})
		case i < len(names):
			v := f.blockParam(names[i], yields[i])
			goParams = append(goParams, v.goName)
		default:
			goParams = append(goParams, "_")
		}
	}
	return goParams, func() {
		for _, d := range destructure {
			d()
		}
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
	case *parser.LambdaNode:
		names = f.blockParamNames(b.Parameters)
		body = b.Body
	case *parser.BlockArgumentNode:
		if f.isBlockParam(b.Expression) {
			return f.forwardClosure(b, sig, env)
		}
		sym, ok := b.Expression.(*parser.SymbolNode)
		if !ok {
			if pb := f.procBlock(b); pb != nil {
				return f.genClosure(n, pb, sig, env)
			}
			f.errorf(b, "only &:symbol, a Proc and a method's own &block are supported as block arguments")
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
	// A block is its own Go function: a begin around the call (or in the
	// method) must not route the block's value into the enclosing result.
	// A begin in the block keeps the value in the closure's own ret_.
	savedBegins, savedRetVar, savedWrap := f.begins, f.retVar, f.wrap
	f.begins, f.retVar, f.wrap = 0, "ret_", nil
	defer func() { f.begins, f.retVar, f.wrap = savedBegins, savedRetVar, savedWrap }()
	// probe the body's type if the block return has unbound vars
	ret := closed(sig.Ret, env)
	if ret == nil {
		var types []Type
		f.probe(func() {
			saved, savedRuby := f.enterRubyBlock(block, names)
			f.closures++
			f.pushLoop(loopClosure)
			_, pro := f.bindBlockParams(n, names, params)
			pro()
			f.withNextTail(tail{kind: tailReturn, types: &types}, gen)
			f.popLoop()
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
		// the probe typed `[k, v]` without the tuple it is wanted as: a
		// same-typed pair reads as Array[X], which the real pass builds as [X, X]
		if tt, ok := sig.Ret.(TTuple); ok {
			if ga, ok := got.(TClass); ok && ga.C.RubyName == "Array" && len(ga.Args) == 1 {
				elems := make([]Type, len(tt.Elems))
				for i := range elems {
					elems[i] = ga.Args[0]
				}
				got = TTuple{Elems: elems}
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
	saved, savedRuby := f.enterRubyBlock(block, names)
	f.closures++
	f.pushLoop(loopClosure)
	goParams, pro := f.bindBlockParams(n, names, params)
	ps := make([]string, 0, len(goParams))
	for i, gp := range goParams {
		ps = append(ps, gp+" "+f.c.goType(params[i]))
	}
	retS := ""
	if !isVoid(ret) {
		retS = " " + f.c.goType(ret)
		if containsRescue(body) {
			retS = " (ret_" + retS + ")"
		}
	}
	f.emit("func(%s)%s {", strings.Join(ps, ", "), retS)
	f.indent++
	pro()
	f.hoistLocals(f.rbFrames[len(f.rbFrames)-1].key)
	if isVoid(ret) {
		f.withNextTail(tail{}, gen)
	} else {
		f.withNextTail(tail{kind: tailReturn, typ: ret}, gen)
	}
	f.indent--
	if strings.Contains(b.String(), "\n//line ") {
		// the block's statements moved the line mapping; what follows it on the Go side is the call's line again. Only then: a block without its own directives (&:sym) would look to linters like it ends in blank lines.
		f.lineOf(n)
	}
	f.emit("}")
	f.popLoop()
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
// A T? receiver is guarded first: `&.` skips the loop on nil, `.` raises.
func (f *fctx) genIterCall(n *parser.CallNode, t tail) bool {
	if cls := f.classRef(n.Receiver); cls != nil && cls.meta == nil {
		return false
	}
	var recvT Type
	if n.Receiver == nil {
		recvT = f.selfType
	} else {
		f.probe(func() { recvT = f.genExpr(n.Receiver, nil).typ })
	}
	opt, isOptRecv := recvT.(TOpt)
	if isOptRecv {
		recvT = opt.Elem
	}
	e := f.resolve(recvT, n.Name)
	if e == nil || !e.M.Iterator {
		return false
	}
	if rc, ok := recvT.(TClass); ok { // literal options pick a twin here too (CSV.foreach(path, headers: true), decision 117)
		if o := f.literalOverload(e.M, rc.C, callArgs(n)); o != nil && o.M.Iterator {
			e = o
		}
	}
	if t.kind != tailNone && t.typ != nil && !isVoid(t.typ) {
		f.errorf(n, "the value of an iterator call (%s) cannot be used", n.Name)
	}
	var recv expr
	if n.Receiver == nil {
		recv = expr{code: f.selfCode, typ: f.selfType, classObj: f.selfClassObj}
	} else {
		recv = f.genExpr(n.Receiver, nil)
	}
	safe := isOptRecv && n.IsSAFE_NAVIGATION()
	switch {
	case safe:
		rt := f.newTmp()
		f.emit("if %s := %s; %s != nil {", rt, recv.code, rt)
		f.indent++
		recv = expr{code: "(*" + rt + ")", typ: opt.Elem}
	case isOptRecv:
		recv = f.nilGuard(n, recv, n.Name)
	}
	f.genIterLoop(n, e, recv)
	if safe {
		f.indent--
		f.emit("}")
	}
	if t.kind != tailNone {
		f.emptyTail(n, t)
	}
	return true
}

// iterEnv is the type environment of iterator entry e called on recv.
func iterEnv(e *entry, recv expr) map[string]Type {
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
	return env
}

// genFor is `for x in coll`: coll.each as a range loop whose variables and
// body locals stay in the enclosing scope, since Ruby's for opens no block.
func (f *fctx) genFor(n *parser.ForNode, t tail) {
	var recvT Type
	f.probe(func() { recvT = f.genExpr(n.Collection, nil).typ })
	e := f.resolve(recvT, "each")
	if e == nil || !e.M.Iterator {
		f.errorf(n.Collection, "for needs a collection whose each is an iterator (Array, Range, Hash, Set, ...), not %s", recvT)
	}
	recv := f.genExpr(n.Collection, nil)
	env := iterEnv(e, recv)
	yields := substAll(e.M.Block.Params, env)
	call := f.callCode(e, recv, nil, env)
	tmps := make([]string, len(yields))
	for i := range tmps {
		tmps[i] = f.newTmp()
	}
	f.pushLoop(loopIter)
	saved := f.enterBlock()
	if len(tmps) == 0 {
		f.emit("for range %s {", call)
	} else {
		f.emit("for %s := range %s {", strings.Join(tmps, ", "), call)
	}
	f.indent++
	vals := make([]expr, len(yields))
	for i, y := range yields {
		vals[i] = expr{code: tmps[i], typ: y}
	}
	switch ix := n.Index.(type) {
	case *parser.LocalVariableTargetNode:
		if len(vals) != 1 {
			f.errorf(ix, "for with one variable over a %d-value each", len(vals))
		}
		f.assignLocal(ix, ix.Name, vals[0], nil)
	case *parser.MultiTargetNode:
		if len(vals) == 1 {
			f.genMultiWrite(&parser.MultiWriteNode{Location: ix.Location, Lefts: ix.Lefts, Rest: ix.Rest, Rights: ix.Rights, Value: &exprNode{Node: ix, e: vals[0]}})
			break
		}
		if ix.Rest != nil || len(ix.Rights) > 0 || len(ix.Lefts) != len(vals) {
			f.errorf(ix, "for needs %d variables here", len(vals))
		}
		for i, l := range ix.Lefts {
			lt, ok := l.(*parser.LocalVariableTargetNode)
			if !ok {
				f.errorf(l, "unsupported for variable %s", nodeType(l))
			}
			f.assignLocal(lt, lt.Name, vals[i], nil)
		}
	default:
		f.errorf(n.Index, "unsupported for variable %s", nodeType(n.Index))
	}
	f.genStmts(n.Statements, tail{})
	f.indent--
	f.leaveBlock(saved)
	f.emit("}")
	f.popLoop()
	f.emptyTail(n, t)
}

// genIterLoop emits the range loop of iterator entry e on recv.
func (f *fctx) genIterLoop(n *parser.CallNode, e *entry, recv expr) {
	m := e.M
	env := iterEnv(e, recv)
	codes, _ := f.genArgs(n, m, env, callArgs(n), nil)
	yields := substAll(m.Block.Params, env)
	call := f.callCode(e, recv, codes, env)
	blk, ok := n.Block.(*parser.BlockNode)
	if !ok {
		f.forwardIter(n, call, yields)
		return
	}
	names := f.blockParamNames(blk.Parameters)
	saved, savedRuby := f.enterRubyBlock(blk, names)
	goParams, pro := f.bindBlockParams(n, names, yields)
	allBlank := true
	for _, gp := range goParams {
		if gp != "_" {
			allBlank = false
		}
	}
	f.pushLoop(loopIter)
	if allBlank {
		f.emit("for range %s {", call)
	} else {
		f.emit("for %s := range %s {", strings.Join(goParams, ", "), call)
	}
	f.indent++
	pro()
	f.hoistLocals(f.rbFrames[len(f.rbFrames)-1].key)
	f.genStmts(blk.Body, tail{})
	f.indent--
	f.leaveRubyBlock(saved, savedRuby)
	f.emit("}")
	f.popLoop()
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
		f.indent++
		f.emitReturn()
		f.indent--
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
	if e != nil {
		f.c.inferRet(e.M)
	}
	if e == nil && f.m.superBridge {
		// the target depends on the includer: its bridge calls it (superBridges)
		env := map[string]Type{"Self": f.selfType}
		codes, _ := f.genArgs(n, f.m, env, f.superArgs(n, args, forwarding), nil)
		return expr{code: f.selfCode + "." + bridgeName(f.m) + "(" + strings.Join(codes, ", ") + ")", typ: subst(f.m.Ret, env)}
	}
	if e == nil {
		switch f.m.Name {
		case "initialize":
			return expr{code: "", typ: TVoid{}, stmt: true}
		case "respond_to_missing?": // Object's answers false
			return expr{code: "Boolean(false)", typ: f.cls("Boolean")}
		case "method_missing": // Object's raises NoMethodError for the name
			var name parser.Node
			if forwarding && len(f.m.Params) > 0 {
				name = &parser.LocalVariableReadNode{Name: f.m.Params[0].Name, Location: n.(*parser.ForwardingSuperNode).Location}
			} else if args != nil && len(args.Arguments) > 0 {
				name = args.Arguments[0]
			}
			if name == nil {
				f.errorf(n, "super in method_missing needs the method name")
			}
			sym := f.coerce(name, f.genExpr(name, f.cls("Symbol")), f.cls("Symbol"))
			return expr{code: "panic(rbNoMethod(string(" + sym + "), " + f.selfCode + ", false))", typ: TVoid{}, stmt: true, noreturn: true}
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
	codes, _ := f.genArgs(n, e.M, env, f.superArgs(n, args, forwarding), nil)
	if e.M.Block != nil {
		f.errorf(n, "super to a block-taking method is not supported")
	}
	code := staticCallCode(e.M, "", f.selfCode, strings.Join(codes, ", "))
	return expr{code: code, typ: subst(e.M.Ret, env)}
}

// superArgs is what `super` passes: its arguments, or for a zsuper the
// current params re-read as locals, so genArgs coerces them to the
// parent's types and fills the parent's remaining defaults.
func (f *fctx) superArgs(n parser.Node, args *parser.ArgumentsNode, forwarding bool) []parser.Node {
	if !forwarding {
		if args == nil {
			return nil
		}
		return args.Arguments
	}
	var an []parser.Node
	var kws []parser.Node // zsuper passes keywords on by name
	loc := n.(*parser.ForwardingSuperNode).Location
	for _, p := range f.m.Params {
		var a parser.Node = &parser.LocalVariableReadNode{Name: p.Name, Location: loc}
		switch {
		case p.KwRest:
			kws = append(kws, &parser.AssocSplatNode{Value: a, Location: loc})
			continue
		case p.Keyword:
			key := &parser.SymbolNode{Location: loc, Unescaped: parser.RubyString{Value: p.Name}}
			kws = append(kws, &parser.AssocNode{Key: key, Value: a, Location: loc})
			continue
		case p.Rest:
			a = &parser.SplatNode{Expression: a, Location: loc}
		}
		an = append(an, a)
	}
	if len(kws) > 0 {
		an = append(an, &parser.KeywordHashNode{Elements: kws, Location: loc})
	}
	return an
}

func (f *fctx) genNew(n parser.Node, cls *Class, args []parser.Node, exprs []expr, expected Type) expr {
	if c, ok := n.(*parser.CallNode); ok && c.Receiver != nil && !f.insideClass(cls) {
		for k := cls; k != nil; k = k.Super {
			if k.privateNew {
				f.errorf(n, "private method 'new' called for class %s", cls.RubyName)
			}
		}
	}
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
		codes, _ = f.genArgs(n, init.M, env, args, exprs)
	} else if len(args) > 0 || len(exprs) > 0 {
		f.errorf(n, "%s.new takes no arguments", cls.Name)
	}
	return expr{code: "New" + cls.Name + "(" + strings.Join(codes, ", ") + ")", typ: TClass{C: cls}, ctor: true}
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
	code := val.code
	if f.rescues > 0 {
		code = "rbWithCause(" + code + ", r_)" // raised while handling r_: MRI's cause
	}
	return expr{code: "panic(" + code + ")", typ: TVoid{}, stmt: true, noreturn: true}
}

// ---- calls on nilable, tuple and untyped receivers

func (f *fctx) optCall(n parser.Node, recv expr, name string, args []parser.Node, block parser.Node) expr {
	switch name {
	case "nil?":
		return expr{code: "Boolean(" + recv.code + " == nil)", typ: f.cls("Boolean")}
	case "to_s":
		return expr{code: "rbToS(Opt(" + recv.code + "))", typ: f.cls("String")}
	case "inspect":
		return expr{code: "rbInspect(Opt(" + recv.code + "))", typ: f.cls("String")}
	case "to_json":
		return expr{code: "rbToJson(" + strings.Join(append([]string{"Opt(" + recv.code + ")"}, f.jsonArgs(args)...), ", ") + ")", typ: f.cls("String")}
	case "==", "equal?":
		if len(args) != 1 {
			f.errorf(n, "%s takes one argument", name)
		}
		a := f.genExpr(args[0], nil)
		arg := f.coerce(args[0], a, TAny{})
		if name == "equal?" {
			return expr{code: "Boolean(rbIdentical(Opt(" + recv.code + "), " + arg + "))", typ: f.cls("Boolean")}
		}
		return expr{code: "rbEq[any](Opt(" + recv.code + "), " + arg + ")", typ: f.cls("Boolean")}
	case "!":
		if isClass(recv.typ.(TOpt).Elem, "Boolean") {
			return expr{code: "Boolean(!" + optTruthy(recv.code, recv.typ) + ")", typ: f.cls("Boolean")}
		}
		return expr{code: "Boolean(" + recv.code + " == nil)", typ: f.cls("Boolean")}
	case "to_i", "to_f", "to_a", "to_h", "=~":
		if e, ok := f.nilClassCall(n, recv, name, args); ok {
			return e
		}
	}
	return f.genMethodCall(n, f.nilGuard(n, recv, name), name, args, block)
}

// nilGuard emits Ruby's NoMethodError for a call on a nil T? receiver and
// returns the receiver dereferenced to T.
func (f *fctx) nilGuard(n parser.Node, recv expr, name string) expr {
	elem := recv.typ.(TOpt).Elem
	f.warn(n, "%s called on possibly-nil %s (raises NoMethodError on nil)", name, elem)
	code := recv.code
	if !isSimpleGo(code) {
		code = f.newTmp()
		f.emit("%s := %s", code, recv.code)
	}
	f.emit("if %s == nil {", code)
	f.emit("\tpanic(rbNoMethod(%q, nil, false))", name)
	f.emit("}")
	return expr{code: "(*" + code + ")", typ: elem}
}

// nilClassCall: nil answers to_i/=~/... itself (0, nil), not NoMethodError, when T's method's type can hold that answer.
func (f *fctx) nilClassCall(n parser.Node, recv expr, name string, args []parser.Node) (expr, bool) {
	if (name == "=~") != (len(args) == 1) || len(args) > 1 {
		return expr{}, false
	}
	rt := f.newTmp()
	inner := expr{code: "(*" + rt + ")", typ: recv.typ.(TOpt).Elem}
	var probe expr
	f.probe(func() { probe = f.genMethodCall(n, inner, name, args, nil) })
	t := probe.typ
	var zero string // "" is Go's zero value
	switch {
	case name == "to_i" && isClass(t, "Integer"), name == "to_f" && isClass(t, "Float"),
		name == "=~" && (isOpt(t) || isAny(t)):
	case name == "to_a" && isClass(t, "Array"):
		zero = "(&Array[" + f.c.goType(t.(TClass).Args[0]) + "]{})"
	case name == "to_h" && isClass(t, "Hash"):
		a := t.(TClass).Args
		zero = "NewHash[" + f.c.goType(a[0]) + ", " + f.c.goType(a[1]) + "]()"
	default:
		return expr{}, false
	}
	// ponytail: =~'s argument is only evaluated when the receiver is non-nil; hoist it if a side-effecting pattern ever matters
	tmp := f.newTmp()
	f.emit("var %s %s", tmp, f.c.goType(t))
	f.emit("if %s := %s; %s != nil {", rt, recv.code, rt)
	f.indent++
	e := f.genMethodCall(n, inner, name, args, nil)
	f.emit("%s = %s", tmp, f.coerce(n, e, t))
	f.indent--
	if zero != "" {
		f.emit("} else {")
		f.emit("\t%s = %s", tmp, zero)
	}
	f.emit("}")
	return expr{code: tmp, typ: t}, true
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
	case "to_json":
		return expr{code: recv.code + ".ToJson(" + strings.Join(f.jsonArgs(args), ", ") + ")", typ: f.cls("String")}
	case "<=>":
		a := f.genExpr(args[0], recv.typ)
		return expr{code: recv.code + ".Op_cmp(" + f.coerce(args[0], a, recv.typ) + ")", typ: f.cls("Integer")}
	case "==":
		a := f.genExpr(args[0], nil)
		code := a.code // the same tuple type compares field-wise, without converting
		if !typeEq(a.typ, recv.typ) {
			code = f.coerce(args[0], a, TAny{})
		}
		return expr{code: recv.code + ".Op_eq(" + code + ")", typ: f.cls("Boolean")}
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
		return expr{code: "rbToJson(" + strings.Join(append([]string{recv.code}, f.jsonArgs(args)...), ", ") + ")", typ: f.cls("String")}
	case "nil?":
		return expr{code: "Boolean(rbUnbox(any(" + recv.code + ")) == nil)", typ: f.cls("Boolean")} // T may be X?, a *X box
	case "!":
		return expr{code: "Boolean(!rbTruthy(" + recv.code + "))", typ: f.cls("Boolean")}
	case "==":
		a := one(recv.typ)
		code := "rbEq(" + recv.code + ", " + f.coerce(args[0], a, recv.typ) + ")"
		if isAny(recv.typ) || isNil(recv.typ) || isAbstract(recv.typ) {
			code = "rbEq[any](" + recv.code + ", " + f.coerce(args[0], a, TAny{}) + ")"
		}
		return expr{code: code, typ: f.cls("Boolean")}
	case "equal?":
		a := one(nil)
		return expr{code: "Boolean(rbIdentical(" + recv.code + ", " + f.coerce(args[0], a, TAny{}) + "))", typ: f.cls("Boolean")}
	case "<=>":
		if isAny(recv.typ) || isNil(recv.typ) {
			break // DynOp_cmp: untyped, since MRI answers nil for incomparable values
		}
		a := one(recv.typ)
		return expr{code: "rbCmp(" + recv.code + ", " + f.coerce(args[0], a, recv.typ) + ")", typ: f.cls("Integer")}
	case "hash":
		return expr{code: "rbHash(" + recv.code + ")", typ: f.cls("Integer")}
	}
	if isNil(recv.typ) && len(args) == 0 {
		if t, ok := map[string]Type{"to_a": TClass{C: f.c.classes["Array"], Args: []Type{TAny{}}}, "to_h": TClass{C: f.c.classes["Hash"], Args: []Type{TAny{}, TAny{}}}, "to_i": f.cls("Integer"), "to_f": f.cls("Float")}[name]; ok {
			f.discard(recv)
			return expr{code: nilConversions[name], typ: t}
		}
	}
	if recv.nilable {
		f.warn(n, "%s called on possibly-nil untyped (raises NoMethodError on nil)", name)
	}
	return f.genDynCall(n, recv, name, args)
}

// nilConversions are NilClass's conversions, for nil held statically or untyped.
var nilConversions = map[string]string{"to_a": "(&Array[any]{})", "to_h": "NewHash[any, any]()", "to_i": "Integer(0)", "to_f": "Float(0)"}

// blockParam declares a block parameter: a fresh local that Go syntax
// declares, so it is never hoisted.
func (f *fctx) blockParam(name string, typ Type) *local {
	key := f.localKey(name)
	info := f.locals[key]
	if info == nil {
		info = &localInfo{}
		f.locals[key] = info
	}
	info.noHoist = true
	v := f.declareLocal(name, typ)
	v.declared = true
	return v
}

// insideClass reports code in cls's own class or instance methods (or a subclass's), where a private class method may be called.
func (f *fctx) insideClass(cls *Class) bool {
	o := f.owner
	if o != nil && o.metaOf != nil {
		o = o.metaOf
	}
	return o != nil && o.isSubclassOf(cls)
}

// definedKind answers `defined?(v)` at compile time: the closed world knows
// every local, constant and method. "" is nil. Run-time state (whether an
// ivar was ever assigned, whether a block was passed) is not tracked.
func (f *fctx) definedKind(v parser.Node) string {
	switch v := v.(type) {
	case *parser.ParenthesesNode:
		if st, ok := v.Body.(*parser.StatementsNode); ok && len(st.Body) == 1 {
			return f.definedKind(st.Body[0])
		}
		return "expression"
	case *parser.LocalVariableReadNode:
		if e := f.genExpr(v, nil); e.code != "" {
			f.emit("_ = %s", e.code) // read only by defined?: Go still wants it used
		}
		return "local-variable"
	case *parser.ConstantReadNode, *parser.ConstantPathNode:
		if cls, k := f.c.lookupConst(f.f, v, f.lex); cls != nil || k != nil {
			return "constant"
		}
		return ""
	case *parser.SelfNode:
		return "self"
	case *parser.NilNode:
		return "nil"
	case *parser.TrueNode:
		return "true"
	case *parser.FalseNode:
		return "false"
	case *parser.SuperNode, *parser.ForwardingSuperNode:
		if f.m != nil && f.owner != nil && f.c.inheritedSig(f.m) != nil {
			return "super"
		}
		return ""
	case *parser.LocalVariableWriteNode, *parser.InstanceVariableWriteNode, *parser.ConstantWriteNode,
		*parser.LocalVariableOperatorWriteNode, *parser.LocalVariableOrWriteNode, *parser.LocalVariableAndWriteNode,
		*parser.InstanceVariableOperatorWriteNode, *parser.InstanceVariableOrWriteNode, *parser.MultiWriteNode:
		return "assignment"
	case *parser.ClassVariableReadNode:
		if f.lookupClassVar(v.Name) != nil {
			return "class variable"
		}
		return ""
	case *parser.InstanceVariableReadNode, *parser.GlobalVariableReadNode:
		f.errorf(v, "defined?(%s) depends on whether it was ever assigned, which rb2go does not track", f.f.text(v.GetLocation()))
	case *parser.YieldNode:
		f.errorf(v, "defined?(yield) is not supported (block_given? is not either)")
	case *parser.CallNode:
		return f.definedCall(v)
	}
	return "expression"
}

// definedCall is `defined?(recv.name)`: "method" when the receiver's static type has a public name (any visibility without a receiver).
func (f *fctx) definedCall(v *parser.CallNode) string {
	if v.Receiver == nil {
		switch {
		case f.c.topDefs[v.Name] != nil, f.resolve(f.selfType, v.Name) != nil:
			return "method"
		case slices.Contains([]string{"raise", "fail", "require", "require_relative", "lambda", "proc"}, v.Name):
			return "method"
		}
		return ""
	}
	if f.definedKind(v.Receiver) == "" {
		return ""
	}
	if cls := f.classRef(v.Receiver); cls != nil {
		if v.Name == "new" && !cls.IsModule || cls.meta != nil && f.resolve(TClass{C: cls.meta}, v.Name) != nil {
			return "method"
		}
		return ""
	}
	var rt Type
	f.probe(func() { rt = f.genExpr(v.Receiver, nil).typ })
	if isAny(rt) {
		f.errorf(v, "defined?(...%s) on an untyped receiver is not supported; use respond_to?", v.Name)
	}
	e := f.resolve(stripOpt(rt), v.Name)
	if e == nil || e.M.Private {
		return ""
	}
	return "method"
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
	case k != nil && k.guarded:
		code := fmt.Sprintf("rbConstRead(%s, %s, %q)", constSet(k), k.GoName, f.constMissing(n))
		return expr{code: code, typ: f.c.constType(k)}
	case k != nil:
		return expr{code: k.GoName, typ: f.c.constType(k)}
	case cls != nil && cls.meta != nil:
		return expr{code: classVar(cls), typ: TClass{C: cls.meta}, classObj: true}
	case cls != nil:
		f.errorf(n, "class %s used as a value is not supported", cls.RubyName)
	}
	if r, ok := n.(*parser.ConstantReadNode); ok && r.Name == "DATA" && f.f.data != nil {
		return f.genData()
	}
	f.errorf(n, "uninitialized constant %s", f.f.text(n.GetLocation()))
	return expr{}
}

// genData is the main file's DATA: one StringIO over the text after `__END__` (MRI's is a File at that offset; reading it reads the same).
func (f *fctx) genData() expr {
	t := f.cls("StringIO")
	if !f.c.dataVar {
		f.c.dataVar = true
		text := &exprNode{e: expr{code: strconv.Quote(*f.f.data), typ: f.cls("String"), lit: true}}
		io := f.genMethodCall(&parser.NilNode{}, f.genConstRead(&parser.ConstantReadNode{Name: "StringIO"}), "new", []parser.Node{text}, nil)
		f.c.regexps = append(f.c.regexps, "var rbDATA = "+io.code)
	}
	return expr{code: "rbDATA", typ: t}
}

// constMissing is MRI's name for constant read n in a NameError: a bare
// name is qualified by the innermost class or module around it.
func (f *fctx) constMissing(n parser.Node) string {
	r, ok := n.(*parser.ConstantReadNode)
	if !ok {
		return strings.TrimPrefix(f.f.text(n.GetLocation()), "::")
	}
	if len(f.lex) > 0 {
		return f.lex[len(f.lex)-1].RubyName + "::" + r.Name
	}
	return r.Name
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
	case TAny, TFunc, TNil, TOpt, TTuple, TVoid: // not a class object
	}
	return nil
}

// genClassOf renders `x.class` for an instance of a class with a metaclass.
// A receiver whose class is known only at run time (untyped, Object, a
// module, nil, T?) asks rbClassOf, typed as a plain Class.
func (f *fctx) genClassOf(n parser.Node, recv expr) (expr, bool) {
	var cls *Class
	switch t := recv.typ.(type) {
	case TClass:
		cls = t.C
	case TVar:
		if t.Name == "Self" {
			cls = f.owner
		}
		if cls != nil && cls.IsModule && !cls.universal { // self is some includer
			return expr{code: recv.code + "._ClassObj()", typ: TClass{C: f.c.classes["Class"]}}, true
		}
	case TAny, TNil, TOpt:
		return f.dynClassOf(n, recv), true
	case TFunc:
		p := f.c.classes["Proc"]
		f.discard(recv)
		return expr{code: classVar(p), typ: TClass{C: p.meta}}, true
	case TTuple: // a tuple is an Array at run time (decision 22)
		a := f.c.classes["Array"]
		f.discard(recv)
		return expr{code: classVar(a), typ: TClass{C: a.meta}}, true
	case TVoid: // a void call's value is nil
		return f.dynClassOf(n, f.voidAsNil(recv)), true
	}
	if cls != nil && (cls.universal || cls.IsModule) { // Go any: asked at run time
		return f.dynClassOf(n, recv), true
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
	if cls.RubyName == "Boolean" {
		return expr{code: "rbBoolClass(" + recv.code + ")", typ: f.cls("Class")}, true
	}
	if !cls.isStruct() {
		return expr{code: classVar(cls), typ: TClass{C: cls.meta}}, true
	}
	out := expr{code: recv.code + "._ClassOf()", typ: TClass{C: cls.meta}}
	if cls != cls.root() {
		out.code, out.assert = out.code+".("+f.c.goType(out.typ)+")", true
	}
	return out, true
}

func (f *fctx) dynClassOf(n parser.Node, recv expr) expr {
	f.c.classOf = true
	return expr{code: "rbClassOf(" + f.coerce(n, recv, TAny{}) + ")", typ: f.cls("Class")}
}

func isIsA(n *parser.CallNode) bool {
	return (n.Name == "is_a?" || n.Name == "kind_of?") && n.Arguments != nil && len(n.Arguments.Arguments) == 1
}

// isACheck renders `recv.is_a?(C)` as a Go bool: a constant when the static
// type decides it, otherwise a type assertion.
func (f *fctx) isACheck(n parser.Node, recv expr, classNode parser.Node) string {
	cls := f.classRef(classNode)
	if cls == nil {
		// a class value: its class object answers at run time (decision 76)
		k := f.genExpr(classNode, nil)
		switch t := k.typ.(type) {
		case TAny:
		case TClass:
			if !t.C.isSubclassOf(f.c.classes["Module"]) {
				f.errorf(classNode, "is_a? needs a class or module, not %s", k.typ)
			}
		default:
			f.errorf(classNode, "is_a? needs a class or module, not %s", k.typ)
		}
		return "rbIsInstanceOf(" + f.coerce(classNode, k, TAny{}) + ", " + f.coerce(n, recv, TAny{}) + ")"
	}
	c := f.isA(n, recv, cls)
	if c == "true" || c == "false" {
		f.discard(recv)
	}
	return c
}

func (f *fctx) isA(n parser.Node, recv expr, cls *Class) string {
	if cls.universal { // every value, nil included, is an Object
		return "true"
	}
	if c, ok := f.markerIsA(recv, cls); ok {
		return c
	}
	t := recv.typ
	if o, ok := t.(TOpt); ok {
		if !isSimpleGo(recv.code) {
			tmp := f.newTmp()
			f.emit("%s := %s", tmp, recv.code)
			recv.code = tmp
		}
		inner := f.isA(n, expr{code: "(*" + recv.code + ")", typ: o.Elem}, cls)
		return "(" + recv.code + " != nil && " + inner + ")"
	}
	if isNil(t) {
		return "false"
	}
	if cls.IsModule {
		return f.moduleIsA(n, t, cls)
	}
	if v, ok := t.(TVar); ok && v.Name == "Self" && f.owner != nil {
		t = TClass{C: f.owner}
		if f.owner.IsModule { // self is some includer: ask it at run time
			t = TAny{}
		}
	}
	if isAbstract(t) && recv.code != f.selfCode {
		t = TAny{}
	}
	switch t := t.(type) {
	case TClass:
		switch {
		case t.C.isSubclassOf(cls):
			return "true"
		case cls.isStruct() && t.C.isStruct() && cls.isSubclassOf(t.C):
			return "rbIsA[" + f.c.goType(TClass{C: cls}) + "](" + recv.code + ")"
		}
		return "false"
	case TTuple:
		return strconv.FormatBool(cls.RubyName == "Array")
	case TAny, TVar: // a generic T is some value known at run time, as untyped is (Self was resolved above)
		return "rbIsA[" + f.isAGoType(cls) + "](" + recv.code + ")"
	case TVoid: // a void call's value is nil
		f.voidAsNil(recv)
		return "false"
	case TFunc, TNil, TOpt: // handled before the switch
	}
	f.errorf(n, "is_a? on %s is not supported", t)
	return ""
}

// markerIsA is is_a? for TrueClass, FalseClass and NilClass, which have no
// instances of their own: the Boolean or nil value decides.
func (f *fctx) markerIsA(recv expr, cls *Class) (string, bool) {
	name := cls.RubyName
	t := recv.typ
	if name == "Proc" {
		if ft, ok := t.(TFunc); ok && ft.Proc {
			return "true", true
		}
		if isAny(t) || isAbstract(t) {
			return "rbIsProc(" + f.coerce(nil, recv, TAny{}) + ")", true
		}
		return "false", true
	}
	if name != "TrueClass" && name != "FalseClass" && name != "NilClass" {
		return "", false
	}
	if name == "NilClass" {
		switch {
		case isNil(t):
			return "true", true
		case isOpt(t):
			return "(" + recv.code + " == nil)", true
		case isAny(t) || isAbstract(t):
			return "(rbUnbox(" + f.coerce(nil, recv, TAny{}) + ") == nil)", true
		}
		return "false", true
	}
	not := map[bool]string{true: "", false: "!"}[name == "TrueClass"]
	switch {
	case isClass(t, "Boolean"):
		return not + "bool(" + recv.code + ")", true
	case isClass(stripOpt(t), "Boolean"):
		return "(" + recv.code + " != nil && " + not + "bool(*" + recv.code + "))", true
	case isAny(t) || isAbstract(t):
		return "rbIsBool(" + f.coerce(nil, recv, TAny{}) + ", " + strconv.FormatBool(name == "TrueClass") + ")", true
	}
	return "false", true
}

// isAGoType is the Go type an untyped value is asserted to for is_a?(cls).
func (f *fctx) isAGoType(cls *Class) string { return f.c.isAGoType(cls) }

func (c *Compiler) isAGoType(cls *Class) string {
	if len(cls.TypeParams) > 0 {
		return cls.Name + "_Any"
	}
	return c.goType(TClass{C: cls})
}

// moduleIsA decides `x.is_a?(mod)` for a module mod and x of static type t.
// There is no runtime record of included modules, so it is "true" or "false"
// when t's class decides it, and a compile error when t is untyped or a
// subclass of its class includes mod (decision 21).
func (f *fctx) moduleIsA(n parser.Node, t Type, mod *Class) string {
	t = stripOpt(t)
	if v, ok := t.(TVar); ok && v.Name == "Self" && f.owner != nil {
		t = TClass{C: f.owner}
	}
	if _, ok := t.(TTuple); ok {
		t = TClass{C: f.c.classes["Array"]}
	}
	c, ok := t.(TClass)
	switch {
	case mod.universal || ok && c.C.isSubclassOf(mod):
		return "true"
	case isNil(t):
	case !ok || c.C.includedBelow(mod):
		f.errorf(n, "is_a?(%s) on %s cannot be checked: rb2go has no runtime record of included modules", mod.RubyName, t)
	}
	return "false"
}

// narrowIsA renders `x.is_a?(C)` in a condition and narrows x to C inside.
func (f *fctx) narrowIsA(call *parser.CallNode, v *local) (string, []narrowInfo) {
	recv := expr{code: v.goName, typ: v.typ}
	cond := f.isACheck(call, recv, call.Arguments.Arguments[0])
	cls := f.classRef(call.Arguments.Arguments[0])
	if cls == nil { // a class value: nothing static to narrow to
		return cond, nil
	}
	base := stripOpt(v.typ)
	code := v.goName
	if isOpt(v.typ) {
		code = "(*" + v.goName + ")"
	}
	if cond == "true" || cond == "false" || cls.universal {
		return cond, nil
	}
	if bt, ok := base.(TClass); cls.IsModule || ok && bt.C.isSubclassOf(cls) {
		// the class part is static: all the check can rule out is nil
		if isOpt(v.typ) && !isAny(base) {
			return cond, []narrowInfo{{local: v, typ: base}}
		}
		return cond, nil
	}
	typ := TClass{C: cls}
	if len(cls.TypeParams) > 0 {
		for range cls.TypeParams {
			typ.Args = append(typ.Args, TAny{})
		}
		return cond, []narrowInfo{{local: v, typ: typ, code: code + ".(" + cls.Name + "_Any)._ToAny()", view: code}}
	}
	return cond, []narrowInfo{{local: v, typ: typ, code: code + ".(" + f.c.goType(typ) + ")"}}
}

var simpleGo = regexp.MustCompile(`^(?:[A-Za-z_][A-Za-z0-9_]*|\(\*[A-Za-z_][A-Za-z0-9_]*\)|[0-9.]+|"(?:[^"\\]|\\.)*")$`)

// isSimpleGo reports whether code can be evaluated twice without effects.
func isSimpleGo(code string) bool { return simpleGo.MatchString(code) }

// discard evaluates e for its effects where a fold drops its value. It
// discards simple code too: readLocal already counted a local's read, so
// noteUnused will not, and Go rejects the unused variable.
func (f *fctx) discard(e expr) {
	if e.code != "" && e.code != "nil" && !e.lit {
		f.emit("_ = %s", e.code)
	}
}

// genOrAssign renders `x ||= value` for a target whose current value is cur.
// The value is evaluated only when x is nil (or falsy, for untyped/bool).
func (f *fctx) genOrAssign(n parser.Node, cur expr, value parser.Node, written ...func(expr)) expr {
	elem := stripOpt(cur.typ)
	var cond string
	var want Type
	switch {
	case isAny(elem):
		cond, want = "!"+f.truthy(n, cur), TAny{}
	case isOpt(cur.typ) && isClass(elem, "Boolean"):
		cond, want = "!"+optTruthy(cur.code, cur.typ), cur.typ
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
	if v.noreturn { // `x ||= raise(...)`
		f.emit("%s", v.code)
	} else {
		f.emit("%s = %s", cur.code, f.coerce(value, v, want))
		for _, w := range written {
			w(expr{code: cur.code, typ: want})
		}
	}
	f.leaveBlock(saved)
	f.indent--
	f.emit("}")
	if isOpt(cur.typ) && !isAny(elem) && !isOpt(v.typ) && !isNil(v.typ) {
		return expr{code: "(*" + cur.code + ")", typ: elem, done: true}
	}
	return expr{code: cur.code, typ: cur.typ, done: true}
}

func (f *fctx) genOrAssignLocal(n *parser.LocalVariableOrWriteNode) expr {
	if f.visibleLocal(n.Name) == nil {
		// a new local starts out nil, so this is a plain assignment
		return f.assignLocal(n, n.Name, f.genExpr(n.Value, nil), nil)
	}
	v := f.readLocal(&parser.LocalVariableReadNode{Name: n.Name, Location: n.Location})
	if v.base != nil {
		return expr{code: v.goName, typ: v.typ, done: true} // narrowed: already set
	}
	if isNil(v.typ) { // only ever nil so far (`x = nil; x ||= v`): an assignment, which widens x to T?
		return f.assignLocal(n, n.Name, f.genExpr(n.Value, nil), nil)
	}
	e := f.genOrAssign(n, expr{code: v.goName, typ: v.typ}, n.Value)
	if info := f.localInfo(n.Name); info != nil {
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
		var t Type
		if a := f.f.trailingAnnotation(n); a != "" {
			t = f.parseTypeAnn(n, a) // `@x ||= [] #: Array[T]` declares the ivar's type
		} else {
			var v expr
			f.probe(func() { v = f.genExpr(n.Value, nil) })
			t = v.typ
		}
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

func (f *fctx) genOrWrite(n parser.Node) expr {
	switch n := n.(type) {
	case *parser.LocalVariableOrWriteNode:
		return f.genOrAssignLocal(n)
	case *parser.InstanceVariableOrWriteNode:
		return f.genOrAssignIvar(n)
	case *parser.IndexOrWriteNode:
		return f.genIndexOrWrite(n)
	}
	return f.genOrAssignAttr(n.(*parser.CallOrWriteNode))
}

func (f *fctx) genOpWrite(n parser.Node) expr {
	if n, ok := n.(*parser.IndexOperatorWriteNode); ok {
		return f.genIndexOpWrite(n)
	}
	return f.genOpAssignAttr(n.(*parser.CallOperatorWriteNode))
}

// firstType is ts[0], or nil when ts is empty.
func firstType(ts []Type) Type {
	if len(ts) == 0 {
		return nil
	}
	return ts[0]
}

// genOpAssignAttr: `recv.x += v` is recv.x=(recv.x + v), and its value is the new one, not the writer's result.
func (f *fctx) genOpAssignAttr(n *parser.CallOperatorWriteNode) expr {
	recv := f.attrRecv(n, n.Receiver, n.IsSAFE_NAVIGATION(), n.ReadName)
	val := f.genOp(n, f.genMethodCall(n, recv, n.ReadName, nil, nil), n.BinaryOperator, n.Value)
	if !isSimpleGo(val.code) {
		tmp := f.newTmp()
		f.emit("%s := %s", tmp, val.code)
		val.code = tmp
	}
	f.callWriter(n, recv, n.WriteName, val)
	return expr{code: val.code, typ: val.typ, lit: val.lit, done: true}
}

// genOrAssignAttr: Ruby's `recv.x || recv.x = v`, so the writer runs only when the reader is nil/false.
func (f *fctx) genOrAssignAttr(n *parser.CallOrWriteNode) expr {
	recv := f.attrRecv(n, n.Receiver, n.IsSAFE_NAVIGATION(), n.ReadName)
	cur := f.genMethodCall(n, recv, n.ReadName, nil, nil)
	if !isAny(cur.typ) && !isOpt(cur.typ) && !isClass(cur.typ, "Boolean") {
		return cur // never nil or false: the writer never runs
	}
	tmp := f.newTmp()
	f.emit("%s := %s", tmp, cur.code)
	return f.genOrAssign(n, expr{code: tmp, typ: cur.typ}, n.Value, func(v expr) { f.callWriter(n, recv, n.WriteName, v) })
}

// indexOperands evaluates an index assignment's receiver and arguments
// once, for both its `[]` read and its `[]=` write.
func (f *fctx) indexOperands(n, rn parser.Node, args *parser.ArgumentsNode, block *parser.BlockArgumentNode, safe bool) (expr, []parser.Node) {
	if block != nil {
		f.errorf(n, "a block argument in an index assignment is not supported")
	}
	recv := f.attrRecv(n, rn, safe, "[]")
	var as []parser.Node
	if args != nil {
		for _, a := range args.Arguments {
			e := f.valueOf(f.genExpr(a, nil))
			if !isSimpleGo(e.code) {
				tmp := f.newTmp()
				f.emit("%s := %s", tmp, f.materialize(e))
				e.code = tmp
			}
			as = append(as, &exprNode{Node: a, e: e})
		}
	}
	return recv, as
}

// genIndexOrWrite: `h[k] ||= v` is `h[k] || h[k] = v`, with h and k evaluated once.
func (f *fctx) genIndexOrWrite(n *parser.IndexOrWriteNode) expr {
	recv, as := f.indexOperands(n, n.Receiver, n.Arguments, n.Block, n.IsSAFE_NAVIGATION())
	cur := f.genMethodCall(n, recv, "[]", as, nil)
	if !isAny(cur.typ) && !isOpt(cur.typ) && !isClass(cur.typ, "Boolean") {
		return cur // never nil or false: the writer never runs
	}
	tmp := f.newTmp()
	f.emit("%s := %s", tmp, cur.code)
	return f.genOrAssign(n, expr{code: tmp, typ: cur.typ}, n.Value, func(v expr) {
		if isOpt(v.typ) && !isAny(stripOpt(v.typ)) {
			v = expr{code: "(*" + v.code + ")", typ: stripOpt(v.typ)} // just assigned: not nil
		}
		f.emitExprStmt(n, f.genMethodCall(n, recv, "[]=", append(slices.Clone(as), &exprNode{Node: n.Value, e: v}), nil))
	})
}

// genIndexOpWrite: `a[i] += v` is `a[i] = a[i] + v`, with a and i evaluated once; its value is the new one.
func (f *fctx) genIndexOpWrite(n *parser.IndexOperatorWriteNode) expr {
	recv, as := f.indexOperands(n, n.Receiver, n.Arguments, n.Block, n.IsSAFE_NAVIGATION())
	val := f.genOp(n, f.genMethodCall(n, recv, "[]", as, nil), n.BinaryOperator, n.Value)
	if !isSimpleGo(val.code) {
		tmp := f.newTmp()
		f.emit("%s := %s", tmp, val.code)
		val.code = tmp
	}
	f.emitExprStmt(n, f.genMethodCall(n, recv, "[]=", append(slices.Clone(as), &exprNode{Node: n.Value, e: val}), nil))
	return expr{code: val.code, typ: val.typ, lit: val.lit, done: true}
}

// attrRecv: the reader and the writer share one evaluation of the receiver.
func (f *fctx) attrRecv(n, rn parser.Node, safe bool, name string) expr {
	if safe {
		f.errorf(n, "&. with an operator assignment is not supported")
	}
	if isSelf(rn) {
		f.unnarrow("attr:" + name)
	}
	recv := f.valueOf(f.genExpr(rn, nil))
	if !isSimpleGo(recv.code) {
		tmp := f.newTmp()
		f.emit("%s := %s", tmp, recv.code)
		recv.code = tmp
	}
	return recv
}

func (f *fctx) callWriter(n parser.Node, recv expr, name string, val expr) {
	f.emitExprStmt(n, f.genMethodCall(n, recv, name, []parser.Node{&exprNode{e: val}}, nil))
}

// genMultiWrite renders `a, b = x, y`, `a, b = tuple`, and with splat or
// nested targets `a, *rest, z = arr` and `a, (b, c) = x`.
func (f *fctx) genMultiWrite(n *parser.MultiWriteNode) expr {
	if arr, ok := n.Value.(*parser.ArrayNode); ok && n.Rest == nil && len(n.Rights) == 0 && len(arr.Elements) == len(n.Lefts) && !slices.ContainsFunc(arr.Elements, isSplat) {
		for i, v := range f.multiLiteral(arr, n.Lefts) {
			f.assignTarget(n.Lefts[i], v)
		}
		return expr{code: "", typ: TVoid{}, stmt: true, done: true}
	}
	v := f.genExpr(n.Value, nil)
	tmp := f.newTmp()
	f.emit("%s := %s", tmp, v.code)
	f.destructureInto(n, expr{code: tmp, typ: v.typ}, n.Lefts, n.Rest, n.Rights)
	return expr{code: "", typ: TVoid{}, stmt: true, done: true}
}

func isSplat(n parser.Node) bool {
	_, ok := n.(*parser.SplatNode)
	return ok
}

// destructureInto assigns tuple or Array v (held in a temporary) to the
// leading targets, a `*rest` target and the trailing ones, as Ruby: an
// Array gives each target its element or nil, rest the middle.
func (f *fctx) destructureInto(n parser.Node, v expr, lefts []parser.Node, rest parser.Node, rights []parser.Node) {
	lead, trail := len(lefts), len(rights)
	var restT parser.Node // the rest's target; nil for a bare `*`
	switch r := rest.(type) {
	case nil, *parser.ImplicitRestNode: // `a, = xs` drops the rest
	case *parser.SplatNode:
		restT = r.Expression
	default:
		f.errorf(rest, "unsupported rest target %s", nodeType(rest))
	}
	switch t := v.typ.(type) {
	case TTuple:
		m := len(t.Elems)
		if rest == nil && m != lead || rest != nil && m < lead+trail {
			f.errorf(n, "%d targets for a %d-tuple", lead+trail, m)
		}
		for i, l := range lefts {
			f.assignTarget(l, expr{code: fmt.Sprintf("%s.F%d", v.code, i), typ: t.Elems[i]})
		}
		if restT != nil {
			midTs := t.Elems[lead : m-trail]
			et := joinOrAny(midTs)
			parts := make([]string, len(midTs))
			for i, mt := range midTs {
				parts[i] = f.coerce(n, expr{code: fmt.Sprintf("%s.F%d", v.code, lead+i), typ: mt}, et)
			}
			f.assignTarget(restT, expr{code: fmt.Sprintf("&Array[%s]{%s}", f.c.goType(et), strings.Join(parts, ", ")), typ: TClass{C: f.c.classes["Array"], Args: []Type{et}}})
		}
		for j, r := range rights {
			f.assignTarget(r, expr{code: fmt.Sprintf("%s.F%d", v.code, m-trail+j), typ: t.Elems[m-trail+j]})
		}
	case TClass:
		if t.C.RubyName != "Array" {
			f.errorf(n, "cannot destructure %s", v.typ)
		}
		et := f.c.goType(t.Args[0])
		for i, l := range lefts {
			f.assignTarget(l, flatOpt(expr{code: fmt.Sprintf("Array_Op_idx[%s](%s, %d)", et, v.code, i), typ: TOpt{Elem: t.Args[0]}}))
		}
		if restT != nil {
			f.assignTarget(restT, expr{code: fmt.Sprintf("rbMidSplat(%s, %d, %d)", v.code, lead, trail), typ: v.typ})
		}
		for j, r := range rights {
			f.assignTarget(r, flatOpt(expr{code: fmt.Sprintf("rbTrailIdx(%s, %d, %d, %d)", v.code, lead, trail, j), typ: TOpt{Elem: t.Args[0]}}))
		}
	case TAny, TFunc, TNil, TOpt, TVar, TVoid: // only tuples and Arrays split
		f.errorf(n, "cannot destructure %s", v.typ)
	}
}

// assignTarget writes v to one target of a multiple assignment.
func (f *fctx) assignTarget(target parser.Node, v expr) {
	switch t := target.(type) {
	case *parser.LocalVariableTargetNode:
		f.assignLocal(t, t.Name, v, nil)
	case *parser.InstanceVariableTargetNode:
		iv := f.ivar(t, t.Name, v.typ)
		f.emit("%s = %s", f.ivarCode(iv), f.coerce(t, v, iv.Type))
	case *parser.MultiTargetNode:
		if isOpt(v.typ) {
			f.errorf(t, "nested destructuring of a possibly-nil %s is not supported", v.typ)
		}
		tmp := f.newTmp()
		f.emit("%s := %s", tmp, f.materialize(v))
		f.destructureInto(t, expr{code: tmp, typ: v.typ}, t.Lefts, t.Rest, t.Rights)
	default:
		f.errorf(target, "unsupported assignment target %s", nodeType(target))
	}
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
		code := f.materialize(e)
		if isNil(e.typ) || code == "nil" {
			f.emit("var %s %s", tmp, f.c.goType(e.typ))
		} else {
			f.emit("%s := %s", tmp, code)
		}
		vals = append(vals, expr{code: tmp, typ: e.typ})
	}
	return vals
}

// targetType is the current type of an assignment target, if it has one.
func (f *fctx) targetType(n parser.Node) Type {
	switch t := n.(type) {
	case *parser.LocalVariableTargetNode:
		if v := f.visibleLocal(t.Name); v != nil {
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
		scope := []*Class{mod}
		if mod.RubyName == "Object" {
			scope = nil
		}
		// Through a non-module MRI raises TypeError at run time; lookupConst would fail compilation.
		parts := strings.Split(lit, "::")
		for i := 1; i < len(parts); i++ {
			if cls, k := f.c.lookupConst(f.f, constPath(strings.Join(parts[:i], "::")), scope); cls == nil && k != nil {
				return nil
			}
		}
		return f.c.constTypeOf(f.c.lookupConst(f.f, constPath(lit), scope))
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
func (f *fctx) forwardIter(n parser.Node, call string, yields []Type) {
	vars := make([]string, len(yields))
	for i := range yields {
		vars[i] = f.newTmp()
	}
	list := strings.Join(vars, ", ")
	args := list
	var tt TTuple
	if len(yields) == 1 {
		tt, _ = yields[0].(TTuple)
	}
	// A proc auto-splats a lone yielded tuple, like bindBlockParams for a literal block.
	if len(f.blockSig.Params) > 1 && len(tt.Elems) == len(f.blockSig.Params) {
		fields := make([]string, len(tt.Elems))
		for i := range fields {
			fields[i] = fmt.Sprintf("%s.F%d", list, i)
		}
		args = strings.Join(fields, ", ")
	} else if len(yields) != len(f.blockSig.Params) {
		f.errorf(n, "the forwarded block takes %d values but %d are yielded", len(f.blockSig.Params), len(yields))
	}
	f.emit("for %s := range %s {", list, call)
	if f.iterator {
		f.emit("\tif !yield(%s) {", args)
		f.emit("\t\treturn")
		f.emit("\t}")
	} else {
		f.emit("\tblk(%s)", args)
	}
	f.emit("}")
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
			v = &parser.NilNode{Location: n.GetLocation()}
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
	case TAny, TFunc, TNil, TOpt, TTuple, TVoid: // no class known here: not a Data value's with
	}
	if cls == nil || cls.valueRoot() == nil || cls.valueRoot().valueKind != "data" {
		return expr{}, false
	}
	if len(args) == 0 {
		return recv, true // MRI: `with` without keywords is the receiver
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
var rubyPrivate = map[string]bool{"initialize": true, "respond_to_missing?": true, "initialize_copy": true}

// genRespondTo decides `recv.respond_to?(:name, include_all)` for a typed
// receiver, a literal name and a literal include_all: true for a public
// method (any method with include_all), else respond_to_missing?, else
// false. Returns false when the answer depends on the runtime class.
func (f *fctx) genRespondTo(n parser.Node, recv expr, args []parser.Node) (expr, bool) {
	name := literalName(args[0])
	var includeAll parser.Node = &parser.FalseNode{}
	if len(args) > 1 {
		switch args[1].(type) {
		case *parser.TrueNode:
			includeAll = args[1]
		case *parser.FalseNode, *parser.NilNode:
		default:
			return expr{}, false
		}
	}
	_, priv := includeAll.(*parser.TrueNode)
	var cls *Class
	switch t := recv.typ.(type) {
	case TClass:
		cls = t.C
	case TVar:
		if t.Name == "Self" {
			cls = f.owner
		}
	case TAny, TFunc, TNil, TOpt, TTuple, TVoid: // no class known here: answered at run time
	}
	if name == "" || cls == nil {
		return expr{}, false
	}
	if e := cls.lookup(name); e != nil && (priv || !e.M.Private && !rubyPrivate[name]) {
		f.discard(recv)
		return expr{code: "Boolean(true)", typ: f.cls("Boolean")}, true
	}
	if cls.IsModule || cls.descendantDefines(name, priv) { // a module's value is some includer
		return expr{}, false
	}
	if rm := cls.lookup("respond_to_missing?"); rm != nil {
		f.implicitCall = true
		defer func() { f.implicitCall = false }()
		// Ruby hands respond_to_missing? a Symbol even for respond_to?("x").
		sym := &parser.SymbolNode{Unescaped: parser.RubyString{Value: name}, Location: args[0].GetLocation()}
		return f.callEntry(n, rm, recv, []parser.Node{sym, includeAll}, nil), true
	}
	f.discard(recv)
	return expr{code: "Boolean(false)", typ: f.cls("Boolean")}, true
}

// genCallValue renders a call whose value is used. For an attribute write
// (`recv.x = v`, `recv[k] = v`) Ruby's value is v, whatever the setter
// returns, so v is evaluated once and named after the call.
func (f *fctx) genCallValue(n *parser.CallNode, expected Type) expr {
	if expected != nil {
		saved, savedNode := f.retHint, f.retHintNode
		f.retHint, f.retHintNode = expected, n
		defer func() { f.retHint, f.retHintNode = saved, savedNode }()
	}
	args := callArgs(n)
	if !n.IsATTRIBUTE_WRITE() || len(args) == 0 || n.IsSAFE_NAVIGATION() {
		return f.genCall(n, expected)
	}
	var val expr
	c, a := *n, *n.Arguments
	a.Arguments = append(append([]parser.Node{}, args[:len(args)-1]...), &assignedArg{Node: args[len(args)-1], out: &val})
	c.Arguments = &a
	e := f.genCall(&c, expected)
	if val.typ == nil {
		return e // the setter never generated the value as an expression
	}
	f.emitExprStmt(n, e)
	if val.lit {
		val.code = f.c.goType(val.typ) + "(" + val.code + ")"
	}
	return expr{code: val.code, typ: val.typ, classObj: val.classObj, done: true}
}

// assignedArg is an attribute write's value argument: it records the
// generated value (in a temp unless it is side-effect free) for genCallValue.
type assignedArg struct {
	parser.Node
	out *expr
}

func (f *fctx) genAssignedArg(n *assignedArg, expected Type) expr {
	e := f.genExpr(n.Node, expected)
	if !e.lit && !isVoid(e.typ) && !isSimpleGo(e.code) {
		tmp := f.newTmp()
		f.emit("%s := %s", tmp, e.code)
		e.code = tmp
	}
	*n.out = e
	return e
}

// exprNode carries an already generated expression where a node is
// expected (wrapper arguments).
type exprNode struct {
	parser.Node
	e expr
}

func (x *exprNode) GetLocation() parser.Location {
	if x.Node != nil {
		return x.Node.GetLocation() // the source node it was generated from
	}
	return parser.Location{}
}
func (x *exprNode) CompactChildNodes() []parser.Node { return nil } // a leaf: already generated
func (x *exprNode) ChildNodes() []parser.Node        { return nil }

// genDynCall sends a method to an untyped value: see prelude/dynamic.rb.
// Like MRI, only a call with an explicit receiver (other than self) cannot
// reach a private method; send (implicitCall) can.
func (f *fctx) genDynCall(n parser.Node, recv expr, name string, args []parser.Node) expr {
	f.c.noteDyn(name)
	if f.m == nil || !f.m.quietDynamic {
		f.warn(n, "dynamic call: %s on %s", name, recv.typ)
	}
	how := "rbCall"
	call, _ := n.(*parser.CallNode)
	switch {
	case call != nil && call.IsVARIABLE_CALL():
		how = "rbVCall"
	case f.implicitCall || call != nil && (call.Receiver == nil || isSelf(call.Receiver)):
		how = "rbFCall"
	}
	codes := append([]string{how, f.coerce(n, recv, TAny{})}, f.anyArgs(args)...)
	return expr{code: "rbDyn" + goMethodName(name) + "(" + strings.Join(codes, ", ") + ")", typ: TAny{}}
}

// jsonArgs is anyArgs for to_json, plus its `to_json(*a)` idiom: a lone
// splat of an Array passes the elements on.
func (f *fctx) jsonArgs(args []parser.Node) []string {
	if len(args) != 1 {
		return f.anyArgs(args)
	}
	sp, ok := args[0].(*parser.SplatNode)
	if !ok {
		return f.anyArgs(args)
	}
	a := f.genExpr(sp.Expression, nil)
	ac, ok := a.typ.(TClass)
	if !ok || ac.C.Name != "Array" {
		f.errorf(sp, "splat of non-array %s", a.typ)
	}
	if !isAny(ac.Args[0]) {
		a.code += "._ToAny()"
	}
	return []string{"(*" + a.code + ")..."}
}

// anyArgs generates call arguments for an `...any` parameter.
func (f *fctx) anyArgs(args []parser.Node) []string {
	codes := make([]string, 0, len(args))
	for _, a := range args {
		if _, ok := a.(*parser.SplatNode); ok {
			f.errorf(a, "splat arguments in a dynamic call are not supported")
		}
		e := f.genExpr(a, nil)
		codes = append(codes, f.coerce(a, e, TAny{}))
	}
	return codes
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
	if f.m == nil || !f.m.quietDynamic {
		f.warn(n, "dynamic call: %s with a computed name", name)
	}
	nameExpr := f.genExpr(args[0], nil)
	how := "rbFCall"
	if name == "public_send" {
		how = "rbCall"
	}
	codes := []string{f.coerce(n, recv, TAny{}), "rbConstName(" + f.coerce(args[0], nameExpr, TAny{}) + ")", how}
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
	includeAll := func() string {
		if len(args) < 2 {
			return "false"
		}
		return "rbTruthy(" + f.coerce(args[1], f.genExpr(args[1], nil), TAny{}) + ")"
	}
	if lit := literalName(args[0]); lit != "" {
		if universalNames[lit] {
			return expr{code: "Boolean(true)", typ: f.cls("Boolean")}
		}
		f.c.noteRespond(lit)
		return expr{code: "rbResponds" + goMethodName(lit) + "(" + r + ", " + includeAll() + ")", typ: f.cls("Boolean")}
	}
	f.c.dynAll = true
	nameExpr := f.genExpr(args[0], nil)
	return expr{code: "rbRespondsByName(" + r + ", rbConstName(" + f.coerce(args[0], nameExpr, TAny{}) + "), " + includeAll() + ")", typ: f.cls("Boolean")}
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
	switch {
	case r.noreturn: // `expr rescue raise(...)`
		typ = e.typ
	case !ok:
		typ = TAny{}
	}
	if isVoid(typ) && !isNil(typ) {
		typ = TAny{}
	}
	tmp := f.newTmp()
	f.emit("var %s %s", tmp, f.c.goType(typ))
	f.emit("rbBegin(func() {")
	f.indent++
	saved := f.enterBlock()
	f.emit("defer func() {")
	f.emit("\tif r_ := recover(); r_ != nil {")
	f.emit("\t\tr_ = rbWrapPanic(r_)")
	f.emit("\t\tif !rbIsA[StandardErrorI](r_) {")
	f.emit("\t\t\tpanic(r_)")
	f.emit("\t\t}")
	f.indent += 2
	r = f.genExpr(n.RescueExpression, typ)
	if r.noreturn {
		f.emit("%s", r.code)
	} else {
		f.emit("%s = %s", tmp, f.coerce(n, r, typ))
	}
	f.indent -= 2
	f.emit("\t}")
	f.emit("}()")
	e = f.genExpr(n.Expression, typ)
	f.emit("%s = %s", tmp, f.coerce(n, e, typ))
	f.leaveBlock(saved)
	f.indent--
	f.emit("})")
	return expr{code: tmp, typ: typ}
}

// genRange builds a Range literal directly: generic classes have no class methods.
func (f *fctx) genRange(n *parser.RangeNode, expected Type) expr {
	if n.Left == nil && n.Right == nil {
		f.errorf(n, "a range needs a begin or an end")
	}
	var want Type
	if et, ok := expected.(TClass); ok && et.C.RubyName == "Range" && len(et.Args) == 1 {
		want = et.Args[0]
	}
	var parts []parser.Node
	for _, p := range []parser.Node{n.Left, n.Right} {
		if p != nil {
			parts = append(parts, p)
		}
	}
	var types []Type
	f.probe(func() {
		for _, p := range parts {
			types = append(types, f.genExpr(p, want).typ)
		}
	})
	elem := want
	if elem == nil {
		elem = f.joinAll(n, types)
	}
	t := TClass{C: f.c.classes["Range"], Args: []Type{elem}}
	code := "(&" + strings.TrimPrefix(f.c.goType(t), "*") + "{"
	if n.Left != nil {
		code += "b: " + f.coerce(n.Left, f.genExpr(n.Left, elem), elem)
	} else {
		code += "beginless: true"
	}
	if n.Right != nil {
		code += ", e: " + f.coerce(n.Right, f.genExpr(n.Right, elem), elem)
	} else {
		code += ", endless: true"
	}
	if n.IsEXCLUDE_END() {
		code += ", excl: true"
	}
	return expr{code: code + "})", typ: t}
}

func requiredArgs(m *Method) int {
	n := 0
	for _, p := range m.Params {
		if !p.Rest && !p.Keyword && p.Default == nil {
			n++
		}
	}
	return n
}

// genLambda builds a Proc value (`->(x) { }`, `lambda { |x| }`, `proc { }`)
// as a pointer to a Go closure. Its parameter types come from the expected
// type (`#: ^(Integer) -> Integer`); a lambda without parameters may infer
// its return type from its body.
func (f *fctx) genLambda(n, block, params parser.Node, expected Type) expr {
	sig := &BlockSig{Ret: TVar{Name: "Ret_"}}
	if ft, ok := expected.(TFunc); ok && ft.Proc {
		sig = &BlockSig{Params: ft.Params, Ret: ft.Ret}
	} else if len(f.blockParamNames(params)) > 0 {
		f.errorf(n, "a lambda with parameters needs a type annotation (`#: ^(T) -> R`)")
	}
	env := map[string]Type{}
	saved := f.lambdaClosure
	f.lambdaClosure = f.closures + 1
	code := f.genClosure(n, block, sig, env)
	f.lambdaClosure = saved
	return expr{code: "Ref(" + code + ")", typ: TFunc{Params: sig.Params, Ret: subst(sig.Ret, env), Proc: true}}
}

// procBlock desugars `&f` for a Proc f to `{ |x_0, ...| f.call(x_0, ...) }`, or nil when f is not a Proc.
func (f *fctx) procBlock(ba *parser.BlockArgumentNode) *parser.BlockNode {
	var pt Type
	f.probe(func() { pt = f.genExpr(ba.Expression, nil).typ })
	ft, ok := pt.(TFunc)
	if !ok || !ft.Proc {
		return nil
	}
	loc := ba.Location
	var ps, args []parser.Node
	var locals []string
	for i := range ft.Params {
		name := "x_" + strconv.Itoa(i)
		locals = append(locals, name)
		ps = append(ps, &parser.RequiredParameterNode{Location: loc, Name: name})
		args = append(args, &parser.LocalVariableReadNode{Location: loc, Name: name})
	}
	call := &parser.CallNode{Location: loc, Receiver: ba.Expression, Name: "call", Arguments: &parser.ArgumentsNode{Location: loc, Arguments: args}}
	return &parser.BlockNode{
		Location:   loc,
		Locals:     locals,
		Parameters: &parser.BlockParametersNode{Location: loc, Parameters: &parser.ParametersNode{Location: loc, Requireds: ps}},
		Body:       &parser.StatementsNode{Location: loc, Body: []parser.Node{call}},
	}
}

// procCall is a method on a Proc value.
func (f *fctx) procCall(n parser.Node, recv expr, t TFunc, name string, args []parser.Node, block parser.Node) expr {
	if block != nil {
		f.errorf(n, "a Proc's %s does not take a block", name)
	}
	switch name {
	case "call", "()", "[]", "yield", "===":
		if len(args) != len(t.Params) {
			f.errorf(n, "wrong number of arguments (given %d, expected %d)", len(args), len(t.Params))
		}
		codes := make([]string, len(args))
		for i, a := range args {
			codes[i] = f.coerce(a, f.genExpr(a, t.Params[i]), t.Params[i])
		}
		return expr{code: "(*" + recv.code + ")(" + strings.Join(codes, ", ") + ")", typ: t.Ret}
	case "arity":
		f.discard(recv)
		return expr{code: strconv.Itoa(len(t.Params)), typ: f.cls("Integer"), lit: true}
	case "lambda?":
		f.discard(recv)
		return expr{code: "true", typ: f.cls("Boolean"), lit: true}
	case "to_proc":
		return recv
	case ">>", "<<":
		if len(args) != 1 {
			f.errorf(n, "%s takes one Proc", name)
		}
		g := f.genExpr(args[0], nil)
		gt, ok := g.typ.(TFunc)
		if !ok || !gt.Proc {
			f.errorf(n, "%s takes a Proc, got %s", name, g.typ)
		}
		first, firstT, second, secondT := recv.code, t, g.code, gt
		if name == "<<" {
			first, firstT, second, secondT = g.code, gt, recv.code, t
		}
		if len(secondT.Params) != 1 || !typeEq(secondT.Params[0], firstT.Ret) {
			f.errorf(n, "cannot compose %s with %s", firstT, secondT)
		}
		out := TFunc{Params: firstT.Params, Ret: secondT.Ret, Proc: true}
		var ps, as []string
		for i, p := range firstT.Params {
			ps = append(ps, "a"+strconv.Itoa(i)+" "+f.c.goType(p))
			as = append(as, "a"+strconv.Itoa(i))
		}
		inner := "(*first)(" + strings.Join(as, ", ") + ")"
		body := "return (*second)(" + inner + ")"
		if isVoid(secondT.Ret) {
			body = "(*second)(" + inner + ")"
		}
		ret := ""
		if !isVoid(secondT.Ret) {
			ret = " " + f.c.goType(secondT.Ret)
		}
		code := fmt.Sprintf("func(first %s, second %s) %s { return Ref(func(%s)%s { %s }) }(%s, %s)",
			f.c.goType(firstT), f.c.goType(secondT), f.c.goType(out), strings.Join(ps, ", "), ret, body, first, second)
		return expr{code: code, typ: out}
	}
	return f.universalCall(n, recv, name, args, block)
}

// genGlobalRead maps the read-only globals rb2go knows (decision 61); $0 is the Ruby file as named at compile time, as `ruby main.rb` sets it.
func (f *fctx) genGlobalRead(n *parser.GlobalVariableReadNode) expr {
	switch n.Name {
	case "$0", "$PROGRAM_NAME":
		name := f.c.mainFile.Name
		f.c.strLits[name] = true
		return expr{code: strconv.Quote(name), typ: f.cls("String"), lit: true}
	case "$stdin":
		return f.genConstRead(&parser.ConstantReadNode{Name: "STDIN", Location: n.Location})
	case "$stdout", "$stderr": // the IO bound to the assigned object, if any (decision 109)
		return expr{code: "rb" + strings.ToUpper(n.Name[1:2]) + n.Name[2:] + "IO()", typ: f.cls("IO")}
	case "$?": // read directly: a Kernel call on main would go dynamic inside another class's method
		return expr{code: "rbLastStatusOpt()", typ: TOpt{Elem: TClass{C: f.c.classes["Process::Status"]}}}
	}
	f.errorf(n, "global variable %s is unsupported; only $0, $PROGRAM_NAME, $stdin, $stdout, $stderr and $? are (docs/design.md decision 61)", n.Name)
	return expr{}
}

// genGlobalWrite is `$stdout = io` / `$stderr = io` (decision 109): the
// object takes the stream's writes. Reading the global still answers the
// IO constant (decision 61), so the assignment's value is what was given.
func (f *fctx) genGlobalWrite(n *parser.GlobalVariableWriteNode) expr {
	fn := map[string]string{"$stdout": "rbSetStdout", "$stderr": "rbSetStderr"}[n.Name]
	if fn == "" {
		f.errorf(n, "global variable %s cannot be assigned; only $stdout and $stderr can (docs/design.md decision 109)", n.Name)
	}
	v := f.genExpr(n.Value, nil)
	return expr{code: fn + "(" + f.coerce(n.Value, v, TAny{}) + ")", typ: TAny{}, stmt: true}
}

// kernelCall calls a private Kernel prelude method as a receiverless call
// on main would: what user syntax like backticks and $? stands for.
func (f *fctx) kernelCall(n parser.Node, name string, args []parser.Node) expr {
	f.implicitCall = true
	defer func() { f.implicitCall = false }()
	return f.genMethodCall(n, expr{code: "rb_main", typ: f.cls("Object")}, name, args, nil)
}

// csvOptions are the CSV options rb2go implements; any other literal key
// is a compile error rather than silently ignored (decision 117).
var csvOptions = map[string]bool{"col_sep": true, "quote_char": true, "row_sep": true, "skip_blanks": true, "force_quotes": true, "headers": true, "converters": true}

// csvConverters are the converters CSV's literal `converters:` may name.
var csvConverters = map[string]bool{"numeric": true, "integer": true, "float": true}

// csvOverload routes CSV.parse/read/foreach/parse_line by their literal
// options (decision 117): `headers: true` reads a CSV::Table, `converters:`
// gives untyped fields; unknown keys and converters are compile errors.
func (f *fctx) csvOverload(m *Method, owner *Class, args []parser.Node) *entry {
	if len(args) == 0 {
		return nil
	}
	kw, ok := args[len(args)-1].(*parser.KeywordHashNode)
	if !ok {
		return nil
	}
	headers, converters := false, false
	for _, el := range kw.Elements {
		a, ok := el.(*parser.AssocNode)
		key, _ := a.Key.(*parser.SymbolNode)
		if !ok || key == nil {
			continue
		}
		name := key.Unescaped.Value
		if !csvOptions[name] {
			f.errorf(el, "CSV option %s: is not supported (docs/design.md decision 117)", name)
		}
		switch name {
		case "headers":
			switch a.Value.(type) {
			case *parser.TrueNode:
				headers = true
			case *parser.FalseNode, *parser.NilNode:
			default:
				f.errorf(a.Value, "CSV headers: must be a literal true or false (an Array of names is not supported)")
			}
		case "converters":
			converters = true
			f.csvCheckConverters(a.Value)
		}
	}
	if (headers || converters) && m.Name != "parse" && m.Name != "read" && m.Name != "foreach" && m.Name != "parse_line" {
		f.errorf(kw, "CSV.%s does not take headers: or converters:", m.Name)
	}
	switch {
	case headers && m.Name == "parse_line":
		f.errorf(kw, "CSV.parse_line with headers: is not supported")
	case headers:
		return owner.lookup("__" + m.Name + "_headers")
	case converters:
		return owner.lookup("__" + m.Name + "_converted")
	}
	return nil
}

func (f *fctx) csvCheckConverters(v parser.Node) {
	var names []parser.Node
	if arr, ok := v.(*parser.ArrayNode); ok {
		names = arr.Elements
	} else {
		names = []parser.Node{v}
	}
	for _, n := range names {
		sym, ok := n.(*parser.SymbolNode)
		if !ok || !csvConverters[sym.Unescaped.Value] {
			f.errorf(n, "CSV converters: takes :numeric, :integer or :float (or an Array of them)")
		}
	}
}
