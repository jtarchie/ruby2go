# rbs_inline: enabled
# args: --seed 1
# Multiple assignment from tuples and array literals.

require "minitest/autorun"

#: (Integer, Integer) -> [Integer, Integer]
def divmod2(a, b) = [a / b, a % b]

class MultipleAssignTest < Minitest::Test
  #: () -> void
  def test_from_tuple_return
    q, r = divmod2(17, 5)
    assert_equal 3, q
    assert_equal 2, r
  end

  # Each target takes its element's own type.
  #: () -> void
  def test_from_array_literal
    status, headers, body = [200, { "A" => "b" }, "ok"]
    assert_equal 200, status
    assert_equal({ "A" => "b" }, headers)
    assert_equal "ok", body
  end

  #: () -> void
  def test_swap
    x = 1
    y = 2
    x, y = y, x
    assert_equal [2, 1], [x, y]
  end
end
