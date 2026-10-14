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
