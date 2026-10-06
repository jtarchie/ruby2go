package compiler

import (
	"strconv"
	"strings"

	parser "github.com/danielgatis/go-ruby-prism/parser"
)

// Pattern matching (decision 143): case/in, `v => pat` and `v in pat`. Each pattern is a chain of Go ifs, one per check, with the rest of the match nested in the success branch; bindings are ordinary local writes.

// patMatch is one match's state.
type patMatch struct {
	single  bool   // `=>`, or a case/in of one arm and no else: a failure raises MRI's detailed message
	err     string // Go var: why the match failed (single only)
	keyHit  string // Go var: it failed on a missing hash-pattern key (NoMatchingPatternKeyError)
	key     string // Go var: that key
	matchee string // Go var: the Hash it was missing from
	usesKey bool
	keyFail bool // the failure being recorded is a missing key
	dead    bool // a check failed at compile time, so this arm never matches
	bound   []patBound
}

// patBound is a local the pattern binds and the type of the value it binds.
type patBound struct {
	name string
	typ  Type
}

func (f *fctx) newPatMatch(single bool) *patMatch {
	pm := &patMatch{single: single}
	if single {
		pm.err, pm.keyHit, pm.key, pm.matchee = f.newTmp(), f.newTmp(), f.newTmp(), f.newTmp()
	}
	return pm
}

// patSubject evaluates a match's subject once, into a temp.
func (f *fctx) patSubject(n parser.Node) expr {
	e := f.genExpr(n, nil)
	if _, ok := e.typ.(TVoid); ok {
		e = f.voidAsNil(e)
	}
	if isNil(e.typ) {
		if strings.Trim(e.code, "()") != "nil" {
			f.voidAsNil(e) // `case log_nil`: the call still runs
		}
		return expr{code: "nil", typ: TNil{}}
	}
	tmp := f.newTmp()
	f.emit("%s := %s", tmp, f.materialize(e))
	return expr{code: tmp, typ: e.typ}
}

// patWrap emits the match's declarations before its code: the subject temp's use (a pattern like `in Object` reads nothing) and the failure message vars.
func (f *fctx) patWrap(pm *patMatch, subj expr, code string) {
	if subj.code != "nil" && !usesGoName(code, subj.code) {
		f.emit("_ = %s", subj.code)
	}
	if pm.single {
		f.emit("var %s string", pm.err)
		if pm.usesKey {
			f.emit("var %s bool", pm.keyHit)
			f.emit("var %s, %s any", pm.key, pm.matchee)
		}
	}
	f.buf.WriteString(code)
}

// usesGoName reports whether code mentions the Go identifier name as a whole word.
func usesGoName(code, name string) bool {
	word := func(b byte) bool {
		return b == '_' || '0' <= b && b <= '9' || 'a' <= b && b <= 'z' || 'A' <= b && b <= 'Z'
	}
	for i := 0; ; {
		j := strings.Index(code[i:], name)
		if j < 0 {
			return false
		}
		j += i
		end := j + len(name)
		if (j == 0 || !word(code[j-1])) && (end == len(code) || !word(code[end])) {
			return true
		}
		i = j + 1
	}
}

// genCaseMatch is `case v; in pat [if guard] then ...; else ...; end`.
func (f *fctx) genCaseMatch(n *parser.CaseMatchNode, t tail) {
	subj := f.patSubject(n.Predicate)
	pm := f.newPatMatch(len(n.Conditions) == 1 && n.ElseClause == nil)
	var subjLocal *local
	if lv, ok := n.Predicate.(*parser.LocalVariableReadNode); ok {
		subjLocal = f.scope.lookup(lv.Name)
	}
	code := f.capture(func() { f.genInArms(n, subj, subjLocal, n.Conditions, n.ElseClause, pm, t) })
	f.patWrap(pm, subj, code)
}

// genInArms emits the first arm and, in its else, the rest.
func (f *fctx) genInArms(n parser.Node, subj expr, subjLocal *local, arms []parser.Node, els *parser.ElseNode, pm *patMatch, t tail) {
	if len(arms) == 0 {
		if els != nil {
			f.genStmts(els.Statements, t)
			return
		}
		f.patRaise(n, pm, subj)
		return
	}
	in, ok := arms[0].(*parser.InNode)
	if !ok {
		f.c.unsupported(f.f, arms[0])
	}
	pat, guard, negate := splitGuard(in.Pattern)
	okVar := f.newTmp()
	pm.dead = false
	code := f.capture(func() {
		// a scope of its own, like an if's: a binding the body or later code reads is hoisted, even when no check wraps it
		saved := f.enterBlock()
		f.genPat(pm, pat, subj, func(expr) { f.patGuard(pm, guard, negate, okVar) })
		f.leaveBlock(saved)
	})
	if pm.dead && !pm.single { // MRI tries it and moves on; there is nothing to try
		f.genInArms(n, subj, subjLocal, arms[1:], els, pm, t)
		return
	}
	if pm.dead {
		f.buf.WriteString(code)
		f.patRaise(n, pm, subj)
		return
	}
	f.emit("var %s bool", okVar)
	f.buf.WriteString(code)
	f.emit("if %s {", okVar)
	saved := f.enterBlock()
	f.indent++
	if subjLocal != nil {
		if cls := f.classRef(topClass(pat)); cls != nil {
			v := subjLocal
			if ne := f.patNarrow(expr{code: v.goName, typ: v.typ, view: v.view}, cls); ne.code != v.goName {
				f.applyNarrow([]narrowInfo{{local: v, typ: ne.typ, code: ne.code, view: ne.view}})
			}
		}
	}
	f.genStmts(in.Statements, t)
	f.indent--
	f.leaveBlock(saved)
	f.emit("} else {")
	saved = f.enterBlock()
	f.indent++
	f.genInArms(n, subj, subjLocal, arms[1:], els, pm, t)
	f.indent--
	f.leaveBlock(saved)
	f.emit("}")
}

