# rbs_inline: enabled

require "minitest/autorun"

# Helpers for the checks that were testdata/run/string_reopen.rb.
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

#: (untyped) -> untyped
def string_reopen_ident(v) = v

module StringReopenTests
  class StringReopenTest < Minitest::Test
    def test_reopened_methods_reach_every_receiver
      assert_equal "HI!", "hi".shout
      assert_equal true, "abba".palindrome?
      assert_equal false, "abc".palindrome?
      assert_equal "hello...", "hello world".truncate(5)
      assert_equal "hi", "hi".truncate(5)
      assert_equal ["A!", "BB!"], %w[a bb].map(&:shout)
      assert_equal "X!", "x".then(&:shout)
      assert_equal "A!B!", "a b".split.map(&:shout).join
      assert_equal 5, "education".vowel_count
      assert_equal "c", "b".at_least("c")
      assert_equal "d", "d".at_least("c")
      assert_equal :c, :b.at_least(:c)
      assert_equal :ABC, :abc.up
      assert_equal [:X, :Y], %i[x y].map(&:up)
      assert_equal "BA...", "ab".reverse.shout.truncate(2)
      assert_equal false, ("x" + "y").palindrome?
      assert_equal true, "noon".strip.palindrome?
      u = string_reopen_ident("dynamo")
      assert_equal "DYNAMO!", u.shout
      assert_equal false, u.palindrome?
      assert_equal "d...", u.truncate(1)
      assert_equal 2, u.vowel_count
    end
  end
end
