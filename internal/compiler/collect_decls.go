package compiler

import (
	"context"
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/danielgatis/go-ruby-prism/parser"
)

// collectTopCall takes the top-level calls that act at compile time: require_relative, describe, include, require.
func (c *Compiler) collectTopCall(ctx context.Context, f *File, n *parser.CallNode) {
	switch {
	case n.Receiver == nil && n.Name == "require_relative" && f.isPrelude():
		c.requireRelative(ctx, f, n)
	case n.Receiver == nil && n.Name == "require_relative":
		if uf := c.userRequireRelative(ctx, f, n); uf != nil {
			c.addMainStmt(f, &loadFile{call: n, file: uf})
		}
	case !f.isPrelude() && isDescribe(n):
		c.collectDescribe(ctx, f, nil, n)
	case c.collectIOExtend(f, n, nil): // before objectConstExtend, which would drop `ENV.extend(EnvSensitive)` (decision 248)
	case c.topConstVisibility(f, n), c.objectConstExtend(f, n):
	case n.Receiver == nil && n.Name == "include" && !f.isPrelude():
		for _, a := range callArgs(n) { // main's include is Object's, as in MRI
			c.addInclude(f, n, c.classes["Object"], a, nil)
		}
	case n.Receiver == nil && n.Name == "require":
		// a file on the -I path loads like require_relative (decision 131);
		// stdlib requires are meaningless here, unless the prelude
		// hooks one (decision 78): `require "a/b"` → __require_a_b
		if uf, ok := c.userRequire(ctx, f, n); ok {
			if uf != nil {
				c.addMainStmt(f, &loadFile{call: n, file: uf})
			}
		} else if hook := c.requireHook(f, n); hook != nil {
			c.addMainStmt(f, hook)
		}
	case n.Name == "prepend" && !f.isPrelude() && prependTarget(n) != nil: // `X.prepend(M)`, `X.singleton_class.prepend(M)` (decision 199)
		recv, single := prependTarget(n), false
		if sc, ok := recv.(*parser.CallNode); ok {
			recv, single = sc.Receiver, true
		}
		for _, a := range callArgs(n) {
			c.prepends = append(c.prepends, pendingPrepend{targetRef: constRef{node: recv, file: f}, singleton: single, mod: constRef{node: a, file: f}})
		}
	case n.Receiver == nil && n.Name == "autoload":
		if sym, ok := symbolName(firstArg(callArgs(n))); ok {
			c.noteAutoload(sym)
		}
		c.loadAutoload(ctx, f, n) // compile-time autoload (decision 161)
	default:
		c.scanAnon(ctx, f, n, nil)
		c.addMainStmt(f, n)
	}
}

// beginRequires splices the files a top-level begin requires where they stand, as rackup.rb's `begin; require_relative ...; rescue LoadError`.
func (c *Compiler) beginRequires(ctx context.Context, f *File, n *parser.BeginNode) parser.Node {
	if f.isPrelude() || n.Statements == nil {
		c.scanAnon(ctx, f, n, nil)
		return n
	}
	body := slices.Clone(n.Statements.Body)
	for i, s := range body {
		call, ok := s.(*parser.CallNode)
		if !ok || call.Receiver != nil {
			continue
		}
		var uf *File
		switch call.Name {
		case "require_relative":
			uf = c.userRequireRelative(ctx, f, call)
		case "require":
			uf, _ = c.userRequire(ctx, f, call)
		default:
			continue
		}
		body[i] = &parser.NilNode{Location: call.Location} // already loaded: false in MRI, unused here
		if uf != nil {
			body[i] = &loadFile{call: call, file: uf}
		}
	}
	b := *n
	b.Statements = &parser.StatementsNode{Location: n.Statements.Location, Body: body}
	c.scanAnon(ctx, f, &b, nil)
	return &b
}

