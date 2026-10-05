# rbs_inline: enabled

# A module so its values are Go any, as a module type's are; its methods only type calls on a Numeric, each includer defining or undefining every one (decision 142).
module Numeric
  include Comparable

  #: (Numeric) -> Numeric
  def +(other) = raise(NotImplementedError)

  #: (Numeric) -> Numeric
  def -(other) = raise(NotImplementedError)

  #: (Numeric) -> Numeric
  def *(other) = raise(NotImplementedError)

  #: (Numeric) -> Numeric
  def /(other) = raise(NotImplementedError)

  #: (Numeric) -> Numeric
  def %(other) = raise(NotImplementedError)

  #: (Numeric) -> Numeric
  def **(other) = raise(NotImplementedError)

  #: () -> self
  def -@ = raise(NotImplementedError)

  #: () -> bool
  def integer? = raise(NotImplementedError)

  #: () -> bool
  def zero? = raise(NotImplementedError)

  #: () -> self?
  def nonzero? = raise(NotImplementedError)

  #: () -> bool
  def positive? = raise(NotImplementedError)

  #: () -> bool
  def negative? = raise(NotImplementedError)

  #: () -> bool
  def finite? = raise(NotImplementedError)

  #: () -> Integer?
  def infinite? = raise(NotImplementedError)

  #: () -> bool
  def real? = raise(NotImplementedError)

  #: () -> Numeric
  def abs = raise(NotImplementedError)

  #: () -> Numeric
  def magnitude = raise(NotImplementedError)

  #: (Numeric) -> Numeric
  def fdiv(other) = raise(NotImplementedError)

  #: (Numeric) -> Integer
  def div(other) = raise(NotImplementedError)

  #: (Numeric) -> Array[Numeric]
  def divmod(other) = raise(NotImplementedError)

  #: (Numeric) -> Numeric
  def modulo(other) = raise(NotImplementedError)

  #: (Numeric) -> Numeric
  def remainder(other) = raise(NotImplementedError)

  #: (Numeric) -> Numeric
  def quo(other) = raise(NotImplementedError)

  #: (Numeric) -> Array[Numeric]
  def coerce(other) = raise(NotImplementedError)

  #: () -> Complex
  def to_c = raise(NotImplementedError)

  #: () -> Complex
  def i = raise(NotImplementedError)

  #: () -> Integer
  def to_i = raise(NotImplementedError)

  #: () -> Float
  def to_f = raise(NotImplementedError)

  #: () -> Rational
  def to_r = raise(NotImplementedError)

  #: () -> Integer
  def floor = raise(NotImplementedError)

  #: () -> Integer
  def ceil = raise(NotImplementedError)

  #: () -> Integer
  def round = raise(NotImplementedError)

  #: () -> Integer
  def truncate = raise(NotImplementedError)
end
