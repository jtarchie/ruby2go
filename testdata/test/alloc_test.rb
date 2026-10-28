# rbs_inline: enabled

require "minitest/autorun"

#: () { () -> untyped } -> Array[String]
def alloc_raised
  yield
  ["none", ""]
rescue Exception => e
  [e.class.to_s, e.message]
end

# Sizes are 2**62 so nothing here allocates: MRI and rugaroo both refuse up front (#101).
module AllocTests
  NOMEM = ["NoMemoryError", "failed to allocate memory"].freeze
  TOO_FEW = ["ArgumentError", "too few arguments"].freeze
  HUGE = 2**62

  class AllocTest < Minitest::Test
    def test_string_repeat
      assert_equal(NOMEM, alloc_raised { "x" * HUGE })
    end

    def test_string_repeat_ok
      assert_equal(["ArgumentError", "negative argument"], alloc_raised { "x" * -1 })
      assert_equal "", "" * HUGE
      assert_equal 100_000, ("x" * 100_000).size
    end

    def test_ljust
      assert_equal(NOMEM, alloc_raised { "x".ljust(HUGE) })
    end

    def test_rjust
      assert_equal(NOMEM, alloc_raised { "x".rjust(HUGE, "ab") })
    end

    def test_center
      assert_equal(NOMEM, alloc_raised { "x".center(HUGE) })
    end

    def test_pad_ok
      assert_equal 100_001, "x".ljust(100_001).size
      assert_equal "abababax", "x".rjust(8, "ab")
      assert_equal "ababxabab", "x".center(9, "ab")
      assert_equal "xabcab", "x".ljust(6, "abc")
    end

    def test_array_new_value
      assert_equal(["ArgumentError", "array size too big"], alloc_raised { Array.new(HUGE, 1) })
    end

    def test_array_new_block
      assert_equal(["ArgumentError", "array size too big"], alloc_raised { Array.new(HUGE) { |i| i } })
    end

    def test_array_new_ok
      assert_equal(["ArgumentError", "negative array size"], alloc_raised { Array.new(-1, 1) })
      assert_equal 100_000, Array.new(100_000, 1).size
      assert_equal 3, Array.new(3) { |i| i }.last + 1
    end

    def test_array_repeat
      assert_equal(["ArgumentError", "argument too big"], alloc_raised { [1, 2] * HUGE })
    end

    def test_array_repeat_ok
      assert_equal(["ArgumentError", "negative argument"], alloc_raised { [1, 2] * -1 })
      assert_equal [], [] * HUGE
      assert_equal 200_000, ([1, 2] * 100_000).size
      assert_equal [1, 2, 1, 2], [1, 2] * 2
    end

    def test_pack_bits
      assert_equal(NOMEM, alloc_raised { ["1"].pack("B#{HUGE}") })
    end

    def test_pack_hex
      assert_equal(NOMEM, alloc_raised { ["1"].pack("H#{HUGE}") })
    end

    def test_pack_null
      assert_equal(NOMEM, alloc_raised { [].pack("x#{HUGE}") })
    end

    def test_pack_at
      assert_equal(NOMEM, alloc_raised { [].pack("@#{HUGE}") })
    end

    def test_pack_a
      assert_equal(NOMEM, alloc_raised { ["a"].pack("a#{HUGE}") })
    end

    def test_pack_upper_a
      assert_equal(NOMEM, alloc_raised { ["a"].pack("A#{HUGE}") })
    end

    def test_pack_z
      assert_equal(NOMEM, alloc_raised { ["a"].pack("Z#{HUGE}") })
    end

    def test_pack_ok
      assert_equal(TOO_FEW, alloc_raised { [1].pack("C#{HUGE}") })
      assert_equal 100_001, [].pack("x100001").size
      assert_equal 100_000, ["a"].pack("a100000").size
      assert_equal "a  ", ["a"].pack("A3")
    end

    def test_unpack_padding_byte
      n = 2**60
      assert_equal(["IndexError", "index #{n - 1} too big"], alloc_raised { "abc".unpack("C#{n}") })
    end

    def test_unpack_padding_float
      n = 2**60
      assert_equal(["IndexError", "index #{n - 1} too big"], alloc_raised { "abc".unpack("E#{n}") })
    end

    def test_unpack_ok
      assert_equal [97, 98, 99, nil, nil], "abc".unpack("C5")
      assert_equal 1, "abc".unpack("a#{HUGE}").size
    end

    def test_strftime_width
      cls, msg = alloc_raised { Time.at(0).utc.strftime("%1073741824N") }
      assert_equal "Errno::ERANGE", cls
      assert msg.to_s.end_with?(" - %1073741824N")
    end

    def test_strftime_width_in_format
      cls, msg = alloc_raised { Time.at(0).utc.strftime("x%Y %_1073741824d y") }
      assert_equal "Errno::ERANGE", cls
      assert msg.to_s.end_with?(" - x%Y %_1073741824d y")
    end

    def test_strftime_ok
      t = Time.at(0).utc
      assert_equal "%2147483648N", t.strftime("%2147483648N")
      assert_equal "%4611686018427387904Y1970", t.strftime("%4611686018427387904Y%Y")
      assert_equal 100_000, t.strftime("%100000N").size
      assert_equal "000000000", t.strftime("%N")
    end

    def test_format_twos_width
      assert_equal(["ArgumentError", "width too big"], alloc_raised { format("%0100000000000x", -1) })
    end

    def test_format_width
      assert_equal(["ArgumentError", "width too big"], alloc_raised { format("%#{HUGE}d", 1) })
    end

    def test_format_string_width
      assert_equal(["ArgumentError", "width too big"], alloc_raised { format("%#{2**40}s", "a") })
    end

    def test_format_precision
      assert_equal(["ArgumentError", "precision too big"], alloc_raised { format("%.#{HUGE}f", 1.0) })
    end

    def test_format_string_precision
      assert_equal(["ArgumentError", "precision too big"], alloc_raised { format("%.#{HUGE}s", "a") })
    end

    def test_format_star
      assert_equal(["RangeError", "integer #{HUGE} too big to convert to 'int'"], alloc_raised { format("%*d", HUGE, 1) })
    end

    def test_format_ok
      assert_equal "..f01", format("%05x", -255)
      assert_equal "ffffffffff", format("%010x", -1).tr(".", "f")
      assert_equal 100_000, format("%100000d", 1).size
    end
  end
end
