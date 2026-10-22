package compiler

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/danielgatis/go-ruby-prism/parser"
)

// yielderClass is Enumerator::Yielder, whose element type Enumerator.new infers from what its block feeds (decision 140).
const yielderClass = "Enumerator::Yielder"

// inferYielder binds X of a block taking a Yielder[X] (`Enumerator.new { |y| y << 1 }`) from the annotation, else from what a probe of the block feeds y.
func (f *fctx) inferYielder(n parser.Node, m *Method, env map[string]Type, block parser.Node) {
	if m.Block == nil || len(m.Block.Params) != 1 {
		return
	}
	yt, ok := m.Block.Params[0].(TClass)
	if !ok || yt.C.RubyName != yielderClass || len(yt.Args) != 1 {
		return
	}
	v, ok := yt.Args[0].(TVar)
	if !ok {
		return
	}
	if _, bound := env[v.Name]; bound {
		return
	}
	if n == f.retHintNode && f.retHint != nil && unify(m.Ret, f.retHint, env) {
		if _, bound := env[v.Name]; bound {
			return
		}
	}
	var fed []Type
	saved := f.yielderFed
	f.yielderFed = &fed
	probeSig := &BlockSig{Params: []Type{TClass{C: yt.C, Args: []Type{TAny{}}}}, Ret: TVoid{}}
	f.probe(func() { f.genClosure(n, block, probeSig, env) })
	f.yielderFed = saved
	if len(fed) == 0 {
		f.errorf(n, "cannot infer the element type of %s: the block feeds its yielder nothing (`y << value`); annotate the result (`#: Enumerator[Integer]`)", m.Name)
	}
	env[v.Name] = f.joinAll(n, fed)
}

// isProbedYielder is a receiver of the Yielder[untyped] inferYielder probes with.
func (f *fctx) isProbedYielder(t Type) bool {
	tc, ok := t.(TClass)
	return ok && f.yielderFed != nil && tc.C.RubyName == yielderClass && len(tc.Args) == 1 && isAny(tc.Args[0])
}

// noteYielderFeed records the type of what `y << v` (or y.yield v) feeds the probed yielder.
func (f *fctx) noteYielderFeed(recv expr, name string, args []parser.Node) {
	if !f.isProbedYielder(recv.typ) || len(args) != 1 || (name != "<<" && name != "yield") {
		return
	}
	var a expr
	f.probe(func() { a = f.genExpr(args[0], nil) })
	*f.yielderFed = append(*f.yielderFed, a.typ)
}

// yielderBlock is `&y` for a Yielder y: the block `{ |x_0| y << x_0 }`.
func yielderBlock(ba *parser.BlockArgumentNode) *parser.BlockNode {
	loc := ba.Location
	arg := &parser.LocalVariableReadNode{Location: loc, Name: "x_0"}
	call := &parser.CallNode{Location: loc, Receiver: ba.Expression, Name: "<<", Arguments: &parser.ArgumentsNode{Location: loc, Arguments: []parser.Node{arg}}}
	return &parser.BlockNode{
		Location:   loc,
		Locals:     []string{"x_0"},
		Parameters: &parser.BlockParametersNode{Location: loc, Parameters: &parser.ParametersNode{Location: loc, Requireds: []parser.Node{&parser.RequiredParameterNode{Location: loc, Name: "x_0"}}}},
		Body:       &parser.StatementsNode{Location: loc, Body: []parser.Node{call}},
	}
}

// iterEnum is a blockless call of a prelude iterator with no `__<name>_enum` (`(1..3).each`): an Enumerator over its sequence (a user's each without a block is MRI's LocalJumpError).
func (f *fctx) iterEnum(n parser.Node, e *entry, recv expr, codes []string, env map[string]Type) (expr, bool) {
	m := e.M
	rt, ok := recv.typ.(TClass)
	if !ok || rt.C.IsModule || !m.File.isPrelude() || len(m.Block.Params) == 0 || len(codes) != len(m.Params) || slices.ContainsFunc(m.Params, func(p Param) bool { return p.Rest || p.Keyword }) {
		return expr{}, false
	}
	enum := f.c.classes["Enumerator"]
	f.bindTypeParams(n, m, env)
	params := substAll(m.Block.Params, env)
	elem := params[0]
	if len(params) == 2 {
		elem = TTuple{Elems: params}
	}
	et := TClass{C: enum, Args: []Type{elem}}
	ps := []string{"r_ " + f.c.goType(recv.typ)}
	names := make([]string, len(codes))
	inspected := []string{}
	for i := range codes {
		names[i] = "a" + strconv.Itoa(i) + "_"
		ps = append(ps, names[i]+" "+f.c.goType(subst(m.Params[i].Type, env)))
		inspected = append(inspected, "string(rbInspect("+names[i]+"))")
	}
	seq := f.callCode(e, expr{code: "r_", typ: recv.typ}, names, env)
	if len(params) == 2 {
		seq = "rbPairSeq(" + seq + ")"
	}
	meth := strconv.Quote(m.Name)
	if len(codes) > 0 {
		meth += ` + "(" + ` + strings.Join(inspected, ` + ", " + `) + ` + ")"`
	}
	code := fmt.Sprintf("func(%s) %s { return rbEnumOf(%s, any(r_), %s, nil, nil) }(%s)",
		strings.Join(ps, ", "), f.c.goType(et), seq, meth, strings.Join(append([]string{f.materialize(recv)}, codes...), ", "))
	return expr{code: code, typ: et}, true
}

