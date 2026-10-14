package compiler

import (
	"fmt"
	"slices"
	"strings"

	"github.com/danielgatis/go-ruby-prism/parser"
)

// dynEach is a gem's `x.each { |a, b| }` on an untyped x, as a Rack app's headers and body are (decision 176).
func (f *fctx) dynEach(n parser.Node, recv expr, b *parser.BlockNode) expr {
	k := len(f.blockParamList(b.Parameters))
	if k == 0 || k > 2 {
		f.errorf(n, "each on an untyped receiver takes a block of one or two parameters")
	}
	f.c.dynEach = true
	clo := f.genClosure(n, b, &BlockSig{Params: []Type{TAny{}, TAny{}}[:k], Ret: TVoid{}}, nil)
	xs := []string{"xs_[0]", "xs_[1]"}[:k]
	return expr{code: fmt.Sprintf("rbEachAny(%s, %d, func(xs_ ...any) { (%s)(%s) })", recv.code, k, clo, strings.Join(xs, ", ")), typ: TAny{}}
}

// universalBlockCall is universalCall with a block: one method every receiver shares, or a gem's untyped each.
func (f *fctx) universalBlockCall(n parser.Node, recv expr, name string, args []parser.Node, block parser.Node) expr {
	if e := f.c.universalOnly(name); e != nil { // the same method on every receiver: no dispatch needed
		return f.callEntry(n, e, recv, args, block)
	}
	if f.c.isNumericMod(classOf(recv.typ)) {
		f.errorf(n, "a block on a Numeric is not supported: narrow it with is_a? first (decision 142)")
	}
	if b, ok := block.(*parser.BlockNode); ok && name == "each" && len(args) == 0 && f.lenient() && isAny(recv.typ) {
		return f.dynEach(n, recv, b)
	}
	if ba, ok := block.(*parser.BlockArgumentNode); ok && name == "each" && len(args) == 0 && f.lenient() && f.c.goType(recv.typ) != "" && (isAny(recv.typ) || isTypeVar(recv.typ)) &&
		f.isBlockParam(ba.Expression) && !f.iterator && f.blockSig != nil && len(f.blockSig.Params) >= 1 && len(f.blockSig.Params) <= 2 {
		return f.dynEachForward(n, recv)
	}
	if _, tv := recv.typ.(TVar); f.lenient() && (isAny(recv.typ) || tv) && dynEnumNames[name] { // untyped, or a gem module's self
		return f.dynEnumBlock(n, expr{code: f.coerce(n, recv, TAny{}), typ: TAny{}}, name, args, block)
	}
	if b, ok := block.(*parser.BlockNode); ok && f.lenient() {
		return f.genDynBlockCall(n, recv, name, args, b)
	}
	if _, ok := block.(*parser.BlockArgumentNode); ok && f.lenient() { // a method's own `&block` passed on (concurrent-ruby's `ns_initialize(opts, &block)`)
		if v, ok := f.dynBlockValue(n, block); ok {
			if f.c.dynBlockNames == nil {
				f.c.dynBlockNames = map[string]bool{}
			}
			f.c.dynBlockNames[name] = true
			return f.genDynCall(n, recv, name, append(slices.Clone(args), &exprNode{e: expr{code: v, typ: TAny{}}}))
		}
	}
	f.errorf(n, "blocks on untyped receivers are not supported")
	return expr{}
}

