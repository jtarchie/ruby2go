package compiler

import (
	"fmt"
	"regexp"
	"strings"
	"testing"
)

// compileRuby mirrors genRegexp's static path: translate, then compile under the flag prefix.
func compileRuby(src string, f rubyRegexFlags) (*regexp.Regexp, error) {
	pat, err := translateRegexp(src)
	if err != nil {
		return nil, err
	}
	re, err := regexp.Compile(f.goPrefix() + pat)
	if err != nil {
		return nil, fmt.Errorf("RE2: %w", err)
	}
	return re, nil
}

func TestRegexpFlags(t *testing.T) {
	cases := []struct {
		name         string
		f            rubyRegexFlags
		prefix, opts string
	}{
		{"none", rubyRegexFlags{}, "(?m)", ""},
		{"i", rubyRegexFlags{ignoreCase: true}, "(?mi)", "i"},
		{"m", rubyRegexFlags{multiline: true}, "(?ms)", "m"},
		// MRI prints /a/im as /a/mi
		{"mi", rubyRegexFlags{ignoreCase: true, multiline: true}, "(?mis)", "mi"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := c.f.goPrefix(); got != c.prefix {
				t.Errorf("goPrefix() = %q, want %q", got, c.prefix)
			}
			if got := c.f.opts(); got != c.opts {
				t.Errorf("opts() = %q, want %q", got, c.opts)
			}
		})
	}
}

