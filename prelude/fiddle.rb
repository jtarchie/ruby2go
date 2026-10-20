# rbs_inline: enabled

# Fiddle over purego without cgo (#32, decision 326); addresses are Integers, type constants are MRI LP64.

module Fiddle
  VERSION = "1.1.8"
  WINDOWS = false

  TYPE_VOID = 0
  TYPE_VOIDP = 1
  TYPE_CHAR = 2
  TYPE_UCHAR = -2
  TYPE_SHORT = 3
  TYPE_USHORT = -3
  TYPE_INT = 4
  TYPE_UINT = -4
  TYPE_LONG = 5
  TYPE_ULONG = -5
  TYPE_LONG_LONG = 6
  TYPE_ULONG_LONG = -6
  TYPE_FLOAT = 7
  TYPE_DOUBLE = 8
  TYPE_VARIADIC = 9
  TYPE_CONST_STRING = 10
  TYPE_BOOL = 11
  TYPE_INT8_T = 2
  TYPE_UINT8_T = -2
  TYPE_INT16_T = 3
  TYPE_UINT16_T = -3
  TYPE_INT32_T = 4
  TYPE_UINT32_T = -4
  TYPE_INT64_T = 5
  TYPE_UINT64_T = -5
  TYPE_SSIZE_T = 5
  TYPE_SIZE_T = -5
  TYPE_PTRDIFF_T = 5
  TYPE_INTPTR_T = 5
  TYPE_UINTPTR_T = -5

  SIZEOF_VOIDP = 8
  SIZEOF_CHAR = 1
  SIZEOF_UCHAR = 1
  SIZEOF_SHORT = 2
  SIZEOF_USHORT = 2
  SIZEOF_INT = 4
  SIZEOF_UINT = 4
  SIZEOF_LONG = 8
  SIZEOF_ULONG = 8
  SIZEOF_LONG_LONG = 8
  SIZEOF_ULONG_LONG = 8
  SIZEOF_FLOAT = 4
  SIZEOF_DOUBLE = 8
  SIZEOF_SIZE_T = 8
  SIZEOF_SSIZE_T = 8
  SIZEOF_PTRDIFF_T = 8
  SIZEOF_INTPTR_T = 8
  SIZEOF_UINTPTR_T = 8
  SIZEOF_CONST_STRING = 8
  SIZEOF_BOOL = 1
  SIZEOF_INT8_T = 1
  SIZEOF_UINT8_T = 1
  SIZEOF_INT16_T = 2
  SIZEOF_UINT16_T = 2
  SIZEOF_INT32_T = 4
  SIZEOF_UINT32_T = 4
  SIZEOF_INT64_T = 8
  SIZEOF_UINT64_T = 8

  ALIGN_VOIDP = 8
  ALIGN_CHAR = 1
  ALIGN_SHORT = 2
  ALIGN_INT = 4
  ALIGN_LONG = 8
  ALIGN_LONG_LONG = 8
  ALIGN_FLOAT = 4
  ALIGN_DOUBLE = 8
  ALIGN_SIZE_T = 8
  ALIGN_SSIZE_T = 8
  ALIGN_PTRDIFF_T = 8
  ALIGN_INTPTR_T = 8
  ALIGN_UINTPTR_T = 8
  ALIGN_BOOL = 1
  ALIGN_INT8_T = 1
  ALIGN_INT16_T = 2
  ALIGN_INT32_T = 4
  ALIGN_INT64_T = 8

  RTLD_LAZY = 1
  RTLD_NOW = 2
  RTLD_GLOBAL = RUBY_PLATFORM.include?("darwin") ? 8 : 256
  RTLD_LOCAL = RUBY_PLATFORM.include?("darwin") ? 4 : 0

  # libc's free: a free function C memory can carry (MRI's is ruby_xfree).
  RUBY_FREE = Fiddle.__free_addr #: Integer

  class Error < StandardError
  end

  class DLError < Error
  end

  class ClearedReferenceError < Error
  end

  # A loaded library: dlopen(3)'s handle; nil opens the program and every library it loaded.
  class Handle
    RTLD_LAZY = Fiddle::RTLD_LAZY
    RTLD_NOW = Fiddle::RTLD_NOW
    RTLD_GLOBAL = Fiddle::RTLD_GLOBAL
    RTLD_LOCAL = Fiddle::RTLD_LOCAL

    #: (?String?, ?Integer) -> void
    def initialize(lib = nil, flags = RTLD_LAZY | RTLD_GLOBAL)
      @lib = lib
      @handle = Fiddle.__dlopen(lib, flags)
      @closed = false
      @close_enabled = false
    end

    #: (String) -> Integer
    def sym(name)
      raise DLError, "closed handle" if @closed

      Fiddle.__dlsym(@handle, name)
    end

    #: (String) -> Integer
    def [](name) = sym(name)

    #: (String) -> Integer?
    def sym_defined?(name)
      sym(name)
    rescue DLError
      nil
    end

    #: () -> Integer
    def to_i = @handle

    #: () -> Pointer
    def to_ptr = Pointer.new(@handle)

    #: () -> String?
    def file_name = @lib

    #: () -> Integer
    def close
      raise DLError, "dlclose() called too many times" if @closed

      Fiddle.__dlclose(@handle)
      @closed = true
      0
    end

    #: () -> nil
    def enable_close
      @close_enabled = true
      nil
    end

    #: () -> nil
    def disable_close
      @close_enabled = false
      nil
    end

    #: () -> bool
    def close_enabled? = @close_enabled

    #: (String) -> Integer
    def self.sym(name) = DEFAULT.sym(name)

    #: (String) -> Integer
    def self.[](name) = DEFAULT.sym(name)

    #: (String) -> Integer?
    def self.sym_defined?(name) = DEFAULT.sym_defined?(name)

    DEFAULT = Handle.new(nil) #: Handle
  end

  # Memory at an address: a C pointer with an optional size and free function.
  class Pointer
    #: (untyped, ?Integer, ?untyped) -> void
    def initialize(addr, size = 0, free = nil)
      @addr = Fiddle.__addr(addr) #: Integer
      @size = size
      @free = Fiddle.__addr_or_nil(free) #: Integer?
      @freed = false
    end

    # ponytail: no block form (MRI frees after the block); add a __malloc_ overload when a program needs it.
    #: (Integer, ?untyped) -> Pointer
    def self.malloc(size, free = nil) = new(Fiddle.malloc(size), size, free)

    # A String is copied: rb2go Strings are immutable, so C's writes to it never reach the String.
    #: (untyped) -> Pointer
    def self.to_ptr(val)
      case val
      when Pointer then val
      when nil then new(0)
      when Integer then new(val)
      when String
        ptr = malloc(val.bytesize + 1)
        ptr[0, val.bytesize + 1] = val + "\0"
        ptr.size = val.bytesize
        ptr
      # ponytail: MRI's is the IO's FILE*; an fd number as an address would crash C. Add fdopen(3) by the fd's flags if needed.
      when IO then raise NotImplementedError, "rb2go: Fiddle::Pointer of an IO (MRI's FILE*)"
      when CStruct then val.to_ptr
      else raise TypeError, "can't convert #{val.class} into Fiddle::Pointer"
      end
    end

    #: (untyped) -> Pointer
    def self.[](val) = to_ptr(val)

    #: (Integer, Integer) -> String
    def self.read(addr, n) = %x{ rbFiddleRead(addr, n) }

    #: (Integer, String) -> void
    def self.write(addr, str) = %x{ rbFiddleWrite(addr, str) }

    #: () -> Integer
    def to_i = @addr

    #: () -> Integer
    def to_int = @addr

    #: () -> Integer
    def size = @size

    #: (Integer) -> Integer
    def size=(n)
      @size = n
    end

    #: () -> bool
    def null? = @addr.zero?

    #: () -> Function?
    def free
      f = @free
      f.nil? ? nil : Function.new(f, [TYPE_VOIDP], TYPE_VOID)
    end

    #: (untyped) -> untyped
    def free=(f)
      @free = Fiddle.__addr_or_nil(f)
      f
    end

    #: () -> nil
    def call_free
      f = @free
      if !f.nil? && !@freed && !@addr.zero?
        if f == RUBY_FREE
          Fiddle.free(@addr)
        else
          Function.new(f, [TYPE_VOIDP], TYPE_VOID).call(@addr)
        end
        @freed = true
      end
      nil
    end

    #: () -> bool
    def freed? = @freed

    #: () -> Pointer
    def ptr = Pointer.new(Fiddle.__word(@addr))

    #: () -> Pointer
    def +@ = ptr

    #: () -> Pointer
    def ref
      mem = Pointer.malloc(SIZEOF_VOIDP)
      mem[0, SIZEOF_VOIDP] = [@addr].pack("Q")
      mem
    end

    #: () -> Pointer
    def -@ = ref

    #: (Integer) -> Pointer
    def +(n) = Pointer.new(@addr + n, @size - n)

    #: (Integer) -> Pointer
    def -(n) = Pointer.new(@addr - n, @size + n)

    #: (untyped) -> bool
    def ==(other)
      case other
      when Pointer then other.to_i == @addr
      else false
      end
    end

    #: (untyped) -> bool
    def eql?(other) = self == other

    #: (untyped) -> Integer?
    def <=>(other)
      case other
      when Pointer then @addr <=> other.to_i
      end
    end

    #: () -> Integer
    def hash = @addr.hash

    #: (?Integer?) -> String
    def to_s(len = nil)
      len.nil? ? Fiddle.__cstring(@addr) : Pointer.read(@addr, len)
    end

    #: (?Integer?) -> String
    def to_str(len = nil) = Pointer.read(@addr, len.nil? ? @size : len)

    #: () -> String
    def inspect
      format("#<Fiddle::Pointer:0x%016x ptr=0x%016x size=%d free=0x%016x>", object_id, @addr, @size, @free || 0)
    end

    #: (Integer) -> Integer
    def [](offset)
      raise DLError, "NULL pointer dereference" if @addr.zero?

      Fiddle.__char(@addr + offset)
    end

    #: (Integer, Integer) -> String
    def __idx_2(offset, len)
      raise DLError, "NULL pointer dereference" if @addr.zero?

      Pointer.read(@addr + offset, len)
    end

    #: (Integer, Integer) -> Integer
    def []=(offset, byte)
      raise DLError, "NULL pointer dereference" if @addr.zero?

      Pointer.write(@addr + offset, [byte].pack("C"))
      byte
    end

    # Copies len bytes from a String, or from the memory a Pointer or address names.
    #: (Integer, Integer, untyped) -> untyped
    def __idxset_3(offset, len, src)
      raise DLError, "NULL pointer dereference" if @addr.zero?

      bytes = src.is_a?(String) ? src.byteslice(0, len).to_s : Pointer.read(Fiddle.__addr(src), len)
      Pointer.write(@addr + offset, bytes)
      src
    end
  end

  NULL = Pointer.new(0) #: Pointer

  # A struct's memory: typed member reads and writes at an offset.
  class CStructEntity < Pointer
    #: (Integer, Integer) -> Integer
    def __get_i(off, type) = Fiddle.__get_i(to_i + off, type)

    #: (Integer, Integer, Integer) -> void
    def __set_i(off, type, v) = Fiddle.__set_i(to_i + off, type, v)

    #: (Integer, Integer) -> Float
    def __get_f(off, type) = Fiddle.__get_f(to_i + off, type)

    #: (Integer, Integer, Float) -> void
    def __set_f(off, type, v) = Fiddle.__set_f(to_i + off, type, v)
  end

  # A C struct's Ruby wrapper; Importer#struct's literal members make each subclass at compile time (decision 326).
  class CStruct
    #: () -> CStructEntity
    def to_ptr = raise(NotImplementedError, "#{self.class}#to_ptr")

    #: () -> Integer
    def to_i = to_ptr.to_i

    #: () -> Array[String]
    def __members = []

    #: (String) -> untyped
    def [](name) = raise(ArgumentError, "no such member: #{name}")

    #: () -> Hash[String, untyped]
    def to_h
      h = {} #: Hash[String, untyped]
      __members.each { |m| h[m] = self[m] }
      h
    end

    #: () { (untyped) -> void } -> void
    def each = __members.each { |m| yield self[m] }

    #: () { (String, untyped) -> void } -> void
    def each_pair = __members.each { |m| yield m, self[m] }
  end

  # A C union's wrapper: every member at offset 0.
  class CUnion < CStruct
  end

  # A C function at an address, called with arguments converted by its TYPE_* list.
  class Function
    DEFAULT = 1

    attr_reader :ptr #: Integer
    attr_reader :abi #: Integer
    attr_reader :name #: String?

    #: (untyped, Array[Integer], Integer, ?Integer, ?name: String?, ?need_gvl: bool) -> void
    def initialize(ptr, args, ret_type, abi = DEFAULT, name: nil, need_gvl: false)
      @ptr = Fiddle.__addr(ptr)
      @args = args
      @ret = ret_type
      @abi = abi
      @name = name
      @need_gvl = need_gvl
      @fn = Fiddle.__prepare(@ptr, args, ret_type)
    end

    #: () -> bool
    def need_gvl? = @need_gvl

    #: () -> Integer
    def to_i = @ptr

    # Go converts Integers, Floats, Strings, true/false and nil (rbFiddleInvoke); objects become their address here.
    #: (*untyped) -> untyped
    def call(*args)
      i = 0
      while i < args.size
        a = args[i]
        case a
        when Integer, Float, String, nil, true, false then nil
        else
          t = @args.fetch(i, TYPE_VOIDP)
          args[i] = Fiddle.__addr(a) unless t == TYPE_BOOL || t == TYPE_FLOAT || t == TYPE_DOUBLE || t == TYPE_VOID
        end
        i += 1
      end
      Fiddle.__ret(@ret, Fiddle.__invoke(@fn, args))
    end
  end

  # A Ruby method C can call: a subclass defines call, or BlockCaller wraps a block.
  class Closure
    attr_reader :ctype #: Integer
    attr_reader :args #: Array[Integer]

    #: (Integer, Array[Integer], ?Integer) -> void
    def initialize(ctype, args, abi = Function::DEFAULT)
      @ctype = ctype
      @args = args
      @freed = false
      @ptr = Fiddle.__callback(args, ctype) { |vals| __invoke(vals) }
    end

    #: (*untyped) -> untyped
    def call(*args)
      raise NotImplementedError, "#{self.class}#call"
    end

    #: (Array[untyped]) -> untyped
    def __invoke(vals)
      i = 0
      while i < vals.size
        t = @args.fetch(i, TYPE_VOIDP)
        vals[i] = Fiddle.__ret(t, vals[i]) if t == TYPE_VOIDP || t == TYPE_CONST_STRING
        i += 1
      end
      Fiddle.__arg(@ctype, call(*vals), [])
    end

    #: () -> Integer
    def to_i = @ptr

    # purego never releases a callback (decision 326).
    #: () -> nil
    def free
      @freed = true
      nil
    end

    #: () -> bool
    def freed? = @freed

    # ponytail: no block form of create (it frees after the block, a no-op here).
    #: (Integer, Array[Integer], ?Integer) -> Closure
    def self.create(ctype, args, abi = Function::DEFAULT) = new(ctype, args, abi)

    # A Closure that runs a block.
    class BlockCaller < Closure
      #: (Integer, Array[Integer], ?Integer) { (*untyped) -> untyped } -> void
      def initialize(ctype, args, abi = Function::DEFAULT, &block)
        @run = ->(vals) { block.call(*vals) } #: ^(Array[untyped]) -> untyped
        super(ctype, args, abi)
      end

      #: (*untyped) -> untyped
      def call(*args) = @run.call(args)
    end
  end

  #: () -> Integer
  def self.__free_addr = %x{ rbFiddleFreeAddr() }

  #: (String?, Integer) -> Integer
  def self.__dlopen(lib, flags) = %x{ rbFiddleDlopen(lib, flags) }

  #: (Integer, String) -> Integer
  def self.__dlsym(h, name) = %x{ rbFiddleDlsym(h, name) }

  #: (Integer) -> void
  def self.__dlclose(h) = %x{ rbFiddleDlclose(h) }

  #: (Integer) -> Integer
  def self.__word(addr) = %x{ rbFiddleWord(addr) }

  #: (Integer) -> Integer
  def self.__char(addr) = %x{ rbFiddleChar(addr) }

  #: (Integer, Integer) -> Integer
  def self.__get_i(addr, t) = %x{ rbFiddleGetInt(addr, t) }

  #: (Integer, Integer, Integer) -> void
  def self.__set_i(addr, t, v) = %x{ rbFiddleSetInt(addr, t, v) }

  #: (Integer, Integer) -> Float
  def self.__get_f(addr, t) = %x{ rbFiddleGetFloat(addr, t) }

  #: (Integer, Integer, Float) -> void
  def self.__set_f(addr, t, v) = %x{ rbFiddleSetFloat(addr, t, v) }

  #: (Integer) -> String
  def self.__cstring(addr) = %x{ rbFiddleCString(addr) }

  #: (Integer, Array[Integer], Integer) -> untyped
  def self.__prepare(addr, args, ret) = %x{ rbFiddlePrepare(addr, args, ret) }

  #: (untyped, Array[untyped]) -> untyped
  def self.__invoke(fn, vals) = %x{ rbFiddleInvoke(fn, vals) }

  # What an extern method calls (decision 329): f.call(a0, ..., a(n-1)), without call's *args Array when Go can
  # convert every argument itself and f is a plain Function; __fast answers f itself (never a C result) otherwise.
  #: (Function, Integer, ?untyped, ?untyped, ?untyped, ?untyped, ?untyped, ?untyped, ?untyped, ?untyped) -> untyped
  def self.__call(f, n, a0 = nil, a1 = nil, a2 = nil, a3 = nil, a4 = nil, a5 = nil, a6 = nil, a7 = nil)
    r = __fast(f, n, a0, a1, a2, a3, a4, a5, a6, a7)
    return r unless r.equal?(f)

    f.call(*[a0, a1, a2, a3, a4, a5, a6, a7].first(n))
  end

  #: (Function, Integer, untyped, untyped, untyped, untyped, untyped, untyped, untyped, untyped) -> untyped
  def self.__fast(f, n, a0, a1, a2, a3, a4, a5, a6, a7) = %x{
    a := [8]any{a0, a1, a2, a3, a4, a5, a6, a7}
    return rbFiddleFast(f, a[:n]...)
  }

  # __call for one to three arguments, the common arities: no eight-slot copy.
  #: (Function, untyped) -> untyped
  def self.__call1(f, a0)
    r = __fast1(f, a0)
    r.equal?(f) ? f.call(a0) : r
  end

  #: (Function, untyped, untyped) -> untyped
  def self.__call2(f, a0, a1)
    r = __fast2(f, a0, a1)
    r.equal?(f) ? f.call(a0, a1) : r
  end

  #: (Function, untyped, untyped, untyped) -> untyped
  def self.__call3(f, a0, a1, a2)
    r = __fast3(f, a0, a1, a2)
    r.equal?(f) ? f.call(a0, a1, a2) : r
  end

  #: (Function, untyped) -> untyped
  def self.__fast1(f, a0) = %x{ rbFiddleFast(f, a0) }

  #: (Function, untyped, untyped) -> untyped
  def self.__fast2(f, a0, a1) = %x{ rbFiddleFast(f, a0, a1) }

  #: (Function, untyped, untyped, untyped) -> untyped
  def self.__fast3(f, a0, a1, a2) = %x{ rbFiddleFast(f, a0, a1, a2) }

  #: (Array[Integer], Integer) { (Array[untyped]) -> untyped } -> Integer
  def self.__callback(args, ret) = %x{ rbFiddleCallback(args, ret, blk) }

  #: (untyped) -> Integer
  def self.__addr(x)
    case x
    when Integer then x
    when nil then 0
    when Pointer then x.to_i
    when Closure then x.to_i
    when Function then x.to_i
    when Handle then x.to_i
    when CStruct then x.to_ptr.to_i
    else raise ArgumentError, "unknown argument type: #{x.class}"
    end
  end

  #: (untyped) -> Integer?
  def self.__addr_or_nil(x) = x.nil? ? nil : __addr(x)

  # One argument as Fiddle's C conversion sees it; a String passed as a pointer is a temporary C copy.
  #: (Integer, untyped, Array[Integer]) -> untyped
  def self.__arg(type, a, temps)
    case type
    when TYPE_VOID then nil
    when TYPE_BOOL then a ? true : false
    when TYPE_FLOAT, TYPE_DOUBLE then a
    when TYPE_VOIDP, TYPE_CONST_STRING
      if a.is_a?(String)
        mem = malloc(a.bytesize + 1)
        Pointer.write(mem, a + "\0")
        temps << mem
        mem
      else
        __addr(a)
      end
    else
      case a
      when Integer, Float then a
      when true then 1
      when false then 0
      else __addr(a)
      end
    end
  end

  # One C value as Ruby sees it.
  #: (Integer, untyped) -> untyped
  def self.__ret(type, v)
    case type
    when TYPE_VOID then nil
    when TYPE_VOIDP then Pointer.new(v)
    when TYPE_CONST_STRING then v == 0 ? nil : Pointer.new(v).to_s
    else v
    end
  end

  #: (String?) -> Handle
  def self.dlopen(lib)
    Handle.new(lib)
  rescue DLError => e
    raise unless RUBY_PLATFORM.include?("linux") && (m = e.message.match(%r{\A(/.+?): (?:invalid ELF header|file too short)}))

    # A Linux libfoo.so can be a linker script naming the real library.
    File.foreach(m[1].to_s) do |line|
      if (g = line.match(/\A\s*(?:INPUT|GROUP)\s*\(\s*([^\s,)]+)/))
        first = g[1].to_s
        first = "lib#{first[2..]}.so" if first.start_with?("-l")
        return dlopen(first)
      end
    end
    raise
  end

  #: (Integer) -> Integer
  def self.malloc(size) = %x{ rbFiddleMalloc(size) }

  #: (Integer, Integer) -> Integer
  def self.realloc(addr, size) = %x{ rbFiddleRealloc(addr, size) }

  #: (Integer) -> void
  def self.free(addr) = %x{ rbFiddleFree(addr) }

  #: () -> Integer?
  def self.last_error = Thread.current[:__FIDDLE_LAST_ERROR__]

  #: (Integer?) -> Integer?
  def self.last_error=(err)
    Thread.current[:__FIDDLE_LAST_ERROR__] = err
  end
end
