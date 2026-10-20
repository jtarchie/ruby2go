# rbs_inline: enabled

module SQLite3
  module Constants
    module TextRep
      UTF8 = 1
      UTF16LE = 2
      UTF16BE = 3
      UTF16 = 4
      ANY = 5
      DETERMINISTIC = 0x800
    end

    module ColumnType
      INTEGER = 1
      FLOAT = 2
      TEXT = 3
      BLOB = 4
      NULL = 5
    end

    module ErrorCode
      OK = 0
      ERROR = 1
      INTERNAL = 2
      PERM = 3
      ABORT = 4
      BUSY = 5
      LOCKED = 6
      NOMEM = 7
      READONLY = 8
      INTERRUPT = 9
      IOERR = 10
      CORRUPT = 11
      NOTFOUND = 12
      FULL = 13
      CANTOPEN = 14
      PROTOCOL = 15
      EMPTY = 16
      SCHEMA = 17
      TOOBIG = 18
      CONSTRAINT = 19
      MISMATCH = 20
      MISUSE = 21
      NOLFS = 22
      AUTH = 23
      FORMAT = 24
      RANGE = 25
      NOTADB = 26
      NOTICE = 27
      WARNING = 28
      ROW = 100
      DONE = 101
    end

    module Status
      MEMORY_USED = 0

      PAGECACHE_USED = 1

      PAGECACHE_OVERFLOW = 2

      SCRATCH_USED = 3

      SCRATCH_OVERFLOW = 4

      MALLOC_SIZE = 5

      PARSER_STACK = 6

      PAGECACHE_SIZE = 7

      SCRATCH_SIZE = 8

      MALLOC_COUNT = 9
    end

    module Optimize
      DEBUG = 0x00001

      ANALYZE_TABLES = 0x00002

      LIMIT_ANALYZE = 0x00010

      CHECK_ALL_TABLES = 0x10000

      DEFAULT = ANALYZE_TABLES | LIMIT_ANALYZE
    end
  end
end
