package compiler

import (
	"fmt"
	"slices"

	"github.com/danielgatis/go-ruby-prism/parser"
)

// lenientBlockParams rewrites a gem file's block and lambda params rb2go cannot bind as Ruby binds them: `&blk` (dropBlockArgParam)
// and a lambda's defaults (decision 241).
func lenientBlockParams(root parser.Node) {
	anyNode(root, func(n parser.Node) bool {
		switch b := n.(type) {
		case *parser.BlockNode:
			body := b.Body
			dropBlockArgParam(b.Parameters, &body)
			b.Body = body
		case *parser.LambdaNode:
			body := b.Body
			dropBlockArgParam(b.Parameters, &body)
			b.Body = body
			optionalLambdaParams(b)
		}
		return false
	})
}

// dropBlockArgParam turns a gem block's or lambda's `&blk` param into a local that is nil: rb2go passes no block to a
// block (decision 241). ponytail: a caller's block is dropped (concurrent-ruby's logger then logs nil for a lazy message).
func dropBlockArgParam(params parser.Node, body *parser.Node) {
	bp, ok := params.(*parser.BlockParametersNode)
	if !ok || bp.Parameters == nil || bp.Parameters.Block == nil {
		return
	}
	blk := bp.Parameters.Block
	bp.Parameters.Block = nil
	if blk.Name == nil {
		return
	}
	w := &parser.LocalVariableWriteNode{Location: blk.Location, Name: *blk.Name, NameLoc: blk.Location, Value: &parser.NilNode{Location: blk.Location}}
	switch b := (*body).(type) {
	case nil:
		*body = &parser.StatementsNode{Location: blk.Location, Body: []parser.Node{w}}
	case *parser.StatementsNode:
		b.Body = append([]parser.Node{w}, b.Body...)
	case *parser.BeginNode:
		if b.Statements == nil {
			b.Statements = &parser.StatementsNode{Location: b.Location}
		}
		b.Statements.Body = append([]parser.Node{w}, b.Statements.Body...)
	}
}

// optionalLambdaParams gives a gem lambda's optional params their defaults as a block's get them (optionalBlockParams).
func optionalLambdaParams(l *parser.LambdaNode) {
	b := &parser.BlockNode{Location: l.Location, Parameters: l.Parameters, Body: l.Body, Locals: l.Locals}
	optionalBlockParams(b)
	l.Parameters, l.Body = b.Parameters, b.Body
}

// lenientBlockRet types a gem block's untyped value where the method wants Array[U] (flat_map): its unbound type
// variables are untyped, and the value is converted on return (decision 241).
func (f *fctx) lenientBlockRet(want, got Type, env map[string]Type) bool {
	if !f.lenient() || !isAny(got) {
		return false
	}
	var vs []string
	freeVars(want, &vs)
	for _, v := range vs {
		if _, bound := env[v]; !bound {
			env[v] = TAny{}
		}
	}
	return true
}

// lenientMember: a gem's `include?(x)` of a T? is false for nil, as in MRI, where rbMust would raise (decision 172).
func (f *fctx) lenientMember(n parser.Node, recv expr, name string, args []parser.Node) (expr, bool) {
	if !f.lenient() || len(args) != 1 || name != "include?" && name != "member?" && name != "key?" && name != "has_key?" {
		return expr{}, false
	}
	var a expr
	f.probe(func() { a = f.genExpr(args[0], nil) })
	o, ok := a.typ.(TOpt)
	if !ok || f.c.goType(a.typ) != "*"+f.c.goType(o.Elem) {
		return expr{}, false
	}
	tmp := f.newTmp()
	f.emit("%s := %s", tmp, f.genExpr(args[0], nil).code)
	call := f.genMethodCall(n, recv, name, []parser.Node{&exprNode{Node: args[0], e: expr{code: "*" + tmp, typ: o.Elem}}}, nil)
	return expr{code: fmt.Sprintf("Boolean(%s != nil && bool(%s))", tmp, call.code), typ: f.cls("Boolean")}, true
}

// lookupMiss: what each key-comparing Hash/Array lookup answers when no element equals the key (decision 307).
var lookupMiss = map[string]map[string]string{
	"Hash":  {"[]": "default", "key?": "false", "has_key?": "false", "include?": "false", "member?": "false", "delete": "nil", "dig": "nil", "fetch": "fetch"},
	"Array": {"include?": "false", "member?": "false", "delete": "nil", "index": "nil"},
}

