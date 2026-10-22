package compiler

import "github.com/danielgatis/go-ruby-prism/parser"

// loopStopVar names the StopIteration a rewritten `loop` rescues; not a Ruby identifier a program can write.
const loopStopVar = "loop_stop__"

// loopRescue rewrites a Kernel#loop whose block may raise StopIteration (an external next/peek, a Ractor receive,
// the constant itself) into `begin; loop { }; rescue StopIteration => e; e.result; end`, MRI's loop; other loops stay
// plain Go loops with no recover (decision 140).
func (f *fctx) loopRescue(n *parser.CallNode) *parser.BeginNode {
	if !isStopLoop(n) || f.c.loopInner[n] {
		return nil
	}
	recvT := f.selfType
	if n.Receiver != nil { // Kernel.loop
		f.probe(func() { recvT = f.genExpr(n.Receiver, nil).typ })
	}
	if e := f.resolve(recvT, "loop"); e == nil || !e.M.File.isPrelude() {
		return nil
	}
	return f.c.rewrite(n, func() parser.Node {
		loc := n.Location
		inner := *n
		if f.c.loopInner == nil {
			f.c.loopInner = map[*parser.CallNode]bool{}
		}
		f.c.loopInner[&inner] = true
		ref := &parser.LocalVariableTargetNode{Location: loc, Name: loopStopVar}
		result := &parser.CallNode{Location: loc, Receiver: &parser.LocalVariableReadNode{Location: loc, Name: loopStopVar}, Name: "result"}
		return &parser.BeginNode{
			Location:   loc,
			Statements: &parser.StatementsNode{Location: loc, Body: []parser.Node{&inner}},
			RescueClause: &parser.RescueNode{
				Location:   loc,
				Exceptions: []parser.Node{&parser.ConstantReadNode{Location: loc, Name: "StopIteration"}},
				Reference:  ref,
				Statements: &parser.StatementsNode{Location: loc, Body: []parser.Node{result}},
			},
		}
	}).(*parser.BeginNode)
}

// isStopLoop is a `loop { }` whose block may raise StopIteration, which loopRescue wraps.
func isStopLoop(n *parser.CallNode) bool {
	b, ok := n.Block.(*parser.BlockNode)
	return ok && n.Name == "loop" && kernelRecv(n) && n.Arguments == nil && raisesStop(b.Body)
}

// raisesStop is a loop body that may end the loop with StopIteration: lexically, since what a called method raises is not known here.
func raisesStop(n parser.Node) bool {
	switch n := n.(type) {
	case nil:
		return false
	case *parser.CallNode:
		if n.Receiver != nil && (n.Name == "next" || n.Name == "peek" || n.Name == "receive") {
			return true
		}
	case *parser.ConstantReadNode:
		if n.Name == "StopIteration" || n.Name == "ClosedQueueError" {
			return true
		}
	case *parser.ConstantPathNode:
		if n.Name != nil && *n.Name == "ClosedError" {
			return true
		}
	case *parser.DefNode:
		return false
	}
	for _, ch := range n.CompactChildNodes() {
		if raisesStop(ch) {
			return true
		}
	}
	return false
}
