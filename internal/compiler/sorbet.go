package compiler

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/danielgatis/go-ruby-prism/parser"
)

// Sorbet's types are run-time calls; rewriting them into `#:` annotations at parse time keeps the rest of the compiler Sorbet-blind (decision 196).

// sorbetModifiers are sig chain links that only Sorbet's checker reads.
var sorbetModifiers = map[string]bool{
	"override": true, "overridable": true, "abstract": true, "final": true, "checked": true, "on_failure": true,
	"allow_incompatible": true,
}

// sorbetDirectives are class-body statements with no run-time meaning in a closed world.
var sorbetDirectives = map[string]bool{"abstract!": true, "interface!": true, "final!": true, "sealed!": true}

type sorbetErr struct {
	n   parser.Node
	msg string
}

// lowerSorbet rewrites f's Sorbet forms in place; it reports the first unsupported one as file:line.
func (f *File) lowerSorbet() (err error) {
	if !bytes.Contains(f.Src, []byte("sig")) && !bytes.Contains(f.Src, []byte("T.")) {
		return nil
	}
	defer func() {
		if r := recover(); r != nil {
			se, ok := r.(sorbetErr)
			if !ok {
				panic(r)
			}
			err = fmt.Errorf("%s:%d: %s", f.Name, f.line(se.n.GetLocation().StartOffset), se.msg)
		}
	}()
	f.sorbetWalk(f.Root)
	return nil
}

func sorbetFail(n parser.Node, format string, args ...any) {
	panic(sorbetErr{n: n, msg: fmt.Sprintf(format, args...)})
}

func (f *File) sorbetWalk(n parser.Node) {
	switch n := n.(type) {
	case nil:
		return
	case *parser.StatementsNode:
		n.Body = f.sorbetStatements(n.Body)
	case *parser.CallNode:
		if isTCall(n, "let") || isTCall(n, "cast") {
			args := callArgs(n)
			if len(args) != 2 {
				sorbetFail(n, "T.%s takes a value and a type", n.Name)
			}
			if f.sorbetCasts == nil {
				f.sorbetCasts = map[*parser.CallNode]string{}
			}
			f.sorbetCasts[n] = sorbetType(args[1])
			f.sorbetWalk(args[0])
			return
		}
	}
	if v := writeValue(n); v != nil {
		if c, ok := (*v).(*parser.CallNode); ok && isTCall(c, "let") && len(callArgs(c)) == 2 {
			ln := f.line(n.GetLocation().StartOffset + n.GetLocation().Length - 1)
			if _, taken := f.trailing[ln]; !taken {
				f.trailing[ln] = sorbetType(callArgs(c)[1])
				*v = callArgs(c)[0]
			}
		}
	}
	for _, ch := range n.CompactChildNodes() {
		f.sorbetWalk(ch)
	}
}

// writeValue is the assigned value's slot of a variable or constant write, which `#:` can annotate.
func writeValue(n parser.Node) *parser.Node {
	switch n := n.(type) {
	case *parser.LocalVariableWriteNode:
		return &n.Value
	case *parser.InstanceVariableWriteNode:
		return &n.Value
	case *parser.InstanceVariableOrWriteNode:
		return &n.Value
	case *parser.ClassVariableWriteNode:
		return &n.Value
	case *parser.ConstantWriteNode:
		return &n.Value
	case *parser.ConstantPathWriteNode:
		return &n.Value
	}
	return nil
}

func (f *File) sorbetStatements(body []parser.Node) []parser.Node {
	out := body[:0:0]
	for i, st := range body {
		c, ok := st.(*parser.CallNode)
		if ok && isTCall(c, "bind") { // `T.bind(self, X)`: a closed world already knows self's class
			continue
		}
		if !ok || c.Receiver != nil {
			out = append(out, st)
			continue
		}
		switch {
		case c.Name == "sig" && c.Block != nil:
			if i+1 == len(body) {
				sorbetFail(c, "sig with no method after it")
			}
			f.attachSig(c, body[i+1])
		case (c.Name == "extend" || c.Name == "include") && isSorbetModuleArg(c):
		case sorbetDirectives[c.Name] && c.Arguments == nil:
		default:
			out = append(out, st)
		}
	}
	return out
}

