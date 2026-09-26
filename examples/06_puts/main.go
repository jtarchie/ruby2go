package main

import (
	"bufio"
	"os"
	"strconv"
	"strings"
)

// ---- prelude (subset)
type String string
type Integer int
type Float float64

func (self String) ToS() String            { return self }
func (self String) Plus(o String) String   { return self + o }
func (self String) EndWithQ(s String) bool { return strings.HasSuffix(string(self), string(s)) }
func (self Integer) ToS() String           { return String(strconv.Itoa(int(self))) }
func (self Float) ToS() String {
	s := strconv.FormatFloat(float64(self), 'f', -1, 64)
	if !strings.ContainsAny(s, ".eE") {
		s += ".0"
	}
	return String(s)
}

// @go_type []E
type Array[E any] []E

// Array#[] : (Integer) -> E?
func (self *Array[E]) Index(i Integer) *E {
	if int(i) < 0 || int(i) >= len(*self) {
		return nil
	}
	return &(*self)[i]
}

// `when Array` can't match a generic *Array[E] in a Go type switch; every
// Array instantiation satisfies this non-generic interface instead.
type Array_Any interface{ ToAAny() []any }

func (self *Array[E]) ToAAny() []any {
	out := make([]any, len(*self))
	for i, x := range *self {
		out[i] = x
	}
	return out
}

// T? → any: typed nil pointer must become untyped nil or `case nil` misses it.
func Opt[T any](p *T) any {
	if p == nil {
		return nil
	}
	return *p
}

// Kernel#to_s exists on every object, so `a.to_s` on untyped is a safe assertion.
type Kernel_ToS interface{ ToS() String }

var stdout = bufio.NewWriter(os.Stdout)

//line prelude.rb:43
func Kernel___write(s String) { stdout.WriteString(string(s)) }

//line prelude.rb:27
func Kernel_Puts(args ...any) {
	if len(args) == 0 {
		Kernel___write("\n")
		return
	}
	for _, a := range args {
		switch a := a.(type) {
		case nil:
			Kernel___write("\n")
		case Array_Any:
			Kernel_Puts(a.ToAAny()...) // puts(*a)
		default:
			s := a.(Kernel_ToS).ToS()
			if s.EndWithQ("\n") {
				Kernel___write(s)
			} else {
				Kernel___write(s.Plus("\n"))
			}
		}
	}
}

// ---- user code

type Point struct{ x, y Integer }

//line main.rb:8
func NewPoint(x, y Integer) *Point { return &Point{x, y} }

func (self *Point) X() Integer { return self.x }
func (self *Point) Y() Integer { return self.y }

//line main.rb:14
func (self *Point) ToS() String {
	return String("(").Plus(self.X().ToS()).Plus(", ").Plus(self.Y().ToS()).Plus(")")
}

func main() {
	defer stdout.Flush() // runs on normal return and on panic
//line main.rb:17
	words := &Array[String]{"a", "b"}

	Kernel_Puts(String("hello")) // literal → any needs explicit type
	Kernel_Puts(String("no double newline\n"))
	Kernel_Puts()
	Kernel_Puts(Integer(42), Float(2.0))
	Kernel_Puts(nil)
	Kernel_Puts(Opt(words.Index(5)))
	Kernel_Puts(&Array[*Array[Integer]]{{1, 2}, {3}})
	Kernel_Puts(NewPoint(1, 2))
	Kernel_Puts(String("interp: ").Plus(NewPoint(3, 4).ToS()))
}
