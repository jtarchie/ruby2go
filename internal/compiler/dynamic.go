package compiler

import (
	"fmt"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/danielgatis/go-ruby-prism/parser"
)

// Dynamic dispatch on untyped values, generated from the closed world:
// no reflection. For every method name called on an untyped value there is
// a `rbDynName` dispatcher, and a `DynName(args ...any) any` wrapper on
// every concrete class whose public, non-generic, block-less method has
// that name (`_DynName` when the method is private: only send and
// receiver-less calls reach it). Wrappers check arity, convert arguments to
// the declared types and box the result, reusing the typed call path.

// emitDynamic emits wrappers and dispatchers for every name asked for;
// generating a wrapper may ask for more (a default argument calling
// something dynamically), so this runs to a fixed point.
// emitDynamic emits the dispatchers that became needed (dynNeeded) and reports whether there were any: with the computed-send switches in play (dynAll) nearly every method is noted, and a program reaches few of them. A wrapper body notes what it calls, and a name noted that late is needed by what noted it.
func (c *Compiler) emitDynamic(reached, selected func(string) bool) bool {
	if c.dynLazy == nil {
		c.noteDyn("<=>") // rbCmp falls back to DynOp_cmp, and which instantiations see untyped values is unknown here
		if c.dynAll {
			for _, name := range c.allMethodNames() {
				c.noteDyn(name)
				c.noteRespond(name)
			}
		}
		c.dynLazy = map[string]bool{}
		for _, n := range slices.Concat(c.dynNames, c.respondNames) {
			c.dynLazy[n] = true
		}
	}
	c.dynEvery = c.dynEvery || c.dynAll && (reached("rbSendByName") || reached("rbRespondsByName") || selected("_Call"))
	var emitted []string
	for progress := true; progress; {
		progress = false
		for i := 0; i < len(c.dynNames); i++ { //nolint:intrange // emitDynName appends what a wrapper body notes, so len is re-read
			if n := c.dynNames[i]; !c.dynOut[n] && c.dynNeeded(n, reached, selected) {
				c.dynOut[n] = true
				c.emitDynName(n)
				emitted, progress = append(emitted, n), true
			}
		}
		for i := 0; i < len(c.respondNames); i++ { //nolint:intrange // same
			if n := c.respondNames[i]; !c.respondOut[n] && c.dynNeeded(n, reached, selected) {
				c.respondOut[n] = true
				c.emitRespond(n)
				emitted, progress = append(emitted, n), true
			}
		}
	}
	for _, name := range emitted {
		c.emitMarkers("_Private"+goMethodName(name), c.privateIn(name))
	}
	return len(emitted) > 0
}

// dynNeeded: name was noted by an emitted body, or kept code calls its dispatcher, asks respond_to? of it, or asserts its wrapper (rbCmp's DynOp_cmp, the json generator's DynToJson).
func (c *Compiler) dynNeeded(name string, reached, selected func(string) bool) bool {
	if c.dynEvery || !c.dynLazy[name] {
		return true
	}
	gn := goMethodName(name)
	return reached("rbDyn"+gn) || reached("rbResponds"+gn) || reached("rbHas_Private"+gn) || selected("Dyn"+gn) || selected("_Dyn"+gn)
}

// dynWrapped is one Dyn wrapper emitted on a class; own when a user file defines the method.
type dynWrapped struct {
	name, goName string
	own          bool
}

// callByName: what minitest calls by name, tests and the predicates and operators of
// assert_predicate/assert_operator. Only these, because naming a common method
// (to_s, name) here would keep its Dyn wrapper on every class (decision 49).
func callByName(name string) bool {
	_, op := opNames[name]
	return strings.HasPrefix(name, "test_") || strings.HasSuffix(name, "?") || op
}

