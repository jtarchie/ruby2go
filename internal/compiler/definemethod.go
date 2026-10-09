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
		site := sites[fmt.Sprintf("%s:%d", realPath(e.File), e.Line)]
		owner := c.classes[e.Owner]
		if site == nil || owner == nil {
			continue // a site outside the closed world (sorbet-runtime's own), or an anonymous owner
		}
		src := c.defineMethodSource(site, e)
		sf, err := parseFile(ctx, c.parser, site.file.Name, []byte(src), false)
		if err != nil {
			c.errorf(site.file, site.call, "define_method %s: %v", e.Name, err)
		}
		sf.path, sf.pinLine = site.file.path, site.file.line(site.call.Location.StartOffset)
		def := sf.Root.Statements.Body[0].(*parser.DefNode) //nolint:forcetypeassert // built just above
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
	b.WriteString(e.Name)
	if bp, ok := blk.Parameters.(*parser.BlockParametersNode); ok && bp.Parameters != nil {
		b.WriteString("(" + f.text(bp.Parameters.Location) + ")")
	}
	b.WriteString("\n")
	for _, name := range c.dmCaptures(site) {
		lit, ok := e.Captures[name]
		if !ok {
			c.errorf(f, site.call, "define_method %s captures %s, which boot could not record as plain data (decision 198)", e.Name, name)
		}
		b.WriteString(name + " = " + lit + "\n")
	}
	if blk.Body != nil {
		b.WriteString(blockAsMethodBody(f, blk.Body))
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
func blockAsMethodBody(f *File, body parser.Node) string {
	loc := body.GetLocation()
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
