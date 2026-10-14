package compiler

import (
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"

	"github.com/danielgatis/go-ruby-prism/parser"
)

// genAnonClass is a Class.new/Module.new literal: the class declared for it
// (decision 145). An unnamed Class.new runs Super.inherited each time it is
// evaluated, as MRI's; a named one's hook runs where its constant is
// assigned (collectNamedAnon).
func (f *fctx) genAnonClass(n *parser.CallNode, cls *Class, expected Type) expr {
	if cls.isPerRun {
		return f.genPerRunClass(n, cls)
	}
	if f.c.unnamedAnon[n] {
		if h := f.c.hookCall(classHook{name: "inherited", cls: cls, node: n}); h != nil {
			f.genStmt(h, tail{})
		}
	}
	return f.genExpr(&parser.ConstantReadNode{Name: cls.RubyName, Location: n.Location}, expected)
}

// useGenNew is true when `Foo.new` is the direct Go constructor (genNew)
// rather than a call to a class method: no metaclass, a synthesized new the
// call's arity does not overload, a bare Hash.new (its args/block go to
// Hash.self.new / __new_block, decision 5), or a bare generic `.new`.
func (f *fctx) useGenNew(cls *Class, n *parser.CallNode) bool {
	if n.Name != "new" {
		return false
	}
	if cls.hashBase {
		return true // the generated NewX builds the embedded Hash or Array; genNew runs the subclass's own initialize on it
	}
	if cls.meta == nil {
		return true
	}
	if isSynthNew(cls.meta.lookup("new")) && !f.newOverloaded(cls, n) {
		return true
	}
	if cls == f.c.classes["Hash"] && n.Arguments == nil && n.Block == nil {
		return true
	}
	if len(cls.TypeParams) > 0 && n.Arguments == nil && (n.Block == nil || cls.meta.lookup("new") == nil) {
		return true
	}
	return false
}

// newOverloaded is true when initialize cannot take the call's argument count
// and the class defines decision 12's `self.__new_<count>` (TCPServer.new(port)).
func (f *fctx) newOverloaded(cls *Class, n *parser.CallNode) bool {
	args := callArgs(n)
	if cls.meta == nil || cls.meta.lookup("__new_"+strconv.Itoa(len(args))) == nil {
		return false
	}
	init := cls.lookup("initialize")
	if init == nil {
		return true
	}
	rest := slices.ContainsFunc(init.M.Params, func(p Param) bool { return p.Rest })
	return len(args) < requiredArgs(init.M) || !rest && len(args) > len(init.M.Params)
}

func (f *fctx) genNew(n parser.Node, cls *Class, args []parser.Node, exprs []expr, expected Type, block parser.Node) expr {
	if err, bad := f.c.badClasses[cls]; bad {
		panic(err)
	}
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
		return f.genGoTypeNew(n, cls, args, exprs, expected, block)
	}
	if vr := cls.valueRoot(); vr != nil && exprs == nil && generatedInit(cls) {
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
		if init.M.Block != nil {
			blkCode := "nil"
			switch {
			case block != nil:
				f.inferYielder(n, init.M, env, block)
				blkCode = f.genClosure(n, block, init.M.Block, env)
			case !init.M.Block.Optional:
				f.errorf(n, "%s.new requires a block", cls.RubyName)
			}
			codes = append(codes, blkCode)
		}
	} else if len(args) > 0 || len(exprs) > 0 {
		f.errorf(n, "%s.new takes no arguments", cls.Name)
	}
	return expr{code: "New" + cls.Name + "(" + strings.Join(codes, ", ") + ")", typ: TClass{C: cls}, ctor: true}
}

