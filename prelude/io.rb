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

  #: (Integer) -> Integer
  def putc(c)
    write((c & 0xff).chr)
    c
  end

  #: (String) -> String
  def __putc_string(s)
    write(s[0] || "")
    s
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

  #: (?chomp: bool) -> String
  def readline(chomp: false)
    line = gets
    raise EOFError, "end of file reached" unless line
    chomp && line.end_with?("\n") ? line.chomp : line # a final "x\r" keeps its \r, as MRI
  end

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

# @go_type struct { fd int; via rbWriter; lineno int }
class IO < Object
  include IOWritable
  include IOReadable

  SEEK_SET = 0 #: Integer
  SEEK_CUR = 1 #: Integer
  SEEK_END = 2 #: Integer

  #: (Integer) -> IO
  def self.__new(fd) = %x{ return &IO{fd: int(fd)} }

  #: (String) -> String
  def self.read(path) = File.read(path)

  #: (String) -> String
  def self.binread(path) = File.read(path)

  #: (String, untyped) -> Integer
  def self.write(path, data) = File.write(path, data)

  #: (String, untyped) -> Integer
  def self.binwrite(path, data) = File.write(path, data)

  #: (String) -> Array[String]
  def self.readlines(path) = File.readlines(path)

  #: (String) { (String) -> void } -> void
  def self.foreach(path)
    File.foreach(path) { |line| yield line }
  end

  # A connected [reader, writer]; the writer is sync, as MRI's. Both are Files here, so their class prints File, not IO.
  #: () -> [File, File]
  def self.pipe = %x{
    r, w, err := os.Pipe()
    if err != nil {
      panic(rbSysErr(err, "rb_io_s_pipe", ""))
    }
    return Tuple2[*File, *File]{&File{f: r, r: bufio.NewReader(r), path: "pipe"}, &File{f: w, w: bufio.NewWriter(w), path: "pipe", sync: true}}
  }

  # The command's standard output as a File for the block; $? is its status after. A String
  # follows system's shell rule, an Array of Strings runs directly.
  # @rbs [T] (untyped) { (File) -> T } -> T
  def self.popen(cmd)
    io = __popen_start(cmd)
    begin
      yield io
    ensure
      __popen_finish(io)
    end
  end

  #: (untyped) -> File
  def self.__popen_start(cmd) = %x{ return rbPopen(cmd) }

  #: (File) -> void
  def self.__popen_finish(io) = %x{ rbPopenWait(io) }

  # Path to path, as MRI's with two filenames; the count of bytes copied.
  #: (String, String) -> Integer
  def self.copy_stream(src, dst) = File.write(dst, File.read(src))

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
    if self.via != nil { // $stdout/$stderr read while assigned (decision 109)
      self.via.Write(String(s))
      return Integer(len(s))
    }
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

  # STDERR is always sync, as in MRI; STDOUT until `sync = true` only flushes per write on a terminal.
  #: () -> bool
  def sync = %x{ Boolean(self.fd == 2 || self.fd == 1 && stdoutSync.Load()) }

  #: (bool) -> bool
  def sync=(on)
    %x{
    if self.fd == 1 {
      stdoutSync.Store(bool(on))
      if on {
        rbFlush()
      }
    }
    return on
    }
  end

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
    self.lineno++
    s := String(line)
    return &s
  }

  #: () -> Integer
  def lineno = %x{ Integer(self.lineno) }

  #: (Integer) -> Integer
  def lineno=(n)
    %x{
    self.lineno = int(n)
    return n
    }
  end

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

  #: () -> String?
  def getc = %x{
    self.rbReadable()
    return rbGetc(rbStdin)
  }

  #: () -> Integer?
  def getbyte = %x{
    self.rbReadable()
    return rbGetbyte(rbStdin)
  }

  #: () -> String
  def readchar = %x{
    self.rbReadable()
    return *rbEOF(rbGetc(rbStdin))
  }

  #: () -> Integer
  def readbyte = %x{
    self.rbReadable()
    return *rbEOF(rbGetbyte(rbStdin))
  }

  #: (String) -> nil
  def ungetc(s) = %x{
    self.rbReadable()
    rbUngetc(&rbStdin, string(s))
  }

  #: () { (String) -> void } -> void
  def each_char
    while (c = getc)
      yield c
    end
  end

  #: () { (Integer) -> void } -> void
  def each_byte
    while (b = getbyte)
      yield b
    end
  end

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

    raise KeyError.__for("key not found: #{name.inspect}", self, name)
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

  #: () { (String, String) -> void } -> void
  def each_pair
    to_h.each { |k, v| yield k, v }
  end

  #: () { (String) -> void } -> void
  def each_key
    to_h.each { |k, _v| yield k }
  end

  #: () { (String) -> void } -> void
  def each_value
    to_h.each { |_k, v| yield v }
  end

  #: () -> Hash[String, String]
  def to_hash = to_h

  #: () -> Array[[String, String]]
  def to_a = to_h.to_a

  #: () -> Array[String]
  def values = to_h.values

  #: (*String) -> Array[String?]
  def values_at(*names) = names.map { |n| self[n] }

  #: () -> Integer
  def size = to_h.size

  #: () -> Integer
  def length = size

  #: () -> bool
  def empty? = size == 0

  #: (String) -> bool
  def has_key?(name) = key?(name)

  #: (String) -> bool
  def member?(name) = key?(name)

  #: (String) -> bool
  def value?(value) = to_h.value?(value)

  #: (String) -> bool
  def has_value?(value) = value?(value)

  #: (String) -> String?
  def key(value) = to_h.key(value)

  #: () -> Hash[String, String]
  def invert = to_h.invert

  #: (String) -> [String, String]?
  def assoc(name) = to_h.assoc(name)

  #: (*String) -> Hash[String, String]
  def slice(*names) = to_h.slice(*names)

  #: (*String) -> Hash[String, String]
  def except(*names) = to_h.except(*names)

  #: [U] () { (String, String) -> U } -> Array[U]
  def map
    out = [] #: Array[U]
    to_h.each { |k, v| out << yield(k, v) }
    out
  end

  #: [U] () { (String, String) -> U? } -> Array[U]
  def filter_map
    out = [] #: Array[U]
    to_h.each do |k, v|
      x = yield(k, v)
      out << x if x
    end
    out
  end

  #: () { (String, String) -> bool } -> Hash[String, String]
  def select
    out = {} #: Hash[String, String]
    to_h.each { |k, v| out[k] = v if yield(k, v) }
    out
  end

  #: () { (String, String) -> bool } -> Hash[String, String]
  def filter
    out = {} #: Hash[String, String]
    to_h.each { |k, v| out[k] = v if yield(k, v) }
    out
  end

  #: () { (String, String) -> bool } -> Hash[String, String]
  def reject
    out = {} #: Hash[String, String]
    to_h.each { |k, v| out[k] = v unless yield(k, v) }
    out
  end

  #: () { (String, String) -> bool } -> bool
  def any?
    to_h.each { |k, v| return true if yield(k, v) }
    false
  end

  #: (String, String) -> String
  def store(name, value)
    self[name] = value
    value
  end

  #: (Hash[String, String]) -> self
  def update(other)
    other.each { |k, v| self[k] = v }
    self
  end

  #: (Hash[String, String]) -> self
  def merge!(other) = update(other)

  #: () { (String, String) -> bool } -> self
  def delete_if
    to_h.each { |k, v| self[k] = nil if yield(k, v) }
    self
  end

  #: () { (String, String) -> bool } -> self
  def keep_if
    to_h.each { |k, v| self[k] = nil unless yield(k, v) }
    self
  end

  #: (Hash[String, String]) -> self
  def replace(other)
    to_h.each { |k, _v| self[k] = nil unless other.key?(k) }
    update(other)
  end

  #: () -> self
  def clear
    to_h.each { |k, _v| self[k] = nil }
    self
  end
