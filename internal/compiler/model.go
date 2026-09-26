package compiler

import (
	"fmt"
	"strings"

	"github.com/danielgatis/go-ruby-prism/parser"

	"rb2go/internal/rbs"
)

// Class is a Ruby class or module.
type Class struct {
	Name       string
	IsModule   bool
	GoType     string // `@go_type` underlying Go type; "" for struct classes
	TypeParams []string
	SuperName  string
	Super      *Class
	Includes   []Include
	Methods    map[string]*Method
	MethodList []*Method
	Ivars      map[string]*Ivar
	IvarList   []*Ivar
	File       *File
	Line       int
	Subclasses []*Class
	// universal classes (BasicObject, Object, Kernel) are ancestors of the
	// primitive types too, so their free funcs take `Self any`.
	universal bool
	// ivar type annotations `# @rbs @x: T`, resolved in resolveSigs
	ivarDecls     []ivarDecl
	msetCache     []entry
	selfCallCache map[string]bool
}

type ivarDecl struct {
	name string
	rbs  rbs.Type
	line int
}

// Include is `include Mod #[Args]`.
type Include struct {
	Mod  *Class
	Args []Type
	name string
	args []rbs.Type
	line int
	file *File
}

// Ivar is an instance variable of a struct class.
type Ivar struct {
	Name  string // with @
	Type  Type
	Owner *Class
}

type methodKind int

const (
	kindDef methodKind = iota
	kindPrimitive
	kindAttrReader
	kindAttrWriter
)

// Method is a Ruby method definition.
type Method struct {
	Name       string
	GoName     string
	Owner      *Class // nil for top-level defs
	Kind       methodKind
	Attr       string // ivar for attr kinds
	Private    bool
	Node       *parser.DefNode
	File       *File
	Line       int
	sigText    string
	sig        *rbs.MethodType
	TypeParams []string
	Params     []Param
	Block      *BlockSig
	Ret        Type
	Iterator   bool // block returns void → iter.Seq
	resolved   bool
	inherited  *Method // signature source for unannotated overrides
}

// Param is a positional parameter.
type Param struct {
	Name    string
	Type    Type
	Default parser.Node // literal default for optional params
	Rest    bool
}

// BlockSig is the block a method takes.
type BlockSig struct {
	Params []Type
	Ret    Type
}

func (m *Method) generic() bool { return len(m.TypeParams) > 0 }

// entry is a method as seen from a concrete class's method set.
type entry struct {
	M     *Method
	Owner *Class
	Env   map[string]Type // owner's type params → types in terms of the class's params
	Entry *Class          // class where the method entered the hierarchy (for Self)
}

func (c *Class) isSubclassOf(other *Class) bool {
	for k := c; k != nil; k = k.Super {
		if k == other {
			return true
		}
		for _, inc := range k.Includes {
			if inc.Mod == other || inc.Mod.isSubclassOf(other) {
				return true
			}
		}
	}
	return false
}

func (c *Class) isStruct() bool { return !c.IsModule && c.GoType == "" }

// mutable reports whether the @go_type is a reference-like Go type, in which
// case instances are handled as pointers.
func (c *Class) mutable() bool {
	g := c.GoType
	return strings.HasPrefix(g, "[]") || strings.HasPrefix(g, "map[") ||
		strings.HasPrefix(g, "struct") || strings.HasPrefix(g, "*")
}

func (c *Class) selfVar() TVar { return TVar{Name: "Self"} }

func (c *Class) paramTypes() []Type {
	out := make([]Type, len(c.TypeParams))
	for i, p := range c.TypeParams {
		out[i] = TVar{Name: p}
	}
	return out
}

// instance is the type of an instance with the class's own params as vars.
func (c *Class) instance() TClass { return TClass{C: c, Args: c.paramTypes()} }

// methodSet returns the full method set in lookup order: own methods,
// included modules (last include first), then the superclass chain.
func (c *Class) methodSet() []entry {
	if c.msetCache != nil {
		return c.msetCache
	}
	seen := map[string]bool{}
	var out []entry
	add := func(e entry) {
		if seen[e.M.Name] {
			return
		}
		seen[e.M.Name] = true
		out = append(out, e)
	}
	identity := map[string]Type{}
	for _, p := range c.TypeParams {
		identity[p] = TVar{Name: p}
	}
	for _, m := range c.MethodList {
		add(entry{M: m, Owner: c, Env: identity, Entry: c})
	}
	for i := len(c.Includes) - 1; i >= 0; i-- {
		inc := c.Includes[i]
		env := map[string]Type{}
		for j, p := range inc.Mod.TypeParams {
			if j < len(inc.Args) {
				env[p] = inc.Args[j]
			}
		}
		for _, e := range inc.Mod.methodSet() {
			e2 := entry{M: e.M, Owner: e.Owner, Env: composeEnv(e.Env, env), Entry: c}
			if c.IsModule {
				e2.Entry = e.Entry
			}
			add(e2)
		}
	}
	if c.Super != nil {
		for _, e := range c.Super.methodSet() {
			add(e)
		}
	}
	c.msetCache = out
	return out
}