// emitCallTables gives each user-defined class _Call(name, args...): send
// over the methods user code defined on it, own or inherited. It is the
// narrow alternative to rbSendByName, whose switch over every method name
// keeps the whole prelude in a build; minitest calls tests through it
// (decision 79). Only rbMtCall asks for _Call, so other programs prune it.
func (c *Compiler) emitCallTables() {
	for _, cls := range c.classList {
		if cls.File == nil || cls.File.prelude || !cls.isStruct() || cls.metaOf != nil || len(cls.TypeParams) > 0 {
			continue
		}
		var cases []dynWrapped
		for _, w := range c.dynWrapped[cls] {
			if w.own && callByName(w.name) {
				cases = append(cases, w)
			}
		}
		c.w("func (self %s) _Call(name string, args ...any) (any, bool) {\n", c.recvType(cls))
		switch len(cases) {
		case 0:
		case 1: // gocritic rejects a one-case switch
			c.w("\tif name == %q {\n\t\treturn self.%s(args...), true\n\t}\n", cases[0].name, cases[0].goName)
		default: // Go compiles a string switch to a search, not a chain of compares
			c.w("\tswitch name {\n")
			for _, w := range cases {
				c.w("\tcase %q:\n\t\treturn self.%s(args...), true\n", w.name, w.goName)
			}
			c.w("\t}\n")
		}
		c.w("\treturn nil, false\n}\n\n")
	}
}

// dynEntry is the method a dynamic call of name reaches on cls, if it can
// be called without a block and without type arguments, and whether it is
// private (the dispatchers call method_missing and respond_to_missing?
// whatever their visibility).
func (c *Compiler) dynEntry(cls *Class, name string) (*entry, bool) {
	if cls.IsModule || cls.universal || name == "initialize" {
		return nil, false
	}
	e := cls.lookup(name)
	if e == nil || e.M.generic() || e.M.Block != nil {
		return nil, false
	}
	if len(cls.TypeParams) > 0 && growsTypeParams(cls, e) {
		return nil, false
	}
	return e, e.M.Private && !rubyPrivate[name] && name != "method_missing"
}

// privateIn lists the concrete classes on which name is a private method.
func (c *Compiler) privateIn(name string) []*Class {
	var out []*Class
	for _, cls := range c.classList {
		if cls.IsModule || cls.universal {
			continue
		}
		if e := cls.lookup(name); e != nil && e.M.Private {
			out = append(out, cls)
		}
	}
	return out
}

// growsTypeParams reports whether a generic class's method mentions its
// type parameters other than bare (E, E?) or as the class itself
// (Array[E] on Array). Wrapping such a method instantiates some other
// generic at a type built from E (Hash[E, Integer], Array[[E, Integer]]),
// whose own methods and wrappers can instantiate a bigger one still: a Go
// instantiation cycle.
func growsTypeParams(cls *Class, e *entry) bool {
	own := func(t TClass) bool {
		if t.C != cls || len(t.Args) != len(cls.TypeParams) {
			return false
		}
		for i, a := range t.Args {
			if v, ok := a.(TVar); !ok || v.Name != cls.TypeParams[i] {
				return false
			}
		}
		return true
	}
	var grows func(t Type) bool
	grows = func(t Type) bool {
		switch t := t.(type) {
		case TOpt:
			return grows(t.Elem)
		case TTuple:
			return mentionsVar(t)
		case TClass:
			return !own(t) && mentionsVar(t)
		case TAny, TFunc, TNil, TVar, TVoid: // no class of its own whose wrappers could grow
		}
		return false
	}
	ts := make([]Type, 0, 1+len(e.M.Params))
	ts = append(ts, e.M.Ret)
	for _, p := range e.M.Params {
		ts = append(ts, p.Type)
	}
	for _, t := range ts {
		if grows(subst(t, e.Env)) {
			return true
		}
	}
	return false
}

func mentionsVar(t Type) bool {
	var vars []string
	freeVars(t, &vars)
	return len(vars) > 0
}

// dispatchers are package-level, so a capital or digit that decision 3 merges (`foo_bar`/`fooBar`) collides across classes too
func (c *Compiler) dynGoName(name string) string {
	gn := goMethodName(name)
	if p, ok := c.dynGo[gn]; ok && p != name {
		c.errorf(nil, nil, "`%s` and `%s` both become Go %s, so dynamic calls cannot tell them apart; rename one", p, name, gn)
	}
	c.dynGo[gn] = name
	return gn
}

