package compiler

import (
	"context"
	"fmt"
	"maps"
	"regexp"
	"slices"
	"strings"

	"github.com/danielgatis/go-ruby-prism/parser"
)

// ERB (decision 111). A template that is a string literal is compiled to
// Ruby at compile time, as ERB::Compiler does, and spliced where `result`
// is called, so the template's code runs in the caller's scope and sees
// its locals; `binding` is never needed at run time.

// erbTemplate is one ERB.new(literal) call's template.
type erbTemplate struct {
	src     string
	trim    string // "", "-", "<>", ">"
	percent bool
	line    int // the file's line of the template's first line
}

// collectERB runs after collect over a user file: every `result`,
// `result_with_hash` and `run` call whose receiver is `ERB.new(literal)`,
// a local assigned one earlier in the same def, or a constant holding one,
// gets its template compiled to Ruby and parsed now (parsing needs the
// context), keyed by the call for genERBCall to splice.
func (c *Compiler) collectERB(ctx context.Context, f *File) {
	var scope []*Class
	type frame struct {
		locals  map[string]bool
		erbVars map[string]*parser.CallNode
	}
	frames := []frame{{locals: erbLocalNames(f.Root), erbVars: map[string]*parser.CallNode{}}}
	var walk func(n parser.Node)
	walk = func(n parser.Node) {
		if n == nil {
			return
		}
		switch x := n.(type) {
		case *parser.ClassNode, *parser.ModuleNode:
			var path parser.Node
			if cn, ok := x.(*parser.ClassNode); ok {
				path = cn.ConstantPath
			} else {
				path = x.(*parser.ModuleNode).ConstantPath
			}
			cls, _ := c.lookupConst(f, path, scope)
			if cls != nil {
				scope = append(scope, cls)
				defer func() { scope = scope[:len(scope)-1] }()
			}
		case *parser.DefNode:
			frames = append(frames, frame{locals: erbLocalNames(x), erbVars: map[string]*parser.CallNode{}})
			defer func() { frames = frames[:len(frames)-1] }()
		case *parser.LocalVariableWriteNode:
			if call := erbNewCall(x.Value); call != nil {
				frames[len(frames)-1].erbVars[x.Name] = call
			}
		case *parser.CallNode:
			if x.Receiver != nil && (x.Name == "result" || x.Name == "result_with_hash" || x.Name == "run") {
				var call *parser.CallNode
				tplFile := f
				switch r := x.Receiver.(type) {
				case *parser.CallNode:
					call = erbNewCall(r)
				case *parser.LocalVariableReadNode:
					call = frames[len(frames)-1].erbVars[r.Name]
				case *parser.ConstantReadNode, *parser.ConstantPathNode:
					if _, k := c.lookupConst(f, r, scope); k != nil {
						call, tplFile = erbNewCall(k.Value), k.File
					}
				}
				if call != nil {
					tpl := c.erbNewTemplate(tplFile, call)
					c.erbSnippets[x] = c.erbParse(ctx, f, x, tpl, frames[len(frames)-1].locals)
				}
			}
		}
		for _, ch := range n.ChildNodes() {
			walk(ch)
		}
	}
	walk(f.Root)
}

// erbNewCall is n when it is `ERB.new(...)`.
func erbNewCall(n parser.Node) *parser.CallNode {
	call, ok := n.(*parser.CallNode)
	if !ok || call.Name != "new" {
		return nil
	}
	switch r := call.Receiver.(type) {
	case *parser.ConstantReadNode:
		if r.Name == "ERB" {
			return call
		}
	case *parser.ConstantPathNode:
		if r.Parent == nil && r.Name != nil && *r.Name == "ERB" {
			return call
		}
	}
	return nil
}

