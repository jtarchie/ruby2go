package main

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strconv"
	"strings"

	"github.com/danielgatis/go-ruby-prism/parser"
)

// edit replaces src[start:end]; removals keep their newlines so later errors' line numbers still match the file.
type edit struct {
	start, end int
	text       string
}

func apply(src string, edits []edit) string {
	slices.SortFunc(edits, func(a, b edit) int { return a.start - b.start })
	var b strings.Builder
	at := 0
	for _, e := range edits {
		if e.start < at {
			continue // inside a region already replaced
		}
		b.WriteString(src[at:e.start])
		b.WriteString(e.text)
		at = e.end
	}
	b.WriteString(src[at:])
	return b.String()
}

func blank(src string, start, end int) edit {
	return edit{start, end, strings.Repeat("\n", strings.Count(src[start:end], "\n"))}
}

func blankLoc(src string, l parser.Location) edit {
	s, e := span(l)
	return blank(src, s, e)
}

func span(l parser.Location) (int, int) { return l.StartOffset, l.StartOffset + l.Length }

// sharedSpec is a `describe :name, shared: true` body, inlined where it_behaves_like names it.
type sharedSpec struct {
	body string
}

type rewriter struct {
	prog  *program
	ctx   context.Context //nolint:containedctx // one rewrite's walk
	name  string
	src   string
	depth int
	edits []edit
	err   error
}

// rewrite makes what mspec does at run time static, as rb2go's describe needs: context, guards, shared specs, before(:each).
func (prog *program) rewrite(ctx context.Context, name, src string, depth int) (string, error) {
	if depth > 8 {
		return "", errors.New("shared specs nest too deep")
	}
	res, err := prog.p.Parse(ctx, []byte(src))
	if err != nil {
		return "", fmt.Errorf("%s: %w", name, err)
	}
	if len(res.Errors) > 0 {
		return "", fmt.Errorf("%s: syntax error: %s", name, res.Errors[0].Message)
	}
	rw := &rewriter{prog: prog, ctx: ctx, name: name, src: src, depth: depth}
	rw.visit(res.Value)
	if rw.err != nil {
		return "", rw.err
	}
	return apply(src, rw.edits), nil
}

func (rw *rewriter) visit(n parser.Node) {
	call, ok := n.(*parser.CallNode)
	if ok && call.Receiver == nil && rw.call(call) {
		return
	}
	if ok && (call.Name == "should" || call.Name == "should_not") && call.Arguments == nil && isProc(call.Receiver) {
		// a Proc is a Go func, with no methods for Kernel#should to add (decision 47)
		ls, le := span(call.Receiver.GetLocation())
		_, ce := span(call.Location)
		rw.edits = append(rw.edits, edit{ls, ls, "ProcExpectation.new("}, edit{le, ce, ", " + strconv.FormatBool(call.Name == "should") + ")"})
	}
	for _, ch := range n.CompactChildNodes() {
		rw.visit(ch)
	}
}

// call answers true when it already walked (or blanked) the children, so visit must not.
func (rw *rewriter) call(call *parser.CallNode) bool {
	blk, _ := call.Block.(*parser.BlockNode)
	args := []parser.Node{}
	if call.Arguments != nil {
		args = call.Arguments.Arguments
	}
	switch {
	case call.Name == "require_relative" && len(args) == 1:
		rw.require(call, args[0])
		return true
	case call.Name == "context" && blk != nil && call.MessageLoc != nil:
		s, e := span(*call.MessageLoc)
		rw.edits = append(rw.edits, edit{s, e, "describe"})
	case call.Name == "describe" && blk != nil && isShared(args):
		rw.prog.shared[literal(args[0])] = sharedSpec{body: rw.text(blk.Body)}
		rw.edits = append(rw.edits, blankLoc(rw.src, call.Location))
		return true
	case call.Name == "it_behaves_like" && len(args) >= 2:
		rw.inline(call, args)
		return true
	case (call.Name == "before" || call.Name == "after") && blk != nil && call.Arguments != nil:
		s, e := span(call.Arguments.Location)
		rw.edits = append(rw.edits, blank(rw.src, s, e)) // rb2go's before is minitest's setup: :each, and :all once per example
	case guards[call.Name] && blk != nil:
		rw.guard(call, blk, args)
		return true
	}
	return false
}

func (rw *rewriter) text(n parser.Node) string {
	if n == nil {
		return ""
	}
	s, e := span(n.GetLocation())
	return rw.src[s:e]
}

func (rw *rewriter) require(call *parser.CallNode, arg parser.Node) {
	rel := literal(arg)
	if rel == "" {
		return
	}
	rw.edits = append(rw.edits, blankLoc(rw.src, call.Location))
	if filepath.Base(rel) == "spec_helper" || filepath.Base(rel) == "spec_helper.rb" {
		return
	}
	target := filepath.Join(filepath.Dir(rw.name), rel)
	if !strings.HasSuffix(target, ".rb") {
		target += ".rb"
	}
	err := rw.prog.load(rw.ctx, target)
	if err != nil && rw.err == nil {
		rw.err = err
	}
}

