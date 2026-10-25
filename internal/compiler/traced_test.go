package compiler

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/jtarchie/rugaroo/internal/boot"
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

// A loaded gem file's literal require of the same gem's file joins the entry; another gem's or the app's does not.
func TestTracedEntryGemOwnRequire(t *testing.T) {
	d := t.TempDir()
	lib := d + "/gems/patchy-1.0/lib"
	write := func(p, s string) {
		err := os.MkdirAll(filepath.Dir(p), 0o750)
		if err == nil {
			err = os.WriteFile(p, []byte(s), 0o600)
		}
		if err != nil {
			t.Fatal(err)
		}
	}
	write(lib+"/patchy.rb", "module Patchy\n  def self.save\n    require 'patchy/alt'\n    require \"other\"\n  end\nend\n")
	write(lib+"/patchy/alt.rb", "require 'patchy/deep'\n")
	write(lib+"/patchy/deep.rb", "")
	write(d+"/app/other.rb", "")
	l := &boot.Loaded{Features: []string{lib + "/patchy.rb"}, LoadPath: []string{lib, d + "/app"}}
	src, _ := TracedEntry(l, d+"/main.rb")
	want := "require \"patchy\"\nrequire \"patchy/alt\"\nrequire \"patchy/deep\"\n"
	if string(src) != want {
		t.Errorf("entry:\n%s\nwant:\n%s", src, want)
	}
}

// entry.rb is Ruby: a path with `#{` (Go's %q leaves it bare) must not interpolate when the entry is parsed.
func TestTracedEntryQuotesForRuby(t *testing.T) {
	d := t.TempDir()
	lib := filepath.Join(d, "lib")
	err := os.MkdirAll(lib, 0o750)
	if err != nil {
		t.Fatal(err)
	}
	name := "x#{1}y\"z\\"
	err = os.WriteFile(filepath.Join(lib, name+".rb"), nil, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	want := `require "x\#{1}y\"z\\"` + "\n"
	src, _ := TracedEntry(&boot.Loaded{Features: []string{filepath.Join(lib, name+".rb")}, LoadPath: []string{lib}}, d+"/main.rb")
	if string(src) != want {
		t.Errorf("TracedEntry:\n%s\nwant:\n%s", src, want)
	}
	main := filepath.Join(d, "main.rb")
	err = os.WriteFile(main, []byte("require 'x#{1}y\"z\\\\'\n"), 0o600)
	if err != nil {
		t.Fatal(err)
	}
	more, err := mainRequires(t.Context(), main, []string{lib}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if string(more) != want {
		t.Errorf("mainRequires:\n%s\nwant:\n%s", more, want)
	}
}
