# rbs_inline: enabled

# @go_type struct { f *os.File; r *bufio.Reader; w *bufio.Writer; path string }
class File < Object
  include IOWritable
  include IOReadable

  # Modes "r", "w", "a" and their "+" forms; failures raise MRI's Errno classes (decision 62).
  #: (String, ?String) -> File
  def self.new(path, mode = "r") = %x{
    flags := map[String]int{
      "r": os.O_RDONLY, "r+": os.O_RDWR,
      "w": os.O_WRONLY | os.O_CREATE | os.O_TRUNC, "w+": os.O_RDWR | os.O_CREATE | os.O_TRUNC,
      "a": os.O_WRONLY | os.O_CREATE | os.O_APPEND, "a+": os.O_RDWR | os.O_CREATE | os.O_APPEND,
    }
    flag, ok := flags[mode]
    if !ok {
      panic(NewArgumentError(Ref("invalid access mode " + mode)))
    }
    f, err := os.OpenFile(string(path), flag, 0o666) //nolint:gosec // MRI's mode; the umask applies
    if err != nil {
      panic(rbSysErr(err, "rb_sysopen", string(path)))
    }
    out := &File{f: f, path: string(path)}
    if mode != "w" && mode != "a" {
      out.r = bufio.NewReader(f)
    }
    if mode != "r" {
      out.w = bufio.NewWriter(f)
    }
    return out
  }

  #: [T] (String, ?String) { (File) -> T } -> T
  def self.open(path, mode = "r")
    f = File.new(path, mode)
    begin
      yield f
    ensure
      f.close
    end
  end

  #: (String) -> String
  def self.read(path) = %x{
    b, err := os.ReadFile(string(path))
    if err != nil {
      fn := "rb_sysopen"
      if errors.Is(err, syscall.EISDIR) {
        fn = "io_fread"
      }
      panic(rbSysErr(err, fn, string(path)))
    }
    return String(b)
  }

  #: (String, untyped) -> Integer
  def self.write(path, data) = %x{
    s := string(rbToS(data))
    err := os.WriteFile(string(path), []byte(s), 0o666) //nolint:gosec // MRI's mode; the umask applies
    if err != nil {
      panic(rbSysErr(err, "rb_sysopen", string(path)))
    }
    return Integer(len(s))
  }

  #: (String) -> Array[String]
  def self.readlines(path) = File.open(path) { |f| f.readlines }

  #: (String) { (String) -> void } -> void
  def self.foreach(path)
    f = File.new(path)
    begin
      while (line = f.gets)
        yield line
      end
    ensure
      f.close
    end
  end

  #: (String) -> bool
  def self.exist?(path) = %x{
    _, err := os.Stat(string(path))
    return Boolean(err == nil)
  }

  #: (String) -> bool
  def self.file?(path) = %x{
    fi, err := os.Stat(string(path))
    return Boolean(err == nil && fi.Mode().IsRegular())
  }

  #: (String) -> bool
  def self.symlink?(path) = %x{
    fi, err := os.Lstat(string(path))
    return Boolean(err == nil && fi.Mode()&os.ModeSymlink != 0)
  }

  #: (String) -> bool
  def self.directory?(path) = %x{
    fi, err := os.Stat(string(path))
    return Boolean(err == nil && fi.IsDir())
  }

  #: (String) -> Integer
  def self.size(path) = %x{
    fi, err := os.Stat(string(path))
    if err != nil {
      panic(rbSysErr(err, "rb_file_s_size", string(path)))
    }
    return Integer(fi.Size())
  }

  #: (String) -> Integer
  def self.delete(path) = %x{
    err := os.Remove(string(path))
    if err != nil {
      panic(rbSysErr(err, "apply2files", string(path)))
    }
    return 1
  }

  #: (String) -> Integer
  def self.unlink(path) = File.delete(path)

  #: (String, String) -> Integer
  def self.rename(from, to) = %x{
    err := os.Rename(string(from), string(to))
    if err != nil {
      panic(rbSysErr(err, "rb_file_s_rename", "("+string(from)+", "+string(to)+")"))
    }
    return 0
  }

  #: (String, ?String) -> String
  def self.basename(path, suffix = "") = %x{ String(rbBasename(string(path), string(suffix))) }

  #: (String) -> String
  def self.dirname(path) = %x{ String(rbDirname(string(path))) }

  #: (String) -> String
  def self.extname(path) = %x{ String(rbExtname(string(path))) }

  #: (*String) -> String
  def self.join(*parts) = %x{
    out := ""
    for i, p := range rest_ {
      s := string(p)
      switch {
      case i == 0:
      case strings.HasSuffix(out, "/"):
        s = strings.TrimLeft(s, "/")
      case !strings.HasPrefix(s, "/"):
        s = "/" + s
      }
      out += s
    }
    return String(out)
  }

  #: (String) -> String
  def self.expand_path(path) = %x{
    p := string(path)
    if p == "~" || strings.HasPrefix(p, "~/") {
      p = os.Getenv("HOME") + p[1:]
    }
    abs, err := filepath.Abs(p)
    if err != nil {
      panic(rbSysErr(err, "rb_file_expand_path", p))
    }
    return String(abs)
  }

  #: (String) -> bool
  def self.absolute_path?(path) = %x{ Boolean(strings.HasPrefix(string(path), "/")) }

  #: () -> String
  def path = %x{ String(self.path) }

  #: (untyped) -> Integer
  def write(x) = %x{
    if self.w == nil {
      panic(NewIOError(Ref[String]("not opened for writing")))
    }
    s := string(rbToS(x))
    _, _ = self.w.WriteString(s)
    return Integer(len(s))
  }

  #: (untyped) -> File
  def <<(x)
    write(x)
    self
  end

  #: () -> String?
  def gets = %x{
    self.rbReadable()
    line, err := self.r.ReadString('\\n')
    if line == "" && err != nil {
      return nil
    }
    s := String(line)
    return &s
  }

  #: () -> String
  def read = %x{
    self.rbReadable()
    b, _ := io.ReadAll(self.r)
    return String(b)
  }

  #: () -> bool
  def eof? = %x{
    self.rbReadable()
    _, err := self.r.Peek(1)
    return Boolean(err != nil)
  }

  #: () -> File
  def flush = %x{
    if self.w != nil {
      _ = self.w.Flush()
    }
    return self
  }

  #: () -> nil
  def close = %x{
    if self.f == nil {
      return
    }
    if self.w != nil {
      _ = self.w.Flush()
    }
    _ = self.f.Close()
    self.f = nil
  }

  #: () -> bool
  def closed? = %x{ Boolean(self.f == nil) }

  #: () -> String
  def inspect = %x{
    s := "#<File:" + self.path
    if self.f == nil {
      s += " (closed)"
    }
    return String(s + ">")
  }
