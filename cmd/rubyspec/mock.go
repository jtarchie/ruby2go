package main

import (
	"fmt"
	"strings"

	"github.com/danielgatis/go-ruby-prism/parser"
)

// mspec decides a mock's answers at run time; one with literal answers becomes its own class here, so the compiler sees its methods (decision 179).
// ponytail: should_receive's call count is not checked (a mock never
// called still passes); count calls in an after hook when a spec needs it.

// mockClass is one mock's synthesized class.
type mockClass struct {
	name    string
	methods map[string]string // method name -> body (a literal, or a raise)
	args    map[string]bool   // methods given `.with(...)`: they take arguments
	order   []string
	defs    []string // `def x.m ... end` moved in as `def m ... end`: a def captures no locals, so its body reads the same
}

func (m *mockClass) add(name, body string, args bool) {
	if _, ok := m.methods[name]; !ok {
		m.order = append(m.order, name)
	}
	m.methods[name] = body
	m.args[name] = m.args[name] || args
}

func (m *mockClass) source() string {
	var b strings.Builder
	fmt.Fprintf(&b, "class %s\n", m.name)
	for _, name := range m.order {
		params, sig := "", "()"
		if m.args[name] {
			params, sig = "(*_args)", "(*untyped)"
		}
		if strings.HasPrefix(m.methods[name], "raise(") {
			fmt.Fprintf(&b, "  #: %s -> bot\n", sig)
		}
		fmt.Fprintf(&b, "  def %s%s = %s\n", name, params, m.methods[name])
	}
	for _, d := range m.defs {
		b.WriteString(d + "\n")
	}
	b.WriteString("end\n")
	return b.String()
}

func (prog *program) newMock() *mockClass {
	prog.mockN++
	return &mockClass{name: fmt.Sprintf("RbSpecMock%d", prog.mockN), methods: map[string]string{}, args: map[string]bool{}}
}

// mockLiteral is a value a top-level def can return: no locals, no self.
func (rw *rewriter) mockLiteral(n parser.Node) (string, bool) {
	switch n := n.(type) {
	case *parser.StringNode, *parser.IntegerNode, *parser.FloatNode, *parser.SymbolNode,
		*parser.NilNode, *parser.TrueNode, *parser.FalseNode, *parser.RationalNode, *parser.ImaginaryNode:
		return rw.text(n), true
	case *parser.ArrayNode:
		for _, e := range n.Elements {
			if _, ok := rw.mockLiteral(e); !ok {
				return "", false
			}
		}
		return rw.text(n), true
	case *parser.ConstantReadNode, *parser.ConstantPathNode:
		return rw.text(n), true
	}
	return "", false
}

// inlineMock rewrites `mock_int(3)` and `mock_to_path("p")` wherever they are.
func (rw *rewriter) inlineMock(call *parser.CallNode) bool {
	conv := map[string]string{"mock_int": "to_int", "mock_to_path": "to_path"}[call.Name]
	if conv == "" || call.Receiver != nil || call.Block != nil || call.Arguments == nil || len(call.Arguments.Arguments) != 1 {
		return false
	}
	v, ok := rw.mockLiteral(call.Arguments.Arguments[0])
	if !ok {
		return false
	}
	m := rw.prog.newMock()
	m.add(conv, v, false)
	rw.prog.mocks = append(rw.prog.mocks, m)
	s, e := span(call.Location)
	rw.edits = append(rw.edits, edit{s, e, m.name + ".new"})
	return true
}

// mocks rewrites each `x = mock(...)` whose later expectations in the list are all understood; one it can't read leaves the mock alone.
func (rw *rewriter) mocks(stmts []parser.Node) {
	for i, st := range stmts {
		name, val := mockTarget(st)
		if name == "" {
			continue
		}
		c, ok := val.(*parser.CallNode)
		if !ok || c.Block != nil || (c.Receiver != nil || c.Name != "mock") && !isObjectNew(c) {
			continue
		}
		m := rw.prog.newMock()
		var drop []parser.Node
		ok = true
		for _, later := range stmts[i+1:] {
			if !mentions(later, name) {
				continue
			}
			if sc, isSC := later.(*parser.SingletonClassNode); isSC && isReceiver(sc.Expression, name) {
				if sc.Body != nil { // `class << x ... end`: its body is the mock class's
					m.defs = append(m.defs, rw.text(sc.Body))
				}
				drop = append(drop, later)
				continue
			}
			if d, isDef := later.(*parser.DefNode); isDef && isReceiver(d.Receiver, name) {
				ns, _ := span(d.NameLoc)
				_, de := span(d.Location)
				m.defs = append(m.defs, "  def "+rw.src[ns:de])
				drop = append(drop, later)
				continue
			}
			exp, isExp := later.(*parser.CallNode)
			if !isExp || !rw.expectation(exp, name, m) {
				if isExpectationOf(later, name) {
					ok = false // an expectation not understood: leave the mock alone
				}
				continue
			}
			drop = append(drop, later)
		}
		if !ok || len(drop) == 0 {
			rw.prog.mockN-- // not used
			continue
		}
		rw.prog.mocks = append(rw.prog.mocks, m)
		s, e := span(c.Location)
		rw.edits = append(rw.edits, edit{s, e, m.name + ".new"})
		for _, d := range drop {
			rw.edits = append(rw.edits, blankLoc(rw.src, d.GetLocation()))
		}
	}
}

