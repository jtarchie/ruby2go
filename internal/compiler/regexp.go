package compiler

import (
	_ "embed"
	"errors"
	"fmt"
	"regexp"
	"regexp/syntax"
	"slices"
	"strconv"
	"strings"

	"github.com/danielgatis/go-ruby-prism/parser"
)

// rubyRegexFlags carries a literal's options.
type rubyRegexFlags struct{ ignoreCase, multiline, extended bool }

func (r rubyRegexFlags) goPrefix() string {
	// Ruby's ^ and $ always match at line boundaries: Go's (?m). Ruby's /m
	// (dot matches newline) is Go's (?s).
	p := "(?m"
	if r.ignoreCase {
		p += "i"
	}
	if r.multiline {
		p += "s"
	}
	return p + ")"
}

func (r rubyRegexFlags) opts() string {
	o := ""
	if r.multiline {
		o += "m"
	}
	if r.ignoreCase {
		o += "i"
	}
	if r.extended {
		o += "x"
	}
	return o
}

//go:embed rxtranslate.go
var rxTranslateSrc string

// rxTranslateGo is rxtranslate.go minus its package clause and imports
// (goimports restores them), for emitProgram. The generated lint config
// excludes G115, so the file's nolint for it would be flagged as unused.
func rxTranslateGo() string {
	code := rxTranslateSrc[strings.Index(rxTranslateSrc, "\n)\n")+3:]
	return regexp.MustCompile(` //nolint:gosec //.*`).ReplaceAllString(code, "")
}

// rxUnsupported are the RE2 parse errors an interpolated regexp's static
// parts can cause on their own: syntax RE2 lacks (lookaround,
// backreferences, possessive quantifiers). Others (unbalanced groups or
// classes) an interpolated value may cause or cure, so they wait for run time.
var rxUnsupported = []syntax.ErrorCode{syntax.ErrInvalidPerlOp, syntax.ErrInvalidNamedCapture, syntax.ErrInvalidEscape, syntax.ErrInvalidRepeatOp}

// regexpSource is a literal's Regexp#source: as in MRI's lexer, an escaped
// terminator loses its backslash unless it is a regexp metacharacter
// (/a\/b/ is "a/b"; %r{a\}b} keeps "a\\}b").
func regexpSource(raw string, term byte) string {
	if strings.IndexByte("$*+.?^|)]}>", term) >= 0 {
		return raw
	}
	var b strings.Builder
	for i := 0; i < len(raw); i++ {
		if raw[i] == '\\' && i+1 < len(raw) {
			i++
			if raw[i] != term {
				b.WriteByte('\\')
			}
		}
		b.WriteByte(raw[i])
	}
	return b.String()
}

// literalGroups is the number of capture groups in a static regexp
// literal; ok is false for anything else.
func (f *fctx) literalGroups(n parser.Node) (int, bool) {
	r, ok := n.(*parser.RegularExpressionNode)
	if !ok {
		return 0, false
	}
	goPat, err := translateRegexp(regexpSource(f.f.text(r.ContentLoc), f.f.text(r.ClosingLoc)[0]))
	if err != nil {
		return 0, false
	}
	re, err := regexp.Compile(goPat)
	if err != nil {
		return 0, false
	}
	return re.NumSubexp(), true
}

