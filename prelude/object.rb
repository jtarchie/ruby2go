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
  def !=(other) = !(self == other)

  #: () -> bool
  def ! = false
end

module Kernel
  # @rbs [X] () { (self) -> X } -> X
  def then = yield(self)

  #: () -> String
  def to_s = %x{ String("#<" + rbClassName(self) + ">") }

  #: () -> String
  def inspect = to_s

  #: () -> bool
  def nil? = false

  #: () -> bool
  def frozen? = true

  #: (?Integer) -> void
  def exit(status = 0) = %x{ _ = stdout.Flush(); os.Exit(int(status)) }

  private

  #: (*untyped) -> nil
  def puts(*args)
    return __write("\n") if args.empty?

    args.each do |a|
      case a
      when nil then __write("\n")
      when Array then puts(*a)
      else
        s = a.to_s
        __write(s.end_with?("\n") ? s : s + "\n")
      end
    end
    nil
  end

  #: (*untyped) -> nil
  def print(*args)
    args.each { |a| __write(a.to_s) }
    nil
  end

  #: (String) -> nil
  def __write(s) = %x{ rbWrite(string(s)) }

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
  def between?(lo, hi) = !(self < lo) && !(hi < self)

  #: (self, self) -> self
  def clamp(lo, hi)
    raise ArgumentError, "min argument must be less than or equal to max argument" if (lo <=> hi) > 0
    return lo if self < lo
    return hi if (self <=> hi) > 0
    self
  end
end
