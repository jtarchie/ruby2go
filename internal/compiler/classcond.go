package compiler

import (
	"github.com/danielgatis/go-ruby-prism/parser"
)

// rubyVersion is what class-body RUBY_VERSION guards fold against: rb2go compiles Ruby 4.0 (decision 167).
const rubyVersion = "4.0.0"

// takenBranch is the branch MRI would run of a class-body if/unless whose condition folds (decision 167).
func (c *Compiler) takenBranch(f *File, n parser.Node, scope []*Class) (stmts []parser.Node, ok bool) {
	switch n := n.(type) {
	case *parser.IfNode:
		v, ok := c.foldCond(f, n.Predicate, scope)
		if !ok {
			return nil, false
		}
		if v {
			return statements(n.Statements), true
		}
		switch s := n.Subsequent.(type) {
		case *parser.ElseNode:
			return statements(s.Statements), true
		case *parser.IfNode: // elsif
			return c.takenBranch(f, s, scope)
		}
		return nil, true
	case *parser.UnlessNode:
		v, ok := c.foldCond(f, n.Predicate, scope)
		if !ok {
			return nil, false
		}
		if !v {
			return statements(n.Statements), true
		}
		if n.ElseClause != nil {
			return statements(n.ElseClause.Statements), true
		}
		return nil, true
	}
	return nil, false
}

func statements(s *parser.StatementsNode) []parser.Node {
	if s == nil {
		return nil
	}
	return s.Body
}

// foldCond checks defined? against the world collected so far: what MRI sees at that point of the load.
func (c *Compiler) foldCond(f *File, n parser.Node, scope []*Class) (value, ok bool) {
	switch n := n.(type) {
	case *parser.TrueNode:
		return true, true
	case *parser.FalseNode, *parser.NilNode:
		return false, true
	case *parser.ParenthesesNode:
		if s, isStmts := n.Body.(*parser.StatementsNode); isStmts && len(s.Body) == 1 {
			return c.foldCond(f, s.Body[0], scope)
		}
	case *parser.AndNode:
		l, ok := c.foldCond(f, n.Left, scope)
		if !ok || !l {
			return false, ok
		}
		return c.foldCond(f, n.Right, scope)
	case *parser.OrNode:
		l, ok := c.foldCond(f, n.Left, scope)
		if !ok || l {
			return l, ok
		}
		return c.foldCond(f, n.Right, scope)
	case *parser.CallNode:
		if n.Name == "!" && n.Receiver != nil && n.Arguments == nil {
			v, ok := c.foldCond(f, n.Receiver, scope)
			return !v, ok
		}
		return foldVersion(n)
	case *parser.DefinedNode:
		return c.foldDefined(f, n.Value, scope)
	}
	return false, false
}

// foldVersion compares as strings, not versions, because MRI does ("10.0" < "4.0").
func foldVersion(n *parser.CallNode) (value, ok bool) {
	args := callArgs(n)
	if len(args) != 1 || !isRubyVersion(n.Receiver) {
		return false, false
	}
	s, isStr := args[0].(*parser.StringNode)
	if !isStr {
		return false, false
	}
	v := s.Unescaped.Value
	switch n.Name {
	case ">=":
		return rubyVersion >= v, true
	case ">":
		return rubyVersion > v, true
	case "<=":
		return rubyVersion <= v, true
	case "<":
		return rubyVersion < v, true
	case "==":
		return rubyVersion == v, true
	case "!=":
		return rubyVersion != v, true
	}
	return false, false
}

func isRubyVersion(n parser.Node) bool {
	switch n := n.(type) {
	case *parser.ConstantReadNode:
		return n.Name == "RUBY_VERSION"
	case *parser.ConstantPathNode:
		return n.Parent == nil && *n.Name == "RUBY_VERSION"
	}
	return false
}

// foldDefined folds defined?(Const), defined?(A::B) and defined?(Const.meth).
func (c *Compiler) foldDefined(f *File, n parser.Node, scope []*Class) (value, ok bool) {
	switch n := n.(type) {
	case *parser.ConstantReadNode, *parser.ConstantPathNode:
		cls, k := c.lookupConst(f, n, scope)
		return cls != nil || k != nil, true
	case *parser.CallNode:
		switch n.Receiver.(type) {
		case *parser.ConstantReadNode, *parser.ConstantPathNode:
		default:
			return false, false
		}
		if n.Arguments != nil || n.Block != nil {
			return false, false
		}
		cls, _ := c.lookupConst(f, n.Receiver, scope)
		return cls != nil && c.hasClassMethod(cls, n.Name), true
	}
	return false, false
}

// hasClassMethod reports whether cls or a superclass declares `def self.name`.
// ponytail: methods from `extend` and Module's own instance methods read as
// absent; walk cls.extends and Module when a guard needs them.
func (c *Compiler) hasClassMethod(cls *Class, name string) bool {
	for k := cls; k != nil; {
		for _, d := range k.singletonDefs {
			if d.node.Name == name {
				return true
			}
		}
		switch {
		case k.Super != nil:
			k = k.Super
		case k.superRef != nil:
			k = c.resolveClassRef(k.superRef)
		default:
			k = nil
		}
	}
	return false
}

// absentGuard reports whether pred is `defined?(C)`, or an `&&` led by one, for a constant no file defines: false at
// run time as well, since the closed world is the whole program, so the guarded code (which names C) is dropped.
func (c *Compiler) absentGuard(f *File, pred parser.Node, scope []*Class) bool {
	switch p := pred.(type) {
	case *parser.ParenthesesNode:
		if s, ok := p.Body.(*parser.StatementsNode); ok && len(s.Body) == 1 {
			return c.absentGuard(f, s.Body[0], scope)
		}
	case *parser.AndNode:
		return c.absentGuard(f, p.Left, scope)
	case *parser.DefinedNode:
		switch v := p.Value.(type) {
		case *parser.ConstantReadNode, *parser.ConstantPathNode:
			var cls *Class
			var k *Const
			ce := catchCompileError(func() { cls, k = c.lookupConst(f, v, scope) })
			return ce == nil && cls == nil && k == nil
		}
	}
	return false
}
