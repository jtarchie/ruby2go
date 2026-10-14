package compiler

import (
	"bytes"
	"fmt"
	"hash/fnv"
	"maps"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/danielgatis/go-ruby-prism/parser"

	"github.com/jtarchie/ruby2go/internal/rbs"
)

// ---- Go type rendering

// goType renders a resolved Ruby type as a Go type.
func (c *Compiler) goType(t Type) string {
	switch t := t.(type) {
	case TClass:
		cls := t.C
		switch {
		case cls.universal:
			return "any"
		case cls.IsModule:
			return "any"
		case cls.GoType != "":
			name := cls.Name
			if len(t.Args) > 0 {
				name += "[" + c.goTypes(t.Args) + "]"
				c.noteArgBoxes(t.Args)
				c.noteMarshal(t)
			}
			if cls.mutable() {
				return "*" + name
			}
			return name
		default:
			return cls.Name + "I"
		}
	case TOpt:
		s := "*" + c.goType(t.Elem)
		var vars []string
		if freeVars(t, &vars); len(vars) == 0 && s != "*" {
			if _, fn := t.Elem.(TFunc); !fn {
				c.boxes[s] = isOpt(t.Elem)
			}
		}
		return s
	case TTuple:
		c.tupleN[len(t.Elems)] = true
		name := fmt.Sprintf("Tuple%d[%s]", len(t.Elems), c.goTypes(t.Elems))
		c.noteMarshal(t)
		return name
	case TVar:
		return t.Name
	case TFunc:
		params := make([]string, 0, len(t.Params)+1)
		if t.Self != nil {
			params = append(params, c.goType(t.Self))
		}
		for _, p := range t.Params {
			params = append(params, c.goType(p))
		}
		s := "func(" + strings.Join(params, ", ") + ")"
		if t.Rest {
			last := len(t.Params) - 1
			rest := make([]string, 0, len(t.Params))
			if t.Self != nil {
				rest = append(rest, c.goType(t.Self))
			}
			for _, p := range t.Params[:last] {
				rest = append(rest, c.goType(p))
			}
			s = "func(" + strings.Join(rest, ", ")
			if len(rest) > 0 {
				s += ", "
			}
			s += "..." + c.goType(t.Params[last]) + ")"
		}
		if !isVoid(t.Ret) {
			s += " " + c.goType(t.Ret)
		}
		if t.Proc {
			var vars []string
			if freeVars(t, &vars); len(vars) == 0 {
				c.procTypes["*"+s] = true
				c.procFuncs["*"+s] = t
			}
			return "*" + s
		}
		return s
	case TAny, TNil, TUnion: // a union is an any whose classes the compiler knows (decision 150)
		return "any"
	case TVoid:
		return ""
	}
	panic(fmt.Sprintf("goType: %T", t))
}

func (c *Compiler) goTypes(ts []Type) string {
	parts := make([]string, len(ts))
	for i, t := range ts {
		parts[i] = c.goType(t)
	}
	return strings.Join(parts, ", ")
}

// recvType is the Go receiver type for methods on a concrete class.
func (c *Compiler) recvType(cls *Class) string {
	name := cls.Name
	if len(cls.TypeParams) > 0 {
		name += "[" + strings.Join(cls.TypeParams, ", ") + "]"
	}
	if cls.isStruct() || cls.mutable() {
		return "*" + name
	}
	return name
}

// selfTypeFor is the Ruby type `self` denotes in signatures of entry e as
// seen from class cls: the entry class for struct hierarchies, cls itself
// for primitives.
func (c *Compiler) selfTypeFor(e entry, cls *Class) Type {
	if cls.isStruct() {
		return TClass{C: e.Entry}
	}
	return cls.instance()
}

// blockGoType renders a block signature as a Go func type.
func (c *Compiler) blockGoType(b *BlockSig, env map[string]Type) string {
	return c.goType(TFunc{Params: substAll(b.Params, env), Ret: subst(b.Ret, env), Self: subst(b.Self, env), Rest: b.Rest})
}

// iterGoType renders an iterator method's return type.
func (c *Compiler) iterGoType(b *BlockSig, env map[string]Type) string {
	ps := substAll(b.Params, env)
	switch len(ps) {
	case 0:
		return "func(func() bool)"
	case 1:
		return "iter.Seq[" + c.goType(ps[0]) + "]"
	case 2:
		return "iter.Seq2[" + c.goTypes(ps) + "]"
	}
	panic(compileError{msg: "iterator blocks must yield at most 2 values"})
}

// sig renders the Go parameter list and result of method m under env.
func (c *Compiler) sig(m *Method, env map[string]Type) (params string, ret string) {
	var ps []string
	var restParam string // held back and appended last: Go requires the variadic param to be final, but Ruby puts a block after *rest
	if m.calleeDefaults {
		ps = append(ps, "rbArgc int")
	}
	if m.kwMask() {
		ps = append(ps, "rbKw int")
	}
	for i, p := range m.Params {
		name := goParamName(p.Name, i)
		if p.Rest {
			restParam = "rest_ ..." + c.goType(subst(p.Type, env))
			continue
		}
		ps = append(ps, name+" "+c.goType(subst(p.Type, env)))
	}
	if m.Block != nil && !m.Iterator {
		ps = append(ps, "blk "+c.blockGoType(m.Block, env))
	}
	if restParam != "" {
		ps = append(ps, restParam)
	}
	if m.Iterator {
		return strings.Join(ps, ", "), c.iterGoType(m.Block, env)
	}
	if m.inferRet && m.Ret == nil {
		c.pendingSigs++ // still being inferred (its body is generating): what was read from it is provisional
	}
	r := subst(m.Ret, env)
	if isVoid(r) {
		return strings.Join(ps, ", "), ""
	}
	return strings.Join(ps, ", "), c.goType(r)
}

// argNames lists the Go argument names used to forward a call.
func (c *Compiler) argNames(m *Method) string {
	var as []string
	var restArg string // held back and appended last, matching sig()'s reordering
	if m.calleeDefaults {
		as = append(as, "rbArgc")
	}
	if m.kwMask() {
		as = append(as, "rbKw")
	}
	for i, p := range m.Params {
		name := goParamName(p.Name, i)
		if p.Rest {
			restArg = "rest_..."
			continue
		}
		as = append(as, name)
	}
	if m.Block != nil && !m.Iterator {
		as = append(as, "blk")
	}
	if restArg != "" {
		as = append(as, restArg)
	}
	return strings.Join(as, ", ")
}

// freeFuncName is the Go name of a method emitted as a free function.
func freeFuncName(m *Method) string {
	if m.freeName != "" {
		return m.freeName
	}
	return m.Owner.Name + "_" + m.GoName
}

// separateFreeFuncs renames a method's free func whose name is a class's Go type (`Sandbox#landlock` and
// `Sandbox::Landlock` are both Sandbox_Landlock).
func (c *Compiler) separateFreeFuncs() {
	types := map[string]bool{}
	for _, cls := range c.classList {
		types[cls.Name] = true
	}
	for _, cls := range c.classList {
		for _, m := range cls.MethodList {
			if name := cls.Name + "_" + m.GoName; types[name] {
				m.freeName = name + "_fn"
			}
		}
	}
}

// staticCallCode renders a non-virtual call of m on recv (private calls,
// super). Attr accessors have no free func: they are plain methods on the
// owner's struct, reached through the `_Owner()` every struct constraint has.
func staticCallCode(m *Method, targs, recv, args string) string {
	if m.Kind == kindAttrReader || m.Kind == kindAttrWriter {
		return recv + "._" + m.Owner.Name + "()." + m.GoName + "(" + args + ")"
	}
	return freeFuncName(m) + targs + "(" + recv + comma(args) + ")"
}

// isDirectMethod reports whether m is emitted as a plain Go method on its
// owner (non-generic primitive classes' own non-generic methods, attr
// accessors). A generic primitive's (Array, Hash) methods are free funcs
// with a forwarder, since Go compiles every method of a generic type for
// every instantiation, used or not, but a free func only when called
// (decision 86).
func (c *Compiler) isDirectMethod(m *Method) bool {
	if m.Owner == nil {
		return false
	}
	if m.Kind == kindAttrReader || m.Kind == kindAttrWriter || m.Kind == kindSynth {
		return true
	}
	return m.Owner.GoType != "" && !m.generic() && len(m.Owner.TypeParams) == 0
}

// constraint renders the Self constraint for free funcs of owner.
func (c *Compiler) constraint(owner *Class) string {
	switch {
	case owner.universal:
		return "any"
	case owner.IsModule:
		return owner.Name + "_Self[Self" + comma(strings.Join(owner.TypeParams, ", ")) + "]"
	default:
		return owner.Name + "I"
	}
}

// comma prefixes a non-empty list fragment with ", ".
func comma(s string) string {
	if s == "" {
		return ""
	}
	return ", " + s
}

// plainSelf reports a free func that takes self as its owner's interface (any for Kernel), not a type parameter
// Self: a class body, or a universal one, whose signature names no Self. It compiles once rather than once per GC
// shape plus a dictionary wrapper per class, and, not generic, it can be a function value (decision 210).
func (c *Compiler) plainSelf(m *Method) bool {
	o := m.Owner
	if o == nil || o.GoType != "" || len(o.TypeParams) > 0 || m.generic() || m.SelfType != nil || o.IsModule && !o.universal {
		return false
	}
	if m.Block != nil && (mentionsVar(m.Block.Ret) || mentionsVar(m.Block.Self) || slices.ContainsFunc(m.Block.Params, mentionsVar)) {
		return false
	}
	return !mentionsVar(m.Ret) && !slices.ContainsFunc(m.Params, func(p Param) bool { return mentionsVar(p.Type) })
}

// plainSelfType is the Go type of a plainSelf func's self.
func (c *Compiler) plainSelfType(owner *Class) string {
	if owner.universal {
		return "any"
	}
	return owner.Name + "I"
}

// typeParamDecl renders the type parameter list of a free func.
func (c *Compiler) typeParamDecl(m *Method) string {
	var tps []string
	if m.Owner.GoType == "" && !c.plainSelf(m) {
		tps = append(tps, "Self "+c.constraint(m.Owner))
	}
	rest := make([]string, 0, len(m.Owner.TypeParams)+len(m.TypeParams))
	rest = append(rest, m.Owner.TypeParams...)
	rest = append(rest, m.TypeParams...)
	if len(rest) > 0 {
		tps = append(tps, strings.Join(rest, ", ")+" comparable")
	}
	if len(tps) == 0 {
		return ""
	}
	return "[" + strings.Join(tps, ", ") + "]"
}

// ---- emission

func (c *Compiler) w(format string, args ...any) {
	fmt.Fprintf(&c.out, format, args...)
}

func (c *Compiler) lineDirective(f *File, line int) {
	c.w("//line %s:%d\n", f.Name, line)
}

func (c *Compiler) emitProgram() {
	c.w("// Code generated by rb2go from %s. DO NOT EDIT.\n\npackage main\n\n", c.mainFile.Name)
	// Order matters for `//line` readability only; Go doesn't care. The
	// translator goes first, so no Ruby file's directive covers it.
	c.w("%s\n\n", strings.TrimSpace(rxTranslateGo()))
	for _, v := range c.verbatim {
		c.lineDirective(v.file, v.line)
		c.w("%s\n\n", strings.TrimSpace(v.code))
	}
	if slices.ContainsFunc(c.classList, func(cls *Class) bool { return cls.hashBase && cls.Super.RubyName == "Hash" }) {
		c.w("type %s[V comparable] = Hash[any, V]\n\n", superField(c.classes["Hash"]))
	}
	based := map[*Class]bool{}
	for _, cls := range c.classList {
		if sup := cls.Super; cls.hashBase && sup.RubyName != "Hash" && !based[sup] {
			based[sup] = true
			if tps := strings.Join(sup.TypeParams, ", "); tps != "" {
				c.w("type %s[%s comparable] = %s[%s]\n\n", superField(sup), tps, sup.Name, tps)
			} else {
				c.w("type %s = %s\n\n", superField(sup), sup.Name)
			}
		}
	}
	classes := c.sortedClasses()
	for _, cls := range classes {
		c.emitClassType(cls)
	}
	for _, cls := range classes {
		for _, m := range cls.MethodList {
			c.emitMethod(m)
		}
	}
	for _, m := range c.topDefList {
		c.emitTopDef(m)
	}
	for _, k := range c.constList {
		c.emitConst(k)
	}

	c.emitMain()
	c.noteUserToJson()
}

// emitNext is one batch of the rest of the program, given what the pruned program so far reaches: forwarders of reached classes and dispatchers of reached names, until a round adds nothing; then the tables every body has contributed to by now, once; then nil. The tables open paths to more code, so rounds go on after them; a late body that adds to a table's inputs (a tuple type, a literal) is errPruneIncomplete, and the caller recompiles with everything eager.
func (c *Compiler) emitNext(reached, selected func(string) bool) ([]byte, error) {
	c.out.Reset()
	for _, cls := range c.sortedClasses() {
		if !c.fwdOut[cls] && (c.dynEvery || reached(cls.Name)) {
			c.fwdOut[cls] = true
			c.emitForwarders(cls)
		}
	}
	c.emitDynamic(reached, selected)
	c.emitSends(reached)
	if c.tablesOut {
		c.emitUnions() // a late body's new union helper is a function like any other, not a table input
	}
	marshal := reached("rbMDumpGen") || reached("rbMLoadGen")
	// Marshal's cases were rendered with the tables: a later value type, or a class they skipped and something now reaches, needs them redone
	stale := marshal && len(c.marshalSeen) != c.marshalAt || slices.ContainsFunc(c.marshalSkipped, reached)
	if c.tablesOut && (c.tableInputs() != c.tablesAt || stale) {
		return nil, errPruneIncomplete
	}
	if c.out.Len() == 0 {
		switch {
		case !c.tablesOut:
			c.tablesOut = true
			c.prepareMarshal() // first: its types note boxes and tuples the tables list
			c.tablesAt = c.tableInputs()
			c.emitTables()
		case marshal && !c.marshalOut: // Marshal may be reached only through the tables (minitest's _Call)
			if c.marshalErr != nil {
				panic(*c.marshalErr)
			}
			c.marshalOut = true
			c.w("%s", c.marshalCode)
		case c.marshalOut && !c.marshalLate: // last: its cases are only the classes reached by then
			c.marshalLate = true
			c.emitMarshalClasses(reached)
		default:
			return nil, nil
		}
	}
	return []byte(c.out.String()), nil
}

