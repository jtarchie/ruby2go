# rbs_inline: enabled

# Zlib: checksums, Deflate/Inflate, gzip, and GzipWriter/GzipReader. Always defined.
module Zlib
  DEFAULT_COMPRESSION = -1 #: Integer
  NO_COMPRESSION = 0 #: Integer
  BEST_SPEED = 1 #: Integer
  BEST_COMPRESSION = 9 #: Integer

  #: (?String, ?Integer) -> Integer
  def self.crc32(str = "", crc = 0) = %x{ Integer(crc32.Update(uint32(crc), crc32.IEEETable, []byte(str))) }

  #: (?String, ?Integer) -> Integer
  def self.adler32(str = "", adler = 1) = %x{
    // hash/adler32 cannot resume from a value; this is its update loop
    s1, s2 := uint32(adler)&0xffff, uint32(adler)>>16
    for i := range len(str) {
      s1 = (s1 + uint32(str[i])) % 65521
      s2 = (s2 + s1) % 65521
    }
    return Integer(s2<<16 | s1)
  }

  class Error < StandardError; end

  class DataError < Error; end

  # A module, not MRI's class: GzipWriter/GzipReader dispatch dynamically instead of subclassing it.
  module GzipFile
    class Error < Zlib::Error; end
  end

  class Deflate < Object
    #: (String, ?Integer) -> String
    def self.deflate(str, level = Zlib::DEFAULT_COMPRESSION) = %x{
      var buf bytes.Buffer
      zw, err := zlib.NewWriterLevel(&buf, int(level))
      if err != nil {
        panic(NewArgumentError(Ref(String(err.Error()))))
      }
      _, _ = zw.Write([]byte(str))
      if err := zw.Close(); err != nil {
        panic(NewZlib_DataError(Ref(String(err.Error()))))
      }
      return String(buf.Bytes())
    }
  end

  class Inflate < Object
    #: (String) -> String
    def self.inflate(str) = %x{
      zr, err := zlib.NewReader(bytes.NewReader([]byte(str)))
      if err != nil {
        panic(NewZlib_DataError(Ref(String(err.Error()))))
      }
      b, err := io.ReadAll(zr)
      if err != nil {
        panic(NewZlib_DataError(Ref(String(err.Error()))))
      }
      return String(b)
    }
  end

  #: (String, ?Integer) -> String
  def self.deflate(str, level = Zlib::DEFAULT_COMPRESSION) = Deflate.deflate(str, level)

  #: (String) -> String
  def self.inflate(str) = Inflate.inflate(str)

  #: (String, ?Integer) -> String
  def self.gzip(str, level = Zlib::DEFAULT_COMPRESSION) = %x{
    var buf bytes.Buffer
    zw, err := gzip.NewWriterLevel(&buf, int(level))
    if err != nil {
      panic(NewArgumentError(Ref(String(err.Error()))))
    }
    _, _ = zw.Write([]byte(str))
    if err := zw.Close(); err != nil {
      panic(NewZlib_GzipFile_Error(Ref(String(err.Error()))))
    }
    return String(buf.Bytes())
  }

  #: (String) -> String
  def self.gunzip(str) = %x{
    zr, err := gzip.NewReader(bytes.NewReader([]byte(str)))
    if err != nil {
      panic(NewZlib_GzipFile_Error(Ref(String(err.Error()))))
    }
    b, err := io.ReadAll(zr)
    if err != nil {
      panic(NewZlib_GzipFile_Error(Ref(String(err.Error()))))
    }
    return String(b)
  }

  # @go_type struct { io any; gzw *gzip.Writer; buf *bytes.Buffer }
  class GzipWriter < Object
    include IOWritable

    #: (untyped, ?Integer) -> GzipWriter
    def self.new(io, level = Zlib::DEFAULT_COMPRESSION) = %x{
      buf := &bytes.Buffer{}
      zw, err := gzip.NewWriterLevel(buf, int(level))
      if err != nil {
        panic(NewArgumentError(Ref(String(err.Error()))))
      }
      return &Zlib_GzipWriter{io: io, gzw: zw, buf: buf}
    }

    #: () -> untyped
    def __io = %x{ return self.io }

    #: (untyped) -> Integer
    def write(x) = %x{
      s := string(rbToS(x))
      n, err := self.gzw.Write([]byte(s))
      if err != nil {
        panic(NewZlib_GzipFile_Error(Ref(String(err.Error()))))
      }
      return Integer(n)
    }

    #: (untyped) -> GzipWriter
    def <<(x)
      write(x)
      self
    end

    #: () -> GzipWriter
    def flush = %x{
      if err := self.gzw.Flush(); err != nil {
        panic(NewZlib_GzipFile_Error(Ref(String(err.Error()))))
      }
      return self
    }

    #: () -> String
    def __finish_bytes = %x{
      if err := self.gzw.Close(); err != nil {
        panic(NewZlib_GzipFile_Error(Ref(String(err.Error()))))
      }
      return String(self.buf.Bytes())
    }

    #: () -> untyped
    # @dynamic
    def finish
      io = __io
      io.write(__finish_bytes) # leaves io open, so e.g. StringIO#string still reads afterwards
      io
    end

    #: () -> untyped
    # @dynamic
    def close
      io = finish
      io.close
      io
    end
  end

  # @go_type struct { io any; buf []byte; pos int }
  class GzipReader < Object
    include IOReadable

    #: (untyped) -> GzipReader
    # @dynamic
    def self.new(io)
      __from_raw(io, io.read)
    end

    #: (untyped, String) -> GzipReader
    def self.__from_raw(src, raw) = %x{
      zr, err := gzip.NewReader(bytes.NewReader([]byte(raw)))
      if err != nil {
        panic(NewZlib_GzipFile_Error(Ref(String(err.Error()))))
      }
      b, err := io.ReadAll(zr)
      if err != nil {
        panic(NewZlib_GzipFile_Error(Ref(String(err.Error()))))
      }
      return &Zlib_GzipReader{io: src, buf: b}
    }

    #: () -> untyped
    def __io = %x{ return self.io }

    #: () -> String
    def read = %x{
      s := String(self.buf[min(self.pos, len(self.buf)):])
      self.pos = max(self.pos, len(self.buf))
      return s
    }

    #: () -> String?
    def gets = %x{
      if self.pos >= len(self.buf) {
        return nil
      }
      rest := self.buf[self.pos:]
      n := len(rest)
      if i := bytes.IndexByte(rest, '\\n'); i >= 0 {
        n = i + 1
      }
      s := String(rest[:n])
      self.pos += n
      return &s
    }

    #: () -> bool
    def eof? = %x{ Boolean(self.pos >= len(self.buf)) }

    #: () -> Integer
    def rewind = %x{
      self.pos = 0
      return 0
    }

    #: () -> untyped
    # @dynamic
    def close
      io = __io
      io.close
      io
    end
  end
end
