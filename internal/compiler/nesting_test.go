package compiler

import (
	"context"
	"os"
	"strings"
	"testing"
)

func nest(open string, d int, core, close string) string {
	return strings.Repeat(open, d) + core + strings.Repeat(close, d)
}

// nestedPrograms: per construct, one expression nested d deep on the operand side that used to be generated twice.
var nestedPrograms = []struct {
	name string
	src  func(d int) string
}{
	{"or", func(d int) string {
		return "def f(a, b) #: (Integer?, Integer) -> Integer?\n  " + nest("(a || ", d, "b", ")") + "\nend\nputs f(nil, 1).inspect\n"
	}},
	{"and", func(d int) string {
		return "def f(a, b) #: (Integer?, Integer) -> Integer?\n  " + nest("(a && ", d, "b", ")") + "\nend\nputs f(1, 1).inspect\n"
	}},
	{"or_bool", func(d int) string {
		return "def f(a, b) #: (bool, bool) -> bool\n  " + nest("(a || ", d, "b", ")") + "\nend\nputs f(false, true).inspect\n"
	}},
	{"and_bool", func(d int) string {
		return "def f(a, b) #: (bool, bool) -> bool\n  " + nest("(a && ", d, "b", ")") + "\nend\nputs f(true, true).inspect\n"
	}},
	{"or_union", func(d int) string {
		return "def f(a, b) #: (Integer | String | nil, Integer) -> Integer | String\n  " + nest("(a || ", d, "b", ")") + "\nend\nputs f(nil, 1).inspect\n"
	}},
	{"or_call_arg", func(d int) string {
		return "def id(x) = x #: (Integer?) -> Integer?\ndef f(a, b) #: (Integer?, Integer) -> Integer?\n  " + nest("id(a || ", d, "b", ")") + "\nend\nputs f(nil, 1).inspect\n"
	}},
	{"safe_nav_arg", func(d int) string {
		return "def f(a) #: (Integer?) -> Integer?\n  " + nest("a&.+(", d, "1", ")") + "\nend\nputs f(1).inspect\n"
	}},
	{"safe_nav_block", func(d int) string {
		return "def f(a) #: (Integer?) -> Integer?\n  " + nest("a&.then { |x| ", d, "1", " }") + "\nend\nputs f(1).inspect\n"
	}},
	{"or_assign", func(d int) string {
		return "def f(a, b) #: (Integer?, Integer) -> Integer?\n  x = a #: Integer?\n  " + nest("x ||= (x ||= ", d, "b", ")") + "\nend\nputs f(nil, 1).inspect\n"
	}},
	{"and_assign", func(d int) string {
		return "def f(a, b) #: (Integer?, Integer) -> Integer?\n  " + nest("a &&= (a &&= ", d, "b", ")") + "\nend\nputs f(1, 1).inspect\n"
	}},
	{"if_expr", func(d int) string {
		return "def f(a) #: (bool) -> Integer\n  " + nest("x = if a then ", d, "1", " else 2 end") + "\n  x\nend\nputs f(true).inspect\n"
	}},
	{"case_expr", func(d int) string {
		return "def f(a) #: (Integer) -> Integer\n  " + nest("x = case a when 1 then ", d, "1", " else 2 end") + "\n  x\nend\nputs f(1).inspect\n"
	}},
	{"begin_expr", func(d int) string {
		return "def f(a) #: (Integer) -> Integer\n  " + nest("x = begin; ", d, "1", "; end") + "\n  x\nend\nputs f(1).inspect\n"
	}},
	{"ternary", func(d int) string {
		return "def f(a) #: (bool) -> Integer\n  " + nest("(a ? ", d, "1", " : 2)") + "\nend\nputs f(true).inspect\n"
	}},
	{"or_in_if", func(d int) string {
		return "def f(a, b) #: (Integer?, Integer) -> Integer\n  " + nest("if (a || ", d, "b", ") then 1 else 2 end") + "\nend\nputs f(nil, 1).inspect\n"
	}},
}

// genCountOf compiles src and returns its genExpr calls; over is set when they pass limit (the budget fails the compile fast).
func genCountOf(t *testing.T, src string, limit int) (n int, over bool) {
	t.Helper()
	opts := options{genCount: &n}
	if limit > 0 {
		opts.genLimit = &limit
	}
	_, err := compile(context.Background(), os.DirFS("../.."), []Source{{Name: "main.rb", Src: []byte(src)}}, &opts)
	if limit > 0 && n > limit {
		return n, true
	}
	if err != nil {
		t.Fatalf("%v\n%s", err, src)
	}
	return n, false
}

// A dry run per level regenerated its whole subtree (2^d, or worse); now a level adds work linear in its depth.
func TestNestedOperandsGenerateOnce(t *testing.T) {
	const depth = 14
	for _, p := range nestedPrograms {
		t.Run(p.name, func(t *testing.T) {
			t.Parallel()
			base, _ := genCountOf(t, p.src(1), 0)
			budget := base + 150*depth*depth
			got, over := genCountOf(t, p.src(depth), budget)
			t.Logf("depth %d: %d genExpr calls (depth 1: %d, budget %d)", depth, got, base, budget)
			if over {
				t.Fatalf("nested operands are regenerated per level: depth %d passed its budget of %d genExpr calls", depth, budget)
			}
		})
	}
}