end

class Dir < Object
  #: () -> String
  def self.pwd = %x{
    d, err := os.Getwd()
    if err != nil {
      panic(rbSysErr(err, "rb_dir_getwd", "."))
    }
    return String(d)
  }

  # ponytail: sorted, where MRI's children/entries come in readdir order; sort both sides to compare.
  #: (String) -> Array[String]
  def self.children(path) = %x{
    es, err := os.ReadDir(string(path))
    if err != nil {
      panic(rbSysErr(err, "dir_initialize", string(path)))
    }
    names := make([]string, len(es))
    for i, e := range es {
      names[i] = e.Name()
    }
    return rbStrs(names)
  }

  #: (String) -> Array[String]
  def self.entries(path) = [".", ".."] + children(path)

  # ponytail: filepath.Glob syntax, so no `**` or `{a,b}`; walk the tree for those.
  #: (String) -> Array[String]
  def self.glob(pattern) = %x{
    ms, err := filepath.Glob(string(pattern))
    if err != nil {
      return &Array[String]{}
    }
    return rbStrs(ms)
  }

  #: (String) -> bool
  def self.exist?(path) = File.directory?(path)

  #: (String) -> Integer
  def self.mkdir(path) = %x{
    err := os.Mkdir(string(path), 0o777) //nolint:gosec // MRI's mode; the umask applies
    if err != nil {
      panic(rbSysErr(err, "rb_dir_s_mkdir", string(path)))
    }
    return 0
  }

  #: (String) -> Integer
  def self.rmdir(path) = %x{
    err := os.Remove(string(path))
    if err != nil {
      panic(rbSysErr(err, "rb_dir_s_rmdir", string(path)))
    }
    return 0
  }

  # From `require "tmpdir"` in MRI; the directory and its contents go when the block returns.
  #: [T] () { (String) -> T } -> T
  def self.mktmpdir
    dir = __mktmp
    begin
      yield dir
    ensure
      __rm_rf(dir)
    end
  end

  #: () -> String
  def self.__mktmp = %x{
    d, err := os.MkdirTemp("", "d")
    if err != nil {
      panic(rbSysErr(err, "rb_dir_s_mkdir", os.TempDir()))
    }
    return String(d)
  }

  #: (String) -> nil
  def self.__rm_rf(path) = %x{ _ = os.RemoveAll(string(path)) }
end
