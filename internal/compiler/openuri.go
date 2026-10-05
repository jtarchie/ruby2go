package compiler

import (
	"regexp"
	"slices"
	"strings"

	"github.com/danielgatis/go-ruby-prism/parser"
)

// openURIReadMode is MRI's OpenURI.open_uri mode check: "r", "rb", optionally with ":encoding".
var openURIReadMode = regexp.MustCompile(`\Arb?(?:\n?\z|:[^:])`)

// isOpenURIRecv reports whether t is a URI::Generic (or subclass) whose open/read is open-uri's.
func (f *fctx) isOpenURIRecv(t Type, name string) bool {
	tc, ok := t.(TClass)
	g := f.c.classes["URI::Generic"]
	return ok && g != nil && (name == "open" || name == "read") && (tc.C == g || tc.C.isSubclassOf(g))
}

// genOpenURI compiles URI.open (recv nil) and URI::Generic#open/#read: MRI's one options Hash mixes headers (String keys) and options (Symbol keys), so it is split into a Hash[String, String] and OpenOptions.new keywords to type both (decision 134).
func (f *fctx) genOpenURI(n parser.Node, recv *expr, name string, args []parser.Node, block parser.Node) expr {
	if recv == nil {
		if len(args) == 0 {
			f.errorf(n, "URI.open: wrong number of arguments (given 0, expected 1+)")
		}
		var nameE expr
		f.probe(func() { nameE = f.genExpr(args[0], nil) })
		if f.isOpenURIRecv(nameE.typ, "open") {
			r := f.valueOf(f.genExpr(args[0], nil))
			return f.genOpenURI(n, &r, "open", args[1:], block)
		}
	}
	rest := args
	if recv == nil {
		rest = args[1:]
	}
	var mode parser.Node
	if len(rest) > 0 && !isHashLiteral(rest[0]) {
		var e expr
		f.probe(func() { e = f.genExpr(rest[0], nil) })
		if t, ok := e.typ.(TClass); !ok || t.C != f.c.classes["Hash"] {
			if !ok || t.C != f.c.classes["String"] {
				f.errorf(rest[0], "open-uri: the mode must be a String (an Integer mode or permission is not supported)")
			}
			mode, rest = rest[0], rest[1:]
		}
	}
	if mode != nil && name == "read" {
		f.errorf(mode, "URI#read takes only an options Hash")
	}
	if s, ok := mode.(*parser.StringNode); ok && !openURIReadMode.MatchString(s.Unescaped.Value) {
		f.errorf(mode, "open-uri: mode %q: an open-uri resource is read only; use File.open to write a local file", s.Unescaped.Value)
	}
	loc := n.GetLocation()
	var headers parser.Node = &parser.HashNode{Location: loc}
	var kws []parser.Node
	if len(rest) > 0 {
		var els []parser.Node
		switch h := rest[0].(type) {
		case *parser.KeywordHashNode:
			els = h.Elements
		case *parser.HashNode:
			els = h.Elements
		default:
			f.checkOpenURIHeaders(rest[0])
			headers = rest[0]
		}
		var hs []parser.Node
		for _, el := range els {
			a, ok := el.(*parser.AssocNode)
			if !ok {
				f.errorf(el, "open-uri: a **splat of options is not supported; pass each by name")
			}
			if sym, ok := a.Key.(*parser.SymbolNode); ok {
				f.checkOpenURIOption(sym)
				kws = append(kws, a)
			} else {
				hs = append(hs, a)
			}
		}
		if els != nil {
			headers = &parser.HashNode{Location: rest[0].GetLocation(), Elements: hs}
		}
		rest = rest[1:]
	}
	if len(rest) > 0 {
		f.errorf(rest[0], "open-uri: extra arguments")
	}
	opts := &parser.CallNode{Location: loc, Receiver: constPath("OpenURI::OpenOptions"), Name: "new"}
	if len(kws) > 0 {
		opts.Arguments = &parser.ArgumentsNode{Location: loc, Arguments: []parser.Node{&parser.KeywordHashNode{Location: loc, Elements: kws}}}
	}
	call := []parser.Node{headers, opts}
	if mode != nil {
		call = append(call, mode)
	}
	suffix := ""
	if block != nil {
		suffix = "_block"
	}
	if recv != nil {
		if name == "read" {
			return f.genMethodCall(n, *recv, "__read", call, block)
		}
		return f.genMethodCall(n, *recv, "__open"+suffix, call, block)
	}
	cls := f.c.classes["OpenURI"]
	r := expr{code: classVar(cls), typ: TClass{C: cls.meta}, classObj: true}
	return f.genMethodCall(n, r, "__open_name"+suffix, append([]parser.Node{args[0]}, call...), block)
}

// checkOpenURIHeaders rejects a Symbol-keyed Hash variable, which MRI reads as options, before it fails as a bare type mismatch.
func (f *fctx) checkOpenURIHeaders(n parser.Node) {
	var e expr
	f.probe(func() { e = f.genExpr(n, nil) })
	if t, ok := e.typ.(TClass); ok && len(t.Args) > 0 {
		if k, ok := t.Args[0].(TClass); ok && k.C == f.c.classes["Symbol"] {
			f.errorf(n, "open-uri: a Hash variable is taken as request headers (String keys); pass options such as read_timeout: by name")
		}
	}
}

// checkOpenURIOption rejects an option OpenOptions does not take, naming open-uri rather than the internal class.
func (f *fctx) checkOpenURIOption(sym *parser.SymbolNode) {
	var known []string
	for _, p := range f.c.classes["OpenURI::OpenOptions"].lookup("initialize").M.Params {
		if p.Keyword {
			known = append(known, p.Name)
		}
	}
	if !slices.Contains(known, sym.Unescaped.Value) {
		f.errorf(sym, "open-uri: option :%s is not supported (supported: %s)", sym.Unescaped.Value, strings.Join(known, ", "))
	}
}

// isHashLiteral reports whether n is a `{...}` or brace-less `k => v` argument.
func isHashLiteral(n parser.Node) bool {
	switch n.(type) {
	case *parser.HashNode, *parser.KeywordHashNode:
		return true
	}
	return false
}
