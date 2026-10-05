# rbs_inline: enabled

# BigDecimal (decision 112): arbitrary-precision decimals over math/big,
# with bigdecimal 4.1's precision rules (prelude/go/bigdecimal.go).
# Operands may be a BigDecimal, Integer, Float or Rational.
# @go_type struct { mant *big.Int; exp int; neg bool; kind int }
class BigDecimal < Object
  include Numeric

  VERSION = "4.1.2" #: String

  ROUND_MODE = 256 #: Integer
  ROUND_UP = 1 #: Integer
  ROUND_DOWN = 2 #: Integer
  ROUND_HALF_UP = 3 #: Integer
  ROUND_HALF_DOWN = 4 #: Integer
  ROUND_CEILING = 5 #: Integer
  ROUND_FLOOR = 6 #: Integer
  ROUND_HALF_EVEN = 7 #: Integer

  SIGN_NaN = 0 #: Integer
  SIGN_POSITIVE_ZERO = 1 #: Integer
  SIGN_NEGATIVE_ZERO = -1 #: Integer
  SIGN_POSITIVE_FINITE = 2 #: Integer
  SIGN_NEGATIVE_FINITE = -2 #: Integer
  SIGN_POSITIVE_INFINITE = 3 #: Integer
  SIGN_NEGATIVE_INFINITE = -3 #: Integer

  INFINITY = BigDecimal.__special(1) #: BigDecimal
  NAN = BigDecimal.__special(2) #: BigDecimal

  #: (Integer) -> BigDecimal
  def self.__special(kind) = %x{ return rbBDSpecial(int(kind), false) }

  #: () -> Integer
  def self.double_fig = 16

  #: (untyped) -> BigDecimal
  def +(other) = %x{ return rbBDAddSub(self, rbBDArg(other, rbBDCoercePrec(self)), false, 0) }

  #: (untyped) -> BigDecimal
  def -(other) = %x{ return rbBDAddSub(self, rbBDArg(other, rbBDCoercePrec(self)), true, 0) }

  #: (untyped) -> BigDecimal
  def *(other) = %x{ return rbBDMul(self, rbBDArg(other, rbBDCoercePrec(self)), 0) }

  #: (untyped) -> BigDecimal
  def /(other) = %x{ return rbBDDiv(self, rbBDArg(other, rbBDCoercePrec(self)), 0) }

  #: (untyped) -> BigDecimal
  def quo(other) = self / other

  #: (untyped, Integer) -> BigDecimal
  def __quo_2(other, digits) = __div_2(other, digits)

  #: (untyped) -> BigDecimal
  def %(other) = %x{
    _, m := rbBDDivmod(self, rbBDArg(other, rbBDCoercePrec(self)), false)
    return m
  }

  #: (untyped) -> BigDecimal
  def modulo(other) = self % other

  #: (untyped) -> BigDecimal
  def remainder(other) = %x{
    _, m := rbBDDivmod(self, rbBDArg(other, rbBDCoercePrec(self)), true)
    return m
  }

  # The quotient rounded toward negative infinity, as an Integer, and the modulus.
  #: (untyped) -> [Integer, BigDecimal]
  def divmod(other)
    [div(other), self % other]
  end

  # The floored quotient as an Integer; with digits, a BigDecimal quotient to that many significant digits.
  #: (untyped) -> Integer
  def div(other) = %x{
    q, _ := rbBDDivmod(self, rbBDArg(other, rbBDCoercePrec(self)), false)
    return rbBDToI(q)
  }

  #: (untyped, Integer) -> BigDecimal
  def __div_2(other, digits) = %x{
    if digits < 0 {
      panic(NewArgumentError(Ref(String("negative precision"))))
    }
    return rbBDDiv(self, rbBDArg(other, max(int(digits), rbBDCoercePrec(self))), int(digits))
  }

  #: (untyped, Integer) -> BigDecimal
  def add(other, digits) = %x{ return rbBDAddSub(self, rbBDArg(other, rbBDCoercePrec(self)), false, rbBDDigits(digits)) }

  #: (untyped, Integer) -> BigDecimal
  def sub(other, digits) = %x{ return rbBDAddSub(self, rbBDArg(other, rbBDCoercePrec(self)), true, rbBDDigits(digits)) }

  #: (untyped, Integer) -> BigDecimal
  def mult(other, digits) = %x{ return rbBDMul(self, rbBDArg(other, rbBDCoercePrec(self)), rbBDDigits(digits)) }

  #: (untyped) -> BigDecimal
  def **(other) = %x{ return rbBDPow(self, other, 0) }

  #: (untyped, ?Integer) -> BigDecimal
  def power(other, prec = 0) = %x{ return rbBDPow(self, other, rbBDDigits(prec)) }

  #: (Integer) -> BigDecimal
  def sqrt(prec) = %x{ return rbBDSqrt(self, rbBDDigits(prec)) }

  #: () -> BigDecimal
  def -@ = %x{
    if self.kind == rbBDNaN {
      return self
    }
    out := *self
    out.neg = !self.neg
    return &out
  }

  #: () -> BigDecimal
  def +@ = self

  #: () -> BigDecimal
  def abs = %x{
    out := *self
    if self.kind != rbBDNaN {
      out.neg = false
    }
    return &out
  }

  #: (BigDecimal) -> Integer?
  def <=>(other) = %x{
    c, ok := rbBDCmp(self, other)
    if !ok {
      return nil
    }
    return Ref(Integer(c))
  }

  #: (Integer) -> Integer?
  def __cmp_integer(other) = self <=> BigDecimal.__from(other)

  #: (Float) -> Integer?
  def __cmp_float(other) = self <=> BigDecimal.__from(other)

  #: (untyped) -> BigDecimal
  def self.__from(v) = %x{ return rbBDArg(v, 32) }

  #: (untyped) -> bool
  def ==(other) = %x{ return Boolean(rbBDRel(self, other, "==")) }

  #: (untyped) -> bool
  def ===(other) = self == other

  #: (untyped) -> bool
  def eql?(other) = self == other

  #: (untyped) -> bool
  def <(other) = %x{ return Boolean(rbBDRel(self, other, "<")) }

  #: (untyped) -> bool
  def <=(other) = %x{ return Boolean(rbBDRel(self, other, "<=")) }

  #: (untyped) -> bool
  def >(other) = %x{ return Boolean(rbBDRel(self, other, ">")) }

  #: (untyped) -> bool
  def >=(other) = %x{ return Boolean(rbBDRel(self, other, ">=")) }

  # Comparable's, over any number: its own take a BigDecimal, and clamp answers the bound itself.
  #: (untyped, untyped) -> bool
  def between?(lo, hi) = self >= lo && self <= hi

  #: (untyped, untyped) -> untyped
  def clamp(lo, hi)
    raise ArgumentError, "min argument must be less than or equal to max argument" if BigDecimal.__from(lo) > hi
    return lo if self < lo
    return hi if self > hi
    self
  end

  #: () -> Integer
  def hash = %x{ return rbBDHash(self) }

  #: (untyped) -> [BigDecimal, BigDecimal]
  def coerce(other) = [BigDecimal.__from(other), self]

  # Rounded to an Integer (half up).
  #: () -> Integer
  def round = %x{ return rbBDToI(rbBDRoundAt(self, 0, rbBDRoundHalfUp, false)) }

  # n digits after the point: a BigDecimal, or an Integer when n < 1, as MRI's.
  #: (Integer) -> untyped
  def __round_1(n) = %x{
    r := rbBDRoundAt(self, int(n), rbBDRoundHalfUp, false)
    if n < 1 {
      return rbBDToI(r)
    }
    return r
  }

  # round(2): a literal n >= 1 is known to give a BigDecimal, so the compiler picks this typed twin.
  #: (Integer) -> BigDecimal
  def __round_digits(n) = %x{ return rbBDRoundAt(self, int(n), rbBDRoundHalfUp, false) }

  # With a mode (a Symbol such as :half_even, or a ROUND_* constant): always a BigDecimal.
  #: (Integer, untyped) -> BigDecimal
  def __round_2(n, mode) = %x{ return rbBDRoundAt(self, int(n), rbBDRoundMode(mode), false) }

  #: () -> Integer
  def floor = %x{ return rbBDToI(rbBDRoundAt(self, 0, rbBDRoundFloor, false)) }

  #: (Integer) -> BigDecimal
  def __floor_1(n) = %x{ return rbBDRoundAt(self, int(n), rbBDRoundFloor, false) }

  #: () -> Integer
  def ceil = %x{ return rbBDToI(rbBDRoundAt(self, 0, rbBDRoundCeiling, false)) }

  #: (Integer) -> BigDecimal
  def __ceil_1(n) = %x{ return rbBDRoundAt(self, int(n), rbBDRoundCeiling, false) }

  #: () -> Integer
  def truncate = %x{ return rbBDToI(rbBDRoundAt(self, 0, rbBDRoundDown, false)) }

  #: (Integer) -> BigDecimal
  def __truncate_1(n) = %x{ return rbBDRoundAt(self, int(n), rbBDRoundDown, false) }

  #: () -> BigDecimal
  def fix = %x{ return rbBDRoundAt(self, 0, rbBDRoundDown, false) }

  #: () -> BigDecimal
  def frac = self - fix

  # "0.123e1" (E form); "F" gives "1.23", a leading "+" or " " signs a positive value, and a digit count groups the digits with spaces.
  #: () -> String
  def to_s = %x{ return String(rbBDToS(self, "")) }

  #: (untyped) -> String
  def __to_s_1(format) = %x{
    switch f := rbUnbox(format).(type) {
    case Integer:
      if f <= 0 {
        panic(NewArgumentError(Ref(String("argument must be positive"))))
      }
      return String(rbBDToS(self, strconv.Itoa(int(f))))
    case String:
      return String(rbBDToS(self, string(f)))
    }
    return String(rbBDToS(self, ""))
  }

  #: () -> String
  def inspect = to_s

  #: () -> Float
  def to_f = %x{ return rbBDToF(self) }

  #: () -> Integer
  def to_i = %x{ return rbBDToI(self) }

  #: () -> Integer
  def to_int = to_i

  #: () -> Rational
  def to_r = %x{ return rbBDToR(self) }

  #: () -> BigDecimal
  def to_d = self

  # The plain decimal form, "1.23" (bigdecimal/util's own algorithm, which drops the sign of a value between -1 and 0: "-0.5" is "0.5").
  #: () -> String
  def to_digits
    return to_s if nan? || infinite? || zero?

    f = frac
    "#{to_i}.#{"0" * -f.exponent}#{f.__split_digits}"
  end

  # [sign, significant digits, 10, exponent]: 0.123e1 is [1, "123", 10, 1] (an Array: rb2go tuples stop at 3).
  #: () -> Array[untyped]
  def split = [__split_sign, __split_digits, 10, exponent]

  #: () -> Integer
  def __split_sign = %x{
    switch {
    case self.kind == rbBDNaN:
      return 0
    case self.neg:
      return -1
    }
    return 1
  }

  #: () -> String
  def __split_digits = %x{
    switch self.kind {
    case rbBDNaN:
      return "NaN"
    case rbBDInf:
      return "Infinity"
    }
    if self.isZero() {
      return "0"
    }
    return String(self.mant.String())
  }

  #: () -> Integer
  def exponent = %x{ return Integer(self.exponent10()) }

  #: () -> Integer
  def precision = %x{ return Integer(self.precision()) }

  #: () -> Integer
  def scale = %x{ return Integer(self.scale()) }

  #: () -> Integer
  def n_significant_digits = %x{ return Integer(self.digits()) }

  #: () -> Integer
  def sign = %x{ return Integer(rbBDSign(self)) }

  #: () -> bool
  def zero? = %x{ return Boolean(self.isZero()) }

  #: () -> BigDecimal?
  def nonzero? = %x{
    if self.isZero() {
      return nil
    }
    return &self
  }

  #: () -> bool
  def nan? = %x{ return Boolean(self.kind == rbBDNaN) }

  #: () -> Integer?
  def infinite? = %x{
    if self.kind != rbBDInf {
      return nil
    }
    if self.neg {
      return Ref(Integer(-1))
    }
    return Ref(Integer(1))
  }

  #: () -> bool
  def finite? = %x{ return Boolean(self.kind == rbBDFinite) }

  #: () -> bool
  def positive? = %x{ return Boolean(self.kind != rbBDNaN && !self.neg && !self.isZero()) }

  #: () -> bool
  def negative? = %x{ return Boolean(self.kind != rbBDNaN && self.neg && !self.isZero()) }

  # rb2go's Complex parts are Integer, Rational or Float only (decision 142).
  undef_method :to_c, :i

  #: () -> bool
  def integer? = false

  #: () -> bool
  def real? = true

  #: () -> BigDecimal
  def magnitude = abs

  # Numeric#fdiv: a Float divided by the argument.
  #: (untyped) -> Float
  def fdiv(other) = %x{ return Float(float64(rbBDToF(self)) / rbNumFloat(rbUnbox(other))) }

  #: (BigDecimal) -> BigDecimal
  def __fdiv_big_decimal(other) = to_f / other

  # Numeric#step: self, then each step added, while within limit.
  #: (untyped, ?untyped) { (BigDecimal) -> void } -> void
  def step(limit, by = 1)
    lim = BigDecimal.__from(limit)
    inc = BigDecimal.__from(by)
    raise ArgumentError, "step can't be 0" if inc.zero?
    i = self
    while inc.positive? ? i <= lim : i >= lim
      yield i
      i += inc
    end
  end

  #: (untyped, ?untyped) -> Array[BigDecimal]
  def __step_enum(limit, by = 1)
    out = [] #: Array[BigDecimal]
    step(limit, by) { |x| out << x }
    out
  end
