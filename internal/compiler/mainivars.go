package compiler

import (
	"reflect"
	"slices"

	"github.com/danielgatis/go-ruby-prism/parser"
)

// selfChangingBlocks are the calls whose block runs with another self: an `@x` there is not main's.
var selfChangingBlocks = map[string]bool{
	"instance_eval": true, "instance_exec": true, "class_eval": true, "class_exec": true, "module_eval": true,
	"module_exec": true, "define_method": true, "define_singleton_method": true, "new": true, "define": true,
}

var nodeIface = reflect.TypeFor[parser.Node]()

type ivarRead struct {
	slot reflect.Value
	name string
	loc  parser.Location
}

type mainIvars struct {
	written []string
	reads   []ivarRead // each nil when nothing writes its ivar: main's unset ivar is nil
}

// lowerMainIvars makes an `@x` whose self is main a program local named `@x`, typed by its writes as a local is (decision 362).
func lowerMainIvars(root parser.Node) {
	prog, ok := root.(*parser.ProgramNode)
	if !ok {
		return
	}
	m := &mainIvars{}
	m.walk(reflect.ValueOf(prog), 0)
	for _, r := range m.reads {
		if !slices.Contains(m.written, r.name) {
			r.slot.Set(reflect.ValueOf(parser.Node(&parser.NilNode{Location: r.loc})))
		}
	}
	for _, n := range m.written {
		if !slices.Contains(prog.Locals, n) {
			prog.Locals = append(prog.Locals, n)
		}
	}
}

func (m *mainIvars) write(name string) string {
	if !slices.Contains(m.written, name) {
		m.written = append(m.written, name)
	}
	return name
}

// lower is n as a local of main, or nil when n is no ivar node.
func (m *mainIvars) lower(n parser.Node, depth uint32) parser.Node {
	switch x := n.(type) {
	case *parser.InstanceVariableReadNode:
		return &parser.LocalVariableReadNode{Location: x.Location, Name: x.Name, Depth: depth}
	case *parser.InstanceVariableWriteNode:
		return &parser.LocalVariableWriteNode{Location: x.Location, Name: m.write(x.Name), Depth: depth, NameLoc: x.NameLoc, Value: x.Value, OperatorLoc: x.OperatorLoc}
	case *parser.InstanceVariableOperatorWriteNode:
		return &parser.LocalVariableOperatorWriteNode{Location: x.Location, Name: m.write(x.Name), Depth: depth, NameLoc: x.NameLoc, BinaryOperatorLoc: x.BinaryOperatorLoc, Value: x.Value, BinaryOperator: x.BinaryOperator}
	case *parser.InstanceVariableOrWriteNode:
		return &parser.LocalVariableOrWriteNode{Location: x.Location, Name: m.write(x.Name), Depth: depth, NameLoc: x.NameLoc, OperatorLoc: x.OperatorLoc, Value: x.Value}
	case *parser.InstanceVariableAndWriteNode:
		return &parser.LocalVariableAndWriteNode{Location: x.Location, Name: m.write(x.Name), Depth: depth, NameLoc: x.NameLoc, OperatorLoc: x.OperatorLoc, Value: x.Value}
	case *parser.InstanceVariableTargetNode:
		return &parser.LocalVariableTargetNode{Location: x.Location, Name: m.write(x.Name), Depth: depth}
	}
	return nil
}

// visit lowers the node in one child slot and walks into it where self is still main.
func (m *mainIvars) visit(slot reflect.Value, depth uint32) {
	n, _ := slot.Interface().(parser.Node)
	if n == nil || reflect.ValueOf(n).IsNil() {
		return
	}
	if d, ok := n.(*parser.DefinedNode); ok {
		if iv, ok := d.Value.(*parser.InstanceVariableReadNode); ok { // defined?(@x): as the read, nil when nothing writes it
			m.reads = append(m.reads, ivarRead{slot: slot, name: iv.Name, loc: d.Location})
			d.Value = &parser.LocalVariableReadNode{Location: iv.Location, Name: iv.Name, Depth: depth}
			return
		}
	}
	if l := m.lower(n, depth); l != nil {
		slot.Set(reflect.ValueOf(l))
		if r, ok := l.(*parser.LocalVariableReadNode); ok {
			m.reads = append(m.reads, ivarRead{slot: slot, name: r.Name, loc: r.Location})
		}
		n = l
	}
	switch x := n.(type) {
	case *parser.DefNode, *parser.ClassNode, *parser.ModuleNode, *parser.SingletonClassNode:
		return // self is not main inside
	case *parser.CallNode:
		if x.Block != nil && selfChangingBlocks[x.Name] {
			m.walk(reflect.ValueOf(x.Receiver), depth)
			m.walk(reflect.ValueOf(x.Arguments), depth)
			return
		}
	case *parser.BlockNode, *parser.LambdaNode:
		depth++
	}
	m.walk(reflect.ValueOf(n), depth)
}

// walk visits each Node child of the node v points to.
func (m *mainIvars) walk(v reflect.Value, depth uint32) {
	if !v.IsValid() || v.Kind() != reflect.Pointer || v.IsNil() || v.Elem().Kind() != reflect.Struct {
		return
	}
	s := v.Elem()
	for i := range s.NumField() {
		fv := s.Field(i)
		switch {
		case !fv.CanSet():
		case fv.Type() == nodeIface:
			m.visit(fv, depth)
		case fv.Kind() == reflect.Pointer && fv.Type().Implements(nodeIface):
			m.walk(fv, depth) // a typed child (*StatementsNode, *ArgumentsNode): never an ivar itself
		case fv.Kind() == reflect.Slice && fv.Type().Elem() == nodeIface:
			for j := range fv.Len() {
				m.visit(fv.Index(j), depth)
			}
		}
	}
}
