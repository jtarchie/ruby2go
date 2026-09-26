package compiler

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/danielgatis/go-ruby-prism/parser"
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
	if m.Kind == kindAttrReader || m.Kind == kindAttrWriter {
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
		return owner.Name + "_Self[Self" + prefixed(", ", strings.Join(owner.TypeParams, ", ")) + "]"
	default:
		return owner.Name + "I"
	}
}

func prefixed(p, s string) string {
	if s == "" {
		return ""
	}
	return p + s
}

// typeParamDecl renders the type parameter list of a free func.
func (c *Compiler) typeParamDecl(m *Method) string {
	var tps []string
	if m.Owner.GoType == "" {
		tps = append(tps, "Self "+c.constraint(m.Owner))
	}
	var rest []string
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
	c.w("package main\n\n")
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
	c.emitMain()
	c.emitTuples()
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
		env := composeEnv(e.Env, nil)
		env["Self"] = c.selfTypeFor(e, cls)
		ps, ret := c.sig(e.M, env)
		c.w("\t%s(%s) %s\n", e.M.GoName, ps, ret)
	}
	c.w("}\n\n")
	c.w("func (self *%s) _%s() *%s { return self }\n\n", cls.Name, cls.Name, cls.Name)
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
		body := fmt.Sprintf("%s%s(self%s)", freeFuncName(m), targs, prefixed(", ", c.argNames(m)))
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
		var tps, fields, cmp, tos, insp, eq []string
		for i := 0; i < n; i++ {
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