// genToEnum is `to_enum`/`enum_for` (bare, or naming the enclosing method) in a method taking a one-value closure
// block: an Enumerator that calls the method again with a block feeding it (rbSeq stops it early).
func (f *fctx) genToEnum(n *parser.CallNode) (expr, bool) {
	m := f.m
	if n.Name != "to_enum" && n.Name != "enum_for" || n.Receiver != nil && !isSelf(n.Receiver) || n.Block != nil ||
		m == nil || m.Block == nil || m.Iterator || len(m.Block.Params) != 1 || !inInterface(m) && !f.lenient() || m.kwMask() || m.calleeDefaults {
		return expr{}, false
	}
	args := callArgs(n)
	if len(args) > 0 {
		if enumName(args[0], m) != m.Name || len(args)-1 > len(m.Params) {
			return expr{}, false
		}
		args = args[1:]
	}
	var as []string
	for i, p := range m.Params {
		switch {
		case p.Rest:
			return expr{}, false
		case i < len(args):
			as = append(as, f.coerce(n, f.genExpr(args[i], p.Type), p.Type))
		case len(args) > 0 && p.Default != nil: // `to_enum(:each)` leaves the rest at their defaults
			as = append(as, f.coerce(n, f.genExpr(p.Default, p.Type), p.Type))
		default:
			as = append(as, goLocalName(p.Name))
		}
	}
	elem := f.c.goType(m.Block.Params[0])
	as = append(as, f.enumBlock(m, elem))
	call := f.selfCode + "." + m.GoName + "(" + strings.Join(as, ", ") + ")"
	if m.Owner != nil && (m.Owner.IsModule || f.lenient()) && !f.c.isDirectMethod(m) { // the free func: a module's constraint, or a gem class's pruned interface, may not name it (elftools' each_symbol)
		targs := "[Self]"
		if f.c.plainSelf(m) {
			targs = ""
		}
		call = freeFuncName(m) + targs + "(" + f.selfCode + comma(strings.Join(as, ", ")) + ")"
	}
	code := fmt.Sprintf("rbEnumOf(rbSeq(func(y func(%s)) { %s }), any(%s), %q, nil, nil)", elem, call, f.selfCode, m.Name)
	return expr{code: code, typ: TClass{C: f.c.classes["Enumerator"], Args: []Type{m.Block.Params[0]}}}, true
}

// eachReturnsSelf are the iterators whose value in MRI is their receiver.
var eachReturnsSelf = map[string]bool{"each": true, "each_with_index": true, "each_pair": true, "each_key": true, "each_value": true, "reverse_each": true,
	"each_line": true, "each_char": true, "each_byte": true, "times": true}

// enumName is the method a to_enum names: a Symbol literal, or `__method__` for m itself (patchelf's
// `to_enum(__method__, offset)`); "" otherwise.
func enumName(arg parser.Node, m *Method) string {
	switch a := arg.(type) {
	case *parser.SymbolNode:
		return a.Unescaped.Value
	case *parser.CallNode:
		if a.Name == "__method__" && a.Receiver == nil && a.Arguments == nil {
			return m.Name
		}
	}
	return ""
}

// enumBlock is the block to_enum hands m: each value it yields goes to the Enumerator's y.
func (f *fctx) enumBlock(m *Method, elem string) string {
	if isVoid(m.Block.Ret) {
		return fmt.Sprintf("func(x %s) { y(x) }", elem)
	}
	r := f.c.goType(m.Block.Ret)
	if m.Block.Rest { // `def each_symbol(&block)` of a gem: the block is variadic
		pack := "x[0]"
		if elem == "any" {
			pack = "rbPackYield(x)"
		}
		return fmt.Sprintf("func(x ...%s) %s { y(%s); return %s }", elem, r, pack, goZero(r))
	}
	return fmt.Sprintf("func(x %s) %s { y(x); return %s }", elem, r, goZero(r))
}
