package compiler

import (
	"context"
	"fmt"
	"regexp"
	"slices"
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
	superRef   *constRef   // superclass expression, resolved in link
	reSupers   []*constRef // superclasses named when reopening; must resolve to Super
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
	slotsLinked   bool   // linkOverrides ran
	Display       string // the name Ruby shows, when the declaration has none of its own (a describe's class)
	specChild     bool   // a nested describe's class: MRI undefines the test methods it inherits
	specTests     int    // its it/specify count, for MRI's test_0001_ names
}

// displayName is the class's name as Ruby shows it.
func (c *Class) displayName() string {
	if c.Display != "" {
		return c.Display
	}
	return c.RubyName
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
	guarded   bool // may be read before its assignment runs (guardConsts)
}

// Ivar is an instance variable of a struct class.
type Ivar struct {
	Name  string // with @
	Type  Type
	Owner *Class
	open  bool      // first assigned an unannotated `[]`/`{}` (refineIvars)
	elems [2][]Type // what the class's methods put in it
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
	seqAdapter bool   // a closure overriding an iterator: GoName gains _blk, an iter.Seq adapter keeps the name
	shadowed   []slot // ancestors' interface slots this override's signature differs from, nearest first; adapters answer them
	BlockParam string // name of an explicit &block parameter
	resolved   bool
	inherited  *Method // signature source for unannotated overrides
	inferRet   bool    // no return annotation: Ret comes from the body (inferRet)
	inferring  bool
	structDef  *Method // the generated Struct/Data method a block def overrides; super reaches it

	calleeDefaults bool // Ruby runs defaults in the callee: Go takes rbArgc first, callers pass zero values for the rest
	superBridge    bool // a module method whose `super` target depends on the includer (superBridges)
	quietDynamic   bool // prelude `# @dynamic`: dynamic by design, so no dynamic-call warnings (decision 78)
	specForm       bool // declared by a Minitest::Spec DSL call, not a def (decision 83)
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
	// Want is T for a `T | untyped` parameter (Type is untyped): typed
	// arguments are checked against T, untyped ones pass as they are.
	Want Type
}

// BlockSig is the block a method takes.
type BlockSig struct {
	Params   []Type
	Ret      Type
	Optional bool // `?{ ... }`: a call may leave the block out, and the method sees a nil func
}

func (m *Method) generic() bool { return len(m.TypeParams) > 0 }

// slot is method entry e as its Go signature appears in class in's interface.
type slot struct {
	e  entry
	in *Class
}

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

// includedBelow reports whether a subclass of c includes mod.
func (c *Class) includedBelow(mod *Class) bool {
	for _, sub := range c.Subclasses {
		if sub.isSubclassOf(mod) || sub.includedBelow(mod) {
			return true
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
			if !f.prelude && isDescribe(n) {
				c.collectDescribe(ctx, f, nil, n)
				continue
			}
			if n.Receiver == nil && n.Name == "require" {
				// stdlib requires are meaningless here, unless the prelude
				// hooks one (decision 78): `require "a/b"` → __require_a_b
				if hook := c.requireHook(f, n); hook != nil {
					c.addMainStmt(f, hook)
				}
				continue
			}
			c.addMainStmt(f, n)
		default:
			c.addMainStmt(f, n)
		}
	}
}

// requireHook is the call to Kernel#__require_<lib> that stands in for a
// user file's top-level `require "lib"`, or nil when the prelude has none.
func (c *Compiler) requireHook(f *File, n *parser.CallNode) *parser.CallNode {
	if f.prelude || n.Arguments == nil || len(n.Arguments.Arguments) != 1 {
		return nil
	}
	lib, ok := n.Arguments.Arguments[0].(*parser.StringNode)
	if !ok {
		return nil
	}
	name := "__require_" + strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			return r
		}
		return '_'
	}, lib.Unescaped.Value)
	if k := c.classes["Kernel"]; k == nil || k.Methods[name] == nil {
		return nil
	}
	hook := *n
	hook.Name = name
	hook.Arguments = nil
	return &hook
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

// goDecl matches a package-level Go name declared in a top-level %x{}.
var goDecl = regexp.MustCompile(`(?m)^\s*(?:func|type|var|const)\s+([A-Z]\w*)`)

