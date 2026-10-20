# frozen_string_literal: true

Gem::Specification.new do |s|
  s.name = "sqlite3-fiddle"
  s.version = "0.1.0"
  s.summary = "The sqlite3 gem's API in pure Ruby over Fiddle: no C extension, runs on MRI and compiles under rb2go"
  s.description = "SQLite3::Database, Statement and ResultSet as in the sqlite3 gem (2.10), calling the system libsqlite3 through Fiddle. Vendored in rb2go (decision 328)."
  s.authors = ["JT Archie"]
  s.license = "MIT"
  s.homepage = "https://github.com/jtarchie/ruby2go/tree/main/gems/sqlite3-fiddle"
  s.files = ["lib/sqlite3.rb", "README.md"]
  s.required_ruby_version = ">= 3.3"
  s.add_dependency "fiddle", ">= 1.1"
end
