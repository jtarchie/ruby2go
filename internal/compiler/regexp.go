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

// genRegexp renders a regexp literal. Static ones are validated now and
// compiled once into a package variable.
func (f *fctx) genRegexp(n parser.Node) expr {
	var flags rubyRegexFlags
	var parts []parser.Node
	switch n := n.(type) {
	case *parser.RegularExpressionNode:
		flags = rubyRegexFlags{n.IsIGNORE_CASE(), n.IsMULTI_LINE(), n.IsEXTENDED()}
		src := f.f.text(n.ContentLoc)
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
			raw := f.f.text(p.Location)
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
	return expr{code: fmt.Sprintf("rbRegexpNew(%s, %s, %q)", strings.Join(goParts, " + "), strings.Join(srcParts, " + "), flags.opts()),
		typ: f.cls("Regexp")}
}
