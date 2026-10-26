package compiler

import (
	"bytes"
	"go/format"
	"go/parser"
	"go/token"
	"strings"
	"testing"
)

func TestSpecializeSelf(t *testing.T) {
	src := `package main

type Array[E comparable] struct{ s []E }
type Hash[K, V comparable] struct{}

func Enumerable_Sum[Self Enumerable_Self[Self, E], E comparable](self Self) E { return Enumerable_First[Self, E](self) }
func Enumerable_First[Self Enumerable_Self[Self, E], E comparable](self Self) E { var z E; return z }
func Enumerable_Cnt[Self Enumerable_Self[Self, E], E any](self Self) int { return 0 }

func main() {
	_ = Enumerable_Sum[*Array[int], int](nil)
	_ = Enumerable_Sum[*Array[int], string](nil)
	_ = Enumerable_Sum[*Hash[int, int], int](nil)
	_ = Enumerable_Cnt[*Array[int], int](nil)
}
`
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "main.go", src, parser.ParseComments)
	if err != nil {
		t.Fatal(err)
	}
	labels := map[string]string{"Enumerable_Sum": "Enumerable#sum"}
	err = specializeSelf(fset, f, labels)
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	err = format.Node(&buf, fset, f)
	if err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{
		"_ = Enumerable_Sum_onArray[int](nil)",                          // rewritten
		"_ = Enumerable_Sum[*Array[int], string](nil)",                  // E is not Array's element: left alone
		"_ = Enumerable_Sum[*Hash[int, int], int](nil)",                 // two-parameter container: left alone
		"_ = Enumerable_Cnt[*Array[int], int](nil)",                     // E any cannot instantiate Array[E comparable]
		"func Enumerable_Sum_onArray[E comparable](self *Array[E]) E {", // the copy
		"return Enumerable_First_onArray[E](self)",                      // calls inside a copy specialize too
		"func Enumerable_First_onArray[E comparable](self *Array[E]) E {",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in\n%s", want, out)
		}
	}
	if labels["Enumerable_Sum_onArray"] != "Enumerable#sum" {
		t.Errorf("copy has no frame label: %v", labels)
	}
}
