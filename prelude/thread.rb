# prelude/thread.rb
# rbs_inline: enabled
#
# Thread as a goroutine. Like MRI, an exception ends only its thread (and is
# reported on stderr); join re-raises it in the joining thread.

# @go_type struct { done chan struct{}; err any }
class Thread < Object
  #: () { () -> void } -> Thread
  def self.new = %x{
    t := &Thread{done: make(chan struct{})}
    go func() {
      defer close(t.done)
      defer func() {
        if r := recover(); r != nil {
          if e, ok := r.(SystemExitI); ok {
            // ponytail: MRI re-raises exit in the main thread, running its ensures; this exits here.
            rbFlush()
            os.Exit(int(e.Status()))
          }
          t.err = rbWrapPanic(r)
          fmt.Fprintln(os.Stderr, "#<Thread> terminated with exception (report_on_exception is true):", rbToS(t.err), "("+rbClassName(t.err)+")")
        }
      }()
      blk()
    }()
    return t
  }

  #: () -> Thread
  def join = %x{
    <-self.done
    if self.err != nil {
      panic(self.err)
    }
    return self
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
