# rbs_inline: enabled

# fiddle/import: MRI's CParser and Importer; extern/bind's methods are compiled from their literal signature (decision 326).

require_relative "fiddle"

module Fiddle
  # C declarations to Fiddle types (MRI's lib/fiddle/cparser.rb).
  module CParser
    #: (String, ?Hash[String, String]?) -> [String, Integer, Array[Integer]]
    def parse_signature(signature, tymap = nil)
      tymap ||= {}
      sig = __compact(signature)
      if (m = sig.match(/^(?:[\w*\s]+)\(\*(\w+)\((.*?)\)\)(?:\[\w*\]|\(.*?\));?$/))
        return [m[1].to_s, TYPE_VOIDP, __split_arguments(m[2].to_s).map { |arg| parse_ctype(arg, tymap) }]
      end
      if (m = sig.match(/^([\w*\s]+[*\s])(\w+)\((.*?)\);?$/))
        return [m[2].to_s, parse_ctype(m[1].to_s.strip, tymap), __split_arguments(m[3].to_s).map { |arg| parse_ctype(arg, tymap) }]
      end
      raise RuntimeError, "can't parse the function prototype: #{signature}"
    end

    #: (String, ?Hash[String, String]?) -> Integer
    def parse_ctype(ty, tymap = nil)
      tymap ||= {}
      ty = ty.sub(/\Aconst\s+/, "")
      case ty
      when "void" then TYPE_VOID
      when /\A(?:(?:signed\s+)?long\s+long(?:\s+int\s+)?|int64_t)(?:\s+\w+)?\z/ then TYPE_LONG_LONG
      when /\A(?:unsigned\s+long\s+long(?:\s+int\s+)?|uint64_t)(?:\s+\w+)?\z/ then TYPE_ULONG_LONG
      when /\Aunsigned\s+long(?:\s+int\s+)?(?:\s+\w+)?\z/, /\Aunsigned\s+int\s+long(?:\s+\w+)?\z/,
           /\Along(?:\s+int)?\s+unsigned(?:\s+\w+)?\z/, /\Aint\s+unsigned\s+long(?:\s+\w+)?\z/,
           /\A(?:int\s+)?long\s+unsigned(?:\s+\w+)?\z/
        TYPE_ULONG
      when /\A(?:signed\s+)?long(?:\s+int\s+)?(?:\s+\w+)?\z/, /\A(?:signed\s+)?int\s+long(?:\s+\w+)?\z/,
           /\Along(?:\s+int)?\s+signed(?:\s+\w+)?\z/
        TYPE_LONG
      when /\Aunsigned\s+short(?:\s+int\s+)?(?:\s+\w+)?\z/, /\Aunsigned\s+int\s+short(?:\s+\w+)?\z/,
           /\Ashort(?:\s+int)?\s+unsigned(?:\s+\w+)?\z/, /\Aint\s+unsigned\s+short(?:\s+\w+)?\z/,
           /\A(?:int\s+)?short\s+unsigned(?:\s+\w+)?\z/
        TYPE_USHORT
      when /\A(?:signed\s+)?short(?:\s+int\s+)?(?:\s+\w+)?\z/, /\A(?:signed\s+)?int\s+short(?:\s+\w+)?\z/,
           /\Aint\s+(?:signed\s+)?short(?:\s+\w+)?\z/
        TYPE_SHORT
      when /\A(?:signed\s+)?int(?:\s+\w+)?\z/ then TYPE_INT
      when /\A(?:unsigned\s+int|uint)(?:\s+\w+)?\z/ then TYPE_UINT
      when /\A(?:signed\s+)?char(?:\s+\w+)?\z/ then TYPE_CHAR
      when /\Aunsigned\s+char(?:\s+\w+)?\z/ then TYPE_UCHAR
      when /\Aint8_t(?:\s+\w+)?\z/ then TYPE_INT8_T
      when /\Auint8_t(?:\s+\w+)?\z/ then TYPE_UINT8_T
      when /\Aint16_t(?:\s+\w+)?\z/ then TYPE_INT16_T
      when /\Auint16_t(?:\s+\w+)?\z/ then TYPE_UINT16_T
      when /\Aint32_t(?:\s+\w+)?\z/ then TYPE_INT32_T
      when /\Auint32_t(?:\s+\w+)?\z/ then TYPE_UINT32_T
      when /\Afloat(?:\s+\w+)?\z/ then TYPE_FLOAT
      when /\Adouble(?:\s+\w+)?\z/ then TYPE_DOUBLE
      when /\Asize_t(?:\s+\w+)?\z/ then TYPE_SIZE_T
      when /\Assize_t(?:\s+\w+)?\z/ then TYPE_SSIZE_T
      when /\Aptrdiff_t(?:\s+\w+)?\z/ then TYPE_PTRDIFF_T
      when /\Aintptr_t(?:\s+\w+)?\z/ then TYPE_INTPTR_T
      when /\Auintptr_t(?:\s+\w+)?\z/ then TYPE_UINTPTR_T
      when /\Abool(?:\s+\w+)?\z/ then TYPE_BOOL
      when /\*/, /\[[\s\d]*\]/ then TYPE_VOIDP
      when "..." then TYPE_VARIADIC
      else
        name = ty.split(" ", 2)[0].to_s
        orig = tymap[name]
        raise DLError, "unknown type: #{name}" if orig.nil?

        parse_ctype(orig, tymap)
      end
    end

    #: (String, ?String) -> Array[String]
    def __split_arguments(arguments, sep = ",")
      return [] if arguments.strip == "void"

      arguments.scan(/([\w*\s]+\(\*\w*\)\(.*?\)|[\w*\s\[\]]+|\.\.\.)(?:#{sep}\s*|\z)/).map { |m| m[0].to_s }
    end

    #: (String) -> String
    def __compact(signature)
      signature.gsub(/\s+/, " ").gsub(/\s*([()\[\]*,;])\s*/, '\1').strip
    end
  end

  # The libraries an Importer looks symbols up in, first match wins.
  class CompositeHandler
    #: (Array[Handle?]) -> void
    def initialize(handlers)
      @handlers = handlers
    end

    #: () -> Array[Handle?]
    def handlers = @handlers

    #: (String) -> Integer?
    def sym(symbol)
      @handlers.each do |handle|
        next if handle.nil?

        begin
          return handle.sym(symbol)
        rescue DLError
          nil
        end
      end
      nil
    end

    #: (String) -> Integer?
    def [](symbol) = sym(symbol)
  end

  # The DSL a module extends to bind C functions as its own methods.
  module Importer
    include CParser

    #: (*untyped) -> void
    def dlload(*libs)
      handles = [] #: Array[Handle?]
      libs.each do |lib|
        case lib
        when nil then handles << nil
        when Handle then handles << lib
        when CompositeHandler then handles.concat(lib.handlers)
        else handles << Fiddle.dlopen(lib.to_s)
        end
      end
      @handler = CompositeHandler.new(handles) #: CompositeHandler?
      @func_map = {} #: Hash[String, Function]
      @type_alias = {} #: Hash[String, String]
    end

    #: () -> Hash[String, String]
    def __type_alias = (@type_alias ||= {})

    #: (String, String) -> String
    def typealias(alias_type, orig_type)
      __type_alias[alias_type] = orig_type
    end

    #: (String) -> Integer
    def sizeof(ty)
      t = parse_ctype(ty, __type_alias).abs
      case t
      when TYPE_CHAR then SIZEOF_CHAR
      when TYPE_SHORT then SIZEOF_SHORT
      when TYPE_INT then SIZEOF_INT
      when TYPE_LONG then SIZEOF_LONG
      when TYPE_LONG_LONG then SIZEOF_LONG_LONG
      when TYPE_FLOAT then SIZEOF_FLOAT
      when TYPE_DOUBLE then SIZEOF_DOUBLE
      when TYPE_VOIDP then SIZEOF_VOIDP
      when TYPE_CONST_STRING then SIZEOF_CONST_STRING
      when TYPE_BOOL then SIZEOF_BOOL
      else raise DLError, "unknown type: #{t}"
      end
    end

    #: () -> CompositeHandler
    def handler
      h = @handler
      raise "call dlload before importing symbols and functions" if h.nil?

      h
    end

    #: (String) -> Function?
    def [](name) = (@func_map ||= {})[name]

    #: (String) -> Function
    def __fiddle_func(name) = (@func_map ||= {}).fetch(name)

    # An extern method's Function by the slot the compiler gave it (decision 329): an Array read, not a hash of the
    # name per call. extern and bind empty the cache, so a redefined name is seen, as MRI's @func_map lookup sees it.
    #: (Integer, String) -> Function
    def __fiddle_func_at(k, name)
      cache = (@func_cache ||= []) #: Array[Function?]
      cache[k] || (cache[k] = __fiddle_func(name))
    end

    #: (String, *untyped) -> Function
    def extern(signature, *opts)
      symname, ctype, argtype = parse_signature(signature, __type_alias)
      f = import_function(symname, ctype, argtype)
      (@func_map ||= {})[symname.sub(/@.+/, "")] = f
      @func_cache = nil
      f
    end

    #: (String, *untyped) { (*untyped) -> untyped } -> Function
    def bind(signature, *opts, &blk)
      name, ctype, argtype = parse_signature(signature, __type_alias)
      f = bind_function(name, ctype, argtype, &blk)
      (@func_map ||= {})[name] = f
      @func_cache = nil
      f
    end

    #: (String) -> Pointer
    def import_symbol(name)
      addr = handler.sym(name)
      raise DLError, "cannot find the symbol: #{name}" if addr.nil?

      Pointer.new(addr)
    end

    #: (String, Integer, Array[Integer], ?untyped) -> Function
    def import_function(name, ctype, argtype, call_type = nil)
      addr = handler.sym(name)
      raise DLError, "cannot find the function: #{name}()" if addr.nil?

      Function.new(addr, argtype, ctype, Function::DEFAULT, name: name)
    end

    #: (String, Integer, Array[Integer], ?untyped) { (*untyped) -> untyped } -> Function
    def bind_function(name, ctype, argtype, call_type = nil, &block)
      closure = Closure::BlockCaller.new(ctype, argtype, Function::DEFAULT, &block)
      Function.new(closure, argtype, ctype, Function::DEFAULT, name: name)
    end
  end
end
