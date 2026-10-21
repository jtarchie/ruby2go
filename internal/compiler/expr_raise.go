package compiler

import (
	"github.com/danielgatis/go-ruby-prism/parser"
)

func (f *fctx) genRaise(n *parser.CallNode) expr {
	args := callArgs(n)
	if k, ok := lastNode(args).(*parser.KeywordHashNode); ok && len(args) > 1 && len(k.Elements) == 1 {
		if a, ok := k.Elements[0].(*parser.AssocNode); ok {
			if s, ok := a.Key.(*parser.SymbolNode); ok && s.Unescaped.Value == "cause" {
				args = args[:len(args)-1] // ponytail: `cause:` is dropped; the lexical cause (decision 103) stands
			}
		}
	}
	exc := f.c.classes["Exception"]
	var val expr
	switch len(args) {
	case 0:
		if f.rescues > 0 { // re-raises the exception being handled
			return expr{code: "panic(r_)", typ: TVoid{}, stmt: true, noreturn: true}
		}
		if f.lenient() { // MRI re-raises $!, the exception a calling rescue clause handles (decision 331), else an empty RuntimeError (233)
			val = f.genNew(n, f.c.classes["RuntimeError"], nil, []expr{{code: `""`, typ: f.cls("String"), lit: true}}, nil, nil)
			tmp := f.newTmp()
			f.emit("if %s := rbErrinfo(); %s != nil {\n\tpanic(%s)\n}", tmp, tmp, tmp)
			break
		}
		f.errorf(n, "bare `raise` outside a rescue clause is not supported")
	case 1:
		val = f.raiseOne(n, args[0], exc)
	case 2, 3:
		cls := f.classRef(args[0])
		if c, ok := args[0].(*parser.CallNode); ok && cls == nil && c.Name == "class" && c.Receiver != nil && c.Arguments == nil {
			// `raise e.class, msg` (rack's re-raise): a copy of e with msg, as Exception#exception(msg) makes
			val = f.genMethodCall(n, f.genExpr(c.Receiver, nil), "exception", args[1:2], nil)
			if cl := classOf(val.typ); !isAny(val.typ) && (cl == nil || !cl.isSubclassOf(exc)) {
				f.errorf(args[0], "raise %s: not an exception", val.typ)
			}
			break
		}
		switch {
		case cls == nil && f.dynRaise() && f.raiseObject(args[0]) == nil: // `raise klass, msg` with klass held untyped (decision 233; the prelude's Thread#raise)
			f.c.noteDyn("new")
			k := f.genExpr(args[0], nil)
			msg := f.genExpr(args[1], f.cls("String"))
			val = expr{code: "rbRaiseValue(" + f.coerce(args[0], k, TAny{}) + ", " + f.coerce(args[1], msg, TAny{}) + ")", typ: TClass{C: exc}}
		case cls == nil:
			// `raise err, msg[, backtrace]`: Exception#exception(msg), a copy of err with msg (decision 241)
			e := f.raiseObject(args[0])
			if e == nil {
				f.errorf(args[0], "raise Class, message: first argument must be a class or an exception object")
			}
			val = f.genMethodCall(n, *e, "exception", args[1:2], nil)
		default:
			if !cls.isSubclassOf(exc) {
				f.errorf(args[0], "%s is not an exception class", cls.RubyName)
			}
			msg := f.genExpr(args[1], f.cls("String"))
			val = f.genNew(n, cls, nil, []expr{msg}, nil, nil)
		}
		if len(args) == 3 { // `raise C, msg, backtrace`
			tmp := f.newTmp()
			f.emit("%s := %s", tmp, val.code)
			val.code = tmp
			f.emitExprStmt(n, f.genMethodCall(n, val, "set_backtrace", args[2:], nil))
		}
	default:
		f.errorf(n, "raise with %d arguments is not supported", len(args))
	}
	return expr{code: "panic(" + f.withCause(val.code) + ")", typ: TVoid{}, stmt: true, noreturn: true}
}

// raiseObject is `raise x, msg`'s x when it is an exception object, else nil; it generates x only when it is one.
func (f *fctx) raiseObject(arg parser.Node) *expr {
	t := f.probeType(arg, nil)
	if c := classOf(t); c == nil || !c.isSubclassOf(f.c.classes["Exception"]) {
		return nil
	}
	e := f.genExpr(arg, nil)
	return &e
}

// raiseOne is the exception `raise x` raises: a class's new, a String's RuntimeError, an exception itself.
func (f *fctx) raiseOne(n, arg parser.Node, exc *Class) expr {
	if cls := f.classRef(arg); cls != nil {
		if !cls.isSubclassOf(exc) {
			f.errorf(arg, "%s is not an exception class", cls.RubyName)
		}
		return f.genNew(n, cls, nil, []expr{}, nil, nil)
	}
	a := f.genExpr(arg, nil)
	if isClass(a.typ, "String") {
		return f.genNew(n, f.c.classes["RuntimeError"], nil, []expr{a}, nil, nil)
	}
	if c := classOf(a.typ); c != nil && c.isSubclassOf(exc) {
		return a
	}
	if isNil(a.typ) && f.pass < 2 { // `e = nil; ...; e = err`: pass 1 reads the local before its later writes widen it (decision 241)
		return expr{code: "nil", typ: TClass{C: exc}}
	}
	if (isAny(a.typ) || isUnion(a.typ)) && f.dynRaise() { // an untyped (or mixed) class, message or exception, decided at run time (decision 233)
		f.c.noteDyn("new")
		return expr{code: "rbRaiseValue(" + f.coerce(arg, a, TAny{}) + ", nil)", typ: TClass{C: exc}}
	}
	f.errorf(arg, "raise needs an exception class, a String, or an exception object (got %s)", a.typ)
	return expr{}
}

// dynRaise: an untyped class or message is raised as decided at run time, in gem code (decision 233) and the
// prelude's Thread#raise (decision 257).
func (f *fctx) dynRaise() bool { return f.lenient() || f.f.isPrelude() }

// gemBareRaise reports whether some gem file has a receiverless `raise` with no arguments: only then do gem rescue
// clauses record $! for it (decision 331).
func (c *Compiler) gemBareRaise() bool {
	if c.bareRaise == 0 {
		c.bareRaise = 1
		for _, f := range c.userFiles {
			if c.gemFile(f) && anyNode(f.Root, func(n parser.Node) bool {
				call, ok := n.(*parser.CallNode)
				return ok && call.Name == "raise" && call.Receiver == nil && call.Arguments == nil
			}) {
				c.bareRaise = 2
				break
			}
		}
	}
	return c.bareRaise == 2
}
