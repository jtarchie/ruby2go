//go:build rb2go_prelude

// Package prelude is concatenated verbatim into the output (loadPreludeGo). `go vet -tags rb2go_prelude ./prelude/go` type-checks it against generated stubs for the types generated code declares (0_stubs.go, decision 154).
package prelude

// rbMathDomain raises MRI's Math::DomainError for an out-of-domain argument.
func rbMathDomain(bad Boolean, name string) {
	if bad {
		panic(NewMath_DomainError(Ref(String(`Numerical argument is out of domain - ` + name))))
	}
}
