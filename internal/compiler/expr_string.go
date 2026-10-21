package compiler

import (
	"fmt"
	"slices"
	"strings"

	"github.com/danielgatis/go-ruby-prism/parser"
)

// genMatchWrite is `/(?<year>\d+)/ =~ s`: =~ through Regexp#match, then
// each named group into the local of its name (nil when nothing matched).
func (f *fctx) genMatchWrite(n *parser.MatchWriteNode) expr {
	call := n.Call
	re := f.genExpr(call.Receiver, nil)
	m := f.genMethodCall(call, re, "match", callArgs(call), nil)
	tmp := f.newTmp()
	f.emit("%s := %s", tmp, f.materialize(m))
	for _, t := range n.Targets {
		lt, ok := t.(*parser.LocalVariableTargetNode)
		if !ok {
			f.errorf(t, "unsupported named capture target %s", nodeType(t))
		}
		f.assignLocal(lt, lt.Name, expr{code: fmt.Sprintf("rbMatchCapture(%s, %q)", tmp, lt.Name), typ: TOpt{Elem: f.cls("String")}}, nil)
	}
	return expr{code: "rbMatchPos(" + tmp + ")", typ: TOpt{Elem: f.cls("Integer")}}
}

// subBlockLastMatch is `s.sub(re) { $1 }` / gsub in a frame that reads $~ or $1: MRI sets them to each match while the
// block runs, so the block runner writes the frame's last match first (decision 241).
func (f *fctx) subBlockLastMatch(n parser.Node, recv expr, name string, args []parser.Node, block parser.Node) (expr, bool) {
	b, ok := block.(*parser.BlockNode)
	if f.lastMatch == "" || !ok || name != "sub" && name != "gsub" || len(args) != 1 || !isClass(recv.typ, "String") {
		return expr{}, false
	}
	e := f.c.classes["String"].lookup("__" + name + "_block")
	if e == nil || e.M.Block == nil {
		return expr{}, false
	}
	r := f.newTmp()
	f.emit("%s := %s", r, recv.code)
	pat := f.newTmp()
	f.emit("%s := rbPattern(%s)", pat, f.coerce(args[0], f.genExpr(args[0], nil), TAny{}))
	blk := f.genClosure(n, b, e.M.Block, map[string]Type{})
	count := "-1"
	if name == "sub" {
		count = "1"
	}
	code := fmt.Sprintf("String(rbReSub(%s, string(%s), %s, rbReBlockLM(&%s, %s, %s)))", pat, r, count, f.lastMatch, pat, blk)
	return expr{code: code, typ: f.cls("String")}, true
}

// grepBlockLastMatch is `xs.grep(re) { $1 }` / grep_v on an Array in a frame that reads $~: each test sets the
// frame's last match before the block runs, as MRI's (decision 171's frame variable, handed to the runner).
func (f *fctx) grepBlockLastMatch(n parser.Node, recv expr, name string, args []parser.Node, block parser.Node) (expr, bool) {
	b, ok := block.(*parser.BlockNode)
	if f.lastMatch == "" || !ok || name != "grep" && name != "grep_v" || len(args) != 1 {
		return expr{}, false
	}
	at, ok := recv.typ.(TClass)
	if !ok || at.C.RubyName != "Array" || len(at.Args) != 1 {
		return expr{}, false
	}
	if !isClass(f.probeType(args[0], nil), "Regexp") {
		return expr{}, false
	}
	e := f.c.classes["Array"].lookup("__" + name + "_block")
	if e == nil || e.M.Block == nil {
		return expr{}, false
	}
	r := f.newTmp()
	f.emit("%s := %s", r, recv.code)
	pat := f.newTmp()
	f.emit("%s := %s", pat, f.genExpr(args[0], nil).code)
	env := map[string]Type{"E": at.Args[0]}
	blk := f.genClosure(n, b, e.M.Block, env)
	code := fmt.Sprintf("rbGrepLM(&%s, %s, %s.s, %v, %s)", f.lastMatch, pat, r, name == "grep_v", blk)
	return expr{code: code, typ: TClass{C: f.c.classes["Array"], Args: []Type{subst(e.M.Block.Ret, env)}}}, true
}