// lenientLookup: a gem's untyped key of another class misses, as in MRI, instead of raising TypeError (decisions 307, 314).
func (f *fctx) lenientLookup(n parser.Node, recv expr, name string, args []parser.Node, block parser.Node) (expr, bool) {
	rc, ok := recv.typ.(TClass)
	if !ok || !f.lenient() || len(args) == 0 || len(rc.Args) == 0 || holdsAny(rc.Args[0]) || mentionsVar(rc.Args[0]) || isSplatArg(args[0]) {
		return expr{}, false
	}
	miss := lookupMissMode(lookupMiss[rc.C.RubyName][name], name, len(args), block != nil)
	kt := rc.Args[0]
	if miss == "" || f.c.goType(f.probeType(args[0], nil)) != "any" || f.c.goType(kt) == "any" { // untyped, or a union of classes
		return expr{}, false
	}
	key, k := f.newTmp(), f.newTmp()
	f.emit("%s := %s", key, f.genExpr(args[0], nil).code)
	rest := append([]parser.Node{&exprNode{Node: args[0], e: expr{code: k, typ: kt}}}, args[1:]...)
	var call expr
	stmts := f.capture(func() { call = f.genMethodCall(n, recv, name, rest, block) })
	res := f.newTmp()
	f.emit("var %s %s", res, f.c.goType(call.typ))
	f.emit("if %s, ok := rbConv[%s](%s); ok {", k, f.c.goType(kt), key)
	f.buf.WriteString(stmts)
	f.emit("\t%s = %s", res, call.code)
	f.emit("} else {")
	f.indent++
	f.lookupMissed(n, miss, recv, kt, key, args, block, expr{code: res, typ: call.typ})
	f.indent--
	f.emit("}")
	return expr{code: res, typ: call.typ}, true
}

// lookupMissMode is lookupMiss's answer for this call's shape, "" when lenientLookup leaves it alone: fetch(k, d) is d,
// fetch(k) { |x| } the block's value, fetch(k) KeyError (decision 314).
func lookupMissMode(miss, name string, nargs int, hasBlock bool) string {
	switch {
	case miss == "fetch" && hasBlock && nargs == 1:
		return "block"
	case miss == "fetch" && !hasBlock && nargs == 2:
		return "arg"
	case miss == "fetch" && !hasBlock && nargs == 1:
		return "keyerror"
	case miss == "fetch" || hasBlock || nargs != 1 && name != "dig":
		return ""
	}
	return miss
}

// lookupMissed assigns res the miss's answer (lookupMiss) for a key that does not convert to kt.
func (f *fctx) lookupMissed(n parser.Node, miss string, recv expr, kt Type, key string, args []parser.Node, block parser.Node, res expr) {
	switch miss {
	case "default":
		// a default proc takes (Hash[K, V], K): it cannot be handed this key, so the miss is loud rather than the static default
		f.emit("if %s != nil {", f.genMethodCall(n, recv, "default_proc", nil, nil).code)
		f.emit("\tpanic(NewTypeError(Ref(String(%q))))", fmt.Sprintf("rb2go: a non-%s key reached a default proc taking %s (%s:%d, decision 314)",
			kt, kt, f.f.Name, f.f.line(n.GetLocation().StartOffset)))
		f.emit("}")
		f.emit("%s = %s", res.code, f.coerce(n, f.genMethodCall(n, recv, "default", nil, nil), res.typ))
	case "block": // the block again, its param untyped: MRI yields the key
		f.emit("%s = (%s)(%s)", res.code, f.genClosure(n, block, &BlockSig{Params: []Type{TAny{}}, Ret: res.typ}, nil), key)
	case "keyerror":
		loc := n.GetLocation()
		ke := &parser.CallNode{Location: loc, Receiver: &parser.ConstantReadNode{Location: loc, Name: "KeyError"}, Name: "__for",
			Arguments: &parser.ArgumentsNode{Location: loc, Arguments: []parser.Node{
				&exprNode{Node: n, e: expr{code: fmt.Sprintf("String(\"key not found: \") + rbInspect(%s)", key), typ: f.cls("String")}},
				&exprNode{Node: n, e: expr{code: "any(" + recv.code + ")", typ: TAny{}}},
				&exprNode{Node: n, e: expr{code: key, typ: TAny{}}}}}}
		f.emit("panic(%s)", f.genExpr(ke, nil).code)
	case "arg":
		f.emit("%s = %s", res.code, f.coerce(args[1], f.genExpr(args[1], res.typ), res.typ))
	case "false":
		f.emit("%s = Boolean(false)", res.code)
	default:
		f.emit("%s = nil", res.code)
	}
}

// lenient reports gem code (decision 172): what would be a nil or union type error compiles to a run-time check.
func (f *fctx) lenient() bool { return f.c.pruneGems && f.c.gemFile(f.f) }

