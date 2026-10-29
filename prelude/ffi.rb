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
    buffer_in: Fiddle::TYPE_VOIDP, buffer_out: Fiddle::TYPE_VOIDP, buffer_inout: Fiddle::TYPE_VOIDP
  } #: Hash[Symbol, Integer]

  SIZES = {
    bool: 1, char: 1, uchar: 1, int8: 1, uint8: 1, short: 2, ushort: 2, int16: 2, uint16: 2,
    int: 4, uint: 4, int32: 4, uint32: 4, float: 4,
    long: 8, ulong: 8, int64: 8, uint64: 8, long_long: 8, ulong_long: 8, size_t: 8, ssize_t: 8,
    double: 8, pointer: 8, string: 8
  } #: Hash[Symbol, Integer]

  #: (Symbol) -> Integer
  def self.type_size(type)
    SIZES.fetch(type) { raise TypeError, "unable to resolve type '#{type}'" }
  end

  #: (untyped) -> untyped
  def self.__ptr(x)
    x.is_a?(Struct) ? x.pointer : x
  end

  # Past 2**63 the value needs a Bignum: it raises unless the program is built with --int-overflow=promote (decision 372).
  #: (Integer) -> Integer
  def self.__u64(v) = v < 0 ? v + (1 << 64) : v

  #: (Integer, Integer) -> Integer
  def self.__u64_if(code, v) = code == -5 || code == -6 ? __u64(v) : v

  #: () -> Integer
  def self.errno = LastError.error

  # The gem's name mangling: "c" is libc, a bare "foo" is tried as libfoo.so / libfoo.dylib too.
  #: (String) -> String
  def self.map_library_name(lib)
    lib = Platform::LIBC if lib == "c"
    return lib unless File.basename(lib) == lib

    lib = "lib#{lib}" unless lib.start_with?("lib")
    mac = Platform::IS_MAC
    done = mac ? lib.end_with?(".dylib") : lib.match?(/\.so(\z|\.\d+)/)
    if done
      lib
    else
      "#{lib}.#{mac ? "dylib" : "so"}"
    end
  end

  module Platform
    OS = RUBY_PLATFORM.include?("darwin") ? "darwin" : "linux" #: String
    ARCH = RUBY_PLATFORM.start_with?("arm64", "aarch64") ? "aarch64" : "x86_64" #: String
    IS_LINUX = OS == "linux" #: bool
    IS_MAC = OS == "darwin" #: bool
    IS_WINDOWS = false #: bool
    LONG_SIZE = 64 #: Integer
    ADDRESS_SIZE = 64 #: Integer
    LIBC = RUBY_PLATFORM.include?("darwin") ? "libc.dylib" : "libc.so.6" #: String
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

    #: (Integer, Integer) -> Array[Pointer]
    def get_array_of_pointer(off, n)
      Array.new(n) { |i| get_pointer(off + i * 8) }
    end

    #: (Integer) -> Array[Pointer]
    def read_array_of_pointer(n) = get_array_of_pointer(0, n)

    #: (Integer, Array[untyped]) -> void
    def put_array_of_pointer(off, ptrs)
      ptrs.each_with_index { |p, i| put_pointer(off + i * 8, p) }
    end

    #: (Array[untyped]) -> void
    def write_array_of_pointer(ptrs)
      put_array_of_pointer(0, ptrs)
    end

    # Reads count C strings, or up to the NULL entry when count is nil.
    #: (Integer, ?Integer?) -> Array[String?]
    def get_array_of_string(off, count = nil)
      out = [] #: Array[String?]
      i = 0
      while count.nil? || i < count
        w = Fiddle.__word(to_i + off + i * 8)
        break if count.nil? && w == 0

        out << (w == 0 ? nil : Fiddle.__cstring(w))
        i += 1
      end
      out
    end

    #: (?Integer?) -> Array[String?]
    def read_array_of_string(count = nil) = get_array_of_string(0, count)

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

    #: (Integer, Integer) -> Integer
    def __gi(off, code) = FFI.__u64_if(code, Fiddle.__get_i(to_i + off, code))

    #: (Integer, Integer, Integer) -> void
    def __pi(off, code, v)
      Fiddle.__set_i(to_i + off, code, v)
    end

    #: (Integer, Integer) -> Float
    def __gf(off, code) = Fiddle.__get_f(to_i + off, code)

    #: (Integer, Integer, Float) -> void
    def __pf(off, code, v)
      Fiddle.__set_f(to_i + off, code, v)
    end

    #: (Integer, Integer, Integer, Integer) -> Array[Integer]
    def __gai(off, n, code, size) = Array.new(n) { |i| __gi(off + i * size, code) }

    #: (Integer, Array[Integer], Integer, Integer) -> void
    def __pai(off, a, code, size)
      a.each_with_index { |v, i| __pi(off + i * size, code, v) }
    end

    #: (Integer, Integer, Integer, Integer) -> Array[Float]
    def __gaf(off, n, code, size) = Array.new(n) { |i| __gf(off + i * size, code) }

    #: (Integer, Array[Float], Integer, Integer) -> void
    def __paf(off, a, code, size)
      a.each_with_index { |v, i| __pf(off + i * size, code, v) }
    end

    #: (Integer) -> Integer
    def get_char(off) = __gi(off, 2)

    #: (Integer, Integer) -> void
    def put_char(off, v)
      __pi(off, 2, v)
    end

    #: () -> Integer
    def read_char = __gi(0, 2)

    #: (Integer) -> void
    def write_char(v)
      __pi(0, 2, v)
    end

    #: (Integer) -> Array[Integer]
    def read_array_of_char(n) = __gai(0, n, 2, 1)

    #: (Integer, Integer) -> Array[Integer]
    def get_array_of_char(off, n) = __gai(off, n, 2, 1)

    #: (Array[Integer]) -> void
    def write_array_of_char(a)
      __pai(0, a, 2, 1)
    end

    #: (Integer, Array[Integer]) -> void
    def put_array_of_char(off, a)
      __pai(off, a, 2, 1)
    end

    #: (Integer) -> Integer
    def get_uchar(off) = __gi(off, -2)

    #: (Integer, Integer) -> void
    def put_uchar(off, v)
      __pi(off, -2, v)
    end

    #: () -> Integer
    def read_uchar = __gi(0, -2)

    #: (Integer) -> void
    def write_uchar(v)
      __pi(0, -2, v)
    end

    #: (Integer) -> Array[Integer]
    def read_array_of_uchar(n) = __gai(0, n, -2, 1)

    #: (Integer, Integer) -> Array[Integer]
    def get_array_of_uchar(off, n) = __gai(off, n, -2, 1)

    #: (Array[Integer]) -> void
    def write_array_of_uchar(a)
      __pai(0, a, -2, 1)
    end

    #: (Integer, Array[Integer]) -> void
    def put_array_of_uchar(off, a)
      __pai(off, a, -2, 1)
    end

    #: (Integer) -> Integer
    def get_short(off) = __gi(off, 3)

    #: (Integer, Integer) -> void
    def put_short(off, v)
      __pi(off, 3, v)
    end

    #: () -> Integer
    def read_short = __gi(0, 3)

    #: (Integer) -> void
    def write_short(v)
      __pi(0, 3, v)
    end

    #: (Integer) -> Array[Integer]
    def read_array_of_short(n) = __gai(0, n, 3, 2)

    #: (Integer, Integer) -> Array[Integer]
    def get_array_of_short(off, n) = __gai(off, n, 3, 2)

    #: (Array[Integer]) -> void
    def write_array_of_short(a)
      __pai(0, a, 3, 2)
    end

    #: (Integer, Array[Integer]) -> void
    def put_array_of_short(off, a)
      __pai(off, a, 3, 2)
    end

    #: (Integer) -> Integer
    def get_ushort(off) = __gi(off, -3)

    #: (Integer, Integer) -> void
    def put_ushort(off, v)
      __pi(off, -3, v)
    end

    #: () -> Integer
    def read_ushort = __gi(0, -3)

    #: (Integer) -> void
    def write_ushort(v)
      __pi(0, -3, v)
    end

    #: (Integer) -> Array[Integer]
    def read_array_of_ushort(n) = __gai(0, n, -3, 2)

    #: (Integer, Integer) -> Array[Integer]
    def get_array_of_ushort(off, n) = __gai(off, n, -3, 2)

    #: (Array[Integer]) -> void
    def write_array_of_ushort(a)
      __pai(0, a, -3, 2)
    end

    #: (Integer, Array[Integer]) -> void
    def put_array_of_ushort(off, a)
      __pai(off, a, -3, 2)
    end

    #: (Integer) -> Integer
    def get_int(off) = __gi(off, 4)

    #: (Integer, Integer) -> void
    def put_int(off, v)
      __pi(off, 4, v)
    end

    #: () -> Integer
    def read_int = __gi(0, 4)

    #: (Integer) -> void
    def write_int(v)
      __pi(0, 4, v)
    end

    #: (Integer) -> Array[Integer]
    def read_array_of_int(n) = __gai(0, n, 4, 4)

    #: (Integer, Integer) -> Array[Integer]
    def get_array_of_int(off, n) = __gai(off, n, 4, 4)

    #: (Array[Integer]) -> void
    def write_array_of_int(a)
      __pai(0, a, 4, 4)
    end

    #: (Integer, Array[Integer]) -> void
    def put_array_of_int(off, a)
      __pai(off, a, 4, 4)
    end

    #: (Integer) -> Integer
    def get_uint(off) = __gi(off, -4)

    #: (Integer, Integer) -> void
    def put_uint(off, v)
      __pi(off, -4, v)
    end

    #: () -> Integer
    def read_uint = __gi(0, -4)

    #: (Integer) -> void
    def write_uint(v)
      __pi(0, -4, v)
    end

    #: (Integer) -> Array[Integer]
    def read_array_of_uint(n) = __gai(0, n, -4, 4)

    #: (Integer, Integer) -> Array[Integer]
    def get_array_of_uint(off, n) = __gai(off, n, -4, 4)

    #: (Array[Integer]) -> void
    def write_array_of_uint(a)
      __pai(0, a, -4, 4)
    end

    #: (Integer, Array[Integer]) -> void
    def put_array_of_uint(off, a)
      __pai(off, a, -4, 4)
    end

    #: (Integer) -> Integer
    def get_long(off) = __gi(off, 5)

    #: (Integer, Integer) -> void
    def put_long(off, v)
      __pi(off, 5, v)
    end

    #: () -> Integer
    def read_long = __gi(0, 5)

    #: (Integer) -> void
    def write_long(v)
      __pi(0, 5, v)
    end

    #: (Integer) -> Array[Integer]
    def read_array_of_long(n) = __gai(0, n, 5, 8)

    #: (Integer, Integer) -> Array[Integer]
    def get_array_of_long(off, n) = __gai(off, n, 5, 8)

    #: (Array[Integer]) -> void
    def write_array_of_long(a)
      __pai(0, a, 5, 8)
    end

    #: (Integer, Array[Integer]) -> void
    def put_array_of_long(off, a)
      __pai(off, a, 5, 8)
    end

    #: (Integer) -> Integer
    def get_ulong(off) = __gi(off, -5)

    #: (Integer, Integer) -> void
    def put_ulong(off, v)
      __pi(off, -5, v)
    end

    #: () -> Integer
    def read_ulong = __gi(0, -5)

    #: (Integer) -> void
    def write_ulong(v)
      __pi(0, -5, v)
    end

    #: (Integer) -> Array[Integer]
    def read_array_of_ulong(n) = __gai(0, n, -5, 8)

    #: (Integer, Integer) -> Array[Integer]
    def get_array_of_ulong(off, n) = __gai(off, n, -5, 8)

    #: (Array[Integer]) -> void
    def write_array_of_ulong(a)
      __pai(0, a, -5, 8)
    end

    #: (Integer, Array[Integer]) -> void
    def put_array_of_ulong(off, a)
      __pai(off, a, -5, 8)
    end

    #: (Integer) -> Integer
    def get_long_long(off) = __gi(off, 6)

    #: (Integer, Integer) -> void
    def put_long_long(off, v)
      __pi(off, 6, v)
    end

    #: () -> Integer
    def read_long_long = __gi(0, 6)

    #: (Integer) -> void
    def write_long_long(v)
      __pi(0, 6, v)
    end

    #: (Integer) -> Array[Integer]
    def read_array_of_long_long(n) = __gai(0, n, 6, 8)

    #: (Integer, Integer) -> Array[Integer]
    def get_array_of_long_long(off, n) = __gai(off, n, 6, 8)

    #: (Array[Integer]) -> void
    def write_array_of_long_long(a)
      __pai(0, a, 6, 8)
    end

    #: (Integer, Array[Integer]) -> void
    def put_array_of_long_long(off, a)
      __pai(off, a, 6, 8)
    end

    #: (Integer) -> Integer
    def get_ulong_long(off) = __gi(off, -6)

    #: (Integer, Integer) -> void
    def put_ulong_long(off, v)
      __pi(off, -6, v)
    end

    #: () -> Integer
    def read_ulong_long = __gi(0, -6)

    #: (Integer) -> void
    def write_ulong_long(v)
      __pi(0, -6, v)
    end

    #: (Integer) -> Array[Integer]
    def read_array_of_ulong_long(n) = __gai(0, n, -6, 8)

    #: (Integer, Integer) -> Array[Integer]
    def get_array_of_ulong_long(off, n) = __gai(off, n, -6, 8)

    #: (Array[Integer]) -> void
    def write_array_of_ulong_long(a)
      __pai(0, a, -6, 8)
    end

    #: (Integer, Array[Integer]) -> void
    def put_array_of_ulong_long(off, a)
      __pai(off, a, -6, 8)
    end

    #: (Integer) -> Integer
    def get_int8(off) = __gi(off, 2)

    #: (Integer, Integer) -> void
    def put_int8(off, v)
      __pi(off, 2, v)
    end

    #: () -> Integer
    def read_int8 = __gi(0, 2)

    #: (Integer) -> void
    def write_int8(v)
      __pi(0, 2, v)
    end

    #: (Integer) -> Array[Integer]
    def read_array_of_int8(n) = __gai(0, n, 2, 1)

    #: (Integer, Integer) -> Array[Integer]
    def get_array_of_int8(off, n) = __gai(off, n, 2, 1)

    #: (Array[Integer]) -> void
    def write_array_of_int8(a)
      __pai(0, a, 2, 1)
    end

    #: (Integer, Array[Integer]) -> void
    def put_array_of_int8(off, a)
      __pai(off, a, 2, 1)
    end

    #: (Integer) -> Integer
    def get_uint8(off) = __gi(off, -2)

    #: (Integer, Integer) -> void
    def put_uint8(off, v)
      __pi(off, -2, v)
    end

    #: () -> Integer
    def read_uint8 = __gi(0, -2)

    #: (Integer) -> void
    def write_uint8(v)
      __pi(0, -2, v)
    end

    #: (Integer) -> Array[Integer]
    def read_array_of_uint8(n) = __gai(0, n, -2, 1)

    #: (Integer, Integer) -> Array[Integer]
    def get_array_of_uint8(off, n) = __gai(off, n, -2, 1)

    #: (Array[Integer]) -> void
    def write_array_of_uint8(a)
      __pai(0, a, -2, 1)
    end

    #: (Integer, Array[Integer]) -> void
    def put_array_of_uint8(off, a)
      __pai(off, a, -2, 1)
    end

    #: (Integer) -> Integer
    def get_int16(off) = __gi(off, 3)

    #: (Integer, Integer) -> void
    def put_int16(off, v)
      __pi(off, 3, v)
    end

    #: () -> Integer
    def read_int16 = __gi(0, 3)

    #: (Integer) -> void
    def write_int16(v)
      __pi(0, 3, v)
    end

    #: (Integer) -> Array[Integer]
    def read_array_of_int16(n) = __gai(0, n, 3, 2)

    #: (Integer, Integer) -> Array[Integer]
    def get_array_of_int16(off, n) = __gai(off, n, 3, 2)

    #: (Array[Integer]) -> void
    def write_array_of_int16(a)
      __pai(0, a, 3, 2)
    end

    #: (Integer, Array[Integer]) -> void
    def put_array_of_int16(off, a)
      __pai(off, a, 3, 2)
    end

    #: (Integer) -> Integer
    def get_uint16(off) = __gi(off, -3)

    #: (Integer, Integer) -> void
    def put_uint16(off, v)
      __pi(off, -3, v)
    end

    #: () -> Integer
    def read_uint16 = __gi(0, -3)

    #: (Integer) -> void
    def write_uint16(v)
      __pi(0, -3, v)
    end

    #: (Integer) -> Array[Integer]
    def read_array_of_uint16(n) = __gai(0, n, -3, 2)

    #: (Integer, Integer) -> Array[Integer]
    def get_array_of_uint16(off, n) = __gai(off, n, -3, 2)

    #: (Array[Integer]) -> void
    def write_array_of_uint16(a)
      __pai(0, a, -3, 2)
    end

    #: (Integer, Array[Integer]) -> void
    def put_array_of_uint16(off, a)
      __pai(off, a, -3, 2)
    end

    #: (Integer) -> Integer
    def get_int32(off) = __gi(off, 4)

    #: (Integer, Integer) -> void
    def put_int32(off, v)
      __pi(off, 4, v)
    end

    #: () -> Integer
    def read_int32 = __gi(0, 4)

    #: (Integer) -> void
    def write_int32(v)
      __pi(0, 4, v)
    end

    #: (Integer) -> Array[Integer]
    def read_array_of_int32(n) = __gai(0, n, 4, 4)

    #: (Integer, Integer) -> Array[Integer]
    def get_array_of_int32(off, n) = __gai(off, n, 4, 4)

    #: (Array[Integer]) -> void
    def write_array_of_int32(a)
      __pai(0, a, 4, 4)
    end

    #: (Integer, Array[Integer]) -> void
    def put_array_of_int32(off, a)
      __pai(off, a, 4, 4)
    end

    #: (Integer) -> Integer
    def get_uint32(off) = __gi(off, -4)

    #: (Integer, Integer) -> void
    def put_uint32(off, v)
      __pi(off, -4, v)
    end

    #: () -> Integer
    def read_uint32 = __gi(0, -4)

    #: (Integer) -> void
    def write_uint32(v)
      __pi(0, -4, v)
    end

    #: (Integer) -> Array[Integer]
    def read_array_of_uint32(n) = __gai(0, n, -4, 4)

    #: (Integer, Integer) -> Array[Integer]
    def get_array_of_uint32(off, n) = __gai(off, n, -4, 4)

    #: (Array[Integer]) -> void
    def write_array_of_uint32(a)
      __pai(0, a, -4, 4)
    end

    #: (Integer, Array[Integer]) -> void
    def put_array_of_uint32(off, a)
      __pai(off, a, -4, 4)
    end

    #: (Integer) -> Integer
    def get_int64(off) = __gi(off, 5)

    #: (Integer, Integer) -> void
    def put_int64(off, v)
      __pi(off, 5, v)
    end

    #: () -> Integer
    def read_int64 = __gi(0, 5)

    #: (Integer) -> void
    def write_int64(v)
      __pi(0, 5, v)
    end

    #: (Integer) -> Array[Integer]
    def read_array_of_int64(n) = __gai(0, n, 5, 8)

    #: (Integer, Integer) -> Array[Integer]
    def get_array_of_int64(off, n) = __gai(off, n, 5, 8)

    #: (Array[Integer]) -> void
    def write_array_of_int64(a)
      __pai(0, a, 5, 8)
    end

    #: (Integer, Array[Integer]) -> void
    def put_array_of_int64(off, a)
      __pai(off, a, 5, 8)
    end

    #: (Integer) -> Integer
    def get_uint64(off) = __gi(off, -5)

    #: (Integer, Integer) -> void
    def put_uint64(off, v)
      __pi(off, -5, v)
    end

    #: () -> Integer
    def read_uint64 = __gi(0, -5)

    #: (Integer) -> void
    def write_uint64(v)
      __pi(0, -5, v)
    end

    #: (Integer) -> Array[Integer]
    def read_array_of_uint64(n) = __gai(0, n, -5, 8)

    #: (Integer, Integer) -> Array[Integer]
    def get_array_of_uint64(off, n) = __gai(off, n, -5, 8)

    #: (Array[Integer]) -> void
    def write_array_of_uint64(a)
      __pai(0, a, -5, 8)
    end

    #: (Integer, Array[Integer]) -> void
    def put_array_of_uint64(off, a)
      __pai(off, a, -5, 8)
    end

    #: (Integer) -> Float
    def get_float(off) = __gf(off, 7)

    #: (Integer, Float) -> void
    def put_float(off, v)
      __pf(off, 7, v)
    end

    #: () -> Float
    def read_float = __gf(0, 7)

    #: (Float) -> void
    def write_float(v)
      __pf(0, 7, v)
    end

    #: (Integer) -> Array[Float]
    def read_array_of_float(n) = __gaf(0, n, 7, 4)

    #: (Integer, Integer) -> Array[Float]
    def get_array_of_float(off, n) = __gaf(off, n, 7, 4)

    #: (Array[Float]) -> void
    def write_array_of_float(a)
      __paf(0, a, 7, 4)
    end

    #: (Integer, Array[Float]) -> void
    def put_array_of_float(off, a)
      __paf(off, a, 7, 4)
    end

    #: (Integer) -> Float
    def get_double(off) = __gf(off, 8)

    #: (Integer, Float) -> void
    def put_double(off, v)
      __pf(off, 8, v)
    end

    #: () -> Float
    def read_double = __gf(0, 8)

    #: (Float) -> void
    def write_double(v)
      __pf(0, 8, v)
    end

    #: (Integer) -> Array[Float]
    def read_array_of_double(n) = __gaf(0, n, 8, 8)

    #: (Integer, Integer) -> Array[Float]
    def get_array_of_double(off, n) = __gaf(off, n, 8, 8)

    #: (Array[Float]) -> void
    def write_array_of_double(a)
      __paf(0, a, 8, 8)
    end

    #: (Integer, Array[Float]) -> void
    def put_array_of_double(off, a)
      __paf(off, a, 8, 8)
    end

    NULL = Pointer.new(0) #: Pointer
  end

  # ponytail: memory is never freed unless #free is called (no GC finalizers); autorelease only records the flag.
  class MemoryPointer < Pointer
    attr_reader :total #: Integer

    # @dynamic
    #: (untyped, ?Integer, ?bool) ?{ (MemoryPointer) -> untyped } -> void
    def initialize(size, count = 1, clear = true, &block)
      unit = case size
             when Symbol then FFI.type_size(size)
             when Integer then size
             else size.size
             end
      n = unit * count
      @total = n
      mem = Fiddle.malloc(n == 0 ? 1 : n)
      Fiddle::Pointer.write(mem, "\0" * n) if clear
      super(mem)
      @auto = true
      return if block.nil?

      # ponytail: new returns the pointer, not the block's value (the gem returns the value); the memory is freed after the block.
      block.call(self)
      free
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
      when :ulong, :uint64, :ulong_long, :size_t then FFI.__u64(Fiddle.__get_i(at, TYPES.fetch(t)))
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
    LIBC = Platform::LIBC

    #: (*untyped) -> Array[Fiddle::Handle?]
    def ffi_lib(*names)
      names = names.flatten
      names = [CURRENT_PROCESS] if names.empty?
      libs = [] #: Array[Fiddle::Handle?]
      names.each do |n|
        libs << if n.equal?(CURRENT_PROCESS) || n.nil?
                  Fiddle.dlopen(nil)
                else
                  __open(n.to_s)
                end
      end
      __libs.replace(libs)
    end

    #: (String) -> Fiddle::Handle
    def __open(name)
      cands = [name, FFI.map_library_name(name)].uniq
      err = ""
      i = 0
      while i < cands.size
        begin
          return Fiddle.dlopen(cands[i])
        rescue Fiddle::DLError => e
          err = e.message
        end
        i += 1
      end
      raise LoadError, "Could not open library '#{name}': #{err}"
    end

    #: () -> Array[Fiddle::Handle?]
    def ffi_libraries
      libs = __libs
      raise LoadError, "no library specified" if libs.empty?

      libs
    end

    #: () -> Hash[Symbol, [Array[Symbol], Symbol]]
    def __cbs
      @ffi_callbacks ||= {} #: Hash[Symbol, [Array[Symbol], Symbol]]
      @ffi_callbacks
    end

    #: () -> Array[Fiddle::Handle?]
    def __libs
      @ffi_libs ||= [] #: Array[Fiddle::Handle?]
      @ffi_libs
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

    # A Struct.by_ref target is a pointer.
    #: (untyped, Symbol) -> Symbol
    def typedef(old, add)
      __tdefs[add] = old.is_a?(Symbol) ? old : :pointer
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
      t = __tdefs.fetch(t) while __tdefs.key?(t)
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

    #: () -> Array[[untyped, Symbol, FFI::Function]]
    def __cbcache
      @ffi_cbcache ||= [] #: Array[[untyped, Symbol, FFI::Function]]
      @ffi_cbcache
    end

    # An enum travels as its int; a typedef resolves.
    #: (Symbol) -> Symbol
    def __ffi_cbtype(type)
      t = __ffi_resolve(type)
      __enums.key?(t) ? :int : t
    end

    # A callback argument: a Proc becomes a C function pointer, once per Proc (closures are never freed; decision 326).
    # @dynamic
    #: (Symbol, untyped) -> untyped
    def __ffi_cb(name, x)
      return x if x.nil? || x.is_a?(Fiddle::Pointer)

      cache = __cbcache
      cache.each { |c| return c[2] if c[0].equal?(x) && c[1] == name }
      args, ret = __cbs.fetch(__ffi_resolve(name))
      fn = FFI::Function.new(__ffi_cbtype(ret), args.map { |a| __ffi_cbtype(a) }) { |*vals| x.call(*vals) }
      cache << [x, name, fn]
      fn
    end

    #: (Symbol, untyped) -> untyped
    def __ffi_enum_in(name, x) = __enums.fetch(__ffi_resolve(name)).to_native(x)

    #: (Symbol, untyped) -> untyped
    def __ffi_enum_out(name, x) = __enums.fetch(__ffi_resolve(name)).from_native(x)
  end
end
