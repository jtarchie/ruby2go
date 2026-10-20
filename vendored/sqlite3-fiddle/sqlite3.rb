# rbs_inline: enabled

# The sqlite3 gem's lib/sqlite3.rb, with its C extension swapped for the Fiddle port in sqlite3/native.

require "sqlite3/native"
require "sqlite3/database"
require "sqlite3/version"

module SQLite3
  #: () -> bool
  def self.threadsafe? = threadsafe > 0 # Was sqlite3 compiled with thread safety on?
end

require "sqlite3/version_info"
