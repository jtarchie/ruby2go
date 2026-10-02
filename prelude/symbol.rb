# prelude/symbol.rb
# rbs_inline: enabled
#
# Symbol as a named Go string, distinct from String.

# @go_type string
class Symbol < Object
  include Comparable

  #: (Symbol) -> Integer
  def <=>(other) = %x{ Integer(strings.Compare(string(self), string(other))) }

  #: (untyped) -> bool
  def ==(other) = %x{
    o, ok := other.(Symbol)
    return Boolean(ok && self == o)
  }

  # A new String each call; name is the one frozen String, as in MRI.
  #: () -> String
  def to_s = %x{ rbStrClone(String(self)) }

  #: () -> String
  def name = %x{ String(self) }

  #: () -> Symbol
  def to_sym = self

  # Immediates are always frozen.
  #: () -> bool
  def frozen? = true

  #: () -> String
  def inspect = %x{ rbSymbolInspect(string(self)) }

  #: () -> Integer
  def size = name.size

  #: () -> Integer
  def hash = name.hash

  #: () -> Integer
  def length = name.size

  #: () -> bool
  def empty? = name.empty?

  #: () -> Symbol
  def upcase = name.upcase.to_sym

  #: () -> Symbol
  def downcase = name.downcase.to_sym

  #: () -> Symbol
  def capitalize = name.capitalize.to_sym

  #: () -> Symbol
  def swapcase = name.swapcase.to_sym

  #: () -> Symbol
  def succ = name.succ.to_sym

  #: () -> Symbol
  def next = succ

  #: (Symbol) -> Integer
  def casecmp(other) = name.casecmp(other.name)

  #: (Symbol) -> bool
  def casecmp?(other) = name.casecmp?(other.name)

  #: (String) -> bool
  def start_with?(prefix) = name.start_with?(prefix)

  #: (String) -> bool
  def end_with?(suffix) = name.end_with?(suffix)

  #: () -> String
  def id2name = to_s

  #: (Integer) -> String?
  def [](i) = name[i]

  #: (Integer, Integer) -> String?
  def __idx_2(start, count) = name[start, count]

  #: (Range[Integer]) -> String?
  def __idx_range(r) = name[r]
end

class String
  #: () -> Symbol
  def to_sym = %x{ Symbol(self) }
end
