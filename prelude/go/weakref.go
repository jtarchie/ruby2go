//go:build rb2go_prelude

// Package prelude is concatenated verbatim into the output (loadPreludeGo). `go vet -tags rb2go_prelude ./prelude/go` type-checks it against generated stubs for the types generated code declares (0_stubs.go, decision 154).
package prelude

import (
	"unsafe"
	"weak"
)

// rbWeakSet points r at obj (decision 110): weakly, through the object's
// first byte, when obj has identity (its Go type is a pointer,
// rbClassRefs); strongly otherwise, so a value is always alive. T is
// usually an interface (a struct class's FooI), so the pointer is the
// data word of obj as `any`, and the type word is kept to rebuild it.
func rbWeakSet[T comparable](r *WeakRef[T], obj T) {
	a := any(obj)
	if !rbIsRef(a) {
		r.weak, r.strong = false, a
		return
	}
	words := (*[2]unsafe.Pointer)(unsafe.Pointer(&a)) //nolint:gosec // an interface is a type word and a data word; a pointer value is the data word itself
	r.weak, r.strong, r.typ = true, nil, words[0]
	r.wp = weak.Make((*byte)(words[1]))
}

// rbWeakGet is __getobj__: the referent, or RefError once it was collected.
func rbWeakGet[T comparable](r *WeakRef[T]) T {
	if !r.weak {
		return r.strong.(T)
	}
	p := r.wp.Value()
	if p == nil {
		panic(NewWeakRef_RefError(Ref(String("Invalid Reference - probably recycled"))))
	}
	var a any
	words := (*[2]unsafe.Pointer)(unsafe.Pointer(&a)) //nolint:gosec // the inverse of rbWeakSet's split
	words[0], words[1] = r.typ, unsafe.Pointer(p)
	return a.(T)
}

// WeakRef_Any is the untyped view a class switch asks for (rbClassOf), as Queue_Any is.
type WeakRef_Any interface{ _ToAny() *WeakRef[any] }

// rbWeakAny views a WeakRef[T] as WeakRef[any]: the fields do not depend on T.
func rbWeakAny[T comparable](r *WeakRef[T]) *WeakRef[any] {
	return (*WeakRef[any])(unsafe.Pointer(r)) //nolint:gosec // same layout for every T
}
