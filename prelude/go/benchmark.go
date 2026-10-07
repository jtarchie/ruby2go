//go:build rb2go_prelude

// Package prelude is concatenated verbatim into the output (loadPreludeGo). `go vet -tags rb2go_prelude ./prelude/go` type-checks it against generated stubs for the types generated code declares (0_stubs.go, decision 154).
package prelude

import (
	"syscall"
)

// rbBenchJobItem is a Benchmark::Job entry: a label and its deferred block, run twice by Benchmark.bmbm.
type rbBenchJobItem struct {
	label string
	fn    func()
}

// rbBenchRusage reads getrusage(2) CPU time for the process and its children, in seconds.
func rbBenchRusage() (utime, stime, cutime, cstime float64) {
	var self, children syscall.Rusage

	_ = syscall.Getrusage(syscall.RUSAGE_SELF, &self)
	_ = syscall.Getrusage(syscall.RUSAGE_CHILDREN, &children)

	return rbTimevalSeconds(self.Utime), rbTimevalSeconds(self.Stime), rbTimevalSeconds(children.Utime), rbTimevalSeconds(children.Stime)
}

// rbTimevalSeconds converts a syscall.Timeval to seconds; Usec's width differs by platform, so it is cast rather than read directly.
func rbTimevalSeconds(tv syscall.Timeval) float64 {
	return float64(tv.Sec) + float64(tv.Usec)/1e6
}
