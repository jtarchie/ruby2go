package compiler

import (
	"cmp"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/danielgatis/go-ruby-prism/parser"
)

// fctx is the per-function code generation context.
type fctx struct {
	curBlock      parser.Node // the block of the call overload is resolving
	lambdaClosure int         // f.closures inside the innermost lambda body, whose `return` is its own
	retHint       Type        // the expected type of retHintNode's result, for type params its arguments leave open
	retHintNode   parser.Node
	c             *Compiler
	f             *File
	owner         *Class
	m             *Method
	implicitCall  bool     // calling method_missing/respond_to_missing? on the program\'s behalf
	lex           []*Class // lexical scope for constant lookup
	selfType      Type
	selfCode      string
	selfClassObj  bool // self is exactly a class constant (a class body)
	ret           Type
	iterator      bool
	blockSig      *BlockSig
	buf           *strings.Builder
	indent        int
	tmp           int
	pass          int // 0 = ivar discovery, 1 = local analysis, 2 = emit
	discover      bool
	locals        map[localKey]*localInfo
	refined       map[localKey]Type // `x = []`/`{}` typed by what is put in it (analyze)
	convs         map[int]bool      // pass 1's rbAs/OptOf conversion sites, by offset
	retTypes      *[]Type           // `return` values, while inferring the method's return type
	unset         map[localKey]bool // locals read where they may be unassigned (maybeUnset)
	scope         *scope
	block         string
	rbScope       string    // the block path where the current Ruby scope (method or block) starts
	rbFrames      []rbFrame // the Ruby blocks enclosing the current position, outermost first
	blockCtr      int
	loops         []*loopFrame
	switches      int  // nesting depth of emitted Go switch statements
	labels        int  // not rewound by probe, so labels stay unique
	closures      int  // nesting depth of Go closures (non-iterator blocks)
	nextTail      tail // the innermost closure's result, for `next`
	begins        int  // nesting depth of rescue wrappers
	retVar        string
	wrap          *wrapFrame // the innermost begin wrapper
	hasNamedRet   bool
	rescues       int // nesting depth of rescue clause bodies, where `raise` sets the new exception's cause (r_)
}

type loopKind int

const (
	loopFor loopKind = iota
	loopClosure
	loopIter
)

// loopFrame lets a `break` inside a Go switch (case/when) exit the loop via a label, not just the switch.
type loopFrame struct {
	kind     loopKind
	switches int    // f.switches when the loop was entered
	begins   int    // f.begins when the loop was entered
	start    int    // where the label goes
	label    string // set by the first `break` that needs it
}

// pushLoop goes just before the `for`; a label sits above its //line directive so the `for` keeps its Ruby line.
func (f *fctx) pushLoop(kind loopKind) {
	s := f.buf.String()
	start := len(s)
	if i := strings.LastIndexByte(strings.TrimSuffix(s, "\n"), '\n') + 1; strings.HasPrefix(s[i:], "//line ") {
		start = i
	}
	f.loops = append(f.loops, &loopFrame{kind: kind, switches: f.switches, begins: f.begins, start: start})
}

// jumpKind is how a jump left a begin wrapper's func literal. Go's
// return/break/continue stop at the func literal, so the wrapper records
// the jump in its flag and the jump is re-issued after the call.
type jumpKind int

const (
	jumpReturn jumpKind = iota + 1
	jumpBreak
	jumpNext
)

type wrapFrame struct {
	flag    string
	used    [jumpNext + 1]bool
	endsRet bool // the call is followed by the function's return
}

// leaveWrapper jumps out of the innermost begin wrapper.
func (f *fctx) leaveWrapper(k jumpKind) {
	if k == jumpReturn && f.wrap.endsRet {
		f.emit("return")
		return
	}
	f.wrap.used[k] = true
	f.emit("%s = %d", f.wrap.flag, k)
	f.emit("return")
}

// emitReturn leaves the method once its value (if any) is in place.
func (f *fctx) emitReturn() {
	if f.begins > 0 {
		f.leaveWrapper(jumpReturn)
		return
	}
	f.emit("return")
}

// popLoop inserts the label only once a break used it: Go rejects unused labels.
func (f *fctx) popLoop() {
	l := f.loops[len(f.loops)-1]
	f.loops = f.loops[:len(f.loops)-1]
	if l.label == "" {
		return
	}
	s := f.buf.String()
	f.buf.Reset()
	f.buf.WriteString(s[:l.start] + l.label + ":\n" + s[l.start:])
}

// localKey names one Ruby local: Ruby scopes are the method and each
// block, so the same name in two blocks (or in a block and the method) is
// two variables with their own type, hoisting and use counts.
type localKey struct {
	scope int // the block's source offset; methodScope for the method
	name  string
}

const methodScope = -1

// rbFrame is a Ruby block scope: the names prism puts in it (params,
// `|x; y|` block locals, locals first assigned inside) are its own.
type rbFrame struct {
	key    int
	locals []string
}

type localInfo struct {
	declBlock string
	declRuby  string // Ruby scope (method or block) the local belongs to
	declPass  int    // the generation pass that last saw its first assignment
	hoist     bool
	reads     int
	writes    int
	typ       Type
	annotated bool
	noHoist   bool      // params and block params: declared by Go syntax
	open      bool      // an unannotated `[]`/`{}` that later writes may type (analyze)
	elems     [2][]Type // what those writes put in it: elements, or keys and values
}

type scope struct {
	parent *scope
	vars   map[string]*local
}

type local struct {
	name     string
	goName   string
	typ      Type
	base     *local // non-nil for a narrowed view of another local
	view     string // see expr.view
	declared bool
	info     *localInfo // the Ruby local this binds; nil when not tracked
}

// owner is the Ruby local v binds, looking through narrowed views.
func (v *local) owner() *localInfo {
	if v.base != nil {
		return v.base.owner()
	}
	return v.info
}

func (s *scope) lookup(name string) *local {
	for sc := s; sc != nil; sc = sc.parent {
		if v := sc.vars[name]; v != nil {
			return v
		}
	}
	return nil
}

func (f *fctx) push() { f.scope = &scope{parent: f.scope, vars: map[string]*local{}} }
func (f *fctx) pop()  { f.scope = f.scope.parent }
func (f *fctx) errorf(n parser.Node, format string, args ...any) {
	f.c.errorf(f.f, n, format, args...)
}

// warn reports from the emitting pass only: earlier passes see provisional types.
func (f *fctx) warn(n parser.Node, format string, args ...any) {
	if f.pass == 2 {
		f.c.warn(f.f, n, format, args...)
	}
}

func (f *fctx) emit(format string, args ...any) {
	f.buf.WriteString(strings.Repeat("\t", f.indent))
	fmt.Fprintf(f.buf, format, args...)
	f.buf.WriteByte('\n')
}

// lineOf emits a //line directive (column 1) for node n.
func (f *fctx) lineOf(n parser.Node) {
	fmt.Fprintf(f.buf, "//line %s:%d\n", f.f.Name, f.f.line(n.GetLocation().StartOffset))
}

func (f *fctx) newTmp() string {
	f.tmp++
	return fmt.Sprintf("t%d_", f.tmp)
}

// enterBlock starts a new Go block scope (for local hoisting analysis).
func (f *fctx) enterBlock() string {
	saved := f.block
	f.blockCtr++
	f.block = fmt.Sprintf("%s.%d", f.block, f.blockCtr)
	f.push()
	return saved
}

// enterRubyBlock starts a Go block that is also a Ruby block (a closure or
// an iterator's loop body): locals first assigned inside stay inside.
func (f *fctx) enterRubyBlock(block parser.Node, params []string) (string, string) {
	saved := f.enterBlock()
	savedRuby := f.rbScope
	f.rbScope = f.block
	fr := rbFrame{key: block.GetLocation().StartOffset, locals: params}
	switch b := block.(type) {
	case *parser.BlockNode:
		fr.locals = append(fr.locals, b.Locals...)
	case *parser.LambdaNode:
		fr.locals = append(fr.locals, b.Locals...)
	}
	f.rbFrames = append(f.rbFrames, fr)
	return saved, savedRuby
}

func (f *fctx) leaveRubyBlock(saved, savedRuby string) {
	f.rbFrames = f.rbFrames[:len(f.rbFrames)-1]
	f.rbScope = savedRuby
	f.leaveBlock(saved)
}

// localKey resolves name as Ruby does: to the innermost enclosing block
// that owns it, else to the method.
func (f *fctx) localKey(name string) localKey {
	for i := len(f.rbFrames) - 1; i >= 0; i-- {
		if slices.Contains(f.rbFrames[i].locals, name) {
			return localKey{f.rbFrames[i].key, name}
		}
	}
	return localKey{methodScope, name}
}

func (f *fctx) localInfo(name string) *localInfo { return f.locals[f.localKey(name)] }

// visibleLocal is the Go binding of the Ruby local name in scope here,
// skipping a same-named local of an outer Ruby scope that a block param or
// a `|x; y|` block local shadows.
func (f *fctx) visibleLocal(name string) *local {
	v := f.scope.lookup(name)
	if v != nil && v.owner() != nil && v.owner() != f.localInfo(name) {
		return nil
	}
	return v
}

// sameScopeLocal finds a local assigned earlier in the same Ruby scope but
// inside another Go block (an if branch, a begin body read from ensure):
// Ruby locals are method- or block-scoped, not branch-scoped. It is
// hoisted to a var at the top of its Ruby scope (hoistLocals).
func (f *fctx) sameScopeLocal(name string) *local {
	info := f.localInfo(name)
	if info == nil || info.noHoist || info.typ == nil || info.declPass != f.pass || !isAncestorBlock(info.declRuby, f.rbScope) {
		return nil
	}
	info.hoist = true
	return &local{name: name, goName: goLocalName(name), typ: info.typ, declared: true, info: info}
}

