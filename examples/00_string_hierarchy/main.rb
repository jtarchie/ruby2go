# frozen_string_literal: true
# rbs_inline: enabled
# args: --seed 1

require "minitest/autorun"

# One String value answers methods from every level of its ancestry.
class StringHierarchyTest < Minitest::Test
  S = "hello, world"

  #: () -> void
  def test_string
    assert_equal "HELLO, WORLD", S.upcase
  end

  #: () -> void
  def test_comparable
    assert S < "world"
    assert_equal "c", S.clamp("a", "c")
  end

  #: () -> void
  def test_kernel
    assert_equal S + S, S.then { |x| x + x }
  end

  #: () -> void
  def test_basic_object
    assert S.equal?(S)
    refute S.equal?(S.dup)
  end
end
