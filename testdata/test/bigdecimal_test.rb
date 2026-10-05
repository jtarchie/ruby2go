# rbs_inline: enabled

require "minitest/autorun"
require "bigdecimal"
require "bigdecimal/util"

# BigDecimal (decision 112): every expected value here is MRI's inspect of
# the same expression, so precision rules, rounding modes and formats are
# checked digit for digit.
module BigDecimalTests
  class BigDecimalTest < Minitest::Test
    def test_division_precision
      assert_equal "0.14285714285714285714285714285714e0", (BigDecimal("1") / BigDecimal("7")).inspect
      assert_equal "0.21428571428571428571428571428571e0", (BigDecimal("1.5") / BigDecimal("7")).inspect
      assert_equal "0.17636571428571428571428571428571e2", (BigDecimal("123.456") / BigDecimal("7")).inspect
      assert_equal "0.14285714285714285714285714285714e-5", (BigDecimal("0.00001") / BigDecimal("7")).inspect
      assert_equal "0.90000000000900000000009e0", (BigDecimal("1") / BigDecimal("1.1111111111")).inspect
      assert_equal "0.999999999900000000009999999999e-10", (BigDecimal("1") / BigDecimal("10000000001")).inspect
      assert_equal "0.17636684144620811271428571428571429e9", (BigDecimal("1234567890.123456789") / BigDecimal("7")).inspect
      assert_equal "0.2700000000000000000000000000000000000000027e1", (BigDecimal("3") / BigDecimal("1.1111111111111111111111111111111111111111")).inspect
      assert_equal "0.23333333333333333333333333333333333e20", (BigDecimal("7") / BigDecimal("0.0000000000000000003")).inspect
      assert_equal "0.25e1", (BigDecimal("10") / BigDecimal("4")).inspect
      assert_equal "0.66666666666666666666666666666667e0", (BigDecimal("2") / BigDecimal("3")).inspect
      assert_equal "-0.66666666666666666666666666666667e0", (BigDecimal("-2") / BigDecimal("3")).inspect
      assert_equal "0.33333333333333333333333333333333e0", (BigDecimal("1") / 3).inspect
      assert_equal "0.2e1", (BigDecimal("1") / 0.5).inspect
      assert_equal "0.33333e0", (BigDecimal("1").div(BigDecimal("3"), 5)).inspect
      assert_equal "0.66666666666666666666666666666667e0", (BigDecimal("2").div(BigDecimal("3"), 0)).inspect
      assert_equal "0.125e0", (BigDecimal("1").quo(BigDecimal("8"))).inspect
      assert_equal "0.300000000000000000000000000000003e1", (BigDecimal("1") / Rational(1, 3)).inspect
    end

    def test_add_sub_mult
      assert_equal "0.3e0", (BigDecimal("0.1") + BigDecimal("0.2")).inspect
      assert_equal "0.10000000000000000000000000000000000000001e21", (BigDecimal("1e20") + BigDecimal("1e-20")).inspect
      assert_equal "0.123e1", (BigDecimal("1.23456").add(BigDecimal("0"), 3)).inspect
      assert_equal "0.1235e1", (BigDecimal("1.23456").sub(BigDecimal("0.00001"), 4)).inspect
      assert_equal "0.5e1", (BigDecimal("1.5").mult(3, 1)).inspect
      assert_equal "0.125e2", (BigDecimal("12.5").add(BigDecimal("0.001"), 3)).inspect
      assert_equal "0.375e1", (BigDecimal("1.5") * 2.5).inspect
      assert_equal "0.9e0", (BigDecimal("1") - 0.1).inspect
      assert_equal "0.99999999999999999999999999999999e0", (BigDecimal("3") * Rational(1, 3)).inspect
      assert_equal "0.35e1", (2 + BigDecimal("1.5")).inspect
      assert_equal "0.5e0", (2 - BigDecimal("1.5")).inspect
      assert_equal "0.3e1", (1.5 * BigDecimal("2")).inspect
      assert_equal "-0.15e1", (-BigDecimal("1.5")).inspect
      assert_equal "0.15e1", (BigDecimal("-1.5").abs).inspect
      assert_equal "-0.0", (BigDecimal("-0") + BigDecimal("-0")).inspect
      long = BigDecimal("0.1234567890123456789012345678901234567891")
      assert_equal "0.456790122345679012234567901223456790122433333e0", (Rational(1, 3) + long).inspect
      assert_equal "0.209876544320987654432098765443209876544233333e0", (Rational(1, 3) - long).inspect
      r = Rational(1, 3) #: untyped
      assert_equal "0.86419755308641975530864197553086419755133333e-1", (r % long).inspect
    end

    def test_rounding
      assert_equal "0.12346e3", (BigDecimal("123.456").round(2)).inspect
      assert_equal "120", (BigDecimal("123.456").round(-1)).inspect
      assert_equal "123", (BigDecimal("123.456").round).inspect
      assert_equal "-3", (BigDecimal("-2.5").round).inspect
      assert_equal "1", (BigDecimal("0.5").round).inspect
      assert_equal "0.1234e3", (BigDecimal("123.456").floor(1)).inspect
      assert_equal "0.1235e3", (BigDecimal("123.456").ceil(1)).inspect
      assert_equal "0.12345e3", (BigDecimal("123.456").truncate(2)).inspect
      assert_equal "-124", (BigDecimal("-123.456").floor).inspect
      assert_equal "-123", (BigDecimal("-123.456").ceil).inspect
      assert_equal "0.12e4", (BigDecimal("1234.5").floor(-2)).inspect
      assert_equal "0.13e4", (BigDecimal("1234.5").ceil(-2)).inspect
      assert_equal "0.12e1", (BigDecimal("1.25").round(1, :half_even)).inspect
      assert_equal "0.14e1", (BigDecimal("1.35").round(1, :banker)).inspect
      assert_equal "0.12e1", (BigDecimal("1.25").round(1, :half_down)).inspect
      assert_equal "0.13e1", (BigDecimal("1.251").round(1, :half_down)).inspect
      assert_equal "0.13e1", (BigDecimal("1.21").round(1, :up)).inspect
      assert_equal "0.12e1", (BigDecimal("1.29").round(1, :down)).inspect
      assert_equal "-0.13e1", (BigDecimal("-1.21").round(1, :floor)).inspect
      assert_equal "-0.12e1", (BigDecimal("-1.29").round(1, :ceiling)).inspect
      assert_equal "0.13e1", (BigDecimal("1.25").round(1, BigDecimal::ROUND_HALF_UP)).inspect
      assert_equal "0.1e-1", (BigDecimal("0.004").round(2, :up)).inspect
      assert_equal "-0.1e-1", (BigDecimal("-0.004").round(2, :floor)).inspect
      assert_equal "0.1e2", (BigDecimal("9.99").round(1)).inspect
      assert_equal "0.1e1", (BigDecimal("1.23456").fix).inspect
      assert_equal "0.23456e0", (BigDecimal("1.23456").frac).inspect
      assert_equal "-0.23456e0", (BigDecimal("-1.23456").frac).inspect
    end

    def test_formatting
      assert_equal "\"1234567.89012345\"", (BigDecimal("1234567.89012345").to_s("F")).inspect
      assert_equal "\"1 234 567.890 123 45\"", (BigDecimal("1234567.89012345").to_s("3F")).inspect
      assert_equal "\"-123 4567.8901 2345\"", (BigDecimal("-1234567.89012345").to_s("+4F")).inspect
      assert_equal "\" 1234567.89012345\"", (BigDecimal("1234567.89012345").to_s(" F")).inspect
      assert_equal "\"0.123456789012345e7\"", (BigDecimal("1234567.89012345").to_s("E")).inspect
      assert_equal "\"0.1234 5678 9012 345e7\"", (BigDecimal("1234567.89012345").to_s(4)).inspect
      assert_equal "\"100000000000000000000.0\"", (BigDecimal("1e20").to_s("F")).inspect
      assert_equal "\"0.00000000000000000001\"", (BigDecimal("1e-20").to_s("F")).inspect
      assert_equal "\"100.0\"", (BigDecimal("100").to_s("F")).inspect
      assert_equal "\"0.0\"", (BigDecimal("0").to_s).inspect
      assert_equal "\"-0.0\"", (BigDecimal("-0").to_s("F")).inspect
      assert_equal "\"-Infinity\"", (BigDecimal("-Infinity").to_s).inspect
      assert_equal "\"0.1e-5\"", (BigDecimal("0.000001").to_s).inspect
      assert_equal "\"1.23\"", (BigDecimal("1.23").to_digits).inspect
      assert_equal "\"0.5\"", (BigDecimal("-0.5").to_digits).inspect
      assert_equal "\"0.123e1\"", (BigDecimal("1.23").inspect).inspect
    end

    def test_parsing
      assert_equal "0.15e1", (BigDecimal("  1.5  ")).inspect
      assert_equal "0.10005e4", (BigDecimal("1_000.5")).inspect
      assert_equal "0.15e4", (BigDecimal("1.5e3")).inspect
      assert_equal "0.5e0", (BigDecimal(".5")).inspect
      assert_equal "0.5e1", (BigDecimal("5.")).inspect
      assert_equal "0.15e1", (BigDecimal("+1.5")).inspect
      assert_equal "-0.15e-2", (BigDecimal("-1.5E-3")).inspect
      assert_equal "0.1e3", (BigDecimal("1d2")).inspect
      assert_equal "0.125e2", ("12.5kg".to_d).inspect
      assert_equal "0.0", ("abc".to_d).inspect
      assert_equal "-0.5e0", ("-.5".to_d).inspect
      assert_equal "0.1e1", ("1e".to_d).inspect
      assert_equal "0.1235e1", (BigDecimal(1.23456789, 4)).inspect
      assert_equal "0.1e0", (BigDecimal(0.1)).inspect
      assert_equal "0.3333333333333333e0", (BigDecimal(1.0 / 3)).inspect
      assert_equal "0.123e3", (BigDecimal(123)).inspect
      assert_equal "0.33333e0", (BigDecimal(Rational(1, 3), 5)).inspect
      assert_equal "0.15e1", (1.5.to_d).inspect
      assert_equal "0.3e1", (3.to_d).inspect
      assert_equal "0.123456e1", (BigDecimal(BigDecimal("1.23456"), 3)).inspect
    end

    def test_conversions
      assert_equal "1.23", (BigDecimal("1.23").to_f).inspect
      assert_equal "Infinity", (BigDecimal("1e400").to_f).inspect
      assert_equal "1", (BigDecimal("1.99").to_i).inspect
      assert_equal "-1", (BigDecimal("-1.99").to_i).inspect
      assert_equal "(123/100)", (BigDecimal("1.23").to_r).inspect
      assert_equal "(12000/1)", (BigDecimal("12e3").to_r).inspect
      assert_equal "[1, \"123\", 10, 1]", (BigDecimal("1.23").split).inspect
      assert_equal "[-1, \"123\", 10, -2]", (BigDecimal("-0.00123").split).inspect
      assert_equal "3", (BigDecimal("1.23").precision).inspect
      assert_equal "6", (BigDecimal("0.000123").precision).inspect
      assert_equal "5", (BigDecimal("12300").precision).inspect
      assert_equal "2", (BigDecimal("1.23").scale).inspect
      assert_equal "0", (BigDecimal("12300").scale).inspect
      assert_equal "-3", (BigDecimal("0.000123").exponent).inspect
      assert_equal "5", (BigDecimal("12300").exponent).inspect
      assert_equal "2", (BigDecimal("1.23").sign).inspect
      assert_equal "-1", (BigDecimal("-0").sign).inspect
      assert_equal "0", (BigDecimal("NaN").sign).inspect
      assert_equal "-3", (BigDecimal("-Infinity").sign).inspect
      assert_equal "3", (BigDecimal("1.23").n_significant_digits).inspect
    end

    def test_divmod_and_power
      assert_equal "[2, 0.1e1]", (BigDecimal("7").divmod(BigDecimal("3"))).inspect
      assert_equal "[-3, -0.2e1]", (BigDecimal("7").divmod(BigDecimal("-3"))).inspect
      assert_equal "[-3, 0.2e1]", (BigDecimal("-7").divmod(BigDecimal("3"))).inspect
      assert_equal "0.15e1", (BigDecimal("7.5") % 2).inspect
      assert_equal "0.2e1", (BigDecimal("-7") % BigDecimal("3")).inspect
      assert_equal "0.1e1", (BigDecimal("7").remainder(BigDecimal("-3"))).inspect
      assert_equal "3", (BigDecimal("7").div(2)).inspect
      assert_equal "-4", (BigDecimal("-7").div(2)).inspect
      assert_equal "0.121e1", (BigDecimal("1.1") ** 2).inspect
      assert_equal "0.25e0", (BigDecimal("2") ** -2).inspect
      assert_equal "0.1024e4", (BigDecimal("2") ** 10).inspect
      assert_equal "-0.8e1", (BigDecimal("-2") ** 3).inspect
      assert_equal "0.1e21", (BigDecimal("10") ** 20).inspect
      assert_equal "0.14142135623730950488016887242097e1", (BigDecimal("2") ** 0.5).inspect
      assert_equal "0.8e1", (BigDecimal("2").power(3)).inspect
      assert_equal "0.1414213562e1", (BigDecimal("2").sqrt(10)).inspect
      assert_equal "0.141e1", (BigDecimal("2").sqrt(3)).inspect
      assert_equal "0.31623e2", (BigDecimal("1000").sqrt(5)).inspect
      assert_equal "0.2e-1", (BigDecimal("0.0004").sqrt(4)).inspect
      assert_equal "0.4e1", (BigDecimal("16").sqrt(0)).inspect
      assert_equal "0.1e1", (BigDecimal("1.5") ** 0).inspect
    end

    def test_comparisons
      assert_equal "true", (BigDecimal("1.23") == BigDecimal("1.230")).inspect
      assert_equal "true", (BigDecimal("1.23") == 1.23).inspect
      assert_equal "true", (BigDecimal("2") == 2).inspect
      assert_equal "false", (BigDecimal("2") == "2").inspect
      assert_equal "true", (BigDecimal("1") < BigDecimal("2")).inspect
      assert_equal "true", (BigDecimal("1") >= 1).inspect
      assert_equal "true", (BigDecimal("1.5") > 1.25).inspect
      assert_equal "false", (BigDecimal("NaN") == BigDecimal("NaN")).inspect
      assert_equal "false", (BigDecimal("NaN") < 1).inspect
      assert_equal "-1", (BigDecimal("1.23") <=> BigDecimal("1.3")).inspect
      assert_equal "nil", (BigDecimal("1.23") <=> BigDecimal("NaN")).inspect
      assert_equal "true", (BigDecimal("1.23").eql?(BigDecimal("1.230"))).inspect
      assert_equal "true", (BigDecimal("1.23").hash == BigDecimal("1.230").hash).inspect
      assert_equal "true", (BigDecimal("0").zero?).inspect
      assert_equal "0.1e1", (BigDecimal("1").nonzero?).inspect
      assert_equal "nil", (BigDecimal("0").nonzero?).inspect
      assert_equal "1", (BigDecimal("Infinity").infinite?).inspect
      assert_equal "nil", (BigDecimal("1").infinite?).inspect
      assert_equal "false", (BigDecimal("NaN").finite?).inspect
      assert_equal "true", (BigDecimal("-1").negative?).inspect
      assert_equal "false", (BigDecimal("-0").negative?).inspect
      assert_equal "[0.1e1, 0.2e1, 0.3e1]", ([BigDecimal("3"), BigDecimal("1"), BigDecimal("2")].sort).inspect
      assert_equal "0.3e1", ([BigDecimal("3"), BigDecimal("1")].max).inspect
      assert_equal "0.375e1", ([BigDecimal("1.5"), BigDecimal("2.25")].reduce(BigDecimal("0")) { |s, x| s + x }).inspect
      assert_equal "\"a\"", ({ BigDecimal("1.0") => "a" }[BigDecimal("1")]).inspect
      assert_equal "[0.2e1, 0.123e1]", (BigDecimal("1.23").coerce(2)).inspect
      assert_equal "true", (BigDecimal("1.5").between?(1, 2)).inspect
      assert_equal "0.2e1", (BigDecimal("3").clamp(1, BigDecimal("2"))).inspect
      e = assert_raises(ArgumentError) { BigDecimal("NaN").between?(1, 2) }
      assert_equal "comparison of BigDecimal with 1 failed", e.message
      assert_raises(ArgumentError) { BigDecimal("NaN").clamp(1, 2) }
    end

    def test_errors
      e = assert_raises(ArgumentError) { BigDecimal("abc") }
      assert_equal "invalid value for BigDecimal(): \"abc\"", e.message
      e = assert_raises(ArgumentError) { BigDecimal("1e") }
      assert_equal "invalid value for BigDecimal(): \"1e\"", e.message
      e = assert_raises(ArgumentError) { BigDecimal(Rational(1, 3)) }
      assert_equal "can't omit precision for a Rational.", e.message
      e = assert_raises(ZeroDivisionError) { BigDecimal("1").divmod(0) }
      assert_equal "divided by 0", e.message
      e = assert_raises(FloatDomainError) { BigDecimal("-1").sqrt(5) }
      assert_equal "sqrt of negative value", e.message
      e = assert_raises(ArgumentError) { BigDecimal("1").round(1, :sideways) }
      assert_equal "invalid rounding mode (sideways)", e.message
      e = assert_raises(ArgumentError) { BigDecimal("1") < "2" }
      assert_equal "comparison of BigDecimal with String failed", e.message
      e = assert_raises(FloatDomainError) { BigDecimal("NaN").to_i }
      assert_equal "Computation results in 'NaN' (Not a Number)", e.message
      e = assert_raises(FloatDomainError) { BigDecimal("-Infinity").round }
      assert_equal "Computation results in '-Infinity'", e.message
    end
  end
end
