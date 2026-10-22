# rbs_inline: enabled

require_relative "fileutils"
require_relative "tmpdir"
require_relative "etc"
require_relative "delegate"

# Wraps File over os.CreateTemp; no finalizer (unlike MRI's GC-driven one), so callers must close/unlink explicitly (decision 63).
class Tempfile < DelegateClass(File)

  # tmpdir nil is Dir.tmpdir.
  #: (?(String | Array[String]), ?String?) -> void
  def initialize(basename = "", tmpdir = nil)
    dir = tmpdir || ""
    @file = basename.is_a?(String) ? Tempfile.__mktemp(basename, "", dir) : Tempfile.__mktemp(basename[0] || "", basename[1] || "", dir)
    super(@file)
    @unlinked = false
  end

  # Tempfile.new, then the block with it, closed (not unlinked) after: the block's value (Homebrew's File.atomic_write).
  #: [T] (?(String | Array[String]), ?String?) { (Tempfile) -> T } -> T
  def self.open(basename = "", tmpdir = nil)
    t = new(basename, tmpdir)
    begin
      yield t
    ensure
      t.close
    end
  end

  # The os.CreateTemp handle itself (O_EXCL, 0600), never reopened by path, so a symlink swapped in between cannot redirect it.
  #: (String, ?String, ?String) -> File
  def self.__mktemp(basename, suffix = "", dir = "") = %x{
    f, err := os.CreateTemp(string(dir), string(basename)+"*"+string(suffix))
    if err != nil {
      panic(rbSysErr(err, "rb_sysopen", os.TempDir()))
    }
    return &File{f: f, path: f.Name(), r: bufio.NewReader(f), w: bufio.NewWriter(f)}
  }

  #: [T] (?String) { (File) -> T } -> T
  def self.create(basename = "")
    f = __mktemp(basename)
    path = f.path
    begin
      yield f
    ensure
      f.close
      File.delete(path) if File.exist?(path)
    end
  end

  # nil once unlinked, as MRI's.
  #: () -> String?
  def path = @unlinked ? nil : @file.path

  # Kernel's private puts, print, gets, ... would answer before the delegation does, so these forward by hand.
  #: (*untyped) -> nil
  def puts(*args) = @file.puts(*args)

  #: (*untyped) -> nil
  def print(*args) = @file.print(*args)

  #: (String, *untyped) -> nil
  def printf(fmt, *args) = @file.printf(fmt, *args)

  #: (Integer) -> Integer
  def putc(c) = @file.putc(c)

  #: () -> String?
  def gets = @file.gets

  #: (?chomp: bool) -> String
  def readline(chomp: false) = @file.readline(chomp: chomp)

  #: () -> Array[String]
  def readlines = @file.readlines

  # unlink_now removes the file too; true then, else nil, as MRI's.
  #: (?bool) -> bool?
  def close(unlink_now = false)
    @file.close
    unlink_now ? unlink : nil
  end

  #: () -> bool?
  def close!
    close(true)
  end

  # Reopen a closed Tempfile read-write on the same path.
  #: (*untyped) -> untyped
  def open(*)
    @file.close
    @file = File.open(@file.path, "r+")
    __setobj__(@file)
    nil
  end

  # An open file asks the descriptor, a closed one the path: so unlinked-but-open still answers.
  #: () -> Integer
  def size
    return File.size(@file.path) if @file.closed?

    @file.flush
    @file.stat.size
  end

  #: () -> Integer
  def length = size

  #: () -> bool?
  def unlink
    return nil if @unlinked

    File.delete(@file.path) if File.exist?(@file.path)
    @unlinked = true
  end

  #: () -> bool?
  def delete = unlink

  #: () -> String
  def inspect = "#<Tempfile:#{path}#{@file.closed? ? " (closed)" : ""}>"
end
