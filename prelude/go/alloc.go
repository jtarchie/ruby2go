//go:build rb2go_prelude

// Package prelude is concatenated verbatim into the output (loadPreludeGo). `go vet -tags rb2go_prelude ./prelude/go` type-checks it against generated stubs for the types generated code declares (0_stubs.go, decision 154).
package prelude

// rbAllocMax is the largest request a user-controlled size may make: a Go out-of-memory is fatal and `rescue` cannot catch it, so past this MRI's NoMemoryError or ArgumentError is raised instead (MRI could still satisfy a request between this and its own limit; accepted).
const rbAllocMax = 4 << 30

// rbAllocTooBig is whether n elements of elemSize bytes pass rbAllocMax (a negative n, from an overflowed count, does).
func rbAllocTooBig(n, elemSize int) bool {
	return n < 0 || elemSize > 0 && n > rbAllocMax/elemSize
}

// rbCheckAlloc raises MRI's NoMemoryError for a request of n elements of elemSize bytes past rbAllocMax; a call site whose MRI raises an ArgumentError tests rbAllocTooBig itself.
func rbCheckAlloc(n, elemSize int) {
	if rbAllocTooBig(n, elemSize) {
		panic(NewNoMemoryError(Ref(String("failed to allocate memory"))))
	}
}
