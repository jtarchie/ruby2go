# rbs_inline: enabled

# Monitor (decision 108): a Mutex the owning thread may enter again, and
# MonitorMixin, which gives any object one; new_cond makes a condition
# variable that releases the monitor however deep it is entered.
# @go_type struct { m *Mutex; count int }
class Monitor < Object
  #: () -> Monitor
  def self.new = %x{ return &Monitor{m: &Mutex{}} }

  #: () -> nil
  def enter = %x{
    if self.m.owner.Load() == rbGoID() {
      self.count++
      return
    }
    self.m.Lock()
    self.count = 1
    return
  }

  #: () -> nil
  def exit = %x{
    rbMonitorCheckOwner(self)
    self.count--
    if self.count == 0 {
      self.m.Unlock()
    }
    return
  }

  #: () -> bool
  def try_enter = %x{
    if self.m.owner.Load() == rbGoID() {
      self.count++
      return true
    }
    if !self.m.TryLock() {
      return false
    }
    self.count = 1
    return true
  }

  # @rbs [T] () { () -> T } -> T
  def synchronize
    enter
    begin
      yield
    ensure
      exit
    end
  end

  #: () -> nil
  def mon_enter = enter

  #: () -> nil
  def mon_exit = exit

  #: () -> bool
  def mon_try_enter = try_enter

  #: () -> bool
  def try_mon_enter = try_enter

  # @rbs [T] () { () -> T } -> T
  def mon_synchronize
    enter
    begin
      yield
    ensure
      exit
    end
  end

  #: () -> bool
  def mon_locked? = m.locked?

  #: () -> bool
  def mon_owned? = m.owned?

  #: () -> nil
  def mon_check_owner = %x{
    rbMonitorCheckOwner(self)
    return
  }

  #: () -> MonitorMixin::ConditionVariable
  def new_cond = MonitorMixin::ConditionVariable.__new(self)

  # Releases the monitor however deep it is entered, waits, and takes it back as deep; true, or false on a timeout.
  #: (MonitorMixin::ConditionVariable, untyped) -> bool
  def wait_for_cond(cond, timeout) = %x{
    rbMonitorCheckOwner(self)
    count := self.count
    self.count = 0
    ok := true
    if timeout == nil {
      cond.cv.wait(self.m)
    } else {
      ok = cond.cv.waitTimeout(self.m, time.Duration(math.Round(rbNumFloat(timeout)*1e9)))
    }
    self.count = count
    return Boolean(ok)
  }

  #: () -> Mutex
  def m = %x{ return self.m }
end

module MonitorMixin
  # @go_type struct { mon *Monitor; cv rbCondVar }
  class ConditionVariable < Object
    #: (Monitor) -> ConditionVariable
    def self.__new(mon) = %x{ return &MonitorMixin_ConditionVariable{mon: mon} }

    #: (?untyped) -> bool
    def wait(timeout = nil) = %x{ return self.mon.WaitForCond(self, timeout) }

    #: () { () -> boolish } -> nil
    def wait_while
      wait while yield
      nil
    end

    #: () { () -> boolish } -> nil
    def wait_until
      wait until yield
      nil
    end

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

  #: () -> nil
  def mon_enter = __monitor.enter

  #: () -> nil
  def mon_exit = __monitor.exit

  #: () -> bool
  def mon_try_enter = __monitor.try_enter

  #: () -> bool
  def try_mon_enter = __monitor.try_enter

  #: () -> bool
  def mon_locked? = __monitor.mon_locked?

  #: () -> bool
  def mon_owned? = __monitor.mon_owned?

  # @rbs [T] () { () -> T } -> T
  def mon_synchronize
    __monitor.enter
    begin
      yield
    ensure
      __monitor.exit
    end
  end

  # @rbs [T] () { () -> T } -> T
  def synchronize
    __monitor.enter
    begin
      yield
    ensure
      __monitor.exit
    end
  end

  #: () -> MonitorMixin::ConditionVariable
  def new_cond = __monitor.new_cond

  #: () -> nil
  def mon_initialize = nil

  # The object's monitor, in a Go-side table by identity (a module has no ivars; decision 74's pattern).
  #: () -> Monitor
  def __monitor = %x{ return rbMonitorFor(any(self)) }
end