func composeEnv(inner, outer map[string]Type) map[string]Type {
	out := map[string]Type{}
	for k, v := range inner {
		out[k] = subst(v, outer)
	}
	return out
}

func (c *Class) lookup(name string) *entry {
	for _, e := range c.methodSet() {
		if e.M.Name == name {
			e := e
			return &e
		}
	}
	return nil
}

// ancestors of a struct class up to (excluding) Object, nearest first.
func (c *Class) structChain() []*Class {
	var out []*Class
	for k := c; k != nil && !k.universal; k = k.Super {
		out = append(out, k)
	}
	return out
}

// ---- declaration collection

func (c *Compiler) collect(f *File) {
	for _, n := range f.Root.Statements.Body {
		switch n := n.(type) {
		case *parser.ClassNode:
			c.collectClass(f, n)
		case *parser.ModuleNode:
			c.collectModule(f, n)
		case *parser.DefNode:
			c.collectTopDef(f, n)
		case *parser.XStringNode:
			if !f.prelude {
				c.errorf(f, n, "top-level %%x{} is only allowed in the prelude")
			}
			c.verbatim = append(c.verbatim, verbatim{file: f, line: f.line(n.Location.StartOffset), code: n.Unescaped.Value})
		case *parser.CallNode:
			if n.Receiver == nil && n.Name == "require" || n.Name == "require_relative" {
				continue
			}
			c.mainStmts = append(c.mainStmts, n)
		default:
			c.mainStmts = append(c.mainStmts, n)
		}
	}
}

func (c *Compiler) declareClass(f *File, name string, line int, isModule bool) *Class {
	cls := c.classes[name]
	if cls == nil {
		cls = &Class{Name: name, IsModule: isModule, Methods: map[string]*Method{}, Ivars: map[string]*Ivar{}, File: f, Line: line}
		cls.universal = name == "BasicObject" || name == "Object" || name == "Kernel"
		c.classes[name] = cls
		c.classList = append(c.classList, cls)
	} else if cls.IsModule != isModule {
		c.errorf(f, nil, "%s:%d: %s is already defined as a %s", f.Name, line, name, map[bool]string{true: "module", false: "class"}[cls.IsModule])
	}
	ann := f.annotations(line)
	for _, g := range ann["generic"] {
		fields := strings.Fields(g)
		if len(fields) == 0 {
			continue
		}
		// `unchecked out E < Bound` → take the identifier
		for _, fld := range fields {
			if fld == "unchecked" || fld == "in" || fld == "out" {
				continue
			}
			cls.TypeParams = append(cls.TypeParams, fld)
			break
		}
	}
	if g := ann["go_type"]; len(g) > 0 {
		if isModule {
			c.errorf(f, nil, "%s:%d: @go_type on a module", f.Name, line)
		}
		cls.GoType = g[0]
	}
	return cls
}

func (c *Compiler) collectClass(f *File, n *parser.ClassNode) {
	if _, ok := n.ConstantPath.(*parser.ConstantReadNode); !ok {
		c.errorf(f, n, "unsupported class name %s", f.text(n.ConstantPath.GetLocation()))
	}
	line := f.line(n.Location.StartOffset)
	cls := c.declareClass(f, n.Name, line, false)
	if n.Superclass != nil {
		sup, ok := n.Superclass.(*parser.ConstantReadNode)
		if !ok {
			c.errorf(f, n.Superclass, "unsupported superclass expression")
		}
		if cls.SuperName != "" && cls.SuperName != sup.Name {
			c.errorf(f, n, "class %s reopened with a different superclass", n.Name)
		}
		cls.SuperName = sup.Name
	} else if cls.SuperName == "" && n.Name != "BasicObject" {
		cls.SuperName = "Object"
	}
	c.collectBody(f, cls, n.Body)
}

