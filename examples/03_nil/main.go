package main

import (
	"fmt"
	"strconv"
	"strings"
)

// ---- prelude (subset)
type String string
type Integer int

func (self String) Upcase() String  { return String(strings.ToUpper(string(self))) }
func (self String) Size() Integer   { return Integer(len(self)) }
func (self String) Inspect() String { return String(strconv.Quote(string(self))) }
func (self Integer) ToS() String    { return String(strconv.Itoa(int(self))) }
func NilClass_Inspect() String      { return "nil" }

// Hash[K,V]#[] : (K) -> V?   — Ruby core RBS says V; prelude says V? (honest).
type Hash[K comparable, V any] map[K]V

func (self Hash[K, V]) Index(k K) *V {
	if v, ok := self[k]; ok {
		return &v
	}
	return nil
}

// ---- user code

//line main.rb:4
func greeting(h Hash[String, String], k String) String {
	// h[k]&.upcase  → String?
	var t1 *String
	if v := h.Index(k); v != nil {
		u := v.Upcase() // *String auto-derefs for value-receiver methods
		t1 = &u
	}
	// t1 || "DEFAULT" → String? | String = String. Only nil/false are falsy.
	if t1 != nil {
		return *t1
	}
	return "DEFAULT"
}

func main() {
//line main.rb:8
	h := Hash[String, String]{"a": "hi"}
	fmt.Println(greeting(h, "a"))
	fmt.Println(greeting(h, "b"))

	name := h.Index("a") // String? → *String
	if name != nil {     // `if name` narrows to String; no cast needed in Go
		fmt.Println(name.Size().ToS())
	}
	// x.inspect on String? → static branch on nil, NilClass#inspect
	inspect := func(s *String) String {
		if s == nil {
			return NilClass_Inspect()
		}
		return s.Inspect()
	}
	fmt.Println(inspect(name))
	fmt.Println(inspect(h.Index("zz")))
}
