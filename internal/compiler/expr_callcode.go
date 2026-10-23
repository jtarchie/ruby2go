package compiler

import (
	"fmt"
	"math/big"
	"slices"
	"strings"

	"github.com/danielgatis/go-ruby-prism/parser"
)

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

// bindTypeParams checks every type parameter of m is bound, first
// unifying m's return type with the call's expected type, if any
// (`SizedQueue.new(2) #: SizedQueue[Integer]`).
func (f *fctx) bindTypeParams(n parser.Node, m *Method, env map[string]Type) {
	if n == f.retHintNode && f.retHint != nil {
		unify(m.Ret, f.retHint, env)
	}
	for _, tp := range m.TypeParams {
		if _, ok := env[tp]; !ok {
			if f.c.pruneGems && f.c.gemFile(f.f) { // a gem's `Set[]` is a Set[untyped] (decision 169)
				env[tp] = TAny{}
				continue
			}
			if owner := containerOf(m); owner != "" && m.Block == nil && f.f != nil && !f.f.isPrelude() { // a block was typed with the parameter already
				// an empty core container nothing types: untyped elements, its calls dynamic (decision 192)
				f.warn(n, "%s's %s is untyped: nothing in the program gives it a type (annotate `#: %s[...]` for static calls; decision 192)", m, tp, owner)
				env[tp] = TAny{}
				continue
			}
			f.errorf(n, "cannot infer type parameter %s of %s", tp, m)
		}
	}
}

// callEntry calls e; in gem code, a call the static signature cannot take falls back to a run-time dispatch (genDynCall).
func (f *fctx) callEntry(n parser.Node, e *entry, recv expr, args []parser.Node, block parser.Node) expr {
	if f.lenient() && block == nil && len(args) > 0 && f.unionArm == 0 && !slices.ContainsFunc(args, f.readsUnionLocal) { // a union argument: unionArgCall first
		// gem code: arguments the static signature cannot take (a splat, a computed keyword) are matched per call at run time, as MRI does
		if f.probing() { // output is discarded: no dry run needed
			var r expr
			err := f.try(func() { r = f.callEntry0(n, e, recv, args, block) })
			if err != nil {
				return f.genDynCall(n, recv, e.M.Name, args)
			}
			return r
		}
		err := f.try(func() { f.probe(func() { f.callEntry0(n, e, recv, args, block) }) })
		if err != nil {
			return f.genDynCall(n, recv, e.M.Name, args)
		}
	}
	return f.callEntry0(n, e, recv, args, block)
}

func isSplat(a parser.Node) bool {
	_, ok := a.(*parser.SplatNode)
	return ok
}

