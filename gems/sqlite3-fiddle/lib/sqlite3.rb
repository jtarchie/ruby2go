# rbs_inline: enabled

# sqlite3-fiddle: the sqlite3 gem's API in pure Ruby over Fiddle, so it runs on MRI and compiles under rb2go (decision 328).

require "fiddle"
require "fiddle/import"

module SQLite3
  VERSION = "2.10.1"

  module Constants
    module Open
      READONLY = 0x00000001
      READWRITE = 0x00000002
      CREATE = 0x00000004
      DELETEONCLOSE = 0x00000008
      EXCLUSIVE = 0x00000010
      AUTOPROXY = 0x00000020
      URI = 0x00000040
      MEMORY = 0x00000080
      MAIN_DB = 0x00000100
      TEMP_DB = 0x00000200
      TRANSIENT_DB = 0x00000400
      MAIN_JOURNAL = 0x00000800
      TEMP_JOURNAL = 0x00001000
      SUBJOURNAL = 0x00002000
      SUPER_JOURNAL = 0x00004000
      NOMUTEX = 0x00008000
      FULLMUTEX = 0x00010000
      SHAREDCACHE = 0x00020000
      PRIVATECACHE = 0x00040000
      WAL = 0x00080000
      NOFOLLOW = 0x01000000
      EXRESCODE = 0x02000000
    end

    module TextRep
      UTF8 = 1
      UTF16LE = 2
      UTF16BE = 3
      UTF16 = 4
      ANY = 5
      DETERMINISTIC = 0x800
    end

    module ColumnType
      INTEGER = 1
      FLOAT = 2
      TEXT = 3
      BLOB = 4
      NULL = 5
    end

    module ErrorCode
      OK = 0
      ERROR = 1
      INTERNAL = 2
      PERM = 3
      ABORT = 4
      BUSY = 5
      LOCKED = 6
      NOMEM = 7
      READONLY = 8
      INTERRUPT = 9
      IOERR = 10
      CORRUPT = 11
      NOTFOUND = 12
      FULL = 13
      CANTOPEN = 14
      PROTOCOL = 15
      EMPTY = 16
      SCHEMA = 17
      TOOBIG = 18
      CONSTRAINT = 19
      MISMATCH = 20
      MISUSE = 21
      NOLFS = 22
      AUTH = 23
      FORMAT = 24
      RANGE = 25
      NOTADB = 26
      ROW = 100
      DONE = 101
    end
  end

  # Marks a String to bind as a BLOB. Differs from MRI: not a String subclass, so a `[name, Blob.new(data)]` literal stays
  # `Array[String | Blob]` instead of widening the Blob to a String (decision 150); columns read back as plain Strings, as MRI's do.
  class Blob
    #: (String) -> void
    def initialize(str)
      @str = str
    end

    #: () -> String
    def to_s = @str

    #: () -> String
    def to_str = @str

    #: () -> Integer
    def bytesize = @str.bytesize

    #: () -> Integer
    def length = @str.length

    #: () -> Integer
    def size = @str.length

    #: () -> Array[Integer]
    def bytes = @str.bytes

    #: (untyped) -> bool
    def ==(other)
      case other
      when Blob then other.to_s == @str
      when String then other == @str
      else false
      end
    end

    #: (untyped) -> bool
    def eql?(other) = self == other

    #: () -> Integer
    def hash = @str.hash

    #: () -> String
    def inspect = @str.inspect
  end

  class Exception < StandardError
    attr_reader :code #: Integer
    attr_reader :sql #: String?
    attr_reader :sql_offset #: Integer

    #: (?String) -> void
    def initialize(msg = "SQLite3::Exception")
      super
      @code = 0
      @sql = nil
      @sql_offset = -1
    end

    #: (Integer, String?, Integer) -> void
    def __set(code, sql, offset)
      @code = code
      @sql = sql
      @sql_offset = offset
    end

    #: () -> String
    def message
      err = sql_error
      err.nil? ? super : "#{super}:\n#{err}"
    end

    #: () -> String?
    def sql_error
      sql = @sql
      return nil if sql.nil?
      return sql.chomp unless @sql_offset >= 0

      offset = @sql_offset
      out = [] #: Array[String]
      sql.lines.each do |line|
        if offset >= 0 && line.length > offset
          out << line.chomp
          out << ((" " * offset) + "^")
          offset = -1
        else
          offset -= line.length if offset >= 0
          out << line.chomp
        end
      end
      out.join("\n")
    end
  end

  class SQLException < Exception; end
  class InternalException < Exception; end
  class PermissionException < Exception; end
  class AbortException < Exception; end
  class BusyException < Exception; end
  class LockedException < Exception; end
  class MemoryException < Exception; end
  class ReadOnlyException < Exception; end
  class InterruptException < Exception; end
  class IOException < Exception; end
  class CorruptException < Exception; end
  class NotFoundException < Exception; end
  class FullException < Exception; end
  class CantOpenException < Exception; end
  class ProtocolException < Exception; end
  class EmptyException < Exception; end
  class SchemaChangedException < Exception; end
  class TooBigException < Exception; end
  class ConstraintException < Exception; end
  class MismatchException < Exception; end
  class MisuseException < Exception; end
  class UnsupportedException < Exception; end
  class AuthorizationException < Exception; end
  class FormatException < Exception; end
  class RangeException < Exception; end
  class NotADatabaseException < Exception; end

  #: (untyped) -> Integer
  def self.__int(v)
    case v
    when Integer then v
    else raise TypeError, "no implicit conversion of #{v.class} into Integer"
    end
  end

  #: () -> String
  def self.__lib_name = RUBY_PLATFORM.include?("darwin") ? "libsqlite3.dylib" : "libsqlite3.so.0"

  # The C API through Fiddle::Importer: each extern is a typed module method (rb2go compiles the literal signatures, decision 326).
  module C
    extend Fiddle::Importer
    HANDLE = Fiddle.dlopen(SQLite3.__lib_name) #: Fiddle::Handle
    dlload HANDLE
    TRANSIENT = -1 # SQLITE_TRANSIENT: sqlite copies the bytes before the call returns

    extern "void *sqlite3_libversion(void)"
    extern "int sqlite3_libversion_number(void)"
    extern "int sqlite3_open_v2(void *, void *, int, void *)"
    extern "int sqlite3_close_v2(void *)"
    extern "void *sqlite3_errmsg(void *)"
    extern "int sqlite3_errcode(void *)"
    extern "int sqlite3_extended_errcode(void *)"
    extern "int sqlite3_prepare_v2(void *, void *, int, void *, void *)"
    extern "int sqlite3_step(void *)"
    extern "int sqlite3_reset(void *)"
    extern "int sqlite3_finalize(void *)"
    extern "int sqlite3_clear_bindings(void *)"
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
    extern "int sqlite3_bind_text(void *, int, void *, int, void *)"
    extern "int sqlite3_bind_blob(void *, int, void *, int, void *)"
    extern "int sqlite3_bind_int64(void *, int, long long)"
    extern "int sqlite3_bind_double(void *, int, double)"
    extern "int sqlite3_bind_null(void *, int)"
    extern "void *sqlite3_sql(void *)"
    extern "int sqlite3_changes(void *)"
    extern "int sqlite3_total_changes(void *)"
    extern "long long sqlite3_last_insert_rowid(void *)"
    extern "int sqlite3_get_autocommit(void *)"
    extern "int sqlite3_busy_timeout(void *, int)"
    extern "void *sqlite3_db_filename(void *, void *)"
    extern "int sqlite3_complete(void *)"
    extern "void sqlite3_interrupt(void *)"
    extern "int sqlite3_create_function(void *, void *, int, int, void *, void *, void *, void *)"
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

    # sqlite3_error_offset arrived in SQLite 3.38; older libraries have no symbol.
    ERROR_OFFSET = HANDLE.sym_defined?("sqlite3_error_offset").nil? ? nil : Fiddle::Function.new(HANDLE["sqlite3_error_offset"], [Fiddle::TYPE_VOIDP], Fiddle::TYPE_INT) #: Fiddle::Function?

    # Pointer.read's String is binary under MRI; a TEXT column is UTF-8 (rb2go derives encoding from the bytes, decision 136).
    #: (Fiddle::Pointer, Integer) -> String
    def self.bytes(p, n) = n <= 0 || p.null? ? "" : Fiddle::Pointer.read(p.to_i, n)

    #: (Fiddle::Pointer, Integer) -> String
    def self.text(p, n) = bytes(p, n).force_encoding(Encoding::UTF_8)

    #: (Integer) -> untyped
    def self.value(v)
      case sqlite3_value_type(v)
      when 1 then sqlite3_value_int64(v)
      when 2 then sqlite3_value_double(v)
      when 3 then text(sqlite3_value_text(v), sqlite3_value_bytes(v))
      when 4 then bytes(sqlite3_value_blob(v), sqlite3_value_bytes(v))
      end
    end

    #: (Integer, untyped) -> void
    def self.result(ctx, val)
      case val
      when Blob then sqlite3_result_blob(ctx, val.to_s, val.bytesize, TRANSIENT)
      when String then sqlite3_result_text(ctx, val.to_s, val.bytesize, TRANSIENT)
      when Integer then sqlite3_result_int64(ctx, val)
      when Float then sqlite3_result_double(ctx, val)
      when nil then sqlite3_result_null(ctx)
      else raise RuntimeError, "can't return #{val.class}"
      end
    end
  end

  #: () -> Integer
  def self.libversion = C.sqlite3_libversion_number

  SQLITE_VERSION = C.sqlite3_libversion.to_s #: String
  SQLITE_VERSION_NUMBER = libversion #: Integer
  SQLITE_LOADED_VERSION = SQLITE_VERSION #: String

  # The exception class for a result code's primary code.
  #: (Integer, String) -> Exception
  def self.__exception(code, msg)
    case code & 0xff
    when 1 then SQLException.new(msg)
    when 2 then InternalException.new(msg)
    when 3 then PermissionException.new(msg)
    when 4 then AbortException.new(msg)
    when 5 then BusyException.new(msg)
    when 6 then LockedException.new(msg)
    when 7 then MemoryException.new(msg)
    when 8 then ReadOnlyException.new(msg)
    when 9 then InterruptException.new(msg)
    when 10 then IOException.new(msg)
    when 11 then CorruptException.new(msg)
    when 12 then NotFoundException.new(msg)
    when 13 then FullException.new(msg)
    when 14 then CantOpenException.new(msg)
    when 15 then ProtocolException.new(msg)
    when 16 then EmptyException.new(msg)
    when 17 then SchemaChangedException.new(msg)
    when 18 then TooBigException.new(msg)
    when 19 then ConstraintException.new(msg)
    when 20 then MismatchException.new(msg)
    when 21 then MisuseException.new(msg)
    when 22 then UnsupportedException.new(msg)
    when 23 then AuthorizationException.new(msg)
    when 24 then FormatException.new(msg)
    when 25 then RangeException.new(msg)
    when 26 then NotADatabaseException.new(msg)
    else Exception.new(msg)
    end
  end

  # PRAGMA accessors: the gem's Pragmas module, folded into Database so self is typed (a subset; the rest follow the same four helpers).
  class Database
    SYNCHRONOUS_MODES = { "full" => 2, "normal" => 1, "off" => 0 }.freeze #: Hash[String, Integer]
    TEMP_STORE_MODES = { "default" => 0, "file" => 1, "memory" => 2 }.freeze #: Hash[String, Integer]
    AUTO_VACUUM_MODES = { "none" => 0, "full" => 1, "incremental" => 2 }.freeze #: Hash[String, Integer]
    JOURNAL_MODES = %w[delete truncate persist memory wal off].to_h { |m| [m, m] }.freeze #: Hash[String, String]
    LOCKING_MODES = { "normal" => "normal", "exclusive" => "exclusive" }.freeze #: Hash[String, String]

    #: (String) -> bool
    def get_boolean_pragma(name) = get_first_value("PRAGMA #{name}") != 0

    #: (String, untyped) -> void
    def set_boolean_pragma(name, mode)
      value = case mode
              when String
                case mode.to_s.downcase
                when "on", "yes", "true", "y", "t" then "'ON'"
                when "off", "no", "false", "n", "f" then "'OFF'"
                else raise SQLite3::Exception, "unrecognized pragma parameter #{mode.inspect}"
                end
              when true, 1 then "ON"
              when false, 0, nil then "OFF"
              else raise SQLite3::Exception, "unrecognized pragma parameter #{mode.inspect}"
              end
      execute("PRAGMA #{name}=#{value}")
    end

    #: (String, *untyped) ?{ (untyped) -> void } -> Array[untyped]
    def get_query_pragma(name, *params)
      sql = params.empty? ? "PRAGMA #{name}" : "PRAGMA #{name}( '#{params.join("','")}' )"
      rows = execute(sql) #: Array[untyped]
      rows.each { |row| yield row } if block_given?
      rows
    end

    #: (String) -> untyped
    def get_enum_pragma(name) = get_first_value("PRAGMA #{name}")

    #: (String, untyped, Hash[String, untyped]) -> void
    def set_enum_pragma(name, mode, enums)
      key = mode.to_s.downcase
      raise SQLite3::Exception, "unrecognized #{name} #{mode.inspect}" unless enums.key?(key)

      execute("PRAGMA #{name}='#{enums[key]}'")
    end

    #: (String) -> Integer
    def get_int_pragma(name)
      case (v = get_first_value("PRAGMA #{name}"))
      when Integer then v
      else 0
      end
    end

    #: (String, Integer) -> void
    def set_int_pragma(name, value) = execute("PRAGMA #{name}=#{value}")

    #: () -> Integer
    def application_id = get_int_pragma("application_id")

    #: (Integer) -> void
    def application_id=(v)
      set_int_pragma("application_id", v)
    end

    #: () -> untyped
    def auto_vacuum = get_enum_pragma("auto_vacuum")

    #: (untyped) -> void
    def auto_vacuum=(mode)
      set_enum_pragma("auto_vacuum", mode, AUTO_VACUUM_MODES)
    end

    #: () -> Integer
    def cache_size = get_int_pragma("cache_size")

    #: (Integer) -> void
    def cache_size=(v)
      set_int_pragma("cache_size", v)
    end

    #: (untyped) -> void
    def case_sensitive_like=(mode)
      set_boolean_pragma("case_sensitive_like", mode)
    end

    #: () -> Integer
    def data_version = get_int_pragma("data_version")

    #: () ?{ (untyped) -> void } -> Array[untyped]
    def database_list(&block) = get_query_pragma("database_list", &block)

    #: () -> bool
    def defer_foreign_keys = get_boolean_pragma("defer_foreign_keys")

    #: (untyped) -> void
    def defer_foreign_keys=(mode)
      set_boolean_pragma("defer_foreign_keys", mode)
    end

    #: (*untyped) ?{ (untyped) -> void } -> Array[untyped]
    def foreign_key_check(*table, &block) = get_query_pragma("foreign_key_check", *table, &block)

    #: (String) ?{ (untyped) -> void } -> Array[untyped]
    def foreign_key_list(table, &block) = get_query_pragma("foreign_key_list", table, &block)

    #: () -> bool
    def foreign_keys = get_boolean_pragma("foreign_keys")

    #: (untyped) -> void
    def foreign_keys=(mode)
      set_boolean_pragma("foreign_keys", mode)
    end

    #: () -> Integer
    def freelist_count = get_int_pragma("freelist_count")

    #: () -> bool
    def ignore_check_constraints = get_boolean_pragma("ignore_check_constraints")

    #: (untyped) -> void
    def ignore_check_constraints=(mode)
      set_boolean_pragma("ignore_check_constraints", mode)
    end

    #: (String) ?{ (untyped) -> void } -> Array[untyped]
    def index_info(index, &block) = get_query_pragma("index_info", index, &block)

    #: (String) ?{ (untyped) -> void } -> Array[untyped]
    def index_list(table, &block) = get_query_pragma("index_list", table, &block)

    #: (*untyped) ?{ (untyped) -> void } -> Array[untyped]
    def integrity_check(*num_errors, &block) = get_query_pragma("integrity_check", *num_errors, &block)

    #: () -> untyped
    def journal_mode = get_enum_pragma("journal_mode")

    #: (untyped) -> void
    def journal_mode=(mode)
      set_enum_pragma("journal_mode", mode, JOURNAL_MODES)
    end

    #: () -> Integer
    def journal_size_limit = get_int_pragma("journal_size_limit")

    #: (Integer) -> void
    def journal_size_limit=(v)
      set_int_pragma("journal_size_limit", v)
    end

    #: () -> untyped
    def locking_mode = get_enum_pragma("locking_mode")

    #: (untyped) -> void
    def locking_mode=(mode)
      set_enum_pragma("locking_mode", mode, LOCKING_MODES)
    end

    #: () -> Integer
    def max_page_count = get_int_pragma("max_page_count")

    #: (Integer) -> void
    def max_page_count=(v)
      set_int_pragma("max_page_count", v)
    end

    #: () -> Integer
    def page_count = get_int_pragma("page_count")

    #: () -> Integer
    def page_size = get_int_pragma("page_size")

    #: (Integer) -> void
    def page_size=(v)
      set_int_pragma("page_size", v)
    end

    #: () ?{ (untyped) -> void } -> Array[untyped]
    def quick_check(&block) = get_query_pragma("quick_check", &block)

    #: () -> bool
    def query_only = get_boolean_pragma("query_only")

    #: (untyped) -> void
    def query_only=(mode)
      set_boolean_pragma("query_only", mode)
    end

    #: () -> bool
    def read_uncommitted = get_boolean_pragma("read_uncommitted")

    #: (untyped) -> void
    def read_uncommitted=(mode)
      set_boolean_pragma("read_uncommitted", mode)
    end

    #: () -> bool
    def recursive_triggers = get_boolean_pragma("recursive_triggers")

    #: (untyped) -> void
    def recursive_triggers=(mode)
      set_boolean_pragma("recursive_triggers", mode)
    end

    #: () -> bool
    def reverse_unordered_selects = get_boolean_pragma("reverse_unordered_selects")

    #: (untyped) -> void
    def reverse_unordered_selects=(mode)
      set_boolean_pragma("reverse_unordered_selects", mode)
    end

    #: () -> Integer
    def schema_version = get_int_pragma("schema_version")

    #: (Integer) -> void
    def schema_version=(v)
      set_int_pragma("schema_version", v)
    end

    #: () -> bool
    def secure_delete = get_boolean_pragma("secure_delete")

    #: (untyped) -> void
    def secure_delete=(mode)
      set_boolean_pragma("secure_delete", mode)
    end

    #: () -> untyped
    def synchronous = get_enum_pragma("synchronous")

    #: (untyped) -> void
    def synchronous=(mode)
      set_enum_pragma("synchronous", mode, SYNCHRONOUS_MODES)
    end

    #: () -> untyped
    def temp_store = get_enum_pragma("temp_store")

    #: (untyped) -> void
    def temp_store=(mode)
      set_enum_pragma("temp_store", mode, TEMP_STORE_MODES)
    end

    #: () -> Integer
    def user_version = get_int_pragma("user_version")

    #: (Integer) -> void
    def user_version=(v)
      set_int_pragma("user_version", v)
    end

    #: () -> Integer
    def wal_autocheckpoint = get_int_pragma("wal_autocheckpoint")

    #: (Integer) -> void
    def wal_autocheckpoint=(v)
      set_int_pragma("wal_autocheckpoint", v)
    end

    #: (*untyped) ?{ (untyped) -> void } -> Array[untyped]
    def wal_checkpoint(*mode, &block) = get_query_pragma("wal_checkpoint", *mode, &block)
  end

  # The value a `create_function` block fills: `fn.result = x`, plus a per-call context Hash.
  class FunctionProxy
    attr_accessor :result #: untyped

    #: () -> void
    def initialize
      @result = nil
      @context = {} #: Hash[untyped, untyped]
    end

    #: (untyped) -> untyped
    def [](key) = @context[key]

    #: (untyped, untyped) -> untyped
    def []=(key, value)
      @context[key] = value
    end
  end

  class Database
    attr_accessor :results_as_hash #: bool

    #: (String, ?Hash[Symbol, untyped]) ?{ (Database) -> untyped } -> Database
    def self.open(file, options = {})
      database = new(file, options)
      if block_given?
        begin
          yield database
        ensure
          database.close
        end
      end
      database
    end

    #: (String) -> String
    def self.quote(string) = string.gsub("'", "''")

    # Options: readonly, readwrite, flags, results_as_hash, default_transaction_mode.
    #: (String, ?Hash[Symbol, untyped]) ?{ (Database) -> untyped } -> void
    def initialize(file, options = {})
      mode = Constants::Open::READWRITE | Constants::Open::CREATE
      mode = Constants::Open::READONLY if options[:readonly]
      if options[:readwrite]
        raise "conflicting options: readonly and readwrite" if options[:readonly]

        mode = Constants::Open::READWRITE
      end
      case (flags = options[:flags])
      when Integer
        raise "conflicting options: flags with readonly and/or readwrite" if options[:readonly] || options[:readwrite]

        mode = SQLite3.__int(flags)
      end
      out = Fiddle::Pointer.malloc(Fiddle::SIZEOF_VOIDP)
      rc = C.sqlite3_open_v2(file, out, mode, nil)
      @db = out.ptr.to_i #: Integer
      Fiddle.free(out.to_i)
      @closed = false
      if rc != 0
        msg = @db.zero? ? "out of memory" : C.sqlite3_errmsg(@db).to_s
        C.sqlite3_close_v2(@db) unless @db.zero?
        @db = 0
        @closed = true
        e = SQLite3.__exception(rc, msg)
        e.__set(rc, nil, -1)
        raise e
      end
      @functions = [] #: Array[Fiddle::Closure]
      @pending_error = nil #: StandardError?
      @results_as_hash = options[:results_as_hash] ? true : false
      @readonly = mode & Constants::Open::READONLY != 0
      @default_transaction_mode = (options[:default_transaction_mode] || :deferred).to_s
      if block_given?
        begin
          yield self
        ensure
          close
        end
      end
    end

    #: () -> Integer
    def __db = @db

    # Raises the exception for a non-OK result code, with the database's message (and the SQL, for prepare).
    #: (Integer, ?String?) -> void
    def __check(rc, sql = nil)
      return if rc == 0 || rc == 100 || rc == 101

      msg = C.sqlite3_errmsg(@db).to_s
      offset = -1
      eo = C::ERROR_OFFSET
      offset = SQLite3.__int(eo.call(@db)) if !sql.nil? && !eo.nil?
      e = SQLite3.__exception(rc, msg)
      e.__set(rc, sql, offset)
      raise e
    end

    #: () -> Encoding
    def encoding = Encoding.find(get_first_value("PRAGMA encoding").to_s)

    #: () -> bool
    def closed? = @closed

    #: () -> nil
    def close
      unless @closed
        C.sqlite3_close_v2(@db)
        @closed = true
      end
      nil
    end

    # With a block, yields the statement and closes it after; the block's value is dropped (MRI returns it).
    #: (String) ?{ (Statement) -> untyped } -> Statement
    def prepare(sql)
      stmt = Statement.new(self, sql)
      if block_given?
        begin
          yield stmt
        ensure
          stmt.close unless stmt.closed?
        end
      end
      stmt
    end

    #: (?String) -> String
    def filename(db_name = "main") = C.sqlite3_db_filename(@db, db_name).to_s

    # bind_vars: one value, an Array of values (`?` in order), or a Hash of names (`:name`); with a block, each row
    # is yielded and the result is empty (MRI returns the ResultSet).
    #: (String, ?untyped) ?{ (untyped) -> void } -> Array[untyped]
    def execute(sql, bind_vars = [])
      stmt = prepare(sql)
      begin
        stmt.bind_params(bind_vars)
        rs = build_result_set(stmt)
        if block_given?
          rs.each { |row| yield row }
          []
        else
          rs.to_a.freeze
        end
      ensure
        stmt.close unless stmt.closed?
      end
    end

    # The column names first, then the rows.
    #: (String, *untyped) ?{ (untyped) -> void } -> Array[untyped]
    def execute2(sql, *bind_vars)
      stmt = prepare(sql)
      begin
        rs = stmt.execute(*bind_vars)
        out = [] #: Array[untyped]
        if block_given?
          yield stmt.columns
          rs.each { |row| yield row }
        else
          out << stmt.columns
          rs.each { |row| out << row }
        end
        out
      ensure
        stmt.close unless stmt.closed?
      end
    end

    # Runs every statement in sql; bind_vars apply to each whose parameter count matches.
    #: (String, ?untyped) -> nil
    def execute_batch(sql, bind_vars = [])
      sql = sql.strip
      until sql.empty?
        stmt = prepare(sql)
        begin
          unless stmt.closed?
            vars = __bind_list(bind_vars)
            stmt.bind_params(vars) if vars.length == stmt.bind_parameter_count
            stmt.step
          end
          sql = stmt.remainder.strip
        ensure
          stmt.close unless stmt.closed?
        end
      end
      nil
    end

    #: (untyped) -> Array[untyped]
    def __bind_list(bind_vars)
      case bind_vars
      when Array then bind_vars
      when Hash then [bind_vars]
      else [bind_vars]
      end
    end

    # A ResultSet to step through; with a block, closed after it.
    #: (String, ?untyped) ?{ (ResultSet) -> untyped } -> ResultSet
    def query(sql, bind_vars = [])
      result = prepare(sql).execute(bind_vars)
      if block_given?
        begin
          yield result
        ensure
          result.close
        end
      end
      result
    end

    #: (String, *untyped) -> untyped
    def get_first_row(sql, *bind_vars) = execute(sql, bind_vars).first

    #: (String, *untyped) -> untyped
    def get_first_value(sql, *bind_vars)
      stmt = prepare(sql)
      begin
        stmt.bind_params(bind_vars)
        row = stmt.step
        row.nil? ? nil : row[0]
      ensure
        stmt.close unless stmt.closed?
      end
    end

    #: () -> Integer
    def last_insert_row_id = C.sqlite3_last_insert_rowid(@db)

    #: () -> Integer
    def changes = C.sqlite3_changes(@db)

    #: () -> Integer
    def total_changes = C.sqlite3_total_changes(@db)

    #: () -> String
    def errmsg = C.sqlite3_errmsg(@db).to_s

    #: () -> Integer
    def errcode = C.sqlite3_errcode(@db)

    #: () -> Integer
    def extended_errcode = C.sqlite3_extended_errcode(@db)

    #: (Integer) -> Integer
    def busy_timeout(ms)
      __check(C.sqlite3_busy_timeout(@db, ms))
      ms
    end

    #: (Integer) -> Integer
    def busy_timeout=(ms)
      busy_timeout(ms)
    end

    #: (String) -> bool
    def complete?(sql) = C.sqlite3_complete(sql) != 0

    #: () -> nil
    def interrupt
      C.sqlite3_interrupt(@db)
      nil
    end

    #: () -> bool
    def readonly? = @readonly

    #: () -> bool
    def transaction_active? = C.sqlite3_get_autocommit(@db).zero?

    # Begins a transaction (deferred, immediate or exclusive); with a block, commits after it or rolls back on an
    # exception and returns the block's value.
    #: (?untyped) ?{ (Database) -> untyped } -> untyped
    def transaction(mode = nil)
      mode = @default_transaction_mode if mode.nil?
      execute("begin #{mode} transaction")
      return true unless block_given?

      abort = false
      begin
        yield self
      rescue ::Exception
        abort = true
        raise
      ensure
        abort ? rollback : commit
      end
    end

    #: () -> bool
    def commit
      execute("commit transaction")
      true
    end

    #: () -> bool
    def rollback
      execute("rollback transaction")
      true
    end

    # A SQL function running a block, `create_function("twice", 1) { |fn, x| fn.result = x * 2 }`, with up to six arguments (the block's are typed, so a splat cannot carry them; ponytail: raise past six). An exception in the block becomes the statement's error.
    #: (String, Integer, ?Integer) { (FunctionProxy, untyped, untyped, untyped, untyped, untyped, untyped) -> void } -> Database
    def create_function(name, arity, text_rep = Constants::TextRep::UTF8, &block)
      cb = Fiddle::Closure::BlockCaller.new(Fiddle::TYPE_VOID, [Fiddle::TYPE_VOIDP, Fiddle::TYPE_INT, Fiddle::TYPE_VOIDP]) do |ctx, argc, argv|
        __call_function(ctx, argc, argv) { |fp, a| block.call(fp, a[0], a[1], a[2], a[3], a[4], a[5]) }
      end
      @functions << cb
      __check(C.sqlite3_create_function(@db, name, arity, text_rep, nil, cb, nil, nil))
      self
    end

    #: (Fiddle::Pointer, Integer, Fiddle::Pointer) { (FunctionProxy, Array[untyped]) -> void } -> void
    def __call_function(ctx, argc, argv, &block)
      fp = FunctionProxy.new
      args = [] #: Array[untyped]
      i = 0
      while i < argc
        args << C.value(Fiddle::Pointer.new(argv.to_i + (i * Fiddle::SIZEOF_VOIDP)).ptr.to_i)
        i += 1
      end
      begin
        block.call(fp, args)
        C.result(ctx.to_i, fp.result)
      rescue StandardError => e
        # A Ruby exception cannot unwind through C: it is parked, and the step that ran the function raises it (as the gem does).
        @pending_error = e
        C.sqlite3_result_error(ctx.to_i, e.message, -1)
      end
    end

    # define_function's block gets the arguments and returns the result.
    #: (String) { (*untyped) -> untyped } -> Database
    def define_function(name, &block)
      cb = Fiddle::Closure::BlockCaller.new(Fiddle::TYPE_VOID, [Fiddle::TYPE_VOIDP, Fiddle::TYPE_INT, Fiddle::TYPE_VOIDP]) do |ctx, argc, argv|
        __call_function(ctx, argc, argv) { |fp, a| fp.result = block.call(*a) }
      end
      @functions << cb
      __check(C.sqlite3_create_function(@db, name, -1, Constants::TextRep::UTF8, nil, cb, nil, nil))
      self
    end

    # One Hash per column: cid, name, type (downcased), notnull, dflt_value (unquoted), pk.
    #: (String) ?{ (Hash[String, untyped]) -> void } -> Array[Hash[String, untyped]]
    def table_info(table)
      stmt = prepare("PRAGMA table_info(#{table})")
      columns = stmt.columns
      result = [] #: Array[Hash[String, untyped]]
      stmt.each do |row|
        new_row = {} #: Hash[String, untyped]
        columns.each_with_index { |c, i| new_row[c] = row[i] }
        dflt = new_row["dflt_value"]
        if dflt.is_a?(String)
          case dflt
          when /\Anull\z/i then new_row["dflt_value"] = nil
          when /\A'(.*)'\z/m then new_row["dflt_value"] = $1.to_s.gsub("''", "'")
          when /\A"(.*)"\z/m then new_row["dflt_value"] = $1.to_s.gsub('""', '"')
          end
        end
        type = new_row["type"]
        new_row["type"] = type.downcase if type.is_a?(String)
        if block_given?
          yield new_row
        else
          result << new_row
        end
      end
      stmt.close
      result
    end

    #: () -> StandardError?
    def __take_error
      e = @pending_error
      @pending_error = nil
      e
    end

    #: (Statement) -> ResultSet
    def build_result_set(stmt) = @results_as_hash ? HashResultSet.new(self, stmt) : ResultSet.new(self, stmt)
  end

  # A compiled statement: bind, step, read columns.
  class Statement
    include Enumerable #[Array[untyped]]

    attr_reader :remainder #: String

    #: (Database, String) -> void
    def initialize(db, sql)
      raise ArgumentError, "prepare called on a closed database" if db.closed?

      @connection = db
      @columns = nil #: Array[String]?
      @types = nil #: Array[String?]?
      @done = false
      src = Fiddle::Pointer.malloc(sql.bytesize + 1)
      src[0, sql.bytesize + 1] = sql + "\0"
      out = Fiddle::Pointer.malloc(Fiddle::SIZEOF_VOIDP)
      tail = Fiddle::Pointer.malloc(Fiddle::SIZEOF_VOIDP)
      rc = C.sqlite3_prepare_v2(db.__db, src, sql.bytesize, out, tail)
      @st = out.ptr.to_i #: Integer
      used = tail.ptr.to_i - src.to_i
      Fiddle.free(out.to_i)
      Fiddle.free(tail.to_i)
      Fiddle.free(src.to_i)
      @remainder = rc == 0 && used >= 0 && used <= sql.bytesize ? sql.byteslice(used, sql.bytesize - used).to_s : ""
      db.__check(rc, sql)
    end

    #: () -> Integer
    def __st = @st

    #: () -> void
    def must_be_open!
      raise SQLite3::Exception, "cannot use a closed statement" if closed?
    end

    #: () -> bool
    def closed? = @st.zero?

    #: () -> nil
    def close
      must_be_open!
      C.sqlite3_finalize(@st)
      @st = 0
      nil
    end

    #: () -> bool
    def done? = @done

    #: () -> bool
    def active? = !@done

    #: () -> Statement
    def reset!
      must_be_open!
      C.sqlite3_reset(@st)
      @done = false
      self
    end

    #: () -> Statement
    def clear_bindings!
      must_be_open!
      C.sqlite3_clear_bindings(@st)
      self
    end

    #: () -> Integer
    def column_count
      must_be_open!
      C.sqlite3_column_count(@st)
    end

    #: (Integer) -> String
    def column_name(i)
      must_be_open!
      C.sqlite3_column_name(@st, i).to_s
    end

    #: (Integer) -> String?
    def column_decltype(i)
      must_be_open!
      p = C.sqlite3_column_decltype(@st, i)
      p.null? ? nil : p.to_s
    end

    #: () -> Integer
    def bind_parameter_count
      must_be_open!
      C.sqlite3_bind_parameter_count(@st)
    end

    #: () -> String
    def sql
      must_be_open!
      C.sqlite3_sql(@st).to_s
    end

    #: () -> Array[String]
    def columns
      memo = @columns
      return memo unless memo.nil?

      n = column_count
      cols = [] #: Array[String]
      i = 0
      while i < n
        cols << column_name(i)
        i += 1
      end
      @columns = cols.freeze
      cols
    end

    # Declared column types, downcased (nil for an expression).
    #: () -> Array[String?]
    def types
      must_be_open!
      memo = @types
      return memo unless memo.nil?

      n = column_count
      ts = [] #: Array[String?]
      i = 0
      while i < n
        t = column_decltype(i)
        ts << (t.nil? ? nil : t.downcase)
        i += 1
      end
      @types = ts.freeze
      ts
    end

    # key: a 1-based index, or a parameter's name (with or without the leading `:`).
    #: (untyped, untyped) -> void
    def bind_param(key, value)
      must_be_open!
      index = case key
              when Integer then key
              when String, Symbol
                k = key.to_s
                k = ":#{k}" unless k.start_with?(":")
                C.sqlite3_bind_parameter_index(@st, k)
              else raise TypeError, "wrong argument type #{key.class} (expected Integer, String or Symbol)"
              end
      raise SQLite3::Exception, "no such bind parameter" if index.zero?

      rc = case value
           when Blob then C.sqlite3_bind_blob(@st, index, value.to_s, value.bytesize, C::TRANSIENT)
           when String then C.sqlite3_bind_text(@st, index, value.to_s, value.bytesize, C::TRANSIENT)
           when Integer then C.sqlite3_bind_int64(@st, index, value)
           when Float then C.sqlite3_bind_double(@st, index, value)
           when nil then C.sqlite3_bind_null(@st, index)
           else raise RuntimeError, "can't prepare #{value.class}"
           end
      @connection.__check(rc)
    end

    # Values bind by position; a Hash binds by name.
    #: (*untyped) -> void
    def bind_params(*bind_vars)
      index = 1
      bind_vars.flatten.each do |var|
        if var.is_a?(Hash)
          var.each { |key, val| bind_param(key, val) }
        else
          bind_param(index, var)
          index += 1
        end
      end
    end

    #: (*untyped) ?{ (ResultSet) -> void } -> ResultSet
    def execute(*bind_vars)
      reset!
      bind_params(*bind_vars) unless bind_vars.empty?
      results = @connection.build_result_set(self)
      step if column_count.zero?
      yield results if block_given?
      results
    end

    #: (*untyped) ?{ (untyped) -> void } -> Array[untyped]
    def execute!(*bind_vars)
      rs = execute(*bind_vars)
      if block_given?
        rs.each { |row| yield row }
        []
      else
        rs.to_a
      end
    end

    # The next row, or nil once the statement is done.
    #: () -> Array[untyped]?
    def step
      must_be_open!
      return nil if @done

      rc = C.sqlite3_step(@st)
      case rc
      when 100 then __row
      when 101
        @done = true
        nil
      else
        C.sqlite3_reset(@st)
        @done = false
        err = @connection.__take_error
        raise err unless err.nil?

        @connection.__check(rc)
        nil
      end
    end

    #: () -> Array[untyped]
    def __row
      n = C.sqlite3_column_count(@st)
      row = [] #: Array[untyped]
      i = 0
      while i < n
        row << __column(i)
        i += 1
      end
      row
    end

    #: (Integer) -> untyped
    def __column(i)
      case C.sqlite3_column_type(@st, i)
      when 1 then C.sqlite3_column_int64(@st, i)
      when 2 then C.sqlite3_column_double(@st, i)
      when 3 then C.text(C.sqlite3_column_text(@st, i), C.sqlite3_column_bytes(@st, i))
      when 4 then C.bytes(C.sqlite3_column_blob(@st, i), C.sqlite3_column_bytes(@st, i))
      end
    end

    #: () { (Array[untyped]) -> void } -> Statement
    def each
      while (row = step)
        yield row
      end
      self
    end
  end

  # Rows of an executed statement, as Arrays.
  class ResultSet
    include Enumerable #[untyped]

    #: (Database, Statement) -> void
    def initialize(db, stmt)
      @db = db
      @stmt = stmt
    end

    #: (*untyped) -> void
    def reset(*bind_params)
      @stmt.reset!
      @stmt.bind_params(*bind_params)
    end

    #: () -> bool
    def eof? = @stmt.done?

    #: () -> untyped
    def next = @stmt.step

    #: () { (untyped) -> void } -> ResultSet
    def each
      while (row = self.next)
        yield row
      end
      self
    end

    #: () { (Hash[String, untyped]) -> void } -> ResultSet
    def each_hash
      while (row = next_hash)
        yield row
      end
      self
    end

    #: () -> nil
    def close = @stmt.close

    #: () -> bool
    def closed? = @stmt.closed?

    #: () -> Array[String?]
    def types = @stmt.types

    #: () -> Array[String]
    def columns = @stmt.columns

    #: () -> Hash[String, untyped]?
    def next_hash
      row = @stmt.step
      return nil if row.nil?

      h = {} #: Hash[String, untyped]
      @stmt.columns.each_with_index { |c, i| h[c] = row[i] }
      h
    end
  end

  # Rows as Hashes (results_as_hash).
  class HashResultSet < ResultSet
    #: () -> untyped
    def next = next_hash
  end
end

class String
  #: () -> SQLite3::Blob
  def to_blob = SQLite3::Blob.new(self)
end
