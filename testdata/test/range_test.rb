# rbs_inline: enabled

require "minitest/autorun"

module RangeTests
  # include?/member? iterate a String range; cover? and === compare.
  class RangeIncludeTest < Minitest::Test
    def test_string_include_iterates
      az = "a".."z"
      assert_equal [false, true, false, true], [az.include?("bb"), az.cover?("bb"), az.member?("bb"), az === "bb"]
      assert_equal [true, true, true], [az.include?("a"), az.include?("m"), az.include?("z")]
    end

    def test_string_include_multi_char_and_exclusive
      assert_equal [true, false, false], [("a".."zz").include?("bb"), ("aa".."zz").include?("c"), ("a"..."c").include?("c")]
      assert_equal [true, false, true], [("1".."10").include?("5"), ("1".."10").cover?("5"), ("1".."10").include?("10")]
      assert_equal [true, false], [("a".."e").include?("d"), ("a".."e").include?("ab")]
    end

    def test_string_include_endless_raises
      r = ("a"..) #: Range[String]
      cls = begin
        r.include?("b")
        nil
      rescue TypeError
        TypeError
      end
      assert_equal TypeError, cls
      assert_equal true, r.cover?("zzz")
    end

    def test_numeric_include_is_cover
      assert_equal [true, true, false], [(1.0..2.0).include?(1.5), (1..10).include?(5), (1.0..2.0).member?(2.5)]
      assert_equal [true, false], [(1..).include?(5), (1...3).member?(3)]
    end
  end

  class RangeEqTest < Minitest::Test
    def test_eq_compares_ends_with_eq
      one = 1..2
      flt = 1.0..2.0
      assert_equal [true, false, true, false], [one == flt, one.eql?(flt), one.eql?(1..2), one == (1...2)]
    end
  end
end
