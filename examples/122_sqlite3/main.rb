# SQLite through the sqlite3 gem's API: rugaroo binds libsqlite3 over Fiddle instead of the gem's C extension.
require "sqlite3"
require "tmpdir"

db = SQLite3::Database.new(":memory:")
db.execute("create table books (id integer primary key, title text not null, year integer, price real, cover blob)")

# Binds by position, as an Array...
db.execute("insert into books (title, year, price) values (?, ?, ?)", ["Dune", 1965, 9.99])
# ...as a single value, and by name.
db.execute("insert into books (title) values (?)", "Untitled")
db.execute("insert into books (title, year, price, cover) values (:t, :y, :p, :c)",
           { "t" => "Neuromancer", "y" => 1984, "p" => 12.5, "c" => SQLite3::Blob.new("\x89PNG\x00") })
puts "inserted #{db.total_changes} rows; last id #{db.last_insert_row_id}"

# Rows are Arrays of Integer, Float, String, binary String or nil.
db.execute("select id, title, year, price from books order by id").each do |row|
  puts "  #{row[0]}: #{row[1]} (#{row[2].inspect}, #{row[3].inspect})"
end
cover = db.get_first_value("select cover from books where title = ?", "Neuromancer")
puts "cover is #{cover.bytesize} bytes, starts with #{cover.unpack1("H8")}"

# execute2 puts the column names first; results_as_hash gives Hashes.
p db.execute2("select title, year from books where year > ?", 1900)
db.results_as_hash = true
db.execute("select title, price from books where price is not null") { |row| puts "  #{row["title"]}: $#{row["price"]}" }
db.results_as_hash = false

# A prepared statement runs many times.
stmt = db.prepare("select count(*) from books where year < ?")
puts "columns #{stmt.columns.inspect}, #{stmt.bind_parameter_count} parameter"
[1970, 1990, 2000].each { |y| puts "  before #{y}: #{stmt.execute(y).next[0]}" }
stmt.close

# A transaction rolls back when its block raises.
begin
  db.transaction do
    db.execute("insert into books (title) values ('Draft')")
    db.execute("insert into books (id, title) values (1, 'Duplicate')")
  end
rescue SQLite3::ConstraintException => e
  puts "rolled back: #{e.message} (code #{e.code})"
end
puts "still #{db.get_first_value("select count(*) from books")} books"

# A SQL function defined in Ruby.
db.create_function("shout", 1) { |fn, s| fn.result = s.to_s.upcase + "!" }
p db.execute("select shout(title) from books where id = 1")

# Schema introspection, and a syntax error shows where it is.
db.table_info("books").each { |col| puts "  #{col["name"]} #{col["type"]}#{" pk" if col["pk"] == 1}" }
begin
  db.execute("selec 1")
rescue SQLite3::SQLException => e
  puts e.message
end

# A database file.
path = File.join(Dir.tmpdir, "rugaroo-example-122.db")
File.delete(path) if File.exist?(path)
SQLite3::Database.open(path) do |d|
  d.execute("create table kv (k text primary key, v text)")
  d.execute("insert into kv values ('a', '1')")
end
ro = SQLite3::Database.new(path, readonly: true)
p ro.execute("select * from kv"), ro.readonly?
begin
  ro.execute("insert into kv values ('b', '2')")
rescue SQLite3::ReadOnlyException => e
  puts "readonly: #{e.message}"
end
ro.close
File.delete(path)
db.close
puts "closed: #{db.closed?}; SQLite #{SQLite3::SQLITE_VERSION.split(".").first}.x"