// genDynBlockCall is a gem's call with a block on an untyped receiver (concurrent-ruby's `synchronize { }` on a mixin's
// self): the block goes last, as an rbBlockArg each wrapper of a block-taking method of that name adapts (decision 243).
func (f *fctx) genDynBlockCall(n parser.Node, recv expr, name string, args []parser.Node, b *parser.BlockNode) expr {
	names := f.blockParamList(b.Parameters)
	if names.special() {
		f.errorf(n, "a block with a rest or destructuring parameter on an untyped receiver is not supported")
	}
	if f.c.dynBlockNames == nil {
		f.c.dynBlockNames = map[string]bool{}
	}
	f.c.dynBlockNames[name] = true
	ps := make([]Type, len(names))
	vals := make([]string, len(names))
	for i := range ps {
		ps[i] = TAny{}
		vals[i] = fmt.Sprintf("rbArgAt(xs_, %d)", i)
	}
	clo := f.genClosure(n, b, &BlockSig{Params: ps, Ret: TAny{}}, map[string]Type{})
	arg := fmt.Sprintf("rbBlockArg{fn: func(xs_ ...any) any { return (%s)(%s) }}", clo, strings.Join(vals, ", "))
	return f.genDynCall(n, recv, name, append(slices.Clone(args), &exprNode{e: expr{code: arg, typ: TAny{}}}))
}

// dynEnumNames are the Enumerable, Array and Hash methods that take a block and leave their receiver as it is: what
// dynEnumBlock answers on an untyped receiver from a Hash or Array copy of it.
var dynEnumNames = map[string]bool{
	"select": true, "filter": true, "reject": true, "map": true, "collect": true, "flat_map": true, "collect_concat": true,
	"filter_map": true, "find": true, "detect": true, "find_index": true, "find_all": true, "any?": true, "all?": true,
	"none?": true, "one?": true, "count": true, "sum": true, "min_by": true, "max_by": true, "sort_by": true, "group_by": true,
	"partition": true, "each_with_index": true, "each_with_object": true, "inject": true, "reduce": true, "to_h": true,
	"fetch": true, "uniq": true, "sort": true, "min": true, "max": true, "each_slice": true, "each_cons": true,
	"sum_by": true, "tally_by": true, "chunk_while": true, "slice_when": true, "zip": true, "each_key": true,
	"each_value": true, "each_pair": true, "transform_values": true, "transform_keys": true, "index_by": true,
}

// dynEnumBlock is a gem's `x.select { }` on an untyped x (decision 224): a Hash takes Hash[untyped, untyped]'s method,
// anything else Array[untyped]'s over its elements (an Array's own, else what its each yields, rbEnumAny). Each branch
// compiles on its own; one that cannot (a Hash-only name on the Array side) raises NoMethodError if taken.
func (f *fctx) dynEnumBlock(n parser.Node, recv expr, name string, args []parser.Node, block parser.Node) expr {
	f.c.dynEach = true
	hashT := TClass{C: f.c.classes["Hash"], Args: []Type{TAny{}, TAny{}}}
	arrT := TClass{C: f.c.classes["Array"], Args: []Type{TAny{}}}
	tmp, r := f.newTmp(), f.newTmp()
	f.emit("var %s any", tmp)
	f.emit("switch %s := rbUnbox(%s).(type) {", r, recv.code)
	branch := func(head string, on expr) {
		f.emit("%s", head)
		if call, ok := n.(*parser.CallNode); ok {
			if e := classOf(on.typ).lookup(name); e != nil && e.M.Iterator && e.M.Block != nil {
				if _, isBlock := call.Block.(*parser.BlockNode); isBlock && len(args) == 0 { // `each_key do ... end`: a range loop, valued its receiver as MRI's
					var err *compileError
					f.probe(func() { err = catchCompileError(func() { f.genIterLoop(call, e, on) }) })
					f.indent++
					if err != nil {
						f.emit("panic(rbNoMethod(%q, %s, false))", name, r)
					} else {
						f.genIterLoop(call, e, on)
						f.emit("%s = %s", tmp, r)
					}
					f.indent--
					return
				}
			}
		}
		var err *compileError
		f.probe(func() { err = catchCompileError(func() { f.genMethodCall(n, on, name, args, block) }) })
		f.indent++
		if err != nil {
			f.emit("panic(rbNoMethod(%q, %s, false))", name, r)
		} else if e := f.genMethodCall(n, on, name, args, block); isVoid(e.typ) || e.stmt {
			f.emitExprStmt(n, e)
		} else {
			f.emit("%s = %s", tmp, f.coerce(n, e, TAny{}))
		}
		f.indent--
	}
	branch("case Hash_Any:", expr{code: r + "._ToAny()", typ: hashT})
	if env := f.c.classes["ENVClass"]; env != nil && env.lookup("to_h") != nil { // ENV is Hash-like: its pairs, as a Hash (decision 229)
		branch("case *ENVClass:", expr{code: r + ".ToH()._ToAny()", typ: hashT})
	}
	branch("default:", expr{code: "rbElemsAny(" + r + ")", typ: arrT})
	f.emit("}")
	return expr{code: tmp, typ: TAny{}}
}