// erbLocalNames are the locals a def (or the program) declares anywhere:
// parameters and assignment targets, inner defs and class bodies excluded.
func erbLocalNames(root parser.Node) map[string]bool {
	names := map[string]bool{}
	var walk func(n parser.Node)
	walk = func(n parser.Node) {
		if n == nil {
			return
		}
		switch x := n.(type) {
		case *parser.DefNode:
			if n != root {
				return
			}
		case *parser.ClassNode, *parser.ModuleNode:
			if n != root {
				return
			}
		case *parser.RequiredParameterNode:
			names[x.Name] = true
		case *parser.OptionalParameterNode:
			names[x.Name] = true
		case *parser.RestParameterNode:
			if x.Name != nil {
				names[*x.Name] = true
			}
		case *parser.RequiredKeywordParameterNode:
			names[x.Name] = true
		case *parser.OptionalKeywordParameterNode:
			names[x.Name] = true
		case *parser.LocalVariableWriteNode:
			names[x.Name] = true
		case *parser.LocalVariableOperatorWriteNode:
			names[x.Name] = true
		case *parser.LocalVariableOrWriteNode:
			names[x.Name] = true
		case *parser.LocalVariableAndWriteNode:
			names[x.Name] = true
		case *parser.LocalVariableTargetNode:
			names[x.Name] = true
		}
		for _, ch := range n.ChildNodes() {
			walk(ch)
		}
	}
	walk(root)
	return names
}

// erbNewTemplate reads `ERB.new(literal, trim_mode: "-")`.
func (c *Compiler) erbNewTemplate(f *File, n *parser.CallNode) *erbTemplate {
	args := callArgs(n)
	if len(args) == 0 {
		c.errorf(f, n, "ERB.new needs a template")
	}
	src, line, ok := erbLiteral(f, args[0])
	if !ok {
		c.errorf(f, args[0], "ERB.new needs a literal template, a String or heredoc without interpolation: the template is compiled with the program (docs/design.md decision 111)")
	}
	tpl := &erbTemplate{src: src, line: line}
	if len(args) > 1 {
		kw, ok := args[1].(*parser.KeywordHashNode)
		if !ok || len(args) > 2 {
			c.errorf(f, args[1], "ERB.new takes a template and trim_mode:")
		}
		for _, el := range kw.Elements {
			c.erbOption(f, el, tpl)
		}
	}
	if tpl.line < 2 {
		c.errorf(f, n, "an ERB template cannot start on the file's first line")
	}
	return tpl
}

// erbOption applies one `trim_mode: "-"` option.
func (c *Compiler) erbOption(f *File, el parser.Node, tpl *erbTemplate) {
	a, ok := el.(*parser.AssocNode)
	if !ok {
		c.errorf(f, el, "ERB.new takes a template and trim_mode:")
	}
	key, _ := a.Key.(*parser.SymbolNode)
	val, _ := a.Value.(*parser.StringNode)
	if key == nil || key.Unescaped.Value != "trim_mode" || val == nil {
		c.errorf(f, el, "ERB.new's only option is trim_mode: with a literal String")
	}
	mode := val.Unescaped.Value
	if !erbTrimMode.MatchString(mode) {
		c.errorf(f, val, "invalid ERB trim mode %q (trim_mode: a String composed of '%%' and/or '-', '>', '<>')", mode)
	}
	tpl.percent = strings.Contains(mode, "%")
	switch {
	case strings.Contains(mode, "-"):
		tpl.trim = "-"
	case strings.Contains(mode, "<>"):
		tpl.trim = "<>"
	case strings.Contains(mode, ">"):
		tpl.trim = ">"
	}
}

var erbTrimMode = regexp.MustCompile(`\A(%|-|>|<>){1,2}\z`)

