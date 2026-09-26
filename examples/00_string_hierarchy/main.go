package main

import (
	"strings"
	"unsafe"
)

// @go_type bool
type Boolean bool

// @go_type int
type Integer int

// @go_type string
type String string

// top-level %x{}
//
//line prelude.rb:6
func identical(a, b any) bool {
	switch a := a.(type) {
	case String:
		b, ok := b.(String)
		return ok && len(a) == len(b) && unsafe.StringData(string(a)) == unsafe.StringData(string(b))
	default:
		return a == b
	}
}

// BasicObject: inherited by everything → free func, generic over Self.
//
//line prelude.rb:19
func BasicObject_EqualQ[Self any](self Self, other any) Boolean {
	return Boolean(identical(self, other))
}

// Kernel#then has a type param [X] → free func only, no forwarder.
//
//line prelude.rb:24
func Kernel_Then[Self, X any](self Self, blk func(Self) X) X {
	return blk(self)
}

// Comparable: constraint = methods the module body calls on self.
type Comparable_Self[T any] interface {
	Cmp(T) Integer
}

//line prelude.rb:33
func Comparable_Lt[Self Comparable_Self[Self]](self, other Self) Boolean {
	return self.Cmp(other).Lt(0)
}

//line prelude.rb:36
func Comparable_Clamp[Self Comparable_Self[Self]](self, lo, hi Self) Self {
	if Comparable_Lt(self, lo) {
		return lo
	}
	if self.Cmp(hi).Gt(0) {
		return hi
	}
	return self
}

//line prelude.rb:50
func (self Integer) Lt(other Integer) Boolean { return Boolean(self < other) }

//line prelude.rb:53
func (self Integer) Gt(other Integer) Boolean { return Boolean(self > other) }

//line prelude.rb:61
func (self String) Upcase() String { return String(strings.ToUpper(string(self))) }

//line prelude.rb:64
func (self String) Cmp(other String) Integer {
	return Integer(strings.Compare(string(self), string(other)))
}

//line prelude.rb:67
func (self String) Eq(other String) Boolean { return Boolean(self == other) }

//line prelude.rb:70
func (self String) Plus(other String) String { return self + other }

//line prelude.rb:73
func (self String) Dup() String { return String(strings.Clone(string(self))) }

// String forwarders for inherited/included non-generic methods.
func (self String) Lt(other String) Boolean    { return Comparable_Lt(self, other) }
func (self String) Clamp(lo, hi String) String { return Comparable_Clamp(self, lo, hi) }
func (self String) EqualQ(other any) Boolean   { return BasicObject_EqualQ(self, other) }

// ---- main.rb

//line main.rb:5
func assert(value Boolean, msg String) {
	if !value {
		panic(msg)
	}
}

func main() {
//line main.rb:9
	s := String("hello, world")
	assert(s.Upcase().Eq("HELLO, WORLD"), "upcase")
	assert(s.Lt("world"), "<")
	assert(s.Clamp("a", "c").Eq("c"), "clamp")
	assert(Kernel_Then(s, func(x String) String { return x.Plus(x) }).Eq(s.Plus(s)), "then")
	assert(s.EqualQ(s), "equal?")
	assert(!s.EqualQ(s.Dup()), "equal? on dup")
}
