package compiler

import (
	"context"
	"fmt"
	"strings"

	"github.com/danielgatis/go-ruby-prism/parser"

	"rb2go/internal/rbs"
)

// Class is a Ruby class or module.
type Class struct {
	Name       string // Go identifier: "Resty_Actions_Show"
	RubyName   string // constant path: "Resty::Actions::Show"
	IsModule   bool
	GoType     string // `@go_type` underlying Go type; "" for struct classes
	TypeParams []string
	superRef   *constRef // superclass expression, resolved in link
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
	singletonDefs []singletonDef
	extends       []Include // `extend M`: included into the class object
	meta          *Class    // the class object's class (holds `def self.` methods)
	metaOf        *Class    // for a metaclass: the class it describes
	constNames    []string  // constants (classes included) declared directly inside, in order
	valueMembers  []string  // Struct.new / Data.define members, in order
	valueKind     string    // "struct" or "data"
	msetCache     []entry
	selfCallCache map[string]bool
}

type ivarDecl struct {
	name  string
	rbs   rbs.Type
	line  int
	scope []*Class
}

// constRef is a constant expression to resolve once every class is known.
type constRef struct {
	node  parser.Node
	scope []*Class
	file  *File
}

// Include is `include Mod #[Args]`.
type Include struct {
	Mod   *Class
	Args  []Type
	ref   constRef
	args  []rbs.Type
	line  int
	file  *File
	scope []*Class
}

// Const is a non-class constant (`VERSION = "1.0"`), emitted as a Go
// package-level variable.
type Const struct {
	RubyName  string
	GoName    string
	Value     parser.Node
	File      *File
	Line      int
	Scope     []*Class // lexical scope of the assignment
	ann       string   // trailing `#: T`
	Type      Type
	resolving bool
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
	kindSynth // generated per metaclass: new, name, to_s, inspect
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
	Scope      []*Class // lexical scope (Module.nesting), innermost last
	sigText    string
	sig        *rbs.MethodType
	TypeParams []string
	Params     []Param
	Block      *BlockSig
	Ret        Type
	Iterator   bool   // block returns void → iter.Seq
	BlockParam string // name of an explicit &block parameter
	resolved   bool
	inherited  *Method // signature source for unannotated overrides
}

