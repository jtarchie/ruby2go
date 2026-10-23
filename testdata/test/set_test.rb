# rbs_inline: enabled

require "minitest/autorun"

# Set is core in Ruby 4; results are compared as arrays because Set's order is insertion order and MRI's set algebra fixes which side's order wins.
module SetTests
  class SetAlgebraTest < Minitest::Test
    def test_union_keeps_self_then_new_elements
      a = Set[1, 2, 3, 4]
      b = Set[4, 3, 9]
      assert_equal [1, 2, 3, 4, 9], (a | b).to_a
      assert_equal [4, 3, 9, 1, 2], (b | a).to_a
      assert_equal [1, 2, 3, 5], (Set[1, 2, 3] | [5, 1]).to_a
      assert_equal [1, 2, 3, 7], (Set[1, 2, 3] + Set[7]).to_a
      assert_equal [1, 2, 3, 0], Set[1, 2, 3].union([0]).to_a
    end

    # a Set argument at most as big as self sets the order; a bigger one leaves it to self; an Array is walked as given
    def test_intersection_follows_the_smaller_side
      a = Set[1, 2, 3, 4]
      b = Set[4, 3, 9]
      assert_equal [4, 3], (a & b).to_a
      assert_equal [4, 3], (b & a).to_a
      assert_equal [3, 2, 1], (Set[1, 2, 3] & Set[3, 2, 1]).to_a
      assert_equal [1, 2, 3], (Set[1, 2, 3] & Set[3, 2, 1, 0]).to_a
      assert_equal [3, 2, 1], (Set[1, 2, 3, 0] & Set[3, 2, 1]).to_a
      assert_equal [3, 1, 2], (Set[3, 1, 2] & Set[2, 3, 1, 7, 8]).to_a
      assert_equal [5, 2], (Set[1, 2, 3, 4, 5] & Set[5, 2]).to_a
      assert_equal [5, 2], (Set[5, 2] & Set[1, 2, 3, 4, 5]).to_a
      assert_equal [2, 1], (Set[1, 2] & [2, 1]).to_a
      assert_equal [3, 2, 1], (Set[1, 2, 3] & [3, 2, 1, 3, 3, 0]).to_a
      assert_equal [4, 3], (Set[1, 2, 3, 4] & [9, 4, 3, 3]).to_a
      assert_equal [3, 1], Set[1, 2, 3].intersection([3, 1]).to_a
    end

    def test_difference_keeps_self_order
      assert_equal [1, 2], (Set[1, 2, 3, 4] - Set[4, 3, 9]).to_a
      assert_equal [9], (Set[4, 3, 9] - Set[1, 2, 3, 4]).to_a
      assert_equal [1, 3], Set[1, 2, 3].difference([2]).to_a
    end

    # self-only elements in self's order, then the argument's own
    def test_symmetric_difference
      a = Set[1, 2, 3, 4]
      b = Set[4, 3, 9]
      assert_equal [1, 2, 9], (a ^ b).to_a
      assert_equal [9, 1, 2], (b ^ a).to_a
      assert_equal [2, 3, 9, 8], (Set[1, 2, 3, 4] ^ Set[4, 9, 8, 1]).to_a
      assert_equal [2, 3], (Set[1] ^ Set[2, 1, 3]).to_a
      assert_equal [2, 9], (Set[1, 2, 3] ^ [3, 9, 1]).to_a
      assert_equal [1, 9], (Set[1] ^ [9, 9]).to_a
      assert_equal [2, 3], (Set[1, 2] ^ [3, 1, 3]).to_a
    end

    def test_subset_superset_and_comparison
      assert_equal [true, false, true, false], [Set[1, 2] <= Set[1, 2, 3], Set[1, 2] < Set[1, 2], Set[1, 2, 3] >= Set[1], Set[1, 2, 3] > Set[1, 2, 3]]
      assert_equal [true, true, true, true], [Set[1, 2].subset?(Set[1, 2, 3]), Set[1, 2, 3].superset?(Set[1]), Set[1, 2].disjoint?([3]), Set[1, 2].intersect?([2])]
      assert_equal [true, true], [Set[1, 2, 3].intersect?(Set[5, 3]), Set[1, 2, 3].disjoint?([5])]
    end

    def test_equality_ignores_order
      assert_equal [true, true, true], [Set[1, 2] == Set[2, 1], Set[1, 2].eql?(Set[2, 1]), Set[1, 2].hash == Set[2, 1].hash]
    end
  end

  class SetIterationTest < Minitest::Test
    def test_each_map_and_to_a_follow_insertion_order
      s = Set[3, 1, 2]
      seen = [] #: Array[Integer]
      s.each { |x| seen << x }
      assert_equal [3, 1, 2], seen
      assert_equal [3, 1, 2], s.to_a
      assert_equal [6, 2, 4], s.map { |x| x * 2 }
      assert_equal [1, 2, 3], s.sort
      assert_equal [3, 1], [s.first, s.min]
      assert_equal [[3, 1], [2]], s.partition(&:odd?)
      assert_equal [[3, 1], [2]], [s.select(&:odd?).to_a, s.reject(&:odd?).to_a]
    end

    def test_add_delete_and_duplicates
      s = Set[3, 1, 2]
      s.add(1)
      s << 9
      assert_equal [3, 1, 2, 9], s.to_a
      assert_nil s.add?(9)
      assert_equal [3, 1, 2, 9, 10], s.add?(10)&.to_a
      s.delete(3)
      assert_nil s.delete?(77)
      assert_equal [1, 2, 9, 10], s.to_a
      assert_equal 4, s.length
      s.delete(1)
      s << 1
      assert_equal [2, 9, 10, 1], s.to_a
    end

    def test_merge_subtract_replace_and_filters
      s = Set[1, 2, 3]
      s.merge([5, 1])
      s.merge([7])
      assert_equal [1, 2, 3, 5, 7], s.to_a
      s.subtract([1, 5])
      assert_equal [2, 3, 7], s.to_a
      s.replace([9, 8])
      assert_equal [9, 8], s.to_a
      assert_equal [true, true], [s.clear.empty?, s.empty?]
      assert_equal [2], Set[1, 2, 3].delete_if(&:odd?).to_a
      assert_equal [1, 3], Set[1, 2, 3].keep_if(&:odd?).to_a
      assert_equal [[1, 3], nil], [Set[1, 2, 3].select!(&:odd?)&.to_a, Set[1, 3].select!(&:odd?)&.to_a]
      assert_equal [[1, 3], nil], [Set[1, 2, 3].reject!(&:even?)&.to_a, Set[1].reject!(&:even?)&.to_a]
    end

    def test_merge_takes_many_enumerables_and_returns_self
      s = Set[1]
      assert_same s, s.merge([2, 3], Set[3, 4], 5..6)
      assert_equal [1, 2, 3, 4, 5, 6], s.to_a
      assert_same s, s.merge
      assert_same s, s.merge(7..8)
      assert_equal [1, 2, 3, 4, 5, 6, 7, 8], s.to_a
    end

    def test_spaceship_orders_by_inclusion
      assert_equal 0, Set[1, 2] <=> Set[2, 1]
      assert_equal(-1, Set[1] <=> Set[1, 2])
      assert_equal 1, Set[1, 2] <=> Set[1]
      assert_nil Set[1, 2] <=> Set[2, 3]
      assert_nil Set[1] <=> [1]
      assert_nil Set[1] <=> 1
    end

    def test_inspect_classify_and_divide
      assert_equal ["Set[1, 2, 3]", "Set[]", "Set[1]"], [Set[1, 2, 3].inspect, Set[].inspect, Set[1].to_s]
      assert_equal [[true, [1, 3]], [false, [2]]], Set[1, 2, 3].classify(&:odd?).map { |k, v| [k, v.to_a] }
      assert_equal [[1, 3], [2]], Set[1, 2, 3].divide(&:odd?).map(&:to_a)
      assert_equal [[1, 2], [3]], Set[1, 2, 3].each_slice(2).to_a
    end
  end
end
