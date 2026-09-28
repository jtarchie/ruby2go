//go:build ignore

// Package prelude is concatenated verbatim into the output (loadPreludeGo), never built for real: types like String come from generated code.
package prelude

func rbBoolClass(b Boolean) ClassI {
	if b {
		return TrueClass_class
	}
	return FalseClass_class
}

// rbIsBool is is_a?(TrueClass/FalseClass) on an untyped value.
func rbIsBool(a any, want bool) bool {
	b, ok := rbUnbox(a).(Boolean)
	return ok && bool(b) == want
}

// rbIsProc is is_a?(Proc) on an untyped value: procs are pointers to Go funcs.
func rbIsProc(a any) bool {
	t := reflect.TypeOf(a)
	return t != nil && t.Kind() == reflect.Pointer && t.Elem().Kind() == reflect.Func
}
