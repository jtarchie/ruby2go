# rbs_inline: enabled

require "minitest/autorun"
require "sqlite3"
require "tmpdir"

# sqlite3-ruby's API over Fiddle (decision 327), against the real gem under MRI.
module Sqlite3Tests
  class DatabaseTest < Minitest::Test
    #: () -> SQLite3::Database
    def seeded
      db = SQLite3::Database.new(":memory:")
      db.execute("create table t (id integer primary key, name text, score real, data blob)")
      db.execute("insert into t (name, score, data) values (?, ?, ?)", ["a", 1.5, SQLite3::Blob.new("\x00\x01")])
      db.execute("insert into t (name, score) values (?, ?)", ["b", nil])
      db.execute("insert into t (name, score) values (:n, :s)", { "n" => "c", "s" => 2 })
      db
    end

    def test_rows_and_types
      db = seeded
      rows = db.execute("select * from t order by id")
      assert_equal 3, rows.length
      assert_equal [1, "a", 1.5], rows[0][0, 3]
      assert_equal [0, 1], rows[0][3].bytes
      assert_equal 2, rows[0][3].bytesize
      assert_equal [2, "b", nil, nil], rows[1]
      assert_equal [3, "c", 2.0, nil], rows[2]
      assert rows.frozen?
      assert_equal 3, db.last_insert_row_id
      assert_equal 1, db.changes
      assert_equal 3, db.total_changes
    end

    def test_binds
      db = seeded
      assert_equal [[1, "a"]], db.execute("select id, name from t where id = ?", 1)
      assert_equal [[1, "a"]], db.execute("select id, name from t where id = ?", [1])
      assert_equal [[2, "b"]], db.execute("select id, name from t where name = ?", "b")
      assert_equal [[3, "c"]], db.execute("select id, name from t where name = :n", { "n" => "c" })
      assert_equal [[3, "c"]], db.execute("select id, name from t where name = :n", { n: "c" })
      assert_equal [[4_611_686_018_427_387_904, 1.0e300, "é"]], db.execute("select ?, ?, ?", [2**62, 1e300, "é"])
      assert_equal [["a\0b"]], db.execute("select ?", ["a\0b"])
      assert_equal [%w[blob text]], db.execute("select typeof(?), typeof(?)", [SQLite3::Blob.new("ab"), "ab"])
      assert_equal [%w[blob text]], db.execute("select typeof(:b), typeof(:t)", { "b" => SQLite3::Blob.new("ab"), "t" => "ab" })
      assert_equal "ab", SQLite3::Blob.new("ab").to_s
      assert_equal 2, SQLite3::Blob.new("ab").bytesize
      assert_equal [["blob"]], db.execute("select typeof(?)", ["ab".to_blob])
      e = assert_raises(RuntimeError) { db.execute("insert into t (name) values (?)", [true]) }
      assert_equal "can't prepare TrueClass", e.message
      e = assert_raises(RuntimeError) { db.execute("select ?", [:x]) }
      assert_equal "can't prepare Symbol", e.message
    end

    def test_execute2_first_row_first_value
      db = seeded
      assert_equal [%w[id name], [1, "a"], [2, "b"]], db.execute2("select id, name from t where id < 3")
      seen = [] #: Array[untyped]
      db.execute2("select name from t where id < 3") { |r| seen << r }
      assert_equal [["name"], ["a"], ["b"]], seen
      assert_equal ["a", 1.5], db.get_first_row("select name, score from t where id = ?", 1)
      assert_nil db.get_first_row("select name from t where id = 99")
      assert_equal 3, db.get_first_value("select count(*) from t")
      assert_nil db.get_first_value("select name from t where id = 99")
      rows = [] #: Array[untyped]
      db.execute("select id from t order by id") { |r| rows << r }
      assert_equal [[1], [2], [3]], rows
    end

    def test_results_as_hash
      db = seeded
      db.results_as_hash = true
      assert_equal [{ "id" => 1, "name" => "a" }], db.execute("select id, name from t where id = 1")
      assert_equal "a", db.get_first_value("select name from t where id = 1")
      assert db.results_as_hash
      db.results_as_hash = false
      assert_equal [[1, "a"]], db.execute("select id, name from t where id = 1")
      hashed = SQLite3::Database.new(":memory:", results_as_hash: true)
      assert_equal [{ "a" => 1 }], hashed.execute("select 1 as a")
      hashed.close
    end

    def test_statement
      db = seeded
      stmt = db.prepare("select name from t where id > ?")
      assert_equal ["name"], stmt.columns
      assert_equal ["text"], stmt.types
      assert_equal 1, stmt.column_count
      assert_equal 1, stmt.bind_parameter_count
      assert_equal "select name from t where id > ?", stmt.sql
      assert_equal "", stmt.remainder
      assert stmt.active?
      refute stmt.done?
      assert_equal [["b"], ["c"]], stmt.execute(1).to_a
      assert stmt.done?
      refute stmt.active?
      stmt.reset!
      stmt.bind_param(1, 2)
      assert_equal ["c"], stmt.step
      assert_nil stmt.step
      assert stmt.done?
      assert_equal ["a"], stmt.execute(0).next
      refute stmt.closed?
      stmt.close
      assert stmt.closed?
      e = assert_raises(SQLite3::Exception) { stmt.execute }
      assert_equal "cannot use a closed statement", e.message
      rows = [] #: Array[untyped]
      db.prepare("select id from t where id < 3") { |s| s.each { |r| rows << r } }
      assert_equal [[1], [2]], rows
      assert_equal " select 2", db.prepare("select 1; select 2").remainder
      assert_equal [[1]], db.execute("select 1; select 2")
      s = db.prepare("select 1 as n, 2")
      assert_equal [%w[n 2], [nil, nil]], [s.columns, s.types]
      assert_equal "n", s.column_name(0)
      assert_nil s.column_decltype(0)
      s.close
    end

    def test_result_set
      db = seeded
      got = [] #: Array[untyped]
      db.query("select id from t order by id limit 2") { |rs| got.push(rs.columns, rs.next, rs.eof?, rs.next, rs.next, rs.eof?) }
      assert_equal [["id"], [1], false, [2], nil, true], got
      rs = db.query("select id, name from t order by id limit 1")
      assert_equal({ "id" => 1, "name" => "a" }, rs.next_hash)
      assert_nil rs.next_hash
      rs.close
      assert rs.closed?
      rs = db.query("select id from t where id = ?", [2])
      hashes = [] #: Array[Hash[String, untyped]]
      rs.each_hash { |h| hashes << h }
      assert_equal [{ "id" => 2 }], hashes
      rs.reset(3)
      assert_equal [[3]], rs.to_a
      assert_equal ["integer"], rs.types
      rs.close
    end

    def test_errors
      db = seeded
      begin
        db.execute("selec 1")
        flunk "no SQLException"
      rescue SQLite3::SQLException => e
        assert_equal "near \"selec\": syntax error:\nselec 1\n^", e.message
        assert_equal 1, e.code
        assert_equal "selec 1", e.sql
        assert_equal 0, e.sql_offset
      end
      begin
        db.execute("insert into t (id, name) values (1, 'dup')")
        flunk "no ConstraintException"
      rescue SQLite3::ConstraintException => e
        assert_equal "UNIQUE constraint failed: t.id", e.message
        assert_equal 19, e.code
        assert_nil e.sql
      end
      e = assert_raises(SQLite3::SQLException) { db.execute("insert into t values (?)", [1, 2]) }
      assert_equal "table t has 4 columns but 1 values were supplied:\ninsert into t values (?)", e.message
      assert_equal "no such column: x:\nselect x\n       ^", assert_raises(SQLite3::SQLException) { db.execute("select x") }.message
      assert_equal "no such column: x", db.errmsg
      assert_equal 1, db.errcode
      db.close
      assert db.closed?
      ae = assert_raises(ArgumentError) { db.execute("select 1") }
      assert_equal "prepare called on a closed database", ae.message
      begin
        SQLite3::Database.new("/nonexistent/dir/x.db")
        flunk "no CantOpenException"
      rescue SQLite3::CantOpenException => e
        assert_equal "unable to open database file", e.message
        assert_equal 14, e.code
      end
    end

    def test_transactions
      db = seeded
      assert_equal(7, db.transaction do |d| 
                        d.execute("insert into t (name) values ('tx')")
 7
end)
      refute db.transaction_active?
      e = assert_raises(RuntimeError) do
        db.transaction do 
          db.execute("insert into t (name) values ('rb')")
 raise "boom"
