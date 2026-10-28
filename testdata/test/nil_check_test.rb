# rbs_inline: enabled

# nil_check: runtime

require "minitest/autorun"

# Decision 393: MRI's own error at a core operator; elsewhere rugaroo raises at the boundary where MRI fails later, so only "raised" is checked.

module NilCheckTests
  class Leaf
    attr_reader :v #: Integer

    #: (Integer) -> void
    def initialize(v)
      @v = v
    end
  end

  class Base
    #: () -> String
    def name = "base"
  end

  class Kid < Base
    #: () -> String
    def name = "kid"
  end

  class OperatorTest < Minitest::Test
    #: (Integer) -> String?
    def str(i) = i.positive? ? "s#{i}" : nil

    #: (Integer) -> Integer?
    def int(i) = i.positive? ? i : nil

    #: (Integer) -> Float?
    def flt(i) = i.positive? ? i.to_f : nil

    #: () { () -> untyped } -> [untyped, String]?
    def error_of
      yield
      nil
    rescue TypeError, ArgumentError => e
      [e.class, e.message]
    end

    def test_present_values_pass
      assert_equal "xs1", "x" + str(1)
      assert_equal 4, 3 + int(1)
      assert_equal 2.5, 1.5 + flt(1)
      assert_equal 2.0, 1 + flt(1)
      assert_equal 2, [1, 2, 3][int(1)]
      assert_equal true, int(1).positive?
    end

    def test_string
      assert_equal([TypeError, "no implicit conversion of nil into String"], error_of { "x" + str(0) })
      assert_equal([ArgumentError, "comparison of String with nil failed"], error_of { "a" < str(0) })
      assert_equal([TypeError, "no implicit conversion from nil to integer"], error_of { "ab" * int(0) })
      assert_equal([TypeError, "no implicit conversion from nil to integer"], error_of { "abc"[int(0)] })
    end

    def test_integer
      assert_equal([TypeError, "nil can't be coerced into Integer"], error_of { 1 + int(0) })
      assert_equal([TypeError, "nil can't be coerced into Integer"], error_of { 1 - int(0) })
      assert_equal([TypeError, "nil can't be coerced into Integer"], error_of { 3 * int(0) })
      assert_equal([TypeError, "nil can't be coerced into Integer"], error_of { 3 / int(0) })
      assert_equal([TypeError, "nil can't be coerced into Integer"], error_of { 3 % int(0) })
      assert_equal([TypeError, "nil can't be coerced into Integer"], error_of { 1 + flt(0) })
      assert_equal([ArgumentError, "comparison of Integer with nil failed"], error_of { 3 < int(0) })
      assert_equal([ArgumentError, "comparison of Integer with nil failed"], error_of { 3 >= int(0) })
      assert_equal([ArgumentError, "comparison of Integer with nil failed"], error_of { 5.between?(1, int(0)) })
      assert_equal([TypeError, "no implicit conversion of nil into Integer"], error_of { 1 << int(0) })
    end

    def test_float
      assert_equal([TypeError, "nil can't be coerced into Float"], error_of { 1.5 + flt(0) })
      assert_equal([TypeError, "nil can't be coerced into Float"], error_of { 1.5 * flt(0) })
      assert_equal([TypeError, "nil can't be coerced into Float"], error_of { 1.5 + int(0) })
      assert_equal([ArgumentError, "comparison of Float with nil failed"], error_of { 1.5 < flt(0) })
    end

    def test_index
      assert_equal([TypeError, "no implicit conversion from nil to integer"], error_of { [1, 2, 3][int(0)] })
    end

    def test_equality_answers
      assert_equal false, 1 == int(0)
      assert_equal true, 1 != int(0)
      assert_nil 1 <=> int(0)
      assert_nil 1.0 <=> flt(0)
      assert_nil "a" <=> str(0)
      assert_equal false, "a" == str(0)
    end
  end

  class BoundaryTest < Minitest::Test
    #: (Integer) -> Integer?
    def int(i) = i.positive? ? i : nil

    #: (Integer) -> Leaf?
    def leaf(i) = i.positive? ? Leaf.new(i) : nil

    #: (Integer) -> Base?
    def base(i) = i.positive? ? Kid.new : nil

    #: (Integer) -> Integer
    def twice(x) = x * 2

    #: (Leaf) -> Integer
    def leaf_value(l) = l.v

    #: (Base) -> String
    def base_name(b) = b.name

    #: () { () -> untyped } -> bool
    def raised?
      yield
      false
    rescue StandardError
      true
    end

    def test_present_values_pass
      assert_equal 2, twice(int(1))
      assert_equal 1, leaf_value(leaf(1))
      assert_equal "kid", base_name(base(1))
    end

    def test_nil_raises
      assert(raised? { twice(int(0)) })
      assert(raised? { leaf_value(leaf(0)) })
      assert(raised? { base_name(base(0)) })
    end
  end
end
