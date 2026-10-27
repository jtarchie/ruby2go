# rbs_inline: enabled

# Stands in for the blog's runtime/db.rb (sqlite through Spinel's FFI): the same Db API on the sqlite3 gem (README.md).
require "sqlite3"
require_relative "../blog/runtime/active_support_time_parsing"

module Db
  # A prepared statement and the row its last step read; callers hold it as an opaque handle.
  class Stmt
    attr_reader :st #: SQLite3::Statement
    attr_accessor :row #: Array[untyped]?

    #: (SQLite3::Statement) -> void
    def initialize(st)
      @st = st
      @row = nil
    end
  end

  POOL = Queue.new #: Queue[SQLite3::Database]
  MAIN = [] #: Array[SQLite3::Database]

  #: (String, ?pool_size: Integer) -> void
  def self.configure(path, pool_size: 8)
    MAIN.replace([open(path)])
    pool_size.times { POOL << open(path) }
  end

  #: (String) -> SQLite3::Database
  def self.open(path)
    db = SQLite3::Database.new(path)
    db.busy_timeout = 5000
    db.execute("PRAGMA journal_mode=WAL")
    db
  end

  #: () -> SQLite3::Database
  def self.conn # the request's leased connection, else the one boot and seeding use
    leased = Thread.current[:rh_db_conn] #: SQLite3::Database?
    leased || MAIN.first || raise("Db.configure was not called")
  end

  #: [T] () { () -> T } -> T
  def self.with_connection
    db = POOL.pop
    Thread.current[:rh_db_conn] = db
    begin
      yield
    ensure
      Thread.current[:rh_db_conn] = nil
      POOL << db
    end
  end

  #: () -> bool
  def self.in_lease? = !Thread.current[:rh_db_conn].nil?

  #: () -> bool
  def self.sql_trace? = ENV.fetch("RH_SQL_TRACE", "") != ""

  #: () -> nil
  def self.query_cache_begin = nil

  #: () -> nil
  def self.query_cache_end = nil

  #: (String) -> void
  def self.exec(sql)
    $stderr.puts "  SQL " + sql if sql_trace?
    begin
      conn.execute(sql)
    rescue SQLite3::Exception => e
      raise ActiveRecord::RecordNotUnique, e.message if unique_violation?(e.message)
      raise
    end
  end

  #: (String) -> bool
  def self.unique_violation?(message) = message.include?("UNIQUE constraint failed")

  #: (String) -> Stmt
  def self.prepare(sql)
    $stderr.puts "  SQL " + sql if sql_trace?
    Stmt.new(conn.prepare(sql))
  end

  #: (Stmt) -> bool
  def self.step?(stmt)
    stmt.row = stmt.st.step
    !stmt.row.nil?
  end

  #: (Stmt, Integer) -> untyped
  def self.column_value(stmt, i) = (stmt.row || raise("Db: no row; call Db.step? first"))[i]

  #: (Stmt, Integer) -> Integer
  def self.column_int(stmt, i) = column_value(stmt, i).to_i

  #: (Stmt, Integer) -> Float
  def self.column_float(stmt, i) = column_value(stmt, i).to_f

  #: (Stmt, Integer) -> String
  def self.column_text(stmt, i) = column_value(stmt, i).to_s

  #: (Stmt, Integer) -> bool
  def self.column_bool(stmt, i) = column_int(stmt, i) != 0

  #: (Stmt, Integer) -> Integer?
  def self.column_int_opt(stmt, i) = column_value(stmt, i).nil? ? nil : column_int(stmt, i)

  #: (Stmt, Integer) -> Float?
  def self.column_float_opt(stmt, i) = column_value(stmt, i).nil? ? nil : column_float(stmt, i)

  #: (Stmt, Integer) -> String?
  def self.column_text_opt(stmt, i) = column_value(stmt, i).nil? ? nil : column_text(stmt, i)

  #: (Stmt, Integer) -> bool?
  def self.column_bool_opt(stmt, i) = column_value(stmt, i).nil? ? nil : column_bool(stmt, i)

  #: (Stmt) -> Integer
  def self.column_count(stmt) = stmt.st.column_count

  #: (Stmt, Integer) -> String
  def self.column_name(stmt, i) = stmt.st.column_name(i).to_s

  #: (Stmt) -> nil
  def self.finalize(stmt)
    stmt.st.close
    nil
  end

  #: (Stmt, Integer, Integer) -> void
  def self.bind_int(stmt, idx, value) = stmt.st.bind_param(idx, value)

  #: (Stmt, Integer, String) -> void
  def self.bind_text(stmt, idx, value) = stmt.st.bind_param(idx, value)

  #: (Stmt, Integer, bool) -> void
  def self.bind_bool(stmt, idx, value) = stmt.st.bind_param(idx, value ? 1 : 0)

  #: () -> Integer
  def self.last_insert_rowid = conn.last_insert_row_id

  #: () -> Integer
  def self.changes = conn.changes

  #: (untyped) -> String
  def self.escape_string(s) = "'" + s.to_s.gsub("'", "''") + "'"

  #: (untyped) -> String
  def self.escape_int(n) = n.to_i.to_s

  #: (untyped) -> String
  def self.escape_string_opt(s) = s.nil? ? "NULL" : escape_string(s)

  #: (untyped) -> String
  def self.escape_int_opt(n) = n.nil? ? "NULL" : escape_int(n)

  #: (untyped) -> String
  def self.escape_float_opt(f) = f.nil? ? "NULL" : f.to_f.to_s

  #: (untyped) -> String
  def self.escape_bool(b) = b ? "1" : "0"

  #: (untyped) -> String
  def self.escape_bool_opt(b) = b.nil? ? "NULL" : escape_bool(b)

  #: (Array[untyped]) -> String
  def self.escape_int_list(ids) = ids.empty? ? "NULL" : ids.map { |i| i.to_i.to_s }.join(", ")
end
