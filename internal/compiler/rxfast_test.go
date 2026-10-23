package compiler

import (
	"math/rand/v2"
	"reflect"
	"regexp"
	"strings"
	"testing"
)

// rxFast must answer what RE2 answers for every pattern it takes, on any subject: ASCII, multibyte, invalid UTF-8.
func TestRxFastMatchesRE2(t *testing.T) {
	t.Parallel()
	taken := []string{`\d`, `[aeiou]`, `[\s/?=]+`, `\w+`, `\A\d+`, `\A[a-z_]`, `abc`, `\Ax=`, `[0-9]`, `-`}
	refused := []string{`\d+?`, `(\d)`, `(?i)abc`, `é`, `[^a]`, `\d{2}`, `\w+@\w+\.com`, `(?m)^a`, `\ba`, `a*`, `.`, `[^\x00-\x2f\x3a-\x7f]`}
	for _, p := range refused {
		if r := rxNew(regexp.MustCompile(p)); r.fast != nil {
			t.Errorf("%s: taken by rxFast", p)
		}
	}
	alphabet := []string{"a", "b", "c", "x", "=", "1", "9", "0", " ", "\n", "/", "?", "_", "é", "日", "\xff", "\xc3", "abc", "-"}
	rng := rand.New(rand.NewPCG(1, 2)) //nolint:gosec // test data
	for _, p := range taken {
		re := regexp.MustCompile(p)
		r := rxNew(re)
		if r.fast == nil {
			t.Fatalf("%s: not taken by rxFast", p)
		}
		for range 3000 {
			var b strings.Builder
			for range rng.IntN(12) {
				b.WriteString(alphabet[rng.IntN(len(alphabet))])
			}
			s := b.String()
			n := rng.IntN(4) - 1
			if got, want := r.fast.find(s), re.FindStringSubmatchIndex(s); !reflect.DeepEqual(got, want) {
				t.Fatalf("%s on %q: find %v, RE2 %v", p, s, got, want)
			}
			if got, want := r.fast.findAll(s, n), re.FindAllStringSubmatchIndex(s, n); !reflect.DeepEqual(got, want) {
				t.Fatalf("%s on %q n=%d: findAll %v, RE2 %v", p, s, n, got, want)
			}
			if got, want := r.MatchString(s), re.MatchString(s); got != want {
				t.Fatalf("%s on %q: MatchString %v, RE2 %v", p, s, got, want)
			}
		}
	}
}
