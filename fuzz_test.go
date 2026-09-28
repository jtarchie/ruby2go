package rb2go

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// FuzzCompile: any input either compiles or fails with a compile error; an internal panic is a bug. Seeded from every example and testdata/run file.
func FuzzCompile(f *testing.F) {
	seeds, _ := filepath.Glob("testdata/run/*.rb")
	examples, _ := filepath.Glob("examples/*/main.rb")
	for _, p := range append(seeds, examples...) {
		src, err := os.ReadFile(p) //nolint:gosec // repo path
		if err == nil {
			f.Add(src)
		}
	}
	f.Fuzz(func(t *testing.T, src []byte) {
		_, _, err := compileSafe("main.rb", src)
		if err != nil && strings.HasPrefix(err.Error(), "compiler panic") {
			t.Fatal(err)
		}
	})
}
