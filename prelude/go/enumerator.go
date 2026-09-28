//go:build ignore

// Package prelude is concatenated verbatim into the output (loadPreludeGo), never built for real: types like String come from generated code.
package prelude

type Enumerator_Map_Any interface{ _ToAny() *Enumerator_Map[any] }

type Enumerator_Select_Any interface{ _ToAny() *Enumerator_Select[any] }
