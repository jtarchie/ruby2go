//go:build rugaroo_prelude

// Package prelude is concatenated verbatim into the output (loadPreludeGo). `go vet -tags rugaroo_prelude ./prelude/go` type-checks it against generated stubs for the types generated code declares (0_stubs.go, decision 154).
package prelude

import (
	"runtime"
	"slices"
	"sync"
	"sync/atomic"
	"time"
)

// rbQueue is Thread::Queue's core: max 0 is unbounded (Queue), else SizedQueue's bound; waiting counts goroutines currently blocked in push or pop, for num_waiting.
type rbQueue[E any] struct {
	mu      sync.Mutex
	cond    *sync.Cond
	items   []E
	max     int
	closed  bool
	waiting atomic.Int64
}

func newRbQueue[E any](max int) *rbQueue[E] {
	q := &rbQueue[E]{max: max}
	q.cond = sync.NewCond(&q.mu)
	return q
}

func (q *rbQueue[E]) push(x E) {
	q.mu.Lock()
	defer q.mu.Unlock()
	waiting := false
	for q.max > 0 && len(q.items) >= q.max && !q.closed {
		if !waiting {
			q.waiting.Add(1)
			waiting = true
		}
		q.cond.Wait()
	}
	if waiting {
		q.waiting.Add(-1)
	}
	if q.closed {
		panic(NewClosedQueueError(Ref(String("queue closed"))))
	}
	q.items = append(q.items, x)
	q.cond.Broadcast()
}

// pop blocks until an item arrives; nil once the queue is closed and empty (or empty with nonBlock, which raises in MRI).
func (q *rbQueue[E]) pop(nonBlock bool) *E {
	q.mu.Lock()
	defer q.mu.Unlock()
	waiting, main := false, false
	for len(q.items) == 0 && !q.closed {
		if nonBlock {
			panic(NewThreadError(Ref(String("queue empty"))))
		}
		if !waiting {
			q.waiting.Add(1)
			waiting = true
			if main = rbGoID() == rbMainGoID; main { // Ctrl-C wakes this wait (decision 60)
				rbInterruptWake.Store(q, func() {
					q.mu.Lock()
					q.cond.Broadcast()
					q.mu.Unlock()
				})
				defer rbInterruptWake.Delete(q)
			}
		}
		q.cond.Wait()
		if main && rbInterruptPending() {
			q.waiting.Add(-1)
			rbTakeInterrupt() // raises; the deferred unlock runs
			q.waiting.Add(1)
		}
	}
	if waiting {
		q.waiting.Add(-1)
	}
	return q.take()
}

// popDeadline is pop with a wall-clock deadline: nil if it passes before an item arrives or the queue closes.
func (q *rbQueue[E]) popDeadline(deadline time.Time) *E {
	timer := time.AfterFunc(time.Until(deadline), func() {
		q.mu.Lock()
		q.cond.Broadcast()
		q.mu.Unlock()
	})
	defer timer.Stop()
	q.mu.Lock()
	defer q.mu.Unlock()
	waiting := false
	for len(q.items) == 0 && !q.closed && time.Now().Before(deadline) {
		if !waiting {
			q.waiting.Add(1)
			waiting = true
		}
		q.cond.Wait()
	}
	if waiting {
		q.waiting.Add(-1)
	}
	return q.take()
}

// take removes and returns the front item, once the caller has confirmed the queue is non-empty or closed.
func (q *rbQueue[E]) take() *E {
	if len(q.items) == 0 {
		return nil
	}
	x := q.items[0]
	var zero E
	q.items[0] = zero
	q.items = q.items[1:]
	q.cond.Broadcast()
	return &x
}

func (q *rbQueue[E]) size() Integer {
	q.mu.Lock()
	defer q.mu.Unlock()
	return Integer(len(q.items))
}

func (q *rbQueue[E]) close() {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.closed = true
	q.cond.Broadcast()
}

func (q *rbQueue[E]) isClosed() bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.closed
}

func (q *rbQueue[E]) clear() {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.items = nil
	q.cond.Broadcast()
}

