# rbs_inline: enabled

require "sqlite3/constants"
require "sqlite3/errors"

module SQLite3
  # The result of executing a select query: rows as Arrays, stepped through one at a time.
  class ResultSet
    include Enumerable #[untyped]

    #: (Database, Statement) -> void
    def initialize(db, stmt)
      @db = db
      @stmt = stmt
    end

    #: (*untyped) -> void
    def reset(*bind_params)
      @stmt.reset!
      @stmt.bind_params(*bind_params)
    end

    #: () -> bool
    def eof? = @stmt.done?

    #: () -> untyped
    def next = @stmt.step

    #: () ?{ (untyped) -> void } -> untyped
    def each
      return __enum(:each) unless block_given?

      while (node = self.next)
        yield node
      end
      self
    end

    #: () ?{ (Hash[String, untyped]) -> void } -> untyped
    def each_hash
      return __enum(:each_hash) unless block_given?

      while (node = next_hash)
        yield node
      end
      self
    end

    if defined?(::Binding) # rugaroo's enum_for cannot name a method of a class with subclasses; iterate with a block there
      #: (Symbol) -> untyped
      def __enum(name) = enum_for(name)
    else
      #: (Symbol) -> untyped
      def __enum(name) = raise(NotImplementedError, "#{name} without a block")
    end

    #: () -> untyped
    def close = @stmt.close

    #: () -> bool
    def closed? = @stmt.closed?

    #: () -> Array[String?]
    def types = @stmt.types

    #: () -> Array[String]
    def columns = @stmt.columns

    #: () -> Hash[String, untyped]?
    def next_hash
      row = @stmt.step
      return nil if @stmt.done?

      @stmt.columns.zip(row || []).to_h
    end
  end

  class HashResultSet < ResultSet # :nodoc:
    #: () -> untyped
    def next = next_hash
  end
end
