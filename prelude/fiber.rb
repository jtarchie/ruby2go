# rbs_inline: enabled

class FiberError < StandardError; end

# A coroutine (iter.Pull): resume is next(), Fiber.yield is yield() (decisions 140, 403); values are untyped, as Ractor messages are.

# @go_type struct { blk func(any) any; next func() (any, bool); stop func(); yield func(any) bool; arg any; val any; err any; state rbFiberState; catches []any }
class Fiber < Object
  #: () { (untyped) -> untyped } -> Fiber
  def self.new = %x{ return &Fiber{blk: blk} }

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
