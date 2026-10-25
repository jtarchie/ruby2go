package compiler

import (
	"testing"

	"github.com/danielgatis/go-ruby-prism/parser"

	"github.com/jtarchie/rugaroo/internal/boot"
)

// A boot "path:line" splits at the last colon, so a gem path holding one still finds its compiled file.
func TestCallerFileColonInPath(t *testing.T) {
	const path = "/gems/odd:dir/lib/x.rb"
	f := &File{path: path}
	c := &Compiler{parsed: map[string]*File{path: f}, pathCache: pathCache{realPaths: map[string]string{path: path}}}
	for loc, wantLine := range map[string]int{path + ":12": 12, "/gems/odd:dir/lib/y.rb:3": 3} {
		got, ln := c.callerFile(loc)
		if loc == path+":12" && (got != f || ln != wantLine) {
			t.Errorf("callerFile(%q) = %v, %d", loc, got, ln)
		}
		if loc != path+":12" && got != nil {
			t.Errorf("callerFile(%q) found a file for an uncompiled path", loc)
		}
	}
	for _, loc := range []string{"", "x.rb", "x.rb:abc", path} {
		if got, _ := c.callerFile(loc); got != nil {
			t.Errorf("callerFile(%q) = %v, want nil", loc, got)
		}
	}
}

// A branch boot recorded at line:col is found from the node's byte offset, and a different column on the line is not it.
func TestBootTookKey(t *testing.T) {
	f := &File{path: "/x.rb", lines: []int{0, 10, 25}}
	c := &Compiler{pathCache: pathCache{realPaths: map[string]string{"/x.rb": "/x.rb"}}}
	c.indexBootBranches([]boot.BranchInfo{{Loc: boot.Loc{File: "/x.rb", Line: 2}, Col: 3, Kind: "if", Then: 1}})
	if v, ok := c.bootTook(f, parser.Location{StartOffset: 13}, false); !ok || !v {
		t.Errorf("bootTook at 2:3 = %v, %v; want true, true", v, ok)
	}
	if _, ok := c.bootTook(f, parser.Location{StartOffset: 14}, false); ok {
		t.Errorf("bootTook at 2:4 found a branch recorded at 2:3")
	}
}
