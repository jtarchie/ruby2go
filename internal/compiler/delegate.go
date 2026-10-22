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
		src := fmt.Sprintf(`class %[1]s < Delegator
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
		const label = "internal error: generated DelegateClass does not parse"
		sf := c.synthFile(ctx, f, call, label, src, synthOpt{Origin: originSource, Pad: line - 1, NoPath: true})
		cn, ok := sf.Root.Statements.Body[0].(*parser.ClassNode)
		if !ok {
			c.errorf(f, call, "%s: no class generated", label)
		}
		c.collectClass(ctx, sf, cn, scope)
	}
	return &parser.ConstantReadNode{Name: name, Location: call.Location}
}

// isDelegator reports whether cls is a Delegator (SimpleDelegator, a DelegateClass) or a subclass of one.
func (c *Compiler) isDelegator(cls *Class) bool {
	d := c.classes["Delegator"]
	return d != nil && cls != nil && cls != d && cls.isSubclassOf(d)
}

// delegateCall is a method a delegator doesn't define, compiled to the same
// call on its __getobj__ (decision 118): untyped, so dispatched at run time,
// for SimpleDelegator; typed for DelegateClass(Foo).
func (f *fctx) delegateCall(n parser.Node, recv expr, name string, args []parser.Node, block parser.Node) expr {
	obj := f.genMethodCall(n, recv, "__getobj__", nil, nil) // typed when delegateType knows the wrapped object
	return f.genMethodCall(n, obj, name, args, block)
}

// resetsDelegate: cls or a subclass may wrap something else (a subclass's own initialize, a `__setobj__` call).
func resetsDelegate(cls *Class) bool {
	family := []*Class{cls}
	for i := 0; i < len(family); i++ {
		family = append(family, family[i].Subclasses...)
	}
	for _, k := range family {
		if k != cls && k.Methods["initialize"] != nil {
			return true
		}
		for _, m := range k.MethodList {
			if m.Node != nil && anyNode(m.Node.Body, func(x parser.Node) bool {
				call, ok := x.(*parser.CallNode)
				return ok && call.Name == "__setobj__"
			}) {
				return true
			}
		}
	}
	return false
}

// delegateType is what a SimpleDelegator subclass wraps, when its own initialize sets it only by `super(p)` from a
// typed parameter p (a rest parameter wraps Array[T]; Homebrew's `Dependencies#initialize(*args)`) and neither it nor
// a subclass calls `__setobj__` or defines initialize: nil otherwise, and forwarded calls stay dynamic (decision 118).
// `super(Set.new(p))` wraps Set[T] (decision 291).
func (c *Compiler) delegateType(cls *Class) Type {
	if cls == nil || !c.isDelegator(cls) || cls.RubyName == "SimpleDelegator" {
		return nil
	}
	init := cls.Methods["initialize"]
	if init == nil || init.Node == nil || init.File == nil || init.File.isPrelude() || !init.declared {
		return nil
	}
	setobj := resetsDelegate(cls)
	var supers []*parser.SuperNode
	anyNode(init.Node.Body, func(x parser.Node) bool {
		if sn, ok := x.(*parser.SuperNode); ok {
			supers = append(supers, sn)
		}
		_, fwd := x.(*parser.ForwardingSuperNode)
		setobj = setobj || fwd
		return false
	})
	if setobj || len(supers) != 1 {
		return nil
	}
	if supers[0].Arguments == nil || len(supers[0].Arguments.Arguments) != 1 {
		return nil
	}
	arg := supers[0].Arguments.Arguments[0]
	inSet := false
	if call, ok := arg.(*parser.CallNode); ok && call.Name == "new" && constName(call.Receiver) == "Set" && len(callArgs(call)) == 1 && call.Block == nil {
		arg, inSet = callArgs(call)[0], true // `super(Set.new(args))`: Homebrew's Requirements wraps Set[Requirement]
	}
	lv, ok := arg.(*parser.LocalVariableReadNode)
	if !ok {
		return nil
	}
	c.resolveMethod(init)
	return c.delegateParamType(init, lv.Name, inSet)
}

// delegateParamType is what a delegator's `super(name)` (or `super(Set.new(name))` when inSet) wraps, from the typed
// parameter name: nil when it is untyped.
func (c *Compiler) delegateParamType(init *Method, name string, inSet bool) Type {
	for _, p := range init.Params {
		var elem Type
		switch {
		case p.Name != name || isAny(p.Type) || p.Type == nil || p.Keyword:
			continue
		case p.Rest:
			elem = p.Type
		case !inSet:
			return p.Type
		default:
			pc, isCls := p.Type.(TClass)
			if !isCls || len(pc.Args) != 1 || !isClass(pc, "Array") && !isClass(pc, "Set") || isAny(pc.Args[0]) {
				return nil
			}
			elem = pc.Args[0]
		}
		if inSet {
			return TClass{C: c.classes["Set"], Args: []Type{elem}}
		}
		return TClass{C: c.classes["Array"], Args: []Type{elem}}
	}
	return nil
}
