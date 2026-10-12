# prelude/pathname.rb
# rbs_inline: enabled
#
# Pathname: a frozen-string wrapper with a fluent API over File/Dir
# (decision 62). See docs/design.md decision 65 for scope and limitations.

# @go_type string
class Pathname < Object
  include Comparable

  #: (untyped) -> Pathname
  def self.new(path) = %x{ Pathname(string(rbToS(path))) }

  #: () -> Pathname
  def self.pwd = Pathname.new(Dir.pwd)

  #: () -> Pathname
  def self.getwd = pwd

  #: (Pathname) -> Integer
  def <=>(other) = %x{ Integer(strings.Compare(string(self), string(other))) }

  #: (untyped) -> bool
  def ==(other) = %x{
    o, ok := other.(Pathname)
    return Boolean(ok && self == o)
  }

  #: (untyped) -> bool
  def eql?(other) = self == other

  #: () -> Integer
  def hash = to_s.hash

  #: () -> String
  def to_s = %x{ rbStrClone(String(self)) }

  #: () -> String
  def to_path = to_s

  #: () -> String
  def inspect = %x{ String("#<Pathname:" + string(self) + ">") }

  #: (untyped) -> Pathname
  def +(other) = %x{
    o := string(rbToS(other))
    if strings.HasPrefix(o, "/") {
      return Pathname(o)
    }
    return Pathname(rbPathJoin(string(self), o))
  }

  #: (untyped) -> Pathname
  def /(other) = self + other

  #: (*untyped) -> Pathname
  def join(*parts)
    result = self
    parts.each { |p| result = result + p }
    result
  end

  #: (String | Pathname) -> Integer
  def rename(to) = File.rename(to_s, to.to_s)

  # Depth first, each directory's entries sorted, as Find.find (ponytail: no Find.prune).
  #: () { (Pathname) -> void } -> nil
  def find(&blk)
    blk.call(self)
    Dir.children(to_s).sort.each { |c| (self + c).find(&blk) } if directory? && !File.symlink?(to_s)
    nil
  end

  #: (?String?) -> Pathname
  def basename(ext = nil)
    if ext
      Pathname.new(File.basename(to_s, ext))
    else
      Pathname.new(File.basename(to_s))
    end
  end

  #: () -> Pathname
  def dirname = Pathname.new(File.dirname(to_s))

  #: () -> String
  def extname = File.extname(to_s)

  #: () -> Pathname
  def parent = dirname

  #: (?bool) -> Array[Pathname]
  def children(with_directory = true)
    names = Dir.children(to_s)
    if with_directory
      names.map { |n| self + n }
    else
      names.map { |n| Pathname.new(n) }
    end
  end

  #: () -> bool
  def exist? = File.exist?(to_s)

  #: () -> bool
  def file? = File.file?(to_s)

  #: () -> bool
  def directory? = File.directory?(to_s)

  #: () -> String
  def read = File.read(to_s)

  #: (untyped) -> Integer
  def write(data) = File.write(to_s, data)

  # File.open on this path, as MRI's Pathname#open.
  #: [T] (?(String | Integer | nil), ?Integer?, **untyped) { (File) -> T } -> T
  def open(mode = nil, perm = nil, **opts) = File.open(to_s, mode, perm, **opts) { |f| yield f }

  #: () { (String) -> void } -> void
  def each_filename
    to_s.split("/").each { |c| yield c if c != "" }
  end

  #: (untyped) -> Pathname
  def relative_path_from(base) = %x{
    b := string(rbToS(base))
    rel, err := filepath.Rel(b, string(self))
    if err != nil {
      panic(NewArgumentError(Ref(String("different prefix: " + string(self) + " and " + b))))
    }
    return Pathname(rel)
  }
end

module Kernel

  private

  #: (untyped) -> Pathname
  def Pathname(path) = Pathname.new(path)
end

