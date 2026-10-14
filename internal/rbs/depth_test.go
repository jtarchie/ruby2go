package rbs

import (
	"strings"
	"testing"
)

// TestIncludeKind: prepend is not recorded as include.
func TestIncludeKind(t *testing.T) {
	f, err := ParseFile("class C\n  include A\n  extend B\n  prepend D\nend\n")
	if err != nil {
		t.Fatal(err)
	}
	got := make([]string, 0, 3)
	for _, inc := range f.Decls[0].Includes {
		got = append(got, inc.Kind+" "+inc.Module)
	}
	if strings.Join(got, ",") != "include A,extend B,prepend D" {
		t.Errorf("kinds = %v", got)
	}
}

// TestParseFileNestingLimit: each nested name embeds its parent's, so unbounded nesting is quadratic memory.
func TestParseFileNestingLimit(t *testing.T) {
	src := strings.Repeat("module M\n", 20000)
	_, err := ParseFile(src)
	if err == nil || !strings.Contains(err.Error(), "nested too deeply") {
		t.Errorf("want a nesting error, got %v", err)
	}
	_, err = ParseFile(strings.Repeat("module M\n", 50) + strings.Repeat("end\n", 50))
	if err != nil {
		t.Errorf("50-deep modules: %v", err)
	}
}

// TestParseDepthLimit: deep nesting in `#:` or .rbs input is an error, not a fatal Go stack overflow.
func TestParseDepthLimit(t *testing.T) {
	const n = 10000
	rep := strings.Repeat
	for name, s := range map[string]string{
		"paren":    rep("(", n) + "A" + rep(")", n),
		"tuple":    rep("[", n) + "A" + rep("]", n),
		"proc":     rep("^(", n) + "A" + rep(") -> A", n),
		"colons":   rep("::", n) + "A",
		"generic":  rep("A[", n) + "A" + rep("]", n),
		"optional": "A" + rep("?", n),
		"unclosed": rep("(", 5_000_000), // the size that was a fatal overflow
	} {
		_, err := ParseType(s)
		if err == nil || !strings.Contains(err.Error(), "nested too deeply") {
			t.Errorf("ParseType(%s): want a nesting error, got %v", name, err)
		}
		_, err = ParseMethodType("(" + s + ") -> void")
		if err == nil {
			t.Errorf("ParseMethodType(%s): want an error", name)
		}
	}
	// the limit leaves ordinary depth alone
	_, err := ParseType(rep("Array[", 50) + "A" + rep("]", 50))
	if err != nil {
		t.Errorf("50-deep generic: %v", err)
	}
}
