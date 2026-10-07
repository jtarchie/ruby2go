//go:build rb2go_prelude

// Package prelude is concatenated verbatim into the output (loadPreludeGo). `go vet -tags rb2go_prelude ./prelude/go` type-checks it against generated stubs for the types generated code declares (0_stubs.go, decision 154).
package prelude

type Set_Any interface{ _ToAny() *Set[any] }

func NewSet[E comparable]() *Set[E] { return &Set[E]{h: NewHash[E, Boolean]()} }

// rbFrom converts v, a Set of any instantiation, into this one: a copy, each element converted (rbConv), as Array's rbFrom.
func (*Set[E]) rbFrom(v any) (*Set[E], bool) {
	s, ok := v.(Set_Any)
	if !ok {
		return nil, false
	}
	src := s._ToAny()
	if out, ok := any(src).(*Set[E]); ok {
		return out, true
	}
	out := NewSet[E]()
	for x := range src.Each() {
		e, ok := rbConv[E](x)
		if !ok {
			return nil, false
		}
		Set_Add(out, e)
	}
	return out, true
}

// rbEnumElems is xs's elements boxed as any, for any Array/Range/Set/Hash (every one has _to_any, a Hash's elements are [k, v] tuples); nil, false for anything else.
func rbEnumElems(xs any) (*Array[any], bool) {
	switch v := xs.(type) {
	case interface{ _ToAny() *Array[any] }:
		return v._ToAny(), true
	case interface{ _ToAny() *Range[any] }:
		return Range_ToA(v._ToAny()), true
	case interface{ _ToAny() *Set[any] }:
		return Set_ToA(v._ToAny()), true
	case interface{ _ToAny() *Hash[any, any] }:
		out := &Array[any]{}
		for _, t := range v._ToAny().ToA().s { // Enumerable's forwarder
			out.s = append(out.s, t)
		}
		return out, true
	}
	return nil, false
}
