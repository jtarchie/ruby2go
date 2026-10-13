package compiler

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/danielgatis/go-ruby-prism/parser"

	"github.com/jtarchie/ruby2go/internal/boot"
)

// No run-time define_method: boot's methods become defs with captures baked in, and the call is a no-op (decision 198).

// dmSite is one define_method call in a compiled file, with the lexical scope it stands in.
type dmSite struct {
	file  *File
	call  *parser.CallNode
	block *parser.BlockNode
	scope []*Class
}

// collectDefineMethods declares the boot-recorded define_method methods whose site is in the closed world.
func (c *Compiler) collectDefineMethods(ctx context.Context, entries []boot.MethodInfo) {
	if len(entries) == 0 {
		return
	}
	sites := map[string]*dmSite{}
	for _, uf := range c.userFiles {
		c.indexDefineMethods(uf, uf.Root, nil, sites)
	}
	for _, e := range entries {
		c.keepGoing(func() { c.collectDefineMethod(ctx, sites, e) })
	}
}

func (c *Compiler) collectDefineMethod(ctx context.Context, sites map[string]*dmSite, e boot.MethodInfo) {
	{
		site := sites[fmt.Sprintf("%s:%d", realPath(e.File), e.Line)]
		owner := c.classes[e.Owner]
		if site == nil || owner == nil {
			return // a site outside the closed world (sorbet-runtime's own), or an anonymous owner
		}
		src := c.defineMethodSource(site, e)
		sf, err := parseFile(ctx, c.parser, site.file.Name, []byte(src), false)
		if err != nil {
			c.errorf(site.file, site.call, "define_method %s: %v\n%s", e.Name, err, src)
		}
		sf.path, sf.pinLine = site.file.path, site.file.line(site.call.Location.StartOffset)
		def := sf.Root.Statements.Body[0].(*parser.DefNode) //nolint:forcetypeassert // built just above
		def.Name = e.Name
		switch {
		case e.Singleton:
			owner.singletonDefs = append(owner.singletonDefs, singletonDef{node: def, scope: site.scope, file: sf})
		case e.ModuleFunction:
			c.addDef(sf, owner, def, true, site.scope)
			owner.singletonDefs = append(owner.singletonDefs, singletonDef{node: def, scope: site.scope, file: sf})
		default:
			c.addDef(sf, owner, def, e.Private, site.scope)
		}
	}
}

// indexDefineMethods records each define_method call with a literal block by file:line, with its lexical scope.
func (c *Compiler) indexDefineMethods(f *File, n parser.Node, scope []*Class, out map[string]*dmSite) {
	switch n := n.(type) {
	case nil:
		return
	case *parser.ClassNode:
		if cls := c.classes[c.declName(f, n.ConstantPath, scope)]; cls != nil {
			scope = append(slices.Clip(scope), cls)
		}
	case *parser.ModuleNode:
		if cls := c.classes[c.declName(f, n.ConstantPath, scope)]; cls != nil {
			scope = append(slices.Clip(scope), cls)
		}
	case *parser.CallNode:
		if blk, ok := n.Block.(*parser.BlockNode); ok && (n.Name == "define_method" || n.Name == "define_singleton_method") {
			loc := n.Location
			if n.MessageLoc != nil {
				loc = *n.MessageLoc
			}
			out[fmt.Sprintf("%s:%d", f.path, f.line(loc.StartOffset))] = &dmSite{file: f, call: n, block: blk, scope: scope}
		}
	}
	for _, ch := range n.CompactChildNodes() {
		c.indexDefineMethods(f, ch, scope, out)
	}
}

// defineMethodSource rebuilds the block as a def; captured locals become literal assignments so each method is self-contained.
func (c *Compiler) defineMethodSource(site *dmSite, e boot.MethodInfo) string {
	f, blk := site.file, site.block
	var b strings.Builder
	b.WriteString("def ")
	if e.Singleton {
		b.WriteString("self.")
	}
	b.WriteString("__rb2go_dm") // the real name may not be an identifier (`gcc-11`); collectDefineMethod sets it on the node
	if bp, ok := blk.Parameters.(*parser.BlockParametersNode); ok && bp.Parameters != nil {
		b.WriteString("(" + f.text(bp.Parameters.Location) + ")")
	}
	b.WriteString("\n")
	for _, name := range c.dmCaptures(site) {
		lit, ok := e.Captures[name]
		if !ok {
			c.errorf(f, site.call, "define_method %s captures %s, which boot could not record as plain data or a constant's element (decision 198)", e.Name, name)
		}
		if loc, ok := strings.CutPrefix(lit, "proc@"); ok {
			lit = c.procLiteral(f, site.call, loc)
		}
		b.WriteString(name + " = " + lit + "\n")
	}
	if blk.Body != nil {
		b.WriteString(blockAsMethodBody(f, blk))
	}
	b.WriteString("\nend\n")
	return b.String()
}

