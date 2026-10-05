# rbs_inline: enabled

# Components keep their class (Integer, Rational or Float), as MRI's do, so exact parts stay exact.
# @go_type struct { re any; im any }
class Complex < Object
  include Numeric

  #: (untyped, ?untyped) -> Complex
  def self.rectangular(re, im = 0) = Complex(re, im)

  #: (untyped, ?untyped) -> Complex
  def self.rect(re, im = 0) = Complex(re, im)

  #: (untyped, ?untyped) -> Complex
  def self.polar(r, theta = 0) = %x{ rbComplexPolar(rbNumArg(r), rbNumArg(theta)) }

  #: () -> untyped
  def real = %x{ self.re }

  #: () -> untyped
  def imaginary = %x{ self.im }

  #: () -> untyped
  def imag = %x{ self.im }

  #: () -> [untyped, untyped]
  def rectangular = %x{ Tuple2[any, any]{self.re, self.im} }

  #: () -> [untyped, untyped]
  def rect = rectangular

  #: () -> [Float, Float]
  def polar = %x{ Tuple2[Float, Float]{Float(rbComplexAbsF(self)), Float(rbComplexArg(self))} }

  #: (Complex) -> Complex
  def +(o) = %x{ return &Complex{rbNumOp('+', self.re, o.re), rbNumOp('+', self.im, o.im)} }

  #: (Complex) -> Complex
  def -(o) = %x{ return &Complex{rbNumOp('-', self.re, o.re), rbNumOp('-', self.im, o.im)} }

  #: (Complex) -> Complex
  def *(o) = %x{ return rbComplexMul(self, o) }

  #: (Complex) -> Complex
  def /(o) = %x{ return rbComplexQuo(self, o) }

  #: (Complex) -> Complex
  def quo(o) = self / o

  #: (Integer) -> Complex
  def **(n) = %x{ return rbComplexPowInt(self, int(n)) }

  #: (Float) -> Complex
  def __pow_float(x) = %x{
    r, theta := rbComplexAbsF(self), rbComplexArg(self)
    return rbComplexPolar(Float(rbFloatPow(r, float64(x))), Float(theta*float64(x)))
  }

  #: () -> Complex
  def -@ = %x{ return &Complex{rbNumNeg(self.re), rbNumNeg(self.im)} }

  #: () -> Complex
  def conjugate = %x{ return &Complex{self.re, rbNumNeg(self.im)} }

  #: () -> Complex
  def conj = conjugate

  # Integer when the other part is an exact zero, as MRI.
  #: () -> untyped
  def abs = %x{ rbComplexAbs(self) }

  #: () -> untyped
  def magnitude = abs

  #: () -> untyped
  def abs2 = %x{ rbNumOp('+', rbNumOp('*', self.re, self.re), rbNumOp('*', self.im, self.im)) }

  #: () -> Float
  def arg = %x{ Float(rbComplexArg(self)) }

  #: () -> Float
  def angle = arg

  #: () -> Float
  def phase = arg

  #: (untyped) -> Complex
  def fdiv(n) = %x{
    d := rbNumFloat(rbNumArg(n))
    return &Complex{Float(rbNumFloat(self.re) / d), Float(rbNumFloat(self.im) / d)}
  }

  #: () -> bool
  def real? = false

  #: () -> bool
  def finite? = %x{ Boolean(!math.IsInf(rbNumFloat(self.re), 0) && !math.IsInf(rbNumFloat(self.im), 0)) }

  #: (untyped) -> bool
  def ==(other) = %x{
    switch o := rbUnbox(other).(type) {
    case *Complex:
      return Boolean(rbNumEq(self.re, o.re) && rbNumEq(self.im, o.im))
    case Integer, Float, *Rational:
      return Boolean(rbNumEq(self.re, o) && rbNumEq(self.im, Integer(0)))
    }
    return false
  }

  #: (untyped) -> bool
  def eql?(other) = %x{
    o, ok := rbUnbox(other).(*Complex)
    return Boolean(ok && rbKeyEql(self.re, o.re) && rbKeyEql(self.im, o.im) && rbClassName(self.re) == rbClassName(o.re) && rbClassName(self.im) == rbClassName(o.im))
  }

  #: () -> Integer
  def hash = %x{ rbHash(self.re)*31 + rbHash(self.im) }

  #: () -> bool
  def frozen? = true

  #: () -> Integer
  def to_i = %x{
    rbComplexReal(self)
    return rbAs[Integer](rbNumTo(self.re, 0), "Integer")
  }

  #: () -> Float
  def to_f = %x{
    rbComplexReal(self)
    return Float(rbNumFloat(self.re))
  }

  #: () -> Rational
  def to_r = %x{
    rbComplexReal(self)
    return rbNumTo(self.re, 1).(*Rational)
  }

  #: () -> Complex
  def to_c = self

  #: () -> String
  def to_s = %x{ String(rbComplexFmt(self, rbToS)) }

  #: () -> String
  def inspect = %x{ String("(" + rbComplexFmt(self, rbInspect) + ")") }

  #: (Integer) -> Complex
  def __plus_integer(o) = %x{ return rbComplexScalar('+', self, o, false) }

  #: (Integer) -> Complex
  def __minus_integer(o) = %x{ return rbComplexScalar('-', self, o, false) }

  #: (Integer) -> Complex
  def __mul_integer(o) = %x{ return rbComplexScalar('*', self, o, false) }

  #: (Integer) -> Complex
  def __div_integer(o) = %x{ return rbComplexScalar('/', self, o, false) }

  #: (Float) -> Complex
  def __plus_float(o) = %x{ return rbComplexScalar('+', self, o, false) }

  #: (Float) -> Complex
  def __minus_float(o) = %x{ return rbComplexScalar('-', self, o, false) }

  #: (Float) -> Complex
  def __mul_float(o) = %x{ return rbComplexScalar('*', self, o, false) }

  #: (Float) -> Complex
  def __div_float(o) = %x{ return rbComplexScalar('/', self, o, false) }

  #: (Rational) -> Complex
  def __plus_rational(o) = %x{ return rbComplexScalar('+', self, o, false) }

  #: (Rational) -> Complex
  def __minus_rational(o) = %x{ return rbComplexScalar('-', self, o, false) }

  #: (Rational) -> Complex
  def __mul_rational(o) = %x{ return rbComplexScalar('*', self, o, false) }

  #: (Rational) -> Complex
  def __div_rational(o) = %x{ return rbComplexScalar('/', self, o, false) }

  # A Complex has no order: MRI undefines these.
  undef_method :%, :<, :<=, :>, :>=, :between?, :clamp, :div, :divmod, :modulo, :remainder, :positive?, :negative?, :floor, :ceil, :round, :truncate, :i

  # Only real values compare (an imaginary part of zero, exact or not), as MRI's nucomp_cmp.
  #: (Complex) -> Integer?
  def <=>(other) = %x{ return rbComplexCmp(self, other) }

  #: () -> bool
  def zero? = %x{ Boolean(rbNumZero(self.re) && rbNumZero(self.im)) }

  #: () -> Complex?
  def nonzero? = zero? ? nil : self

  #: () -> bool
  def integer? = false

  #: () -> Integer?
  def infinite? = %x{
    if math.IsInf(rbNumFloat(self.re), 0) || math.IsInf(rbNumFloat(self.im), 0) {
      return Ref(Integer(1))
    }
    return nil
  }

  #: (untyped) -> [Complex, Complex]
  def coerce(other) = %x{
    if c, ok := rbUnbox(other).(*Complex); ok {
      return Tuple2[*Complex, *Complex]{c, self}
    }
    return Tuple2[*Complex, *Complex]{&Complex{rbNumArg(other), Integer(0)}, self}
  }