// universalCall handles Kernel-level methods on values of unknown type.
func (f *fctx) universalCall(n parser.Node, recv expr, name string, args []parser.Node, block parser.Node) expr {
	if block != nil {
		return f.universalBlockCall(n, recv, name, args, block)
	}
	one := func(exp Type) expr {
		if len(args) != 1 {
			f.errorf(n, "%s takes one argument", name)
		}
		return f.genExpr(args[0], exp)
	}
	switch name {
	case "to_s":
		return expr{code: "rbToS(" + recv.code + ")", typ: f.cls("String")}
	case "inspect":
		return expr{code: "rbInspect(" + recv.code + ")", typ: f.cls("String")}
	case "to_json":
		return expr{code: "rbToJson(" + strings.Join(append([]string{recv.code}, f.jsonArgs(args)...), ", ") + ")", typ: f.cls("String")}
	case "nil?":
		return expr{code: "Boolean(rbUnbox(any(" + recv.code + ")) == nil)", typ: f.cls("Boolean")} // T may be X?, a *X box
	case "!":
		return expr{code: "Boolean(!rbTruthy(" + recv.code + "))", typ: f.cls("Boolean")}
	case "==":
		a := one(recv.typ)
		code := "rbEq(" + recv.code + ", " + f.coerce(args[0], a, recv.typ) + ")"
		if isAny(recv.typ) || isNil(recv.typ) || isAbstract(recv.typ) || isAny(a.typ) { // `x == v` with v untyped: Enumerable#include?
			code = "rbEq[any](" + recv.code + ", " + f.coerce(args[0], a, TAny{}) + ")"
		}
		return expr{code: code, typ: f.cls("Boolean")}
	case "equal?":
		a := one(nil)
		return expr{code: "Boolean(rbIdentical(" + recv.code + ", " + f.coerce(args[0], a, TAny{}) + "))", typ: f.cls("Boolean")}
	case "<=>":
		if isAny(recv.typ) || isNil(recv.typ) || f.c.isNumericMod(classOf(recv.typ)) { // a Numeric's <=> is nil for an incomparable argument too
			break // DynOp_cmp: untyped, since MRI answers nil for incomparable values
		}
		a := one(recv.typ)
		return expr{code: "rbCmp(" + recv.code + ", " + f.coerce(args[0], a, recv.typ) + ")", typ: f.cls("Integer")}
	case "hash":
		return expr{code: "rbHash(" + recv.code + ")", typ: f.cls("Integer")}
	case "eql?":
		if c := classOf(recv.typ); c != nil && c.universal && !c.IsModule { // Object has no eql? to dispatch to: the value's own, else identity
			a := one(nil)
			return expr{code: "rbEql(" + recv.code + ", " + f.coerce(args[0], a, TAny{}) + ")", typ: f.cls("Boolean")}
		}
	}
	if isNil(recv.typ) && len(args) == 0 {
		if t, ok := map[string]Type{"to_a": TClass{C: f.c.classes["Array"], Args: []Type{TAny{}}}, "to_h": TClass{C: f.c.classes["Hash"], Args: []Type{TAny{}, TAny{}}}, "to_i": f.cls("Integer"), "to_f": f.cls("Float")}[name]; ok {
			f.discard(recv)
			return expr{code: nilConversions[name], typ: t}
		}
	}
	if recv.nilable {
		f.warn(n, "%s called on possibly-nil untyped (raises NoMethodError on nil)", name)
	}
	return f.genDynCall(n, recv, name, args)
}

