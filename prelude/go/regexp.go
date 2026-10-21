//go:build rb2go_prelude

// Package prelude is concatenated verbatim into the output (loadPreludeGo). `go vet -tags rb2go_prelude ./prelude/go` type-checks it against generated stubs for the types generated code declares (0_stubs.go, decision 154).
package prelude

import (
	"errors"
	"fmt"
	"regexp"
	"regexp/syntax"
	"slices"
	"strings"
	"sync/atomic"
	"unicode/utf8"
)

// rbRegexpOf wraps a compiled pattern; anch is the slot StringScanner fills with the \A-anchored twin, so the twin lives and dies with the Regexp.
func rbRegexpOf(re *rxRegexp, src, opts string) *Regexp {
	return &Regexp{re: re, src: src, opts: opts, anch: new(atomic.Pointer[rxRegexp])}
}

// rbRegexpNew compiles a translated pattern; dynamic (interpolated) ones
// may fail at run time, like Ruby's.
func rbRegexpNew(pattern, src, opts string) *Regexp {
	re, err := rxCompile(rxFold(pattern))
	if err != nil {
		panic(NewRegexpError(Ref(String(rbRegexpErr(err, src)))))
	}
	return rbRegexpOf(re, src, opts)
}

// rbRegexpNewAfter is rbRegexpNew for a pattern variable that must initialize after the regexp2 engine registers (decision 242).
func rbRegexpNewAfter(_ bool, pattern, src, opts string) *Regexp {
	return rbRegexpNew(pattern, src, opts)
}

// rbRegexpLenient is a gem's regexp (decision 176): MRI builds it, so rb2go does too, and one RE2 cannot match raises
// RegexpError when it is used rather than when the gem loads (Rack::Lint's URI patterns).
func rbRegexpLenient(prefix, src, opts string) *Regexp {
	pat, err := rxPrepare(src, strings.Contains(opts, "x"))
	var re *rxRegexp
	if err == nil {
		re, err = rxCompile(rxFold(prefix + pat))
	}
	if err != nil {
		return rbRegexpOf(&rxRegexp{unsupported: "regexp /" + src + "/ is not supported by rb2go: " + err.Error()}, src, opts)
	}
	return rbRegexpOf(re, src, opts)
}

// rbRegexpErr words a Go regexp syntax error as Onigmo words the same
// mistake, with the Ruby source after it (`end pattern with unmatched
// parenthesis: /a(b/`); an error with no Onigmo twin keeps Go's wording.
func rbRegexpErr(err error, src string) string {
	var se *syntax.Error
	if !errors.As(err, &se) {
		return err.Error()
	}
	msg := ""
	//exhaustive:ignore // the codes with an Onigmo twin; the rest keep Go's wording
	switch se.Code {
	case syntax.ErrMissingParen:
		msg = "end pattern with unmatched parenthesis"
	case syntax.ErrUnexpectedParen:
		msg = "unmatched close parenthesis"
	case syntax.ErrMissingBracket:
		msg = "premature end of char-class"
	case syntax.ErrMissingRepeatArgument:
		msg = "target of repeat operator is not specified"
	case syntax.ErrTrailingBackslash:
		msg = "too short escape sequence"
	case syntax.ErrInvalidCharRange:
		msg = "empty range in char class"
	case syntax.ErrInvalidRepeatSize:
		if lo, hi, ok := strings.Cut(strings.Trim(se.Expr, "{}"), ","); ok && hi != "" && len(hi) <= len(lo) && hi < lo {
			msg = "upper is smaller than lower in repeat range"
		}
	}
	if msg == "" {
		return err.Error()
	}
	return msg + ": /" + rbRegexpDesc(src) + "/"
}

// rbRegexpDyn compiles an interpolated regexp from its Ruby source, which
// only exists at run time; translateRegexp is the compiler's own
// (internal/compiler/rxtranslate.go, emitted into every program).
func rbRegexpDyn(prefix, src, opts string) *Regexp {
	// the interpolated values' spacing too under /x, as MRI's; an inline (?x-mi:...) from a value either way
	pat, err := rxPrepare(src, strings.Contains(opts, "x"))
	if err != nil {
		panic(NewRegexpError(Ref(String(err.Error()))))
	}
	return rbRegexpNew(prefix+pat, src, opts)
}

