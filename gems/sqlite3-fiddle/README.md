# sqlite3-fiddle

The [sqlite3 gem](https://github.com/sparklemotion/sqlite3-ruby)'s API (`SQLite3::Database`, `Statement`,
`ResultSet`, pragmas, error classes) in pure Ruby, calling the system `libsqlite3` through Fiddle. No C
extension, so it runs on MRI and compiles under [rb2go](https://github.com/jtarchie/ruby2go), which vendors
it here and answers `require "sqlite3"` with it (docs/design.md decision 328).

```ruby
require "sqlite3"

db = SQLite3::Database.new(":memory:")
db.execute("create table t (id integer primary key, name text)")
db.execute("insert into t (name) values (?)", ["a"])
p db.execute("select * from t")   # [[1, "a"]]
```

Under MRI with the real `sqlite3` gem installed too, put this gem's `lib` first on the load path
(`ruby -I gems/sqlite3-fiddle/lib`) or do not install the other. The library is `libsqlite3.dylib` on macOS
and `libsqlite3.so.0` elsewhere.

Differences from the sqlite3 gem: `SQLite3::Blob` is a marker class rather than a String subclass (columns
still read back as Strings); `prepare`, `query` and `execute` with a block return the Statement, ResultSet or
an empty Array, not the block's value; `create_function` blocks take up to six arguments; no aggregates,
authorizers, busy handlers, tracing, collations, extensions, backup or serialize.

Tests: rb2go's `testdata/test/sqlite3_test.rb` runs against this gem under MRI
(`ruby -I gems/sqlite3-fiddle/lib ... testdata/test/sqlite3_test.rb`) and against the real gem, and the
compiled program's output must match MRI's.