// hoistLocals declares, at the top of a Ruby scope's Go body, the locals
// of that scope that sameScopeLocal found outside their Go block: a block's
// are fresh for each call, as in Ruby.
func (f *fctx) hoistLocals(scope int) {
	if f.pass != 2 {
		return
	}
	for _, li := range sortedLocals(f.locals) {
		if li.scope == scope && li.hoist && !li.noHoist && li.typ != nil && !isNil(li.typ) {
			f.emit("var %s %s", goLocalName(li.name), f.c.goType(li.typ))
		}
	}
}

func (f *fctx) leaveBlock(saved string) {
	f.pop()
	f.block = saved
}

func isAncestorBlock(decl, use string) bool {
	return use == decl || strings.HasPrefix(use, decl+".")
}

// probe runs gen without emitting, restoring the output/temp state after.
func (f *fctx) probe(fn func()) {
	savedBuf, savedTmp, savedCtr, savedIndent := f.buf, f.tmp, f.blockCtr, f.indent
	f.buf = &strings.Builder{}
	fn()
	f.buf, f.tmp, f.blockCtr, f.indent = savedBuf, savedTmp, savedCtr, savedIndent
}

// ---- tails

type tailKind int

const (
	tailNone tailKind = iota
	tailReturn
	tailAssign
)

type tail struct {
	kind   tailKind
	target string
	typ    Type    // nil: infer (probe)
	types  *[]Type // collector while inferring
}

func (t tail) record(ty Type) {
	if t.types != nil {
		*t.types = append(*t.types, ty)
	}
}

// applyTail finishes a value-producing statement in tail position.
func (f *fctx) applyTail(n parser.Node, e expr, t tail) {
	if e.noreturn {
		f.emit("%s", e.code)
		return
	}
	if _, void := e.typ.(TVoid); void && t.kind != tailNone && isAny(t.typ) {
		// A void call where an untyped value is wanted (`tap { work }`,
		// a Thread's block) runs as a statement and yields nil.
		f.emitExprStmt(n, e)
		e = expr{code: "nil", typ: TNil{}}
	}
	switch t.kind {
	case tailNone:
		f.emitExprStmt(n, e)
	case tailReturn:
		if t.typ == nil {
			t.record(e.typ)
			f.emit("_ = %s", e.code)
			return
		}
		if isVoid(t.typ) {
			f.emitExprStmt(n, e)
			return
		}
		if f.begins > 0 {
			f.emit("%s = %s", f.retVar, f.coerce(n, e, t.typ))
			return
		}
		f.emit("return %s", f.coerce(n, e, t.typ))
	case tailAssign:
		if t.typ == nil {
			t.record(e.typ)
			f.emit("_ = %s", e.code)
			return
		}
		f.emit("%s = %s", t.target, f.coerce(n, e, t.typ))
	}
}

func (f *fctx) emitExprStmt(n parser.Node, e expr) {
	if e.code == "" || e.done || e.lit {
		return
	}
	switch n.(type) {
	case *parser.NilNode, *parser.SelfNode, *parser.LocalVariableReadNode, *parser.InstanceVariableReadNode:
		return
	}
	if e.view != "" {
		e.code = e.view // the same call, without converting its result
	}
	switch n.(type) {
	case *parser.CallNode, *parser.YieldNode, *parser.SuperNode, *parser.ForwardingSuperNode,
		*parser.LocalVariableWriteNode, *parser.InstanceVariableWriteNode,
		*parser.LocalVariableOperatorWriteNode, *parser.InstanceVariableOperatorWriteNode,
		*parser.CallOperatorWriteNode, *parser.CallOrWriteNode, *parser.IndexOrWriteNode, *parser.IndexOperatorWriteNode:
		if e.stmt || strings.HasSuffix(e.code, ")") && !e.assert {
			f.emit("%s", e.code)
			return
		}
	}
	if e.stmt {
		f.emit("%s", e.code)
		return
	}
	f.emit("_ = %s", e.code)
}

// ---- statements

func (f *fctx) genStmts(n parser.Node, t tail) {
	stmts, ok := n.(*parser.StatementsNode)
	// An empty body arrives as a typed-nil *StatementsNode, which is != nil.
	if n == nil || ok && stmts == nil {
		f.emptyTail(nil, t)
		return
	}
	if !ok {
		f.genStmt(n, t)
		return
	}
	if len(stmts.Body) == 0 {
		f.emptyTail(n, t)
		return
	}
	for i, s := range stmts.Body {
		if i == len(stmts.Body)-1 {
			f.genStmt(s, t)
		} else {
			f.genStmt(s, tail{})
			f.applyNarrow(f.guardNarrowing(s))
		}
	}
}

// emptyTail handles an empty body in value position: its value is nil.
func (f *fctx) emptyTail(n parser.Node, t tail) {
	if t.kind == tailNone {
		return
	}
	if t.typ == nil {
		t.record(TNil{})
		return
	}
	if isVoid(t.typ) {
		return
	}
	f.applyTail(n, expr{code: "nil", typ: TNil{}}, t)
}

func (f *fctx) genStmt(n parser.Node, t tail) {
	if ci, ok := n.(*constInit); ok {
		f.genConstInit(ci.k)
		return
	}
	f.lineOf(n)
	switch n := n.(type) {
	case *parser.IfNode:
		f.genIf(n, n.Predicate, n.Statements, n.Subsequent, false, t)
	case *parser.UnlessNode:
		var els parser.Node
		if n.ElseClause != nil {
			els = n.ElseClause
		}
		f.genIf(n, n.Predicate, n.Statements, els, true, t)
	case *parser.CaseNode:
		f.genCase(n, t)
	case *parser.BeginNode:
		f.genBegin(n, t)
	case *parser.WhileNode:
		f.genWhile(n.Predicate, n.Statements, false, n.IsBEGIN_MODIFIER(), t)
	case *parser.UntilNode:
		f.genWhile(n.Predicate, n.Statements, true, n.IsBEGIN_MODIFIER(), t)
	case *parser.ReturnNode:
		f.genReturn(n)
	case *parser.BreakNode:
		f.genBreak(n)
	case *parser.NextNode:
		f.genNext(n)
	case *parser.ParenthesesNode:
		f.genStmts(n.Body, t)
	case *parser.CallNode:
		f.genCallStmt(n, t)
	default:
		e := f.genExpr(n, t.typ)
		f.applyTail(n, e, t)
	}
}

func (f *fctx) genIf(n parser.Node, pred parser.Node, then parser.Node, els parser.Node, negate bool, t tail) {
	cond, narrow := f.genCond(pred)
	if negate {
		cond = "!(" + cond + ")"
		narrow = nil
	}
	f.emit("if %s {", cond)
	saved := f.enterBlock()
	f.indent++
	f.applyNarrow(narrow)
	f.genStmts(then, t)
	f.indent--
	f.leaveBlock(saved)
	switch e := els.(type) {
	case nil:
		if t.kind != tailNone {
			f.emit("} else {")
			saved := f.enterBlock()
			f.indent++
			f.emptyTail(n, t)
			f.indent--
			f.leaveBlock(saved)
		}
		f.emit("}")
	case *parser.ElseNode:
		f.emit("} else {")
		saved := f.enterBlock()
		f.indent++
		f.genStmts(e.Statements, t)
		f.indent--
		f.leaveBlock(saved)
		f.emit("}")
	case *parser.IfNode:
		f.emit("} else {")
		saved := f.enterBlock()
		f.indent++
		f.genIf(e, e.Predicate, e.Statements, e.Subsequent, false, t)
		f.indent--
		f.leaveBlock(saved)
		f.emit("}")
	default:
		f.c.unsupported(f.f, els)
	}
}

type narrowInfo struct {
	local *local
	typ   Type
	code  string // Go expression for the narrowed view; "" means deref
	view  string // see expr.view
}

// applyNarrow shadows narrowed locals in the current scope.
func (f *fctx) applyNarrow(ns []narrowInfo) {
	for _, nw := range ns {
		base := nw.local
		if base.base != nil {
			base = base.base
		}
		code := nw.code
		if code == "" {
			code = "(*" + nw.local.goName + ")"
		}
		f.scope.vars[nw.local.name] = &local{name: nw.local.name, goName: code, typ: nw.typ, base: base, view: nw.view, declared: true}
	}
}

// unnarrow forgets every narrowed view of a local after it is reassigned:
// the new value may be nil or another class again.
func (f *fctx) unnarrow(name string) {
	for sc := f.scope; sc != nil; sc = sc.parent {
		if v := sc.vars[name]; v != nil && v.base != nil {
			delete(sc.vars, name)
		}
	}
}

// guardNarrowing is what an early-exit guard proves about the statements
// after it: `return x unless cond` narrows like `if cond`, and
// `return if x.nil?` narrows x to non-nil.
func (f *fctx) guardNarrowing(s parser.Node) []narrowInfo {
	var ns []narrowInfo
	switch s := s.(type) {
	case *parser.UnlessNode:
		if s.ElseClause == nil && terminates(s.Statements) {
			f.probe(func() { _, ns = f.genCond(s.Predicate) })
		}
	case *parser.IfNode:
		if s.Subsequent != nil || !terminates(s.Statements) {
			return nil
		}
		call, ok := s.Predicate.(*parser.CallNode)
		if !ok || call.Arguments != nil || (call.Name != "nil?" && call.Name != "!") {
			return nil
		}
		if lv, ok := call.Receiver.(*parser.LocalVariableReadNode); ok {
			if v := f.scope.lookup(lv.Name); v != nil && isOpt(v.typ) {
				ns = []narrowInfo{{local: v, typ: v.typ.(TOpt).Elem}}
			}
		}
	}
	return ns
}