// lenientClass converts what gem code passes where class to is wanted and user code would be a compile error
// (decision 172): an Array[Sub] for Array[Base] (a converted copy, Go's instantiations being invariant), self where an
// unrelated class is wanted (concurrent-ruby's Event as a Future, checked at run time), a T? for T (nil raises).
func (f *fctx) lenientClass(n parser.Node, e expr, to TClass) (string, bool) {
	if !f.lenient() {
		return "", false
	}
	if ec, ok := e.typ.(TClass); ok && converts(to) && ec.C == to.C && f.c.goType(e.typ) != f.c.goType(to) {
		return f.noteConv(n, fmt.Sprintf("rbAs[%s](%s, %q)", f.c.goType(to), e.code, to.String())), true
	}
	if ec, ok := e.typ.(TClass); ok && ec.C == to.C && f.c.goType(e.typ) == f.c.goType(to) {
		return e.code, true // Array[untyped] where its sig says Array[A | B]: the same Go type, unions being untyped (decision 241)
	}
	if tv, ok := e.typ.(TVar); ok && tv.Name == "Self" && to.C.isStruct() && !to.C.universal && f.owner != nil &&
		!f.owner.isSubclassOf(to.C) && !slices.Contains(f.owner.ancestors(), to.C) {
		return f.noteConv(n, fmt.Sprintf("rbAs[%s](any(%s), %q)", f.c.goType(to), e.code, to.String())), true
	}
	if o, ok := e.typ.(TOpt); ok && f.c.goType(e.typ) == "*"+f.c.goType(o.Elem) {
		if e.code == "nil" { // `@x = T.let(nil, T.nilable(X))` into a non-nil slot: its zero value, read before set
			return "rbZero[" + f.c.goType(to) + "]()", true
		}
		if oc, ok := o.Elem.(TClass); ok && oc.C == to.C && oc.C.isStruct() {
			// an object's interface holds nil as MRI's variable does, and gem truthiness tests it (decision 257): concurrent-ruby's
			// `reduce(nil) { |d, f| add_delayed d, ... }` passes its nil accumulator on
			return "rbDerefOrNil(" + e.code + ")", true
		}
		return f.coerce(n, expr{code: fmt.Sprintf("rbMust(%s, %q)", e.code, o.Elem.String()), typ: o.Elem}, to), true
	}
	if isNil(e.typ) && to.C.RubyName == "Boolean" { // a gem's predicate falling off with nil: false where a bool is kept (decision 233)
		return "Boolean(false)", true
	}
	if to.C.isStruct() && !to.C.universal && len(to.C.TypeParams) == 0 && structValued(e.typ) {
		// Ruby checks no types: a gem's `AbstractTab` where its sig says `Tab`, or nil where a class is expected, is the
		// value it is; Go asserts it to the class's interface when it arrives (decision 225)
		if isNil(e.typ) {
			return "nil", true
		}
		return f.noteConv(n, fmt.Sprintf("rbAs[%s](any(%s), %q)", f.c.goType(to), e.code, to.String())), true
	}
	return "", false
}

// bindGemUnbound: what a gem's argument left unbound (untyped, or T? for T[E]) is untyped, as for
// `T::Array[T.type_parameter(:Elem)]` (decision 172).
func (f *fctx) bindGemUnbound(m *Method, env map[string]Type, t Type) {
	if f.lenient() {
		bindUntyped(m, env, t)
	}
}

// lenientObject: in gem code a value of a struct class's type may hold nil (an untyped nil converted in, rbAsSlow), so
// `x.nil?` checks it at run time, where Object#nil? is false (decision 253).
func (f *fctx) lenientObject(t Type) bool {
	c, ok := t.(TClass)
	return ok && f.lenient() && c.C.isStruct() && !c.C.universal && c.C.metaOf == nil
}

// lenientCall is a gem's call compiled as MRI dispatches it where the static types would pick another method
// (decision 253): an inherited class method's `new` on self, and `nil?` on an object that may hold nil.
func (f *fctx) lenientCall(n *parser.CallNode) (expr, bool) {
	if e, ok := f.inheritedSelfNew(n); ok {
		return e, true
	}
	if n.Name == "nil?" && n.Arguments == nil && n.Block == nil && n.Receiver != nil && f.lenient() {
		t := f.probeType(n.Receiver, nil)
		if f.lenientObject(t) {
			r := f.genExpr(n.Receiver, nil)
			return expr{code: "Boolean(any(" + r.code + ") == nil)", typ: f.cls("Boolean")}, true
		}
	}
	return expr{}, false
}

// lenientSplat is a gem's `f(x, *args)` of an untyped args: spread at run time (rbSplatAny, decision 255).
func (f *fctx) lenientSplat(a expr) expr {
	if !isAny(a.typ) || !f.lenient() {
		return a
	}
	return expr{code: "(&Array[any]{s: rbSplatAny(" + a.code + ")})", typ: TClass{C: f.c.classes["Array"], Args: []Type{TAny{}}}}
}
