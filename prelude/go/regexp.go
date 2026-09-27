//go:build ignore

// Package prelude is concatenated verbatim into the output (loadPreludeGo), never built for real: types like String come from generated code.
package prelude

// rbRegexpNew compiles a translated pattern; dynamic (interpolated) ones
// may fail at run time, like Ruby's.
func rbRegexpNew(pattern, src, opts string) *Regexp {
	re, err := regexp.Compile(rxFold(pattern))
	if err != nil {
		panic(NewRegexpError(Ref(String(err.Error()))))
	}
	return &Regexp{re: rxNew(re), src: src, opts: opts}
}

// rbRegexpDyn compiles an interpolated regexp from its Ruby source, which
// only exists at run time; translateRegexp is the compiler's own
// (internal/compiler/rxtranslate.go, emitted into every program).
func rbRegexpDyn(prefix, src, opts string) *Regexp {
	pat, err := translateRegexp(src)
	if err != nil {
		panic(NewRegexpError(Ref(String(err.Error()))))
	}
	return rbRegexpNew(prefix+pat, src, opts)
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
	return &MatchData{groups: groups, names: r.re.SubexpNames(), pre: s[:loc[0]], post: s[loc[1]:]}
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
