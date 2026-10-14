package compiler

import (
	"slices"

	"github.com/danielgatis/go-ruby-prism/parser"

	"github.com/jtarchie/ruby2go/internal/rbs"
)

// isForwardAll reports `def f(...)` with no other parameters.
func isForwardAll(d *parser.DefNode) bool {
	if d == nil || d.Parameters == nil {
		return false
	}
	ps := d.Parameters
	_, ok := ps.KeywordRest.(*parser.ForwardingParameterNode)
	return ok && len(ps.Optionals) == 0 && ps.Rest == nil && len(ps.Posts) == 0 && len(ps.Keywords) == 0
}

// resolveForwarding types `def f(...)` from the one call in its body that
// forwards (`g(...)` on its own class or a top-level def, or `super(...)`):
// f takes g's parameters and block, under their names, its return is
// inferred, and the call's `...` becomes those parameters passed on.
func (c *Compiler) resolveForwarding(m *Method) {
	call, given := c.forwardingCall(m)
	target := c.forwardingTarget(m, call)
	c.resolveMethod(target)
	if target.forwardAll {
		c.errorf(m.File, call, "def %s(...) forwards to another (...) method; annotate one of them", m.Name)
	}
	m.forwardAll = true
	m.TypeParams = target.TypeParams
	// `def f(a, ...)`: a leads, typed by `# @rbs a: T` or from use (decision 146)
	ann := m.File.annotations(m.Line)
	for _, r := range m.Node.Parameters.Requireds {
		rp, ok := r.(*parser.RequiredParameterNode)
		if !ok {
			c.errorf(m.File, r, "def %s(...): a destructuring leading parameter is not supported", m.Name)
		}
		prm := Param{Name: rp.Name}
		if t := ann[rp.Name+":"]; len(t) > 0 {
			rt, err := rbs.ParseType(t[0])
			if err != nil {
				c.errorf(m.File, m.Node, "%v", err)
			}
			prm.Type = c.resolveType(rt, typeScope{class: m.Owner, lex: m.Scope, file: m.File, line: m.Line})
		} else {
			c.resolvePending(m, &prm, pendingParam{key: pendingKey(m, rp.Name), name: rp.Name})
		}
		m.Params = append(m.Params, prm)
	}
	lead := len(m.Params)
	// `g(x, ...)`: x fills g's first parameter; the rest are f's
	tps := target.Params
	if given > 0 {
		if given > len(tps) || slices.ContainsFunc(tps[:given], func(p Param) bool { return p.Keyword || p.Rest || p.Post || p.Default != nil }) {
			c.errorf(m.File, call, "def %s(...): the arguments before ... must fill %s's leading required parameters", m.Name, target.Name)
		}
		tps = tps[given:]
	}
	for _, p := range tps {
		if p.Default != nil && target.File != m.File && !isPlainLiteral(p.Default) { // a literal means the same in any file
			c.errorf(m.File, call, "def %s(...): %s's defaults are in another file; annotate %s instead", m.Name, target, m.Name)
		}
		m.Params = append(m.Params, p)
	}
	if target.Block != nil {
		b := *target.Block
		m.Block, m.BlockParam = &b, anonBlock
	}
	m.inferRet = true
	if e := c.forwardParent(m); e != nil { // an override keeps its parent's result, as decision 8's slots need it before inference
		c.resolveMethod(e.M)
		m.inherited, m.Ret, m.inferRet = e.M, subst(e.M.Ret, e.Env), e.M.inferRet
	}
	c.forwardArgs(m, call, lead, given)
}

// forwardingCall finds the one call or super in m's body whose last argument
// is `...`, and how many arguments precede it.
func (c *Compiler) forwardingCall(m *Method) (parser.Node, int) {
	var call parser.Node
	given := 0
	anyNode(m.Node.Body, func(n parser.Node) bool {
		var args *parser.ArgumentsNode
		switch n := n.(type) {
		case *parser.CallNode:
			args = n.Arguments
		case *parser.SuperNode:
			args = n.Arguments
		case *parser.DefNode, *parser.BlockNode, *parser.LambdaNode:
			return false
		}
		if args == nil || len(args.Arguments) == 0 {
			return false
		}
		if _, ok := args.Arguments[len(args.Arguments)-1].(*parser.ForwardingArgumentsNode); ok {
			if call != nil {
				c.errorf(m.File, n, "def %s(...) forwards more than once; annotate its signature instead", m.Name)
			}
			call, given = n, len(args.Arguments)-1
		}
		return false
	})
	if call == nil {
		c.errorf(m.File, m.Node, "def %s(...) needs one call that forwards with (...)", m.Name)
	}
	return call, given
}

