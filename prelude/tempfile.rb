# rbs_inline: enabled

require_relative "fileutils"
require_relative "tmpdir"
require_relative "etc"
require_relative "delegate"

# Wraps File over os.CreateTemp; no finalizer (unlike MRI's GC-driven one), so callers must close/unlink explicitly (decision 63).
class Tempfile < Object
  include IOWritable
  include IOReadable

  #: (?(String | Array[String])) -> void
  def initialize(basename = "")
    @file = basename.is_a?(String) ? Tempfile.__mktemp(basename) : Tempfile.__mktemp(basename[0] || "", basename[1] || "")
    @unlinked = false
  end

  # The os.CreateTemp handle itself (O_EXCL, 0600), never reopened by path, so a symlink swapped in between cannot redirect it.
  #: (String, ?String) -> File
  def self.__mktemp(basename, suffix = "") = %x{
    f, err := os.CreateTemp("", string(basename)+"*"+string(suffix))
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

  #: () -> String?
  def path = @unlinked ? nil : @file.path

  #: (untyped) -> Integer
  def write(x) = @file.write(x)

  #: (untyped) -> Tempfile
  def <<(x)
    write(x)
    self
  end

  #: () -> String?
  def gets = @file.gets

  #: () -> String
  def read = @file.read

  #: () -> bool
  def eof? = @file.eof?

  #: () -> void
  def close = @file.close

  #: () -> void
  def unlink
    return if @unlinked

    File.delete(@file.path) if File.exist?(@file.path)
    @unlinked = true
  end

  #: () -> void
  def delete = unlink

  #: () -> String
  def inspect
    p = path
    p ? "#<Tempfile:#{p}>" : "#<Tempfile:(closed)>"
  end
end
