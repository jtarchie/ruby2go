//go:build ignore

// Package prelude is concatenated verbatim into the output (loadPreludeGo), never built for real: types like String come from generated code.
package prelude

type Set_Any interface{ _ToAny() *Set[any] }

func NewSet[E comparable]() *Set[E] { return &Set[E]{h: NewHash[E, Boolean]()} }
