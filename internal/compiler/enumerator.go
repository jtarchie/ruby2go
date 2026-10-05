package compiler

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/danielgatis/go-ruby-prism/parser"
)

// yielderClass is Enumerator::Yielder, whose element type Enumerator.new infers from what its block feeds (decision 140).
const yielderClass = "Enumerator::Yielder"

// inferYielder binds X of a block taking a Yielder[X] (`Enumerator.new { |y| y << 1 }`) from the annotation, else from what a probe of the block feeds y.
func (f *fctx) inferYielder(n parser.Node, m *Method, env map[string]Type, block parser.Node) {
	if m.Block == nil || len(m.Block.Params) != 1 {
		return
	}
	yt, ok := m.Block.Params[0].(TClass)
	if !ok || yt.C.RubyName != yielderClass || len(yt.Args) != 1 {
		return
	}
	v, ok := yt.Args[0].(TVar)
	if !ok {
		return
	}
	if _, bound := env[v.Name]; bound {
		return
	}
	if n == f.retHintNode && f.retHint != nil && unify(m.Ret, f.retHint, env) {
		if _, bound := env[v.Name]; bound {
			return
		}
	}
	var fed []Type
	saved := f.yielderFed
	f.yielderFed = &fed
	probeSig := &BlockSig{Params: []Type{TClass{C: yt.C, Args: []Type{TAny{}}}}, Ret: TVoid{}}
	f.probe(func() { f.genClosure(n, block, probeSig, env) })
	f.yielderFed = saved
	if len(fed) == 0 {
		f.errorf(n, "cannot infer the element type of %s: the block feeds its yielder nothing (`y << value`); annotate the result (`#: Enumerator[Integer]`)", m.Name)
	}
	env[v.Name] = f.joinAll(n, fed)
}

// isProbedYielder is a receiver of the Yielder[untyped] inferYielder probes with.
func (f *fctx) isProbedYielder(t Type) bool {
	tc, ok := t.(TClass)
	return ok && f.yielderFed != nil && tc.C.RubyName == yielderClass && len(tc.Args) == 1 && isAny(tc.Args[0])
}

// noteYielderFeed records the type of what `y << v` (or y.yield v) feeds the probed yielder.
func (f *fctx) noteYielderFeed(recv expr, name string, args []parser.Node) {
	if !f.isProbedYielder(recv.typ) || len(args) != 1 || (name != "<<" && name != "yield") {
		return
	}
	var a expr
	f.probe(func() { a = f.genExpr(args[0], nil) })
	*f.yielderFed = append(*f.yielderFed, a.typ)
}

// yielderBlock is `&y` for a Yielder y: the block `{ |x_0| y << x_0 }`.
func yielderBlock(ba *parser.BlockArgumentNode) *parser.BlockNode {
	loc := ba.Location
	arg := &parser.LocalVariableReadNode{Location: loc, Name: "x_0"}
	call := &parser.CallNode{Location: loc, Receiver: ba.Expression, Name: "<<", Arguments: &parser.ArgumentsNode{Location: loc, Arguments: []parser.Node{arg}}}
	return &parser.BlockNode{
		Location:   loc,
		Locals:     []string{"x_0"},
		Parameters: &parser.BlockParametersNode{Location: loc, Parameters: &parser.ParametersNode{Location: loc, Requireds: []parser.Node{&parser.RequiredParameterNode{Location: loc, Name: "x_0"}}}},
		Body:       &parser.StatementsNode{Location: loc, Body: []parser.Node{call}},
	}
}

// loopStopVar names the StopIteration a rewritten `loop` rescues; not a Ruby identifier a program can write.
const loopStopVar = "loop_stop__"

// loopRescue rewrites a Kernel#loop whose block may raise StopIteration (an external next/peek, a Ractor receive,
// the constant itself) into `begin; loop { }; rescue StopIteration => e; e.result; end`, MRI's loop; other loops stay
// plain Go loops with no recover (decision 140).
func (f *fctx) loopRescue(n *parser.CallNode) *parser.BeginNode {
	if !isStopLoop(n) || f.c.loopInner[n] {
		return nil
	}
	if e := f.resolve(f.selfType, "loop"); e == nil || !e.M.File.prelude {
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
	return ok && n.Name == "loop" && n.Receiver == nil && n.Arguments == nil && raisesStop(b.Body)
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

// iterEnum is a blockless call of a prelude iterator with no `__<name>_enum` (`(1..3).each`): an Enumerator over its sequence (a user's each without a block is MRI's LocalJumpError).
func (f *fctx) iterEnum(n parser.Node, e *entry, recv expr, codes []string, env map[string]Type) (expr, bool) {
	m := e.M
	rt, ok := recv.typ.(TClass)
	if !ok || rt.C.IsModule || !m.File.prelude || len(m.Block.Params) == 0 || len(codes) != len(m.Params) || slices.ContainsFunc(m.Params, func(p Param) bool { return p.Rest || p.Keyword }) {
		return expr{}, false
	}
	enum := f.c.classes["Enumerator"]
	f.bindTypeParams(n, m, env)
	params := substAll(m.Block.Params, env)
	elem := params[0]
	if len(params) == 2 {
		elem = TTuple{Elems: params}
	}
	et := TClass{C: enum, Args: []Type{elem}}
	ps := []string{"r_ " + f.c.goType(recv.typ)}
	names := make([]string, len(codes))
	inspected := []string{}
	for i := range codes {
		names[i] = "a" + strconv.Itoa(i) + "_"
		ps = append(ps, names[i]+" "+f.c.goType(subst(m.Params[i].Type, env)))
		inspected = append(inspected, "string(rbInspect("+names[i]+"))")
	}
	seq := f.callCode(e, expr{code: "r_", typ: recv.typ}, names, env)
	if len(params) == 2 {
		seq = "rbPairSeq(" + seq + ")"
	}
	meth := strconv.Quote(m.Name)
	if len(codes) > 0 {
		meth += ` + "(" + ` + strings.Join(inspected, ` + ", " + `) + ` + ")"`
	}
	code := fmt.Sprintf("func(%s) %s { return rbEnumOf(%s, any(r_), %s, nil, nil) }(%s)",
		strings.Join(ps, ", "), f.c.goType(et), seq, meth, strings.Join(append([]string{f.materialize(recv)}, codes...), ", "))
	return expr{code: code, typ: et}, true
}

// isFloatInfinity is the constant Float::INFINITY.
func isFloatInfinity(n parser.Node) bool {
	cp, ok := n.(*parser.ConstantPathNode)
	if !ok || cp.Name == nil || *cp.Name != "INFINITY" {
		return false
	}
	p, ok := cp.Parent.(*parser.ConstantReadNode)
	return ok && p.Name == "Float"
}