// tableInputs fingerprints what emitTables reads; the maps only grow, so sizes tell.
func (c *Compiler) tableInputs() [12]int {
	b2i := func(b bool) int {
		if b {
			return 1
		}
		return 0
	}
	return [12]int{len(c.tupleN), len(c.argBoxes), len(c.boxes), len(c.procTypes), len(c.pkgVars), len(c.strLits), b2i(c.classOf), b2i(c.dynEvery), b2i(c.callable != nil), b2i(c.dynEach), b2i(c.needRxAlt), b2i(c.needRequireTable)}
}

// emitTables: what every emitted body has contributed to.
func (c *Compiler) emitTables() {
	if c.dynEvery {
		c.emitNameSwitches()
	}
	if c.dynEvery || c.callable != nil {
		c.emitCallTables()
	}
	c.emitClassOf()
	c.emitModIsA()
	c.emitFrozenFlag()
	c.emitDynEach()
	c.emitTuples()
	c.emitBoxes()
	c.emitUnions()
	c.emitClassMeta()
	// last: every body, main included, has registered its literals by now
	if c.needRequireTable {
		c.emitRequireTable()
	}
	if c.needRxAlt { // a pattern RE2 rejects, or one built at run time from what may hold one (decision 242)
		c.w("%s\n\nvar _ = rxAltReady\n\n", strings.TrimSpace(rxAltGo()))
	}
	for _, r := range c.pkgVars {
		if c.needRxAlt { // a pattern variable initializes after the engine registers, not before (Go's dependency order)
			r = strings.Replace(r, " = rbRegexpNew(", " = rbRegexpNewAfter(rxAltReady, ", 1)
		}
		c.w("%s\n\n", r)
	}
	// The linker shares one copy of each literal's bytes, so this table's
	// entries alias every literal in the program (rbStrFrozen).
	c.w("var rbStringLits = [...]string{\n")
	for _, s := range slices.Sorted(maps.Keys(c.strLits)) {
		c.w("\t%s,\n", strconv.Quote(s))
	}
	c.w("}\n")
}

// noteUserToJson: the json generator calls to_json(state) on what it
// renders, which reaches a user to_json declared other than
// (*untyped) -> String only through its Dyn wrapper (docs/design.md decision 25).
func (c *Compiler) noteUserToJson() {
	for _, cls := range c.classList {
		m := cls.Methods["to_json"]
		if m == nil || m.File.prelude {
			continue
		}
		if len(m.Params) != 1 || !m.Params[0].Rest || !isAny(m.Params[0].Type) || !isClass(m.Ret, "String") {
			c.noteDyn("to_json")
			return
		}
	}
}

func (c *Compiler) emitClassType(cls *Class) {
	switch {
	case cls.universal:
		if !cls.IsModule {
			c.w("type %s struct{ _ byte }\n\n", cls.Name) // not zero-size: Go may give every new zero-size value one address, and Object.new must be unique
		}
	case cls.IsModule:
		c.emitModuleInterface(cls)
	case cls.GoType != "":
		tp := ""
		if len(cls.TypeParams) > 0 {
			tp = "[" + strings.Join(cls.TypeParams, ", ") + " comparable]"
		}
		goType := cls.GoType
		if cls.hashBase && len(cls.IvarList) > 0 { // a container subclass's own ivars sit beside the embedded container
			var fields strings.Builder
			for _, iv := range cls.IvarList {
				fmt.Fprintf(&fields, "; %s %s", goFieldName(iv.Name), c.goType(iv.Type))
			}
			if cls.ivarBits {
				fmt.Fprintf(&fields, "; %s uint64", ivarSetField)
			}
			goType = strings.TrimSuffix(goType, " }") + fields.String() + " }"
		}
		c.w("type %s%s %s\n\n", cls.Name, tp, goType)
		inst, decl := cls.Name, ""
		if tps := strings.Join(cls.TypeParams, ", "); tps != "" {
			inst, decl = cls.Name+"["+tps+"]", "["+tps+" comparable]"
		}
		if cls.hashBase && len(cls.IvarList) > 0 {
			c.w("func (self *%s) _%s() *%s { return self }\n\n", inst, cls.Name, inst)
		}
		if cls.hashBase {
			switch cls.Super.RubyName {
			case "Hash":
				c.w("func New%s%s() *%s { return &%s{%s: *NewHash[any, %s]()} }\n\n", cls.Name, decl, inst, inst, superField(cls.Super), strings.Join(cls.TypeParams, ", "))
			case "Array", "Set", "Queue":
				c.w("func New%s%s() *%s { return &%s{%s: *New%s%s()} }\n\n", cls.Name, decl, inst, inst, superField(cls.Super), cls.Super.Name, anyArgs(len(cls.Super.TypeParams)))
			default:
				if sup := cls.Super; sup.hashBase && sup.mutable() && len(sup.TypeParams) == 0 && decl == "" { // Concurrent::Set < CRubySet < ::Set
					c.w("func New%s() *%s { return &%s{%s: *New%s()} }\n\n", cls.Name, inst, inst, superField(sup), sup.Name)
					break
				}
				// a value base starts zero; super in initialize sets it
				c.w("func New%s() *%s { return &%s{} }\n\n", cls.Name, inst, inst)
			}
		}
	default:
		c.emitStructClass(cls)
	}
}

// publicEntries lists the entries that make up a class's Go interface:
// public, non-generic methods.
func (c *Compiler) publicEntries(cls *Class) []entry {
	var out []entry
	for _, e := range cls.methodSet() {
		if staticPrivate(e.M) || e.M.generic() || e.M.Name == "initialize" {
			continue
		}
		out = append(out, e)
	}
	return out
}

// userTypeLine maps a user class's struct or module's constraint to its `class`/`module` line, so a Go error in it points at Ruby and `rb2go web` can tell the type, and what it names, from the prelude's.
func (c *Compiler) userTypeLine(cls *Class) {
	if cls.File != nil && !cls.File.prelude && cls.metaOf == nil {
		c.lineDirective(cls.File, cls.Line)
	}
}

func (c *Compiler) emitModuleInterface(mod *Class) {
	tps := ""
	if len(mod.TypeParams) > 0 {
		tps = ", " + strings.Join(mod.TypeParams, ", ") + " comparable"
	}
	c.userTypeLine(mod)
	if len(mod.IvarList) > 0 { // its state, a field of each includer (decision 147)
		c.w("type %s struct {\n", ivarsType(mod))
		for _, iv := range mod.IvarList {
			c.w("\t%s %s\n", goFieldName(iv.Name), c.goType(iv.Type))
		}
		if mod.ivarBits {
			c.w("\t%s uint64\n", ivarSetField)
		}
		c.w("}\n\n")
	}
	c.w("type %s_Self[Self any%s] interface {\n", mod.Name, tps)
	if len(mod.IvarList) > 0 {
		c.w("\t_%s() *%s\n", mod.Name, ivarsType(mod))
	}
	for _, im := range ivarModules(mod) { // its included modules' state too (Obligation writes Dereferenceable's @value)
		if im != mod {
			c.w("\t_%s() *%s\n", im.Name, ivarsType(im))
		}
	}
	// Constraint = what the module's bodies call on self (docs/design.md).
	called := c.selfCalls(mod)
	for _, e := range c.publicEntries(mod) {
		if !called[e.M.Name] || !c.renderingConstraint && c.dispatchable(e.Owner, e.M) {
			continue // a dispatched name is called through rbSend_, not the constraint (dispatch.go)
		}
		env := composeEnv(e.Env, nil)
		env["Self"] = TVar{Name: "Self"}
		ps, ret := c.sig(e.M, env)
		c.w("\t%s(%s) %s\n", e.M.GoName, ps, ret)
	}
	c.emitBridgeSigs(mod, TVar{Name: "Self"})
	// self is some includer, whose to_s/inspect/... may override Object's
	if mod.lookup("method_missing") == nil {
		for _, e := range c.publicEntries(c.classes["Object"]) {
			if called[e.M.Name] && mod.lookup(e.M.Name) == nil && c.topDefs[e.M.Name] == nil && !c.dispatchable(e.Owner, e.M) {
				ps, ret := c.sig(e.M, map[string]Type{"Self": TVar{Name: "Self"}})
				c.w("\t%s(%s) %s\n", e.M.GoName, ps, ret)
			}
		}
	}
	if called["class"] {
		c.w("\t_ClassObj() %s\n", c.goType(TClass{C: c.classes["Class"]}))
	}
	c.w("}\n\n")
}

// bridgeName is the includer method a module method's `super` calls.
func bridgeName(m *Method) string { return "_Super_" + m.Owner.Name + "_" + m.GoName }

// superBridges lists, as seen from cls, the module methods in its
// ancestors (itself, for a module) whose `super` target differs per
// includer. The module's constraint requires a bridge for each, and every
// concrete includer implements it by calling whatever follows the module
// method in its own ancestors.
func (c *Compiler) superBridges(cls *Class) []entry {
	var out []entry
	seen := map[*Method]bool{}
	for _, anc := range append([]*Class{cls}, cls.allAncestors()...) {
		if !anc.IsModule {
			continue
		}
		for _, m := range anc.MethodList {
			if !m.superBridge || seen[m] {
				continue
			}
			seen[m] = true
			for _, e := range cls.defsOf(m.Name) {
				if e.M == m {
					out = append(out, e)
					break
				}
			}
		}
	}
	return out
}

// emitBridgeSigs lists cls's super bridges in its interface.
func (c *Compiler) emitBridgeSigs(cls *Class, self Type) {
	for _, e := range c.superBridges(cls) {
		env := composeEnv(e.Env, nil)
		env["Self"] = self
		if self == nil {
			env["Self"] = c.selfTypeFor(e, cls)
		}
		ps, ret := c.sig(e.M, env)
		c.w("\t%s(%s) %s\n", bridgeName(e.M), ps, ret)
	}
}

// emitBridges implements cls's super bridges: each calls the definition
// after the module method in cls's ancestors, or fails as MRI does.
func (c *Compiler) emitBridges(cls *Class, recv string) {
	for _, e := range c.superBridges(cls) {
		m := e.M
		env := composeEnv(e.Env, nil)
		env["Self"] = c.selfTypeFor(e, cls)
		ps, ret := c.sig(m, env)
		defs := cls.defsOf(m.Name)
		i := slices.IndexFunc(defs, func(d entry) bool { return d.M == m })
		var body string
		switch {
		case i+1 < len(defs):
			t := defs[i+1]
			if c.isDirectMethod(t.M) {
				c.errorf(m.File, m.Node, "super into a primitive class method is not supported")
			}
			body = fmt.Sprintf("%s%s(self%s)", freeFuncName(t.M), c.forwardTypeArgs(t, cls), comma(c.argNames(m)))
			if ret != "" {
				body = "return " + body
			}
		case m.Name == "initialize": // BasicObject's does nothing
		case hookNames[m.Name]: // Class's and Module's hooks do nothing (decision 232): a mixed-in `inherited` calling super (decision 241)
			if ret != "" {
				body = "return *new(" + ret + ")"
			}
		default:
			body = fmt.Sprintf("panic(NewNoMethodError(Ref(String(\"super: no superclass method '%s' for \" + rbDescribe(self)))))", m.Name)
		}
		c.w("func (self %s) %s(%s) %s { %s }\n", recv, bridgeName(m), ps, ret, body)
	}
}

// hookNames are Class's and Module's callbacks, whose own definitions do nothing (decision 232).
var hookNames = map[string]bool{"inherited": true, "included": true, "extended": true, "prepended": true, "method_added": true}

var goSelfCall = regexp.MustCompile(`\bself\.([A-Za-z_]\w*)\(`)

// selfCalls lists the method names a module's bodies invoke on self,
// including `self.X(` inside %x{} leaves.
func (c *Compiler) selfCalls(mod *Class) map[string]bool {
	if mod.selfCallCache != nil {
		return mod.selfCallCache
	}
	out := map[string]bool{}
	byGo := map[string]string{}
	for _, e := range mod.methodSet() {
		byGo[e.M.GoName] = e.M.Name
	}
	var walk func(n parser.Node)
	walk = func(n parser.Node) {
		if n == nil {
			return
		}
		switch n := n.(type) {
		case *parser.CallNode:
			if n.Receiver == nil {
				out[n.Name] = true
			} else if _, ok := n.Receiver.(*parser.SelfNode); ok {
				out[n.Name] = true
			} else if n.Name == "class" {
				out[n.Name] = true // the receiver may be a copy of self
			}
		case *parser.XStringNode:
			for _, m := range goSelfCall.FindAllStringSubmatch(n.Unescaped.Value, -1) {
				if rb, ok := byGo[m[1]]; ok {
					out[rb] = true
				}
			}
		case *parser.InterpolatedStringNode:
			// interpolation calls to_s on every part
			out["to_s"] = true
		}
		for _, ch := range n.CompactChildNodes() {
			walk(ch)
		}
	}
	core := mod.File != nil && mod.File.prelude
	var gem []*Method
	for _, m := range mod.MethodList {
		if m.Node == nil {
			continue
		}
		if core && c.gemFile(m.File) {
			gem = append(gem, m)
			continue
		}
		walk(m.Node.Body)
		if m.Node.Parameters != nil {
			walk(m.Node.Parameters) // defaults run in the callee
		}
	}
	for _, inc := range mod.Includes {
		for k := range c.selfCalls(inc.Mod) {
			out[k] = true
		}
	}
	// A gem's method added to a core module (`module Enumerable; def exclude?(x) = !include?(x)`) calls self
	// dynamically: the core includers (Range, Hash) are Go types whose include? Enumerable's slot can't adapt to.
	own := out
	out = map[string]bool{}
	for _, m := range gem {
		walk(m.Node.Body)
	}
	mod.gemSelfCalls = map[string]bool{}
	for k := range out {
		if !own[k] {
			mod.gemSelfCalls[k] = true
		}
	}
	mod.selfCallCache = own
	return own
}

