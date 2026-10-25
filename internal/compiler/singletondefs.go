package compiler

import (
	"github.com/danielgatis/go-ruby-prism/parser"
)

// lowerSingletonDefs gives a fresh object's `def x.m` its own anonymous class, which the closed world compiles (decision 364).
func lowerSingletonDefs(n parser.Node) {
	if n == nil {
		return
	}
	if st, ok := n.(*parser.StatementsNode); ok && st != nil {
		st.Body = singletonDefs(st.Body)
	}
	for _, ch := range n.CompactChildNodes() {
		lowerSingletonDefs(ch)
	}
}

// singletonDefs moves each `def x.m` after x's fresh `C.new` into that object's anonymous class.
func singletonDefs(body []parser.Node) []parser.Node {
	type fresh struct {
		call *parser.CallNode
		defs *parser.StatementsNode
	}
	live := map[string]*fresh{}
	for i, s := range body {
		if d, ok := s.(*parser.DefNode); ok && d.Receiver != nil {
			if fr := live[varName(d.Receiver)]; fr != nil && varName(d.Receiver) != "" {
				if fr.defs == nil {
					fr.defs = &parser.StatementsNode{Location: d.Location}
					blk := &parser.BlockNode{Location: d.Location, Body: fr.defs}
					super := fr.call.Receiver
					fr.call.Receiver = &parser.CallNode{Location: fr.call.Location, Receiver: &parser.ConstantReadNode{Location: super.GetLocation(), Name: "Class"},
						Name: "new", Arguments: &parser.ArgumentsNode{Location: super.GetLocation(), Arguments: []parser.Node{super}}, Block: blk}
				}
				d.Receiver = nil
				fr.defs.Body = append(fr.defs.Body, d)
				body[i] = &parser.SymbolNode{Location: d.Location, Unescaped: parser.RubyString{Value: d.Name}} // a def's value
				continue
			}
		}
		name, value := writeOf(s)
		if name == "" {
			continue
		}
		delete(live, name)
		if c, ok := value.(*parser.CallNode); ok && c.Name == "new" && c.Block == nil && isConstNode(c.Receiver) {
			live[name] = &fresh{call: c}
		}
	}
	return body
}

// varName is a local or ivar read's name, or "".
func varName(n parser.Node) string {
	switch r := n.(type) {
	case *parser.LocalVariableReadNode:
		return r.Name
	case *parser.InstanceVariableReadNode:
		return r.Name
	}
	return ""
}

// writeOf is a statement's plain local or ivar write: the name and value, or "".
func writeOf(n parser.Node) (string, parser.Node) {
	switch w := n.(type) {
	case *parser.LocalVariableWriteNode:
		return w.Name, w.Value
	case *parser.InstanceVariableWriteNode:
		return w.Name, w.Value
	}
	return "", nil
}

// lowerBlockParamWrites gives a def that reassigns its `&b` a local of its own for b, so `yield` keeps calling the block (decision 364).
func lowerBlockParamWrites(n parser.Node) {
	if n == nil {
		return
	}
	if d, ok := n.(*parser.DefNode); ok && d.Parameters != nil && d.Parameters.Block != nil && d.Parameters.Block.Name != nil && d.Body != nil {
		b := *d.Parameters.Block.Name
		if writesLocal(d.Body, b) {
			shadow := b + "_rugaroo"
			renameLocal(d.Body, b, shadow)
			d.Locals = append(d.Locals, shadow)
			init := &parser.LocalVariableWriteNode{Location: d.Body.GetLocation(), Name: shadow, NameLoc: d.Body.GetLocation(), Value: &parser.LocalVariableReadNode{Location: d.Body.GetLocation(), Name: b}}
			switch body := d.Body.(type) {
			case *parser.StatementsNode:
				body.Body = append([]parser.Node{init}, body.Body...)
			case *parser.BeginNode:
				if body.Statements != nil {
					body.Statements.Body = append([]parser.Node{init}, body.Statements.Body...)
				}
			}
		}
	}
	for _, ch := range n.CompactChildNodes() {
		lowerBlockParamWrites(ch)
	}
}
