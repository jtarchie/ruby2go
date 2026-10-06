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
		"-1.send(:abs).should == 1", // @method substituted
	} {
		if !strings.Contains(got, want) {
			t.Errorf("rewritten spec lacks %q:\n%s", want, got)
		}
	}
	for _, gone := range []string{"is gone", "ruby_version_is", "require_relative", "context"} {
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