func (c *Compiler) emitStructClass(cls *Class) {
	if len(cls.Subclasses) > 0 && !cls.universal {
		c.w("type %s = %s\n\n", superField(cls), cls.Name)
	}
	c.userTypeLine(cls)
	c.w("type %s struct {\n", cls.Name)
	switch {
	case cls.Super != nil && !cls.Super.universal:
		c.w("\t%s\n", superField(cls.Super))
	case c.objFreeze() && len(cls.TypeParams) == 0:
		c.w("\t%s bool\n", frozenField) // Kernel#freeze's flag, on the root: every subclass embeds it (decision 221)
	case len(cls.IvarList) == 0:
		// zero-size allocations may share an address, merging distinct instances' identity
		c.w("\t_ byte\n")
	}
	for _, iv := range cls.IvarList {
		c.w("\t%s %s\n", goFieldName(iv.Name), c.goType(iv.Type))
	}
	if cls.isPerRun {
		c.w("\trbCls_ *%s\n", cls.meta.Name) // the class object whose new made it (decision 259)
	}
	if cls.ivarBits {
		c.w("\t%s uint64\n", ivarSetField)
	}
	for _, mod := range ownIvarModules(cls) {
		c.w("\t%s %s\n", ivarsField(mod), ivarsType(mod))
	}
	c.w("}\n\n")
	// interface
	c.w("type %sI interface {\n", cls.Name)
	for _, k := range cls.structChain() {
		c.w("\t_%s() *%s\n", k.Name, k.Name)
	}
	for _, mod := range ivarModules(cls) {
		c.w("\t_%s() *%s\n", mod.Name, ivarsType(mod))
	}
	c.emitEntrySigs(cls)
	c.emitBridgeSigs(cls, nil)
	if c.includerCalls(cls, "class") {
		// forwardTypeArgs binds Self to this interface, so it needs the module's `self.class` hook
		c.w("\t_ClassObj() %s\n", c.goType(TClass{C: c.classes["Class"]}))
	}
	if cls.meta != nil || cls.metaOf != nil {
		// a metaclass inherits _ClassOf from Class/Module; listing it lets a
		// singleton(C) value pass where Class or Module is expected
		c.w("\t_ClassOf() %s\n", c.goType(TClass{C: cls.root().meta}))
	}
	isModule := cls.isSubclassOf(c.classes["Module"])
	if isModule {
		c.w("\t_Kind() string\n")
	}
	c.w("}\n\n")
	c.w("func (self *%s) _%s() *%s { return self }\n\n", cls.Name, cls.Name, cls.Name)
	for _, mod := range ownIvarModules(cls) {
		c.w("func (self *%s) _%s() *%s { return &self.%s }\n\n", cls.Name, mod.Name, ivarsType(mod), ivarsField(mod))
	}
	switch {
	case cls.isPerRun:
		c.w("func (self *%s) _ClassOf() %s {\n\tif self.rbCls_ != nil {\n\t\treturn self.rbCls_\n\t}\n\treturn %s\n}\n\n", cls.Name, c.goType(TClass{C: cls.root().meta}), classVar(cls))
	case cls.meta != nil:
		c.w("func (self *%s) _ClassOf() %s { return %s }\n\n", cls.Name, c.goType(TClass{C: cls.root().meta}), classVar(cls))
	}
	if isModule {
		c.emitConstTable(cls)
	}
	if cls.metaOf != nil {
		// a metaclass has exactly one instance: the class object
		c.w("var %s = &%s{}\n\n", classVar(cls.metaOf), cls.Name)
		if len(ownIvarModules(cls)) > 0 || len(cls.IvarList) > 0 { // class-level ivars by name: an `inherited`'s `child.instance_eval { @env = ... }` on an untyped child (decisions 241, 259)
			c.emitIvarList(cls)
		}
		return
	}
	c.emitIvarList(cls)
	c.emitCopy(cls)
	c.emitDup(cls)
	// constructor
	if cls.File != nil && !cls.File.prelude && cls.Line > 0 {
		c.lineDirective(cls.File, cls.Line) // a Go error here names the class, not a line counted on from the last directive
	}
	init := cls.lookup("initialize")
	if init != nil {
		env := composeEnv(init.Env, nil)
		env["Self"] = TClass{C: cls}
		ps, _ := c.sig(init.M, env)
		c.labels["New"+cls.Name] = "Class#new"
		c.w("func New%s(%s) *%s {\n\tself := &%s{}\n\t%s(self, %s)\n\treturn self\n}\n\n",
			cls.Name, ps, cls.Name, cls.Name, freeFuncName(init.M), c.argNames(init.M))
		if cls.isPerRun { // its class object's new: initialize already sees self.class (decision 259)
			c.w("func New%sIn(k *%s, %s) *%s {\n\tself := &%s{rbCls_: k}\n\t%s(self, %s)\n\treturn self\n}\n\n",
				cls.Name, cls.meta.Name, ps, cls.Name, cls.Name, freeFuncName(init.M), c.argNames(init.M))
		}
	} else {
		c.w("func New%s() *%s { return &%s{} }\n\n", cls.Name, cls.Name, cls.Name)
	}
}

// emitEntrySigs writes cls's interface lines for its public methods, but those dispatched by type switch.
func (c *Compiler) emitEntrySigs(cls *Class) {
	for _, e := range c.publicEntries(cls) {
		if cls.metaOf != nil && e.M.Name == "new" {
			// Subclasses may take different initialize arguments, so `new`
			// is not part of the shared class-object interface; calls
			// through `singleton(C)` assert for it instead.
			continue
		}
		if c.dispatchableAll(cls, e.M.Name) {
			continue // dispatched by type switch, not through this interface (dispatch.go)
		}
		env := composeEnv(e.Env, nil)
		env["Self"] = c.selfTypeFor(e, cls)
		ps, ret := c.sig(e.M, env)
		c.w("\t%s(%s) %s\n", e.M.GoName, ps, ret)
		if e.M.seqAdapter {
			name, ps, ret := c.seqAdapterSig(e.M, env)
			c.w("\t%s(%s) %s\n", name, ps, ret)
		}
		for _, s := range e.M.shadowed {
			ps, ret := c.sig(s.e.M, c.slotEnv(s))
			c.w("\t%s(%s) %s\n", s.e.M.GoName, ps, ret)
		}
		if e.M.GoName == "cmpNil" && len(e.M.Params) == 1 { // emitCmpAdapter's Comparable slot
			c.w("\tOp_cmp(%s) Integer\n", c.goType(c.cmpIncluder(cls)))
		}
	}
}

// superField names the field a subclass embeds cls through: an alias, since
// an embedded field takes its type's name and a method of that Go name
// (`Calc#calc`) would clash with it. Lowercase with a trailing `_`, it is
// neither a method nor an ivar name (goLocalName).
func superField(cls *Class) string { return "super_" + cls.Name + "_" }

// emitIvarList feeds Kernel#inspect; a class adding no ivars inherits its parent's through embedding.
func (c *Compiler) emitIvarList(cls *Class) {
	if len(cls.IvarList) == 0 && len(ownIvarModules(cls)) == 0 {
		return
	}
	var ivs []string
	for _, iv := range c.ivarOrder(cls) {
		field := ivarPath("self", iv)
		val, opt, isNilCode := field, isAny(iv.Type), "false"
		switch t := iv.Type.(type) {
		case TOpt:
			val, opt = "Opt("+val+")", true
		case TFunc:
			isNilCode = field + " == nil"
		case TVar:
			isNilCode = "rbUnbox(any(" + field + ")) == nil"
		case TClass:
			if t.C.isStruct() || t.C.mutable() { // an interface or a pointer
				isNilCode = field + " == nil"
			}
		case TUnion: // an any, like untyped
			opt = true
		case TAny, TNil, TTuple, TVoid: // held by value: never a nil pointer of its own
		}
		if set := ivarIsSet("self", iv); set != "" { // the bit, not the value, says whether it was assigned
			ivs = append(ivs, fmt.Sprintf("{%q, %s, %s, !(%s)}", iv.Name, val, set, set))
			continue
		}
		ivs = append(ivs, fmt.Sprintf("{%q, %s, %t, %s}", iv.Name, val, opt, isNilCode))
	}
	c.w("func (self *%s) _Ivars() []rbIvar { return []rbIvar{%s} }\n\n", cls.Name, strings.Join(ivs, ", "))
	// Kernel#instance_variable_set: the closed world knows every ivar's type, so a write converts to it
	c.w("func (self *%s) _IvarSet(name string, v any) bool {\n\tswitch name {\n", cls.Name)
	for _, iv := range c.ivarOrder(cls) {
		field := "&" + ivarPath("self", iv)
		if o, ok := iv.Type.(TOpt); ok {
			c.w("\tcase %q:\n\t\trbIvarAssignOpt[%s](%s, v, %q)\n", iv.Name, c.goType(o.Elem), field, iv.Name)
		} else {
			c.w("\tcase %q:\n\t\trbIvarAssign(%s, v, %q)\n", iv.Name, field, iv.Name)
		}
		if s := ivarAssigned("self", iv); s != "" {
			c.w("\t\t%s\n", s)
		}
	}
	c.w("\tdefault:\n\t\treturn false\n\t}\n\treturn true\n}\n\n")
	c.emitIvarDel(cls)
}

// emitIvarDel feeds Kernel#remove_instance_variable: the field back to its zero value and its bit cleared.
func (c *Compiler) emitIvarDel(cls *Class) {
	var cases []string
	for _, iv := range c.ivarOrder(cls) {
		if field, mask := ivarBit("self", iv); field != "" {
			cases = append(cases, fmt.Sprintf("\tcase %q:\n\t\trbIvarClear(&%s)\n\t\t%s &^= %s\n", iv.Name, ivarPath("self", iv), field, mask))
		}
	}
	if len(cases) == 0 {
		return
	}
	c.w("func (self *%s) _IvarDel(name string) {\n\tswitch name {\n%s\t}\n}\n\n", cls.Name, strings.Join(cases, ""))
}

// emitCopy feeds Ractor's deep copy (decision 103): a fresh struct with every
// ivar copied through rbCopyAs, identity kept through seen. Pruned with
// rbCopyDeep when no Ractor is used.
func (c *Compiler) emitCopy(cls *Class) {
	c.w("func (self *%s) _Copy(seen map[any]any) any {\n\tif self == nil {\n\t\treturn self\n\t}\n\tdup := *self\n\tseen[self] = &dup\n", cls.Name)
	for _, iv := range c.ivarOrder(cls) {
		c.w("\t%s = rbCopyAs(%s, seen)\n", ivarPath("dup", iv), ivarPath("self", iv))
	}
	c.w("\treturn &dup\n}\n\n")
}

// emitDup is Object#dup for a struct class: a shallow copy, then the class's own initialize_copy (rbDup).
func (c *Compiler) emitDup(cls *Class) {
	c.w("func (self *%s) _Dup() any {\n\tdup := *self\n", cls.Name)
	if e := cls.lookup("initialize_dup"); e != nil && e.M.File != nil && !e.M.File.prelude {
		// ponytail: its super reaches Kernel's no-op, not the class's initialize_copy as MRI's Object#initialize_dup would
		c.w("\t%s%s(&dup, self)\n", freeFuncName(e.M), c.forwardTypeArgs(*e, cls))
	} else if e := cls.lookup("initialize_copy"); e != nil && e.M.File != nil && !e.M.File.prelude {
		c.w("\t%s%s(&dup, self)\n", freeFuncName(e.M), c.forwardTypeArgs(*e, cls)) // private: no forwarder
	}
	c.w("\treturn &dup\n}\n\n")
}

// ivarOrder approximates MRI's (first assignment) with initialize's write order, super splicing in the parent's.
func (c *Compiler) ivarOrder(cls *Class) []*Ivar {
	var out []*Ivar
	seen := map[string]bool{} // by name: a subclass's and an ancestor's @x are one Ruby ivar
	add := func(iv *Ivar) {
		if iv != nil && !seen[iv.Name] {
			seen[iv.Name] = true
			out = append(out, iv)
		}
	}
	var fromInit func(k *Class)
	fromInit = func(k *Class) {
		if k == nil || k.universal {
			return
		}
		e := k.lookup("initialize")
		if e == nil || e.M.Node == nil {
			return
		}
		var walk func(n parser.Node)
		walk = func(n parser.Node) {
			if n == nil {
				return
			}
			for _, ch := range n.CompactChildNodes() {
				walk(ch)
			}
			switch n := n.(type) {
			case *parser.InstanceVariableWriteNode:
				add(c.findIvar(cls, n.Name))
			case *parser.InstanceVariableOrWriteNode:
				add(c.findIvar(cls, n.Name))
			case *parser.SuperNode, *parser.ForwardingSuperNode:
				fromInit(e.Owner.Super)
			}
		}
		walk(e.M.Node.Body)
	}
	fromInit(cls)
	chain := cls.structChain()
	for i := len(chain) - 1; i >= 0; i-- {
		for _, iv := range chain[i].IvarList {
			add(iv)
		}
	}
	for _, mod := range ivarModules(cls) { // decision 147's, in each includer's struct
		for _, iv := range mod.IvarList {
			add(iv)
		}
	}
	return out
}

