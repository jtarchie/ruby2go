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

    #: (Symbol, Symbol) -> Symbol
    def typedef(old, add)
      (@ffi_types ||= {})[add] = old
    end

    #: (Symbol) -> Integer
    def __ffi_code(type)
      t = type
      t = (@ffi_types ||= {}).fetch(t, t) while (@ffi_types ||= {}).key?(t)
      code = TYPES[t]
      raise TypeError, "unable to resolve type '#{type}'" if code.nil?

      code
    end

    #: (*untyped) -> untyped
    def attach_function(*args)
      name = args[0].to_s
      named = args[1].is_a?(Symbol) || args[1].is_a?(String)
      cname = named ? args[1].to_s : name
      arg_types = (named ? args[2] : args[1]) #: Array[Symbol]
      ret_type = (named ? args[3] : args[2]) #: Symbol
      sym = nil #: Integer?
      ffi_libraries.each do |h|
        sym = h&.sym(cname) rescue nil # rubocop:disable Style/RescueModifier
        break if sym
      end
      raise NotFoundError.new(cname, ffi_libraries.map { |h| h&.file_name.to_s }) if sym.nil?

      codes = arg_types.map { |t| __ffi_code(t) } #: Array[Integer]
      raise NotImplementedError, "varargs functions are not supported (decision 326)" if codes.include?(Fiddle::TYPE_VARIADIC)

      f = Fiddle::Function.new(sym, codes, __ffi_code(ret_type), name: cname)
      (@ffi_funcs ||= {})[name] = f
      @ffi_cache = nil
      f
    end

    # Slot read, not a hash of the name per call (decision 329).
    #: (Integer, String) -> Fiddle::Function
    def __ffi_func_at(k, name)
      cache = (@ffi_cache ||= []) #: Array[Fiddle::Function?]
      cache[k] || (cache[k] = (@ffi_funcs ||= {}).fetch(name))
    end
  end
end