// emitDynName emits rbDynName(how, recv, args...): the public wrapper, the
// private one unless the call had a receiver (how is rbCall), then
// method_missing, then NoMethodError. Wrappers whose bodies are the same
// for many classes (an inherited prelude method: `self.ToS()`) become one
// arm of the dispatcher, chosen by class ID, instead of a copy per class.
func (c *Compiler) emitDynName(name string) {
	gn := c.dynGoName(name)
	hidden := len(c.privateIn(name)) > 0
	own, shared := c.dynWrappers(name)
	c.w("func rbDyn%s(how int, recv any, args ...any) any {\n", gn)
	c.w("\tif r, ok := recv.(interface{ Dyn%s(...any) any }); ok {\n\t\treturn r.Dyn%s(args...)\n\t}\n", gn, gn)
	c.emitDynArms(shared, false)
	if v, ok := nilConversions[name]; ok {
		c.w("\tif recv == nil {\n\t\trbArity(len(args), 0, 0)\n\t\treturn %s\n\t}\n", v)
	}
	if hidden {
		c.w("\tif r, ok := recv.(interface{ _Dyn%s(...any) any }); ok && how != rbCall {\n\t\treturn r._Dyn%s(args...)\n\t}\n", gn, gn)
	}
	c.emitDynArms(shared, true)
	// every object has Kernel#===, so method_missing is never reached for it
	if name != "method_missing" && name != "===" {
		delete(c.dynLazy, "method_missing") // the fallback below needs its wrappers
		c.w("\tif r, ok := recv.(interface{ DynMethodMissing(...any) any }); ok {\n")
		c.w("\t\treturn r.DynMethodMissing(append([]any{Symbol(%q)}, args...)...)\n\t}\n", name)
	}
	if hidden {
		c.w("\tif rbHas_Private%s(recv) {\n\t\tpanic(rbPrivateMethod(%q, recv))\n\t}\n", gn, name)
	}
	if name == "===" {
		// Kernel#===: == unless its class defines one.
		// ponytail: a class object held untyped gets ==, not Module#=== (is_a?);
		// needs a generated per-metaclass instance check.
		c.w("\trbArity(len(args), 1, 1)\n\treturn Boolean(rbEq[any](recv, args[0]))\n}\n\n")
	} else {
		c.w("\tpanic(rbNoMethod(%q, recv, how == rbVCall))\n}\n\n", name)
	}
	for _, w := range own {
		c.emitDynWrapper(w.cls, w.e, name, w.private, w.body)
	}
}

// dynWrapper is one class's wrapper for a name, before emission.
type dynWrapper struct {
	cls     *Class
	e       *entry
	private bool
	body    string
}

// dynArm is a wrapper body shared by the classes in ids: self is recv
// asserted to iface, or recv itself when iface is empty.
type dynArm struct {
	ids     []string
	iface   string
	body    string
	private bool
	members []dynWrapper
}

// dynPinned names wrappers Go code asserts for by method (rbCmp's DynOp_cmp,
// the dispatchers' fallbacks): each class keeps its own. (rbToJson asserts
// DynToJson only after ToJson(...any), the signature of every prelude to_json.)
var dynPinned = map[string]bool{"<=>": true, "method_missing": true, "respond_to_missing?": true}

// dynWrappers splits name's wrappers into those emitted per class and the
// arms shared by two or more classes. Growing with definitions, not with
// classes × names, is what keeps a program of many classes buildable
// (decision 85).
func (c *Compiler) dynWrappers(name string) ([]dynWrapper, []*dynArm) {
	var own []dynWrapper
	arms := map[string]*dynArm{}
	var keys []string
	for _, cls := range c.classList {
		e, private := c.dynEntry(cls, name)
		if e == nil {
			continue
		}
		w := dynWrapper{cls: cls, e: e, private: private, body: c.dynWrapperBodyOrWarn(cls, e, name)}
		if w.body == "" {
			continue
		}
		w.body = c.freeCall(e, w.body)
		iface, body, ok := c.dynShareable(w, name)
		if !ok {
			own = append(own, w)
			continue
		}
		key := fmt.Sprintf("%t\x00%s\x00%s", private, iface, body)
		a := arms[key]
		if a == nil {
			a = &dynArm{iface: iface, body: body, private: private}
			arms[key] = a
			keys = append(keys, key)
		}
		a.ids = append(a.ids, strconv.Itoa(c.classID(cls)))
		a.members = append(a.members, w)
	}
	var shared []*dynArm
	for _, k := range keys {
		if a := arms[k]; len(a.members) > 1 {
			shared = append(shared, a)
		} else {
			own = append(own, a.members[0])
		}
	}
	return own, shared
}

