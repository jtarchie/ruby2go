# rbs_inline: enabled

require "sqlite3/constants"
require "sqlite3/errors"
require "sqlite3/pragmas"
require "sqlite3/statement"
require "sqlite3/value"
require "sqlite3/fork_safety"

module SQLite3
  # A single connection to a SQLite3 database: prepare and execute SQL, define functions, aggregates and collations.
  class Database
    attr_reader :collations #: Hash[String, untyped]

    include Pragmas

    #: (*untyped) ?{ (Database) -> untyped } -> untyped
    def self.open(*args)
      database = new(*args)

      if block_given?
        begin
          yield database
        ensure
          database.close
        end
      else
        database
      end
    end

    #: (String) -> String
    def self.quote(string) = string.gsub("'", "''")

    attr_accessor :results_as_hash #: untyped

    #: (untyped, ?Hash[Symbol, untyped], ?String?) ?{ (Database) -> untyped } -> void
    def initialize(file, options = {}, zvfs = nil)
      mode = Constants::Open::READWRITE | Constants::Open::CREATE

      file = file.to_path if file.respond_to? :to_path
      if file.encoding == ::Encoding::UTF_16LE || file.encoding == ::Encoding::UTF_16BE || options[:utf16]
        open16 file
      else
        mode = Constants::Open::READONLY if options[:readonly]

        if options[:readwrite]
          raise "conflicting options: readonly and readwrite" if options[:readonly]

          mode = Constants::Open::READWRITE
        end

        if options[:flags]
          if options[:readonly] || options[:readwrite]
            raise "conflicting options: flags with readonly and/or readwrite"
          end

          mode = options[:flags]
        end

        open_v2 file.encode("utf-8"), mode, zvfs

        disable_quirk_mode if options[:strict]
      end

      @progress_handler = nil
      @collations = {}
      @functions = [] #: Array[untyped]
      @results_as_hash = options[:results_as_hash]
      @readonly = mode & Constants::Open::READONLY != 0
      @default_transaction_mode = options[:default_transaction_mode] || :deferred

      initialize_extensions(options[:extensions])

      ForkSafety.track(self)

      if block_given?
        begin
          yield self
        ensure
          close
        end
      end
    end

    if defined?(::Binding) # rb2go cannot store a block it never calls; there, pass a callable to authorizer=
      #: () { (Integer, String?, String?, String?, String?) -> untyped } -> untyped
      def authorizer(&block)
        self.authorizer = block
      end
    end

    #: (String) { (Statement) -> untyped } -> untyped
    def prepare(sql)
      stmt = __prepare_1(sql)
      return stmt unless block_given?

      begin
        yield stmt
      ensure
        stmt.close unless stmt.closed?
      end
    end

    #: (String) -> Statement
    def __prepare_1(sql) = SQLite3::Statement.new(self, sql) # rb2go's overload for a blockless prepare: typed, where the block form returns the block's value

    #: (?String) -> String?
    def filename(db_name = "main") = db_filename(db_name)

    #: (String, ?untyped) { (untyped) -> void } -> untyped
    def execute(sql, bind_vars = [])
      return __execute_2(sql, bind_vars) unless block_given?

      prepare(sql) do |stmt|
        stmt.bind_params(bind_vars)
        build_result_set(stmt).each { |row| yield row }
      end
    end

    #: (String) -> Array[untyped]
    def __execute_1(sql) = __execute_2(sql, [])

    #: (String, untyped) -> Array[untyped]
    def __execute_2(sql, bind_vars)
      stmt = __prepare_1(sql)
      begin
        stmt.bind_params(bind_vars)
        build_result_set(stmt).to_a.freeze
      ensure
        stmt.close unless stmt.closed?
      end
    end

    #: (String, *untyped) { (untyped) -> void } -> untyped
    def execute2(sql, *bind_vars)
      return __execute2_rows(sql, bind_vars) unless block_given?

      prepare(sql) do |stmt|
        result = stmt.execute(*bind_vars)
        yield stmt.columns
        result.each { |row| yield row }
      end
    end

    #: (String) -> Array[untyped]
    def __execute2_1(sql) = __execute2_rows(sql, [])

    #: (String, untyped) -> Array[untyped]
    def __execute2_2(sql, a) = __execute2_rows(sql, [a])

    #: (String, untyped, untyped) -> Array[untyped]
    def __execute2_3(sql, a, b) = __execute2_rows(sql, [a, b])

    #: (String, Array[untyped]) -> Array[untyped]
    def __execute2_rows(sql, bind_vars) # rb2go cannot return from inside prepare's block, as the gem does
      stmt = __prepare_1(sql)
      begin
        result = stmt.execute(*bind_vars)
        rows = [stmt.columns] #: Array[untyped]
        result.each { |row| rows << row }
        rows
      ensure
        stmt.close unless stmt.closed?
      end
    end

    #: (String, ?untyped) -> untyped
    def execute_batch(sql, bind_vars = [])
      sql = sql.strip
      result = nil
      until sql.empty?
        prepare(sql) do |stmt|
          unless stmt.closed?
            if bind_vars.length == stmt.bind_parameter_count
              stmt.bind_params(bind_vars)
            end
            result = stmt.step
          end
          sql = stmt.remainder.strip
        end
      end

      result
    end

    #: (String) ?{ (untyped) -> untyped } -> untyped
    def execute_batch2(sql, &block)
      if block
        result = exec_batch(sql, @results_as_hash)
        result.map do |val|
          yield val
        end
      else
        exec_batch(sql, @results_as_hash)
      end
    end

    #: (String, ?untyped) { (ResultSet) -> untyped } -> untyped
    def query(sql, bind_vars = [])
      result = __query_2(sql, bind_vars)
      if block_given?
        begin
          yield result
        ensure
          result.close
        end
      else
        result
      end
    end

    #: (String) -> ResultSet
    def __query_1(sql) = __query_2(sql, [])

    #: (String, untyped) -> ResultSet
    def __query_2(sql, bind_vars) = __prepare_1(sql).execute(bind_vars)

    #: (String, *untyped) -> untyped
    def get_first_row(sql, *bind_vars)
      raise ArgumentError, "wrong number of arguments (given #{bind_vars.length + 1}, expected 1..2)" if bind_vars.length > 1 # execute(sql, *bind_vars), as the gem forwards it

      __execute_2(sql, bind_vars.empty? ? [] : bind_vars[0]).first
    end

    #: (String, *untyped) -> untyped
    def get_first_value(sql, *bind_vars)
      rs = __query_2(sql, bind_vars)
      begin
        row = rs.next
        return nil unless row

        @results_as_hash ? row[rs.columns[0]] : row[0]
      ensure
        rs.close
      end
    end

    alias_method :busy_timeout, :busy_timeout=

    #: (String, Integer, ?Integer) { (*untyped) -> void } -> self
    def create_function(name, arity, text_rep = Constants::TextRep::UTF8, &block)
      define_function_with_flags(name, text_rep) do |*args|
        fp = FunctionProxy.new
        block.call(fp, *args)
        fp.result
      end
      self
    end

    if defined?(::Binding) # aggregates are Class.new/define_method metaprogramming: rb2go has no Binding, so it folds this out
      #: (String, Integer, ?untyped, ?untyped, ?Integer) ?{ () -> void } -> self
      def create_aggregate(name, arity, step = nil, finalize = nil, text_rep = Constants::TextRep::ANY, &block)
        proxy = Class.new do
          def self.step(&block)
            define_method(:step_with_ctx, &block)
          end

          def self.finalize(&block)
            define_method(:finalize_with_ctx, &block)
          end
        end

        if block
          proxy.instance_eval(&block)
        else
          proxy.class_eval do
            define_method(:step_with_ctx, step)
            define_method(:finalize_with_ctx, finalize)
          end
        end

        proxy.class_eval do
          @name = name
          @arity = arity

          def self.name
            @name
          end

          def self.arity
            @arity
          end

          def initialize
            @ctx = FunctionProxy.new
          end

          def step(*args)
            step_with_ctx(@ctx, *args)
          end

          def finalize
            finalize_with_ctx(@ctx)
            @ctx.result
          end
        end
        define_aggregator2(proxy, name)
      end

      #: (untyped) -> self
      def create_aggregate_handler(handler)
        # passes the FunctionProxy ctx first to step and finalize, as handlers written for the gem's 1.x API expect
        proxy = Class.new(handler) do
          def initialize
            super
            @fp = FunctionProxy.new
          end

          def step(*args)
            super(@fp, *args)
          end

          def finalize
            super(@fp)
            @fp.result
          end
        end
        define_aggregator2(proxy, proxy.name)
        self
      end

      class AggregatorFactory # :nodoc:
        #: (untyped) -> void
        def initialize(template)
          @template = template
        end

        #: () -> untyped
        def new = @template.clone

        #: () -> Integer
        def arity = @template.method(:step).arity
      end
      private_constant :AggregatorFactory

      #: (String, untyped) -> self
      def define_aggregator(name, aggregator)
        define_aggregator2(AggregatorFactory.new(aggregator), name)
        self
      end
    end

    #: (?untyped) ?{ (Database) -> untyped } -> untyped
    def transaction(mode = nil)
      mode = @default_transaction_mode if mode.nil?
      execute "begin #{mode} transaction"

      if block_given?
        abort = false
        begin
          yield self
        rescue
          abort = true
          raise
        ensure
          abort and rollback or commit
        end
      else
        true
      end
    end

    #: () -> bool
    def commit
      execute "commit transaction"
      true
    end

    #: () -> bool
    def rollback
      execute "rollback transaction"
      true
    end

    #: () -> bool
    def readonly? = @readonly

    #: (Integer) -> untyped
    def busy_handler_timeout=(milliseconds) # sleeps between retries while releasing the GVL, unlike busy_timeout's sqlite-side sleep
      timeout_seconds = milliseconds.fdiv(1000)

      busy_handler do |count|
        now = Process.clock_gettime(Process::CLOCK_MONOTONIC)
        if count.zero?
          @timeout_deadline = now + timeout_seconds
        elsif now > @timeout_deadline
          next false
        else
          sleep(0.001)
        end
      end
    end

    if defined?(::Binding) # rb2go folds out the load_extension_internal it would call (see native/database.rb)
      #: (untyped) -> untyped
      def load_extension(extension_specifier)
        if extension_specifier.respond_to?(:to_path)
          extension_specifier = extension_specifier.to_path
        elsif !extension_specifier.is_a?(String)
          raise TypeError, "extension_specifier #{extension_specifier.inspect} is not a String or a valid extension specifier object"
        end
        load_extension_internal(extension_specifier)
      end

      #: (untyped) -> void
      def initialize_extensions(extensions) # :nodoc:
        return if extensions.nil?
        raise TypeError, "extensions must be an Array" unless extensions.is_a?(Array)
        return if extensions.empty?

        begin
          enable_load_extension(true)
          extensions.each do |extension|
            load_extension(extension)
          end
        ensure
          enable_load_extension(false)
        end
      end
    else
      #: (untyped) -> void
      def initialize_extensions(extensions) # :nodoc:
        raise NotImplementedError, "extensions are not supported under rb2go" unless extensions.nil?
      end
    end

    # The value a create_function or create_aggregate block fills: `fp.result = x`, plus a per-call context Hash.
    class FunctionProxy
      attr_accessor :result #: untyped

      #: () -> void
      def initialize
        @result = nil
        @context = {} #: Hash[untyped, untyped]
      end

      #: (untyped) -> untyped
      def [](key) = @context[key]

      #: (untyped, untyped) -> untyped
      def []=(key, value)
        @context[key] = value
      end
    end

    #: (Statement) -> ResultSet
    def build_result_set(stmt) = results_as_hash ? HashResultSet.new(self, stmt) : ResultSet.new(self, stmt)
  end
end
