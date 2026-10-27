package compiler

import (
	"fmt"
	"regexp"
	"slices"

	"github.com/danielgatis/go-ruby-prism/parser"
)

// nilCheckComment is a source's `# nil_check: mode` line (decision 393).
var nilCheckComment = regexp.MustCompile(`(?m)^# nil_check: (\S+)\s*$`)

// checkNilMode rejects a mode other than compile or runtime ("" is compile).
func checkNilMode(where, mode string) string {
	if !slices.Contains([]string{"", "compile", "runtime"}, mode) {
		panic(compileError{msg: fmt.Sprintf("%s: %q is not compile or runtime", where, mode)})
	}
	return mode
}

// nilRuntime: a T? where T is expected raises on nil at run time instead of decision 20's error (file's line, else the flag; decision 393).
func (f *fctx) nilRuntime() bool {
	if f.f != nil && f.f.isPrelude() {
		return false
	}
	if f.f != nil && f.f.nilCheck != "" {
		return f.f.nilCheck == "runtime"
	}
	return f.c.env.NilCheck == "runtime"
}

// nilCall is the call whose argument is being coerced, for the error MRI raises when that argument is nil.
type nilCall struct {
	m     *Method
	recv  Type
	param Type // the parameter as declared: an `E` element (Array#[]=) is no conversion MRI makes
}

var (
	nilCoerceOps  = []string{"+", "-", "*", "/", "%", "**", "&", "|", "^", "fdiv", "div", "divmod", "modulo", "quo", "pow", "remainder"}
	nilCompareOps = []string{"<", ">", "<=", ">=", "between?"}
	nilEqualOps   = map[string]bool{"==": true, "!=": true, "<=>": true, "eql?": true, "equal?": true, "===": true}
)

// nilMessage is MRI's error where a core method's argument is nil, else a TypeError at the boundary (decision 393).
func (f *fctx) nilMessage(to Type) (string, bool) {
	want := to.String()
	if c, ok := to.(TClass); ok {
		want = c.C.RubyName
	}
	m := f.nilArg.m
	if m == nil || m.Owner == nil || m.Owner.File == nil || !m.Owner.File.isPrelude() {
		return "no implicit conversion of nil into " + want, false
	}
	recv := m.Owner.RubyName
	if rc, ok := stripOpt(f.nilArg.recv).(TClass); ok {
		recv = rc.C.RubyName
	}
	numeric := recv == "Integer" || recv == "Float"
	_, declared := f.nilArg.param.(TClass)
	switch {
	case slices.Contains(nilCompareOps, m.Name):
		return "comparison of " + recv + " with nil failed", true
	case numeric && slices.Contains(nilCoerceOps, m.Name):
		return "nil can't be coerced into " + recv, false
	case !declared || recv == "Integer" && (m.Name == "<<" || m.Name == ">>"):
	case want == "Integer":
		return "no implicit conversion from nil to integer", false
	case want == "Float":
		return "can't convert nil into Float", false
	}
	return "no implicit conversion of nil into " + want, false
}

// nilNumArgs unwraps a numeric T? argument before numericTower picks the class to convert to, as MRI's coerce fails on nil first.
func (f *fctx) nilNumArgs(m *Method, recv Type, xs []expr, nodes []parser.Node) {
	if !f.nilRuntime() {
		return
	}
	for i, x := range xs {
		if o, ok := x.typ.(TOpt); ok && numLevel(classOf(o.Elem)) >= 0 {
			prev := f.nilArg
			f.nilArg = nilCall{m: m, recv: recv}
			xs[i] = f.nilUnwrap(x, o.Elem)
			f.nilArg = prev
			nodes[i].(*exprNode).e = xs[i] //nolint:forcetypeassert // genNumArgs makes exprNodes
		}
	}
}

// nilSpaceship is `a <=> b` of a T? b: MRI answers nil for a nil b, where the typed `<=>` would take its unwrapped T.
func (f *fctx) nilSpaceship(n parser.Node, recv expr, name string, args []parser.Node, block parser.Node) (expr, bool) {
	if name != "<=>" || len(args) != 1 || block != nil || !f.nilRuntime() || classOf(recv.typ) == nil {
		return expr{}, false
	}
	if e := classOf(recv.typ).lookup(name); e == nil || e.Owner.File == nil || !e.Owner.File.isPrelude() {
		return expr{}, false // a program's own <=> is called with the nil, as MRI does
	}
	var a expr
	f.probe(func() { a = f.genExpr(args[0], nil) })
	o, ok := a.typ.(TOpt)
	if !ok || isAny(o.Elem) {
		return expr{}, false
	}
	rt, at, res := f.newTmp(), f.newTmp(), f.newTmp()
	f.emit("%s := %s", rt, f.materialize(recv))
	a = f.genExpr(args[0], nil)
	f.emit("%s := %s", at, a.code)
	ret := TOpt{Elem: f.cls("Integer")}
	f.emit("var %s %s", res, f.c.goType(ret))
	f.emit("if %s != nil {", at)
	x := f.genMethodCall(n, expr{code: rt, typ: recv.typ}, name, []parser.Node{&exprNode{Node: args[0], e: expr{code: derefOpt(at, a.typ), typ: o.Elem}}}, nil)
	f.emit("%s = %s", res, f.coerce(n, x, ret))
	f.emit("}")
	return expr{code: res, typ: ret}, true
}

// nilUnwrap is a T? value as its T under --nil-check=runtime, raising nilMessage's error when it is nil (decision 393).
func (f *fctx) nilUnwrap(e expr, to Type) expr {
	o := e.typ.(TOpt) //nolint:forcetypeassert // callers test isOpt
	msg, arg := f.nilMessage(to)
	if leafOpt(o) { // a leaf's *C or an interface: nil is the value's own zero (decisions 360, 378)
		return expr{code: fmt.Sprintf("rbNilArgV(%s, %q, %t)", e.code, msg, arg), typ: o.Elem}
	}
	return expr{code: fmt.Sprintf("rbNilArg(%s, %q, %t)", e.code, msg, arg), typ: o.Elem}
}