// freeCall rewrites a wrapper's forwarder call of a Kernel method (`self.Sleep(`) into the free func over
// `any` (`Kernel_Sleep[any](self, `): one instantiation for every class, and no selector or interface
// literal naming the method, which would keep the forwarder on every reached class (decision 121). Only a
// universal owner: a named constraint's free func (`Module_Name[ModuleI]`) is not the method a metaclass's
// synth `Name()` overrides in Go, and a module constraint (`Comparable_Self[Self]`) needs the concrete class.
func (c *Compiler) freeCall(e *entry, body string) string {
	call := "self." + e.M.GoName + "("
	if !strings.Contains(body, call) || c.isDirectMethod(e.M) || !e.Owner.universal {
		return body
	}
	body = strings.ReplaceAll(body, call, freeFuncName(e.M)+"[any](self, ")
	return strings.ReplaceAll(body, "(self, )", "(self)")
}

// dynGenerated: e comes from the prelude, or is a metaclass's generated
// new/name/to_s/inspect; a user's own method keeps its wrapper for _Call.
func (c *Compiler) dynGenerated(e *entry) bool {
	return e.M.Kind == kindSynth || e.M.File != nil && e.M.File.prelude
}

var selfIdent = regexp.MustCompile(`\bself\b`)

// dynShareable is w's body with nothing particular to its class, and the
// interface its self must satisfy: a struct class (it has a _ClassID) reaching
// a prelude or generated method whose body only calls that method on self, or passes self
// to a universal owner's free func, whose Self is any.
func (c *Compiler) dynShareable(w dynWrapper, name string) (iface, body string, ok bool) {
	cls, e := w.cls, w.e
	if !cls.isStruct() || len(cls.TypeParams) > 0 || dynPinned[name] || !c.dynGenerated(e) {
		return "", "", false
	}
	body = w.body
	if e.Owner.universal {
		for _, t := range []string{c.goType(TClass{C: cls}), c.recvType(cls)} {
			body = strings.ReplaceAll(body, "["+t+"](self", "[any](self")
		}
	}
	rest := strings.ReplaceAll(body, "[any](self", "")
	call := "self." + e.M.GoName + "("
	if !w.private && strings.Contains(rest, call) {
		env := composeEnv(e.Env, nil)
		env["Self"] = c.selfTypeFor(*e, cls)
		ps, ret := c.sig(e.M, env)
		iface = fmt.Sprintf("interface{ %s(%s) %s }", e.M.GoName, ps, ret)
		rest = strings.ReplaceAll(rest, call, "")
	}
	if selfIdent.MatchString(rest) || strings.Contains(body+iface, cls.Name) {
		return "", "", false
	}
	return iface, body, true
}

// emitDynArms emits the shared public (or private) arms: a switch on the
// receiver's class ID, so only the classes that reach the method take it.
func (c *Compiler) emitDynArms(arms []*dynArm, private bool) {
	var cases []*dynArm
	for _, a := range arms {
		if a.private == private {
			cases = append(cases, a)
		}
	}
	if len(cases) == 0 {
		return
	}
	cond := ""
	if private {
		cond = " && how != rbCall"
	}
	c.w("\tif id, ok := recv.(interface{ _ClassID() int }); ok%s {\n", cond)
	arm := func(a *dynArm) {
		switch {
		case a.iface != "":
			c.w("\tif self, ok := recv.(%s); ok {\n%s}\n", a.iface, a.body)
		case selfIdent.MatchString(a.body):
			c.w("\tself := recv\n%s", a.body)
		default:
			c.w("%s", a.body)
		}
	}
	c.w("\tswitch id._ClassID() {\n")
	for _, a := range cases {
		c.w("\tcase %s:\n", strings.Join(a.ids, ", "))
		arm(a)
	}
	c.w("\tdefault:\n\t}\n\t}\n") // default: gocritic rejects a one-case switch; Go compiles the cases to a binary search
}

// dynWrapperBodyOrWarn is cls's wrapper body for e, or "" with a warning
// when its types cannot cross `any`: calling it dynamically raises
// NoMethodError.
func (c *Compiler) dynWrapperBodyOrWarn(cls *Class, e *entry, name string) (body string) {
	defer func() {
		if r := recover(); r != nil {
			ce, ok := r.(compileError)
			if !ok {
				panic(r)
			}
			c.Warnings = append(c.Warnings, fmt.Sprintf("%s#%s cannot be called dynamically: %s", cls.RubyName, name, ce.msg))
			body = ""
		}
	}()
	return c.dynWrapperBody(cls, e)
}

