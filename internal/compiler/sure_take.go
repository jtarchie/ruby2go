package compiler

import "github.com/danielgatis/go-ruby-prism/parser"

// ---- a shift/pop an emptiness guard proves non-nil (decision 391)

// markSureTake: nothing runs between `!x.empty?` and a body's first call `x.shift`, so no alias or callback emptied x; `redo` skips the check.
func (c *Compiler) markSureTake(pred parser.Node, truthy bool, body parser.Node) {
	x, ok := nonEmptyLocal(pred, truthy)
	st, _ := body.(*parser.StatementsNode)
	if !ok || st == nil || len(st.Body) == 0 || walkNode(body, func(n parser.Node) (bool, bool) {
		_, redo := n.(*parser.RedoNode)
		return redo, !opensScope(n)
	}) {
		return
	}
	first, ok := firstCall(st.Body[0])
	if !ok || first == nil || first.Name != "shift" && first.Name != "pop" || first.Arguments != nil || first.Block != nil || first.IsSAFE_NAVIGATION() {
		return
	}
	if r, ok := first.Receiver.(*parser.LocalVariableReadNode); !ok || r.Name != x.Name || r.Depth != x.Depth {
		return
	}
	if c.sureTake == nil {
		c.sureTake = map[*parser.CallNode]bool{}
	}
	c.sureTake[first] = true
}

// nonEmptyLocal is the local x that pred's being truthy (or falsy) proves not empty.
func nonEmptyLocal(pred parser.Node, truthy bool) (*parser.LocalVariableReadNode, bool) {
	for {
		p, ok := pred.(*parser.ParenthesesNode)
		if !ok {
			break
		}
		st, ok := p.Body.(*parser.StatementsNode)
		if !ok || len(st.Body) != 1 {
			return nil, false
		}
		pred = st.Body[0]
	}
	call, ok := pred.(*parser.CallNode)
	if !ok || call.Arguments != nil || call.Block != nil {
		return nil, false
	}
	switch call.Name {
	case "!":
		return nonEmptyLocal(call.Receiver, !truthy)
	case "empty?":
		x, ok := call.Receiver.(*parser.LocalVariableReadNode)
		return x, ok && !truthy
	}
	return nil, false
}

// firstCall follows Ruby's order (receiver, arguments, call); false when n holds a node it does not model.
func firstCall(n parser.Node) (*parser.CallNode, bool) {
	switch n := n.(type) {
	case *parser.LocalVariableReadNode, *parser.InstanceVariableReadNode, *parser.SelfNode, *parser.NilNode,
		*parser.TrueNode, *parser.FalseNode, *parser.IntegerNode, *parser.FloatNode, *parser.SymbolNode, *parser.StringNode:
		return nil, true
	case *parser.LocalVariableWriteNode:
		return firstCall(n.Value)
	case *parser.InstanceVariableWriteNode:
		return firstCall(n.Value)
	case *parser.MultiWriteNode: // `node, state = stack.pop`: the value first, then the targets
		return firstCall(n.Value)
	case *parser.CallNode:
		parts := []parser.Node{}
		if n.Receiver != nil {
			parts = append(parts, n.Receiver)
		}
		if n.Arguments != nil {
			parts = append(parts, n.Arguments.Arguments...)
		}
		for _, p := range parts {
			if c, ok := firstCall(p); !ok || c != nil {
				return c, ok
			}
		}
		return n, true
	}
	return nil, false
}

// sureTakeCall: a marked take answers E itself, so no E? is allocated; only the prelude's own shift/pop/empty?.
func (f *fctx) sureTakeCall(n parser.Node, recv expr, name string, args []parser.Node, block parser.Node) (expr, bool) {
	call, ok := n.(*parser.CallNode)
	if !ok || !f.c.sureTake[call] || len(args) != 0 || block != nil || recv.view != "" {
		return expr{}, false
	}
	t, ok := recv.typ.(TClass)
	arr := f.c.classes["Array"]
	if !ok || t.C != arr || len(t.Args) != 1 || isOpt(t.Args[0]) || holdsAny(t.Args[0]) {
		return expr{}, false
	}
	for _, m := range []string{name, "empty?"} {
		if e := arr.lookup(m); e == nil || e.M.File == nil || !e.M.File.isPrelude() {
			return expr{}, false
		}
	}
	// at the site, not a helper: the frozen check's call puts a helper over Go's inline budget; firstCall proved nothing runs before it
	tmp, a := f.newTmp(), recv.code
	f.emit("%s.rbCheckFrozen()", a)
	if name == "shift" {
		f.emit("%s := %s.s[0]", tmp, a)
		f.emit("%s.s = %s.s[1:]", a, a)
	} else {
		f.emit("%s := %s.s[len(%s.s)-1]", tmp, a, a)
		f.emit("%s.s = %s.s[:len(%s.s)-1]", a, a, a)
	}
	return expr{code: tmp, typ: t.Args[0]}, true
}
