package compiler

import "testing"

func TestLookaheadBody(t *testing.T) {
	for _, c := range []struct{ in, want string }{
		{`(?m)(?=(?-si:\*|_|(  |\\)(?=\n)|\\))`, `(?m)(?-si:\*|_|(  |\\)(?:\n)|\\)`},
		{`(?=a|b(?=c))`, `a|b(?:c)`},
		{`(?=(a(?=b)))`, `(a(?=b))`},     // inside a capturing group: left alone
		{`(?=a(?=b)c)`, `a(?=b)c`},       // not at a branch's end: left alone (RE2 then rejects it)
		{`(?=[)(]x(?=y))`, `[)(]x(?:y)`}, // parens in a class
		{`(?i)(?=x)`, `(?i)x`},
	} {
		got, ok := rxLookaheadBody(c.in)
		if !ok || got != c.want {
			t.Errorf("rxLookaheadBody(%q) = %q, %v; want %q", c.in, got, ok, c.want)
		}
	}
	for _, in := range []string{`a(?=b)`, `(?=a)b`, `(?=a)|(?=b)`, `(?<=a)`, `(?!a)`} {
		if got, ok := rxLookaheadBody(in); ok {
			t.Errorf("rxLookaheadBody(%q) = %q; want no body", in, got)
		}
	}
}
