# rbs_inline: enabled

require "minitest/autorun"
require "ffi"

# --cgo's FFI structs by value (decision 426), each signature a C stub typed from the literal layout, against MRI's ffi.
module CgoFFITests
  module LibC
    extend FFI::Library
    ffi_lib FFI::Library::LIBC

    class Div < FFI::Struct
      layout :quot, :int, :rem, :int
    end

    class LDiv < FFI::Struct
      layout :quot, :long, :rem, :long
    end

    class InAddr < FFI::Struct
      layout :s_addr, :uint32
    end

    attach_function :div, [:int, :int], Div.by_value
    attach_function :ldiv, [:long, :long], LDiv.by_value
    attach_function :inet_ntoa, [InAddr.by_value], :string
  end

  class CgoFFITest < Minitest::Test
    def test_struct_results
      d = LibC.div(17, 5)
      l = LibC.ldiv(-(1 << 40) - 3, 1 << 20)
      assert_equal [3, 2, LibC::Div, -1_048_576, -3], [d[:quot], d[:rem], d.class, l[:quot], l[:rem]]
    end

    def test_struct_argument
      a = LibC::InAddr.new
      a[:s_addr] = 0x0100007f # 127.0.0.1 in network byte order on a little-endian machine
      assert_equal "127.0.0.1", LibC.inet_ntoa(a)
    end

    def test_struct_argument_type
      assert_raises(ArgumentError) { LibC.inet_ntoa(42) }
    end
  end
end
