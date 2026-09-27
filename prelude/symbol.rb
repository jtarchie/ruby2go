# prelude/symbol.rb
# rbs_inline: enabled
#
# Symbol as a named Go string, distinct from String.

%x{
  var rbPlainSymbol = regexp.MustCompile(`\\A(?:[A-Za-z_][A-Za-z0-9_]*[?!=]?|\\[\\]=?|[+\\-*/%<>!~^&|]|\\*\\*|<=>|==|===|=~|!=|!~|<<|>>|<=|>=|[+\\-]@)\\z`)

  func rbSymbolInspect(s string) String {
    if rbPlainSymbol.MatchString(s) {
      return String(":" + s)
    }
    return ":" + rbStringInspect(s)
  }
}

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
end

class String
  #: () -> Symbol
  def to_sym = %x{ Symbol(self) }
end
