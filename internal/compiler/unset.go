package compiler

import (
	"fmt"
	"maps"

	parser "github.com/danielgatis/go-ruby-prism/parser"
)

// assigned: the locals every path to a point assigned; dead = unreachable (after a jump).
type assigned struct {
	dead bool
	set  map[localKey]bool
}

func (s assigned) clone() assigned {
	return assigned{dead: s.dead, set: maps.Clone(s.set)}
}

// meet is the state where two paths join: what both assigned.
func meet(a, b assigned) assigned {
	switch {
	case a.dead:
		return b
	case b.dead:
		return a
	}
	out := assigned{set: map[localKey]bool{}}
	for k := range a.set {
		if b.set[k] {
			out.set[k] = true
		}
	}
	return out
}

// unsetWalk is a definite-assignment pass over one method body.
type unsetWalk struct {
	st     assigned
	scopes []int         // Ruby scopes, innermost last: methodScope, then block offsets
	breaks []*[]assigned // per enclosing while, the states its breaks leave with; nil for a block
	raises []*[]assigned // per enclosing begin body, the states its explicit raises leave with
	reads  map[localKey]bool
	writes map[localKey]bool
}

// maybeUnset: locals some read may reach before any assignment ran; Ruby reads nil there, so they are T? (decision 14).
func maybeUnset(body parser.Node, params []*local) map[localKey]bool {
	w := &unsetWalk{st: assigned{set: map[localKey]bool{}}, scopes: []int{methodScope}, reads: map[localKey]bool{}, writes: map[localKey]bool{}}
	for _, p := range params {
		w.st.set[localKey{methodScope, p.name}] = true
	}
	w.walk(body)
	maps.DeleteFunc(w.reads, func(k localKey, _ bool) bool { return !w.writes[k] }) // params and block params
	return w.reads
}

func (w *unsetWalk) key(name string, depth uint32) localKey {
	return localKey{w.scopes[max(len(w.scopes)-1-int(depth), 0)], name}
}

func (w *unsetWalk) read(k localKey) {
	if !w.st.dead && !w.st.set[k] {
		w.reads[k] = true
	}
}

func (w *unsetWalk) write(k localKey) {
	w.st.set[k] = true
	w.writes[k] = true
}

// fork runs each arm from the current state and joins where they end.
func (w *unsetWalk) fork(arms ...func()) {
	start := w.st
	for i, arm := range arms {
		cur := w.st
		w.st = start.clone()
		arm()
		if i > 0 {
			w.st = meet(cur, w.st)
		}
	}
}

func (w *unsetWalk) stmts(s *parser.StatementsNode) {
	if s != nil {
		w.walk(s)
	}
}

func (w *unsetWalk) walk(n parser.Node) {
	if w.local(n) {
		return
	}
	switch n := n.(type) {
	case nil, *parser.DefNode, *parser.ClassNode, *parser.ModuleNode, *parser.SingletonClassNode, *parser.LambdaNode:
		// no locals of this scope inside
	case *parser.IfNode:
		w.walk(n.Predicate)
		w.fork(func() { w.stmts(n.Statements) }, func() { w.walk(n.Subsequent) })
	case *parser.UnlessNode:
		w.walk(n.Predicate)
		w.fork(func() { w.stmts(n.Statements) }, func() {
			if n.ElseClause != nil {
				w.walk(n.ElseClause)
			}
		})
	case *parser.CaseNode:
		w.walk(n.Predicate)
		arms := make([]func(), 0, len(n.Conditions)+1)
		for _, c := range n.Conditions {
			arms = append(arms, func() { w.walk(c) })
		}
		arms = append(arms, func() {
			if n.ElseClause != nil {
				w.walk(n.ElseClause)
			}
		})
		w.fork(arms...)
	case *parser.WhileNode:
		_, forever := n.Predicate.(*parser.TrueNode)
		w.loop(n.Predicate, n.Statements, forever)
	case *parser.UntilNode:
		_, forever := n.Predicate.(*parser.FalseNode)
		w.loop(n.Predicate, n.Statements, forever)
	case *parser.AndNode:
		w.walk(n.Left)
		w.fork(func() { w.walk(n.Right) }, func() {})
	case *parser.OrNode:
		w.walk(n.Left)
		w.fork(func() { w.walk(n.Right) }, func() {})
	case *parser.BeginNode:
		w.begin(n)
	case *parser.RescueModifierNode:
		w.fork(func() { w.walk(n.Expression) }, func() { w.walk(n.RescueExpression) })
	case *parser.BreakNode:
		w.children(n)
		if top := w.breaks; len(top) > 0 && top[len(top)-1] != nil {
			*top[len(top)-1] = append(*top[len(top)-1], w.st.clone())
		}
		w.st.dead = true
	case *parser.ReturnNode, *parser.NextNode, *parser.RetryNode, *parser.RedoNode:
		w.children(n)
		w.st.dead = true
	case *parser.CallNode:
		w.children(n)
		if n.Receiver == nil && n.Name == "raise" {
			for _, r := range w.raises {
				*r = append(*r, w.st.clone())
			}
			w.st.dead = true
		}
	case *parser.BlockNode:
		// it may run any number of times: what it assigns outside stays unknown
		saved := w.st.clone()
		w.scopes = append(w.scopes, n.Location.StartOffset)
		w.breaks = append(w.breaks, nil)
		w.walk(n.Parameters)
		w.walk(n.Body)
		w.scopes = w.scopes[:len(w.scopes)-1]
		w.breaks = w.breaks[:len(w.breaks)-1]
		w.st = saved
	default:
		w.children(n)
	}
}

