package compiler

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// pruneGemMethods precedes checkCycles, so reach.inherits spun forever on a cycle not through its owner (a gem sig types a param as cyclic A; Other#zorp is not on the cycle).
func TestReachCyclicClassReportsError(t *testing.T) {
	_, err := exec.LookPath("ruby")
	if err != nil {
		t.Fatal("boot needs ruby: ", err) // the suite requires ruby (CLAUDE.md); a skip would be a silent pass
	}
	dir := t.TempDir()
	write := func(p, s string) {
		p = filepath.Join(dir, p)
		err := os.MkdirAll(filepath.Dir(p), 0o750)
		if err == nil {
			err = os.WriteFile(p, []byte(s), 0o600)
		}
		if err != nil {
			t.Fatal(err)
		}
	}
	write("lib/thing.rb", "class Thing\n  def foo(x)\n    x.zorp\n  end\nend\nclass Other\n  def zorp = 1\nend\n")
	write("sig/thing.rbs", "class Thing\n  def foo: (A x) -> void\nend\n")
	write("main.rb", "class A < B\nend\nclass B < A\nend\nThing.new.foo(A.new)\n")
	entry := "require \"thing\"\n"
	fsys := os.DirFS("../..")
	_, _, cerr := CompileTraced(context.Background(), fsys, fsys,
		[]Source{{Name: "entry.rb", Src: []byte(entry), Path: filepath.Join(dir, "entry.rb")}}, filepath.Join(dir, "main.rb"), filepath.Join(dir, "lib"))
	if cerr == nil || !strings.Contains(cerr.Error(), "inherits from itself") {
		t.Fatalf("want an inherits-from-itself error, got %v", cerr)
	}
}
