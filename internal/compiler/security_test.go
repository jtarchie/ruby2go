package compiler

import (
	"context"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A source name reaches `//line` directives and the generated-file header, so a newline in it would land as Go source.
func TestHostileSourceNameStaysAComment(t *testing.T) {
	t.Parallel()
	for _, name := range []string{
		"a\nvar _ = injected_marker()\nb.rb",
		"a\rvar _ = injected_marker()\rb.rb",
		"a\x00b\x7f\u2028\ufeffc\xffd.rb",
	} {
		out, _, err := CompileFilesWithWarnings(context.Background(), os.DirFS("../.."), []Source{{Name: name, Src: []byte("puts 1\n")}})
		if err != nil {
			t.Fatalf("%q: %v", name, err)
		}
		_, err = parser.ParseFile(token.NewFileSet(), "main.go", out, parser.ParseComments)
		if err != nil {
			t.Fatalf("%q: output does not parse: %v", name, err)
		}
		for line := range strings.SplitSeq(string(out), "\n") {
			if strings.Contains(line, "injected_marker") && !strings.HasPrefix(line, "//") {
				t.Fatalf("%q: injected Go line %q", name, line)
			}
		}
	}
}

// A `require` name that climbs out of its -I dir (or names an absolute path) must not load files outside it.
func TestRequireStaysInLoadPath(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	lib := filepath.Join(root, "lib")
	for name, body := range map[string]string{
		"lib/inside.rb":   "puts 'inside_marker'\n",
		"lib/..dots/x.rb": "puts 'dots_marker'\n",
		"secret/s.rb":     "puts 'outside_marker'\n",
	} {
		p := filepath.Join(root, name)
		err := os.MkdirAll(filepath.Dir(p), 0o750)
		if err == nil {
			err = os.WriteFile(p, []byte(body), 0o600)
		}
		if err != nil {
			t.Fatal(err)
		}
	}
	compile := func(src string) string {
		t.Helper()
		out, _, err := CompileFilesWithWarnings(context.Background(), os.DirFS("../.."), []Source{{Name: "main.rb", Src: []byte(src)}}, lib)
		if err != nil {
			t.Fatal(err)
		}
		return string(out)
	}
	if out := compile("require 'inside'\n"); !strings.Contains(out, "inside_marker") {
		t.Fatal("a require inside the load path no longer loads")
	}
	if out := compile("require '..dots/x'\n"); !strings.Contains(out, "dots_marker") {
		t.Fatal("a first path element named ..dots counts as inside the load path")
	}
	if out := compile("require '../secret/s'\n"); strings.Contains(out, "outside_marker") {
		t.Fatal("require '../secret/s' loaded a file outside the load path")
	}
	// MRI loads an absolute name as itself, not under each load-path dir.
	if out := compile("require '" + filepath.Join(root, "secret", "s") + "'\n"); !strings.Contains(out, "outside_marker") {
		t.Fatal("an absolute require should load that file, as MRI does")
	}
}