func isSorbetModuleArg(c *parser.CallNode) bool {
	args := callArgs(c)
	if len(args) != 1 {
		return false
	}
	p, ok := args[0].(*parser.ConstantPathNode)
	if !ok {
		return false
	}
	return strings.TrimPrefix(constName(p.Parent), "::") == "T" && p.Name != nil && (*p.Name == "Sig" || *p.Name == "Helpers")
}

func constName(n parser.Node) string {
	switch n := n.(type) {
	case *parser.ConstantReadNode:
		return n.Name
	case *parser.ConstantPathNode:
		if n.Name == nil {
			return ""
		}
		if n.Parent == nil {
			return "::" + *n.Name
		}
		return constName(n.Parent) + "::" + *n.Name
	}
	return ""
}

func isTCall(c *parser.CallNode, name string) bool {
	r := constName(c.Receiver)
	return (r == "T" || r == "::T") && c.Name == name
}

// sigChain reads `params(...).returns(R)` (or `.void`) into its parts.
type sigChain struct {
	params   map[string]parser.Node
	order    []string
	ret      string
	typeVars []string
}

func readSig(sig *parser.CallNode) sigChain {
	blk, ok := sig.Block.(*parser.BlockNode)
	if !ok {
		sorbetFail(sig, "sig takes a block")
	}
	body, ok := blk.Body.(*parser.StatementsNode)
	if !ok || len(body.Body) != 1 {
		sorbetFail(sig, "sig's block must be one params/returns chain")
	}
	var s sigChain
	for n := body.Body[0]; n != nil; {
		c, ok := n.(*parser.CallNode)
		if !ok {
			sorbetFail(n, "unsupported sig expression")
		}
		switch {
		case c.Name == "returns":
			s.ret = sorbetType(oneArg(c))
		case c.Name == "void":
			s.ret = "void"
		case c.Name == "params":
			s.params = map[string]parser.Node{}
			for _, a := range callArgs(c) {
				kh, ok := a.(*parser.KeywordHashNode)
				if !ok {
					sorbetFail(c, "params takes name: Type pairs")
				}
				for _, el := range kh.Elements {
					as, ok := el.(*parser.AssocNode)
					sym, sok := as.Key.(*parser.SymbolNode)
					if !ok || !sok {
						sorbetFail(el, "params takes name: Type pairs")
					}
					name := sym.Unescaped.Value
					s.params[name] = as.Value
					s.order = append(s.order, name)
				}
			}
		case c.Name == "type_parameters":
			for _, a := range callArgs(c) {
				sym, ok := a.(*parser.SymbolNode)
				if !ok {
					sorbetFail(a, "type_parameters takes symbols")
				}
				s.typeVars = append(s.typeVars, sym.Unescaped.Value)
			}
		case sorbetModifiers[c.Name]:
		default:
			sorbetFail(c, "unsupported sig call %s", c.Name)
		}
		n = c.Receiver
	}
	if s.ret == "" {
		sorbetFail(sig, "sig needs returns(...) or void")
	}
	return s
}

func oneArg(c *parser.CallNode) parser.Node {
	args := callArgs(c)
	if len(args) != 1 {
		sorbetFail(c, "%s takes one type", c.Name)
	}
	return args[0]
}

// attachSig turns sig into an annotation on target: a def's `#:` method type, or an attr_*'s trailing `#: T`.
func (f *File) attachSig(sig *parser.CallNode, target parser.Node) {
	s := readSig(sig)
	ln := f.line(target.GetLocation().StartOffset)
	switch t := target.(type) {
	case *parser.DefNode:
		f.leading[ln] = append(f.leading[ln], comment{line: ln, text: ": " + s.methodType(t)})
		return
	case *parser.CallNode:
		if d := wrappedDef(t); d != nil { // `private def x`
			f.leading[ln] = append(f.leading[ln], comment{line: ln, text: ": " + s.methodType(d)})
			return
		}
		switch t.Name {
		case "attr_reader", "attr_accessor":
			f.trailing[ln] = s.ret
			return
		case "attr_writer":
			if len(s.order) != 1 {
				sorbetFail(sig, "an attr_writer sig takes one param")
			}
			f.trailing[ln] = sorbetType(s.params[s.order[0]])
			return
		}
	}
	sorbetFail(sig, "sig must precede a def or attr_*")
}