// lastMatchVar is the Go variable holding the frame's last match ($~, decision 171).
func (f *fctx) lastMatchVar(n parser.Node) string {
	if f.lastMatch == "" {
		f.errorf(n, "$~ and $1..$9 are read only in the method whose =~ sets them (decision 171)")
	}
	return f.lastMatch
}

// genMatchSet is `str =~ re` / `re =~ str` in a frame that reads $~ or $1..$9: it records the match (decision 171).
func (f *fctx) genMatchSet(n *parser.CallNode) (expr, bool) {
	args := callArgs(n)
	if f.lastMatch == "" || f.inMatchSet || n.Receiver == nil || len(args) != 1 || n.Block != nil {
		return expr{}, false
	}
	var recvT, argT Type
	f.probe(func() { recvT, argT = f.genExpr(n.Receiver, nil).typ, f.genExpr(args[0], nil).typ })
	re, subj := args[0], n.Receiver
	if isClass(recvT, "Regexp") {
		re, subj = n.Receiver, args[0]
	}
	switch {
	case n.Name == "=~":
	case n.Name == "index" && isClass(recvT, "String") && isClass(argT, "Regexp"): // String#index(re) sets $~ too
	case n.Name == "match" && (isClass(recvT, "Regexp") || isClass(argT, "Regexp")):
	default:
		return expr{}, false
	}
	f.inMatchSet = true // the match call below is the plain one
	m := f.genExpr(&parser.CallNode{Location: n.Location, Receiver: re, Name: "match", MessageLoc: n.MessageLoc,
		Arguments: &parser.ArgumentsNode{Location: n.Location, Arguments: []parser.Node{subj}}}, nil)
	f.inMatchSet = false
	if n.Name == "match" {
		return expr{code: fmt.Sprintf("rbMatchKeep(&%s, %s)", f.lastMatch, m.code), typ: m.typ}, true
	}
	return expr{code: fmt.Sprintf("rbMatchSet(&%s, %s)", f.lastMatch, m.code), typ: TOpt{Elem: f.cls("Integer")}}, true
}

// parenBang is a gem's `(o = q.downcase).tr!('_', '-')`: the assignment, then the bang on o (decision 244).
func (f *fctx) parenBang(n *parser.CallNode) (expr, bool) {
	w := parenWrite(n.Receiver)
	if w == nil || !f.lenient() || !strings.HasSuffix(n.Name, "!") {
		return expr{}, false
	}
	f.emitExprStmt(w, f.genExpr(w, nil))
	bang := *n
	bang.Receiver = &parser.LocalVariableReadNode{Location: w.NameLoc, Name: w.Name, Depth: w.Depth}
	return f.genStringBang(&bang)
}

// parenWrite is `(x = v)`'s write, or nil.
func parenWrite(n parser.Node) *parser.LocalVariableWriteNode {
	pn, ok := n.(*parser.ParenthesesNode)
	if !ok {
		return nil
	}
	st, ok := pn.Body.(*parser.StatementsNode)
	if !ok || len(st.Body) != 1 {
		return nil
	}
	w, _ := st.Body[0].(*parser.LocalVariableWriteNode)
	return w
}