// dmCaptures finds outer locals the block reads; a write is an error since baked-in copies can't share state.
func (c *Compiler) dmCaptures(site *dmSite) []string {
	var out []string
	var walk func(n parser.Node, nest uint32)
	walk = func(n parser.Node, nest uint32) {
		switch n := n.(type) {
		case nil:
			return
		case *parser.LocalVariableReadNode:
			if n.Depth > nest && !slices.Contains(out, n.Name) {
				out = append(out, n.Name)
			}
		case *parser.LocalVariableWriteNode:
			if n.Depth > nest {
				c.errorf(site.file, n, "define_method block assigns the captured local %s (decision 198)", n.Name)
			}
		case *parser.LocalVariableOperatorWriteNode:
			if n.Depth > nest {
				c.errorf(site.file, n, "define_method block assigns the captured local %s (decision 198)", n.Name)
			}
		case *parser.BlockNode, *parser.LambdaNode:
			for _, ch := range n.CompactChildNodes() {
				walk(ch, nest+1)
			}
			return
		}
		for _, ch := range n.CompactChildNodes() {
			walk(ch, nest)
		}
	}
	walk(site.block.Body, 0)
	return out
}

// blockAsMethodBody is body's text with each `next` that leaves the block itself (not a nested one) as `return`.
func blockAsMethodBody(f *File, blk *parser.BlockNode) string {
	body := blk.Body
	loc := body.GetLocation()
	if bn, ok := body.(*parser.BeginNode); ok { // a block with rescue: its implicit begin starts at the block's `do`
		from := bn.Location.StartOffset
		switch {
		case bn.Statements != nil:
			from = bn.Statements.Location.StartOffset
		case bn.RescueClause != nil:
			from = bn.RescueClause.Location.StartOffset
		}
		loc = parser.Location{StartOffset: from, Length: blk.ClosingLoc.StartOffset - from}
	}
	start := loc.StartOffset
	text := []byte(f.text(loc))
	var nexts []int
	var walk func(n parser.Node)
	walk = func(n parser.Node) {
		switch n := n.(type) {
		case nil, *parser.BlockNode, *parser.LambdaNode, *parser.DefNode:
			return
		case *parser.NextNode:
			nexts = append(nexts, n.KeywordLoc.StartOffset-start)
		}
		for _, ch := range n.CompactChildNodes() {
			walk(ch)
		}
	}
	walk(body)
	var b strings.Builder
	prev := 0
	for _, at := range nexts {
		b.Write(text[prev:at])
		b.WriteString("return")
		prev = at + len("next")
	}
	b.Write(text[prev:])
	return b.String()
}

// markDefineMethods marks the define_method calls boot recorded before collect, so a class body can drop a statement
// that only makes methods.
func (c *Compiler) markDefineMethods(entries []boot.MethodInfo) {
	keys := map[string]bool{}
	for _, e := range entries {
		keys[fmt.Sprintf("%s:%d", realPath(e.File), e.Line)] = true
	}
	c.dmSites = map[*parser.CallNode]bool{}
	for path, f := range c.parsed {
		anyNode(f.Root, func(n parser.Node) bool {
			if call, ok := n.(*parser.CallNode); ok && (call.Name == "define_method" || call.Name == "define_singleton_method") {
				loc := call.Location
				if call.MessageLoc != nil {
					loc = *call.MessageLoc
				}
				if keys[fmt.Sprintf("%s:%d", path, f.line(loc.StartOffset))] {
					c.dmSites[call] = true
				}
			}
			return false
		})
	}
}

// onlyDefinesMethods reports whether n's calls into define_method are all ones boot recorded.
func (c *Compiler) onlyDefinesMethods(n parser.Node) bool {
	found := false
	anyNode(n, func(x parser.Node) bool {
		if call, ok := x.(*parser.CallNode); ok && c.dmSites[call] {
			found = true
		}
		return false
	})
	return found
}

// procLiteral is `proc <block>` for the literal block boot saw a captured Proc made from, at "path:line".
func (c *Compiler) procLiteral(f *File, at parser.Node, loc string) string {
	path, line, _ := strings.Cut(loc, ":")
	src := c.parsed[c.real(path)]
	var ln int
	_, _ = fmt.Sscanf(line, "%d", &ln)
	var found *parser.BlockNode
	if src != nil {
		anyNode(src.Root, func(n parser.Node) bool {
			if b, ok := n.(*parser.BlockNode); ok && src.line(b.Location.StartOffset) == ln {
				found = b
				return true
			}
			return false
		})
	}
	if found == nil {
		c.errorf(f, at, "define_method captures a Proc from %s, which is not compiled (decision 198)", loc)
	}
	return "proc " + src.text(found.Location)
}

// collectSingletonTemplates takes a gem method's `@x.define_singleton_method(name) do ... end`, whose block reads only
// the name it is given (Homebrew's CLI::Parser#set_args_method: `instance_variable_get(:@table).fetch(name)` per parsed
// flag), as a template over that name: @x's class (its `#:`/T.let type) gets `method_missing(name, *)` with the block's
// body, so `args.sizes?` is that body at :sizes?, and the run-time call is a no-op (decision 230). No method is made
// at run time; what MRI would make for any name is compiled once.
func (c *Compiler) collectSingletonTemplates(ctx context.Context) {
	for _, cls := range slices.Clone(c.classList) {
		for _, m := range slices.Clone(cls.MethodList) {
			if m.Node == nil || m.File == nil || !c.gemFile(m.File) {
				continue
			}
			c.keepGoing(func() { c.singletonTemplatesIn(ctx, cls, m, m.Node.Body) })
		}
	}
}

