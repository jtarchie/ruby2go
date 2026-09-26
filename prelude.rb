# prelude.rb
# rbs_inline: enabled

# Top-level %x{} is emitted verbatim. Imports are resolved by goimports.
%x{
  func identical(a, b any) bool {
    switch a := a.(type) {
    case String:
      b, ok := b.(String)
      return ok && len(a) == len(b) && unsafe.StringData(string(a)) == unsafe.StringData(string(b))
    default:
      return a == b
    }
  }
}

class BasicObject
  #: (untyped) -> bool
  def equal?(other) = %x{ Boolean(identical(self, other)) }
end

module Kernel
  # @rbs [X] () { (self) -> X } -> X
  def then = yield(self)

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

  #: (String) -> nil
  def __write(s) = %x{ stdout.WriteString(string(s)) }
end

class Object < BasicObject
  include Kernel
end

module Comparable
  #: (self) -> bool
  def <(other) = (self <=> other) < 0

  #: (self, self) -> self
  def clamp(lo, hi)
    return lo if self < lo
    return hi if (self <=> hi) > 0
    self
  end
end

# RBS `bool`; stands in for TrueClass/FalseClass.
# @go_type bool
class Boolean < Object; end

# @go_type int
class Integer < Object
  #: (Integer) -> bool
  def <(other) = %x{ Boolean(self < other) }

  #: (Integer) -> bool
  def >(other) = %x{ Boolean(self > other) }
end

# @go_type string
class String < Object
  include Comparable

  #: () -> String
  def upcase = %x{ String(strings.ToUpper(string(self))) }

  #: (String) -> Integer
  def <=>(other) = %x{ Integer(strings.Compare(string(self), string(other))) }

  #: (String) -> bool
  def ==(other) = %x{ Boolean(self == other) }

  #: (String) -> String
  def +(other) = %x{ self + other }

  #: () -> String
  def dup = %x{ String(strings.Clone(string(self))) }
end