// genStringBang compiles a String bang method (and <<) on a local in a gem file as a rebinding of that local
// (decision 172): Strings stay values (decision 136), and `x.sub!(a, b)` is `x = x.sub(a, b)`, nil when unchanged.
func (f *fctx) genStringBang(n *parser.CallNode) (expr, bool) {
	if e, ok := f.genOutBuffer(n); ok {
		return e, true
	}
	if e, ok := f.parenBang(n); ok {
		return e, true
	}
	if e, ok := f.freshAppend(n); ok {
		return e, true
	}
	if inner, ok := n.Receiver.(*parser.CallNode); ok && n.Name == "<<" && inner.Name == "<<" { // `x << a << b`: x << a, then x << b
		root := inner
		for r, ok := root.Receiver.(*parser.CallNode); ok && r.Name == "<<"; r, ok = r.Receiver.(*parser.CallNode) {
			root = r
		}
		lv, ok := root.Receiver.(*parser.LocalVariableReadNode)
		if !ok {
			return expr{}, false
		}
		if _, ok := f.genStringBang(inner); !ok {
			return expr{}, false
		}
		outer := *n
		outer.Receiver = lv
		return f.genStringBang(&outer)
	}
	lv, write, ok := f.stringBangTarget(n, n.Receiver)
	if !ok || !f.lenient() || !strings.HasSuffix(n.Name, "!") && n.Name != "<<" && n.Name != "concat" && n.Name != "replace" && n.Name != "clear" && n.Name != "[]=" {
		return expr{}, false
	}
	if n.IsSAFE_NAVIGATION() { // kramdown's `second&.gsub!(re, s)`: nothing when the local is nil
		if _, local := lv.(*parser.LocalVariableReadNode); local {
			return f.safeStringBang(n, lv, write)
		}
		return expr{}, false
	}
	return f.stringBangOn(n, lv, write)
}

// safeStringBang is `x&.sub!(...)` on a local: the edit when x holds a value, nil otherwise.
func (f *fctx) safeStringBang(n *parser.CallNode, lv parser.Node, write func(expr)) (expr, bool) {
	plain := &parser.CallNode{Location: n.Location, Receiver: n.Receiver, Name: n.Name, MessageLoc: n.MessageLoc, Arguments: n.Arguments, Block: n.Block}
	x := f.genExpr(lv, nil)
	res := f.newTmp()
	f.emit("var %s any", res)
	f.emit("if rbUnbox(any(%s)) != nil {", x.code)
	f.indent++
	saved := f.enterBlock()
	if lr, ok := lv.(*parser.LocalVariableReadNode); ok && isOpt(x.typ) {
		if v := f.visibleLocal(lr.Name); v != nil {
			f.applyNarrow([]narrowInfo{{local: v, typ: stripOpt(x.typ)}}) // `if x`: non-nil inside
		}
	}
	e, ok := f.stringBangOn(plain, lv, write)
	if !ok {
		e = f.genExpr(plain, nil)
	}
	if e.stmt || isVoid(e.typ) {
		f.emit("%s", e.code)
	} else {
		f.emit("%s = %s", res, f.coerce(n, e, TAny{}))
	}
	f.leaveBlock(saved)
	f.indent--
	f.emit("}")
	f.emit("_ = %s", res)
	return expr{code: res, typ: TAny{}}, true
}

