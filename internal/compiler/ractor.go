package compiler

import (
	"slices"
	"strconv"
	"strings"

	"github.com/danielgatis/go-ruby-prism/parser"
)

// Ractor (decision 103). Ractor.new's block is checked for isolation here,
// at compile time, where MRI checks at run time, and is compiled with a
// hidden Go local naming its ractor, which Ractor.receive/current/main?
// inside it resolve to lexically: goroutines have no identity (decision 45).

// genRactorCall compiles the Ractor class methods the compiler answers
// itself; ok is false for the rest (main, count, select).
func (f *fctx) genRactorCall(n *parser.CallNode, cls *Class) (expr, bool) {
	switch n.Name {
	case "new":
		return f.genRactorNew(n, cls), true
	case "receive", "recv":
		return f.genMethodCall(n, f.currentRactor(n), "__receive", nil, nil), true
	case "current":
		return f.currentRactor(n), true
	case "main?":
		f.currentRactor(n) // the same lexical rule
		return expr{code: "Boolean(" + strconv.FormatBool(f.ractor == "") + ")", typ: f.cls("Boolean")}, true
	}
	return expr{}, false
}

// currentRactor is the ractor the code at n runs in: the enclosing
// Ractor.new block's, or main at the top level. A method body cannot tell,
// so it is a compile error there.
func (f *fctx) currentRactor(n *parser.CallNode) expr {
	t := f.cls("Ractor")
	if f.ractor != "" {
		return expr{code: f.ractor, typ: t}
	}
	if f.m == nil && f.owner == nil {
		return expr{code: "rbMainRactor", typ: t}
	}
	f.errorf(n, "Ractor.%s in a method: rb2go cannot tell which ractor is running (decision 103); call it in the Ractor.new block or at the top level", n.Name)
	return expr{}
}

// genRactorNew emits `tmp := rbNewRactor()` and calls Ractor.__start*(tmp,
// args...) { block }, the block compiled with f.ractor = tmp.
func (f *fctx) genRactorNew(n *parser.CallNode, cls *Class) expr {
	bn, ok := n.Block.(*parser.BlockNode)
	if !ok {
		f.errorf(n, "Ractor.new needs a literal block")
	}
	f.checkIsolated(bn)
	args := callArgs(n)
	name := "__start"
	var kw *parser.KeywordHashNode
	if len(args) > 0 {
		kw, _ = args[len(args)-1].(*parser.KeywordHashNode)
	}
	switch {
	case kw != nil && len(args) == 1:
		name = "__start_named"
	case kw != nil:
		f.errorf(kw, "Ractor.new: name: together with positional arguments is not supported")
	case len(args) > 3:
		f.errorf(n, "Ractor.new takes at most 3 arguments")
	case len(args) > 0:
		name = "__start_" + strconv.Itoa(len(args)+1)
	}
	for _, a := range args {
		var e expr
		f.probe(func() { e = f.genExpr(a, nil) })
		if _, ok := e.typ.(TFunc); ok {
			f.errorf(a, "allocator undefined for Proc: a Proc cannot be passed to a Ractor")
		}
	}
	tmp := f.newTmp()
	f.emit("%s := rbNewRactor()", tmp)
	saved := f.ractor
	f.ractor = tmp
	defer func() { f.ractor = saved }()
	recv := expr{code: classVar(cls), typ: TClass{C: cls.meta}, classObj: true}
	nodes := append([]parser.Node{&exprNode{Node: n, e: expr{code: tmp, typ: TClass{C: cls}}}}, args...)
	return f.genMethodCall(n, recv, name, nodes, n.Block)
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

// isRactorRecv reports a Ractor or Ractor::Port receiver, whose `send` is a message, not Kernel#send.
func isRactorRecv(t Type) bool {
	c := classOf(t)
	return c != nil && (c.RubyName == "Ractor" || c.RubyName == "Ractor::Port")
}