end
      end
      assert_equal "boom", e.message
      assert_equal 4, db.get_first_value("select count(*) from t")
      assert_equal true, db.transaction
      assert db.transaction_active?
      assert_equal true, db.commit
      refute db.transaction_active?
      db.transaction(:immediate)
      db.execute("insert into t (name) values ('x')")
      assert_equal true, db.rollback
      assert_equal 4, db.get_first_value("select count(*) from t")
      assert_nil db.execute_batch("insert into t (name) values ('x'); insert into t (name) values ('y');")
      assert_equal 6, db.get_first_value("select count(*) from t")
      db.execute_batch("insert into t (name) values (?)", ["z"])
      assert_equal 7, db.get_first_value("select count(*) from t")
    end

    def test_functions
      db = seeded
      db.create_function("twice", 1) { |fn, x| fn.result = x * 2 }
      assert_equal [[42]], db.execute("select twice(21)")
      assert_equal [[3.0]], db.execute("select twice(1.5)")
      db.create_function("joined", 2) { |fn, a, b| fn.result = "#{a}-#{b}" }
      assert_equal [["a-1"]], db.execute("select joined(name, id) from t where id = 1")
      db.create_function("nada", 0) { |fn| fn.result = nil }
      assert_equal [[nil]], db.execute("select nada()")
      db.create_function("blobby", 1) { |fn, s| fn.result = SQLite3::Blob.new(s.to_s) }
      assert_equal [["blob"]], db.execute("select typeof(blobby('x'))")
      db.create_function("boom", 0) { |_fn| raise "kaboom" }
      e = assert_raises(RuntimeError) { db.execute("select boom()") }
      assert_equal "kaboom", e.message
      db.define_function("add3") { |a, b, c| a + b + c }
      assert_equal [[6]], db.execute("select add3(1, 2, 3)")
    end

    def test_pragmas_and_schema
      db = seeded
      info = db.table_info("t")
      assert_equal(%w[id name score data], info.map { |c| c["name"] })
      assert_equal(%w[integer text real blob], info.map { |c| c["type"] })
      assert_equal([1, 0, 0, 0], info.map { |c| c["pk"] })
      assert_equal([nil, nil, nil, nil], info.map { |c| c["dflt_value"] })
      db.execute("create table d (a text default 'x''y', b integer default null, c integer default 3)")
      assert_equal(["x'y", nil, "3"], db.table_info("d").map { |c| c["dflt_value"] })
      assert_equal 0, db.user_version
      db.user_version = 3
      assert_equal 3, db.user_version
      assert_equal "memory", db.journal_mode
      refute db.foreign_keys
      db.foreign_keys = true
      assert db.foreign_keys
      db.foreign_keys = "off"
      refute db.foreign_keys
      assert_equal [], db.index_list("t")
      db.execute("create index i on t(name)")
      assert_equal [[0, "i", 0, "c", 0]], db.index_list("t")
      assert_equal [[0, 1, "name"]], db.index_info("i")
      assert_equal 2, db.synchronous
      db.synchronous = :off
      assert_equal 0, db.synchronous
      assert_equal 100, (db.busy_timeout = 100)
      assert db.complete?("select 1;")
      refute db.complete?("select 1")
      assert_equal "", db.filename
      assert_equal Encoding::UTF_8, db.encoding
      refute db.readonly?
      assert_equal "it''s", SQLite3::Database.quote("it's")
      assert_equal 3, SQLite3::SQLITE_VERSION.split(".").first.to_i
      assert SQLite3.libversion > 3_000_000 # the gem bundles its own SQLite; rugaroo loads the system's, so the numbers differ
    end

    def test_file_database
      Dir.mktmpdir do |dir|
        path = File.join(dir, "t.db")
        SQLite3::Database.open(path) do |d|
          d.execute("create table k (v)")
          d.execute("insert into k values (1)")
          assert_equal "t.db", File.basename(d.filename.to_s)
        end
        ro = SQLite3::Database.new(path, readonly: true)
        assert ro.readonly?
        assert_equal [[1]], ro.execute("select * from k")
        e = assert_raises(SQLite3::ReadOnlyException) { ro.execute("insert into k values (2)") }
        assert_equal "attempt to write a readonly database", e.message
        ro.close
        rw = SQLite3::Database.new(path, readwrite: true)
        rw.execute("insert into k values (2)")
        assert_equal 2, rw.get_first_value("select count(*) from k")
        rw.close
        assert_raises(SQLite3::CantOpenException) { SQLite3::Database.new(File.join(dir, "missing.db"), readonly: true) }
      end
    end
  end
end
