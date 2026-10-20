# rbs_inline: enabled

# ext/sqlite3/backup.c over Fiddle.

require "sqlite3/native/ffi"
require "sqlite3/native/exception"

module SQLite3
  class Backup
    #: (Database, String, Database, String) -> void
    def initialize(dstdb, dstname, srcdb, srcname)
      raise ArgumentError, "cannot backup from a closed database" if srcdb.closed?
      raise ArgumentError, "cannot backup to a closed database" if dstdb.closed?

      dst = dstdb.__handle
      p = C.sqlite3_backup_init(dst, C.z(dstname.to_str), srcdb.__handle, C.z(srcname.to_str))
      SQLite3.__check(dst, C.sqlite3_errcode(dst)) if p.null?
      @p = p.to_i #: Integer?
    end

    #: () -> Integer
    def __p
      p = @p
      raise SQLite3::Exception, "cannot use a closed backup" if p.nil? || p.zero?

      p
    end

    #: (Integer) -> Integer
    def step(n_page) = C.sqlite3_backup_step(__p, Integer(n_page))

    #: () -> nil
    def finish
      C.sqlite3_backup_finish(__p)
      @p = nil
    end

    #: () -> Integer
    def remaining = C.sqlite3_backup_remaining(__p)

    #: () -> Integer
    def pagecount = C.sqlite3_backup_pagecount(__p)
  end
end
