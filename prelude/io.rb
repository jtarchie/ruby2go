# rbs_inline: enabled

# Shared by IO and File, as MRI's IO::generic_writable/readable; includers supply write and gets.
module IOWritable
  #: (untyped) -> Integer
  def write(x) = raise(NotImplementedError)

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

  #: (String, *untyped) -> nil
  def printf(fmt, *args)
    write(format(fmt, *args))
    nil
  end
end

module IOReadable
  #: () -> String?
  def gets = raise(NotImplementedError)

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
end

# @go_type struct { fd int }
class IO < Object
  include IOWritable
  include IOReadable

  SEEK_SET = 0 #: Integer
  SEEK_CUR = 1 #: Integer
  SEEK_END = 2 #: Integer

  #: (Integer) -> IO
  def self.__new(fd) = %x{ return &IO{fd: int(fd)} }

  #: () -> Array[String]
  def self.__argv = %x{
    a := make(Array[String], 0, len(os.Args)-1)
    for _, s := range os.Args[1:] {
      a = append(a, String(s))
    }
    return &a
  }

  #: (untyped) -> Integer
  def write(x) = %x{
    s := string(rbToS(x))
    switch self.fd {
    case 1:
      rbWrite(s)
    case 2:
      rbFlushIfTTY()
      _, _ = os.Stderr.WriteString(s)
    default:
      panic(NewIOError(Ref[String]("not opened for writing")))
    }
    return Integer(len(s))
  }

  #: (untyped) -> IO
  def <<(x)
    write(x)
    self
  end

  #: () -> IO
  def flush = %x{
    if self.fd == 1 {
      rbFlush()
    }
    return self
  }

  #: () -> Integer
  def fileno = %x{ Integer(self.fd) }

  #: () -> bool
  def tty? = %x{
    fi, err := []*os.File{os.Stdin, os.Stdout, os.Stderr}[self.fd].Stat()
    return Boolean(err == nil && fi.Mode()&os.ModeCharDevice != 0)
  }

  #: () -> String?
  def gets = %x{
    self.rbReadable()
    line, err := rbStdin.ReadString('\\n')
    if line == "" && err != nil {
      return nil
    }
    s := String(line)
    return &s
  }

  #: () -> String
  def read = %x{
    self.rbReadable()
    b, _ := io.ReadAll(rbStdin)
    return String(b)
  }

  #: () -> bool
  def eof? = %x{
    self.rbReadable()
    _, err := rbStdin.Peek(1)
    return Boolean(err != nil)
  }

  #: () -> String
  def inspect = %x{ String([]string{"#<IO:<STDIN>>", "#<IO:<STDOUT>>", "#<IO:<STDERR>>"}[self.fd]) }
end

STDIN = IO.__new(0) #: IO
STDOUT = IO.__new(1) #: IO
STDERR = IO.__new(2) #: IO

ARGV = IO.__argv #: Array[String]

# @go_type struct{}
class ENVClass < Object
  #: () -> ENVClass
  def self.new = %x{ return &ENVClass{} }

  #: (String) -> String?
  def [](name) = %x{
    v, ok := os.LookupEnv(string(name))
    if !ok {
      return nil
    }
    s := String(v)
    return &s
  }

  #: (String, String?) -> String?
  def []=(name, value)
    %x{
    if value == nil {
      _ = os.Unsetenv(string(name))
    } else {
      _ = os.Setenv(string(name), string(*value))
    }
    return value
    }
  end

  #: (String, ?String?) -> String
  def fetch(name, default = nil)
    v = self[name]
    return v if v
    return default if default

    raise KeyError, "key not found: #{name.inspect}"
  end

  #: (String) -> bool
  def key?(name) = !self[name].nil?

  #: (String) -> bool
  def include?(name) = key?(name)

  #: (String) -> String?
  def delete(name)
    v = self[name]
    self[name] = nil
    v
  end

  #: () -> Hash[String, String]
  def to_h = %x{
    h := NewHash[String, String]()
    for _, kv := range os.Environ() {
      k, v, _ := strings.Cut(kv, "=")
      Hash_Op_idxSet(h, String(k), String(v))
    }
    return h
  }

  #: () -> Array[String]
  def keys = to_h.keys

  #: () { (String, String) -> void } -> void
  def each
    to_h.each { |k, v| yield k, v }
  end

  #: () -> String
  def inspect = to_h.inspect
end

ENV = ENVClass.new #: ENVClass

module Kernel
  private

  #: (*untyped) -> nil
  def warn(*msgs)
    msgs.each { |m| STDERR.puts(m) }
    nil
  end

  #: (?String?) -> void
  def abort(msg = nil)
    STDERR.puts(msg) if msg
    raise SystemExit.new(1)
  end

  # ponytail: MRI's Kernel#gets reads the files named in ARGV first (ARGF); this reads stdin only.
  #: () -> String?
  def gets = STDIN.gets
end
