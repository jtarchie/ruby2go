//go:build ignore

// Package prelude is concatenated verbatim into the output (loadPreludeGo), never built for real: types like String come from generated code.
package prelude

// How a dynamic call was made, as MRI tells them apart: with a receiver
// (public methods only), without one or through send (private methods
// too), or as a bare name (NameError when missing).
const (
	rbCall = iota
	rbFCall
	rbVCall
)

// rbArity raises MRI's ArgumentError for a wrong argument count.
func rbArity(given, min, max int) {
	if given >= min && (max < 0 || given <= max) {
		return
	}
	expected := strconv.Itoa(min)
	switch {
	case max < 0:
		expected += "+"
	case max != min:
		expected += ".." + strconv.Itoa(max)
	}
	panic(NewArgumentError(Ref(String(fmt.Sprintf("wrong number of arguments (given %d, expected %s)", given, expected)))))
}

// rbNumMixed reports an Integer or Float argument of the other class than
// self (an Integer or a Float).
func rbNumMixed(self any, args []any) bool {
	_, selfInt := self.(Integer)
	for _, a := range args {
		switch a.(type) {
		case Integer:
			if !selfInt {
				return true
			}
		case Float:
			if selfInt {
				return true
			}
		}
	}
	return false
}

// rbNum is an Integer or a Float as one Go type: Comparable's methods run
// on it when given both, so clamp returns the winning argument itself.
type rbNum struct{ v any }

func (a rbNum) Op_cmp(b rbNum) Integer { return rbCmp(a.v, b.v) }

func (a rbNum) Op_lt(b rbNum) Boolean { return rbCmp(a.v, b.v) < 0 }

// rbConv is v as a T: v itself, nil for untyped, an Array or Hash of
// another instantiation converted (rbFrom), or an Array as a tuple
// (_FromAny, decision 22). Go instantiations are invariant, so
// Array[Integer] where Array[untyped] is expected, or back, is a copy
// whose elements are converted in turn. An Integer where a Float is
// expected widens (MRI's coerce: Float's operators take Integers).
// ponytail: a T? element (*T) is not converted from its value; box it via OptOf when needed.
func rbConv[T any](v any) (T, bool) {
	if t, ok := v.(T); ok {
		return t, true
	}
	if n, ok := v.(Integer); ok {
		if t, ok := any(Float(n)).(T); ok {
			return t, true
		}
	}
	var zero T
	if v == nil {
		_, untyped := any(&zero).(*any)
		return zero, untyped
	}
	if c, ok := any(zero).(interface{ rbFrom(v any) (T, bool) }); ok {
		return c.rbFrom(v)
	}
	if tup, ok := any(zero).(interface{ _FromAny(any) (T, bool) }); ok {
		return tup._FromAny(v)
	}
	return zero, false
}

func rbRest[T any](args []any, from int, want string) []T {
	out := make([]T, 0, len(args))
	for i := from; i < len(args); i++ {
		out = append(out, rbAs[T](args[i], want))
	}
	return out
}

// rbDescribe names a value the way MRI's messages do.
func rbDescribe(v any) string {
	switch r := v.(type) {
	case nil:
		return "nil"
	case Boolean:
		return strconv.FormatBool(bool(r))
	case rbModule:
		return r._Kind() + " " + string(r.Name())
	}
	return "an instance of " + rbClassName(v)
}

// rbNoMethod is MRI's error for a missing method; a bare `name` (a
// "vcall": no receiver, arguments or parentheses) is a NameError.
func rbNoMethod(name string, recv any, vcall bool) any {
	if vcall {
		e := NewNameError(Ref(String("undefined local variable or method '" + name + "' for " + rbDescribe(recv))))
		e.__SetCall(Symbol(name), recv)
		return e
	}
	e := NewNoMethodError(Ref(String("undefined method '" + name + "' for " + rbDescribe(recv))))
	e.__SetCall(Symbol(name), recv)
	return e
}

// rbPrivateMethod is MRI's error for a private method called with a receiver.
func rbPrivateMethod(name string, recv any) any {
	return NewNoMethodError(Ref(String("private method '" + name + "' called for " + rbDescribe(recv))))
}
