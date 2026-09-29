# rbs_inline: enabled
# args: --seed 1

require "minitest/autorun"

class EnumerableTest < Minitest::Test
  NUMS = [1, 2, 3, 4]

  # `&:even?` resolves to a closure at compile time.
  #: () -> void
  def test_select_with_symbol_proc
    assert_equal [2, 4], NUMS.select(&:even?)
  end

  #: () -> void
  def test_map_then_reduce
    total = NUMS.map { |n| n * 10 }.reduce(0) { |acc, n| acc + n }
    assert_equal 100, total
  end
end
