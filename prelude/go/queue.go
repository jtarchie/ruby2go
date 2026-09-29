//go:build ignore

// Package prelude is concatenated verbatim into the output (loadPreludeGo), never built for real: types like String come from generated code.
package prelude

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
	waiting := false
	for len(q.items) == 0 && !q.closed {
		if nonBlock {
			panic(NewThreadError(Ref(String("queue empty"))))
		}
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
	<-ch
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
