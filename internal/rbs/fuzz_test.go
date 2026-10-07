package rbs

import "testing"

var typeSeeds = []string{
	"String", "Array[String]?", "[Integer, String]", "Hash[String, Array[Integer]]", "singleton(A::B)",
	"Integer | String", "^(Integer) -> String", "^() -> void", "Array[^(Integer, String) -> Integer?]",
	"self", "untyped", "bool", "nil", "::Foo::Bar", "[A, B, C]?",
}

// FuzzParseType: whatever parses prints back to text that parses to the same type.
func FuzzParseType(f *testing.F) {
	for _, s := range typeSeeds {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		ty, err := within(t, s, ParseType)
		if err != nil {
			return
		}
		again, err := within(t, ty.String(), ParseType)
		if err != nil {
			t.Fatalf("%q parsed as %q, which does not reparse: %v", s, ty, err)
		}
		if again.String() != ty.String() {
			t.Fatalf("%q parsed as %q, which reparses as %q", s, ty, again)
		}
	})
}

// FuzzParseFile: arbitrary declaration text never panics; every declaration
// it parses is closed (ParseFile rejects an unclosed class).
func FuzzParseFile(f *testing.F) {
	f.Add("class Foo\n  def bar: (Integer) -> String\nend\n")
	f.Add("module Rack\n  class Request\n    attr_reader env: Hash[String, untyped]\n  end\nend\n")
	f.Add("class Box[E]\n  include Enumerable[E]\n  VERSION: String\n  type Handler = ^(untyped) -> void\nend\n")
	f.Fuzz(func(t *testing.T, s string) {
		_, _ = within(t, s, ParseFile)
	})
}

// FuzzParseMethodType: the same round trip for method signatures.
func FuzzParseMethodType(f *testing.F) {
	for _, s := range []string{
		"(Integer) -> String", "[X] (Array[X]) { (X) -> void } -> X", "(?Integer, *String) -> void",
		"() ?{ () -> void } -> self", "(^(Integer) -> Integer, Integer) -> Integer", "(Integer x, ?String y) -> bool",
	} {
		f.Add(s)
	}
	for _, s := range typeSeeds {
		f.Add("(" + s + ") -> " + s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		m, err := within(t, s, ParseMethodType)
		if err != nil {
			return
		}
		again, err := within(t, m.String(), ParseMethodType)
		if err != nil {
			t.Fatalf("%q parsed as %q, which does not reparse: %v", s, m, err)
		}
		if again.String() != m.String() {
			t.Fatalf("%q parsed as %q, which reparses as %q", s, m, again)
		}
	})
}
