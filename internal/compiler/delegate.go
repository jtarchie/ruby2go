package compiler

import (
	"context"
	"fmt"
	"strings"

	"github.com/danielgatis/go-ruby-prism/parser"
)

// delegateClassSuper handles `class W < DelegateClass(Foo)` (decision 118):
// the first use for Foo generates a Delegator subclass whose __getobj__ is
// typed Foo, so the calls W forwards are typed calls on Foo; the returned
// node names it as W's superclass.
func (c *Compiler) delegateClassSuper(ctx context.Context, f *File, call *parser.CallNode, scope []*Class) parser.Node {
	args := callArgs(call)
	if len(args) != 1 || call.Receiver != nil || call.Block != nil {
		c.errorf(f, call, "DelegateClass takes one class")
	}
	switch args[0].(type) {
	case *parser.ConstantReadNode, *parser.ConstantPathNode:
	default:
		c.errorf(f, args[0], "DelegateClass takes a class constant")
	}
	target := f.text(args[0].GetLocation())
	name := "DelegateClass_" + strings.ReplaceAll(strings.TrimPrefix(target, "::"), "::", "_")
	if c.classes[qualify(scope, name)] == nil {
		line := f.line(call.Location.StartOffset)
		src := strings.Repeat("\n", line-1) + fmt.Sprintf(`class %[1]s < Delegator
  #: (%[2]s) -> void
  def initialize(obj)
    @delegate_dc_obj = obj
  end

  #: () -> %[2]s
  def __getobj__ = @delegate_dc_obj

  #: (%[2]s) -> %[2]s
  def __setobj__(obj)
    @delegate_dc_obj = obj
  end

  #: () -> String
  def to_s = __getobj__.to_s

  #: () -> String
  def inspect = __getobj__.inspect

  #: (untyped) -> bool
  def ==(other) = other.equal?(self) || __getobj__ == other
end
`, name, target)
		sf, err := parseFile(ctx, c.parser, f.Name, []byte(src), false)
		if err != nil {
			c.errorf(f, call, "internal error: generated DelegateClass does not parse: %v", err)
		}
		c.collectClass(ctx, sf, sf.Root.Statements.Body[0].(*parser.ClassNode), scope)
	}
	return &parser.ConstantReadNode{Name: name, Location: call.Location}
}