// splitGuard takes `pat if g` / `pat unless g` apart.
func splitGuard(p parser.Node) (parser.Node, parser.Node, bool) {
	switch g := p.(type) {
	case *parser.IfNode:
		if g.Statements != nil && len(g.Statements.Body) == 1 {
			return g.Statements.Body[0], g.Predicate, false
		}
	case *parser.UnlessNode:
		if g.Statements != nil && len(g.Statements.Body) == 1 {
			return g.Statements.Body[0], g.Predicate, true
		}
	}
	return p, nil, false
}

// topClass is the class a pattern checks first, if any: the subject local is that class in the arm's body.
func topClass(p parser.Node) parser.Node {
	switch p := p.(type) {
	case *parser.ConstantReadNode, *parser.ConstantPathNode:
		return p
	case *parser.ArrayPatternNode:
		return p.Constant
	case *parser.FindPatternNode:
		return p.Constant
	case *parser.HashPatternNode:
		return p.Constant
	case *parser.CapturePatternNode:
		return topClass(p.Value)
	}
	return nil
}

func (f *fctx) patGuard(pm *patMatch, guard parser.Node, negate bool, okVar string) {
	if guard == nil {
		f.emit("%s = true", okVar)
		return
	}
	quiet := f.patQuiet
	f.patQuiet = false // the guard is the user's code, not the match: its dynamic calls warn
	cond, nw := f.genCond(guard)
	f.patQuiet = quiet
	if negate {
		cond, nw = "!("+cond+")", nil
	}
	f.patIf(pm, cond, func() string { return `"guard clause does not return true"` }, func() {
		f.applyNarrow(nw)
		f.emit("%s = true", okVar)
	})
}

// genMatchRequired is `v => pat`: bind or raise.
func (f *fctx) genMatchRequired(n *parser.MatchRequiredNode) expr {
	subj := f.patSubject(n.Value)
	pm := f.newPatMatch(true)
	okVar := f.newTmp()
	code := f.capture(func() {
		f.emit("var %s bool", okVar)
		saved := f.enterBlock()
		f.genPat(pm, n.Pattern, subj, func(expr) { f.emit("%s = true", okVar) })
		f.leaveBlock(saved)
		f.emit("if !%s {", okVar)
		f.indent++
		f.patRaise(n, pm, subj)
		f.indent--
		f.emit("}")
	})
	f.patWrap(pm, subj, code)
	return expr{code: "nil", typ: TNil{}, done: true}
}

// genMatch is `v => pat` (bind or raise) or `v in pat` (true when it matches, binding as it goes).
func (f *fctx) genMatch(n parser.Node) expr {
	if r, ok := n.(*parser.MatchRequiredNode); ok {
		return f.genMatchRequired(r)
	}
	e, _ := f.matchPredicate(n.(*parser.MatchPredicateNode))
	return e
}

// condMatchPredicate is `if v in pat`: the locals it binds to a value that is never nil are not nil inside, though a path that skips the match leaves them unset.
func (f *fctx) condMatchPredicate(n *parser.MatchPredicateNode) (string, []narrowInfo) {
	e, pm := f.matchPredicate(n)
	var nw []narrowInfo
	for _, b := range pm.bound {
		if isOpt(b.typ) || isNil(b.typ) || isVoid(b.typ) {
			continue
		}
		v := f.visibleLocal(b.name)
		if v == nil {
			v = f.sameScopeLocal(b.name)
		}
		if v != nil && v.base == nil && isOpt(v.typ) && !isAny(stripOpt(v.typ)) && f.unset[f.localKey(b.name)] {
			nw = append(nw, narrowInfo{local: v, typ: stripOpt(v.typ)})
		}
	}
	return "bool(" + e.code + ")", nw
}

