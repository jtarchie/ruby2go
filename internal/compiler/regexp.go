package compiler

import (
	"errors"
	"fmt"
	"regexp"
	"regexp/syntax"
	"slices"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

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

// translateRegexp rewrites Ruby (Onigmo) regexp syntax RE2 spells or reads
// differently. Constructs RE2 lacks (lookaround, backreferences, \Z,
// possessive quantifiers) are left in place for regexp.Compile to reject.
func translateRegexp(src string) (string, error) {
	t := rxTranslator{src: src}
	err := t.run()
	if err == nil && t.sawNamed {
		// MRI does not capture plain (...) once a pattern names a group.
		t = rxTranslator{src: src, named: true}
		err = t.run()
	}
	return string(t.out), err
}

type rxTranslator struct {
	src             string
	i               int
	out             []byte
	named, sawNamed bool
}

var (
	rxFlagGroup  = regexp.MustCompile(`^\(\?[imx]*(?:-[imx]*)?[:)]`)
	rxInterval   = regexp.MustCompile(`^\{(\d*)(,?)(\d*)\}`)
	rxUnicodeEsc = regexp.MustCompile(`^\\u(?:([0-9a-fA-F]{4})|\{ *([0-9a-fA-F]{1,6}(?: +[0-9a-fA-F]{1,6})*) *\})`)
	rxPosixName  = regexp.MustCompile(`^\[:(\^?)([a-z]+):\]`)
)

// rxSpace is Onigmo's \s, which unlike RE2's includes \v.
const rxSpace = `\t\n\v\f\r `

// rxPosix spells Onigmo's POSIX bracket classes, which are Unicode
// properties (RE2's are ASCII), as RE2 class bodies.
var rxPosix = func() map[string]string {
	alpha := `\p{L}\p{Nl}` + rxTable(unicode.Other_Alphabetic)
	graph := `\p{L}\p{M}\p{N}\p{P}\p{S}\p{Cf}\p{Co}`
	return map[string]string{
		"alnum": alpha + `\p{Nd}`, "alpha": alpha, "ascii": `\x00-\x7f`,
		"blank": `\t\p{Zs}`, "cntrl": `\p{Cc}`, "digit": `\p{Nd}`,
		"graph": graph, "lower": `\p{Ll}` + rxTable(unicode.Other_Lowercase),
		"print": graph + `\p{Zs}`, "punct": `\p{P}$+<=>\^` + "`|~",
		"space": `\t-\r\x{85}\p{Z}`, "upper": `\p{Lu}` + rxTable(unicode.Other_Uppercase),
		"word": alpha + `\p{M}\p{Nd}\p{Pc}`, "xdigit": `0-9A-Fa-f`,
	}
}()

func (t *rxTranslator) run() error {
	var groups []int // out offsets of open groups
	atom := -1       // out offset of the last quantifiable atom
	for t.i < len(t.src) {
		start := len(t.out)
		switch c := t.src[t.i]; c {
		case '\\':
			s, err := t.escape(false)
			if err != nil {
				return err
			}
			t.out, atom = append(t.out, s...), start
		case '[':
			err := t.class()
			if err != nil {
				return err
			}
			atom = start
		case '(':
			if t.group() {
				groups = append(groups, start)
			}
			atom = -1
		case ')':
			if len(groups) > 0 {
				atom, groups = groups[len(groups)-1], groups[:len(groups)-1]
			}
			t.out = append(t.out, c)
			t.i++
		case '{':
			t.interval(atom)
		case '|', '^', '$':
			t.out, atom = append(t.out, c), -1
			t.i++
		case '*', '+', '?':
			t.out = append(t.out, c)
			t.i++
		default:
			_, n := utf8.DecodeRuneInString(t.src[t.i:])
			t.out, atom = append(t.out, t.src[t.i:t.i+n]...), start
			t.i += n
		}
	}
	return nil
}

// group copies a group opener, reporting whether it opened a group: an
// inline option like (?i) does not. Ruby's m option is RE2's s; RE2's m is
// always on (goPrefix).
func (t *rxTranslator) group() bool {
	rest := t.src[t.i:]
	if m := rxFlagGroup.FindString(rest); m != "" {
		t.out = append(t.out, strings.ReplaceAll(m, "m", "s")...)
		t.i += len(m)
		return strings.HasSuffix(m, ":")
	}
	open := "("
	switch {
	case strings.HasPrefix(rest, "(?<") && !strings.HasPrefix(rest, "(?<=") && !strings.HasPrefix(rest, "(?<!"):
		t.sawNamed = true
		open = "(?<"
	case strings.HasPrefix(rest, "(?"):
		open = "(?"
	}
	t.i += len(open)
	if open == "(" && t.named {
		open = "(?:"
	}
	t.out = append(t.out, open...)
	return true
}

// interval copies a {n,m} quantifier. Ruby's {,n} is {0,n}; X{n}? and a
// following + or * repeat (?:X{n}) where RE2 reads laziness or rejects.
func (t *rxTranslator) interval(atom int) {
	m := rxInterval.FindStringSubmatch(t.src[t.i:])
	if m == nil || m[1] == "" && m[3] == "" {
		t.out = append(t.out, '{')
		t.i++
		return
	}
	t.i += len(m[0])
	lo := m[1]
	if lo == "" {
		lo = "0"
	}
	q := "{" + lo + m[2] + m[3] + "}"
	if next := byte(0); atom >= 0 {
		if t.i < len(t.src) {
			next = t.src[t.i]
		}
		if next == '+' || next == '*' || next == '?' && m[2] == "" {
			t.out = slices.Insert(t.out, atom, []byte("(?:")...)
			q += ")"
		}
	}
	t.out = append(t.out, q...)
}

// escape translates the escape at t.src[t.i], inside a class body or not.
func (t *rxTranslator) escape(inClass bool) (string, error) {
	if m := rxUnicodeEsc.FindStringSubmatch(t.src[t.i:]); m != nil {
		t.i += len(m[0])
		var b strings.Builder
		for _, h := range strings.Fields(m[1] + m[2]) {
			b.WriteString(`\x{` + h + `}`)
		}
		return b.String(), nil
	}
	if t.i+1 >= len(t.src) {
		t.i++
		return `\`, nil
	}
	_, n := utf8.DecodeRuneInString(t.src[t.i+1:])
	esc := t.src[t.i : t.i+1+n]
	t.i += len(esc)
	class := func(body string) string {
		if inClass {
			return body
		}
		return "[" + body + "]"
	}
	switch esc[1] {
	case 'h':
		return class("0-9a-fA-F"), nil
	case 's':
		return class(rxSpace), nil
	case 'H':
		if inClass {
			return "", errors.New(`\H inside a character class`)
		}
		return "[^0-9a-fA-F]", nil
	case 'S':
		if inClass {
			return rxNot(rxSpace)
		}
		return "[^" + rxSpace + "]", nil
	case 'e':
		return `\x1b`, nil
	case 'Q', 'E':
		return esc[1:], nil
	case 'p', 'P':
		if end := strings.IndexByte(t.src[t.i:], '}'); strings.HasPrefix(t.src[t.i:], "{") && end > 0 {
			esc += t.src[t.i : t.i+end+1]
			t.i += end + 1
		}
	case 'Z':
		return "", errors.New(`\Z is not supported by RE2; use \z`)
	case 'G', 'K', 'R', 'X':
		return "", fmt.Errorf(`%s is not supported by RE2`, esc)
	}
	return esc, nil
}

// class translates the bracket expression at t.src[t.i].
func (t *rxTranslator) class() error {
	start := t.i
	body, neg, closed, err := t.classBody()
	if err != nil {
		return err
	}
	switch {
	case !closed:
		// An interpolated regexp's class can span parts.
		t.out = append(t.out, t.src[start:]...)
	case body == "" && neg:
		t.out = append(t.out, `[\x00-\x{10ffff}]`...)
	case body == "":
		t.out = append(t.out, `[^\x00-\x{10ffff}]`...)
	case neg:
		t.out = append(t.out, "[^"+body+"]"...)
	default:
		t.out = append(t.out, "["+body+"]"...)
	}
	return nil
}

// classBody reads a bracket expression into a positive RE2 class body ("" is
// empty). Onigmo's set syntax (nested classes, &&) and negated members have
// no RE2 spelling, so those are computed as rune ranges.
func (t *rxTranslator) classBody() (body string, neg, closed bool, err error) {
	t.i++
	if strings.HasPrefix(t.src[t.i:], "^") {
		neg = true
		t.i++
	}
	var union strings.Builder
	var and []string
	for t.i < len(t.src) {
		rest := t.src[t.i:]
		var s string
		switch {
		case rest[0] == ']':
			t.i++
			if and == nil {
				return union.String(), neg, true, nil
			}
			body, err = rxAnd(append(and, union.String()))
			return body, neg, true, err
		case strings.HasPrefix(rest, "&&"):
			and = append(and, union.String())
			union.Reset()
			t.i += 2
		case rest[0] == '[':
			s, err = t.classMember()
		case rest[0] == '\\':
			s, err = t.escape(true)
		default:
			_, n := utf8.DecodeRuneInString(rest)
			s = rest[:n]
			t.i += n
		}
		if err != nil {
			return "", false, false, err
		}
		union.WriteString(s)
	}
	return union.String(), neg, false, nil
}

// classMember reads a POSIX bracket or a nested class inside a class.
func (t *rxTranslator) classMember() (string, error) {
	if m := rxPosixName.FindStringSubmatch(t.src[t.i:]); m != nil {
		body, ok := rxPosix[m[2]]
		if !ok {
			return "", fmt.Errorf("invalid POSIX bracket type [:%s:]", m[2])
		}
		t.i += len(m[0])
		if m[1] != "" {
			return rxNot(body)
		}
		return body, nil
	}
	body, neg, _, err := t.classBody()
	if err != nil || !neg {
		return body, err
	}
	return rxNot(body)
}

// rxAnd intersects class bodies: the complement of their complements' union.
func rxAnd(bodies []string) (string, error) {
	var union strings.Builder
	for _, b := range bodies {
		nb, err := rxNot(b)
		if err != nil {
			return "", err
		}
		union.WriteString(nb)
	}
	return rxNot(union.String())
}

// rxNot is the complement of a class body, computed by RE2's own parser.
func rxNot(body string) (string, error) {
	if body == "" {
		return `\x00-\x{10ffff}`, nil
	}
	re, err := syntax.Parse("[^"+body+"]", syntax.Perl)
	if err != nil {
		return "", fmt.Errorf("character class: %w", err)
	}
	var rs []rune
	//exhaustive:ignore // a parsed class is one of these, or OpNoMatch (empty)
	switch re.Op {
	case syntax.OpCharClass:
		rs = re.Rune
	case syntax.OpAnyChar:
		rs = []rune{0, unicode.MaxRune}
	case syntax.OpLiteral: // one rune left, or one case-fold orbit
		for r := re.Rune[0]; ; {
			rs = append(rs, r, r)
			if r = unicode.SimpleFold(r); re.Flags&syntax.FoldCase == 0 || r == re.Rune[0] {
				break
			}
		}
	}
	var b strings.Builder
	for i := 0; i < len(rs); i += 2 {
		b.WriteString(rxRune(rs[i]))
		if rs[i+1] != rs[i] {
			b.WriteString("-" + rxRune(rs[i+1]))
		}
	}
	return b.String(), nil
}

// rxTable spells a Unicode table RE2 cannot name as a class body.
func rxTable(tab *unicode.RangeTable) string {
	var b strings.Builder
	add := func(lo, hi, stride rune) {
		if stride == 1 {
			b.WriteString(rxRune(lo))
			if hi != lo {
				b.WriteString("-" + rxRune(hi))
			}
			return
		}
		for r := lo; r <= hi; r += stride {
			b.WriteString(rxRune(r))
		}
	}
	for _, r := range tab.R16 {
		add(rune(r.Lo), rune(r.Hi), rune(r.Stride))
	}
	for _, r := range tab.R32 {
		add(rune(r.Lo), rune(r.Hi), rune(r.Stride)) //nolint:gosec // Unicode tables hold runes
	}
	return b.String()
}

func rxRune(r rune) string {
	if r < utf8.RuneSelf && (unicode.IsLetter(r) || unicode.IsDigit(r)) {
		return string(r)
	}
	return `\x{` + strconv.FormatInt(int64(r), 16) + `}`
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
