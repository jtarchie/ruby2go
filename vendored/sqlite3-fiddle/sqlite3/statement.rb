# rbs_inline: enabled

require "sqlite3/errors"
require "sqlite3/resultset"

class String
  #: () -> SQLite3::Blob
  def to_blob = SQLite3::Blob.new(self)
end

module SQLite3
  # A statement represents a prepared-but-unexecuted SQL query.
  class Statement
    include Enumerable #[Array[untyped]]

    attr_reader :remainder #: String

    #: (Database, String) -> void
    def initialize(db, sql)
      raise ArgumentError, "prepare called on a closed database" if db.closed?

      sql = sql.encode(Encoding::UTF_8) if sql && sql.encoding != Encoding::UTF_8

      @connection = db #: Database
      @columns = nil #: Array[String]?
      @types = nil #: Array[String?]?
      @remainder = prepare(db, sql)
    end

    #: (*untyped) -> void
    def bind_params(*bind_vars)
      index = 1
      bind_vars.flatten.each do |var|
        if Hash === var
          var.each { |key, val| bind_param key, val }
        else
          bind_param index, var
          index += 1
        end
      end
    end

    #: (*untyped) ?{ (ResultSet) -> void } -> ResultSet
    def execute(*bind_vars)
      reset! if active? || done?

      bind_params(*bind_vars) unless bind_vars.empty?
      results = @connection.build_result_set self

      step if column_count == 0

      yield results if block_given?
      results
    end

    #: (*untyped) ?{ (Array[untyped]) -> void } -> untyped
    def execute!(*bind_vars, &block)
      execute(*bind_vars)
      block ? each(&block) : to_a
    end

    #: () -> bool
    def active? = !done?

    #: () -> Array[String]
    def columns
      get_metadata unless @columns
      @columns || []
    end

    #: () { (Array[untyped]) -> void } -> self
    def each
      loop do
        val = step
        break if done?
        yield val || []
      end
      self
    end

    #: () -> Array[String?]
    def types
      must_be_open!
      get_metadata unless @types
      @types || []
    end

    #: () -> void
    def must_be_open! # :nodoc:
      raise SQLite3::Exception, "cannot use a closed statement" if closed?
    end

    #: (?Symbol?) -> untyped
    def stat(key = nil) = key ? stat_for(key) : stats_as_hash

    private

    #: () -> void
    def get_metadata
      @columns = Array.new(column_count) { |column| column_name(column).to_s }.freeze
      @types = Array.new(column_count) { |column| column_decltype_downcased(column) }.freeze
    end
  end
end