// rbRegexpFromValue is Regexp.new(pattern, options): a Regexp is copied
// (options ignored, as MRI warns); a String is translated like an
// interpolated literal's source.
func rbRegexpFromValue(pattern, options any) *Regexp {
	switch p := rbUnbox(pattern).(type) {
	case *Regexp:
		return p
	case String:
		ignore, multi, ext, noenc := false, false, false, false
		switch o := rbUnbox(options).(type) {
		case nil:
		case Boolean:
			ignore = bool(o)
		case Integer:
			ignore, multi, ext, noenc = o&1 != 0, o&4 != 0, o&2 != 0, o&32 != 0
		case String:
			for _, f := range string(o) {
				switch f {
				case 'i':
					ignore = true
				case 'm':
					multi = true
				case 'x':
					ext = true
				default:
					panic(NewArgumentError(Ref(String("unknown regexp option: " + string(o)))))
				}
			}
		default:
			ignore = true // any other truthy value is /i, as MRI
		}
		prefix, opts := "(?m", ""
		if ignore {
			prefix += "i"
		}
		if multi {
			prefix += "s"
			opts += "m"
		}
		if ignore {
			opts += "i"
		}
		if ext {
			opts += "x"
		}
		if noenc { // ponytail: only shown (/n); matching stays UTF-8 as RE2's
			opts += "n"
		}
		return rbRegexpDyn(prefix+")", string(p), opts)
	}
	panic(NewTypeError(Ref(String("no implicit conversion of " + rbClassName(pattern) + " into String"))))
}

// rbRegexpDesc is the source as inspect and to_s show it: a bare / is
// escaped, as MRI's rb_reg_desc does.
func rbRegexpDesc(src string) string {
	var b strings.Builder
	for i := 0; i < len(src); i++ {
		switch {
		case src[i] == '\\' && i+1 < len(src):
			b.WriteString(src[i : i+2])
			i++
		case src[i] == '/':
			b.WriteString(`\/`)
		default:
			b.WriteByte(src[i])
		}
	}
	return b.String()
}

func rbMatch(r *Regexp, s string) *MatchData {
	loc := rbFindLoc(r, s)
	if loc == nil {
		return nil
	}
	return rbMatchAt(r, s, loc)
}

// rbFindLoc is r's first match of s with its groups, as byte offsets. A binary Regexp (/n) reads a String that is not
// valid UTF-8 byte by byte: each byte is the character of its value (Latin-1), so `/\xFD7zXZ/n` matches the byte 0xFD
// (Homebrew's UnpackStrategy::Xz.can_extract?), and the offsets are mapped back to the bytes (decision 324). Any other
// Regexp raises ArgumentError on it, as MRI's does on a UTF-8 String with a bad byte.
func rbFindLoc(r *Regexp, s string) []int {
	if utf8.ValidString(s) {
		return r.re.FindStringSubmatchIndex(s)
	}
	if !strings.ContainsRune(r.opts, 'n') { // MRI's UTF-8 Regexp on a broken UTF-8 String
		panic(NewArgumentError(Ref(String("invalid byte sequence in UTF-8"))))
	}
	var b strings.Builder
	back := make([]int, 0, len(s)*2+1) // back[j]: the byte of s where Latin-1 offset j starts
	for i := range len(s) {
		n := b.Len()
		b.WriteRune(rune(s[i]))
		for range b.Len() - n {
			back = append(back, i)
		}
	}
	back = append(back, len(s))
	loc := r.re.FindStringSubmatchIndex(b.String())
	for i, x := range loc {
		if x >= 0 {
			loc[i] = back[x]
		}
	}
	return loc
}

// rbMatches is whether r matches s: Go's MatchString, or rbFindLoc's byte reading of a binary String.
func rbMatches(r *Regexp, s string) bool {
	if utf8.ValidString(s) {
		return r.re.MatchString(s)
	}
	return rbFindLoc(r, s) != nil
}