// callEntry0 calls e; a call to a `bot` method is noreturn, and Go must
// see its panic to know the statement list ends there.
func (f *fctx) callEntry0(n parser.Node, e *entry, recv expr, args []parser.Node, block parser.Node) expr {
	if r, ok := f.gemCallShortcut(n, e, recv, args, block); ok {
		return r
	}
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

// receiverEnv binds the type variables a call to e on recv starts from: the receiver's class arguments and Self.
func receiverEnv(e *entry, recv expr) map[string]Type {
	env := map[string]Type{}
	if rt, ok := recv.typ.(TClass); ok {
		classEnv := map[string]Type{}
		for i, p := range rt.C.TypeParams {
			if i < len(rt.Args) {
				classEnv[p] = rt.Args[i]
			}
		}
		env = composeEnv(e.Env, classEnv)
		rt.C.containerEnv(env) // also a Hash def aliased into the subclass (linkAliases), which keeps Hash's K
	} else {
		for k, v := range e.Env {
			env[k] = v
		}
	}
	env["Self"] = recv.typ
	return env
}

// callEnv is receiverEnv plus what the method's `@self` binds (decision 92).
func (f *fctx) callEnv(n parser.Node, e *entry, recv expr) map[string]Type {
	m := e.M
	env := receiverEnv(e, recv)
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
	if m.Private && !kernelModuleFunction(m, recv.typ) {
		f.errorf(n, "private method %s called on %s", m.Name, recv.typ)
	}
	if m.Protected && (f.owner == nil || !f.owner.isSubclassOf(m.Owner)) {
		f.errorf(n, "protected method %s called on %s", m.Name, recv.typ)
	}
}

// MRI's Kernel.singleton_methods; the rest of Kernel's private methods (pp, initialize_copy) stay private on Kernel too.
var kernelModuleNames = func() map[string]bool {
	out := map[string]bool{}
	for _, s := range strings.Fields("Array Complex Float Hash Integer Pathname Rational String __callee__ __dir__ __method__ ` abort at_exit autoload autoload? binding block_given? caller caller_locations catch eval exec exit exit! fail fork format gets global_variables iterator? lambda load local_variables loop open p print printf proc putc puts raise rand readline readlines require require_relative select set_trace_func sleep spawn sprintf srand syscall system test throw trace_var trap untrace_var warn") {
		out[s] = true
	}
	return out
}()

func kernelMeta(t Type) bool {
	c, ok := t.(TClass)
	return ok && c.C.metaOf != nil && c.C.metaOf.RubyName == "Kernel"
}

// isTopConst is a syntactic guess (`Kernel`, `::Kernel`) for passes with no scope to resolve it in; genCall checks the type.
func isTopConst(n parser.Node, name string) bool {
	switch r := n.(type) {
	case *parser.ConstantReadNode:
		return r.Name == name
	case *parser.ConstantPathNode:
		return r.Parent == nil && r.Name != nil && *r.Name == name
	}
	return false
}

// kernelRecv: a receiverless call, or one on Kernel, the module function form.
func kernelRecv(n *parser.CallNode) bool {
	return n.Receiver == nil || isTopConst(n.Receiver, "Kernel")
}

// `Kernel.puts`: module_function makes these public on Kernel itself (decision 139).
func kernelModuleFunction(m *Method, recv Type) bool {
	if !kernelMeta(recv) || m.Owner == nil || m.Owner.RubyName != "Kernel" {
		return false
	}
	if kernelModuleNames[m.Name] {
		return true
	}
	for pub := range kernelModuleNames { // an overload twin (`__Integer_string`, decision 12) stands in for its public method
		if strings.HasPrefix(m.Name, "__"+overloadBase(pub)+"_") {
			return true
		}
	}
	return false
}

// callMethod is callMethod0 with binary Strings (decision 352): a `@binary` method's String result is binary.
func (f *fctx) callMethod(n parser.Node, e *entry, recv expr, args []parser.Node, block parser.Node) expr {
	out := f.callMethod0(n, e, recv, args, block)
	if b := e.M.binary; (b == 1 || b == 2 && recv.bin) && isClass(stripOpt(out.typ), "String") {
		out.bin = true
		if b == 2 {
			out.binDeps = recv.binDeps
		}
	}
	return out
}

// binRegexpTwins are the Regexp methods whose String argument, when binary, takes a `__bin_*` twin.
var binRegexpTwins = map[string]bool{"=~": true, "match": true, "match?": true, "===": true}

// binTwin is m's `__bin_<name>` on a binary receiver, or a Regexp's on a binary argument (decision 352). It runs
// after overload picks a twin (decision 12), so `__gsub_block` has `__bin_gsub_block`.
func (f *fctx) binTwin(m *Method, recv expr, args []parser.Node) *entry {
	c, ok := recv.typ.(TClass)
	if !ok || strings.HasPrefix(m.Name, "__bin_") {
		return nil
	}
	base := "__bin_" + overloadBase(m.Name)
	if strings.HasPrefix(m.Name, "__") {
		base = "__bin_" + m.Name[2:]
	}
	switch {
	case recv.bin && c.C.RubyName == "String":
	case c.C.RubyName == "Regexp" && len(args) == 1 && binRegexpTwins[m.Name]:
		var a expr
		f.probe(func() { a = f.genExpr(args[0], nil) })
		if !a.bin {
			return nil
		}
		return c.C.lookup(base)
	default:
		return nil
	}
	if e := c.C.lookup(base); e != nil && len(args) >= requiredArgs(e.M) && len(args) <= len(e.M.Params) {
		return e
	}
	return nil
}

func (f *fctx) callMethod0(n parser.Node, e *entry, recv expr, args []parser.Node, block parser.Node) expr {
	m := e.M
	f.c.inferRet(m)
	if o := f.nilableFetch(m, args, block); o != nil {
		return f.callEntry(n, o, recv, args, block)
	}
	if r, ok := f.gemCallForm(n, e, recv, args, block); ok {
		return r
	}
	f.curBlock = block
	o := f.overload(e, recv.typ, args)
	f.curBlock = nil
	if o != nil {
		return f.callEntry(n, o, recv, args, block)
	}
	if o := f.binTwin(m, recv, args); o != nil {
		return f.callEntry(n, o, recv, args, block)
	}
	env := f.callEnv(n, e, recv)
	f.checkVisibility(n, m, recv)
	var armT Type // the `.rbs` overload arm this call takes (decision 12), picked before the merged parameters check the arguments
	if len(m.arms) > 0 {
		armT = f.armRet(n, m, args)
	}
	codes, restIdx := f.genArgs(n, m, env, args, nil)
	if m.Block != nil && m.Iterator && block == nil && !m.Block.Optional {
		if r, ok := f.iterEnum(n, e, recv, codes, env); ok {
			return r
		}
	}
	if m.Block != nil {
		if m.Iterator {
			f.errorf(n, "%s is an iterator (its block returns void); call it as a statement with a block", m.Name)
		}
		// Bind type params from the expected type before the block's closure,
		// whose parameter types may name them (`Hash.new { |h, k| }`).
		if n == f.retHintNode && f.retHint != nil {
			unify(m.Ret, f.retHint, env)
		}
		blkCode := f.blockArg(n, m, env, block)
		if restIdx >= 0 {
			codes = slices.Insert(codes, restIdx, blkCode)
		} else {
			codes = append(codes, blkCode)
		}
	} else if block != nil && (!f.lenient() || m.File != nil && m.File.isPrelude()) {
		// gem code: MRI ignores a block the method never yields to; a core method's block form the prelude lacks
		// (`xs.uniq(&:name)`) is an error there too, not a silently different answer
		f.errorf(n, "%s does not take a block", m.Name)
	}
	f.bindTypeParams(n, m, env)
	ret := subst(m.Ret, env)
	out := expr{code: f.callCode(e, recv, codes, env), typ: ret}
	out = f.blocklessReturn(n, m, block, env, out)
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
	if f.c.selfReturnAsserts(e, recv.typ) {
		out.code, out.assert = out.code+".("+f.c.goType(recv.typ)+")", true
	} else if f.c.selfVarReturns(e, recv.typ) {
		out.code, out.assert = "any("+out.code+").(Self)", true
	}
	if armT != nil {
		return f.narrowToArm(n, flatOpt(out), subst(armT, env))
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
	case isUnion(o.Elem): // E? with E = A | B: the box opens into the union, nil a member (decision 150)
		return expr{code: "Opt(" + e.code + ")", typ: optOf(o.Elem)}
	case isOpt(o.Elem):
		return expr{code: "rbFlat(" + e.code + ")", typ: o.Elem}
	}
	return e
}

// newBySubclass is `klass.new(v)` where klass is a class or one of its subclasses and some subclass's new takes other Go
// parameter types (Homebrew's Version::Token.create: StringToken.new(String), NumericToken.new(String | Integer)): one
// interface assertion cannot name both, so a type switch picks the class and converts each argument to its new's
// parameter (decision 290). Positional parameters only; anything else keeps the single assertion.
func (f *fctx) newBySubclass(m *Method, recv expr, args []string, env map[string]Type, ret string) (string, bool) {
	if m.calleeDefaults || m.kwMask() || m.Block != nil || len(args) != len(m.Params) || slices.ContainsFunc(m.Params, func(p Param) bool { return p.Rest || p.Keyword || p.KwRest }) {
		return "", false
	}
	type arm struct {
		meta *Class
		m    *Method
	}
	var arms []arm
	queue := []*Class{m.Owner.metaOf}
	for len(queue) > 0 {
		k := queue[0]
		queue = queue[1:]
		queue = append(queue, k.Subclasses...)
		if k.meta == nil {
			continue
		}
		e := k.meta.lookup("new")
		if e == nil || e.M == m || len(e.M.Params) != len(m.Params) || e.M.calleeDefaults || e.M.kwMask() || e.M.Block != nil ||
			slices.ContainsFunc(e.M.Params, func(p Param) bool { return p.Rest || p.Keyword || p.KwRest }) {
			continue
		}
		arms = append(arms, arm{k.meta, e.M})
	}
	if len(arms) == 0 {
		return "", false
	}
	pinned := make([]string, len(args))
	var b strings.Builder
	b.WriteString("func() " + ret + " {\n")
	for i, a := range args {
		pinned[i] = fmt.Sprintf("a%d", i)
		fmt.Fprintf(&b, "var %s %s = %s\n", pinned[i], f.c.goType(subst(m.Params[i].Type, env)), a)
	}
	b.WriteString("r := any(" + recv.code + ")\nswitch c := r.(type) {\n")
	for _, a := range arms {
		conv := make([]string, len(pinned))
		for i, p := range a.m.Params {
			base := subst(m.Params[i].Type, env)
			if typeEq(base, p.Type) {
				conv[i] = pinned[i]
				continue
			}
			// the subclass may take less than the base (String where String? is): checked when this class is the one
			boxed := f.coerce(nil, expr{code: pinned[i], typ: base}, TAny{})
			conv[i] = f.coerce(nil, expr{code: boxed, typ: TAny{}}, p.Type)
		}
		fmt.Fprintf(&b, "case *%s:\nreturn c.New(%s)\n", a.meta.Name, strings.Join(conv, ", "))
	}
	ps, _ := f.c.sig(m, env)
	fmt.Fprintf(&b, "}\nreturn r.(interface{ New(%s) %s }).New(%s)\n}()", ps, ret, strings.Join(pinned, ", "))
	return b.String(), true
}

// callCode renders the call for entry e.
func (f *fctx) callCode(e *entry, recv expr, args []string, env map[string]Type) string {
	m := e.M
	argList := strings.Join(args, ", ")
	recv.code = f.materialize(recv)
	if m.Owner == nil {
		return m.GoName + "(" + argList + ")"
	}
	if code, ok := f.containerCallCode(e, recv, args, env); ok {
		return code
	}
	if m.Owner.metaOf != nil && m.Name == "new" && !recv.classObj {
		// `new` on a class object of unknown exact class: assert for it.
		ps, ret := f.c.sig(m, env)
		if code, ok := f.newBySubclass(m, recv, args, env, ret); ok {
			return code
		}
		return "any(" + recv.code + ").(interface{ New(" + ps + ") " + ret + " }).New(" + argList + ")"
	}
	if code, ok := f.dispatchCall(e, recv, argList); ok {
		return code
	}
	// A primitive's non-direct method is called by its free func, never its forwarder, so the pruner drops unused forwarders (decision 86).
	direct := f.c.isDirectMethod(m)
	free := f.staticDef == m || m.generic() || (staticPrivate(m) && !direct) || (m.Owner.GoType == "" && !f.hasForwarder(recv.typ, e)) || (m.Owner.GoType != "" && !direct) ||
		m.Owner.universal && mentionsSelf(m.Ret) // an interface declares `-> self` as `any`; the free func returns Self
	if !free && f.plainCalls && recv.code == "self" && f.owner != nil && !direct {
		if code, ok := f.wrapperSelfCall(e, argList); ok {
			return code
		}
	}
	if !free && m.Name == "initialize" && !inInterface(m) {
		// an overridden private initialize dispatches (decision 253) but is never an interface slot: through an
		// interface-typed receiver (SystemCallErrorI) it is the free func
		t := f.methodExprType(m, recv)
		free = t != "" && !strings.HasPrefix(t, "(")
	}
	if !free {
		if t := f.methodExprType(m, recv); t != "" {
			return t + "." + m.GoName + "(" + recv.code + comma(argList) + ")"
		}
		return recv.code + "." + m.GoName + "(" + argList + ")"
	}
	return staticCallCode(m, f.typeArgs(m, recv.typ, env), recv.code, argList)
}

// wrapperSelfCall is a wrapper body's self call: the owner's free func over its interface (any for Kernel), so no
// forwarder is selected (decision 209).
func (f *fctx) wrapperSelfCall(e *entry, argList string) (string, bool) {
	m := e.M
	targ := "any"
	if !m.Owner.universal {
		targ = f.c.ownerIface(f.owner, e)
	}
	switch {
	case targ != "" && f.c.plainSelf(m):
		return freeFuncName(m) + "(self" + comma(argList) + ")", true
	case targ != "":
		return freeFuncName(m) + "[" + targ + "](self" + comma(argList) + ")", true
	}
	return "", false
}

// dispatchCall renders a call that bypasses Go method dispatch (dispatch.go): a direct call or an rbSend_ type switch,
// for a dispatchable method on a struct-typed receiver. Not for the method a Method object binds (staticDef: MRI runs
// that definition, not an override) nor for a Dyn wrapper body's self call (its owner's free func, decision 209).
func (f *fctx) dispatchCall(e *entry, recv expr, argList string) (string, bool) {
	if f.staticDef == e.M || f.plainCalls && recv.code == "self" {
		return "", false
	}
	owner := f.staticStruct(recv)
	if owner == nil || !f.c.dispatchable(e.Owner, e.M) {
		return "", false
	}
	if owner.IsModule && !owner.universal && !e.Owner.universal && e.Owner != owner && e.Owner.IsModule {
		// self in a module body reaching a module it includes: everything that includes this module includes that
		// one, so self is one of its includers, asserted to its constraint (decision 234)
		mod := e.Owner.selfName() + "[any" + comma(strings.Join(f.c.moduleCallArgs(e), ", ")) + "]"
		return f.c.sendCall(e, "any("+recv.code+").("+mod+")", argList), true
	}
	if owner.IsModule && !owner.universal && !e.Owner.universal && e.Owner != owner {
		return "", false // self in a module body reaching another class's method: that is the constraint's
	}
	return f.c.sendCall(e, recv.code, argList), true
}

// staticStruct is the non-generic struct class a receiver is statically known to be an instance of (a subclass at run
// time), or nil: a class-typed value, or self in a struct class body.
func (f *fctx) staticStruct(recv expr) *Class {
	switch t := recv.typ.(type) {
	case TClass:
		if t.C.isStruct() && len(t.C.TypeParams) == 0 && !t.C.hashBase && !t.C.universal {
			return t.C
		}
	case TVar:
		if t.Name == "Self" && f.owner != nil && (f.owner.isStruct() && len(f.owner.TypeParams) == 0 || f.owner.IsModule) {
			return f.owner // a struct class's body, or a module's: self is some includer, a universal name dispatches over all
		}
	case TAny, TFunc, TNil, TOpt, TTuple, TUnion, TVoid:
	}
	return nil
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
			return f.owner.ifaceName()
		}
	case TAny, TFunc, TNil, TOpt, TTuple, TUnion, TVoid:
	}
	return ""
}

