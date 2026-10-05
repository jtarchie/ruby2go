package compiler

import (
	"context"
	"fmt"

	"github.com/danielgatis/go-ruby-prism/parser"
)

// MRI's Class.new/Module.new make a class at run time; a closed world
// declares one per literal at compile time, as describe does (decision 145).

// anonClassCall recognizes `Class.new(Super?) { ... }` and `Module.new { ... }`.
func anonClassCall(n parser.Node) (*parser.CallNode, bool) {
	call, ok := n.(*parser.CallNode)
	if !ok || call.Name != "new" {
		return nil, false
	}
	r, ok := call.Receiver.(*parser.ConstantReadNode)
	if !ok || r.Name != "Class" && r.Name != "Module" {
		return nil, false
	}
	return call, r.Name == "Module"
}

// collectNamedAnon declares `Name = Class.new(Super) do ... end` as
// `class Name < Super`: the constant names the class, as in MRI.
func (c *Compiler) collectNamedAnon(ctx context.Context, f *File, n *parser.ConstantWriteNode, call *parser.CallNode, isModule bool, scope []*Class) {
	cls := c.declareAnon(ctx, f, call, isModule, qualify(scope, n.Name), scope)
	if !isModule && !f.prelude {
		c.hooks = append(c.hooks, classHook{name: "inherited", cls: cls, node: n, file: f})
	}
}

// scanAnon declares a hidden class for each Class.new/Module.new inside n
// (a method body, a block, a top-level statement). Class and module
// statements are collected where they stand; an anonymous class's block is
// its body, collected by declareAnon.
func (c *Compiler) scanAnon(ctx context.Context, f *File, n parser.Node, scope []*Class) {
	if n == nil || f.prelude {
		return
	}
	switch n := n.(type) {
	case *parser.ClassNode, *parser.ModuleNode:
		return
	case *parser.CallNode:
		if call, isModule := anonClassCall(n); call != nil {
			c.anonCount++
			cls := c.declareAnon(ctx, f, call, isModule, fmt.Sprintf("RbAnon%d", c.anonCount), scope)
			// MRI's anonymous class prints its address; one class per literal has none, so its place stands in
			cls.Display = fmt.Sprintf("#<%s:%s:%d>", map[bool]string{true: "Module", false: "Class"}[isModule], f.Name, f.line(call.Location.StartOffset))
			c.topConstNames = c.topConstNames[:len(c.topConstNames)-1] // not a name user code can reach
			return
		}
	}
	for _, ch := range n.CompactChildNodes() {
		c.scanAnon(ctx, f, ch, scope)
	}
}

func (c *Compiler) declareAnon(ctx context.Context, f *File, call *parser.CallNode, isModule bool, name string, scope []*Class) *Class {
	line := f.line(call.Location.StartOffset)
	args := callArgs(call)
	if isModule && len(args) > 0 || len(args) > 1 {
		c.errorf(f, call, "wrong number of arguments (given %d, expected 0%s)", len(args), map[bool]string{true: "", false: "..1"}[isModule])
	}
	if c.classes[name] != nil {
		c.errorf(f, call, "%s is already defined", name)
	}
	cls := c.declareClass(f, name, line, isModule)
	if c.anonClasses == nil {
		c.anonClasses = map[*parser.CallNode]*Class{}
	}
	c.anonClasses[call] = cls
	if len(args) == 1 {
		switch args[0].(type) {
		case *parser.ConstantReadNode, *parser.ConstantPathNode:
		default:
			c.errorf(f, args[0], "Class.new's superclass must be a constant (decision 145)")
		}
		cls.superRef = &constRef{node: args[0], scope: scope, file: f}
	}
	if call.Block == nil {
		return cls
	}
	blk, ok := call.Block.(*parser.BlockNode)
	if !ok || blk.Parameters != nil {
		c.errorf(f, call, "%s.new needs a block without parameters (decision 145)", map[bool]string{true: "Module", false: "Class"}[isModule])
	}
	// a block opens no constant scope in Ruby: the body's constants are the enclosing scope's
	c.collectBody(ctx, f, cls, blk.Body, scope)
	return cls
}