// rbMatchAt is the MatchData of r's match of s at loc.
func rbMatchAt(r *Regexp, s string, loc []int) *MatchData {
	groups := make([]*String, len(loc)/2)
	for i := range groups {
		if loc[2*i] >= 0 {
			g := String(s[loc[2*i]:loc[2*i+1]])
			groups[i] = &g
		}
	}
	return &MatchData{groups: groups, names: r.re.SubexpNames(), pre: s[:loc[0]], post: s[loc[1]:], subj: s, loc: loc}
}

// rbSubject is the text a Regexp matches: a String, or a Symbol's name.
// nil matches nothing (ok is false); anything else is MRI's TypeError, and
// a String that is not valid UTF-8 is MRI's ArgumentError.
func rbSubject(v any) (string, bool) {
	switch s := rbUnbox(v).(type) { // a String? held untyped is a *String box (liquid's `markup = $2; markup =~ re`, decision 331)
	case nil:
		return "", false
	case String: // not valid UTF-8: ASCII-8BIT (decision 136), matched byte by byte (rbFindLoc)
		return string(s), true
	case Symbol:
		return string(s), true
	}
	panic(NewTypeError(Ref(String("no implicit conversion of " + rbClassName(v) + " into String"))))
}

// rbPattern is a sub/gsub pattern: a Regexp, or a String matched literally.
func rbPattern(p any) *Regexp {
	switch v := rbUnbox(p).(type) {
	case *Regexp:
		return v
	case String:
		return rbRegexpNew(regexp.QuoteMeta(string(v)), string(v), "")
	}
	panic(NewTypeError(Ref(String("wrong argument type " + rbClassName(p) + " (expected Regexp)"))))
}

// rbReRepl expands a replacement string's back-references for one match.
func rbReRepl(rep string) func(s string, loc []int, names []string) string {
	return func(s string, loc []int, names []string) string {
		group := func(i int) string {
			if 2*i+1 >= len(loc) || loc[2*i] < 0 {
				return ""
			}
			return s[loc[2*i]:loc[2*i+1]]
		}
		var b strings.Builder
		for j := 0; j < len(rep); j++ {
			if rep[j] != '\\' || j+1 == len(rep) {
				b.WriteByte(rep[j])
				continue
			}
			j++
			switch c := rep[j]; {
			case c == '&':
				b.WriteString(group(0))
			case c >= '0' && c <= '9':
				b.WriteString(group(int(c - '0')))
			case c == '+': // the last group that matched, empty with none
				for i := len(loc)/2 - 1; i >= 1; i-- {
					if loc[2*i] >= 0 {
						b.WriteString(group(i))
						break
					}
				}
			case c == '`':
				b.WriteString(s[:loc[0]])
			case c == '\'':
				b.WriteString(s[loc[1]:])
			case c == '\\':
				b.WriteByte('\\')
			case c == 'k' && j+1 < len(rep) && rep[j+1] == '<':
				end := strings.IndexByte(rep[j:], '>')
				if end < 0 {
					b.WriteString(rep[j-1 : j+1])
					continue
				}
				name := rep[j+2 : j+end]
				if i := slices.Index(names, name); i > 0 {
					b.WriteString(group(i))
				}
				j += end
			default:
				b.WriteString(rep[j-1 : j+1])
			}
		}
		return b.String()
	}
}

// rbReBlock replaces each match with the block's value, as a String.
func rbReBlock(blk func(String) any) func(s string, loc []int, names []string) string {
	return func(s string, loc []int, _ []string) string {
		return string(rbToS(blk(String(s[loc[0]:loc[1]]))))
	}
}

// rbReBlockLM is rbReBlock in a frame that reads $~ or $1: each match is the frame's last match while its block runs
// (decision 241).
func rbReBlockLM(lm ***MatchData, re *Regexp, blk func(String) any) func(s string, loc []int, names []string) string {
	return func(s string, loc []int, _ []string) string {
		m := rbMatchAt(re, s, loc)
		*lm = &m
		return string(rbToS(blk(String(s[loc[0]:loc[1]]))))
	}
}

