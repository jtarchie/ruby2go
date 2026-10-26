require_relative "fiddle"

# ffi_c is a C extension and attach_function evals methods, so a literal attach_function is compiled instead (decision 375).
module FFI
  VERSION = "1.17.3"

  CURRENT_PROCESS = Object.new #: Object
  USE_THIS_PROCESS_AS_LIBRARY = CURRENT_PROCESS #: Object

  class NotFoundError < LoadError
    #: (String, Array[String]) -> void
    def initialize(function, libraries)
      super("Function '#{function}' not found in [#{libraries.empty? ? "current process" : libraries.join(", ")}]")
    end
  end

  # FFI type name => Fiddle TYPE_* code (LP64: long and long long are both 8 bytes).
  TYPES = {
    void: Fiddle::TYPE_VOID,
    bool: Fiddle::TYPE_BOOL,
    char: Fiddle::TYPE_CHAR, uchar: Fiddle::TYPE_UCHAR,
    int8: Fiddle::TYPE_CHAR, uint8: Fiddle::TYPE_UCHAR,
    short: Fiddle::TYPE_SHORT, ushort: Fiddle::TYPE_USHORT,
    int16: Fiddle::TYPE_SHORT, uint16: Fiddle::TYPE_USHORT,
    int: Fiddle::TYPE_INT, uint: Fiddle::TYPE_UINT,
    int32: Fiddle::TYPE_INT, uint32: Fiddle::TYPE_UINT,
    long: Fiddle::TYPE_LONG, ulong: Fiddle::TYPE_ULONG,
    int64: Fiddle::TYPE_LONG, uint64: Fiddle::TYPE_ULONG,
    long_long: Fiddle::TYPE_LONG, ulong_long: Fiddle::TYPE_ULONG,
    size_t: Fiddle::TYPE_SIZE_T, ssize_t: Fiddle::TYPE_SSIZE_T,
    float: Fiddle::TYPE_FLOAT, double: Fiddle::TYPE_DOUBLE,
    string: Fiddle::TYPE_CONST_STRING, pointer: Fiddle::TYPE_VOIDP,
    buffer_in: Fiddle::TYPE_VOIDP, buffer_out: Fiddle::TYPE_VOIDP, buffer_inout: Fiddle::TYPE_VOIDP,
  } #: Hash[Symbol, Integer]

  SIZES = {
    bool: 1, char: 1, uchar: 1, int8: 1, uint8: 1, short: 2, ushort: 2, int16: 2, uint16: 2,
    int: 4, uint: 4, int32: 4, uint32: 4, float: 4,
    long: 8, ulong: 8, int64: 8, uint64: 8, long_long: 8, ulong_long: 8, size_t: 8, ssize_t: 8,
    double: 8, pointer: 8, string: 8,
  } #: Hash[Symbol, Integer]

  #: (Symbol) -> Integer
  def self.type_size(type)
    SIZES.fetch(type) { raise TypeError, "unable to resolve type '#{type}'" }
  end

  #: (untyped) -> untyped
  def self.__ptr(x)
    x.is_a?(Struct) ? x.pointer : x
  end

  #: () -> Integer
  def self.errno = LastError.error

  module Platform
    OS = RUBY_PLATFORM.include?("darwin") ? "darwin" : "linux" #: String
    ARCH = RUBY_PLATFORM.start_with?("arm64", "aarch64") ? "aarch64" : "x86_64" #: String
    IS_LINUX = OS == "linux" #: bool
    IS_MAC = OS == "darwin" #: bool
    IS_WINDOWS = false #: bool
    LONG_SIZE = 64 #: Integer
    ADDRESS_SIZE = 64 #: Integer
    LIBC = RUBY_PLATFORM.include?("darwin") ? "libSystem.B.dylib" : "libc.so.6" #: String
  end

  module LastError
    #: () -> Integer
    def self.error = Fiddle.last_error || 0

    #: (Integer) -> void
    def self.error=(n)
      Fiddle.last_error = n
    end
  end

  class Pointer < Fiddle::Pointer
    #: (untyped, ?untyped) -> void
    def initialize(a, b = nil)
      super(b.nil? ? a : b)
    end

    #: (untyped) -> Pointer
    def self.__wrap(p) = new(p)

    #: () -> Integer
    def address = to_i

    #: (Integer) -> Pointer
    def +(n) = Pointer.new(to_i + n)

    #: (Integer) -> Pointer
    def -(n) = Pointer.new(to_i - n)

    #: (Integer, ?Integer?) -> Pointer
    def slice(off, len = nil) = Pointer.new(to_i + off)

    #: () -> String
    def inspect = format("#<FFI::Pointer address=0x%016x>", to_i)

    #: (?Integer?) -> String
    def read_string(len = nil) = get_string(0, len)

    #: (Integer, ?Integer?) -> String
    def get_string(off, len = nil)
      raise Fiddle::DLError, "NULL pointer dereference" if null?

      len.nil? ? Fiddle.__cstring(to_i + off) : Fiddle::Pointer.read(to_i + off, len)
    end

    #: (Integer) -> String
    def read_bytes(len) = get_bytes(0, len)

    #: (Integer, Integer) -> String
    def get_bytes(off, len)
      raise Fiddle::DLError, "NULL pointer dereference" if null?

      Fiddle::Pointer.read(to_i + off, len)
    end

    #: (Integer, String) -> void
    def put_string(off, str)
      Fiddle::Pointer.write(to_i + off, str + "\0")
    end

    #: (String, ?Integer?) -> void
    def write_string(str, len = nil)
      Fiddle::Pointer.write(to_i, len.nil? ? str : str.byteslice(0, len).to_s)
    end

    #: (String) -> void
    def write_bytes(str)
      Fiddle::Pointer.write(to_i, str)
    end

    #: (Integer, String) -> void
    def put_bytes(off, str)
      Fiddle::Pointer.write(to_i + off, str)
    end

    #: (Integer) -> Pointer
    def get_pointer(off) = Pointer.new(Fiddle.__word(to_i + off))

    #: () -> Pointer
    def read_pointer = get_pointer(0)

    #: (Integer, untyped) -> void
    def put_pointer(off, p)
      Fiddle.__set_i(to_i + off, 5, Fiddle.__addr(p))
    end

    #: (untyped) -> void
    def write_pointer(p)
      put_pointer(0, p)
    end

    #: (Integer) -> bool
    def get_bool(off) = get_uint8(off) != 0

    #: () -> bool
    def read_bool = get_bool(0)

    #: (Integer, bool) -> void
    def put_bool(off, v)
      put_uint8(off, v ? 1 : 0)
    end

    #: (bool) -> void
    def write_bool(v)
      put_bool(0, v)
    end

    #: (Integer) -> Integer
    def get_char(off) = Fiddle.__get_i(to_i + off, 2)

    #: (Integer, Integer) -> void
    def put_char(off, v)
      Fiddle.__set_i(to_i + off, 2, v)
    end

    #: () -> Integer
    def read_char = get_char(0)

    #: (Integer) -> void
    def write_char(v)
      put_char(0, v)
    end

    #: (Integer) -> Array[Integer]
    def read_array_of_char(n) = get_array_of_char(0, n)

    #: (Integer, Integer) -> Array[Integer]
    def get_array_of_char(off, n)
      Array.new(n) { |i| get_char(off + i * 1) }
    end

    #: (Array[Integer]) -> void
    def write_array_of_char(a)
      put_array_of_char(0, a)
    end

    #: (Integer, Array[Integer]) -> void
    def put_array_of_char(off, a)
      a.each_with_index { |v, i| put_char(off + i * 1, v) }
    end

    #: (Integer) -> Integer
    def get_uchar(off) = Fiddle.__get_i(to_i + off, -2)

    #: (Integer, Integer) -> void
    def put_uchar(off, v)
      Fiddle.__set_i(to_i + off, -2, v)
    end

    #: () -> Integer
    def read_uchar = get_uchar(0)

    #: (Integer) -> void
    def write_uchar(v)
      put_uchar(0, v)
    end

    #: (Integer) -> Array[Integer]
    def read_array_of_uchar(n) = get_array_of_uchar(0, n)

    #: (Integer, Integer) -> Array[Integer]
    def get_array_of_uchar(off, n)
      Array.new(n) { |i| get_uchar(off + i * 1) }
    end

    #: (Array[Integer]) -> void
    def write_array_of_uchar(a)
      put_array_of_uchar(0, a)
    end

    #: (Integer, Array[Integer]) -> void
    def put_array_of_uchar(off, a)
      a.each_with_index { |v, i| put_uchar(off + i * 1, v) }
    end

    #: (Integer) -> Integer
    def get_short(off) = Fiddle.__get_i(to_i + off, 3)

    #: (Integer, Integer) -> void
    def put_short(off, v)
      Fiddle.__set_i(to_i + off, 3, v)
    end

    #: () -> Integer
    def read_short = get_short(0)

    #: (Integer) -> void
    def write_short(v)
      put_short(0, v)
    end

    #: (Integer) -> Array[Integer]
    def read_array_of_short(n) = get_array_of_short(0, n)

    #: (Integer, Integer) -> Array[Integer]
    def get_array_of_short(off, n)
      Array.new(n) { |i| get_short(off + i * 2) }
    end

    #: (Array[Integer]) -> void
    def write_array_of_short(a)
      put_array_of_short(0, a)
    end

    #: (Integer, Array[Integer]) -> void
    def put_array_of_short(off, a)
      a.each_with_index { |v, i| put_short(off + i * 2, v) }
    end

    #: (Integer) -> Integer
    def get_ushort(off) = Fiddle.__get_i(to_i + off, -3)

    #: (Integer, Integer) -> void
    def put_ushort(off, v)
      Fiddle.__set_i(to_i + off, -3, v)
    end

    #: () -> Integer
    def read_ushort = get_ushort(0)

    #: (Integer) -> void
    def write_ushort(v)
      put_ushort(0, v)
    end

    #: (Integer) -> Array[Integer]
    def read_array_of_ushort(n) = get_array_of_ushort(0, n)

    #: (Integer, Integer) -> Array[Integer]
    def get_array_of_ushort(off, n)
      Array.new(n) { |i| get_ushort(off + i * 2) }
    end

    #: (Array[Integer]) -> void
    def write_array_of_ushort(a)
      put_array_of_ushort(0, a)
    end

    #: (Integer, Array[Integer]) -> void
    def put_array_of_ushort(off, a)
      a.each_with_index { |v, i| put_ushort(off + i * 2, v) }
    end

    #: (Integer) -> Integer
    def get_int(off) = Fiddle.__get_i(to_i + off, 4)

    #: (Integer, Integer) -> void
    def put_int(off, v)
      Fiddle.__set_i(to_i + off, 4, v)
    end

    #: () -> Integer
    def read_int = get_int(0)

    #: (Integer) -> void
    def write_int(v)
      put_int(0, v)
    end

    #: (Integer) -> Array[Integer]
    def read_array_of_int(n) = get_array_of_int(0, n)

    #: (Integer, Integer) -> Array[Integer]
    def get_array_of_int(off, n)
      Array.new(n) { |i| get_int(off + i * 4) }
    end

    #: (Array[Integer]) -> void
    def write_array_of_int(a)
      put_array_of_int(0, a)
    end

    #: (Integer, Array[Integer]) -> void
    def put_array_of_int(off, a)
      a.each_with_index { |v, i| put_int(off + i * 4, v) }
    end

    #: (Integer) -> Integer
    def get_uint(off) = Fiddle.__get_i(to_i + off, -4)

    #: (Integer, Integer) -> void
    def put_uint(off, v)
      Fiddle.__set_i(to_i + off, -4, v)
    end

    #: () -> Integer
    def read_uint = get_uint(0)

    #: (Integer) -> void
    def write_uint(v)
      put_uint(0, v)
    end

    #: (Integer) -> Array[Integer]
    def read_array_of_uint(n) = get_array_of_uint(0, n)

    #: (Integer, Integer) -> Array[Integer]
    def get_array_of_uint(off, n)
      Array.new(n) { |i| get_uint(off + i * 4) }
    end

    #: (Array[Integer]) -> void
    def write_array_of_uint(a)
      put_array_of_uint(0, a)
    end

    #: (Integer, Array[Integer]) -> void
    def put_array_of_uint(off, a)
      a.each_with_index { |v, i| put_uint(off + i * 4, v) }
    end

    #: (Integer) -> Integer
    def get_long(off) = Fiddle.__get_i(to_i + off, 5)

    #: (Integer, Integer) -> void
    def put_long(off, v)
      Fiddle.__set_i(to_i + off, 5, v)
    end

    #: () -> Integer
    def read_long = get_long(0)

    #: (Integer) -> void
    def write_long(v)
      put_long(0, v)
    end

    #: (Integer) -> Array[Integer]
    def read_array_of_long(n) = get_array_of_long(0, n)

    #: (Integer, Integer) -> Array[Integer]
    def get_array_of_long(off, n)
      Array.new(n) { |i| get_long(off + i * 8) }
    end

    #: (Array[Integer]) -> void
    def write_array_of_long(a)
      put_array_of_long(0, a)
    end

    #: (Integer, Array[Integer]) -> void
    def put_array_of_long(off, a)
      a.each_with_index { |v, i| put_long(off + i * 8, v) }
    end

    #: (Integer) -> Integer
    def get_ulong(off) = Fiddle.__get_i(to_i + off, -5)

    #: (Integer, Integer) -> void
    def put_ulong(off, v)
      Fiddle.__set_i(to_i + off, -5, v)
    end

    #: () -> Integer
    def read_ulong = get_ulong(0)

    #: (Integer) -> void
    def write_ulong(v)
      put_ulong(0, v)
    end

    #: (Integer) -> Array[Integer]
    def read_array_of_ulong(n) = get_array_of_ulong(0, n)

    #: (Integer, Integer) -> Array[Integer]
    def get_array_of_ulong(off, n)
      Array.new(n) { |i| get_ulong(off + i * 8) }
    end

    #: (Array[Integer]) -> void
    def write_array_of_ulong(a)
      put_array_of_ulong(0, a)
    end

    #: (Integer, Array[Integer]) -> void
    def put_array_of_ulong(off, a)
      a.each_with_index { |v, i| put_ulong(off + i * 8, v) }
    end

    #: (Integer) -> Integer
    def get_long_long(off) = Fiddle.__get_i(to_i + off, 6)

    #: (Integer, Integer) -> void
    def put_long_long(off, v)
      Fiddle.__set_i(to_i + off, 6, v)
    end

    #: () -> Integer
    def read_long_long = get_long_long(0)

    #: (Integer) -> void
    def write_long_long(v)
      put_long_long(0, v)
    end

    #: (Integer) -> Array[Integer]
    def read_array_of_long_long(n) = get_array_of_long_long(0, n)

    #: (Integer, Integer) -> Array[Integer]
    def get_array_of_long_long(off, n)
      Array.new(n) { |i| get_long_long(off + i * 8) }
    end

    #: (Array[Integer]) -> void
    def write_array_of_long_long(a)
      put_array_of_long_long(0, a)
    end

    #: (Integer, Array[Integer]) -> void
    def put_array_of_long_long(off, a)
      a.each_with_index { |v, i| put_long_long(off + i * 8, v) }
    end

    #: (Integer) -> Integer
    def get_ulong_long(off) = Fiddle.__get_i(to_i + off, -6)

    #: (Integer, Integer) -> void
    def put_ulong_long(off, v)
      Fiddle.__set_i(to_i + off, -6, v)
    end

    #: () -> Integer
    def read_ulong_long = get_ulong_long(0)

    #: (Integer) -> void
    def write_ulong_long(v)
      put_ulong_long(0, v)
    end

    #: (Integer) -> Array[Integer]
    def read_array_of_ulong_long(n) = get_array_of_ulong_long(0, n)

    #: (Integer, Integer) -> Array[Integer]
    def get_array_of_ulong_long(off, n)
      Array.new(n) { |i| get_ulong_long(off + i * 8) }
    end

    #: (Array[Integer]) -> void
    def write_array_of_ulong_long(a)
      put_array_of_ulong_long(0, a)
    end

    #: (Integer, Array[Integer]) -> void
    def put_array_of_ulong_long(off, a)
      a.each_with_index { |v, i| put_ulong_long(off + i * 8, v) }
    end

    #: (Integer) -> Integer
    def get_int8(off) = Fiddle.__get_i(to_i + off, 2)

    #: (Integer, Integer) -> void
    def put_int8(off, v)
      Fiddle.__set_i(to_i + off, 2, v)
    end

    #: () -> Integer
    def read_int8 = get_int8(0)

    #: (Integer) -> void
    def write_int8(v)
      put_int8(0, v)
    end

    #: (Integer) -> Array[Integer]
    def read_array_of_int8(n) = get_array_of_int8(0, n)

    #: (Integer, Integer) -> Array[Integer]
    def get_array_of_int8(off, n)
      Array.new(n) { |i| get_int8(off + i * 1) }
    end

    #: (Array[Integer]) -> void
    def write_array_of_int8(a)
      put_array_of_int8(0, a)
    end

    #: (Integer, Array[Integer]) -> void
    def put_array_of_int8(off, a)
      a.each_with_index { |v, i| put_int8(off + i * 1, v) }
    end

    #: (Integer) -> Integer
    def get_uint8(off) = Fiddle.__get_i(to_i + off, -2)

    #: (Integer, Integer) -> void
    def put_uint8(off, v)
      Fiddle.__set_i(to_i + off, -2, v)
    end

    #: () -> Integer
    def read_uint8 = get_uint8(0)

    #: (Integer) -> void
    def write_uint8(v)
      put_uint8(0, v)
    end

    #: (Integer) -> Array[Integer]
    def read_array_of_uint8(n) = get_array_of_uint8(0, n)

    #: (Integer, Integer) -> Array[Integer]
    def get_array_of_uint8(off, n)
      Array.new(n) { |i| get_uint8(off + i * 1) }
    end

    #: (Array[Integer]) -> void
    def write_array_of_uint8(a)
      put_array_of_uint8(0, a)
    end

    #: (Integer, Array[Integer]) -> void
    def put_array_of_uint8(off, a)
      a.each_with_index { |v, i| put_uint8(off + i * 1, v) }
    end

    #: (Integer) -> Integer
    def get_int16(off) = Fiddle.__get_i(to_i + off, 3)

    #: (Integer, Integer) -> void
    def put_int16(off, v)
      Fiddle.__set_i(to_i + off, 3, v)
    end

    #: () -> Integer
    def read_int16 = get_int16(0)

    #: (Integer) -> void
    def write_int16(v)
      put_int16(0, v)
    end

    #: (Integer) -> Array[Integer]
    def read_array_of_int16(n) = get_array_of_int16(0, n)

    #: (Integer, Integer) -> Array[Integer]
    def get_array_of_int16(off, n)
      Array.new(n) { |i| get_int16(off + i * 2) }
    end

    #: (Array[Integer]) -> void
    def write_array_of_int16(a)
      put_array_of_int16(0, a)
    end

    #: (Integer, Array[Integer]) -> void
    def put_array_of_int16(off, a)
      a.each_with_index { |v, i| put_int16(off + i * 2, v) }
    end

    #: (Integer) -> Integer
    def get_uint16(off) = Fiddle.__get_i(to_i + off, -3)

    #: (Integer, Integer) -> void
    def put_uint16(off, v)
      Fiddle.__set_i(to_i + off, -3, v)
    end

    #: () -> Integer
    def read_uint16 = get_uint16(0)

    #: (Integer) -> void
    def write_uint16(v)
      put_uint16(0, v)
    end

    #: (Integer) -> Array[Integer]
    def read_array_of_uint16(n) = get_array_of_uint16(0, n)

    #: (Integer, Integer) -> Array[Integer]
    def get_array_of_uint16(off, n)
      Array.new(n) { |i| get_uint16(off + i * 2) }
    end

    #: (Array[Integer]) -> void
    def write_array_of_uint16(a)
      put_array_of_uint16(0, a)
    end

    #: (Integer, Array[Integer]) -> void
    def put_array_of_uint16(off, a)
      a.each_with_index { |v, i| put_uint16(off + i * 2, v) }
    end

    #: (Integer) -> Integer
    def get_int32(off) = Fiddle.__get_i(to_i + off, 4)

    #: (Integer, Integer) -> void
    def put_int32(off, v)
      Fiddle.__set_i(to_i + off, 4, v)
    end

    #: () -> Integer
    def read_int32 = get_int32(0)

    #: (Integer) -> void
    def write_int32(v)
      put_int32(0, v)
    end

    #: (Integer) -> Array[Integer]
    def read_array_of_int32(n) = get_array_of_int32(0, n)

    #: (Integer, Integer) -> Array[Integer]
    def get_array_of_int32(off, n)
      Array.new(n) { |i| get_int32(off + i * 4) }
    end

    #: (Array[Integer]) -> void
    def write_array_of_int32(a)
      put_array_of_int32(0, a)
    end

    #: (Integer, Array[Integer]) -> void
    def put_array_of_int32(off, a)
      a.each_with_index { |v, i| put_int32(off + i * 4, v) }
    end

    #: (Integer) -> Integer
    def get_uint32(off) = Fiddle.__get_i(to_i + off, -4)

    #: (Integer, Integer) -> void
    def put_uint32(off, v)
      Fiddle.__set_i(to_i + off, -4, v)
    end

    #: () -> Integer
    def read_uint32 = get_uint32(0)

    #: (Integer) -> void
    def write_uint32(v)
      put_uint32(0, v)
    end

    #: (Integer) -> Array[Integer]
    def read_array_of_uint32(n) = get_array_of_uint32(0, n)

    #: (Integer, Integer) -> Array[Integer]
    def get_array_of_uint32(off, n)
      Array.new(n) { |i| get_uint32(off + i * 4) }
    end

    #: (Array[Integer]) -> void
    def write_array_of_uint32(a)
      put_array_of_uint32(0, a)
    end

    #: (Integer, Array[Integer]) -> void
    def put_array_of_uint32(off, a)
      a.each_with_index { |v, i| put_uint32(off + i * 4, v) }
    end

    #: (Integer) -> Integer
    def get_int64(off) = Fiddle.__get_i(to_i + off, 5)

    #: (Integer, Integer) -> void
    def put_int64(off, v)
      Fiddle.__set_i(to_i + off, 5, v)
    end

    #: () -> Integer
    def read_int64 = get_int64(0)

    #: (Integer) -> void
    def write_int64(v)
      put_int64(0, v)
    end

    #: (Integer) -> Array[Integer]
    def read_array_of_int64(n) = get_array_of_int64(0, n)

    #: (Integer, Integer) -> Array[Integer]
    def get_array_of_int64(off, n)
      Array.new(n) { |i| get_int64(off + i * 8) }
    end

    #: (Array[Integer]) -> void
    def write_array_of_int64(a)
      put_array_of_int64(0, a)
    end

    #: (Integer, Array[Integer]) -> void
    def put_array_of_int64(off, a)
      a.each_with_index { |v, i| put_int64(off + i * 8, v) }
    end

    #: (Integer) -> Integer
    def get_uint64(off) = Fiddle.__get_i(to_i + off, -5)

    #: (Integer, Integer) -> void
    def put_uint64(off, v)
      Fiddle.__set_i(to_i + off, -5, v)
    end

    #: () -> Integer
    def read_uint64 = get_uint64(0)

    #: (Integer) -> void
    def write_uint64(v)
      put_uint64(0, v)
    end

    #: (Integer) -> Array[Integer]
    def read_array_of_uint64(n) = get_array_of_uint64(0, n)

    #: (Integer, Integer) -> Array[Integer]
    def get_array_of_uint64(off, n)
      Array.new(n) { |i| get_uint64(off + i * 8) }
    end

    #: (Array[Integer]) -> void
    def write_array_of_uint64(a)
      put_array_of_uint64(0, a)
    end

    #: (Integer, Array[Integer]) -> void
    def put_array_of_uint64(off, a)
      a.each_with_index { |v, i| put_uint64(off + i * 8, v) }
    end

    #: (Integer) -> Float
    def get_float(off) = Fiddle.__get_f(to_i + off, 7)

    #: (Integer, Float) -> void
    def put_float(off, v)
      Fiddle.__set_f(to_i + off, 7, v)
    end

    #: () -> Float
    def read_float = get_float(0)

    #: (Float) -> void
    def write_float(v)
      put_float(0, v)
    end

    #: (Integer) -> Array[Float]
    def read_array_of_float(n) = get_array_of_float(0, n)

    #: (Integer, Integer) -> Array[Float]
    def get_array_of_float(off, n)
      Array.new(n) { |i| get_float(off + i * 4) }
    end

    #: (Array[Float]) -> void
    def write_array_of_float(a)
      put_array_of_float(0, a)
    end

    #: (Integer, Array[Float]) -> void
    def put_array_of_float(off, a)
      a.each_with_index { |v, i| put_float(off + i * 4, v) }
    end

    #: (Integer) -> Float
    def get_double(off) = Fiddle.__get_f(to_i + off, 8)

    #: (Integer, Float) -> void
    def put_double(off, v)
      Fiddle.__set_f(to_i + off, 8, v)
    end

    #: () -> Float
    def read_double = get_double(0)

    #: (Float) -> void
    def write_double(v)
      put_double(0, v)
    end

    #: (Integer) -> Array[Float]
    def read_array_of_double(n) = get_array_of_double(0, n)

    #: (Integer, Integer) -> Array[Float]
    def get_array_of_double(off, n)
      Array.new(n) { |i| get_double(off + i * 8) }
    end

    #: (Array[Float]) -> void
    def write_array_of_double(a)
      put_array_of_double(0, a)
    end

    #: (Integer, Array[Float]) -> void
    def put_array_of_double(off, a)
      a.each_with_index { |v, i| put_double(off + i * 8, v) }
    end
    NULL = Pointer.new(0) #: Pointer
  end

  # ponytail: memory is never freed unless #free is called (no GC finalizers); autorelease only records the flag.
  class MemoryPointer < Pointer
    attr_reader :total #: Integer

    # @dynamic
    #: (untyped, ?Integer, ?bool) -> void
    def initialize(size, count = 1, clear = true)
      unit = case size
             when Symbol then FFI.type_size(size)
             when Integer then size
             else size.to_i
             end
      n = unit * count
      @total = n
      mem = Fiddle.malloc(n == 0 ? 1 : n)
      Fiddle::Pointer.write(mem, "\0" * n) if clear
      super(mem)
      @auto = true
    end

    #: (String) -> MemoryPointer
    def self.from_string(str)
      mp = new(str.bytesize + 1)
      mp.put_string(0, str)
      mp
    end

    #: () -> Integer
    def size = @total

    #: (bool) -> bool
    def autorelease=(v)
      @auto = v
    end

    #: () -> bool
    def autorelease? = @auto

    #: () -> nil
    def free
      Fiddle.free(to_i)
      nil
    end
  end

  # A Ruby callable as a C function pointer.
  class Function < Pointer
    #: (Symbol, Array[Symbol]) { (*untyped) -> untyped } -> void
    def initialize(ret, args, &block)
      kinds = args.map { |a| FFI::TYPES.fetch(a) } #: Array[Integer]
      conv = lambda do |vals|
        vals.each_with_index.map do |v, i|
          kinds[i] == Fiddle::TYPE_VOIDP ? Pointer.new(v) : v
        end
      end #: ^(Array[untyped]) -> Array[untyped]
      @closure = Fiddle::Closure::BlockCaller.new(FFI::TYPES.fetch(ret), kinds) { |*vals| block.call(*conv.call(vals)) }
      super(@closure.to_i)
    end
  end

  class Enum
    attr_reader :tag #: Symbol?

    #: (Array[untyped], ?Symbol?) -> void
    def initialize(values, tag = nil)
      @tag = tag
      @sym = {} #: Hash[Symbol, Integer]
      @int = {} #: Hash[Integer, untyped]
      next_val = 0
      i = 0
      while i < values.size
        name = values[i]
        raise ArgumentError, "enum symbol expected" unless name.is_a?(Symbol)

        v = values[i + 1]
        if v.is_a?(Integer)
          next_val = v
          i += 2
        else
          i += 1
        end
        @sym[name] = next_val
        @int[next_val] = name
        next_val += 1
      end
    end

    #: () -> Array[Symbol]
    def symbols = @sym.keys

    #: () -> Hash[Symbol, Integer]
    def symbol_map = @sym

    #: () -> Hash[Symbol, Integer]
    def to_h = @sym

    #: (untyped) -> untyped
    def [](x)
      x.is_a?(Integer) ? @int[x] : @sym[x]
    end

    #: (untyped) -> Integer
    def to_native(x)
      return x if x.is_a?(Integer)

      @sym.fetch(x) { raise ArgumentError, "invalid enum value, #{x.inspect}" }
    end

    #: (Integer) -> untyped
    def from_native(i) = @int.fetch(i, i)
  end

  class Struct
    class Field
      attr_reader :name #: Symbol
      attr_reader :offset #: Integer
      attr_reader :type #: untyped
      attr_reader :count #: Integer
      attr_reader :size #: Integer

      #: (Symbol, Integer, untyped, Integer, Integer) -> void
      def initialize(name, offset, type, count, size)
        @name = name
        @offset = offset
        @type = type
        @count = count
        @size = size
      end
    end

    class ByReference
      attr_reader :klass #: untyped

      #: (untyped) -> void
      def initialize(klass)
        @klass = klass
      end
    end

    #: () -> Array[Field]
    def self.__fields = (@fields ||= [])

    #: () -> Integer
    def self.size = (@size ||= 0)

    #: () -> Integer
    def self.alignment = (@align ||= 1)

    #: () -> Array[Symbol]
    def self.members = __fields.map(&:name)

    #: () -> Array[[Symbol, Integer]]
    def self.offsets = __fields.map { |f| [f.name, f.offset] }

    #: (Symbol) -> Field
    def self.__field(name)
      __fields.each { |f| return f if f.name == name }
      raise ArgumentError, "Unknown field #{name}"
    end

    #: (Symbol) -> Integer
    def self.offset_of(name) = __field(name).offset

    #: (?Symbol) -> ByReference
    def self.by_ref(_flags = :inout) = ByReference.new(self)

    #: () -> untyped
    def self.by_value = raise(NotImplementedError, "FFI::Struct.by_value: structs by value are not supported (decision 375)")

    # @dynamic
    #: (untyped) -> Integer
    def self.__size_of(type)
      case type
      when Symbol then FFI.type_size(type)
      when ByReference then 8
      else type.size
      end
    end

    # @dynamic
    #: (untyped) -> Integer
    def self.__align_of(type)
      case type
      when Symbol then FFI.type_size(type)
      when ByReference then 8
      else type.alignment
      end
    end

    #: (Integer, Integer) -> Integer
    def self.__round(n, a) = (n + a - 1) / a * a

    #: () -> bool
    def self.__union? = false

    # @dynamic
    #: (*untyped) -> void
    def self.layout(*spec)
      fields = [] #: Array[Field]
      off = 0
      total = 0
      align = 1
      i = 0
      while i < spec.size
        name = spec[i]
        type = spec[i + 1]
        raise ArgumentError, "layout needs a name and a type" unless name.is_a?(Symbol)

        explicit = spec[i + 2]
        i += 2
        count = 1
        elem = type
        if type.is_a?(Array)
          elem = type[0]
          count = type[1].to_i
        end
        sz = __size_of(elem)
        al = __align_of(elem)
        if explicit.is_a?(Integer)
          pos = explicit
          i += 1
        else
          pos = __union? ? 0 : __round(off, al)
        end
        fields << Field.new(name, pos, elem, count, sz)
        off = pos + sz * count
        total = off if off > total
        align = al if al > align
      end
      @fields = fields
      @align = align
      @size = __round(total, align)
    end

    #: (?untyped) -> void
    def initialize(ptr = nil)
      @ptr = ptr.nil? ? MemoryPointer.new(self.class.size) : Pointer.new(ptr) #: Pointer
    end

    #: () -> Pointer
    def pointer = @ptr

    #: () -> Pointer
    def to_ptr = @ptr

    #: () -> Integer
    def size = self.class.size

    #: () -> Array[Symbol]
    def members = self.class.members

    #: () -> Array[untyped]
    def values = members.map { |m| self[m] }

    #: (Symbol) -> Integer
    def offset_of(name) = self.class.offset_of(name)

    # @dynamic
    #: (Field, Integer) -> untyped
    def __read(f, idx)
      at = @ptr.to_i + f.offset + idx * f.size
      t = f.type
      case t
      when :float, :double then Fiddle.__get_f(at, TYPES.fetch(t))
      when :bool then Fiddle.__get_i(at, 2) != 0
      when :pointer, :buffer_in, :buffer_out, :buffer_inout then Pointer.new(Fiddle.__word(at))
      when :string
        w = Fiddle.__word(at)
        w == 0 ? nil : Fiddle.__cstring(w)
      when Symbol then Fiddle.__get_i(at, TYPES.fetch(t))
      when ByReference then Pointer.new(Fiddle.__word(at))
      else t.new(at)
      end
    end

    # @dynamic
    #: (Field, Integer, untyped) -> void
    def __write(f, idx, v)
      at = @ptr.to_i + f.offset + idx * f.size
      t = f.type
      case t
      when :float, :double then Fiddle.__set_f(at, TYPES.fetch(t), v.to_f)
      when :bool then Fiddle.__set_i(at, 2, v ? 1 : 0)
      when :pointer, :buffer_in, :buffer_out, :buffer_inout, ByReference then Fiddle.__set_i(at, 5, Fiddle.__addr(FFI.__ptr(v)))
      when :string then raise NotImplementedError, "FFI::Struct :string fields are read-only (decision 375)"
      when Symbol then Fiddle.__set_i(at, TYPES.fetch(t), v)
      else Fiddle::Pointer.write(at, Fiddle::Pointer.read(Fiddle.__addr(FFI.__ptr(v)), f.size))
      end
    end

    #: (Symbol) -> untyped
    def [](name)
      f = self.class.__field(name)
      return __read(f, 0) if f.count == 1

      Array.new(f.count) { |i| __read(f, i) }
    end

    #: (Symbol, untyped) -> untyped
    def []=(name, v)
      f = self.class.__field(name)
      if f.count == 1
        __write(f, 0, v)
      else
        vals = v #: Array[untyped]
        vals.each_with_index { |x, i| __write(f, i, x) }
      end
      v
    end
  end

  class Union < Struct
    #: () -> bool
    def self.__union? = true
  end

  module Library
    LIBC = RUBY_PLATFORM.include?("darwin") ? "/usr/lib/libSystem.B.dylib" : "libc.so.6"

    #: (*untyped) -> Array[Fiddle::Handle?]
    def ffi_lib(*names)
      names = names.flatten
      names = [CURRENT_PROCESS] if names.empty?
      libs = [] #: Array[Fiddle::Handle?]
      names.each do |n|
        if n.equal?(CURRENT_PROCESS) || n.nil?
          libs << Fiddle.dlopen(nil)
        else
          begin
            libs << Fiddle.dlopen(n.to_s)
          rescue Fiddle::DLError => e
            raise LoadError, "Could not open library '#{n}': #{e.message}"
          end
        end
      end
      @ffi_libs = libs
    end

    #: () -> Array[Fiddle::Handle?]
    def ffi_libraries = (@ffi_libs ||= [Fiddle.dlopen(nil)])

    #: () -> Hash[Symbol, [Array[Symbol], Symbol]]
    def __cbs
      @ffi_callbacks ||= {} #: Hash[Symbol, [Array[Symbol], Symbol]]
      @ffi_callbacks
    end

    #: () -> Hash[Symbol, Enum]
    def __enums
      @ffi_enums ||= {} #: Hash[Symbol, Enum]
      @ffi_enums
    end

    #: () -> Hash[Symbol, Symbol]
    def __tdefs
      @ffi_types ||= {} #: Hash[Symbol, Symbol]
      @ffi_types
    end

    #: () -> Hash[String, Fiddle::Function]
    def __funcs
      @ffi_funcs ||= {} #: Hash[String, Fiddle::Function]
      @ffi_funcs
    end

    #: (*untyped) -> void
    def ffi_convention(*_args); end

    #: (*untyped) -> void
    def ffi_lib_flags(*_args); end

    #: (Symbol, Symbol) -> Symbol
    def typedef(old, add)
      __tdefs[add] = old
    end

    #: (Symbol, Array[Symbol], Symbol) -> void
    def callback(name, args, ret)
      __cbs[name] = [args, ret]
    end

    # enum :name, [...]  or  enum [...]
    #: (*untyped) -> Enum
    def enum(*args)
      tag = args[0].is_a?(Symbol) ? args[0] : nil
      values = (tag.nil? ? args[0] : args[1]) #: Array[untyped]
      e = Enum.new(values, tag)
      __enums[tag] = e unless tag.nil?
      e
    end

    #: (Symbol) -> Enum
    def enum_type(name) = __enums.fetch(name)

    #: (Symbol) -> Integer
    def enum_value(sym)
      __enums.each_value { |e| return e.symbol_map.fetch(sym) if e.symbol_map.key?(sym) }
      raise ArgumentError, "unknown enum value #{sym}"
    end

    #: (Symbol) -> Symbol
    def __ffi_resolve(type)
      t = type
      while __tdefs.key?(t)
        t = __tdefs.fetch(t)
      end
      t
    end

    #: (untyped) -> Integer
    def __ffi_code(type)
      return TYPES.fetch(:pointer) unless type.is_a?(Symbol)

      t = __ffi_resolve(type)
      return TYPES.fetch(:pointer) if __cbs.key?(t)
      return TYPES.fetch(:int) if __enums.key?(t)

      code = TYPES[t]
      raise TypeError, "unable to resolve type '#{type}'" if code.nil?

      code
    end

    # attach_function name, args, ret  or  attach_function name, cname, args, ret
    #: (*untyped) -> untyped
    def attach_function(*args)
      name = args[0].to_s
      named = args[1].is_a?(Symbol) || args[1].is_a?(String)
      cname = named ? args[1].to_s : name
      arg_types = (named ? args[2] : args[1]) #: Array[untyped]
      ret_type = (named ? args[3] : args[2]) #: Symbol
      sym = nil #: Integer?
      ffi_libraries.each do |h|
        sym = h&.sym(cname) rescue nil # rubocop:disable Style/RescueModifier
        break if sym
      end
      raise NotFoundError.new(cname, ffi_libraries.map { |h| h&.file_name.to_s }) if sym.nil?

      codes = arg_types.map { |t| __ffi_code(t) } #: Array[Integer]
      f = Fiddle::Function.new(sym, codes, __ffi_code(ret_type), name: cname)
      __funcs[name] = f
      @ffi_cache = nil
      f
    end

    # Slot read, not a hash of the name per call (decision 329).
    #: (Integer, String) -> Fiddle::Function
    def __ffi_func_at(k, name)
      cache = (@ffi_cache ||= []) #: Array[Fiddle::Function?]
      cache[k] || (cache[k] = __funcs.fetch(name))
    end

    # A callback argument: a Proc becomes a C function pointer; a Pointer passes through.
    # @dynamic
    #: (Symbol, untyped) -> untyped
    def __ffi_cb(name, x)
      return x if x.nil? || x.is_a?(Fiddle::Pointer)

      args, ret = __cbs.fetch(__ffi_resolve(name))
      FFI::Function.new(ret, args.map { |a| __ffi_resolve(a) }) { |*vals| x.call(*vals) }
    end

    #: (Symbol, untyped) -> untyped
    def __ffi_enum_in(name, x) = __enums.fetch(__ffi_resolve(name)).to_native(x)

    #: (Symbol, untyped) -> untyped
    def __ffi_enum_out(name, x) = __enums.fetch(__ffi_resolve(name)).from_native(x)
  end
end
