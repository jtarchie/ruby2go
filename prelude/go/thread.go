//go:build ignore

// Package prelude is concatenated verbatim into the output (loadPreludeGo), never built for real: types like String come from generated code.
package prelude

var (
	rbThreadOf   sync.Map // goroutine id → *Thread, for Thread.current (decision 104)
	rbMainThread = &Thread{done: make(chan struct{})}
)

// rbGoID is the running goroutine's id, parsed from runtime.Stack's "goroutine N [" header: Go exposes no other identity (decision 104). About a microsecond; only blocking or locking paths pay it.
func rbGoID() int64 {
	var buf [64]byte
	n := runtime.Stack(buf[:], false)
	var id int64
	for _, c := range buf[len("goroutine "):n] {
		if c < '0' || c > '9' {
			break
		}
		id = id*10 + int64(c-'0')
	}
	return id
}

// rbCurrentThread is Thread.current: the Thread this goroutine runs, else main (a ractor's own goroutine is its main thread, as in MRI).
func rbCurrentThread() *Thread {
	if t, ok := rbThreadOf.Load(rbGoID()); ok {
		return t.(*Thread)
	}
	return rbMainThread
}

// notSelf is join/value's guard: MRI raises rather than deadlock when a thread joins itself.
func (t *Thread) notSelf() {
	if t == rbCurrentThread() {
		panic(NewThreadError(Ref(String("Target thread must not be current thread"))))
	}
}

// rbThreadRun starts run on a goroutine and returns its Thread handle: a panic re-raises on join (or, for SystemExit, exits like MRI's main thread would). The thread belongs to the ractor that started it, so Ractor.receive inside it reads that ractor's port.
func rbThreadRun(run func() any) *Thread {
	t := &Thread{done: make(chan struct{}), loc: rbCallerLoc()}
	ractor, inRactor := rbRactorOf.Load(rbGoID())
	rbLiveThreads.Store(t, rbThreadSeq.Add(1))
	go func() {
		id := rbGoID()
		rbThreadOf.Store(id, t)
		defer rbThreadOf.Delete(id)
		if inRactor {
			rbRactorOf.Store(id, ractor)
			defer rbRactorOf.Delete(id)
		}
		defer close(t.done)
		defer rbLiveThreads.Delete(t) // before done closes: a joined thread is off the list
		defer func() {
			if r := recover(); r != nil {
				t.aborting.Store(true)
				t.err = rbThreadAbort(r)
			}
		}()
		t.val = run()
	}()
	return t
}

// rbThreadAbort ends a thread's or ractor's goroutine on a panic: SystemExit exits the program, anything else is reported on stderr as MRI does and returned wrapped, for join to re-raise.
func rbThreadAbort(r any) any {
	if e, ok := r.(SystemExitI); ok {
		// ponytail: MRI re-raises exit in the main thread, running its ensures; this exits here.
		rbFinish(int(e.Status()), nil)
		os.Exit(0)
	}
	err := rbWrapPanic(rbCaptureBacktrace(r)) // join/value re-raise it on another goroutine: these are its frames
	fmt.Fprintln(os.Stderr, "#<Thread> terminated with exception (report_on_exception is true):", rbToS(err), "("+rbClassName(err)+")")
	return err
}

// rbThreadKey is a thread-local's key: a Symbol or String, as MRI takes either (decision 108).
func rbThreadKey(key any) string {
	switch k := rbUnbox(key).(type) {
	case Symbol:
		return string(k)
	case String:
		return string(k)
	}
	panic(NewTypeError(Ref(String(string(rbInspect(key)) + " is not a symbol nor a string"))))
}

// rbThreadKeys lists a local map's keys as Symbols, sorted (sync.Map has no order).
func rbThreadKeys(m *sync.Map) *Array[Symbol] {
	out := &Array[Symbol]{}
	m.Range(func(k, _ any) bool {
		*out = append(*out, Symbol(k.(string)))
		return true
	})
	slices.Sort(*out)
	return out
}

// rbLiveThreads are the threads not yet finished, each with its creation number, for Thread.list.
var (
	rbLiveThreads sync.Map
	rbThreadSeq   atomic.Int64
)

// rbThreadList is Thread.list: the main thread, then the live threads in creation order.
func rbThreadList() *Array[*Thread] {
	type entry struct {
		t   *Thread
		seq int64
	}
	var live []entry
	rbLiveThreads.Range(func(k, v any) bool {
		live = append(live, entry{k.(*Thread), v.(int64)})
		return true
	})
	slices.SortFunc(live, func(a, b entry) int { return cmp.Compare(a.seq, b.seq) })
	out := &Array[*Thread]{rbMainThread}
	for _, e := range live {
		*out = append(*out, e.t)
	}
	return out
}
