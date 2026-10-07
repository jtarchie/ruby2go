# rbs_inline: enabled

# @go_type struct { f *os.File; r *bufio.Reader; w *bufio.Writer; path string; sync bool; lineno int; enc rbIOEnc; conv func(string) string }
class File < Object
  include IOWritable
  include IOReadable

  # Modes "r", "w", "a", their "+" forms, "b", and ":ext[:int]" encodings (decision 136); failures raise MRI's Errno classes (decision 62).
  #: (String, ?String) -> File
  def self.new(path, mode = "r") = %x{
    flags := map[string]int{
      "r": os.O_RDONLY, "r+": os.O_RDWR,
      "w": os.O_WRONLY | os.O_CREATE | os.O_TRUNC, "w+": os.O_RDWR | os.O_CREATE | os.O_TRUNC,
      "a": os.O_WRONLY | os.O_CREATE | os.O_APPEND, "a+": os.O_RDWR | os.O_CREATE | os.O_APPEND,
    }
    access, spec, _ := strings.Cut(string(mode), ":")
    bin := strings.ContainsRune(access, 'b')
    access = strings.NewReplacer("b", "", "t", "").Replace(access)
    flag, ok := flags[access]
    if !ok {
      panic(NewArgumentError(Ref("invalid access mode " + mode)))
    }
    var withEnc func(*File)
    if spec != "" {
      hook := rbFileEncHook.Load()
      if hook == nil {
        panic(NewNotImplementedError(Ref(String("rb2go: a File mode naming encodings must be passed to File.open, File.new or CSV.open directly (decision 136)"))))
      }
      withEnc = (*hook)(access, spec, bin)
    }
    f, err := os.OpenFile(string(path), flag, 0o666) //nolint:gosec // MRI's mode; the umask applies
    if err != nil {
      panic(rbSysErr(err, "rb_sysopen", string(path)))
    }
    out := &File{f: f, path: string(path)}
    if bin {
      out.enc = rbIOEnc{ext: "ASCII-8BIT", bin: true}
    }
    if access != "w" && access != "a" {
      out.r = bufio.NewReader(f)
    }
    if access != "r" {
      out.w = bufio.NewWriter(f)
    }
    if withEnc != nil {
      withEnc(out)
    }
    return out
  }

  SEPARATOR = "/" #: String
  ALT_SEPARATOR = nil #: String?
  PATH_SEPARATOR = ":" #: String

  # Blockless File.open is File.new (decision 12's `__<name>_enum` stands for "no block").
  #: (String, ?String) -> File
  def self.__open_enum(path, mode = "r") = File.new(path, mode)

  #: (String) -> Time
  def self.mtime(path) = %x{
    fi, err := os.Stat(string(path))
    if err != nil {
      panic(rbSysErr(err, "rb_file_s_mtime", string(path)))
    }
    return &Time{t: fi.ModTime()}
  }

  #: (String) -> Time
  def self.atime(path) = %x{
    fi, err := os.Stat(string(path))
    if err != nil {
      panic(rbSysErr(err, "rb_file_s_atime", string(path)))
    }
    return &Time{t: rbAtime(fi)}
  }

  LOCK_SH = 1 #: Integer
  LOCK_EX = 2 #: Integer
  LOCK_NB = 4 #: Integer
  LOCK_UN = 8 #: Integer

  # Sets access and modification times; the count of paths, as MRI's.
  #: (Time, Time, *String) -> Integer
  def self.utime(atime, mtime, *paths) = %x{
    for _, p := range rest_ {
      if err := os.Chtimes(string(p), atime.t, mtime.t); err != nil {
        panic(rbSysErr(err, "rb_file_s_utime", string(p)))
      }
    }
    return Integer(len(rest_))
  }

  #: (String) -> File::Stat
  def self.stat(path) = %x{
    fi, err := os.Stat(string(path))
    if err != nil {
      panic(rbSysErr(err, "rb_file_s_stat", string(path)))
    }
    return &File_Stat{fi: fi}
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

  #: (String) -> String
  def self.binread(path) = read(path)

  #: (String, untyped) -> Integer
  def self.binwrite(path, data) = write(path, data)

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

  NULL = "/dev/null" #: String

  #: (String) -> bool
  def self.zero?(path) = exist?(path) && size(path) == 0

  #: (String) -> bool
  def self.empty?(path) = zero?(path)

  #: (String) -> bool
  def self.readable?(path) = %x{ Boolean(syscall.Access(string(path), 4) == nil) }

  #: (String) -> bool
  def self.writable?(path) = %x{ Boolean(syscall.Access(string(path), 2) == nil) }

  #: (String) -> bool
  def self.executable?(path) = %x{ Boolean(syscall.Access(string(path), 1) == nil) }

  #: (String) -> bool
  def self.owned?(path) = %x{
    fi, err := os.Stat(string(path))
    if err != nil {
      return false
    }
    st, ok := fi.Sys().(*syscall.Stat_t)
    return Boolean(ok && int(st.Uid) == os.Geteuid())
  }

  #: (String) -> String
  def self.realpath(path) = %x{
    p, err := filepath.Abs(string(path))
    if err == nil {
      p, err = filepath.EvalSymlinks(p)
    }
    if err != nil {
      panic(rbSysErr(err, "rb_check_realpath_internal", string(path)))
    }
    return String(p)
  }

  #: (String) -> [String, String]
  def self.split(path) = [dirname(path), basename(path)]

  #: (String) -> String
  def self.ftype(path) = lstat(path).ftype

  #: (String) -> File::Stat
  def self.lstat(path) = %x{
    fi, err := os.Lstat(string(path))
    if err != nil {
      panic(rbSysErr(err, "rb_file_s_lstat", string(path)))
    }
    return &File_Stat{fi: fi}
  }

  #: (String) -> String
  def self.readlink(path) = %x{
    s, err := os.Readlink(string(path))
    if err != nil {
      panic(rbSysErr(err, "rb_readlink", string(path)))
    }
    return String(s)
  }

  #: (Integer, *String) -> Integer
  def self.chmod(mode, *paths) = %x{
    for _, p := range rest_ {
      if err := os.Chmod(string(p), fs.FileMode(mode&0o777)); err != nil {
        panic(rbSysErr(err, "apply2files", string(p)))
      }
    }
    return Integer(len(rest_))
  }

  #: (String, String) -> Integer
  def self.symlink(old, dst) = %x{
    if err := os.Symlink(string(old), string(dst)); err != nil {
      panic(rbSysErr(err, "rb_file_s_symlink", "("+string(old)+", "+string(dst)+")"))
    }
    return 0
  }

  #: (String, String) -> Integer
  def self.link(old, dst) = %x{
    if err := os.Link(string(old), string(dst)); err != nil {
      panic(rbSysErr(err, "rb_file_s_link", "("+string(old)+", "+string(dst)+")"))
    }
    return 0
  }

  #: (String, Integer) -> Integer
  def self.truncate(path, n) = %x{
    if err := os.Truncate(string(path), int64(n)); err != nil {
      panic(rbSysErr(err, "rb_file_s_truncate", string(path)))
    }
    return 0
  }

  #: (String) -> Integer
  def self.size(path) = %x{
    fi, err := os.Stat(string(path))
    if err != nil {
      panic(rbSysErr(err, "rb_file_s_size", string(path)))
    }
    return Integer(fi.Size())
  }

  #: (String) -> Integer?
  def self.size?(path) = zero?(path) || !exist?(path) ? nil : size(path)

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

  # Arrays (nested too) are flattened into the parts, as MRI does.
  #: (*(String | Array[untyped])) -> String
  def self.join(*parts) = %x{
    flat := &Array[any]{}
    rbFlattenInto(flat, rest_, -1)
    out := ""
    for i, p := range flat.s {
      str, ok := p.(String)
      if !ok {
        panic(NewTypeError(Ref(String("no implicit conversion of " + rbClassName(p) + " into String"))))
      }
      s := string(str)
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

  # A reading File without one is default_external's, a writing one has none, as in MRI.
  #: () -> Encoding?
  def external_encoding
    e = __ext
    return Encoding.find(e) unless e.empty?

    __readable? ? Encoding.default_external : nil
  end

  #: () -> Encoding?
  def internal_encoding
    e = __int
    e.empty? ? nil : Encoding.find(e)
  end

  #: (untyped, ?untyped) -> File
  def set_encoding(ext, intern = nil) = %x{
    e := rbSetEncoding(ext, intern)
    e.bin, e.raw, e.unread, e.restart = self.enc.bin, self.enc.raw, self.enc.unread, self.enc.restart
    same := e.ext == self.enc.ext && e.intern == self.enc.intern // keep the transcoder and what it has read ahead
    self.enc = e
    if self.r != nil && !same {
      self.r = self.enc.readerFor(self.r)
    }
    self.conv = self.enc.writeConv
    return self
  }

  # Binary from here on: no conversion either way, external encoding ASCII-8BIT.
  #: () -> File
  def binmode = %x{
    self.enc = rbIOEnc{ext: "ASCII-8BIT", bin: true, raw: self.enc.raw, unread: self.enc.unread}
    if self.r != nil {
      self.r = self.enc.readerFor(self.r)
    }
    self.conv = nil
    return self
  }

  #: () -> bool
  def binmode? = %x{ Boolean(self.enc.bin) }

  #: () -> String
  def __ext = %x{ String(self.enc.ext) }

  #: () -> String
  def __int = %x{ String(self.enc.intern) }

  #: () -> bool
  def __readable? = %x{ Boolean(self.r != nil) }

  #: (untyped) -> Integer
  def write(x) = %x{
    if self.w == nil {
      panic(NewIOError(Ref[String]("not opened for writing")))
    }
    s := string(rbToS(x))
    if self.conv != nil {
      s = self.conv(s)
    }
    _, _ = self.w.WriteString(s)
    if self.sync {
      _ = self.w.Flush()
    }
    return Integer(len(s))
  }

  #: () -> bool
  def sync = %x{ Boolean(self.sync) }

  #: (bool) -> bool
  def sync=(on)
    %x{
    self.sync = bool(on)
    if self.sync && self.w != nil {
      _ = self.w.Flush()
    }
    return on
    }
  end

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
    self.lineno++
    s := String(line)
    return &s
  }

  # The count of lines gets has read (each_line and readline too); rewind sets it back to 0.
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
    b, _ := io.ReadAll(self.r)
    return String(b)
  }

  #: () -> bool
  def eof? = %x{
    self.rbReadable()
    _, err := self.r.Peek(1)
    return Boolean(err != nil)
  }

  #: () -> String?
  def getc = %x{
    self.rbReadable()
    return rbGetc(self.r)
  }

  #: () -> Integer?
  def getbyte = %x{
    self.rbReadable()
    return rbGetbyte(self.r)
  }

  #: () -> String
  def readchar = %x{
    self.rbReadable()
    return *rbEOF(rbGetc(self.r))
  }

  #: () -> Integer
  def readbyte = %x{
    self.rbReadable()
    return *rbEOF(rbGetbyte(self.r))
  }

  #: (String) -> nil
  def ungetc(s) = %x{
    self.rbReadable()
    rbUngetc(&self.r, string(s))
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

  #: () -> Integer
  def pos = %x{
    off, err := self.f.Seek(0, io.SeekCurrent)
    if err != nil {
      panic(rbSysErr(err, "rb_io_tell", self.path))
    }
    if self.r != nil {
      off -= int64(self.r.Buffered())
    }
    if self.w != nil {
      off += int64(self.w.Buffered())
    }
    return Integer(off)
  }

  #: () -> Integer
  def tell = pos

  # LOCK_SH, LOCK_EX or LOCK_UN, with LOCK_NB not to wait: 0, or false when LOCK_NB finds it held.
  #: (Integer) -> untyped
  def flock(op) = %x{
    if self.w != nil {
      _ = self.w.Flush()
    }
    if err := syscall.Flock(int(self.f.Fd()), int(op)); err != nil {
      if errors.Is(err, syscall.EWOULDBLOCK) {
        return Boolean(false)
      }
      panic(rbSysErr(err, "rb_file_flock", self.path))
    }
    return Integer(0)
  }

  #: (Integer, ?Integer) -> Integer
  def seek(offset, whence = 0) = %x{
    if self.w != nil {
      _ = self.w.Flush()
    }
    if whence == 1 && self.r != nil {
      offset -= Integer(self.r.Buffered())
    }
    if _, err := self.f.Seek(int64(offset), int(whence)); err != nil {
      panic(rbSysErr(err, "rb_io_seek", self.path))
    }
    if self.enc.raw != nil { // the file's own reader under any transcoder
      self.enc.raw.Reset(self.f)
    }
    switch {
    case self.enc.restart != nil: // what the transcoder read ahead is stale
      self.r = self.enc.restart()
    case self.enc.raw != nil:
      self.r = self.enc.raw
    case self.r != nil:
      self.r.Reset(self.f)
    }
    return 0
  }

  #: () -> Integer
  def rewind
    self.lineno = 0
    seek(0)
  end

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

# File.stat's result: a snapshot of os.Stat.
# @go_type struct { fi os.FileInfo }
class File::Stat < Object
  #: () -> Integer
  def size = %x{ Integer(self.fi.Size()) }

  #: () -> Time
  def mtime = %x{ &Time{t: self.fi.ModTime()} }

  #: () -> Time
  def atime = %x{ &Time{t: rbAtime(self.fi)} }

  #: () -> bool
  def file? = %x{ Boolean(self.fi.Mode().IsRegular()) }

  #: () -> bool
  def directory? = %x{ Boolean(self.fi.IsDir()) }

  #: () -> bool
  def zero? = size == 0

  #: () -> bool
  def symlink? = %x{ Boolean(self.fi.Mode()&fs.ModeSymlink != 0) }

  #: () -> String
  def ftype = %x{
    m := self.fi.Mode()
    switch {
    case m.IsRegular():
      return "file"
    case m.IsDir():
      return "directory"
    case m&fs.ModeSymlink != 0:
      return "link"
    case m&fs.ModeCharDevice != 0:
      return "characterSpecial"
    case m&fs.ModeDevice != 0:
      return "blockSpecial"
    case m&fs.ModeNamedPipe != 0:
      return "fifo"
    case m&fs.ModeSocket != 0:
      return "socket"
    }
    return "unknown"
  }

  # st_mode, file type bits included (0100644 for a plain file), as MRI's.
  #: () -> Integer
  def mode = %x{
    if st, ok := self.fi.Sys().(*syscall.Stat_t); ok {
      return Integer(st.Mode)
    }
    return Integer(self.fi.Mode().Perm())
  }
end

# FileTest: File's predicates as module functions.
module FileTest
  #: (String) -> bool
  def self.exist?(path) = File.exist?(path)

  #: (String) -> bool
  def self.file?(path) = File.file?(path)

  #: (String) -> bool
  def self.directory?(path) = File.directory?(path)

  #: (String) -> bool
  def self.symlink?(path) = File.symlink?(path)

  #: (String) -> bool
  def self.zero?(path) = File.zero?(path)

  #: (String) -> bool
  def self.empty?(path) = File.empty?(path)

  #: (String) -> Integer
  def self.size(path) = File.size(path)

  #: (String) -> Integer?
  def self.size?(path) = File.size?(path)

  #: (String) -> bool
  def self.readable?(path) = File.readable?(path)

  #: (String) -> bool
  def self.writable?(path) = File.writable?(path)

  #: (String) -> bool
  def self.executable?(path) = File.executable?(path)

  #: (String) -> bool
  def self.owned?(path) = File.owned?(path)
end

class Dir < Object
  attr_reader :path #: String

  #: (String) -> void
  def initialize(path)
    raise Errno::ENOTDIR, "Not a directory @ dir_initialize - #{path}" if File.exist?(path) && !File.directory?(path)
    Dir.children(path) # ENOENT, as MRI's dir_initialize
    @path = path
  end

  #: [T] (String) { (Dir) -> T } -> T
  def self.open(path)
    d = Dir.new(path)
    yield d
  end

  #: (String) -> Dir
  def self.__open_enum(path) = Dir.new(path)

  # ponytail: sorted, as Dir.children; MRI walks readdir order.
  #: () { (String) -> void } -> void
  def each
    Dir.entries(path).each { |e| yield e }
  end

  #: () -> Array[String]
  def children = Dir.children(path)

  #: () -> Array[String]
  def entries = Dir.entries(path)

  #: () { (String) -> void } -> void
  def each_child
    Dir.children(path).each { |e| yield e }
  end

  #: () -> String
  def to_path = path

  #: () -> nil
  def close = nil

  #: () -> String
  def inspect = "#<Dir:#{path}>"

  #: () -> String
  def self.home = %x{
    if h := os.Getenv("HOME"); h != "" {
      return String(h)
    }
    h, err := os.UserHomeDir()
    if err != nil {
      panic(NewArgumentError(Ref(String("couldn't find login name -- expanding '~'"))))
    }
    return String(h)
  }

  #: () -> String
  def self.getwd = pwd

  #: (String) -> bool
  def self.empty?(path) = %x{
    es, err := os.ReadDir(string(path))
    return Boolean(err == nil && len(es) == 0)
  }

  #: (String) -> Integer
  def self.delete(path) = rmdir(path)

  #: (String) -> Integer
  def self.unlink(path) = rmdir(path)

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

  # `*`, `?`, `[set]`, `**/`, `{a,b}` and a trailing `/`, in MRI's sorted order (rbGlob, decision 94).
  #: (String) -> Array[String]
  def self.glob(pattern) = %x{ return rbStrs(rbGlob(string(pattern))) }

  #: (*String) -> Array[String]
  def self.[](*patterns)
    out = [] #: Array[String]
    patterns.each { |pat| out.concat(glob(pat)) }
    out
  end

  # ponytail: sorted, like children.
  #: (String) { (String) -> void } -> void
  def self.each_child(path)
    children(path).each { |c| yield c }
  end

  # The working directory is the process's: other threads see the change too, as in MRI.
  #: [T] (String) { (String) -> T } -> T
  def self.chdir(path)
    old = pwd
    __chdir(path)
    begin
      yield path
    ensure
      __chdir(old)
    end
  end

  # Blockless Dir.chdir (decision 12's `__<name>_enum` stands for "no block").
  #: (String) -> Integer
  def self.__chdir_enum(path)
    __chdir(path)
    0
  end

  #: (String) -> void
  def self.__chdir(path) = %x{
    if err := os.Chdir(string(path)); err != nil {
      panic(rbSysErr(err, "dir_s_chdir", string(path)))
    }
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
end
