package compiler

import (
	"strings"
	"testing"
)

// A rejected func becomes a raise in the Go text, and an import only its old body used is dropped (decision 303).
func TestStubGoFuncs(t *testing.T) {
	code := []byte(`package main

import (
	"fmt"
	"strings"
)

type String string

func Ref[T any](v T) *T { return &v }

func NewNotImplementedError(*String) any { return nil }

type X[T any] struct{}

func (self *X[T]) Bad() int {
	return strings.ToUpper(1)
}

func Good() int { return Bad2() + 1 }

func Bad2() int { var s string = 1; return len(s) }

func main() { fmt.Println(Good(), (&X[int]{}).Bad()) }
`)
	stubbed := map[string]bool{}
	code = stubRejected(code, func(fn, _ string) (string, bool) {
		stubbed[fn] = true
		return "rb2go: " + fn + " does not compile", fn != ""
	})
	if bad, unused := goTypeErrors(code); len(bad) > 0 || len(unused) > 0 {
		t.Fatalf("still rejected: %v %v\n%s", bad, unused, code)
	}
	if !stubbed["X.Bad"] || !stubbed["Bad2"] || stubbed["Good"] || strings.Contains(string(code), `"strings"`) {
		t.Fatalf("stubbed %v\n%s", stubbed, code)
	}
}
