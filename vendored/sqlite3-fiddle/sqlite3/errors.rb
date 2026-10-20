# rbs_inline: enabled

require "sqlite3/constants"

module SQLite3
  class Exception < ::StandardError
    attr_reader :code #: Integer?
    attr_reader :sql #: String?
    attr_reader :sql_offset #: Integer?

    #: () -> String
    def message
      [super, sql_error].compact.join(":\n")
    end

    #: () -> String?
    private def sql_error
      sql = @sql
      return nil unless sql

      offset = @sql_offset.to_i
      return sql.chomp unless offset >= 0

      sql.lines.flat_map do |line|
        if offset >= 0 && line.length > offset
          blanks = " " * offset
          offset = -1
          [line.chomp, blanks + "^"]
        else
          offset -= line.length
          [line.chomp]
        end
      end.join("\n")
    end
  end

  class SQLException < Exception; end

  class InternalException < Exception; end

  class PermissionException < Exception; end

  class AbortException < Exception; end

  class BusyException < Exception; end

  class LockedException < Exception; end

  class MemoryException < Exception; end

  class ReadOnlyException < Exception; end

  class InterruptException < Exception; end

  class IOException < Exception; end

  class CorruptException < Exception; end

  class NotFoundException < Exception; end

  class FullException < Exception; end

  class CantOpenException < Exception; end

  class ProtocolException < Exception; end

  class EmptyException < Exception; end

  class SchemaChangedException < Exception; end

  class TooBigException < Exception; end

  class ConstraintException < Exception; end

  class MismatchException < Exception; end

  class MisuseException < Exception; end

  class UnsupportedException < Exception; end

  class AuthorizationException < Exception; end

  class FormatException < Exception; end

  class RangeException < Exception; end

  class NotADatabaseException < Exception; end
end
