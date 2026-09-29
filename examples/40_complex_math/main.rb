# rbs_inline: enabled
# args: --seed 1

require "minitest/autorun"

# Roots of a quadratic, real or complex: Complex parts keep their class.
#: (Float, Float, Float) -> Array[Complex]
def roots(a, b, c)
  disc = b * b - 4 * a * c
  if disc >= 0
    s = Math.sqrt(disc)
    return [Complex((-b + s) / (2 * a)), Complex((-b - s) / (2 * a))]
  end
  s = Math.sqrt(-disc)
  [Complex(-b / (2 * a), s / (2 * a)), Complex(-b / (2 * a), -s / (2 * a))]
end

class ComplexTest < Minitest::Test
  Z = 3 + 4i
  W = Complex(1, -2)

  def test_quadratic_roots_real_and_complex
    assert_equal "[(2.0+0i), (1.0+0i)]", roots(1.0, -3.0, 2.0).inspect
    assert_equal "[(-1.0+2.0i), (-1.0-2.0i)]", roots(1.0, 2.0, 5.0).inspect
  end

  def test_to_s_inspect_and_magnitude
    assert_equal "3+4i", Z.to_s
    assert_equal "(1-2i)", W.inspect
    assert_equal 5.0, Z.abs
    assert_equal 25, Z.abs2
    assert_equal "3-4i", Z.conjugate.to_s
  end

  def test_arithmetic_keeps_integer_and_rational_parts
    assert_equal "4+2i", (Z + W).to_s
    assert_equal "2+6i", (Z - W).to_s
    assert_equal "11-2i", (Z * W).to_s
    assert_equal "(-1+2i)", (Z / W).inspect
    assert_equal "(-7+24i)", (Z ** 2).inspect
    assert_equal "((1/5)+(2/5)*i)", (W ** -1).inspect
  end

  def test_mixing_with_other_numerics
    assert_equal "(6+8i)", (Z * 2).inspect
    assert_equal "(2.5-5.0i)", (2.5 * W).inspect
    assert_equal "((3/2)+2i)", (Z / 2).inspect
    assert_equal "((7/2)+4i)", (Z + Rational(1, 2)).inspect
  end

  def test_parts_and_equality
    assert_equal 3, Z.real
    assert_equal 4, Z.imaginary
    assert_equal [3, 4], Z.rectangular
    assert_equal true, Z == Complex(3, 4)
    assert_equal true, Complex(5, 0) == 5
  end

  def test_polar_and_conversion
    assert_equal "(-2+0.0i)", Complex.polar(2, Math::PI).inspect
    assert_equal true, Complex(0, 1).arg == Math::PI / 2
    assert_equal "(-1+0i)", (1i ** 2).inspect
    assert_equal 1.5, Complex(1.5, 0).to_f
  end

  def test_to_i_with_an_imaginary_part_raises
    e = assert_raises(RangeError) { Z.to_i }
    assert_equal "can't convert 3+4i into Integer", e.message
  end
end

class MathTest < Minitest::Test
  def test_roots_and_logs
    assert_equal 1.4142135623730951, Math.sqrt(2)
    assert_equal 3.0, Math.cbrt(27)
    assert_equal 13.0, Math.hypot(5, 12)
    assert_equal 1.0, Math.log(Math::E)
    assert_equal 10.0, Math.log(1024, 2)
    assert_equal(-3.0, Math.log10(0.001))
  end

  def test_trig_is_ieee_exact
    assert_equal 1.2246467991473532e-16, Math.sin(Math::PI)
    assert_equal 6.123233995736766e-17, Math.cos(Math::PI / 2)
    assert_equal true, Math.atan2(1, 1) * 4 == Math::PI
    assert_equal true, Math.exp(1) == Math::E
  end

  def test_float_constants
    assert_equal true, Float::INFINITY > 1e308
    assert_equal false, Float::NAN == Float::NAN
    assert_equal 2.220446049250313e-16, Float::EPSILON
  end

  def test_domain_errors_raise
    e = assert_raises(Math::DomainError) { Math.sqrt(-1) }
    assert_equal "Numerical argument is out of domain - sqrt", e.message
  end
end
