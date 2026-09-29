# rbs_inline: enabled
# args: --seed 1
# Ruby 4.0: a continuation line may start with `&&` / `||`.

require "minitest/autorun"

#: (Integer) -> bool
def teen?(n)
  n >= 13
    && n <= 19
end

#: (Integer) -> String
def describe(n)
  small = n < 10
    || n == 10
  kind = small ? "small" : "big"
  "#{n} is #{kind}"
end

class Ruby4SyntaxTest < Minitest::Test
  #: () -> void
  def test_leading_and
    assert_equal true, teen?(15)
    assert_equal false, teen?(20)
  end

  #: () -> void
  def test_leading_or
    assert_equal ["3 is small", "10 is small", "42 is big"], [3, 10, 42].map { |n| describe(n) }
  end

  # `it` names a block's single parameter.
  #: () -> void
  def test_it_parameter
    words = ["it", "works"]
    assert_equal "IT WORKS", words.map { it.upcase }.join(" ")
  end
end