// nameGo keeps user classes and constants off every other package-level Go
// name: runtime helpers (`Opt`) and what other classes generate (`NewUser`,
// `ShapeI`, `Foo::Bar`'s `Foo_Bar`). Prelude names are fixed, since `%x{}`
// spells them; a clashing user name gains `_` until it is free.
func (c *Compiler) nameGo() {
	taken := map[string]bool{"Tuple2": true, "Tuple3": true}
	for _, v := range c.verbatim {
		for _, m := range goDecl.FindAllStringSubmatch(v.code, -1) {
			taken[m[1]] = true
		}
	}
	claim := func(f *File, name *string, gen func(string) []string) {
		for !f.prelude && slices.ContainsFunc(gen(*name), func(n string) bool { return taken[n] }) {
			*name += "_"
		}
		for _, n := range gen(*name) {
			taken[n] = true
		}
	}
	for _, cls := range c.classList {
		claim(cls.File, &cls.Name, cls.goNames)
	}
	for _, k := range c.constList {
		claim(k.File, &k.GoName, func(n string) []string { return []string{n} })
	}
}

// goNames lists the package-level Go names generated for cls if its Go
// name is n: type, interface, constructor, metaclass and free functions.
func (cls *Class) goNames(n string) []string {
	out := make([]string, 0, 8+len(cls.MethodList)+len(cls.singletonDefs))
	out = append(out, n, n+"I", "New"+n, n+"_Self", n+"_Any", n+"_class", n+"_Meta", n+"_MetaI")
	for _, m := range cls.MethodList {
		out = append(out, n+"_"+m.GoName)
	}
	for _, d := range cls.singletonDefs {
		out = append(out, n+"_Meta_"+goMethodName(d.node.Name))
	}
	return out
}

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
	reopen := c.classes[name] != nil
	cls := c.declareClass(f, name, line, false)
	if n.Superclass != nil {
		// The superclass expression is evaluated outside the class body.
		ref := &constRef{node: n.Superclass, scope: scope, file: f}
		if reopen {
			// the first declaration fixed the superclass (Object when it named none)
			cls.reSupers = append(cls.reSupers, ref)
		} else {
			cls.superRef = ref
		}
	}
	if !reopen && !f.prelude {
		c.hooks = append(c.hooks, classHook{name: "inherited", cls: cls, node: n, file: f})
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
			c.collectClassCall(ctx, f, cls, n, &private, scope)
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
func (c *Compiler) collectClassCall(ctx context.Context, f *File, cls *Class, n *parser.CallNode, private *bool, scope []*Class) {
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
			if r, ok := a.(*parser.ConstantReadNode); ok && r.Name == "Singleton" && !cls.IsModule {
				c.addSingletonInstance(ctx, f, n, cls, scope)
			}
		}
		c.noteHooks(f, "included", cls, n, args, scope)
	case "extend":
		for _, a := range args {
			c.addInclude(f, n, cls, a, scope)
			last := len(cls.Includes) - 1
			cls.extends = append(cls.extends, cls.Includes[last])
			cls.Includes = cls.Includes[:last]
		}
		c.noteHooks(f, "extended", cls, n, args, scope)
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
		if specForms[n.Name] && !f.prelude {
			c.collectSpecForm(ctx, f, cls, n, scope)
			return
		}
		c.errorf(f, n, "unsupported call in class body: %s", n.Name)
	}
}

// addSingletonInstance gives a class that includes Singleton its
// `instance`, memoized in a class-level ivar (decision 54).
func (c *Compiler) addSingletonInstance(ctx context.Context, f *File, n *parser.CallNode, cls *Class, scope []*Class) {
	src := fmt.Sprintf("#: () -> %s\ndef self.instance = @__singleton_instance ||= new\n", cls.RubyName)
	sf, err := parseFile(ctx, c.parser, f.Name, []byte(src), f.prelude)
	if err != nil {
		c.errorf(f, n, "%v", err)
	}
	c.addMethod(sf, cls, sf.Root.Statements.Body[0].(*parser.DefNode), false, scope)
}

// classHook is a hook MRI calls while it evaluates a class body in main.rb:
// Super.inherited(cls) where the class is first opened, Mod.included(cls)
// and Mod.extended(cls) at the include/extend.
type classHook struct {
	name string
	cls  *Class
	mod  *constRef // nil: the superclass
	node parser.Node
	file *File // where the hook site is, for its place in load order
}

// addMainStmt records a top-level statement and the file it runs in.
func (c *Compiler) addMainStmt(f *File, n parser.Node) {
	if c.stmtFile == nil {
		c.stmtFile = map[parser.Node]*File{}
	}
	c.stmtFile[n] = f
	c.mainStmts = append(c.mainStmts, n)
}