// forwarderEnv is the signature environment for a forwarder of cls.
func (c *Compiler) forwarderEnv(cls *Class, e entry) map[string]Type {
	env := composeEnv(e.Env, nil)
	env["Self"] = c.selfTypeFor(e, cls)
	cls.containerEnv(env)
	return env
}

// emitForwarders emits, for a concrete class, a Go method per inherited or
// included public non-generic method so the class satisfies its interface.
func (c *Compiler) emitForwarders(cls *Class) {
	if cls.IsModule || cls.universal {
		return
	}
	recv := c.recvType(cls)
	for _, e := range c.publicEntries(cls) {
		m := e.M
		if e.Owner == cls && c.isDirectMethod(m) {
			continue
		}
		if cls.hashBase && e.Owner != cls && slices.Contains(cls.Super.ancestors(), e.Owner) {
			continue // promoted from the embedded Hash; a typed call is intercepted in callCode
		}
		if m.Kind == kindAttrReader || m.Kind == kindAttrWriter {
			continue // promoted through embedding; never touches virtual self
		}
		if !c.wantsForwarder(cls, e) {
			continue
		}
		env := c.forwarderEnv(cls, e)
		ps, ret := c.sig(m, env)
		targs := c.forwardTypeArgs(e, cls)
		body := fmt.Sprintf("%s%s(self%s)", freeFuncName(m), targs, comma(c.argNames(m)))
		if ret == "" {
			c.w("func (self %s) %s(%s) { %s }\n", recv, m.GoName, ps, body)
		} else {
			c.w("func (self %s) %s(%s) %s { return %s }\n", recv, m.GoName, ps, ret, body)
		}
	}
	c.emitBridges(cls, recv)
	for _, e := range c.publicEntries(cls) {
		if !e.M.seqAdapter {
			continue
		}
		env := c.forwarderEnv(cls, e)
		name, ps, ret := c.seqAdapterSig(e.M, env)
		if c.slotSeqAdapter(cls, e, env, recv) {
			continue
		}
		seq := "rbSeq"
		if len(e.M.Block.Params) == 2 {
			seq = "rbSeq2"
		}
		if !isVoid(e.M.Block.Ret) && len(e.M.Block.Params) == 1 { // `each { |x| ... }` whose block value is ignored: drop it
			elem := c.goType(subst(e.M.Block.Params[0], env))
			args := c.seqArgs(e.M)
			wrapped := fmt.Sprintf("func(x %s) %s { blk(x); return *new(%s) }", elem, c.goType(subst(e.M.Block.Ret, env)), c.goType(subst(e.M.Block.Ret, env)))
			args = strings.Replace(args, "blk", wrapped, 1)
			if !strings.Contains(c.argNames(e.M), "blk") {
				args = strings.TrimPrefix(args+", "+wrapped, ", ")
			}
			c.w("func (self %s) %s(%s) %s {\n\treturn %s(func(blk func(%s)) { %s })\n}\n",
				recv, name, ps, ret, seq, elem, c.blkCall(cls, e, args))
			continue
		}
		c.w("func (self %s) %s(%s) %s {\n\treturn %s(func(blk %s) { %s })\n}\n",
			recv, name, ps, ret, seq, c.blockGoType(e.M.Block, env), c.blkCall(cls, e, c.seqArgs(e.M)))
	}
	for _, e := range c.publicEntries(cls) {
		for _, s := range e.M.shadowed {
			ps, ret := c.sig(s.e.M, c.slotEnv(s))
			c.w("func (self %s) %s(%s) %s {\n%s}\n", recv, s.e.M.GoName, ps, ret, c.adapterBody(cls, e, s))
		}
	}
	if c.includerCalls(cls, "class") {
		// a module's `self.class`; a class object's class is Class (a module's, Module)
		k := cls
		switch {
		case cls.metaOf != nil && cls.metaOf.IsModule:
			k = c.classes["Module"]
		case cls.metaOf != nil:
			k = c.classes["Class"]
		}
		c.w("func (self %s) _ClassObj() %s { return %s }\n", recv, c.goType(TClass{C: c.classes["Class"]}), classVar(k))
	}
	c.w("\n")
	c.emitEqAdapter(cls, recv)
	c.emitCmpAdapter(cls, recv)
}

// emitCmpAdapter: Comparable_Self and rbCmp need Op_cmp(T) Integer, so a nil <=> (Float's NaN) raises there, as MRI's rb_cmpint.
func (c *Compiler) emitCmpAdapter(cls *Class, recv string) {
	e := cls.lookup("<=>")
	if e == nil || e.M.GoName != "cmpNil" || len(e.M.Params) != 1 || !c.wantsForwarder(cls, *e) {
		return
	}
	arg := c.argNames(e.M)
	// Comparable's slot is `Op_cmp(Self)`; a wider parameter (Sorbet's `other: BasicObject`) still takes a Self
	c.w("func (self %s) Op_cmp(%s %s) Integer {\n\tif r := self.cmpNil(%s); r != nil {\n\t\treturn *r\n\t}\n\tpanic(rbCmpErr(self, %s))\n}\n\n",
		recv, arg, c.goType(c.cmpIncluder(cls)), arg, arg)
}

// cmpIncluder is the type the Op_cmp adapter takes for cls: the topmost superclass that has Comparable (whose Self
// binds there) or the adapter itself, so each subclass answers the one Op_cmp slot.
func (c *Compiler) cmpIncluder(cls *Class) Type {
	cmp := c.classes["Comparable"]
	k := cls
	for sup := k.Super; sup != nil && !sup.universal; sup = k.Super {
		if e := sup.lookup("<=>"); !slices.Contains(sup.ancestors(), cmp) && (e == nil || e.M.GoName != "cmpNil") {
			break
		}
		k = sup
	}
	if k.metaOf != nil || !k.isStruct() {
		return c.selfTypeFor(entry{Owner: k, Entry: k}, k)
	}
	return TClass{C: k}
}

// emitEqAdapter lets rbEq (include?, Array#==, == on untyped or T?) reach
// a == typed on its argument, Op_eq(VecI), which Go cannot also declare as
// Op_eq(any): _EqAny asserts the class and answers false for anything else.
func (c *Compiler) emitEqAdapter(cls *Class, recv string) {
	e := cls.lookup("==")
	if !cls.isStruct() || e == nil || e.M.Private || e.M.generic() || len(e.M.Params) != 1 {
		return
	}
	env := composeEnv(e.Env, nil)
	env["Self"] = c.selfTypeFor(*e, cls)
	t, ok := subst(e.M.Params[0].Type, env).(TClass)
	r, isCls := subst(e.M.Ret, env).(TClass)
	if !ok || !isCls || r.C != c.classes["Boolean"] || c.goType(t) == "any" {
		return // ponytail: T? or union params keep identity; assert via rbOptArg to widen
	}
	c.w("func (self %s) _EqAny(o any) Boolean {\n\tif o, ok := o.(%s); ok {\n\t\treturn self.%s(o)\n\t}\n\treturn false\n}\n\n",
		recv, c.goType(t), e.M.GoName)
}

// slotEnv binds the type variables of slot s's signature, Self included.
func (c *Compiler) slotEnv(s slot) map[string]Type {
	env := composeEnv(s.e.Env, nil)
	env["Self"] = c.selfTypeFor(s.e, s.in)
	s.in.containerEnv(env) // a Set subclass's slot of Set's own: E is untyped
	return env
}

// slotKey is slot s's Go signature without parameter names.
func (c *Compiler) slotKey(s slot) string {
	m := *s.e.M
	m.Params = make([]Param, len(s.e.M.Params))
	for i, p := range s.e.M.Params {
		m.Params[i] = Param{Name: "_", Type: p.Type, Rest: p.Rest}
	}
	ps, ret := c.sig(&m, c.slotEnv(s))
	return "(" + ps + ") " + ret
}

// sigText renders slot s's signature, block aside, in RBS for messages.
func (c *Compiler) sigText(s slot) string {
	env := c.slotEnv(s)
	ps := make([]string, len(s.e.M.Params))
	for i, p := range s.e.M.Params {
		ps[i] = subst(p.Type, env).String()
		if p.Rest {
			ps[i] = "*" + ps[i]
		}
	}
	ret := "untyped" // a return inferred later (decision 36)
	if s.e.M.Ret != nil {
		ret = subst(s.e.M.Ret, env).String()
	}
	return "(" + strings.Join(ps, ", ") + ") -> " + ret
}

// arity is the argument count range m takes; max is -1 with a rest param.
func arity(m *Method) (req, maxArgs int) {
	for _, p := range m.Params {
		switch {
		case p.Rest:
			return req, -1
		case p.Default == nil:
			req++
		}
		maxArgs++
	}
	return req, maxArgs
}

// paramAt is the parameter taking positional argument i.
func paramAt(m *Method, i int) Param {
	for j, p := range m.Params {
		if p.Rest || j == i {
			return p
		}
	}
	panic("paramAt: out of range")
}

// adapts reports whether an adapter can hand a from where a to is expected:
// the same Go type, through untyped, or up a struct hierarchy (down, with a
// type assertion, for arguments).
func (c *Compiler) adapts(from, to Type, arg bool) bool {
	switch {
	case isVoid(to) && !arg:
		return true
	case isNil(from) && holdsNil(to) && !arg: // `-> nil` over a nilable result
		return true
	case !arg && isOpt(to) && !isOpt(from) && c.adapts(from, to.(TOpt).Elem, false): // `-> T` over `-> T?`
		return true
	case isVoid(from):
		return false
	case c.goType(from) == c.goType(to) || isAny(to) || c.goType(to) == "any": // a union is Go any: any value boxes into it
		return true
	case isAny(from):
		_, ok := to.(TClass)
		_, opt := to.(TOpt)
		return ok || opt
	}
	fc, ok1 := from.(TClass)
	tc, ok2 := to.(TClass)
	if !ok1 || !ok2 || !fc.C.isStruct() || !tc.C.isStruct() {
		return false
	}
	return fc.C.isSubclassOf(tc.C) || arg && tc.C.isSubclassOf(fc.C)
}

// checkAdaptable rejects an override whose class cannot answer slot s of an
// ancestor's interface through an adapter.
func (c *Compiler) checkAdaptable(own, s slot) {
	m, sm := own.e.M, s.e.M
	fail := func(why string) {
		c.errorf(nil, nil, "%s:%d: %s: %s overrides %s: %s, but %s; give it the parent's signature or another name",
			m.File.Name, m.Line, m, c.sigText(own), sm, c.sigText(s), why)
	}
	if (m.Block != nil || sm.Block != nil) && !c.sameIterator(own, s) {
		fail("a block-taking override must keep the signature")
	}
	if restSlot(own.e.M, sm) { // Object#to_json(*args) answered by `to_json(options = nil)`: by argument count
		elem := subst(sm.Params[0].Type, c.slotEnv(s))
		for _, p := range m.Params {
			if to := subst(p.Type, c.slotEnv(own)); !c.adapts(elem, to, true) {
				fail(fmt.Sprintf("its rest elements cannot pass a %s as a %s", elem, to))
			}
		}
		from, to := subst(m.Ret, c.slotEnv(own)), subst(sm.Ret, c.slotEnv(s))
		if !c.adapts(from, to, false) {
			fail(fmt.Sprintf("its %s result cannot stand in for %s", from, to))
		}
		return
	}
	for _, p := range sm.Params {
		if p.Rest {
			fail("an override of a rest-parameter method must keep the signature")
		}
	}
	if kwSlot(m, sm) {
		c.checkKwAdaptable(own, s, fail)
		return
	}
	req, maxArgs := arity(m)
	if n := len(sm.Params); n < req || maxArgs >= 0 && n > maxArgs {
		return // the adapter raises ArgumentError, as MRI would
	}
	env, oenv := c.slotEnv(s), c.slotEnv(own)
	// a gem's Hash[String, untyped] over Hash[String, String]: the adapter converts a copy (rbAs, decision 172)
	gemConv := func(from, to Type) bool {
		tc, ok := to.(TClass)
		return c.gemFile(m.File) && ok && converts(tc) && sameButUntyped(from, to)
	}
	for i, p := range sm.Params {
		from, to := subst(p.Type, env), subst(paramAt(m, i).Type, oenv)
		if !c.adapts(from, to, true) && !gemConv(from, to) {
			fail(fmt.Sprintf("argument %d cannot pass a %s as a %s", i+1, from, to))
		}
	}
	from, to := subst(m.Ret, oenv), subst(sm.Ret, env)
	if !c.adapts(from, to, false) && !gemConv(from, to) {
		fail(fmt.Sprintf("its %s result cannot stand in for %s", from, to))
	}
}

// kwSlot reports an override and a slot that both take keywords: the adapter passes them by name (decision 241).
func kwSlot(m, sm *Method) bool {
	kw := func(p Param) bool { return p.Keyword || p.KwRest }
	return slices.ContainsFunc(m.Params, kw) && slices.ContainsFunc(sm.Params, kw) && m.Block == nil && sm.Block == nil
}

// positionals is m's leading positional parameters (no rest, keywords or post).
func positionals(m *Method) []Param {
	var out []Param
	for _, p := range m.Params {
		if p.Rest || p.Keyword || p.KwRest || p.Post {
			break
		}
		out = append(out, p)
	}
	return out
}