// descendantResponds: a self call the class can't answer may still reach a subclass's method (or one it includes) at run time.
func descendantResponds(c *Class, name string) bool {
	for _, sub := range c.Subclasses {
		if sub.metaOf != nil && c.metaOf == nil {
			continue
		}
		if sub.lookup(name) != nil || descendantResponds(sub, name) {
			return true
		}
	}
	return false
}

// untypedIntrinsics are what universalCall answers inline, for any value.
var untypedIntrinsics = map[string]bool{"to_s": true, "inspect": true, "to_json": true, "nil?": true, "!": true, "==": true, "equal?": true, "<=>": true, "hash": true}

// nilConversions are NilClass's conversions, for nil held statically or untyped.
var nilConversions = map[string]string{"to_a": "(&Array[any]{})", "to_h": "NewHash[any, any]()", "to_i": "Integer(0)", "to_f": "Float(0)"}

func (f *fctx) dynClassOf(n parser.Node, recv expr) expr {
	f.c.classOf = true
	return expr{code: "rbClassOf(" + f.coerce(n, recv, TAny{}) + ")", typ: f.cls("Class")}
}

// callMissing sends an unknown method to the receiver's method_missing,
// with the name as a Symbol; Ruby calls it even when it is private.
func (f *fctx) callMissing(n parser.Node, mm *entry, recv expr, name string, args []parser.Node, block parser.Node) expr {
	if block != nil {
		f.errorf(n, "a block passed to %s (handled by method_missing) is not supported", name)
	}
	nodes := append([]parser.Node{&parser.SymbolNode{Unescaped: parser.RubyString{Value: name}}}, args...)
	f.implicitCall = true
	defer func() { f.implicitCall = false }()
	return f.callEntry(n, mm, recv, nodes, nil)
}

// rubyPrivate names methods Ruby makes private whoever defines them.
var rubyPrivate = map[string]bool{"initialize": true, "respond_to_missing?": true, "initialize_copy": true}

// genRespondTo decides `recv.respond_to?(:name, include_all)` for a typed
// receiver, a literal name and a literal include_all: true for a public
// method (any method with include_all), else respond_to_missing?, else
// false. Returns false when the answer depends on the runtime class.
func (f *fctx) genRespondTo(n parser.Node, recv expr, args []parser.Node) (expr, bool) {
	name := literalName(args[0])
	var includeAll parser.Node = &parser.FalseNode{}
	if len(args) > 1 {
		switch args[1].(type) {
		case *parser.TrueNode:
			includeAll = args[1]
		case *parser.FalseNode, *parser.NilNode:
		default:
			return expr{}, false
		}
	}
	_, priv := includeAll.(*parser.TrueNode)
	var cls *Class
	switch t := recv.typ.(type) {
	case TClass:
		cls = t.C
	case TVar:
		if t.Name == "Self" {
			cls = f.owner
		}
	case TAny, TFunc, TNil, TOpt, TTuple, TUnion, TVoid: // no class known here: answered at run time
	}
	if name == "" || cls == nil {
		return expr{}, false
	}
	if e := cls.lookup(name); e != nil && (priv || !e.M.Private && !rubyPrivate[name] || kernelModuleFunction(e.M, recv.typ)) {
		f.discard(recv)
		return expr{code: "Boolean(true)", typ: f.cls("Boolean")}, true
	}
	if cls.IsModule || cls.descendantDefines(name, priv) { // a module's value is some includer
		return expr{}, false
	}
	if rm := cls.lookup("respond_to_missing?"); rm != nil {
		f.implicitCall = true
		defer func() { f.implicitCall = false }()
		// Ruby hands respond_to_missing? a Symbol even for respond_to?("x").
		sym := &parser.SymbolNode{Unescaped: parser.RubyString{Value: name}, Location: args[0].GetLocation()}
		return f.callEntry(n, rm, recv, []parser.Node{sym, includeAll}, nil), true
	}
	f.discard(recv)
	return expr{code: "Boolean(false)", typ: f.cls("Boolean")}, true
}