func (f *fctx) matchPredicate(n *parser.MatchPredicateNode) (expr, *patMatch) {
	subj := f.patSubject(n.Value)
	pm := f.newPatMatch(false)
	okVar := f.newTmp()
	code := f.capture(func() {
		f.emit("var %s bool", okVar)
		saved := f.enterBlock()
		f.genPat(pm, n.Pattern, subj, func(expr) { f.emit("%s = true", okVar) })
		f.leaveBlock(saved)
	})
	f.patWrap(pm, subj, code)
	return expr{code: "Boolean(" + okVar + ")", typ: f.cls("Boolean")}, pm
}

// patRaise raises NoMatchingPatternError: the subject's inspect, and in a single pattern why it failed; a missing key raises NoMatchingPatternKeyError.
func (f *fctx) patRaise(n parser.Node, pm *patMatch, subj expr) {
	msg := f.patInspect(n, subj)
	if pm.single {
		msg += ` + ": " + ` + pm.err
	}
	strT := f.cls("String")
	if pm.single && pm.usesKey {
		f.emit("if %s {", pm.keyHit)
		f.indent++
		args := []parser.Node{
			&exprNode{Node: n, e: expr{code: "String(" + msg + ")", typ: strT}},
			&exprNode{Node: n, e: expr{code: pm.matchee, typ: TAny{}}},
			&exprNode{Node: n, e: expr{code: pm.key, typ: TAny{}}},
		}
		recv := &parser.ConstantReadNode{Location: n.GetLocation(), Name: "NoMatchingPatternKeyError"}
		e := f.genExpr(&parser.CallNode{Location: n.GetLocation(), Receiver: recv, Name: "__for", Arguments: &parser.ArgumentsNode{Location: n.GetLocation(), Arguments: args}}, nil)
		f.emit("panic(%s)", f.withCause(e.code))
		f.indent--
		f.emit("}")
	}
	e := f.genNew(n, f.c.classes["NoMatchingPatternError"], nil, []expr{{code: "String(" + msg + ")", typ: strT}}, nil)
	f.emit("panic(%s)", f.withCause(e.code))
}

func (f *fctx) withCause(code string) string {
	if f.rescues > 0 {
		return "rbWithCause(" + code + ", r_)" // raised while handling r_: MRI's cause
	}
	return code
}

// patInspect is e.inspect as a Go string.
func (f *fctx) patInspect(n parser.Node, e expr) string {
	if isNil(e.typ) {
		return `"nil"`
	}
	quiet := f.patQuiet
	f.patQuiet = true
	defer func() { f.patQuiet = quiet }()
	return "string(" + f.genMethodCall(n, e, "inspect", nil, nil).code + ")"
}

// patIf emits `if cond { then }`, with an else recording fail's message in a single pattern. A cond of "false" never runs then: the arm is dead.
func (f *fctx) patIf(pm *patMatch, cond string, fail func() string, then func()) {
	switch cond {
	case "true":
		then()
		return
	case "false":
		pm.dead = true
		if pm.single && fail != nil {
			f.patFail(pm, fail)
		}
		return
	}
	f.emit("if %s {", cond)
	saved := f.enterBlock()
	f.indent++
	then()
	f.indent--
	f.leaveBlock(saved)
	if pm.single && fail != nil {
		f.emit("} else {")
		saved := f.enterBlock()
		f.indent++
		f.patFail(pm, fail)
		f.indent--
		f.leaveBlock(saved)
	}
	f.emit("}")
}

// patFail records why a single pattern failed. Only the last failure counts, as MRI, so one that is not a missing key clears an earlier key failure (an alternative's).
func (f *fctx) patFail(pm *patMatch, fail func() string) {
	pm.keyFail = false
	msg := fail()
	if pm.usesKey && !pm.keyFail {
		f.emit("%s = false", pm.keyHit)
	}
	f.emit("%s = %s", pm.err, msg)
}

// genPat emits the checks for pattern p against subj; then runs (once, in the success branch) with subj as the pattern narrowed it.
func (f *fctx) genPat(pm *patMatch, p parser.Node, subj expr, then func(expr)) {
	quiet := f.patQuiet
	f.patQuiet = true
	defer func() { f.patQuiet = quiet }()
	switch p.(type) {
	case *parser.ImplicitNode, *parser.LocalVariableTargetNode:
	default:
		if !subj.lit && !isSimpleGo(subj.code) && !isNil(subj.typ) {
			// a checked element or value is read by the check and again by what binds it: evaluate it once, if at all
			tmp := f.newTmp()
			s := expr{code: tmp, typ: subj.typ, view: subj.view}
			code := f.capture(func() { f.genPat(pm, p, s, then) })
			if usesGoName(code, tmp) {
				f.emit("%s := %s", tmp, subj.code)
			}
			f.buf.WriteString(code)
			return
		}
	}
	switch p := p.(type) {
	case *parser.ImplicitNode: // `{k:}` binds k
		f.genPat(pm, p.Value, subj, then)
	case *parser.LocalVariableTargetNode:
		f.patBind(pm, p, p.Name, subj)
		then(subj)
	case *parser.CapturePatternNode:
		f.genPat(pm, p.Value, subj, func(s expr) {
			f.patBind(pm, p.Target, p.Target.Name, s)
			then(s)
		})
	case *parser.AlternationPatternNode:
		f.patAlt(pm, p, subj, then)
	case *parser.PinnedVariableNode:
		f.patValue(pm, p.Variable, subj, then)
	case *parser.PinnedExpressionNode:
		f.patValue(pm, p.Expression, subj, then)
	case *parser.ArrayPatternNode:
		f.patConst(pm, p.Constant, subj, func(s expr) { f.patArray(pm, p, s, then) })
	case *parser.FindPatternNode:
		f.patConst(pm, p.Constant, subj, func(s expr) { f.patFind(pm, p, s, then) })
	case *parser.HashPatternNode:
		f.patConst(pm, p.Constant, subj, func(s expr) { f.patHash(pm, p, s, then) })
	case *parser.SplatNode, *parser.AssocSplatNode, *parser.NoKeywordsParameterNode, *parser.ImplicitRestNode:
		f.c.unsupported(f.f, p)
	default:
		f.patValue(pm, p, subj, then)
	}
}

