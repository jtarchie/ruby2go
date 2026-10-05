package compiler

import (
	"slices"
	"strings"

	"github.com/danielgatis/go-ruby-prism/parser"
)

// Ractor (decision 103). Ractor.new's block is checked for isolation here,
// at compile time, where MRI checks at run time; the call itself is an
// ordinary prelude call (Ractor.new / __new_N by arity).

// genRactorCall runs the compile-time checks on Ractor.new and routes a
// `name:` keyword to __new_named (a Hash argument is a message); otherwise
// ok is false and the call compiles as any other.
func (f *fctx) genRactorCall(n *parser.CallNode, cls *Class) (expr, bool) {
	if n.Name != "new" {
		return expr{}, false
	}
	bn, ok := n.Block.(*parser.BlockNode)
	if !ok {
		f.errorf(n, "Ractor.new needs a literal block")
	}
	f.checkIsolated(bn)
	args := callArgs(n)
	for _, a := range args {
		var e expr
		f.probe(func() { e = f.genExpr(a, nil) })
		if _, ok := e.typ.(TFunc); ok {
			f.errorf(a, "allocator undefined for Proc: a Proc cannot be passed to a Ractor")
		}
	}
	var kw *parser.KeywordHashNode
	if len(args) > 0 {
		kw, _ = args[len(args)-1].(*parser.KeywordHashNode)
	}
	switch {
	case kw != nil && len(args) == 1:
		recv := expr{code: classVar(cls), typ: TClass{C: cls.meta}, classObj: true}
		return f.genMethodCall(n, recv, "__new_named", args, n.Block), true
	case kw != nil:
		f.errorf(kw, "Ractor.new: name: together with positional arguments is not supported")
	case len(args) > 3:
		f.errorf(n, "Ractor.new takes at most 3 arguments")
	}
	return expr{}, false
}

// checkIsolated rejects what MRI's Proc isolation and IsolationError reject
// at run time: outer locals, instance variables, globals and class variables
// in a Ractor.new block. depth counts the blocks between n and the ractor
// block, whose own locals a nested block may read.
func (f *fctx) checkIsolated(bn *parser.BlockNode) {
	var outer []string
	note := func(name string, d, depth uint32) {
		if d > depth && !slices.Contains(outer, name) {
			outer = append(outer, name)
		}
	}
	var walk func(n parser.Node, depth uint32)
	walk = func(n parser.Node, depth uint32) {
		switch x := n.(type) {
		case nil, *parser.DefNode, *parser.ClassNode, *parser.ModuleNode, *parser.SingletonClassNode:
			return
		case *parser.BlockNode, *parser.LambdaNode:
			depth++
		case *parser.LocalVariableReadNode:
			note(x.Name, x.Depth, depth)
		case *parser.LocalVariableWriteNode:
			note(x.Name, x.Depth, depth)
		case *parser.LocalVariableTargetNode:
			note(x.Name, x.Depth, depth)
		case *parser.LocalVariableAndWriteNode:
			note(x.Name, x.Depth, depth)
		case *parser.LocalVariableOrWriteNode:
			note(x.Name, x.Depth, depth)
		case *parser.LocalVariableOperatorWriteNode:
			note(x.Name, x.Depth, depth)
		case *parser.InstanceVariableReadNode, *parser.InstanceVariableWriteNode, *parser.InstanceVariableTargetNode,
			*parser.InstanceVariableAndWriteNode, *parser.InstanceVariableOrWriteNode, *parser.InstanceVariableOperatorWriteNode:
			f.errorf(n, "can not access instance variables of shareable objects from non-main Ractors (Ractor::IsolationError)")
		case *parser.GlobalVariableReadNode:
			f.errorf(n, "can not access global variables %s from non-main Ractors (Ractor::IsolationError)", x.Name)
		case *parser.GlobalVariableWriteNode:
			f.errorf(n, "can not access global variables %s from non-main Ractors (Ractor::IsolationError)", x.Name)
		case *parser.ClassVariableReadNode, *parser.ClassVariableWriteNode, *parser.ClassVariableTargetNode,
			*parser.ClassVariableAndWriteNode, *parser.ClassVariableOrWriteNode, *parser.ClassVariableOperatorWriteNode:
			f.errorf(n, "can not access class variables from non-main Ractors (Ractor::IsolationError)")
		}
		for _, ch := range n.CompactChildNodes() {
			walk(ch, depth)
		}
	}
	walk(bn.Body, 0)
	if len(outer) > 0 {
		f.errorf(bn, "can not isolate a Proc because it accesses outer variables (%s).", strings.Join(outer, ", "))
	}
}
