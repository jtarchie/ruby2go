# rbs_inline: enabled

# Stands in for the sqlite3 gem's C extension (sqlite3_native): the same classes and methods, over Fiddle and the system libsqlite3.

require "sqlite3/native/sqlite3"
require "sqlite3/native/exception"
require "sqlite3/native/database"
require "sqlite3/native/statement"
require "sqlite3/native/backup"