func wrappedDef(c *parser.CallNode) *parser.DefNode {
	if c.Receiver != nil {
		return nil
	}
	args := callArgs(c)
	if len(args) != 1 {
		return nil
	}
	d, _ := args[0].(*parser.DefNode)
	return d
}

func (s sigChain) methodType(d *parser.DefNode) string {
	used := map[string]bool{}
	typeOf := func(n parser.Node, name string) string {
		t, ok := s.params[name]
		if !ok {
			sorbetFail(n, "sig has no type for parameter %s of %s", name, d.Name)
		}
		used[name] = true
		return sorbetType(t)
	}
	var parts []string
	block := ""
	if p := d.Parameters; p != nil {
		for _, r := range append(append([]parser.Node{}, p.Requireds...), p.Posts...) {
			rp, ok := r.(*parser.RequiredParameterNode)
			if !ok {
				sorbetFail(r, "a destructuring parameter cannot take a sig")
			}
			parts = append(parts, typeOf(r, rp.Name))
		}
		for _, o := range p.Optionals {
			parts = append(parts, "?"+typeOf(o, o.(*parser.OptionalParameterNode).Name))
		}
		if r, ok := p.Rest.(*parser.RestParameterNode); ok && r.Name != nil {
			parts = append(parts, "*"+typeOf(r, *r.Name))
		}
		for _, k := range p.Keywords {
			switch k := k.(type) {
			case *parser.RequiredKeywordParameterNode:
				parts = append(parts, k.Name+": "+typeOf(k, k.Name))
			case *parser.OptionalKeywordParameterNode:
				parts = append(parts, "?"+k.Name+": "+typeOf(k, k.Name))
			}
		}
		if r, ok := p.KeywordRest.(*parser.KeywordRestParameterNode); ok && r.Name != nil {
			parts = append(parts, "**"+typeOf(r, *r.Name))
		}
		if b := p.Block; b != nil && b.Name != nil {
			block = sorbetBlock(s.params[*b.Name], b)
			used[*b.Name] = true
		}
	}
	for _, name := range s.order {
		if !used[name] {
			sorbetFail(s.params[name], "sig names parameter %s, which %s does not have", name, d.Name)
		}
	}
	head := ""
	if len(s.typeVars) > 0 {
		head = "[" + strings.Join(s.typeVars, ", ") + "] "
	}
	return head + "(" + strings.Join(parts, ", ") + ")" + block + " -> " + s.ret
}

// sorbetBlock is a `&blk` param's type as an RBS block: `T.proc...` or `T.nilable(T.proc...)`.
func sorbetBlock(t parser.Node, b parser.Node) string {
	if t == nil {
		sorbetFail(b, "sig has no type for the block parameter")
	}
	opt := ""
	if c, ok := t.(*parser.CallNode); ok && isTCall(c, "nilable") {
		opt, t = "?", oneArg(c)
	}
	if c, ok := t.(*parser.CallNode); ok && isTCall(c, "untyped") || strings.TrimPrefix(constName(t), "::") == "Proc" {
		return " " + opt + "{ (*untyped) -> untyped }"
	}
	fn := sorbetType(t)
	if !strings.HasPrefix(fn, "^") {
		sorbetFail(t, "a block parameter's type must be T.proc...")
	}
	return " " + opt + "{ " + fn[1:] + " }"
}

// sorbetGenerics maps T::X[...] to its RBS class.
var sorbetGenerics = map[string]string{
	"T::Array": "Array", "T::Hash": "Hash", "T::Set": "Set", "T::Range": "Range", "T::Enumerable": "Enumerable",
	"T::Enumerator": "Enumerator", "T::Enumerator::Lazy": "Enumerator::Lazy",
}

