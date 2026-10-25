//go:build rugaroo_prelude

// Package prelude is concatenated verbatim into the output (loadPreludeGo). `go vet -tags rugaroo_prelude ./prelude/go` type-checks it against generated stubs for the types generated code declares (0_stubs.go, decision 154).
package prelude

import (
	"math"
	"os"
)

// Emitted code names these instead of math.Copysign, os.Getpid and the min builtin: a Ruby local called math, os or min would shadow the package or builtin inside its Go function.

// rbNegZero is Ruby's -0.0 literal, which Go's constant folding would make +0.
func rbNegZero() Float { return Float(math.Copysign(0, -1)) }

// rbPid is $$.
func rbPid() Integer { return Integer(os.Getpid()) }

// rbMinInt is the min builtin for the argument count of a callee-defaults call with a splat.
func rbMinInt(a, b int) int { return min(a, b) }
