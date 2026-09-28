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
end

class Float
  INFINITY = 1.0 / 0 #: Float
  NAN = 0.0 / 0 #: Float
  EPSILON = 2.220446049250313e-16 #: Float
  MAX = 1.7976931348623157e+308 #: Float
  MIN = 2.2250738585072014e-308 #: Float
  DIG = 15 #: Integer
end
