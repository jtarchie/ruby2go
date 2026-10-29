# rbs_inline: enabled

# Loaded only under --int-overflow=promote (decision 372); MRI calls the value Integer.

# @go_type rbBigV
class Bignum < Object
  include Comparable

  #: (Bignum) -> Bignum
  def +(o) = %x{ rbBigAdd(self, o) }

  #: (Bignum) -> Bignum
  def -(o) = %x{ rbBigSub(self, o) }

  #: (Bignum) -> Bignum
  def *(o) = %x{ rbBigMul(self, o) }

  #: (Bignum) -> Bignum
  def /(o) = %x{
    q, _ := rbBigDivMod(self, o)
    return q
  }

  #: (Bignum) -> Bignum
  def div(o) = self / o

  #: (Bignum) -> Bignum
  def %(o) = %x{
    _, m := rbBigDivMod(self, o)
    return m
  }

  #: (Bignum) -> Bignum
  def modulo(o) = self % o

  #: (Bignum) -> [Bignum, Bignum]
  def divmod(o) = %x{
    q, m := rbBigDivMod(self, o)
    return Tuple2[Bignum, Bignum]{q, m}
  }

  #: (Bignum) -> Float
  def fdiv(o) = to_f / o.to_f

  #: (Bignum) -> Bignum
  def remainder(o) = %x{
    if o.b == nil && o.i == 0 {
      panic(NewZeroDivisionError(Ref(String("divided by 0"))))
    }
    return rbBigOp(self, o, (*big.Int).Rem)
  }

  #: (Bignum) -> Rational
  def quo(o) = to_r / o.to_r

  #: (Integer) -> Bignum
  def **(e) = %x{ rbBigPow(self, e) }

  #: (Integer) -> Bignum
  def pow(e) = self**e

  #: () -> Bignum
  def -@ = %x{ rbBigNeg(self) }

  #: () -> Bignum
  def +@ = self

  #: () -> Bignum
  def abs = %x{
    if rbBigCmp(self, Bignum{}) < 0 {
      return rbBigNeg(self)
    }
    return self
  }

  #: () -> Bignum
  def magnitude = abs

  #: (Integer) -> Bignum
  def <<(n) = %x{ rbBigShl(self, n) }

  #: (Integer) -> Bignum
  def >>(n) = %x{ rbBigShr(self, n) }

  #: (Bignum) -> Integer
  def <=>(o) = %x{ Integer(rbBigCmp(self, o)) }

  #: (Bignum) -> bool
  def <(o) = %x{ Boolean(rbBigCmp(self, o) < 0) }

  #: (Bignum) -> bool
  def <=(o) = %x{ Boolean(rbBigCmp(self, o) <= 0) }

  #: (Bignum) -> bool
  def >(o) = %x{ Boolean(rbBigCmp(self, o) > 0) }

  #: (Bignum) -> bool
  def >=(o) = %x{ Boolean(rbBigCmp(self, o) >= 0) }

  #: (untyped) -> bool
  def ==(o) = %x{
    if b, ok := rbBigAny(rbUnbox(o)); ok {
      return Boolean(rbBigCmp(self, b) == 0)
    }
    if f, ok := rbUnbox(o).(Float); ok {
      return Boolean(rbBigToF(self) == f)
    }
    return false
  }

  #: (untyped) -> bool
  def ===(o) = self == o

  #: (untyped) -> bool
  def eql?(o) = %x{
    if b, ok := rbUnbox(o).(Bignum); ok {
      return Boolean(rbBigCmp(self, b) == 0)
    }
    i, ok := rbUnbox(o).(Integer)
    return Boolean(ok && self.b == nil && self.i == i)
  }

  #: () -> Integer
  def hash = %x{
    if self.b == nil {
      return self.i
    }
    return Integer(rbKeyHash(String(self.b.String())))
  }

  #: () -> String
  def to_s = %x{ rbBigToS(self, 10) }

  #: (Integer) -> String
  def __to_s_1(base) = %x{ rbBigToS(self, base) }

  #: () -> String
  def inspect = to_s

  #: () -> Integer
  def to_i = %x{ rbBignumToInt(self) }

  #: () -> Integer
  def to_int = to_i

  #: () -> Float
  def to_f = %x{ rbBigToF(self) }

  #: () -> Rational
  def to_r = %x{
    out := &Rational{}
    out.v.SetInt(rbBigBig(self))
    return out
  }

  #: () -> Bignum
  def succ = self + 1

  #: () -> Bignum
  def pred = self - 1

  #: () -> bool
  def zero? = %x{ Boolean(self.b == nil && self.i == 0) }

  #: () -> bool
  def positive? = %x{ Boolean(rbBigCmp(self, Bignum{}) > 0) }

  #: () -> bool
  def negative? = %x{ Boolean(rbBigCmp(self, Bignum{}) < 0) }

  #: () -> bool
  def even? = %x{ Boolean(rbBigBig(self).Bit(0) == 0) }

  #: () -> bool
  def odd? = !even?

  #: () -> Integer
  def bit_length = %x{
    b := rbBigBig(self)
    if b.Sign() < 0 {
      return Integer(new(big.Int).Not(b).BitLen())
    }
    return Integer(b.BitLen())
  }

  #: () -> Array[Integer]
  def digits
    raise Math::DomainError, "out of domain" if negative?

    __digits
  end

  #: () -> Array[Integer]
  def __digits = %x{ rbBigDigits(self) }

  #: (Bignum) -> Bignum
  def &(o) = %x{ rbBigOp(self, o, (*big.Int).And) }

  #: (Bignum) -> Bignum
  def |(o) = %x{ rbBigOp(self, o, (*big.Int).Or) }

  #: (Bignum) -> Bignum
  def ^(o) = %x{ rbBigOp(self, o, (*big.Int).Xor) }

  #: () -> Bignum
  def ~ = -self - 1

  #: () -> bool
  def integer? = true

  #: () -> bool
  def frozen? = true

  #: () -> singleton(Integer)
  def class = Integer

  #: (Module) -> bool
  def is_a?(c) = c == Integer || c == Comparable || c == Numeric || c == Object || c == Kernel || c == BasicObject

  #: (Module) -> bool
  def kind_of?(c) = is_a?(c)

  #: (Module) -> bool
  def instance_of?(c) = c == Integer
end

class Integer
  #: () -> Bignum
  def __to_big = %x{ Bignum{i: self} }

  #: (Integer) -> Bignum
  def **(e) = %x{ rbBigPow(Bignum{i: self}, e) }

  #: (Integer) -> Bignum
  def pow(e) = self**e
end