// genDynCall sends a method to an untyped value: see prelude/dynamic.rb.
// Like MRI, only a call with an explicit receiver (other than self) cannot
// reach a private method; send (implicitCall) can.
func (f *fctx) genDynCall(n parser.Node, recv expr, name string, args []parser.Node) expr {
	if e, ok := f.dynContainerCall(n, recv, name, args); ok {
		return e
	}
	f.c.noteDyn(name)
	if (f.m == nil || !f.m.quietDynamic) && !f.patQuiet && !f.c.isNumericMod(classOf(recv.typ)) { // a Numeric's class is chosen at run time by design (decision 142)
		f.warn(n, "dynamic call: %s on %s", name, recv.typ)
	}
	how := "rbCall"
	call, _ := n.(*parser.CallNode)
	switch {
	case call != nil && call.IsVARIABLE_CALL():
		how = "rbVCall"
	case f.implicitCall || call != nil && (call.Receiver == nil || isSelf(call.Receiver)):
		how = "rbFCall"
	}
	if spread, ok := f.anySpread(args); ok { // `x.m(a, *rest, *more, **kw)` untyped: one slice into the wrapper's ...any (decision 227)
		return expr{code: "rbDyn" + goMethodName(name) + "(" + how + ", " + f.coerce(n, recv, TAny{}) + ", " + spread + "...)", typ: TAny{}}
	}
	argCodes := f.anyArgs(args)
	if i := slices.IndexFunc(argCodes, func(c string) bool { return strings.HasSuffix(c, "...") }); i >= 0 {
		// an argument already spread (a forwarded *rest) with more after it: one slice, spread once at the end
		spread := "append([]any{" + strings.Join(argCodes[:i], ", ") + "}, " + argCodes[i] + ")"
		if len(argCodes) > i+1 {
			spread = "append(" + spread + ", " + strings.Join(argCodes[i+1:], ", ") + ")"
		}
		return expr{code: "rbDyn" + goMethodName(name) + "(" + how + ", " + f.coerce(n, recv, TAny{}) + ", " + spread + "...)", typ: TAny{}}
	}
	codes := append([]string{how, f.coerce(n, recv, TAny{})}, argCodes...)
	return expr{code: "rbDyn" + goMethodName(name) + "(" + strings.Join(codes, ", ") + ")", typ: TAny{}}
}

// jsonArgs is anyArgs for to_json, plus its `to_json(*a)` idiom: a lone
// splat of an Array passes the elements on.
func (f *fctx) jsonArgs(args []parser.Node) []string {
	if len(args) != 1 {
		return f.anyArgs(args)
	}
	sp, ok := args[0].(*parser.SplatNode)
	if !ok {
		return f.anyArgs(args)
	}
	a := f.genExpr(sp.Expression, nil)
	ac, ok := a.typ.(TClass)
	if !ok || ac.C.Name != "Array" {
		f.errorf(sp, "splat of non-array %s", a.typ)
	}
	if !isAny(ac.Args[0]) {
		a.code += "._ToAny()"
	}
	return []string{"(" + a.code + ").s..."}
}