// checkKwAdaptable checks a keyword slot: the positionals pair up by place, the slot's keywords the override takes by
// name; one the override lacks goes to its **rest, or the slot cannot be answered; an override keyword the slot lacks
// comes from the slot's **rest, or is defaulted (decision 241).
func (c *Compiler) checkKwAdaptable(own, s slot, fail func(string)) {
	m, sm := own.e.M, s.e.M
	env, oenv := c.slotEnv(s), c.slotEnv(own)
	pos, spos := positionals(m), positionals(sm)
	if len(spos) > len(pos) || slices.ContainsFunc(m.Params, func(p Param) bool { return p.Rest || p.Post }) {
		fail("its positional parameters do not line up")
	}
	for i, p := range spos {
		if from, to := subst(p.Type, env), subst(pos[i].Type, oenv); !c.adapts(from, to, true) {
			fail(fmt.Sprintf("argument %d cannot pass a %s as a %s", i+1, from, to))
		}
	}
	for i := len(spos); i < len(pos); i++ {
		if pos[i].Default == nil {
			fail(fmt.Sprintf("its argument %d is required", i+1))
		}
	}
	ownRest := slices.ContainsFunc(m.Params, func(p Param) bool { return p.KwRest })
	for _, p := range sm.Params {
		if !p.Keyword || p.KwRest {
			continue
		}
		i := slices.IndexFunc(m.Params, func(q Param) bool { return q.Keyword && !q.KwRest && q.Name == p.Name })
		switch {
		case i >= 0:
			if from, to := subst(p.Type, env), subst(m.Params[i].Type, oenv); !c.adapts(from, to, true) && !c.gemFile(m.File) {
				fail(fmt.Sprintf("keyword %s cannot pass a %s as a %s", p.Name, from, to))
			}
		case !ownRest:
			fail("it takes no keyword " + p.Name)
		}
	}
	from, to := subst(m.Ret, oenv), subst(sm.Ret, env)
	if !c.adapts(from, to, false) && !c.gemFile(m.File) {
		fail(fmt.Sprintf("its %s result cannot stand in for %s", from, to))
	}
}

// kwAdapterArgs is a keyword slot's call of the override: the slot's positionals, its keywords by name, and its
// **rest splatted, from which the override's own keywords are taken (decision 241).
func kwAdapterArgs(sm *Method, env map[string]Type) []parser.Node {
	var args []parser.Node
	kh := &parser.KeywordHashNode{}
	for i, p := range sm.Params {
		a := &exprNode{e: expr{code: goParamName(p.Name, i), typ: subst(p.Type, env)}}
		switch {
		case p.KwRest:
			kh.Elements = append(kh.Elements, &parser.AssocSplatNode{Value: a})
		case p.Keyword:
			kh.Elements = append(kh.Elements, &parser.AssocNode{Key: &parser.SymbolNode{Unescaped: parser.RubyString{Value: p.Name}}, Value: a})
		default:
			args = append(args, a)
		}
	}
	if len(kh.Elements) > 0 {
		args = append(args, kh)
	}
	return args
}

// seqDropped counts the trailing nil-defaulted params an `each` seq adapter leaves out, so it answers Enumerable's
// `each()` slot (concurrent-ruby's `each(head = nil)`); it passes nil for them.
func seqDropped(m *Method) int {
	if m.Name != "each" {
		return 0
	}
	k := 0
	for i := len(m.Params) - 1; i >= 0; i-- {
		if _, ok := m.Params[i].Default.(*parser.NilNode); !ok || m.Params[i].Keyword || m.calleeDefaults {
			break
		}
		k++
	}
	return k
}

// seqArgs is argNames(m) with the params seqDropped leaves out passed as nil.
func (c *Compiler) seqArgs(m *Method) string {
	as := strings.Split(c.argNames(m), ", ")
	for _, p := range m.Params[len(m.Params)-seqDropped(m):] {
		if i := slices.Index(as, goLocalName(p.Name)); i >= 0 {
			as[i] = "nil"
		}
	}
	return strings.Join(as, ", ")
}

// sameIterator reports two iterators yielding the same Go types: an adapter returns the override's sequence.
func (c *Compiler) sameIterator(own, s slot) bool {
	m, sm := own.e.M, s.e.M
	if !m.Iterator || !sm.Iterator || len(m.Block.Params) != len(sm.Block.Params) {
		return false
	}
	if c.iterGoType(m.Block, c.slotEnv(own)) == c.iterGoType(sm.Block, c.slotEnv(s)) {
		return true
	}
	return len(sm.Block.Params) == 1 && c.goType(subst(sm.Block.Params[0], c.slotEnv(s))) == "any" // elements boxed (iterAdapterBody)
}

// iterAdapterBody answers an iterator slot whose elements are untyped with an override yielding typed ones.
func (c *Compiler) iterAdapterBody(cls *Class, e entry, s slot) string {
	env, oenv := c.slotEnv(s), c.forwarderEnv(cls, e)
	args := make([]string, 0, len(e.M.Params))
	for i, p := range e.M.Params {
		if i >= len(s.e.M.Params) { // `each(*arg)` answering `each`: the slot passes nothing for the rest (ruby/spec's EachCounter)
			if p.Rest {
				continue
			}
			panic(compileError{msg: fmt.Sprintf("%s#%s takes %d parameters where %s#%s's iterator takes %d", cls.RubyName, e.M.Name, len(e.M.Params), s.e.Owner.RubyName, s.e.M.Name, len(s.e.M.Params))})
		}
		from, to := c.goType(subst(s.e.M.Params[i].Type, env)), c.goType(subst(p.Type, oenv))
		a := goParamName(s.e.M.Params[i].Name, i)
		if from != to && to != "any" {
			a = fmt.Sprintf("rbAs[%s](any(%s), %q)", to, a, to)
		}
		args = append(args, a)
	}
	elem, x := "any", "x"
	if len(s.e.M.Block.Params) == 1 && len(e.M.Block.Params) == 1 { // the slot's element type: typed when the parent's yields are
		elem = c.goType(subst(s.e.M.Block.Params[0], env))
		if own := c.goType(subst(e.M.Block.Params[0], oenv)); own != elem && elem != "any" {
			x = fmt.Sprintf("rbAs[%s](any(x), %q)", elem, elem)
		}
	}
	return fmt.Sprintf("\treturn func(y func(%s) bool) {\n\t\tfor x := range %s {\n\t\t\tif !y(%s) {\n\t\t\t\treturn\n\t\t\t}\n\t\t}\n\t}\n",
		elem, c.blkCall(cls, e, strings.Join(args, ", ")), x)
}

// adapterBody answers slot s of an ancestor's interface with cls's method e
// (checked by checkAdaptable): an argument count e cannot take raises MRI's
// ArgumentError, anything else converts across the two signatures.
func (c *Compiler) adapterBody(cls *Class, e entry, s slot) string {
	if s.dyn {
		return c.dynAdapterBody(cls, e, s)
	}
	file := cls.File
	if e.M.File != nil {
		file = e.M.File
	}
	f := c.newFctx(file, cls, nil)
	f.lex = e.M.Scope
	f.locals = map[localKey]*localInfo{}
	f.scope = &scope{vars: map[string]*local{}}
	f.pass = 2
	f.indent = 1
	if restSlot(e.M, s.e.M) {
		return c.restAdapterBody(f, cls, e, s)
	}
	if e.M.Iterator && s.e.M.Iterator {
		return c.iterAdapterBody(cls, e, s)
	}
	if kwSlot(e.M, s.e.M) {
		env := c.slotEnv(s)
		recv := expr{code: "self", typ: TClass{C: cls}}
		res := f.callEntry(&parser.NilNode{}, &e, recv, kwAdapterArgs(s.e.M, env), nil)
		ret := subst(s.e.M.Ret, env)
		switch {
		case !isVoid(ret):
			f.emit("return %s", f.coerce(&parser.NilNode{}, res, ret))
		case res.code != "":
			f.emit("%s", res.code)
		}
		return f.buf.String()
	}
	n := len(s.e.M.Params)
	if req, maxArgs := arity(e.M); n < req || maxArgs >= 0 && n > maxArgs {
		f.emit("rbArity(%d, %d, %d)\n\tpanic(\"unreachable\")", n, req, maxArgs)
		return f.buf.String()
	}
	recv := expr{code: "self", typ: TClass{C: cls}}
	env, oenv := c.slotEnv(s), composeEnv(e.Env, nil)
	oenv["Self"] = recv.typ
	args := make([]parser.Node, n)
	for i, p := range s.e.M.Params {
		a := expr{code: goParamName(p.Name, i), typ: subst(p.Type, env)}
		to := subst(paramAt(e.M, i).Type, oenv)
		if tc, ok := to.(TClass); ok && tc.C.isStruct() && !c.adapts(a.typ, to, false) {
			a = expr{code: a.code + ".(" + c.goType(to) + ")", typ: to} // narrower: assert
		}
		args[i] = &exprNode{e: a}
	}
	res := f.callEntry(&parser.NilNode{}, &e, recv, args, nil)
	ret := subst(s.e.M.Ret, env)
	switch {
	case !isVoid(ret):
		f.emit("return %s", f.coerce(&parser.NilNode{}, res, ret))
	case res.code != "":
		f.emit("%s", res.code)
	}
	return f.buf.String()
}

// restSlot reports a slot of only `*args` that m, with positional parameters alone, answers by argument count.
func restSlot(m, sm *Method) bool {
	if len(sm.Params) != 1 || !sm.Params[0].Rest || m.Block != nil || sm.Block != nil || m.kwMask() || m.calleeDefaults {
		return false
	}
	return !slices.ContainsFunc(m.Params, func(p Param) bool { return p.Rest || p.Keyword || p.KwRest || p.Post })
}

// restAdapterBody calls m with as many of rest_'s elements as were passed, or raises MRI's ArgumentError.
func (c *Compiler) restAdapterBody(f *fctx, cls *Class, e entry, s slot) string {
	recv := expr{code: "self", typ: TClass{C: cls}}
	env := c.slotEnv(s)
	elem := subst(s.e.M.Params[0].Type, env)
	ret := subst(s.e.M.Ret, env)
	req, maxArgs := arity(e.M)
	for k := req; k <= maxArgs; k++ {
		args := make([]parser.Node, k)
		for i := range k {
			args[i] = &exprNode{e: expr{code: fmt.Sprintf("rest_[%d]", i), typ: elem}}
		}
		f.emit("if len(rest_) == %d {", k)
		f.indent++
		res := f.callEntry(&parser.NilNode{}, &e, recv, args, nil)
		switch {
		case !isVoid(ret):
			f.emit("return %s", f.coerce(&parser.NilNode{}, res, ret))
		case res.code != "":
			f.emit("%s", res.code)
			f.emit("return")
		default:
			f.emit("return")
		}
		f.indent--
		f.emit("}")
	}
	f.emit("rbArity(len(rest_), %d, %d)\n\tpanic(\"unreachable\")", req, maxArgs)
	return f.buf.String()
}

// seqAdapterSig renders the iter.Seq adapter a closure override of an
// iterator answers to under the iterator's Go name (decision 4).
func (c *Compiler) seqAdapterSig(m *Method, env map[string]Type) (name, params, ret string) {
	it := *m
	it.Iterator = true
	it.Params = m.Params[:len(m.Params)-seqDropped(m)]
	params, ret = c.sig(&it, env)
	return goMethodName(m.Name), params, ret
}

// sigMentions reports whether m's parameters, block or return name cls, which
// the free func types as Self: bound to *C, they would not match the
// forwarder's CI (a `self?` return is **C where *CI is wanted).
// ponytail: matches by rendered name, so a same-named class elsewhere also opts out; walk the Type if that costs a hot method.
func sigMentions(m *Method, cls *Class) bool {
	sig := fmt.Sprint(m.Params, m.Block, m.Ret)
	return strings.Contains(sig, cls.RubyName) || strings.Contains(sig, "Self")
}

// forwardTypeArgs renders explicit type args for a forwarder call.
func (c *Compiler) forwardTypeArgs(e entry, cls *Class) string {
	var args []string
	switch {
	case e.Owner.GoType != "", c.plainSelf(e.M):
	case e.Owner == cls && cls.isStruct() && len(cls.TypeParams) == 0 && !sigMentions(e.M, cls):
		// the class's own method: Self is *C, so the body's self._C() and
		// self-calls are direct calls Go can inline, not interface calls
		args = append(args, c.recvType(cls))
	case cls.isStruct() && e.Owner.universal && len(cls.TypeParams) == 0 && !sigMentions(e.M, cls):
		args = append(args, "any") // Self is any: one instantiation for every class, where *C was a dictionary wrapper per class (decision 208)
	case cls.isStruct() && e.Owner.IsModule && !e.Owner.universal && len(cls.TypeParams) == 0 && !sigMentions(e.M, cls) && !c.constraintNamesSelf(e.Owner):
		// the constraint itself, one instantiation for every includer; an includer's own CI is a GC shape per class (decision 208)
		tps := make([]string, 0, len(e.Owner.TypeParams))
		for _, p := range e.Owner.TypeParams {
			tps = append(tps, c.goType(e.Env[p]))
		}
		args = append(args, e.Owner.Name+"_Self[any"+comma(strings.Join(tps, ", "))+"]")
	case cls.isStruct() && !e.Owner.universal:
		// Self matches the forwarder's `self` (CI), else *C fails Comparable_Self[*C] and `(self)` args don't pass
		args = append(args, c.goType(c.selfTypeFor(e, cls)))
	default:
		args = append(args, c.recvType(cls))
	}
	for _, p := range e.Owner.TypeParams {
		args = append(args, c.goType(e.Env[p]))
	}
	if len(args) == 0 {
		return ""
	}
	return "[" + strings.Join(args, ", ") + "]"
}

