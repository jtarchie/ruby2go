package compiler

import (
	"fmt"
	"slices"
	"strings"
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
			return cls.ifaceName()
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
	return m.Owner.freeFunc(m)
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
			if name := cls.freeFunc(m); types[name] {
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
		return owner.selfName() + "[Self" + comma(strings.Join(owner.TypeParams, ", ")) + "]"
	default:
		return owner.ifaceName()
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
	return owner.ifaceName()
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

// goZero is the zero value of Go type t as code: nil for any, which gocritic's newDeref rejects as `*new(any)`.
func goZero(t string) string {
	if t == "any" {
		return "nil"
	}
	return "*new(" + t + ")"
}