// stringBangOn is genStringBang on the local lv, whose String write rebinds.
func (f *fctx) stringBangOn(n *parser.CallNode, lv parser.Node, write func(expr)) (expr, bool) {
	t := f.probeType(lv, nil)
	str := f.cls("String")
	if _, local := lv.(*parser.LocalVariableReadNode); isAny(t) && !f.noStrIndexSet {
		switch {
		case n.Name == "[]=" && local && f.stringEditArgs(callArgs(n)):
			return f.untypedIndexSet(n, lv, write), true
		case n.Name == "<<" || n.Name == "concat" || strings.HasSuffix(n.Name, "!") && f.c.classes["String"].lookup(strings.TrimSuffix(n.Name, "!")) != nil:
			// kramdown's adapt_source: `source = source.encode('UTF-8'); source.gsub!(...); source << "\n"` (decision 330)
			return f.untypedStringEdit(n, lv, func(sv string) (expr, bool) {
				if n.Name == "<<" || n.Name == "concat" { // its value is the local re-read after the write: lv itself
					return f.stringAppendBang(n, lv, callArgs(n), func() string { return sv }, write)
				}
				return f.stringBangOn(n, &exprNode{Node: lv, e: expr{code: sv, typ: str}}, write)
			}), true
		}
	}
	u, isUnion := stripOpt(t).(TUnion)
	if !typeEq(t, str) && !typeEq(t, TOpt{Elem: str}) && !(isUnion && slices.ContainsFunc(u.Members, func(m Type) bool { return typeEq(m, str) })) {
		return expr{}, false
	}
	cur := func() string { // the local's String; anything else raises NoMethodError, as the bang call would in MRI
		x := f.genExpr(lv, nil)
		switch {
		case typeEq(x.typ, str):
			return x.code
		case typeEq(x.typ, TOpt{Elem: str}):
			return fmt.Sprintf("rbStrRecv(%s, %q)", x.code, n.Name)
		}
		return fmt.Sprintf("rbStrRecvAny(%s, %q)", x.code, n.Name)
	}
	args := callArgs(n)
	switch n.Name {
	case "<<", "concat":
		return f.stringAppendBang(n, lv, args, cur, write)
	case "replace", "clear": // rackup's Stream::Reader#read refills its buffer
		return f.stringReplaceBang(n, args, cur, write)
	case "slice!":
		return f.stringSliceBang(n, args, cur, write)
	case "[]=":
		return f.stringIndexSet(n, args, cur(), write)
	}
	plain := strings.TrimSuffix(n.Name, "!")
	if f.c.classes["String"].lookup(plain) == nil {
		return expr{}, false
	}
	before, after := f.newTmp(), f.newTmp()
	f.emit("%s := %s", before, cur())
	next := f.genExpr(&parser.CallNode{Location: n.Location, Receiver: &exprNode{Node: lv, e: expr{code: before, typ: str}},
		Name: plain, MessageLoc: n.MessageLoc, Arguments: n.Arguments, Block: n.Block}, str) // `gsub!(re) { }` keeps its block
	f.emit("%s := %s", after, f.coerce(n, next, str))
	write(expr{code: after, typ: str})
	return expr{code: fmt.Sprintf("rbStrChanged(%s, %s)", before, after), typ: TOpt{Elem: str}}, true
}

// stringReplaceBang is a gem's `s.replace(t)` / `s.clear`: s rebound to t, or to "".
func (f *fctx) stringReplaceBang(n *parser.CallNode, args []parser.Node, cur func() string, write func(expr)) (expr, bool) {
	str := f.cls("String")
	if len(args) != map[string]int{"replace": 1, "clear": 0}[n.Name] {
		return expr{}, false
	}
	with := `""`
	if n.Name == "replace" {
		with = f.coerce(args[0], f.genExpr(args[0], str), str)
	}
	tmp := f.newTmp()
	f.emit("%s := rbStrReplace(%s, %s)", tmp, cur(), with) // cur: a non-String receiver raises as MRI's would
	write(expr{code: tmp, typ: str})
	return expr{code: tmp, typ: str}, true
}

// untypedIndexSet is a gem's `s[i] = v` on an untyped local: a String is rebound, anything else (a parsed JSON Hash)
// takes []= as before.
func (f *fctx) untypedIndexSet(n *parser.CallNode, lv parser.Node, write func(expr)) expr {
	return f.untypedStringEdit(n, lv, func(sv string) (expr, bool) { return f.stringIndexSet(n, callArgs(n), sv, write) })
}

// untypedStringEdit is a gem's String edit on an untyped local: onString (the edit rebinding the local) when it holds a
// String, the call as before on anything else.
func (f *fctx) untypedStringEdit(n *parser.CallNode, lv parser.Node, onString func(sv string) (expr, bool)) expr {
	x := f.genExpr(lv, nil)
	res, sv := f.newTmp(), f.newTmp()
	f.emit("var %s any", res)
	f.emit("if %s, ok := %s.(String); ok {", sv, x.code)
	f.indent++
	f.emit("_ = %s // an append chain's edit may read the local again instead", sv)
	saved := f.enterBlock()
	var r expr
	ok := false
	err := f.tryClean(func() { r, ok = onString(sv) })
	if err != nil { // `@tree.children << el`: no String takes an Element
		ok = false
	}
	if ok {
		f.emit("%s = %s", res, f.coerce(n, r, TAny{}))
	} else {
		f.emit("panic(rbNoMethod(%q, %s, false))", n.Name, sv)
	}
	f.leaveBlock(saved)
	f.indent--
	f.emit("} else {")
	f.indent++
	saved = f.enterBlock()
	f.noStrIndexSet = true
	e := f.genExpr(n, nil)
	f.noStrIndexSet = false
	if e.stmt || isVoid(e.typ) {
		f.emit("%s", e.code)
	} else {
		f.emit("%s = %s", res, f.coerce(n, e, TAny{}))
	}
	f.leaveBlock(saved)
	f.indent--
	f.emit("}")
	f.emit("_ = %s // `x << a << b`: the chain reads x again, not this", res)
	return expr{code: res, typ: TAny{}}
}

