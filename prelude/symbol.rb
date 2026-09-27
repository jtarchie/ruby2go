# prelude/symbol.rb
# rbs_inline: enabled
#
# Symbol as a named Go string, distinct from String.

%x{
  // MRI's identifier: an ASCII letter or `_`, or any non-ASCII character,
  // then those or ASCII digits. \\x60 is a backtick.
  const rbIdentRe = `[A-Za-z_\\x{80}-\\x{10FFFF}][\\w\\x{80}-\\x{10FFFF}]*`

  var rbPlainSymbol = regexp.MustCompile(`\\A(?:` + rbIdentRe + `[?!=]?|@@?` + rbIdentRe +
    `|\\$(?:` + rbIdentRe + `|[~*$?!@/\\\\;,.=:<>"&\\x60'+0]|-[\\w\\x{80}-\\x{10FFFF}]|[1-9]\\d*)` +
    `|\\[\\]=?|[+\\-*/%<>!~^&|\\x60]|\\*\\*|<=>|==|===|=~|!=|!~|<<|>>|<=|>=|[+\\-]@)\\z`)

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

  #: () -> String
  def to_s = %x{ String(self) }

  #: () -> String
  def name = to_s

  #: () -> Symbol
  def to_sym = self

  #: () -> String
  def inspect = %x{ rbSymbolInspect(string(self)) }

  #: () -> Integer
  def size = to_s.size

  #: () -> Integer
  def hash = to_s.hash
end

class String
  #: () -> Symbol
  def to_sym = %x{ Symbol(self) }
end