func (f *fctx) patBind(pm *patMatch, n parser.Node, name string, v expr) {
	f.patQuiet = false // a binding is the user's local: its later dynamic uses warn as usual
	f.assignLocal(n, name, v, nil)
	f.patQuiet = true
	pm.bound = append(pm.bound, patBound{name, v.typ})
}

func (f *fctx) patConst(pm *patMatch, c parser.Node, subj expr, then func(expr)) {
	if c == nil {
		then(subj)
		return
	}
	f.patValue(pm, c, subj, then)
}

// patValue is a value pattern: `pat === subj`. A class narrows subj.
func (f *fctx) patValue(pm *patMatch, v parser.Node, subj expr, then func(expr)) {
	if _, ok := v.(*parser.NilNode); ok {
		cond, _ := f.markerIsA(subj, f.c.classes["NilClass"])
		f.patIf(pm, cond, func() string { return `"nil === " + ` + f.patInspect(v, subj) + ` + " does not return true"` }, func() { then(subj) })
		return
	}
	if cls := f.classRef(v); cls != nil {
		cond := f.caseEqq(v, subj)
		f.patIf(pm, cond, func() string {
			return strconv.Quote(cls.RubyName+" === ") + " + " + f.patInspect(v, subj) + ` + " does not return true"`
		}, func() { then(f.patNarrow(subj, cls)) })
		return
	}
	// the pattern's value is evaluated once: === and the failure message both read it
	quiet := f.patQuiet
	f.patQuiet = false // `^(expr)` is the user's code
	e := f.genExpr(v, subj.typ)
	f.patQuiet = quiet
	if !e.lit && !isSimpleGo(e.code) && !isNil(e.typ) {
		tmp := f.newTmp()
		f.emit("%s := %s", tmp, e.code)
		e.code = tmp
	}
	pv := &exprNode{Node: v, e: e}
	cond := f.caseEqq(pv, subj)
	f.patIf(pm, cond, func() string {
		return f.patInspect(v, e) + ` + " === " + ` + f.patInspect(v, subj) + ` + " does not return true"`
	}, func() { then(subj) })
}

// patNarrow is e seen as cls once `cls === e` held.
func (f *fctx) patNarrow(e expr, cls *Class) expr {
	if cls.universal || cls.IsModule {
		return e
	}
	switch cls.RubyName {
	case "TrueClass", "FalseClass", "NilClass", "Proc": // no Go type of their own
		return e
	}
	base := stripOpt(e.typ)
	code := e.code
	if isOpt(e.typ) && !isAny(base) {
		code = "(*" + code + ")"
	}
	if bt, ok := base.(TClass); ok && bt.C.isSubclassOf(cls) {
		return expr{code: code, typ: base, view: e.view}
	}
	switch b := base.(type) {
	case TVar:
		if b.Name == "Self" && f.owner != nil && !f.owner.IsModule {
			if f.owner.isSubclassOf(cls) {
				return expr{code: code, typ: base}
			}
		}
		code = "any(" + code + ")"
	case TAny:
		if e.view != "" {
			code = e.view
		}
	case TUnion: // an any already
	case TClass:
		if !b.C.isStruct() && !isAbstract(b) { // a @go_type value is not an interface: the check was static
			return e
		}
	case TFunc, TNil, TOpt, TTuple, TVoid:
		return e
	}
	typ := TClass{C: cls}
	if len(cls.TypeParams) > 0 {
		for range cls.TypeParams {
			typ.Args = append(typ.Args, TAny{})
		}
		return expr{code: code + ".(" + cls.Name + "_Any)._ToAny()", typ: typ, view: code}
	}
	return expr{code: code + ".(" + f.c.goType(typ) + ")", typ: typ}
}