func isObjectNew(c *parser.CallNode) bool {
	r, ok := c.Receiver.(*parser.ConstantReadNode)
	return ok && r.Name == "Object" && c.Name == "new" && c.Arguments == nil
}

func mockTarget(st parser.Node) (string, parser.Node) {
	switch w := st.(type) {
	case *parser.LocalVariableWriteNode:
		return w.Name, w.Value
	case *parser.InstanceVariableWriteNode:
		return w.Name, w.Value
	}
	return "", nil
}

// isReceiver reports whether n reads the mock named name.
func isReceiver(n parser.Node, name string) bool {
	switch r := n.(type) {
	case *parser.LocalVariableReadNode:
		return r.Name == name
	case *parser.InstanceVariableReadNode:
		return r.Name == name
	}
	return false
}

func mentions(n parser.Node, name string) bool {
	if isReceiver(n, name) {
		return true
	}
	for _, ch := range n.CompactChildNodes() {
		if mentions(ch, name) {
			return true
		}
	}
	return false
}

// isExpectationOf reports whether st is a should_receive-family chain on the mock.
func isExpectationOf(st parser.Node, name string) bool {
	for c, ok := st.(*parser.CallNode); ok; c, ok = c.Receiver.(*parser.CallNode) {
		if expectNames[c.Name] && isReceiver(c.Receiver, name) {
			return true
		}
	}
	return false
}

var expectNames = map[string]bool{"should_receive": true, "stub!": true, "should_not_receive": true}

// chainMods are expectation modifiers that change nothing a literal answer needs.
var chainMods = map[string]bool{"once": true, "twice": true, "at_least": true, "at_most": true, "exactly": true, "times": true, "any_number_of_times": true}

// expectation adds what `x.should_receive(:m)[.with(..)][.once...].and_return(v)` says to m.
func (rw *rewriter) expectation(c *parser.CallNode, name string, m *mockClass) bool {
	body, args := "", false
	for {
		switch {
		case c.Name == "and_return" && body == "" && c.Arguments != nil && len(c.Arguments.Arguments) == 1:
			v, ok := rw.mockLiteral(c.Arguments.Arguments[0])
			if !ok {
				return false
			}
			body = v
		case c.Name == "and_raise" && body == "" && c.Arguments != nil && len(c.Arguments.Arguments) >= 1:
			v, ok := rw.mockLiteral(c.Arguments.Arguments[0])
			if !ok {
				return false
			}
			msg := ""
			if len(c.Arguments.Arguments) == 2 {
				s, ok := rw.mockLiteral(c.Arguments.Arguments[1])
				if !ok {
					return false
				}
				msg = ", " + s
			}
			body = "raise(" + v + msg + ")"
		case c.Name == "with":
			args = true
		case chainMods[c.Name]:
		case expectNames[c.Name] && isReceiver(c.Receiver, name) && c.Arguments != nil && len(c.Arguments.Arguments) == 1:
			meth := literal(c.Arguments.Arguments[0])
			if meth == "" {
				return false
			}
			if c.Name == "should_not_receive" {
				return body == "" // answered by not defining it
			}
			if body == "" {
				body = "nil"
			}
			m.add(meth, body, args)
			return true
		default:
			return false
		}
		next, ok := c.Receiver.(*parser.CallNode)
		if !ok {
			return false
		}
		c = next
	}
}

// hoist moves an example's class and module statements to the program's top level: a block opens no constant
// scope, so MRI defines them there too, only when the example runs rather than at load.
func (rw *rewriter) hoist(stmts []parser.Node) {
	for _, st := range stmts {
		switch st := st.(type) {
		case *parser.ClassNode, *parser.ModuleNode:
			if t := rw.text(st); strings.Contains(t, "should") || strings.Contains(t, "ScratchPad") {
				continue // its body checks or records as the example runs: at load it would run before (or outside) the example
			}
		case *parser.DefNode: // MRI puts it on the example's class; at the top level every example still reaches it
			if st.Receiver != nil {
				continue
			}
		default:
			continue
		}
		rw.prog.hoisted = append(rw.prog.hoisted, rw.text(st))
		rw.edits = append(rw.edits, blankLoc(rw.src, st.GetLocation()))
	}
}