// emitDynWrapper emits cls's DynName (_DynName for a private method).
func (c *Compiler) emitDynWrapper(cls *Class, e *entry, name string, private bool, body string) {
	prefix := "Dyn"
	if private {
		prefix = "_Dyn"
	}
	c.w("func (self %s) %s%s(args ...any) any {\n%s}\n\n", c.recvType(cls), prefix, goMethodName(name), body)
	c.dynWrapped[cls] = append(c.dynWrapped[cls], dynWrapped{name: name, goName: prefix + goMethodName(name), own: !e.M.File.prelude})
}

func (c *Compiler) dynWrapperBody(cls *Class, e *entry) string {
	m := e.M
	file := cls.File
	if m.File != nil {
		file = m.File // default arguments are read from the method's source
	}
	f := c.newFctx(file, cls, nil)
	f.lex = m.Scope
	f.locals = map[localKey]*localInfo{}
	f.scope = &scope{vars: map[string]*local{}}
	f.pass = 2
	f.indent = 1
	f.implicitCall = true // the method may be private: method_missing, or one send reaches
	f.plainCalls = cls.isStruct()
	recv := expr{code: "self", typ: cls.instance()}
	if cls.metaOf != nil || cls.isStruct() {
		recv.typ = TClass{C: cls}
	}
	env := composeEnv(e.Env, nil)
	for _, p := range cls.TypeParams {
		env[p] = TVar{Name: p}
	}
	env["Self"] = recv.typ
	if m.hasKeywords() {
		// ponytail: an untyped call passes keywords as a trailing Hash; unpacking it per keyword (and running defaults) would make these callable
		f.emit("panic(NewArgumentError(Ref(String(%q))))", "rb2go: "+m.String()+" takes keyword arguments, which a call on an untyped value cannot pass (decision 23)")
		return f.buf.String()
	}
	var req, opt int
	var rest *Param
	for i := range m.Params {
		p := &m.Params[i]
		switch {
		case p.Rest:
			rest = p
		case p.Default != nil:
			opt++
		default:
			req++
		}
	}
	maxArgs := req + opt
	if rest != nil {
		maxArgs = -1
	}
	f.emit("rbArity(len(args), %d, %d)", req, maxArgs)
	if isNumeric(recv.typ) && rest == nil && opt == 0 {
		c.dynNumericMix(f, e, env)
	}
	if m.Name == "<=>" && len(m.Params) == 1 && req == 1 {
		if t, ok := subst(m.Params[0].Type, env).(TClass); ok { // MRI's <=> answers nil for an incomparable argument
			f.emit("if _, ok := rbConv[%s](args[0]); !ok {", c.goType(t))
			f.emit("\treturn nil")
			f.emit("}")
		}
	}
	call := func(k int, withRest bool) {
		nodes := make([]parser.Node, 0, k+1)
		for i := range k {
			t := subst(m.Params[i].Type, env)
			nodes = append(nodes, &exprNode{e: expr{code: c.dynArg(t, i), typ: t}})
		}
		if withRest {
			t := subst(rest.Type, env)
			code := fmt.Sprintf("rbRest[%s](args, %d, %q)...", c.goType(t), k, t.String())
			if isAny(t) {
				code = fmt.Sprintf("args[%d:]...", k) // untyped takes anything, nil included
			}
			nodes = append(nodes, &exprNode{e: expr{code: code, typ: t}})
		}
		res := f.callEntry(&parser.NilNode{}, e, recv, nodes, nil)
		if isVoid(res.typ) {
			if res.code != "" {
				f.emit("%s", res.code)
			}
			if !res.noreturn {
				f.emit("return nil")
			}
			return
		}
		f.emit("return %s", f.coerce(&parser.NilNode{}, res, TAny{}))
	}
	if opt == 0 {
		call(req, rest != nil)
		return f.buf.String()
	}
	f.emit("switch len(args) {")
	for k := req; k < req+opt; k++ {
		f.emit("case %d:", k)
		f.indent++
		call(k, false)
		f.indent--
	}
	f.emit("default:")
	f.indent++
	call(req+opt, rest != nil)
	f.indent--
	f.emit("}")
	return f.buf.String()
}