// genCond renders a Ruby truthiness test as a Go bool expression.
func (f *fctx) genCond(n parser.Node) (string, []narrowInfo) {
	switch n := n.(type) {
	case *parser.ParenthesesNode:
		if st, ok := n.Body.(*parser.StatementsNode); ok && len(st.Body) == 1 {
			c, nw := f.genCond(st.Body[0])
			return "(" + c + ")", nw
		}
	case *parser.AndNode:
		l, nl := f.genCond(n.Left)
		var r string
		var nr []narrowInfo
		f.push()
		f.applyNarrow(nl)
		stmts := f.capture(func() { r, nr = f.genCond(n.Right) })
		f.pop()
		if stmts == "" {
			return l + " && " + r, append(nl, nr...)
		}
		// the right side needs statements: run them only if the left holds
		tmp := f.newTmp()
		f.emit("%s := false", tmp)
		f.emit("if %s {", l)
		f.buf.WriteString(stmts)
		f.emit("\t%s = %s", tmp, r)
		f.emit("}")
		return tmp, nl
	case *parser.OrNode:
		l, _ := f.genCond(n.Left)
		var r string
		stmts := f.capture(func() { r, _ = f.genCond(n.Right) })
		if stmts == "" {
			return l + " || " + r, nil
		}
		tmp := f.newTmp()
		f.emit("%s := true", tmp)
		f.emit("if !(%s) {", l)
		f.buf.WriteString(stmts)
		f.emit("\t%s = %s", tmp, r)
		f.emit("}")
		return tmp, nil
	case *parser.CallNode:
		if n.Name == "!" && n.Arguments == nil && n.Receiver != nil {
			c, _ := f.genCond(n.Receiver)
			return "!(" + c + ")", nil
		}
		if n.Name == "nil?" && n.Arguments == nil && n.Receiver != nil {
			e := f.genExpr(n.Receiver, nil)
			if isOpt(e.typ) || isAny(e.typ) {
				return e.code + " == nil", nil
			}
		}
	case *parser.LocalVariableReadNode:
		v := f.readLocal(n)
		if isOpt(v.typ) && !isAny(v.typ.(TOpt).Elem) {
			return optTruthy(v.goName, v.typ), []narrowInfo{{local: v, typ: v.typ.(TOpt).Elem}}
		}
	case *parser.LocalVariableWriteNode:
		// `if (x = h[k])` / `while (job = q.pop)`: assign, then test and narrow x like a read
		f.genStmt(n, tail{})
		return f.genCond(&parser.LocalVariableReadNode{Name: n.Name, Depth: n.Depth, Location: n.Location})
	}
	if call, ok := n.(*parser.CallNode); ok && isIsA(call) {
		if lv, ok := call.Receiver.(*parser.LocalVariableReadNode); ok {
			return f.narrowIsA(call, f.readLocal(lv))
		}
		if v := f.attrLocal(call.Receiver); v != nil {
			return f.narrowIsA(call, v)
		}
	}
	if v := f.attrLocal(n); v != nil && v.base == nil && isOpt(v.typ) && !isAny(v.typ.(TOpt).Elem) {
		return optTruthy(v.goName, v.typ), []narrowInfo{{local: v, typ: v.typ.(TOpt).Elem}}
	}
	e := f.genExpr(n, nil)
	return f.truthy(n, e), nil
}

// optTruthy tests a T? value: non-nil, and not false for a Boolean?.
func optTruthy(code string, t Type) string {
	if isClass(stripOpt(t), "Boolean") {
		return "rbTruthyOpt(" + code + ")"
	}
	return code + " != nil"
}

func (f *fctx) truthy(n parser.Node, e expr) string {
	switch {
	case isClass(e.typ, "Boolean"):
		// genCond's other results (rbIsA, `!= nil`, temps) are Go bool; mixing in Boolean fails go build
		return "bool(" + e.code + ")"
	case isOpt(e.typ):
		return optTruthy(e.code, e.typ)
	case isNil(e.typ):
		return "false"
	case isAny(e.typ):
		return "rbTruthy(" + e.code + ")"
	case isOpt(e.typ) && isAny(e.typ.(TOpt).Elem):
		return "rbTruthy(Opt(" + e.code + "))"
	}
	f.c.Warnings = append(f.c.Warnings, fmt.Sprintf("%s:%d: condition of type %s is always true", f.f.Name, f.f.line(n.GetLocation().StartOffset), e.typ))
	// value types (Integer, String) can't be compared to nil; keep the evaluation and the local's use
	f.emit("_ = %s", e.code)
	return "true"
}

func (f *fctx) genWhile(pred parser.Node, body *parser.StatementsNode, negate bool, doWhile bool, t tail) {
	if doWhile {
		f.errorf(pred, "begin/end while is not supported")
	}
	f.pushLoop(loopFor)
	saved := f.enterBlock()
	f.indent++
	var cond string
	var narrow []narrowInfo
	// the condition reruns each iteration, so its statements (`while (x = q.shift)`) go inside the loop
	stmts := f.capture(func() { cond, narrow = f.genCond(pred) })
	if negate {
		cond = "!(" + cond + ")"
		narrow = nil
	}
	f.indent--
	if stmts == "" {
		f.emit("for %s {", cond)
	} else {
		f.emit("for {")
		f.buf.WriteString(stmts)
		f.emit("\tif !(%s) {", cond)
		f.emit("\t\tbreak")
		f.emit("\t}")
	}
	f.indent++
	f.applyNarrow(narrow)
	f.genStmts(body, tail{})
	f.indent--
	f.leaveBlock(saved)
	f.emit("}")
	f.popLoop()
	f.emptyTail(pred, t)
}

func (f *fctx) genReturn(n *parser.ReturnNode) {
	if f.closures > 0 && f.closures == f.lambdaClosure {
		f.closureValue(n, n.Arguments) // a lambda's return leaves only the lambda
		return
	}
	if f.closures > 0 {
		f.errorf(n, "non-local return from a block is not supported (docs/design.md decision 4)")
	}
	var e expr
	hasVal := false
	if n.Arguments != nil {
		if len(n.Arguments.Arguments) != 1 {
			f.errorf(n, "return with multiple values is not supported")
		}
		e = f.genExpr(n.Arguments.Arguments[0], f.ret)
		hasVal = true
	}
	if f.retTypes != nil && f.closures == 0 {
		*f.retTypes = append(*f.retTypes, e.typ)
	}
	if f.iterator || isVoid(f.ret) || f.ret == nil {
		if hasVal {
			f.emitExprStmt(n.Arguments.Arguments[0], e)
		}
		f.emitReturn()
		return
	}
	if !hasVal {
		e = expr{code: "nil", typ: TNil{}}
	}
	if f.begins > 0 {
		f.emit("%s = %s", f.retVar, f.coerce(n, e, f.ret))
		f.leaveWrapper(jumpReturn)
		return
	}
	f.emit("return %s", f.coerce(n, e, f.ret))
}

func (f *fctx) genBreak(n *parser.BreakNode) {
	if n.Arguments != nil {
		f.errorf(n, "break with a value is not supported")
	}
	if len(f.loops) == 0 {
		f.errorf(n, "break outside a loop")
	}
	l := f.loops[len(f.loops)-1]
	if l.kind == loopClosure {
		f.errorf(n, "break inside a non-iterator block is not supported")
	}
	f.emitBreak(l)
}

func (f *fctx) emitBreak(l *loopFrame) {
	if f.begins > l.begins {
		f.leaveWrapper(jumpBreak)
		return
	}
	if f.switches > l.switches {
		if l.label == "" {
			f.labels++
			l.label = fmt.Sprintf("loop%d", f.labels)
		}
		f.emit("break %s", l.label)
		return
	}
	f.emit("break")
}

func (f *fctx) genNext(n *parser.NextNode) {
	if len(f.loops) == 0 {
		f.errorf(n, "next outside a loop")
	}
	l := f.loops[len(f.loops)-1]
	if n.Arguments != nil {
		if l.kind == loopClosure {
			f.closureValue(n, n.Arguments)
			return
		}
		f.errorf(n, "next with a value is not supported")
	}
	f.emitNext(n, l)
}

// closureValue ends a value-returning block (`next v`) or lambda (`return v`) with v.
func (f *fctx) closureValue(n parser.Node, args *parser.ArgumentsNode) {
	if args == nil {
		f.emitNext(n, f.loops[len(f.loops)-1])
		return
	}
	if len(args.Arguments) != 1 {
		f.errorf(n, "multiple values are not supported")
	}
	t := f.nextTail
	if t.kind != tailReturn {
		f.errorf(n, "a block's value is not used here")
	}
	f.applyTail(n, f.genExpr(args.Arguments[0], t.typ), t)
}

// emitNext leaves the innermost loop's iteration; n locates coerce errors.
func (f *fctx) emitNext(n parser.Node, l *loopFrame) {
	switch {
	case f.begins > l.begins:
		f.leaveWrapper(jumpNext)
	case l.kind == loopClosure:
		// the block's value is nil
		switch t := f.nextTail; {
		case t.kind == tailNone:
			f.emit("return")
		case t.typ == nil:
			t.record(TNil{})
			f.emit("return")
		case isClass(t.typ, "Boolean"): // nil is falsy
			f.emit("return false")
		default:
			f.emit("return %s", f.coerce(n, expr{code: "nil", typ: TNil{}}, t.typ))
		}
	default:
		f.emit("continue")
	}
}

// ---- case/when

func (f *fctx) genCase(n *parser.CaseNode, t tail) {
	if n.Predicate == nil {
		f.errorf(n, "case without a subject is not supported")
	}
	typeSwitch := true
	for _, w := range n.Conditions {
		wn := w.(*parser.WhenNode)
		for _, cond := range wn.Conditions {
			switch c := cond.(type) {
			case *parser.NilNode:
			case *parser.ConstantReadNode, *parser.ConstantPathNode:
				if cls := f.classRef(c); cls == nil || slices.Contains([]string{"TrueClass", "FalseClass", "NilClass", "Proc"}, cls.RubyName) {
					typeSwitch = false
				}
			default:
				typeSwitch = false
			}
		}
	}
	if typeSwitch {
		f.genTypeCase(n, t)
		return
	}
	subj := f.genExpr(n.Predicate, nil)
	tmp := f.newTmp()
	f.emit("%s := %s", tmp, f.materialize(subj))
	f.emit("switch {")
	f.switches++
	for _, w := range n.Conditions {
		wn := w.(*parser.WhenNode)
		var conds []string
		for _, cond := range wn.Conditions {
			conds = append(conds, f.caseEqq(cond, expr{code: tmp, typ: subj.typ}))
		}
		f.emit("case %s:", strings.Join(conds, " || "))
		saved := f.enterBlock()
		f.indent++
		f.genStmts(wn.Statements, t)
		f.indent--
		f.leaveBlock(saved)
	}
	f.emit("default:")
	saved := f.enterBlock()
	f.indent++
	if n.ElseClause != nil {
		f.genStmts(n.ElseClause.Statements, t)
	} else {
		f.emptyTail(n, t)
	}
	f.indent--
	f.leaveBlock(saved)
	f.switches--
	f.emit("}")
}

