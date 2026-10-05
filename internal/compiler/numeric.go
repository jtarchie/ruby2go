package compiler

import (
	"slices"
	"strconv"
	"strings"

	"github.com/danielgatis/go-ruby-prism/parser"
)

// numTower orders the number classes as MRI's coerce converts between them: a mix of two runs as the higher one (decision 142).
var numTower = []string{"Integer", "Rational", "Float", "BigDecimal", "Complex"}

// numLevel is c's place in numTower, -1 for any other class.
func numLevel(c *Class) int {
	if c == nil {
		return -1
	}
	return slices.Index(numTower, c.Name)
}

// towerOps coerce mixed operands; not `**`, whose class in MRI depends on the values (2 ** Rational(1, 2) is a Float).
var towerOps = map[string]bool{
	"+": true, "-": true, "*": true, "/": true, "%": true, "modulo": true, "remainder": true, "div": true, "divmod": true,
	"fdiv": true, "quo": true, "<=>": true, "<": true, "<=": true, ">": true, ">=": true, "step": true,
}

// numericTower: a mix no twin takes converts its lower operand up numTower and runs typed, as MRI's coerce; Comparable's go through rbNum, as clamp answers the bound itself. Arguments are generated once and handed back as exprNodes, since probing them first made nested arithmetic exponential.
func (f *fctx) numericTower(n parser.Node, recv expr, e *entry, args []parser.Node, block parser.Node) (expr, []parser.Node, bool) {
	a := classOf(recv.typ)
	la := numLevel(a)
	name := e.M.Name
	viaCmp := e.M.Owner == f.c.classes["Comparable"]
	if la < 0 || len(args) == 0 || !viaCmp && !towerOps[name] {
		return expr{}, args, false
	}
	self := map[string]Type{"Self": recv.typ}
	xs, nodes := f.genNumArgs(e.M, self, args)
	if xs == nil {
		return expr{}, args, false
	}
	f.curBlock = block
	if tw := f.overload(e, recv.typ, nodes); tw != nil {
		if len(args) == 1 && classTwins(name) && tw.M.Name != "__"+overloadBase(name)+"_"+strconv.Itoa(len(args)) && !strings.HasSuffix(tw.M.Name, "_enum") {
			f.curBlock = nil
			return expr{}, nodes, false // a twin for the argument's class: MRI's own mix
		}
		e = tw // `r.step(2)` is __step_enum: its parameters are the ones to convert to
	}
	f.curBlock = nil
	m := e.M
	if len(args) > len(m.Params) || (block != nil) != (m.Block != nil) {
		return expr{}, nodes, false
	}
	for i := range args {
		if m.Params[i].Rest || !typeEq(subst(m.Params[i].Type, self), TClass{C: a}) {
			return expr{}, nodes, false
		}
	}
	top, mixed, intFloat := la, false, isNumeric(recv.typ)
	for _, x := range xs {
		if l := numLevel(classOf(x.typ)); l >= 0 && l != la {
			mixed, top, intFloat = true, max(top, l), intFloat && isNumeric(x.typ)
		}
	}
	if !mixed || intFloat || top > la && (len(args) != 1 || block != nil) {
		return expr{}, nodes, false
	}
	if viaCmp {
		codes := make([]string, len(xs))
		for i, x := range xs {
			codes[i] = f.coerce(args[i], x, TAny{})
		}
		return f.rbNumCall(n, m, f.coerce(n, recv, TAny{}), codes), nil, true
	}
	target := f.c.classes[numTower[top]]
	for i, x := range xs {
		nodes[i] = &exprNode{Node: args[i], e: f.numConvert(args[i], x, target)}
	}
	if top == la {
		return f.callEntry(n, e, recv, nodes, block), nil, true
	}
	return f.genMethodCall(n, f.numConvert(n, recv, target), name, nodes, block), nil, true
}

// genNumArgs generates plain arguments once, hinted as numericMix does by m's parameter types; nil for a splat or keywords.
func (f *fctx) genNumArgs(m *Method, self map[string]Type, args []parser.Node) ([]expr, []parser.Node) {
	for _, x := range args {
		switch x.(type) {
		case *parser.SplatNode, *parser.KeywordHashNode:
			return nil, nil
		}
	}
	xs := make([]expr, len(args))
	nodes := make([]parser.Node, len(args))
	for i, x := range args {
		var hint Type
		if i < len(m.Params) && !m.Params[i].Rest {
			hint = subst(m.Params[i].Type, self)
		}
		xs[i] = f.genExpr(x, hint)
		nodes[i] = &exprNode{Node: x, e: xs[i]}
	}
	return xs, nodes
}

// checkNumeric: Numeric's methods only type calls on a Numeric value (Go any), so each number class must define or undefine every one, or a call would reach a body that raises.
func (c *Compiler) checkNumeric() {
	num := c.classes["Numeric"]
	if num == nil {
		return
	}
	for _, name := range numTower {
		cls := c.classes[name]
		if cls == nil {
			continue
		}
		for _, m := range num.MethodList {
			if e := cls.lookup(m.Name); e != nil && e.Owner == num {
				c.errorf(cls.File, nil, "%s:%d: %s must define or undef Numeric#%s", cls.File.Name, cls.Line, cls.RubyName, m.Name)
			}
		}
	}
}

// numericIterArgs is numericTower for an iterator call (`Rational(1, 3).step(1) { }`): lower arguments convert, an Integer receiver with a Float one widens.
func (f *fctx) numericIterArgs(e *entry, recv expr, args []parser.Node) (*entry, expr, []parser.Node) {
	a := classOf(recv.typ)
	la := numLevel(a)
	if la < 0 || !towerOps[e.M.Name] || len(args) > len(e.M.Params) {
		return e, recv, args
	}
	xs := make([]expr, len(args))
	top := la
	f.probe(func() {
		for i, x := range args {
			if _, splat := x.(*parser.SplatNode); !splat {
				xs[i] = f.genExpr(x, nil)
				top = max(top, numLevel(classOf(xs[i].typ)))
			}
		}
	})
	target := a
	if top > la {
		fe := f.c.classes["Float"].lookup(e.M.Name)
		if a.Name != "Integer" || numTower[top] != "Float" || fe == nil || !fe.M.Iterator {
			return e, recv, args
		}
		e, target, recv = fe, fe.Owner, f.numConvert(nil, recv, f.c.classes["Float"])
	}
	out := slices.Clone(args)
	self := map[string]Type{"Self": TClass{C: target}}
	for i, x := range args {
		if l := numLevel(classOf(xs[i].typ)); l >= 0 && typeEq(subst(e.M.Params[i].Type, self), TClass{C: target}) {
			out[i] = &exprNode{Node: x, e: f.numConvert(x, f.genExpr(x, nil), target)}
		}
	}
	return e, recv, out
}

// numConvert converts number x to class target, higher in numTower; a BigDecimal has no Complex form here.
func (f *fctx) numConvert(n parser.Node, x expr, target *Class) expr {
	from := classOf(x.typ)
	if numLevel(from) < 0 || from == target {
		return x
	}
	name := map[string]string{"Rational": "to_r", "Float": "to_f", "BigDecimal": "to_d", "Complex": "to_c"}[target.Name]
	switch {
	case target.Name == "BigDecimal" && from.Name == "Rational":
		name = "__to_d" // to_d needs digits; MRI's coerce takes the BigDecimal's
	case target.Name == "Complex" && from.Name == "BigDecimal":
		f.errorf(n, "a BigDecimal does not mix with a Complex (rb2go's Complex parts are Integer, Rational or Float; decision 142)")
	}
	return f.genMethodCall(n, x, name, nil, nil)
}
