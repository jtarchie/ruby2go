# rbs_inline: enabled

# @go_type struct { done chan struct{}; err any; val any; aborting atomic.Bool; name atomic.Pointer[String]; loc string }
class Thread < Object
  # The block's value is kept for #value, untyped: Thread is not generic.
  #: () { () -> untyped } -> Thread
  def self.new = %x{ return rbThreadRun(func() any { return blk() }) }

  # @rbs [A] (A) { (A) -> untyped } -> Thread
  def self.__new_1(a) = %x{ return rbThreadRun(func() any { return blk(a) }) }

  # @rbs [A, B] (A, B) { (A, B) -> untyped } -> Thread
  def self.__new_2(a, b) = %x{ return rbThreadRun(func() any { return blk(a, b) }) }

  # @rbs [A, B, C] (A, B, C) { (A, B, C) -> untyped } -> Thread
  def self.__new_3(a, b, c) = %x{ return rbThreadRun(func() any { return blk(a, b, c) }) }

  # The Thread this goroutine runs (decision 104); main for the main goroutine and a ractor's own, as MRI's per-ractor main thread.
  #: () -> Thread
  def self.current = %x{ return rbCurrentThread() }

  #: () -> Thread
  def self.main = %x{ return rbMainThread }

  # Joining yourself is MRI's ThreadError, where a bare channel read would wait forever.
  #: () -> self
  def join = %x{
    self.notSelf()
    <-self.done
    if self.err != nil {
      panic(self.err)
    }
    return self
  }

  # nil on timeout, self once the thread finishes within it.
  #: (Float) -> Thread?
  def __join_1(timeout) = %x{
    self.notSelf()
    select {
    case <-self.done:
      if self.err != nil {
        panic(self.err)
      }
      return &self
    case <-time.After(time.Duration(math.Round(float64(timeout) * 1e9))):
      return nil
    }
  }

  # Blocks and re-raises like join, then answers the block's value.
  #: () -> untyped
  def value = %x{
    self.notSelf()
    <-self.done
    if self.err != nil {
      panic(self.err)
    }
    return self.val
  }

  #: () -> String?
  def name = %x{
    p := self.name.Load()
    if p == nil {
      return nil
    }
    return p
  }

  #: (String) -> String
  def name=(n)
    %x{
    self.name.Store(&n)
    return n}
  end

  # MRI's `#<Thread:0x... file:line status>`: the call site that made it (main has none), and run/aborting/dead; no "sleep" (decision 45).
  #: () -> String
  def inspect = %x{
    status := "run"
    select {
    case <-self.done:
      status = "dead"
    default:
      if self.aborting.Load() {
        status = "aborting"
      }
    }
    if self.loc == "" {
      return String(fmt.Sprintf("#<Thread:%p %s>", self, status))
    }
    return String(fmt.Sprintf("#<Thread:%p %s %s>", self, self.loc, status))
  }

  # "run", "aborting", false (finished) or nil (unhandled exception); MRI's "sleep" is not reported (decision 45).
  #: () -> untyped
  def status = %x{
    select {
    case <-self.done:
      if self.err != nil {
        return nil
      }
      return Boolean(false)
    default:
    }
    if self.aborting.Load() {
      return String("aborting")
    }
    return String("run")
  }

  #: () -> bool
  def alive? = %x{
    select {
    case <-self.done:
      return false
    default:
      return true
    }
  }
end
