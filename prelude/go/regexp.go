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
	"unicode/utf8"
)

// rbRegexpNew compiles a translated pattern; dynamic (interpolated) ones
// may fail at run time, like Ruby's.
func rbRegexpNew(pattern, src, opts string) *Regexp {
	re, err := regexp.Compile(rxFold(pattern))
	if err != nil {
		panic(NewRegexpError(Ref(String(rbRegexpErr(err, src)))))
	}
	return &Regexp{re: rxNew(re), src: src, opts: opts}
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
	pat, err := translateRegexp(stripExtended(src, strings.Contains(opts, "x")))
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
		ignore, multi, noenc := false, false, false
		switch o := rbUnbox(options).(type) {
		case nil:
		case Boolean:
			ignore = bool(o)
		case Integer:
			ignore, multi, noenc = o&1 != 0, o&4 != 0, o&32 != 0
			if o&2 != 0 {
				rbRegexpNoExtended()
			}
		case String:
			for _, f := range string(o) {
				switch f {
				case 'i':
					ignore = true
				case 'm':
					multi = true
				case 'x':
					rbRegexpNoExtended()
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
		if noenc { // ponytail: only shown (/n); matching stays UTF-8 as RE2's
			opts += "n"
		}
		return rbRegexpDyn(prefix+")", string(p), opts)
	}
	panic(NewTypeError(Ref(String("no implicit conversion of " + rbClassName(pattern) + " into String"))))
}

func rbRegexpNoExtended() {
	panic(NewNotImplementedError(Ref(String("rb2go: extended (x) regexps are not supported (docs/design.md decision 24)"))))
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
	loc := r.re.FindStringSubmatchIndex(s)
	if loc == nil {
		return nil
	}
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
	switch s := v.(type) {
	case nil:
		return "", false
	case String:
		if !utf8.ValidString(string(s)) {
			panic(NewArgumentError(Ref(String("invalid byte sequence in UTF-8"))))
		}
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

// rbReSub replaces the first n matches of re in s (all when n < 0).
// ponytail: matches come from RE2's FindAll, so the Onigmo corrections of
// rxRegexp (Unicode \b, ^ before a final newline) do not apply here.
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
