package compiler

import (
	"fmt"

	"github.com/danielgatis/go-ruby-prism/parser"
)

// countedLoopHead: Go does not inline a loop body past its budget into an iterator, so each element of times/upto/downto/(a..b).each was an indirect closure call (decision 385).
func (f *fctx) countedLoopHead(n *parser.CallNode, e *entry, recv expr, codes []string, names blockParams, goParams []string) string {
	m := e.M
	if f.c.env.NoCountedLoop || m.Owner == nil || m.File == nil || !m.File.isPrelude() || len(goParams) > 1 {
		return ""
	}
	gp := "_"
	if len(goParams) == 1 {
		gp = goParams[0]
	}
	switch {
	case m.Owner == f.c.classes["Integer"] && m.Name == "times" && len(codes) == 0:
		if gp == "_" {
			return fmt.Sprintf("for range Integer(%s) {", recv.code)
		}
		return fmt.Sprintf("for %s := range Integer(%s) {", gp, recv.code) // a range over an int copies: the block may assign its param
	case m.Owner == f.c.classes["Integer"] && (m.Name == "upto" || m.Name == "downto") && len(codes) == 1:
		if gp = f.countedVar(n, names, gp); gp == "" {
			return ""
		}
		from, to := f.newTmp(), f.newTmp()
		f.emit("%s, %s := Integer(%s), Integer(%s)", from, to, recv.code, codes[0])
		if m.Name == "upto" { // gp >= from: past MaxInt gp++ wraps below it
			return fmt.Sprintf("for %s := %s; %s <= %s && %s >= %s; %s++ {", gp, from, gp, to, gp, from, gp)
		}
		return fmt.Sprintf("for %s := %s; %s >= %s && %s <= %s; %s-- {", gp, from, gp, to, gp, from, gp)
	case m.Owner == f.c.classes["Range"] && m.Name == "each" && len(codes) == 0 && isClass(rangeElem(recv.typ), "Integer"):
		rn := rangeLiteral(n.Receiver)
		if rn == nil || rn.Left == nil || rn.Right == nil {
			return "" // endless, beginless or a Range value: rbRangeEach knows its flags
		}
		if gp = f.countedVar(n, names, gp); gp == "" {
			return ""
		}
		r, from, to := f.newTmp(), f.newTmp(), f.newTmp()
		f.emit("%s := %s", r, recv.code)
		f.emit("%s, %s := %s.b, %s.e", from, to, r, r) // locals, not the struct's fields: Go reloaded them every iteration
		if rn.IsEXCLUDE_END() {
			return fmt.Sprintf("for %s := %s; %s < %s; %s++ {", gp, from, gp, to, gp)
		}
		return fmt.Sprintf("for %s := %s; %s <= %s && %s >= %s; %s++ {", gp, from, gp, to, gp, from, gp)
	}
	return ""
}

// countedVar: a block that assigns its param keeps the iterator, as Ruby's next element ignores that write.
func (f *fctx) countedVar(n *parser.CallNode, names blockParams, gp string) string {
	if gp == "_" {
		return f.newTmp()
	}
	if blk, ok := n.Block.(*parser.BlockNode); !ok || len(names) != 1 || assignsLocal(blk.Body, names[0].Name) {
		return ""
	}
	return gp
}

func rangeElem(t Type) Type {
	if c, ok := t.(TClass); ok && c.C.RubyName == "Range" && len(c.Args) == 1 {
		return c.Args[0]
	}
	return nil
}

// rangeLiteral is n as a Range literal, through parentheses.
func rangeLiteral(n parser.Node) *parser.RangeNode {
	for {
		switch x := n.(type) {
		case *parser.RangeNode:
			return x
		case *parser.ParenthesesNode:
			s, ok := x.Body.(*parser.StatementsNode)
			if !ok || len(s.Body) != 1 {
				return nil
			}
			n = s.Body[0]
		default:
			return nil
		}
	}
}
