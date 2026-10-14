package compiler

import (
	"errors"
	"fmt"
	"math"
	"regexp"
	"slices"
	"strconv"
	"strings"

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

// genSugar renders syntax that reduces to a literal or a plain read:
// `defined?` (answered at compile time), `__LINE__`, `{ a: }`, `:"#{x}"`.
func (f *fctx) genSugar(n parser.Node) expr {
	switch n := n.(type) {
	case *parser.DefinedNode:
		if _, ok := n.Value.(*parser.YieldNode); ok && f.m.optionalBlockLocal() != "" { // the block may be missing: answered at run time
			c, _ := f.genCond(&parser.LocalVariableReadNode{Name: f.m.optionalBlockLocal(), Location: n.Location})
			f.c.strLits["yield"] = true
			t := TOpt{Elem: f.cls("String")}
			return expr{code: fmt.Sprintf("func() %s { if %s { return Ref[String](\"yield\") }; return nil }()", f.c.goType(t), c), typ: t}
		}
		switch v := n.Value.(type) {
		case *parser.InstanceVariableReadNode: // decision 153: the object's assigned bit, or nil for an ivar self's class never has
			var iv *Ivar
			if f.owner != nil && !f.owner.universal {
				iv = f.c.findIvar(f.owner, v.Name)
			}
			if iv == nil {
				return expr{code: "nil", typ: TNil{}}
			}
			set := ivarIsSet(f.selfCode, iv)
			switch {
			case set == "" && f.c.ivarBitsMarked:
				f.errorf(v, "rb2go: defined?(%s) in a class it does not mark (decision 153)", v.Name)
			case set == "": // discovery, before markIvarBits: only the type matters
				set = "true"
			}
			f.c.strLits["instance-variable"] = true
			t := TOpt{Elem: f.cls("String")}
			return expr{code: fmt.Sprintf("func() %s { if %s { return Ref[String](\"instance-variable\") }; return nil }()", f.c.goType(t), set), typ: t}
		case *parser.ConstantReadNode, *parser.ConstantPathNode:
			// a constant main may read before its assignment runs (BEGIN, a method called first) is defined once it has
			var k *Const
			_ = catchCompileError(func() { _, k = f.c.lookupConst(f.f, v, f.lex) }) // a private path: definedKind's nil
			if k != nil && k.guarded {
				f.c.strLits["constant"] = true
				t := TOpt{Elem: f.cls("String")}
				return expr{code: fmt.Sprintf("func() %s { if %s { return Ref[String](\"constant\") }; return nil }()", f.c.goType(t), constSet(k)), typ: t}
			}
		}
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
	case *parser.NumberedReferenceReadNode, *parser.BackReferenceReadNode:
		return f.genMatchRef(n), true
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

// genExpr notes each value's type for Marshal: an inferred local's container type is never rendered (decision 137).
func (f *fctx) genExpr(n parser.Node, expected Type) expr {
	if f.c.genCount != nil {
		*f.c.genCount++
		if f.c.genLimit != nil && *f.c.genCount > *f.c.genLimit {
			f.errorf(n, "generation budget exceeded")
		}
	}
	e := f.genExpr0(n, expected)
	switch n.(type) {
	case *parser.YieldNode, *parser.SuperNode, *parser.ForwardingSuperNode: // the block or the parent may reassign ivars
		f.unnarrowIvars()
	}
	f.c.noteMarshal(e.typ)
	if fn, ok := e.typ.(TFunc); ok && fn.Proc {
		f.c.goType(fn) // a proc literal's Go type is written out inline, so rbIsProc (Marshal's "Proc" TypeError) would miss it
	}
	return e
}

func (f *fctx) genExpr0(n parser.Node, expected Type) expr {
	if e, ok := f.genLiteral(n); ok {
		return e
	}
	switch n := n.(type) {
	case *parser.LocalVariableReadNode:
		return f.genLocalRead(n)
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
		*parser.ClassVariableReadNode, *parser.ClassVariableWriteNode, *parser.ClassVariableOperatorWriteNode, *parser.ClassVariableOrWriteNode:
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
	case *parser.LocalVariableOrWriteNode, *parser.InstanceVariableOrWriteNode, *parser.CallOrWriteNode, *parser.IndexOrWriteNode,
		*parser.InstanceVariableAndWriteNode, *parser.LocalVariableAndWriteNode:
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
	case *parser.IfNode, *parser.UnlessNode, *parser.CaseNode, *parser.CaseMatchNode, *parser.BeginNode:
		return f.lift(n, expected, func(t tail) { f.genStmt(n, t) })
	case *parser.MatchRequiredNode, *parser.MatchPredicateNode:
		return f.genMatch(n)
	case *parser.OrNode:
		return f.genOr(n)
	case *parser.AndNode:
		return f.genAnd(n)
	case *parser.YieldNode:
		return f.genYield(n)
	case *parser.SuperNode, *parser.ForwardingSuperNode:
		return f.genSuperNode(n)
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
	if c, ok := n.Value.(*parser.CallNode); ok && exp == nil && empty && f.pass == 1 {
		if cls := f.c.classes[emptyNewClass(c)]; cls != nil { // pass 1: untyped, so what goes in it can type it
			args := make([]Type, len(cls.TypeParams))
			for i := range args {
				args[i] = TAny{}
			}
			exp = TClass{C: cls, Args: args}
		}
	}
	seed := f.widenSeed(n, ann, &exp)
	if r, ok := n.Value.(*parser.LocalVariableReadNode); ok && r.Name == n.Name && f.erbDepth > 0 { // `x = x` declares x for the template's parse: a no-op
		if v := f.visibleLocal(n.Name); v != nil {
			return expr{code: v.goName, typ: v.typ, view: v.view, done: true}
		}
		return expr{code: "", typ: TNil{}, done: true} // declared later in the def: not in scope here
	}
	var val expr
	if f.lenient() && ann == nil && exp != nil { // a gem's local rebound to what its type does not hold (`data = { **data, ... }`)
		err := f.tryClean(func() { val = f.genExpr(n.Value, exp) })
		if err != nil {
			val = f.genExpr(n.Value, nil) // as itself; the local widens (decision 233)
		}
	} else {
		val = f.genExpr(n.Value, exp)
	}
	if val.noreturn { // `x = raise "e"`: x still needs a Go type for the (unreachable) code after
		t := exp
		if t == nil {
			t = TAny{}
		}
		val = expr{code: f.coerce(n, val, t), typ: t}
	}
	e := f.assignLocal(n, n.Name, val, ann)
	if info := f.localInfo(n.Name); seed != nil && info != nil { // what later writes put in it may widen it (decision 193)
		info.open, info.seeded = true, true
		info.elems[0] = append(info.elems[0], seed)
	}
	if info := f.localInfo(n.Name); empty && f.pass == 1 && info != nil {
		info.open = true
	}
	return e
}

// widenSeed is an unannotated `x = [1, 2]`'s element type, for later writes to widen (decision 193): in pass 1 the
// local is Array[untyped] so any write is evidence; in pass 2 it is what the evidence joined to. nil otherwise.
func (f *fctx) widenSeed(n *parser.LocalVariableWriteNode, ann Type, exp *Type) Type {
	an, ok := n.Value.(*parser.ArrayNode)
	if !ok || ann != nil || *exp != nil || len(an.Elements) == 0 || slices.ContainsFunc(an.Elements, func(e parser.Node) bool { _, s := e.(*parser.SplatNode); return s }) {
		return nil
	}
	if r := f.refined[f.localKey(n.Name)]; r != nil {
		*exp = r
		return nil
	}
	if f.pass != 1 {
		return nil
	}
	var nat expr
	if f.attempt(func() { f.probe(func() { nat = f.genExpr(n.Value, nil) }) }) != nil {
		return nil
	}
	tc, ok := nat.typ.(TClass)
	if !ok || tc.C.RubyName != "Array" || len(tc.Args) != 1 || holdsAny(tc.Args[0]) {
		return nil
	}
	*exp = TClass{C: tc.C, Args: []Type{TAny{}}}
	return tc.Args[0]
}

func isRangeNode(n parser.Node) bool {
	_, ok := n.(*parser.RangeNode)
	return ok
}

func isEmptyLit(n parser.Node) bool {
	switch n := n.(type) {
	case *parser.ArrayNode:
		return len(n.Elements) == 0
	case *parser.HashNode:
		return len(n.Elements) == 0
	case *parser.CallNode:
		return emptyNewClass(n) != ""
	}
	return false
}

// emptyNewClass is the class of an empty container's `X.new` (`Hash.new`, `Queue.new`, `SizedQueue.new(n)`): an open
// literal, typed from what is put in it as `[]` and `{}` are (decision 182).
func emptyNewClass(n *parser.CallNode) string {
	r, ok := n.Receiver.(*parser.ConstantReadNode)
	if !ok || n.Name != "new" || n.Block != nil {
		return ""
	}
	switch args := callArgs(n); {
	case r.Name == "SizedQueue" && len(args) == 1, len(args) == 0 && (r.Name == "Hash" || r.Name == "Array" || r.Name == "Set" || r.Name == "Queue"):
		return r.Name
	}
	return ""
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
	seeded := false
	switch r := n.Receiver.(type) {
	case *parser.LocalVariableReadNode:
		if info := f.localInfo(r.Name); f.pass == 1 && info != nil && info.open {
			t, open, elems, seeded = info.typ, &info.open, &info.elems, info.seeded
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
	case !hash && slices.Contains([]string{"<<", "push", "append", "unshift", "prepend", "add", "enq"}, n.Name):
		f.noteElem(open, &elems[0], args...)
	case !hash && n.Name == "fill" && len(args) >= 1:
		f.noteElem(open, &elems[0], args[0])
	case !hash && n.Name == "[]=" && len(args) == 2 && !isRangeNode(args[0]): // a[i] = v; a[range] = xs splices xs
		f.noteElem(open, &elems[0], args[1])
		if !seeded {
			elems[0] = append(elems[0], TNil{}) // a gap before the index reads nil (a literal's own elements say otherwise)
		}
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
		t := f.probeType(a, nil)
		*elems = append(*elems, t)
	}
}

// noteConv records a conversion site for analyze.
func (f *fctx) noteConv(n parser.Node, code string) string {
	if n == nil { // an adapter's own conversion (procArityAdapter): no source site to record
		return code
	}
	if f.pass == 1 && f.convs != nil {
		f.convs[n.GetLocation().StartOffset] = true
	}
	if f.pass == 2 && f.c.convs != nil {
		f.c.convs[fmt.Sprintf("%s:%d", f.f.Name, n.GetLocation().StartOffset)] = true
	}
	return code
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

// genInterp compiles an interpolated string (`"a#{b}"`) to a Go string concatenation of its parts.
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

func isSelfVar(t Type) bool { v, ok := t.(TVar); return ok && v.Name == "Self" }

var simpleGo = regexp.MustCompile(`^(?:[A-Za-z_][A-Za-z0-9_]*|\(\*[A-Za-z_][A-Za-z0-9_]*\)|[0-9.]+|"(?:[^"\\]|\\.)*")$`)

// isSimpleGo reports whether code can be evaluated twice without effects.
func isSimpleGo(code string) bool { return simpleGo.MatchString(code) }

// pinSimple returns code itself when it can be evaluated twice, else the temp it was just stored in.
func (f *fctx) pinSimple(code string) string {
	if !isSimpleGo(code) {
		tmp := f.newTmp()
		f.emit("%s := %s", tmp, code)
		code = tmp
	}
	return code
}

// discard evaluates e for its effects where a fold drops its value. It
// discards simple code too: readLocal already counted a local's read, so
// noteUnused will not, and Go rejects the unused variable.
func (f *fctx) discard(e expr) {
	if e.code != "" && e.code != "nil" && !e.lit {
		f.emit("_ = %s", e.code)
	}
}

// firstType is ts[0], or nil when ts is empty.
func firstType(ts []Type) Type {
	if len(ts) == 0 {
		return nil
	}
	return ts[0]
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

func isSelf(n parser.Node) bool { _, ok := n.(*parser.SelfNode); return ok }

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
	inf := len(types) == 2 && isClass(types[0], "Integer") && isFloatInfinity(n.Right) && (want == nil || isClass(want, "Integer"))
	switch {
	case inf: // `1..Float::INFINITY` iterates Integers forever, as MRI: an endless Range[Integer] (decision 140)
		elem = types[0]
	case elem == nil:
		elem = f.joinAll(n, types)
	}
	t := TClass{C: f.c.classes["Range"], Args: []Type{elem}}
	code := "(&" + strings.TrimPrefix(f.c.goType(t), "*") + "{"
	if n.Left != nil {
		code += "b: " + f.coerce(n.Left, f.genExpr(n.Left, elem), elem)
	} else {
		code += "beginless: true"
	}
	switch {
	case inf:
		code += ", endless: true, inf: true"
	case n.Right != nil:
		code += ", e: " + f.coerce(n.Right, f.genExpr(n.Right, elem), elem)
	default:
		code += ", endless: true"
	}
	if n.IsEXCLUDE_END() {
		code += ", excl: true"
	}
	return expr{code: code + "})", typ: t}
}

// mentionsSelf reports whether t holds the Self type variable.
func mentionsSelf(t Type) bool {
	switch t := t.(type) {
	case TVar:
		return t.Name == "Self"
	case TOpt:
		return mentionsSelf(t.Elem)
	case TClass:
		return slices.ContainsFunc(t.Args, mentionsSelf)
	case TTuple:
		return slices.ContainsFunc(t.Elems, mentionsSelf)
	case TAny, TFunc, TNil, TUnion, TVoid:
	}
	return false
}

// probing reports a dry-run generation (probe), whose output is discarded.
func (f *fctx) probing() bool { return f.probeDepth > 0 }

// rangeHead opens a range over an iterator; one yielding no values binds none (`for range seq`).
func rangeHead(vars, call string) string {
	if strings.TrimSpace(vars) == "" {
		return "for range " + call + " {"
	}
	return "for " + vars + " := range " + call + " {"
}

// defaultsFile is where m's parameter defaults are written: a synthesized `new`'s are its initialize's.
func defaultsFile(m *Method) *File {
	if m.Kind == kindSynth && m.Name == "new" && m.Owner != nil && m.Owner.metaOf != nil {
		if init := m.Owner.metaOf.lookup("initialize"); init != nil && init.M.File != nil {
			return init.M.File
		}
	}
	return m.File
}

// genToEnum is `to_enum`/`enum_for` (bare, or naming the enclosing method) in a method taking a one-value closure
// block: an Enumerator that calls the method again with a block feeding it (rbSeq stops it early).
func (f *fctx) genToEnum(n *parser.CallNode) (expr, bool) {
	m := f.m
	if n.Name != "to_enum" && n.Name != "enum_for" || n.Receiver != nil && !isSelf(n.Receiver) || n.Block != nil ||
		m == nil || m.Block == nil || m.Iterator || len(m.Block.Params) != 1 || !inInterface(m) || m.kwMask() || m.calleeDefaults {
		return expr{}, false
	}
	args := callArgs(n)
	if len(args) > 0 {
		sym, ok := args[0].(*parser.SymbolNode)
		if !ok || sym.Unescaped.Value != m.Name || len(args)-1 > len(m.Params) {
			return expr{}, false
		}
		args = args[1:]
	}
	var as []string
	for i, p := range m.Params {
		switch {
		case p.Rest:
			return expr{}, false
		case i < len(args):
			as = append(as, f.coerce(n, f.genExpr(args[i], p.Type), p.Type))
		case len(args) > 0 && p.Default != nil: // `to_enum(:each)` leaves the rest at their defaults
			as = append(as, f.coerce(n, f.genExpr(p.Default, p.Type), p.Type))
		default:
			as = append(as, goLocalName(p.Name))
		}
	}
	elem := f.c.goType(m.Block.Params[0])
	blk := fmt.Sprintf("func(x %s) { y(x) }", elem)
	if !isVoid(m.Block.Ret) {
		r := f.c.goType(m.Block.Ret)
		blk = fmt.Sprintf("func(x %s) %s { y(x); return *new(%s) }", elem, r, r)
	}
	as = append(as, blk)
	code := fmt.Sprintf("rbEnumOf(rbSeq(func(y func(%s)) { %s.%s(%s) }), any(%s), %q, nil, nil)",
		elem, f.selfCode, m.GoName, strings.Join(as, ", "), f.selfCode, m.Name)
	return expr{code: code, typ: TClass{C: f.c.classes["Enumerator"], Args: []Type{m.Block.Params[0]}}}, true
}

// eachReturnsSelf are the iterators whose value in MRI is their receiver.
var eachReturnsSelf = map[string]bool{"each": true, "each_with_index": true, "each_pair": true, "each_key": true, "each_value": true, "reverse_each": true,
	"each_line": true, "each_char": true, "each_byte": true, "times": true}

// isTypeVar is a type parameter other than Self (a Hash subclass's untyped V).
func isTypeVar(t Type) bool {
	v, ok := t.(TVar)
	return ok && v.Name != "Self"
}
