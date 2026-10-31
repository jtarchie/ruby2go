# rbs_inline: enabled

# ext/sqlite3/database.c and aggregator.c over Fiddle.

require "sqlite3/native/ffi"
require "sqlite3/native/exception"

module SQLite3
  class Database
    #: () -> Integer
    def __handle # the open sqlite3*; REQUIRE_OPEN_DB
      h = @db
      raise SQLite3::Exception, "cannot use a closed database" if h.nil? || h.zero?

      h
    end

    #: () -> bool
    def __discarded? = @discarded ? true : false

    #: (::Exception) -> void
    def __park(e) # Ruby exceptions cannot unwind through sqlite's C frames: a callback parks its exception here and the call that ran it (step, prepare, exec) raises it.
      @pending_error = e #: ::Exception?
    end

    #: () -> void
    def __raise_pending
      e = @pending_error
      return if e.nil?

      @pending_error = nil
      raise e
    end

    #: () -> void
    def __clear_deadline
      @stmt_deadline = nil #: Float?
    end

    #: (untyped, ?untyped) -> untyped
    def __keep(closure, slot = nil) # sqlite holds the raw function pointer, so the closure must outlive its registration; a slot's previous closure is released
      pinned = [closure]
      pinned << Fiddle::Pointer.to_ptr(closure) if GC.respond_to?(:compact) && !closure.nil? # fiddle hands libffi the closure's VALUE unpinned; a Pointer wrapping it marks it with rb_gc_mark, which pins
      if slot.nil?
        @closures = [] if @closures.nil? #: Array[Array[untyped]]?
        @closures&.push(pinned)
      else
        @slots = {} if @slots.nil? #: Hash[untyped, Array[untyped]]?
        @slots&.store(slot, pinned)
      end
      closure
    end

    #: (String, Integer, untyped) -> self
    private def open_v2(file, mode, zvfs)
      raise SQLite3::Exception, "cannot open a database that is already open" if !@db.nil? && !@db.zero?

      @owner = Process.pid #: Integer?
      file = file.to_str
      status = 0
      @db = C.out { |out| status = C.sqlite3_open_v2(C.zc(file), out, mode, zvfs.nil? ? nil : C.zc(zvfs.to_str)) } #: Integer?
      if status != 0
        msg = C.cstr(C.sqlite3_errmsg(@db)).to_s
        C.sqlite3_close_v2(@db)
        @db = nil
        raise SQLite3.__exception(status, msg).__set(status)
      end
      @readonly_flag = mode & Constants::Open::READONLY != 0 #: bool?
      self
    end

    #: (String) -> Integer
    private def open16(file)
      raise SQLite3::Exception, "cannot open a database that is already open" if !@db.nil? && !@db.zero?

      @owner = Process.pid
      str = file.to_str.dup
      unless str.encoding == Encoding::UTF_16LE || str.encoding == Encoding::UTF_16BE
        str.force_encoding([1].pack("S") == "\x01\x00" ? Encoding::UTF_16LE : Encoding::UTF_16BE) # an untagged string holds native-order UTF-16
      end
      raise ArgumentError, "string contains null char" if str.codepoints.include?(0)

      bytes = "#{str.b}\0\0"
      status = 0
      @db = C.out { |out| status = C.sqlite3_open16(bytes, out) }
      if status != 0
        msg = C.cstr(C.sqlite3_errmsg(@db)).to_s
        C.sqlite3_close_v2(@db)
        @db = nil
        raise SQLite3.__exception(status, msg).__set(status)
      end
      status
    end

    #: () -> bool
    private def disable_quirk_mode
      h = @db
      cfg = C::DB_CONFIG
      return false if h.nil? || h.zero? || cfg.nil?

      cfg.call(Fiddle::Pointer.new(h), 1013, :int, 0, :voidp, nil) # SQLITE_DBCONFIG_DQS_DDL
      cfg.call(Fiddle::Pointer.new(h), 1014, :int, 0, :voidp, nil) # SQLITE_DBCONFIG_DQS_DML
      true
    end

    #: () -> self
    def close # a writable connection carried across fork() is discarded, not closed: closing it there may corrupt the database
      h = @db
      if !h.nil? && !h.zero?
        if @readonly_flag || @owner == Process.pid
          C.sqlite3_close_v2(h)
          @db = nil
        else
          discard
        end
      end
      self
    end

    #: () -> self
    private def discard
      h = @db
      return self if h.nil? || h.zero?

      C.sqlite3_db_release_memory(h)
      names = [] #: Array[String?]
      db_name = C::DB_NAME
      if db_name.nil?
        names << nil
      else
        i = 0
        while (name = C.cstr(Fiddle::Pointer.new(db_name.call(h, i).to_i)))
          names << name
          i += 1
        end
      end
      names.each { |name| __close_file(h, name, 7) } # SQLITE_FCNTL_FILE_POINTER
      __close_file(h, nil, 28) # SQLITE_FCNTL_JOURNAL_POINTER
      @db = nil
      @discarded = true #: bool?
      self
    end

    #: (Integer, String?, Integer) -> void
    def __close_file(h, name, op) # file descriptors only: sqlite3_close would write to a database another process may own
      status = 0
      sfile = C.out { |out| status = C.sqlite3_file_control(h, name.nil? ? nil : C.z(name), op, out) }
      return unless status == 0 && sfile != 0

      methods = Fiddle::Pointer.new(sfile).ptr.to_i
      return if methods.zero?

      x_close = Fiddle::Pointer.new(methods + Fiddle::SIZEOF_VOIDP).ptr.to_i # sqlite3_io_methods: int iVersion (padded), then xClose
      Fiddle::Function.new(x_close, [Fiddle::TYPE_VOIDP], Fiddle::TYPE_INT).call(sfile)
    end

    #: () -> bool
    def closed?
      h = @db
      h.nil? || h.zero?
    end

    #: () -> Integer
    def total_changes = C.sqlite3_total_changes(__handle)

    #: () -> Integer
    def last_insert_row_id = C.sqlite3_last_insert_rowid(__handle)

    #: () -> Integer
    def changes = C.sqlite3_changes(__handle)

    #: () -> self
    def interrupt
      C.sqlite3_interrupt(__handle)
      self
    end

    #: () -> String
    def errmsg = C.cstr(C.sqlite3_errmsg(__handle)).to_s

    #: () -> Integer
    def errcode = C.sqlite3_errcode(__handle)

    #: (String) -> bool
    def complete?(sql) = C.sqlite3_complete(C.z(sql.to_str)) != 0

    #: () -> bool
    def transaction_active? = C.sqlite3_get_autocommit(__handle).zero?

    #: (untyped) -> self
    def extended_result_codes=(enable)
      h = __handle
      SQLite3.__check(h, C.sqlite3_extended_result_codes(h, enable ? 1 : 0))
      self
    end

    #: (Integer) -> self
    def busy_timeout=(timeout)
      h = __handle
      SQLite3.__check(h, C.sqlite3_busy_timeout(h, Integer(timeout)))
      self
    end

    #: (?untyped) ?{ (Integer) -> untyped } -> self
    def busy_handler(handler = nil, &block)
      h = __handle
      handler = block if handler.nil? && block
      @busy_handler = handler #: untyped
      cb = nil
      unless handler.nil?
        cb = Fiddle::Closure::BlockCaller.new(Fiddle::TYPE_INT, [Fiddle::TYPE_VOIDP, Fiddle::TYPE_INT]) do |_, count|
          (@busy_handler.call(count) == false) ? 0 : 1
        rescue ::Exception => e
          __park(e)
          0
        end
      end
      SQLite3.__check(h, C.sqlite3_busy_handler(h, cb, nil))
      __keep(cb, :busy)
      self
    end

    #: (Integer) -> self
    def statement_timeout=(milliseconds)
      @stmt_timeout = Integer(milliseconds) #: Integer?
      @stmt_deadline = nil
      @progress ||= __keep(Fiddle::Closure::BlockCaller.new(Fiddle::TYPE_INT, [Fiddle::TYPE_VOIDP]) do |_|
        now = Process.clock_gettime(Process::CLOCK_MONOTONIC)
        deadline = @stmt_deadline
        if deadline.nil?
          @stmt_deadline = now + (@stmt_timeout.to_i / 1000.0)
          0
        else
          (now > deadline) ? 1 : 0
        end
      end)
      C.sqlite3_progress_handler(@db, @stmt_timeout.to_i.zero? ? -1 : 1000, @progress, nil)
      self
    end

    #: (?untyped) ?{ (String) -> untyped } -> self
    def trace(tracer = nil, &block)
      h = __handle
      tracer = block if tracer.nil? && block
      @trace_handler = tracer #: untyped
      cb = nil
      unless tracer.nil?
        cb = Fiddle::Closure::BlockCaller.new(Fiddle::TYPE_VOID, [Fiddle::TYPE_VOIDP, Fiddle::TYPE_VOIDP]) do |_, sql|
          @trace_handler.call(C.cstr(Fiddle::Pointer.new(sql.to_i)))
        rescue ::Exception => e
          __park(e)
        end
      end
      C.sqlite3_trace(h, cb, nil)
      __keep(cb, :trace)
      self
    end

    #: (untyped) -> self
    def authorizer=(authorizer)
      h = __handle
      cb = nil
      unless authorizer.nil?
        cb = Fiddle::Closure::BlockCaller.new(Fiddle::TYPE_INT, [Fiddle::TYPE_VOIDP, Fiddle::TYPE_INT, Fiddle::TYPE_VOIDP, Fiddle::TYPE_VOIDP, Fiddle::TYPE_VOIDP, Fiddle::TYPE_VOIDP]) do |_, action, a, b, c, d|
          args = [a, b, c, d].map { |p| C.cstr(Fiddle::Pointer.new(p.to_i)) }
          result = @authorizer.call(action, args[0], args[1], args[2], args[3])
          case result
          when Integer then result
          when true then 0 # SQLITE_OK
          when false then 1 # SQLITE_DENY
          else 2 # SQLITE_IGNORE
          end
        rescue ::Exception => e
          __park(e)
          1
        end
      end
      SQLite3.__check(h, C.sqlite3_set_authorizer(h, cb, nil))
      __keep(cb, :authorizer)
      @authorizer = authorizer #: untyped
      self
    end

    #: (String, untyped) -> self
    def collation(name, comparator)
      h = __handle
      cb = comparator.nil? ? nil : __comparator_closure(name)
      SQLite3.__check(h, C.sqlite3_create_collation(h, C.z(name.to_str), Constants::TextRep::UTF8, nil, cb))
      __keep(cb, [:collation, name.to_str.downcase])
      @collations[name] = comparator
      self
    end

    #: (String) -> Fiddle::Closure::BlockCaller
    def __comparator_closure(name) # its own scope, without the comparator: a replaced closure that lingers must not keep its comparator alive
      Fiddle::Closure::BlockCaller.new(Fiddle::TYPE_INT, [Fiddle::TYPE_VOIDP, Fiddle::TYPE_INT, Fiddle::TYPE_VOIDP, Fiddle::TYPE_INT, Fiddle::TYPE_VOIDP]) do |_, a_len, a, b_len, b|
        a_str = C.text(Fiddle::Pointer.new(a.to_i), a_len)
        b_str = C.text(Fiddle::Pointer.new(b.to_i), b_len)
        internal = Encoding.default_internal
        if internal
          a_str = a_str.encode(internal)
          b_str = b_str.encode(internal)
        end
        Integer(@collations[name].compare(a_str, b_str))
      rescue ::Exception => e
        __park(e)
        0
      end
    end

    #: (String) ?{ (*untyped) -> untyped } -> self
    def define_function(name, &block) = define_function_with_flags(name, Constants::TextRep::UTF8, &block)

    if defined?(::Binding) # rugaroo has no Binding (folds to the else arm) nor Proc#arity: its functions register variadic
      #: (String, Integer) { (*untyped) -> untyped } -> self
      def define_function_with_flags(name, flags, &block)
        raise ArgumentError, "tried to create Proc object without a block" if block.nil?

        __define_function(name, flags, block.arity) { |*args| block.call(*args) }
      end
    else
      #: (String, Integer) { (*untyped) -> untyped } -> self
      def define_function_with_flags(name, flags, &block)
        __define_function(name, flags, -1) { |*args| block.call(*args) }
      end
    end

    #: (String, Integer, Integer) { (*untyped) -> untyped } -> self
    def __define_function(name, flags, arity, &block)
      h = __handle
      cb = Fiddle::Closure::BlockCaller.new(Fiddle::TYPE_VOID, [Fiddle::TYPE_VOIDP, Fiddle::TYPE_INT, Fiddle::TYPE_VOIDP]) do |ctx, argc, argv|
        C.result(ctx.to_i, block.call(*C.values(argc, argv.to_i)))
      rescue ::Exception => e
        __park(e)
        C.sqlite3_result_error(ctx.to_i, "Ruby Exception occurred", -1)
      end
      SQLite3.__check(h, C.sqlite3_create_function(h, C.z(name.to_str), arity, Integer(flags), nil, cb, nil, nil))
      __keep(cb) # the closure holds the block, as the C extension's @functions does
      self
    end

    if defined?(::Binding) # see Database#create_aggregate
      #: (untyped, String) -> self
      private def define_aggregator2(aggregator, name)
        h = __handle
        arity = aggregator.respond_to?(:arity) ? Integer(aggregator.arity) : -1
        raise ArgumentError, "#{self} arity=#{arity} out of range -1..127" if arity < -1 || arity > 127

        instances = {} #: Hash[Integer, Array[untyped]]
        next_id = 0
        instance = lambda do |ctx, create|
          p = C.sqlite3_aggregate_context(ctx, create ? 8 : 0)
          raise NoMemoryError, "SQLite is out-of-memory" if p.null? && create
          next nil if p.null?

          id = p[0, 8].unpack1("q")
          if id.zero?
            next nil unless create

            next_id += 1
            id = next_id
            p[0, 8] = [id].pack("q")
            inst = [nil, nil] #: Array[untyped]
            begin
              inst[0] = aggregator.new
            rescue ::Exception => e
              inst[1] = e
              __park(e)
            end
            instances[id] = inst
          end
          [id, instances[id]]
        end
        step = Fiddle::Closure::BlockCaller.new(Fiddle::TYPE_VOID, [Fiddle::TYPE_VOIDP, Fiddle::TYPE_INT, Fiddle::TYPE_VOIDP]) do |ctx, argc, argv|
          _, inst = instance.call(ctx.to_i, true)
          next unless inst[1].nil?

          begin
            inst[0].step(*C.values(argc, argv.to_i))
          rescue ::Exception => e
            inst[1] = e
            __park(e)
          end
        end
        final = Fiddle::Closure::BlockCaller.new(Fiddle::TYPE_VOID, [Fiddle::TYPE_VOIDP]) do |ctx|
          id, inst = instance.call(ctx.to_i, true)
          if inst[1].nil?
            begin
              C.result(ctx.to_i, inst[0].finalize)
            rescue ::Exception => e
              inst[1] = e
              __park(e)
            end
          end
          C.sqlite3_result_error(ctx.to_i, "Ruby Exception occurred", -1) unless inst[1].nil?
          instances.delete(id)
        end
        SQLite3.__check(h, C.sqlite3_create_function(h, C.zc(name.to_str), arity, Constants::TextRep::UTF8, nil, nil, step, final))
        __keep(step)
        __keep(final)
        (@aggregators ||= []) << [aggregator, instances]
        self
      end
    end

    #: (String, untyped) -> Array[untyped]
    private def exec_batch(sql, results_as_hash)
      h = __handle
      @stmt_deadline = nil
      rows = [] #: Array[untyped]
      cb = Fiddle::Closure::BlockCaller.new(Fiddle::TYPE_INT, [Fiddle::TYPE_VOIDP, Fiddle::TYPE_INT, Fiddle::TYPE_VOIDP, Fiddle::TYPE_VOIDP]) do |_, count, data, columns|
        values = (0...count).map { |i| C.cstr(Fiddle::Pointer.new(data.to_i + (i * Fiddle::SIZEOF_VOIDP)).ptr) }
        if results_as_hash == true
          names = (0...count).map { |i| C.cstr(Fiddle::Pointer.new(columns.to_i + (i * Fiddle::SIZEOF_VOIDP)).ptr) }
          rows << names.zip(values).to_h
        else
          rows << values
        end
        0
      rescue ::Exception => e
        __park(e)
        1
      end
      status = 0
      err = C.out { |out| status = C.sqlite3_exec(h, C.z(sql.to_str), cb, nil, out) }
      __raise_pending
      SQLite3.__check_msg(status, err)
      rows
    end

    #: (String) -> String?
    private def db_filename(db_name) = C.ustr(C.sqlite3_db_filename(__handle, C.zc(db_name.to_str)))

    if C.respond_to?(:sqlite3_load_extension)
      #: (String) -> self
      private def load_extension_internal(file)
        h = __handle
        status = 0
        err = C.out { |out| status = C.sqlite3_load_extension(h, C.z(file.to_str), nil, out) }
        SQLite3.__check_msg(status, err)
        self
      end
    end

    if C.respond_to?(:sqlite3_load_extension) && C.respond_to?(:sqlite3_enable_load_extension)
      #: (untyped) -> self
      def enable_load_extension(onoff)
        h = __handle
        param = case onoff
                when true then 1
                when false then 0
                else Integer(onoff)
                end
        SQLite3.__check(h, C.sqlite3_enable_load_extension(h, param))
        self
      end
    end

    #: (?String?) -> String
    def serialize(db_name = nil)
      h = __handle
      schema = db_name.nil? ? nil : C.zc(db_name.to_str)
      size = Fiddle::Pointer.malloc(8, Fiddle::RUBY_FREE)
      begin
        data = C.sqlite3_serialize(h, schema, size, 0)
        n = size[0, 8].unpack1("q")
        raise SQLite3::Exception, "unable to serialize database \"#{db_name || "main"}\"" if data.null? && n < 0
        raise NoMemoryError, "failed to allocate #{n} bytes for serialization" if data.null? && n > 0

        out = C.bytes(data, n)
        C.sqlite3_free(data) unless data.null?
        out
      ensure
        size.call_free
      end
    end

    if C.respond_to?(:sqlite3_deserialize) && SQLite3.libversion >= 3_054_000 # before 3.54.0, sqlite3_deserialize frees memory an unfinished statement still uses
      #: (String, ?String?) -> self
      def deserialize(data, db_name = nil)
        h = __handle
        data = data.to_str
        schema = db_name.nil? ? nil : C.zc(db_name.to_str)
        n = data.bytesize
        buf = C.sqlite3_malloc64(n + 20) # padding: a malformed image may make sqlite read past its end
        raise NoMemoryError, "failed to allocate #{n + 20} bytes for deserialization" if buf.null?

        buf[0, n + 20] = data.b + ("\0" * 20)
        status = C.sqlite3_deserialize(h, schema, buf, n, n, 1 | 2) # SQLITE_DESERIALIZE_FREEONCLOSE | RESIZEABLE
        SQLite3.__check(h, status)
        self
      end
    end
  end
end