// stringEditArgs reports `[]=` arguments only a String edit takes: Integer or Range indexes and a String value (a
// Hash's `h[file] = libs` keeps its own path).
func (f *fctx) stringEditArgs(args []parser.Node) bool {
	if len(args) < 2 || len(args) > 3 {
		return false
	}
	for i, a := range args {
		t := f.probeType(a, nil)
		if i == len(args)-1 {
			return isClass(t, "String")
		}
		if !isClass(t, "Integer") && !isClass(t, "Range") {
			return false
		}
	}
	return false
}

// stringIndexSet is a gem's `s[range] = v` / `s[i, n] = v` / `s[i] = v` (patchelf's `new_dynstr[i..j] = "..."`):
// s rebound to the edited String, the value v (decision 324).
func (f *fctx) stringIndexSet(n *parser.CallNode, args []parser.Node, cur string, write func(expr)) (expr, bool) {
	str := f.cls("String")
	if len(args) < 2 || len(args) > 3 {
		return expr{}, false
	}
	var at expr
	f.probe(func() { at = f.genExpr(args[0], nil) })
	helper, idx := "__with_span", slices.Clone(args[:len(args)-1])
	switch {
	case isClass(at.typ, "Range") && len(args) == 2:
		helper = "__with_range"
	case len(args) == 2:
		idx = append(idx, &exprNode{e: expr{code: "Integer(1)", typ: f.cls("Integer"), lit: true}})
	}
	val := f.genExpr(args[len(args)-1], str)
	if !typeEq(val.typ, str) { // `h[k] = obj` of an untyped h that is a Hash at run time: a String only if it is one
		val = expr{code: f.coerce(args[len(args)-1], val, TAny{}), typ: TAny{}}
	}
	vt, s0 := f.newTmp(), f.newTmp()
	f.emit("%s := String(%s)", vt, f.coerce(args[len(args)-1], val, str))
	f.emit("%s := String(%s)", s0, cur)
	write(f.genMethodCall(n, expr{code: s0, typ: str}, helper, append(idx, &exprNode{e: expr{code: vt, typ: str}}), nil))
	return expr{code: vt, typ: str}, true
}

// stringAppendBang is a gem's `s << x` or `s.concat(a, b)` (decision 172, 233): s rebound to the joined String.
func (f *fctx) stringAppendBang(n *parser.CallNode, lv parser.Node, args []parser.Node, cur func() string, write func(expr)) (expr, bool) {
	str := f.cls("String")
	if len(args) == 0 || n.Name == "<<" && len(args) != 1 {
		return expr{}, false
	}
	acc := cur()
	for _, arg := range args {
		a := f.genExpr(arg, nil)
		code := f.coerce(arg, a, str)
		if typeEq(a.typ, TOpt{Elem: str}) { // `s << nil` raises in MRI too
			code = "rbMustStr(" + a.code + ")"
		}
		acc = "String.Op_plus(" + acc + ", " + code + ")"
	}
	write(expr{code: acc, typ: str})
	v := f.genExpr(lv, nil)
	return expr{code: v.code, typ: v.typ, stmt: true, done: true}, true
}

