# rbs_inline: enabled

module Math
  class DomainError < ArgumentError; end

  PI = 3.141592653589793 #: Float
  E = 2.718281828459045 #: Float

  #: (Float) -> Float
  def self.sqrt(x) = %x{
    rbMathDomain(x < 0, "sqrt")
    return Float(math.Sqrt(float64(x)))
  }

  #: (Float) -> Float
  def self.cbrt(x) = %x{ Float(rbCbrt(float64(x))) }

  #: (Float) -> Float
  def self.sin(x) = %x{ Float(rbSin(float64(x))) }

  #: (Float) -> Float
  def self.cos(x) = %x{ Float(rbCos(float64(x))) }

  #: (Float) -> Float
  def self.tan(x) = %x{ Float(rbTan(float64(x))) }

  #: (Float) -> Float
  def self.asin(x) = %x{
    rbMathDomain(x < -1 || x > 1, "asin")
    return Float(rbAsin(float64(x)))
  }

  #: (Float) -> Float
  def self.acos(x) = %x{
    rbMathDomain(x < -1 || x > 1, "acos")
    return Float(rbAcos(float64(x)))
  }

  #: (Float) -> Float
  def self.atan(x) = %x{ Float(rbAtan(float64(x))) }

  #: (Float, Float) -> Float
  def self.atan2(y, x) = %x{ Float(rbAtan2(float64(y), float64(x))) }

  #: (Float) -> Float
  def self.sinh(x) = %x{ Float(rbHyper(float64(x), 0)) }

  #: (Float) -> Float
  def self.cosh(x) = %x{ Float(rbHyper(float64(x), 1)) }

  #: (Float) -> Float
  def self.tanh(x) = %x{ Float(rbHyper(float64(x), 2)) }

  #: (Float) -> Float
  def self.exp(x) = %x{ Float(rbExp(float64(x))) }

  #: (Float) -> Float
  def self.log(x) = %x{
    rbMathDomain(x < 0, "log")
    return Float(rbLog(float64(x)))
  }

  #: (Float, Float) -> Float
  def self.__log_2(x, base) = %x{
    rbMathDomain(x < 0 || base < 0, "log")
    return Float(rbLogBase(float64(x), rbBigLog(rbBF().SetFloat64(float64(base)))))
  }

  #: (Float) -> Float
  def self.log2(x) = %x{
    rbMathDomain(x < 0, "log2")
    return Float(rbLogBase(float64(x), rbLn2))
  }

  #: (Float) -> Float
  def self.log10(x) = %x{
    rbMathDomain(x < 0, "log10")
    return Float(rbLogBase(float64(x), rbLn10))
  }

  #: (Float, Float) -> Float
  def self.hypot(x, y) = %x{ Float(rbHypot(float64(x), float64(y))) }

  #: (Float) -> Float
  def self.log1p(x) = %x{
    rbMathDomain(x < -1, "log1p")
    return Float(rbMathBig(float64(x), 1e-20, -1, func(b *big.Float) *big.Float { return rbBigLog(b.Add(b, rbBFInt(1))) }))
  }

  #: (Float) -> Float
  def self.expm1(x) = %x{
    if x > 710 || x < -50 {
      return Float(math.Expm1(float64(x)))
    }
    return Float(rbMathBig(float64(x), 1e-20, 0, func(b *big.Float) *big.Float { return b.Sub(rbBigExp(b), rbBFInt(1)) }))
  }

  #: (Float) -> Float
  def self.asinh(x) = %x{ Float(rbAsinh(float64(x))) }

  #: (Float) -> Float
  def self.acosh(x) = %x{
    rbMathDomain(x < 1, "acosh")
    return Float(rbMathBig(float64(x), 0, 1, func(b *big.Float) *big.Float {
      r := rbBF().Sub(rbBF().Mul(b, b), rbBFInt(1))
      return rbBigLog(r.Add(r.Sqrt(r), b))
    }))
  }

  #: (Float) -> Float
  def self.atanh(x) = %x{
    rbMathDomain(x < -1 || x > 1, "atanh")
    if x == 1 || x == -1 {
      return Float(math.Inf(int(x)))
    }
    return Float(rbMathBig(float64(x), 1e-20, -1, func(b *big.Float) *big.Float {
      q := rbBF().Quo(rbBF().Add(rbBFInt(1), b), rbBF().Sub(rbBFInt(1), b))
      return q.Quo(rbBigLog(q), rbBFInt(2))
    }))
  }

  #: (Float) -> [Float, Integer]
  def self.frexp(x) = %x{
    f, e := math.Frexp(float64(x))
    return Tuple2[Float, Integer]{Float(f), Integer(e)}
  }

  #: (Float, Integer) -> Float
  def self.ldexp(x, e) = %x{ Float(math.Ldexp(float64(x), int(e))) }

  #: (Float) -> Float
  def self.erf(x) = %x{ Float(rbErf(float64(x))) }

  #: (Float) -> Float
  def self.erfc(x) = %x{ Float(rbErfc(float64(x))) }

  #: (Float) -> Float
  def self.gamma(x) = %x{ Float(rbGamma(float64(x))) }

  #: (Float) -> [Float, Integer]
  def self.lgamma(x) = %x{
    v, sign := rbLgamma(float64(x))
    return Tuple2[Float, Integer]{Float(v), Integer(sign)}
  }
end

class Float
  INFINITY = 1.0 / 0 #: Float
  NAN = 0.0 / 0 #: Float
  EPSILON = 2.220446049250313e-16 #: Float
  MAX = 1.7976931348623157e+308 #: Float
  MIN = 2.2250738585072014e-308 #: Float
  DIG = 15 #: Integer
end
