package compiler

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
	"unicode"

	"github.com/danielgatis/go-ruby-prism/parser"
)

// ---- calls

// genKernelIntrinsic handles the receiverless calls the compiler builds itself.
func (f *fctx) genKernelIntrinsic(n *parser.CallNode, expected Type) (expr, bool) {
	switch n.Name {
	case "raise", "fail":
		return f.genRaise(n), true
	case "require_relative", "require":
		return f.genInMethodRequire(n), true
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
	case "block_given?":
		if n.Arguments != nil || n.Block != nil {
			break
		}
		switch {
		case f.m.optionalBlockLocal() != "":
			c, _ := f.genCond(n)
			return expr{code: "Boolean(" + c + ")", typ: f.cls("Boolean")}, true
		case f.m != nil && f.m.Block != nil: // a required block (or an iterator's loop body) is always there
			return expr{code: "Boolean(true)", typ: f.cls("Boolean")}, true
		}
		return expr{code: "Boolean(false)", typ: f.cls("Boolean")}, true
	case "define_method":
		if f.c.dmSites[n] {
			return expr{code: "nil", typ: TNil{}, lit: true}, true // boot made its methods defs (decision 198)
		}
		f.errorf(n, "%s is not supported", n.Name)
	case "send", "__send__", "public_send":
		if len(callArgs(n)) == 0 {
			f.errorf(n, "%s needs a method name", n.Name)
		}
		return f.genSend(n, expr{code: f.selfCode, typ: f.selfType, classObj: f.selfClassObj}, n.Name, callArgs(n), n.Block), true // `send(:x)` is self's
	case "binding", "method_missing":
		f.errorf(n, "%s is not supported", n.Name)
	}
	return expr{}, false
}

// genKernelCall compiles a receiverless call, or `Kernel.x` for one of Kernel's module functions, as a Kernel intrinsic when it is one (decision 139).
func (f *fctx) genKernelCall(n *parser.CallNode, expected Type) (expr, bool) {
	if n.Receiver == nil {
		return f.genKernelIntrinsic(n, expected)
	}
	runtimeRequire := n.Name == "require" && f.c.methodRequiresRuntime && !f.f.isPrelude() // `Kernel.require path` (decision 246)
	if !kernelConst(n.Receiver) || !kernelModuleNames[n.Name] && !runtimeRequire || n.Name == "require" && !runtimeRequire || n.Name == "require_relative" {
		return expr{}, false
	}
	t := f.probeType(n.Receiver, nil)
	if !kernelMeta(t) {
		return expr{}, false
	}
	bare := *n
	bare.Receiver = nil
	return f.genKernelIntrinsic(&bare, expected)
}