// genRegexp renders a regexp literal. Static ones are validated now and
// compiled once into a package variable.
func (f *fctx) genRegexp(n parser.Node) expr {
	var flags rubyRegexFlags
	var parts []parser.Node
	var term byte
	var once bool
	switch n := n.(type) {
	case *parser.RegularExpressionNode:
		flags = rubyRegexFlags{n.IsIGNORE_CASE(), n.IsMULTI_LINE(), n.IsEXTENDED()}
		src := regexpSource(f.f.text(n.ContentLoc), f.f.text(n.ClosingLoc)[0])
		pat := src
		if flags.extended {
			pat = stripExtended(src) // source and inspect keep the spacing; only the matcher drops it
		}
		goPat, err := translateRegexp(pat)
		if err == nil {
			_, err = regexp.Compile(flags.goPrefix() + goPat)
		}
		if err != nil {
			f.errorf(n, "regexp /%s/ is not supported: %v", src, err)
		}
		args := fmt.Sprintf("%s, %s, %q", strconv.Quote(flags.goPrefix()+goPat), strconv.Quote(src), flags.opts())
		name, ok := f.c.regexpVars[args]
		if !ok {
			name = fmt.Sprintf("rbRe%d", len(f.c.regexps))
			f.c.regexpVars[args] = name
			f.c.regexps = append(f.c.regexps, "var "+name+" = rbRegexpNew("+args+")")
		}
		return expr{code: name, typ: f.cls("Regexp")}
	case *parser.InterpolatedRegularExpressionNode:
		flags = rubyRegexFlags{n.IsIGNORE_CASE(), n.IsMULTI_LINE(), n.IsEXTENDED()}
		parts = n.Parts
		term = f.f.text(n.ClosingLoc)[0]
		once = n.IsONCE()
	}
	if flags.extended {
		f.errorf(n, "interpolated extended (/x) regexps are not supported")
	}
	// Interpolated values are inserted raw, as Ruby does, so the joined
	// source is translated at run time (rbRegexpDyn); each value is
	// evaluated once. The static parts are checked now, with a letter
	// standing in for each value: valid wherever one plausibly sits (an
	// atom, a flag, a group name).
	var srcParts []string
	var probe strings.Builder
	for _, p := range parts {
		switch p := p.(type) {
		case *parser.StringNode:
			raw := regexpSource(f.f.text(p.Location), term)
			probe.WriteString(raw)
			srcParts = append(srcParts, strconv.Quote(raw))
		case *parser.EmbeddedStatementsNode:
			if p.Statements == nil || len(p.Statements.Body) != 1 {
				f.errorf(p, "interpolation must contain a single expression")
			}
			e := f.genExpr(p.Statements.Body[0], nil)
			probe.WriteString("i")
			srcParts = append(srcParts, "string("+f.toS(p.Statements.Body[0], e)+")")
		default:
			f.c.unsupported(f.f, p)
		}
	}
	goPat, err := translateRegexp(probe.String())
	if err == nil {
		_, err = regexp.Compile(flags.goPrefix() + goPat)
		var se *syntax.Error
		if errors.As(err, &se) && !slices.Contains(rxUnsupported, se.Code) {
			err = nil
		}
	}
	if err != nil {
		f.errorf(n, "regexp is not supported: %v", err)
	}
	if len(srcParts) == 0 {
		srcParts = []string{`""`}
	}
	code := fmt.Sprintf("rbRegexpDyn(%s, %s, %q)", strconv.Quote(flags.goPrefix()), strings.Join(srcParts, " + "), flags.opts())
	if once {
		// /o interpolates on first evaluation and keeps that Regexp. Not
		// sync.Once: a raise (RegexpError) must leave it unset to retry, as
		// MRI does; the lock is because threads are goroutines.
		name := fmt.Sprintf("rbRe%d", len(f.c.regexps))
		f.c.regexps = append(f.c.regexps, "var "+name+" struct {\n\tsync.Mutex\n\tre *Regexp\n}")
		code = fmt.Sprintf("func() *Regexp {\n%[1]s.Lock()\ndefer %[1]s.Unlock()\nif %[1]s.re == nil {\n%[1]s.re = %[2]s\n}\nreturn %[1]s.re\n}()", name, code)
	}
	return expr{code: code, typ: f.cls("Regexp")}
}

// stripExtended drops /x's insignificant whitespace and `#` comments,
// leaving escapes (`\ `, `\#`) and character classes as they are.
func stripExtended(src string) string {
	var b strings.Builder
	depth := 0 // inside [...], where space and # are literal
	for i := 0; i < len(src); i++ {
		c := src[i]
		switch {
		case c == '\\' && i+1 < len(src):
			b.WriteByte(c)
			i++
			b.WriteByte(src[i])
			continue
		case c == '[':
			depth++
		case c == ']' && depth > 0:
			depth--
		case depth > 0:
		case strings.IndexByte(" \t\n\r\f\v", c) >= 0:
			continue
		case c == '#':
			for i < len(src) && src[i] != '\n' {
				i++
			}
			continue
		}
		b.WriteByte(c)
	}
	return b.String()
}
