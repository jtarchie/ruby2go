# rbs_inline: enabled

require "minitest/autorun"
require "ffi"

# FFI over Fiddle's purego path (decision 375): C calls, Pointer bytes, structs, callbacks, against the real ffi gem.
module FfiTests
  module Lib
    extend FFI::Library
    ffi_lib FFI::Library::LIBC

    typedef :int, :myint
    enum :color, [:red, :green, 5, :blue]
    callback :cmp, [:pointer, :pointer], :int

    attach_function :labs, [:long], :long
    attach_function :ffi_abs, :abs, [:myint], :myint
    attach_function :strlen, [:string], :size_t
    attach_function :atoi, [:string], :int
    attach_function :ldexp, [:double, :int], :double
    attach_function :fabsf, [:float], :float
    attach_function :toupper, [:int], :int
    attach_function :strchr, [:string, :int], :pointer
    attach_function :memcpy, [:pointer, :pointer, :size_t], :pointer
    attach_function :qsort, [:pointer, :size_t, :size_t, :cmp], :void
    attach_function :chdir, [:string], :int
    attach_function :color_id, :abs, [:color], :color

    class Pair < FFI::Struct
      layout :a, :short, :b, :long, :c, [:uchar, 3], :f, :float
    end

    class Outer < FFI::Struct
      layout :id, :int, :pair, Pair, :tag, :uint8
    end

    class Bits < FFI::Union
      layout :u, :uint, :b, [:uint8, 4]
    end
  end

  module Bare
    extend FFI::Library
  end

  class FfiTest < Minitest::Test
    def test_calls
      assert_equal [7, 3, 5, 42], [Lib.labs(-7), Lib.ffi_abs(-3), Lib.strlen("hello"), Lib.atoi("42abc")]
      assert_equal [1280.0, 2.5, 65], [Lib.ldexp(5.0, 8), Lib.fabsf(-2.5), Lib.toupper(97)]
    end

    def test_enum
      assert_equal :green, Lib.color_id(:green)
      assert_equal [0, 5, 6], [Lib.enum_value(:red), Lib.enum_value(:green), Lib.enum_value(:blue)]
    end

    def test_pointer_returns
      p = Lib.strchr("hello", "l".ord)
      assert_equal "llo", p.read_string
      assert_equal true, Lib.strchr("abc", "z".ord).null?
    end

    def test_memory_pointer
      mp = FFI::MemoryPointer.new(:int, 4)
      mp.write_array_of_int([1, -2, 3, -4])
      assert_equal [1, -2, 3, -4], mp.read_array_of_int(4)
      assert_equal -2, (mp + 4).read_int
      assert_equal 3, mp.get_int(8)
      mp.put_int(8, 99)
      assert_equal 99, mp.get_int(8)
      assert_equal 16, mp.size
      assert_equal [0, 0, 0, 0], FFI::MemoryPointer.new(:int, 4).read_array_of_int(4)
    end

    def test_scalars_and_strings
      mp = FFI::MemoryPointer.new(:char, 32)
      mp.put_uint8(0, 200)
      mp.put_int8(1, -5)
      mp.put_uint16(2, 65_535)
      mp.put_int64(8, -(2**40))
      mp.put_double(16, 1.5)
      mp.put_float(24, 0.25)
      assert_equal [200, -5, 65_535, -(2**40), 1.5, 0.25], [mp.get_uint8(0), mp.get_int8(1), mp.get_uint16(2), mp.get_int64(8), mp.get_double(16), mp.get_float(24)]
      s = FFI::MemoryPointer.from_string("pointer")
      assert_equal ["pointer", "nter", "po"], [s.read_string, (s + 3).read_string, s.read_string(2)]
      assert_equal "ab", FFI::MemoryPointer.from_string("abc").read_bytes(2)
    end

    def test_null_and_address
      assert_equal true, FFI::Pointer::NULL.null?
      assert_equal 4096, FFI::Pointer.new(4096).address
      assert_equal 4100, (FFI::Pointer.new(4096) + 4).address
      mp = FFI::MemoryPointer.new(:long)
      mp.write_pointer(FFI::Pointer.new(77))
      assert_equal 77, mp.read_pointer.address
    end

    def test_memcpy
      src = FFI::MemoryPointer.from_string("copy me")
      dst = FFI::MemoryPointer.new(:char, 16)
      Lib.memcpy(dst, src, 8)
      assert_equal "copy me", dst.read_string
    end

    def test_struct_layout
      assert_equal [24, 8, [[:a, 0], [:b, 8], [:c, 16], [:f, 20]]], [Lib::Pair.size, Lib::Pair.alignment, Lib::Pair.offsets]
      assert_equal [:id, :pair, :tag], Lib::Outer.members
      assert_equal [4, 4], [Lib::Bits.size, Lib::Bits.alignment]
      assert_equal 8, Lib::Pair.offset_of(:b)
    end

    def test_struct_fields
      pair = Lib::Pair.new
      pair[:a] = -3
      pair[:b] = 1 << 40
      pair.pointer.put_array_of_uint8(16, [7, 8, 9])
      pair[:f] = 1.5
      assert_equal [-3, 1 << 40, [7, 8, 9], 1.5], [pair[:a], pair[:b], pair[:c].to_a, pair[:f]]
      assert_equal 7, pair.pointer.get_uint8(16)
      other = Lib::Pair.new(pair.pointer)
      assert_equal -3, other[:a]
    end

    def test_nested_struct
      o = Lib::Outer.new
      o[:id] = 5
      o[:tag] = 250
      o[:pair][:a] = 11
      assert_equal [5, 250, 11], [o[:id], o[:tag], o[:pair][:a]]
    end

    def test_union
      u = Lib::Bits.new
      u[:u] = 0x01020304
      assert_equal [4, 3, 2, 1], u[:b].to_a
    end

    def test_callback
      mp = FFI::MemoryPointer.new(:int, 5)
      mp.write_array_of_int([50, 10, 40, 20, 30])
      calls = 0
      cmp = proc do |a, b|
        calls += 1
        a.read_int <=> b.read_int
      end
      Lib.qsort(mp, 5, 4, cmp)
      assert_equal [10, 20, 30, 40, 50], mp.read_array_of_int(5)
      assert_equal true, calls > 0
    end

    def test_last_error
      assert_equal -1, Lib.chdir("/nonexistent/dir/for/ffi")
      assert_equal 2, FFI.errno
      assert_equal 2, FFI::LastError.error
    end

    def test_missing
      assert_raises(FFI::NotFoundError) do
        Lib.attach_function(:no_such_function_ffi, [], :void)
      end
      begin
        Bare.ffi_lib("/nonexistent/libnope.so")
        flunk "expected LoadError"
      rescue LoadError => e
        assert_equal LoadError, e.class
      end
      assert_raises(LoadError) { Bare.attach_function(:abs, [:int], :int) }
    end
  end
end
