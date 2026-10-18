package compiler

import (
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"

	"github.com/jtarchie/ruby2go/internal/rbs"
)

func sigReturn(t *testing.T, c *Compiler, name string) string {
	t.Helper()
	m := c.sigMethod(&Class{RubyName: "Foo"}, name)
	if m == nil {
		return ""
	}
	return m.Overloads[0].Return.String()
}

// A reopened class keeps every sig; the user's declaration beats a vendored gem's for the same method.
func TestSigsMergeUserWins(t *testing.T) {
	dir := t.TempDir()
	write := func(name, src string) {
		err := os.WriteFile(filepath.Join(dir, name), []byte(src), 0o600)
		if err != nil {
			t.Fatal(err)
		}
	}
	write("a.rbs", "class Foo[E]\n  def a: () -> Integer\n  attr_reader x: Integer\n  include Comparable\nend\n")
	write("b.rbs", "class Foo\n  def b: () -> String\n  def a: () -> bool\n  prepend Enumerable[E]\nend\n")
	c := &Compiler{sigs: map[string]*rbs.ClassDecl{}, gemSigs: fstest.MapFS{
		"sig/gems/g/foo.rbs": {Data: []byte("class Foo\n  def a: () -> Symbol\n  def g: () -> Float\n  attr_reader x: Symbol\n  attr_reader y: Symbol\nend\n")},
	}}
	c.loadSigDir(dir, false)
	c.loadGemSigs()
	for name, want := range map[string]string{"a": "Integer", "b": "String", "g": "Float"} {
		if got := sigReturn(t, c, name); got != want {
			t.Errorf("Foo#%s returns %q, want %q", name, got, want)
		}
	}
	if got := c.sigAttr(&Class{RubyName: "Foo"}, "x"); got == nil || got.Type.String() != "Integer" {
		t.Errorf("attr x = %+v, want the user's Integer", got)
	}
	if got := c.sigAttr(&Class{RubyName: "Foo"}, "y"); got == nil {
		t.Error("the gem's attr y was dropped")
	}
	d := c.sigs["Foo"]
	if len(d.TypeParams) != 1 || len(d.Includes) != 2 {
		t.Errorf("type params %v, includes %v: want E and two includes", d.TypeParams, d.Includes)
	}
}

// A gem's shipped sig/ loads after the vendored sigs, leniently, with its generics erased and its aliases followed (decision 306).
func TestSigsShipped(t *testing.T) {
	root := t.TempDir()
	for name, src := range map[string]string{
		"sig/foo.rbs":           "class Foo[E]\n  def a: () -> E\n  def g: () -> Integer\n  def h: (E) -> Array[E]?\n  alias b a\n  alias c b\nend\n",
		"sig/bad.rbs":           "class Foo\n  def z: (((\nend\n",
		"sig/_scripts/skip.rbs": "class Foo\n  def s: () -> String\nend\n",
	} {
		p := filepath.Join(root, name)
		err := os.MkdirAll(filepath.Dir(p), 0o750)
		if err == nil {
			err = os.WriteFile(p, []byte(src), 0o600)
		}
		if err != nil {
			t.Fatal(err)
		}
	}
	c := &Compiler{sigs: map[string]*rbs.ClassDecl{}, classes: map[string]*Class{}, warned: map[string]bool{}, env: envConfig{GemWarnings: true},
		gemDirs: []string{filepath.Join(root, "lib")}, gemSigs: fstest.MapFS{
			"sig/gems/g/foo.rbs": {Data: []byte("class Foo\n  def g: () -> Float\nend\n")},
		}}
	c.loadSigs(nil)
	for name, want := range map[string]string{"a": "untyped", "b": "untyped", "c": "untyped", "g": "Float", "s": ""} {
		if got := sigReturn(t, c, name); got != want {
			t.Errorf("Foo#%s returns %q, want %q", name, got, want)
		}
	}
	if got := c.sigMethod(&Class{RubyName: "Foo"}, "h").Overloads[0].String(); got != "(untyped) -> Array[untyped]?" {
		t.Errorf("Foo#h = %s", got)
	}
	if tps := c.sigs["Foo"].TypeParams; len(tps) != 0 {
		t.Errorf("type params %v survived", tps)
	}
	if len(c.Warnings) != 2 {
		t.Errorf("warnings = %q, want bad.rbs skipped and Foo[E] erased", c.Warnings)
	}
}
