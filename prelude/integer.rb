# prelude/integer.rb
# rbs_inline: enabled
#
# Integer as a named Go int. Where MRI would promote to a Bignum or a
# Rational, it raises RangeError instead (decision 35).

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
      return self.Op_eq(Opt(o))
    case *Float:
      return self.Op_eq(Opt(o))
    }
    return rbEq[any](other, self) // MRI asks other == self
  }

  #: (Integer) -> Integer
  def +(other) = %x{
    r := self + other
    if (r > self) != (other > 0) {
      rbIntOverflow(self, "+", other)
    }
    return r
  }

  #: (Integer) -> Integer
  def -(other) = %x{
    r := self - other
    if (r < self) != (other > 0) {
      rbIntOverflow(self, "-", other)
    }
    return r
  }

  # Operands within 32 bits cannot overflow; only larger ones pay for the check.
  #: (Integer) -> Integer
  def *(other) = %x{
    if (uint64(self)+1<<31)|(uint64(other)+1<<31) >= 1<<32 {
      return rbIntMul(self, other)
    }
    return self * other
  }

  # Ruby floors; Go truncates.
  #: (Integer) -> Integer
  def /(other) = %x{
    if other == 0 {
      panic(NewZeroDivisionError(Ref[String]("divided by 0")))
    }
    if other == -1 && self == math.MinInt {
      rbIntOverflow(self, "/", other)
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

  # A negative exponent is a Rational in MRI unless the base is 0 or ±1.
  #: (Integer) -> Integer
  def **(other) = %x{
    if other < 0 {
      switch self {
      case 0:
        panic(NewZeroDivisionError(Ref[String]("divided by 0")))
      case 1:
        return 1
      case -1:
        return 1 - 2*(other&1)
      }
      panic(NewRangeError(Ref(String(fmt.Sprintf("%d ** %d is a Rational (rb2go has no Rational)", self, other)))))
    }
    // Square-and-multiply; the base is squared only while bits remain, so
    // an overflowing square means the result overflows too.
    result, base, ok := Integer(1), self, true
    for e := other; ; {
      if e&1 != 0 {
        if result, ok = rbIntMulOk(result, base); !ok {
          break
        }
      }
      if e >>= 1; e == 0 {
        break
      }
      if base, ok = rbIntMulOk(base, base); !ok {
        break
      }
    }
    if !ok {
      rbIntOverflow(self, "**", other)
    }
    return result
  }

  #: () -> Integer
  def -@ = %x{
    if self == math.MinInt {
      rbIntOverflow(0, "-", self)
    }
    return -self
  }

  #: () -> Integer
  def abs = %x{
    if self < 0 {
      return self.Op_neg()
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
  #: (Integer) -> Integer
  def gcd(other) = %x{
    a, b := self, other
    if a < 0 {
      a = -a
    }
    if b < 0 {
      b = -b
    }
    for b != 0 {
      a, b = b, a%b
    }
    return a
  }

  #: (Integer) -> Integer
  def lcm(other)
    return 0 if zero? || other.zero?
    (self / gcd(other) * other).abs
  end

  # Modular exponentiation (square-and-multiply on big.Int, so no overflow).
  #: (Integer, Integer) -> Integer
  def __pow_2(exp, mod) = %x{
    if exp < 0 {
      panic(NewRangeError(Ref(String("Integer#pow() 2nd argument not allowed to be negative when 3rd argument specified"))))
    }
    if mod == 0 {
      panic(NewZeroDivisionError(Ref(String("divided by 0"))))
    }
    r := new(big.Int).Exp(big.NewInt(int64(self)), big.NewInt(int64(exp)), big.NewInt(int64(mod)))
    // big.Int.Exp's result is in [0, |mod|); Ruby's takes the modulus's sign
    if r.Sign() != 0 && mod < 0 {
      r.Add(r, big.NewInt(int64(mod)))
    }
    return Integer(r.Int64())
  }

  #: (Integer) -> Integer
  def pow(exp) = self**exp

  #: (?Integer) -> Array[Integer]
  def digits(base = 10)
    raise Math::DomainError, "out of domain" if negative?
    raise ArgumentError, "invalid radix #{base}" if base < 2
    return [0] if zero?
    out = [] #: Array[Integer]
    n = self
    while n > 0
      out << n % base
      n /= base
    end
    out
  end

  #: (Integer) -> Float
  def fdiv(other) = to_f / other

  #: (Float) -> Float
  def __fdiv_float(other) = to_f / other

  #: (Integer) -> [Integer, Integer]
  def divmod(other) = [self / other, self % other]

  #: (Integer) -> Integer
  def div(other) = self / other

  #: (Integer) -> Integer
  def modulo(other) = self % other

  #: (Integer) -> Integer
  def remainder(other) = %x{
    if other == 0 {
      panic(NewZeroDivisionError(Ref[String]("divided by 0")))
    }
    return self % other
  }

  #: () -> Integer
  def bit_length = %x{
    if self < 0 {
      return Integer(bits.Len64(uint64(^self)))
    }
    return Integer(bits.Len64(uint64(self)))
  }

  #: (Integer) -> String
  def __to_s_1(base) = %x{
    if base < 2 || base > 36 {
      panic(NewArgumentError(Ref(String("invalid radix " + strconv.Itoa(int(base))))))
    }
    return String(strconv.FormatInt(int64(self), int(base)))
  }

  #: () -> bool
  def integer? = true

  #: () -> bool
  def finite? = true

  #: () -> Integer
  def ord = self

  # Integer.sqrt: the floor of the square root, exact for any size.
  #: (Integer) -> Integer
  def self.sqrt(n) = %x{
    if n < 0 {
      panic(NewMath_DomainError(Ref(String(`Numerical argument is out of domain - "isqrt"`))))
    }
    return Integer(new(big.Int).Sqrt(big.NewInt(int64(n))).Int64())
  }

  #: (Integer, ?Integer) { (Integer) -> void } -> void
  def step(limit, by = 1) = %x{
    if by == 0 {
      panic(NewArgumentError(Ref(String("step can't be 0"))))
    }
    return func(yield func(Integer) bool) {
      for i := self; by > 0 && i <= limit || by < 0 && i >= limit; i += by {
        if !yield(i) {
          return
        }
      }
    }
  }

  #: (Integer, ?Integer) -> Array[Integer]
  def __step_enum(limit, by = 1)
    out = [] #: Array[Integer]
    step(limit, by) { |i| out << i }
    out
  end

  #: () -> Array[Integer]
  def __times_enum
    out = [] #: Array[Integer]
    times { |i| out << i }
    out
  end

  #: (Integer) -> Array[Integer]
  def __upto_enum(limit)
    out = [] #: Array[Integer]
    upto(limit) { |i| out << i }
    out
  end

  #: (Integer) -> Array[Integer]
  def __downto_enum(limit)
    out = [] #: Array[Integer]
    downto(limit) { |i| out << i }
    out
  end
end