// forwardingTarget is the method call reaches: on m's class (or self), a top-level def, or m's parent for super.
func (c *Compiler) forwardingTarget(m *Method, call parser.Node) *Method {
	var target *Method
	switch n := call.(type) {
	case *parser.CallNode:
		if _, self := n.Receiver.(*parser.SelfNode); n.Receiver != nil && !self {
			return c.forwardingOther(m, n)
		}
		if m.Owner != nil {
			if e := m.Owner.lookup(n.Name); e != nil {
				target = e.M
			}
		}
		if target == nil {
			target = c.topDefs[n.Name]
		}
	case *parser.SuperNode:
		if e := c.forwardParent(m); e != nil {
			target = e.M
		}
	}
	if target == nil || target == m {
		c.errorf(m.File, call, "def %s(...): its forwarding target is not a method rb2go can see", m.Name)
	}
	return target
}

// isPlainLiteral is a default that needs no scope to evaluate.
func isPlainLiteral(n parser.Node) bool {
	switch n := n.(type) {
	case *parser.NilNode, *parser.TrueNode, *parser.FalseNode, *parser.IntegerNode, *parser.FloatNode, *parser.SymbolNode, *parser.StringNode:
		return true
	case *parser.ArrayNode:
		return len(n.Elements) == 0
	case *parser.HashNode:
		return len(n.Elements) == 0
	}
	return false
}

// forwardingOther is the target of `recv.g(...)` on another receiver, from
// the type signatures resolve before bodies are typed: a constant's class
// methods, or an ivar declared with `# @rbs @x: T`.
func (c *Compiler) forwardingOther(m *Method, n *parser.CallNode) *Method {
	cls := m.fwdTarget
	switch r := n.Receiver.(type) {
	case *parser.ConstantReadNode, *parser.ConstantPathNode:
		if k, _ := c.lookupConst(m.File, r, m.Scope); k != nil {
			cls = k.meta
		}
	case *parser.InstanceVariableReadNode:
		if m.Owner != nil && cls == nil {
			if iv := c.findIvar(m.Owner, r.Name); iv != nil {
				if t, ok := stripOpt(iv.Type).(TClass); ok {
					cls = t.C
				}
			}
		}
	case *parser.CallNode: // `resource` with a declared return type (an annotated reader)
		if r.Receiver == nil && r.Arguments == nil && m.Owner != nil {
			if re := m.Owner.lookup(r.Name); re != nil {
				c.resolveMethod(re.M)
				if t, ok := stripOpt(re.M.Ret).(TClass); ok {
					cls = t.C
				}
			}
		}
	}
	var e *entry
	if cls != nil {
		e = cls.lookup(n.Name)
	}
	if e == nil || len(e.Owner.TypeParams) > 0 {
		c.errorf(m.File, n, "def %s(...) forwards to %s, whose type rb2go does not know before typing bodies: forward to a constant or an ivar declared with `# @rbs @x: T`", m.Name, m.File.text(n.Receiver.GetLocation()))
	}
	return e.M
}

// forwardArgs rewrites call's `...` into m's parameters in Ruby's order:
// positional, *rest, posts, keywords, then the block.
func (c *Compiler) forwardArgs(m *Method, call parser.Node, lead, given int) {
	var pos, posts, kws []parser.Node
	var rest parser.Node
	loc := call.GetLocation()
	var args0 *parser.ArgumentsNode
	switch n := call.(type) {
	case *parser.CallNode:
		args0 = n.Arguments
	case *parser.SuperNode:
		args0 = n.Arguments
	}
	pos = append(pos, args0.Arguments[:given]...) // `g(x, ...)`'s own x
	for _, p := range m.Params[lead:] {
		var a parser.Node = &parser.LocalVariableReadNode{Name: p.Name, Location: loc}
		switch {
		case p.KwRest:
			kws = append(kws, &parser.AssocSplatNode{Value: a, Location: loc})
		case p.Keyword:
			kws = append(kws, &parser.AssocNode{Key: &parser.SymbolNode{Location: loc, Unescaped: parser.RubyString{Value: p.Name}}, Value: a, Location: loc})
		case p.Rest:
			rest = &parser.SplatNode{Expression: a, Location: loc}
		case p.Post:
			posts = append(posts, a)
		default:
			pos = append(pos, a)
		}
	}
	args := pos
	if rest != nil {
		args = append(args, rest)
	}
	args = append(args, posts...)
	if len(kws) > 0 {
		args = append(args, &parser.KeywordHashNode{Elements: kws, Location: loc})
	}
	var blk parser.Node
	if m.Block != nil {
		blk = &parser.BlockArgumentNode{Expression: &parser.LocalVariableReadNode{Name: anonBlock, Location: loc}, Location: loc}
	}
	switch n := call.(type) {
	case *parser.CallNode:
		n.Arguments.Arguments = args
		if n.Block == nil {
			n.Block = blk
		}
	case *parser.SuperNode:
		n.Arguments.Arguments = args
		if n.Block == nil {
			n.Block = blk
		}
	}
}
