package compiler

import (
	"fmt"
	"slices"
	"strings"

	"github.com/danielgatis/go-ruby-prism/parser"
	"github.com/jtarchie/ruby2go/internal/rbs"
)

// armRet is the return type of the first of m's `.rbs` overloads the
// call's arguments fit (decision 12): by count, then each argument's class,
// then the literal value of a literal or constant argument against a
// literal type.
func (f *fctx) armRet(n parser.Node, m *Method, args []parser.Node) Type {
	sc := typeScope{class: m.Owner, lex: m.Scope, srcPos: srcPos{m.File, m.Line}}
arms:
	for _, arm := range m.arms {
		req := 0
		for _, p := range arm.Params {
			if !p.Optional {
				req++
			}
		}
		if len(args) < req || len(args) > len(arm.Params) {
			continue
		}
		sc.methodTPs = arm.TypeParams
		for i, a := range args {
			if !f.argFits(a, arm.Params[i].Type, sc) {
				continue arms
			}
		}
		return f.c.resolveType(arm.Return, sc)
	}
	arms := make([]string, len(m.arms))
	for i, a := range m.arms {
		arms[i] = a.String()
	}
	f.errorf(n, "no signature of %s matches this call: %s", m.Name, strings.Join(arms, " | "))
	return nil
}

// argFits reports whether argument a may be passed as an arm's parameter of type t.
func (f *fctx) argFits(a parser.Node, t rbs.Type, sc typeScope) bool {
	switch t := t.(type) {
	case rbs.Literal:
		v, ok := f.c.literalOf(f.f, a, f.lex)
		return ok && v == t
	case rbs.Union:
		return slices.ContainsFunc(t.Elems, func(e rbs.Type) bool { return f.argFits(a, e, sc) })
	case rbs.Bool, rbs.Bot, rbs.Intersection, rbs.Name, rbs.Nil, rbs.Optional, rbs.Proc, rbs.Record, rbs.Self, rbs.Instance, rbs.Singleton, rbs.Tuple, rbs.Untyped, rbs.Void: // by the argument's class
	}
	var at expr
	f.probe(func() { at = f.genExpr(a, nil) })
	return fits(at.typ, f.c.resolveType(t, sc))
}

// literalOf is the literal type node n spells, following constants to their value (`Rack::PATH_INFO`).
func (c *Compiler) literalOf(file *File, n parser.Node, lex []*Class) (rbs.Literal, bool) {
	switch n := n.(type) {
	case *parser.StringNode:
		return rbs.Literal{Kind: "String", Value: n.Unescaped.Value}, true
	case *parser.SymbolNode:
		return rbs.Literal{Kind: "Symbol", Value: n.Unescaped.Value}, true
	case *parser.IntegerNode:
		return rbs.Literal{Kind: "Integer", Value: file.text(n.Location)}, true
	case *parser.ConstantReadNode, *parser.ConstantPathNode:
		if _, k := c.lookupConst(file, n, lex); k != nil && k.Value != nil {
			return c.literalOf(k.File, k.Value, k.Scope)
		}
	}
	return rbs.Literal{}, false
}

// narrowToArm types a call by its overload arm's return: the merged
// result, a union or optional, narrows to it, checked at run time.
func (f *fctx) narrowToArm(n parser.Node, out expr, ret Type) expr {
	switch {
	case typeEq(out.typ, ret) || isAny(ret):
		return out
	case isOpt(out.typ) && !isOpt(ret) && !isUnion(out.typ.(TOpt).Elem):
		out = expr{code: fmt.Sprintf("rbMust(%s, %q)", out.code, ret.String()), typ: out.typ.(TOpt).Elem}
		return f.narrowToArm(n, out, ret)
	case isUnion(out.typ) || isAny(out.typ):
		switch r := ret.(type) {
		case TOpt:
			return expr{code: fmt.Sprintf("OptOf[%s](%s, %q)", f.c.goType(r.Elem), out.code, r.Elem.String()), typ: ret}
		default:
			return expr{code: out.code + ".(" + f.c.goType(ret) + ")", typ: ret, assert: true}
		}
	}
	return expr{code: f.coerce(n, out, ret), typ: ret}
}