// dynNumericMix emits an Integer or Float wrapper's answer to an argument
// of the other numeric class, as numericMix compiles typed calls:
// Comparable's methods run on rbNum, and an Integer's own operator is
// redone by its Float, whose wrappers widen Integer arguments (rbAs).
func (c *Compiler) dynNumericMix(f *fctx, e *entry, env map[string]Type) {
	m := e.M
	if len(m.Params) == 0 {
		return
	}
	for _, p := range m.Params {
		if !isNumeric(subst(p.Type, env)) {
			return
		}
	}
	var ret string
	switch {
	case m.Owner == c.classes["Comparable"]:
		args := make([]string, len(m.Params))
		for i := range args {
			args[i] = c.dynArg(TAny{}, i)
		}
		ret = f.rbNumCall(nil, m, "self", args).code
	case isClass(env["Self"], "Integer"):
		if fe, private := c.dynEntry(c.classes["Float"], m.Name); fe == nil || private {
			return
		}
		ret = "Float(self).Dyn" + goMethodName(m.Name) + "(args...)"
	default:
		return
	}
	f.emit("if rbNumMixed(self, args) {")
	f.emit("\treturn %s", ret)
	f.emit("}")
}

// dynArg converts argument i to type t. Past the first it is read through
// rbArg: gosec's G602 tracks the argument count of each call site, and a
// dispatcher branch for more arguments than one site passes (Logger's
// one-argument `write` against REXML's two) reads past it, though rbArity
// has already ruled that branch out.
func (c *Compiler) dynArg(t Type, i int) string {
	arg := fmt.Sprintf("args[%d]", i)
	if i > 0 {
		arg = fmt.Sprintf("rbArg(args, %d)", i)
	}
	switch t := t.(type) {
	case TAny:
		return arg
	case TOpt:
		return fmt.Sprintf("OptOf[%s](%s, %q)", c.goType(t.Elem), arg, t.Elem.String())
	case TClass, TFunc, TNil, TTuple, TVar, TVoid: // converted and checked by rbAs below
	}
	return fmt.Sprintf("rbAs[%s](%s, %q)", c.goType(t), arg, t.String())
}

// emitMarkers emits rbHas<marker>(recv), true for the classes given: a
// marker method on each non-generic primitive class, and one switch on the
// class ID for struct classes, which may be many, and generic classes, where
// Go would compile a marker per instantiation (decision 87).
func (c *Compiler) emitMarkers(marker string, classes []*Class) {
	if c.markers[marker] {
		return
	}
	c.markers[marker] = true
	var ids []string
	for _, cls := range classes {
		if cls.isStruct() || len(cls.TypeParams) > 0 {
			ids = append(ids, strconv.Itoa(c.classID(cls)))
		} else {
			c.w("func (self %s) %s() {}\n\n", c.recvType(cls), marker)
		}
	}
	c.w("func rbHas%s(recv any) bool {\n", marker)
	c.w("\tif _, ok := recv.(interface{ %s() }); ok {\n\t\treturn true\n\t}\n", marker)
	if len(ids) > 0 {
		c.w("\tif id, ok := recv.(interface{ _ClassID() int }); ok {\n")
		c.w("\t\tswitch id._ClassID() {\n\t\tcase %s:\n\t\t\treturn true\n\t\tdefault:\n\t\t}\n\t}\n", strings.Join(ids, ", "))
	}
	c.w("\treturn false\n}\n\n")
}

// emitRespond emits rbRespondsName(recv, priv), backed by a
// marker method on every class that has the public method (with or without
// a block) and, when private methods count, the _PrivateName markers.
func (c *Compiler) emitRespond(name string) {
	gn := c.dynGoName(name)
	var public []*Class
	for _, cls := range c.classList {
		if cls.IsModule || cls.universal || rubyPrivate[name] {
			continue
		}
		if e := cls.lookup(name); e != nil && !e.M.Private {
			public = append(public, cls)
		}
	}
	c.emitMarkers("_Responds"+gn, public)
	c.w("func rbResponds%s(recv any, priv bool) Boolean {\n", gn)
	c.w("\tif rbHas_Responds%s(recv) {\n\t\treturn true\n\t}\n", gn)
	if len(c.privateIn(name)) > 0 {
		c.w("\tif priv && rbHas_Private%s(recv) {\n\t\treturn true\n\t}\n", gn)
	}
	delete(c.dynLazy, "respond_to_missing?") // the fallback below needs its wrappers
	c.w("\tif r, ok := recv.(interface{ DynRespondToMissingQ(...any) any }); ok {\n")
	c.w("\t\treturn Boolean(rbTruthy(r.DynRespondToMissingQ(Symbol(%q), Boolean(priv))))\n\t}\n", name)
	c.w("\treturn false\n}\n\n")
}

