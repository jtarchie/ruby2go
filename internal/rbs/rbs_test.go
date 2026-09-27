package rbs

import (
	"cmp"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"
)

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
		"(?String msg) -> void":              "(?String msg) -> void",
		"(String | nil) -> (Integer | nil)":  "(String?) -> Integer?",
		"(::String) -> ::Integer":            "(String) -> Integer",
		"(?Integer n, *String rest) -> void": "(?Integer n, *String rest) -> void",
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
	for _, in := range []string{"String", "Array[String]?", "[Integer, String]", "Hash[String, Array[Integer]]", "singleton(A::B)", "Array[singleton(Base)]", "Integer | String", "Array[Integer | String]"} {
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

// known skips a case that disagrees with the README or RBS unless RB2GO_RUN_SKIPPED=1.
func known(t *testing.T, reason string) {
	t.Helper()
	if os.Getenv("RB2GO_RUN_SKIPPED") == "" {
		t.Skip("known: " + reason)
	}
}

// within fails instead of hanging when the parser loops.
func within[T any](t *testing.T, s string, parse func(string) (T, error)) (T, error) {
	t.Helper()
	type res struct {
		v   T
		err error
	}
	ch := make(chan res, 1)
	go func() {
		v, err := parse(s)
		ch <- res{v, err}
	}()
	select {
	case r := <-ch:
		return r.v, r.err
	case <-time.After(250 * time.Millisecond):
	}
	select {
	case r := <-ch:
		return r.v, r.err
	default:
		t.Fatalf("parsing %q did not return", s)
	}
	var zero T
	return zero, nil
}

func nm(name string, args ...Type) Name { return Name{Name: name, Args: args} }

func TestParseTypeShapes(t *testing.T) {
	cases := []struct {
		in   string
		want Type
	}{
		{"String", nm("String")},
		{"  \tString ", nm("String")},
		{"::String", nm("String")},
		{"A::B::C", nm("A::B::C")},
		{"::A::B", nm("A::B")},
		{"_Each[Integer]", nm("_Each", nm("Integer"))},
		{"Array[String]", nm("Array", nm("String"))},
		{"Hash[Symbol, Array[Integer]]", nm("Hash", nm("Symbol"), nm("Array", nm("Integer")))},
		{"Array[String?]", nm("Array", Optional{Elem: nm("String")})},
		// optionals and unions
		{"String?", Optional{Elem: nm("String")}},
		{"(String)?", Optional{Elem: nm("String")}},
		{"(String)", nm("String")},
		{"String | nil", Optional{Elem: nm("String")}},
		{"nil | String", Optional{Elem: nm("String")}},
		{"Array[String | nil]", nm("Array", Optional{Elem: nm("String")})},
		{"Integer | String", Union{Elems: []Type{nm("Integer"), nm("String")}}},
		{"Integer | String | nil", Union{Elems: []Type{nm("Integer"), nm("String"), Nil{}}}},
		{"(Integer | String)?", Optional{Elem: Union{Elems: []Type{nm("Integer"), nm("String")}}}},
		// tuples
		{"[]", Tuple{}},
		{"[Integer]", Tuple{Elems: []Type{nm("Integer")}}},
		{"[Integer, String]", Tuple{Elems: []Type{nm("Integer"), nm("String")}}},
		{"[[Integer, String], Symbol]", Tuple{Elems: []Type{Tuple{Elems: []Type{nm("Integer"), nm("String")}}, nm("Symbol")}}},
		{"[Integer, String]?", Optional{Elem: Tuple{Elems: []Type{nm("Integer"), nm("String")}}}},
		{"Array[[String, Integer]]", nm("Array", Tuple{Elems: []Type{nm("String"), nm("Integer")}})},
		// singleton
		{"singleton(Foo)", Singleton{Name: "Foo"}},
		{"singleton(::Foo)", Singleton{Name: "Foo"}},
		{"singleton(A::B)", Singleton{Name: "A::B"}},
		{"singleton(Foo)?", Optional{Elem: Singleton{Name: "Foo"}}},
		{"Array[singleton(Base)]", nm("Array", Singleton{Name: "Base"})},
		// the RBS gem accepts these too
		{"Array[]", nm("Array")},
		{"Array[String,]", nm("Array", nm("String"))},
		{"Hash[String, Integer,]", nm("Hash", nm("String"), nm("Integer"))},
		{"[Integer,]", Tuple{Elems: []Type{nm("Integer")}}},
		{"Base64", nm("Base64")},
		{"Array[Int32]", nm("Array", nm("Int32"))},
		{"String | Integer?", Union{Elems: []Type{nm("String"), Optional{Elem: nm("Integer")}}}},
		// keywords
		{"self", Self{}},
		{"self?", Optional{Elem: Self{}}},
		{"void", Void{}},
		{"nil", Nil{}},
		{"untyped", Untyped{}},
		{"top", Untyped{}},
		{"bool", Bool{}},
		{"boolish", Bool{}},
	}
	for _, c := range cases {
		t.Run(c.in, func(t *testing.T) {
			got, err := ParseType(c.in)
			if err != nil {
				t.Fatalf("ParseType(%q): %v", c.in, err)
			}
			if !reflect.DeepEqual(got, c.want) {
				t.Errorf("ParseType(%q) = %#v, want %#v", c.in, got, c.want)
			}
		})
	}
}

// The RBS gem reads a repeated `?` as one, so these must parse.
func TestParseTypeAccepts(t *testing.T) {
	for _, in := range []string{"String??", "(String)??", "singleton(Foo)??"} {
		_, err := ParseType(in)
		if err != nil {
			t.Errorf("ParseType(%q): %v", in, err)
		}
	}
}

func TestParseTypeErrors(t *testing.T) {
	cases := []struct{ in, errSub, known, name string }{
		{"", "", "", ""},
		{"   ", "", "", ""},
		{"Array[", "", "", ""},
		{"Array[String", "", "", ""},
		{"Array[String,", "", "", ""},
		{"(String", "", "", ""},
		{"String)", "trailing", "", ""},
		{"[String", "", "", ""},
		{"String]", "trailing", "", ""},
		{"String |", "", "", ""},
		{"| String", "", "", ""},
		{"?", "", "", ""},
		{"()", "", "", ""},
		{"String name", "trailing", "", ""},
		{"^(Integer) -> String", "proc types", "", ""},
		{":sym", "", "", ""},
		{`"str"`, "", "", ""},
		{"1", "", "", ""},
		{"String!", "", "", ""},
		{"singleton()", "singleton", "", ""},
		{"singleton(Foo", "", "", ""},
		{"singleton(Foo]", "", "", ""},
		{"::", "", "", ""},
		{"(String", "expected", "", ""},
		{"Hash[String,,Integer]", "", "", ""},
		{"singleton(?)", "singleton", "", ""},
		// the RBS gem rejects each of these
		{"Foo::", "", "", "trailing_namespace"},
		{"Foo::Bar::", "", "", "trailing_namespace_nested"},
		{"Hash[String Integer]", "", "", "args_missing_comma"},
		{"[Integer String]", "", "", "tuple_missing_comma"},
		{"singleton", "", "", "singleton_without_parens"},
	}
	for _, c := range cases {
		t.Run(cmp.Or(c.name, c.in), func(t *testing.T) {
			if c.known != "" {
				known(t, c.known)
			}
			got, err := within(t, c.in, ParseType)
			if err == nil {
				t.Fatalf("ParseType(%q) = %v, want an error", c.in, got)
			}
			if !strings.Contains(err.Error(), c.errSub) {
				t.Errorf("ParseType(%q): error %q does not mention %q", c.in, err, c.errSub)
			}
		})
	}
}

func TestParseMethodTypeParams(t *testing.T) {
	cases := []struct {
		in     string
		tps    []string
		params []Param
		ret    Type
	}{
		{"() -> void", nil, nil, Void{}},
		{"(String a, ?Integer b, *Symbol rest) -> void", nil, []Param{
			{Type: nm("String"), Name: "a"},
			{Type: nm("Integer"), Name: "b", Optional: true},
			{Type: nm("Symbol"), Name: "rest", Rest: true},
		}, Void{}},
		{"(?String? x) -> void", nil, []Param{{Type: Optional{Elem: nm("String")}, Name: "x", Optional: true}}, Void{}},
		{"(?String?) -> void", nil, []Param{{Type: Optional{Elem: nm("String")}, Optional: true}}, Void{}},
		{"(*untyped) -> nil", nil, []Param{{Type: Untyped{}, Rest: true}}, Nil{}},
		{"(String, ) -> void", nil, []Param{{Type: nm("String")}}, Void{}},
		{"(Integer) -> [String, Integer]", nil, []Param{{Type: nm("Integer")}}, Tuple{Elems: []Type{nm("String"), nm("Integer")}}},
		{"(Integer) -> (Integer | nil)", nil, []Param{{Type: nm("Integer")}}, Optional{Elem: nm("Integer")}},
		{"() -> singleton(Foo)", nil, nil, Singleton{Name: "Foo"}},
		{"[X] () -> X", []string{"X"}, nil, nm("X")},
		{"[K, V] (K, V) -> Hash[K, V]", []string{"K", "V"}, []Param{{Type: nm("K")}, {Type: nm("V")}}, nm("Hash", nm("K"), nm("V"))},
		{"(Integer, ?String) -> void", nil, []Param{{Type: nm("Integer")}, {Type: nm("String"), Optional: true}}, Void{}},
		// RBS reads the second word as a parameter name
		{"(String Integer) -> void", nil, []Param{{Type: nm("String"), Name: "Integer"}}, Void{}},
		{"(String | nil x) -> void", nil, []Param{{Type: Optional{Elem: nm("String")}, Name: "x"}}, Void{}},
	}
	for _, c := range cases {
		t.Run(c.in, func(t *testing.T) {
			m, err := ParseMethodType(c.in)
			if err != nil {
				t.Fatalf("ParseMethodType(%q): %v", c.in, err)
			}
			if !reflect.DeepEqual(m.TypeParams, c.tps) {
				t.Errorf("type params = %v, want %v", m.TypeParams, c.tps)
			}
			if !reflect.DeepEqual(m.Params, c.params) {
				t.Errorf("params = %#v, want %#v", m.Params, c.params)
			}
			if !reflect.DeepEqual(m.Return, c.ret) {
				t.Errorf("return = %#v, want %#v", m.Return, c.ret)
			}
			if m.Block != nil {
				t.Errorf("unexpected block %#v", m.Block)
			}
		})
	}
}

func TestParseMethodTypeBlocks(t *testing.T) {
	cases := []struct {
		in   string
		want Block
	}{
		{"() { (String) -> void } -> void", Block{Params: []Param{{Type: nm("String")}}, Return: Void{}}},
		{"() ?{ (String) -> void } -> void", Block{Params: []Param{{Type: nm("String")}}, Return: Void{}, Optional: true}},
		{"() { () -> Integer } -> Integer", Block{Return: nm("Integer")}},
		{"() { (String, *Integer) -> bool } -> void", Block{Params: []Param{{Type: nm("String")}, {Type: nm("Integer"), Rest: true}}, Return: Bool{}}},
		{"(Integer n) { (Integer i) -> void } -> void", Block{Params: []Param{{Type: nm("Integer"), Name: "i"}}, Return: Void{}}},
		{"[X] () { (self) -> X } -> X", Block{Params: []Param{{Type: Self{}}}, Return: nm("X")}},
		{"() { ([String, Integer]) -> String? } -> void", Block{Params: []Param{{Type: Tuple{Elems: []Type{nm("String"), nm("Integer")}}}}, Return: Optional{Elem: nm("String")}}},
	}
	for _, c := range cases {
		t.Run(c.in, func(t *testing.T) {
			m, err := ParseMethodType(c.in)
			if err != nil {
				t.Fatalf("ParseMethodType(%q): %v", c.in, err)
			}
			if m.Block == nil {
				t.Fatalf("ParseMethodType(%q): no block", c.in)
			}
			if !reflect.DeepEqual(*m.Block, c.want) {
				t.Errorf("block = %#v, want %#v", *m.Block, c.want)
			}
		})
	}
}

func TestParseMethodTypeErrors(t *testing.T) {
	cases := []struct{ in, errSub, known, name string }{
		{"", "", "", ""},
		{"()", "", "", ""},
		{"() ->", "", "", ""},
		{"String -> String", "", "", ""},
		{"(String -> void", "", "", ""},
		{"(String, Integer", "", "", ""},
		{"(,) -> void", "", "", ""},
		{"(String) String", "", "", ""},
		{"(String) -> void extra", "trailing", "", ""},
		{"(**String) -> void", "not supported", "", ""},
		{"(&Proc) -> void", "not supported", "", ""},
		{"^() -> void", "", "", ""},
		{"() -> ^() -> void", "proc types", "", ""},
		{"(String) {} -> void", "", "", ""},
		{"(String) { (String) } -> void", "", "", ""},
		{"(String) { (String) -> void -> void", "", "", ""},
		{"() ?{ (String) -> void }", "", "", ""},
		{"() ? -> void", "", "", ""},
		{"(name: String) -> void", "keyword", "", "keyword_param_message"},
		{"(?name: String) -> void", "keyword", "", "optional_keyword_param_message"},
		{"(String,,) -> void", "", "", ""},
		{"(String, Integer", "unterminated", "", ""},
		{"() { () -> } -> void", "", "", ""},
		{"() { (,) -> void } -> void", "", "", ""},
		{"() { (String) -> void } -> ", "", "", ""},
		{"[X (String) -> X", "", "", "type_params_unterminated"},
		{"[(] () -> void", "", "", "type_params_non_ident"},
		{"[X Y] () -> X", "", "", "type_params_missing_comma"},
		{"[] () -> void", "", "", "type_params_empty"},
		{"[X,] () -> X", "", "", "type_params_trailing_comma"},
		{"(String a Integer b) -> void", "", "", "params_missing_comma"},
		{"() ?( (String) -> void } -> void", "", "", "optional_block_brace"},
	}
	for _, c := range cases {
		t.Run(cmp.Or(c.name, c.in), func(t *testing.T) {
			if c.known != "" {
				known(t, c.known)
			}
			got, err := within(t, c.in, ParseMethodType)
			if err == nil {
				t.Fatalf("ParseMethodType(%q) = %v, want an error", c.in, got)
			}
			if !strings.Contains(err.Error(), c.errSub) {
				t.Errorf("ParseMethodType(%q): error %q does not mention %q", c.in, err, c.errSub)
			}
		})
	}
}
