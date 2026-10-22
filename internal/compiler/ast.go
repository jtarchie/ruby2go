package compiler

import "github.com/danielgatis/go-ruby-prism/parser"

// walkNode is a pre-order walk for stateless predicates (hit stops it, !descend prunes); walks threading state down keep their own recursion.
func walkNode(n parser.Node, visit func(parser.Node) (hit, descend bool)) bool {
	if n == nil {
		return false
	}
	hit, descend := visit(n)
	if hit {
		return true
	}
	if !descend {
		return false
	}
	for _, ch := range n.CompactChildNodes() {
		if walkNode(ch, visit) {
			return true
		}
	}
	return false
}

// opensScope is a node whose body is another method's or class's, not part of the enclosing one.
func opensScope(n parser.Node) bool {
	switch n.(type) {
	case *parser.DefNode, *parser.ClassNode, *parser.ModuleNode, *parser.SingletonClassNode:
		return true
	}
	return false
}

// anyNode reports whether pred holds for n or any node below it.
func anyNode(n parser.Node, pred func(parser.Node) bool) bool {
	return walkNode(n, func(x parser.Node) (bool, bool) { return pred(x), true })
}
