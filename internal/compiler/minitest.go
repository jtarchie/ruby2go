package compiler

import (
	"strconv"
	"strings"

	"github.com/danielgatis/go-ruby-prism/parser"
)

// mtLitAsserts are the assertions that send a Symbol, and their typed
// twin for a literal one (decision 93).
var mtLitAsserts = map[string]string{
	"assert_operator": "__assert_operator_lit", "refute_operator": "__assert_operator_lit",
	"assert_predicate": "__assert_predicate_lit", "refute_predicate": "__assert_predicate_lit",
	"assert_respond_to": "__assert_respond_to_lit", "refute_respond_to": "__assert_respond_to_lit",
}

// mtLiteral compiles minitest's `assert_operator a, :<, b` (and the
// predicate and respond_to forms) with a literal Symbol on a typed value
// into the call itself, `a < b`, handed to a typed twin that builds the
// message: no send by name at run time (decision 93).
func (f *fctx) mtLiteral(n parser.Node, e *entry, recv expr, args []parser.Node) (expr, bool) {
	lit, ok := mtLitAsserts[e.M.Name]
	if !ok || e.M.Owner == nil || e.M.Owner.RubyName != "Minitest::Test" || len(args) < 2 || len(args) > 4 {
		return expr{}, false
	}
	sym, ok := args[1].(*parser.SymbolNode)
	if !ok {
		return expr{}, false
	}
	var o1 expr
	f.probe(func() { o1 = f.genExpr(args[0], nil) })
	if isAny(o1.typ) || isVoid(o1.typ) {
		return expr{}, false
	}
	op := sym.Unescaped.Value
	refute := strings.HasPrefix(e.M.Name, "refute_")
	msgAt := 2
	if strings.HasSuffix(e.M.Name, "_operator") {
		if len(args) == 2 {
			lit = "__assert_predicate_lit" // assert_operator(o1, :even?) is assert_predicate
		} else {
			msgAt = 3
		}
	}
	if len(args) > msgAt+1 {
		return expr{}, false
	}
	target := e.M.Owner.lookup(lit)
	if target == nil {
		return expr{}, false
	}
	hold := func(x parser.Node) expr {
		v := f.genExpr(x, nil)
		t := f.newTmp()
		f.emit("%s := %s", t, f.materialize(v))
		return expr{code: t, typ: v.typ}
	}
	a := hold(args[0])
	opStr := expr{code: "String(" + strconv.Quote(op) + ")", typ: f.cls("String")}
	litArgs := make([]parser.Node, 0, 6)
	switch lit {
	case "__assert_operator_lit":
		b := hold(args[2])
		test := f.genMethodCall(n, a, op, []parser.Node{&exprNode{Node: args[2], e: b}}, nil)
		litArgs = append(litArgs, &exprNode{Node: args[0], e: a}, &exprNode{Node: args[1], e: opStr}, &exprNode{Node: args[2], e: b}, &exprNode{Node: n, e: test})
	case "__assert_predicate_lit":
		test := f.genMethodCall(n, a, op, nil, nil)
		litArgs = append(litArgs, &exprNode{Node: args[0], e: a}, &exprNode{Node: args[1], e: opStr}, &exprNode{Node: n, e: test})
	default:
		test := f.genMethodCall(n, a, "respond_to?", []parser.Node{sym}, nil)
		litArgs = append(litArgs, &exprNode{Node: args[0], e: a}, &exprNode{Node: args[1], e: opStr}, &exprNode{Node: n, e: test})
	}
	msg := parser.Node(&exprNode{Node: n, e: expr{code: "nil", typ: TNil{}}})
	if len(args) > msgAt {
		msg = args[msgAt]
	}
	litArgs = append(litArgs, msg, &exprNode{Node: n, e: expr{code: "Boolean(" + strconv.FormatBool(refute) + ")", typ: f.cls("Boolean")}})
	return f.callMethod(n, target, recv, litArgs, nil), true
}
