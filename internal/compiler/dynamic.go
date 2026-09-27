package compiler

import (
	"fmt"
	"sort"
	"strings"

	"github.com/danielgatis/go-ruby-prism/parser"
)

// Dynamic dispatch on untyped values, generated from the closed world:
// no reflection. For every method name called on an untyped value there is
// a `rbDynName` dispatcher, and a `DynName(args ...any) any` wrapper on
// every concrete class whose public, non-generic, block-less method has
// that name. Wrappers check arity, convert arguments to the declared types
// and box the result, reusing the typed call path.

// emitDynamic emits wrappers and dispatchers for every name asked for;
// generating a wrapper may ask for more (a default argument calling
// something dynamically), so this runs to a fixed point.
func (c *Compiler) emitDynamic() {
	if len(c.dynNames) == 0 && len(c.respondNames) == 0 && !c.dynAll {
		return
	}
	if c.dynAll {
		for _, name := range c.allMethodNames() {
			c.noteDyn(name)
			c.noteRespond(name)
		}
	}
	// a wrapper may ask for more names, so walk the growing list
	done := 0
	for done < len(c.dynNames) {
		c.emitDynName(c.dynNames[done])
		done++
	}
	for _, name := range c.respondNames {
		c.emitRespond(name)
	}
	if c.dynAll {
		c.emitNameSwitches()
	}
}

