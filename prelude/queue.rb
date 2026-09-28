# rbs_inline: enabled

class ThreadError < StandardError; end

class ClosedQueueError < StopIteration; end

# sync.Mutex plus a flag for locked?; unlike MRI, relocking from the owning thread deadlocks instead of raising.
# @go_type struct { mu sync.Mutex; held atomic.Bool }
class Mutex < Object
  #: () -> Mutex
  def self.new = %x{ return &Mutex{} }

  #: () -> self
  def lock = %x{
    self.mu.Lock()
    self.held.Store(true)
    return self
  }

  #: () -> self
  def unlock = %x{
    if !self.held.Load() {
      panic(NewThreadError(Ref(String("Attempt to unlock a mutex which is not locked"))))
    }
    self.held.Store(false)
    self.mu.Unlock()
    return self
  }

  #: () -> bool
  def try_lock = %x{
    if !self.mu.TryLock() {
      return false
    }
    self.held.Store(true)
    return true
  }

  #: () -> bool
  def locked? = %x{ Boolean(self.held.Load()) }

  # @rbs [X] () { () -> X } -> X
  def synchronize
    lock
    begin
      yield
    ensure
      unlock
    end
  end
end

# @go_type struct { cv rbCondVar }
class ConditionVariable < Object
  #: () -> ConditionVariable
  def self.new = %x{ return &ConditionVariable{} }

  #: (Mutex) -> self
  def wait(m) = %x{
    self.cv.wait(m)
    return self
  }

  #: () -> self
  def signal = %x{
    self.cv.wake(false)
    return self
  }

  #: () -> self
  def broadcast = %x{
    self.cv.wake(true)
    return self
  }
end

# @rbs generic E
# @go_type struct { q *rbQueue[E] }
class Queue < Object

  #: (E) -> self
  def push(x) = %x{
    self.q.push(x)
    return self
  }

  #: (E) -> self
  def <<(x) = push(x)

  #: (E) -> self
  def enq(x) = push(x)

  # nil once closed and drained, as MRI.
  #: (?bool) -> E?
  def pop(non_block = false) = %x{ return self.q.pop(bool(non_block)) }

  #: (?bool) -> E?
  def shift(non_block = false) = pop(non_block)

  #: (?bool) -> E?
  def deq(non_block = false) = pop(non_block)

  #: () -> Integer
  def size = %x{ self.q.size() }

  #: () -> Integer
  def length = size

  #: () -> bool
  def empty? = size == 0

  #: () -> self
  def close = %x{
    self.q.close()
    return self
  }

  #: () -> bool
  def closed? = %x{ Boolean(self.q.isClosed()) }

  #: () -> self
  def clear = %x{
    self.q.clear()
    return self
  }

  #: () -> Integer
  def num_waiting = 0

  #: () -> Queue[untyped]
  def _to_any = %x{
    if same, ok := any(self).(*Queue[any]); ok {
      return same
    }
    panic(NewTypeError(Ref(String("rb2go: a typed Queue cannot be viewed as untyped"))))
  }
end

# @rbs generic E
# @go_type struct { q *rbQueue[E] }
class SizedQueue < Object
  #: [X] (Integer) -> SizedQueue[X]
  def self.new(max) = %x{
    if max <= 0 {
      panic(NewArgumentError(Ref(String("queue size must be positive"))))
    }
    return &SizedQueue[X]{q: newRbQueue[X](int(max))}
  }

  #: () -> Integer
  def max = %x{ Integer(self.q.max) }

  #: (E) -> self
  def push(x) = %x{
    self.q.push(x)
    return self
  }

  #: (E) -> self
  def <<(x) = push(x)

  #: (E) -> self
  def enq(x) = push(x)

  # nil once closed and drained, as MRI.
  #: (?bool) -> E?
  def pop(non_block = false) = %x{ return self.q.pop(bool(non_block)) }

  #: (?bool) -> E?
  def shift(non_block = false) = pop(non_block)

  #: (?bool) -> E?
  def deq(non_block = false) = pop(non_block)

  #: () -> Integer
  def size = %x{ self.q.size() }

  #: () -> Integer
  def length = size

  #: () -> bool
  def empty? = size == 0

  #: () -> self
  def close = %x{
    self.q.close()
    return self
  }

  #: () -> bool
  def closed? = %x{ Boolean(self.q.isClosed()) }

  #: () -> self
  def clear = %x{
    self.q.clear()
    return self
  }

  #: () -> Integer
  def num_waiting = 0

  #: () -> SizedQueue[untyped]
  def _to_any = %x{
    if same, ok := any(self).(*SizedQueue[any]); ok {
      return same
    }
    panic(NewTypeError(Ref(String("rb2go: a typed SizedQueue cannot be viewed as untyped"))))
  }
end
