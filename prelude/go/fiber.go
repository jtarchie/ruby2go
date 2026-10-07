//go:build rb2go_prelude

// Package prelude is concatenated verbatim into the output (loadPreludeGo). `go vet -tags rb2go_prelude ./prelude/go` type-checks it against generated stubs for the types generated code declares (0_stubs.go, decision 154).
package prelude

import (
	"sync"
)

type rbFiberState int

const (
	rbFiberCreated rbFiberState = iota
	rbFiberResumed
	rbFiberSuspended
	rbFiberDead
)

func (s rbFiberState) String() string {
	return [...]string{"created", "resumed", "suspended", "terminated"}[s]
}

// rbFiberMsg is what a fiber hands its resumer: a Fiber.yield's values, or the block's value or exception at the end.
type rbFiberMsg struct {
	val  any
	err  any
	done bool
}

var (
	rbFiberOf    sync.Map // goroutine id → the *Fiber it runs, for Fiber.current and Fiber.yield (decision 104)
	rbRootFibers sync.Map // *Thread → its root *Fiber
)

// rbCurrentFiber is the fiber this goroutine runs, else the running thread's root fiber.
func rbCurrentFiber() *Fiber {
	if f, ok := rbFiberOf.Load(rbGoID()); ok {
		return f.(*Fiber)
	}
	f, _ := rbRootFibers.LoadOrStore(rbCurrentThread(), &Fiber{state: rbFiberResumed})
	return f.(*Fiber)
}

// rbFiberPack is a value list as Ruby passes it between fibers: none is nil, one is itself, more an Array.
func rbFiberPack(vals []any) any {
	switch len(vals) {
	case 0:
		return nil
	case 1:
		return vals[0]
	}
	return &Array[any]{s: vals}
}

// resume hands control to f until it yields or ends; its exception re-raises here. The checks and messages are MRI's.
func (f *Fiber) resume(args []any) any {
	cur := rbCurrentFiber()
	switch {
	case f == cur:
		panic(NewFiberError(Ref(String("attempt to resume the current fiber"))))
	case f.state == rbFiberDead:
		panic(NewFiberError(Ref(String("attempt to resume a terminated fiber"))))
	case f.state == rbFiberResumed:
		panic(NewFiberError(Ref(String("attempt to resume a resuming fiber"))))
	}
	if f.state == rbFiberCreated {
		go f.run(rbGoID())
	}
	f.state = rbFiberResumed
	f.in <- rbFiberPack(args)
	m := <-f.out
	f.state = rbFiberSuspended
	if m.done {
		f.state = rbFiberDead
	}
	if m.err != nil {
		panic(m.err)
	}
	return m.val
}

// run is the fiber's goroutine: it belongs to the thread and ractor of the goroutine that first resumed it.
func (f *Fiber) run(parent int64) {
	id := rbGoID()
	rbFiberOf.Store(id, f)
	defer rbFiberOf.Delete(id)
	if t, ok := rbThreadOf.Load(parent); ok {
		rbThreadOf.Store(id, t)
		defer rbThreadOf.Delete(id)
	}
	if r, ok := rbRactorOf.Load(parent); ok {
		rbRactorOf.Store(id, r)
		defer rbRactorOf.Delete(id)
	}
	arg := <-f.in
	var m rbFiberMsg
	defer func() {
		if r := recover(); r != nil {
			m.err = rbWrapPanic(rbCaptureBacktrace(r))
		}
		m.done = true
		f.out <- m
	}()
	m.val = f.blk(arg)
}

// rbFiberYield is Fiber.yield: hand vals to the resumer and wait for the next resume's arguments.
func rbFiberYield(vals []any) any {
	v, ok := rbFiberOf.Load(rbGoID())
	if !ok {
		panic(NewFiberError(Ref(String("attempt to yield on a not resumed fiber"))))
	}
	f := v.(*Fiber)
	f.out <- rbFiberMsg{val: rbFiberPack(vals)}
	return <-f.in
}
