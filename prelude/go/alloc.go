//go:build rugaroo_prelude

// Package prelude is concatenated verbatim into the output (loadPreludeGo). `go vet -tags rugaroo_prelude ./prelude/go` type-checks it against generated stubs for the types generated code declares (0_stubs.go, decision 154).
package prelude

import "fmt"

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

// rbAryMax is MRI's ARY_MAX_SIZE (LONG_MAX / sizeof(VALUE)): an index at or past it is an IndexError there.
const rbAryMax = 1<<60 - 1

// rbCheckPad guards padding an Array with nils out to index n (a far `a[n] = v`, `insert`, `a[n, l] = v`): MRI's IndexError "index N too big" at rbAryMax, else rbCheckAlloc's test for n+1 elements.
func rbCheckPad(n int) {
	if n >= rbAryMax {
		panic(NewIndexError(Ref(String(fmt.Sprintf("index %d too big", n)))))
	}
	rbCheckAlloc(n+1, 16) // an untyped slot (the widest common element)
}
