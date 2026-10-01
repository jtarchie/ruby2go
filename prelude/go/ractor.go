//go:build ignore

// Package prelude is concatenated verbatim into the output (loadPreludeGo), never built for real: types like String come from generated code.
package prelude

// rbPort is Ractor::Port's queue. Every port shares one lock and condition so rbPortSelect can wait on several at once.
// ponytail: one condition wakes every parked receiver per send; a lock per port plus a one-slot channel per waiter (as rbCondVar) when many ractors idle on receive.
type rbPort struct {
	items  []any
	closed bool
}

var (
	rbPortMu     sync.Mutex
	rbPortCond   = sync.NewCond(&rbPortMu)
	rbRactorIDs  atomic.Int64 // main is #1
	rbRactorLive atomic.Int64 // started and not yet finished, for Ractor.count
	rbMainRactor = &Ractor{port: &Ractor_Port{}, done: make(chan struct{}), id: 1}
)

func rbPortClosed() any {
	return NewRactor_ClosedError(Ref(String("The port was already closed")))
}

func (p *rbPort) send(x any) {
	rbPortMu.Lock()
	defer rbPortMu.Unlock()
	if p.closed {
		panic(rbPortClosed())
	}
	p.items = append(p.items, x)
	rbPortCond.Broadcast()
}

// receive blocks until a message arrives; a closed, drained port raises ClosedError, as MRI's.
func (p *rbPort) receive() any {
	rbPortMu.Lock()
	defer rbPortMu.Unlock()
	for len(p.items) == 0 && !p.closed {
		rbPortCond.Wait()
	}
	return p.take()
}

// take pops under the lock; the port must have a message or be closed.
func (p *rbPort) take() any {
	if len(p.items) == 0 {
		panic(rbPortClosed())
	}
	x := p.items[0]
	p.items[0] = nil
	p.items = p.items[1:]
	return x
}

func (p *rbPort) close() {
	rbPortMu.Lock()
	defer rbPortMu.Unlock()
	p.closed = true
	rbPortCond.Broadcast()
}

func (p *rbPort) isClosed() bool {
	rbPortMu.Lock()
	defer rbPortMu.Unlock()
	return p.closed
}

// rbPortSelect answers the first port holding a message, as Ractor.select does; every port closed raises ClosedError.
func rbPortSelect(ports []*Ractor_Port) (*Ractor_Port, any) {
	if len(ports) == 0 {
		panic(NewArgumentError(Ref(String("specify at least one port"))))
	}
	rbPortMu.Lock()
	defer rbPortMu.Unlock()
	for {
		open := false
		for _, p := range ports {
			if len(p.q.items) > 0 {
				return p, p.q.take()
			}
			if !p.q.closed {
				open = true
			}
		}
		if !open {
			panic(rbPortClosed())
		}
		rbPortCond.Wait()
	}
}

func rbNewRactor() *Ractor {
	return &Ractor{port: &Ractor_Port{}, done: make(chan struct{}), id: int(rbRactorIDs.Add(1)) + 1}
}

// start runs the block on a goroutine. Its port closes when it ends, so later sends raise ClosedError; an exception is kept for value/join and reported on stderr like a thread's.
func (r *Ractor) start(run func() any) *Ractor {
	rbRactorLive.Add(1)
	go func() {
		defer close(r.done)
		defer r.port.q.close()
		defer rbRactorLive.Add(-1)
		defer func() {
			if p := recover(); p != nil {
				r.err = rbThreadAbort(p)
			}
		}()
		r.val = run()
	}()
	return r
}

// reraise, after done, raises RemoteError around the exception that ended the ractor.
func (r *Ractor) reraise() {
	if r.err == nil {
		return
	}
	if e, ok := r.err.(ExceptionI); ok {
		panic(NewRactor_RemoteError(r, e))
	}
	panic(r.err)
}

// rbCopier is what Ractor messages copy through: generated on every struct class (emitCopy), written here for Array and Hash and for the classes MRI refuses to copy.
type rbCopier interface{ _Copy(seen map[any]any) any }

// rbRactorCopy deep-copies a message or Ractor.new argument, as MRI does for a non-shareable object: frozen Arrays/Hashes, values, classes and ractors are shared; identity inside the graph is kept (seen).
func rbRactorCopy(v any) any { return rbCopyDeep(v, map[any]any{}) }

func rbCopyDeep(v any, seen map[any]any) any {
	c, ok := v.(rbCopier)
	if !ok || rbIsFrozen(v) {
		return v
	}
	if dup, ok := seen[v]; ok {
		return dup
	}
	return c._Copy(seen)
}

// rbCopyAs copies a typed value; a nil interface stays nil, since asserting it to an interface type would panic.
func rbCopyAs[T any](v T, seen map[any]any) T {
	a := any(v)
	if a == nil {
		return v
	}
	return rbCopyDeep(a, seen).(T)
}

func (self *Array[E]) _Copy(seen map[any]any) any {
	if self == nil {
		return self
	}
	out := make(Array[E], len(*self))
	seen[self] = &out
	for i, x := range *self {
		out[i] = rbCopyAs(x, seen)
	}
	return &out
}

func (self *Hash[K, V]) _Copy(seen map[any]any) any {
	if self == nil {
		return self
	}
	out := NewHash[K, V]()
	seen[self] = out
	for _, k := range self.keys {
		Hash_Op_idxSet(out, rbCopyAs(k, seen), rbCopyAs(self.vals[k], seen))
	}
	return out
}

// A Set copies only while every element is shareable, as MRI's does.
func (self *Set[E]) _Copy(seen map[any]any) any {
	if self == nil {
		return self
	}
	for _, k := range self.h.keys {
		if _, ok := any(k).(rbCopier); ok && !rbIsFrozen(k) {
			panic(NewRactor_Error(Ref(String("can not copy Set object."))))
		}
	}
	out := &Set[E]{h: NewHash[E, Boolean]()}
	seen[self] = out
	for _, k := range self.h.keys {
		Hash_Op_idxSet(out.h, k, self.h.vals[k])
	}
	return out
}

// What MRI cannot copy into a ractor, with its errors.

func (*Queue[E]) _Copy(map[any]any) any {
	panic(NewNoMethodError(Ref(String("undefined method 'initialize_copy' for an instance of Thread::Queue"))))
}

func (*SizedQueue[E]) _Copy(map[any]any) any {
	panic(NewNoMethodError(Ref(String("undefined method 'initialize_copy' for an instance of Thread::SizedQueue"))))
}

func (*Thread) _Copy(map[any]any) any {
	panic(NewTypeError(Ref(String("allocator undefined for Thread"))))
}
