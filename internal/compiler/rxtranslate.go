package compiler

// The Ruby-to-RE2 regexp translator, and the matcher for what no
// translation can express. It is self-contained (stdlib only) because this
// file is also emitted verbatim into every program (rxTranslateGo): an
// interpolated regexp's source only exists at run time.

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
)

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
	rxFlagGroup  = regexp.MustCompile(`^\(\?([imx]*)(?:-([imx]*))?([:)])`)
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
// always on (goPrefix). x off is RE2's only mode, so it is dropped (an
// embedded Regexp#to_s is (?i-mx:...)); x on is left for RE2 to reject.
func (t *rxTranslator) group() bool {
	rest := t.src[t.i:]
	if m := rxFlagGroup.FindStringSubmatch(rest); m != nil {
		flags := m[1]
		if off := strings.ReplaceAll(m[2], "x", ""); off != "" {
			flags += "-" + off
		}
		t.out = append(t.out, "(?"+strings.ReplaceAll(flags, "m", "s")+m[3]...)
		t.i += len(m[0])
		return m[3] == ":"
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
		// Left for RE2 to reject.
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

// rxRegexp is a compiled Ruby regexp: RE2, plus Onigmo's readings of ^ and
// \b, which RE2 cannot spell. Onigmo's ^ does not match at the end after a
// final newline, and its \b's word characters are Unicode's (Ruby's
// WORD_BOUND_ALL_RANGE; \w stays ASCII). Subjects where they could change
// the match run on prog, the pattern's program under those readings; prog
// is nil when the pattern has neither.
type rxRegexp struct {
	*regexp.Regexp
	prog        *syntax.Prog
	caret, word bool
}

// rxNew wraps re, compiled from a translated pattern (after rxFold).
func rxNew(re *regexp.Regexp) *rxRegexp {
	r := &rxRegexp{Regexp: re}
	tree, _ := syntax.Parse(re.String(), syntax.Perl)
	var walk func(*syntax.Regexp)
	walk = func(t *syntax.Regexp) {
		r.caret = r.caret || t.Op == syntax.OpBeginLine
		r.word = r.word || t.Op == syntax.OpWordBoundary || t.Op == syntax.OpNoWordBoundary
		for _, sub := range t.Sub {
			walk(sub)
		}
	}
	walk(tree)
	if r.caret || r.word {
		r.prog, _ = syntax.Compile(tree.Simplify())
	}
	return r
}

func (r *rxRegexp) MatchString(s string) bool {
	if r.prog == nil {
		return r.Regexp.MatchString(s)
	}
	return r.FindStringIndex(s) != nil
}

func (r *rxRegexp) FindStringIndex(s string) []int {
	loc := r.Regexp.FindStringIndex(s)
	if r.differs(s, loc) {
		if loc = r.backtrack(s); loc != nil {
			loc = loc[:2]
		}
	}
	return loc
}

func (r *rxRegexp) FindStringSubmatchIndex(s string) []int {
	loc := r.Regexp.FindStringSubmatchIndex(s)
	if r.differs(s, loc) {
		loc = r.backtrack(s)
	}
	return loc
}

// differs reports whether Onigmo could match s otherwise than RE2 did (loc):
// s has a non-ASCII word character, for \b; or RE2's match ends at a final
// newline, where its ^ may have matched and Onigmo's cannot. Onigmo's ^
// matches nowhere RE2's does not, so any other RE2 match is Onigmo's first.
func (r *rxRegexp) differs(s string, loc []int) bool {
	if r.word && strings.ContainsFunc(s, func(c rune) bool { return c >= utf8.RuneSelf && rxWord(c) }) {
		return true
	}
	return r.caret && loc != nil && loc[1] == len(s) && strings.HasSuffix(s, "\n")
}

// rxWord is Onigmo's Unicode word character: Alphabetic, Mark,
// Decimal_Number or Connector_Punctuation.
func rxWord(c rune) bool {
	return unicode.In(c, unicode.L, unicode.Nl, unicode.Other_Alphabetic, unicode.M, unicode.Nd, unicode.Pc)
}

// rxEmpty reports whether the empty-width assertions op hold at pos, as
// Onigmo reads them.
func rxEmpty(op syntax.EmptyOp, s string, pos int) bool {
	before, _ := utf8.DecodeLastRuneInString(s[:pos])
	after, _ := utf8.DecodeRuneInString(s[pos:])
	flags := syntax.EmptyNoWordBoundary
	if rxWord(before) != rxWord(after) {
		flags = syntax.EmptyWordBoundary
	}
	if pos == 0 {
		flags |= syntax.EmptyBeginText | syntax.EmptyBeginLine
	} else if before == '\n' && pos < len(s) {
		flags |= syntax.EmptyBeginLine
	}
	if pos == len(s) {
		flags |= syntax.EmptyEndText | syntax.EmptyEndLine
	} else if after == '\n' {
		flags |= syntax.EmptyEndLine
	}
	return op&^flags == 0
}

// backtrack is regexp/backtrack.go's bit-state backtracker over prog:
// leftmost-first, as RE2 is, with rxEmpty's assertions.
// ponytail: visited is len(prog)×len(s) bits; a Pike VM would bound it for huge subjects.
func (r *rxRegexp) backtrack(s string) []int {
	b := rxBacktrack{
		prog:    r.prog,
		s:       s,
		visited: make([]uint64, (len(r.prog.Inst)*(len(s)+1)+63)/64),
		caps:    make([]int, 2*r.NumSubexp()+2),
	}
	for start := 0; !b.try(start); {
		if start == len(s) {
			return nil
		}
		_, w := utf8.DecodeRuneInString(s[start:])
		start += w
	}
	return b.caps
}

type rxBacktrack struct {
	prog    *syntax.Prog
	s       string
	visited []uint64 // (pc, pos) pairs already tried: they cannot match
	caps    []int
	jobs    []rxJob
}

// rxJob is a lower-priority thread to try, or (slot >= 0) a capture to
// restore to pos when the threads after it have failed.
type rxJob struct{ pc, pos, slot int }

// try reports whether a match starts at start, leaving it in caps.
func (b *rxBacktrack) try(start int) bool {
	for i := range b.caps {
		b.caps[i] = -1
	}
	b.caps[0] = start
	b.jobs = append(b.jobs[:0], rxJob{b.prog.Start, start, -1})
	for len(b.jobs) > 0 {
		j := b.jobs[len(b.jobs)-1]
		b.jobs = b.jobs[:len(b.jobs)-1]
		if j.slot >= 0 {
			b.caps[j.slot] = j.pos
		} else if b.thread(j.pc, j.pos) {
			return true
		}
	}
	return false
}

// thread runs one thread until it fails or matches, queueing the
// alternatives it passes.
func (b *rxBacktrack) thread(pc, pos int) bool {
	for {
		k := pc*(len(b.s)+1) + pos
		if b.visited[k/64]&(1<<(k%64)) != 0 {
			return false
		}
		b.visited[k/64] |= 1 << (k % 64)
		inst := &b.prog.Inst[pc]
		switch inst.Op {
		case syntax.InstMatch:
			b.caps[1] = pos
			return true
		case syntax.InstFail:
			return false
		case syntax.InstAlt, syntax.InstAltMatch:
			b.jobs = append(b.jobs, rxJob{int(inst.Arg), pos, -1})
		case syntax.InstCapture:
			if slot := int(inst.Arg); slot < len(b.caps) {
				b.jobs = append(b.jobs, rxJob{0, b.caps[slot], slot})
				b.caps[slot] = pos
			}
		case syntax.InstEmptyWidth:
			if !rxEmpty(syntax.EmptyOp(inst.Arg), b.s, pos) { //nolint:gosec // an EmptyWidth Arg holds EmptyOp bits
				return false
			}
		case syntax.InstNop:
		case syntax.InstRune, syntax.InstRune1, syntax.InstRuneAny, syntax.InstRuneAnyNotNL:
			c, w := utf8.DecodeRuneInString(b.s[pos:])
			if w == 0 || !inst.MatchRune(c) {
				return false
			}
			pos += w
		}
		pc = int(inst.Out)
	}
}

// rxFold expands pattern's case-insensitive literals to also match
// Unicode's multi-character case folds, as Onigmo's /i does and RE2's does
// not: "ss" and "ß", "ff" and "ﬀ". Other patterns are returned unchanged.
func rxFold(pattern string) string {
	tree, err := syntax.Parse(pattern, syntax.Perl)
	if err != nil {
		return pattern // for regexp.Compile to report
	}
	root := &syntax.Regexp{Op: syntax.OpConcat, Sub: []*syntax.Regexp{tree}}
	changed := false
	var walk func(*syntax.Regexp)
	walk = func(t *syntax.Regexp) {
		for i, sub := range t.Sub {
			if sub.Op == syntax.OpLiteral && sub.Flags&syntax.FoldCase != 0 {
				if e := rxFoldLiteral(sub); e != nil {
					t.Sub[i], changed = e, true
				}
			}
			walk(sub)
		}
	}
	walk(root)
	if !changed {
		return pattern
	}
	return root.String()
}

// rxFoldLiteral is lit, a case-insensitive literal, spelled to match every
// string whose full case folding is lit's, or nil when that is lit itself.
// It is cut where no multi-character fold spans the cut, and each piece
// lists its alternatives; a piece stops growing at 8 runes (Onigmo also
// caps its expansion) to bound the product.
func rxFoldLiteral(lit *syntax.Regexp) *syntax.Regexp {
	var f []rune // lit's runes folded, as minimal fold runes like lit's own
	for _, c := range lit.Rune {
		if m, ok := rxFoldMulti[c]; ok {
			f = append(f, []rune(strings.Map(rxMinFold, m))...)
		} else {
			f = append(f, c)
		}
	}
	out := &syntax.Regexp{Op: syntax.OpConcat, Flags: lit.Flags}
	multi := false
	for i := 0; i < len(f); {
		j := i + 1
		for p := i; p < j && j-i < 8; p++ {
			for k := 2; k <= 3 && p+k <= len(f); k++ {
				if rxUnfold[string(f[p:p+k])] != nil {
					j, multi = max(j, p+k), true
				}
			}
		}
		out.Sub = append(out.Sub, rxFoldAlts(f[i:j], lit.Flags))
		i = j
	}
	if !multi {
		return nil
	}
	return out
}

// rxFoldAlts matches the strings that fold to f (not empty): its first rune,
// or a rune whose multi-character fold is a prefix of f, then the rest.
func rxFoldAlts(f []rune, flags syntax.Flags) *syntax.Regexp {
	alt := &syntax.Regexp{Op: syntax.OpAlternate, Flags: flags}
	add := func(c rune, rest []rune) {
		e := &syntax.Regexp{Op: syntax.OpLiteral, Flags: flags, Rune: []rune{c}}
		if len(rest) > 0 {
			e = &syntax.Regexp{Op: syntax.OpConcat, Flags: flags, Sub: []*syntax.Regexp{e, rxFoldAlts(rest, flags)}}
		}
		alt.Sub = append(alt.Sub, e)
	}
	add(f[0], f[1:])
	for k := 2; k <= 3 && k <= len(f); k++ {
		for _, c := range rxUnfold[string(f[:k])] {
			add(c, f[k:])
		}
	}
	if len(alt.Sub) == 1 {
		return alt.Sub[0]
	}
	return alt
}

// rxMinFold is the least rune c simply case-folds to, as RE2's parser
// stores a case-insensitive literal's runes.
func rxMinFold(c rune) rune {
	m := c
	for d := unicode.SimpleFold(c); d != c; d = unicode.SimpleFold(d) {
		m = min(m, d)
	}
	return m
}

// rxUnfold maps a multi-character fold, as minimal fold runes, to the
// minimal fold runes that fold to it.
var rxUnfold = func() map[string][]rune {
	m := map[string][]rune{}
	for c, f := range rxFoldMulti {
		if c != rxMinFold(c) {
			continue
		}
		k := strings.Map(rxMinFold, f)
		m[k] = append(m[k], c)
	}
	for _, cs := range m {
		slices.Sort(cs)
	}
	return m
}()

// rxFoldMulti is Unicode's multi-character case folding (CaseFolding.txt,
// status F).
var rxFoldMulti = map[rune]string{
	0xdf: "ss", 0x130: "i\u0307", 0x149: "\u02bcn", 0x1f0: "j\u030c", 0x390: "\u03b9\u0308\u0301",
	0x3b0: "\u03c5\u0308\u0301", 0x587: "\u0565\u0582", 0x1e96: "h\u0331", 0x1e97: "t\u0308", 0x1e98: "w\u030a",
	0x1e99: "y\u030a", 0x1e9a: "a\u02be", 0x1e9e: "ss", 0x1f50: "\u03c5\u0313", 0x1f52: "\u03c5\u0313\u0300",
	0x1f54: "\u03c5\u0313\u0301", 0x1f56: "\u03c5\u0313\u0342", 0x1f80: "\u1f00\u03b9", 0x1f81: "\u1f01\u03b9", 0x1f82: "\u1f02\u03b9",
	0x1f83: "\u1f03\u03b9", 0x1f84: "\u1f04\u03b9", 0x1f85: "\u1f05\u03b9", 0x1f86: "\u1f06\u03b9", 0x1f87: "\u1f07\u03b9",
	0x1f88: "\u1f00\u03b9", 0x1f89: "\u1f01\u03b9", 0x1f8a: "\u1f02\u03b9", 0x1f8b: "\u1f03\u03b9", 0x1f8c: "\u1f04\u03b9",
	0x1f8d: "\u1f05\u03b9", 0x1f8e: "\u1f06\u03b9", 0x1f8f: "\u1f07\u03b9", 0x1f90: "\u1f20\u03b9", 0x1f91: "\u1f21\u03b9",
	0x1f92: "\u1f22\u03b9", 0x1f93: "\u1f23\u03b9", 0x1f94: "\u1f24\u03b9", 0x1f95: "\u1f25\u03b9", 0x1f96: "\u1f26\u03b9",
	0x1f97: "\u1f27\u03b9", 0x1f98: "\u1f20\u03b9", 0x1f99: "\u1f21\u03b9", 0x1f9a: "\u1f22\u03b9", 0x1f9b: "\u1f23\u03b9",
	0x1f9c: "\u1f24\u03b9", 0x1f9d: "\u1f25\u03b9", 0x1f9e: "\u1f26\u03b9", 0x1f9f: "\u1f27\u03b9", 0x1fa0: "\u1f60\u03b9",
	0x1fa1: "\u1f61\u03b9", 0x1fa2: "\u1f62\u03b9", 0x1fa3: "\u1f63\u03b9", 0x1fa4: "\u1f64\u03b9", 0x1fa5: "\u1f65\u03b9",
	0x1fa6: "\u1f66\u03b9", 0x1fa7: "\u1f67\u03b9", 0x1fa8: "\u1f60\u03b9", 0x1fa9: "\u1f61\u03b9", 0x1faa: "\u1f62\u03b9",
	0x1fab: "\u1f63\u03b9", 0x1fac: "\u1f64\u03b9", 0x1fad: "\u1f65\u03b9", 0x1fae: "\u1f66\u03b9", 0x1faf: "\u1f67\u03b9",
	0x1fb2: "\u1f70\u03b9", 0x1fb3: "\u03b1\u03b9", 0x1fb4: "\u03ac\u03b9", 0x1fb6: "\u03b1\u0342", 0x1fb7: "\u03b1\u0342\u03b9",
	0x1fbc: "\u03b1\u03b9", 0x1fc2: "\u1f74\u03b9", 0x1fc3: "\u03b7\u03b9", 0x1fc4: "\u03ae\u03b9", 0x1fc6: "\u03b7\u0342",
	0x1fc7: "\u03b7\u0342\u03b9", 0x1fcc: "\u03b7\u03b9", 0x1fd2: "\u03b9\u0308\u0300", 0x1fd3: "\u03b9\u0308\u0301", 0x1fd6: "\u03b9\u0342",
	0x1fd7: "\u03b9\u0308\u0342", 0x1fe2: "\u03c5\u0308\u0300", 0x1fe3: "\u03c5\u0308\u0301", 0x1fe4: "\u03c1\u0313", 0x1fe6: "\u03c5\u0342",
	0x1fe7: "\u03c5\u0308\u0342", 0x1ff2: "\u1f7c\u03b9", 0x1ff3: "\u03c9\u03b9", 0x1ff4: "\u03ce\u03b9", 0x1ff6: "\u03c9\u0342",
	0x1ff7: "\u03c9\u0342\u03b9", 0x1ffc: "\u03c9\u03b9", 0xfb00: "ff", 0xfb01: "fi", 0xfb02: "fl",
	0xfb03: "ffi", 0xfb04: "ffl", 0xfb05: "st", 0xfb06: "st", 0xfb13: "\u0574\u0576",
	0xfb14: "\u0574\u0565", 0xfb15: "\u0574\u056b", 0xfb16: "\u057e\u0576", 0xfb17: "\u0574\u056d",
}