func (c *Compiler) collectModule(f *File, n *parser.ModuleNode) {
	if _, ok := n.ConstantPath.(*parser.ConstantReadNode); !ok {
		c.errorf(f, n, "unsupported module name %s", f.text(n.ConstantPath.GetLocation()))
	}
	cls := c.declareClass(f, n.Name, f.line(n.Location.StartOffset), true)
	c.collectBody(f, cls, n.Body)
}

func (c *Compiler) collectBody(f *File, cls *Class, body parser.Node) {
	if body == nil {
		return
	}
	stmts, ok := body.(*parser.StatementsNode)
	if !ok {
		c.errorf(f, body, "unsupported class body %T", body)
	}
	private := false
	for _, n := range stmts.Body {
		switch n := n.(type) {
		case *parser.DefNode:
			c.addMethod(f, cls, n, private)
		case *parser.CallNode:
			if n.Receiver != nil {
				c.errorf(f, n, "unsupported statement in class body: %s", f.text(n.Location))
			}
			args := callArgs(n)
			switch n.Name {
			case "attr_reader", "attr_writer", "attr_accessor":
				c.addAttrs(f, cls, n, args, private)
			case "include":
				for _, a := range args {
					cr, ok := a.(*parser.ConstantReadNode)
					if !ok {
						c.errorf(f, a, "unsupported include argument")
					}
					inc := Include{name: cr.Name, line: f.line(n.Location.StartOffset), file: f}
					if t, ok := f.trailing[inc.line]; ok && strings.HasPrefix(t, "[") {
						tup, err := rbs.ParseType(t)
						if err != nil {
							c.errorf(f, n, "bad include type args %q: %v", t, err)
						}
						inc.args = tup.(rbs.Tuple).Elems
					}
					cls.Includes = append(cls.Includes, inc)
				}
			case "private":
				if len(args) == 0 {
					private = true
				} else if d, ok := args[0].(*parser.DefNode); ok && len(args) == 1 {
					c.addMethod(f, cls, d, true)
				} else {
					c.errorf(f, n, "unsupported private form")
				}
			case "public":
				private = false
			default:
				c.errorf(f, n, "unsupported call in class body: %s", n.Name)
			}
		default:
			c.errorf(f, n, "unsupported node in class body: %T", n)
		}
	}
	// `# @rbs @x: T` annotations anywhere in the body
	start := f.line(body.GetLocation().StartOffset)
	end := f.line(body.GetLocation().StartOffset + body.GetLocation().Length)
	for ln := cls.Line; ln <= end+1; ln++ {
		if ln < cls.Line {
			continue
		}
		for _, iv := range f.annotations(ln)["ivar"] {
			name, ty, ok := strings.Cut(iv, ":")
			if !ok {
				c.errorf(f, nil, "%s:%d: bad ivar annotation %q", f.Name, ln, iv)
			}
			t, err := rbs.ParseType(strings.TrimSpace(ty))
			if err != nil {
				c.errorf(f, nil, "%s:%d: %v", f.Name, ln, err)
			}
			cls.ivarDecls = append(cls.ivarDecls, ivarDecl{name: strings.TrimSpace(name), rbs: t, line: ln})
		}
	}
	_ = start
}

func callArgs(n *parser.CallNode) []parser.Node {
	if n.Arguments == nil {
		return nil
	}
	return n.Arguments.Arguments
}

func (c *Compiler) addMethod(f *File, cls *Class, n *parser.DefNode, private bool) {
	if n.Receiver != nil {
		c.errorf(f, n, "singleton methods (def self.x) are not supported")
	}
	line := f.line(n.Location.StartOffset)
	m := &Method{Name: n.Name, GoName: goMethodName(n.Name), Owner: cls, Node: n, File: f, Line: line, Private: private}
	if sig, found := f.sigComment(line); found {
		if sig == "" {
			c.errorf(f, n, "overloaded signatures (#|) are not supported")
		}
		m.sigText = sig
	}
	if body, ok := n.Body.(*parser.StatementsNode); ok && len(body.Body) == 1 {
		if _, ok := body.Body[0].(*parser.XStringNode); ok {
			m.Kind = kindPrimitive
		}
	}
	if cls == nil {
		if c.topDefs[n.Name] != nil {
			c.errorf(f, n, "duplicate top-level def %s", n.Name)
		}
		m.GoName = goFuncName(n.Name)
		c.topDefs[n.Name] = m
		c.topDefList = append(c.topDefList, m)
		return
	}
	if old := cls.Methods[n.Name]; old != nil {
		// reopening: replace
		for i, x := range cls.MethodList {
			if x == old {
				cls.MethodList[i] = m
			}
		}
	} else {
		cls.MethodList = append(cls.MethodList, m)
	}
	cls.Methods[n.Name] = m
}