// patAlt is `a | b | c`: the first that matches (alternatives cannot bind).
func (f *fctx) patAlt(pm *patMatch, p *parser.AlternationPatternNode, subj expr, then func(expr)) {
	var alts []parser.Node
	var flat func(parser.Node)
	flat = func(n parser.Node) {
		if a, ok := n.(*parser.AlternationPatternNode); ok {
			flat(a.Left)
			flat(a.Right)
			return
		}
		alts = append(alts, n)
	}
	flat(p)
	hit := f.newTmp()
	dead := pm.dead
	bound := len(pm.bound)
	defer func() { pm.bound = pm.bound[:bound] }() // a `_x` one alternative binds is unset when another matched: `if v in` must not narrow it
	allDead := true
	codes := make([]string, 0, len(alts))
	for _, a := range alts {
		saved := f.enterBlock()
		pm.dead = false
		codes = append(codes, f.capture(func() { f.genPat(pm, a, subj, func(expr) { f.emit("%s = true", hit) }) }))
		allDead = allDead && pm.dead
		f.leaveBlock(saved)
	}
	pm.dead = dead || allDead
	if allDead { // only failure messages, if any: MRI reports the last
		f.buf.WriteString(codes[len(codes)-1])
		return
	}
	f.emit("var %s bool", hit)
	first := true
	for _, c := range codes {
		switch {
		case c == "": // statically no match, and nothing to report
		case first:
			f.buf.WriteString(c)
			first = false
		default:
			f.emit("if !%s {", hit)
			f.buf.WriteString(c)
			f.emit("}")
		}
	}
	f.emit("if %s {", hit)
	saved := f.enterBlock()
	f.indent++
	then(subj)
	f.indent--
	f.leaveBlock(saved)
	f.emit("}")
}

// ---- array and find patterns

// seq is a deconstructed value: static elements (a tuple, a Struct/Data's members), or an Array in a Go temp.
type seq struct {
	elems []expr // static elements; nil for an Array
	arr   expr   // the Array (*Array[E])
	insp  func() string
}

func (s seq) elemType() Type {
	return s.arr.typ.(TClass).Args[0]
}

func (f *fctx) patArray(pm *patMatch, p *parser.ArrayPatternNode, subj expr, then func(expr)) {
	restBinds := false
	if sp, ok := p.Rest.(*parser.SplatNode); ok && sp.Expression != nil {
		restBinds = true
	}
	f.patDeconstruct(pm, p, subj, restBinds, func(s seq) {
		nreq, npost := len(p.Requireds), len(p.Posts)
		want := strconv.Itoa(nreq + npost)
		if p.Rest != nil {
			want += "+"
		}
		lenMsg := func(given string) func() string {
			return func() string {
				return s.insp() + ` + " length mismatch (given " + ` + given + ` + ", expected ` + want + `)"`
			}
		}
		type item struct {
			pat parser.Node
			e   expr
		}
		var items []item
		if s.elems != nil {
			n := len(s.elems)
			ok := n == nreq+npost || p.Rest != nil && n >= nreq+npost
			cond := strconv.FormatBool(ok)
			f.patIf(pm, cond, lenMsg(strconv.Quote(strconv.Itoa(n))), func() {
				for i, r := range p.Requireds {
					items = append(items, item{r, s.elems[i]})
				}
				for j, r := range p.Posts {
					items = append(items, item{r, s.elems[n-npost+j]})
				}
			})
			if !ok {
				return
			}
		} else {
			d := s.arr.code
			et := s.elemType()
			cond := "len(*" + d + ") == " + strconv.Itoa(nreq+npost)
			if p.Rest != nil {
				cond = "len(*" + d + ") >= " + strconv.Itoa(nreq+npost)
			}
			for i, r := range p.Requireds {
				items = append(items, item{r, expr{code: "(*" + d + ")[" + strconv.Itoa(i) + "]", typ: et}})
			}
			if sp, ok := p.Rest.(*parser.SplatNode); ok && sp.Expression != nil {
				items = append(items, item{sp.Expression, expr{code: "rbMidSplat(" + d + ", " + strconv.Itoa(nreq) + ", " + strconv.Itoa(npost) + ")", typ: s.arr.typ}})
			}
			for j, r := range p.Posts {
				items = append(items, item{r, expr{code: "(*" + d + ")[len(*" + d + ")-" + strconv.Itoa(npost-j) + "]", typ: et}})
			}
			var steps func(k int)
			steps = func(k int) {
				if k == len(items) {
					then(subj)
					return
				}
				f.genPat(pm, items[k].pat, items[k].e, func(expr) { steps(k + 1) })
			}
			f.patIf(pm, cond, lenMsg("strconv.Itoa(len(*"+d+"))"), func() { steps(0) })
			return
		}
		var steps func(k int)
		steps = func(k int) {
			if k == len(items) {
				then(subj)
				return
			}
			f.genPat(pm, items[k].pat, items[k].e, func(expr) { steps(k + 1) })
		}
		steps(0)
	})
}