func (q *rbQueue[E]) setMax(max int) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.max = max
	q.cond.Broadcast()
}

func NewQueue[E comparable]() *Queue[E] { return &Queue[E]{q: newRbQueue[E](0)} }

// rbCondVar is ConditionVariable: each waiter parks on its own channel.
type rbCondVar struct {
	mu      sync.Mutex
	waiters []chan struct{}
}

func (c *rbCondVar) wait(m *Mutex) {
	ch := make(chan struct{})
	c.mu.Lock()
	c.waiters = append(c.waiters, ch)
	c.mu.Unlock()
	m.Unlock()
	select {
	case <-ch:
	case <-rbInterruptC(): // Ctrl-C (decision 60): the main goroutine re-locks and raises
		if rbGoID() == rbMainGoID {
			c.mu.Lock()
			if i := slices.Index(c.waiters, ch); i >= 0 {
				c.waiters = slices.Delete(c.waiters, i, i+1)
			}
			c.mu.Unlock()
			m.Lock()
			rbTakeInterrupt()
			m.Unlock()
		}
		<-ch
	}
	m.Lock()
}

// waitTimeout is wait with a deadline: false if it passes before signal/broadcast wakes this waiter.
func (c *rbCondVar) waitTimeout(m *Mutex, d time.Duration) bool {
	ch := make(chan struct{})
	c.mu.Lock()
	c.waiters = append(c.waiters, ch)
	c.mu.Unlock()
	m.Unlock()
	defer m.Lock()
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ch:
		return true
	case <-timer.C:
		c.mu.Lock()
		for i, w := range c.waiters {
			if w == ch {
				c.waiters = append(c.waiters[:i], c.waiters[i+1:]...)
				break
			}
		}
		c.mu.Unlock()
		select {
		case <-ch: // signalled right at the boundary, after we stopped tracking it
			return true
		default:
			return false
		}
	}
}

func (c *rbCondVar) wake(all bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for len(c.waiters) > 0 {
		close(c.waiters[0])
		c.waiters = c.waiters[1:]
		if !all {
			return
		}
	}
}

type Queue_Any interface{ _ToAny() *Queue[any] }

type SizedQueue_Any interface{ _ToAny() *SizedQueue[any] }

// rbBounded is SizedQueue: a bounded Go channel, so a push or pop is one channel operation with no lock to contend on
// (decision 403). Resizing (max=) swaps in a new channel: it raises resizing, so no push starts on either channel,
// waits out the pushes already past that check, moves the old channel's items over (poppers may still take from
// it meanwhile), then publishes the new one; whoever was blocked on the old one wakes through its gen and waits for
// the resize to finish. So no item is lost or reordered. close never closes the channel (a racing push would
// panic): done wakes the blocked instead.
type rbBounded[E any] struct {
	state    atomic.Pointer[rbBoundedState[E]]
	mu       sync.Mutex // max=, close and clear, one at a time
	closed   atomic.Bool
	done     chan struct{} // closed by close: a blocked push raises ClosedQueueError, a blocked pop drains then answers nil
	inflight atomic.Int64  // pushes between checking resizing and finishing their send, which setMax waits out
	resizing atomic.Bool   // set by setMax while it moves items: a push waits for it instead of sending
	waiting  atomic.Int64  // goroutines blocked in push or pop, for num_waiting
	max      atomic.Int64
}

type rbBoundedState[E any] struct {
	ch  chan E
	gen chan struct{} // closed when this state is replaced
}

func newRbBounded[E any](max int) *rbBounded[E] {
	q := &rbBounded[E]{done: make(chan struct{})}
	q.max.Store(int64(max))
	q.state.Store(&rbBoundedState[E]{ch: make(chan E, max), gen: make(chan struct{})})
	return q
}

