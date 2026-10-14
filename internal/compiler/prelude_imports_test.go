package compiler

import (
	"io/fs"
	"maps"
	"os"
	"path"
	"testing"
)

func preludeGoImports(t *testing.T, fsys fs.FS) []string {
	t.Helper()
	names, err := fs.Glob(fsys, "prelude/go/[a-z]*.go")
	if err != nil || len(names) == 0 {
		t.Fatalf("glob prelude/go: %v (%d files)", err, len(names))
	}
	var all []string
	for _, name := range names {
		src, err := fs.ReadFile(fsys, name)
		if err != nil {
			t.Fatal(err)
		}
		_, _, imports, err := stripGoPackage(src)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		all = append(all, imports...)
	}
	return all
}

// Every import of every prelude/go file resolves through stdImports to that path, with no name claimed for two paths (decision 154).
func TestPreludeImportsResolve(t *testing.T) {
	fsys := os.DirFS("../..")
	(&Compiler{preludeFS: fsys}).loadPreludeGo()
	imports := preludeGoImports(t, fsys)
	if bad := mergeStdImports(maps.Clone(stdImports), imports); len(bad) > 0 {
		t.Fatalf("import name collisions: %v", bad)
	}
	for _, imp := range imports {
		if got := stdImports[path.Base(imp)]; got != imp {
			t.Errorf("stdImports[%q] = %q, want %q", path.Base(imp), got, imp)
		}
	}
}

// The merge covers every file's imports, not only the first file's, and a hand entry wins a name clash.
func TestMergeStdImports(t *testing.T) {
	table := map[string]string{"rand": "crypto/rand"}
	bad := mergeStdImports(table, []string{"slices", "net/netip", "syscall", "math/rand"})
	for base, want := range map[string]string{"slices": "slices", "netip": "net/netip", "syscall": "syscall", "rand": "crypto/rand"} {
		if table[base] != want {
			t.Errorf("table[%q] = %q, want %q", base, table[base], want)
		}
	}
	if len(bad) != 1 {
		t.Errorf("collisions = %v, want the rand clash", bad)
	}
}
