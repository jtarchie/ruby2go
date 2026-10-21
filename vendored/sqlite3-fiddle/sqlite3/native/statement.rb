# rbs_inline: enabled

# ext/sqlite3/statement.c over Fiddle.

require "sqlite3/native/ffi"
require "sqlite3/native/exception"

module SQLite3
  class Statement
    STAT_KEYS = { fullscan_steps: 1, sorts: 2, autoindexes: 3, vm_steps: 4, reprepares: 5, runs: 6, filter_misses: 7, filter_hits: 8 }.freeze #: Hash[Symbol, Integer]

    #: () -> Integer
    def __st # REQUIRE_LIVE_DB then REQUIRE_OPEN_STMT
      raise SQLite3::Exception, "cannot use a statement associated with a discarded database" if @connection&.__discarded?

      st = @st
      raise SQLite3::Exception, "cannot use a closed statement" if st.nil? || st.zero?

      st
    end

    #: (Database, String) -> String
    private def prepare(db, sql)
      h = db.__handle
      sql = C.string(sql)
      db.__clear_deadline
      src = Fiddle::Pointer.malloc(sql.bytesize + 1, Fiddle::RUBY_FREE)
      src[0, sql.bytesize + 1] = "#{sql.b}\0"
      tail = 0
      status = 0
      begin
        @st = C.out { |out| tail = C.out { |t| status = C.sqlite3_prepare_v2(h, src, sql.bytesize, out, t) } } #: Integer?
        db.__raise_pending
        SQLite3.__check_prepare(h, status, C.cstr(src).to_s)
        used = tail.zero? ? sql.bytesize : tail - src.to_i
        C.ustr(Fiddle::Pointer.new(src.to_i + used)).to_s
      ensure
        src.call_free
      end
    end

    #: () -> self
    def close
      st = __st_open
      C.sqlite3_finalize(st)
      @st = nil
      self
    end

    #: () -> Integer
    def __st_open # sqlite3_rb_close skips REQUIRE_LIVE_DB
      st = @st
      raise SQLite3::Exception, "cannot use a closed statement" if st.nil? || st.zero?

      st
    end

    #: () -> bool
    def closed?
      st = @st
      st.nil? || st.zero?
    end

    #: () -> Array[untyped]?
    def step
      st = __st
      return nil if @done

      @connection.__clear_deadline if C.sqlite3_stmt_busy(st).zero?
      value = C.sqlite3_step(st)
      @connection.__raise_pending
      case value
      when 100 then __row(st)
      when 101
        @done = true #: bool?
        nil
      else
        C.sqlite3_reset(st)
        @done = false
        SQLite3.__check(C.sqlite3_db_handle(st), value)
        nil
      end
    end

    #: (Integer) -> Array[untyped]
    def __row(st)
      n = C.sqlite3_column_count(st)
      internal = Encoding.default_internal
      row = [] #: Array[untyped]
      i = 0
      while i < n
        row << case C.sqlite3_column_type(st, i)
               when 1 then C.sqlite3_column_int64(st, i)
               when 2 then C.sqlite3_column_double(st, i)
               when 3
                 text = C.text(C.sqlite3_column_text(st, i), C.sqlite3_column_bytes(st, i))
                 text = text.encode(internal) if internal
                 text.freeze
               when 4 then C.bytes(C.sqlite3_column_blob(st, i), C.sqlite3_column_bytes(st, i)).freeze
               end
        i += 1
      end
      row.freeze
    end

    #: (untyped, untyped) -> self
    def bind_param(key, value)
      st = __st
      index = case key
              when Symbol, String
                k = key.to_s
                k = ":#{k}" unless k.start_with?(":")
                C.sqlite3_bind_parameter_index(st, C.z(k))
              else Integer(key)
              end
      raise SQLite3::Exception, "no such bind parameter" if index.zero?

      status = case value
               when Blob # its own arm: rb2go's `when String` does not match a String subclass
                 blob = value.to_s
                 C.sqlite3_bind_blob(st, index, blob, blob.bytesize, C::TRANSIENT)
               when String
                 if value.instance_of?(Blob) || value.encoding == Encoding::BINARY
                   C.sqlite3_bind_blob(st, index, value, value.bytesize, C::TRANSIENT)
                 elsif value.encoding == Encoding::UTF_16LE || value.encoding == Encoding::UTF_16BE
                   C.sqlite3_bind_text16(st, index, value, value.bytesize, C::TRANSIENT)
                 else
                   value = value.encode(Encoding::UTF_8)
                   C.sqlite3_bind_text(st, index, value, value.bytesize, C::TRANSIENT)
                 end
               when Integer
                 if value.bit_length < 64 # fits int64; a wider Integer binds as a double, as the C extension's bignum_to_int64 fallback does
                   C.sqlite3_bind_int64(st, index, value)
                 else
                   C.sqlite3_bind_double(st, index, value.to_f)
                 end
               when Float then C.sqlite3_bind_double(st, index, value)
               when nil then C.sqlite3_bind_null(st, index)
               else raise RuntimeError, "can't prepare #{value.class}"
               end
      SQLite3.__check(C.sqlite3_db_handle(st), status) unless status.zero? # the handle is only for the error message
      self
    end

    #: () -> self
    def reset!
      C.sqlite3_reset(__st)
      @done = false
      self
    end

    #: () -> self
    def clear_bindings!
      C.sqlite3_clear_bindings(__st)
      @done = false
      self
    end

    #: () -> bool
    def done? = @done ? true : false

    #: () -> Integer
    def column_count = C.sqlite3_column_count(__st)

    #: (Integer) -> String?
    def column_name(index)
      name = C.ustr(C.sqlite3_column_name(__st, Integer(index)))
      name.nil? ? nil : -name
    end

    #: (Integer) -> String?
    def column_decltype(index) = C.ustr(C.sqlite3_column_decltype(__st, Integer(index)))

    #: (Integer) -> String?
    private def column_decltype_downcased(index)
      t = column_decltype(index)
      t.nil? ? nil : -t.b.tr("A-Z", "a-z").force_encoding(Encoding::UTF_8) # ASCII-only, byte-wise: a decltype need not be valid UTF-8
    end

    #: () -> Integer
    def bind_parameter_count = C.sqlite3_bind_parameter_count(__st)

    #: () -> Array[String]
    def named_params
      st = __st
      params = [] #: Array[String]
      (1..C.sqlite3_bind_parameter_count(st)).each do |i|
        name = C.ustr(C.sqlite3_bind_parameter_name(st, i))
        params << -name[1..].to_s if name && !name.start_with?("?")
      end
      params.freeze
    end

    #: () -> String
    def sql = C.ustr(C.sqlite3_sql(__st)).to_s.freeze

    #: () -> String?
    def expanded_sql
      p = C.sqlite3_expanded_sql(__st)
      out = C.ustr(p)
      C.sqlite3_free(p) unless p.null?
      out&.freeze
    end

    if C.respond_to?(:sqlite3_column_database_name)
      #: (Integer) -> String?
      def database_name(index) = C.ustr(C.sqlite3_column_database_name(__st, Integer(index)))
    end

    #: () -> Integer
    def memused = C.sqlite3_stmt_status(__st, 99, 0) # SQLITE_STMTSTATUS_MEMUSED

    #: () -> Hash[Symbol, Integer]
    private def stats_as_hash
      st = __st
      STAT_KEYS.to_h { |k, op| [k, C.sqlite3_stmt_status(st, op, 0)] }
    end

    #: (untyped) -> Integer
    private def stat_for(key)
      st = __st
      raise TypeError, "non-symbol given" unless key.is_a?(Symbol)

      op = STAT_KEYS[key]
      raise ArgumentError, "unknown key: #{key}" if op.nil?

      C.sqlite3_stmt_status(st, op, 0)
    end
  end
end
