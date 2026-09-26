package rbs

import "testing"

func TestParseMethodType(t *testing.T) {
	cases := map[string]string{
		"() -> String":                             "() -> String",
		"(Integer, Integer) -> void":               "(Integer, Integer) -> void",
		"[X] () { (self) -> X } -> X":              "[X] () { (self) -> X } -> X",
		"(*untyped) -> nil":                        "(*untyped) -> nil",
		"(Hash[String, String], String) -> String": "(Hash[String, String], String) -> String",
		"(Integer) -> E?":                          "(Integer) -> E?",
		"[U] () { (E) -> U } -> Array[U]":          "[U] () { (E) -> U } -> Array[U]",
		"[K] () { ([String, Integer]) -> K } -> Array[[String, Integer]]": "[K] () { ([String, Integer]) -> K } -> Array[[String, Integer]]",
		"(?String msg) -> void":             "(?String msg) -> void",
		"(String | nil) -> (Integer | nil)": "(String?) -> Integer?",
		"(::String) -> ::Integer":           "(String) -> Integer",
	}
	for in, want := range cases {
		m, err := ParseMethodType(in)
		if err != nil {
			t.Errorf("%q: %v", in, err)
			continue
		}
		if got := m.String(); got != want {
			t.Errorf("%q: got %q want %q", in, got, want)
		}
	}
}

func TestParseType(t *testing.T) {
	for _, in := range []string{"String", "Array[String]?", "[Integer, String]", "Hash[String, Array[Integer]]"} {
		ty, err := ParseType(in)
		if err != nil {
			t.Errorf("%q: %v", in, err)
			continue
		}
		if ty.String() != in {
			t.Errorf("%q: got %q", in, ty.String())
		}
	}
}