// patFind is `[*pre, a, b, *post]`: the first window where a, b match.
func (f *fctx) patFind(pm *patMatch, p *parser.FindPatternNode, subj expr, then func(expr)) {
	f.patDeconstruct(pm, p, subj, true, func(s seq) {
		d := s.arr.code
		n := len(p.Requireds)
		et := s.elemType()
		cond := "len(*" + d + ") >= " + strconv.Itoa(n)
		lenMsg := func() string {
			return s.insp() + ` + " length mismatch (given " + strconv.Itoa(len(*` + d + `)) + ", expected ` + strconv.Itoa(n) + `+)"`
		}
		f.patIf(pm, cond, lenMsg, func() {
			found, i := f.newTmp(), f.newTmp()
			f.emit("var %s bool", found)
			f.emit("for %s := 0; %s+%d <= len(*%s); %s++ {", i, i, n, d, i)
			saved := f.enterBlock()
			f.indent++
			single, dead := pm.single, pm.dead
			pm.single = false // MRI reports the find as a whole
			var steps func(k int)
			steps = func(k int) {
				if k == n {
					if sp := p.Left; sp != nil && sp.Expression != nil {
						f.genPat(pm, sp.Expression, expr{code: "rbMidSplat(" + d + ", 0, len(*" + d + ")-" + i + ")", typ: s.arr.typ}, func(expr) {})
					}
					if sp, ok := p.Right.(*parser.SplatNode); ok && sp.Expression != nil {
						f.genPat(pm, sp.Expression, expr{code: "rbMidSplat(" + d + ", " + i + "+" + strconv.Itoa(n) + ", 0)", typ: s.arr.typ}, func(expr) {})
					}
					f.emit("%s = true", found)
					f.emit("break")
					return
				}
				f.genPat(pm, p.Requireds[k], expr{code: "(*" + d + ")[" + i + "+" + strconv.Itoa(k) + "]", typ: et}, func(expr) { steps(k + 1) })
			}
			steps(0)
			pm.single = single
			deadInside := pm.dead
			pm.dead = dead
			f.indent--
			f.leaveBlock(saved)
			f.emit("}")
			if deadInside {
				pm.dead = true
				if pm.single {
					f.patFail(pm, func() string { return s.insp() + ` + " does not match to find pattern"` })
				}
				return
			}
			f.patIf(pm, found, func() string { return s.insp() + ` + " does not match to find pattern"` }, func() { then(subj) })
		})
	})
}

// patDeconstruct takes subj apart for an array or find pattern, as MRI's `deconstruct`: an Array is itself, a tuple and a Struct/Data its fields (statically, unless an Array is needed), another class its own deconstruct, and an untyped value whatever its class answers.
func (f *fctx) patDeconstruct(pm *patMatch, p parser.Node, subj expr, needArray bool, then func(seq)) {
	f.patShape(pm, p, subj, "deconstruct", func(s expr) {
		switch t := s.typ.(type) {
		case TTuple:
			if !needArray {
				elems := make([]expr, len(t.Elems))
				for i, et := range t.Elems {
					elems[i] = expr{code: s.code + ".F" + strconv.Itoa(i), typ: et}
				}
				then(seq{elems: elems, insp: func() string { return f.patInspect(p, s) }})
				return
			}
			s = expr{code: s.code + "._ToAny()", typ: TClass{C: f.c.classes["Array"], Args: []Type{TAny{}}}}
		case TClass:
			if t.C.RubyName == "Array" {
				break
			}
			if e := f.resolve(t, "deconstruct"); e != nil && e.M.valueGen && !needArray && !t.C.descendantDefines("deconstruct", false) {
				members := t.C.valueRoot().valueMembers
				elems := make([]expr, len(members))
				for i, m := range members {
					elems[i] = f.genMethodCall(p, s, m, nil, nil)
				}
				then(seq{elems: elems, insp: func() string { return f.patInspect(p, f.genMethodCall(p, s, "deconstruct", nil, nil)) }})
				return
			}
			s = f.genMethodCall(p, s, "deconstruct", nil, nil)
		case TAny:
			s = f.genMethodCall(p, s, "deconstruct", nil, nil)
		case TFunc, TNil, TOpt, TUnion, TVar, TVoid: // patShape leaves only classes, tuples and untyped
		}
		arrT := TClass{C: f.c.classes["Array"], Args: []Type{TAny{}}}
		switch t := s.typ.(type) {
		case TClass:
			if t.C.RubyName != "Array" {
				f.errorf(p, "deconstruct must return an Array, not %s", s.typ)
			}
		case TTuple:
			s = expr{code: s.code + "._ToAny()", typ: arrT}
		case TAny:
			s = expr{code: f.coerce(p, s, arrT), typ: arrT}
		default:
			f.errorf(p, "deconstruct must return an Array, not %s", s.typ)
		}
		if !isSimpleGo(s.code) {
			tmp := f.newTmp()
			f.emit("%s := %s", tmp, s.code)
			s.code = tmp
		}
		then(seq{arr: s, insp: func() string { return f.patInspect(p, s) }})
	})
}

