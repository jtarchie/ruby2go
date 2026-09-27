# prelude/float.rb
# rbs_inline: enabled
#
# Float as a named Go float64.

# @go_type float64
class Float < Object
  include Comparable

  #: (Float) -> Integer?
  def <=>(other) = %x{
    switch {
    case self < other:
      return Ref(Integer(-1))
    case self > other:
      return Ref(Integer(1))
    case self == other:
      return Ref(Integer(0))
    }
    return nil // NaN, as MRI; Comparable and sort call the generated Op_cmp, which raises (decision 3)
  }

  #: (Float) -> bool
  def <(other) = %x{ Boolean(self < other) }

  #: (Float) -> bool
  def <=(other) = %x{ Boolean(self <= other) }

  #: (Float) -> bool
  def >(other) = %x{ Boolean(self > other) }

  #: (Float) -> bool
  def >=(other) = %x{ Boolean(self >= other) }

  #: (untyped) -> bool
  def ==(other) = %x{
    switch o := other.(type) {
    case Float:
      return Boolean(self == o)
    case Integer:
      return Boolean(self == Float(o))
    case *Integer: // a T? box that reached untyped unconverted; nil must not reach rbEq
      return self.Op_eq(Opt(o))
    case *Float:
      return self.Op_eq(Opt(o))
    }
    return rbEq[any](other, self) // MRI asks other == self
  }

  #: (Float) -> Float
  def +(other) = %x{ self + other }

  #: (Float) -> Float
  def -(other) = %x{ self - other }

  #: (Float) -> Float
  def *(other) = %x{ self * other }

  #: (Float) -> Float
  def /(other) = %x{ self / other }

  #: (Float) -> Float
  def **(other) = %x{ Float(rbFloatPow(float64(self), float64(other))) }

  #: () -> Float
  def -@ = %x{ -self }

  #: () -> Float
  def abs = %x{ Float(math.Abs(float64(self))) }

  #: () -> Integer
  def to_i = %x{ rbFloatToI(float64(self)) }

  #: () -> Integer
  def floor = %x{ rbFloatToI(math.Floor(float64(self))) }

  #: () -> Integer
  def ceil = %x{ rbFloatToI(math.Ceil(float64(self))) }

  #: () -> Integer
  def round = %x{ rbFloatToI(math.Round(float64(self))) }

  # MRI's round_half_up; digits <= 0 would be an Integer in MRI, so they raise.
  #: (Integer) -> Float
  def __round_1(digits) = %x{
    if digits <= 0 {
      panic(NewArgumentError(Ref(String("rb2go: Float#round(digits) needs digits > 0"))))
    }
    x := float64(self)
    if digits >= 15 || math.IsInf(x, 0) || math.IsNaN(x) {
      return self
    }
    s := math.Pow(10, float64(digits))
    f := math.Round(x * s)
    if x > 0 && (f+0.5)/s <= x {
      f++
    } else if x < 0 && (f-0.5)/s >= x {
      f--
    }
    return Float(f / s)
  }

  #: () -> Float
  def to_f = self

  #: () -> bool
  def zero? = %x{ self == 0 }

  #: () -> bool
  def nan? = %x{ Boolean(math.IsNaN(float64(self))) }

  #: () -> String
  def to_s = %x{ rbFloatToS(float64(self)) }

  #: () -> String
  def inspect = to_s

  # Immediates are always frozen.
  #: () -> bool
  def frozen? = true
end
