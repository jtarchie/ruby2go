# rbs_inline: enabled
# args: --seed 1

require "minitest/autorun"

# Exact recipe scaling: fractions never drift the way Floats do.
class Ingredient
  attr_reader :name #: String
  attr_reader :qty #: Rational

  #: (String, Rational) -> void
  def initialize(name, qty)
    @name = name
    @qty = qty
  end

  #: (Rational) -> Ingredient
  def scale(by) = Ingredient.new(name, qty * by)

  #: () -> String
  def to_s
    whole = qty.to_i
    rest = qty - whole
    return "#{whole} #{name}" if rest.zero?
    return "#{rest} #{name}" if whole.zero?
    "#{whole} #{rest} #{name}"
  end
end

#: () -> Array[Ingredient]
def recipe
  [
    Ingredient.new("cup flour", 3/4r),
    Ingredient.new("tsp salt", Rational(1, 8)),
    Ingredient.new("eggs", 2r),
  ]
end

class RationalTest < Minitest::Test
  def test_scaling_a_recipe_stays_exact
    scaled = [Rational(1, 2), 3r, Rational(4, 3)].map do |k|
      "x#{k.inspect}: #{recipe.map { |i| i.scale(k).to_s }.join(", ")}"
    end
    assert_equal [
      "x(1/2): 3/8 cup flour, 1/16 tsp salt, 1 eggs",
      "x(3/1): 2 1/4 cup flour, 3/8 tsp salt, 6 eggs",
      "x(4/3): 1 cup flour, 1/6 tsp salt, 2 2/3 eggs",
    ], scaled
  end

  def test_sum_to_s_inspect_and_to_f
    total = recipe.map(&:qty).reduce(0r) { |a, b| a + b }
    assert_equal "23/8", total.to_s
    assert_equal "(23/8)", total.inspect
    assert_equal 2.875, total.to_f
  end

  def test_rationals_do_not_drift_like_floats
    assert_equal true, (0.1r + 0.2r) == 0.3r
    assert_equal false, (0.1 + 0.2) == 0.3
  end

  def test_mixed_arithmetic_stays_rational
    assert_equal "1/2", (1.quo(3) + 1.quo(6)).to_s
    assert_equal "5/3", (2 - Rational(1, 3)).to_s
    assert_equal "5/1", (Rational(5, 2) * 2).to_s
    assert_equal "(9/4)", (Rational(2, 3) ** -2).inspect
    assert_equal "3/4", 0.75.to_r.to_s
    assert_equal true, Rational(6, 4) == Rational(3, 2)
  end

  def test_rounding
    assert_equal 4, Rational(7, 2).round
    assert_equal(-4, Rational(-7, 2).round)
    assert_equal(-4, Rational(-7, 2).floor)
    assert_equal(-3, Rational(-7, 2).ceil)
  end

  def test_ordering_and_float_interop
    assert_equal "[(1/2), (5/8), (3/4)]", [3/4r, 1/2r, 5/8r].sort.inspect
    assert_equal 0.333, (1/3r).to_f.round(3)
    assert_equal true, Rational(1, 3) < 1
    assert_equal 0.8333333333333333, Rational(1, 3) + 0.5
  end

  def test_zero_denominator_raises
    e = assert_raises(ZeroDivisionError) { Rational(1, 0) }
    assert_equal "divided by 0", e.message
  end
end
