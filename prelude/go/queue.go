//go:build ignore

// Package prelude is concatenated verbatim into the output (loadPreludeGo), never built for real: types like String come from generated code.
package prelude

// rbQueue is Thread::Queue's core: max 0 is unbounded (Queue), else SizedQueue's bound.
type rbQueue[E any] struct {
	mu     sync.Mutex
	cond   *sync.Cond
	items  []E
	max    int
	closed bool
}

func newRbQueue[E any](max int) *rbQueue[E] {
	q := &rbQueue[E]{max: max}
	q.cond = sync.NewCond(&q.mu)
	return q
}

func (q *rbQueue[E]) push(x E) {
	q.mu.Lock()
	defer q.mu.Unlock()
	for q.max > 0 && len(q.items) >= q.max && !q.closed {
		q.cond.Wait()
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
	for len(q.items) == 0 && !q.closed {
		if nonBlock {
			panic(NewThreadError(Ref(String("queue empty"))))
		}
		q.cond.Wait()
	}
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
