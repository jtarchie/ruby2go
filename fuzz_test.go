package rb2go

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jtarchie/ruby2go/internal/compiler"
)

// FuzzCompile: any input either compiles or fails with a compile error; an internal panic is a bug, whether compileSafe caught it or the compiler returned it as compiler.ErrInternal (decision 263). Seeded from every example and testdata/run file.
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
		if err != nil && (errors.Is(err, compiler.ErrInternal) || strings.HasPrefix(err.Error(), "compiler panic")) {
			t.Fatal(err)
		}
	})
}
