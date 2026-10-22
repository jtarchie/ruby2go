package compiler

import "testing"

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
