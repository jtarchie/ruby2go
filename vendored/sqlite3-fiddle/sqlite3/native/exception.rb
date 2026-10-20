# rbs_inline: enabled

# ext/sqlite3/exception.c over Fiddle: a result code becomes its SQLite3::*Exception.

require "sqlite3/errors"
require "sqlite3/native/ffi"

module SQLite3
  class Exception
    #: (Integer, ?String?, ?Integer?) -> self
    def __set(code, sql = nil, offset = nil) # rb_iv_set in the C extension: rb_exc_new2 skips initialize, so sql and offset stay nil
      @code = code
      @sql = sql
      @sql_offset = offset
      self
    end
  end

  #: (Integer, String) -> Exception
  def self.__exception(status, msg)
    case status & 0xff
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

  #: (Integer, Integer) -> void
  def self.__check(db, status) # rb_sqlite3_raise
    return if status == 0

    raise __exception(status, C.cstr(C.sqlite3_errmsg(db)).to_s).__set(status)
  end

  #: (Integer, Integer) -> void
  def self.__check_msg(status, msg) # rb_sqlite3_raise_msg: msg came from sqlite3_malloc (an errmsg out-parameter) and is freed here
    return if status == 0

    text = msg.zero? ? "" : C.cstr(Fiddle::Pointer.new(msg)).to_s
    C.sqlite3_free(msg) unless msg.zero?
    raise __exception(status, text).__set(status)
  end

  #: (Integer, Integer, String) -> void
  def self.__check_prepare(db, status, sql) # rb_sqlite3_raise_with_sql
    return if status == 0

    offset = -1
    eo = C::ERROR_OFFSET
    offset = eo.call(db).to_i unless eo.nil?
    raise __exception(status, C.cstr(C.sqlite3_errmsg(db)).to_s).__set(status, sql, offset)
  end
end