// dynContainerCall is an untyped `x.keys`, `x.values`, `x.to_a`: a Hash or Array method decision 207 gives no Dyn
// wrapper, since its result grows the class's type (Array[K]). On the [untyped] instantiation it grows nothing, so the
// receiver is switched on (Hash_Any, Array_Any) and the method called there; anything else goes to the dispatcher
// (decision 232).
func (f *fctx) dynContainerCall(n parser.Node, recv expr, name string, args []parser.Node) (expr, bool) {
	if f.inDynContainer {
		return expr{}, false
	}
	hash, arr := f.c.classes["Hash"], f.c.classes["Array"]
	grows := func(cls *Class) bool {
		e := cls.lookup(name)
		return e != nil && !e.M.generic() && e.M.Block == nil && growsTypeParams(cls, e)
	}
	onHash, onArr := grows(hash), grows(arr)
	if !onHash && !onArr || slices.ContainsFunc(args, func(a parser.Node) bool { _, sp := a.(*parser.SplatNode); return sp }) {
		return expr{}, false
	}
	f.inDynContainer = true // the [untyped] method may itself call name dynamically (an arithmetic mix): plain dispatch there
	defer func() { f.inDynContainer = false }()
	tmp, r := f.newTmp(), f.newTmp()
	f.emit("var %s any", tmp)
	f.emit("switch %s := rbUnbox(%s).(type) {", r, f.coerce(n, recv, TAny{}))
	branch := func(head string, on expr) {
		f.emit("%s", head)
		var err *compileError
		f.probe(func() { err = catchCompileError(func() { f.genMethodCall(n, on, name, args, nil) }) })
		f.indent++
		if err != nil {
			f.emit("panic(rbNoMethod(%q, %s, false))", name, r)
		} else {
			f.emit("%s = %s", tmp, f.coerce(n, f.genMethodCall(n, on, name, args, nil), TAny{}))
		}
		f.indent--
	}
	if onHash {
		branch("case Hash_Any:", expr{code: r + "._ToAny()", typ: TClass{C: hash, Args: []Type{TAny{}, TAny{}}}})
	}
	if onArr {
		branch("case Array_Any:", expr{code: r + "._ToAny()", typ: TClass{C: arr, Args: []Type{TAny{}}}})
	}
	f.emit("default:")
	f.indent++
	other := f.genDynCall(n, expr{code: r, typ: TAny{}}, name, args)
	f.emit("%s = %s", tmp, other.code)
	f.indent--
	f.emit("}")
	return expr{code: tmp, typ: TAny{}}, true
}

// anySpread is args as one []any when any of them is a splat: each splat's elements (an Array's own, else rbSplatAny's
// MRI to_a), the rest in place, keywords last as a Hash (rbDynKw takes them back). ok is false without a splat.
func (f *fctx) anySpread(args []parser.Node) (string, bool) {
	if !slices.ContainsFunc(args, func(a parser.Node) bool { _, ok := a.(*parser.SplatNode); return ok }) {
		return "", false
	}
	var parts, plain []string
	flush := func() {
		if len(plain) > 0 {
			parts = append(parts, "[]any{"+strings.Join(plain, ", ")+"}")
			plain = nil
		}
	}
	for _, a := range args {
		sp, ok := a.(*parser.SplatNode)
		if !ok {
			plain = append(plain, f.anyArgs([]parser.Node{a})...)
			continue
		}
		flush()
		e := f.genExpr(sp.Expression, nil)
		if ac, ok := e.typ.(TClass); ok && ac.C.RubyName == "Array" && len(ac.Args) == 1 {
			if isAny(ac.Args[0]) || f.c.goType(ac.Args[0]) == "any" { // a union element is already Array[any] in Go
				parts = append(parts, "("+e.code+").s")
			} else {
				parts = append(parts, "("+e.code+")._ToAny().s")
			}
			continue
		}
		parts = append(parts, "rbSplatAny("+f.coerce(sp, e, TAny{})+")")
	}
	flush()
	if len(parts) == 1 {
		return parts[0], true
	}
	return "rbArgsCat(" + strings.Join(parts, ", ") + ")", true
}

