# rbs_inline: enabled

require "sqlite3/errors"

module SQLite3
  module Pragmas # the gem's pragma mode tables; its accessors live on Database, where rb2go can type self
    SYNCHRONOUS_MODES = { "full" => 2, "normal" => 1, "off" => 0 }.freeze #: Hash[String, Integer]

    TEMP_STORE_MODES = { "default" => 0, "file" => 1, "memory" => 2 }.freeze #: Hash[String, Integer]

    AUTO_VACUUM_MODES = { "none" => 0, "full" => 1, "incremental" => 2 }.freeze #: Hash[String, Integer]

    JOURNAL_MODES = { "delete" => "delete", "truncate" => "truncate", "persist" => "persist", "memory" => "memory", "wal" => "wal", "off" => "off" }.freeze #: Hash[String, String]

    LOCKING_MODES = { "normal" => "normal", "exclusive" => "exclusive" }.freeze #: Hash[String, String]

    ENCODINGS = { "utf-8" => "utf-8", "utf-16" => "utf-16", "utf-16le" => "utf-16le", "utf-16be" => "utf-16be" }.freeze #: Hash[String, String]

    WAL_CHECKPOINTS = { "passive" => "passive", "full" => "full", "restart" => "restart", "truncate" => "truncate" }.freeze #: Hash[String, String]
  end

  class Database
    #: (String) -> bool
    def get_boolean_pragma(name)
      get_first_value("PRAGMA #{name}") != 0
    end

    #: (String, untyped) -> untyped
    def set_boolean_pragma(name, mode)
      case mode
      when String
        case mode.downcase
        when "on", "yes", "true", "y", "t" then mode = "'ON'"
        when "off", "no", "false", "n", "f" then mode = "'OFF'"
        else
          raise SQLite3::Exception, "unrecognized pragma parameter #{mode.inspect}"
        end
      when true, 1
        mode = "ON"
      when false, 0, nil
        mode = "OFF"
      else
        raise SQLite3::Exception, "unrecognized pragma parameter #{mode.inspect}"
      end

      execute("PRAGMA #{name}=#{mode}")
    end

    #: (String, *untyped) ?{ (untyped) -> void } -> untyped
    def get_query_pragma(name, *params) # :yields: row
      sql = params.empty? ? "PRAGMA #{name}" : "PRAGMA #{name}( '#{params.join("','")}' )"
      return __execute_1(sql) unless block_given? # rb2go reads a forwarded nil &block as given

      execute(sql) { |row| yield row }
    end

    #: (String) -> untyped
    def get_enum_pragma(name)
      get_first_value("PRAGMA #{name}")
    end

    #: (String, untyped, Hash[String, untyped] | Array[Array[untyped]]) -> untyped
    def set_enum_pragma(name, mode, enums)
      match = if enums.is_a?(Array)
        enums.find { |p| p.find { |i| i.to_s.downcase == mode.to_s.downcase } }
      elsif mode.is_a?(String)
        enums.fetch(mode.downcase)
      else
        mode
      end

      unless match
        raise SQLite3::Exception, "unrecognized #{name} #{mode.inspect}"
      end

      execute("PRAGMA #{name}='#{match}'")
    end

    #: (String) -> Integer
    def get_int_pragma(name)
      get_first_value("PRAGMA #{name}").to_i
    end

    #: (String, untyped) -> untyped
    def set_int_pragma(name, value)
      execute("PRAGMA #{name}=#{value.to_i}")
    end

    #: () -> Integer
    def application_id
      get_int_pragma "application_id"
    end

    #: (untyped) -> untyped
    def application_id=(integer)
      set_int_pragma "application_id", integer
    end

    #: () -> untyped
    def auto_vacuum
      get_enum_pragma "auto_vacuum"
    end

    #: (untyped) -> untyped
    def auto_vacuum=(mode)
      set_enum_pragma "auto_vacuum", mode, AUTO_VACUUM_MODES
    end

    #: () -> bool
    def automatic_index
      get_boolean_pragma "automatic_index"
    end

    #: (untyped) -> untyped
    def automatic_index=(mode)
      set_boolean_pragma "automatic_index", mode
    end

    #: () -> Integer
    def cache_size
      get_int_pragma "cache_size"
    end

    #: (untyped) -> untyped
    def cache_size=(size)
      set_int_pragma "cache_size", size
    end

    #: () -> bool
    def cache_spill
      get_boolean_pragma "cache_spill"
    end

    #: (untyped) -> untyped
    def cache_spill=(mode)
      set_boolean_pragma "cache_spill", mode
    end

    #: (untyped) -> untyped
    def case_sensitive_like=(mode)
      set_boolean_pragma "case_sensitive_like", mode
    end

    #: () -> bool
    def cell_size_check
      get_boolean_pragma "cell_size_check"
    end

    #: (untyped) -> untyped
    def cell_size_check=(mode)
      set_boolean_pragma "cell_size_check", mode
    end

    #: () -> bool
    def checkpoint_fullfsync
      get_boolean_pragma "checkpoint_fullfsync"
    end

    #: (untyped) -> untyped
    def checkpoint_fullfsync=(mode)
      set_boolean_pragma "checkpoint_fullfsync", mode
    end

    #: () ?{ (untyped) -> void } -> untyped
    def collation_list(&block) # :yields: row
      get_query_pragma "collation_list", &block
    end

    #: () ?{ (untyped) -> void } -> untyped
    def compile_options(&block) # :yields: row
      get_query_pragma "compile_options", &block
    end

    #: () -> bool
    def count_changes
      get_boolean_pragma "count_changes"
    end

    #: (untyped) -> untyped
    def count_changes=(mode)
      set_boolean_pragma "count_changes", mode
    end

    #: () -> Integer
    def data_version
      get_int_pragma "data_version"
    end

    #: () ?{ (untyped) -> void } -> untyped
    def database_list(&block) # :yields: row
      get_query_pragma "database_list", &block
    end

    #: () -> Integer
    def default_cache_size
      get_int_pragma "default_cache_size"
    end

    #: (untyped) -> untyped
    def default_cache_size=(size)
      set_int_pragma "default_cache_size", size
    end

    #: () -> untyped
    def default_synchronous
      get_enum_pragma "default_synchronous"
    end

    #: (untyped) -> untyped
    def default_synchronous=(mode)
      set_enum_pragma "default_synchronous", mode, SYNCHRONOUS_MODES
    end

    #: () -> untyped
    def default_temp_store
      get_enum_pragma "default_temp_store"
    end

    #: (untyped) -> untyped
    def default_temp_store=(mode)
      set_enum_pragma "default_temp_store", mode, TEMP_STORE_MODES
    end

    #: () -> bool
    def defer_foreign_keys
      get_boolean_pragma "defer_foreign_keys"
    end

    #: (untyped) -> untyped
    def defer_foreign_keys=(mode)
      set_boolean_pragma "defer_foreign_keys", mode
    end

    #: () -> Encoding
    def encoding = Encoding.find(get_enum_pragma("encoding").to_s)

    #: (untyped) -> untyped
    def encoding=(mode)
      set_string_pragma "encoding", mode, ENCODINGS
    end

    #: (*untyped) ?{ (untyped) -> void } -> untyped
    def foreign_key_check(*table, &block) # :yields: row
      get_query_pragma "foreign_key_check", *table, &block
    end

    #: (untyped) ?{ (untyped) -> void } -> untyped
    def foreign_key_list(table, &block) # :yields: row
      get_query_pragma "foreign_key_list", table, &block
    end

    #: () -> bool
    def foreign_keys
      get_boolean_pragma "foreign_keys"
    end

    #: (untyped) -> untyped
    def foreign_keys=(mode)
      set_boolean_pragma "foreign_keys", mode
    end

    #: () -> Integer
    def freelist_count
      get_int_pragma "freelist_count"
    end

    #: () -> bool
    def full_column_names
      get_boolean_pragma "full_column_names"
    end

    #: (untyped) -> untyped
    def full_column_names=(mode)
      set_boolean_pragma "full_column_names", mode
    end

    #: () -> bool
    def fullfsync
      get_boolean_pragma "fullfsync"
    end

    #: (untyped) -> untyped
    def fullfsync=(mode)
      set_boolean_pragma "fullfsync", mode
    end

    #: (untyped) -> untyped
    def ignore_check_constraints=(mode)
      set_boolean_pragma "ignore_check_constraints", mode
    end

    #: (untyped) ?{ (untyped) -> void } -> untyped
    def incremental_vacuum(pages, &block) # :yields: row
      get_query_pragma "incremental_vacuum", pages, &block
    end

    #: (untyped) ?{ (untyped) -> void } -> untyped
    def index_info(index, &block) # :yields: row
      get_query_pragma "index_info", index, &block
    end

    #: (untyped) ?{ (untyped) -> void } -> untyped
    def index_list(table, &block) # :yields: row
      get_query_pragma "index_list", table, &block
    end

    #: (untyped) ?{ (untyped) -> void } -> untyped
    def index_xinfo(index, &block) # :yields: row
      get_query_pragma "index_xinfo", index, &block
    end

    #: (*untyped) ?{ (untyped) -> void } -> untyped
    def integrity_check(*num_errors, &block) # :yields: row
      get_query_pragma "integrity_check", *num_errors, &block
    end

    #: () -> untyped
    def journal_mode
      get_enum_pragma "journal_mode"
    end

    #: (untyped) -> untyped
    def journal_mode=(mode)
      set_string_pragma "journal_mode", mode, JOURNAL_MODES
    end

    #: () -> Integer
    def journal_size_limit
      get_int_pragma "journal_size_limit"
    end

    #: (untyped) -> untyped
    def journal_size_limit=(size)
      set_int_pragma "journal_size_limit", size
    end

    #: () -> bool
    def legacy_file_format
      get_boolean_pragma "legacy_file_format"
    end

    #: (untyped) -> untyped
    def legacy_file_format=(mode)
      set_boolean_pragma "legacy_file_format", mode
    end

    #: () -> untyped
    def locking_mode
      get_enum_pragma "locking_mode"
    end

    #: (untyped) -> untyped
    def locking_mode=(mode)
      set_string_pragma "locking_mode", mode, LOCKING_MODES
    end

    #: () -> Integer
    def max_page_count
      get_int_pragma "max_page_count"
    end

    #: (untyped) -> untyped
    def max_page_count=(size)
      set_int_pragma "max_page_count", size
    end

    #: () -> Integer
    def mmap_size
      get_int_pragma "mmap_size"
    end

    #: (untyped) -> untyped
    def mmap_size=(size)
      set_int_pragma "mmap_size", size
    end

    #: (?Integer?) -> untyped
    def optimize(bitmask = nil)
      if bitmask
        set_int_pragma "optimize", bitmask
      else
        execute("PRAGMA optimize")
      end
    end

    #: () -> Integer
    def page_count
      get_int_pragma "page_count"
    end

    #: () -> Integer
    def page_size
      get_int_pragma "page_size"
    end

    #: (untyped) -> untyped
    def page_size=(size)
      set_int_pragma "page_size", size
    end

    #: (untyped) -> untyped
    def parser_trace=(mode)
      set_boolean_pragma "parser_trace", mode
    end

    #: () -> bool
    def query_only
      get_boolean_pragma "query_only"
    end

    #: (untyped) -> untyped
    def query_only=(mode)
      set_boolean_pragma "query_only", mode
    end

    #: (*untyped) ?{ (untyped) -> void } -> untyped
    def quick_check(*num_errors, &block) # :yields: row
      get_query_pragma "quick_check", *num_errors, &block
    end

    #: () -> bool
    def read_uncommitted
      get_boolean_pragma "read_uncommitted"
    end

    #: (untyped) -> untyped
    def read_uncommitted=(mode)
      set_boolean_pragma "read_uncommitted", mode
    end

    #: () -> bool
    def recursive_triggers
      get_boolean_pragma "recursive_triggers"
    end

    #: (untyped) -> untyped
    def recursive_triggers=(mode)
      set_boolean_pragma "recursive_triggers", mode
    end

    #: () -> bool
    def reverse_unordered_selects
      get_boolean_pragma "reverse_unordered_selects"
    end

    #: (untyped) -> untyped
    def reverse_unordered_selects=(mode)
      set_boolean_pragma "reverse_unordered_selects", mode
    end

    #: () -> Integer
    def schema_cookie
      get_int_pragma "schema_cookie"
    end

    #: (untyped) -> untyped
    def schema_cookie=(cookie)
      set_int_pragma "schema_cookie", cookie
    end

    #: () -> Integer
    def schema_version
      get_int_pragma "schema_version"
    end

    #: (untyped) -> untyped
    def schema_version=(version)
      set_int_pragma "schema_version", version
    end

    #: () -> bool
    def secure_delete
      get_boolean_pragma "secure_delete"
    end

    #: (untyped) -> untyped
    def secure_delete=(mode)
      set_boolean_pragma "secure_delete", mode
    end

    #: () -> bool
    def short_column_names
      get_boolean_pragma "short_column_names"
    end

    #: (untyped) -> untyped
    def short_column_names=(mode)
      set_boolean_pragma "short_column_names", mode
    end

    #: () -> untyped
    def shrink_memory
      execute("PRAGMA shrink_memory")
    end

    #: () -> Integer
    def soft_heap_limit
      get_int_pragma "soft_heap_limit"
    end

    #: (untyped) -> untyped
    def soft_heap_limit=(mode)
      set_int_pragma "soft_heap_limit", mode
    end

    #: () ?{ (untyped) -> void } -> untyped
    def stats(&block) # :yields: row
      get_query_pragma "stats", &block
    end

    #: () -> untyped
    def synchronous
      get_enum_pragma "synchronous"
    end

    #: (untyped) -> untyped
    def synchronous=(mode)
      set_enum_pragma "synchronous", mode, SYNCHRONOUS_MODES
    end

    #: () -> untyped
    def temp_store
      get_enum_pragma "temp_store"
    end

    #: (untyped) -> untyped
    def temp_store=(mode)
      set_enum_pragma "temp_store", mode, TEMP_STORE_MODES
    end

    #: () -> Integer
    def threads
      get_int_pragma "threads"
    end

    #: (untyped) -> untyped
    def threads=(count)
      set_int_pragma "threads", count
    end

    #: () -> Integer
    def user_cookie
      get_int_pragma "user_cookie"
    end

    #: (untyped) -> untyped
    def user_cookie=(cookie)
      set_int_pragma "user_cookie", cookie
    end

    #: () -> Integer
    def user_version
      get_int_pragma "user_version"
    end

    #: (untyped) -> untyped
    def user_version=(version)
      set_int_pragma "user_version", version
    end

    #: (untyped) -> untyped
    def vdbe_addoptrace=(mode)
      set_boolean_pragma "vdbe_addoptrace", mode
    end

    #: (untyped) -> untyped
    def vdbe_debug=(mode)
      set_boolean_pragma "vdbe_debug", mode
    end

    #: (untyped) -> untyped
    def vdbe_listing=(mode)
      set_boolean_pragma "vdbe_listing", mode
    end

    #: () -> bool
    def vdbe_trace
      get_boolean_pragma "vdbe_trace"
    end

    #: (untyped) -> untyped
    def vdbe_trace=(mode)
      set_boolean_pragma "vdbe_trace", mode
    end

    #: () -> Integer
    def wal_autocheckpoint
      get_int_pragma "wal_autocheckpoint"
    end

    #: (untyped) -> untyped
    def wal_autocheckpoint=(mode)
      set_int_pragma "wal_autocheckpoint", mode
    end

    #: () -> untyped
    def wal_checkpoint
      get_enum_pragma "wal_checkpoint"
    end

    #: (untyped) -> untyped
    def wal_checkpoint=(mode)
      set_string_pragma "wal_checkpoint", mode, WAL_CHECKPOINTS
    end

    #: (untyped) -> untyped
    def writable_schema=(mode)
      set_boolean_pragma "writable_schema", mode
    end

    #: (String) { (Hash[String, untyped]) -> void } -> untyped
    def table_info(table)
      rows = __table_info_1(table)
      return rows unless block_given?

      rows.each { |row| yield row }
      nil
    end

    #: (String) -> Array[Hash[String, untyped]]
    def __table_info_1(table) # rb2go's overload for a blockless table_info: typed, where the block form returns nil
      stmt = prepare "PRAGMA table_info(#{table})"
      columns = stmt.columns

      needs_tweak_default =
        version_compare(SQLite3.libversion.to_s, "3.3.7") > 0

      result = [] #: Array[Hash[String, untyped]]
      stmt.each do |row|
        new_row = columns.zip(row).to_h #: Hash[String, untyped]

        tweak_default(new_row) if needs_tweak_default

        if new_row["type"]
          new_row["type"] = new_row["type"].downcase
        end

        result << new_row
      end
      stmt.close

      result
    end

    private

    #: (String, untyped, Hash[String, String]) -> untyped
    def set_string_pragma(pragma_name, value, valid_values)
      valid_values.fetch(value.to_s.downcase) {
        raise SQLite3::Exception, "unrecognized #{pragma_name} #{value.inspect}"
      }
      set_enum_pragma(pragma_name, value, valid_values)
    end

    #: (String, String) -> Integer
    def version_compare(v1, v2)
      a1 = v1.split(".").map { |i| i.to_i }
      a2 = v2.split(".").map { |i| i.to_i }
      parts = [a1.length, a2.length].max.to_i
      a1.push 0 while a1.length < parts
      a2.push 0 while a2.length < parts
      a1 <=> a2
    end

    #: (Hash[String, untyped]) -> void
    def tweak_default(hash)
      dflt = hash["dflt_value"]
      return unless dflt.is_a?(String)

      case dflt
      when /^null$/i
        hash["dflt_value"] = nil
      when /^'(.*)'$/m
        hash["dflt_value"] = $1.to_s.gsub("''", "'")
      when /^"(.*)"$/m
        hash["dflt_value"] = $1.to_s.gsub('""', '"')
      end
    end
  end
end
