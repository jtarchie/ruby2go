package compiler

import (
	"fmt"
	"strings"

	"github.com/danielgatis/go-ruby-prism/parser"
)

// fctx is the per-function code generation context.
type fctx struct {
	c            *Compiler
	f            *File
	owner        *Class
	m            *Method
	implicitCall bool     // calling method_missing/respond_to_missing? on the program\'s behalf
	lex          []*Class // lexical scope for constant lookup
	selfType     Type
	selfCode     string
	ret          Type
	iterator     bool
	blockSig     *BlockSig
	buf          *strings.Builder
	indent       int
	tmp          int
	pass         int // 0 = ivar discovery, 1 = local analysis, 2 = emit
	discover     bool
	locals       map[string]*localInfo
	scope        *scope
	block        string
	rbScope      string // the block path where the current Ruby scope (method or block) starts
	blockCtr     int
	loops        []loopKind
	closures     int // nesting depth of Go closures (non-iterator blocks)
	begins       int // nesting depth of rescue wrappers
	retVar       string
	retFlag      string // set inside begin wrappers when a return must propagate
	hasNamedRet  bool
}

type loopKind int

const (
	loopFor loopKind = iota
	loopClosure
	loopIter
)

type localInfo struct {
	declBlock string
	declRuby  string // Ruby scope (method or block) the local belongs to
	declPass  int    // the generation pass that last saw its first assignment
	hoist     bool
	reads     int
	writes    int
	typ       Type
	annotated bool
	noHoist   bool // params and block params: declared by Go syntax
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
	declared bool
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
	return fmt.Sprintf("t%d", f.tmp)
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
func (f *fctx) enterRubyBlock() (string, string) {
	saved := f.enterBlock()
	savedRuby := f.rbScope
	f.rbScope = f.block
	return saved, savedRuby
}

func (f *fctx) leaveRubyBlock(saved, savedRuby string) {
	f.rbScope = savedRuby
	f.leaveBlock(saved)
}

// sameScopeLocal finds a local assigned earlier in the same Ruby scope but
// inside another Go block (an if branch, a begin body read from ensure):
// Ruby locals are method- or block-scoped, not branch-scoped. It is
// hoisted to a function-level var.
func (f *fctx) sameScopeLocal(name string) *local {
	info := f.locals[name]
	if info == nil || info.noHoist || info.typ == nil || info.declPass != f.pass || !isAncestorBlock(info.declRuby, f.rbScope) {
		return nil
	}
	info.hoist = true
	return &local{name: name, goName: goLocalName(name), typ: info.typ, declared: true}
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
	switch n.(type) {
	case *parser.CallNode, *parser.YieldNode, *parser.SuperNode, *parser.ForwardingSuperNode,
		*parser.LocalVariableWriteNode, *parser.InstanceVariableWriteNode,
		*parser.LocalVariableOperatorWriteNode, *parser.InstanceVariableOperatorWriteNode:
		if e.stmt || strings.HasSuffix(e.code, ")") {
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
	if n == nil {
		f.emptyTail(nil, t)
		return
	}
	stmts, ok := n.(*parser.StatementsNode)
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
		if n.Block != nil {
			ba, forwards := n.Block.(*parser.BlockArgumentNode)
			if _, ok := n.Block.(*parser.BlockNode); ok || (forwards && f.isBlockParam(ba.Expression)) {
				if f.genIterCall(n, t) {
					return
				}
			}
		}
		e := f.genExpr(n, t.typ)
		f.applyTail(n, e, t)
	case *parser.XStringNode:
		f.errorf(n, "%%x{} is only allowed as the whole body of a prelude method")
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
		f.scope.vars[nw.local.name] = &local{name: nw.local.name, goName: code, typ: nw.typ, base: base, declared: true}
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
			return v.goName + " != nil", []narrowInfo{{local: v, typ: v.typ.(TOpt).Elem}}
		}
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
		return v.goName + " != nil", []narrowInfo{{local: v, typ: v.typ.(TOpt).Elem}}
	}
	e := f.genExpr(n, nil)
	return f.truthy(n, e), nil
}

func (f *fctx) truthy(n parser.Node, e expr) string {
	switch {
	case isClass(e.typ, "Boolean"):
		return e.code
	case isOpt(e.typ):
		return e.code + " != nil"
	case isNil(e.typ):
		return "false"
	case isAny(e.typ):
		return "rbTruthy(" + e.code + ")"
	case isOpt(e.typ) && isAny(e.typ.(TOpt).Elem):
		return "rbTruthy(Opt(" + e.code + "))"
	}
	f.c.Warnings = append(f.c.Warnings, fmt.Sprintf("%s:%d: condition of type %s is always true", f.f.Name, f.f.line(n.GetLocation().StartOffset), e.typ))
	return "(" + e.code + " != nil || true)"
}

func (f *fctx) genWhile(pred parser.Node, body *parser.StatementsNode, negate bool, doWhile bool, t tail) {
	if doWhile {
		f.errorf(pred, "begin/end while is not supported")
	}
	cond, narrow := f.genCond(pred)
	if negate {
		cond = "!(" + cond + ")"
		narrow = nil
	}
	f.emit("for %s {", cond)
	saved := f.enterBlock()
	f.indent++
	f.loops = append(f.loops, loopFor)
	f.applyNarrow(narrow)
	f.genStmts(body, tail{})
	f.loops = f.loops[:len(f.loops)-1]
	f.indent--
	f.leaveBlock(saved)
	f.emit("}")
	f.emptyTail(pred, t)
}

func (f *fctx) genReturn(n *parser.ReturnNode) {
	if f.closures > 0 {
		f.errorf(n, "non-local return from a block is not supported (README open decision 4)")
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
	if f.iterator {
		if hasVal {
			f.emitExprStmt(n.Arguments.Arguments[0], e)
		}
		f.emit("return")
		return
	}
	if isVoid(f.ret) || f.ret == nil {
		if hasVal {
			f.emitExprStmt(n.Arguments.Arguments[0], e)
		}
		if f.begins > 0 && f.retFlag != "" {
			f.emit("%s = true", f.retFlag)
		}
		f.emit("return")
		return
	}
	if !hasVal {
		e = expr{code: "nil", typ: TNil{}}
	}
	if f.begins > 0 {
		f.emit("%s = %s", f.retVar, f.coerce(n, e, f.ret))
		if f.retFlag != "" {
			f.emit("%s = true", f.retFlag)
		}
		f.emit("return")
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
	if f.loops[len(f.loops)-1] == loopClosure {
		f.errorf(n, "break inside a non-iterator block is not supported")
	}
	f.emit("break")
}

func (f *fctx) genNext(n *parser.NextNode) {
	if len(f.loops) == 0 {
		f.errorf(n, "next outside a loop")
	}
	if f.loops[len(f.loops)-1] == loopClosure {
		if n.Arguments != nil {
			f.errorf(n, "next with a value inside a block is not supported")
		}
		f.emit("return")
		return
	}
	if n.Arguments != nil {
		f.errorf(n, "next with a value is not supported")
	}
	f.emit("continue")
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
				if f.classRef(c) == nil {
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
	f.emit("%s := %s", tmp, subj.code)
	f.emit("switch {")
	for _, w := range n.Conditions {
		wn := w.(*parser.WhenNode)
		var conds []string
		for _, cond := range wn.Conditions {
			var condT Type
			f.probe(func() { condT = f.genExpr(cond, subj.typ).typ })
			if isClass(condT, "Regexp") {
				// `when /re/` is Regexp#===, not ==.
				re := f.genExpr(cond, nil)
				conds = append(conds, "bool("+re.code+".Eqq("+f.coerce(cond, expr{code: tmp, typ: subj.typ}, TAny{})+"))")
				continue
			}
			eq := f.genMethodCall(cond, expr{code: tmp, typ: subj.typ}, "==", []parser.Node{cond}, nil)
			conds = append(conds, "bool("+eq.code+")")
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
	f.emit("}")
}

func (f *fctx) genTypeCase(n *parser.CaseNode, t tail) {
	subj := f.genExpr(n.Predicate, nil)
	var subjLocal *local
	if lv, ok := n.Predicate.(*parser.LocalVariableReadNode); ok {
		subjLocal = f.scope.lookup(lv.Name)
	}
	code := f.coerce(n.Predicate, subj, TAny{})
	name := f.newTmp()
	if subjLocal != nil {
		name = subjLocal.goName
		if subjLocal.base != nil || strings.HasPrefix(name, "(") {
			name = f.newTmp()
			subjLocal = nil
		}
	}
	f.emit("switch %s := %s.(type) {", name, code)
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
			case *parser.ConstantReadNode, *parser.ConstantPathNode:
				cls := f.classRef(c)
				if len(cls.TypeParams) > 0 {
					cases = append(cases, cls.Name+"_Any")
					args := make([]Type, len(cls.TypeParams))
					for i := range args {
						args[i] = TAny{}
					}
					armType = TClass{C: cls, Args: args}
					convert = "._ToAny()"
				} else {
					cases = append(cases, f.c.goType(TClass{C: cls}))
					armType = TClass{C: cls}
				}
			}
		}
		if len(wn.Conditions) != 1 {
			armType = TAny{}
			convert = ""
		}
		f.emit("case %s:", strings.Join(cases, ", "))
		saved := f.enterBlock()
		f.indent++
		armName := name
		if convert != "" {
			armName = f.newTmp()
			f.emit("%s := %s%s", armName, name, convert)
		}
		if subjLocal != nil {
			f.scope.vars[subjLocal.name] = &local{name: subjLocal.name, goName: armName, typ: armType, base: subjLocal, declared: true}
		} else if convert != "" {
			f.emit("_ = %s", armName)
		}
		f.genStmts(wn.Statements, t)
		f.indent--
		f.leaveBlock(saved)
	}
	f.emit("default:")
	saved := f.enterBlock()
	f.indent++
	f.emit("_ = %s", name)
	if n.ElseClause != nil {
		f.genStmts(n.ElseClause.Statements, t)
	} else {
		f.emptyTail(n, t)
	}
	f.indent--
	f.leaveBlock(saved)
	f.emit("}")
}

// ---- begin/rescue/ensure

func containsReturn(n parser.Node) bool {
	if n == nil {
		return false
	}
	if _, ok := n.(*parser.ReturnNode); ok {
		return true
	}
	if _, ok := n.(*parser.DefNode); ok {
		return false
	}
	for _, ch := range n.CompactChildNodes() {
		if containsReturn(ch) {
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
	needFlag := containsReturn(n.Statements) || (n.RescueClause != nil && containsReturn(n.RescueClause))
	flag := ""
	if needFlag && t.kind != tailReturn {
		flag = f.newTmp()
		f.emit("%s := false", flag)
	}
	f.emit("func() {")
	saved := f.enterBlock()
	f.indent++
	f.begins++
	savedFlag := f.retFlag
	f.retFlag = flag
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
			f.emit("if r := recover(); r != nil {")
			f.indent++
			f.emit("r = rbWrapPanic(r)")
			for rc := n.RescueClause; rc != nil; rc = rc.Subsequent {
				f.genRescueClause(rc, inner)
			}
			f.emit("panic(r)")
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
			// `return` in ensure discards a pending exception, as in Ruby:
			// hold it, and re-raise only if the ensure body falls through.
			pending := ""
			switch {
			case terminates(n.EnsureClause.Statements):
				f.emit("_ = recover()") // it always returns: the exception is dropped
			case containsReturn(n.EnsureClause):
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
	f.retFlag = savedFlag
	f.indent--
	f.leaveBlock(saved)
	f.emit("}()")
	if t.kind == tailReturn && t.typ != nil && f.begins == 0 {
		f.emit("return") // the value is already in the named result
	} else if flag != "" {
		f.emit("if %s {", flag)
		if f.begins > 0 && f.retFlag != "" {
			f.emit("\t%s = true", f.retFlag)
		}
		f.emit("\treturn")
		f.emit("}")
	}
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
		conds = append(conds, fmt.Sprintf("rbIsA[%s](r)", f.c.goType(TClass{C: cls})))
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
		// A fresh binding per clause: the same Ruby name may hold a
		// different exception class in each rescue.
		v := f.blockParam(lt.Name, TClass{C: bind})
		f.emit("%s := r.(%s)", v.goName, f.c.goType(TClass{C: bind}))
		f.noteUnused(v)
	}
	f.genStmts(rc.Statements, t)
	if !terminates(rc.Statements) {
		f.emit("return")
	}
	f.indent--
	f.leaveBlock(saved)
	f.emit("}")
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
		return last.Receiver == nil && last.Name == "raise"
	}
	return false
}

// ---- locals

func (f *fctx) declareLocal(name string, typ Type) *local {
	info := f.locals[name]
	if info == nil {
		info = &localInfo{declBlock: f.block, declRuby: f.rbScope, typ: typ}
		f.locals[name] = info
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
	v := &local{name: name, goName: goName, typ: typ}
	f.scope.vars[name] = v
	return v
}

func (f *fctx) noteUnused(v *local) {
	info := f.locals[v.name]
	if f.pass == 2 && info != nil && info.reads == 0 {
		f.emit("_ = %s", v.goName)
	}
}

func (f *fctx) readLocal(n *parser.LocalVariableReadNode) *local {
	v := f.scope.lookup(n.Name)
	if v == nil {
		v = f.sameScopeLocal(n.Name)
	}
	if v == nil && f.m != nil && n.Name == f.m.BlockParam {
		f.errorf(n, "the block parameter &%s can only be called (%s.call) or passed on (&%s)", n.Name, n.Name, n.Name)
	}
	if v == nil {
		f.errorf(n, "undefined local %s", n.Name)
	}
	info := f.locals[n.Name]
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
	existing := f.scope.lookup(name)
	if existing == nil {
		existing = f.sameScopeLocal(name)
	}
	info := f.locals[name]
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
	if existing != nil && existing.base != nil {
		existing = existing.base
		defer f.unnarrow(name)
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
		if f.pass == 2 {
			existing.typ = info.typ
		}
	}
	f.emit("%s = %s", existing.goName, f.coerce(n, val, existing.typ))
	return expr{code: existing.goName, typ: existing.typ, stmt: true, done: true}
}

// declareAssign emits the first assignment of a local.
func (f *fctx) declareAssign(n parser.Node, name string, typ Type, val expr, annotated Type) expr {
	hoisted := f.pass == 2 && f.locals[name] != nil && f.locals[name].hoist
	v := f.declareLocal(name, typ)
	info := f.locals[name]
	switch {
	case annotated != nil:
		info.annotated = true
		info.typ = annotated
	case f.pass < 2:
		info.typ = typ
		if j, ok := join(info.typ, val.typ); ok {
			info.typ = j
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
	case isAny(v.typ) && !isAny(val.typ):
		// `:=` would give the local the value's Go type; later writes need any
		f.emit("var %s %s = %s", v.goName, f.c.goType(v.typ), code)
	default:
		f.emit("%s := %s", v.goName, code)
	}
	if f.pass < 2 {
		info.writes++
	}
	f.noteUnused(v)
	return expr{code: v.goName, typ: v.typ, stmt: true, done: true}
}

// ---- function bodies

// genBody runs the two-pass body generation into f.buf.
func (f *fctx) genBody(body parser.Node, params []*local, t tail, prologue func()) {
	final := f.buf
	f.locals = map[string]*localInfo{}
	for _, p := range params {
		f.locals[p.name] = &localInfo{declBlock: "", typ: p.typ, annotated: true, reads: 1, noHoist: true}
	}
	for pass := 1; pass <= 2; pass++ {
		f.pass = pass
		if f.discover {
			f.pass = 0
			if pass == 2 {
				break
			}
		}
		f.buf = &strings.Builder{}
		f.tmp, f.blockCtr, f.block = 0, 0, ""
		f.scope = &scope{vars: map[string]*local{}}
		for _, p := range params {
			f.scope.vars[p.name] = &local{name: p.name, goName: p.goName, typ: p.typ, declared: true}
		}
		if prologue != nil {
			prologue()
		}
		if pass == 2 {
			for name, info := range sortedLocals(f.locals) {
				_ = name
				if info.hoist && !info.noHoist && info.typ != nil && !isNil(info.typ) {
					f.emit("var %s %s", goLocalName(info.name), f.c.goType(info.typ))
				}
			}
		}
		f.genStmts(body, t)
	}
	final.WriteString(f.buf.String())
	f.buf = final
}

type namedInfo struct {
	name string
	*localInfo
}

func sortedLocals(m map[string]*localInfo) []namedInfo {
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
		c.w("func (self %s) %s(%s) %s {\n", c.recvType(cls), m.GoName, params, retDecl)
	} else {
		self := "self Self"
		if cls.GoType != "" {
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
	c.w("func %s(%s) %s {\n", m.GoName, params, retDecl)
	c.emitBody(m, namedRet)
	c.w("}\n\n")
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
				f.emit("%s := (*Array[%s])(&%s_)", goLocalName(p.Name), c.goType(p.Type), goLocalName(p.Name))
				f.emit("_ = %s", goLocalName(p.Name)) // `*_args` may go unused
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

func (c *Compiler) emitMain() {
	c.w("var rb_main = &Object{}\n\n")
	c.w("func main() {\n\tdefer func() { _ = stdout.Flush() }()\n\tdefer rbTopRecover()\n")
	f := c.newFctx(c.mainFile, nil, nil)
	f.indent = 1
	f.retVar = ""
	stmts := &parser.StatementsNode{Body: c.mainBody()}
	f.genBody(stmts, nil, tail{}, nil)
	c.out.WriteString(f.buf.String())
	c.w("}\n\n")
}

// discoverIvars dry-runs struct class method bodies to learn ivar types
// from assignments.
func (c *Compiler) discoverIvars() {
	for range 2 {
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
				if m.Kind != kindDef {
					continue
				}
				func() {
					defer func() {
						if r := recover(); r != nil {
							if _, ok := r.(compileError); !ok {
								panic(r)
							}
						}
					}()
					f := c.newFctx(m.File, cls, m)
					f.discover = true
					f.retVar = "ret_"
					t := tail{kind: tailReturn, typ: m.Ret}
					if m.Iterator {
						t = tail{}
					}
					f.genBody(m.Node.Body, c.paramLocals(m), t, nil)
				}()
			}
		}
	}
}

// emitSynth emits a metaclass's generated method as a Go method.
func (c *Compiler) emitSynth(m *Method) {
	meta := m.Owner
	cls := meta.metaOf
	params, ret := c.sig(m, map[string]Type{"Self": TClass{C: cls}})
	body := fmt.Sprintf("return String(%q)", cls.RubyName)
	if m.Name == "new" {
		body = "return New" + cls.Name + "(" + c.argNames(m) + ")"
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

// mainBody merges main.rb's top-level statements with every constant
// assignment: prelude constants first, then main.rb's by position.
func (c *Compiler) mainBody() []parser.Node {
	var body []parser.Node
	var mine []*Const
	for _, k := range c.constList {
		if k.File == c.mainFile {
			mine = append(mine, k)
		} else {
			body = append(body, &constInit{k: k})
		}
	}
	stmts := c.mainStmts
	for len(stmts) > 0 || len(mine) > 0 {
		if len(mine) > 0 && (len(stmts) == 0 || mine[0].Value.GetLocation().StartOffset < stmts[0].GetLocation().StartOffset) {
			body = append(body, &constInit{k: mine[0]})
			mine = mine[1:]
			continue
		}
		body = append(body, stmts[0])
		stmts = stmts[1:]
	}
	return body
}

// genConstInit assigns a constant's package variable.
func (f *fctx) genConstInit(k *Const) {
	typ := f.c.constType(k)
	sub := f.c.constFctx(k)
	sub.indent = f.indent + 1
	e := sub.genExpr(k.Value, typ)
	code := sub.coerce(k.Value, e, typ)
	if e.lit {
		code = f.c.goType(typ) + "(" + code + ")"
	}
	fmt.Fprintf(f.buf, "//line %s:%d\n", k.File.Name, k.Line)
	if sub.buf.Len() == 0 {
		f.emit("%s = %s", k.GoName, code)
		return
	}
	f.emit("%s = func() %s {", k.GoName, f.c.goType(typ))
	f.buf.WriteString(sub.buf.String())
	f.emit("\treturn %s", code)
	f.emit("}()")
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