// sorbetType translates a Sorbet type expression to RBS text.
func sorbetType(n parser.Node) string {
	switch n := n.(type) {
	case *parser.ConstantReadNode, *parser.ConstantPathNode:
		name := constName(n)
		if strings.HasPrefix(name, "::T::") {
			name = name[2:]
		}
		switch name {
		case "T::Boolean":
			return "bool"
		case "NilClass":
			return "nil"
		}
		if g, ok := sorbetGenerics[name]; ok {
			return g + "[untyped]"
		}
		return name
	case *parser.NilNode:
		return "nil"
	case *parser.ArrayNode:
		var els []string
		for _, e := range n.Elements {
			els = append(els, sorbetType(e))
		}
		return "[" + strings.Join(els, ", ") + "]"
	case *parser.HashNode:
		var els []string
		for _, e := range n.Elements {
			as, ok := e.(*parser.AssocNode)
			sym, sok := as.Key.(*parser.SymbolNode)
			if !ok || !sok {
				sorbetFail(e, "a shape type takes symbol keys")
			}
			els = append(els, sym.Unescaped.Value+": "+sorbetType(as.Value))
		}
		return "{ " + strings.Join(els, ", ") + " }"
	case *parser.CallNode:
		return sorbetCallType(n)
	}
	sorbetFail(n, "unsupported Sorbet type")
	return ""
}

func sorbetCallType(n *parser.CallNode) string {
	if n.Name == "[]" {
		base := strings.TrimPrefix(constName(n.Receiver), "::")
		if g, ok := sorbetGenerics[base]; ok {
			base = g
		}
		args := make([]string, 0, len(callArgs(n)))
		for _, a := range callArgs(n) {
			args = append(args, sorbetType(a))
		}
		return base + "[" + strings.Join(args, ", ") + "]"
	}
	if fn, ok := sorbetProc(n); ok {
		return fn
	}
	if !isTCall(n, n.Name) {
		sorbetFail(n, "unsupported Sorbet type")
	}
	switch n.Name {
	case "nilable":
		t := sorbetType(oneArg(n))
		if strings.Contains(t, "|") || strings.HasPrefix(t, "^") {
			t = "(" + t + ")"
		}
		return t + "?"
	case "any":
		var arms []string
		for _, a := range callArgs(n) {
			arms = append(arms, sorbetType(a))
		}
		return "(" + strings.Join(arms, " | ") + ")"
	case "untyped", "anything":
		return "untyped"
	case "noreturn":
		return "bot"
	case "self_type":
		return "self"
	case "attached_class":
		return "instance"
	case "class_of":
		return "singleton(" + sorbetType(oneArg(n)) + ")"
	case "type_parameter":
		sym, ok := oneArg(n).(*parser.SymbolNode)
		if !ok {
			sorbetFail(n, "T.type_parameter takes a symbol")
		}
		return sym.Unescaped.Value
	}
	sorbetFail(n, "unsupported Sorbet type T.%s", n.Name)
	return ""
}

// sorbetProc reads `T.proc[.bind(S)][.params(...)].returns(R)` / `.void` into `^(A, B) [self: S] -> R`.
func sorbetProc(n *parser.CallNode) (string, bool) {
	var params []string
	ret, self := "", ""
	for c := n; ; {
		switch {
		case isTCall(c, "proc"):
			if ret == "" {
				sorbetFail(n, "T.proc needs returns(...) or void")
			}
			if self != "" {
				self = " [self: " + self + "]"
			}
			return "^(" + strings.Join(params, ", ") + ")" + self + " -> " + ret, true
		case c.Name == "returns":
			ret = sorbetType(oneArg(c))
		case c.Name == "void":
			ret = "void"
		case c.Name == "bind":
			self = sorbetType(oneArg(c))
		case c.Name == "params":
			for _, a := range callArgs(c) {
				kh, ok := a.(*parser.KeywordHashNode)
				if !ok {
					return "", false
				}
				for _, el := range kh.Elements {
					as, ok := el.(*parser.AssocNode)
					if !ok {
						return "", false
					}
					params = append(params, sorbetType(as.Value))
				}
			}
		default:
			return "", false
		}
		r, ok := c.Receiver.(*parser.CallNode)
		if !ok {
			return "", false
		}
		c = r
	}
}

// genSorbetCast is `T.let(v, T)` / `T.cast(v, T)` inside an expression: v as T, unchecked, as `#:` is.
func (f *fctx) genSorbetCast(n *parser.CallNode, ann string) expr {
	t := f.parseTypeAnn(n, ann)
	v := f.genExpr(callArgs(n)[0], t)
	return expr{code: f.coerce(n, v, t), typ: t}
}
