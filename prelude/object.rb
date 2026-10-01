# prelude/object.rb
# rbs_inline: enabled
#
# The root of the hierarchy: BasicObject, Kernel, Object, Comparable.

class BasicObject
  #: (untyped) -> bool
  def equal?(other) = %x{ Boolean(rbIdentical(self, other)) }

  #: (untyped) -> bool
  def ==(other) = %x{ Boolean(rbIdentical(self, other)) }

  #: (untyped) -> bool
  def !=(other) = %x{ !rbEq[any](self, other) }

  #: () -> bool
  def ! = false
end

module Kernel
  # @rbs [X] () { (self) -> X } -> X
  def then = yield(self)

  # @rbs [X] () { (self) -> X } -> self
  def tap
    yield(self)
    self
  end

  #: () -> String
  def to_s = %x{ rbObjToS(self) }

  # MRI's default inspect lists the ivars and never calls to_s.
  #: () -> String
  def inspect = %x{ rbObjInspect(self) }

  #: () -> bool
  def nil? = false

  # Heap objects by address, other values by hash: unique while alive, but not MRI's numbers.
  #: () -> Integer
  def object_id = %x{ return rbObjectID(self) }

  # Exact class, unlike is_a?: the generated class IDs are equal (decision 82).
  #: (Module) -> bool
  def instance_of?(klass) = %x{
    d, ok := any(klass).(interface{ _DescID() int })
    return Boolean(ok && rbClassID(self) == d._DescID())
  }

  # Objects, Array, Hash and Struct values are mutable; frozen classes override.
  #: () -> bool
  def frozen? = false

  #: (?Integer) -> void
  def exit(status = 0) = raise(SystemExit.new(status))

  # Seconds slept, rounded, as MRI returns them. The argument's class picks the overload (decision 12).
  #: (Float) -> Integer
  def sleep(secs) = %x{ return rbSleep(float64(secs)) }

  #: (Integer) -> Integer
  def __sleep_integer(secs) = %x{ return rbSleep(float64(secs)) }

  # No argument: forever, as MRI (nothing wakes it; Go reports a deadlock if no other goroutine runs).
  #: () -> Integer
  def __sleep_0 = %x{ rbSleepForever() }

  # A labelled break that carries a value (decision 91): throw unwinds to
  # the innermost catch with an identical tag, running ensures, passing
  # rescues. The thrown value's type is only known at the throw, so the
  # result is untyped.
  #: (?untyped) { (untyped) -> untyped } -> untyped
  def catch(tag = __fresh_object) = %x{ return rbCatch(tag, blk) }

  #: () -> untyped
  def __fresh_object = %x{ return &Object{} }

  #: (untyped, ?untyped) -> bot
  def throw(tag, value = nil) = %x{ rbThrowTag(tag, value) }

  private

  # Kernel#exit! skips at_exit handlers and doesn't flush stdout.
  #: (?Integer) -> void
  def __exit_bang(status = 0) = %x{ os.Exit(int(status)) }

  # For at_exit handlers, in place of `$!`: true unless an uncaught exception or a failing exit is pending.
  #: () -> bool
  def __exit_status_ok? = %x{ return Boolean(rbExitStatusNow.Load() == 0) }

  # Handlers run LIFO after main, exit or an uncaught exception (rbTopRecover).
  #: () { () -> void } -> void
  def at_exit = %x{ rbAtExitPush(blk) }

  #: (*untyped) -> nil
  def puts(*args)
    return __write("\n") if args.empty?

    args.each do |a|
      case a
      when nil then __write("\n")
      when Array then a.each { |e| puts(e) }
      else
        s = a.to_s
        __write(s.end_with?("\n") ? s : s + "\n")
      end
    end
    nil
  end

  # MRI returns its argument(s); rb2go's p is a statement-only nil.
  #: (*untyped) -> nil
  def p(*args)
    args.each { |a| __write(a.inspect + "\n") }
    nil
  end

  #: (String, *untyped) -> String
  def format(fmt, *args) = %x{ String(rbFormat(string(fmt), rest_)) }

  #: (String, *untyped) -> String
  def sprintf(fmt, *args) = %x{ String(rbFormat(string(fmt), rest_)) }

  #: (String, *untyped) -> nil
  def printf(fmt, *args)
    __write(format(fmt, *args))
    nil
  end

  #: (*untyped) -> nil
  def print(*args)
    args.each { |a| __write(a.to_s) }
    nil
  end

  #: (String) -> nil
  def __write(s) = %x{ rbWriteOut(string(s)) }

  #: () -> String
  def __class_name = %x{ String(rbClassName(self)) }
end

class Object < BasicObject
  include Kernel
end

module Comparable
  #: (self) -> Integer
  def <=>(other) = raise(NotImplementedError)

  #: (self) -> bool
  def <(other) = (self <=> other) < 0

  #: (self) -> bool
  def <=(other) = (self <=> other) <= 0

  #: (self) -> bool
  def >(other) = (self <=> other) > 0

  #: (self) -> bool
  def >=(other) = (self <=> other) >= 0

  #: (self, self) -> bool
  def between?(lo, hi) = (self <=> lo) >= 0 && (self <=> hi) <= 0

  #: (self, self) -> self
  def clamp(lo, hi)
    raise ArgumentError, "min argument must be less than or equal to max argument" if (lo <=> hi) > 0
    return lo if (self <=> lo) < 0
    return hi if (self <=> hi) > 0
    self
  end
end
