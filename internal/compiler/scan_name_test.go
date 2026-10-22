package compiler

import (
	"io/fs"
	"math/rand/v2"
	"os"
	"regexp"
	"slices"
	"strings"
	"testing"
)

// The regexps the scanners in scan_name.go replaced, kept as their oracle.
var (
	oldGoDecl = regexp.MustCompile(`(?m)^\s*(?:func|type|var|const)\s+([A-Z]\w*)`)
	oldGoCall = regexp.MustCompile(`(?:\.|_)([A-Z][A-Za-z0-9]*)[(\[]`)
)

func submatches(re *regexp.Regexp, s string) []string {
	out := make([]string, 0, 8)
	for _, m := range re.FindAllStringSubmatch(s, -1) {
		out = append(out, m[1])
	}
	return out
}

func sameScan(s string) (decl, call bool) {
	var d, c []string
	declaredGoNames(s, func(n string) { d = append(d, n) })
	goCallNames(s, func(n string) { c = append(c, n) })
	return slices.Equal(d, submatches(oldGoDecl, s)), slices.Equal(c, submatches(oldGoCall, s))
}

// scanCorpus is every prelude file, example and test input: the text the scanners see in practice (verbatim blocks are slices of these).
func scanCorpus(t *testing.T) map[string]string {
	t.Helper()
	fsys := os.DirFS("../..")
	out := map[string]string{}
	for _, pattern := range []string{"prelude.rb", "prelude/*.rb", "prelude/go/*.go", "examples/*/main.rb", "testdata/run/*.rb", "testdata/test/*.rb", "internal/compiler/*.go"} {
		names, _ := fs.Glob(fsys, pattern)
		for _, name := range names {
			b, err := fs.ReadFile(fsys, name)
			if err != nil {
				t.Fatal(err)
			}
			out[name] = string(b)
		}
	}
	if len(out) < 100 {
		t.Fatalf("corpus has %d files", len(out))
	}
	return out
}

func TestScanNameMatchesRegexps(t *testing.T) {
	t.Parallel()
	for name, s := range scanCorpus(t) {
		d, c := sameScan(s)
		if !d {
			t.Errorf("%s: declaredGoNames and the regexp disagree", name)
		}
		if !c {
			t.Errorf("%s: goCallNames and the regexp disagree", name)
		}
	}
	// random text over the alphabet of every boundary rule: keywords, each kind of whitespace (newlines included), names, brackets, dots, underscores, non-ASCII
	words := []string{"func", "type", "var", "const", "funcs", "constant", "fun", "Foo", "foo", "A9", "_", "9", " ", "  ", "\t", "\n", "\n\n", "\r", "\f", ".", "(", "[", "{", ")", "Bar_Baz(", "x.Y(", "é", "\xff", "Z"}
	rng := rand.New(rand.NewPCG(1, 2)) //nolint:gosec // a fixed seed: a reproducible fuzz, not a secret
	for range 60000 {
		var b strings.Builder
		for range 1 + rng.IntN(14) {
			b.WriteString(words[rng.IntN(len(words))])
		}
		s := b.String()
		if d, c := sameScan(s); !d || !c {
			t.Fatalf("scanner and regexp disagree (decl %v, call %v) on %q", d, c, s)
		}
	}
}