// typeArgs are explicit because Go would infer from argument Go types (untyped const, *Foo, *Foo_Meta), not the Ruby binding.
func (f *fctx) typeArgs(m *Method, recvT Type, env map[string]Type) string {
	ts := []Type{}
	if m.Owner.GoType == "" && !f.c.plainSelf(m) {
		ts = append(ts, recvT)
	}
	for _, p := range m.Owner.TypeParams {
		ts = append(ts, env[p])
	}
	for _, p := range m.TypeParams {
		ts = append(ts, env[p])
		if o, ok := env[p].(TOpt); ok && isClassValue(o.Elem) { // the generic body's rbAs[U] boxes into it at run time
			if f.c.optConv == nil {
				f.c.optConv = map[string]bool{}
			}
			f.c.optConv[f.c.goType(o.Elem)] = true
		}
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
	if e.Owner == nil || staticPrivate(e.M) || e.M.generic() {
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
	case TAny, TFunc, TNil, TOpt, TTuple, TUnion, TVoid: // no Go method set to forward to
	}
	return false
}

// selfReturnAsserts reports whether a `-> self` call on a struct value needs a Go type assertion: an inherited
// forwarder returns the defining class's interface. A universal method's is its typed free func (callCode).
func (c *Compiler) selfReturnAsserts(e *entry, recvT Type) bool {
	v, ok := e.M.Ret.(TVar)
	if !ok || v.Name != "Self" || e.Owner == nil || e.M.Owner.universal {
		return false
	}
	rc, ok := recvT.(TClass)
	return ok && rc.C.isStruct() && e.Entry != rc.C
}

// selfVarReturns reports whether a `-> self` call on self in a generic method body (MacOSVersion#strip_patch calling
// Version#major_minor) needs asserting back to Self: the Go interface method returns its class's interface.
func (c *Compiler) selfVarReturns(e *entry, recvT Type) bool {
	v, ok := e.M.Ret.(TVar)
	return ok && v.Name == "Self" && isSelfVar(recvT) && e.Owner != nil && !e.M.Owner.universal
}

// genInMethodRequire is a require in a method body: resolved at run time against what was compiled (decision 197),
// else a no-op for a library the prelude has, and a compile error for a file rb2go would have to load.
func (f *fctx) genInMethodRequire(n *parser.CallNode) expr {
	if f.c.methodRequiresRuntime && !f.f.isPrelude() {
		return f.genRuntimeRequire(n)
	}
	switch {
	case n.Name == "require_relative" && !f.f.isPrelude():
		f.errorf(n, "require_relative must be a top-level statement: rb2go loads files at compile time")
	case n.Name == "require" && f.c.findRequire(f.f, n) != "":
		f.errorf(n, "require of a file on the -I path must be a top-level statement: rb2go loads files at compile time")
	}
	return expr{code: "", stmt: true, typ: TVoid{}}
}

// bindUntyped binds each of m's type parameters ts name that nothing bound yet to untyped.
func bindUntyped(m *Method, env map[string]Type, ts ...Type) {
	var vs []string
	for _, t := range ts {
		freeVars(t, &vs)
	}
	for _, v := range vs {
		if _, bound := env[v]; !bound && slices.Contains(m.TypeParams, v) {
			env[v] = TAny{}
		}
	}
}

// isClassValue: a concrete class type (no type variables, not Object's any), whose T? is a pointer to its Go type.
func isClassValue(t Type) bool {
	c, ok := t.(TClass)
	return ok && !c.C.universal && !mentionsVar(t)
}

// blocklessReturn converts a call's untyped result to the gem sig's `U | String` without the members only the
// absent block could bind (decision 321): `popen_read(cmd)` is a String where `popen_read(cmd) { |io| ... }` is not.
func (f *fctx) blocklessReturn(n parser.Node, m *Method, block parser.Node, env map[string]Type, out expr) expr {
	if m.retUnion == nil || !isAny(out.typ) || block != nil || m.Block == nil {
		return out
	}
	var blockVars []string
	freeVars(TFunc{Params: m.Block.Params, Ret: m.Block.Ret}, &blockVars)
	t := subst(dropVars(m.retUnion, blockVars), env)
	if isAny(t) || mentionsVar(t) {
		return out
	}
	return expr{code: f.coerce(n, out, t), typ: t}
}

// dropVars removes from a union return type (retUnion) the method type variables in vars, those only a block the
// call did not pass could bind (decision 321): Sorbet's `.returns(T.any(T.type_parameter(:U), String))` is String
// when no block is given.
func dropVars(t Type, vars []string) Type {
	u, ok := t.(TUnion)
	if !ok {
		return t
	}
	var keep []Type
	for _, m := range u.Members {
		if v, isVar := stripOpt(m).(TVar); isVar && slices.Contains(vars, v.Name) {
			continue
		}
		keep = append(keep, m)
	}
	if len(keep) == 0 {
		return TAny{}
	}
	return unionOf(keep...)
}

// foldInt is an Integer expression of literals only (`(1 << 64) - 1`), computed exactly.
func (f *fctx) foldInt(n parser.Node) (*big.Int, bool) {
	switch n := n.(type) {
	case *parser.IntegerNode:
		v, ok := new(big.Int).SetString(strings.ReplaceAll(f.f.text(n.Location), "_", ""), 0)
		return v, ok
	case *parser.ParenthesesNode:
		if st, ok := n.Body.(*parser.StatementsNode); ok && len(st.Body) == 1 {
			return f.foldInt(st.Body[0])
		}
	case *parser.CallNode:
		if n.Receiver == nil || n.Block != nil {
			return nil, false
		}
		a, ok := f.foldInt(n.Receiver)
		if !ok {
			return nil, false
		}
		args := callArgs(n)
		if n.Name == "-@" && len(args) == 0 {
			return new(big.Int).Neg(a), true
		}
		if len(args) != 1 {
			return nil, false
		}
		b, ok := f.foldInt(args[0])
		if !ok {
			return nil, false
		}
		switch n.Name {
		case "+":
			return new(big.Int).Add(a, b), true
		case "-":
			return new(big.Int).Sub(a, b), true
		case "*":
			return new(big.Int).Mul(a, b), true
		case "<<":
			if b.Sign() >= 0 && b.IsInt64() && b.Int64() < 1024 {
				return new(big.Int).Lsh(a, uint(b.Int64())), true //nolint:gosec // 0 <= b < 1024, checked above
			}
		}
	}
	return nil, false
}

// newSigsDiffer reports whether a subclass's new takes keywords or a rest that m's Go signature does not name alike.
func (f *fctx) newSigsDiffer(m *Method) bool {
	ps, _ := f.c.sig(m, nil)
	queue := slices.Clone(m.Owner.metaOf.Subclasses)
	for len(queue) > 0 {
		k := queue[0]
		queue = append(queue[1:], k.Subclasses...)
		if k.meta == nil {
			continue
		}
		if e := k.meta.lookup("new"); e != nil && e.M != m && (e.M.kwMask() || m.kwMask() || slices.ContainsFunc(e.M.Params, func(p Param) bool { return p.Keyword || p.KwRest })) {
			if qs, _ := f.c.sig(e.M, nil); qs != ps {
				return true
			}
		}
	}
	return false
}

// gemCallShortcut is a gem's call that decision 324 compiles otherwise: a `new` over subclasses of another keyword
// shape (dispatched at run time), unpack's wrapping twin, literal Integer arithmetic folded, a wrapping `<<`.
func (f *fctx) gemCallShortcut(n parser.Node, e *entry, recv expr, args []parser.Node, block parser.Node) (expr, bool) {
	if f.lenient() && f.m != nil && e.M.Name == "new" && e.M.Owner != nil && e.M.Owner.metaOf != nil && !recv.classObj && f.newSigsDiffer(e.M) {
		// a gem's `klass.new(h, s, *args, **kw)` over subclasses whose initialize takes other keywords (elftools'
		// Section.create): dispatched on the class object at run time (decision 324)
		return f.genDynCall(n, recv, "new", args), true
	}
	if f.lenient() && block == nil && (e.M.Name == "unpack" || e.M.Name == "unpack1") && e.M.Owner == f.c.classes["String"] {
		if w := f.c.classes["String"].lookup("__" + e.M.Name + "_wrap"); w != nil { // a gem's 64-bit fields wrap (decision 324)
			return f.callMethod(n, w, recv, args, block), true
		}
	}
	if f.lenient() && block == nil && e.M.Owner == f.c.classes["Integer"] {
		return f.gemIntegerCall(n, e, recv, args)
	}
	return expr{}, false
}

// gemIntegerCall is a gem's Integer arithmetic: literals folded exactly and saturated to int64, `<<` mod 2**64, as
// Go's shift (decision 324).
func (f *fctx) gemIntegerCall(n parser.Node, e *entry, recv expr, args []parser.Node) (expr, bool) {
	call, isCall := n.(*parser.CallNode)
	if isCall && call.Name == e.M.Name { // not case's `when 0`, which hands its === the literal
		if v, ok := f.foldInt(n); ok {
			if v.IsInt64() {
				return expr{code: "Integer(" + v.String() + ")", typ: f.cls("Integer"), lit: true}, true
			}
			f.warn(n, "Integer %s does not fit in 64 bits (no Bignum); using the nearest", v)
			bound := "Integer(9223372036854775807)" // math.MaxInt64 spelled out: a gem local named math would shadow the package
			if v.Sign() < 0 {
				bound = "Integer(-9223372036854775808)"
			}
			return expr{code: bound, typ: f.cls("Integer"), lit: true}, true
		}
	}
	if e.M.Name == "<<" && len(args) == 1 && !isSplat(args[0]) {
		a := f.genExpr(args[0], f.cls("Integer"))
		return expr{code: "rbIntShlWrap(" + f.coerce(n, recv, f.cls("Integer")) + ", " + f.coerce(args[0], a, f.cls("Integer")) + ")", typ: f.cls("Integer")}, true
	}
	return expr{}, false
}

// untypedTwinArg: a gem's call whose first argument is untyped, of a method with decision-12 twins by class
// (`sub`'s `__sub_regexp`): the dispatcher picks the twin by the argument's run-time class (dynClassOverloads).
func (f *fctx) untypedTwinArg(e *entry, recvT Type, args []parser.Node, block parser.Node) bool {
	m := e.M
	rc, ok := recvT.(TClass)
	if !f.lenient() || block != nil || len(args) == 0 || !ok || strings.HasPrefix(m.Name, "__") || !classTwins(m.Name) {
		return false
	}
	prefix := "__" + overloadBase(m.Name) + "_"
	twin := slices.ContainsFunc(rc.C.methodSet(), func(x entry) bool {
		rest := strings.TrimPrefix(x.M.Name, prefix)
		return rest != x.M.Name && rest != "" && (rest[0] < '0' || rest[0] > '9') && len(x.M.Params) == len(m.Params)
	})
	if !twin {
		return false
	}
	var a expr
	f.probe(func() { a = f.genExpr(args[0], nil) })
	return isAny(a.typ)
}

// nilKeyLookup is a gem's `h[k]` or `h.key?(k)` with k a K? where the Hash's K holds no nil (kramdown's
// `HTML_MARKDOWN_ATTR_MAP[attrs.delete('markdown')]`): MRI's nil key is a miss, so nil answers nil (or false).
func (f *fctx) nilKeyLookup(n parser.Node, e *entry, recv expr, args []parser.Node, block parser.Node) (expr, bool) {
	name := e.M.Name
	miss := map[string]string{"[]": "nil", "key?": "false", "has_key?": "false", "include?": "false", "member?": "false"}[name]
	if !f.lenient() || block != nil || len(args) != 1 || miss == "" || !isClass(recv.typ, "Hash") {
		return expr{}, false
	}
	kt := recv.typ.(TClass).Args[0]
	var a expr
	f.probe(func() { a = f.genExpr(args[0], nil) })
	o, ok := a.typ.(TOpt)
	if !ok || isOpt(kt) || isAny(kt) || !typeEq(o.Elem, kt) {
		return expr{}, false
	}
	k := f.newTmp()
	f.emit("%s := %s", k, f.genExpr(args[0], nil).code)
	hit := f.callEntry(n, e, recv, []parser.Node{&exprNode{Node: args[0], e: expr{code: "(*" + k + ")", typ: kt}}}, nil)
	res := f.newTmp()
	if miss == "false" {
		f.emit("%s := Boolean(false)", res)
		f.emit("if %s != nil {\n\t%s = %s\n}", k, res, f.coerce(n, hit, f.cls("Boolean")))
		return expr{code: res, typ: f.cls("Boolean")}, true
	}
	t := optOf(hit.typ)
	f.emit("var %s %s", res, f.c.goType(t))
	f.emit("if %s != nil {\n\t%s = %s\n}", k, res, f.coerce(n, hit, t))
	return expr{code: res, typ: t}, true
}

// gemCallForm is a gem's call a typed call cannot make as written (decision 330): a nil key into a nil-free Hash, or an
// untyped first argument to a method with by-class twins (kramdown's `result.sub!(indent_re, ”)`, a Regexp or a
// String at run time), which the dispatcher picks.
func (f *fctx) gemCallForm(n parser.Node, e *entry, recv expr, args []parser.Node, block parser.Node) (expr, bool) {
	if r, ok := f.nilKeyLookup(n, e, recv, args, block); ok {
		return r, true
	}
	if f.untypedTwinArg(e, recv.typ, args, block) {
		return f.genDynCall(n, expr{code: f.coerce(n, recv, TAny{}), typ: TAny{}}, e.M.Name, args), true
	}
	return expr{}, false
}
