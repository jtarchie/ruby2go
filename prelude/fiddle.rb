# rbs_inline: enabled

# Fiddle without cgo (decision 296): dlopen raises DLError; MRI's LP64 constants serve signatures built at load.

module Fiddle
  TYPE_VOID = 0
  TYPE_VOIDP = 1
  TYPE_CHAR = 2
  TYPE_UCHAR = -2
  TYPE_SHORT = 3
  TYPE_INT = 4
  TYPE_UINT = -4
  TYPE_INT32_T = 4
  TYPE_UINT32_T = -4
  TYPE_LONG = 5
  TYPE_ULONG = -5
  TYPE_INT64_T = 5
  TYPE_UINT64_T = -5
  TYPE_SSIZE_T = 5
  TYPE_SIZE_T = -5
  TYPE_LONG_LONG = 6
  TYPE_FLOAT = 7
  TYPE_DOUBLE = 8
  TYPE_CONST_STRING = 10
  TYPE_BOOL = 11
  SIZEOF_VOIDP = 8
  SIZEOF_INT = 4
  SIZEOF_LONG = 8
  SIZEOF_SIZE_T = 8
  RUBY_FREE = 0

  class Error < StandardError
  end

  class DLError < Error
  end

  # A loaded library's symbols; dlopen never returns one.
  class Handle
    #: (String) -> Integer
    def [](name)
      raise DLError, "unknown symbol \"#{name}\""
    end
  end

  # Memory at an address; only the address and null? mean anything without a library to pass it to.
  class Pointer
    #: (Integer, ?Integer) -> void
    def initialize(addr, size = 0)
      @addr = addr
      @size = size
    end

    #: (Integer, ?Integer) -> Pointer
    def self.malloc(size, free = 0) = new(0, size)

    #: () -> bool
    def null? = @addr.zero?

    #: () -> Integer
    def to_i = @addr

    #: () -> Integer
    def size = @size

    #: () -> Pointer
    def ptr
      raise DLError, "cannot dereference a pointer in a compiled program"
    end
  end

  # A C function; it needs a symbol from a loaded library, so it cannot be built.
  class Function
    #: (Integer, Array[Integer], Integer) -> void
    def initialize(ptr, args, ret)
      raise DLError, "cannot call C functions in a compiled program"
    end

    #: (*untyped) -> untyped
    def call(*args)
      raise DLError, "cannot call C functions in a compiled program"
    end
  end

  #: (String?) -> Handle
  def self.dlopen(path)
    raise DLError, "#{path}: dlopen is not available in a compiled program"
  end

  #: () -> Integer?
  def self.last_error = nil
end