// allMethodNames lists every method name a computed send may name.
func (c *Compiler) allMethodNames() []string {
	seen := map[string]bool{}
	for _, cls := range c.classList {
		if cls.IsModule || cls.universal {
			continue
		}
		for _, e := range cls.methodSet() {
			if e.M.Name != "initialize" && !strings.HasPrefix(e.M.Name, "__") {
				seen[e.M.Name] = true
			}
		}
	}
	out := make([]string, 0, len(seen))
	for n := range seen {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}

// emitNameSwitches backs send and respond_to? with computed names.
func (c *Compiler) emitNameSwitches() {
	c.w("func rbSendByName(recv any, name string, how int, args ...any) any {\n\tswitch name {\n")
	for _, name := range c.dynNames {
		c.w("\tcase %q:\n\t\treturn rbDyn%s(how, recv, args...)\n", name, goMethodName(name))
	}
	c.w("\t}\n\tif r, ok := recv.(interface{ DynMethodMissing(...any) any }); ok {\n")
	c.w("\t\treturn r.DynMethodMissing(append([]any{Symbol(name)}, args...)...)\n\t}\n")
	c.w("\tpanic(rbNoMethod(name, recv, false))\n}\n\n")
	c.w("func rbRespondsByName(recv any, name string, priv bool) Boolean {\n\tswitch name {\n")
	for _, name := range c.respondNames {
		c.w("\tcase %q:\n\t\treturn rbResponds%s(recv, priv)\n", name, goMethodName(name))
	}
	c.w("\t}\n\tif r, ok := recv.(interface{ DynRespondToMissingQ(...any) any }); ok {\n")
	c.w("\t\treturn Boolean(rbTruthy(r.DynRespondToMissingQ(Symbol(name), Boolean(priv))))\n\t}\n")
	c.w("\treturn false\n}\n\n")
}

// emitClassOf emits rbClassOf, `.class` for a value known only at run
// time: a type switch over the @go_type classes, and over each struct
// hierarchy, whose instances answer _ClassOf. nil is Go nil, not an
// instance of a class, so its class object is a bare Class named NilClass.
func (c *Compiler) emitClassOf() {
	if !c.classOf {
		return
	}
	c.w("type rbNilClass struct{ Class }\n\n")
	for _, m := range []string{"Name", "ToS", "Inspect"} {
		c.w("func (*rbNilClass) %s() String { return \"NilClass\" }\n\n", m)
	}
	c.w("var rbNilClassObj = &rbNilClass{}\n\n")
	c.w("func rbClassOf(a any) ClassI {\n\tswitch v := a.(type) {\n\tcase nil:\n\t\treturn rbNilClassObj\n")
	for _, cls := range c.classList {
		switch {
		case cls.universal || cls.IsModule || cls.meta == nil:
		case cls.RubyName == "Boolean":
			c.w("\tcase Boolean:\n\t\treturn rbBoolClass(v)\n")
		case cls.GoType != "" && len(cls.TypeParams) > 0:
			c.w("\tcase %s_Any:\n\t\treturn %s\n", cls.Name, classVar(cls))
		case cls.GoType != "":
			c.w("\tcase %s:\n\t\treturn %s\n", c.goType(TClass{C: cls}), classVar(cls))
		case cls.root() == cls && len(cls.TypeParams) == 0:
			// a metaclass's interface lacks the _ClassOf its struct inherits
			c.w("\tcase %s:\n\t\treturn v._ClassOf().(ClassI)\n", c.goType(TClass{C: cls}))
		}
	}
	// ponytail: generic struct classes fall to NoMethodError; give them an _Any interface to switch on
	c.w("\t}\n\tpanic(rbNoMethod(\"class\", a, false))\n}\n\n")
}
