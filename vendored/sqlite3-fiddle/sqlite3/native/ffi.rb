# rbs_inline: enabled

# Literal extern signatures, not built dynamically: rugaroo compiles the literals (decision 326).

require "fiddle"
require "fiddle/import"

module SQLite3
  #: () -> String
  def self.__lib_name # host_os: JRuby's RUBY_PLATFORM is "java"
    return "libsqlite3.so.0" unless RbConfig::CONFIG["host_os"].to_s.include?("darwin")

    brew = ["/opt/homebrew/opt/sqlite/lib/libsqlite3.dylib", "/usr/local/opt/sqlite/lib/libsqlite3.dylib"].find { |path| File.exist?(path) }
    brew || "libsqlite3.dylib" # the system library omits load_extension and predates deserialize's 3.54 cutoff
  end

  module C
    extend Fiddle::Importer
    HANDLE = Fiddle.dlopen(ENV["SQLITE3_FIDDLE_LIB"] || SQLite3.__lib_name) #: Fiddle::Handle
    dlload HANDLE

    if defined?(::Binding) # rugaroo compiles the externs itself
      #: (String, Integer, Array[Integer], ?untyped) -> Fiddle::Function
      def self.import_function(name, ctype, argtype, call_type = nil) # keeps the GVL, as the C extension does: fiddle's default hands it off on every call
        Fiddle::Function.new(handler.sym(name), argtype, ctype, Fiddle::Function::DEFAULT, name: name, need_gvl: true)
      end
    end
    TRANSIENT = -1 # SQLITE_TRANSIENT: sqlite copies the bytes before the call returns

    extern "void *sqlite3_libversion(void)"
    extern "int sqlite3_libversion_number(void)"
    extern "int sqlite3_threadsafe(void)"
    extern "int sqlite3_status(int, void *, void *, int)"
    extern "void sqlite3_free(void *)"
    extern "void *sqlite3_malloc64(unsigned long long)"
    extern "int sqlite3_open_v2(void *, void *, int, void *)"
    extern "int sqlite3_open16(void *, void *)"
    extern "int sqlite3_close_v2(void *)"
    extern "void *sqlite3_errmsg(void *)"
    extern "int sqlite3_errcode(void *)"
    extern "int sqlite3_extended_errcode(void *)"
    extern "int sqlite3_extended_result_codes(void *, int)"
    extern "int sqlite3_db_release_memory(void *)"
    extern "int sqlite3_file_control(void *, void *, int, void *)"
    extern "int sqlite3_prepare_v2(void *, void *, int, void *, void *)"
    extern "int sqlite3_step(void *)"
    extern "int sqlite3_reset(void *)"
    extern "int sqlite3_finalize(void *)"
    extern "int sqlite3_clear_bindings(void *)"
    extern "int sqlite3_stmt_busy(void *)"
    extern "void *sqlite3_db_handle(void *)"
    extern "int sqlite3_stmt_status(void *, int, int)"
    extern "int sqlite3_column_count(void *)"
    extern "void *sqlite3_column_name(void *, int)"
    extern "void *sqlite3_column_decltype(void *, int)"
    extern "int sqlite3_column_type(void *, int)"
    extern "long long sqlite3_column_int64(void *, int)"
    extern "double sqlite3_column_double(void *, int)"
    extern "void *sqlite3_column_text(void *, int)"
    extern "void *sqlite3_column_blob(void *, int)"
    extern "int sqlite3_column_bytes(void *, int)"
    extern "int sqlite3_bind_parameter_count(void *)"
    extern "int sqlite3_bind_parameter_index(void *, void *)"
    extern "void *sqlite3_bind_parameter_name(void *, int)"
    extern "int sqlite3_bind_text(void *, int, void *, int, void *)"
    extern "int sqlite3_bind_text16(void *, int, void *, int, void *)"
    extern "int sqlite3_bind_blob(void *, int, void *, int, void *)"
    extern "int sqlite3_bind_int64(void *, int, long long)"
    extern "int sqlite3_bind_double(void *, int, double)"
    extern "int sqlite3_bind_null(void *, int)"
    extern "void *sqlite3_sql(void *)"
    extern "void *sqlite3_expanded_sql(void *)"
    extern "int sqlite3_changes(void *)"
    extern "int sqlite3_total_changes(void *)"
    extern "long long sqlite3_last_insert_rowid(void *)"
    extern "int sqlite3_get_autocommit(void *)"
    extern "int sqlite3_busy_timeout(void *, int)"
    extern "int sqlite3_busy_handler(void *, void *, void *)"
    extern "void sqlite3_progress_handler(void *, int, void *, void *)"
    extern "void *sqlite3_trace(void *, void *, void *)"
    extern "int sqlite3_set_authorizer(void *, void *, void *)"
    extern "void *sqlite3_db_filename(void *, void *)"
    extern "int sqlite3_complete(void *)"
    extern "void sqlite3_interrupt(void *)"
    extern "int sqlite3_exec(void *, void *, void *, void *, void *)"
    extern "int sqlite3_create_function(void *, void *, int, int, void *, void *, void *, void *)"
    extern "int sqlite3_create_collation(void *, void *, int, void *, void *)"
    extern "void *sqlite3_aggregate_context(void *, int)"
    extern "int sqlite3_value_type(void *)"
    extern "long long sqlite3_value_int64(void *)"
    extern "double sqlite3_value_double(void *)"
    extern "void *sqlite3_value_text(void *)"
    extern "void *sqlite3_value_blob(void *)"
    extern "int sqlite3_value_bytes(void *)"
    extern "void sqlite3_result_int64(void *, long long)"
    extern "void sqlite3_result_double(void *, double)"
    extern "void sqlite3_result_text(void *, void *, int, void *)"
    extern "void sqlite3_result_blob(void *, void *, int, void *)"
    extern "void sqlite3_result_null(void *)"
    extern "void sqlite3_result_error(void *, void *, int)"
    extern "void *sqlite3_backup_init(void *, void *, void *, void *)"
    extern "int sqlite3_backup_step(void *, int)"
    extern "int sqlite3_backup_finish(void *)"
    extern "int sqlite3_backup_remaining(void *)"
    extern "int sqlite3_backup_pagecount(void *)"
    extern "void *sqlite3_serialize(void *, void *, void *, unsigned int)"

    #: (String, Array[Integer], Integer) -> Fiddle::Function?
    def self.optional(name, args, ret) = HANDLE.sym_defined?(name) ? Fiddle::Function.new(HANDLE[name], args, ret) : nil # sym_defined?: address/nil on MRI, bool on JRuby

    ERROR_OFFSET = optional("sqlite3_error_offset", [Fiddle::TYPE_VOIDP], Fiddle::TYPE_INT) #: Fiddle::Function?
    DB_NAME = optional("sqlite3_db_name", [Fiddle::TYPE_VOIDP, Fiddle::TYPE_INT], Fiddle::TYPE_VOIDP) #: Fiddle::Function?
    if defined?(::Binding) # sqlite3_db_config is variadic (on arm64 Darwin its varargs go on the stack); rugaroo's purego cannot call one
      DB_CONFIG = HANDLE.sym_defined?("sqlite3_db_config") ? Fiddle::Function.new(HANDLE["sqlite3_db_config"], [Fiddle::TYPE_VOIDP, Fiddle::TYPE_INT, Fiddle::TYPE_VARIADIC], Fiddle::TYPE_INT) : nil #: Fiddle::Function?
    else
      DB_CONFIG = nil #: Fiddle::Function?
    end

    #: (Fiddle::Pointer) -> String?
    def self.cstr(p) = p.null? ? nil : p.to_s.b # rb_str_new2: binary

    #: (Fiddle::Pointer) -> String?
    def self.ustr(p) = p.null? ? nil : p.to_s.force_encoding(Encoding::UTF_8) # rb_utf8_str_new_cstr

    #: (Fiddle::Pointer, Integer) -> String
    def self.bytes(p, n) = n <= 0 || p.null? ? "".b : Fiddle::Pointer.read(p.to_i, n) # Pointer.read's String is binary; a TEXT value is UTF-8 (rugaroo derives encoding from the bytes, decision 136).

    #: (Fiddle::Pointer, Integer) -> String
    def self.text(p, n) = bytes(p, n).force_encoding(Encoding::UTF_8)

    #: (String) -> String
    def self.z(s) = "#{s}\0" # A NUL-terminated copy for a `const char *` parameter: Fiddle passes a String's bytes, and a shared Ruby String need not end in NUL.

    #: (untyped) -> String
    def self.string(v) = String.try_convert(v) || raise(TypeError, "no implicit conversion of #{v.nil? ? "nil" : v.class} into String") # StringValue

    #: (String) -> String
    def self.zc(s) # StringValueCStr
      raise ArgumentError, "string contains null byte" if s.include?("\0")

      z(s)
    end

    #: () { (Fiddle::Pointer) -> untyped } -> Integer
    def self.out # The pointer an out-parameter (`sqlite3 **`, `const char **`) was set to.
      p = Fiddle::Pointer.malloc(Fiddle::SIZEOF_VOIDP, Fiddle::RUBY_FREE)
      p[0, Fiddle::SIZEOF_VOIDP] = "\0" * Fiddle::SIZEOF_VOIDP
      begin
        yield p
        p.ptr.to_i
      ensure
        # call_free, not Fiddle.free: JRuby's Fiddle.free wants a Pointer, rugaroo's an Integer.
        p.call_free
      end
    end

    #: (Integer) -> untyped
    def self.value(v) # sqlite3val2rb: a function argument as Ruby, TEXT as frozen UTF-8 and BLOB as a frozen binary String.
      case sqlite3_value_type(v)
      when 1 then sqlite3_value_int64(v)
      when 2 then sqlite3_value_double(v)
      when 3 then text(sqlite3_value_text(v), sqlite3_value_bytes(v)).freeze
      when 4 then bytes(sqlite3_value_blob(v), sqlite3_value_bytes(v)).freeze
      end
    end

    #: (Integer, Integer) -> Array[untyped]
    def self.values(argc, argv)
      args = [] #: Array[untyped]
      i = 0
      while i < argc
        args << value(Fiddle::Pointer.new(argv + (i * Fiddle::SIZEOF_VOIDP)).ptr.to_i)
        i += 1
      end
      args
    end

    #: (Integer, untyped) -> void
    def self.result(ctx, val) # set_sqlite3_func_result. A Bignum returns as a double, as the C extension's T_BIGNUM case falls through on 64-bit longs.
      case val
      when nil then sqlite3_result_null(ctx)
      when Integer
        if val.bit_length < 63 # a Fixnum on 64-bit MRI
          sqlite3_result_int64(ctx, val)
        else
          sqlite3_result_double(ctx, val.to_f)
        end
      when Float then sqlite3_result_double(ctx, val)
      when Blob # its own arm: rugaroo's `when String` does not match a String subclass
        blob = val.to_s
        sqlite3_result_blob(ctx, blob, blob.bytesize, TRANSIENT)
      when String
        if val.instance_of?(Blob) || val.encoding == Encoding::BINARY
          sqlite3_result_blob(ctx, val, val.bytesize, TRANSIENT)
        else
          sqlite3_result_text(ctx, val, val.bytesize, TRANSIENT)
        end
      else raise RuntimeError, "can't return #{val.class}"
      end
    end
  end
end

# Optional entry points, absent from older or OMIT_ builds (macOS' own libsqlite3 has no load_extension): declared from file-level code, so a
# class body's `C.respond_to?(:name)` guard sees them on MRI and folds to absent in rugaroo, as the C extension's #ifdef HAVE_* does.
[
  "int sqlite3_load_extension(void *, void *, void *, void *)",
  "int sqlite3_enable_load_extension(void *, int)",
  "int sqlite3_deserialize(void *, void *, void *, long long, long long, unsigned int)",
  "void *sqlite3_column_database_name(void *, int)"
].each do |signature|
  SQLite3::C.extern(signature)
rescue Fiddle::DLError
  nil
end
