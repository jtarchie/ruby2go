//go:build ignore

// Package prelude is concatenated verbatim into the output (loadPreludeGo), never built for real: types like String come from generated code.
package prelude

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
