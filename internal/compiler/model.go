package compiler

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/danielgatis/go-ruby-prism/parser"

	"github.com/jtarchie/ruby2go/internal/rbs"
)

// Class is a Ruby class or module.
type Class struct {
	Name       string // Go identifier: "Resty_Actions_Show"
	RubyName   string // constant path: "Resty::Actions::Show"
	IsModule   bool
	GoType     string // `@go_type` underlying Go type; "" for struct classes
	hashBase   bool   // a user class extending Hash or Array: embeds a Hash[any, V] or an Array[E] (decisions 165, 202)
	selfDefs   bool   // some method has `# @self` (selfOverload)
	TypeParams []string
	superRef   *constRef   // superclass expression, resolved in link
	reSupers   []*constRef // superclasses named when reopening; must resolve to Super
	Super      *Class
	Includes   []Include
	Methods    map[string]*Method
	MethodList []*Method
	Ivars      map[string]*Ivar
	IvarList   []*Ivar
	srcPos
	Subclasses []*Class
	// universal classes (BasicObject, Object, Kernel) are ancestors of the
	// primitive types too, so their free funcs take `Self any`.
	universal bool
	// ivar type annotations `# @rbs @x: T`, resolved in resolveSigs
	ivarDecls     []ivarDecl
	singletonDefs []singletonDef
	perRun        []string // a Class.new literal whose block runs statements: a fresh class object per evaluation, its body `self.rb2go_class_body` taking these captured locals (decision 259)
	isPerRun      bool
	singletonMisc []singletonMisc   // attr_*/alias inside `class << self`, applied to the metaclass once it exists
	privateNew    bool              // `private_class_method :new`
	undefs        map[string]bool   // `undef x`: calls through this class find nothing, even inherited
	cvars         map[string]*Const // @@x first assigned in this body, emitted as package variables
	extends       []Include         // `extend M`: included into the class object
	mixesIn       []Include         // Sorbet's `mixes_in_class_methods(M)` in a module: its includers extend M (decision 241)
	delegations   []delegation
	singleFwd     bool     // `extend SingleForwardable` seen: def_single_delegator(s) and single_delegate work
	fwdSingle     bool     // the later of Forwardable/SingleForwardable extended was SingleForwardable: def_delegator(s) and delegate define class methods
	meta          *Class   // the class object's class (holds `def self.` methods)
	metaOf        *Class   // for a metaclass: the class it describes
	constNames    []string // constants (classes included) declared directly inside, in order
	valueMembers  []string // Struct.new / Data.define members, in order
	valueKind     string   // "struct" or "data"
	kwInit        bool     // Struct.new(..., keyword_init: true): new takes keywords only
	msetCache     []entry
	msetIndex     map[string]int // name → index in msetCache; the dynamic pass asks every class about every name
	selfCallCache map[string]bool
	gemSelfCalls  map[string]bool  // what gem methods added to a core module call on self: dynamic, not in the constraint (selfCalls)
	scratchIvars  map[string]*Ivar // a gem module a value class (Pathname) includes: its ivars are per-call locals
	prepended     []*Class         // modules prepended to it, in prepend order: ancestors lists them before it, last first
	slotsLinked   bool             // linkOverrides ran
	Display       string           // the name Ruby shows, when the declaration has none of its own (a describe's class)
	specChild     bool             // a nested describe's class: MRI undefines the test methods it inherits
	specTests     int              // its it/specify count, for MRI's test_0001_ names
	hidden        bool             // `# @hidden` on a prelude module MRI lacks: left out of ancestors (decision 139)
	ivarBits      bool             // its objects record which ivars were assigned (decision 153)
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
	Mod      *Class
	Args     []Type
	fromEach bool // no type args written: they are the includer's each's block parameters (decision 146)
	ref      constRef
	args     []rbs.Type
	srcPos
	scope []*Class
}

// Const is a non-class constant (`VERSION = "1.0"`), emitted as a Go
// package-level variable.
type Const struct {
	RubyName string
	GoName   string
	Value    parser.Node
	srcPos
	Scope     []*Class // lexical scope of the assignment
	ann       string   // trailing `#: T`
	typeAlias string   // `X = T.type_alias { T }`: the RBS type X names in a sig (decision 196)
	poison    string   // a gem constant whose value does not compile: why; it stays nil (decision 205)
	Type      Type
	resolving bool
	guarded   bool // may be read before its assignment runs (guardConsts)
	inMethod  bool // a class variable first assigned in a method: no init in main, reads check its set flag
	inBody    bool // reads a local of its class body: assigned in the body's grouped statements (decision 170)
	initAt    *int // a constant lifted out of another's value: the offset it initializes at (decision 244)
}

