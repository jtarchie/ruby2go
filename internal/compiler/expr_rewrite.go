package compiler

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/danielgatis/go-ruby-prism/parser"
)

// genCallRewrite is a call compiled as another expression: Regexp.last_match on the caller's $~ (decision 232), a
// frozen proc literal at its expected type (decision 226).
func (f *fctx) genCallRewrite(n *parser.CallNode, expected Type) (expr, bool) {
	if r, ok := n.Receiver.(*parser.ConstantReadNode); ok && r.Name == "Regexp" && n.Name == "last_match" && n.Block == nil && len(callArgs(n)) <= 1 {
		// the caller's $~ (decision 171: it is the frame's), handed to the helper that indexes it
		gv := &parser.GlobalVariableReadNode{Location: n.Location, Name: "$~"}
		if len(callArgs(n)) == 0 {
			return f.genExpr(gv, expected), true
		}
		at := &parser.CallNode{Location: n.Location, Receiver: n.Receiver, Name: "__last_match_at",
			Arguments: &parser.ArgumentsNode{Location: n.Location, Arguments: []parser.Node{gv, callArgs(n)[0]}}}
		return f.genCall(at, expected), true
	}
	if r, ok := n.Receiver.(*parser.CallNode); ok && r.Name == "singleton_class" && r.Arguments == nil && n.Block == nil &&
		(n.Name == "<" || n.Name == "<=" || n.Name == "include?") && len(callArgs(n)) == 1 {
		// `x.singleton_class < M`: M is among x's singleton ancestors, which is x.is_a?(M) for a module M (decision 241)
		if cls := f.classRef(callArgs(n)[0]); cls != nil && cls.IsModule {
			self := r.Receiver
			if self == nil {
				self = &parser.SelfNode{Location: r.Location}
			}
			isA := &parser.CallNode{Location: n.Location, Receiver: self, Name: "is_a?", Arguments: n.Arguments}
			if n.Name == "include?" {
				return f.genCall(isA, expected), true
			}
			return f.genExpr(&parser.OrNode{Location: n.Location, Left: isA, Right: &parser.NilNode{Location: n.Location}}, expected), true // nil when unrelated
		}
	}
	if sym, ok := n.Receiver.(*parser.SymbolNode); ok && n.Name == "to_proc" && n.Arguments == nil && n.Block == nil {
		// `:<=.to_proc` where a Proc is expected (`@order = block || :<=.to_proc`): the lambda `&:<=` would pass, typed by it
		ft, ok := stripOpt(expected).(TFunc)
		switch {
		case ok && ft.Proc && untypedRest(ft.Params, ft.Rest):
			// a gem's untyped Proc (decision 244): the first argument receives the name, dispatched at run time
			return expr{code: "Ref(" + f.symbolProcAny(sym.Unescaped.Value) + ")", typ: ft}, true
		case ok && ft.Proc:
			return f.genLambda(n, &parser.BlockArgumentNode{Location: n.Location, Expression: sym}, nil, ft), true
		}
	}
	if f.lenient() && f.undefsSingleton(n) {
		// `singleton_class.send(:undef_method, :inherited)` ahead of a define_singleton_method (bindata's register_subclasses):
		// the closed world has the one definition boot recorded, so removing the earlier one is a no-op (decision 301)
		return expr{code: "", typ: TVoid{}, stmt: true, done: true}, true
	}
	if e, ok := f.runtimeExtend(n); ok {
		return e, true
	}
	f.noteRuntimeRegexp(n)
	if e, ok := f.callSugar(n, expected); ok {
		return e, true
	}
	if procLiteral(n.Receiver) && (n.Name == "freeze" || n.Name == "itself") && n.Arguments == nil && n.Block == nil {
		if _, fn := expected.(TFunc); fn { // `T.let(-> {}.freeze, T.proc.void)`: the lambda at the expected type; freezing a Proc changes nothing it does
			return f.genExpr(n.Receiver, expected), true
		}
	}
	return expr{}, false
}

// runtimeExtend is `extend M` run by a class method (`safe_initialization!`): the closed world extends classes when
// they load (selfExtendCall), so at run time it is a no-op for a class that has M, and NotImplementedError for one that
// does not (decision 241).
func (f *fctx) runtimeExtend(n *parser.CallNode) (expr, bool) {
	if n.Name != "extend" || n.Receiver != nil || n.Block != nil || f.m == nil || f.owner == nil || f.owner.metaOf == nil || len(callArgs(n)) == 0 {
		return expr{}, false
	}
	self := expr{code: f.selfCode, typ: f.selfType}
	var checks []string
	for _, a := range callArgs(n) {
		if f.classRef(a) == nil {
			return expr{}, false
		}
		checks = append(checks, f.isACheck(n, self, a))
	}
	msg := fmt.Sprintf("rb2go: extend at run time is not supported (%s:%d)", f.f.Name, f.f.line(n.Location.StartOffset))
	f.emit("if !(%s) {", strings.Join(checks, " && "))
	f.emit("\tpanic(NewNotImplementedError(Ref(String(%q))))", msg)
	f.emit("}")
	return expr{code: f.selfCode, typ: f.selfType}, true
}