// caseEqq renders a `when`'s `cond === subj` as a Go bool: is_a? for a
// class, the condition's own === when its class defines one (at run time
// when it is untyped or T?), else ==, Object#==='s default.
func (f *fctx) caseEqq(cond parser.Node, subj expr) string {
	if cls := f.classRef(cond); cls != nil {
		// not isACheck: its discard of a folded subject would land in the previous arm's body
		return f.isA(cond, subj, cls)
	}
	var condT Type
	f.probe(func() { condT = f.genExpr(cond, subj.typ).typ })
	c := classOf(stripOpt(condT))
	if !isAny(condT) && (c == nil || c.lookup("===") == nil) {
		return "bool(" + f.genMethodCall(cond, subj, "==", []parser.Node{cond}, nil).code + ")"
	}
	e := f.genExpr(cond, nil)
	arg := []parser.Node{&exprNode{e: subj}}
	if isAny(condT) || isOpt(condT) {
		e = expr{code: f.coerce(cond, e, TAny{}), typ: TAny{}}
		return "rbTruthy(" + f.genDynCall(cond, e, "===", arg).code + ")"
	}
	return "bool(" + f.genMethodCall(cond, e, "===", arg, nil).code + ")"
}

func (f *fctx) genTypeCase(n *parser.CaseNode, t tail) {
	subj := f.genExpr(n.Predicate, nil)
	var subjLocal *local
	if lv, ok := n.Predicate.(*parser.LocalVariableReadNode); ok {
		subjLocal = f.scope.lookup(lv.Name)
	}
	code := f.coerce(n.Predicate, subj, TAny{})
	// a Go type switch needs an interface operand: box a @go_type value (Integer, String)
	if cls, ok := subj.typ.(TClass); !isOpt(subj.typ) && f.c.goType(subj.typ) != "any" && (!ok || !cls.C.isStruct()) {
		code = "any(" + code + ")"
	}
	// a fresh name, not the local's: default and multi-class arms still see the local as declared
	name := f.newTmp()
	f.emit("switch %s := %s.(type) {", name, code)
	f.switches++
	hasNil := false
	for _, w := range n.Conditions {
		wn := w.(*parser.WhenNode)
		var cases []string
		var armType Type = TAny{}
		convert := ""
		for _, cond := range wn.Conditions {
			switch c := cond.(type) {
			case *parser.NilNode:
				cases = append(cases, "nil")
				armType = TNil{}
				hasNil = true
			case *parser.ConstantReadNode, *parser.ConstantPathNode:
				cls := f.classRef(c)
				if cls.IsModule && f.moduleIsA(c, subj.typ, cls) == "false" {
					continue
				}
				var goTypes []string
				goTypes, armType, convert = f.whenClass(cls)
				cases = append(cases, goTypes...)
			}
		}
		if len(cases) == 0 {
			continue // only modules the subject statically lacks: never matches
		}
		if len(wn.Conditions) != 1 {
			convert = ""
		}
		slices.Sort(cases) // Go rejects a type listed twice, e.g. `when nil, Object`
		f.emit("case %s:", strings.Join(slices.Compact(cases), ", "))
		saved := f.enterBlock()
		f.indent++
		armName, view := name, ""
		if convert != "" {
			armName, view = name+convert, name // converted where read, like is_a? narrowing
		}
		if subjLocal != nil && len(wn.Conditions) == 1 {
			f.applyNarrow([]narrowInfo{{local: subjLocal, typ: armType, code: armName, view: view}})
		}
		f.genStmts(wn.Statements, t)
		f.indent--
		f.leaveBlock(saved)
	}
	f.emit("default:")
	saved := f.enterBlock()
	f.indent++
	f.emit("_ = %s", name)
	if o, ok := subj.typ.(TOpt); ok && hasNil && subjLocal != nil && !isAny(o.Elem) {
		f.applyNarrow([]narrowInfo{{local: subjLocal, typ: o.Elem}})
	}
	if n.ElseClause != nil {
		f.genStmts(n.ElseClause.Statements, t)
	} else {
		f.emptyTail(n, t)
	}
	f.indent--
	f.leaveBlock(saved)
	f.switches--
	f.emit("}")
}

// whenClass is the type-switch case for `when cls`: the Go types it lists,
// the arm's type, and the conversion that gives the arm that type.
func (f *fctx) whenClass(cls *Class) ([]string, Type, string) {
	switch {
	case len(cls.TypeParams) > 0:
		args := make([]Type, len(cls.TypeParams))
		for i := range args {
			args[i] = TAny{}
		}
		return []string{cls.Name + "_Any"}, TClass{C: cls, Args: args}, "._ToAny()"
	case cls.universal: // nil is an Object too, but a nil interface misses `case any:`
		return []string{f.c.goType(TClass{C: cls}), "nil"}, TClass{C: cls}, ""
	}
	return []string{f.c.goType(TClass{C: cls})}, TClass{C: cls}, ""
}

// ---- begin/rescue/ensure

// containsJump reports a return, break or next anywhere under n.
func containsJump(n parser.Node) bool {
	switch n.(type) {
	case nil:
		return false
	case *parser.ReturnNode, *parser.BreakNode, *parser.NextNode:
		return true
	case *parser.DefNode:
		return false
	}
	for _, ch := range n.CompactChildNodes() {
		if containsJump(ch) {
			return true
		}
	}
	return false
}

func containsRescue(n parser.Node) bool {
	if n == nil {
		return false
	}
	switch b := n.(type) {
	case *parser.BeginNode:
		if b.RescueClause != nil || b.EnsureClause != nil {
			return true
		}
	case *parser.RescueModifierNode:
		return true
	case *parser.DefNode:
		return false
	}
	for _, ch := range n.CompactChildNodes() {
		if containsRescue(ch) {
			return true
		}
	}
	return false
}

// containsRescueClause is containsRescue without ensure-only begins.
// containsSuper reports whether a method body calls super (nested defs
// are other methods).
func containsSuper(n parser.Node) bool {
	switch n.(type) {
	case nil, *parser.DefNode:
		return false
	case *parser.SuperNode, *parser.ForwardingSuperNode:
		return true
	}
	for _, ch := range n.CompactChildNodes() {
		if containsSuper(ch) {
			return true
		}
	}
	return false
}

func containsRescueClause(n parser.Node) bool {
	if n == nil {
		return false
	}
	switch b := n.(type) {
	case *parser.BeginNode:
		if b.RescueClause != nil {
			return true
		}
	case *parser.RescueModifierNode:
		return true
	case *parser.DefNode:
		return false
	}
	for _, ch := range n.CompactChildNodes() {
		if containsRescueClause(ch) {
			return true
		}
	}
	return false
}

func (f *fctx) genBegin(n *parser.BeginNode, t tail) {
	if n.RescueClause == nil && n.EnsureClause == nil {
		f.genStmts(n.Statements, t)
		return
	}
	if n.ElseClause != nil {
		f.errorf(n, "begin/else is not supported")
	}
	// Where does the value go?
	inner := t
	if t.kind == tailReturn && t.typ != nil && !isVoid(t.typ) {
		inner = tail{kind: tailAssign, target: f.retVar, typ: t.typ}
	}
	if t.kind == tailAssign && t.typ == nil || t.kind == tailReturn && t.typ == nil {
		// probing: just collect types
		inner = t
	}
	if n.EnsureClause != nil && terminates(n.EnsureClause.Statements) {
		// an ensure ending in `return` decides the value; the body's and
		// rescue's values are discarded, as in Ruby
		inner = tail{}
	}
	w := &wrapFrame{flag: f.newTmp(), endsRet: t.kind == tailReturn && t.typ != nil && f.begins == 0}
	call := f.capture(func() { f.genWrapper(n, inner, w) })
	if slices.Contains(w.used[:], true) {
		f.emit("%s := 0", w.flag)
	}
	f.buf.WriteString(call)
	for k, used := range w.used {
		if used {
			f.emit("if %s == %d {", w.flag, k)
			f.indent++
			f.reissue(n, jumpKind(k))
			f.indent--
			f.emit("}")
		}
	}
	if w.endsRet {
		f.emit("return") // the value is already in the named result
	}
}

// reissue repeats, after a begin wrapper's call, the jump that left it.
func (f *fctx) reissue(n parser.Node, k jumpKind) {
	switch k {
	case jumpReturn:
		f.emitReturn()
	case jumpBreak:
		f.emitBreak(f.loops[len(f.loops)-1])
	case jumpNext:
		f.emitNext(n, f.loops[len(f.loops)-1])
	}
}

