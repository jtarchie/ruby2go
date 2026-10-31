# rbs_inline: enabled

require "minitest/autorun"
require "fiddle"
require "fiddle/import"
require "ffi"

# --cgo's Fiddle backend (decision 426): variadic calls, stack-passed arguments and double-returning closures, against MRI's Fiddle and ffi.
module CgoFiddleTests
  BREW_SQLITE = ["/opt/homebrew/opt/sqlite/lib/libsqlite3.dylib", "/usr/local/opt/sqlite/lib/libsqlite3.dylib"].find { |path| File.exist?(path) }
  SQLITE = RbConfig::CONFIG["host_os"].to_s.include?("darwin") ? BREW_SQLITE || "libsqlite3.dylib" : "libsqlite3.so.0"

  module Lib
    extend Fiddle::Importer
    dlload Fiddle.dlopen(nil)

    extern "int snprintf(char *, size_t, const char *, ...)"
  end

  module LibF
    extend FFI::Library
    ffi_lib FFI::Library::LIBC

    attach_function :snprintf, [:pointer, :size_t, :string, :varargs], :int
  end

  module Sqlite
    extend FFI::Library
    ffi_lib SQLITE

    attach_function :sqlite3_open, [:string, :pointer], :int
    attach_function :sqlite3_exec, [:pointer, :string, :pointer, :pointer, :pointer], :int
    attach_function :sqlite3_close, [:pointer], :int
    attach_function :sqlite3_table_column_metadata, %i[pointer string string string pointer pointer pointer pointer pointer], :int
  end

  class FiddleCgoTest < Minitest::Test
    #: () -> Fiddle::Function
    def fiddle_cgo_snprintf
      Fiddle::Function.new(Fiddle.dlopen(nil)["snprintf"], [Fiddle::TYPE_VOIDP, Fiddle::TYPE_SIZE_T, Fiddle::TYPE_CONST_STRING, Fiddle::TYPE_VARIADIC], Fiddle::TYPE_INT)
    end

    def test_variadic_function
      buf = Fiddle::Pointer.malloc(256, Fiddle::RUBY_FREE)
      n = fiddle_cgo_snprintf.call(buf, 256, "%d %s %.2f %ld %c", :int, -42, :const_string, "hi", :double, 2.5, :long, 1 << 40, Fiddle::TYPE_INT, 65)
      assert_equal ["-42 hi 2.50 1099511627776 A", 27], [buf.to_s, n]
      fiddle_cgo_snprintf.call(buf, 256, "%.1f|%s", "double", 1.5, "voidp", "x")
      assert_equal "1.5|x", buf.to_s
      assert_raises(RuntimeError) { fiddle_cgo_snprintf.call(buf, 256, "%f", :float, 1.5) } # libffi refuses what C promotes
    end

    def test_variadic_past_the_registers
      buf = Fiddle::Pointer.malloc(256, Fiddle::RUBY_FREE)
      ints = [] #: Array[untyped]
      (1..10).each { |i| ints << :int << i * 11 }
      fiddle_cgo_snprintf.call(buf, 256, "%d,%d,%d,%d,%d,%d,%d,%d,%d,%d", *ints)
      assert_equal "11,22,33,44,55,66,77,88,99,110", buf.to_s
      doubles = [] #: Array[untyped]
      (1..10).each { |i| doubles << :double << i + 0.5 }
      fiddle_cgo_snprintf.call(buf, 256, "%.1f %.1f %.1f %.1f %.1f %.1f %.1f %.1f %.1f %.1f %d", *doubles, :int, 7)
      assert_equal "1.5 2.5 3.5 4.5 5.5 6.5 7.5 8.5 9.5 10.5 7", buf.to_s
    end

    def test_variadic_errors
      buf = Fiddle::Pointer.malloc(16, Fiddle::RUBY_FREE)
      assert_raises(ArgumentError) { fiddle_cgo_snprintf.call(buf, 16, "%d", :int) }
      assert_raises(ArgumentError) { fiddle_cgo_snprintf.call(buf, 16) }
      assert_raises(ArgumentError) { Fiddle::Function.new(Fiddle.dlopen(nil)["snprintf"], [Fiddle::TYPE_VARIADIC, Fiddle::TYPE_INT], Fiddle::TYPE_INT) }
    end

    def test_importer_variadic
      buf = Fiddle::Pointer.malloc(64, Fiddle::RUBY_FREE)
      assert_equal 3, Lib.snprintf(buf, 64, "%s-%d", :const_string, "x", :int, 7)
      assert_equal "x-7", buf.to_s
    end

    def test_ffi_varargs
      buf = FFI::MemoryPointer.new(:char, 64)
      LibF.snprintf(buf, 64, "%s=%d %.1f %u", :string, "k", :int, 3, :double, 0.5, :uint, 9)
      assert_equal "k=3 0.5 9", buf.read_string
    end

    def test_nine_arguments
      out = FFI::MemoryPointer.new(:pointer)
      assert_equal 0, Sqlite.sqlite3_open(":memory:", out)
      db = out.read_pointer
      assert_equal 0, Sqlite.sqlite3_exec(db, "create table t(id integer primary key, name text not null)", nil, nil, nil)
      type = FFI::MemoryPointer.new(:pointer)
      coll = FFI::MemoryPointer.new(:pointer)
      flags = (1..3).map { FFI::MemoryPointer.new(:int) }
      rc = Sqlite.sqlite3_table_column_metadata(db, "main", "t", "name", type, coll, flags[0], flags[1], flags[2])
      assert_equal [0, "TEXT", "BINARY", [1, 0, 0]], [rc, type.read_pointer.read_string, coll.read_pointer.read_string, flags.map(&:read_int)]
      Sqlite.sqlite3_close(db)
    end

    def test_double_closure
      half = Fiddle::Closure::BlockCaller.new(Fiddle::TYPE_DOUBLE, [Fiddle::TYPE_DOUBLE, Fiddle::TYPE_INT]) { |x, n| x / n }
      call = Fiddle::Function.new(half, [Fiddle::TYPE_DOUBLE, Fiddle::TYPE_INT], Fiddle::TYPE_DOUBLE)
      assert_equal 1.25, call.call(5.0, 4)
    end
  end
end