func (c *Compiler) singletonTemplatesIn(ctx context.Context, cls *Class, m *Method, n parser.Node) {
	switch n := n.(type) {
	case nil, *parser.DefNode:
		return
	case *parser.CallNode:
		if c.singletonTemplate(ctx, cls, m, n) {
			return
		}
	}
	for _, ch := range n.CompactChildNodes() {
		c.singletonTemplatesIn(ctx, cls, m, ch)
	}
}

func (c *Compiler) singletonTemplate(ctx context.Context, cls *Class, m *Method, n *parser.CallNode) bool {
	blk, ok := n.Block.(*parser.BlockNode)
	iv, isIvar := n.Receiver.(*parser.InstanceVariableReadNode)
	args := callArgs(n)
	if !ok || !isIvar || n.Name != "define_singleton_method" || len(args) != 1 || blk.Parameters != nil || blk.Body == nil {
		return false
	}
	nameVar, ok := args[0].(*parser.LocalVariableReadNode)
	if !ok || !defParamNamed(m.Node, nameVar.Name) {
		return false
	}
	site := &dmSite{file: m.File, call: n, block: blk, scope: m.Scope}
	if caps := c.dmCaptures(site); len(caps) != 1 || caps[0] != nameVar.Name {
		return false
	}
	target := c.ivarAnnotatedClass(cls, iv.Name)
	if target == nil || target.lookup("method_missing") != nil && target.lookup("method_missing").Owner == target {
		return false
	}
	src := "def method_missing(" + nameVar.Name + ", *_rb2go_rest)\n" + blockAsMethodBody(m.File, blk) + "\nend\n"
	sf, err := parseFile(ctx, c.parser, m.File.Name, []byte(src), false)
	if err != nil {
		c.errorf(m.File, n, "define_singleton_method template: %v\n%s", err, src)
	}
	sf.path, sf.pinLine = m.File.path, m.File.line(n.Location.StartOffset)
	def := sf.Root.Statements.Body[0].(*parser.DefNode) //nolint:forcetypeassert // built just above
	c.addDef(sf, target, def, false, c.nestingOf(target))
	resetMsets(target)
	if c.dmSites == nil {
		c.dmSites = map[*parser.CallNode]bool{}
	}
	c.dmSites[n] = true // the run-time call: a no-op, as a boot define_method's is (decision 198)
	if !slices.Contains(c.keepNames, "method_missing") {
		c.keepNames = append(c.keepNames, "method_missing") // reached by any unknown call, never by its name
	}
	return true
}

// ivarAnnotatedClass is the class an ivar of cls is declared as by a `#:` (or T.let) annotation on one of its
// assignments in cls's methods, resolved in the method's lexical scope.
func (c *Compiler) ivarAnnotatedClass(cls *Class, name string) *Class {
	for _, m := range cls.MethodList {
		if m.Node == nil || m.File == nil {
			continue
		}
		var found *Class
		var walk func(parser.Node)
		walk = func(n parser.Node) {
			if found != nil || n == nil {
				return
			}
			if w, ok := n.(*parser.InstanceVariableWriteNode); ok && w.Name == name {
				if ann := strings.TrimSpace(m.File.trailingAnnotation(w)); ann != "" {
					found = c.scopedClass(ann, append(slices.Clip(m.Scope), cls))
				}
			}
			for _, ch := range n.CompactChildNodes() {
				walk(ch)
			}
		}
		walk(m.Node.Body)
		if found != nil {
			return found
		}
	}
	return nil
}

// scopedClass resolves a constant path as Ruby would from inside scope (innermost first), then at top level.
func (c *Compiler) scopedClass(name string, scope []*Class) *Class {
	name = strings.TrimPrefix(name, "::")
	for i := len(scope) - 1; i >= 0; i-- {
		if k := c.classes[scope[i].RubyName+"::"+name]; k != nil {
			return k
		}
	}
	return c.classes[name]
}

// nestingOf is the lexical scope a def written in cls's body has: each enclosing constant of its path, then cls.
func (c *Compiler) nestingOf(cls *Class) []*Class {
	var out []*Class
	parts := strings.Split(cls.RubyName, "::")
	for i := range parts {
		if k := c.classes[strings.Join(parts[:i+1], "::")]; k != nil {
			out = append(out, k)
		}
	}
	return out
}

// defParamNamed: d has a required or optional parameter called name (Method.Params are filled only at link).
func defParamNamed(d *parser.DefNode, name string) bool {
	ps := d.Parameters
	if ps == nil {
		return false
	}
	for _, p := range ps.Requireds {
		if r, ok := p.(*parser.RequiredParameterNode); ok && r.Name == name {
			return true
		}
	}
	for _, p := range ps.Optionals {
		if o, ok := p.(*parser.OptionalParameterNode); ok && o.Name == name {
			return true
		}
	}
	return false
}