class Pathname
  # MRI's file-system half of Pathname, over File and Dir (#brew; decision 65's scope widened).
  #: (?(String | Pathname)?) -> Pathname
  def expand_path(dir = nil) = Pathname.new(dir ? File.expand_path(to_s, dir.to_s) : File.expand_path(to_s))

  #: (?(String | Pathname)?) -> Pathname
  def realpath(dir = nil) = Pathname.new(File.realpath(dir ? File.expand_path(to_s, dir.to_s) : to_s))

  #: (?(String | Pathname)?) -> Pathname
  def realdirpath(dir = nil) = Pathname.new(File.expand_path(to_s, dir&.to_s))

  #: () -> File::Stat
  def stat = File.stat(to_s)

  #: () -> File::Stat
  def lstat = File.lstat(to_s)

  #: () -> Time
  def mtime = File.mtime(to_s)

  #: () -> Time
  def atime = File.atime(to_s)

  #: () -> Integer
  def size = File.size(to_s)

  #: () -> Integer?
  def size? = File.size?(to_s)

  #: () -> bool
  def zero? = File.zero?(to_s)

  #: () -> bool
  def empty? = directory? ? Dir.empty?(to_s) : File.empty?(to_s)

  #: () -> bool
  def symlink? = File.symlink?(to_s)

  #: () -> bool
  def executable? = File.executable?(to_s)

  #: () -> bool
  def readable? = File.readable?(to_s)

  #: () -> bool
  def writable? = File.writable?(to_s)

  #: () -> bool
  def owned? = File.owned?(to_s)

  #: () -> bool
  def absolute? = to_s.start_with?("/")

  #: () -> bool
  def relative? = !absolute?

  #: () -> bool
  def root? = to_s.match?(%r{\A/+\z})

  #: () -> String
  def ftype = File.ftype(to_s)

  #: (Integer) -> Integer
  def chmod(mode) = File.chmod(mode, to_s)

  #: (?Integer) -> Integer
  def mkdir(_mode = 0o777) = Dir.mkdir(to_s) # ponytail: mode ignored (Dir.mkdir takes none yet)

  # Every missing parent too; an existing directory is fine.
  #: (?mode: Integer?) -> self
  def mkpath(mode: nil)
    return self if directory?

    up = dirname
    up.mkpath unless up.to_s == to_s
    begin
      Dir.mkdir(to_s) # ponytail: mode ignored
    rescue Errno::EEXIST
      raise unless directory?
    end
    self
  end

  #: () -> Integer
  def rmdir = Dir.rmdir(to_s)

  #: () -> self
  def rmtree
    if directory? && !symlink?
      children.each(&:rmtree)
      Dir.rmdir(to_s)
    elsif symlink? || exist?
      File.unlink(to_s)
    end
    self
  end

  #: () -> Integer
  def unlink = directory? && !symlink? ? Dir.rmdir(to_s) : File.unlink(to_s)

  #: () -> Integer
  def delete = unlink

  #: (String | Pathname) -> Integer
  def make_symlink(old) = File.symlink(old.to_s, to_s)

  #: (String | Pathname) -> Integer
  def make_link(old) = File.link(old.to_s, to_s)

  #: () -> Pathname
  def readlink = Pathname.new(File.readlink(to_s))

  #: (?Integer?, ?Integer?) -> String
  def binread(length = nil, offset = nil)
    data = File.binread(to_s)
    data = data.byteslice(offset, data.bytesize) || "" if offset
    length ? (data.byteslice(0, length) || "") : data
  end

  #: (String) -> Integer
  def binwrite(data) = File.binwrite(to_s, data)

  #: () -> Array[String]
  def readlines = File.readlines(to_s)

  #: () { (String) -> void } -> nil
  def each_line(&blk)
    File.readlines(to_s).each(&blk)
    nil
  end

  #: () -> Array[Pathname]
  def entries = Dir.entries(to_s).map { |e| Pathname.new(e) }

  #: (String) -> Array[Pathname]
  def glob(pattern) = Dir.glob(File.join(to_s, pattern)).map { |p| Pathname.new(p) }

  #: (Time, Time) -> Integer
  def utime(atime, mtime) = File.utime(atime, mtime, to_s)

  #: (Integer) -> Integer
  def truncate(length) = File.truncate(to_s, length)

  #: (Regexp | String, String) -> Pathname
  def sub(pattern, replacement) = Pathname.new(to_s.sub(pattern, replacement))

  #: (String) -> Pathname
  def sub_ext(ext)
    e = extname
    Pathname.new((e.empty? ? to_s : to_s[0, to_s.size - e.size].to_s) + ext)
  end

  #: () -> Array[Pathname]
  def split = [dirname, basename]

  # This path and each parent up to the root.
  #: () { (Pathname) -> void } -> nil
  def ascend(&blk)
    path = self
    loop do
      blk.call(path)
      up = path.dirname
      break if up.to_s == path.to_s

      path = up
    end
    nil
  end

  # `.` and `..` resolved lexically, as MRI's cleanpath.
  #: () -> Pathname
  def cleanpath
    abs = absolute?
    parts = [] #: Array[String]
    to_s.split("/").each do |p|
      next if p.empty? || p == "."

      if p == ".." && !parts.empty? && parts.last != ".."
        parts.pop
      elsif p != ".." || !abs
        parts << p
      end
    end
    joined = parts.join("/")
    return Pathname.new(abs ? "/#{joined}" : (joined.empty? ? "." : joined))
  end

  #: (Pathname | String) -> Pathname
  def /(other) = self + other

  # A Pathname is an immutable value already.
  #: () -> Pathname
  def freeze = self

  #: () -> bool
  def frozen? = true
end
