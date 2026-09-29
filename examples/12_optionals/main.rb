# rbs_inline: enabled
# args: --seed 1

require "minitest/autorun"

class Node
  attr_reader :value #: Integer
  attr_reader :next_node #: Node?

  #: (Integer, Node?) -> void
  def initialize(value, next_node)
    @value = value
    @next_node = next_node
  end

  #: () -> Integer
  def length
    n = next_node
    n ? 1 + n.length : 1
  end

  #: () -> Integer
  def sum
    total = value
    cur = next_node
    while cur
      total += cur.value
      cur = cur.next_node
    end
    total
  end
end

#: (Array[Integer], Integer) -> Integer?
def find_gt(nums, n) = nums.find { |x| x > n }

class OptionalsTest < Minitest::Test
  #: () -> Node
  def list = Node.new(1, Node.new(2, Node.new(3, nil)))

  # A `Node?` field: the ternary and `while cur` both narrow it.
  #: () -> void
  def test_narrowing_optional_field
    assert_equal 3, list.length
    assert_equal 6, list.sum
  end

  # `&.` chains stop at the first nil.
  #: () -> void
  def test_safe_navigation_chain
    assert_equal 3, list.next_node&.next_node&.value
    assert_nil list.next_node&.next_node&.next_node&.value
  end

  #: () -> void
  def test_find_returns_optional
    nums = [1, 5, 9]
    found = find_gt(nums, 4)
    assert_equal 5, found
    assert_nil find_gt(nums, 100)
    assert_equal false, found.nil?
    assert_equal true, find_gt(nums, 100).nil?
    assert_equal 10, (found ? found * 2 : -1)
    assert_equal "big", ("big" if found && found > 3)
    assert_equal "none", ("none" unless find_gt(nums, 100))
  end

  # `|| 0` turns Integer? into Integer.
  #: () -> void
  def test_or_default
    v = find_gt([1, 5, 9], 100) || 0
    assert_equal 1, v + 1
  end

  #: () -> void
  def test_nil_local_then_assigned
    nums = [1, 5, 9]
    name = nil
    assert_equal "|", name.to_s + "|"
    assert_equal "nil", name.inspect
    name = "x" if nums.size > 2
    assert_equal '"x"', name.inspect
  end

  #: () -> void
  def test_min_max_pop
    nums = [1, 5, 9]
    assert_equal 1, nums.min
    assert_equal 9, nums.max
    assert_nil [].min #: Array[Integer]
    assert_equal 9, nums.pop
    assert_equal [1, 5], nums
    assert_equal 5, nums.last
  end
end