// genWrapper emits a begin/rescue/ensure as a called func literal.
func (f *fctx) genWrapper(n *parser.BeginNode, inner tail, w *wrapFrame) {
	f.emit("rbBegin(func() {") // through rbBegin, so a backtrace knows this literal is no block (decision 106)
	saved := f.enterBlock()
	f.indent++
	f.begins++
	savedWrap := f.wrap
	f.wrap = w
	// Generate in Ruby's order (body, rescue, ensure) so locals assigned in
	// the body are known to rescue and ensure; emit in Go's order, since
	// the defers must be registered before the body runs.
	body := f.capture(func() {
		// its own block, so the rescue/ensure closures (emitted before it)
		// see body locals as foreign and hoist them
		saved := f.enterBlock()
		f.genStmts(n.Statements, inner)
		f.leaveBlock(saved)
	})
	rescue := f.capture(func() {
		if n.RescueClause != nil {
			f.emit("defer func() {")
			saved := f.enterBlock()
			f.indent++
			f.emit("if r_ := recover(); r_ != nil {")
			f.indent++
			for rc := n.RescueClause; rc != nil; rc = rc.Subsequent {
				if rc.Reference != nil { // `=> e` may ask for e.backtrace: record the frames now (decision 106)
					f.emit("r_ = rbCaptureBacktrace(r_)")
					break
				}
			}
			f.emit("r_ = rbWrapPanic(r_)")
			for rc := n.RescueClause; rc != nil; rc = rc.Subsequent {
				f.genRescueClause(rc, inner)
			}
			f.emit("panic(r_)")
			f.indent--
			f.emit("}")
			f.indent--
			f.leaveBlock(saved)
			f.emit("}()")
		}
	})
	ensure := f.capture(func() {
		if n.EnsureClause != nil {
			f.emit("defer func() {")
			saved := f.enterBlock()
			f.indent++
			// A jump in ensure discards a pending exception, as in Ruby:
			// hold it, and re-raise only if the ensure body falls through.
			pending := ""
			switch {
			case terminates(n.EnsureClause.Statements):
				f.emit("_ = recover()") // it always jumps: the exception is dropped
			case containsJump(n.EnsureClause):
				pending = f.newTmp()
				f.emit("%s := recover()", pending)
			}
			f.genStmts(n.EnsureClause.Statements, tail{})
			if pending != "" {
				f.emit("if %s != nil {", pending)
				f.emit("\tpanic(%s)", pending)
				f.emit("}")
			}
			f.indent--
			f.leaveBlock(saved)
			f.emit("}()")
		}
	})
	f.buf.WriteString(ensure)
	f.buf.WriteString(rescue)
	f.buf.WriteString(body)
	f.begins--
	f.wrap = savedWrap
	f.indent--
	f.leaveBlock(saved)
	f.emit("})")
}

func (f *fctx) genRescueClause(rc *parser.RescueNode, t tail) {
	var classes []*Class
	if len(rc.Exceptions) == 0 {
		classes = append(classes, f.c.classes["StandardError"])
	}
	for _, ex := range rc.Exceptions {
		cls := f.classRef(ex)
		if cls == nil {
			f.errorf(ex, "rescue needs exception class names")
		}
		classes = append(classes, cls)
	}
	conds := make([]string, 0, len(classes))
	for _, cls := range classes {
		conds = append(conds, fmt.Sprintf("rbIsA[%s](r_)", f.c.goType(TClass{C: cls})))
	}
	f.emit("if %s {", strings.Join(conds, " || "))
	saved := f.enterBlock()
	f.indent++
	if rc.Reference != nil {
		lt, ok := rc.Reference.(*parser.LocalVariableTargetNode)
		if !ok {
			f.errorf(rc.Reference, "unsupported rescue target")
		}
		bind := classes[0]
		if len(classes) > 1 {
			bind = f.c.classes["Exception"]
		}
		f.bindRescue(rc.Reference, lt.Name, TClass{C: bind})
	}
	f.rescues++
	f.genStmts(rc.Statements, t)
	f.rescues--
	if !terminates(rc.Statements) {
		f.emit("return")
	}
	f.indent--
	f.leaveBlock(saved)
	f.emit("}")
}

// bindRescue binds `rescue => name`. The name is a local of the enclosing
// method or block, nil when nothing was rescued: when it is read outside the
// clause, the clause assigns it and narrows it to non-nil; otherwise each
// clause gets a fresh Go binding of its own class. A local of that name
// assigned before keeps its one type, so the clause shadows it.
func (f *fctx) bindRescue(n parser.Node, name string, bind Type) {
	val := expr{code: "r_.(" + f.c.goType(bind) + ")", typ: bind}
	if outer := f.visibleLocal(name); outer != nil {
		v := &local{name: name, goName: goLocalName(name), typ: bind, declared: true, info: outer.owner()}
		f.scope.vars[name] = v
		f.emit("%s := %s", v.goName, val.code)
		f.emit("_ = %s", v.goName) // the reads counted are the outer local's too
		return
	}
	key := f.localKey(name)
	info := f.locals[key]
	if info == nil {
		info = &localInfo{typ: TOpt{Elem: bind}}
		f.locals[key] = info
	}
	if info.declPass != f.pass {
		info.declPass, info.declBlock, info.declRuby = f.pass, f.block, f.rbScope
	}
	if f.pass < 2 && !info.annotated {
		if j, ok := join(info.typ, TOpt{Elem: bind}); ok {
			info.typ = j
		}
	}
	if f.pass < 2 || !info.hoist {
		v := &local{name: name, goName: goLocalName(name), typ: bind, declared: true, info: info}
		f.scope.vars[name] = v
		f.emit("%s := %s", v.goName, val.code)
		f.noteUnused(v)
		return
	}
	outer := &local{name: name, goName: goLocalName(name), typ: info.typ, declared: true, info: info}
	f.emit("%s = %s", outer.goName, f.coerce(n, val, outer.typ))
	if isOpt(outer.typ) {
		f.applyNarrow([]narrowInfo{{local: outer, typ: stripOpt(outer.typ)}})
	}
}

// terminates reports whether a statement list ends in a jump, so no Go
// `return` may follow it (vet flags unreachable code).
func terminates(st *parser.StatementsNode) bool {
	if st == nil || len(st.Body) == 0 {
		return false
	}
	switch last := st.Body[len(st.Body)-1].(type) {
	case *parser.ReturnNode, *parser.BreakNode, *parser.NextNode:
		return true
	case *parser.CallNode:
		return last.Receiver == nil && (last.Name == "raise" || last.Name == "throw")
	}
	return false
}

// ---- locals

func (f *fctx) declareLocal(name string, typ Type) *local {
	key := f.localKey(name)
	info := f.locals[key]
	if info == nil {
		info = &localInfo{declBlock: f.block, declRuby: f.rbScope, typ: typ}
		f.locals[key] = info
	}
	if info.declPass != f.pass {
		info.declPass = f.pass
		info.declBlock, info.declRuby = f.block, f.rbScope
	}
	if info.noHoist {
		// block params are fresh per block; never share analysis
		info.declBlock = f.block
		info.typ = typ
		info.hoist = false
	} else if f.pass < 2 && info.typ != nil && !info.annotated {
		if j, ok := join(info.typ, typ); ok {
			info.typ = j
		}
	}
	goName := goLocalName(name)
	if f.pass == 2 && info.typ != nil {
		typ = info.typ
	}
	v := &local{name: name, goName: goName, typ: typ, info: info}
	f.scope.vars[name] = v
	return v
}

func (f *fctx) noteUnused(v *local) {
	info := v.owner()
	if f.pass == 2 && info != nil && info.reads == 0 && v.goName != "_" { // `|_, v|`: Go's blank needs no use
		f.emit("_ = %s", v.goName)
	}
}

func (f *fctx) readLocal(n *parser.LocalVariableReadNode) *local {
	v := f.visibleLocal(n.Name)
	if v == nil {
		v = f.sameScopeLocal(n.Name)
	}
	if v == nil && f.m != nil && n.Name == f.m.BlockParam {
		f.errorf(n, "the block parameter &%s can only be called (%s.call) or passed on (&%s)", n.Name, n.Name, n.Name)
	}
	if v == nil {
		f.errorf(n, "undefined local %s", n.Name)
	}
	info := f.localInfo(n.Name)
	if info != nil {
		info.reads++
		if !isAncestorBlock(info.declBlock, f.block) {
			info.hoist = true
		}
	}
	return v
}

// assignLocal emits `x := v` / `x = v` and returns the local.
func (f *fctx) assignLocal(n parser.Node, name string, val expr, annotated Type) expr {
	existing := f.visibleLocal(name)
	if existing == nil {
		existing = f.sameScopeLocal(name)
	}
	info := f.localInfo(name)
	var typ Type
	switch {
	case annotated != nil:
		typ = annotated
	case existing != nil && existing.base == nil:
		typ = existing.typ
	default:
		typ = val.typ
		if isNil(typ) && info != nil && info.typ != nil {
			typ = info.typ
		}
	}
	narrowed := existing != nil && existing.base != nil
	if narrowed {
		existing = existing.base
	}
	if isNil(typ) && f.pass == 2 {
		f.errorf(n, "cannot infer the type of %s from nil; add `#: T?`", name)
	}
	if existing == nil {
		return f.declareAssign(n, name, typ, val, annotated)
	}
	if info != nil {
		info.writes++
		if !isAncestorBlock(info.declBlock, f.block) {
			info.hoist = true
		}
		if f.pass < 2 && !info.annotated {
			if j, ok := join(info.typ, val.typ); ok {
				info.typ = j
			} else if !isVoid(val.typ) {
				f.errorf(n, "%s is assigned both %s and %s", name, info.typ, val.typ)
			}
		}
		if f.pass == 2 || isNil(val.typ) {
			existing.typ = info.typ // `s = "a"; s = nil` widens s to String? (decision 14)
		}
	}
	f.emit("%s = %s", existing.goName, f.coerce(n, val, existing.typ))
	if narrowed {
		f.unnarrow(name)
	}
	return f.narrowSet(existing, val)
}

// narrowSet: a maybe-unset local is T? only for the paths that skip its assignments, so after one it is non-nil.
func (f *fctx) narrowSet(v *local, val expr) expr {
	if !f.unset[f.localKey(v.name)] || !isOpt(v.typ) || isOpt(val.typ) || isVoid(val.typ) || isAny(val.typ) {
		return expr{code: v.goName, typ: v.typ, stmt: true, done: true}
	}
	nw := narrowInfo{local: v, typ: stripOpt(v.typ)}
	f.applyNarrow([]narrowInfo{nw})
	return expr{code: "(*" + v.goName + ")", typ: nw.typ, stmt: true, done: true}
}

