# rbs_inline: enabled

require_relative "fileutils"
require_relative "tmpdir"
require_relative "etc"
require_relative "delegate"

# Wraps File over os.CreateTemp; no finalizer (unlike MRI's GC-driven one), so callers must close/unlink explicitly (decision 63).
class Tempfile < Object
  include IOWritable
  include IOReadable

  #: (?String) -> void
  def initialize(basename = "")
    @file = File.new(Tempfile.__mktemp(basename), "w+")
    @unlinked = false
  end

  #: (String) -> String
  def self.__mktemp(basename) = %x{
    f, err := os.CreateTemp("", string(basename)+"*")
    if err != nil {
      panic(rbSysErr(err, "rb_sysopen", os.TempDir()))
    }
    name := f.Name()
    _ = f.Close()
    return String(name)
  }

  #: [T] (?String) { (File) -> T } -> T
  def self.create(basename = "")
    path = __mktemp(basename)
    f = File.new(path, "w+")
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
