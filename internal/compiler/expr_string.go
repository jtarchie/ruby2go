package compiler

import (
	"fmt"

	"github.com/danielgatis/go-ruby-prism/parser"
)

// genMatchWrite is `/(?<year>\d+)/ =~ s`: =~ through Regexp#match, then
// each named group into the local of its name (nil when nothing matched).
func (f *fctx) genMatchWrite(n *parser.MatchWriteNode) expr {
	call := n.Call
	re := f.genExpr(call.Receiver, nil)
	m := f.genMethodCall(call, re, "match", callArgs(call), nil)
	tmp := f.newTmp()
	f.emit("%s := %s", tmp, f.materialize(m))
	for _, t := range n.Targets {
		lt, ok := t.(*parser.LocalVariableTargetNode)
		if !ok {
			f.errorf(t, "unsupported named capture target %s", nodeType(t))
		}
		f.assignLocal(lt, lt.Name, expr{code: fmt.Sprintf("rbMatchCapture(%s, %q)", tmp, lt.Name), typ: TOpt{Elem: f.cls("String")}}, nil)
	}
	return expr{code: "rbOptFromPtr(rbMatchPos(" + tmp + "))", typ: TOpt{Elem: f.cls("Integer")}}
}