// constraintNamesSelf reports a module whose Self constraint has a method typed by Self (Comparable_Self's
// Op_cmp(other Self)): only the includer's own interface satisfies it.
func (c *Compiler) constraintNamesSelf(owner *Class) bool {
	if v, ok := c.selfTyped[owner]; ok {
		return v
	}
	pending := c.pendingSigs
	saved := c.out
	c.out = strings.Builder{}    // ponytail: renders the constraint to read it; saved restores into the same address, so Builder's copy check holds
	c.renderingConstraint = true // every self-called name, dispatched or not: dispatchable asks this, so it must not ask back
	c.emitModuleInterface(owner)
	c.renderingConstraint = false
	decl := c.out.String()
	c.out = saved
	_, body, _ := strings.Cut(decl, "_Self[Self any")
	_, body, _ = strings.Cut(body, "{")
	v := selfWord.MatchString(body)
	if c.pendingSigs == pending { // a return still being inferred (`wait`'s `self`, asked about from inside wait) may name Self yet
		c.selfTyped[owner] = v
	}
	return v
}

var selfWord = regexp.MustCompile(`\bSelf\b`)

// maxTuple is the widest RBS tuple: Socket.getaddrinfo's rows have 7 elements (decision 135).
const maxTuple = 7

func (c *Compiler) emitTuples() {
	for n := 2; n <= maxTuple; n++ {
		if !c.tupleN[n] {
			continue
		}
		tps := make([]string, 0, n)
		fields := make([]string, 0, n)
		cmp := make([]string, 0, n)
		vals := make([]string, 0, n)
		insp := make([]string, 0, n)
		eq := make([]string, 0, n)
		from := make([]string, 0, n)
		plain := make([]string, 0, n)
		for i := range n {
			p := fmt.Sprintf("T%d", i)
			plain = append(plain, "rbPlainKey["+p+"]()")
			tps = append(tps, p)
			fields = append(fields, fmt.Sprintf("F%d %s", i, p))
			cmp = append(cmp, fmt.Sprintf("if c := rbCmp(t.F%d, o.F%d); c != 0 {\n\t\treturn c\n\t}", i, i))
			vals = append(vals, fmt.Sprintf("t.F%d", i))
			insp = append(insp, fmt.Sprintf("rbInspect(t.F%d)", i))
			eq = append(eq, fmt.Sprintf("rbEq(t.F%d, o2.F%d)", i, i))
			from = append(from, fmt.Sprintf("\tif t.F%d, ok = s[%d].(T%d); !ok {\n\t\treturn t, false\n\t}", i, i, i))
		}
		name := fmt.Sprintf("Tuple%d", n)
		c.w("type %s[%s comparable] struct {\n\t%s\n}\n\n", name, strings.Join(tps, ", "), strings.Join(fields, "\n\t"))
		full := fmt.Sprintf("%s[%s]", name, strings.Join(tps, ", "))
		c.w("func (t %s) Op_cmp(o %s) Integer {\n\t%s\n\treturn 0\n}\n\n", full, full, strings.Join(cmp, "\n\t"))
		c.w("func (t %s) Inspect() String { return \"[\" + %s + \"]\" }\n\n", full, strings.Join(insp, ` + ", " + `))
		c.w("func (t %s) ToS() String { return t.Inspect() }\n\n", full)
		c.w("func (t %s) ToJson(state ...any) String { return rbJSONArray([]any{%s}, state) }\n\n", full, strings.Join(vals, ", "))
		// a tuple that reaches untyped answers as an Array (decision 22)
		c.w("func (t %s) _ToAny() *Array[any] { return %s }\n\n", full, arrayLit("any", vals))
		// and converts back where a dynamic call's parameter is a tuple (rbAs)
		c.w("func (%s) _FromAny(a any) (t %s, ok bool) {\n\tarr, ok := a.(Array_Any)\n\tif !ok {\n\t\treturn t, false\n\t}\n\ts := arr._ToAny().s\n\tif len(s) != %d {\n\t\treturn t, false\n\t}\n%s\n\treturn t, true\n}\n\n", full, full, n, strings.Join(from, "\n"))
		c.w("func (t %s) Op_eq(o any) Boolean {\n\to2, ok := o.(%s)\n\tif !ok {\n\t\tif a, isArr := o.(Array_Any); isArr {\n\t\t\treturn t._ToAny().Op_eq(a._ToAny())\n\t\t}\n\t\treturn false\n\t}\n\treturn %s\n}\n\n", full, full, strings.Join(eq, " && "))
		c.w("func (t %s) rbPlain() bool { return %s }\n\n", full, strings.Join(plain, " && "))
	}
}

// unionFn names the generated helpers for union u (decision 150): a hash of
// its members, so the name does not depend on what the program compiled first.
func (c *Compiler) unionFn(u TUnion) string {
	h := fnv.New64a()
	_, _ = h.Write([]byte(u.String())) // a hash.Hash never fails
	name := fmt.Sprintf("rbUnion_%016x", h.Sum64())
	if old, ok := c.unions[name]; ok && old.String() != u.String() {
		panic(compileError{msg: "rb2go: union helper name collision: " + old.String() + " and " + u.String()}) // never silently share one checker
	}
	c.unions[name] = u
	return name
}

// emitUnions emits, per union the program converts into or out of:
// rbUnion_<hash>(a) checks an untyped value is one of the members (MRI's
// TypeError otherwise), converting an Array or Hash of another
// instantiation as rbAs does; rbUnion_<hash>Out(a) shows a tuple member to
// untyped code as the Array it is (decision 22).
func (c *Compiler) emitUnions() {
	for _, name := range slices.Sorted(maps.Keys(c.unions)) {
		if c.unionsOut[name] {
			continue
		}
		c.unionsOut[name] = true
		u := c.unions[name]
		exact := make([]string, 0, len(u.Members))
		var conv, tuples []string
		for _, m := range u.Members {
			g := "nil"
			if !isNil(m) {
				g = c.goType(m)
			}
			exact = append(exact, g)
			switch m := m.(type) {
			case TTuple:
				conv = append(conv, g)
				tuples = append(tuples, g)
			case TClass:
				if len(m.Args) > 0 {
					conv = append(conv, g)
				}
			case TAny, TFunc, TNil, TOpt, TUnion, TVar, TVoid: // matched exactly or not at all
			}
		}
		c.w("func %s(a any) any {\n\tswitch a.(type) {\n\tcase %s:\n\t\treturn a\n\t}\n", name, strings.Join(exact, ", "))
		for _, g := range conv {
			c.w("\tif v, ok := rbConv[%s](a); ok {\n\t\treturn v\n\t}\n", g)
		}
		c.w("\tpanic(rbConvError(a, %q))\n}\n\n", u.String())
		if len(tuples) > 0 {
			c.w("func %sOut(a any) any {\n\tswitch v := a.(type) {\n", name)
			for _, g := range tuples {
				c.w("\tcase %s:\n\t\treturn v._ToAny()\n", g)
			}
			c.w("\t}\n\treturn a\n}\n\n")
		}
	}
}

// emitBoxes emits the helpers that open a T? box (*T) seen as `any`: generic
// code holding E = T? hands the box itself to rbInspect, rbEq, rbCmp...,
// where a nil *T is not a nil interface and a non-nil one has the wrong
// method set. Only concrete T? types rendered somewhere can reach them.
func (c *Compiler) emitBoxes() {
	boxes := slices.Sorted(maps.Keys(c.boxes))
	all := maps.Clone(c.argBoxes)
	maps.Copy(all, c.boxes)
	unbox := slices.Sorted(maps.Keys(all))
	c.w("func rbUnbox(a any) any {\n\tswitch v := a.(type) {\n")
	for _, b := range unbox {
		if all[b] {
			c.w("\tcase %s:\n\t\treturn rbUnbox(Opt(v))\n", b)
		} else {
			c.w("\tcase %s:\n\t\treturn Opt(v)\n", b)
		}
	}
	c.w("\t}\n\treturn a\n}\n\n")
	// rbKeyUnbox opens one level of a box used as a Hash key or in uniq/tally, which match by what it holds
	c.w("func rbKeyUnbox(k any) (any, bool) {\n\tswitch v := k.(type) {\n")
	for _, b := range unbox {
		c.w("\tcase %s:\n\t\treturn Opt(v), true\n", b)
	}
	c.w("\t}\n\treturn nil, false\n}\n\n")
	c.w("func rbCmpBox(a, b any) Integer {\n\tswitch v := a.(type) {\n")
	for _, b := range boxes {
		c.w("\tcase %s:\n\t\treturn rbCmpOpt(v, b.(%s))\n", b, b)
	}
	c.w("\t}\n\treturn rbCmpFailed(a, b)\n}\n\n")
	// Marshal.load fills a T? slot of generic code (rbMAs): a box of what it holds
	c.w("func rbMBox(dst, v any) bool {\n\tswitch d := dst.(type) {\n\tcase nil:\n\t\treturn false\n")
	for _, b := range unbox {
		c.w("\tcase *%s:\n\t\tx := rbMAs[%s](v)\n\t\t*d = &x\n", b, b[1:])
	}
	c.w("\tdefault:\n\t\treturn false\n\t}\n\treturn true\n}\n\n")
}

// wantsForwarder decides whether class cls gets a Go method forwarding to
// the free func of entry e. Struct classes forward everything (their
// interface is the full method set). Primitive classes only forward what a
// module's constraint needs: a forwarder on a generic container can
// instantiate ever-growing types (Hash[K,V].Tally → Hash[[K,V],Integer]...),
// which Go rejects as an instantiation cycle.
func (c *Compiler) wantsForwarder(cls *Class, e entry) bool {
	if growsOwnClass(cls, e) {
		return false // a Go method on Lazy[E] returning Lazy[[E, Integer]] is an instantiation cycle; calls reach the free func (decision 207)
	}
	if c.dispatchableAll(cls, e.M.Name) {
		return false // calls go to the free func or an rbSend_ type switch; no interface declares the name (dispatch.go)
	}
	if e.Owner == cls {
		if len(cls.TypeParams) == 0 || cls.GoType == "" {
			return true
		}
		// a generic primitive's own method is a Go method only where something reaches it through a Go method set:
		// the std library, an included module's self calls, or a universal name (inspect, ==, hash) an ObjectI or
		// rbEq assertion selects; typed calls already reach the free func (decision 86), so Array[E] keeps a fixed,
		// small method set per instantiation instead of one per name (#96 phase 3b)
		return stdMethodNames[e.M.GoName] || genericProtocol[e.M.Name] || c.includerCalls(cls, e.M.Name) || c.universalHas(e.M.Name)
	}
	if cls.isStruct() {
		return true
	}
	if e.Owner.universal {
		return c.includerCalls(cls, e.M.Name)
	}
	return e.Owner.IsModule && c.selfCalls(e.Owner)[e.M.Name] && !c.dispatchable(e.Owner, e.M)
}

// genericProtocol are the names Go code asserts or calls as methods on a generic primitive of unknown instantiation.
var genericProtocol = map[string]bool{
	"_to_any": true, "_from_any": true, "_eq_rec": true, "_eql_rec": true, "_hash_rec": true, "_inspect_rec": true,
	"==": true, "eql?": true, "hash": true, "<=>": true, "each": true, "succ": true, "to_s": true, "inspect": true,
	"to_json": true, "pretty_print": true,
	"_len": true, "_at": true, "_set": true, "_make": true, "_append": true, "_trunc": true, // the untyped views (decision 236)
	"_key_at": true, "_has": true, "_get": true, "_store": true, "_delete": true, "_dup": true,
}

// universalHas reports whether a universal owner (Kernel, Object, BasicObject) defines name, so a class's own method of
// that name overrides a protocol every object answers.
func (c *Compiler) universalHas(name string) bool {
	if c.universalNames == nil {
		c.universalNames = map[string]bool{}
		for _, k := range c.classes {
			if !k.universal {
				continue
			}
			for _, e := range k.methodSet() {
				c.universalNames[e.M.Name] = true
			}
		}
	}
	return c.universalNames[name]
}

// growsOwnClass reports a generic class's method whose signature names the class at a type built from its params (Lazy[[E, Integer]] on Lazy[E]).
func growsOwnClass(cls *Class, e entry) bool {
	if len(cls.TypeParams) == 0 {
		return false
	}
	var grows func(t Type) bool
	grows = func(t Type) bool {
		switch t := t.(type) {
		case TClass:
			if t.C == cls && mentionsVar(t) {
				for i, a := range t.Args {
					if v, ok := a.(TVar); !ok || i >= len(cls.TypeParams) || v.Name != cls.TypeParams[i] {
						return true
					}
				}
			}
			return slices.ContainsFunc(t.Args, grows)
		case TOpt:
			return grows(t.Elem)
		case TTuple:
			return slices.ContainsFunc(t.Elems, grows)
		case TFunc:
			return slices.ContainsFunc(t.Params, grows) || t.Ret != nil && grows(t.Ret)
		case TUnion:
			return slices.ContainsFunc(t.Members, grows)
		case TAny, TNil, TVar, TVoid:
		}
		return false
	}
	ts := []Type{e.M.Ret}
	for _, p := range e.M.Params {
		ts = append(ts, p.Type)
	}
	if b := e.M.Block; b != nil {
		ts = append(ts, b.Params...)
		ts = append(ts, b.Ret)
	}
	return slices.ContainsFunc(ts, func(t Type) bool { return t != nil && grows(subst(t, e.Env)) })
}

// includerCalls: a module method calling name on self needs cls to answer it.
func (c *Compiler) includerCalls(cls *Class, name string) bool {
	if c.universalDispatch(name) {
		return false // the module body dispatches it (dispatch.go), so its constraint does not declare it
	}
	for k := cls; k != nil && !k.universal; k = k.Super {
		for _, inc := range k.Includes {
			if !c.selfCalls(inc.Mod)[name] {
				continue
			}
			if e := inc.Mod.lookup(name); e != nil && e.Owner.IsModule && c.dispatchable(e.Owner, e.M) {
				continue // the module body dispatches it by type switch (dispatch.go); its constraint does not declare it
			}
			return true
		}
	}
	return false
}