// Ivar is an instance variable of a struct class.
type Ivar struct {
	Name    string // with @
	Type    Type
	Owner   *Class
	open    bool      // first assigned an unannotated `[]`/`{}` (refineIvars)
	elems   [2][]Type // what the class's methods put in it
	scratch bool      // a value-included gem module's: a local of each method (declareGemModuleIvars)
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
	ivarUse      *bool                // mentionsIvar's answer, once asked
	attrWriter   bool                 // an unannotated attr_writer/attr_accessor's `def x=(v) = @x = v` (addModuleAttr)
	pendingIdx   map[int]pendingParam // sig param index → a parameter typed from use (decision 146)
	pendingBlock []pendingParam       // the block's parameters, typed from the yields
	// yields that disagree on their count, reported at the yield (yieldShape)
	yieldDisagree   string
	yieldDisagreeAt parser.Node
	Name            string
	GoName          string
	Owner           *Class // nil for top-level defs
	Kind            methodKind
	Attr            string // ivar for attr kinds
	Private         bool
	Protected       bool // callable with an explicit receiver only from inside the owner's family
	Node            *parser.DefNode
	srcPos
	Scope       []*Class // lexical scope (Module.nesting), innermost last
	sigText     string
	declared    bool // a `#:` or Sorbet sig typed it: its param types are the author's, not guessed (decision 241)
	sig         *rbs.MethodType
	arms        []*rbs.MethodType // a `.rbs` sig's overloads; sig is their merge, the call site picks one (decision 12)
	TypeParams  []string
	Params      []Param
	Block       *BlockSig
	Ret         Type
	Iterator    bool   // block returns void → iter.Seq
	seqAdapter  bool   // a closure overriding an iterator: GoName gains _blk, an iter.Seq adapter keeps the name
	shadowed    []slot // ancestors' interface slots this override's signature differs from, nearest first; adapters answer them
	BlockParam  string // name of an explicit &block parameter
	forwardAll  bool   // `def f(...)`: its signature is its forwarding target's (resolveForwarding)
	resolved    bool
	inherited   *Method // signature source for unannotated overrides
	inferRet    bool    // no return annotation: Ret comes from the body (inferRet)
	inferring   bool
	structDef   *Method // the generated Struct/Data method a block def overrides; super reaches it
	superTarget *Method // a prepended module's copy: the class's own method it wraps, which super calls (decision 199)
	fwdTarget   *Class  // a delegation's `(...)` forwarder: the class it forwards to
	poison      string  // a gem method whose body does not compile: why; its body raises NotImplementedError (decision 205)
	freeName    string  // its free func's name when Owner_Name is taken by a class (separateFreeFuncs)
	valueGen    bool    // generated for Struct/Data: a pattern reads the members directly (decision 143)

	calleeDefaults bool // Ruby runs defaults in the callee: Go takes rbArgc first, callers pass zero values for the rest
	superBridge    bool // a module method whose `super` target depends on the includer (superBridges)
	quietDynamic   bool // prelude `# @dynamic`: dynamic by design, so no dynamic-call warnings (decision 78)
	specForm       bool // declared by a Minitest::Spec DSL call, not a def (decision 83)
	noReturn       bool // RBS `bot`: a call ends its statement list, like raise (decision 91)
	selfText       string
	SelfType       Type // prelude `# @self T`: the receiver must unify with T, binding the method's type params (decision 92)
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
	Keyword bool // `name:`: passed by name, a positional Go param after the positional ones
	Post    bool // after *rest or the optional params (`def f(a, *r, z)`): bound from the last arguments
	KwRest  bool // `**opts`: the call's other keywords, a Hash[Symbol, T]
	// Want is T for a `T | untyped` parameter (Type is untyped): typed
	// arguments are checked against T, untyped ones pass as they are.
	Want Type
	// Pending keys a parameter typed from use (decision 146): its
	// arguments are recorded under it during inference rounds.
	Pending string
}

// BlockSig is the block a method takes.
type BlockSig struct {
	Params      []Type
	Ret         Type
	Optional    bool     // `?{ ... }`: a call may leave the block out, and the method sees a nil func
	Pending     []string // per param, the key of one typed from the yields (decision 146), or ""
	Rest        bool     // a lambda's: the last param is a `*rest`'s element type, a Go variadic (decision 152)
	Self        Type     // non-nil: the block escaped (stored, instance_eval'd) and takes `self` as a leading Go param (decision 164)
	SelfPending string   // the key Self's type is inferred from (decision 164): instance_eval records the receiver, Proc#call the lexical self
	Untyped     bool     // an unannotated def's &block (decision 183): calling it when missing is MRI's NoMethodError at run time
}

