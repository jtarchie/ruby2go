# rbs_inline: enabled

# An in-memory IO over a byte buffer; writes overwrite at pos and extend, as MRI's.
# @go_type struct { buf []byte; pos int; lineno int }
class StringIO < Object
  #: (?String) -> StringIO
  def self.new(s = "") = %x{ return &StringIO{buf: []byte(string(s))} }

  #: () -> String
  def string = %x{ String(self.buf) }

  #: (String) -> String
  def string=(s)
    %x{
    self.buf, self.pos, self.lineno = []byte(string(s)), 0, 0
    return s}
  end

  #: (untyped) -> Integer
  def write(x) = %x{
    s := string(rbToS(x))
    if gap := self.pos - len(self.buf); gap > 0 {
      self.buf = append(self.buf, make([]byte, gap)...)
    }
    n := copy(self.buf[self.pos:], s)
    self.buf = append(self.buf, s[n:]...)
    self.pos += len(s)
    return Integer(len(s))
  }

  #: (untyped) -> StringIO
  def <<(x)
    write(x)
    self
  end

  #: (*untyped) -> nil
  def print(*args)
    args.each { |a| write(a) }
    nil
  end

  #: (*untyped) -> nil
  def puts(*args)
    if args.empty?
      write("\n")
      return nil
    end
    args.each do |a|
      case a
      when nil then write("\n")
      when Array then a.each { |e| puts(e) }
      else
        s = a.to_s
        write(s.end_with?("\n") ? s : s + "\n")
      end
    end
    nil
  end

  #: () -> String
  def read = %x{
    s := String(self.buf[min(self.pos, len(self.buf)):])
    self.pos = max(self.pos, len(self.buf))
    return s
  }

  # nil at EOF, as MRI's read(n).
  #: (Integer) -> String?
  def __read_1(n) = %x{
    if self.pos >= len(self.buf) && n > 0 {
      return nil
    }
    end := min(self.pos+int(n), len(self.buf))
    s := String(self.buf[self.pos:end])
    self.pos = end
    return &s
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
    self.lineno++
    return &s
  }

  #: () -> String?
  def getc = %x{
    if self.pos >= len(self.buf) {
      return nil
    }
    _, n := utf8.DecodeRune(self.buf[self.pos:])
    s := String(self.buf[self.pos : self.pos+n])
    self.pos += n
    return &s
  }

  #: () { (String) -> void } -> void
  def each_line
    while (line = gets)
      yield line
    end
  end

  #: () -> Array[String]
  def readlines
    out = [] #: Array[String]
    while (line = gets)
      out << line
    end
    out
  end

  #: () -> bool
  def eof? = %x{ Boolean(self.pos >= len(self.buf)) }

  #: () -> Integer
  def rewind = %x{
    self.pos, self.lineno = 0, 0
    return 0
  }

  #: () -> Integer
  def pos = %x{ Integer(self.pos) }

  #: (Integer) -> Integer
  def pos=(n)
    %x{
    if n < 0 {
      panic(NewErrno_EINVAL(Ref(String("Invalid argument"))))
    }
    self.pos = int(n)
    return n}
  end

  #: () -> Integer
  def lineno = %x{ Integer(self.lineno) }

  #: () -> Integer
  def size = %x{ Integer(len(self.buf)) }

  #: () -> Integer
  def length = size
end