// methodRef is mspec's @method/@object in a shared spec: it_behaves_like's second and third arguments.
var methodRef = regexp.MustCompile(`@(method|object)\b`)

func (rw *rewriter) inline(call *parser.CallNode, args []parser.Node) {
	spec, ok := rw.prog.shared[literal(args[0])]
	if !ok {
		return // left for rb2go to reject, which drops the statement
	}
	object := "nil"
	if len(args) > 2 {
		object = rw.text(args[2])
	}
	body := methodRef.ReplaceAllStringFunc(spec.body, func(m string) string {
		if m == "@method" {
			return rw.text(args[1])
		}
		return object
	})
	body, err := rw.prog.rewrite(rw.ctx, rw.name, body, rw.depth+1)
	if err != nil {
		rw.err = cmpErr(rw.err, err)
		return
	}
	s, e := span(call.Location)
	rw.edits = append(rw.edits, edit{s, e, "describe " + strconv.Quote(literal(args[0])) + " do\n" + body + "\nend"})
}

func cmpErr(old, err error) error {
	if old != nil {
		return old
	}
	return err
}

// guards are mspec's conditional blocks; guard decides each statically.
var guards = map[string]bool{
	"ruby_version_is": true, "ruby_bug": true, "platform_is": true, "platform_is_not": true,
	"little_endian": true, "big_endian": true, "as_user": true, "as_superuser": true,
	"guard": true, "guard_not": true, "with_feature": true, "without_feature": true,
	"not_supported_on": true, "quarantine!": true, "kernel_is": true,
}

func (rw *rewriter) guard(call *parser.CallNode, blk *parser.BlockNode, args []parser.Node) {
	cs, ce := span(call.Location)
	if !rw.keep(call.Name, args) || blk.Body == nil {
		rw.edits = append(rw.edits, blank(rw.src, cs, ce))
		return
	}
	bs, be := span(blk.Body.GetLocation())
	rw.edits = append(rw.edits, blank(rw.src, cs, bs), blank(rw.src, be, ce))
	rw.visit(blk.Body)
}

// keep is whether a guard's block runs here: MRI rubyVersion on this machine's OS, 64-bit, little-endian, not root.
// ponytail: guard/with_feature/kernel_is always drop their block; evaluate the ones rb2go can answer.
func (rw *rewriter) keep(name string, args []parser.Node) bool {
	switch name {
	case "ruby_version_is":
		return len(args) > 0 && versionIn(args[0])
	case "ruby_bug":
		return len(args) < 2 || !versionIn(args[1])
	case "platform_is":
		return platformIs(args)
	case "platform_is_not":
		return !platformIs(args)
	case "little_endian", "as_user", "not_supported_on", "without_feature":
		return true
	}
	return false
}

func versionIn(n parser.Node) bool {
	switch n := n.(type) {
	case *parser.StringNode:
		return compareVersion(rubyVersion, n.Unescaped.Value) >= 0
	case *parser.RangeNode:
		lo, hi := literal(n.Left), literal(n.Right)
		if lo != "" && compareVersion(rubyVersion, lo) < 0 {
			return false
		}
		if hi == "" {
			return true
		}
		c := compareVersion(rubyVersion, hi)
		return c < 0 || c == 0 && !n.IsEXCLUDE_END()
	}
	return false
}

func compareVersion(a, b string) int {
	as, bs := strings.Split(a, "."), strings.Split(b, ".")
	for i := range max(len(as), len(bs)) {
		var x, y int
		if i < len(as) {
			x, _ = strconv.Atoi(as[i])
		}
		if i < len(bs) {
			y, _ = strconv.Atoi(bs[i])
		}
		if x != y {
			return x - y
		}
	}
	return 0
}

// platformIs is mspec's PlatformGuard: an OS symbol matches RUBY_PLATFORM by substring; sizes are 64-bit.
func platformIs(args []parser.Node) bool {
	platform := runtime.GOARCH + "-" + runtime.GOOS
	for _, a := range args {
		switch a := a.(type) {
		case *parser.SymbolNode:
			if a.Unescaped.Value != "windows" && strings.Contains(platform, a.Unescaped.Value) {
				return true
			}
		case *parser.KeywordHashNode:
			for _, el := range a.Elements {
				if as, ok := el.(*parser.AssocNode); ok {
					if v, ok := as.Value.(*parser.IntegerNode); ok && v.Value == 64 {
						return true
					}
				}
			}
		}
	}
	return false
}

func isProc(n parser.Node) bool {
	switch n := n.(type) {
	case *parser.LambdaNode:
		return true
	case *parser.CallNode:
		return n.Receiver == nil && (n.Name == "lambda" || n.Name == "proc") && n.Block != nil
	}
	return false
}

