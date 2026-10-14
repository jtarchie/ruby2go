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
	c.loadSigDir(dir)
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
