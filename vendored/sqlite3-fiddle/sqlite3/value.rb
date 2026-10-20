# rbs_inline: enabled

require "sqlite3/constants"

module SQLite3
  class Value # the gem keeps this 1.x-era class, though nothing builds one since its driver layer went away
    attr_reader :handle #: untyped

    #: (untyped, untyped) -> void
    def initialize(db, handle)
      @driver = db.driver
      @handle = handle
    end

    #: () -> bool
    def null? = type == :null

    #: () -> untyped
    def to_blob = @driver.value_blob(@handle)

    #: (?bool) -> untyped
    def length(utf16 = false) = utf16 ? @driver.value_bytes16(@handle) : @driver.value_bytes(@handle)

    #: () -> untyped
    def to_f = @driver.value_double(@handle)

    #: () -> untyped
    def to_i = @driver.value_int(@handle)

    #: () -> untyped
    def to_int64 = @driver.value_int64(@handle)

    #: (?bool) -> untyped
    def to_s(utf16 = false) = @driver.value_text(@handle, utf16)

    #: () -> Symbol?
    def type
      case @driver.value_type(@handle)
      when Constants::ColumnType::INTEGER then :int
      when Constants::ColumnType::FLOAT then :float
      when Constants::ColumnType::TEXT then :text
      when Constants::ColumnType::BLOB then :blob
      when Constants::ColumnType::NULL then :null
      end
    end
  end
end
