package compiler

import (
	"slices"
	"strconv"

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

// lowerNestedParams gives each destructuring block parameter that nests or splats (`|a, (b, (c, d))|`, `|(a, *b), c|`)
// a fresh name and a multiple assignment from it at the top of the body, which destructures as `(b, (c, d)) = x` does
// (decision 369).
func (f *File) lowerNestedParams(b *parser.BlockNode) {
	bp, ok := b.Parameters.(*parser.BlockParametersNode)
	if !ok || bp.Parameters == nil {
		return
	}
	var pro []parser.Node
	lower := func(nodes []parser.Node) {
		for i, x := range nodes {
			mt, ok := x.(*parser.MultiTargetNode)
			if !ok || !nestedTarget(mt) {
				continue
			}
			name := "__rb2go_d" + strconv.Itoa(len(b.Locals))
			b.Locals = append(b.Locals, name)
			nodes[i] = &parser.RequiredParameterNode{Location: mt.Location, Name: name}
			w := asWriteTarget(mt).(*parser.MultiTargetNode) //nolint:forcetypeassert // a MultiTargetNode stays one
			pro = append(pro, &parser.MultiWriteNode{Location: mt.Location, Lefts: w.Lefts, Rest: w.Rest, Rights: w.Rights,
				Value: &parser.LocalVariableReadNode{Location: mt.Location, Name: name}})
		}
	}
	lower(bp.Parameters.Requireds)
	lower(bp.Parameters.Posts)
	if len(pro) == 0 {
		return
	}
	switch body := b.Body.(type) {
	case nil:
		b.Body = &parser.StatementsNode{Location: b.Location, Body: pro}
	case *parser.StatementsNode:
		body.Body = append(pro, body.Body...)
	case *parser.BeginNode:
		if body.Statements == nil {
			body.Statements = &parser.StatementsNode{Location: b.Location}
		}
		body.Statements.Body = append(pro, body.Statements.Body...)
	}
}

// nestedTarget is a destructuring parameter blockParamList does not take: one with a splat, trailing names or a nested
// destructuring.
func nestedTarget(mt *parser.MultiTargetNode) bool {
	return mt.Rest != nil || len(mt.Rights) > 0 || slices.ContainsFunc(mt.Lefts, func(x parser.Node) bool { _, ok := x.(*parser.MultiTargetNode); return ok })
}

// asWriteTarget is a block parameter's target as a multiple assignment's: names become local targets.
func asWriteTarget(n parser.Node) parser.Node {
	switch x := n.(type) {
	case *parser.RequiredParameterNode:
		return &parser.LocalVariableTargetNode{Location: x.Location, Name: x.Name}
	case *parser.SplatNode:
		s := &parser.SplatNode{Location: x.Location, OperatorLoc: x.OperatorLoc}
		if x.Expression != nil {
			s.Expression = asWriteTarget(x.Expression)
		}
		return s
	case *parser.MultiTargetNode:
		out := &parser.MultiTargetNode{Location: x.Location, LparenLoc: x.LparenLoc, RparenLoc: x.RparenLoc}
		for _, l := range x.Lefts {
			out.Lefts = append(out.Lefts, asWriteTarget(l))
		}
		if x.Rest != nil {
			out.Rest = asWriteTarget(x.Rest)
		}
		for _, r := range x.Rights {
			out.Rights = append(out.Rights, asWriteTarget(r))
		}
		return out
	}
	return n
}

// lowerBlockKeywords is a call's block whose list has keyword parameters: MRI fills them only from keywords the call
// passes explicitly, never from a yielded Hash, so where the method yields none they are their defaults (decision 369).
// They become prologue assignments (`b = :b`, `k = {}`, a required one MRI's ArgumentError); the call checks its method
// yields no keywords (`kwBlocks`). A Proc literal (`proc`, `lambda`, `Proc.new`) keeps them unsupported: its calls
// are not known here.
func (f *File) lowerBlockKeywords(c *parser.CallNode) {
	b, ok := c.Block.(*parser.BlockNode)
	if !ok || c.Receiver == nil && (c.Name == "proc" || c.Name == "lambda") || c.Name == "new" || c.Name == "define_method" {
		return
	}
	bp, ok := b.Parameters.(*parser.BlockParametersNode)
	if !ok || bp.Parameters == nil || len(bp.Parameters.Keywords) == 0 && bp.Parameters.KeywordRest == nil || bp.Parameters.Block != nil {
		return
	}
	ps := bp.Parameters
	loc := b.Location
	var pro []parser.Node
	for _, k := range ps.Keywords {
		switch v := k.(type) {
		case *parser.OptionalKeywordParameterNode:
			pro = append(pro, &parser.LocalVariableWriteNode{Location: v.Location, Name: v.Name, NameLoc: v.NameLoc, Value: v.Value})
		case *parser.RequiredKeywordParameterNode:
			msg := &parser.StringNode{Location: v.Location, Unescaped: parser.RubyString{Value: "missing keyword: :" + v.Name}}
			pro = append(pro, &parser.CallNode{Location: v.Location, Name: "raise", Arguments: &parser.ArgumentsNode{Location: v.Location,
				Arguments: []parser.Node{&parser.ConstantReadNode{Location: v.Location, Name: "ArgumentError"}, msg}}})
		default:
			return
		}
	}
	if kr, ok := ps.KeywordRest.(*parser.KeywordRestParameterNode); ok && kr.Name != nil {
		pro = append(pro, &parser.LocalVariableWriteNode{Location: kr.Location, Name: *kr.Name, NameLoc: kr.Location, Value: &parser.HashNode{Location: kr.Location}})
	}
	ps.Keywords, ps.KeywordRest = nil, nil
	if f.kwBlocks == nil {
		f.kwBlocks = map[*parser.BlockNode]bool{}
	}
	f.kwBlocks[b] = true
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
}

// yieldsKeywords reports a def whose body yields keyword arguments, which a lowered block's keyword parameters would take.
func yieldsKeywords(m *Method) bool {
	return m.Node != nil && anyNode(m.Node.Body, func(x parser.Node) bool {
		y, ok := x.(*parser.YieldNode)
		return ok && y.Arguments != nil && slices.ContainsFunc(y.Arguments.Arguments, func(a parser.Node) bool { _, kw := a.(*parser.KeywordHashNode); return kw })
	})
}
