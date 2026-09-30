package compiler

import (
	"cmp"
	"go/token"
	"io/fs"
	"os"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// known skips a design-record-disagreeing case unless RB2GO_RUN_SKIPPED=1, like `# skip:` in testdata.
func known(t *testing.T, reason string) {
	t.Helper()
	if os.Getenv("RB2GO_RUN_SKIPPED") == "" {
		t.Skip("known: " + reason)
	}
}

func TestGoMethodName(t *testing.T) {
	cases := []struct{ in, want string }{
		// decision 3 table
		{"==", "Op_eq"}, {"!=", "Op_ne"}, {"<=>", "Op_cmp"}, {"<", "Op_lt"}, {"<=", "Op_le"}, {">", "Op_gt"}, {">=", "Op_ge"},
		{"+", "Op_plus"}, {"-", "Op_minus"}, {"*", "Op_mul"}, {"/", "Op_div"}, {"%", "Op_mod"}, {"**", "Op_pow"},
		{"-@", "Op_neg"}, {"+@", "Op_pos"}, {"!", "Op_not"}, {"~", "Op_inv"}, {"<<", "Op_shl"}, {">>", "Op_shr"},
		{"&", "Op_bitAnd"}, {"|", "Op_bitOr"}, {"^", "Op_bitXor"}, {"=~", "Op_eqTilde"}, {"!~", "Op_notTilde"},
		{"===", "Op_eqq"}, {"[]", "Op_idx"}, {"[]=", "Op_idxSet"}, {"`", "Op_backtick"},
		// camel-casing and suffixes
		{"x", "X"},
		{"to_s", "ToS"},
		{"each_with_index", "EachWithIndex"},
		{"end_with?", "EndWithQ"},
		{"empty?", "EmptyQ"},
		{"upcase!", "UpcaseBang"},
		{"name=", "NameSet"},
		// a trailing suffix word stays as written (decision 3)
		{"empty_q", "Empty_q"},
		{"upcase_bang", "Upcase_bang"},
		{"name_set", "Name_set"},
		{"__set", "__Set"},
		{"reset", "Reset"},
		{"utf8?", "Utf8Q"},
		{"a2b", "A2b"},
		{"Integer", "Integer"},
		// leading underscores are kept (decision 3)
		{"__write", "__Write"},
		{"_foo", "_Foo"},
		{"__send__", "__Send__"},
		{"foo__bar", "Foo_Bar"},
		{"_private?", "_PrivateQ"},
	}
	for _, c := range cases {
		t.Run(c.in, func(t *testing.T) {
			if got := goMethodName(c.in); got != c.want {
				t.Errorf("goMethodName(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

// The table must match decision 3 exactly: no extra or missing operators.
func TestOpNamesMatchReadme(t *testing.T) {
	readme := map[string]string{
		"==": "Op_eq", "!=": "Op_ne", "<=>": "Op_cmp", "<": "Op_lt", "<=": "Op_le", ">": "Op_gt", ">=": "Op_ge",
		"+": "Op_plus", "-": "Op_minus", "*": "Op_mul", "/": "Op_div", "%": "Op_mod", "**": "Op_pow",
		"-@": "Op_neg", "+@": "Op_pos", "!": "Op_not", "~": "Op_inv", "<<": "Op_shl", ">>": "Op_shr",
		"&": "Op_bitAnd", "|": "Op_bitOr", "^": "Op_bitXor", "=~": "Op_eqTilde", "!~": "Op_notTilde",
		"===": "Op_eqq", "[]": "Op_idx", "[]=": "Op_idxSet", "`": "Op_backtick",
	}
	if len(opNames) != len(readme) {
		t.Errorf("opNames has %d entries, docs/design.md decision 3 lists %d", len(opNames), len(readme))
	}
	for op, want := range readme {
		if got := opNames[op]; got != want {
			t.Errorf("opNames[%q] = %q, docs/design.md says %q", op, got, want)
		}
	}
}

var (
	defRe  = regexp.MustCompile(`(?m)^[ \t]*def[ \t]+(?:self\.)?([^\s(;]+)`)
	attrRe = regexp.MustCompile(`(?m)^[ \t]*attr_(reader|writer|accessor)[ \t]+([^\n]*)`)
	symRe  = regexp.MustCompile(`:(\w+)`)
)

// rubyMethodNames greps method names out of the prelude and examples.
func rubyMethodNames(t *testing.T) []string {
	t.Helper()
	root := os.DirFS("../..")
	files, err := fs.Glob(root, "prelude/*.rb")
	if err != nil {
		t.Fatal(err)
	}
	ex, err := fs.Glob(root, "examples/*/main.rb")
	if err != nil {
		t.Fatal(err)
	}
	files = append(append(files, "prelude.rb"), ex...)
	if len(files) < 20 {
		t.Fatalf("found only %d Ruby files; wrong working directory?", len(files))
	}
	set := map[string]bool{}
	for _, f := range files {
		src, err := fs.ReadFile(root, f)
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range defRe.FindAllSubmatch(src, -1) {
			set[string(m[1])] = true
		}
		for _, m := range attrRe.FindAllSubmatch(src, -1) {
			args, _, _ := strings.Cut(string(m[2]), "#")
			for _, s := range symRe.FindAllStringSubmatch(args, -1) {
				if string(m[1]) != "writer" {
					set[s[1]] = true
				}
				if string(m[1]) != "reader" {
					set[s[1]+"="] = true
				}
			}
		}
	}
	if len(set) < 100 {
		t.Fatalf("found only %d method names; grep broken?", len(set))
	}
	names := make([]string, 0, len(set))
	for n := range set {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

func TestGoMethodNameInjective(t *testing.T) {
	names := rubyMethodNames(t)
	for op := range opNames {
		names = append(names, op)
	}
	byGo := map[string]string{}
	for _, n := range names {
		g := goMethodName(n)
		if !token.IsIdentifier(g) {
			t.Errorf("goMethodName(%q) = %q is not a Go identifier", n, g)
		}
		if prev, ok := byGo[g]; ok && prev != n {
			t.Errorf("goMethodName collision: %q and %q both map to %q", prev, n, g)
		}
		byGo[g] = n
		if f := goFuncName(n); !token.IsIdentifier(f) {
			t.Errorf("goFuncName(%q) = %q is not a Go identifier", n, f)
		}
	}
}

// Decision 3: the operator table must stay injective against camel-cased names
// (`[]` is not `Index` because String#index exists). These MRI core methods
// sit on the same class as the operator (Integer has `/` and `div`, `**` and
// `pow`; Pathname has `+` and private `plus`), or are core names a user class
// can define beside it (IO#pos next to `+@`). The prelude defines none of the
// four yet, so TestGoMethodNameInjective cannot see them.
func TestGoMethodNameCoreCollisions(t *testing.T) {
	cases := []struct{ a, b, known string }{
		{"[]", "index", ""},
		{"[]=", "index=", ""},
		{"=~", "match", ""},
		{"!~", "match?", ""},
		{"<=>", "compare", ""},
		{"%", "modulo", ""},
		{"-@", "negate", ""},
		{"empty?", "empty", ""},
		{"upcase!", "upcase", ""},
		{"name=", "name", ""},
		{"__write", "write", ""},
		{"_foo", "foo", ""},
		{"/", "div", ""},
		{"**", "pow", ""},
		{"+@", "pos", ""},
		{"+", "plus", ""},
	}
	for _, c := range cases {
		t.Run(c.b, func(t *testing.T) {
			if c.known != "" {
				known(t, c.known)
			}
			if ga, gb := goMethodName(c.a), goMethodName(c.b); ga == gb {
				t.Errorf("%q and %q both map to %q", c.a, c.b, ga)
			}
		})
	}
}

// Every name Ruby lets you `def` must become a Go identifier.
func TestGoMethodNameIsIdentifier(t *testing.T) {
	cases := []struct{ name, label, known string }{
		{"_", "", ""},
		{"__", "", ""},
		{"x_", "", ""},
		{"_x_", "", ""},
		{"a__b?", "", ""},
		{"café", "", ""},
		{"naïve?", "", ""},
		{"call", "", ""},
		{"`", "backtick", ""},
	}
	for _, c := range cases {
		t.Run(cmp.Or(c.label, c.name), func(t *testing.T) {
			if c.known != "" {
				known(t, c.known)
			}
			if g := goMethodName(c.name); !token.IsIdentifier(g) || !token.IsExported(g) && !strings.HasPrefix(g, "_") {
				t.Errorf("goMethodName(%q) = %q, not an exported (or `_`-prefixed) Go identifier", c.name, g)
			}
		})
	}
}

func TestGoLocalName(t *testing.T) {
	cases := []struct{ in, want string }{
		{"x", "x"},
		{"count", "count"},
		{"self", "self"},
		{"type", "type_"},
		{"func", "func_"},
		{"range", "range_"},
		{"len", "len_"},
		{"string", "string_"},
		{"nil", "nil_"},
		{"main", "main_"},
		{"init", "init_"},
		{"stdout", "stdout_"},
		{"append", "append_"},
		{"make", "make_"},
		{"copy", "copy_"},
		{"any", "any_"},
		{"error", "error_"},
		{"bool", "bool_"},
		{"_", "_"},
		{"_tmp", "_tmp"},
		// a trailing `_` is the generated locals' form (t1_, r_, ret_, rest_)
		{"ret_", "ret__"},
		{"x__", "x___"},
		{"len_", "len__"},
		{"blk", "blk_"},
	}
	for _, c := range cases {
		t.Run(c.in, func(t *testing.T) {
			if got := goLocalName(c.in); got != c.want {
				t.Errorf("goLocalName(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

// Every Go keyword is a legal Ruby local name, so each must be renamed.
func TestGoLocalNameKeywords(t *testing.T) {
	for tok := token.BREAK; tok <= token.VAR; tok++ {
		kw := tok.String()
		if !token.IsKeyword(kw) {
			continue
		}
		if got := goLocalName(kw); !token.IsIdentifier(got) {
			t.Errorf("goLocalName(%q) = %q is not a Go identifier", kw, got)
		}
	}
}

func TestGoFieldName(t *testing.T) {
	cases := []struct{ in, want string }{
		{"@x", "x"},
		{"@name", "name"},
		{"@type", "type_"},
		{"@map", "map_"},
		{"@len", "len_"},
		{"@_cache", "_cache"},
	}
	for _, c := range cases {
		t.Run(c.in, func(t *testing.T) {
			if got := goFieldName(c.in); got != c.want {
				t.Errorf("goFieldName(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

func TestGoFuncName(t *testing.T) {
	cases := []struct{ in, want string }{
		{"main", "rb_Main"},
		{"puts", "rb_Puts"},
		{"fib", "rb_Fib"},
		{"prime?", "rb_PrimeQ"},
		{"do_it!", "rb_DoItBang"},
		{"__helper", "rb___Helper"},
	}
	for _, c := range cases {
		t.Run(c.in, func(t *testing.T) {
			if got := goFuncName(c.in); got != c.want {
				t.Errorf("goFuncName(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}