func (f *fctx) genCall(n *parser.CallNode, expected Type) expr {
	defer f.afterCall(n) // after the arguments, which still read narrowed ivars
	if e, ok := f.genCallRewrite(n, expected); ok {
		return e
	}
	if e, ok := f.lenientCall(n); ok {
		return e
	}
	if e, ok := f.genCallForm(n, expected); ok {
		return e
	}
	if strings.HasSuffix(n.Name, "=") && (n.Receiver == nil || isSelf(n.Receiver)) {
		f.unnarrow("attr:" + strings.TrimSuffix(n.Name, "="))
	}
	if e, ok := f.genERBCall(n); ok {
		return e
	}
	if err, ok := f.c.anonErrors[n]; ok {
		panic(err)
	}
	if cls := f.c.anonClasses[n]; cls != nil {
		return f.genAnonClass(n, cls, expected)
	}
	if cls := f.classRef(n.Receiver); cls != nil {
		if err, bad := f.c.badClasses[cls]; bad {
			panic(err)
		}
		if e, ok := f.genSpecialClassCall(n, cls, expected); ok {
			return e
		}
		// Direct constructor unless Foo defines self.new; Hash is the one @go_type class with a Go constructor (NewHash).
		// A generic @go_type class's own self.new takes arguments; bare `.new` is its annotated zero value.
		if f.useGenNew(cls, n) {
			return f.genNew(n, cls, callArgs(n), nil, expected, n.Block)
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
	switch {
	case (n.Receiver == nil || isSelf(n.Receiver)) && f.dynSelfCall(n.Name):
		recv = expr{code: "any(" + f.selfCode + ")", typ: TAny{}}
	case n.Receiver == nil:
		recv = expr{code: f.selfCode, typ: f.selfType, classObj: f.selfClassObj}
	default:
		recv = f.valueOf(f.genExpr(n.Receiver, nil))
	}
	f.noteElems(n)
	if e, ok := f.genInstanceEval(n, recv); ok {
		return e
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
	u, nilUnion := recv.typ.(TUnion)
	nilUnion = nilUnion && slices.ContainsFunc(u.Members, isNil)
	if !isOpt(recv.typ) && !isAny(recv.typ) && !nilUnion {
		return f.genMethodCall(n, recv, n.Name, callArgs(n), n.Block)
	}
	rt := f.newTmp()
	inner := expr{code: rt, typ: recv.typ} // an untyped nil is nil too
	if o, ok := recv.typ.(TOpt); ok {
		inner = expr{code: "(*" + rt + ")", typ: o.Elem}
	}
	if nilUnion { // `@url&.to_s` of a String | URL | nil: a union holds nil as Go's nil, and the call sees the rest
		inner = expr{code: rt, typ: unionOf(slices.DeleteFunc(slices.Clone(u.Members), isNil)...)}
	}
	call := func() expr { return f.genMethodCall(n, inner, n.Name, callArgs(n), n.Block) }
	probe, once := f.genOperandOnce(call)
	if vt := stripOpt(probe.e.typ); isVoid(vt) || isNil(vt) { // also a union whose members' calls are void or nil (`error_pipe&.close`)
		f.emit("if %s := %s; %s != nil {", rt, recv.code, rt)
		f.indent++
		e := f.operandAgain(probe, once, call)
		f.emitExprStmt(n, e)
		f.indent--
		f.emit("}")
		return expr{code: "", typ: TVoid{}, stmt: true}
	}
	resT := optOf(probe.e.typ)
	tmp := f.newTmp()
	f.emit("var %s %s", tmp, f.c.goType(resT))
	f.emit("if %s := %s; %s != nil {", rt, recv.code, rt)
	f.indent++
	e := f.operandAgain(probe, once, call)
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
	case TAny, TFunc, TNil, TOpt, TTuple, TUnion, TVoid: // no class here to look in: only a top-level def can answer
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
	if name == "__getobj__" && len(args) == 0 && block == nil {
		return f.getobjCall(n, recv)
	}
	return f.genMethodCall1(n, recv, name, args, block)
}

// getobjCall is `__getobj__`, typed by what the delegator wraps when delegateType knows it (decision 284).
func (f *fctx) getobjCall(n parser.Node, recv expr) expr {
	x := f.genMethodCall1(n, recv, "__getobj__", nil, nil)
	cls := classOf(recv.typ)
	if isSelfVar(recv.typ) {
		cls = f.owner
	}
	if dt := f.c.delegateType(cls); dt != nil && isAny(x.typ) {
		return expr{code: f.coerce(n, x, dt), typ: dt}
	}
	return x
}

func (f *fctx) genMethodCall1(n parser.Node, recv expr, name string, args []parser.Node, block parser.Node) expr {
	if _, ok := recv.typ.(TVoid); ok { // a void call's value is nil, as in Ruby: run it, then call on nil (decision 119)
		recv = f.voidAsNil(recv)
	}
	if e, ok := f.genIntrinsic(n, recv, name, args, block); ok {
		return e
	}
	if e, ok := f.lenientMember(n, recv, name, args); ok {
		return e
	}
	if meta := f.metaOfType(recv.typ); name == "new" && meta != nil && len(meta.metaOf.Subclasses) == 0 {
		if vr := meta.metaOf.valueRoot(); vr != nil && generatedInit(meta.metaOf) { // `klass.new(a: 1)` on a held Struct/Data class: keywords by member, as genNew's
			args = f.keywordMembers(n, vr, args)
		}
	}
	if meta := f.metaOfType(recv.typ); name == "new" && meta != nil && meta.metaOf.hashBase && len(meta.metaOf.Subclasses) == 0 && len(args) == 0 {
		f.emit("_ = %s", recv.code)
		return f.genNew(n, meta.metaOf, nil, nil, nil, block) // `klass.new` on a singleton(HashSubclass), as rack's `@params_class.new`
	}
	f.checkEncodingNames(n, recv, name, args)
	if recv.view != "" && block == nil {
		if e, ok := f.viewCall(n, recv, name, args); ok {
			return e
		}
	}
	if len(args) > 0 && !isUnion(recv.typ) && slices.ContainsFunc(args, f.readsUnionLocal) {
		// an argument over a union local may need unionArgCall: generated once
		// here, the failed dispatch and the retry reuse it instead of each
		// regenerating the subtree (exponential in its nesting otherwise)
		args = f.pinArgs(args, len(args))
	}
	if len(args) > 0 && !isUnion(recv.typ) && !allLiteral(args) {
		// a union argument the method does not take as it is: once per member (decision 150)
		var e expr
		err := f.attempt(func() { e = f.dispatch(n, recv, name, args, block) })
		if err == nil {
			return f.narrowRaises(name, args, e)
		}
		if e, ok := f.unionArgCall(n, recv, name, args, block); ok {
			return e
		}
		if f.lenient() && block == nil && f.unionArm == 0 { // gem code: matched per call at run time, as MRI does
			return f.genDynCall(n, recv, name, args)
		}
		panic(*err)
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

// genSpecialClassCall is the class-method calls the compiler answers itself: Ractor.new (decision 103), ERB.new (decision 111) and URI.open (decision 134).
func (f *fctx) genSpecialClassCall(n *parser.CallNode, cls *Class, expected Type) (expr, bool) {
	switch {
	case cls.RubyName == "Ractor":
		return f.genRactorCall(n, cls)
	case cls.RubyName == "URI" && n.Name == "open":
		return f.genOpenURI(n, nil, "open", callArgs(n), n.Block), true
	case cls.RubyName == "ERB" && n.Name == "new":
		f.c.erbNewTemplate(f.f, n) // checked here; the object only marks the template
		return expr{code: "NewERB()", typ: TClass{C: cls}, ctor: true}, true
	case n.Name == "new" && cls.RubyName == "Proc" && n.Block != nil:
		if bn, ok := n.Block.(*parser.BlockNode); ok && n.Arguments == nil { // Proc.new { } is proc { }
			return f.genLambda(n, bn, bn.Parameters, expected), true
		}
		f.errorf(n, "Proc.new needs a literal block")
	case n.Name == "new" && cls.universal && !cls.IsModule:
		// a plain object, made to be unique: specs compare it by identity (#56)
		if args := callArgs(n); len(args) > 0 {
			f.errorf(n, "wrong number of arguments (given %d, expected 0)", len(args))
		}
		return expr{code: "&" + cls.Name + "{}", typ: TClass{C: cls}, ctor: true}, true
	case n.Name == "new" && cls.isSubclassOf(f.c.classes["BasicSocket"]) && cls.lookup("initialize") == nil:
		// a socket object is only made with its handle: BasicSocket/IPSocket/Socket.new would be a nil one (decision 135)
		if cls.isSubclassOf(f.c.classes["Socket"]) {
			f.errorf(n, "Socket.new is not supported (decision 135): raw sockets bind and connect to packed sockaddr Strings; use TCPSocket, TCPServer, UDPSocket, UNIXSocket or Socket.tcp")
		}
		f.errorf(n, "undefined method initialize for %s (decision 135): use TCPSocket, TCPServer, UDPSocket, UNIXSocket or Socket.tcp", cls.RubyName)
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
	if e, ok, stop := f.intrinsicGate(n, recv, name, args, block); stop {
		return e, ok
	}
	if f.isOpenURIRecv(recv.typ, name) {
		return f.genOpenURI(n, &recv, name, args, block), true
	}
	if e, ok := f.rubyShapedCall(n, recv, name, args, block); ok {
		return e, true
	}
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
	if (name == "send" || name == "__send__" || name == "public_send") && len(args) >= 1 && !f.ownSend(recv.typ, name) {
		return f.genSend(n, recv, name, args, block), true
	}
	if e, ok := f.genMethodObject(n, recv, name, args, block); ok {
		return e, true
	}
	if name == "is_a?" || name == "kind_of?" {
		if len(args) != 1 || block != nil {
			f.errorf(n, "%s takes one class", name)
		}
		check := f.isACheck(n, recv, args[0])
		return expr{code: "Boolean(" + check + ")", typ: f.cls("Boolean"), lit: check == "true" || check == "false"}, true
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

// ownSend is true when the receiver's class defines its own send (Ractor's, BasicSocket's), which shadows Kernel#send as in MRI.
func (f *fctx) ownSend(t Type, name string) bool {
	if name != "send" {
		return false
	}
	c := classOf(t)
	return c != nil && c.lookup(name) != nil
}

// dispatch resolves a call by the receiver's static type.
func (f *fctx) dispatch(n parser.Node, recv expr, name string, args []parser.Node, block parser.Node) expr {
	switch t := recv.typ.(type) {
	case TOpt:
		return f.optCall(n, recv, name, args, block)
	case TUnion:
		return f.unionCall(n, recv, t, name, args, block)
	case TTuple:
		return f.tupleCall(n, recv, name, args, block)
	case TFunc:
		if t.Proc {
			return f.procCall(n, recv, t, name, args, block)
		}
	case TClass:
		if e, ok := f.methodValueCall(n, recv, t, name, args, block); ok {
			return e
		}
		return f.classCall(n, t, recv, name, args, block)
	case TVar:
		if t.Name == "Self" && f.owner != nil {
			if e := f.owner.lookup(name); e != nil {
				return f.selfEntryCall(n, e, recv, args, block)
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
			if !f.owner.IsModule && !untypedIntrinsics[name] && !descendantResponds(f.owner, name) {
				f.errorf(n, "undefined method %s for %s", name, f.owner.displayName())
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
	obj := f.genMethodCall(n, recv, "__getobj__", nil, nil) // typed when delegateType knows the wrapped object
	return f.genMethodCall(n, obj, name, args, block)
}

// resetsDelegate: cls or a subclass may wrap something else (a subclass's own initialize, a `__setobj__` call).
func resetsDelegate(cls *Class) bool {
	family := []*Class{cls}
	for i := 0; i < len(family); i++ {
		family = append(family, family[i].Subclasses...)
	}
	for _, k := range family {
		if k != cls && k.Methods["initialize"] != nil {
			return true
		}
		for _, m := range k.MethodList {
			if m.Node != nil && anyNode(m.Node.Body, func(x parser.Node) bool {
				call, ok := x.(*parser.CallNode)
				return ok && call.Name == "__setobj__"
			}) {
				return true
			}
		}
	}
	return false
}

// delegateType is what a SimpleDelegator subclass wraps, when its own initialize sets it only by `super(p)` from a
// typed parameter p (a rest parameter wraps Array[T]; Homebrew's `Dependencies#initialize(*args)`) and neither it nor
// a subclass calls `__setobj__` or defines initialize: nil otherwise, and forwarded calls stay dynamic (decision 118).
// `super(Set.new(p))` wraps Set[T] (decision 291).
func (c *Compiler) delegateType(cls *Class) Type {
	if cls == nil || !c.isDelegator(cls) || cls.RubyName == "SimpleDelegator" {
		return nil
	}
	init := cls.Methods["initialize"]
	if init == nil || init.Node == nil || init.File == nil || init.File.isPrelude() || !init.declared {
		return nil
	}
	setobj := resetsDelegate(cls)
	var supers []*parser.SuperNode
	anyNode(init.Node.Body, func(x parser.Node) bool {
		if sn, ok := x.(*parser.SuperNode); ok {
			supers = append(supers, sn)
		}
		_, fwd := x.(*parser.ForwardingSuperNode)
		setobj = setobj || fwd
		return false
	})
	if setobj || len(supers) != 1 {
		return nil
	}
	if supers[0].Arguments == nil || len(supers[0].Arguments.Arguments) != 1 {
		return nil
	}
	arg := supers[0].Arguments.Arguments[0]
	inSet := false
	if call, ok := arg.(*parser.CallNode); ok && call.Name == "new" && constName(call.Receiver) == "Set" && len(callArgs(call)) == 1 && call.Block == nil {
		arg, inSet = callArgs(call)[0], true // `super(Set.new(args))`: Homebrew's Requirements wraps Set[Requirement]
	}
	lv, ok := arg.(*parser.LocalVariableReadNode)
	if !ok {
		return nil
	}
	c.resolveMethod(init)
	return c.delegateParamType(init, lv.Name, inSet)
}

// delegateParamType is what a delegator's `super(name)` (or `super(Set.new(name))` when inSet) wraps, from the typed
// parameter name: nil when it is untyped.
func (c *Compiler) delegateParamType(init *Method, name string, inSet bool) Type {
	for _, p := range init.Params {
		var elem Type
		switch {
		case p.Name != name || isAny(p.Type) || p.Type == nil || p.Keyword:
			continue
		case p.Rest:
			elem = p.Type
		case !inSet:
			return p.Type
		default:
			pc, isCls := p.Type.(TClass)
			if !isCls || len(pc.Args) != 1 || !isClass(pc, "Array") && !isClass(pc, "Set") || isAny(pc.Args[0]) {
				return nil
			}
			elem = pc.Args[0]
		}
		if inSet {
			return TClass{C: c.classes["Set"], Args: []Type{elem}}
		}
		return TClass{C: c.classes["Array"], Args: []Type{elem}}
	}
	return nil
}

// classCall dispatches on a class-typed receiver.
func (f *fctx) classCall(n parser.Node, t TClass, recv expr, name string, args []parser.Node, block parser.Node) expr {
	f.noteYielderFeed(recv, name, args)
	if e, ok := f.moduleCmp(t, recv, name, args, block); ok {
		return e
	}
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
		if plain := strings.TrimSuffix(name, "!"); t.C.RubyName == "String" && plain != name && t.C.lookup(plain) != nil {
			f.errorf(n, "undefined method %s for String: Strings are immutable in rb2go, so assign the result of %s instead (decision 136)", name, plain)
		}
		f.errorf(n, "undefined method %s for %s", name, recv.typ)
	}
	if f.lenient() && e.Owner != nil && e.Owner.universal && e.M.File != nil && e.M.File.isPrelude() && t.C.isStruct() && !t.C.universal && t.C.descendantDefines(name, false) {
		// Kernel's `then` on an AbstractEventFuture whose Event and Future define their own (concurrent-ruby's
		// `.future.then(*args, &task)`): the object's method, found at run time as MRI does (decision 253)
		anyRecv := expr{code: f.coerce(n, recv, TAny{}), typ: TAny{}}
		if block != nil {
			return f.universalBlockCall(n, anyRecv, name, args, block)
		}
		return f.genDynCall(n, anyRecv, name, args)
	}
	if x, ok := f.untypedCmpOp(n, t, recv, e, args, block); ok {
		return x
	}
	return f.numericMix(n, recv, e, args, block)
}

// selfEntryCall is a call on self (Self) of its class's method e, through a `<=>` that takes untyped for a Comparable
// op (MacOSVersion's `self >= "27"`, decision 296).
func (f *fctx) selfEntryCall(n parser.Node, e *entry, recv expr, args []parser.Node, block parser.Node) expr {
	if x, ok := f.untypedCmpOp(n, TClass{C: f.owner}, recv, e, args, block); ok {
		return x
	}
	return f.callEntry(n, e, recv, args, block)
}

// untypedCmpOp is Comparable's `<`, `<=`, `>` or `>=` on a class whose own `<=>` takes untyped (Homebrew's
// `Version < "2.39"`): the argument goes to that `<=>` as it is, as MRI's Comparable passes it, and a nil result is
// MRI's ArgumentError; Comparable's own `(self)` signature would convert it first (decision 287).
func (f *fctx) untypedCmpOp(n parser.Node, t TClass, recv expr, e *entry, args []parser.Node, block parser.Node) (expr, bool) {
	switch e.M.Name {
	case "<", "<=", ">", ">=":
	default:
		return expr{}, false
	}
	if e.M.Owner != f.c.classes["Comparable"] || len(args) != 1 || block != nil || isNumeric(recv.typ) {
		return expr{}, false
	}
	ce := t.C.lookup("<=>")
	if ce == nil || ce.M.Owner == e.M.Owner || len(ce.M.Params) != 1 || !isAny(ce.M.Params[0].Type) {
		return expr{}, false
	}
	r := expr{code: f.pinSimple(recv.code), typ: recv.typ}
	a := f.genExpr(args[0], nil)
	a.code = f.pinSimple(f.coerce(args[0], a, TAny{}))
	a.typ = TAny{}
	c := f.genMethodCall(n, r, "<=>", []parser.Node{&exprNode{Node: args[0], e: a}}, nil)
	code := fmt.Sprintf("rbCmpOp(%s, %q, %s, %s)", f.coerce(n, c, TAny{}), e.M.Name, f.coerce(n, r, TAny{}), a.code)
	return expr{code: code, typ: f.cls("Boolean")}, true
}

// abstractCall: an Object or module value is Go any, so it dispatches as untyped.
func (f *fctx) abstractCall(n parser.Node, t TClass, recv expr, name string, args []parser.Node, block parser.Node) expr {
	d := f.universalCall(n, recv, name, args, block)
	// the declared result keeps the caller typed instead of cascading dynamic calls
	if e := t.C.lookup(name); e != nil && isAny(d.typ) {
		f.c.inferRet(e.M)
		env := composeEnv(e.Env, nil)
		env["Self"] = t // a module's `self` is the value itself, of the module's type
		ret := subst(e.M.Ret, env)
		if name == "<=>" && f.c.isNumericMod(t.C) {
			ret = optOf(f.cls("Integer")) // nil for an incomparable number (a Complex), unlike Comparable's own
		}
		if !isVoid(ret) && !mentionsVar(ret) {
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
	x, args, ok := f.numericTower(n, recv, e, args, block)
	if ok {
		return x
	}
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

// untypedContainers are the core containers whose element types fall back to untyped when nothing types them (decision 192).
var untypedContainers = map[string]bool{"Hash": true, "Array": true, "Set": true, "Queue": true, "SizedQueue": true, "Thread::Queue": true, "Thread::SizedQueue": true}

// containerOf is the core container whose (class) method m is, or "".
func containerOf(m *Method) string {
	k := m.Owner
	if k != nil && k.metaOf != nil {
		k = k.metaOf
	}
	if k != nil && untypedContainers[k.RubyName] {
		return k.RubyName
	}
	return ""
}

// intrinsicGate decides a call before the intrinsics: true/false/nil's reopened marker classes answer (before a
// program's Object#name, which Boolean would take), and a program's own override is called as MRI calls it.
func (f *fctx) intrinsicGate(n parser.Node, recv expr, name string, args []parser.Node, block parser.Node) (expr, bool, bool) {
	if e, ok := f.markerCall(n, recv, name, args, block); ok {
		return e, true, true
	}
	if e := f.resolve(recv.typ, name); e != nil && !e.M.File.isPrelude() {
		return expr{}, false, true
	}
	return expr{}, false, false
}

// reopenedMarker is the program's own definition of name on TrueClass, FalseClass or NilClass (`class FalseClass; def
// present? = false`), whose values are Boolean and nil: nil when only the prelude's markers or Object answer.
func (f *fctx) reopenedMarker(cls, name string) *entry {
	c := f.c.classes[cls]
	if c == nil {
		return nil
	}
	e, ok := c.Methods[name]
	if !ok || e.File == nil || e.File.isPrelude() {
		return nil
	}
	return &entry{M: e, Owner: c, Entry: c}
}

// markerCall is a call on true/false or nil that the program's reopened TrueClass/FalseClass/NilClass answers: MRI
// picks the method by the value's class, where Boolean would take Object's (decision 241).
func (f *fctx) markerCall(n parser.Node, recv expr, name string, args []parser.Node, block parser.Node) (expr, bool) {
	if isNil(recv.typ) {
		if f.reopenedMarker("NilClass", name) == nil {
			return expr{}, false
		}
		return f.genMethodCall(n, expr{code: "(*NilClass)(nil)", typ: f.cls("NilClass")}, name, args, block), true
	}
	if f.inMarker || !isClass(recv.typ, "Boolean") || f.reopenedMarker("TrueClass", name) == nil && f.reopenedMarker("FalseClass", name) == nil {
		return expr{}, false
	}
	arm := func(cls string) expr {
		if f.reopenedMarker(cls, name) == nil { // Boolean's own answer, past this check
			f.inMarker = true
			defer func() { f.inMarker = false }()
			return f.genMethodCall(n, expr{code: recv.code, typ: recv.typ}, name, args, block)
		}
		return f.genMethodCall(n, expr{code: "(*" + cls + ")(nil)", typ: f.cls(cls)}, name, args, block)
	}
	rv := f.newTmp()
	f.emit("%s := %s", rv, recv.code)
	recv = expr{code: rv, typ: recv.typ}
	var te, fe expr
	f.probe(func() { te, fe = arm("TrueClass"), arm("FalseClass") })
	t, ok := join(te.typ, fe.typ)
	if !ok {
		t = TAny{}
	}
	out := f.newTmp()
	f.emit("var %s %s", out, f.c.goType(t))
	f.emit("if bool(%s) {", rv)
	f.indent++
	f.emit("%s = %s", out, f.coerce(n, arm("TrueClass"), t))
	f.indent--
	f.emit("} else {")
	f.indent++
	f.emit("%s = %s", out, f.coerce(n, arm("FalseClass"), t))
	f.indent--
	f.emit("}")
	return expr{code: out, typ: t}, true
}

// rubyShapedCall: calls whose MRI result needs another shape than the prelude method's typing gives (decision 241).
func (f *fctx) rubyShapedCall(n parser.Node, recv expr, name string, args []parser.Node, block parser.Node) (expr, bool) {
	if e, ok := f.mixedArrayOp(n, recv, name, args, block); ok {
		return e, true
	}
	if e, ok := f.grepBlockLastMatch(n, recv, name, args, block); ok {
		return e, true
	}
	return f.subBlockLastMatch(n, recv, name, args, block)
}

// mixedArrayOp is `xs + ys` / `xs | ys` of Arrays, or `h.merge(g)` of Hashes, whose element (value) types differ,
// neither holding the other: MRI's result mixes them, so both sides become containers of the join first (decision 241).
func (f *fctx) mixedArrayOp(n parser.Node, recv expr, name string, args []parser.Node, block parser.Node) (expr, bool) {
	cls := "Array"
	if name == "merge" {
		cls = "Hash"
	} else if name != "+" && name != "|" {
		return expr{}, false
	}
	if len(args) != 1 || block != nil || !isClass(recv.typ, cls) {
		return expr{}, false
	}
	at := f.probeType(args[0], nil)
	if !isClass(at, cls) {
		return expr{}, false
	}
	ra, aa := recv.typ.(TClass).Args, at.(TClass).Args
	if cls == "Hash" && !typeEq(ra[0], aa[0]) {
		if !f.lenient() || isAny(ra[0]) && isAny(ra[1]) {
			return expr{}, false // `{a: 1}.merge(h)`: mixed keys stay merge's own conversion
		}
		// a gem's `options.merge(Sandbox::INHERITANCE_FD => fd)`, an Integer key into Symbol keys: MRI's result holds both,
		// so both sides are Hash[untyped, untyped] (decision 256)
		ra, aa = []Type{TAny{}, ra[1]}, []Type{TAny{}, aa[1]}
		if !typeEq(ra[1], aa[1]) {
			ra[1], aa[1] = TAny{}, TAny{}
		}
	}
	a, b := ra[len(ra)-1], aa[len(aa)-1]
	keysJoined := cls == "Hash" && isAny(ra[0]) && !isAny(recv.typ.(TClass).Args[0])
	switch {
	case keysJoined:
	case isAny(a) && isAny(b), typeEq(a, b):
		return expr{}, false
	case isAny(a) || isAny(b):
		if !f.lenient() {
			return expr{}, false // user code: the untyped side is converted to the typed one, as before
		}
	case fitsValue(expr{typ: b}, a) || fitsValue(expr{typ: a}, b):
		return expr{}, false
	}
	jt := joinUnion(a, b)
	if isAny(a) || isAny(b) { // a gem's `{build_env: true}.merge(options)` with options untyped: MRI's result holds both
		jt = TAny{}
	}
	j := TClass{C: f.c.classes[cls], Args: append(slices.Clone(ra[:len(ra)-1]), jt)}
	r := f.coerce(n, recv, TAny{})
	r = f.pinSimple(r)
	r = f.noteConv(n, fmt.Sprintf("rbAs[%s](%s, %q)", f.c.goType(j), r, j.String()))
	arg := f.genExpr(args[0], nil)
	ac := f.noteConv(n, fmt.Sprintf("rbAs[%s](%s, %q)", f.c.goType(j), f.coerce(args[0], arg, TAny{}), j.String()))
	return f.genMethodCall(n, expr{code: r, typ: j}, name, []parser.Node{&exprNode{e: expr{code: ac, typ: j}}}, nil), true
}

// genCallValue renders a call whose value is used. For an attribute write
// (`recv.x = v`, `recv[k] = v`) Ruby's value is v, whatever the setter
// returns, so v is evaluated once and named after the call.
func (f *fctx) genCallValue(n *parser.CallNode, expected Type) expr {
	if r := f.loopRescue(n); r != nil {
		return f.lift(r, expected, func(t tail) { f.genStmt(r, t) })
	}
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

// kernelCall calls a private Kernel prelude method as a receiverless call
// on main would: what user syntax like backticks and $? stands for.
func (f *fctx) kernelCall(n parser.Node, name string, args []parser.Node) expr {
	f.implicitCall = true
	defer func() { f.implicitCall = false }()
	return f.genMethodCall(n, expr{code: "rb_main", typ: f.cls("Object")}, name, args, nil)
}