end

module Kernel
  private

  # BigDecimal("1.23"), BigDecimal(42), BigDecimal(1.5) (shortest form, or digits significant digits), BigDecimal(Rational(1, 3), 10).
  #: (untyped, ?Integer) -> BigDecimal
  def BigDecimal(value, digits = 0) = %x{ return rbBDConvert(value, int(digits)) }
end

class Integer
  #: () -> BigDecimal
  def to_d = BigDecimal(self)

  #: (BigDecimal) -> BigDecimal
  def __plus_big_decimal(other) = other + self

  #: (BigDecimal) -> BigDecimal
  def __minus_big_decimal(other) = BigDecimal(self) - other

  #: (BigDecimal) -> BigDecimal
  def __mul_big_decimal(other) = other * self

  #: (BigDecimal) -> BigDecimal
  def __div_big_decimal(other) = BigDecimal(self) / other

  #: (BigDecimal) -> [Float, Float]
  def __coerce_big_decimal(other) = [other.to_f, to_f]

  #: (BigDecimal) -> Float
  def __fdiv_big_decimal(other) = (BigDecimal(self) / other).to_f
end

class Float
  #: (?Integer) -> BigDecimal
  def to_d(precision = 0) = BigDecimal(self, precision)

  #: (BigDecimal) -> BigDecimal
  def __plus_big_decimal(other) = BigDecimal(self) + other

  #: (BigDecimal) -> BigDecimal
  def __minus_big_decimal(other) = BigDecimal(self) - other

  #: (BigDecimal) -> BigDecimal
  def __mul_big_decimal(other) = BigDecimal(self) * other

  #: (BigDecimal) -> BigDecimal
  def __div_big_decimal(other) = BigDecimal(self) / other

  #: (BigDecimal) -> [Float, Float]
  def __coerce_big_decimal(other) = [other.to_f, to_f]
end

class String
  # The longest decimal prefix, 0 when there is none (bigdecimal/util).
  #: () -> BigDecimal
  def to_d = %x{
    v, _ := rbBDParse(string(self), false)
    return v
  }
end

class Rational
  #: (Integer) -> BigDecimal
  def to_d(precision) = BigDecimal(self, precision)

  # A mix with a BigDecimal converts at its coerce precision (decision 142).
  #: () -> BigDecimal
  def __to_d = BigDecimal.__from(self)

  #: (BigDecimal) -> Float
  def __fdiv_big_decimal(other) = (__to_d / other).to_f
end
