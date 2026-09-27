# prelude/boolean.rb
# rbs_inline: enabled
#
# Boolean: RBS `bool`, standing in for TrueClass/FalseClass.

# RBS `bool`; stands in for TrueClass/FalseClass.
# @go_type bool
class Boolean < Object
  #: () -> bool
  def ! = %x{ !self }

  #: (untyped) -> bool
  def ==(other) = %x{
    o, ok := other.(Boolean)
    return Boolean(ok && self == o)
  }

  #: () -> String
  def to_s = %x{ String(strconv.FormatBool(bool(self))) }

  #: () -> String
  def inspect = to_s

  # Immediates are always frozen.
  #: () -> bool
  def frozen? = true

  #: (bool) -> bool
  def &(other) = %x{ self && other }

  #: (bool) -> bool
  def |(other) = %x{ self || other }

  #: (bool) -> bool
  def ^(other) = %x{ self != other }
end