end

class Integer
  #: () -> Complex
  def to_c = Complex(self)

  #: () -> Complex
  def i = Complex(0, self)

  #: (Complex) -> Complex
  def __plus_complex(o) = %x{ return rbComplexScalar('+', o, self, true) }

  #: (Complex) -> Complex
  def __minus_complex(o) = %x{ return rbComplexScalar('-', o, self, true) }

  #: (Complex) -> Complex
  def __mul_complex(o) = %x{ return rbComplexScalar('*', o, self, true) }

  #: (Complex) -> Complex
  def __div_complex(o) = Complex(self) / o
end

class Float
  #: () -> Complex
  def to_c = Complex(self)

  #: () -> Complex
  def i = Complex(0, self)

  #: (Complex) -> Complex
  def __plus_complex(o) = %x{ return rbComplexScalar('+', o, self, true) }

  #: (Complex) -> Complex
  def __minus_complex(o) = %x{ return rbComplexScalar('-', o, self, true) }

  #: (Complex) -> Complex
  def __mul_complex(o) = %x{ return rbComplexScalar('*', o, self, true) }

  #: (Complex) -> Complex
  def __div_complex(o) = Complex(self) / o
end

class Rational
  #: () -> Complex
  def to_c = Complex(self)

  #: () -> Complex
  def i = Complex(0, self)

  #: (Complex) -> Complex
  def __plus_complex(o) = %x{ return rbComplexScalar('+', o, self, true) }

  #: (Complex) -> Complex
  def __minus_complex(o) = %x{ return rbComplexScalar('-', o, self, true) }

  #: (Complex) -> Complex
  def __mul_complex(o) = %x{ return rbComplexScalar('*', o, self, true) }

  #: (Complex) -> Complex
  def __div_complex(o) = Complex(self) / o
end

module Kernel
  private

  #: (untyped, ?untyped) -> Complex
  def Complex(re, im = 0) = %x{ return &Complex{rbNumArg(re), rbNumArg(im)} }
end