// noteHooks records `include`/`extend`'s hooks: the last module first, as
// MRI includes them.
func (c *Compiler) noteHooks(f *File, name string, cls *Class, n *parser.CallNode, args []parser.Node, scope []*Class) {
	if f.prelude {
		return
	}
	for _, a := range slices.Backward(args) {
		c.hooks = append(c.hooks, classHook{name: name, cls: cls, mod: &constRef{node: a, scope: scope, file: f}, node: n, file: f})
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
	if _, ok := f.annotations(line)["dynamic"]; ok {
		if !f.prelude {
			c.errorf(f, n, "@dynamic is only allowed in the prelude")
		}
		m.quietDynamic = true
	}
	if body, ok := n.Body.(*parser.StatementsNode); ok && f.prelude && len(body.Body) == 1 {
		if _, ok := body.Body[0].(*parser.XStringNode); ok {
			m.Kind = kindPrimitive
		}
	}
	if cls == nil {
		m.GoName = goFuncName(n.Name)
		if old := c.topDefs[n.Name]; old != nil { // as Ruby: a later def replaces an earlier one
			c.topDefList = deleteMethod(c.topDefList, old)
		}
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

// superclassOf resolves cls's superclass (nil for modules and BasicObject)
// and checks that every reopening that names one names the same class.
func (c *Compiler) superclassOf(cls *Class) *Class {
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
	for _, r := range cls.reSupers {
		if c.resolveClassRef(r) != sup {
			c.errorf(r.file, r.node, "class %s reopened with a different superclass", cls.RubyName)
		}
	}
	return sup
}

func (c *Compiler) link() {
	for _, cls := range c.classList {
		if sup := c.superclassOf(cls); sup != nil {
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
	c.checkSpecUses() // before signatures: a bad let name would otherwise surface as an override mismatch
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
			nameCmpNil(m)
		}
	}
	for _, m := range c.topDefList {
		c.resolveMethod(m)
	}
	c.markCalleeDefaults()
	c.markSuperBridges()
	for _, cls := range c.classList {
		for _, m := range cls.MethodList {
			if c.overridesIterator(m) {
				m.seqAdapter = true
				m.GoName += "_blk"
			}
		}
	}
	for _, cls := range c.classList {
		c.linkOverrides(cls)
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

// markSuperBridges flags module methods whose `super` finds nothing in the
// module itself: the target is whatever follows the module in each
// includer's ancestors. Object's fallbacks (decision 31) stay static.
func (c *Compiler) markSuperBridges() {
	for _, mod := range c.classList {
		if !mod.IsModule || mod.universal {
			continue
		}
		for _, m := range mod.MethodList {
			if m.Kind != kindDef || m.Block != nil || m.generic() || m.Name == "method_missing" || m.Name == "respond_to_missing?" {
				continue
			}
			m.superBridge = containsSuper(m.Node.Body) && c.inheritedSig(m) == nil
		}
	}
}

// defsOf lists every definition of name in c's lookup order (methodSet
// without the shadowing): a module method's `super` target follows it.
func (c *Class) defsOf(name string) []entry {
	var out []entry
	if m := c.Methods[name]; m != nil {
		identity := map[string]Type{}
		for _, p := range c.TypeParams {
			identity[p] = TVar{Name: p}
		}
		out = append(out, entry{M: m, Owner: c, Env: identity, Entry: c})
	}
	for i := len(c.Includes) - 1; i >= 0; i-- {
		inc := c.Includes[i]
		env := map[string]Type{}
		for j, p := range inc.Mod.TypeParams {
			if j < len(inc.Args) {
				env[p] = inc.Args[j]
			}
		}
		for _, e := range inc.Mod.defsOf(name) {
			e2 := entry{M: e.M, Owner: e.Owner, Env: composeEnv(e.Env, env), Entry: c}
			if c.IsModule {
				e2.Entry = e.Entry
			}
			out = append(out, e2)
		}
	}
	if c.Super != nil {
		out = append(out, c.Super.defsOf(name)...)
	}
	return out
}

// markCalleeDefaults: only a literal default means the same at the call site; defs along an ancestor chain share one Go signature, so the mark spreads (universal free funcs take `any` and need not match).
func (c *Compiler) markCalleeDefaults() {
	names := map[string]bool{}
	for _, m := range c.topDefList {
		m.calleeDefaults = m.Kind == kindDef && scopedDefault(m)
	}
	for _, cls := range c.classList {
		for _, m := range cls.MethodList {
			if !cls.universal && m.Kind == kindDef && scopedDefault(m) {
				m.calleeDefaults = true
				names[m.Name] = true
			}
		}
	}
	for changed := len(names) > 0; changed; {
		changed = false
		for _, cls := range c.classList {
			for _, anc := range cls.allAncestors() {
				for name := range names {
					m, a := cls.Methods[name], anc.Methods[name]
					if cls.universal || anc.universal || m == nil || a == nil || m.Kind != kindDef || a.Kind != kindDef || m.calleeDefaults == a.calleeDefaults {
						continue
					}
					m.calleeDefaults, a.calleeDefaults, changed = true, true, true
				}
			}
		}
	}
	for _, cls := range c.classList {
		if cls.metaOf == nil {
			continue
		}
		if m, init := cls.Methods["new"], cls.metaOf.lookup("initialize"); m != nil && m.Kind == kindSynth && init != nil {
			m.calleeDefaults = init.M.calleeDefaults
		}
	}
}

// scopedDefault reports whether some default of m is not a plain literal.
func scopedDefault(m *Method) bool {
	for _, p := range m.Params {
		if p.Default != nil && !literalDefault(p.Default) {
			return true
		}
	}
	return false
}

// literalDefault reports whether a default means the same in any scope.
func literalDefault(n parser.Node) bool {
	switch n := n.(type) {
	case *parser.IntegerNode, *parser.FloatNode, *parser.StringNode, *parser.SymbolNode,
		*parser.NilNode, *parser.TrueNode, *parser.FalseNode, *parser.RegularExpressionNode:
		return true
	case *parser.ArrayNode:
		for _, e := range n.Elements {
			if !literalDefault(e) {
				return false
			}
		}
		return true
	case *parser.HashNode:
		for _, e := range n.Elements {
			if a, ok := e.(*parser.AssocNode); !ok || !literalDefault(a.Key) || !literalDefault(a.Value) {
				return false
			}
		}
		return true
	}
	return false
}

// allAncestors: superclasses and included modules above c, transitively,
// with repeats, universal ones included (unlike ancestors).
func (c *Class) allAncestors() []*Class {
	var out []*Class
	for _, inc := range c.Includes {
		if inc.Mod != nil {
			out = append(append(out, inc.Mod), inc.Mod.allAncestors()...)
		}
	}
	if c.Super != nil {
		out = append(append(out, c.Super), c.Super.allAncestors()...)
	}
	return out
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
		return optOf(c.resolveType(t.Elem, sc))
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
	case rbs.Proc:
		ps := make([]Type, len(t.Params))
		for i, p := range t.Params {
			ps[i] = c.resolveType(p, sc)
		}
		return TFunc{Params: ps, Ret: c.resolveType(t.Ret, sc), Proc: true}
	}
	c.errorf(sc.file, nil, "%s:%d: unsupported type %s", sc.file.Name, sc.line, t)
	return nil
}

// nameCmpNil gives a `<=>` returning Integer? the Go name cmpNil: Op_cmp is
// the Integer one Comparable and rbCmp call (emitCmpAdapter).
func nameCmpNil(m *Method) {
	if r, ok := m.Ret.(TOpt); ok && m.Name == "<=>" && isClass(r.Elem, "Integer") {
		m.GoName = "cmpNil"
	}
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
		m.sigText, m.inferRet = c.paramSig(m)
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
		prm := Param{Name: p.Name, Rest: p.Rest}
		if want := gradualParam(p.Type); want != nil {
			prm.Type, prm.Want = TAny{}, c.resolveType(want, sc)
		} else {
			prm.Type = c.resolveType(p.Type, sc)
		}
		m.Params = append(m.Params, prm)
	}
	if m.sig.Block != nil {
		bs := &BlockSig{Ret: c.resolveType(m.sig.Block.Return, sc), Optional: m.sig.Block.Optional}
		for _, p := range m.sig.Block.Params {
			bs.Params = append(bs.Params, c.resolveType(p.Type, sc))
		}
		m.Block = bs
	}
	m.Ret = c.resolveType(m.sig.Return, sc)
	if m.inferRet {
		m.Ret = nil
	}
	if m.Block != nil {
		m.Iterator = c.isIterator(m, m.Block)
	}
	c.bindParamNames(m)
}

// gradualParam returns T for a `T | untyped` parameter type, else nil.
func gradualParam(t rbs.Type) rbs.Type {
	u, ok := t.(rbs.Union)
	if !ok || len(u.Elems) != 2 {
		return nil
	}
	if _, ok := u.Elems[1].(rbs.Untyped); !ok {
		return nil
	}
	return u.Elems[0]
}

// paramSig builds a def's signature from rbs-inline's per-parameter form
// (`# @rbs x: T`, `# @rbs *xs: T`, `# @rbs return: T`). Without a return
// annotation the return type is inferred from the body (inferRet). A def
// with no parameters needs no annotation at all.
func (c *Compiler) paramSig(m *Method) (string, bool) {
	ann := m.File.annotations(m.Line)
	names, defaults, rest := c.defParams(m, m.Node.Parameters)
	var ps []string
	for i, n := range names {
		t := c.paramAnn(m, ann, n)
		if defaults[i] != nil {
			t = "?" + t
		}
		ps = append(ps, t)
	}
	if rest != "" {
		ps = append(ps, "*"+c.paramAnn(m, ann, "*"+rest))
	}
	sig := "(" + strings.Join(ps, ", ") + ") -> "
	if r := ann["return:"]; len(r) > 0 {
		return sig + r[0], false
	}
	return sig + "untyped", true
}

func (c *Compiler) paramAnn(m *Method, ann map[string][]string, name string) string {
	t := ann[name+":"]
	if len(t) == 0 {
		c.errorf(m.File, m.Node, "method %s has no type for parameter %s (`# @rbs %s: T` or `#: (...) -> T`)", m.Name, strings.TrimPrefix(name, "*"), name)
	}
	return t[0]
}

// inferRet types an unannotated return from the body: a dry run of its
// generation whose returned values are joined. Recursion needs an
// annotation. Before ivar discovery finishes the result is provisional
// (discoverIvars clears it).
func (c *Compiler) inferRet(m *Method) {
	if !m.inferRet || m.Ret != nil {
		return
	}
	if m.inferring {
		c.errorf(m.File, m.Node, "method %s is recursive; annotate its return type (`# @rbs return: T`)", m.Name)
	}
	m.inferring = true
	defer func() { m.inferring = false }()
	if m.inherited != nil {
		c.inferRet(m.inherited)
		m.Ret = m.inherited.Ret
		return
	}
	nw := len(c.Warnings)
	f := c.newFctx(m.File, m.Owner, m)
	f.retVar = "ret_"
	var ts []Type
	f.retTypes = &ts
	f.genBody(m.Node.Body, c.paramLocals(m), tail{kind: tailReturn, types: &ts}, nil)
	c.dropWarnings(nw) // emitMethod warns for real
	var ret Type = TNil{}
	for i, t := range ts {
		if isVoid(t) {
			t = TNil{}
		}
		if i == 0 {
			ret = t
			continue
		}
		j, ok := join(ret, t)
		if !ok {
			c.errorf(m.File, m.Node, "method %s returns both %s and %s; annotate its return type (`# @rbs return: T`)", m.Name, ret, t)
		}
		ret = j
	}
	m.Ret = ret
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
		m.Params = append(m.Params, Param{Name: p.Name, Type: subst(p.Type, e.Env), Default: p.Default, Rest: p.Rest, Want: subst(p.Want, e.Env)})
	}
	if e.M.Block != nil {
		m.Block = &BlockSig{Params: substAll(e.M.Block.Params, e.Env), Ret: subst(e.M.Block.Ret, e.Env)}
	}
	m.Ret = subst(e.M.Ret, e.Env)
	m.inferRet = e.M.inferRet // the parent's inferred type, once it has one (inferRet)
	// decision 4 holds for the override's own body: a rescue around yield makes it a closure
	m.Iterator = e.M.Iterator && (m.Kind != kindDef || !containsRescueClause(m.Node.Body))
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
	if !isVoid(bs.Ret) || m.sig.Block.Optional || len(bs.Params) == 0 || len(bs.Params) > 2 {
		return false // an iter.Seq yields one or two values; other blocks are closures
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

// overridesIterator reports whether m takes its block as a closure yet
// overrides an iterator, e.g. an `each` that rescues around yield under
// Enumerable's. Callers of the iterator (Enumerable's bodies, through its
// constraint) keep its Go name, now an iter.Seq adapter over the closure.
func (c *Compiler) overridesIterator(m *Method) bool {
	if m.Owner == nil || m.Block == nil || m.Iterator || !isVoid(m.Block.Ret) {
		return false
	}
	e := c.inheritedSig(m)
	return e != nil && e.M.Block != nil && (e.M.Iterator || c.overridesIterator(e.M))
}

// linkOverrides names the Go slot each override of a struct class fills. An
// override with the parent's Go signature takes the parent's Go name; one
// that differs (another arity, a narrower return) gets its own name, and the
// class keeps answering to the parent's slot through an adapter (decision 8).
func (c *Compiler) linkOverrides(cls *Class) {
	sup := cls.Super
	if cls.slotsLinked || !cls.isStruct() || cls.universal || sup == nil || sup.universal {
		return
	}
	cls.slotsLinked = true
	c.linkOverrides(sup)
	for _, m := range cls.MethodList {
		pe := sup.lookup(m.Name)
		if pe == nil || !inInterface(m) || !inInterface(pe.M) || m.seqAdapter != pe.M.seqAdapter {
			continue
		}
		own := entry{M: m, Owner: cls, Env: map[string]Type{}, Entry: cls}
		if c.slotKey(slot{own, cls}) == c.slotKey(slot{*pe, sup}) {
			m.GoName = pe.M.GoName
			m.shadowed = pe.M.shadowed
			continue
		}
		// `_` + lowercase never comes out of camel-casing (decision 3)
		m.GoName = goMethodName(m.Name) + "_of" + cls.Name
		m.shadowed = append([]slot{{*pe, sup}}, pe.M.shadowed...)
		for _, s := range m.shadowed {
			c.checkAdaptable(slot{own, cls}, s)
		}
	}
}

// inInterface reports whether m is a slot of its owner's Go interface.
func inInterface(m *Method) bool {
	return !m.Private && !m.generic() && m.Name != "initialize" && (m.Owner.metaOf == nil || m.Name != "new")
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
	if g := m.structDef; g != nil {
		return &entry{M: g, Owner: cls, Env: map[string]Type{}, Entry: cls}
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
	if len(cls.TypeParams) > 0 && len(cls.singletonDefs) > 0 && cls.GoType == "" {
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
		if m.Methods[name] != nil || inheritsUserDef(sup, name) {
			continue
		}
		sm := &Method{Name: name, GoName: goMethodName(name), Owner: m, Kind: kindSynth, File: cls.File, Line: cls.Line}
		m.Methods[name] = sm
		m.MethodList = append(m.MethodList, sm)
	}
	return m
}

// inheritsUserDef: a user-defined class-level name/to_s/inspect wins over a subclass's generated one.
func inheritsUserDef(m *Class, name string) bool {
	for ; m != nil && m.metaOf != nil; m = m.Super {
		if d := m.Methods[name]; d != nil && d.Kind != kindSynth {
			return true
		}
	}
	return false
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
		m.Params = append(m.Params, Param{Name: p.Name, Type: subst(p.Type, env), Default: p.Default, Rest: p.Rest, Want: subst(p.Want, env)})
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
		gen := slices.Clone(cls.MethodList)
		c.collectBody(ctx, f, cls, bn.Body, scope)
		// MRI defines the accessors on the new class but the rest on
		// Struct/Data, so a block def overrides those and super reaches
		// them: keep each one under a hidden private name.
		for _, g := range gen {
			m := cls.Methods[g.Name]
			if m == g || g.Kind != kindDef {
				continue
			}
			m.structDef = g
			g.Name, g.GoName, g.Private = "__struct_"+g.Name, g.GoName+"_struct", true
			cls.Methods[g.Name] = g
			cls.MethodList = append(cls.MethodList, g)
		}
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
	// MRI's ==/eql? want the same class, not a subclass
	fmt.Fprintf(&b, "  #: (untyped) -> bool\n  def ==(__other)\n    return false unless __other.is_a?(::%s)\n    return false unless __other.class.equal?(self.class)\n    %s\n  end\n", full, strings.Join(eqs, " && "))
	fmt.Fprintf(&b, "  #: (untyped) -> bool\n  def eql?(__other)\n    return false unless __other.is_a?(::%s)\n    return false unless __other.class.equal?(self.class)\n    to_h.eql?(__other.to_h)\n  end\n", full)
	b.WriteString("  #: () -> Integer\n  def hash = to_h.hash\n")
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

// descendantDefines reports whether a subclass of c defines a public name
// (or a private one, with private).
func (c *Class) descendantDefines(name string, private bool) bool {
	for _, sub := range c.Subclasses {
		if sub.metaOf != nil && c.metaOf == nil {
			continue
		}
		if m := sub.Methods[name]; m != nil && (private || !m.Private) {
			return true
		}
		if sub.descendantDefines(name, private) {
			return true
		}
	}
	return false
}