// noteRuntimeRegexp: Regexp.new/compile/union of anything but string literals RE2 compiles builds a pattern only run
// time knows, so the program gets the regexp2 engine (decision 242).
func (f *fctx) noteRuntimeRegexp(n *parser.CallNode) {
	isRegexp := false
	switch r := n.Receiver.(type) {
	case *parser.ConstantReadNode:
		isRegexp = r.Name == "Regexp"
	case *parser.ConstantPathNode: // ::Regexp
		isRegexp = r.Parent == nil && r.Name != nil && *r.Name == "Regexp"
	}
	if !isRegexp || n.Name != "new" && n.Name != "compile" && n.Name != "union" || f.c.needRxAlt || f.f.isPrelude() {
		return
	}
	for _, a := range callArgs(n) {
		s, ok := a.(*parser.StringNode)
		if !ok {
			f.c.needRxAlt = true
			return
		}
		p, err := rxPrepare(s.Unescaped.Value, false) // only decides whether the alternate engine is needed
		if err == nil && n.Name != "union" {
			_, err = regexp.Compile(p)
		}
		if err != nil {
			f.c.needRxAlt = true
			return
		}
	}
}

// callSugar rewrites a call into the form rb2go compiles (decision 241): `ENV.fetch(k, d)` is `ENV[k] || d` (an ENV
// value is a String, so this is fetch's answer whatever d is), and renamedCall's.
func (f *fctx) callSugar(n *parser.CallNode, expected Type) (expr, bool) {
	if r, ok := n.Receiver.(*parser.ConstantReadNode); ok && r.Name == "ENV" && n.Name == "fetch" && len(callArgs(n)) == 2 && n.Block == nil {
		idx := &parser.CallNode{Location: n.Location, Receiver: n.Receiver, Name: "[]", Arguments: &parser.ArgumentsNode{Location: n.Location, Arguments: callArgs(n)[:1]}}
		return f.genExpr(&parser.OrNode{Location: n.Location, Left: idx, Right: callArgs(n)[1]}, expected), true
	}
	if to := renamedCall(n); to != "" {
		c := *n
		c.Name = to
		return f.genCall(&c, expected), true
	}
	return expr{}, false
}

// renamedCall is the prelude name a call goes to (decision 241): Thread.fork is MRI's alias of Thread.start (defined as
// itself it would override Kernel#fork), and Kernel#exit! is __exit_bang, a name no dispatcher lists.
func renamedCall(n *parser.CallNode) string {
	if r, ok := n.Receiver.(*parser.ConstantReadNode); ok && r.Name == "Thread" && n.Name == "fork" {
		return "start"
	}
	if n.Name == "exit!" && n.Receiver == nil && n.Block == nil {
		return "__exit_bang"
	}
	return ""
}

// genCallForm is a call some form of genCall's handles before ordinary dispatch: a Sorbet cast, a boot-compiled
// define_method or eval (a no-op), to_enum, a Kernel intrinsic, a String bang, a block call, a bare name.
func (f *fctx) genCallForm(n *parser.CallNode, expected Type) (expr, bool) {
	if t, ok := f.f.sorbetCasts[n]; ok {
		return f.genSorbetCast(n, t), true
	}
	if n.Receiver != nil && f.c.dmSites[n] || f.isEvalSite(n) || f.c.blockEvalSites[n] { // boot made their methods defs (decisions 198, 204)
		return expr{code: "nil", typ: TNil{}, lit: true}, true
	}
	for _, form := range []func() (expr, bool){
		func() (expr, bool) { return f.genToEnum(n) },
		func() (expr, bool) { return f.genKernelCall(n, expected) },
		func() (expr, bool) { return f.genMatchSet(n) },
		func() (expr, bool) { return f.genStringBang(n) },
		func() (expr, bool) { return f.genBlockCall(n) },
		func() (expr, bool) { return f.genBareName(n, expected) },
	} {
		if e, ok := form(); ok {
			return e, true
		}
	}
	return expr{}, false
}

// undefsSingleton is `singleton_class.undef_method(:x)`, `.remove_method(:x)` or `.send(:undef_method, :x)` on the current class's singleton.
func (f *fctx) undefsSingleton(n *parser.CallNode) bool {
	r, ok := n.Receiver.(*parser.CallNode)
	if !ok || r.Name != "singleton_class" || r.Arguments != nil || r.Receiver != nil && !isSelf(r.Receiver) || n.Block != nil {
		return false
	}
	switch n.Name {
	case "undef_method", "remove_method":
		return true
	case "send", "public_send", "__send__":
		if args := callArgs(n); len(args) > 0 {
			if sym, ok := args[0].(*parser.SymbolNode); ok {
				return sym.Unescaped.Value == "undef_method" || sym.Unescaped.Value == "remove_method"
			}
		}
	}
	return false
}

// untypedRest is a gem's untyped Proc or block signature, `^(*untyped) -> untyped` (decision 244).
func untypedRest(params []Type, rest bool) bool { return rest && len(params) == 1 && isAny(params[0]) }

// symbolProcAny is `&:name` as an untyped Proc: its first argument receives name, the rest are passed on, dispatched at
// run time, as MRI's Symbol#to_proc.
func (f *fctx) symbolProcAny(name string) string {
	f.c.noteDyn(name)
	return "func(xs_ ...any) any { if len(xs_) == 0 { panic(NewArgumentError(Ref(String(\"no receiver given\")))) }; " +
		"return rbDyn" + goMethodName(name) + "(rbCall, xs_[0], xs_[1:]...) }"
}