// local handles the nodes that read, write or bind a local.
func (w *unsetWalk) local(n parser.Node) bool {
	switch n := n.(type) {
	case *parser.LocalVariableReadNode:
		w.read(w.key(n.Name, n.Depth))
	case *parser.LocalVariableWriteNode:
		w.walk(n.Value)
		w.write(w.key(n.Name, n.Depth))
	case *parser.LocalVariableTargetNode:
		w.write(w.key(n.Name, n.Depth))
	case *parser.LocalVariableOperatorWriteNode:
		k := w.key(n.Name, n.Depth)
		w.read(k)
		w.walk(n.Value)
		w.write(k)
	case *parser.LocalVariableAndWriteNode:
		k := w.key(n.Name, n.Depth)
		w.read(k)
		w.fork(func() { w.walk(n.Value) }, func() {})
		w.write(k)
	case *parser.LocalVariableOrWriteNode:
		w.fork(func() { w.walk(n.Value) }, func() {})
		w.write(w.key(n.Name, n.Depth))
	case *parser.RequiredParameterNode:
		w.st.set[w.key(n.Name, 0)] = true
	case *parser.NumberedParametersNode:
		for i := 1; i <= int(n.Maximum); i++ {
			w.st.set[w.key(fmt.Sprintf("_%d", i), 0)] = true
		}
	case *parser.ItParametersNode:
		w.st.set[w.key("it", 0)] = true
	case *parser.MultiWriteNode:
		w.walk(n.Value)
		for _, t := range n.Lefts {
			w.walk(t)
		}
		w.walk(n.Rest)
		for _, t := range n.Rights {
			w.walk(t)
		}
	default:
		return false
	}
	return true
}

func (w *unsetWalk) children(n parser.Node) {
	for _, ch := range n.CompactChildNodes() {
		w.walk(ch)
	}
}

// loop: the body may run zero times; `while true` leaves only by break.
func (w *unsetWalk) loop(pred parser.Node, body *parser.StatementsNode, forever bool) {
	var breaks []assigned
	w.breaks = append(w.breaks, &breaks)
	w.walk(pred)
	exit := w.st.clone()
	exit.dead = exit.dead || forever
	w.stmts(body)
	w.breaks = w.breaks[:len(w.breaks)-1]
	for _, b := range breaks {
		exit = meet(exit, b)
	}
	w.st = exit
}

// begin: a callee's raise is taken to come after the body's assignments, so only explicit raises unset locals for a rescue (decision 14).
func (w *unsetWalk) begin(n *parser.BeginNode) {
	start := w.st.clone()
	var raised []assigned
	w.raises = append(w.raises, &raised)
	w.stmts(n.Statements)
	w.raises = w.raises[:len(w.raises)-1]
	from := w.st.clone()
	for _, r := range raised {
		from = meet(from, r)
	}
	if from.dead {
		from = start
	}
	if n.ElseClause != nil {
		w.walk(n.ElseClause)
	}
	after := w.st
	for rc := n.RescueClause; rc != nil; rc = rc.Subsequent {
		w.st = from.clone()
		for _, ex := range rc.Exceptions {
			w.walk(ex)
		}
		w.walk(rc.Reference)
		w.stmts(rc.Statements)
		after = meet(after, w.st)
	}
	w.st = after
	if n.EnsureClause == nil {
		return
	}
	w.st = start
	w.stmts(n.EnsureClause.Statements)
	maps.Copy(after.set, w.st.set)
	after.dead = after.dead || w.st.dead
	w.st = after
}
