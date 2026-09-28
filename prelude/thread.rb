# rbs_inline: enabled

# @go_type struct { done chan struct{}; err any; aborting atomic.Bool; name atomic.Pointer[String] }
class Thread < Object
  #: () { () -> void } -> Thread
  def self.new = %x{ return rbThreadRun(func() { blk() }) }

  # @rbs [A] (A) { (A) -> void } -> Thread
  def self.__new_1(a) = %x{ return rbThreadRun(func() { blk(a) }) }

  # @rbs [A, B] (A, B) { (A, B) -> void } -> Thread
  def self.__new_2(a, b) = %x{ return rbThreadRun(func() { blk(a, b) }) }

  # @rbs [A, B, C] (A, B, C) { (A, B, C) -> void } -> Thread
  def self.__new_3(a, b, c) = %x{ return rbThreadRun(func() { blk(a, b, c) }) }

  #: () -> self
  def join = %x{
    <-self.done
    if self.err != nil {
      panic(self.err)
    }
    return self
  }

  # nil on timeout, self once the thread finishes within it.
  #: (Float) -> Thread?
  def __join_1(timeout) = %x{
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

  # Blocks and re-raises like join; the block's return value is not captured (it is void, decision 45), so this is always nil.
  #: () -> untyped
  def value = %x{
    <-self.done
    if self.err != nil {
      panic(self.err)
    }
    return nil
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