// constFctx is the codegen context a constant's initializer runs in: the
// class body, whose self is the class object (main at top level).
func (c *Compiler) constFctx(k *Const) *fctx {
	f := c.newFctx(k.File, nil, nil)
	if n := len(k.Scope); n > 0 && k.Scope[n-1].meta != nil {
		cls := k.Scope[n-1]
		f.selfType, f.selfCode, f.selfClassObj = TClass{C: cls.meta}, classVar(cls), true
	}
	f.lex = k.Scope
	f.locals = map[localKey]*localInfo{}
	f.scope = &scope{vars: map[string]*local{}}
	f.pass = 2
	return f
}

// constType is the type of constant k: its `#: T` annotation, or else the
// type of its initializer.
func (c *Compiler) constType(k *Const) (typ Type) {
	if c.env.KeepGoing && c.anonDepth == 0 {
		defer func() {
			if r := recover(); r != nil {
				ce, ok := r.(compileError)
				if !ok {
					panic(r)
				}
				c.kept = append(c.kept, ce.msg)
				k.Type, k.resolving, typ = TAny{}, false, TAny{}
			}
		}()
	}
	if k.Type != nil {
		return k.Type
	}
	if _, boot := c.bootConstClasses[k.RubyName]; boot {
		c.constClass(k) // a class picked at load time: its value becomes that class (decision 203)
	}
	if k.ann != "" {
		t, err := rbs.ParseType(k.ann)
		if err != nil {
			c.errorf(k.File, k.Value, "%v", err)
		}
		k.Type = c.resolveType(t, typeScope{lex: k.Scope, file: k.File, line: k.Line})
		return k.Type
	}
	if t := c.sigConst(k); t != nil { // a `.rbs` constant (decision 160)
		k.Type = c.resolveType(t, typeScope{lex: k.Scope, file: k.File, line: k.Line})
		return k.Type
	}
	if k.inBody && c.gemFile(k.File) {
		k.Type = TAny{} // gem code: untyped rather than typed outside its body's unit
		return k.Type
	}
	if k.inBody {
		c.errorf(k.File, k.Value, "constant %s reads a local of its class body: give it a type (`#: T`, or `%s: T` in a .rbs file)", k.RubyName, k.RubyName[strings.LastIndex(k.RubyName, ":")+1:])
	}
	if k.resolving && c.gemFile(k.File) {
		return TAny{} // gem code: a constant whose inferred type loops back to itself is untyped (decision 172)
	}
	if k.resolving {
		c.errorf(k.File, k.Value, "constant %s depends on itself (via %s); annotate it with `#: T`", k.RubyName, strings.Join(c.constChain, " -> "))
	}
	k.resolving = true
	c.constChain = append(c.constChain, k.RubyName)
	f := c.constFctx(k)
	var e expr
	if c.gemFile(k.File) {
		err := catchCompileError(func() { f.probe(func() { e = f.genExpr(k.Value, nil) }) })
		if err != nil {
			c.constChain = c.constChain[:len(c.constChain)-1]
			k.resolving, k.poison, k.Type = false, err.msg, TAny{}
			c.warn(k.File, k.Value, "constant %s does not compile, so it is nil: %s (decision 205)", k.RubyName, err.msg)
			return k.Type
		}
	} else {
		f.probe(func() { e = f.genExpr(k.Value, nil) })
	}
	c.constChain = c.constChain[:len(c.constChain)-1]
	k.resolving = false
	if (isNil(e.typ) || isVoid(e.typ)) && (c.gemFile(k.File) || k.typeAlias != "") {
		e.typ = TAny{} // a Sorbet type alias's run-time value, or a gem constant only ever nil: untyped
	}
	if isNil(e.typ) || isVoid(e.typ) {
		c.errorf(k.File, k.Value, "cannot infer the type of constant %s; annotate it with `#: T`", k.RubyName)
	}
	k.Type = e.typ
	return k.Type
}

// emitConst declares a constant's package variable. Its value is assigned
// in main, in source order, where MRI would evaluate it (see constInit).
func (c *Compiler) emitConst(k *Const) {
	c.lineDirective(k.File, k.Line)
	c.w("var %s %s\n\n", k.GoName, c.goType(c.constType(k)))
	if k.guarded {
		c.w("var %s bool\n\n", constSet(k))
	}
}

// constSet names the flag main sets once a guarded constant is assigned.
func constSet(k *Const) string { return "rbSet_" + k.GoName }

// constEntries lists the constants a class object answers to: its own in
// definition order, then those of its ancestors (Object's are the top-level
// ones).
func (c *Compiler) constEntries(cls *Class) []string {
	if cls.RubyName == "Object" {
		return c.topConstNames
	}
	seen := map[string]bool{}
	var out []string
	for _, k := range cls.ancestors() {
		for _, n := range k.constNames {
			if !seen[n] {
				seen[n] = true
				out = append(out, k.RubyName+"::"+n)
			}
		}
	}
	return out
}

// constValue is the Go expression for a constant's current value, boxed.
func (c *Compiler) constValue(full string) (string, bool) {
	if cls := c.classes[full]; cls != nil {
		if cls.meta == nil {
			return "", false
		}
		return classVar(cls), true
	}
	k := c.consts[full]
	if isOpt(c.constType(k)) {
		return "Opt(" + k.GoName + ")", true
	}
	return k.GoName, true
}

// emitConstTable gives a class object its constant table. Values are read
// when the table is asked for, so constants assigned later are seen.
func (c *Compiler) emitConstTable(cls *Class) {
	desc := cls.metaOf
	kind := "class"
	if desc != nil && desc.IsModule {
		kind = "module"
	}
	c.w("func (self *%s) _Kind() string { return %q }\n\n", cls.Name, kind)
	c.w("func (self *%s) _IsInstance(v any) bool { return %s }\n\n", cls.Name, c.isInstanceTest(desc))
	if desc != nil {
		c.w("func (self *%s) _DescID() int { return %d }\n\n", cls.Name, c.classID(desc))
		c.emitDescendants(cls, desc)
	}
	c.emitMethodTable(cls, desc)
	c.emitPrivateMethodTable(cls, desc)
	c.emitSingletonMethodTable(cls, desc)
	if desc == nil {
		c.w("func (self *%s) _Consts() []rbConst { return nil }\n\n", cls.Name)
		return
	}
	c.w("func (self *%s) _Consts() []rbConst {\n\treturn []rbConst{\n", cls.Name)
	for _, full := range c.constEntries(desc) {
		if v, ok := c.constValue(full); ok {
			short := full[strings.LastIndex(full, ":")+1:]
			inherited := strings.Contains(full, "::") && strings.TrimSuffix(full, "::"+short) != desc.RubyName
			c.w("\t\t{%q, %s, %t},\n", short, v, inherited)
		}
	}
	c.w("\t}\n}\n\n")
}

// emitDescendants gives a class object its descendant classes in definition
// order, which is the order MRI's inherited hooks would see them (rb2go's
// classes are all declared statically). Only rbDescendants asks for it.
func (c *Compiler) emitDescendants(meta, desc *Class) {
	var subs []string
	for _, k := range c.classList {
		if k != desc && !k.IsModule && k.metaOf == nil && k.meta != nil && k.isSubclassOf(desc) {
			subs = append(subs, classVar(k))
		}
	}
	c.w("func (self *%s) _Descendants() []any { return []any{%s} }\n\n", meta.Name, strings.Join(subs, ", "))
	var anc []string
	for _, k := range fullAncestors(desc) {
		for i := len(k.prepended) - 1; i >= 0; i-- { // `prepend M` puts M before the class (decision 199)
			if m := k.prepended[i]; m.meta != nil {
				anc = append(anc, classVar(m))
			}
		}
		if k.meta != nil && (!k.hidden || k == desc) {
			anc = append(anc, classVar(k))
		}
	}
	c.w("func (self *%s) _Ancestors() []any { return []any{%s} }\n\n", meta.Name, strings.Join(anc, ", "))
	if desc.IsModule {
		return
	}
	// Class#subclasses: the direct ones, newest first as MRI lists them
	var direct []string
	for _, k := range slices.Backward(c.classList) {
		if k.Super == desc && k.metaOf == nil && k.meta != nil {
			direct = append(direct, classVar(k))
		}
	}
	c.w("func (self *%s) _Subclasses() []any { return []any{%s} }\n\n", meta.Name, strings.Join(direct, ", "))
	sup := "nil"
	if desc.Super != nil && desc.Super.meta != nil {
		sup = classVar(desc.Super)
	}
	c.w("func (self *%s) _Superclass() any { return %s }\n\n", meta.Name, sup)
}

// fullAncestors is Module#ancestors: k, its modules (last included first,
// each with its own), then its superclass's, through Object, Kernel and
// BasicObject. A module already above keeps that place only, as MRI skips
// re-including it.
func fullAncestors(k *Class) []*Class {
	var all []*Class
	var walk func(k *Class)
	walk = func(k *Class) {
		all = append(all, k)
		for i := len(k.Includes) - 1; i >= 0; i-- {
			if m := k.Includes[i].Mod; m != nil {
				walk(m)
			}
		}
		if k.Super != nil && !k.IsModule {
			walk(k.Super)
		}
	}
	walk(k)
	out := make([]*Class, 0, len(all))
	for i, x := range all {
		if !slices.Contains(all[i+1:], x) {
			out = append(out, x)
		}
	}
	return out
}

// emitMethodTable gives a class object the names of its public instance
// methods, its ancestors' included (decision 77). Object, Kernel and
// BasicObject are left out; MRI lists theirs too. Only
// Module#public_instance_methods selects _Methods, so the pruner drops the
// tables from programs that don't reflect.
func (c *Compiler) emitMethodTable(cls, desc *Class) {
	if desc == nil {
		c.w("func (self *%s) _Methods() []rbConst { return nil }\n\n", cls.Name)
		return
	}
	c.w("func (self *%s) _Methods() []rbConst {\n\treturn []rbConst{\n", cls.Name)
	seen := map[string]bool{}
	for _, k := range desc.ancestors() {
		for _, m := range k.MethodList {
			if m.Private || m.Name == "initialize" || strings.HasPrefix(m.Name, "__") || seen[m.Name] {
				continue
			}
			if desc.specChild && k != desc && strings.HasPrefix(m.Name, "test_") {
				continue // a nested describe runs only its own tests: MRI undefines the rest (nuke_test_methods!)
			}
			seen[m.Name] = true
			c.w("\t\t{%q, nil, %t},\n", m.Name, k != desc)
		}
	}
	c.w("\t}\n}\n\n")
}

// emitSingletonMethodTable lists a class object's own public methods (`def self.x`), and Kernel's module functions
// (each Kernel method is callable as `Kernel.x`), for Module#singleton_methods and public_methods(false) (decision 181).
func (c *Compiler) emitSingletonMethodTable(cls, desc *Class) {
	c.w("func (self *%s) _SingletonMethods() []rbConst {\n\treturn []rbConst{\n", cls.Name)
	seen := map[string]bool{}
	add := func(ms []*Method, priv bool) {
		for _, m := range ms {
			if (m.Private && !priv) || m.Node == nil || m.Name == "initialize" || strings.HasPrefix(m.Name, "__") || seen[m.Name] { // m.Node nil: generated (`new`, `name`)
				continue
			}
			seen[m.Name] = true
			c.w("\t\t{%q, nil, false},\n", m.Name)
		}
	}
	add(cls.MethodList, false)
	if desc != nil && desc.RubyName == "Kernel" {
		add(desc.MethodList, true)
	}
	c.w("\t}\n}\n\n")
}

// emitPrivateMethodTable is emitMethodTable's private half, for Module#private_instance_methods only (decision 181).
func (c *Compiler) emitPrivateMethodTable(cls, desc *Class) {
	if desc == nil {
		c.w("func (self *%s) _PrivateMethods() []rbConst { return nil }\n\n", cls.Name)
		return
	}
	c.w("func (self *%s) _PrivateMethods() []rbConst {\n\treturn []rbConst{\n", cls.Name)
	seen := map[string]bool{}
	for _, k := range desc.ancestors() {
		for _, m := range k.MethodList {
			if seen[m.Name] {
				continue
			}
			seen[m.Name] = true // a public redefinition hides an ancestor's private one
			if m.Private && m.Name != "initialize" && !strings.HasPrefix(m.Name, "__") {
				c.w("\t\t{%q, nil, %t},\n", m.Name, k != desc)
			}
		}
	}
	c.w("\t}\n}\n\n")
}

// isInstanceTest is Module#=== for desc's class object, over an untyped v:
// the run-time half of is_a? (decision 76), a lookup in the generated
// ancestry table by class ID (decision 82).
func (c *Compiler) isInstanceTest(desc *Class) string {
	switch {
	case desc == nil:
		return "false"
	case desc.universal:
		return "true"
	}
	return fmt.Sprintf("rbKindOf(v, %d)", c.classID(desc))
}

