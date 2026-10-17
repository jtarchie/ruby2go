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
	if !ok || !f.lenient() || !strings.HasSuffix(n.Name, "!") && n.Name != "<<" && n.Name != "concat" && n.Name != "replace" && n.Name != "clear" {
		return expr{}, false
	}
	t := f.probeType(lv, nil)
	str := f.cls("String")
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
	case "slice!":
		return f.stringSliceBang(n, args, cur, write)
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
		rest := f.genMethodCall(n, expr{code: s0, typ: str}, "__slice_rest", args, nil)
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
	}
	return nil, nil, false
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
			r, ok := n.Receiver.(*parser.ConstantReadNode)
			return ok && r.Name == "Regexp" && n.Name == "last_match"
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