// patShape checks that subj can be taken apart by method (deconstruct or deconstruct_keys) and runs then with it: statically for a class that defines it, by respond_to? for untyped (and for a class only some subclasses define it on), after a nil check for T?.
func (f *fctx) patShape(pm *patMatch, p parser.Node, subj expr, method string, then func(expr)) {
	noResp := func() string { return f.patInspect(p, subj) + ` + " does not respond to #` + method + `"` }
	t := subj.typ
	if v, ok := t.(TVar); ok && v.Name == "Self" && f.owner != nil && !f.owner.IsModule {
		t = TClass{C: f.owner}
	}
	switch tt := t.(type) {
	case TNil:
		f.patIf(pm, "false", noResp, nil)
		return
	case TOpt:
		if !isAny(tt.Elem) {
			f.patIf(pm, subj.code+" != nil", noResp, func() {
				f.patShape(pm, p, expr{code: "(*" + subj.code + ")", typ: tt.Elem}, method, then)
			})
			return
		}
	case TTuple:
		if method == "deconstruct" {
			then(subj)
			return
		}
		f.errorf(p, "a hash pattern never matches %s: an Array has no deconstruct_keys", t)
	case TClass:
		if !isAbstract(tt) {
			if tt.C.lookup(method) != nil {
				then(expr{code: subj.code, typ: tt, view: subj.view})
				return
			}
			if !tt.C.isStruct() || !tt.C.descendantDefines(method, false) {
				f.errorf(p, "%s pattern never matches %s: %s has no %s", map[string]string{"deconstruct": "an array", "deconstruct_keys": "a hash"}[method], t, tt.C.RubyName, method)
			}
		}
	case TAny, TFunc, TUnion, TVar, TVoid: // a union's member is known at run time: patterns test it as untyped
	}
	// at run time: whatever the value's class answers
	anyS := expr{code: f.coerce(p, subj, TAny{}), typ: TAny{}}
	if !isSimpleGo(anyS.code) {
		tmp := f.newTmp()
		f.emit("%s := %s", tmp, anyS.code)
		anyS.code = tmp
	}
	sym := &parser.SymbolNode{Location: p.GetLocation(), Unescaped: parser.RubyString{Value: method}}
	resp := f.genDynRespondTo(p, anyS, []parser.Node{sym})
	f.patIf(pm, "bool("+resp.code+")", noResp, func() { then(anyS) })
}

// ---- hash patterns

// kv is a value taken apart for a hash pattern: static members of a Struct/Data, or a Hash in a Go temp.
type kv struct {
	members map[string]expr // Struct/Data readers; nil for a Hash
	order   []string        // the members, in order
	hash    expr            // the Hash
	matchee func() string   // Go code for the deconstructed Hash (any), for NoMatchingPatternKeyError
}

func (f *fctx) patHash(pm *patMatch, p *parser.HashPatternNode, subj expr, then func(expr)) {
	type key struct {
		name string
		node *parser.AssocNode
	}
	keys := make([]key, 0, len(p.Elements))
	for _, el := range p.Elements {
		a, ok := el.(*parser.AssocNode)
		if !ok {
			f.c.unsupported(f.f, el)
		}
		sym, ok := a.Key.(*parser.SymbolNode)
		if !ok {
			f.errorf(a.Key, "hash pattern keys must be symbols")
		}
		keys = append(keys, key{sym.Unescaped.Value, a})
	}
	names := make([]string, len(keys))
	for i, k := range keys {
		names[i] = k.name
	}
	// MRI passes nil (every key) when the rest is bound or must be empty
	var restBind parser.Node
	noRest := false
	switch r := p.Rest.(type) {
	case *parser.AssocSplatNode:
		restBind = r.Value
	case *parser.NoKeywordsParameterNode:
		noRest = true
	}
	allKeys := restBind != nil || noRest || len(keys) == 0 // MRI passes nil for `{}` too
	symT := f.cls("Symbol")
	symExpr := func(name string) expr {
		return expr{code: "Symbol(" + strconv.Quote(name) + ")", typ: symT}
	}
	f.patDeconstructKeys(pm, p, subj, names, allKeys, func(h kv) {
		var values func(i int)
		rest := func() {
			switch {
			case restBind != nil:
				f.genPat(pm, restBind, f.patExcept(p, h.hash, names, symExpr), then)
			case noRest:
				size := f.genMethodCall(p, h.hash, "size", nil, nil)
				f.patIf(pm, size.code+" == "+strconv.Itoa(len(keys)), func() string {
					return `"rest of " + ` + f.patInspect(p, f.patExcept(p, h.hash, names, symExpr)) + ` + " is not empty"`
				}, func() { then(subj) })
			case len(keys) == 0 && p.Rest == nil: // `{}` matches only an empty Hash
				cond := f.genMethodCall(p, h.hash, "size", nil, nil).code + " == 0"
				f.patIf(pm, cond, func() string { return f.patInspect(p, h.hash) + ` + " is not empty"` }, func() { then(subj) })
			default:
				then(subj)
			}
		}
		values = func(i int) {
			if i == len(keys) {
				rest()
				return
			}
			k := keys[i]
			var v expr
			if h.members != nil {
				v = h.members[k.name]
			} else {
				v = f.genMethodCall(k.node, h.hash, "fetch", []parser.Node{&exprNode{Node: k.node, e: symExpr(k.name)}}, nil)
			}
			pat := k.node.Value
			if pat == nil {
				f.errorf(k.node, "unsupported hash pattern element")
			}
			f.genPat(pm, pat, v, func(expr) { values(i + 1) })
		}
		// MRI checks every key is there before matching any value
		var present func(i int)
		present = func(i int) {
			if i == len(keys) {
				values(0)
				return
			}
			k := keys[i]
			var cond string
			if h.members != nil {
				_, ok := h.members[k.name]
				// MRI's Struct#deconstruct_keys answers {} for more keys than members
				cond = strconv.FormatBool(ok && len(keys) <= len(h.order))
			} else {
				cond = "bool(" + f.genMethodCall(k.node, h.hash, "key?", []parser.Node{&exprNode{Node: k.node, e: symExpr(k.name)}}, nil).code + ")"
			}
			f.patIf(pm, cond, func() string {
				if pm.single {
					pm.usesKey, pm.keyFail = true, true
					f.emit("%s = true", pm.keyHit)
					f.emit("%s = %s", pm.key, f.coerce(k.node, symExpr(k.name), TAny{}))
					f.emit("%s = %s", pm.matchee, h.matchee())
				}
				return `"key not found: " + ` + f.patInspect(k.node, symExpr(k.name))
			}, func() { present(i + 1) })
		}
		present(0)
	})
}

