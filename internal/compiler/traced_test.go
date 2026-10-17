package compiler

import (
	"slices"
	"testing"

	"github.com/jtarchie/ruby2go/internal/boot"
)

func TestTracedEntry(t *testing.T) {
	g := "/v/gems/4.0.0"
	l := &boot.Loaded{
		Features: []string{
			"/app/main.rb",
			"/std/set.rb", "/std/enc/encdb.bundle", "/std/portable_ruby_gems.rb",
			g + "/gems/json-3.0.2/lib/json/common.rb", g + "/gems/json-3.0.2/lib/json.rb",
			g + "/extensions/arm64-darwin/4.0.0/nokogiri-1.18.0/nokogiri.bundle", g + "/gems/nokogiri-1.18.0/lib/nokogiri.rb",
			g + "/gems/plist-3.7/lib/plist.rb",
			"/app/lib/cmds/info.rb", "/app/lib/cmds/x.bundle", "/app/vendor/bundler/setup.rb",
		},
		LoadPath: []string{"/app/lib", g + "/gems/plist-3.7/lib", g + "/gems/nokogiri-1.18.0/lib", "/std"},
		StdDirs:  []string{"/std"},
	}
	src, dirs := TracedEntry(l, "/app/main.rb")
	want := "require \"set\"\nrequire \"json\"\nrequire \"plist\"\nrequire \"cmds/info\"\n"
	if string(src) != want {
		t.Errorf("entry:\n%s\nwant:\n%s", src, want)
	}
	if !slices.Equal(dirs, []string{g + "/gems/plist-3.7/lib", "/app/lib"}) {
		t.Errorf("dirs: %v", dirs)
	}
}