// erbLiteral is a template literal's text and the file line its content
// starts on: a String, or a heredoc, which Prism may split into one
// StringNode per line.
func erbLiteral(f *File, n parser.Node) (string, int, bool) {
	switch s := n.(type) {
	case *parser.StringNode:
		return s.Unescaped.Value, f.line(s.ContentLoc.StartOffset), true
	case *parser.InterpolatedStringNode:
		var b strings.Builder
		line := 0
		for _, p := range s.Parts {
			part, ok := p.(*parser.StringNode)
			if !ok {
				return "", 0, false
			}
			if line == 0 {
				line = f.line(part.ContentLoc.StartOffset)
			}
			b.WriteString(part.Unescaped.Value)
		}
		return b.String(), line, line > 0
	}
	return "", 0, false
}

// erbParse compiles the template to Ruby and parses it as its own File
// named like the caller's, padded so its lines are the template's lines in
// that file and its byte offsets lie past the file's (locals are keyed by
// offset). The def's locals are declared first (`x = x`, a no-op), so the
// template's parse reads them as locals, not calls, and its blocks assign
// the outer ones; result_with_hash's pairs become assignments of the
// values' source text.
func (c *Compiler) erbParse(ctx context.Context, f *File, n *parser.CallNode, tpl *erbTemplate, locals map[string]bool) *File {
	args := callArgs(n)
	var assigns []string
	switch n.Name {
	case "result", "run":
		if len(args) > 1 {
			c.errorf(f, n, "ERB#%s takes a binding at most", n.Name)
		}
		if len(args) == 1 {
			call, isCall := args[0].(*parser.CallNode)
			_, isNil := args[0].(*parser.NilNode)
			if !isNil && (!isCall || call.Name != "binding" || call.Receiver != nil) {
				c.errorf(f, args[0], "ERB#%s runs in the caller's scope: pass `binding` or nothing", n.Name)
			}
		}
	case "result_with_hash":
		if len(args) != 1 {
			c.errorf(f, n, "ERB#result_with_hash takes one Hash")
		}
		var elems []parser.Node
		switch h := args[0].(type) {
		case *parser.KeywordHashNode:
			elems = h.Elements
		case *parser.HashNode:
			elems = h.Elements
		default:
			c.errorf(f, args[0], "ERB#result_with_hash takes a literal Hash with Symbol keys, each a local the template sees")
		}
		for _, el := range elems {
			a, ok := el.(*parser.AssocNode)
			key, _ := a.Key.(*parser.SymbolNode)
			if !ok || key == nil {
				c.errorf(f, el, "ERB#result_with_hash takes a literal Hash with Symbol keys")
			}
			assigns = append(assigns, key.Unescaped.Value+" = ("+f.text(a.Value.GetLocation())+")")
		}
	}
	c.erbCounter++
	script := erbCompile(tpl, fmt.Sprintf("__erb%d", c.erbCounter))
	var b strings.Builder
	for i := 1; i < tpl.line-1; i++ {
		b.WriteByte('\n')
	}
	for _, name := range slices.Sorted(maps.Keys(locals)) {
		b.WriteString(name + " = " + name + "; ")
	}
	b.WriteString(strings.Join(assigns, "; "))
	if pad := len(f.Src) + 1 - b.Len(); pad > 0 {
		b.WriteString(strings.Repeat(" ", pad))
	}
	b.WriteByte('\n')
	b.WriteString(script)
	snippet, err := parseFile(ctx, c.parser, f.Name, []byte(b.String()), false)
	if err != nil {
		c.errorf(f, n, "ERB template: %v", err)
	}
	return snippet
}