func (c *Compiler) addAttrs(f *File, cls *Class, n *parser.CallNode, args []parser.Node, private bool) {
	line := f.line(n.Location.StartOffset)
	if !cls.isStruct() {
		c.errorf(f, n, "%s on a non-struct class", n.Name)
	}
	ty, ok := f.trailing[line]
	if !ok {
		c.errorf(f, n, "%s needs a trailing `#: Type` annotation", n.Name)
	}
	rt, err := rbs.ParseType(ty)
	if err != nil {
		c.errorf(f, n, "%v", err)
	}
	for _, a := range args {
		sym, ok := a.(*parser.SymbolNode)
		if !ok {
			c.errorf(f, a, "%s argument must be a symbol", n.Name)
		}
		name := sym.Unescaped.Value
		cls.ivarDecls = append(cls.ivarDecls, ivarDecl{name: "@" + name, rbs: rt, line: line})
		if n.Name != "attr_writer" {
			m := &Method{Name: name, GoName: goMethodName(name), Owner: cls, Kind: kindAttrReader, Attr: "@" + name, File: f, Line: line, Private: private}
			m.sig = &rbs.MethodType{Return: rt}
			cls.Methods[name] = m
			cls.MethodList = append(cls.MethodList, m)
		}
		if n.Name != "attr_reader" {
			m := &Method{Name: name + "=", GoName: goMethodName(name + "="), Owner: cls, Kind: kindAttrWriter, Attr: "@" + name, File: f, Line: line, Private: private}
			m.sig = &rbs.MethodType{Params: []rbs.Param{{Type: rt, Name: name}}, Return: rbs.Void{}}
			cls.Methods[name+"="] = m
			cls.MethodList = append(cls.MethodList, m)
		}
	}
}

func (c *Compiler) collectTopDef(f *File, n *parser.DefNode) {
	c.addMethod(f, nil, n, false)
}

// ---- resolution

func (c *Compiler) link() {
	for _, cls := range c.classList {
		if cls.SuperName != "" {
			sup := c.classes[cls.SuperName]
			if sup == nil {
				c.errorf(cls.File, nil, "%s:%d: unknown superclass %s", cls.File.Name, cls.Line, cls.SuperName)
			}
			if sup.IsModule {
				c.errorf(cls.File, nil, "%s:%d: superclass %s is a module", cls.File.Name, cls.Line, cls.SuperName)
			}
			cls.Super = sup
			sup.Subclasses = append(sup.Subclasses, cls)
		}
		for i := range cls.Includes {
			inc := &cls.Includes[i]
			mod := c.classes[inc.name]
			if mod == nil || !mod.IsModule {
				c.errorf(inc.file, nil, "%s:%d: unknown module %s", inc.file.Name, inc.line, inc.name)
			}
			inc.Mod = mod
			if len(inc.args) != len(mod.TypeParams) {
				c.errorf(inc.file, nil, "%s:%d: include %s needs %d type args (`include %s #[...]`)", inc.file.Name, inc.line, inc.name, len(mod.TypeParams), inc.name)
			}
			for _, a := range inc.args {
				inc.Args = append(inc.Args, c.resolveType(a, typeScope{class: cls, file: inc.file, line: inc.line}))
			}
		}
	}
	for _, cls := range c.classList {
		if cls.GoType != "" && cls.Super != nil && cls.Super.isStruct() && !cls.Super.universal {
			c.errorf(cls.File, nil, "%s:%d: @go_type class %s cannot inherit from struct class %s", cls.File.Name, cls.Line, cls.Name, cls.Super.Name)
		}
		if cls.isStruct() && !cls.universal && len(cls.TypeParams) > 0 {
			c.errorf(cls.File, nil, "%s:%d: generic struct classes are not supported", cls.File.Name, cls.Line)
		}
	}
	// ivar declarations
	for _, cls := range c.classList {
		for _, d := range cls.ivarDecls {
			t := c.resolveType(d.rbs, typeScope{class: cls, file: cls.File, line: d.line})
			c.declareIvar(cls, d.name, t, cls.File, d.line)
		}
	}
	// method signatures
	for _, cls := range c.classList {
		for _, m := range cls.MethodList {
			c.resolveMethod(m)
		}
	}
	for _, m := range c.topDefList {
		c.resolveMethod(m)
	}
}