// stringSliceBang is a gem's `s.slice!(i, n)` or `s.slice!(range)` (decision 172, 233): the part taken, s rebound
// to the rest.
func (f *fctx) stringSliceBang(n *parser.CallNode, args []parser.Node, cur func() string, write func(expr)) (expr, bool) {
	str := f.cls("String")
	if len(args) == 1 { // `buf.slice!(0..i)`: a Range's part, as String#[] takes it
		var at expr
		f.probe(func() { at = f.genExpr(args[0], nil) })
		if !isClass(at.typ, "Range") {
			return expr{}, false
		}
		s0, got := f.newTmp(), f.newTmp()
		f.emit("%s := %s", s0, cur())
		g := f.genMethodCall(n, expr{code: s0, typ: str}, "[]", args, nil)
		f.emit("%s := %s", got, g.code)
		rest := f.genMethodCall(n, expr{code: s0, typ: str}, "__rest_of_slice", args, nil)
		write(rest)
		return expr{code: got, typ: g.typ}, true
	}
	if len(args) != 2 {
		return expr{}, false
	}
	i := f.coerce(args[0], f.genExpr(args[0], f.cls("Integer")), f.cls("Integer"))
	k := f.coerce(args[1], f.genExpr(args[1], f.cls("Integer")), f.cls("Integer"))
	got, rest := f.newTmp(), f.newTmp()
	f.emit("%s, %s := rbStrSliceBang(%s, %s, %s)", got, rest, cur(), i, k)
	write(expr{code: rest, typ: str})
	return expr{code: got, typ: TOpt{Elem: str}}, true
}

// genOutBuffer is a gem's `io.read_nonblock(n, buf)` (readpartial, read, sysread): MRI fills buf in place; rb2go's
// Strings do not change, so buf is rebound to what was read, which is also the call's value (decisions 172, 233).
func (f *fctx) genOutBuffer(n *parser.CallNode) (expr, bool) {
	args := callArgs(n)
	if !f.lenient() || len(args) != 2 || n.Receiver == nil || n.Block != nil {
		return expr{}, false
	}
	switch n.Name {
	case "read_nonblock", "readpartial", "read", "sysread":
	default:
		return expr{}, false
	}
	buf, write, ok := f.stringBangTarget(n, args[1])
	if !ok {
		return expr{}, false
	}
	if _, local := buf.(*parser.LocalVariableReadNode); local { // a parameter rebound before any read (staticcheck SA4009)
		f.emit("_ = %s", f.genExpr(buf, nil).code)
	}
	call := *n
	call.Arguments = &parser.ArgumentsNode{Location: n.Arguments.Location, Arguments: args[:1]}
	got := f.genExpr(&call, nil)
	tmp := f.newTmp()
	f.emit("%s := %s", tmp, got.code)
	write(expr{code: tmp, typ: got.typ})
	if _, local := buf.(*parser.LocalVariableReadNode); local { // read after, so the rebinding counts as used (SA4006)
		f.emit("_ = %s", f.genExpr(buf, nil).code)
	}
	return expr{code: tmp, typ: got.typ}, true
}

