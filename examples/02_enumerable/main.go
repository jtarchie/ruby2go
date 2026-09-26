package main

import (
	"fmt"
	"iter"
	"strings"
)

// ---- prelude (subset)
type Integer int
type Boolean bool

func (self Integer) Mul(o Integer) Integer  { return self * o }
func (self Integer) Plus(o Integer) Integer { return self + o }
func (self Integer) EvenQ() Boolean         { return self%2 == 0 }

// module Enumerable: body calls self.each → constraint. `each` maps to a Go iterator.
type Enumerable_Self[E any] interface {
	Each() iter.Seq[E]
}

// [U] type param → free func.
func Enumerable_Map[Self Enumerable_Self[E], E, U any](self Self, blk func(E) U) *Array[U] {
	out := &Array[U]{}
	for x := range self.Each() {
		out.Push(blk(x))
	}
	return out
}

func Enumerable_Select[Self Enumerable_Self[E], E any](self Self, blk func(E) Boolean) *Array[E] {
	out := &Array[E]{}
	for x := range self.Each() {
		if blk(x) {
			out.Push(x)
		}
	}
	return out
}

func Enumerable_Reduce[Self Enumerable_Self[E], E, A any](self Self, acc A, blk func(A, E) A) A {
	for x := range self.Each() {
		acc = blk(acc, x)
	}
	return acc
}

// Array is mutable + aliased in Ruby → always handled as *Array[E].
// @go_type []Elem
type Array[E any] []E

func (self *Array[E]) Each() iter.Seq[E] {
	return func(yield func(E) bool) {
		for _, x := range *self {
			if !yield(x) {
				return
			}
		}
	}
}
func (self *Array[E]) Push(x E) *Array[E] { *self = append(*self, x); return self }
func (self *Array[E]) Inspect() string {
	parts := make([]string, len(*self))
	for i, x := range *self {
		parts[i] = fmt.Sprint(x)
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

// ---- user code
func main() {
//line main.rb:2
	nums := &Array[Integer]{1, 2, 3, 4}
	evens := Enumerable_Select(nums, func(x Integer) Boolean { return x.EvenQ() }) // &:even?
	total := Enumerable_Reduce(
		Enumerable_Map(nums, func(n Integer) Integer { return n.Mul(10) }),
		Integer(0),
		func(acc, n Integer) Integer { return acc.Plus(n) },
	)
	fmt.Println(evens.Inspect())
	fmt.Println(total)
}
