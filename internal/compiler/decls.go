package compiler

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/danielgatis/go-ruby-prism/parser"

	"rb2go/internal/rbs"
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
			}
			if cls.mutable() {
				return "*" + name
			}
			return name
		default:
			return cls.Name + "I"
		}
	case TOpt:
		return "*" + c.goType(t.Elem)
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
	case 1:
		return "iter.Seq[" + c.goType(ps[0]) + "]"
	case 2:
		return "iter.Seq2[" + c.goTypes(ps) + "]"
	}
	panic(compileError{msg: "iterator blocks must yield 1 or 2 values"})
}

// sig renders the Go parameter list and result of method m under env.
func (c *Compiler) sig(m *Method, env map[string]Type) (params string, ret string) {
	var ps []string
	if m.calleeDefaults {
		ps = append(ps, "rbArgc int")
	}
	for _, p := range m.Params {
		name := goLocalName(p.Name)
		if p.Rest {
			ps = append(ps, name+"_ ..."+c.goType(subst(p.Type, env)))
			continue
		}
		ps = append(ps, name+" "+c.goType(subst(p.Type, env)))
	}
	if m.Block != nil && !m.Iterator {
		ps = append(ps, "blk "+c.blockGoType(m.Block, env))
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
	if m.calleeDefaults {
		as = append(as, "rbArgc")
	}
	for _, p := range m.Params {
		name := goLocalName(p.Name)
		if p.Rest {
			as = append(as, name+"_...")
			continue
		}
		as = append(as, name)
	}
	if m.Block != nil && !m.Iterator {
		as = append(as, "blk")
	}
	return strings.Join(as, ", ")
}

// freeFuncName is the Go name of a method emitted as a free function.
func freeFuncName(m *Method) string { return m.Owner.Name + "_" + m.GoName }

// isDirectMethod reports whether m is emitted as a plain Go method on its
// owner (primitive classes' own non-generic methods, attr accessors).
func (c *Compiler) isDirectMethod(m *Method) bool {
	if m.Owner == nil {
		return false
	}
	if m.Kind == kindAttrReader || m.Kind == kindAttrWriter || m.Kind == kindSynth {
		return true
	}
	return m.Owner.GoType != "" && !m.generic()
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
	// Order matters for `//line` readability only; Go doesn't care.
	for _, v := range c.verbatim {
		c.lineDirective(v.file, v.line)
		c.w("%s\n\n", strings.TrimSpace(v.code))
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
	for _, cls := range classes {
		c.emitForwarders(cls)
	}
	for _, m := range c.topDefList {
		c.emitTopDef(m)
	}
	for _, k := range c.constList {
		c.emitConst(k)
	}

	c.emitRubyNames()
	c.emitMain()
	c.emitDynamic()
	c.emitClassOf()
	c.emitTuples()
	// last: every body, main included, has registered its literals by now
	for _, r := range c.regexps {
		c.w("%s\n\n", r)
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

func (c *Compiler) emitModuleInterface(mod *Class) {
	tps := ""
	if len(mod.TypeParams) > 0 {
		tps = ", " + strings.Join(mod.TypeParams, ", ") + " comparable"
	}
	c.w("type %s_Self[Self any%s] interface {\n", mod.Name, tps)
	// Constraint = what the module's bodies call on self (README).
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
	c.w("}\n\n")
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
	// struct
	c.w("type %s struct {\n", cls.Name)
	if cls.Super != nil && !cls.Super.universal {
		c.w("\t%s\n", cls.Super.Name)
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
	}
	if cls.meta != nil {
		c.w("\t_ClassOf() %s\n", c.goType(TClass{C: cls.root().meta}))
	}
	isModule := cls.isSubclassOf(c.classes["Module"])
	if isModule {
		c.w("\t_Consts() []rbConst\n\t_Kind() string\n")
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
	// constructor
	init := cls.lookup("initialize")
	if init != nil {
		env := composeEnv(init.Env, nil)
		env["Self"] = TClass{C: cls}
		ps, _ := c.sig(init.M, env)
		c.w("func New%s(%s) *%s {\n\tself := &%s{}\n\t%s(self, %s)\n\treturn self\n}\n\n",
			cls.Name, ps, cls.Name, cls.Name, freeFuncName(init.M), c.argNames(init.M))
	} else {
		c.w("func New%s() *%s { return &%s{} }\n\n", cls.Name, cls.Name, cls.Name)
	}
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
	c.w("\n")
}

// forwardTypeArgs renders explicit type args for a forwarder call.
func (c *Compiler) forwardTypeArgs(e entry, cls *Class) string {
	var args []string
	if e.Owner.GoType == "" {
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
		tos := make([]string, 0, n)
		insp := make([]string, 0, n)
		eq := make([]string, 0, n)
		for i := range n {
			p := fmt.Sprintf("T%d", i)
			tps = append(tps, p)
			fields = append(fields, fmt.Sprintf("F%d %s", i, p))
			cmp = append(cmp, fmt.Sprintf("if c := rbCmp(t.F%d, o.F%d); c != 0 {\n\t\treturn c\n\t}", i, i))
			tos = append(tos, fmt.Sprintf("rbInspect(t.F%d)", i))
			insp = append(insp, fmt.Sprintf("rbInspect(t.F%d)", i))
			eq = append(eq, fmt.Sprintf("rbEq(t.F%d, o2.F%d)", i, i))
		}
		name := fmt.Sprintf("Tuple%d", n)
		c.w("type %s[%s comparable] struct {\n\t%s\n}\n\n", name, strings.Join(tps, ", "), strings.Join(fields, "\n\t"))
		full := fmt.Sprintf("%s[%s]", name, strings.Join(tps, ", "))
		c.w("func (t %s) Cmp(o %s) Integer {\n\t%s\n\treturn 0\n}\n\n", full, full, strings.Join(cmp, "\n\t"))
		c.w("func (t %s) Inspect() String { return \"[\" + %s + \"]\" }\n\n", full, strings.Join(insp, ` + ", " + `))
		c.w("func (t %s) ToS() String { return t.Inspect() }\n\n", full)
		json := make([]string, 0, n)
		for i := range n {
			json = append(json, fmt.Sprintf("rbToJson(t.F%d)", i))
		}
		c.w("func (t %s) ToJson(...any) String { return \"[\" + %s + \"]\" }\n\n", full, strings.Join(json, ` + "," + `))
		c.w("func (t %s) Eq(o any) Boolean {\n\to2, ok := o.(%s)\n\tif !ok {\n\t\treturn false\n\t}\n\treturn %s\n}\n\n", full, full, strings.Join(eq, " && "))
		_ = tos
	}
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
	return e.Owner.IsModule && !e.Owner.universal && c.selfCalls(e.Owner)[e.M.Name]
}

// constFctx is the codegen context a constant's initializer runs in.
func (c *Compiler) constFctx(k *Const) *fctx {
	f := c.newFctx(k.File, nil, nil)
	f.lex = k.Scope
	f.locals = map[string]*localInfo{}
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
}

// constEntries lists the constants a class object answers to: its own in
// definition order, then inherited ones (Object's are the top-level ones).
func (c *Compiler) constEntries(cls *Class) []string {
	if cls.RubyName == "Object" {
		return c.topConstNames
	}
	seen := map[string]bool{}
	var out []string
	for k := cls; k != nil && !k.universal; k = k.Super {
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
	if desc == nil {
		c.w("func (self *%s) _Consts() []rbConst { return nil }\n\n", cls.Name)
		return
	}
	c.w("func (self *%s) _Consts() []rbConst {\n\treturn []rbConst{\n", cls.Name)
	for _, full := range c.constEntries(desc) {
		if v, ok := c.constValue(full); ok {
			short := full[strings.LastIndex(full, ":")+1:]
			c.w("\t\t{%q, %s},\n", short, v)
		}
	}
	c.w("\t}\n}\n\n")
}

// emitRubyNames maps Go type names back to Ruby constant paths for the
// classes whose names differ (namespaced ones), for messages and #inspect.
func (c *Compiler) emitRubyNames() {
	c.w("var rbRubyNames = map[string]string{\n")
	for _, cls := range c.classList {
		if cls.Name != cls.RubyName {
			c.w("\t%q: %q,\n", cls.Name, cls.RubyName)
		}
	}
	c.w("}\n\n")
}
