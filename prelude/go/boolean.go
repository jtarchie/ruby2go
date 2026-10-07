//go:build rb2go_prelude

// Package prelude is concatenated verbatim into the output (loadPreludeGo). `go vet -tags rb2go_prelude ./prelude/go` type-checks it against generated stubs for the types generated code declares (0_stubs.go, decision 154).
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