// anyArgs generates call arguments for an `...any` parameter.
func (f *fctx) anyArgs(args []parser.Node) []string {
	codes := make([]string, 0, len(args))
	for _, a := range args {
		if _, ok := a.(*parser.SplatNode); ok {
			f.errorf(a, "splat arguments in a dynamic call are not supported")
		}
		if kh, ok := a.(*parser.KeywordHashNode); ok { // keywords travel as a Hash; the wrapper's rbDynKw takes them back
			a = &parser.HashNode{Location: kh.Location, Elements: kh.Elements}
		}
		e := f.genExpr(a, nil)
		codes = append(codes, f.coerce(a, e, TAny{}))
	}
	return codes
}

// genSend compiles send/public_send/__send__. A literal name is an
// ordinary call (send may reach private methods); a computed one switches
// over every method name at run time.
func (f *fctx) genSend(n parser.Node, recv expr, name string, args []parser.Node, block parser.Node) expr {
	if lit := literalName(args[0]); lit != "" {
		if name != "public_send" {
			f.implicitCall = true
		}
		e := f.genMethodCall(n, recv, lit, args[1:], block)
		f.implicitCall = false
		return e
	}
	var blockArg string
	if block != nil {
		code, ok := f.dynBlockValue(n, block)
		if !ok || !f.lenient() {
			f.errorf(n, "a block with a computed send is not supported")
		}
		blockArg = code
		f.c.dynBlockAll = true // any block-taking method a computed name reaches (decision 243)
	}
	f.noteDynAll()
	if f.m == nil || !f.m.quietDynamic {
		f.warn(n, "dynamic call: %s with a computed name", name)
	}
	nameExpr := f.genExpr(args[0], nil)
	how := "rbFCall"
	if name == "public_send" {
		how = "rbCall"
	}
	codes := []string{f.coerce(n, recv, TAny{}), "rbConstName(" + f.coerce(args[0], nameExpr, TAny{}) + ")", how}
	if spread, ok := f.anySpread(args[1:]); ok { // `send(m, state, *args)`
		if blockArg != "" {
			spread = "append(" + spread + ", " + blockArg + ")"
		}
		return expr{code: "rbSendByName(" + strings.Join(codes, ", ") + ", " + spread + "...)", typ: TAny{}}
	}
	for _, a := range args[1:] {
		e := f.genExpr(a, nil)
		codes = append(codes, f.coerce(a, e, TAny{}))
	}
	if blockArg != "" {
		codes = append(codes, blockArg)
	}
	return expr{code: "rbSendByName(" + strings.Join(codes, ", ") + ")", typ: TAny{}}
}

// dynBlockValue is a dynamic call's block as an rbBlockArg (decision 243): a literal block's closure over untyped
// values, or a method's own `&block` passed on (nil when the caller gave none).
func (f *fctx) dynBlockValue(n parser.Node, block parser.Node) (string, bool) {
	switch b := block.(type) {
	case *parser.BlockNode:
		names := f.blockParamList(b.Parameters)
		if names.special() {
			return "", false
		}
		ps := make([]Type, len(names))
		vals := make([]string, len(names))
		for i := range ps {
			ps[i] = TAny{}
			vals[i] = fmt.Sprintf("rbArgAt(xs_, %d)", i)
		}
		clo := f.genClosure(n, b, &BlockSig{Params: ps, Ret: TAny{}}, map[string]Type{})
		return fmt.Sprintf("rbBlockArg{fn: func(xs_ ...any) any { return (%s)(%s) }}", clo, strings.Join(vals, ", ")), true
	case *parser.BlockArgumentNode:
		if b.Expression == nil || !f.isBlockParam(b.Expression) {
			return "", false
		}
		e := f.genExpr(b.Expression, nil)
		fn, opt := e.typ.(TFunc)
		if o, ok := e.typ.(TOpt); ok {
			fn, opt = o.Elem.(TFunc)
			if opt && fn.Rest && len(fn.Params) == 1 && isAny(fn.Params[0]) && isAny(fn.Ret) {
				return "rbBlockArgOf(" + e.code + ")", true
			}
			return "", false
		}
		if opt && fn.Rest && len(fn.Params) == 1 && isAny(fn.Params[0]) && isAny(fn.Ret) {
			return "rbBlockArg{fn: " + e.code + "}", true
		}
	}
	return "", false
}

