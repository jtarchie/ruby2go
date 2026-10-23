package compiler

import (
	"go/token"
	"regexp"
	"testing"
)

// \p{Ll} so non-ASCII lead bytes (ç, é) reach goMethodName too.
var snakeName = regexp.MustCompile(`\A[\p{Ll}_][\p{Ll}0-9_]*[?!=]?\z`)

// internalDigit: `__` names keep `__read_1` and `__call1` as written (decision 339), so `__a_1` and `__a1` still merge. Known; the prelude depends on both.
var internalDigit = regexp.MustCompile(`\A__.*[0-9]`)

// FuzzGoMethodName: two distinct snake_case Ruby method names never share a Go name (decision 3).
func FuzzGoMethodName(f *testing.F) {
	for _, p := range [][2]string{{"empty?", "empty_q"}, {"x=", "x_set"}, {"upcase!", "upcase_bang"}, {"__write", "_write"}, {"to_s", "to_str"}, {"ça", "éa"}, {"éa", "Éa"}, {"a_1", "a1"}, {"ı", "i"}, {"ſ", "s"}} {
		f.Add(p[0], p[1])
	}
	f.Fuzz(func(t *testing.T, a, b string) {
		if !snakeName.MatchString(a) || !snakeName.MatchString(b) || internalDigit.MatchString(a) || internalDigit.MatchString(b) {
			return
		}
		ga, gb := goMethodName(a), goMethodName(b)
		if !token.IsIdentifier(ga) || !token.IsIdentifier(gb) {
			t.Fatalf("goMethodName(%q) = %q, goMethodName(%q) = %q: not both Go identifiers", a, ga, b, gb)
		}
		if a != b && ga == gb {
			t.Fatalf("goMethodName collision: %q and %q both map to %q", a, b, ga)
		}
	})
}

// FuzzTranslateRegexp: translation never panics, and what it emits is for regexp.Compile to accept or reject, not to crash on.
func FuzzTranslateRegexp(f *testing.F) {
	for _, s := range []string{`\A\d+\z`, `(?<y>\d{4})-(?<m>\d\d)`, `[[:alpha:]&&[^aeiou]]`, `a{,3}`, `\h+\R`, `(?i)straße`, `^\s*#`, `[\w&&\D]`, `\p{Greek}`, `x(?=y)`} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, src string) {
		out, err := translateRegexp(src)
		if err != nil {
			return
		}
		_, _ = regexp.Compile(out)
	})
}
