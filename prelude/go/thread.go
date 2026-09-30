//go:build ignore

// Package prelude is concatenated verbatim into the output (loadPreludeGo), never built for real: types like String come from generated code.
package prelude

// rbThreadRun starts run on a goroutine and returns its Thread handle: a panic re-raises on join (or, for SystemExit, exits like MRI's main thread would).
func rbThreadRun(run func() any) *Thread {
	t := &Thread{done: make(chan struct{})}
	go func() {
		defer close(t.done)
		defer func() {
			if r := recover(); r != nil {
				if e, ok := r.(SystemExitI); ok {
					// ponytail: MRI re-raises exit in the main thread, running its ensures; this exits here.
					rbFinish(int(e.Status()), nil)
					os.Exit(0)
				}
				t.aborting.Store(true)
				t.err = rbWrapPanic(r)
				fmt.Fprintln(os.Stderr, "#<Thread> terminated with exception (report_on_exception is true):", rbToS(t.err), "("+rbClassName(t.err)+")")
			}
		}()
		t.val = run()
	}()
	return t
}
