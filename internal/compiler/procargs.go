package compiler

import (
	"github.com/danielgatis/go-ruby-prism/parser"
)

// procArgs is the rest a block with a complex parameter list is lowered to (lowerProcArgs).
const procArgs = "__rb2go_args"

// lowerProcArgs gives a block whose parameters mix optionals, a rest and trailing names (`|a, b = 5, *c, d|`) the
// single rest procArgs and a prologue that splits it MRI's way (decision 367): leading names, then optionals while
// values remain past the trailing names, then the rest, then the trailing names, nil past the end. It reports whether it
// lowered b, keeping the original list for Proc#parameters.
func (f *File) lowerProcArgs(b *parser.BlockNode) bool {
	bp, ok := b.Parameters.(*parser.BlockParametersNode)
	if !ok || bp.Parameters == nil {
		return false
	}
	ps := bp.Parameters
	if len(ps.Posts) == 0 && (len(ps.Optionals) == 0 || ps.Rest == nil) || len(ps.Keywords) > 0 || ps.KeywordRest != nil || ps.Block != nil {
		return false
	}
	simple := func(nodes []parser.Node) bool {
		for _, x := range nodes {
			if _, ok := x.(*parser.RequiredParameterNode); !ok {
				return false
			}
		}
		return true
	}
	rest, isRest := ps.Rest.(*parser.RestParameterNode)
	if !simple(ps.Requireds) || !simple(ps.Posts) || ps.Rest != nil && !isRest {
		return false
	}
	loc := b.Location
	args := func() parser.Node { return &parser.LocalVariableReadNode{Location: loc, Name: procArgs} }
	num := func(i int) parser.Node { // no source text: genInteger takes Value
		return &parser.IntegerNode{Location: parser.Location{StartOffset: loc.StartOffset}, Value: int64(i)}
	}
	call := func(recv parser.Node, name string, a ...parser.Node) parser.Node {
		c := &parser.CallNode{Location: loc, Receiver: recv, Name: name}
		if len(a) > 0 {
			c.Arguments = &parser.ArgumentsNode{Location: loc, Arguments: a}
		}
		return c
	}
	write := func(name string, v parser.Node) parser.Node {
		return &parser.LocalVariableWriteNode{Location: loc, Name: name, NameLoc: loc, Value: v}
	}
	pre, opt, post := len(ps.Requireds), len(ps.Optionals), len(ps.Posts)
	shape := func() []parser.Node { return []parser.Node{num(pre), num(opt), num(post)} }
	var pro []parser.Node
	for i, r := range ps.Requireds {
		pro = append(pro, write(r.(*parser.RequiredParameterNode).Name, call(args(), "[]", num(i)))) //nolint:forcetypeassert // simple
	}
	for j, o := range ps.Optionals {
		op, ok := o.(*parser.OptionalParameterNode)
		if !ok {
			return false
		}
		given := call(call(args(), "__bopts", shape()...), ">", num(j))
		v := &parser.IfNode{Location: loc, Predicate: given,
			Statements: &parser.StatementsNode{Location: loc, Body: []parser.Node{call(args(), "[]", num(pre+j))}},
			Subsequent: &parser.ElseNode{Location: loc, Statements: &parser.StatementsNode{Location: loc, Body: []parser.Node{op.Value}}}}
		pro = append(pro, write(op.Name, v))
	}
	if isRest && rest.Name != nil {
		pro = append(pro, write(*rest.Name, call(args(), "__brest", shape()...)))
	}
	for l, p := range ps.Posts {
		pro = append(pro, write(p.(*parser.RequiredParameterNode).Name, call(args(), "__bpost", append([]parser.Node{num(l)}, shape()...)...))) //nolint:forcetypeassert // simple
	}
	orig := *bp
	if f.procArgsOrig == nil {
		f.procArgsOrig = map[*parser.BlockParametersNode]*parser.BlockParametersNode{}
	}
	nbp := &parser.BlockParametersNode{Location: bp.Location, Parameters: &parser.ParametersNode{Location: ps.Location,
		Rest: &parser.RestParameterNode{Location: ps.Location, Name: new(procArgs)}}}
	f.procArgsOrig[nbp] = &orig
	b.Parameters = nbp
	b.Locals = append(b.Locals, procArgs)
	switch body := b.Body.(type) {
	case nil:
		b.Body = &parser.StatementsNode{Location: loc, Body: pro}
	case *parser.StatementsNode:
		body.Body = append(pro, body.Body...)
	case *parser.BeginNode:
		if body.Statements == nil {
			body.Statements = &parser.StatementsNode{Location: loc}
		}
		body.Statements.Body = append(pro, body.Statements.Body...)
	}
	return true
}
