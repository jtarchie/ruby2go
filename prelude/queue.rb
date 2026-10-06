# rbs_inline: enabled

class ThreadError < StandardError; end

class ClosedQueueError < StopIteration; end

# sync.Mutex plus the owning goroutine's id (decision 104), for owned?, MRI's recursive-locking error and the non-owner unlock error.
# ponytail: a dead thread's mutexes stay locked (MRI releases them); the upgrade is a per-Thread set of held mutexes, unlocked in rbThreadRun's exit defer.
# @go_type struct { mu sync.Mutex; owner atomic.Int64 }
class Mutex < Object
  #: () -> Mutex
  def self.new = %x{ return &Mutex{} }

  #: () -> self
  def lock = %x{
    id := rbGoID()
    if self.owner.Load() == id {
      panic(NewThreadError(Ref(String("deadlock; recursive locking"))))
    }
    self.mu.Lock()
    self.owner.Store(id)
    return self
  }

  #: () -> self
  def unlock = %x{
    switch owner := self.owner.Load(); {
    case owner == 0:
      panic(NewThreadError(Ref(String("Attempt to unlock a mutex which is not locked"))))
    case owner != rbGoID():
      panic(NewThreadError(Ref(String("Attempt to unlock a mutex which is locked by another thread/fiber"))))
    }
    self.owner.Store(0)
    self.mu.Unlock()
    return self
  }

  #: () -> bool
  def try_lock = %x{
    if !self.mu.TryLock() {
      return false
    }
    self.owner.Store(rbGoID())
    return true
  }

  #: () -> bool
  def locked? = %x{ Boolean(self.owner.Load() != 0) }

  #: () -> bool
  def owned? = %x{ Boolean(self.owner.Load() == rbGoID()) }

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

  # 0 once signalled, nil on timeout, as MRI's.
  #: (Mutex, Float) -> Integer?
  def __wait_2(m, timeout) = %x{
    if self.cv.waitTimeout(m, time.Duration(math.Round(float64(timeout)*1e9))) {
      return Ref(Integer(0))
    }
    return nil
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

  # @rbs [X] (Array[X]) -> Queue[X]
  def self.new(items) = %x{
    q := &Queue[X]{q: newRbQueue[X](0)}
    for _, x := range items.s {
      q.q.push(x)
    }
    return q
  }

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

  # `timeout:` from a Hash (decision 23), since keyword params are not supported.
  #: (Hash[Symbol, Float]) -> E?
  def __pop_hash(opts) = %x{
    if t := Hash_Op_idx(opts, Symbol("timeout")); t != nil {
      return self.q.popDeadline(time.Now().Add(time.Duration(math.Round(float64(*t) * 1e9))))
    }
    return self.q.pop(false)
  }

  #: (?bool) -> E?
  def shift(non_block = false) = pop(non_block)

  #: (Hash[Symbol, Float]) -> E?
  def __shift_hash(opts) = __pop_hash(opts)

  #: (?bool) -> E?
  def deq(non_block = false) = pop(non_block)

  #: (Hash[Symbol, Float]) -> E?
  def __deq_hash(opts) = __pop_hash(opts)

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
  def num_waiting = %x{ Integer(self.q.waiting.Load()) }

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

  #: (Integer) -> Integer
  def max=(m)
    %x{
    if m <= 0 {
      panic(NewArgumentError(Ref(String("queue size must be positive"))))
    }
    self.q.setMax(int(m))
    return m}
  end

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

  # `timeout:` from a Hash (decision 23), since keyword params are not supported.
  #: (Hash[Symbol, Float]) -> E?
  def __pop_hash(opts) = %x{
    if t := Hash_Op_idx(opts, Symbol("timeout")); t != nil {
      return self.q.popDeadline(time.Now().Add(time.Duration(math.Round(float64(*t) * 1e9))))
    }
    return self.q.pop(false)
  }

  #: (?bool) -> E?
  def shift(non_block = false) = pop(non_block)

  #: (Hash[Symbol, Float]) -> E?
  def __shift_hash(opts) = __pop_hash(opts)

  #: (?bool) -> E?
  def deq(non_block = false) = pop(non_block)

  #: (Hash[Symbol, Float]) -> E?
  def __deq_hash(opts) = __pop_hash(opts)

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
  def num_waiting = %x{ Integer(self.q.waiting.Load()) }

  #: () -> SizedQueue[untyped]
  def _to_any = %x{
    if same, ok := any(self).(*SizedQueue[any]); ok {
      return same
    }
    panic(NewTypeError(Ref(String("rb2go: a typed SizedQueue cannot be viewed as untyped"))))
  }
end