// requireHook is the call to Kernel#__require_<lib> that stands in for a
// user file's top-level `require "lib"`, or nil when the prelude has none.
func (c *Compiler) requireHook(f *File, n *parser.CallNode) *parser.CallNode {
	if f.isPrelude() || n.Arguments == nil || len(n.Arguments.Arguments) != 1 {
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
		cls = &Class{Name: goClassName(name), RubyName: name, IsModule: isModule, Methods: map[string]*Method{}, Ivars: map[string]*Ivar{}, srcPos: srcPos{f, line}}
		cls.universal = name == "BasicObject" || name == "Object" || name == "Kernel"
		c.classes[name] = cls
		c.classList = append(c.classList, cls)
		c.noteConstName(name)
	} else if cls.IsModule != isModule && (name != "Numeric" || f.isPrelude()) { // MRI's class Numeric is the prelude's module (decision 142): `class Numeric` reopens it
		c.errorAt(srcPos{f, line}, "%s is already defined as a %s", name, map[bool]string{true: "module", false: "class"}[cls.IsModule])
	}
	if c.consts[name] != nil {
		c.errorAt(srcPos{f, line}, "%s is already a constant", name)
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
	if len(cls.TypeParams) == 0 && !f.isPrelude() {
		// A `.rbs` `class Foo[E]` supplies the generics an unannotated class lacks (decision 160).
		cls.TypeParams = c.sigTypeParams(cls)
	}
	if _, ok := ann["hidden"]; ok && f.isPrelude() {
		cls.hidden = true
	}
	if g := ann["go_type"]; len(g) > 0 {
		if isModule {
			c.errorAt(srcPos{f, line}, "@go_type on a module")
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
	taken := map[string]bool{}
	for n := 2; n <= maxTuple; n++ {
		taken[fmt.Sprintf("Tuple%d", n)] = true
	}
	for _, v := range c.verbatim {
		for _, m := range goDecl.FindAllStringSubmatch(v.code, -1) {
			taken[m[1]] = true
		}
	}
	claim := func(f *File, name *string, gen func(string) []string) {
		for !f.isPrelude() && slices.ContainsFunc(gen(*name), func(n string) bool { return taken[n] }) {
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
		sup := n.Superclass
		if call, ok := sup.(*parser.CallNode); ok && call.Name == "DelegateClass" {
			sup = c.delegateClassSuper(ctx, f, call, scope)
		}
		if call, kind := valueClass(sup); call != nil && !reopen { // `class X < Struct.new(:a)`: a hidden value class as the super, as MRI's anonymous one
			anon := "Struct_" + strings.ReplaceAll(name, "::", "_")
			c.collectValueClass(ctx, f, &parser.ConstantWriteNode{Name: anon, Value: call, Location: call.Location, NameLoc: call.Location}, call, kind, scope)
			sup = &parser.ConstantReadNode{Name: anon, Location: call.Location}
		}
		if _, self := sup.(*parser.SelfNode); self && len(scope) > 0 { // `class NoArgument < self` in Switch's body: the enclosing class (decision 244)
			sup = absConstPath(scope[len(scope)-1].RubyName, sup.GetLocation())
		}
		ref := &constRef{node: sup, scope: scope, file: f}
		if reopen {
			// the first declaration fixed the superclass (Object when it named none)
			cls.reSupers = append(cls.reSupers, ref)
		} else {
			cls.superRef = ref
		}
	}
	if !reopen && !f.isPrelude() {
		c.hooks = append(c.hooks, classHook{name: "inherited", cls: cls, node: n, file: f})
	}
	c.collectBody(ctx, f, cls, n.Body, append(append([]*Class(nil), scope...), cls), true)
	if props, ok := f.sorbetStructs[n]; ok && cls.Methods["initialize"] == nil {
		c.addStructInit(ctx, f, n, cls, props, append(append([]*Class(nil), scope...), cls))
	}
}

// bootStructProps are the props boot saw cls's T::Struct end up with that its source does not name literally: a load-time
// loop's `const name, T` (decision 241). Their readers come with them.
func (c *Compiler) bootStructProps(ctx context.Context, f *File, cls *Class, props []structProp) ([]structProp, string) {
	if c.bootStructs == nil {
		return nil, ""
	}
	var extra []structProp
	var readers strings.Builder
	for _, bp := range c.bootStructs[cls.RubyName] {
		if slices.ContainsFunc(props, func(p structProp) bool { return p.name == bp.Name }) {
			continue
		}
		tf, err := parseFile(ctx, c.parser, f.Name, []byte(bp.Type), originSource)
		if err != nil || len(tf.Root.Statements.Body) != 1 {
			continue
		}
		p := structProp{name: bp.Name, rbs: sorbetType(tf.Root.Statements.Body[0])}
		p.optional = strings.HasSuffix(p.rbs, "?") || p.rbs == "untyped"
		if p.optional {
			p.def = "nil"
		}
		if bp.Default != "" {
			p.def, p.optional = bp.Default, true
		}
		extra = append(extra, p)
		readers.WriteString("#: () -> " + p.rbs + "\ndef " + p.name + " = @" + p.name + "\n")
	}
	return extra, readers.String()
}

// computedStructProp reports a `const name, T` / `prop name, T` whose name is not a literal Symbol: boot's props cover it.
func computedStructProp(n parser.Node) bool {
	found := false
	anyNode(n, func(x parser.Node) bool {
		if c, ok := x.(*parser.CallNode); ok && c.Receiver == nil && (c.Name == "const" || c.Name == "prop") {
			if args := callArgs(c); len(args) >= 2 {
				if _, sym := args[0].(*parser.SymbolNode); !sym {
					found = true
				}
			}
		}
		return found
	})
	return found
}

// addStructInit is the keyword initialize a T::Struct's props generate (decision 196).
func (c *Compiler) addStructInit(ctx context.Context, f *File, n *parser.ClassNode, cls *Class, props []structProp, scope []*Class) {
	extra, readers := c.bootStructProps(ctx, f, cls, props)
	props = append(slices.Clone(props), extra...)
	sig, params, body := make([]string, 0, len(props)), make([]string, 0, len(props)), make([]string, 0, len(props))
	for _, p := range props {
		opt := ""
		param := p.name + ":"
		if p.optional {
			opt, param = "?", p.name+": "+p.def
		}
		sig = append(sig, opt+p.name+": "+p.rbs)
		params = append(params, param)
		body = append(body, "@"+p.name+" = "+p.name)
	}
	syms := make([]string, len(props))
	for i, p := range props {
		syms[i] = ":" + p.name
	}
	// sorbet-runtime's `decorator.all_props` (Homebrew's FormulaStruct.from_hash slices by it), the props known here
	// (decision 249). ponytail: the class itself answers all_props, which MRI's does not; a Decorator class to fix.
	decorator := "#: () -> singleton(" + cls.RubyName + ")\ndef self.decorator = self\n#: () -> Array[Symbol]\ndef self.all_props = [" + strings.Join(syms, ", ") + "]\n"
	src := "#: (" + strings.Join(sig, ", ") + ") -> void\ndef initialize(" + strings.Join(params, ", ") + ")\n" + strings.Join(body, "\n") + "\nend\n" + readers + decorator
	sf, err := parseFile(ctx, c.parser, f.Name, []byte(src), originSource)
	if err != nil {
		c.errorf(f, n, "T::Struct initialize: %v", err)
	}
	sf.path, sf.pinLine = f.path, f.line(n.Location.StartOffset)
	for _, st := range sf.Root.Statements.Body {
		d := st.(*parser.DefNode) //nolint:forcetypeassert // built just above
		if d.Receiver != nil {    // `def self.decorator`: a class method, as a class body's
			c.addMethod(sf, cls, d, false, scope)
			continue
		}
		c.addDef(sf, cls, d, false, scope)
	}
}

func (c *Compiler) collectModule(ctx context.Context, f *File, n *parser.ModuleNode, scope []*Class) {
	name := c.declName(f, n.ConstantPath, scope)
	cls := c.declareClass(f, name, f.line(n.Location.StartOffset), true)
	inner := append(append([]*Class(nil), scope...), cls)
	c.includedExtends(f, cls, n.Body, inner)
	c.collectBody(ctx, f, cls, n.Body, inner, true)
}

// collectSingletonClass is `class << self`: its defs are class methods, as `def self.x` (a bare private there makes them private).
func (c *Compiler) collectSingletonClass(f *File, cls *Class, n *parser.SingletonClassNode, scope []*Class) {
	if _, ok := n.Expression.(*parser.SelfNode); !ok || cls == nil {
		target, _ := c.lookupConst(f, n.Expression, scope) // `class << Section`: that class's singleton, from wherever it is opened
		if target == nil || cls == nil {
			c.errorf(f, n, "only `class << self` or `class << Const` in a class or module body is supported")
		}
		cls = target
	}
	st, _ := n.Body.(*parser.StatementsNode)
	if st == nil {
		return
	}
	private := false
	for _, s := range st.Body {
		switch s := s.(type) {
		case *parser.DefNode:
			if s.Receiver != nil {
				c.errorf(f, s, "a def with a receiver inside `class << self` is not supported")
			}
			cls.singletonDefs = append(cls.singletonDefs, singletonDef{node: s, private: private, scope: scope, file: f})
		case *parser.CallNode:
			private = c.collectSingletonCall(f, cls, s, private, scope)
		case *parser.AliasMethodNode:
			cls.singletonMisc = append(cls.singletonMisc, singletonMisc{node: s, scope: scope, file: f})
		case *parser.ConstantWriteNode: // ponytail: the owner's constant, not the singleton class's; lexical lookup from its defs is the same
			c.addConst(f, s, s.Name, nil, scope)
		default:
			c.errorf(f, s, "unsupported node in `class << self`: %s", nodeType(s))
		}
	}
}

// isClassEvalCall reports whether n is a class-body `class_eval`/`module_eval`
// (or the *_exec forms), whose block is collected as class-body statements
// rather than dispatched or scanned here.
func isClassEvalCall(n *parser.CallNode) bool {
	return n.Receiver == nil && (n.Name == "class_eval" || n.Name == "module_eval" || n.Name == "class_exec" || n.Name == "module_exec")
}

// collectClassEval is `class_eval do ... end` in a class body: the block's
// body is collected as class-body statements in the same class and lexical
// scope, so `def` inside defines a method (decision 164). A String argument
// would be a real eval, which the boot snapshot handles at compile time
// (decision 161); block parameters belong to the *_exec forms, not supported.
func (c *Compiler) collectClassEval(ctx context.Context, f *File, cls *Class, n *parser.CallNode, scope []*Class, loadTime bool) {
	if len(callArgs(n)) > 0 {
		c.errorf(f, n, "%s with an argument is not supported (a String to eval is not)", n.Name)
	}
	blk, ok := n.Block.(*parser.BlockNode)
	if !ok {
		c.errorf(f, n, "%s needs a literal block", n.Name)
	}
	if blk.Parameters != nil {
		c.errorf(f, blk, "%s needs a block without parameters", n.Name)
	}
	c.collectBody(ctx, f, cls, blk.Body, scope, loadTime)
}

// addSingletonInstance gives a class that includes Singleton its
// `instance`, memoized in a class-level ivar (decision 54).
func (c *Compiler) addSingletonInstance(ctx context.Context, f *File, n *parser.CallNode, cls *Class, scope []*Class) {
	src := fmt.Sprintf("#: () -> %s\ndef self.instance = @__singleton_instance ||= new\n", cls.RubyName)
	sf, err := parseFile(ctx, c.parser, f.Name, []byte(src), f.synthOrigin())
	if err != nil {
		c.errorf(f, n, "%v", err)
	}
	c.addMethod(sf, cls, sf.Root.Statements.Body[0].(*parser.DefNode), false, scope)
}

// addClassStmt records a class/module body statement that runs at load time,
// where the class is defined (decision 166).
func (c *Compiler) addClassStmt(f *File, n parser.Node, scope []*Class) {
	if c.gemFile(f) && computedStructProp(n) && len(scope) > 0 && c.bootStructs[scope[len(scope)-1].RubyName] != nil {
		return // a load-time loop declaring T::Struct props: boot's props and define_methods are what it made (decision 241)
	}
	if c.gemFile(f) && undefLoop(n) {
		// ponytail: a gem's blank slate (`(instance_methods - ...).each { |m| undef_method m }`, Homebrew's InstallSteps
		// sandbox) keeps its methods, so only a misused DSL behaves differently; the boot snapshot's method list would undo it
		return
	}
	if g := c.bodyGroup; g != nil { // a body with locals runs as one unit (decision 170)
		st := g.Node.(*parser.StatementsNode)
		if len(st.Body) == 0 {
			st.Location = n.GetLocation()
			c.classStmts = append(c.classStmts, g)
		}
		st.Body = append(st.Body, n)
		return
	}
	c.classStmts = append(c.classStmts, &classStmt{Node: n, file: f, scope: scope})
}

// addMainStmt records a top-level statement and the file it runs in.
func (c *Compiler) addMainStmt(f *File, n parser.Node) {
	if c.stmtFile == nil {
		c.stmtFile = map[parser.Node]*File{}
	}
	c.stmtFile[n] = f
	c.mainStmts = append(c.mainStmts, n)
}

// noteAutoloadCall records `autoload :X, path`'s constant: it answers defined? before it loads, as in MRI.
func (c *Compiler) noteAutoloadCall(n *parser.CallNode, args []parser.Node, scope []*Class) {
	if sym, ok := symbolName(firstArg(args)); ok && n.Name == "autoload" {
		c.noteAutoload(qualify(scope, sym))
	}
}

// keepGoing runs fn; under RB2GO_KEEP_GOING=1 a compile error is recorded and collection goes on, listing at once
// every construct a large program needs (a debugging aid, like RB2GO_NO_PRUNE).
func (c *Compiler) keepGoing(fn func()) {
	if !c.env.KeepGoing || c.anonDepth > 0 { // an anonymous class's error already waits for its literal
		fn()
		return
	}
	defer func() {
		if r := recover(); r != nil {
			ce, ok := r.(compileError)
			if !ok {
				panic(r)
			}
			c.kept = append(c.kept, ce.Error())
		}
	}()
	fn()
}

// deferClassError records why cls cannot compile; codegen raises it only where reachable code names the class.
func (c *Compiler) deferClassError(cls *Class, msg string) {
	if c.badClasses == nil {
		c.badClasses = map[*Class]compileError{}
	}
	c.badClasses[cls] = compileError{pos: cls.srcPos, msg: msg}
}

// runsStatements reports a class body whose run-time statements are kept: a load-time body, or a per-run Class.new
// block's (decision 259).
func (c *Compiler) runsStatements(loadTime bool) bool { return loadTime || c.anonRun != nil }
