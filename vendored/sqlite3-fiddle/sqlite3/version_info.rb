# rbs_inline: enabled

module SQLite3
  VERSION_INFO = { # a hash of descriptive metadata about the current version of the sqlite3 gem
    ruby: defined?(::RUBY_DESCRIPTION) ? ::RUBY_DESCRIPTION : "ruby #{RUBY_VERSION}", # rugaroo has no RUBY_DESCRIPTION
    gem: {
      version: SQLite3::VERSION
    },
    sqlite: {
      compiled: SQLite3::SQLITE_VERSION,
      loaded: SQLite3::SQLITE_LOADED_VERSION,
      packaged: SQLite3::SQLITE_PACKAGED_LIBRARIES,
      precompiled: SQLite3::SQLITE_PRECOMPILED_LIBRARIES,
      sqlcipher: SQLite3.sqlcipher?,
      threadsafe: SQLite3.threadsafe?
    }
  } #: Hash[Symbol, untyped]
end