// declareIvar records an ivar on the topmost class of the chain that
// mentions it, so subclasses share the parent's field.
func (c *Compiler) declareIvar(cls *Class, name string, t Type, f *File, line int) *Ivar {
	for k := cls; k != nil; k = k.Super {
		if iv := k.Ivars[name]; iv != nil {
			if !typeEq(iv.Type, t) {
				c.errorf(f, nil, "%s:%d: ivar %s declared as %s but %s has it as %s", f.Name, line, name, t, k.Name, iv.Type)
			}
			return iv
		}
	}
	iv := &Ivar{Name: name, Type: t, Owner: cls}
	cls.Ivars[name] = iv
	cls.IvarList = append(cls.IvarList, iv)
	return iv
}

func (c *Compiler) findIvar(cls *Class, name string) *Ivar {
	for k := cls; k != nil; k = k.Super {
		if iv := k.Ivars[name]; iv != nil {
			return iv
		}
	}
	return nil
}

type typeScope struct {
	class      *Class
	methodTPs  []string
	file       *File
	line       int
	selfIsVar  bool
	allowVoid  bool
	allowUnion bool
}

func (c *Compiler) resolveType(t rbs.Type, sc typeScope) Type {
	switch t := t.(type) {
	case rbs.Name:
		for _, p := range sc.methodTPs {
			if p == t.Name {
				return TVar{Name: p}
			}
		}
		if sc.class != nil {
			for _, p := range sc.class.TypeParams {
				if p == t.Name {
					return TVar{Name: p}
				}
			}
		}
		cls := c.classes[t.Name]
		if cls == nil {
			c.errorf(sc.file, nil, "%s:%d: unknown type %s", sc.file.Name, sc.line, t.Name)
		}
		if len(t.Args) != len(cls.TypeParams) {
			c.errorf(sc.file, nil, "%s:%d: %s takes %d type args, got %d", sc.file.Name, sc.line, cls.Name, len(cls.TypeParams), len(t.Args))
		}
		args := make([]Type, len(t.Args))
		for i, a := range t.Args {
			args[i] = c.resolveType(a, sc)
		}
		return TClass{C: cls, Args: args}
	case rbs.Bool:
		return TClass{C: c.classes["Boolean"]}
	case rbs.Optional:
		return TOpt{Elem: c.resolveType(t.Elem, sc)}
	case rbs.Tuple:
		elems := make([]Type, len(t.Elems))
		for i, e := range t.Elems {
			elems[i] = c.resolveType(e, sc)
		}
		if len(elems) < 2 || len(elems) > 3 {
			c.errorf(sc.file, nil, "%s:%d: only 2- and 3-tuples are supported", sc.file.Name, sc.line)
		}
		return TTuple{Elems: elems}
	case rbs.Self:
		if sc.class == nil {
			c.errorf(sc.file, nil, "%s:%d: `self` type outside a class", sc.file.Name, sc.line)
		}
		return TVar{Name: "Self"}
	case rbs.Void:
		return TVoid{}
	case rbs.Nil:
		return TNil{}
	case rbs.Untyped:
		return TAny{}
	case rbs.Union:
		c.errorf(sc.file, nil, "%s:%d: union types are not supported: %s", sc.file.Name, sc.line, t)
	}
	c.errorf(sc.file, nil, "%s:%d: unsupported type %s", sc.file.Name, sc.line, t)
	return nil
}

