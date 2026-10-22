package compiler

import (
	"io/fs"
	"os"
	"regexp"
	"slices"
	"strings"
	"testing"
	"testing/quick"
)

var oldGoSelectorCall = regexp.MustCompile(`(\b[A-Za-z_][A-Za-z_0-9]*)?\.([A-Z_]\w*)\(`)

func sameGoSelectorPairs(src []byte) bool {
	var got [][2]string
	goSelectorCalls(src, func(r, n []byte) { got = append(got, [2]string{string(r), string(n)}) })
	all := oldGoSelectorCall.FindAllSubmatch(src, -1)
	want := make([][2]string, 0, len(all))
	for _, m := range all {
		want = append(want, [2]string{string(m[1]), string(m[2])})
	}
	return slices.Equal(got, want)
}

// cutPlainSelf must equal the regexp it replaced, on random text over an alphabet that hits every boundary rule.
func TestCutPlainSelfMatchesRegexp(t *testing.T) {
	t.Parallel()
	re := regexp.MustCompile(`\bK_\w+\(self\b`)
	words := []string{"K_", "K_a", "K_ab(self", "(self", "self", ")", " ", "_", "9", "xK_", "(selfx", "(self)", "K__z(self,", "[any](self"}
	err := quick.Check(func(idx []uint8) bool {
		var b strings.Builder
		for _, i := range idx {
			b.WriteString(words[int(i)%len(words)])
		}
		s := b.String()
		got, n := cutPlainSelf(s, "K_", true)
		_, m := cutPlainSelf(s, "K_", false)
		return got == re.ReplaceAllString(s, "") && (n > 0) == re.MatchString(s) && m == n
	}, &quick.Config{MaxCount: 50000})
	if err != nil {
		t.Error(err)
	}
}

// goSelectorCalls must find what the regexp it replaced found, on the real prelude and on random text over the scanner's alphabet.
func TestGoSelectorCallsMatchRegexp(t *testing.T) {
	t.Parallel()
	fsys := os.DirFS("../..")
	n := 0
	for _, pattern := range []string{"prelude.rb", "prelude/*.rb", "prelude/go/*.go"} {
		names, _ := fs.Glob(fsys, pattern)
		for _, name := range names {
			src, err := fs.ReadFile(fsys, name)
			if err != nil {
				t.Fatal(err)
			}
			n++
			if !sameGoSelectorPairs(src) {
				t.Errorf("%s: scanner and regexp disagree", name)
			}
		}
	}
	if n == 0 {
		t.Fatal("no prelude files found")
	}
	const alphabet = "ab_Z9. (\n\xc3\xa9"
	err := quick.Check(func(b []byte) bool {
		for i := range b {
			b[i] = alphabet[int(b[i])%len(alphabet)]
		}
		return sameGoSelectorPairs(b)
	}, &quick.Config{MaxCount: 20000})
	if err != nil {
		t.Error(err)
	}
}