// declareAssign emits the first assignment of a local.
func (f *fctx) declareAssign(n parser.Node, name string, typ Type, val expr, annotated Type) expr {
	prev := f.localInfo(name)
	hoisted := f.pass == 2 && prev != nil && prev.hoist
	v := f.declareLocal(name, typ)
	info := v.info
	switch {
	case annotated != nil:
		info.annotated = true
		info.typ = annotated
	case f.pass < 2:
		info.typ = typ
		if j, ok := join(info.typ, val.typ); ok {
			info.typ = j
		}
		if f.unset[f.localKey(name)] && !isVoid(info.typ) {
			info.typ = optOf(info.typ) // a path that skips this reads nil (decision 14)
		}
	}
	if f.pass == 2 {
		v.typ = info.typ
	}
	code := f.coerce(n, val, v.typ)
	if val.lit && typeEq(val.typ, v.typ) {
		code = f.c.goType(v.typ) + "(" + code + ")"
	}
	v.declared = true
	switch {
	case hoisted:
		f.emit("%s = %s", v.goName, code)
	case code == "nil":
		f.emit("var %s %s", v.goName, f.c.goType(v.typ))
	case f.concreteInit(val, code, v.typ), isAny(v.typ) && !isAny(val.typ):
		// concreteInit, or an untyped local seeded with a typed value: `:=`
		// would give the local the value's Go type; later writes need goType
		f.emit("var %s %s = %s", v.goName, f.c.goType(v.typ), code)
	default:
		f.emit("%s := %s", v.goName, code)
	}
	if f.pass < 2 {
		info.writes++
	}
	f.noteUnused(v)
	return f.narrowSet(v, val)
}

// concreteInit reports whether code, the coerced first value of a local of
// type t, has a Go type other than the interface goType(t): `:=` would then
// give the local the concrete type (*X, *X_Meta, Integer) and a later
// assignment of a subclass, another class object or another value fails.
func (f *fctx) concreteInit(val expr, code string, t Type) bool {
	gt := f.c.goType(t)
	if cls, ok := t.(TClass); gt != "any" && (!ok || !cls.C.isStruct()) {
		return false
	}
	if code != val.code && !val.lit { // coerce converted it to gt
		return false
	}
	return val.classObj || val.ctor || f.c.goType(val.typ) != gt
}

// ---- function bodies

// genBody runs the two-pass body generation into f.buf.
func (f *fctx) genBody(body parser.Node, params []*local, t tail, prologue func()) {
	final := f.buf
	f.unset = maybeUnset(body, params)
	fresh := func() {
		f.locals = map[localKey]*localInfo{}
		for _, p := range params {
			f.locals[localKey{methodScope, p.name}] = &localInfo{declBlock: "", typ: p.typ, annotated: true, reads: 1, noHoist: true}
		}
	}
	run := func(pass int) {
		f.pass = pass
		f.buf = &strings.Builder{}
		f.tmp, f.blockCtr, f.block = 0, 0, ""
		f.scope = &scope{vars: map[string]*local{}}
		for _, p := range params {
			f.scope.vars[p.name] = &local{name: p.name, goName: p.goName, typ: p.typ, declared: true, info: f.locals[localKey{methodScope, p.name}]}
		}
		if prologue != nil {
			prologue()
		}
		f.hoistLocals(methodScope)
		f.genStmts(body, t)
	}
	fresh()
	if f.discover {
		run(0)
	} else {
		f.analyze(fresh, run)
	}
	final.WriteString(f.buf.String())
	f.buf = final
}

// analyze runs the local-analysis pass and the emitting pass (decision 13).
// An unannotated `x = []`/`{}` is typed by the join of what later writes
// put in it (x << v, x.push(v), x[i] = v, h[k] = v), re-running pass 1
// until no more locals are typed: one typed literal can type the next.
// A typing that fails to compile or adds a conversion is dropped, and its
// literal stays untyped: an rbAs between instantiations copies, and the
// copy would lose Ruby's aliasing.
func (f *fctx) analyze(fresh func(), run func(int)) {
	nw := len(f.c.Warnings)
	saved := *f
	f.convs = map[int]bool{}
	orig := catchCompileError(func() { run(1) }) // `[].max` fails until the literal is typed
	before := f.convs
	refined := map[localKey]Type{}
	var order []localKey
	for range 8 {
		add := f.emptyRefinements()
		if len(add) == 0 {
			break
		}
		for _, k := range slices.SortedFunc(maps.Keys(add), func(a, b localKey) int { return strings.Compare(a.name, b.name) }) {
			order = append(order, k)
			refined[k] = add[k]
		}
		*f = saved
		f.refined, f.convs = refined, map[int]bool{}
		fresh()
		_ = catchCompileError(func() { run(1) }) // discovery only: try() decides
	}
	try := func(set map[localKey]Type) bool {
		f.c.dropWarnings(nw) // the untyped run's, e.g. dynamic calls on elements
		*f = saved
		f.refined, f.convs = set, map[int]bool{}
		fresh()
		return catchCompileError(func() { run(1) }) == nil && !f.newConv(before) &&
			catchCompileError(func() { run(2) }) == nil
	}
	if len(order) > 0 {
		if salvage(order, func(set map[localKey]bool) bool {
			sub := map[localKey]Type{}
			for k := range set {
				sub[k] = refined[k]
			}
			return len(sub) > 0 && try(sub)
		}) {
			return
		}
		f.c.dropWarnings(nw)
		*f = saved
		fresh()
		run(1)
	} else if orig != nil {
		panic(*orig)
	}
	run(2)
}

func (f *fctx) newConv(before map[int]bool) bool {
	for off := range f.convs {
		if !before[off] {
			return true
		}
	}
	return false
}

// catchCompileError runs fn, returning the compile error it raised, if any.
func catchCompileError(fn func()) (ce *compileError) {
	defer func() {
		if r := recover(); r != nil {
			e, ok := r.(compileError)
			if !ok {
				panic(r)
			}
			ce = &e
		}
	}()
	fn()
	return nil
}

// emptyRefinements types each open, still untyped local by the join of
// what pass 1 saw put in it. No evidence, or evidence that is or holds
// untyped, leaves it untyped.
func (f *fctx) emptyRefinements() map[localKey]Type {
	out := map[localKey]Type{}
	for k, li := range f.locals {
		if t := refineContainer(li.typ, li.open, li.elems); t != nil {
			out[k] = t
		}
	}
	return out
}

// refineContainer is the type an open `[]`/`{}` of type t gets from what
// was put in it, or nil: still typed, or no typed evidence to join.
// Untyped evidence is skipped (`h[k] = h[k] + 1` is untyped only because h
// is): if it really is untyped, the typed run converts it and is rejected.
func refineContainer(t Type, open bool, elems [2][]Type) Type {
	c, ok := stripOpt(t).(TClass)
	if !open || !ok || slices.ContainsFunc(c.Args, func(a Type) bool { return !isAny(a) }) {
		return nil
	}
	args := make([]Type, len(c.Args))
	for i := range args {
		typed := slices.DeleteFunc(slices.Clone(elems[i]), holdsAny)
		if args[i] = joinOrAny(typed); len(typed) == 0 || holdsAny(args[i]) {
			return nil
		}
	}
	return TClass{C: c.C, Args: args}
}

type namedInfo struct {
	localKey
	*localInfo
}

func sortedLocals(m map[localKey]*localInfo) []namedInfo {
	out := make([]namedInfo, 0, len(m))
	for k, v := range m {
		out = append(out, namedInfo{k, v})
	}
	// deterministic order
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j].name < out[j-1].name; j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out
}

// ---- method emission

func (c *Compiler) newFctx(f *File, owner *Class, m *Method) *fctx {
	fc := &fctx{c: c, f: f, owner: owner, m: m, buf: &strings.Builder{}}
	switch {
	case owner == nil:
		fc.selfType = TClass{C: c.classes["Object"]}
		fc.selfCode = "rb_main"
	case owner.GoType != "":
		fc.selfType = owner.instance()
		fc.selfCode = "self"
	default:
		fc.selfType = TVar{Name: "Self"}
		fc.selfCode = "self"
	}
	if m != nil && m.SelfType != nil {
		fc.selfType = m.SelfType
	}
	if m != nil {
		fc.ret = m.Ret
		fc.iterator = m.Iterator
		fc.blockSig = m.Block
		fc.lex = m.Scope
	}
	return fc
}

func (c *Compiler) paramLocals(m *Method) []*local {
	ps := make([]*local, 0, len(m.Params))
	for _, p := range m.Params {
		t := p.Type
		if p.Rest {
			t = TClass{C: c.classes["Array"], Args: []Type{p.Type}}
		}
		ps = append(ps, &local{name: p.Name, goName: goLocalName(p.Name), typ: t, declared: true})
	}
	return ps
}

func (c *Compiler) emitMethod(m *Method) {
	cls := m.Owner
	if m.Kind == kindSynth {
		c.emitSynth(m)
		return
	}
	if m.Kind == kindAttrReader {
		iv := c.findIvar(cls, m.Attr)
		c.w("func (self *%s) %s() %s { return self.%s }\n\n", cls.Name, m.GoName, c.goType(iv.Type), goFieldName(iv.Name))
		return
	}
	if m.Kind == kindAttrWriter {
		iv := c.findIvar(cls, m.Attr)
		c.w("func (self *%s) %s(v %s) { self.%s = v }\n\n", cls.Name, m.GoName, c.goType(iv.Type), goFieldName(iv.Name))
		return
	}
	env := map[string]Type{}
	if cls.GoType != "" {
		env["Self"] = cls.instance()
	} else {
		env["Self"] = TVar{Name: "Self"}
	}
	params, ret := c.sig(m, env)
	namedRet := containsRescue(m.Node.Body) && ret != ""
	retDecl := ret
	if namedRet {
		retDecl = "(ret_ " + ret + ")"
	}
	c.lineDirective(m.File, m.Line)
	if c.isDirectMethod(m) {
		c.labels[cls.Name+"."+m.GoName] = c.frameLabel(m)
		c.w("func (self %s) %s(%s) %s {\n", c.recvType(cls), m.GoName, params, retDecl)
	} else {
		c.labels[freeFuncName(m)] = c.frameLabel(m)
		self := "self Self"
		switch {
		case m.SelfType != nil:
			self = "self " + c.goType(m.SelfType)
		case cls.GoType != "":
			self = "self " + c.recvType(cls)
		}
		c.w("func %s%s(%s%s) %s {\n", freeFuncName(m), c.typeParamDecl(m), self, comma(params), retDecl)
	}
	c.emitBody(m, namedRet)
	c.w("}\n\n")
}