func (c *Compiler) resolveMethod(m *Method) {
	if m.resolved {
		return
	}
	m.resolved = true
	f := m.File
	if m.sig == nil {
		if m.sigText == "" {
			// unannotated override inherits the parent's signature
			if m.Owner != nil {
				if e := c.inheritedSig(m); e != nil {
					c.resolveMethod(e.M)
					m.inherited = e.M
					m.TypeParams = e.M.TypeParams
					for _, p := range e.M.Params {
						m.Params = append(m.Params, Param{Name: p.Name, Type: subst(p.Type, e.Env), Default: p.Default, Rest: p.Rest})
					}
					if e.M.Block != nil {
						m.Block = &BlockSig{Params: substAll(e.M.Block.Params, e.Env), Ret: subst(e.M.Block.Ret, e.Env)}
					}
					m.Ret = subst(e.M.Ret, e.Env)
					m.Iterator = e.M.Iterator
					c.bindParamNames(m)
					return
				}
			}
			c.errorf(f, m.Node, "method %s has no type annotation (`#: (...) -> T`)", m.Name)
		}
		sig, err := rbs.ParseMethodType(m.sigText)
		if err != nil {
			c.errorf(f, m.Node, "%v", err)
		}
		m.sig = sig
	}
	sc := typeScope{class: m.Owner, methodTPs: m.sig.TypeParams, file: f, line: m.Line}
	m.TypeParams = m.sig.TypeParams
	for _, p := range m.sig.Params {
		m.Params = append(m.Params, Param{Name: p.Name, Type: c.resolveType(p.Type, sc), Rest: p.Rest})
	}
	if m.sig.Block != nil {
		bs := &BlockSig{Ret: c.resolveType(m.sig.Block.Return, sc)}
		for _, p := range m.sig.Block.Params {
			bs.Params = append(bs.Params, c.resolveType(p.Type, sc))
		}
		m.Block = bs
		m.Iterator = isVoid(bs.Ret) && !m.sig.Block.Optional
	}
	m.Ret = c.resolveType(m.sig.Return, sc)
	c.bindParamNames(m)
}

func substAll(ts []Type, env map[string]Type) []Type {
	out := make([]Type, len(ts))
	for i, t := range ts {
		out[i] = subst(t, env)
	}
	return out
}

// inheritedSig finds the method an unannotated def overrides.
func (c *Compiler) inheritedSig(m *Method) *entry {
	cls := m.Owner
	for i := len(cls.Includes) - 1; i >= 0; i-- {
		inc := cls.Includes[i]
		if inc.Mod == nil {
			continue
		}
		if e := inc.Mod.lookup(m.Name); e != nil {
			env := map[string]Type{}
			for j, p := range inc.Mod.TypeParams {
				env[p] = inc.Args[j]
			}
			return &entry{M: e.M, Owner: e.Owner, Env: composeEnv(e.Env, env), Entry: cls}
		}
	}
	if cls.Super != nil {
		return cls.Super.lookup(m.Name)
	}
	return nil
}

// bindParamNames matches RBS positional params to the def's parameter
// names and records literal defaults.
func (c *Compiler) bindParamNames(m *Method) {
	if m.Node == nil {
		return
	}
	var names []string
	var defaults []parser.Node
	var rest string
	if ps := m.Node.Parameters; ps != nil {
		for _, p := range ps.Requireds {
			rp, ok := p.(*parser.RequiredParameterNode)
			if !ok {
				c.errorf(m.File, p, "unsupported parameter form")
			}
			names = append(names, rp.Name)
			defaults = append(defaults, nil)
		}
		for _, p := range ps.Optionals {
			op, ok := p.(*parser.OptionalParameterNode)
			if !ok {
				c.errorf(m.File, p, "unsupported parameter form")
			}
			names = append(names, op.Name)
			defaults = append(defaults, op.Value)
		}
		if ps.Rest != nil {
			rp, ok := ps.Rest.(*parser.RestParameterNode)
			if !ok || rp.Name == nil {
				c.errorf(m.File, ps.Rest, "unsupported rest parameter")
			}
			rest = *rp.Name
		}
		if len(ps.Posts) > 0 || len(ps.Keywords) > 0 || ps.KeywordRest != nil {
			c.errorf(m.File, ps, "keyword and post parameters are not supported")
		}
		if ps.Block != nil {
			c.errorf(m.File, ps, "explicit &block parameters are not supported; use yield")
		}
	}
	nPos := 0
	for i := range m.Params {
		if m.Params[i].Rest {
			if rest == "" {
				c.errorf(m.File, m.Node, "signature has a rest param but def does not")
			}
			m.Params[i].Name = rest
			continue
		}
		if nPos >= len(names) {
			c.errorf(m.File, m.Node, "signature has %d positional params but def has %d", len(m.Params), len(names))
		}
		m.Params[i].Name = names[nPos]
		m.Params[i].Default = defaults[nPos]
		nPos++
	}
	if nPos != len(names) {
		c.errorf(m.File, m.Node, "signature has %d positional params but def has %d", nPos, len(names))
	}
	if rest != "" {
		found := false
		for _, p := range m.Params {
			found = found || p.Rest
		}
		if !found {
			c.errorf(m.File, m.Node, "def has a rest param but signature does not")
		}
	}
}

func (m *Method) String() string {
	owner := "main"
	if m.Owner != nil {
		owner = m.Owner.Name
	}
	return fmt.Sprintf("%s#%s", owner, m.Name)
}
