# rbs_inline: enabled
# args: --seed 1

require "minitest/autorun"

# Grades by score band: case/when uses Range#===.
#: (Integer) -> String
def grade(score)
  case score
  when 90..100 then "A"
  when 80...90 then "B"
  when 70...80 then "C"
  else "F"
  end
end

#: (Range[Integer]) -> Integer
def evens_in(r) = r.select(&:even?).size

class RangesTest < Minitest::Test
  def test_case_when_matches_with_range_triple_equals
    assert_equal %w[A B C F], [95, 85, 72, 10].map { |s| grade(s) }
  end

  def test_integer_range_arithmetic_and_enumerable
    r = 1..10
    assert_equal "1..10", r.to_s
    assert_equal "1..10", r.inspect
    assert_equal 55, r.sum
    assert_equal 10, r.size
    assert_equal 5, evens_in(r)
    assert_equal [1, 2, 3, 4, 5, 6, 7, 8, 9], (1...10).to_a
    assert_equal [1, 4, 9], r.map { |x| x * x }.first(3)
  end

  def test_inclusive_and_exclusive_ends
    r = 1..10
    assert_equal true, r.include?(10)
    assert_equal false, (1...10).include?(10)
    assert_equal 1, r.first
    assert_equal 10, r.last
    assert_equal 1, r.min
    assert_equal 9, (1...10).max
  end

  def test_step_and_reverse_each
    stepped = [] #: Array[Integer]
    (1..10).step(4) { |i| stepped << i }
    assert_equal [1, 5, 9], stepped

    backwards = [] #: Array[Integer]
    (1..3).reverse_each { |i| backwards << i }
    assert_equal [3, 2, 1], backwards
  end

  def test_string_ranges_use_succ
    assert_equal "a,b,c,d,e", ("a".."e").to_a.join(",")
    assert_equal [], ("y".."ab").to_a
  end

  def test_array_slicing_with_ranges
    words = %w[zero one two three four five]
    assert_equal ["one", "two", "three"], words[1..3]
    assert_equal ["two", "three", "four", "five"], words[2..]
    assert_equal ["four", "five"], words[-2..]
    assert_equal ["one", "two"], words[1, 2]
    assert_equal "zero", words.first
    assert_equal ["four", "five"], words.last(2)
  end

  def test_string_slicing_with_ranges
    s = "hello, world"
    assert_equal "hello", s[0..4]
    assert_equal "world", s[7..]
    assert_equal "hello", s[0...-7]
    assert_equal "lo", s[3, 2]
    assert_nil s[50..]
  end

  def test_endless_ranges
    seen = [] #: Array[Integer]
    (1..).each do |i|
      break if i > 3
      seen << i
    end
    assert_equal [1, 2, 3], seen
    assert_equal [1, 2, 3, 4], (1..).first(4)
  end

  def test_range_equality
    assert_equal true, (1..3) == (1..3)
    assert_equal false, (1..3) == (1...3)
  end
end
