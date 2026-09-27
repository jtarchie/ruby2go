package compiler

import (
	"errors"
	"fmt"
	"regexp"
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
	return o
}

// translateRegexp rewrites Ruby regexp syntax RE2 spells differently.
// Constructs RE2 lacks (lookaround, backreferences, \Z, possessive
// quantifiers) are left in place for regexp.Compile to reject.
func translateRegexp(src string) (string, error) {
	var b strings.Builder
	inClass := 0
	for i := 0; i < len(src); i++ {
		ch := src[i]
		switch {
		case ch == '\\' && i+1 < len(src):
			next := src[i+1]
			i++
			switch next {
			case 'h':
				b.WriteString(map[bool]string{true: "0-9a-fA-F", false: "[0-9a-fA-F]"}[inClass > 0])
			case 'H':
				if inClass > 0 {
					return "", errors.New(`\H inside a character class`)
				}
				b.WriteString("[^0-9a-fA-F]")
			case 'Z':
				return "", errors.New(`\Z is not supported by RE2; use \z`)
			case 'G', 'K', 'R', 'X':
				return "", fmt.Errorf(`\%c is not supported by RE2`, next)
			default:
				b.WriteByte('\\')
				b.WriteByte(next)
			}
		case ch == '[':
			inClass++
			b.WriteByte(ch)
		case ch == ']' && inClass > 0:
			inClass--
			b.WriteByte(ch)
		default:
			b.WriteByte(ch)
		}
	}
	return b.String(), nil
}

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
		goPat, err := translateRegexp(src)
		if err == nil {
			_, err = regexp.Compile(flags.goPrefix() + goPat)
		}
		if err != nil {
			f.errorf(n, "regexp /%s/ is not supported: %v", src, err)
		}
		if flags.extended {
			f.errorf(n, "extended (/x) regexps are not supported")
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
		f.errorf(n, "extended (/x) regexps are not supported")
	}
	// Interpolated values are inserted raw, as Ruby does.
	goParts := []string{strconv.Quote(flags.goPrefix())}
	var srcParts []string
	for _, p := range parts {
		switch p := p.(type) {
		case *parser.StringNode:
			raw := regexpSource(f.f.text(p.Location), term)
			goPat, err := translateRegexp(raw)
			if err != nil {
				f.errorf(p, "regexp is not supported: %v", err)
			}
			goParts = append(goParts, strconv.Quote(goPat))
			srcParts = append(srcParts, strconv.Quote(raw))
		case *parser.EmbeddedStatementsNode:
			if p.Statements == nil || len(p.Statements.Body) != 1 {
				f.errorf(p, "interpolation must contain a single expression")
			}
			e := f.genExpr(p.Statements.Body[0], nil)
			s := "string(" + f.toS(p.Statements.Body[0], e) + ")"
			goParts = append(goParts, s)
			srcParts = append(srcParts, s)
		default:
			f.c.unsupported(f.f, p)
		}
	}
	if len(srcParts) == 0 {
		srcParts = []string{`""`}
	}
	code := fmt.Sprintf("rbRegexpNew(%s, %s, %q)", strings.Join(goParts, " + "), strings.Join(srcParts, " + "), flags.opts())
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