// noteDynAll: a computed name may be any method; from a gem file, or a prelude forwarder (SimpleDelegator's
// respond_to_missing?, which only passes a name on), only names gem and user code answer (decision 235).
func (f *fctx) noteDynAll() {
	f.c.dynAll = true
	if !f.lenient() && (!f.f.prelude || !f.c.pruneGems) {
		f.c.dynAllStrict = true
	}
}

// universalNames answer respond_to? for every object.
var universalNames = map[string]bool{"to_s": true, "inspect": true, "==": true, "!=": true, "!": true,
	"nil?": true, "equal?": true, "class": true, "is_a?": true, "kind_of?": true, "respond_to?": true,
	"send": true, "public_send": true, "hash": true, "then": true, "frozen?": true, "to_json": true}

// genDynRespondTo answers respond_to? when the static type cannot.
func (f *fctx) genDynRespondTo(n parser.Node, recv expr, args []parser.Node) expr {
	r := f.coerce(n, recv, TAny{})
	includeAll := func() string {
		if len(args) < 2 {
			return "false"
		}
		return "rbTruthy(" + f.coerce(args[1], f.genExpr(args[1], nil), TAny{}) + ")"
	}
	if lit := literalName(args[0]); lit != "" {
		if universalNames[lit] {
			return expr{code: "Boolean(true)", typ: f.cls("Boolean")}
		}
		f.c.noteRespond(lit)
		return expr{code: "rbResponds" + goMethodName(lit) + "(" + r + ", " + includeAll() + ")", typ: f.cls("Boolean")}
	}
	f.noteDynAll()
	nameExpr := f.genExpr(args[0], nil)
	return expr{code: "rbRespondsByName(" + r + ", rbConstName(" + f.coerce(args[0], nameExpr, TAny{}) + "), " + includeAll() + ")", typ: f.cls("Boolean")}
}

// universalOnly is Object's (or Kernel's) name when no class of the program defines its own, so a call on any value
// reaches it; nil otherwise.
func (c *Compiler) universalOnly(name string) *entry {
	e := c.classes["Object"].lookup(name)
	if e == nil || !e.Owner.universal {
		return nil
	}
	for _, cls := range c.classList {
		if !cls.universal && cls.Methods[name] != nil {
			return nil
		}
	}
	return e
}

// dynSelfCall reports a self call in a gem method added to a core module, which dispatches dynamically (selfCalls).
func (f *fctx) dynSelfCall(name string) bool {
	if f.owner == nil || !f.owner.IsModule || f.m == nil || !f.c.gemFile(f.m.File) {
		return false
	}
	f.c.selfCalls(f.owner)
	return f.owner.gemSelfCalls[name]
}

// dynEachForward is a gem's `x.each(&block)` on an untyped x passing the method's own closure block on.
func (f *fctx) dynEachForward(n parser.Node, recv expr) expr {
	k := len(f.blockSig.Params)
	f.c.dynEach = true
	xs := make([]string, k)
	for i := range k {
		xs[i] = f.coerce(n, expr{code: fmt.Sprintf("xs_[%d]", i), typ: TAny{}}, f.blockSig.Params[i])
	}
	call := "blk(" + strings.Join(xs, ", ") + ")"
	if !isVoid(f.blockSig.Ret) {
		call = "_ = " + call
	}
	return expr{code: fmt.Sprintf("rbEachAny(any(%s), %d, func(xs_ ...any) { %s })", recv.code, k, call), typ: TAny{}}
}