// stringBangTarget is what a gem's String mutation rebinds (decision 172): a local, or (decision 233) an ivar
// (`@buffer.clear` in Homebrew's ReadlineNonblock), with the write that stores the new String.
func (f *fctx) stringBangTarget(n parser.Node, recv parser.Node) (parser.Node, func(expr), bool) {
	switch t := recv.(type) {
	case *parser.LocalVariableReadNode:
		return t, func(e expr) {
			f.assignLocal(n, t.Name, e, nil)
			f.narrowString(t.Name)
			if v := f.visibleLocal(t.Name); v != nil && f.lenient() { // `value.gsub!(re, ' ')` as a block's last use: MRI's edit is in place, ours a dead store
				f.emit("_ = %s", v.goName)
				if src := f.aliasSrc[f.localKey(t.Name)]; src != nil { // liquid's `previous_token = @nodelist.last; previous_token.rstrip!`
					if at, ok := f.elementTarget(src); ok {
						set := &parser.CallNode{Location: src.Location, Receiver: src.Receiver, Name: "[]=", Arguments: &parser.ArgumentsNode{
							Location: src.Location, Arguments: []parser.Node{at, &exprNode{Node: t, e: expr{code: v.goName, typ: v.typ}}}}}
						f.emitExprStmt(src, f.genExpr(set, nil))
					}
				}
			}
		}, true
	case *parser.InstanceVariableReadNode:
		return t, func(e expr) {
			iv := f.ivar(t, t.Name, e.typ)
			f.emit("%s = %s", f.ivarCode(iv), f.coerce(n, e, iv.Type))
			f.markIvar(iv)
		}, true
	case *parser.CallNode:
		if f.freshString(t) { // `path.basename.to_s.delete_prefix!(x)`: nothing else sees the string, so only the value matters
			return t, func(expr) {}, true
		}
		if at, ok := f.elementTarget(t); ok { // kramdown's `item.value.last << result`: item.value[-1] = item.value[-1] + result
			return t, func(e expr) {
				set := &parser.CallNode{Location: t.Location, Receiver: t.Receiver, Name: "[]=", MessageLoc: t.MessageLoc,
					Arguments: &parser.ArgumentsNode{Location: t.Location, Arguments: []parser.Node{at, &exprNode{Node: t, e: e}}}}
				f.emitExprStmt(t, f.genExpr(set, nil))
			}, true
		}
		if f.lenient() && f.attrTarget(t) { // kramdown's `item.value.sub!(re) { ... }`: item.value = item.value.sub(...) (decision 330)
			return t, func(e expr) {
				set := &parser.CallNode{Location: t.Location, Receiver: t.Receiver, Name: t.Name + "=", MessageLoc: t.MessageLoc,
					Arguments: &parser.ArgumentsNode{Location: t.Location, Arguments: []parser.Node{&exprNode{Node: t, e: e}}}}
				f.emitExprStmt(t, f.genExpr(set, nil))
			}, true
		}
	}
	return nil, nil, false
}

// elementTarget is a gem's `r.last`, `r.first` or `r[i]` (i a literal or a local) on a re-readable r: an Array slot a
// String edit writes back, so the Array sees it as MRI's shared String would; another reference to that String does
// not (decision 136). at is the slot's index.
func (f *fctx) elementTarget(t *parser.CallNode) (at parser.Node, ok bool) {
	if !f.lenient() || t.Block != nil || !f.rereadable(t.Receiver) {
		return nil, false
	}
	args := callArgs(t)
	switch {
	case t.Name == "last" && len(args) == 0:
		at = &exprNode{Node: t, e: expr{code: "Integer(-1)", typ: f.cls("Integer"), lit: true}}
	case t.Name == "first" && len(args) == 0:
		at = &exprNode{Node: t, e: expr{code: "Integer(0)", typ: f.cls("Integer"), lit: true}}
	case t.Name == "[]" && len(args) == 1:
		switch args[0].(type) {
		case *parser.IntegerNode, *parser.LocalVariableReadNode:
			at = args[0]
		default:
			return nil, false
		}
	default:
		return nil, false
	}
	rt := stripOpt(f.probeType(t.Receiver, nil))
	return at, isAny(rt) || isClass(rt, "Array")
}

// rereadable is an expression evaluated again for the write-back without a side effect: a local, an ivar, self, or
// an attribute reader of one.
func (f *fctx) rereadable(n parser.Node) bool {
	switch n := n.(type) {
	case *parser.LocalVariableReadNode, *parser.InstanceVariableReadNode, *parser.SelfNode:
		return true
	case *parser.CallNode:
		if (n.Name == "first" || n.Name == "last") && n.Arguments == nil && n.Block == nil { // `kids.first.value << "\n"`
			return f.rereadable(n.Receiver)
		}
		return f.attrTarget(n)
	}
	return false
}

