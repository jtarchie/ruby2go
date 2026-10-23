package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danielgatis/go-ruby-prism/parser"
)

func TestRewrite(t *testing.T) {
	root := t.TempDir()
	write := func(name, src string) {
		t.Helper()
		p := filepath.Join(root, name)
		err := os.MkdirAll(filepath.Dir(p), 0o750)
		if err == nil {
			err = os.WriteFile(p, []byte(src), 0o600)
		}
		if err != nil {
			t.Fatal(err)
		}
	}
	write("spec_helper.rb", "raise 'never loaded'\n")
	write("core/x/shared/abs.rb", `describe :x_abs, shared: true do
  before :each do
    @z = 1
  end
  it "is positive" do
    -1.send(@method).should == 1
  end
end
`)
	spec := `require_relative '../../spec_helper'
require_relative 'shared/abs'

describe "X#abs" do
  context "old" do
    ruby_version_is ""..."3.0" do
      it "is gone" do
        nope
      end
    end
  end
  ruby_version_is "3.4" do
    it "stays" do
      -> { 1 }.should_not.raise(TypeError)
    end
  end
  before :each do
    @a = 1
  end
  it_behaves_like :x_abs, :abs
  it_behaves_like :x_abs, -> { 1 }
end
`
	write("core/x/abs_spec.rb", spec)
	ctx := context.Background()
	p, err := parser.NewParser(ctx, parser.WithVersion(parser.SyntaxVersionLatest))
	if err != nil {
		t.Fatal(err)
	}
	prog, err := load(ctx, p, root, []string{"core/x/abs_spec.rb"})
	if err != nil {
		t.Fatal(err)
	}
	if len(prog.files) != 3 || prog.files[1].name != "core/x/shared/abs.rb" {
		t.Fatalf("want mspec.rb, the shared file, the spec; got %d files", len(prog.files))
	}
	got := prog.files[2].text
	for _, want := range []string{
		`describe "old" do`, // context
		"ProcExpectation.new(-> { 1 }, false).raise(TypeError)",
		"before  do", // before's :each removed
		"describe \"x_abs\" do\n",
		"def __rbspec_before_2;",                     // the ivar hook chains with the shared spec's own before
		"@method = :abs end; def __rbspec_before_2;", // helpers read the ivar at run time
		"-1.send(:abs).should == 1",                  // @method substituted
	} {
		if !strings.Contains(got, want) {
			t.Errorf("rewritten spec lacks %q:\n%s", want, got)
		}
	}
	for _, gone := range []string{"is gone", "ruby_version_is", "require_relative", "context", "@method = ->"} {
		if strings.Contains(got, gone) {
			t.Errorf("rewritten spec still has %q:\n%s", gone, got)
		}
	}
	// up to the inlined shared spec, every line stays where it was, so errors' line numbers find the example
	before, _, _ := strings.Cut(got, "describe \"x_abs\"")
	origBefore, _, _ := strings.Cut(spec, "it_behaves_like")
	if strings.Count(before, "\n") != strings.Count(origBefore, "\n") {
		t.Errorf("rewrite moved lines:\n%s", got)
	}
}

func TestVersionGuards(t *testing.T) {
	for _, c := range []struct {
		a, b string
		want int
	}{{"4.0.0", "4.0", 0}, {"4.0.0", "3.4", 1}, {"4.0.0", "4.1", -1}, {"4.0.1", "4.0", 1}} {
		if got := compareVersion(c.a, c.b); (got > 0) != (c.want > 0) || (got < 0) != (c.want < 0) {
			t.Errorf("compareVersion(%q, %q) = %d, want sign of %d", c.a, c.b, got, c.want)
		}
	}
}

// mspec runs every before/after of a describe; rb2go's, like minitest's, would keep only the last.
func TestRewriteChainsHooks(t *testing.T) {
	ctx := context.Background()
	p, err := parser.NewParser(ctx, parser.WithVersion(parser.SyntaxVersionLatest))
	if err != nil {
		t.Fatal(err)
	}
	prog := &program{p: p}
	src := `describe "x" do
  before :each do
    @b = 1
  end
  before(:all) { @a = 1 }
  after :all do
    @c = 1
  end
  after :each do
    @d = 1
  end
end
`
	got, err := prog.rewrite(ctx, "x_spec.rb", src, 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"def __rbspec_before_1;",
		"def __rbspec_before_2; @a = 1 end; before { __rbspec_before_2; __rbspec_before_1 }", // :all first
		"end; after { __rbspec_after_4; __rbspec_after_3 }",                                  // :each first
	} {
		if !strings.Contains(got, want) {
			t.Errorf("rewritten spec lacks %q:\n%s", want, got)
		}
	}
	if strings.Count(got, "\n") != strings.Count(src, "\n") {
		t.Errorf("rewrite moved lines:\n%s", got)
	}
}

func TestCrashFrames(t *testing.T) {
	spec := "goroutine 1:\n\tgen/core/k/fixtures/a.rb:3 +0x1c\n\tgen/core/k/a_spec.rb:9 +0x2c\n"
	if m := specFrame.FindStringSubmatch(spec); m == nil || m[1] != "core/k/a_spec.rb" || m[2] != "9" {
		t.Errorf("specFrame: %q", m)
	}
	load := "gen/core/k/fixtures/a.rb:8:in '<main>': undefined method 'prepend' for class Module (NoMethodError)\n\tfrom mspec.rb:2:in '<main>'\n"
	if m := loadFrame.FindStringSubmatch(load); m == nil || m[1] != "core/k/fixtures/a.rb" || m[2] != "8" {
		t.Errorf("loadFrame: %q", m)
	}
	if m := loadFrame.FindStringSubmatch("x\n\tfrom mspec.rb:2:in '<main>'\n"); m != nil {
		t.Errorf("loadFrame matched mspec.rb: %q", m)
	}
}

func TestApplyEdits(t *testing.T) {
	// equal starts apply in the order given (an insertion before the replacement it abuts); an unstable sort reorders many
	got, err := applyEdits("abcdef", []edit{{2, 2, "<"}, {2, 4, "X"}})
	if err != nil || got != "ab<Xef" {
		t.Errorf("equal-start edits = %q, %v; want ab<Xef", got, err)
	}
	many := make([]edit, 0, 37)
	many = append(many, edit{5, 5, "!"}, edit{4, 4, "?"}) // out of order, so the sort has work to do
	for _, s := range "123456789abcdefghijklmnopqrstuvwxyz" {
		many = append(many, edit{2, 2, string(s)})
	}
	got, err = applyEdits("abcdef", many)
	if err != nil || got != "ab123456789abcdefghijklmnopqrstuvwxyzcd?e!f" {
		t.Errorf("many equal starts = %q, %v", got, err)
	}
	// an edit wholly inside a replaced region is dropped on purpose
	got, err = applyEdits("abcdef", []edit{{1, 5, "-"}, {2, 3, "Y"}})
	if err != nil || got != "a-f" {
		t.Errorf("nested edit = %q, %v; want a-f", got, err)
	}
	// one that starts inside and ends outside would be silently lost
	_, err = applyEdits("abcdef", []edit{{1, 4, "-"}, {3, 5, "Y"}})
	if err == nil {
		t.Error("partially overlapping edits must be an error, not dropped")
	}
}