// genERBCall splices a compiled template at its `result`, `result_with_hash`
// or `run` call: the current fctx generates the snippet into a Go block, so
// the caller's locals are in scope.
func (f *fctx) genERBCall(n *parser.CallNode) (expr, bool) {
	snippet := f.c.erbSnippets[n]
	if snippet == nil {
		return expr{}, false
	}
	if r, ok := n.Receiver.(*parser.LocalVariableReadNode); ok {
		f.emit("_ = %s // the template object: its result is spliced here", f.readLocal(r).goName)
	}
	tmp := f.newTmp()
	f.emit("var %s String", tmp)
	f.emit("{")
	savedFile, savedBlock := f.f, f.enterBlock()
	f.f = snippet
	f.erbDepth++
	f.indent++
	f.genStmts(snippet.Root.Statements, tail{kind: tailAssign, target: tmp, typ: f.cls("String")})
	f.indent--
	f.erbDepth--
	f.f = savedFile
	f.leaveBlock(savedBlock)
	f.emit("}")
	if n.Name == "run" {
		f.emit("rbWriteOut(string(%s))", tmp)
		return expr{code: "", typ: TNil{}, stmt: true, done: true}, true
	}
	return expr{code: tmp, typ: f.cls("String")}, true
}

// erbCompile is ERB::Compiler#compile: the template as Ruby statements,
// one output line per template line, that append to the Array local out
// and end with its join.
func erbCompile(tpl *erbTemplate, out string) string {
	c := &erbCompiler{tpl: tpl, out: out}
	c.line = append(c.line, out+" = [\"\"]")
	c.scan()
	if c.content.Len() > 0 {
		c.putCmd()
	}
	c.line = append(c.line, out+".join")
	c.script.WriteString(strings.Join(c.line, "; "))
	return c.script.String()
}

type erbCompiler struct {
	tpl     *erbTemplate
	out     string
	stag    string
	head    string // "<>" mode: the line's first token
	content strings.Builder
	line    []string
	script  strings.Builder
}

// erbToken is one scanner token: text, a line end (cr), or a `%` line's code.
type erbToken struct {
	s       string
	cr      bool
	percent bool
}

const erbTags = `<%%|<%=|<%#|<%|%%>|%>`

var (
	erbScanDefault = regexp.MustCompile(`(?s)(.*?)(` + erbTags + `|\n|\z)`)
	erbScanAngle   = regexp.MustCompile(`(?s)(.*?)(%>\r?\n|` + erbTags + `|\n|\z)`)
	erbScanDash    = regexp.MustCompile(`(?sm)(.*?)(^[ \t]*<%-|<%-|-%>\r?\n|-%>|` + erbTags + `|\z)`)
	erbStags       = map[string]bool{"<%=": true, "<%#": true, "<%": true}
)

func (c *erbCompiler) scan() {
	c.stag = ""
	if !c.tpl.percent {
		c.scanLine(c.tpl.src)
		return
	}
	for _, line := range strings.SplitAfter(c.tpl.src, "\n") {
		if line == "" {
			continue
		}
		if c.stag != "" || line[0] != '%' {
			c.scanLine(line)
			continue
		}
		line = line[1:]
		if strings.HasPrefix(line, "%") {
			c.scanLine(line)
		} else {
			c.token(erbToken{s: strings.TrimSuffix(strings.TrimSuffix(line, "\n"), "\r"), percent: true})
		}
	}
}

// scanLine is TrimScanner's scan_line / trim_line1 / trim_line2 / explicit_trim_line.
func (c *erbCompiler) scanLine(line string) {
	re := erbScanDefault
	switch c.tpl.trim {
	case ">", "<>":
		re = erbScanAngle
	case "-":
		re = erbScanDash
	}
	c.head = ""
	for _, m := range re.FindAllStringSubmatchIndex(line, -1) {
		for _, tok := range []string{line[m[2]:m[3]], line[m[4]:m[5]]} {
			if tok == "" {
				continue
			}
			switch c.tpl.trim {
			case ">":
				c.trimToken1(tok)
			case "<>":
				c.trimToken2(tok)
			case "-":
				c.explicitTrimToken(tok)
			default:
				c.token(erbToken{s: tok})
			}
		}
	}
}

// trimToken1 is trim_line1 (">"): the newline after %> is dropped.
func (c *erbCompiler) trimToken1(tok string) {
	if tok == "%>\n" || tok == "%>\r\n" {
		c.token(erbToken{s: "%>"})
		c.token(erbToken{cr: true})
		return
	}
	c.token(erbToken{s: tok})
}

