package compiler

import (
	"fmt"
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
		return fmt.Sprintf("Tuple%d[%s]", len(t.Elems), c.goTypes(t.Elems))
	case TVar:
		return t.Name
	case TFunc:
		s := "func(" + c.goTypes(t.Params) + ")"
		if !isVoid(t.Ret) {
			s += " " + c.goType(t.Ret)
		}
		if t.Proc {
			var vars []string
			if freeVars(t, &vars); len(vars) == 0 {
				c.procTypes["*"+s] = true
			}
			return "*" + s
		}
		return s
	case TAny, TNil:
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
	return c.goType(TFunc{Params: substAll(b.Params, env), Ret: subst(b.Ret, env)})
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
	for _, p := range m.Params {
		name := goLocalName(p.Name)
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
	for _, p := range m.Params {
		name := goLocalName(p.Name)
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
func freeFuncName(m *Method) string { return m.Owner.Name + "_" + m.GoName }

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

// typeParamDecl renders the type parameter list of a free func.
func (c *Compiler) typeParamDecl(m *Method) string {
	var tps []string
	if m.Owner.GoType == "" {
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
	c.guardConsts()
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
	if c.tablesOut && c.tableInputs() != c.tablesAt {
		return nil, errPruneIncomplete
	}
	if c.out.Len() == 0 {
		if c.tablesOut {
			return nil, nil
		}
		c.tablesOut = true
		c.tablesAt = c.tableInputs()
		c.emitTables()
	}
	return []byte(c.out.String()), nil
}

// tableInputs fingerprints what emitTables reads; the maps only grow, so sizes tell.
func (c *Compiler) tableInputs() [8]int {
	b2i := func(b bool) int {
		if b {
			return 1
		}
		return 0
	}
	return [8]int{len(c.tupleN), len(c.argBoxes), len(c.boxes), len(c.procTypes), len(c.regexps), len(c.strLits), b2i(c.classOf), b2i(c.dynEvery)}
}

// emitTables: what every emitted body has contributed to.
func (c *Compiler) emitTables() {
	if c.dynEvery {
		c.emitNameSwitches()
		c.emitCallTables()
	}
	c.emitClassOf()
	c.emitTuples()
	c.emitBoxes()
	c.emitClassMeta()
	// last: every body, main included, has registered its literals by now
	for _, r := range c.regexps {
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
			c.w("type %s struct{}\n\n", cls.Name)
		}
	case cls.IsModule:
		c.emitModuleInterface(cls)
	case cls.GoType != "":
		tp := ""
		if len(cls.TypeParams) > 0 {
			tp = "[" + strings.Join(cls.TypeParams, ", ") + " comparable]"
		}
		c.w("type %s%s %s\n\n", cls.Name, tp, cls.GoType)
	default:
		c.emitStructClass(cls)
	}
}

// publicEntries lists the entries that make up a class's Go interface:
// public, non-generic methods.
func (c *Compiler) publicEntries(cls *Class) []entry {
	var out []entry
	for _, e := range cls.methodSet() {
		if e.M.Private || e.M.generic() || e.M.Name == "initialize" {
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
	c.w("type %s_Self[Self any%s] interface {\n", mod.Name, tps)
	// Constraint = what the module's bodies call on self (docs/design.md).
	called := c.selfCalls(mod)
	for _, e := range c.publicEntries(mod) {
		if !called[e.M.Name] {
			continue
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
			if called[e.M.Name] && mod.lookup(e.M.Name) == nil && c.topDefs[e.M.Name] == nil {
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
		default:
			body = fmt.Sprintf("panic(NewNoMethodError(Ref(String(\"super: no superclass method '%s' for \" + rbDescribe(self)))))", m.Name)
		}
		c.w("func (self %s) %s(%s) %s { %s }\n", recv, bridgeName(m), ps, ret, body)
	}
}

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
	for _, m := range mod.MethodList {
		if m.Node != nil {
			walk(m.Node.Body)
			if m.Node.Parameters != nil {
				walk(m.Node.Parameters) // defaults run in the callee
			}
		}
	}
	for _, inc := range mod.Includes {
		for k := range c.selfCalls(inc.Mod) {
			out[k] = true
		}
	}
	mod.selfCallCache = out
	return out
}

func (c *Compiler) emitStructClass(cls *Class) {
	if len(cls.Subclasses) > 0 && !cls.universal {
		c.w("type %s = %s\n\n", superField(cls), cls.Name)
	}
	c.userTypeLine(cls)
	c.w("type %s struct {\n", cls.Name)
	if cls.Super != nil && !cls.Super.universal {
		c.w("\t%s\n", superField(cls.Super))
	} else if len(cls.IvarList) == 0 {
		// zero-size allocations may share an address, merging distinct instances' identity
		c.w("\t_ byte\n")
	}
	for _, iv := range cls.IvarList {
		c.w("\t%s %s\n", goFieldName(iv.Name), c.goType(iv.Type))
	}
	c.w("}\n\n")
	// interface
	c.w("type %sI interface {\n", cls.Name)
	for _, k := range cls.structChain() {
		c.w("\t_%s() *%s\n", k.Name, k.Name)
	}
	for _, e := range c.publicEntries(cls) {
		if cls.metaOf != nil && e.M.Name == "new" {
			// Subclasses may take different initialize arguments, so `new`
			// is not part of the shared class-object interface; calls
			// through `singleton(C)` assert for it instead.
			continue
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
	}
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
	if cls.meta != nil {
		c.w("func (self *%s) _ClassOf() %s { return %s }\n\n", cls.Name, c.goType(TClass{C: cls.root().meta}), classVar(cls))
	}
	if isModule {
		c.emitConstTable(cls)
	}
	if cls.metaOf != nil {
		// a metaclass has exactly one instance: the class object
		c.w("var %s = &%s{}\n\n", classVar(cls.metaOf), cls.Name)
		return
	}
	c.emitIvarList(cls)
	c.emitCopy(cls)
	// constructor
	init := cls.lookup("initialize")
	if init != nil {
		env := composeEnv(init.Env, nil)
		env["Self"] = TClass{C: cls}
		ps, _ := c.sig(init.M, env)
		c.labels["New"+cls.Name] = "Class#new"
		c.w("func New%s(%s) *%s {\n\tself := &%s{}\n\t%s(self, %s)\n\treturn self\n}\n\n",
			cls.Name, ps, cls.Name, cls.Name, freeFuncName(init.M), c.argNames(init.M))
	} else {
		c.w("func New%s() *%s { return &%s{} }\n\n", cls.Name, cls.Name, cls.Name)
	}
}

// superField names the field a subclass embeds cls through: an alias, since
// an embedded field takes its type's name and a method of that Go name
// (`Calc#calc`) would clash with it. Lowercase with a trailing `_`, it is
// neither a method nor an ivar name (goLocalName).
func superField(cls *Class) string { return "super_" + cls.Name + "_" }

// emitIvarList feeds Kernel#inspect; a class adding no ivars inherits its parent's through embedding.
func (c *Compiler) emitIvarList(cls *Class) {
	if len(cls.IvarList) == 0 {
		return
	}
	var ivs []string
	for _, iv := range c.ivarOrder(cls) {
		field := "self." + goFieldName(iv.Name)
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
		case TAny, TNil, TTuple, TVoid: // held by value: never a nil pointer of its own
		}
		ivs = append(ivs, fmt.Sprintf("{%q, %s, %t, %s}", iv.Name, val, opt, isNilCode))
	}
	c.w("func (self *%s) _Ivars() []rbIvar { return []rbIvar{%s} }\n\n", cls.Name, strings.Join(ivs, ", "))
	// Kernel#instance_variable_set: the closed world knows every ivar's type, so a write converts to it
	c.w("func (self *%s) _IvarSet(name string, v any) bool {\n\tswitch name {\n", cls.Name)
	for _, iv := range c.ivarOrder(cls) {
		field := "&self." + goFieldName(iv.Name)
		if o, ok := iv.Type.(TOpt); ok {
			c.w("\tcase %q:\n\t\trbIvarAssignOpt[%s](%s, v, %q)\n", iv.Name, c.goType(o.Elem), field, iv.Name)
			continue
		}
		c.w("\tcase %q:\n\t\trbIvarAssign(%s, v, %q)\n", iv.Name, field, iv.Name)
	}
	c.w("\tdefault:\n\t\treturn false\n\t}\n\treturn true\n}\n\n")
}

// emitCopy feeds Ractor's deep copy (decision 103): a fresh struct with every
// ivar copied through rbCopyAs, identity kept through seen. Pruned with
// rbCopyDeep when no Ractor is used.
func (c *Compiler) emitCopy(cls *Class) {
	c.w("func (self *%s) _Copy(seen map[any]any) any {\n\tif self == nil {\n\t\treturn self\n\t}\n\tdup := *self\n\tseen[self] = &dup\n", cls.Name)
	for _, iv := range c.ivarOrder(cls) {
		field := goFieldName(iv.Name)
		c.w("\tdup.%s = rbCopyAs(self.%s, seen)\n", field, field)
	}
	c.w("\treturn &dup\n}\n\n")
}

// ivarOrder approximates MRI's (first assignment) with initialize's write order, super splicing in the parent's.
func (c *Compiler) ivarOrder(cls *Class) []*Ivar {
	var out []*Ivar
	seen := map[*Ivar]bool{}
	add := func(iv *Ivar) {
		if iv != nil && !seen[iv] {
			seen[iv] = true
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
	return out
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
		if m.Kind == kindAttrReader || m.Kind == kindAttrWriter {
			continue // promoted through embedding; never touches virtual self
		}
		if !c.wantsForwarder(cls, e) {
			continue
		}
		env := composeEnv(e.Env, nil)
		env["Self"] = c.selfTypeFor(e, cls)
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
		env := composeEnv(e.Env, nil)
		env["Self"] = c.selfTypeFor(e, cls)
		name, ps, ret := c.seqAdapterSig(e.M, env)
		seq := "rbSeq"
		if len(e.M.Block.Params) == 2 {
			seq = "rbSeq2"
		}
		c.w("func (self %s) %s(%s) %s {\n\treturn %s(func(blk %s) { self.%s(%s) })\n}\n",
			recv, name, ps, ret, seq, c.blockGoType(e.M.Block, env), e.M.GoName, c.argNames(e.M))
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
	env := composeEnv(e.Env, nil)
	env["Self"] = c.selfTypeFor(*e, cls)
	ps, _ := c.sig(e.M, env)
	arg := c.argNames(e.M)
	c.w("func (self %s) Op_cmp(%s) Integer {\n\tif r := self.cmpNil(%s); r != nil {\n\t\treturn *r\n\t}\n\tpanic(rbCmpErr(self, %s))\n}\n\n",
		recv, ps, arg, arg)
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
	return "(" + strings.Join(ps, ", ") + ") -> " + subst(s.e.M.Ret, env).String()
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
	case isVoid(from):
		return false
	case c.goType(from) == c.goType(to) || isAny(to):
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
	if m.Block != nil || sm.Block != nil {
		fail("a block-taking override must keep the signature")
	}
	for _, p := range sm.Params {
		if p.Rest {
			fail("an override of a rest-parameter method must keep the signature")
		}
	}
	req, maxArgs := arity(m)
	if n := len(sm.Params); n < req || maxArgs >= 0 && n > maxArgs {
		return // the adapter raises ArgumentError, as MRI would
	}
	env, oenv := c.slotEnv(s), c.slotEnv(own)
	for i, p := range sm.Params {
		from, to := subst(p.Type, env), subst(paramAt(m, i).Type, oenv)
		if !c.adapts(from, to, true) {
			fail(fmt.Sprintf("argument %d cannot pass a %s as a %s", i+1, from, to))
		}
	}
	from, to := subst(m.Ret, oenv), subst(sm.Ret, env)
	if !c.adapts(from, to, false) {
		fail(fmt.Sprintf("its %s result cannot stand in for %s", from, to))
	}
}

// adapterBody answers slot s of an ancestor's interface with cls's method e
// (checked by checkAdaptable): an argument count e cannot take raises MRI's
// ArgumentError, anything else converts across the two signatures.
func (c *Compiler) adapterBody(cls *Class, e entry, s slot) string {
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
		a := expr{code: goLocalName(p.Name), typ: subst(p.Type, env)}
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

// seqAdapterSig renders the iter.Seq adapter a closure override of an
// iterator answers to under the iterator's Go name (decision 4).
func (c *Compiler) seqAdapterSig(m *Method, env map[string]Type) (name, params, ret string) {
	it := *m
	it.Iterator = true
	params, ret = c.sig(&it, env)
	return goMethodName(m.Name), params, ret
}

// forwardTypeArgs renders explicit type args for a forwarder call.
func (c *Compiler) forwardTypeArgs(e entry, cls *Class) string {
	var args []string
	switch {
	case e.Owner.GoType != "":
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

func (c *Compiler) emitTuples() {
	for n := 2; n <= 3; n++ {
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
		c.w("func (t %s) _ToAny() *Array[any] { return &Array[any]{%s} }\n\n", full, strings.Join(vals, ", "))
		// and converts back where a dynamic call's parameter is a tuple (rbAs)
		c.w("func (%s) _FromAny(a any) (t %s, ok bool) {\n\tarr, ok := a.(Array_Any)\n\tif !ok {\n\t\treturn t, false\n\t}\n\ts := *arr._ToAny()\n\tif len(s) != %d {\n\t\treturn t, false\n\t}\n%s\n\treturn t, true\n}\n\n", full, full, n, strings.Join(from, "\n"))
		c.w("func (t %s) Op_eq(o any) Boolean {\n\to2, ok := o.(%s)\n\tif !ok {\n\t\tif a, isArr := o.(Array_Any); isArr {\n\t\t\treturn t._ToAny().Op_eq(a._ToAny())\n\t\t}\n\t\treturn false\n\t}\n\treturn %s\n}\n\n", full, full, strings.Join(eq, " && "))
		c.w("func (t %s) rbPlain() bool { return %s }\n\n", full, strings.Join(plain, " && "))
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
}

// wantsForwarder decides whether class cls gets a Go method forwarding to
// the free func of entry e. Struct classes forward everything (their
// interface is the full method set). Primitive classes only forward what a
// module's constraint needs: a forwarder on a generic container can
// instantiate ever-growing types (Hash[K,V].Tally → Hash[[K,V],Integer]...),
// which Go rejects as an instantiation cycle.
func (c *Compiler) wantsForwarder(cls *Class, e entry) bool {
	if e.Owner == cls {
		return true
	}
	if cls.isStruct() {
		return true
	}
	if e.Owner.universal {
		return c.includerCalls(cls, e.M.Name)
	}
	return e.Owner.IsModule && c.selfCalls(e.Owner)[e.M.Name]
}

// includerCalls: a module method calling name on self needs cls to answer it.
func (c *Compiler) includerCalls(cls *Class, name string) bool {
	for k := cls; k != nil && !k.universal; k = k.Super {
		for _, inc := range k.Includes {
			if c.selfCalls(inc.Mod)[name] {
				return true
			}
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
func (c *Compiler) constType(k *Const) Type {
	if k.Type != nil {
		return k.Type
	}
	if k.ann != "" {
		t, err := rbs.ParseType(k.ann)
		if err != nil {
			c.errorf(k.File, k.Value, "%v", err)
		}
		k.Type = c.resolveType(t, typeScope{lex: k.Scope, file: k.File, line: k.Line})
		return k.Type
	}
	if k.resolving {
		c.errorf(k.File, k.Value, "constant %s depends on itself; annotate it with `#: T`", k.RubyName)
	}
	k.resolving = true
	f := c.constFctx(k)
	var e expr
	f.probe(func() { e = f.genExpr(k.Value, nil) })
	k.resolving = false
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
	for _, cls := range c.classList {
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
	array := c.classID(c.classes["Array"])
	for n := 2; n <= 3; n++ {
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
		c.w("\t}\n")
	}
	c.w("\treturn false\n}\n\n")
}

// emitClassID emits cls's _ClassID and reports whether its values are pointers, whose address is their identity (rbClassRefs: #inspect, object_id).
func (c *Compiler) emitClassID(cls *Class) bool {
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
		case TFunc, TAny, TVoid:
			continue
		case TClass, TNil, TOpt, TTuple, TVar: // a concrete argument type: boxed below (nil is skipped there)
		}
		if isNil(a) {
			continue
		}
		c.argBoxes["*"+c.goType(a)] = isOpt(a)
	}
}
