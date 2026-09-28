//go:build ignore

// Package prelude is concatenated verbatim into the output (loadPreludeGo), never built for real: types like String come from generated code.
package prelude

// rbFrom converts v, an Array of any instantiation, into this one: a
// copy, each element converted (rbConv). The receiver only names the
// instantiation.
func (*Array[E]) rbFrom(v any) (*Array[E], bool) {
	a, ok := v.(Array_Any)
	if !ok {
		return nil, false
	}
	src := a._ToAny()
	if out, ok := any(src).(*Array[E]); ok {
		return out, true // E is untyped: _ToAny copied
	}
	out := make(Array[E], 0, len(*src))
	for _, x := range *src {
		e, ok := rbConv[E](x)
		if !ok {
			return nil, false
		}
		out = append(out, e)
	}
	return &out, true
}

func NewArray[E comparable]() *Array[E] { return &Array[E]{} }