// emitClassMeta gives every Go type that holds a Ruby value its class ID,
// and the program the tables they index: names, and ancestors (the class,
// its superclasses, and every module they include). Class names, is_a? on a
// class value and Module#=== read these instead of reflect (decision 82).
// Each piece is kept only where selected (decision 49).
func (c *Compiler) emitClassMeta() {
	id := func(name string) int { return c.classID(c.classes[name]) }
	c.w("const (\n\trbNilClassID = %d\n\trbProcClassID = %d\n)\n\n", id("NilClass"), id("Proc"))
	names := make([]string, 0, len(c.classList))
	ancestry := make([]string, 0, len(c.classList))
	refs := make([]string, 0, len(c.classList))
	kernelInspect := make([]string, 0, len(c.classList))
	for _, cls := range c.classList {
		e := cls.lookup("inspect") // Kernel's: an object's inspect carries a container's rbSeen into its ivars
		kernelInspect = append(kernelInspect, strconv.FormatBool((cls.isStruct() || cls.RubyName == "Object") && !cls.IsModule && e != nil && e.Owner == c.classes["Kernel"]))
		names = append(names, strconv.Quote(cls.displayName()))
		var ids []string
		for _, k := range c.classList {
			if !k.universal && cls.isSubclassOf(k) {
				ids = append(ids, strconv.Itoa(c.classID(k)))
			}
		}
		ancestry = append(ancestry, "{"+strings.Join(ids, ", ")+"}")
		refs = append(refs, strconv.FormatBool(c.emitClassID(cls)))
	}
	c.w("var rbClassNames = [...]string{%s}\n\n", strings.Join(names, ", "))
	c.w("var rbAncestry = [...][]int{%s}\n\n", strings.Join(ancestry, ", "))
	// a table, not a marker method: a `_Ref()` on every pointer class was 11k of the test programs' functions (decision 89)
	c.w("var rbClassRefs = [...]bool{%s}\n\n", strings.Join(refs, ", "))
	c.w("var rbKernelInspect = [...]bool{%s}\n\n", strings.Join(kernelInspect, ", "))
	array := c.classID(c.classes["Array"])
	for n := 2; n <= maxTuple; n++ {
		if c.tupleN[n] {
			tps := make([]string, n)
			for i := range n {
				tps[i] = fmt.Sprintf("T%d", i)
			}
			c.w("func (Tuple%d[%s]) _ClassID() int { return %d }\n\n", n, strings.Join(tps, ", "), array)
		}
	}
	// func types carry no methods: a type switch over those the program renders
	c.w("func rbIsProc(a any) bool {\n")
	if procs := slices.Sorted(maps.Keys(c.procTypes)); len(procs) > 0 {
		c.w("\tswitch a.(type) {\n")
		for _, p := range procs { // one case each, so the pruner can drop the unused (decision 49)
			c.w("\tcase %s:\n\t\treturn true\n", p)
		}
		c.w("\tdefault: // gocritic rejects the one-case switch pruning can leave\n\t\treturn false\n\t}\n}\n\n")
		c.emitDynProcCall(procs)
		return
	}
	c.w("\treturn false\n}\n\n")
	c.emitDynProcCall(nil)
}

// emitDynProcCall emits rbDynProcCall: `call` on a Proc held untyped, switching over the program's Proc types (amends decision 47).
func (c *Compiler) emitDynProcCall(procs []string) {
	c.w("func rbDynProcCall(recv any, args []any) (any, bool) {\n")
	c.w("\tswitch p := recv.(type) {\n")
	for _, p := range procs {
		t := c.procFuncs[p]
		if t.Self == nil && t.Rest && len(t.Params) == 1 && isAny(t.Params[0]) && isAny(t.Ret) { // a gem's `(*untyped)` block (decision 250)
			c.w("\tcase %s:\n\t\treturn (*p)(args...), true\n", p)
			continue
		}
		if t.Self != nil || slices.ContainsFunc(t.Params, isVoid) || t.Rest {
			continue
		}
		args := make([]string, len(t.Params))
		for i, pt := range t.Params {
			args[i] = c.dynArg(pt, i)
		}
		call := "(*p)(" + strings.Join(args, ", ") + ")"
		c.w("\tcase %s:\n\t\trbArity(len(args), %d, %d)\n", p, len(t.Params), len(t.Params))
		switch {
		case isVoid(t.Ret):
			c.w("\t\t%s\n\t\treturn nil, true\n", call)
		case isOpt(t.Ret):
			c.w("\t\treturn Opt(%s), true\n", call)
		default:
			c.w("\t\treturn %s, true\n", call)
		}
	}
	c.w("\tdefault: // gocritic rejects the one-case switch pruning can leave\n\t\treturn nil, false\n\t}\n}\n\n")
}

// emitClassID emits cls's _ClassID and reports whether its values are pointers, whose address is their identity (rbClassRefs: #inspect, object_id).
func (c *Compiler) emitClassID(cls *Class) bool {
	if cls.universal && !cls.IsModule { // Object.new, BasicObject.new and main: heap objects of their own (#56)
		c.w("func (*%s) _ClassID() int { return %d }\n\n", cls.Name, c.classID(cls))
		c.w("func (self *%s) _Dup() any { dup := *self; return &dup }\n\n", cls.Name)
		return true
	}
	if cls.IsModule || cls.universal || cls.GoType == "" && !cls.isStruct() {
		return false
	}
	recv := c.recvType(cls)
	if cls == c.classes["Boolean"] { // a metaclass shares its class's RubyName, so compare the class itself
		c.w("func (self Boolean) _ClassID() int {\n\tif self {\n\t\treturn %d\n\t}\n\treturn %d\n}\n\n",
			c.classID(c.classes["TrueClass"]), c.classID(c.classes["FalseClass"]))
		return false
	}
	c.w("func (self %s) _ClassID() int { return %d }\n\n", recv, c.classID(cls))
	return strings.HasPrefix(recv, "*")
}

// classID is cls's index in the class list: its ID in the generated tables.
func (c *Compiler) classID(cls *Class) int {
	if cls == nil {
		return -1
	}
	if c.classIDs == nil {
		c.classIDs = make(map[*Class]int, len(c.classList))
		for i, k := range c.classList {
			c.classIDs[k] = i
		}
	}
	return c.classIDs[cls]
}

// noteArgBoxes records a T? box for each concrete type argument: generic code may hold E? (*E) of it.
func (c *Compiler) noteArgBoxes(args []Type) {
	for _, a := range args {
		var vars []string
		if freeVars(a, &vars); len(vars) > 0 {
			continue
		}
		switch a.(type) {
		case TFunc, TAny, TVoid, TUnion: // a union is any: E? of it is a member set, not a box (decision 150)
			continue
		case TClass, TNil, TOpt, TTuple, TVar: // a concrete argument type: boxed below (nil is skipped there)
		}
		if isNil(a) {
			continue
		}
		c.argBoxes["*"+c.goType(a)] = isOpt(a)
	}
}

// blkCall calls the closure form of a seq-adapted method: a Go method, or on a @go_type subclass (whose own methods
// are free funcs, decision 202) its free func.
func (c *Compiler) blkCall(cls *Class, e entry, args string) string {
	if cls.GoType != "" && e.Owner == cls && !c.isDirectMethod(e.M) {
		return freeFuncName(e.M) + c.forwardTypeArgs(e, cls) + "(self" + comma(args) + ")"
	}
	return "self." + e.M.GoName + "(" + args + ")"
}

// slotSeqAdapter emits the iterator adapter of a gem's closure override typed apart from the inherited iterator it
// answers (`tsort_each_child(node)` yielding Strings, where TSort's Node is untyped): the slot's signature, converting
// across (decision 206). False when there is no such slot.
func (c *Compiler) slotSeqAdapter(cls *Class, e entry, env map[string]Type, recv string) bool {
	m := e.M
	if !c.gemFile(m.File) || len(m.Block.Params) != 1 || slices.ContainsFunc(m.Params, func(p Param) bool { return p.Rest || p.Keyword }) {
		return false
	}
	for _, s := range c.parentSlots(cls, m) {
		sm := s.e.M
		if !sm.Iterator || sm.Block == nil || len(sm.Block.Params) != 1 || len(sm.Params) != len(m.Params) {
			continue
		}
		senv := c.slotEnv(s)
		sps, sret := c.sig(sm, senv)
		_, ps, ret := c.seqAdapterSig(m, env)
		if sps == ps && sret == ret {
			return false
		}
		conv := func(code, from, to string) string {
			if from == to || to == "any" {
				return code
			}
			return fmt.Sprintf("rbAs[%s](any(%s), %q)", to, code, to)
		}
		var args []string
		for i, p := range m.Params {
			args = append(args, conv(goParamName(sm.Params[i].Name, i), c.goType(subst(sm.Params[i].Type, senv)), c.goType(subst(p.Type, env))))
		}
		own, slotElem := c.goType(subst(m.Block.Params[0], env)), c.goType(subst(sm.Block.Params[0], senv))
		blk := fmt.Sprintf("func(x %s) { y(%s) }", own, conv("x", own, slotElem))
		if r := subst(m.Block.Ret, env); !isVoid(r) {
			blk = fmt.Sprintf("func(x %s) %s { y(%s); return *new(%s) }", own, c.goType(r), conv("x", own, slotElem), c.goType(r))
		}
		args = append(args, blk)
		c.w("func (self %s) %s(%s) %s {\n\treturn rbSeq(func(y func(%s)) { %s })\n}\n",
			recv, sm.GoName, sps, sret, slotElem, c.blkCall(cls, e, strings.Join(args, ", ")))
		return true
	}
	return false
}

// frozenField is the root struct's Kernel#freeze flag: lowercase with a trailing `_`, never an ivar's Go name.
const frozenField = "frozen_"

// objFreeze reports whether user or gem code names freeze at all: only then does a struct class carry the flag, so a
// program that never freezes its objects pays no byte per object for it (decision 221).
func (c *Compiler) objFreeze() bool {
	if c.objFreezeMemo == 0 {
		c.objFreezeMemo = 1
		for _, uf := range c.userFiles {
			if bytes.Contains(uf.Src, []byte("freeze")) { // ponytail: a textual scan; a call-site index would also skip comments
				c.objFreezeMemo = 2
				break
			}
		}
	}
	return c.objFreezeMemo == 2
}

// emitFrozenFlag emits rbObjFrozenFlag: the frozen flag of a struct object, by its concrete class (a switch Go compiles
// to a search on the type hash), or nil for anything else, which Kernel#freeze leaves as it is.
func (c *Compiler) emitFrozenFlag() {
	c.w("func rbObjFrozenFlag(a any) *bool {\n")
	var cases []string
	if c.objFreeze() {
		for _, cls := range c.classList {
			if cls.isStruct() && !cls.universal && !cls.IsModule && len(cls.TypeParams) == 0 && len(cls.root().TypeParams) == 0 && !cls.hashBase {
				cases = append(cases, fmt.Sprintf("\tcase *%s:\n\t\treturn &v.%s\n", cls.Name, frozenField))
			}
		}
	}
	if len(cases) > 0 {
		c.w("\tswitch v := a.(type) {\n%s\t}\n", strings.Join(cases, ""))
	}
	c.w("\treturn nil\n}\n\n")
}

// emitModIsA emits rbHasIsA_Mod for each module runtimeModuleIsA tested an untyped value against: a class-ID switch
// over the classes that include it (decision 228).
func (c *Compiler) emitModIsA() {
	for _, mod := range c.classList {
		if !c.modIsA[mod] {
			continue
		}
		var classes []*Class
		for _, k := range c.classList {
			if !k.IsModule && !k.universal && slices.Contains(k.allAncestors(), mod) {
				classes = append(classes, k)
			}
		}
		c.emitMarkers("IsA_"+mod.Name, classes)
	}
}

// dynAdapterBody is a dynamic slot's body (decision 249): its arguments boxed, its block an rbBlockArg, the override
// called through the dispatcher, the result converted to the slot's return type (TypeError when it is not one).
func (c *Compiler) dynAdapterBody(cls *Class, e entry, s slot) string {
	f := c.newFctx(cls.File, cls, nil)
	f.pass = 2
	f.indent = 1
	env := c.slotEnv(s)
	sm := s.e.M
	var vals []string
	rest := ""
	for i, p := range sm.Params {
		t := subst(p.Type, env)
		if p.Rest {
			rest = t.String()
			f.emit("args_ := make([]any, 0, %d+len(rest_))", len(sm.Params))
			continue
		}
		vals = append(vals, f.coerce(nil, expr{code: goParamName(p.Name, i), typ: t}, TAny{}))
	}
	if rest == "" {
		f.emit("args_ := make([]any, 0, %d)", len(sm.Params)+1)
	}
	if len(vals) > 0 {
		f.emit("args_ = append(args_, %s)", strings.Join(vals, ", "))
	}
	for _, p := range sm.Params {
		if p.Rest {
			f.emit("for _, x := range rest_ {")
			f.emit("\targs_ = append(args_, %s)", f.coerce(nil, expr{code: "x", typ: subst(p.Type, env)}, TAny{}))
			f.emit("}")
		}
	}
	if sm.Block != nil {
		fn := "blk"
		if !anyRestBlock(sm.Block) { // the slot's typed block as an untyped one: each value asserted in, its result boxed out
			bargs := make([]string, len(sm.Block.Params))
			for i, p := range sm.Block.Params {
				bargs[i] = f.coerce(nil, expr{code: fmt.Sprintf("rbArgAt(xs, %d)", i), typ: TAny{}}, subst(p, env))
			}
			call := "blk(" + strings.Join(bargs, ", ") + ")"
			if bret := subst(sm.Block.Ret, env); isVoid(bret) {
				fn = "func(xs ...any) any { " + call + "; return nil }"
			} else {
				fn = "func(xs ...any) any { return " + f.coerce(nil, expr{code: call, typ: bret}, TAny{}) + " }"
			}
		}
		f.emit("if blk != nil {")
		f.emit("\targs_ = append(args_, rbBlockArg{fn: %s})", fn)
		f.emit("}")
	}
	c.noteDyn(e.M.Name)
	call := "rbDyn" + goMethodName(e.M.Name) + "(rbFCall, self, args_...)" // a slot is self's own call: private methods answer
	ret := subst(sm.Ret, env)
	if isVoid(ret) {
		f.emit("%s", call)
	} else {
		f.emit("return %s", f.coerce(nil, expr{code: call, typ: TAny{}}, ret))
	}
	return f.buf.String()
}
