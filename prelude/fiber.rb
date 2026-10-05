# rbs_inline: enabled

class FiberError < StandardError; end

# A goroutine handed control over unbuffered channels, so one side runs at a time (decision 140); values are untyped, as Ractor messages are.

# @go_type struct { blk func(any) any; in chan any; out chan rbFiberMsg; state rbFiberState }
class Fiber < Object
  #: () { (untyped) -> untyped } -> Fiber
  def self.new = %x{ return &Fiber{blk: blk, in: make(chan any), out: make(chan rbFiberMsg)} }

  #: (*untyped) -> untyped
  def resume(*args) = %x{ return self.resume(rest_) }

  #: (*untyped) -> untyped
  def self.yield(*vals) = %x{ return rbFiberYield(rest_) }

  #: () -> bool
  def alive? = %x{ Boolean(self.state != rbFiberDead) }

  #: () -> Fiber
  def self.current = %x{ return rbCurrentFiber() }

  #: () -> String
  def inspect = %x{ return String(fmt.Sprintf("#<Fiber:%p (%s)>", self, self.state)) }

  #: () -> String
  def to_s = inspect
end