func (c *Compiler) emitTopDef(m *Method) {
	params, ret := c.sig(m, nil)
	namedRet := containsRescue(m.Node.Body) && ret != ""
	retDecl := ret
	if namedRet {
		retDecl = "(ret_ " + ret + ")"
	}
	c.lineDirective(m.File, m.Line)
	c.labels[m.GoName] = "Object#" + m.Name
	c.w("func %s%s(%s) %s {\n", m.GoName, topTypeParamDecl(m), params, retDecl)
	c.emitBody(m, namedRet)
	c.w("}\n\n")
}

// frameLabel is m's name as MRI 4.0 labels its frames: 'K#m', 'K.s' for a
// singleton method. A prelude overload twin (`__integer_string`, decision
// 12) is labelled as the public method it stands in for.
func (c *Compiler) frameLabel(m *Method) string {
	owner, name := m.Owner, m.Name
	if m.File.prelude && strings.HasPrefix(name, "__") {
		for pub := range owner.Methods {
			if strings.HasPrefix(strings.ToLower(name), "__"+strings.ToLower(pub)+"_") {
				name = pub
				break
			}
		}
	}
	if owner.metaOf != nil {
		return owner.RubyName + "." + name
	}
	return owner.RubyName + "#" + name
}

// topTypeParamDecl is typeParamDecl for a top-level def, which has no Owner.
func topTypeParamDecl(m *Method) string {
	if len(m.TypeParams) == 0 {
		return ""
	}
	return "[" + strings.Join(m.TypeParams, ", ") + " comparable]"
}

func (c *Compiler) emitBody(m *Method, namedRet bool) {
	if m.Kind == kindPrimitive {
		x := m.Node.Body.(*parser.StatementsNode).Body[0].(*parser.XStringNode)
		body := strings.TrimSpace(x.Unescaped.Value)
		if !isVoid(m.Ret) && !m.Iterator && !strings.Contains(body, "\n") && !strings.HasPrefix(body, "return ") {
			body = "return " + body
		}
		c.lineDirective(m.File, m.File.line(x.Location.StartOffset))
		c.w("\t%s\n", strings.ReplaceAll(body, "\n", "\n\t"))
		return
	}
	f := c.newFctx(m.File, m.Owner, m)
	f.hasNamedRet = namedRet
	f.retVar = "ret_"
	params := c.paramLocals(m)
	prologue := func() {
		for _, p := range m.Params {
			if p.Rest {
				f.emit("%s := (*Array[%s])(&rest_)", goLocalName(p.Name), c.goType(p.Type))
				f.emit("_ = %s", goLocalName(p.Name)) // `*_args` may go unused
			}
		}
		for i, p := range m.Params {
			if m.calleeDefaults && p.Default != nil {
				f.fillDefault(i, p)
			}
		}
		if m.Iterator {
			ps := make([]string, len(m.Block.Params))
			for i, p := range m.Block.Params {
				ps[i] = c.goType(p)
			}
			f.emit("return func(yield func(%s) bool) {", strings.Join(ps, ", "))
			f.indent++
		}
	}
	t := tail{kind: tailReturn, typ: m.Ret}
	if m.Iterator {
		t = tail{}
	}
	f.indent = 1
	f.genBody(m.Node.Body, params, t, prologue)
	if m.Iterator {
		f.indent--
		f.emit("}")
	}
	c.out.WriteString(f.buf.String())
}

// fillDefault runs a left-out param's default where Ruby does: in the callee, after the params before it.
func (f *fctx) fillDefault(i int, p Param) {
	name := goLocalName(p.Name)
	if info := f.localInfo(p.Name); f.pass == 2 && info != nil && info.reads == 1 {
		name = "_" // never read: evaluated for its effects only
	}
	f.emit("if rbArgc <= %d {", i)
	f.indent++
	d := f.genExpr(p.Default, p.Type)
	f.emit("%s = %s", name, f.coerce(p.Default, d, p.Type))
	f.indent--
	f.emit("}")
}

func (c *Compiler) emitMain() {
	c.w("var rb_main = &Object{}\n\n")
	c.labels["main"] = "<main>"
	c.w("func main() {\n\trbTrapSignals()\n\tdefer rbFlush()\n\tdefer rbTopRecover()\n")
	prelude, bodies := c.mainBodies()
	gen := func(file *File, stmts []parser.Node, indent int) {
		f := c.newFctx(file, nil, nil)
		f.indent = indent
		f.retVar = ""
		f.genBody(&parser.StatementsNode{Body: stmts}, nil, tail{}, nil)
		c.out.WriteString(f.buf.String())
	}
	if len(bodies) == 1 { // one file: its top level is main's
		gen(bodies[0].f, append(prelude, bodies[0].stmts...), 1)
	} else {
		gen(c.mainFile, prelude, 1)
		for _, b := range bodies { // top-level locals are file-scoped in Ruby: a Go block each
			c.w("\t{\n")
			gen(b.f, b.stmts, 2)
			c.w("\t}\n")
		}
	}
	c.w("}\n\n")
}

// refineIvars types an ivar first assigned an unannotated `[]`/`{}` by
// what the class's methods put in it (decision 15), as analyze does for
// locals. A typing is dropped when a dry-run emission of the classes that
// see the ivar fails to compile or adds a conversion; the rest are kept,
// greedily in name order.
func (c *Compiler) refineIvars() {
	var cands []*Ivar
	typed, orig := map[*Ivar]Type{}, map[*Ivar]Type{}
	for range 8 { // one typed ivar can type the next: re-discover with it typed
		n := len(cands)
		for _, cls := range c.classList {
			for _, name := range slices.Sorted(maps.Keys(cls.Ivars)) {
				iv := cls.Ivars[name]
				if t := refineContainer(iv.Type, iv.open, iv.elems); t != nil && typed[iv] == nil {
					cands = append(cands, iv)
					typed[iv], orig[iv] = t, iv.Type
					iv.Type = t
				}
			}
		}
		if len(cands) == n {
			break
		}
		c.discoverIvars()
	}
	if len(cands) == 0 {
		return
	}
	for _, iv := range cands {
		iv.Type = orig[iv]
	}
	base, _ := c.dryRunIvarUsers(cands)
	try := func(set map[*Ivar]bool) bool {
		for _, iv := range cands {
			iv.Type = orig[iv]
			if set[iv] {
				iv.Type = typed[iv]
			}
		}
		convs, ok := c.dryRunIvarUsers(cands)
		for k := range convs {
			ok = ok && base[k]
		}
		return ok
	}
	salvage(cands, try) // a failed last try leaves the original types
}

// salvage finds a set of typings that try accepts: all of them, else all
// but one (the usual lone culprit; an untyped baseline may not compile,
// so adding one at a time can fail throughout), else those that pass when
// added one at a time. The last try's state is left applied; the result
// says whether it passed.
func salvage[K comparable](cands []K, try func(map[K]bool) bool) bool {
	set := map[K]bool{}
	for _, k := range cands {
		set[k] = true
	}
	if try(set) {
		return true
	}
	for _, k := range cands {
		delete(set, k)
		if try(set) {
			return true
		}
		set[k] = true
	}
	clear(set)
	for _, k := range cands {
		if set[k] = true; !try(set) {
			delete(set, k)
		}
	}
	return try(set)
}

// dryRunIvarUsers emits, and discards, every method of the classes that
// can see one of ivs, returning the conversion sites and whether it compiled.
func (c *Compiler) dryRunIvarUsers(ivs []*Ivar) (map[string]bool, bool) {
	saved, nw := c.out.String(), len(c.Warnings)
	for _, m := range c.inferredMethods() {
		m.Ret = nil // provisional: may depend on the ivar types being tried
	}
	c.convs = map[string]bool{}
	err := catchCompileError(func() {
		for _, cls := range c.classList {
			if !slices.ContainsFunc(ivs, func(iv *Ivar) bool { return cls.isSubclassOf(iv.Owner) }) {
				continue
			}
			for _, m := range cls.MethodList {
				if m.Kind == kindDef {
					c.inferRet(m)
					c.emitMethod(m)
				}
			}
		}
	})
	convs := c.convs
	c.convs = nil
	c.out.Reset()
	c.out.WriteString(saved)
	c.dropWarnings(nw)
	return convs, err == nil
}

// inferReturns re-infers every unannotated return type (decision 36): the
// ones inferred during discoverIvars saw ivars not yet typed.
func (c *Compiler) inferReturns() {
	for _, m := range c.inferredMethods() {
		m.Ret = nil
	}
	for _, m := range c.inferredMethods() {
		c.inferRet(m)
	}
}

func (c *Compiler) inferredMethods() []*Method {
	var out []*Method
	for _, m := range c.topDefList {
		if m.inferRet {
			out = append(out, m)
		}
	}
	for _, cls := range c.classList {
		for _, m := range cls.MethodList {
			if m.inferRet {
				out = append(out, m)
			}
		}
	}
	return out
}

// discoverIvars dry-runs struct class method bodies to learn ivar types
// from assignments.
func (c *Compiler) discoverIvars() {
	for range 2 {
		for _, cls := range c.classList {
			for _, iv := range cls.Ivars {
				iv.elems = [2][]Type{} // the last round's, when every ivar has a type
			}
		}
		for _, cls := range c.classList {
			if !cls.isStruct() || cls.universal {
				continue
			}
			ms := append([]*Method(nil), cls.MethodList...)
			for i, m := range ms {
				if m.Name == "initialize" && i > 0 {
					ms[0], ms[i] = ms[i], ms[0]
				}
			}
			for _, m := range ms {
				if m.Kind == kindDef {
					_ = catchCompileError(func() { c.discoverMethod(cls, m) }) // a dry run: emitMethod reports errors
				}
			}
		}
	}
}