end

ENV = ENVClass.new #: ENVClass

# ARGF (decision 95): the files named in ARGV, read in turn and shifted out
# of it as each opens, or stdin when ARGV is empty at the first read.
# @go_type struct { r *bufio.Reader; f *os.File; name string; started, stdin, done bool }
class ARGFClass < Object
  #: () -> ARGFClass
  def self.new = %x{ return &ARGFClass{} }

  #: () -> String?
  def gets = __gets(ARGV)

  #: () -> String
  def read = __read(ARGV)

  #: () -> Array[String]
  def readlines
    out = [] #: Array[String]
    while (line = gets)
      out << line
    end
    out
  end

  #: () { (String) -> void } -> void
  def each_line
    while (line = gets)
      yield line
    end
  end

  #: () -> String
  def filename = __filename(ARGV)

  #: () -> bool
  def eof? = __eof(ARGV)

  #: () -> String
  def inspect = "ARGF"

  #: () -> String
  def to_s = "ARGF"

  #: (Array[String]) -> String?
  def __gets(argv) = %x{
    for r := self.next(argv); r != nil; r = self.next(argv) {
      line, err := r.ReadString('\\n')
      if err != nil {
        self.spent()
      }
      if line != "" {
        s := String(line)
        return &s
      }
    }
    return nil
  }

  #: (Array[String]) -> String
  def __read(argv) = %x{
    var b strings.Builder
    for r := self.next(argv); r != nil; r = self.next(argv) {
      _, _ = io.Copy(&b, r)
      self.spent()
    }
    return String(b.String())
  }

  #: (Array[String]) -> String
  def __filename(argv) = %x{
    self.next(argv)
    return String(self.name)
  }

  #: (Array[String]) -> bool
  def __eof(argv) = %x{
    r := self.next(argv)
    if r == nil && !self.stdin {
      panic(NewIOError(Ref[String]("closed stream"))) // the last file is closed, as in MRI
    }
    if r == nil {
      return true
    }
    _, err := r.Peek(1)
    return Boolean(err != nil)
  }