// rbReSub replaces the first n matches of re in s (all when n < 0); rxRegexp's FindAll applies the Onigmo corrections.
func rbReSub(re *Regexp, s string, n int, repl func(s string, loc []int, names []string) string) string {
	var b strings.Builder
	last := 0
	for _, loc := range re.re.FindAllStringSubmatchIndex(s, n) {
		b.WriteString(s[last:loc[0]])
		b.WriteString(repl(s, loc, re.re.SubexpNames()))
		last = loc[1]
	}
	b.WriteString(s[last:])
	return b.String()
}

// rbRegexpEscape is Regexp.escape (MRI's rb_reg_quote).
func rbRegexpEscape(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch r {
		case '[', ']', '{', '}', '(', ')', '|', '-', '*', '.', '\\', '?', '+', '^', '$', '#':
			b.WriteByte('\\')
			b.WriteRune(r)
		case ' ':
			b.WriteString(`\ `)
		case '\t':
			b.WriteString(`\t`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\f':
			b.WriteString(`\f`)
		case '\v':
			b.WriteString(`\v`)
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// rbUniqNames is the named groups in order, each once (MRI's Regexp#names).
func rbUniqNames(names []string) *Array[String] {
	out := &Array[String]{}
	for _, n := range names {
		if n != "" && !slices.Contains(out.s, String(n)) {
			out.s = append(out.s, String(n))
		}
	}
	return out
}

// rbMatchNamed is MatchData#[] by group name: the last group of that name that matched, as Onigmo picks.
func rbMatchNamed(m *MatchData, name string) *String {
	found := false
	var out *String
	for i, n := range m.names {
		if n == name {
			found = true
			if m.groups[i] != nil {
				out = m.groups[i]
			}
		}
	}
	if !found {
		panic(NewIndexError(Ref(String("undefined group name reference: " + name))))
	}
	return out
}

// rbMatchOffset is MatchData#begin (side 0) or #end (side 1) in characters.
func rbMatchOffset(m *MatchData, n Integer, side int) *Integer {
	if n < 0 || int(n) >= len(m.groups) {
		panic(NewIndexError(Ref(String(fmt.Sprintf("index %d out of matches", n)))))
	}
	b := m.loc[2*int(n)+side]
	if b < 0 {
		return nil
	}
	return Ref(Integer(utf8.RuneCountInString(m.subj[:b])))
}

// rbMatchCapture is a named group's local after `/(?<name>..)/ =~ s`: nil without a match.
func rbMatchCapture(m **MatchData, name string) *String {
	if m == nil {
		return nil
	}
	return rbMatchNamed(*m, name)
}

// rbMatchPos is that =~'s value: the match's character index, or nil.
func rbMatchPos(m **MatchData) *Integer {
	if m == nil {
		return nil
	}
	return rbMatchOffset(*m, 0, 0)
}

// rbMatchSet is `=~` in a frame that reads $~ (decision 171): it records m as the frame's last match and
// answers the match's character index, or nil.
func rbMatchSet(lm ***MatchData, m **MatchData) *Integer {
	*lm = m
	if m == nil {
		return nil
	}
	return Ref(Integer(utf8.RuneCountInString((*m).subj[:(*m).loc[0]])))
}

// rbLMGroup is $n (or $& for 0): group n of the last match, nil without one.
func rbLMGroup(m **MatchData, n int) *String {
	if m == nil || n >= len((*m).groups) {
		return nil
	}
	return (*m).groups[n]
}

// rbMatchKeep is `match` in a frame that reads $~: the MatchData, also kept as the last match.
func rbMatchKeep(lm ***MatchData, m **MatchData) **MatchData {
	*lm = m
	return m
}

// rbGrepLM is `xs.grep(re) { ... }` / grep_v in a frame that reads $~: each test sets the frame's last match before
// the block runs, as MRI's grep_iter_i (a non-String, non-Symbol element is a failed test, nil match).
func rbGrepLM[E any, U comparable](lm ***MatchData, re *Regexp, xs []E, invert bool, blk func(E) U) *Array[U] {
	out := &Array[U]{}
	for _, x := range xs {
		var m **MatchData
		switch any(x).(type) {
		case String, Symbol:
			s, _ := rbSubject(any(x))
			if md := rbMatch(re, s); md != nil {
				m = &md
			}
		}
		*lm = m
		if (m != nil) != invert {
			out.s = append(out.s, blk(x))
		}
	}
	return out
}
