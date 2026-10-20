package compiler

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/danielgatis/go-ruby-prism/parser"
)

// A gem's run-time `extend M` on an object sets a flag ivar; each def M overrides checks it first (decision 324).

// extSite is one `extend M` in an instance method of cls.
type extSite struct {
	cls  *Class
	m    *Method
	call *parser.CallNode
	mod  *Class
}

// collectObjectExtends finds the gem's receiverless `extend M` calls in instance methods.
func (c *Compiler) collectObjectExtends(ctx context.Context) {
	var sites []extSite
	for _, cls := range slices.Clone(c.classList) {
		if cls.IsModule || cls.metaOf != nil {
			continue
		}
		for _, m := range slices.Clone(cls.MethodList) {
			if m.Node == nil || m.File == nil || !c.gemFile(m.File) {
				continue
			}
			anyNode(m.Node.Body, func(n parser.Node) bool {
				call, ok := n.(*parser.CallNode)
				if !ok || call.Name != "extend" || call.Receiver != nil || call.Block != nil || len(callArgs(call)) != 1 {
					return false
				}
				mod, _ := c.lookupName(append(slices.Clip(m.Scope), cls), constName(callArgs(call)[0]))
				if mod != nil && mod.IsModule && mod.File != nil && c.gemFile(mod.File) {
					sites = append(sites, extSite{cls: cls, m: m, call: call, mod: mod})
				}
				return false
			})
		}
	}
	for k, s := range sites {
		c.keepGoing(func() { c.compileObjectExtend(ctx, k, s) })
	}
}

func (c *Compiler) compileObjectExtend(ctx context.Context, k int, s extSite) {
	flag := fmt.Sprintf("@__rb2go_ext_%d", k)
	top := s.cls
	for _, mm := range s.mod.MethodList {
		if mm.Node == nil || mm.Node.Receiver != nil || mm.File == nil {
			continue
		}
		var host *Class
		for h := s.cls; h != nil; h = c.superclassOf(h) {
			if om := h.Methods[mm.Name]; om != nil && om.Node != nil && om.File != nil && c.gemFile(om.File) {
				host = h
				break
			}
		}
		if host == nil { // nothing to override: the module's def, for this class's objects
			c.addSynthDefs(ctx, mm.File, mm.Node, s.cls, mm.File.text(mm.Node.Location), visibleName(mm))
			continue
		}
		if s.cls.isSubclassOf(host) || host != s.cls && c.isAncestor(host, top) {
			top = host
		}
		om := host.Methods[mm.Name]
		fwd, ok := defForwardArgs(om.Node)
		if !ok {
			c.warn(om.File, om.Node, "extend %s: %s's parameters cannot be passed on; the def runs as is", s.mod.RubyName, mm.Name)
			continue
		}
		orig := fmt.Sprintf("__rb2go_orig_%d_%s", k, goMethodName(mm.Name))
		ext := fmt.Sprintf("__rb2go_ext_%d_%s", k, goMethodName(mm.Name))
		c.addSynthDefs(ctx, om.File, om.Node, host, renamedDef(om.File, om.Node, orig))
		c.addSynthDefs(ctx, mm.File, mm.Node, host, superAs(mm.File, mm.Node, ext, orig))
		c.prependToDef(ctx, host, om, fmt.Sprintf("; if %s; return %s(%s); end", flag, ext, fwd))
		c.keepNames = append(c.keepNames, orig, ext)
	}
	set := fmt.Sprintf("__rb2go_extset_%d", k)
	c.addSynthDefs(ctx, s.m.File, s.call, top, fmt.Sprintf("def %s\n  %s = true\n  self\nend\n", set, flag))
	resetMsets(top)
	c.keepNames = append(c.keepNames, set)
	if c.smSites == nil {
		c.smSites = map[*parser.CallNode]*parser.CallNode{}
	}
	c.smSites[s.call] = &parser.CallNode{Location: s.call.Location, Name: set}
}

// isAncestor reports whether a is a superclass of b, from the declared superclasses (before link).
func (c *Compiler) isAncestor(a, b *Class) bool {
	for k := c.superclassOf(b); k != nil; k = c.superclassOf(k) {
		if k == a {
			return true
		}
	}
	return false
}

// renamedDef is d's source with its name replaced.
func renamedDef(f *File, d *parser.DefNode, name string) string {
	from, at, to := d.Location.StartOffset, d.NameLoc.StartOffset, d.NameLoc.StartOffset+d.NameLoc.Length
	return string(f.Src[from:at]) + name + string(f.Src[to:from+d.Location.Length])
}

// superAs is d's source renamed, its own `super`s calling orig: what super reaches once the module is folded in.
func superAs(f *File, d *parser.DefNode, name, orig string) string {
	fwd, _ := defForwardArgs(d)
	type edit struct {
		from, to int
		text     string
	}
	var edits []edit
	var walk func(n parser.Node)
	walk = func(n parser.Node) {
		switch n := n.(type) {
		case nil, *parser.DefNode:
			return
		case *parser.ForwardingSuperNode:
			edits = append(edits, edit{n.Location.StartOffset, n.Location.StartOffset + len("super"), orig + "(" + fwd + ")"})
		case *parser.SuperNode:
			text := orig
			if n.LparenLoc == nil && n.Arguments != nil {
				text += " "
			}
			edits = append(edits, edit{n.KeywordLoc.StartOffset, n.KeywordLoc.StartOffset + n.KeywordLoc.Length, text})
		}
		for _, ch := range n.CompactChildNodes() {
			walk(ch)
		}
	}
	walk(d.Body)
	from := d.Location.StartOffset
	var b strings.Builder
	prev := d.NameLoc.StartOffset
	b.Write(f.Src[from:prev])
	b.WriteString(name)
	prev = d.NameLoc.StartOffset + d.NameLoc.Length
	slices.SortFunc(edits, func(a, b edit) int { return a.from - b.from })
	for _, e := range edits {
		b.Write(f.Src[prev:e.from])
		b.WriteString(e.text)
		prev = e.to
	}
	b.Write(f.Src[prev : from+d.Location.Length])
	return b.String()
}

// prependToDef re-reads cls's own def om with pro right after its parameter list.
func (c *Compiler) prependToDef(ctx context.Context, cls *Class, om *Method, pro string) {
	d, f := om.Node, om.File
	at := d.NameLoc.StartOffset + d.NameLoc.Length
	switch {
	case d.RparenLoc != nil:
		at = d.RparenLoc.StartOffset + d.RparenLoc.Length
	case d.Parameters != nil:
		at = d.Parameters.Location.StartOffset + d.Parameters.Location.Length
	}
	from := d.Location.StartOffset
	text := string(f.Src[from:at]) + pro + string(f.Src[at:from+d.Location.Length])
	if d.EqualLoc != nil { // an endless def: its expression becomes a body
		body := d.EqualLoc.StartOffset + d.EqualLoc.Length
		text = string(f.Src[from:at]) + pro + "\n" + string(f.Src[body:from+d.Location.Length]) + "\nend"
	}
	sf, err := parseFile(ctx, c.parser, f.Name, []byte(text), originSource)
	if err != nil {
		c.errorf(f, d, "%s: %v\n%s", om.Name, err, text)
	}
	sf.path, sf.pinLine = f.path, f.line(from)
	nd := sf.Root.Statements.Body[0].(*parser.DefNode) //nolint:forcetypeassert // a def's own text
	c.addDef(sf, cls, nd, om.Private, om.Scope)
}

// visibleName is m's name when m is public: the def copied out of its module keeps that.
func visibleName(m *Method) string {
	if m.Private {
		return ""
	}
	return m.Name
}