// patExcept is the Hash without the pattern's keys: `**rest`.
func (f *fctx) patExcept(p parser.Node, h expr, names []string, sym func(string) expr) expr {
	args := make([]parser.Node, len(names))
	for i, n := range names {
		args[i] = &exprNode{Node: p, e: sym(n)}
	}
	return f.genMethodCall(p, h, "except", args, nil)
}

// patDeconstructKeys takes subj apart for a hash pattern, as MRI's `deconstruct_keys(keys)` (nil when the pattern needs every key): a Hash is itself, a Struct/Data its members (statically, unless the rest is needed), another class its own deconstruct_keys.
func (f *fctx) patDeconstructKeys(pm *patMatch, p parser.Node, subj expr, names []string, allKeys bool, then func(kv)) {
	symT := f.cls("Symbol")
	keysArg := func() expr {
		if allKeys {
			return expr{code: "nil", typ: TNil{}}
		}
		elems := make([]string, len(names))
		for i, n := range names {
			elems[i] = "Symbol(" + strconv.Quote(n) + ")"
		}
		at := TClass{C: f.c.classes["Array"], Args: []Type{symT}}
		return expr{code: "&" + strings.TrimPrefix(f.c.goType(at), "*") + "{" + strings.Join(elems, ", ") + "}", typ: at}
	}
	hashAny := TClass{C: f.c.classes["Hash"], Args: []Type{TAny{}, TAny{}}}
	f.patShape(pm, p, subj, "deconstruct_keys", func(s expr) {
		if t, ok := s.typ.(TClass); ok && t.C.RubyName != "Hash" {
			if e := f.resolve(t, "deconstruct_keys"); e != nil && e.M.valueGen && !allKeys && !t.C.descendantDefines("deconstruct_keys", false) {
				order := t.C.valueRoot().valueMembers
				members := map[string]expr{}
				for _, m := range order {
					members[m] = f.genMethodCall(p, s, m, nil, nil)
				}
				then(kv{members: members, order: order, matchee: func() string {
					return f.coerce(p, f.genMethodCall(p, s, "deconstruct_keys", []parser.Node{&exprNode{Node: p, e: keysArg()}}, nil), TAny{})
				}})
				return
			}
		}
		if t, ok := s.typ.(TClass); !ok || t.C.RubyName != "Hash" {
			s = f.genMethodCall(p, s, "deconstruct_keys", []parser.Node{&exprNode{Node: p, e: keysArg()}}, nil)
		}
		switch t := s.typ.(type) {
		case TClass:
			if t.C.RubyName != "Hash" {
				f.errorf(p, "deconstruct_keys must return a Hash, not %s", s.typ)
			}
			if k := t.Args[0]; !isAny(k) && !isClass(k, "Symbol") {
				f.errorf(p, "a hash pattern's keys are Symbols, so it never matches %s", s.typ)
			}
		case TAny:
			s = expr{code: f.coerce(p, s, hashAny), typ: hashAny}
		default:
			f.errorf(p, "deconstruct_keys must return a Hash, not %s", s.typ)
		}
		if !isSimpleGo(s.code) {
			tmp := f.newTmp()
			f.emit("%s := %s", tmp, s.code)
			s.code = tmp
		}
		then(kv{hash: s, matchee: func() string { return f.coerce(p, s, TAny{}) }})
	})
}