func (c *Compiler) discoverMethod(cls *Class, m *Method) {
	f := c.newFctx(m.File, cls, m)
	f.discover = true
	f.retVar = "ret_"
	t := tail{kind: tailReturn, typ: m.Ret}
	if m.Iterator {
		t = tail{}
	}
	f.genBody(m.Node.Body, c.paramLocals(m), t, nil)
}

// emitSynth emits a metaclass's generated method as a Go method.
func (c *Compiler) emitSynth(m *Method) {
	meta := m.Owner
	cls := meta.metaOf
	params, ret := c.sig(m, map[string]Type{"Self": TClass{C: cls}})
	body := fmt.Sprintf("return String(%q)", cls.displayName())
	if m.Name == "new" {
		body = "return New" + cls.Name + "(" + c.argNames(m) + ")"
	}
	if m.Name == "new" {
		c.labels[meta.Name+"."+m.GoName] = "Class#new"
	}
	c.w("func (self *%s) %s(%s) %s { %s }\n\n", meta.Name, m.GoName, params, ret, body)
}

// constInit stands in main's statement list for a constant assignment, so
// constants are evaluated in source order like MRI (prelude ones first).
type constInit struct {
	parser.Node
	k *Const
}

func (ci *constInit) GetLocation() parser.Location     { return ci.k.Value.GetLocation() }
func (ci *constInit) CompactChildNodes() []parser.Node { return nil }
func (ci *constInit) ChildNodes() []parser.Node        { return nil }

// fileBody is what one user file's top level runs, in source order.
type fileBody struct {
	f     *File
	stmts []parser.Node
}

// mainBodies splits what main runs into the prelude's constant assignments
// and each user file's top level, in load order (decision 84): the file's
// statements, and what its class bodies run (constant assignments, hook
// calls), by position. A file runs whole before the next, as `require` does.
func (c *Compiler) mainBodies() ([]parser.Node, []fileBody) {
	var prelude []parser.Node
	own := map[*File][]parser.Node{}
	for _, k := range c.constList {
		if k.File.prelude {
			prelude = append(prelude, &constInit{k: k})
		} else {
			own[k.File] = append(own[k.File], &constInit{k: k})
		}
	}
	for _, h := range c.hooks {
		if n := c.hookCall(h); n != nil {
			own[h.file] = append(own[h.file], n)
		}
	}
	stmts := map[*File][]parser.Node{}
	for _, n := range c.mainStmts {
		stmts[c.stmtFile[n]] = append(stmts[c.stmtFile[n]], n)
	}
	bodies := make([]fileBody, 0, len(c.userFiles))
	for _, f := range c.userFiles {
		mine, rest := own[f], stmts[f]
		slices.SortStableFunc(mine, func(a, b parser.Node) int {
			return cmp.Compare(a.GetLocation().StartOffset, b.GetLocation().StartOffset)
		})
		var body []parser.Node
		for len(rest) > 0 || len(mine) > 0 {
			if len(mine) > 0 && (len(rest) == 0 || mine[0].GetLocation().StartOffset < rest[0].GetLocation().StartOffset) {
				body, mine = append(body, mine[0]), mine[1:]
				continue
			}
			body, rest = append(body, rest[0]), rest[1:]
		}
		bodies = append(bodies, fileBody{f: f, stmts: body})
	}
	return prelude, bodies
}

// mainBody is everything main runs, flattened.
func (c *Compiler) mainBody() []parser.Node {
	prelude, bodies := c.mainBodies()
	for _, b := range bodies {
		prelude = append(prelude, b.stmts...)
	}
	return prelude
}

// hookCall is the call MRI makes at hook site h, or nil when the receiver
// does not define the hook (Ruby's own are no-ops).
func (c *Compiler) hookCall(h classHook) parser.Node {
	recv := h.cls.Super
	if h.mod != nil {
		recv = c.resolveClassRef(h.mod)
	}
	if recv == nil || recv.meta == nil || recv.meta.lookup(h.name) == nil {
		return nil
	}
	obj := func(k *Class) parser.Node {
		return &exprNode{e: expr{code: classVar(k), typ: TClass{C: k.meta}, classObj: true}}
	}
	return &parser.CallNode{Location: h.node.GetLocation(), Receiver: obj(recv), Name: h.name, Arguments: &parser.ArgumentsNode{Arguments: []parser.Node{obj(h.cls)}}}
}

// genConstInit assigns a constant's package variable.
func (f *fctx) genConstInit(k *Const) {
	typ := f.c.constType(k)
	sub := f.c.constFctx(k)
	sub.indent = f.indent + 1
	e := sub.genExpr(k.Value, typ)
	code := sub.coerce(k.Value, e, typ)
	if e.lit && typeEq(e.typ, typ) { // coerce boxed or converted any other literal
		code = f.c.goType(typ) + "(" + code + ")"
	}
	fmt.Fprintf(f.buf, "//line %s:%d\n", k.File.Name, k.Line)
	if sub.buf.Len() == 0 {
		f.emit("%s = %s", k.GoName, code)
	} else {
		f.emit("%s = rbBeginV(func() %s {", k.GoName, f.c.goType(typ))
		f.buf.WriteString(sub.buf.String())
		f.emit("\treturn %s", code)
		f.emit("})")
	}
	if k.guarded {
		f.emit("%s = true", constSet(k))
	}
}

// guardConsts marks main.rb's constants that may be read before their
// assignment runs, where MRI raises NameError but a Go package variable
// reads as its zero value. A constant needs no guard when only other
// constant assignments precede it that call no method main.rb defines and
// read no constant not yet assigned, which is how programs usually start.
func (c *Compiler) guardConsts() {
	defs := map[string]bool{}
	for _, uf := range c.userFiles {
		anyNode(uf.Root, func(n parser.Node) bool {
			if d, ok := n.(*parser.DefNode); ok {
				defs[d.Name] = true
			}
			return false
		})
	}
	done := map[*Const]bool{}
	ran := false
	for _, n := range c.mainBody() {
		ci, ok := n.(*constInit)
		if !ok {
			ran = true
			continue
		}
		k := ci.k
		if !k.File.prelude && (ran || c.initRuns(k, defs, done)) {
			k.guarded, ran = true, true
		}
		done[k] = true
	}
}

// initRuns reports whether k's initializer may run main.rb's code or read
// a constant not yet assigned. Methods match by name, erring toward a guard.
func (c *Compiler) initRuns(k *Const, defs map[string]bool, done map[*Const]bool) bool {
	return anyNode(k.Value, func(n parser.Node) bool {
		switch n := n.(type) {
		case *parser.CallNode:
			return defs[n.Name] || n.Name == "new" && defs["initialize"]
		case *parser.EmbeddedStatementsNode:
			return defs["to_s"]
		case *parser.ConstantReadNode, *parser.ConstantPathNode:
			_, r := c.lookupConst(k.File, n, k.Scope)
			return r != nil && !done[r]
		}
		return false
	})
}

// anyNode reports whether pred holds for n or any node below it.
func anyNode(n parser.Node, pred func(parser.Node) bool) bool {
	if n == nil {
		return false
	}
	if pred(n) {
		return true
	}
	for _, ch := range n.CompactChildNodes() {
		if anyNode(ch, pred) {
			return true
		}
	}
	return false
}

// capture runs gen and returns the statements it emitted instead of
// emitting them, for code that must run conditionally.
func (f *fctx) capture(gen func()) string {
	saved := f.buf
	f.buf = &strings.Builder{}
	gen()
	out := f.buf.String()
	f.buf = saved
	return out
}

// symbolBlock desugars `&:name` to `{ |x_| x_.name }`.
func symbolBlock(ba *parser.BlockArgumentNode, name string) *parser.BlockNode {
	loc := ba.Location
	x := &parser.RequiredParameterNode{Location: loc, Name: "x_"}
	call := &parser.CallNode{Location: loc, Receiver: &parser.LocalVariableReadNode{Location: loc, Name: "x_"}, Name: name}
	return &parser.BlockNode{
		Location:   loc,
		Locals:     []string{"x_"},
		Parameters: &parser.BlockParametersNode{Location: loc, Parameters: &parser.ParametersNode{Location: loc, Requireds: []parser.Node{x}}},
		Body:       &parser.StatementsNode{Location: loc, Body: []parser.Node{call}},
	}
}

// genCallStmt is a call in statement position: an iterator with a block
// (literal, `&:name` or `&proc`) becomes a loop.
func (f *fctx) genCallStmt(n *parser.CallNode, t tail) {
	if ba, ok := n.Block.(*parser.BlockArgumentNode); ok && !f.isBlockParam(ba.Expression) {
		c := *n
		if sym, ok := ba.Expression.(*parser.SymbolNode); ok {
			c.Block = symbolBlock(ba, sym.Unescaped.Value) // `workers.each(&:join)`
		} else if pb := f.procBlock(ba); pb != nil {
			c.Block = pb // `xs.each(&printer)`
		}
		if c.Block != n.Block && f.genIterCall(&c, t) {
			return
		}
	}
	if n.Block != nil {
		ba, forwards := n.Block.(*parser.BlockArgumentNode)
		if _, ok := n.Block.(*parser.BlockNode); ok || (forwards && f.isBlockParam(ba.Expression)) {
			if f.genIterCall(n, t) {
				return
			}
		}
	}
	var e expr
	if t.kind == tailNone || t.kind == tailReturn && t.typ != nil && isVoid(t.typ) {
		e = f.genCall(n, t.typ) // value unused: a setter needs no temp for it
	} else {
		e = f.genExpr(n, t.typ)
	}
	f.applyTail(n, e, t)
}