// goTypeNewType is the type a blockless `X.new` of a @go_type class builds: the expected one, X itself, an untyped
// core container (decision 192), or a compile error; a Hash subclass's V defaults to untyped.
func (f *fctx) goTypeNewType(n parser.Node, cls *Class, expected Type) TClass {
	var t TClass
	switch et, ok := expected.(TClass); {
	case ok && et.C == cls:
		t = et
	case cls.hashBase || len(cls.TypeParams) == 0:
		t = TClass{C: cls}
	case untypedContainers[cls.RubyName] && !f.f.isPrelude():
		f.warn(n, "%s.new's element types are untyped: nothing in the program gives them a type (annotate `#: %s[...]` for static calls; decision 192)", cls.Name, cls.Name)
		t = TClass{C: cls}
		for range cls.TypeParams {
			t.Args = append(t.Args, TAny{})
		}
	default:
		f.errorf(n, "%s.new needs a type annotation (`#: %s[...]`)", cls.Name, cls.Name)
	}
	if cls.hashBase {
		for len(t.Args) < len(cls.TypeParams) { // a Hash subclass's V defaults to untyped
			t.Args = append(t.Args, TAny{})
		}
	}
	return t
}

// generatedInit reports a Struct/Data whose initialize is the generated one: a block's own `def initialize(amount:)`
// takes the keywords itself.
func generatedInit(cls *Class) bool {
	init := cls.lookup("initialize")
	return init == nil || init.M.valueGen
}

