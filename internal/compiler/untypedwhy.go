package compiler

import (
	"slices"

	"github.com/danielgatis/go-ruby-prism/parser"
)

// RB2GO_UNTYPED_WHY (debug only): the root a dynamic call's untyped receiver came from, so work on typing gem code
// goes where the untyped values start, not where they are used.

// noteAnyOrigin records, for an untyped local, what its first untyped value was.
func (f *fctx) noteAnyOrigin(n parser.Node, name string, val expr) {
	if !f.c.env.UntypedWhy || f.pass != 2 {
		return
	}
	w, ok := n.(*parser.LocalVariableWriteNode)
	if !ok {
		return
	}
	k := f.localKey(name)
	if _, seen := f.anyOrigin[k]; seen {
		return
	}
	var why string
	switch {
	case isAny(val.typ):
		why = f.untypedWhy(w.Value, 1)
	case f.localInfo(name) != nil && isAny(f.localInfo(name).typ):
		why = "local assigned values of different types"
	default:
		return
	}
	if f.anyOrigin == nil {
		f.anyOrigin = map[localKey]string{}
	}
	f.anyOrigin[k] = why
}

// untypedWhy names the root an untyped value at n came from.
func (f *fctx) untypedWhy(n parser.Node, depth int) string {
	if depth > 6 {
		return "deep chain"
	}
	switch n := n.(type) {
	case nil, *parser.SelfNode:
		return "self"
	case *parser.ParenthesesNode:
		if b, ok := n.Body.(*parser.StatementsNode); ok && len(b.Body) == 1 {
			return f.untypedWhy(b.Body[0], depth+1)
		}
	case *parser.LocalVariableReadNode:
		if why, ok := f.anyOrigin[f.localKey(n.Name)]; ok {
			return why
		}
		return f.paramWhy(n.Name)
	case *parser.InstanceVariableReadNode:
		return "ivar"
	case *parser.ConstantReadNode, *parser.ConstantPathNode:
		return "constant"
	case *parser.CallNode:
		return f.callWhy(n, depth)
	}
	return nodeType(n)
}

func (f *fctx) paramWhy(name string) string {
	if f.m == nil {
		return "block param or local"
	}
	i := slices.IndexFunc(f.m.Params, func(p Param) bool { return p.Name == name })
	switch {
	case i < 0 && f.blockSig != nil:
		return "block param (or local set by a non-plain write)"
	case i < 0:
		return "local set by a non-plain write"
	case f.paramWiden[name] != nil:
		return "param widened by reassignment"
	case f.m.Params[i].Pending != "":
		return "param: no typed caller"
	case f.m.declared:
		return "param: sig untyped"
	}
	return "param: unannotated"
}

func (f *fctx) callWhy(n *parser.CallNode, depth int) string {
	var rt Type
	if n.Receiver == nil {
		rt = f.selfType
	} else {
		rt = f.probeType(n.Receiver, nil)
	}
	if isAny(rt) && n.Receiver != nil {
		return f.untypedWhy(n.Receiver, depth+1) // the root is further up
	}
	if n.Name == "[]" || n.Name == "fetch" || n.Name == "dig" {
		return "element of " + typeString(rt)
	}
	if isSelfVar(rt) && f.owner != nil {
		rt = TClass{C: f.owner}
	}
	c, ok := stripOpt(rt).(TClass)
	if !ok {
		return "return of " + n.Name + " on " + typeString(rt)
	}
	e := c.C.lookup(n.Name)
	switch {
	case e == nil:
		return "return of missing " + c.C.RubyName + "#" + n.Name
	case e.M.File != nil && e.M.File.isPrelude():
		return "return of core " + n.Name
	case e.M.declared:
		return "return: sig untyped"
	}
	return "return: unannotated"
}

func typeString(t Type) string {
	if t == nil {
		return "?"
	}
	return t.String()
}
