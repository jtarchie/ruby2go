# rbs_inline: enabled

# ext/sqlite3/sqlite3.c over Fiddle: module functions, version constants, Blob, Constants::Open.

require "sqlite3/native/ffi"

module SQLite3
  if defined?(::Binding) # rugaroo widens a String subclass to String in a mixed literal (its decision 150), so there Blob is a marker class
    class Blob < String; end
  else
    class Blob
      #: (String) -> void
      def initialize(str)
        @str = str
      end

      #: () -> String
      def to_s = @str

      #: () -> String
      def to_str = @str

      #: () -> Integer
      def bytesize = @str.bytesize

      #: () -> Integer
      def length = @str.length

      #: () -> Integer
      def size = @str.length

      #: () -> Array[Integer]
      def bytes = @str.bytes

      #: (untyped) -> bool
      def ==(other)
        case other
        when Blob then other.to_s == @str
        when String then other == @str
        else false
        end
      end

      #: (untyped) -> bool
      def eql?(other) = self == other

      #: () -> Integer
      def hash = @str.hash

      #: () -> String
      def inspect = @str.inspect
    end
  end

  module Constants
    module Open
      READONLY = 0x00000001
      READWRITE = 0x00000002
      CREATE = 0x00000004
      DELETEONCLOSE = 0x00000008
      EXCLUSIVE = 0x00000010
      AUTOPROXY = 0x00000020
      URI = 0x00000040
      MEMORY = 0x00000080
      MAIN_DB = 0x00000100
      TEMP_DB = 0x00000200
      TRANSIENT_DB = 0x00000400
      MAIN_JOURNAL = 0x00000800
      TEMP_JOURNAL = 0x00001000
      SUBJOURNAL = 0x00002000
      MASTER_JOURNAL = 0x00004000
      SUPER_JOURNAL = 0x00004000
      NOMUTEX = 0x00008000
      FULLMUTEX = 0x00010000
      SHAREDCACHE = 0x00020000
      PRIVATECACHE = 0x00040000
      WAL = 0x00080000
    end
  end

  #: () -> bool
  def self.sqlcipher? = false

  #: () -> Integer
  def self.libversion = C.sqlite3_libversion_number

  #: () -> Integer
  def self.threadsafe = C.sqlite3_threadsafe

  #: (Integer, ?untyped) -> Hash[Symbol, Integer]
  def self.status(parameter, reset_flag = false)
    cur = Fiddle::Pointer.malloc(Fiddle::SIZEOF_INT, Fiddle::RUBY_FREE)
    high = Fiddle::Pointer.malloc(Fiddle::SIZEOF_INT, Fiddle::RUBY_FREE)
    begin
      C.sqlite3_status(parameter, cur, high, reset_flag ? 1 : 0)
      { current: cur[0, Fiddle::SIZEOF_INT].unpack1("l"), highwater: high[0, Fiddle::SIZEOF_INT].unpack1("l") }
    ensure
      cur.call_free
      high.call_free
    end
  end

  SQLITE_LOADED_VERSION = C.sqlite3_libversion.to_s.freeze #: String
  SQLITE_VERSION = SQLITE_LOADED_VERSION # no compile-time header here: the library loaded is the one built against
  SQLITE_VERSION_NUMBER = C.sqlite3_libversion_number #: Integer
  SQLITE_PACKAGED_LIBRARIES = false #: bool
  SQLITE_PRECOMPILED_LIBRARIES = false #: bool
end
