package main

import (
	"fmt"
	"iter"
	"slices"
	"strconv"
	"strings"
)

// ---- prelude (subset)
type String string
type Integer int
type Boolean bool

func (self String) Plus(o String) String   { return self + o }
func (self String) Cmp(o String) Integer   { return Integer(strings.Compare(string(self), string(o))) }
func (self Integer) Cmp(o Integer) Integer { return Integer(self - o) } // sign only matters
func (self Integer) Neg() Integer          { return -self }
func (self Integer) ToS() String           { return String(strconv.Itoa(int(self))) }
func (self String) Split() *Array[String] {
	out := &Array[String]{}
	for _, f := range strings.Fields(string(self)) {
		out.Push(String(f))
	}
	return out
}

type Comparable_Self[T any] interface{ Cmp(T) Integer }
type Enumerable_Self[E any] interface{ Each() iter.Seq[E] }

// RBS tuple [A, B] → generated struct. Array#<=> on tuples = lexicographic.
type Tuple2[A, B any] struct {
	F0 A
	F1 B
}

func Tuple2_Cmp[A Comparable_Self[A], B Comparable_Self[B]](x, y Tuple2[A, B]) Integer {
	if c := x.F0.Cmp(y.F0); c != 0 {
		return c
	}
	return x.F1.Cmp(y.F1)
}

// Comparable_Self is satisfied by a method, so a tuple needs a wrapper type per
// instantiation; transpiler emits it when a tuple is used as a sort key.
type Tuple2IS Tuple2[Integer, String]

func (self Tuple2IS) Cmp(o Tuple2IS) Integer {
	return Tuple2_Cmp(Tuple2[Integer, String](self), Tuple2[Integer, String](o))
}

// Enumerable, written once against Each().
func Enumerable_Tally[Self Enumerable_Self[E], E comparable](self Self) Hash[E, Integer] {
	out := Hash[E, Integer]{}
	for x := range self.Each() {
		out[x]++
	}
	return out
}

// sort_by: keys computed once (Ruby does the same), then sorted by <=>.
func Enumerable_SortBy[Self Enumerable_Self[E], E any, K Comparable_Self[K]](self Self, blk func(E) K) *Array[E] {
	type kv struct {
		k K
		v E
	}
	tmp := []kv{}
	for x := range self.Each() {
		tmp = append(tmp, kv{blk(x), x})
	}
	slices.SortFunc(tmp, func(a, b kv) int { return int(a.k.Cmp(b.k)) })
	out := &Array[E]{}
	for _, p := range tmp {
		out.Push(p.v)
	}
	return out
}

func Enumerable_First[Self Enumerable_Self[E], E any](self Self, n Integer) *Array[E] {
	out := &Array[E]{}
	for x := range self.Each() {
		if Integer(len(*out)) >= n {
			break // Go iterator stops when yield returns false
		}
		out.Push(x)
	}
	return out
}

// @go_type []E
type Array[E any] []E

func (self *Array[E]) Push(x E) *Array[E] { *self = append(*self, x); return self }
func (self *Array[E]) Each() iter.Seq[E] {
	return func(yield func(E) bool) {
		for _, x := range *self {
			if !yield(x) {
				return
			}
		}
	}
}

// Hash#each yields [K, V] pairs → Each() over Tuple2. Go map order is random;
// Ruby preserves insertion order. Prelude Hash must track key order (omitted).
type Hash[K comparable, V any] map[K]V

func (self Hash[K, V]) Each() iter.Seq[Tuple2[K, V]] {
	return func(yield func(Tuple2[K, V]) bool) {
		for k, v := range self {
			if !yield(Tuple2[K, V]{k, v}) {
				return
			}
		}
	}
}

// ---- user code
func main() {
//line main.rb:2
	text := String("the cat and the hat and the bat")
	counts := Enumerable_Tally(text.Split())
	sorted := Enumerable_SortBy(counts, func(p Tuple2[String, Integer]) Tuple2IS {
		w, n := p.F0, p.F1 // |w, n| destructures the pair
		return Tuple2IS{n.Neg(), w}
	})
	for p := range Enumerable_First(sorted, 3).Each() {
		w, n := p.F0, p.F1
		fmt.Println(w.Plus(": ").Plus(n.ToS()))
	}
}