func (q *rbBounded[E]) push(x E) {
	for {
		if q.closed.Load() {
			panic(NewClosedQueueError(Ref(String("queue closed"))))
		}
		q.inflight.Add(1)
		if q.resizing.Load() { // both are sequentially consistent: either setMax sees this push in flight or it sees resizing
			q.inflight.Add(-1)
			q.waitResize()
			continue
		}
		st := q.state.Load()
		select {
		case st.ch <- x:
			q.inflight.Add(-1)
			return
		default:
		}
		q.waiting.Add(1)
		select {
		case st.ch <- x:
			q.waiting.Add(-1)
			q.inflight.Add(-1)
			return
		case <-st.gen: // resized: retry on the new channel
			q.waiting.Add(-1)
			q.inflight.Add(-1)
			q.waitResize()
		case <-q.done:
			q.waiting.Add(-1)
			q.inflight.Add(-1)
			panic(NewClosedQueueError(Ref(String("queue closed"))))
		}
	}
}

// pop blocks until an item arrives; nil once the queue is closed and empty (or empty with nonBlock, which raises in MRI).
func (q *rbBounded[E]) pop(nonBlock bool) *E {
	return q.popUntil(nonBlock, nil)
}

// popDeadline is pop with a wall-clock deadline: nil if it passes before an item arrives or the queue closes.
func (q *rbBounded[E]) popDeadline(deadline time.Time) *E {
	timer := time.NewTimer(time.Until(deadline))
	defer timer.Stop()
	return q.popUntil(false, timer.C)
}

func (q *rbBounded[E]) popUntil(nonBlock bool, timeout <-chan time.Time) *E {
	for {
		st := q.state.Load()
		select {
		case x := <-st.ch:
			return &x
		default:
		}
		if nonBlock {
			panic(NewThreadError(Ref(String("queue empty"))))
		}
		var interrupt <-chan struct{} // Ctrl-C wakes main's wait (decision 60); a nil channel never fires
		if rbGoID() == rbMainGoID {
			interrupt = rbInterruptC()
		}
		q.waiting.Add(1)
		select {
		case x := <-st.ch:
			q.waiting.Add(-1)
			return &x
		case <-st.gen: // resized: retry on the new channel
			q.waiting.Add(-1)
			q.waitResize()
		case <-q.done:
			q.waiting.Add(-1)
			select {
			case x := <-st.ch:
				return &x
			default:
				return nil
			}
		case <-timeout:
			q.waiting.Add(-1)
			return nil
		case <-interrupt:
			q.waiting.Add(-1)
			rbTakeInterrupt() // raises on main
		}
	}
}

// waitResize returns once no resize is running: setMax holds mu throughout.
func (q *rbBounded[E]) waitResize() {
	q.mu.Lock()
	q.mu.Unlock() //nolint:staticcheck // an empty critical section: waiting for the holder is the point
}

func (q *rbBounded[E]) size() Integer { return Integer(len(q.state.Load().ch)) }

func (q *rbBounded[E]) close() {
	q.mu.Lock()
	defer q.mu.Unlock()
	if !q.closed.Swap(true) {
		close(q.done)
	}
}

func (q *rbBounded[E]) isClosed() bool { return q.closed.Load() }

func (q *rbBounded[E]) clear() {
	q.mu.Lock()
	defer q.mu.Unlock()
	ch := q.state.Load().ch
	for {
		select {
		case <-ch:
		default:
			return
		}
	}
}

// setMax replaces the channel with one of the new capacity (or the current length, if more is held: those items
// stay, and pushes block until pops bring the queue under max), wakes what is blocked on the old one, and moves
// the old items over once every push that read the old state has finished with it.
func (q *rbBounded[E]) setMax(n int) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.resizing.Store(true)
	defer q.resizing.Store(false)
	old := q.state.Load()
	close(old.gen) // blocked pushers and poppers let go of the old channel
	for q.inflight.Load() != 0 {
		runtime.Gosched()
	}
	q.max.Store(int64(n))
	st := &rbBoundedState[E]{ch: make(chan E, max(n, len(old.ch))), gen: make(chan struct{})}
	for moving := true; moving; {
		select {
		case x := <-old.ch:
			st.ch <- x
		default:
			moving = false
		}
	}
	q.state.Store(st)
}