func TestTranslateRegexp(t *testing.T) {
	cases := []struct{ in, want string }{
		{`abc`, `abc`},
		{`\h`, `[0-9a-fA-F]`},
		{`\h+`, `[0-9a-fA-F]+`},
		{`[\h_]`, `[0-9a-fA-F_]`},
		{`[^\h]`, `[^0-9a-fA-F]`},
		{`\H`, `[^0-9a-fA-F]`},
		{`[a][\h]\h`, `[a][0-9a-fA-F][0-9a-fA-F]`},
		{`[[:alpha:]\h]`, "[" + rxPosix["alpha"] + "0-9a-fA-F]"},
		{`[\]h]`, `[\]h]`},
		{`\\h`, `\\h`},
		{`\d\w\s\b\A\z`, `\d\w[\t\n\v\f\r ]\b\A\z`},
		{`a\/b`, `a\/b`},
		{`trailing\`, `trailing\`},
		{`(?<y>\d+)`, `(?<y>\d+)`},
		{`a]`, `a]`},
		{`[a]\H`, `[a][^0-9a-fA-F]`},
		{`[^\]]\h`, `[^\]][0-9a-fA-F]`},
		{`\\\h`, `\\[0-9a-fA-F]`},
		{`é\h`, `é[0-9a-fA-F]`},
		{``, ``},
		{`(?m:a)(?-m)(?mi)`, `(?s:a)(?-s)(?si)`},
		{`a{,2}b{2}?(cd){3}+e{1,2}?`, `a{0,2}(?:b{2})?(?:(cd){3})+e{1,2}?`},
		{`(?<a>x)(y)`, `(?<a>x)(?:y)`},
		{`\u0062\u{61 62}\e`, `\x{0062}\x{61}\x{62}\x1b`},
		{`[a-z&&[^b-y]]`, `[az]`},
		{`[a[^\x00-y]]`, `[az-\x{10ffff}]`},
	}
	for _, c := range cases {
		t.Run(c.in, func(t *testing.T) {
			got, err := translateRegexp(c.in)
			if err != nil {
				t.Fatalf("translateRegexp(%q): %v", c.in, err)
			}
			if got != c.want {
				t.Errorf("translateRegexp(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

// Expectations are MRI 4.0's `!!(re =~ s)`.
func TestRegexpMatchesMRI(t *testing.T) {
	m := rubyRegexFlags{multiline: true}
	i := rubyRegexFlags{ignoreCase: true}
	cases := []struct {
		name  string
		src   string
		f     rubyRegexFlags
		in    string
		want  bool
		known string
	}{
		{"caret is line anchor", `^b`, rubyRegexFlags{}, "a\nb", true, ""},
		{"dollar is line anchor", `a$`, rubyRegexFlags{}, "a\nb", true, ""},
		{"dollar before final newline", `abc$`, rubyRegexFlags{}, "abc\n", true, ""},
		{"dot skips newline", `a.b`, rubyRegexFlags{}, "a\nb", false, ""},
		{"m_flag_dot_matches_newline", `a.b`, m, "a\nb", true, ""},
		{"i_flag", `abc`, i, "ABC", true, ""},
		{"inline_i", `(?i)abc`, rubyRegexFlags{}, "ABC", true, ""},
		{"hex_escape", `\A\h+\z`, rubyRegexFlags{}, "09afAF", true, ""},
		{"hex_escape_non_hex", `\A\h+\z`, rubyRegexFlags{}, "g", false, ""},
		{"non_hex_escape", `\A\H\z`, rubyRegexFlags{}, "g", true, ""},
		{"non_hex_escape_hex", `\A\H\z`, rubyRegexFlags{}, "a", false, ""},
		{"hex_escape_in_class", `\A[\h_]+\z`, rubyRegexFlags{}, "a_F9", true, ""},
		{"hex_escape_in_negated_class", `\A[^\h]\z`, rubyRegexFlags{}, "a", false, ""},
		{"escaped backslash then h", `\\h`, rubyRegexFlags{}, `\h`, true, ""},
		{"escaped slash", `a\/b`, rubyRegexFlags{}, "a/b", true, ""},
		{"named group", `(?<y>\d+)-(?<m>\d+)`, rubyRegexFlags{}, "2024-05", true, ""},
		{"z_is_absolute_end", `\Aab\z`, rubyRegexFlags{}, "ab\n", false, ""},
		{"posix class", `[[:alpha:]]+`, rubyRegexFlags{}, "abc", true, ""},
		{"posix_class_plus_hex_escape", `\A[[:xdigit:]\h]+\z`, rubyRegexFlags{}, "fF0", true, ""},
		{"escaped_bracket_in_class", `\A[\]h]+\z`, rubyRegexFlags{}, "]h", true, ""},
		{"dot is one rune", `\A.\z`, rubyRegexFlags{}, "é", true, ""},
		{"word_is_ascii", `\A\w\z`, rubyRegexFlags{}, "é", false, ""},
		{"digit_is_ascii", `x\d`, rubyRegexFlags{}, "x٣", false, ""},
		{"rbracket_outside_class", `a]`, rubyRegexFlags{}, "a]", true, ""},
		{"i_flag_unicode", `é`, i, "É", true, ""},
		{"hex_escape_after_class", `\A[a]\h\z`, rubyRegexFlags{}, "a9", true, ""},
		{"non_hex_escape_after_negated_class", `\A[^a]\H\z`, rubyRegexFlags{}, "bg", true, ""},
		{"escaped_backslash_then_hex_escape", `\A\\\h\z`, rubyRegexFlags{}, `\f`, true, ""},
		{"inline_i_minus_m_group", `(?i-m:a.b)`, rubyRegexFlags{}, "A\nB", false, ""},
		// Ruby's inline m is dot-all; RE2's is multi-line anchors, which (?m) already turns on.
		{"inline_m_is_dotall", `(?m)a.b`, rubyRegexFlags{}, "a\nb", true, ""},
		{"group_m_is_dotall", `(?m:a.b)`, rubyRegexFlags{}, "a\nb", true, ""},
		{"inline_mi_is_dotall", `(?mi)a.b`, rubyRegexFlags{}, "A\nB", true, ""},
		{"inline_minus_m_keeps_line_anchors", `(?-m)^b`, rubyRegexFlags{}, "a\nb", true, ""},
		{"space_matches_vertical_tab", `\A\s\z`, rubyRegexFlags{}, "\v", true, ""},
		{"non_space_rejects_vertical_tab", `\A\S\z`, rubyRegexFlags{}, "\v", false, ""},
		{"non_space_in_negated_class", `\A[^\S\n]\z`, rubyRegexFlags{}, "\v", true, ""},
		{"posix_negated_space", `x[[:^space:]]`, rubyRegexFlags{}, "x\u3000", false, ""},
		{"class_intersection", `\A[a-z&&[^aeiou]]\z`, rubyRegexFlags{}, "b", true, ""},
		{"nested_class", `\A[a[bc]]\z`, rubyRegexFlags{}, "b", true, ""},
		{"posix_alpha_is_unicode", `\A[[:alpha:]]\z`, rubyRegexFlags{}, "é", true, ""},
		{"posix_upper_is_unicode", `\A[[:upper:]]\z`, rubyRegexFlags{}, "É", true, ""},
		{"posix_digit_is_unicode", `\A[[:digit:]]\z`, rubyRegexFlags{}, "٣", true, ""},
		{"posix_space_is_unicode", `\A[[:space:]]\z`, rubyRegexFlags{}, "\u00a0", true, ""},
		{"posix_word_is_unicode", `\A[[:word:]]\z`, rubyRegexFlags{}, "é", true, ""},
		{"word_boundary_is_unicode", `\bcafé\b`, rubyRegexFlags{}, "un café noir", true, "Ruby's \\b treats non-ASCII letters as word characters; RE2's \\b is ASCII-only"},
		{"caret_not_after_final_newline", `\n^`, rubyRegexFlags{}, "a\n", false, "Ruby's ^ never matches at the end of the string after a trailing newline; RE2 (?m)^ does, so gsub(/^/, \"  \") indents an extra empty line"},
		{"open_interval_is_zero_to_n", `\Aa{,3}\z`, rubyRegexFlags{}, "aa", true, "Ruby's {,n} is {0,n}; RE2 reads it as the literal text {,n}"},
		{"fixed_interval_then_question_is_optional", `\Aa{2}?\z`, rubyRegexFlags{}, "", true, "Ruby's X{n}? is (?:X{n})?; RE2 reads it as a lazy X{n}"},
		{"Q_is_literal", `\Q.`, rubyRegexFlags{}, "Qx", true, "Ruby reads \\Q as a literal Q; RE2 starts a quoted run"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if c.known != "" {
				known(t, c.known)
			}
			re, err := compileRuby(c.src, c.f)
			if err != nil {
				t.Fatalf("/%s/%s rejected: %v", c.src, c.f.opts(), err)
			}
			if got := re.MatchString(c.in); got != c.want {
				t.Errorf("/%s/%s =~ %q: got %v, MRI gives %v", c.src, c.f.opts(), c.in, got, c.want)
			}
		})
	}
}

// Decision 24: RE2-incompatible constructs are rejected at transpile time.
func TestRegexpRejected(t *testing.T) {
	cases := []struct{ name, src, errSub string }{
		{"lookahead", `a(?=b)`, ""},
		{"negative lookahead", `a(?!b)`, ""},
		{"lookbehind", `(?<=a)b`, ""},
		{"negative lookbehind", `(?<!a)b`, ""},
		{"backreference", `(a)\1`, ""},
		{"named backreference", `(?<x>a)\k<x>`, ""},
		{"Z_anchor", `a\Z`, `use \z`},
		{"H_in_class", `[\H]`, `\H inside a character class`},
		{"H_in_negated_class", `[^\H]`, `\H inside a character class`},
		{"Z_alone", `\Z`, `\Z is not supported`},
		{"subexpression call", `(a)\g<1>`, ""},
		{"quoted named backreference", `(?<x>a)\k'x'`, ""},
		{"G_anchor", `\Ga`, `\G is not supported by RE2`},
		{"K_keep", `a\Kb`, `\K is not supported by RE2`},
		{"R_linebreak", `a\R`, `\R is not supported by RE2`},
		{"X_cluster", `\X`, `\X is not supported by RE2`},
		{"possessive", `a++`, ""},
		{"atomic group", `(?>a)`, ""},
		{"inline extended", `(?x) a b`, ""},
		{"absent operator", `(?~abc)`, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := compileRuby(c.src, rubyRegexFlags{})
			if err == nil {
				t.Fatalf("/%s/ compiled; want it rejected", c.src)
			}
			if c.errSub != "" && !strings.Contains(err.Error(), c.errSub) {
				t.Errorf("/%s/: error %q does not mention %q", c.src, err, c.errSub)
			}
		})
	}
}