// keywordMembers turns `Point.new(x: 1, y: 2)` into positional arguments
// in member order. Missing members are an error for Data and nil for a
// Struct.
func (f *fctx) keywordMembers(n parser.Node, vr *Class, args []parser.Node) []parser.Node {
	if vr.kwInit && len(args) > 0 {
		if _, kw := args[0].(*parser.KeywordHashNode); !kw || len(args) > 1 {
			f.errorf(n, "wrong number of arguments (given %d, expected 0): %s takes keywords (keyword_init: true)", len(args), vr.RubyName)
		}
	}
	if len(args) != 1 {
		return args
	}
	kw, ok := args[0].(*parser.KeywordHashNode)
	if !ok {
		return args
	}
	byName, order, bad := kwMap(kw)
	if bad != nil {
		return args // a splat or a Hash argument, not keywords
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
	for _, k := range order {
		if _, left := byName[k]; left {
			f.errorf(n, "unknown keyword: :%s", k)
		}
	}
	return out
}

// kwMap: a keyword hash by name plus source order (a map ranges randomly); bad is the first non-symbol pair, else nil.
func kwMap(kw *parser.KeywordHashNode) (byName map[string]parser.Node, order []string, bad parser.Node) {
	byName = map[string]parser.Node{}
	for _, el := range kw.Elements {
		a, ok := el.(*parser.AssocNode)
		if !ok {
			return nil, nil, el
		}
		sym, isSym := a.Key.(*parser.SymbolNode)
		if !isSym {
			return nil, nil, el
		}
		if _, dup := byName[sym.Unescaped.Value]; !dup {
			order = append(order, sym.Unescaped.Value)
		}
		byName[sym.Unescaped.Value] = a.Value
	}
	return byName, order, nil
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
	case TAny, TFunc, TNil, TOpt, TTuple, TUnion, TVoid: // no class known here: not a Data value's with
	}
	if cls == nil || cls.valueRoot() == nil || cls.valueRoot().valueKind != "data" {
		return expr{}, false
	}
	if len(args) == 0 {
		return recv, true // MRI: `with` without keywords is the receiver
	}
	vr := cls.valueRoot()
	byName := map[string]parser.Node{}
	var order []string
	if len(args) == 1 {
		kw, ok := args[0].(*parser.KeywordHashNode)
		if !ok {
			f.errorf(n, "with takes keyword arguments")
		}
		var bad parser.Node
		if byName, order, bad = kwMap(kw); bad != nil {
			f.errorf(bad, "with takes keyword arguments")
		}
	} else if len(args) > 1 {
		f.errorf(n, "with takes keyword arguments")
	}
	recv.code = f.pinSimple(recv.code)
	nodes := make([]parser.Node, len(vr.valueMembers))
	for i, m := range vr.valueMembers {
		v, ok := byName[m]
		if !ok {
			v = &withMember{recv: recv, name: m}
		}
		delete(byName, m)
		nodes[i] = v
	}
	for _, k := range order {
		if _, left := byName[k]; left {
			f.errorf(n, "unknown keyword: :%s", k)
		}
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

// containerCallCode is callCode on a subclass of a core class (decision 202): an inherited method runs on the
// embedded base, a middle subclass's own method on its embedded struct.
func (f *fctx) containerCallCode(e *entry, recv expr, args []string, env map[string]Type) (string, bool) {
	m := e.M
	argList := strings.Join(args, ", ")
	if rc, ok := recv.typ.(TClass); ok && rc.C.hashBase && m.Owner == rc.C.Super && !rc.C.Super.mutable() {
		// a value base's method (Pathname's) on the embedded value, called as the base calls it
		innerEnv := composeEnv(env, nil)
		innerEnv["Self"] = TClass{C: rc.C.Super}
		return f.callCode(e, expr{code: embeddedSuper(rc.C, recv.code), typ: TClass{C: rc.C.Super}}, args, innerEnv), true
	}
	if rc, ok := recv.typ.(TClass); ok && rc.C.hashBase && m.Owner == rc.C.Super && rc.C.Super.hashBase {
		// Concurrent::Set < CRubySet < ::Set: the middle class's own method is a Go method on the embedded struct
		return embeddedSuper(rc.C, recv.code) + "." + m.GoName + "(" + argList + ")", true
	}
	if rc, ok := recv.typ.(TClass); ok && rc.C.hashBase && m.Owner == rc.C.Super {
		// an inherited Hash (or Array) method runs on the embedded Hash[any, V] (Array[E])
		targs := f.c.containerTargs(rc.C, rc.Args)
		call := func(base string) string { return freeFuncName(m) + targs + "(" + base + comma(argList) + ")" }
		if f.c.isDirectMethod(m) { // a non-generic base's own Go method (Thread#name, Time#-): no free func exists
			call = func(base string) string { return base + "." + m.GoName + "(" + argList + ")" }
		}
		if tv, ok := m.Ret.(TVar); ok && tv.Name == "Self" { // `-> self` (Set#add): the subclass object, not its embedded base
			t := f.c.goType(recv.typ)
			return fmt.Sprintf("func(r %s) %s { %s; return r }(%s)", t, t, call(embeddedSuper(rc.C, "r")), recv.code), true
		}
		return call(embeddedSuper(rc.C, recv.code)), true
	}
	if rc, ok := recv.typ.(TClass); ok && rc.C.hashBase && m.Owner != rc.C && !m.Owner.universal && slices.Contains(rc.C.Super.ancestors(), m.Owner) {
		// one the container has from a module (Array's Enumerable): called on the embedded container too
		inner := TClass{C: rc.C.Super}
		if rc.C.Super.RubyName == "Hash" {
			inner.Args = append([]Type{TAny{}}, rc.Args...)
		} else {
			for range rc.C.Super.TypeParams {
				inner.Args = append(inner.Args, TAny{})
			}
		}
		innerEnv := composeEnv(env, nil)
		innerEnv["Self"] = inner
		binds := map[string]Type{}
		rc.C.containerEnv(binds)
		if rc.C.Super.RubyName == "Hash" && len(rc.Args) > 0 {
			binds["V"] = rc.Args[0]
		}
		for k, v := range innerEnv { // Enumerable's E is Hash's [K, V]: bind those inside it
			innerEnv[k] = subst(v, binds)
		}
		maps.Copy(innerEnv, binds)
		return f.callCode(e, expr{code: embeddedSuper(rc.C, recv.code), typ: inner}, args, innerEnv), true
	}
	return "", false
}

// genGoTypeNew is `new` on a @go_type class: its Go constructor, or for a subclass of one (decision 202) the embedded
// base built and its own initialize run.
func (f *fctx) genGoTypeNew(n parser.Node, cls *Class, args []parser.Node, exprs []expr, expected Type, block parser.Node) expr {
	init := cls.lookup("initialize")
	ownInit := init != nil && init.Owner == cls
	baseStarts := init != nil && init.Owner == cls.Super && init.M.Block != nil // Thread's: starts on the embedded base (decision 239)
	// a container subclass's initialize runs on the generated empty one
	if cls.hashBase && (ownInit || baseStarts) {
		t := f.goTypeNewType(n, cls, expected)
		tmp := f.newTmp()
		targs := ""
		if len(t.Args) > 0 {
			targs = "[" + f.c.goTypes(t.Args) + "]"
		}
		f.emit("%s := New%s%s()", tmp, cls.Name, targs)
		obj := expr{code: tmp, typ: t}
		prev := f.implicitCall
		f.implicitCall = true // new calls the private initialize
		init := f.callEntry(n, init, obj, args, block)
		f.implicitCall = prev
		if init.code != "" {
			f.emit("%s", init.code)
		}
		return obj
	}
	if sup := cls.Super; cls.hashBase && len(exprs) == 0 && len(args) > 0 && !sup.mutable() && sup.meta != nil && (init == nil || init.Owner == sup) {
		// `Name.new("bob")` on a String (Pathname) subclass: the base's own new builds the embedded value
		tmp := f.newTmp()
		f.emit("%s := New%s()", tmp, cls.Name)
		built := f.genMethodCall(n, expr{code: classVar(sup), typ: TClass{C: sup.meta}, classObj: true}, "new", args, block)
		f.emit("%s.%s = %s", tmp, superField(sup), f.coerce(n, built, TClass{C: sup}))
		return expr{code: tmp, typ: TClass{C: cls}}
	}
	if len(args) > 0 || len(exprs) > 0 {
		f.errorf(n, "%s.new with arguments is not supported", cls.Name)
	}
	t := f.goTypeNewType(n, cls, expected)
	targs := ""
	if len(t.Args) > 0 {
		targs = "[" + f.c.goTypes(t.Args) + "]"
	}
	return expr{code: "New" + cls.Name + targs + "()", typ: t}
}

// inheritedSelfNew is a gem class method's `new(...)` on self where a subclass defines its own initialize
// (concurrent-ruby's BlockedPromise.new_blocked_by1 called on ChainPromise): the class self is at run time builds the
// object, with that class's initialize, as MRI does (decision 253).
func (f *fctx) inheritedSelfNew(n *parser.CallNode) (expr, bool) {
	if n.Name != "new" || !f.lenient() || n.Receiver != nil && !isSelf(n.Receiver) || f.owner == nil || f.owner.metaOf == nil ||
		!f.owner.metaOf.isStruct() || !f.owner.metaOf.descendantDefines("initialize", true) {
		return expr{}, false
	}
	self := expr{code: f.coerce(n, expr{code: f.selfCode, typ: f.selfType}, TAny{}), typ: TAny{}}
	if n.Block != nil {
		return f.universalBlockCall(n, self, "new", callArgs(n), n.Block), true
	}
	return f.genDynCall(n, self, "new", callArgs(n)), true
}

// genPerRunClass evaluates a Class.new literal whose block runs statements: a fresh class object, Super.inherited
// told of it, then its block's statements (`rb2go_class_body`) run with it as self and the enclosing locals they read
// (decision 259).
func (f *fctx) genPerRunClass(n *parser.CallNode, cls *Class) expr {
	k := f.newTmp()
	f.emit("%s := &%s{}", k, cls.meta.Name)
	obj := &exprNode{Node: n, e: expr{code: k, typ: TClass{C: cls.meta}}}
	if h, ok := f.c.hookCall(classHook{name: "inherited", cls: cls, node: n}).(*parser.CallNode); ok {
		h.Arguments = &parser.ArgumentsNode{Arguments: []parser.Node{obj}}
		f.genStmt(h, tail{})
	}
	args := make([]parser.Node, 0, len(cls.perRun))
	for _, name := range cls.perRun {
		args = append(args, &parser.LocalVariableReadNode{Name: name, Location: n.Location})
	}
	f.genStmt(&parser.CallNode{Location: n.Location, Receiver: obj, Name: anonBodyName, Arguments: &parser.ArgumentsNode{Arguments: args}}, tail{})
	return expr{code: k, typ: TClass{C: cls.meta}}
}