func (m *Method) generic() bool { return len(m.TypeParams) > 0 }

// slot is method entry e as its Go signature appears in class in's interface.
type slot struct {
	e   entry
	in  *Class
	dyn bool // a gem override no typed adapter fits: the slot boxes its arguments into a dynamic call (decision 249)
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

func (c *Class) lookup(name string) *entry {
	set := c.methodSet()
	if c.msetIndex == nil {
		c.msetIndex = make(map[string]int, len(set))
		for i, e := range set {
			c.msetIndex[e.M.Name] = i
		}
	}
	if i, ok := c.msetIndex[name]; ok {
		e := set[i]
		for k := c; k != nil && k != e.Owner; k = k.Super {
			if k.undefs[name] {
				return nil
			}
		}
		return &e
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

// structChain is c and its superclasses up to (excluding) Object, nearest first; mixins are not included.
func (c *Class) structChain() []*Class {
	var out []*Class
	for k := c; k != nil && !k.universal; k = k.Super {
		out = append(out, k)
	}
	return out
}

// collect declares the classes, modules, methods and constants of f's top-level statements (the first pass over a file).
func (c *Compiler) collect(ctx context.Context, f *File) {
	if c.gemFile(f) {
		lenientBlockParams(f.Root)
	}
	queue := f.Root.Statements.Body
	for len(queue) > 0 {
		n := queue[0]
		queue = queue[1:]
		c.keepGoing(func() {
			switch n := n.(type) {
			case *parser.IfNode, *parser.UnlessNode: // a condition known at compile time keeps one branch (decision 167)
				if branch, ok := c.takenBranch(f, n, nil); ok {
					queue = append(slices.Clip(branch), queue...)
					return
				}
				c.scanAnon(ctx, f, n, nil)
				c.addMainStmt(f, n)
			case *parser.ClassNode:
				c.collectClass(ctx, f, n, nil)
			case *parser.ModuleNode:
				c.collectModule(ctx, f, n, nil)
			case *parser.DefNode:
				c.collectTopDef(f, n)
				c.scanAnon(ctx, f, n.Body, nil)
			case *parser.ConstantWriteNode:
				if call, kind := valueClass(n.Value); call != nil {
					c.collectValueClass(ctx, f, n, call, kind, nil)
					return
				}
				if call, isModule := anonClassCall(n.Value); call != nil && !f.isPrelude() {
					c.collectNamedAnon(ctx, f, n, call, isModule, nil)
					return
				}
				c.scanAnon(ctx, f, n.Value, nil)
				c.addConst(f, n, n.Name, nil, nil)
			case *parser.ConstantPathWriteNode: // `Rack::X = v` at the top level
				c.scanAnon(ctx, f, n.Value, nil)
				c.addConstAt(f, n, c.declName(f, n.Target, nil), n.Value, nil)
			case *parser.PreExecutionNode: // BEGIN runs before the rest of its file, in the top-level scope
				if c.beginStmts == nil {
					c.beginStmts = map[*File][]parser.Node{}
				}
				if n.Statements != nil {
					c.beginStmts[f] = append(c.beginStmts[f], n.Statements.Body...)
				}
			case *parser.XStringNode:
				if !f.isPrelude() {
					c.errorf(f, n, "top-level %%x{} is only allowed in the prelude")
				}
				c.verbatim = append(c.verbatim, verbatim{srcPos: srcPos{f, f.line(n.Location.StartOffset)}, code: n.Unescaped.Value})
			case *parser.CallNode:
				c.collectTopCall(ctx, f, n)
			case *parser.BeginNode:
				c.addMainStmt(f, c.beginRequires(ctx, f, n))
			default:
				c.scanAnon(ctx, f, n, nil)
				c.addMainStmt(f, n)
			}
		})
	}
}

func callArgs(n *parser.CallNode) []parser.Node {
	if n.Arguments == nil {
		return nil
	}
	return n.Arguments.Arguments
}

type singletonDef struct {
	node    *parser.DefNode
	private bool
	scope   []*Class
	file    *File
}

// singletonMisc is an attr_* call or an alias inside `class << self`.
type singletonMisc struct {
	node    parser.Node
	private bool
	scope   []*Class
	file    *File
}

func (c *Compiler) link(ctx context.Context) {
	for _, cls := range c.classList {
		if sup := c.superclassOf(cls); sup != nil {
			cls.Super = sup
			sup.Subclasses = append(sup.Subclasses, cls)
		}
		for i := range cls.Includes {
			c.linkInclude(cls, &cls.Includes[i])
		}
	}
	c.checkCycles()
	c.checkNumeric()
	c.buildMetas()
	c.applyAliasNews(ctx)
	c.checkSpecUses() // before signatures: a bad let name would otherwise surface as an override mismatch
	for _, cls := range c.classList {
		if cls.GoType != "" && cls.Super != nil && cls.Super.isStruct() && !cls.Super.universal {
			c.errorAt(cls.srcPos, "@go_type class %s cannot inherit from struct class %s", cls.Name, cls.Super.Name)
		}
		if cls.isStruct() && !cls.universal && len(cls.TypeParams) > 0 {
			c.errorAt(cls.srcPos, "generic struct classes are not supported")
		}
	}
	c.addSigIvars()
	c.inheritRedeclaredReaders()
	c.declareIvarAnnotations()
	c.checkIvarModules()
	c.expandDelegations(ctx)
	c.applyPrepends()
	c.linkAliases(ctx)
	c.separateFreeFuncs()
	c.methodClassVars()
	c.includeFromEach() // before any override inherits from the module, which needs its type args
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
			if c.overridesIterator(m) || c.blockEachOfEnumerable(cls, m) {
				m.seqAdapter = true
				m.GoName += "_blk"
			}
		}
	}
	c.publicGemOverrides()
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
			c.errorAt(m.srcPos, "`%s` and `%s` (%s) both become Go %s in %s; rename one", m.Name, p.Name, p.srcPos, m.GoName, where)
		}
		seen[m.GoName] = m
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

// hasKeywords reports keyword parameters (`a:` or `**opts`).
func (m *Method) hasKeywords() bool {
	return slices.ContainsFunc(m.Params, func(p Param) bool { return p.Keyword })
}

// optionalBlockLocal names the local that holds an optional block (`?{ }`)
// in a def: its &block name, or a hidden one for yield and block_given?.
// The local is a Proc?, so calling it unchecked is a compile error and
// `if block_given?` / `blk&.call` narrow it like any optional value.
func (m *Method) optionalBlockLocal() string {
	if m == nil || m.Block == nil || !m.Block.Optional || m.Kind != kindDef || m.Iterator {
		return ""
	}
	if m.BlockParam != "" {
		return m.BlockParam
	}
	return "__block"
}

// postCount is how many post params m has.
func (m *Method) postCount() int {
	n := 0
	for _, p := range m.Params {
		if p.Post {
			n++
		}
	}
	return n
}

// positionalCount is how many positional arguments m takes, *rest aside.
func (m *Method) positionalCount() int {
	n := 0
	for _, p := range m.Params {
		if !p.Keyword && !p.Rest {
			n++
		}
	}
	return n
}

// kwMask reports a Go `rbKw int` parameter: which optional keywords the
// caller passed, for defaults run in the callee (calleeDefaults).
func (m *Method) kwMask() bool {
	return m.calleeDefaults && slices.ContainsFunc(m.Params, func(p Param) bool { return p.Keyword && !p.KwRest && p.Default != nil })
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

// containerEnv adds the embedded container's fixed type args to env: a Hash subclass's keys are untyped (decision 165).
func (cls *Class) containerEnv(env map[string]Type) {
	if !cls.hashBase || cls.Super == nil {
		return
	}
	base := cls.Super
	for base.hashBase && base.Super != nil { // Concurrent::Set < CRubySet < ::Set: the container itself
		base = base.Super
	}
	if base.RubyName == "Hash" {
		env["K"] = TAny{}
		return
	}
	for _, p := range base.TypeParams { // an Array (Set, Queue) subclass's elements are untyped
		env[p] = TAny{}
	}
}

// containerTargs are the Go type args of the embedded container's free funcs for a subclass instantiated with args.
func (c *Compiler) containerTargs(cls *Class, args []Type) string {
	switch {
	case cls.Super.RubyName != "Hash":
		return anyArgs(len(cls.Super.TypeParams))
	case len(args) == 0:
		return ""
	}
	return "[any, " + c.goTypes(args) + "]"
}

// anyArgs is `[any, ...]` for n type params, or "".
func anyArgs(n int) string {
	if n == 0 {
		return ""
	}
	return "[" + strings.Repeat("any, ", n-1) + "any]"
}

// embeddedSuper is how code reaches a @go_type subclass's embedded base: its address for a mutable base (Hash,
// Array), the value itself for a value type (Pathname).
func embeddedSuper(cls *Class, recv string) string {
	if cls.Super.mutable() {
		return "(&" + recv + "." + superField(cls.Super) + ")"
	}
	return recv + "." + superField(cls.Super)
}