end

ARGF = ARGFClass.new #: ARGFClass

# Kernel#warn writes through Warning.warn; the category switches are MRI 4.0's defaults (decision 129).
module Warning
  CATEGORIES__ = {deprecated: false, experimental: true, performance: false, strict_unused_block: false} #: Hash[Symbol, bool]

  #: (Symbol) -> bool
  def self.[](category)
    on = CATEGORIES__[category]
    raise ArgumentError, "unknown category: #{category}" if on.nil?

    on
  end

  #: (Symbol, bool) -> bool
  def self.[]=(category, flag)
    self[category]
    CATEGORIES__[category] = flag
  end

  #: () -> Array[Symbol]
  def self.categories = CATEGORIES__.keys

  #: (String, ?category: Symbol?) -> nil
  def self.warn(msg, category: nil)
    self[category] if category
    $stderr.write(msg)
    nil
  end
end

module Kernel
  private

  # Built as MRI's rb_warn_m: arrays flatten, each message gets a newline unless it has one, and the whole goes to Warning.warn.
  #: (*untyped, ?uplevel: Integer?, ?category: Symbol?) -> nil
  def warn(*msgs, uplevel: nil, category: nil)
    msgs = msgs.flatten
    return nil if msgs.empty? || (category && !Warning[category])

    s = ""
    if uplevel
      raise ArgumentError, "negative level (#{uplevel})" if uplevel < 0

      loc = __caller_loc(uplevel)
      s += "#{loc}: " unless loc.empty?
      s += "warning: "
    end
    msgs.each do |m|
      t = m.to_s
      s += (t.end_with?("\n") ? t : t + "\n")
    end
    Warning.warn(s, category: category)
  end

  # The user-code frame uplevel frames up from warn's caller (decision 129).
  #: (Integer) -> String
  def __caller_loc(n) = %x{ return String(rbCallerLocN(int(n))) }

  #: (?String?) -> void
  def abort(msg = nil)
    STDERR.puts(msg) if msg
    raise SystemExit.new(1)
  end

  #: () -> String?
  def gets = ARGF.gets
end