func isShared(args []parser.Node) bool {
	for _, a := range args {
		if h, ok := a.(*parser.KeywordHashNode); ok {
			for _, el := range h.Elements {
				if as, ok := el.(*parser.AssocNode); ok && literal(as.Key) == "shared" {
					_, yes := as.Value.(*parser.TrueNode)
					return yes
				}
			}
		}
	}
	return false
}

// literal is a string or symbol literal's value, else "".
func literal(n parser.Node) string {
	switch n := n.(type) {
	case *parser.StringNode:
		return n.Unescaped.Value
	case *parser.SymbolNode:
		return n.Unescaped.Value
	}
	return ""
}

// skipAt turns the example holding name:line into a skip carrying reason, or else drops the innermost statement there.
func (prog *program) skipAt(ctx context.Context, r *result, name string, line int, reason string) error {
	src := prog.byName[name]
	if src == nil || src.name == "mspec.rb" {
		return fmt.Errorf("%s:%d: %s", name, line, reason)
	}
	res, err := prog.p.Parse(ctx, []byte(src.text))
	if err != nil {
		return fmt.Errorf("%s: %w", name, err)
	}
	lines := newLineIndex(src.text)
	where := fmt.Sprintf("%s:%d", name, line)
	if ex := findExample(res.Value, lines, line); ex != nil {
		blk := ex.Block.(*parser.BlockNode)
		s, e := blk.OpeningLoc.StartOffset+blk.OpeningLoc.Length, blk.ClosingLoc.StartOffset
		body := src.text[s:e]
		if strings.HasPrefix(body, " skip \"rb2go: ") {
			return fmt.Errorf("%s: an error remains in an example already skipped: %s", where, reason)
		}
		msg := strings.ReplaceAll(strconv.Quote("rb2go: "+reason), "#", `\#`)
		src.text = apply(src.text, []edit{{s, e, " skip " + msg + strings.Repeat("\n", strings.Count(body, "\n"))}})
		r.unsupported = append(r.unsupported, skipped{where + " " + literal(callArg(ex)), reason})
		return nil
	}
	st := innermostStatement(res.Value.Statements, lines, line)
	if st == nil {
		return fmt.Errorf("%s: %s", where, reason)
	}
	src.text = apply(src.text, []edit{blankLoc(src.text, st.GetLocation())})
	r.dropped = append(r.dropped, skipped{where, reason})
	return nil
}

func callArg(c *parser.CallNode) parser.Node {
	if c.Arguments == nil || len(c.Arguments.Arguments) == 0 {
		return nil
	}
	return c.Arguments.Arguments[0]
}

type lineIndex []int // byte offset of each line's start

func newLineIndex(src string) lineIndex {
	idx := lineIndex{0}
	for i := range len(src) {
		if src[i] == '\n' {
			idx = append(idx, i+1)
		}
	}
	return idx
}

func (l lineIndex) line(off int) int {
	i, found := slices.BinarySearch(l, off)
	if found {
		return i + 1
	}
	return i
}

func (l lineIndex) holds(n parser.Node, line int) bool {
	loc := n.GetLocation()
	return l.line(loc.StartOffset) <= line && line <= l.line(loc.StartOffset+max(loc.Length-1, 0))
}

// findExample is the innermost it/specify block holding line.
func findExample(n parser.Node, lines lineIndex, line int) *parser.CallNode {
	var found *parser.CallNode
	var walk func(parser.Node)
	walk = func(n parser.Node) {
		if !lines.holds(n, line) {
			return
		}
		if c, ok := n.(*parser.CallNode); ok && c.Receiver == nil && (c.Name == "it" || c.Name == "specify") {
			if _, ok := c.Block.(*parser.BlockNode); ok {
				found = c
			}
		}
		for _, ch := range n.CompactChildNodes() {
			walk(ch)
		}
	}
	walk(n)
	return found
}

// innermostStatement is the statement holding line, descending into describe, class and module bodies.
func innermostStatement(stmts *parser.StatementsNode, lines lineIndex, line int) parser.Node {
	if stmts == nil {
		return nil
	}
	for _, st := range stmts.Body {
		if !lines.holds(st, line) {
			continue
		}
		var body parser.Node
		switch n := st.(type) {
		case *parser.CallNode:
			if blk, ok := n.Block.(*parser.BlockNode); ok && n.Receiver == nil && n.Name == "describe" {
				body = blk.Body
			}
		case *parser.ClassNode:
			body = n.Body
		case *parser.ModuleNode:
			body = n.Body
		case *parser.SingletonClassNode:
			body = n.Body
		}
		if inner, ok := body.(*parser.StatementsNode); ok {
			if deeper := innermostStatement(inner, lines, line); deeper != nil {
				return deeper
			}
		}
		return st
	}
	return nil
}
