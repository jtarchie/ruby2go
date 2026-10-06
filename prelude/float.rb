# prelude/float.rb
# rbs_inline: enabled
#
# Float as a named Go float64.

# @go_type float64
class Float < Object
  include Numeric

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

  # == on two Floats (decision 12's class overload): a plain Go compare,
  # inlined, with no boxing of the argument into untyped.
  #: (Float) -> bool
  def __eq_float(other) = %x{ Boolean(self == other) }

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

  # The conversion stops Go fusing a*b+c into one FMA rounding; MRI rounds each op.
  #: (Float) -> Float
  def *(other) = %x{ Float(float64(self * other)) }

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

  # MRI's rb_float_floor/ceil: scale, floor, then step back if the scaled
  # result overshot. digits <= 0 would be an Integer in MRI, so they raise.
  #: (Integer) -> Float
  def __floor_1(digits) = %x{
    x := float64(self)
    if digits <= 0 {
      panic(NewArgumentError(Ref(String("rb2go: Float#floor(digits) needs digits > 0"))))
    }
    if digits >= 15 || math.IsInf(x, 0) || math.IsNaN(x) {
      return self
    }
    f := math.Pow(10, float64(digits))
    mul := math.Floor(x * f)
    if res := (mul + 1) / f; res <= x {
      return Float(res)
    }
    return Float(mul / f)
  }

  #: (Integer) -> Float
  def __ceil_1(digits) = %x{
    x := float64(self)
    if digits <= 0 {
      panic(NewArgumentError(Ref(String("rb2go: Float#ceil(digits) needs digits > 0"))))
    }
    if digits >= 15 || math.IsInf(x, 0) || math.IsNaN(x) {
      return self
    }
    f := math.Pow(10, float64(digits))
    mul := math.Ceil(x * f)
    if res := (mul - 1) / f; res >= x {
      return Float(res)
    }
    return Float(mul / f)
  }

  #: () -> Integer
  def truncate = to_i

  # The result takes the divisor's sign, as MRI's flo_mod.
  #: (Float) -> Float
  def %(other) = %x{
    m := math.Mod(float64(self), float64(other))
    if m != 0 && (m < 0) != (other < 0) {
      m += float64(other)
    }
    return Float(m)
  }

  #: (Float) -> Float
  def modulo(other) = self % other

  #: (Float) -> [Integer, Float]
  def divmod(other) = [(self / other).floor, self % other]

  #: (Float) -> Float
  def fdiv(other) = self / other

  #: () -> bool
  def finite? = %x{ Boolean(!math.IsInf(float64(self), 0) && !math.IsNaN(float64(self))) }

  #: () -> Integer?
  def infinite? = %x{
    switch {
    case math.IsInf(float64(self), 1):
      return Ref(Integer(1))
    case math.IsInf(float64(self), -1):
      return Ref(Integer(-1))
    }
    return nil
  }

  #: () -> bool
  def integer? = false

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

class Float
  #: () -> Float
  def next_float = %x{ Float(math.Nextafter(float64(self), math.Inf(1))) }

  #: () -> Float
  def prev_float = %x{ Float(math.Nextafter(float64(self), math.Inf(-1))) }

  #: () -> bool
  def positive? = self > 0

  #: () -> bool
  def negative? = self < 0

  #: () -> Float
  def magnitude = abs

  #: () -> Integer
  def to_int = to_i

  #: (untyped) -> bool
  def eql?(other) = %x{
    o, ok := rbUnbox(other).(Float)
    return Boolean(ok && o == self)
  }

  #: () -> Float?
  def nonzero? = zero? ? nil : self

  #: () -> Float
  def abs2 = self * self

  #: () -> Float
  def conj = self

  #: () -> Integer
  def imag = 0

  #: () -> bool
  def real? = true

  #: () -> [Float, Integer]
  def rect = [self, 0]

  #: () -> [Float, untyped]
  def polar = negative? ? [abs, Math::PI] : [self, 0]

  # MRI's ruby_float_step: a counted loop, so 1.0.step(2.0, 0.1) ends at 2.0 exactly.
  #: (Float, ?Float) { (Float) -> void } -> void
  def step(limit, by = 1.0) = %x{
    return func(yield func(Float) bool) { rbFloatStep(float64(self), float64(limit), float64(by), yield) }
  }

  #: (Float, ?Float) -> Enumerator::ArithmeticSequence[Float]
  def __step_enum(limit, by = 1.0) = %x{ return rbArithNum(self, limit, by, by != 1) }

  #: (Float) -> Integer
  def div(other)
    raise ZeroDivisionError, "divided by 0" if other == 0
    (self / other).floor
  end

  # Truncated, so the result takes self's sign, as MRI's flo_remainder.
  #: (Float) -> Float
  def remainder(other) = %x{ Float(math.Mod(float64(self), float64(other))) }

  #: (Float) -> Float
  def quo(other) = self / other

  #: (Float) -> [Float, Float]
  def coerce(other) = [other, self]

  #: (Rational) -> [Float, Float]
  def __coerce_rational(other) = [other.to_f, self]
end

# Kernel#Float: strict conversion, overloaded like Kernel#Integer.
module Kernel
  private

  #: (untyped) -> Float
  def Float(x) = %x{
    switch v := rbUnbox(x).(type) {
    case Integer:
      return Float(v)
    case Float:
      return v
    case String:
      return rbStrictFloat(v)
    case nil:
      panic(NewTypeError(Ref(String("can't convert nil into Float"))))
    }
    panic(NewTypeError(Ref(String("can't convert " + rbClassName(x) + " into Float"))))
  }

  #: (Integer) -> Float
  def __Float_integer(x) = x.to_f

  #: (Float) -> Float
  def __Float_float(x) = x

  #: (String) -> Float
  def __Float_string(x) = %x{ rbStrictFloat(x) }
end
