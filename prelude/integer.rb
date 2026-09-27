# prelude/integer.rb
# rbs_inline: enabled
#
# Integer as a named Go int.

# @go_type int
class Integer < Object
  include Comparable

  #: (Integer) -> Integer
  def <=>(other) = %x{ Integer(cmp.Compare(self, other)) }

  #: (Integer) -> bool
  def <(other) = %x{ Boolean(self < other) }

  #: (Integer) -> bool
  def <=(other) = %x{ Boolean(self <= other) }

  #: (Integer) -> bool
  def >(other) = %x{ Boolean(self > other) }

  #: (Integer) -> bool
  def >=(other) = %x{ Boolean(self >= other) }

  #: (untyped) -> bool
  def ==(other) = %x{
    switch o := other.(type) {
    case Integer:
      return Boolean(self == o)
    case Float:
      return Boolean(Float(self) == o)
    case *Integer: // a T? box that reached untyped unconverted; nil must not reach rbEq
      return self.Eq(Opt(o))
    case *Float:
      return self.Eq(Opt(o))
    }
    return rbEq[any](other, self) // MRI asks other == self
  }

  #: (Integer) -> Integer
  def +(other) = %x{ self + other }

  #: (Integer) -> Integer
  def -(other) = %x{ self - other }

  #: (Integer) -> Integer
  def *(other) = %x{ self * other }

  # Ruby floors; Go truncates.
  #: (Integer) -> Integer
  def /(other) = %x{
    if other == 0 {
      panic(NewZeroDivisionError(Ref[String]("divided by 0")))
    }
    q := self / other
    if self%other != 0 && (self < 0) != (other < 0) {
      q--
    }
    return q
  }

  #: (Integer) -> Integer
  def %(other) = %x{
    if other == 0 {
      panic(NewZeroDivisionError(Ref[String]("divided by 0")))
    }
    m := self % other
    if m != 0 && (m < 0) != (other < 0) {
      m += other
    }
    return m
  }

  #: (Integer) -> Integer
  def **(other) = %x{
    if self == 0 && other < 0 {
      panic(NewZeroDivisionError(Ref[String]("divided by 0")))
    }
    result := Integer(1)
    for i := Integer(0); i < other; i++ {
      result *= self
    }
    return result
  }

  #: () -> Integer
  def -@ = %x{ -self }

  #: () -> Integer
  def abs = %x{
    if self < 0 {
      return -self
    }
    return self
  }

  #: () -> bool
  def even? = %x{ self%2 == 0 }

  #: () -> bool
  def odd? = %x{ self%2 != 0 }

  #: () -> bool
  def zero? = %x{ self == 0 }

  #: () -> bool
  def positive? = %x{ self > 0 }

  #: () -> bool
  def negative? = %x{ self < 0 }

  #: () -> Integer
  def succ = self + 1

  #: () -> Integer
  def pred = self - 1

  #: () -> Integer
  def to_i = self

  #: () -> Float
  def to_f = %x{ Float(self) }

  #: () -> String
  def to_s = %x{ String(strconv.Itoa(int(self))) }

  #: () -> String
  def inspect = to_s

  # Immediates are always frozen.
  #: () -> bool
  def frozen? = true

  #: () -> Integer
  def hash = self

  # MRI's Integer#chr without an encoding: one byte, RangeError outside 0..255.
  #: () -> String
  def chr = %x{
    if self < 0 || self > 255 {
      panic(NewRangeError(Ref(String(strconv.Itoa(int(self)) + " out of char range"))))
    }
    return String([]byte{byte(self)})
  }

  #: () { (Integer) -> void } -> void
  def times = %x{
    return func(yield func(Integer) bool) {
      for i := Integer(0); i < self; i++ {
        if !yield(i) {
          return
        }
      }
    }
  }

  #: (Integer) { (Integer) -> void } -> void
  def upto(limit) = %x{
    return func(yield func(Integer) bool) {
      for i := self; i <= limit; i++ {
        if !yield(i) {
          return
        }
      }
    }
  }

  #: (Integer) { (Integer) -> void } -> void
  def downto(limit) = %x{
    return func(yield func(Integer) bool) {
      for i := self; i >= limit; i-- {
        if !yield(i) {
          return
        }
      }
    }
  }
end