// root is the topmost struct class of c's hierarchy (below Object).
func (c *Class) root() *Class {
	k := c
	for k.Super != nil && !k.Super.universal {
		k = k.Super
	}
	return k
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

// ancestors is c then what MRI's Module#ancestors lists after it, short of
// Object (whose constants are the top-level ones): included modules last
// first, each followed by its own, then the superclass's ancestors.
// Includes not yet resolved (during link) are skipped.
func (c *Class) ancestors() []*Class {
	out := []*Class{c}
	for i := len(c.Includes) - 1; i >= 0; i-- {
		if m := c.Includes[i].Mod; m != nil {
			out = append(out, m.ancestors()...)
		}
	}
	if c.Super != nil && !c.Super.universal {
		out = append(out, c.Super.ancestors()...)
	}
	return out
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

func (c *Compiler) collect(ctx context.Context, f *File) {
	for _, n := range f.Root.Statements.Body {
		switch n := n.(type) {
		case *parser.ClassNode:
			c.collectClass(ctx, f, n, nil)
		case *parser.ModuleNode:
			c.collectModule(ctx, f, n, nil)
		case *parser.DefNode:
			c.collectTopDef(f, n)
		case *parser.ConstantWriteNode:
			if call, kind := valueClass(n.Value); call != nil {
				c.collectValueClass(ctx, f, n, call, kind, nil)
				continue
			}
			c.addConst(f, n, n.Name, nil, nil)
		case *parser.XStringNode:
			if !f.prelude {
				c.errorf(f, n, "top-level %%x{} is only allowed in the prelude")
			}
			c.verbatim = append(c.verbatim, verbatim{file: f, line: f.line(n.Location.StartOffset), code: n.Unescaped.Value})
		case *parser.CallNode:
			if n.Receiver == nil && n.Name == "require_relative" && f.prelude {
				c.requireRelative(ctx, f, n)
				continue
			}
			if n.Receiver == nil && n.Name == "require_relative" {
				c.errorf(f, n, "require_relative is not supported in user code")
			}
			if n.Receiver == nil && n.Name == "require" {
				continue // stdlib requires are meaningless here
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
		cls = &Class{Name: goClassName(name), RubyName: name, IsModule: isModule, Methods: map[string]*Method{}, Ivars: map[string]*Ivar{}, File: f, Line: line}
		cls.universal = name == "BasicObject" || name == "Object" || name == "Kernel"
		c.classes[name] = cls
		c.classList = append(c.classList, cls)
		c.noteConstName(name)
	} else if cls.IsModule != isModule {
		c.errorf(f, nil, "%s:%d: %s is already defined as a %s", f.Name, line, name, map[bool]string{true: "module", false: "class"}[cls.IsModule])
	}
	if c.consts[name] != nil {
		c.errorf(f, nil, "%s:%d: %s is already a constant", f.Name, line, name)
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

// goClassName maps a constant path to a Go identifier.
func goClassName(rubyName string) string { return strings.ReplaceAll(rubyName, "::", "_") }

// qualify names the constant `name` declared directly inside scope.
func qualify(scope []*Class, name string) string {
	if len(scope) == 0 {
		return name
	}
	return scope[len(scope)-1].RubyName + "::" + name
}

// declName resolves the name of a `class X` / `class A::X` / `module X`.
func (c *Compiler) declName(f *File, path parser.Node, scope []*Class) string {
	switch p := path.(type) {
	case *parser.ConstantReadNode:
		return qualify(scope, p.Name)
	case *parser.ConstantPathNode:
		if p.Parent == nil {
			return *p.Name
		}
		parent, _ := c.lookupConst(f, p.Parent, scope)
		if parent == nil {
			c.errorf(f, p.Parent, "unknown namespace %s", f.text(p.Parent.GetLocation()))
		}
		return parent.RubyName + "::" + *p.Name
	}
	c.errorf(f, path, "unsupported class name %s", f.text(path.GetLocation()))
	return ""
}

func (c *Compiler) collectClass(ctx context.Context, f *File, n *parser.ClassNode, scope []*Class) {
	line := f.line(n.Location.StartOffset)
	name := c.declName(f, n.ConstantPath, scope)
	cls := c.declareClass(f, name, line, false)
	if n.Superclass != nil {
		if cls.superRef != nil && f.text(cls.superRef.node.GetLocation()) != f.text(n.Superclass.GetLocation()) {
			c.errorf(f, n, "class %s reopened with a different superclass", name)
		}
		// The superclass expression is evaluated outside the class body.
		cls.superRef = &constRef{node: n.Superclass, scope: scope, file: f}
	}
	c.collectBody(ctx, f, cls, n.Body, append(append([]*Class(nil), scope...), cls))
}

func (c *Compiler) collectModule(ctx context.Context, f *File, n *parser.ModuleNode, scope []*Class) {
	name := c.declName(f, n.ConstantPath, scope)
	cls := c.declareClass(f, name, f.line(n.Location.StartOffset), true)
	c.collectBody(ctx, f, cls, n.Body, append(append([]*Class(nil), scope...), cls))
}

// addConst records `NAME = value` declared in scope.
func (c *Compiler) addConst(f *File, n parser.Node, name string, value parser.Node, scope []*Class) {
	if w, ok := n.(*parser.ConstantWriteNode); ok {
		value = w.Value
	}
	full := qualify(scope, name)
	if c.consts[full] != nil || c.classes[full] != nil {
		c.errorf(f, n, "constant %s is already defined", full)
	}
	k := &Const{RubyName: full, GoName: goClassName(full), Value: value, File: f, Line: f.line(n.GetLocation().StartOffset), Scope: scope}
	k.ann = f.trailingAnnotation(n)
	c.consts[full] = k
	c.constList = append(c.constList, k)
	c.noteConstName(full)
}

// noteConstName records a new constant in its namespace's table, in
// definition order (top-level ones belong to Object).
func (c *Compiler) noteConstName(full string) {
	i := strings.LastIndex(full, "::")
	if i < 0 {
		c.topConstNames = append(c.topConstNames, full)
		return
	}
	if parent := c.classes[full[:i]]; parent != nil {
		parent.constNames = append(parent.constNames, full[i+2:])
	}
}
func (c *Compiler) collectBody(ctx context.Context, f *File, cls *Class, body parser.Node, scope []*Class) {
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
			// a bare `private` does not reach `def self.x`
			c.addMethod(f, cls, n, private && n.Receiver == nil, scope)
		case *parser.CallNode:
			c.collectClassCall(f, cls, n, &private, scope)
		case *parser.ClassNode:
			c.collectClass(ctx, f, n, scope)
		case *parser.ModuleNode:
			c.collectModule(ctx, f, n, scope)
		case *parser.ConstantWriteNode:
			if call, kind := valueClass(n.Value); call != nil {
				c.collectValueClass(ctx, f, n, call, kind, scope)
				continue
			}
			c.addConst(f, n, n.Name, nil, scope)
		default:
			c.errorf(f, n, "unsupported node in class body: %s", nodeType(n))
		}
	}
	c.collectIvarDecls(f, cls, body, scope)
}

// collectClassCall handles a bare call in a class body: attr_*, include,
// private/public.
func (c *Compiler) collectClassCall(f *File, cls *Class, n *parser.CallNode, private *bool, scope []*Class) {
	if n.Receiver != nil {
		c.errorf(f, n, "unsupported statement in class body: %s", f.text(n.Location))
	}
	args := callArgs(n)
	switch n.Name {
	case "attr_reader", "attr_writer", "attr_accessor":
		c.addAttrs(f, cls, n, args, *private, scope)
	case "include":
		for _, a := range args {
			c.addInclude(f, n, cls, a, scope)
		}
	case "extend":
		for _, a := range args {
			c.addInclude(f, n, cls, a, scope)
			last := len(cls.Includes) - 1
			cls.extends = append(cls.extends, cls.Includes[last])
			cls.Includes = cls.Includes[:last]
		}
	case "private":
		switch {
		case len(args) == 0:
			*private = true
		case len(args) == 1:
			d, ok := args[0].(*parser.DefNode)
			if !ok {
				c.errorf(f, n, "unsupported private form")
			}
			c.addMethod(f, cls, d, true, scope)
		default:
			c.errorf(f, n, "unsupported private form")
		}
	case "public":
		*private = false
	default:
		c.errorf(f, n, "unsupported call in class body: %s", n.Name)
	}
}

func (c *Compiler) addInclude(f *File, n *parser.CallNode, cls *Class, a parser.Node, scope []*Class) {
	switch a.(type) {
	case *parser.ConstantReadNode, *parser.ConstantPathNode:
	default:
		c.errorf(f, a, "unsupported include argument")
	}
	inc := Include{ref: constRef{node: a, scope: scope, file: f}, line: f.line(n.Location.StartOffset), file: f, scope: scope}
	if t, ok := f.typeArgs[inc.line]; ok {
		tup, err := rbs.ParseType(t)
		if err != nil {
			c.errorf(f, n, "bad include type args %q: %v", t, err)
		}
		inc.args = tup.(rbs.Tuple).Elems
	}
	cls.Includes = append(cls.Includes, inc)
}

// collectIvarDecls picks up `# @rbs @x: T` annotations anywhere in a body.
func (c *Compiler) collectIvarDecls(f *File, cls *Class, body parser.Node, scope []*Class) {
	end := f.line(body.GetLocation().StartOffset + body.GetLocation().Length)
	for ln := cls.Line; ln <= end+1; ln++ {
		for _, iv := range f.annotations(ln)["ivar"] {
			name, ty, ok := strings.Cut(iv, ":")
			if !ok {
				c.errorf(f, nil, "%s:%d: bad ivar annotation %q", f.Name, ln, iv)
			}
			t, err := rbs.ParseType(strings.TrimSpace(ty))
			if err != nil {
				c.errorf(f, nil, "%s:%d: %v", f.Name, ln, err)
			}
			cls.ivarDecls = append(cls.ivarDecls, ivarDecl{name: strings.TrimSpace(name), rbs: t, line: ln, scope: scope})
		}
	}
}

func callArgs(n *parser.CallNode) []parser.Node {
	if n.Arguments == nil {
		return nil
	}
	return n.Arguments.Arguments
}

func (c *Compiler) addMethod(f *File, cls *Class, n *parser.DefNode, private bool, scope []*Class) {
	if n.Receiver != nil {
		if _, ok := n.Receiver.(*parser.SelfNode); !ok || cls == nil {
			c.errorf(f, n, "singleton methods are only supported as `def self.x` in a class or module body")
		}
		cls.singletonDefs = append(cls.singletonDefs, singletonDef{node: n, private: private, scope: scope, file: f})
		return
	}
	c.addDef(f, cls, n, private, scope)
}

func (c *Compiler) addDef(f *File, cls *Class, n *parser.DefNode, private bool, scope []*Class) {
	line := f.line(n.Location.StartOffset)
	m := &Method{Name: n.Name, GoName: goMethodName(n.Name), Owner: cls, Node: n, File: f, Line: line, Private: private || cls != nil && cls.metaOf == nil && rubyPrivate[n.Name], Scope: scope}
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

func (c *Compiler) addAttrs(f *File, cls *Class, n *parser.CallNode, args []parser.Node, private bool, scope []*Class) {
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
		cls.ivarDecls = append(cls.ivarDecls, ivarDecl{name: "@" + name, rbs: rt, line: line, scope: scope})
		if n.Name != "attr_writer" {
			m := &Method{Name: name, GoName: goMethodName(name), Owner: cls, Kind: kindAttrReader, Attr: "@" + name, File: f, Line: line, Private: private, Scope: scope}
			m.sig = &rbs.MethodType{Return: rt}
			cls.Methods[name] = m
			cls.MethodList = append(cls.MethodList, m)
		}
		if n.Name != "attr_reader" {
			m := &Method{Name: name + "=", GoName: goMethodName(name + "="), Owner: cls, Kind: kindAttrWriter, Attr: "@" + name, File: f, Line: line, Private: private, Scope: scope}
			m.sig = &rbs.MethodType{Params: []rbs.Param{{Type: rt, Name: name}}, Return: rbs.Void{}}
			cls.Methods[name+"="] = m
			cls.MethodList = append(cls.MethodList, m)
		}
	}
}

func (c *Compiler) collectTopDef(f *File, n *parser.DefNode) {
	c.addMethod(f, nil, n, false, nil)
}

type singletonDef struct {
	node    *parser.DefNode
	private bool
	scope   []*Class
	file    *File
}

// lookupConst resolves a constant expression (Foo, Foo::Bar, ::Foo) the
// way Ruby does: lexical scope innermost-out, then the innermost class's
// ancestors, then top level. Exactly one of the results is non-nil, or
// both are nil when nothing matched.
func (c *Compiler) lookupConst(f *File, n parser.Node, scope []*Class) (*Class, *Const) {
	switch n := n.(type) {
	case *parser.ConstantReadNode:
		return c.lookupName(scope, n.Name)
	case *parser.ConstantPathNode:
		if n.Parent == nil {
			return c.classes[*n.Name], c.consts[*n.Name]
		}
		parent, k := c.lookupConst(f, n.Parent, scope)
		if parent == nil {
			if k != nil {
				c.errorf(f, n, "%s is not a class or module", k.RubyName)
			}
			return nil, nil
		}
		for _, anc := range parent.ancestors() {
			full := anc.RubyName + "::" + *n.Name
			if cls, k := c.classes[full], c.consts[full]; cls != nil || k != nil {
				return cls, k
			}
		}
	}
	return nil, nil
}

func (c *Compiler) lookupName(scope []*Class, name string) (*Class, *Const) {
	// A qualified name from RBS ("A::B") resolves its head lexically.
	head, rest, qualified := strings.Cut(name, "::")
	find := func(full string) (*Class, *Const) {
		if qualified {
			full += "::" + rest
		}
		return c.classes[full], c.consts[full]
	}
	for i := len(scope) - 1; i >= 0; i-- {
		if cls, k := find(scope[i].RubyName + "::" + head); cls != nil || k != nil {
			return cls, k
		}
	}
	if len(scope) > 0 {
		for _, anc := range scope[len(scope)-1].ancestors()[1:] {
			if cls, k := find(anc.RubyName + "::" + head); cls != nil || k != nil {
				return cls, k
			}
		}
	}
	return find(head)
}

// resolveClassRef resolves a constRef that must name a class or module.
func (c *Compiler) resolveClassRef(r *constRef) *Class {
	cls, _ := c.lookupConst(r.file, r.node, r.scope)
	if cls == nil {
		c.errorf(r.file, r.node, "unknown class or module %s", r.file.text(r.node.GetLocation()))
	}
	return cls
}

// ---- resolution

func (c *Compiler) link() {
	for _, cls := range c.classList {
		var sup *Class
		switch {
		case cls.superRef != nil:
			sup = c.resolveClassRef(cls.superRef)
			if sup.IsModule {
				c.errorf(cls.File, nil, "%s:%d: superclass %s is a module", cls.File.Name, cls.Line, sup.RubyName)
			}
			// a @go_type is a Go value type (string, []E), not a struct a subclass can embed
			if sup.GoType != "" {
				c.errorf(cls.File, nil, "%s:%d: subclassing %s is not supported (it is a @go_type class; hold one in an ivar instead)", cls.File.Name, cls.Line, sup.RubyName)
			}
		case !cls.IsModule && cls.RubyName != "BasicObject":
			sup = c.classes["Object"]
		}
		if sup != nil {
			cls.Super = sup
			sup.Subclasses = append(sup.Subclasses, cls)
		}
		for i := range cls.Includes {
			inc := &cls.Includes[i]
			mod := c.resolveClassRef(&inc.ref)
			if !mod.IsModule {
				c.errorf(inc.file, nil, "%s:%d: %s is not a module", inc.file.Name, inc.line, mod.RubyName)
			}
			inc.Mod = mod
			if len(inc.args) != len(mod.TypeParams) {
				c.errorf(inc.file, nil, "%s:%d: include %s needs %d type args (`include %s #[...]`)", inc.file.Name, inc.line, mod.RubyName, len(mod.TypeParams), mod.RubyName)
			}
			for _, a := range inc.args {
				inc.Args = append(inc.Args, c.resolveType(a, typeScope{class: cls, lex: inc.scope, file: inc.file, line: inc.line}))
			}
		}
	}
	c.buildMetas()
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
			t := c.resolveType(d.rbs, typeScope{class: cls, lex: d.scope, file: cls.File, line: d.line})
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
	for _, cls := range c.classList {
		ms := make([]*Method, 0, len(cls.MethodList))
		for _, e := range cls.methodSet() {
			ms = append(ms, e.M)
		}
		c.checkGoNames(cls.RubyName, ms)
	}
	c.checkGoNames("the top level", c.topDefList)
}

// Decision 3's naming still merges capitals (`foo_bar`/`fooBar`) and digits after `_` (`utf_8`/`utf8`), so reject those here, not at go build.
func (c *Compiler) checkGoNames(where string, ms []*Method) {
	seen := map[string]*Method{}
	for _, m := range ms {
		if p := seen[m.GoName]; p != nil && p.Name != m.Name {
			c.errorf(nil, nil, "%s:%d: `%s` and `%s` (%s:%d) both become Go %s in %s; rename one", m.File.Name, m.Line, m.Name, p.Name, p.File.Name, p.Line, m.GoName, where)
		}
		seen[m.GoName] = m
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
	class     *Class
	lex       []*Class // lexical scope for constant names
	methodTPs []string
	file      *File
	line      int
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
		cls, _ := c.lookupName(sc.lex, t.Name)
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
	case rbs.Singleton:
		cls, _ := c.lookupName(sc.lex, t.Name)
		if cls == nil {
			c.errorf(sc.file, nil, "%s:%d: unknown type %s", sc.file.Name, sc.line, t.Name)
		}
		if cls.meta == nil {
			c.errorf(sc.file, nil, "%s:%d: %s has no class object type", sc.file.Name, sc.line, t)
		}
		return TClass{C: cls.meta}
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
	if m.Kind == kindSynth {
		c.resolveSynth(m)
		return
	}
	f := m.File
	if m.sig == nil && m.sigText == "" {
		// unannotated override inherits the parent's signature
		if c.inheritSignature(m) {
			return
		}
		c.errorf(f, m.Node, "method %s has no type annotation (`#: (...) -> T`)", m.Name)
	}
	if m.sig == nil {
		sig, err := rbs.ParseMethodType(m.sigText)
		if err != nil {
			c.errorf(f, m.Node, "%v", err)
		}
		m.sig = sig
	}
	sc := typeScope{class: m.Owner, lex: m.Scope, methodTPs: m.sig.TypeParams, file: f, line: m.Line}
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
	}
	m.Ret = c.resolveType(m.sig.Return, sc)
	if m.Block != nil {
		m.Iterator = c.isIterator(m, m.Block)
	}
	c.bindParamNames(m)
}

// inheritSignature copies the signature of the method m overrides, if any.
func (c *Compiler) inheritSignature(m *Method) bool {
	if m.Owner == nil {
		return false
	}
	e := c.inheritedSig(m)
	if e == nil {
		return false
	}
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
	return true
}

// isIterator decides whether a block-taking method compiles to a Go
// iterator (its block call sites become `for range` loops) or takes the
// block as a closure. Iterators: the block and the method both return
// nothing, the block is only ever yielded to (a %x{} leaf must build an
// iterator itself; one that stores or calls `blk` takes a closure), and
// nothing rescues around the yield, since Go forbids a range function from
// recovering a panic raised in the loop body.
func (c *Compiler) isIterator(m *Method, bs *BlockSig) bool {
	if !isVoid(bs.Ret) || m.sig.Block.Optional {
		return false
	}
	if _, ok := m.Ret.(TVoid); !ok && m.Ret != nil && !isNil(m.Ret) {
		return false
	}
	if m.Kind == kindPrimitive {
		x := m.Node.Body.(*parser.StatementsNode).Body[0].(*parser.XStringNode)
		return strings.Contains(x.Unescaped.Value, "func(yield ")
	}
	return m.Kind != kindDef || !containsRescueClause(m.Node.Body)
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
	names, defaults, rest := c.defParams(m, m.Node.Parameters)
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
	switch {
	case m.Owner != nil && m.Owner.metaOf != nil:
		return m.Owner.RubyName + "." + m.Name
	case m.Owner != nil:
		owner = m.Owner.Name
	}
	return fmt.Sprintf("%s#%s", owner, m.Name)
}

// ---- metaclasses

// buildMetas gives every struct class, and every class or module with
// `def self.` methods, a metaclass: a struct class whose methods are the
// class methods, whose superclass is the parent's metaclass (so class
// methods inherit and dispatch virtually), and whose single instance is the
// class object.
func (c *Compiler) buildMetas() {
	for _, cls := range append([]*Class(nil), c.classList...) {
		if cls.RubyName == "BasicObject" || cls.RubyName == "Kernel" {
			if len(cls.singletonDefs) > 0 {
				c.errorf(cls.singletonDefs[0].file, cls.singletonDefs[0].node, "class methods on %s are not supported", cls.RubyName)
			}
			continue
		}
		if cls.metaOf == nil {
			c.metaFor(cls)
		}
	}
	// `extend M` is `include M` on the class object; resolved once every
	// class object exists, since its type args may name them.
	for _, cls := range c.classList {
		for _, ext := range cls.extends {
			ext.Mod = c.resolveClassRef(&ext.ref)
			if !ext.Mod.IsModule {
				c.errorf(ext.file, nil, "%s:%d: %s is not a module", ext.file.Name, ext.line, ext.Mod.RubyName)
			}
			if len(ext.args) != len(ext.Mod.TypeParams) {
				c.errorf(ext.file, nil, "%s:%d: extend %s needs %d type args (`extend %s #[...]`)", ext.file.Name, ext.line, ext.Mod.RubyName, len(ext.Mod.TypeParams), ext.Mod.RubyName)
			}
			for _, a := range ext.args {
				ext.Args = append(ext.Args, c.resolveType(a, typeScope{class: cls, lex: ext.scope, file: ext.file, line: ext.line}))
			}
			cls.meta.Includes = append(cls.meta.Includes, ext)
		}
	}
}

func (c *Compiler) metaFor(cls *Class) *Class {
	if cls.meta != nil {
		return cls.meta
	}
	if len(cls.TypeParams) > 0 && len(cls.singletonDefs) > 0 {
		c.errorf(cls.singletonDefs[0].file, cls.singletonDefs[0].node, "class methods on generic class %s are not supported", cls.RubyName)
	}
	// A class object is a Class (a module's, a Module); a subclass's class
	// object inherits from its superclass's, so class methods inherit.
	var sup *Class
	switch {
	case cls.IsModule:
		sup = c.classes["Module"]
	case cls.Super != nil && !cls.Super.universal:
		sup = c.metaFor(cls.Super)
	default:
		sup = c.classes["Class"]
	}
	m := &Class{Name: cls.Name + "_Meta", RubyName: cls.RubyName, Methods: map[string]*Method{}, Ivars: map[string]*Ivar{},
		File: cls.File, Line: cls.Line, Super: sup, metaOf: cls}
	cls.meta = m
	sup.Subclasses = append(sup.Subclasses, m)
	c.classList = append(c.classList, m)
	for _, d := range cls.singletonDefs {
		c.addDef(d.file, m, d.node, d.private, d.scope)
	}
	synth := []string{"name", "to_s", "inspect"}
	if cls.isStruct() && !cls.universal && m.Methods["new"] == nil {
		synth = append(synth, "new")
	}
	for _, name := range synth {
		if m.Methods[name] != nil {
			continue
		}
		sm := &Method{Name: name, GoName: goMethodName(name), Owner: m, Kind: kindSynth, File: cls.File, Line: cls.Line}
		m.Methods[name] = sm
		m.MethodList = append(m.MethodList, sm)
	}
	return m
}

// resolveSynth fills in a metaclass's generated methods. `new` takes the
// described class's initialize params and returns the hierarchy's root
// type, so that every metaclass in a hierarchy shares one Go signature and
// `singleton(Base)` can hold any subclass whose initialize matches.
func (c *Compiler) resolveSynth(m *Method) {
	cls := m.Owner.metaOf
	if m.Name != "new" {
		m.Ret = TClass{C: c.classes["String"]}
		return
	}
	m.Ret = TClass{C: cls.root()}
	init := cls.lookup("initialize")
	if init == nil {
		return
	}
	c.resolveMethod(init.M)
	env := composeEnv(init.Env, nil)
	env["Self"] = TClass{C: cls}
	for _, p := range init.M.Params {
		m.Params = append(m.Params, Param{Name: p.Name, Type: subst(p.Type, env), Default: p.Default, Rest: p.Rest})
	}
}

// isSynthNew reports whether e is a metaclass's generated `new`.
func isSynthNew(e *entry) bool { return e != nil && e.M.Kind == kindSynth && e.M.Name == "new" }

// classVar is the Go variable holding a class object.
func classVar(cls *Class) string { return cls.Name + "_class" }

// defParams reads a def's parameter list: positional names, their literal
// defaults, the rest parameter, and the &block parameter's name.
func (c *Compiler) defParams(m *Method, ps *parser.ParametersNode) (names []string, defaults []parser.Node, rest string) {
	if ps == nil {
		return nil, nil, ""
	}
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
		if m.Block == nil || ps.Block.Name == nil {
			c.errorf(m.File, ps, "a named &block parameter needs a block in the signature (`#: () { (T) -> U } -> R`)")
		}
		m.BlockParam = *ps.Block.Name
	}
	return names, defaults, rest
}

// valueClass recognizes `Struct.new(...)` and `Data.define(...)`.
func valueClass(n parser.Node) (*parser.CallNode, string) {
	call, ok := n.(*parser.CallNode)
	if !ok {
		return nil, ""
	}
	r, ok := call.Receiver.(*parser.ConstantReadNode)
	switch {
	case !ok:
	case r.Name == "Struct" && call.Name == "new":
		return call, "struct"
	case r.Name == "Data" && call.Name == "define":
		return call, "data"
	}
	return nil, ""
}

// collectValueClass declares `Name = Struct.new(:a, :b) do ... end` or
// `Name = Data.define(...)` as a class. Member types come from a `#: T`
// after each member on its own line (rbs-inline's form), or a trailing
// `#: [A, B]` on the assignment. The generated members are written as Ruby
// and parsed; they and the block body live in the outer lexical scope, as
// a block does not open a constant scope in Ruby.
func (c *Compiler) collectValueClass(ctx context.Context, f *File, n *parser.ConstantWriteNode, call *parser.CallNode, kind string, scope []*Class) {
	members := make([]string, 0, len(callArgs(call)))
	for _, a := range callArgs(call) {
		sym, ok := a.(*parser.SymbolNode)
		if !ok {
			c.errorf(f, a, "%s members must be symbols (keyword_init is not supported)", kind)
		}
		members = append(members, sym.Unescaped.Value)
	}
	if len(members) == 0 {
		c.errorf(f, call, "%s needs at least one member", kind)
	}
	types := c.memberTypes(f, n, call, members)
	full := qualify(scope, n.Name)
	line := f.line(n.Location.StartOffset)
	cls := c.declareClass(f, full, line, false)
	cls.superRef = &constRef{node: &parser.ConstantReadNode{Name: map[string]string{"struct": "Struct", "data": "Data"}[kind]}, file: f}
	cls.valueMembers, cls.valueKind = members, kind
	// pad so the generated lines report the declaration's line
	src := strings.Repeat("\n", line-1) + valueClassSource(kind, full, members, types)
	sf, err := parseFile(ctx, c.parser, f.Name, []byte(src), f.prelude)
	if err != nil {
		c.errorf(f, n, "internal error: generated %s does not parse: %v", kind, err)
	}
	c.collectBody(ctx, sf, cls, sf.Root.Statements.Body[0].(*parser.ClassNode).Body, scope)
	if bn, ok := call.Block.(*parser.BlockNode); ok && bn.Body != nil {
		c.collectBody(ctx, f, cls, bn.Body, scope)
	}
}

// memberTypes reads member types: one `#: T` per member line, else a
// trailing `#: [A, B]` tuple on the assignment.
func (c *Compiler) memberTypes(f *File, n *parser.ConstantWriteNode, call *parser.CallNode, members []string) []rbs.Type {
	args := callArgs(call)
	perLine := true
	types := make([]rbs.Type, len(members))
	for i, a := range args {
		ln := f.line(a.GetLocation().StartOffset)
		t, ok := f.trailing[ln]
		if !ok || ln == f.line(n.Location.StartOffset) || (i > 0 && ln == f.line(args[i-1].GetLocation().StartOffset)) {
			perLine = false
			break
		}
		rt, err := rbs.ParseType(t)
		if err != nil {
			c.errorf(f, a, "%v", err)
		}
		types[i] = rt
	}
	if perLine {
		return types
	}
	ann := f.trailingAnnotation(n)
	if ann == "" {
		c.errorf(f, n, "%s needs member types: `:a, #: T` per line, or `#: [A, B]` after the definition", n.Name)
	}
	t, err := rbs.ParseType(ann)
	tup, ok := t.(rbs.Tuple)
	if err != nil || !ok || len(tup.Elems) != len(members) {
		c.errorf(f, n, "`#: %s` must be a %d-element tuple of member types", ann, len(members))
	}
	return tup.Elems
}

// valueClassSource is the Ruby for a Struct's or Data's generated members.
func valueClassSource(kind, full string, members []string, types []rbs.Type) string {
	var b strings.Builder
	b.WriteString("class GeneratedValueClass\n")
	accessor := map[string]string{"struct": "attr_accessor", "data": "attr_reader"}[kind]
	for i, m := range members {
		fmt.Fprintf(&b, "  %s :%s #: %s\n", accessor, m, types[i])
	}
	// A struct's trailing nilable members may be omitted (Ruby fills in
	// nil); every Data member is required.
	firstOpt := len(members)
	for kind == "struct" && firstOpt > 0 && nilableRBS(types[firstOpt-1]) {
		firstOpt--
	}
	// Parameters are __v0, __v1, ... and members are read as self.m, so
	// a member may be named like a keyword (:end) or like a generated
	// parameter (:other).
	sigs := make([]string, len(members))
	params := make([]string, len(members))
	vars := make([]string, len(members))
	syms := make([]string, len(members))
	reads := make([]string, len(members))
	eqs := make([]string, len(members))
	insp := make([]string, len(members))
	pairs := make([]string, len(members))
	for i, m := range members {
		vars[i] = fmt.Sprintf("__v%d", i)
		sigs[i], params[i] = types[i].String(), vars[i]
		if i >= firstOpt {
			sigs[i], params[i] = "?"+sigs[i], vars[i]+" = nil"
		}
		syms[i] = ":" + m
		reads[i] = "self." + m
		eqs[i] = "self." + m + " == __other." + m
		insp[i] = m + "=#{self." + m + ".inspect}"
		pairs[i] = m + ": self." + m
	}
	fmt.Fprintf(&b, "  #: (%s) -> void\n  def initialize(%s)\n", strings.Join(sigs, ", "), strings.Join(params, ", "))
	for i, m := range members {
		fmt.Fprintf(&b, "    @%s = %s\n", m, vars[i])
	}
	b.WriteString("  end\n")
	fmt.Fprintf(&b, "  #: () -> Array[Symbol]\n  def self.members = [%s]\n", strings.Join(syms, ", "))
	fmt.Fprintf(&b, "  #: () -> Array[Symbol]\n  def members = [%s]\n", strings.Join(syms, ", "))
	fmt.Fprintf(&b, "  #: () -> Hash[Symbol, untyped]\n  def to_h = { %s }\n", strings.Join(pairs, ", "))
	// MRI's == wants the same class, not a subclass
	fmt.Fprintf(&b, "  #: (untyped) -> bool\n  def ==(__other)\n    return false unless __other.is_a?(::%s)\n    return false unless __other.class.equal?(self.class)\n    %s\n  end\n", full, strings.Join(eqs, " && "))
	fmt.Fprintf(&b, "  #: () -> String\n  def inspect = \"#<%s #{self.class.name} %s>\"\n", kind, strings.Join(insp, ", "))
	b.WriteString("  #: () -> String\n  def to_s = inspect\n")
	if kind == "struct" {
		fmt.Fprintf(&b, "  #: () -> Array[untyped]\n  def to_a = [%s]\n", strings.Join(reads, ", "))
	} else {
		// `with(k: v)` compiles to this: a copy of the receiver's class
		fmt.Fprintf(&b, "  #: (%s) -> ::%s\n  def __with(%s) = self.class.new(%s)\n", strings.Join(sigs, ", "), full, strings.Join(vars, ", "), strings.Join(vars, ", "))
	}
	b.WriteString("end\n")
	return b.String()
}

func nilableRBS(t rbs.Type) bool {
	switch t.(type) {
	case rbs.Optional, rbs.Nil, rbs.Untyped:
		return true
	}
	return false
}

// valueRoot finds the Struct/Data class that declared cls's members.
func (c *Class) valueRoot() *Class {
	for k := c; k != nil; k = k.Super {
		if k.valueMembers != nil {
			return k
		}
	}
	return nil
}

// descendantDefines reports whether a subclass of c defines a public name.
func (c *Class) descendantDefines(name string) bool {
	for _, sub := range c.Subclasses {
		if sub.metaOf != nil && c.metaOf == nil {
			continue
		}
		if m := sub.Methods[name]; m != nil && !m.Private {
			return true
		}
		if sub.descendantDefines(name) {
			return true
		}
	}
	return false
}