// dynEntry is the method a dynamic call of name reaches on cls, if it can
// be called without a block and without type arguments.
func (c *Compiler) dynEntry(cls *Class, name string) *entry {
	if cls.IsModule || cls.universal || name == "initialize" {
		return nil
	}
	e := cls.lookup(name)
	if e == nil || e.M.generic() || e.M.Block != nil {
		return nil
	}
	if e.M.Private && !rubyPrivate[name] {
		return nil
	}
	if len(cls.TypeParams) > 0 && growsTypeParams(cls, e) {
		return nil
	}
	return e
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

func (c *Compiler) emitDynName(name string) {
	gn := goMethodName(name)
	c.w("func rbDyn%s(vcall bool, recv any, args ...any) any {\n", gn)
	c.w("\tif r, ok := recv.(interface{ Dyn%s(...any) any }); ok {\n\t\treturn r.Dyn%s(args...)\n\t}\n", gn, gn)
	switch name {
	case "===":
		// every object has Kernel#===: == unless its class defines one.
		// ponytail: a class object held untyped gets ==, not Module#=== (is_a?);
		// needs a generated per-metaclass instance check.
		c.w("\trbArity(len(args), 1, 1)\n\treturn Boolean(rbEq[any](recv, args[0]))\n}\n\n")
	case "method_missing":
		c.w("\tpanic(rbNoMethod(%q, recv, vcall))\n}\n\n", name)
	default:
		c.w("\tif r, ok := recv.(interface{ DynMethodMissing(...any) any }); ok {\n")
		c.w("\t\treturn r.DynMethodMissing(append([]any{Symbol(%q)}, args...)...)\n\t}\n", name)
		c.w("\tpanic(rbNoMethod(%q, recv, vcall))\n}\n\n", name)
	}
	for _, cls := range c.classList {
		if e := c.dynEntry(cls, name); e != nil {
			c.emitDynWrapper(cls, e, name)
		}
	}
}

// emitDynWrapper emits cls's DynName. A method that cannot be wrapped (its
// types cannot cross `any`) is left out with a warning: calling it
// dynamically raises NoMethodError.
func (c *Compiler) emitDynWrapper(cls *Class, e *entry, name string) {
	var body string
	func() {
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
		body = c.dynWrapperBody(cls, e)
	}()
	if body != "" {
		c.w("func (self %s) Dyn%s(args ...any) any {\n%s}\n\n", c.recvType(cls), goMethodName(name), body)
	}
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
	f.implicitCall = true // method_missing and respond_to_missing? may be private
	recv := expr{code: "self", typ: cls.instance()}
	if cls.metaOf != nil || cls.isStruct() {
		recv.typ = TClass{C: cls}
	}
	env := composeEnv(e.Env, nil)
	for _, p := range cls.TypeParams {
		env[p] = TVar{Name: p}
	}
	env["Self"] = recv.typ
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
	call := func(k int, withRest bool) {
		nodes := make([]parser.Node, 0, k+1)
		for i := range k {
			t := subst(m.Params[i].Type, env)
			nodes = append(nodes, &exprNode{e: expr{code: c.dynArg(t, i), typ: t}})
		}
		if withRest {
			t := subst(rest.Type, env)
			code := fmt.Sprintf("rbRest[%s](args, %d, %q)...", c.goType(t), k, t.String())
			nodes = append(nodes, &exprNode{e: expr{code: code, typ: t}})
		}
		res := f.callEntry(&parser.NilNode{}, e, recv, nodes, nil)
		if isVoid(res.typ) {
			if res.code != "" {
				f.emit("%s", res.code)
			}
			f.emit("return nil")
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

// dynArg converts argument i to type t.
func (c *Compiler) dynArg(t Type, i int) string {
	switch t := t.(type) {
	case TAny:
		return fmt.Sprintf("args[%d]", i)
	case TOpt:
		if isAny(t.Elem) {
			return fmt.Sprintf("rbOptArg[any](args, %d, %q)", i, "untyped")
		}
		return fmt.Sprintf("rbOptArg[%s](args, %d, %q)", c.goType(t.Elem), i, t.Elem.String())
	}
	return fmt.Sprintf("rbArg[%s](args, %d, %q)", c.goType(t), i, t.String())
}

// emitRespond emits rbRespondsByName, backed by a marker method on every
// class that has the public method (with or without a block).
func (c *Compiler) emitRespond(name string) {
	gn := goMethodName(name)
	for _, cls := range c.classList {
		if cls.IsModule || cls.universal || rubyPrivate[name] {
			continue
		}
		if e := cls.lookup(name); e != nil && !e.M.Private {
			c.w("func (self %s) _Responds%s() {}\n\n", c.recvType(cls), gn)
		}
	}
	c.w("func rbResponds%s(recv any) Boolean {\n", gn)
	c.w("\tif _, ok := recv.(interface{ _Responds%s() }); ok {\n\t\treturn true\n\t}\n", gn)
	c.w("\tif r, ok := recv.(interface{ DynRespondToMissingQ(...any) any }); ok {\n")
	c.w("\t\treturn Boolean(rbTruthy(r.DynRespondToMissingQ(Symbol(%q), Boolean(false))))\n\t}\n", name)
	c.w("\treturn false\n}\n\n")
}

// allMethodNames lists every public method name a computed send may name.
func (c *Compiler) allMethodNames() []string {
	seen := map[string]bool{}
	for _, cls := range c.classList {
		if cls.IsModule || cls.universal {
			continue
		}
		for _, e := range cls.methodSet() {
			if !e.M.Private && e.M.Name != "initialize" && !strings.HasPrefix(e.M.Name, "__") {
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
	c.w("func rbSendByName(recv any, name string, args ...any) any {\n\tswitch name {\n")
	for _, name := range c.dynNames {
		c.w("\tcase %q:\n\t\treturn rbDyn%s(false, recv, args...)\n", name, goMethodName(name))
	}
	c.w("\t}\n\tif r, ok := recv.(interface{ DynMethodMissing(...any) any }); ok {\n")
	c.w("\t\treturn r.DynMethodMissing(append([]any{Symbol(name)}, args...)...)\n\t}\n")
	c.w("\tpanic(rbNoMethod(name, recv, false))\n}\n\n")
	c.w("func rbRespondsByName(recv any, name string) Boolean {\n\tswitch name {\n")
	for _, name := range c.respondNames {
		c.w("\tcase %q:\n\t\treturn rbResponds%s(recv)\n", name, goMethodName(name))
	}
	c.w("\t}\n\tif r, ok := recv.(interface{ DynRespondToMissingQ(...any) any }); ok {\n")
	c.w("\t\treturn Boolean(rbTruthy(r.DynRespondToMissingQ(Symbol(name), Boolean(false))))\n\t}\n")
	c.w("\treturn false\n}\n\n")
}
