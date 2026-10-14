package compiler

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWithin(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		dir, p string
		want   bool
	}{
		{"/a/b", "/a/b", true},
		{"/a/b", "/a/b/c.rb", true},
		{"/a/b", "/a/b/..c/d.rb", true}, // a name that merely starts with ..
		{"/a/b", "/a/b/../c.rb", false},
		{"/a/b", "/a/bc/d.rb", false},
		{"/a/b", "/a", false},
		{"/a/b", "/x", false},
	} {
		if got := within(c.dir, c.p); got != c.want {
			t.Errorf("within(%q, %q) = %v, want %v", c.dir, c.p, got, c.want)
		}
	}
}

func TestGemFileDotDotName(t *testing.T) {
	t.Parallel()
	gem, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	dots := filepath.Join(gem, "..foo")
	err = os.MkdirAll(dots, 0o750)
	if err != nil {
		t.Fatal(err)
	}
	c := &Compiler{gemDirs: []string{gem}}
	if !c.gemFile(&File{path: filepath.Join(dots, "x.rb")}) {
		t.Error("lib/..foo/x.rb is inside the gem dir")
	}
	if c.gemFile(&File{path: filepath.Join(filepath.Dir(gem), "other.rb")}) {
		t.Error("a file beside the gem dir is not gem code")
	}
}

func TestDumpBadOutput(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("TMPDIR", dir)
	// a planted symlink at the old fixed name must not be followed
	victim := filepath.Join(dir, "victim")
	err := os.Symlink(victim, filepath.Join(dir, "rb2go-bad-output.go"))
	if err != nil {
		t.Fatal(err)
	}
	a, b := dumpBadOutput([]byte("one")), dumpBadOutput([]byte("two"))
	pa, pb := strings.TrimPrefix(a, "raw output written to "), strings.TrimPrefix(b, "raw output written to ")
	if pa == a || pb == b || pa == pb {
		t.Fatalf("want two distinct written paths, got %q and %q", a, b)
	}
	got, err := os.ReadFile(filepath.Clean(pa))
	if err != nil || string(got) != "one" {
		t.Fatalf("dump %s = %q, %v", pa, got, err)
	}
	_, err = os.Stat(victim)
	if err == nil {
		t.Fatal("the planted symlink's target was written")
	}
	t.Setenv("TMPDIR", filepath.Join(dir, "missing"))
	if got := dumpBadOutput([]byte("x")); !strings.HasPrefix(got, "raw output not saved:") {
		t.Fatalf("a failed write must say so, got %q", got)
	}
}