// trimToken2 is trim_line2 ("<>"): the newline is dropped on a line that begins with a tag and ends with %>.
func (c *erbCompiler) trimToken2(tok string) {
	if c.head == "" {
		c.head = tok
	}
	if tok == "%>\n" || tok == "%>\r\n" {
		c.token(erbToken{s: "%>"})
		if erbStags[c.head] {
			c.token(erbToken{cr: true})
		} else {
			c.token(erbToken{s: "\n"})
		}
		c.head = ""
		return
	}
	c.token(erbToken{s: tok})
	if tok == "\n" {
		c.head = ""
	}
}

// explicitTrimToken is explicit_trim_line ("-"): <%- strips the indentation before it, -%> the newline after.
func (c *erbCompiler) explicitTrimToken(tok string) {
	switch {
	case c.stag == "" && strings.HasSuffix(tok, "<%-"):
		c.token(erbToken{s: "<%"})
	case c.stag != "" && (tok == "-%>\n" || tok == "-%>\r\n"):
		c.token(erbToken{s: "%>"})
		c.token(erbToken{cr: true})
	case c.stag != "" && tok == "-%>":
		c.token(erbToken{s: "%>"})
	default:
		c.token(erbToken{s: tok})
	}
}

// token is compile_stag / compile_etag.
func (c *erbCompiler) token(t erbToken) {
	if c.stag == "" {
		switch {
		case t.percent:
			if c.content.Len() > 0 {
				c.putCmd()
			}
			c.line = append(c.line, t.s)
			c.cr()
		case t.cr:
			c.cr()
		case t.s == "<%" || t.s == "<%=" || t.s == "<%#":
			c.stag = t.s
			if c.content.Len() > 0 {
				c.putCmd()
			}
		case t.s == "\n":
			c.content.WriteString("\n")
			c.putCmd()
		case t.s == "<%%":
			c.content.WriteString("<%")
		default:
			c.content.WriteString(t.s)
		}
		return
	}
	switch t.s {
	case "%>":
		c.compileContent()
		c.stag = ""
		c.content.Reset()
	case "%%>":
		c.content.WriteString("%>")
	default:
		c.content.WriteString(t.s)
	}
}

func (c *erbCompiler) compileContent() {
	content := c.content.String()
	switch c.stag {
	case "<%":
		if strings.HasSuffix(content, "\n") {
			c.line = append(c.line, strings.TrimSuffix(content, "\n"))
			c.cr()
		} else {
			c.line = append(c.line, content)
		}
	case "<%=":
		c.line = append(c.line, c.out+" << (("+content+").to_s)")
	case "<%#":
		c.line = append(c.line, strings.Repeat("\n", strings.Count(content, "\n"))) // only the line count
	}
}

func (c *erbCompiler) putCmd() {
	content := c.content.String()
	c.line = append(c.line, c.out+" << "+erbDump(content)+strings.Repeat("\n", strings.Count(content, "\n")))
	c.content.Reset()
}

func (c *erbCompiler) cr() {
	c.script.WriteString(strings.Join(c.line, "; "))
	c.script.WriteString("\n")
	c.line = nil
}

// erbDump is String#dump for the generated code: a double-quoted literal with nothing to interpolate.
func erbDump(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for i := range len(s) {
		ch := s[i]
		switch ch {
		case '"', '\\', '#':
			b.WriteByte('\\')
			b.WriteByte(ch)
		case '\n':
			b.WriteString(`\n`)
		case '\t':
			b.WriteString(`\t`)
		case '\r':
			b.WriteString(`\r`)
		default:
			if ch < 0x20 || ch == 0x7f {
				fmt.Fprintf(&b, `\x%02X`, ch)
			} else {
				b.WriteByte(ch)
			}
		}
	}
	b.WriteByte('"')
	return b.String()
}
