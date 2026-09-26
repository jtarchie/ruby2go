# rbs_inline: enabled

# User code reopens String, Symbol and Comparable; the new methods reach literals, results of
# prelude calls, &:sym blocks, then, and untyped receivers.
class String
  #: () -> String
  def shout = upcase + "!"

  #: () -> bool
  def palindrome? = self == reverse

  #: (Integer) -> String
  def truncate(n) = size > n ? (chars.first(n).join + "...") : self

  #: () -> Integer
  def vowel_count
    n = 0
    each_char { |c| n += 1 if "aeiou".include?(c) }
    n
  end
end

class Symbol
  #: () -> Symbol
  def up = to_s.upcase.to_sym
end

module Comparable
  #: (self) -> self
  def at_least(o) = self < o ? o : self
end

puts "hi".shout, "abba".palindrome?.inspect, "abc".palindrome?.inspect, "hello world".truncate(5), "hi".truncate(5)
puts %w[a bb].map(&:shout).inspect, "x".then(&:shout), "a b".split.map(&:shout).join, "education".vowel_count
puts "b".at_least("c"), "d".at_least("c"), :b.at_least(:c).inspect, :abc.up.inspect, %i[x y].map(&:up).inspect
puts "ab".reverse.shout.truncate(2), ("x" + "y").palindrome?.inspect, "noon".strip.palindrome?.inspect

#: (untyped) -> untyped
def ident(v) = v

u = ident("dynamo")
puts u.shout, u.palindrome?.inspect, u.truncate(1), u.vowel_count