// attrTarget is `r.name` with r a local, ivar or self whose class has a `name=` writer: an attribute a String edit
// can rebind, reading r again (no side effect) to write it.
func (f *fctx) attrTarget(t *parser.CallNode) bool {
	if t.Block != nil || len(callArgs(t)) > 0 || t.Receiver == nil || !isIdentMethod(t.Name) {
		return false
	}
	if !f.rereadable(t.Receiver) {
		return false
	}
	rt := stripOpt(f.probeType(t.Receiver, nil))
	if isAny(rt) { // kramdown's `last.value << text` on an untyped tree's child: written back by a dynamic name= call
		return slices.ContainsFunc(f.c.classList, func(k *Class) bool { return k.lookup(t.Name+"=") != nil && k.lookup(t.Name) != nil })
	}
	cls := classOf(rt)
	return cls != nil && cls.lookup(t.Name+"=") != nil
}

// freshAppend is a gem's `' ' * indent << "<" << tag << ">"`: appends onto a String nothing else holds (an operator's
// or interpolation's result), so the chain's value is the concatenation (decision 330). Not for an Integer operand
// (`s << 65` appends a codepoint).
func (f *fctx) freshAppend(n *parser.CallNode) (expr, bool) {
	if n.Name != "<<" || !f.lenient() || n.Block != nil || len(callArgs(n)) != 1 {
		return expr{}, false
	}
	str := f.cls("String")
	var acc expr
	switch r := n.Receiver.(type) {
	case *parser.CallNode:
		if r.Name == "<<" {
			inner, ok := f.freshAppend(r)
			if !ok {
				return expr{}, false
			}
			acc = inner
			break
		}
		if r.Name != "*" && r.Name != "+" && r.Name != "%" && !f.freshString(r) || !isClass(f.probeType(r, nil), "String") {
			return expr{}, false
		}
		acc = f.genExpr(r, str)
	case *parser.InterpolatedStringNode:
		acc = f.genExpr(r, str)
	default:
		return expr{}, false
	}
	arg := callArgs(n)[0]
	at := f.probeType(arg, nil)
	if !isClass(at, "String") && !isAny(at) {
		return expr{}, false
	}
	a := f.genExpr(arg, str)
	return expr{code: "String.Op_plus(" + acc.code + ", " + f.coerce(arg, a, str) + ")", typ: str}, true
}

// freshString: call returns a String no other reference holds, as MRI's do: `x.to_s` on a non-String, `s.dup`, `+s`.
func (f *fctx) freshString(call *parser.CallNode) bool {
	if call.Receiver == nil || call.Block != nil || len(callArgs(call)) > 0 {
		return false
	}
	switch call.Name {
	case "dup", "+@":
		return true
	case "to_s":
		rt := f.probeType(call.Receiver, nil)
		_, cls := rt.(TClass)
		return cls && !isClass(rt, "String")
	}
	return false
}

// readsLastMatch reports whether body reads $~, $& or $1..$9, so its frame keeps the last match (decision 171).
func readsLastMatch(body parser.Node) bool {
	return anyNode(body, func(n parser.Node) bool {
		switch n := n.(type) {
		case *parser.NumberedReferenceReadNode, *parser.BackReferenceReadNode:
			return true
		case *parser.GlobalVariableReadNode:
			return n.Name == "$~" || n.Name == "$LAST_MATCH_INFO"
		case *parser.CallNode: // Regexp.last_match reads the frame's $~ too
			return isRegexpConst(n.Receiver) && n.Name == "last_match"
		}
		return false
	})
}

// genMatchRef is $1..$9 or $&: a group of the frame's last match (decision 171).
func (f *fctx) genMatchRef(n parser.Node) expr {
	group := 0
	switch n := n.(type) {
	case *parser.NumberedReferenceReadNode:
		group = int(n.Number)
	case *parser.BackReferenceReadNode:
		if n.Name != "$&" {
			f.errorf(n, "%s is unsupported; $~, $& and $1..$9 are (decision 171)", n.Name)
		}
	}
	return expr{code: fmt.Sprintf("rbLMGroup(%s, %d)", f.lastMatchVar(n), group), typ: TOpt{Elem: f.cls("String")}}
}
