# prelude/pathname.rb
# rbs_inline: enabled
#
# Pathname: an object holding its path, with a fluent API over File/Dir
# (decision 65).
# An ordinary object as MRI's, so a program can subclass it or give it
# instance variables (decision 289).

class Pathname < Object
  include Comparable

  # @rbs @path: String

  #: (untyped) -> void
  def initialize(path)
    @path = __path_of(path)
  end

  #: (untyped) -> String
  def __path_of(path) = %x{ return rbToS(path) }

  #: () -> Pathname
  def self.pwd = Pathname.new(Dir.pwd)

  #: () -> Pathname
  def self.getwd = pwd

  #: (String | Pathname | Array[String], ?Integer) -> Array[Pathname]
  def self.glob(pattern, flags = 0)
    pats = pattern.is_a?(Array) ? pattern : [pattern.to_s]
    out = [] #: Array[Pathname]
    pats.each { |pat| Dir.glob(pat, flags).each { |p| out << Pathname.new(p) } }
    out
  end

  # Pathname.glob with a block yields each match and answers nil (Homebrew's Keg#empty_installation?).
  #: (String | Pathname | Array[String], ?Integer) { (Pathname) -> untyped } -> nil
  def self.__glob_block(pattern, flags = 0)
    glob(pattern, flags).each { |p| yield p }
    nil
  end

  #: (Pathname) -> Integer
  def <=>(other) = @path <=> other.to_s

  #: (untyped) -> bool
  def ==(other) = other.is_a?(Pathname) && @path == other.to_s

  #: (untyped) -> bool
  def eql?(other) = self == other

  #: () -> Integer
  def hash = to_s.hash

  #: () -> String
  def to_s = @path.dup

  #: () -> String
  def to_path = to_s

  #: () -> String
  def inspect = "#<Pathname:#{@path}>"

  #: (untyped) -> Pathname
  def +(other)
    o = __path_of(other)
    Pathname.new(o.start_with?("/") ? o : __join(@path, o))
  end

  #: (String, String) -> String
  def __join(a, b) = %x{ String(rbPathJoin(string(a), string(b))) }

  #: (*untyped) -> Pathname
  def join(*parts)
    result = self #: Pathname
    parts.each { |p| result = result + p }
    result
  end

  #: (String | Pathname) -> Integer
  def rename(to) = File.rename(to_s, to.to_s)

  # Find.find over this path: depth first, each directory's entries sorted, Find.prune skipping a directory
  # (Homebrew's Keg#link_dir).
  # The block is a closure, not a loop body (its value is untyped), so Find.prune can unwind out of it.
  #: () { (Pathname) -> untyped } -> nil
  def find(&blk)
    Pathname.__find(to_s) do |s|
      blk.call(Pathname.new(s))
      nil
    end
  end

  #: (String) { (String) -> untyped } -> nil
  def self.__find(root, &blk) = %x{
    rbFindEach(string(root), blk)
  }

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

  #: (?Integer?, ?Integer?, **untyped) -> String
  def read(length = nil, offset = nil, **opts) = File.read(to_s, length, offset, **opts)

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
  def relative_path_from(base) = Pathname.new(__rel(__path_of(base), @path))

  #: (String, String) -> String
  def __rel(b, p) = %x{
    rel, err := filepath.Rel(string(b), string(p))
    if err != nil {
      panic(NewArgumentError(Ref(String("different prefix: " + string(p) + " and " + string(b)))))
    }
    return String(rel)
  }

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

  #: () -> Time
  def ctime = File.ctime(to_s)

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
  def grpowned? = File.grpowned?(to_s)

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

  #: (String, ?Integer) -> bool
  def fnmatch?(pattern, flags = 0) = File.fnmatch(pattern, to_s, flags)

  #: (String, ?Integer) -> bool
  def fnmatch(pattern, flags = 0) = File.fnmatch(pattern, to_s, flags)

  #: (Integer?, Integer?) -> Integer
  def chown(owner, group) = File.chown(owner, group, to_s)

  #: (?Integer?) -> Integer
  def mkdir(mode = nil) = Dir.mkdir(to_s, mode)

  # Every missing parent too, each with the mode; an existing directory is fine.
  #: (?mode: Integer?) -> self
  def mkpath(mode: nil)
    return self if directory?

    up = dirname
    up.mkpath(mode: mode) unless up.to_s == to_s
    begin
      Dir.mkdir(to_s, mode)
      File.chmod(mode, to_s) if mode
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

  # glob with a block yields each match and answers nil.
  #: (String) { (Pathname) -> untyped } -> nil
  def __glob_block(pattern)
    glob(pattern).each { |p| yield p }
    nil
  end

  #: (?bool) { (Pathname) -> untyped } -> Array[Pathname]
  def each_child(with_directory = true)
    kids = children(with_directory)
    kids.each { |c| yield c }
    kids
  end

  # The root first, then each path down to this one.
  #: () { (Pathname) -> untyped } -> nil
  def descend
    paths = [] #: Array[Pathname]
    ascend { |p| paths.unshift(p) }
    paths.each { |p| yield p }
    nil
  end

  # Without a block: an Enumerator of the same paths (Homebrew's `target.descend.first`).
  #: () -> Enumerator[Pathname]
  def __descend_enum
    paths = [] #: Array[Pathname]
    descend { |p| paths << p }
    paths.each
  end

  #: () -> Enumerator[Pathname]
  def __ascend_enum
    paths = [] #: Array[Pathname]
    ascend { |p| paths << p }
    paths.each
  end

  #: () -> Integer?
  def world_writable? = File.world_writable?(to_s)

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
    path = self #: Pathname
    loop do
      blk.call(path)
      up = path.dirname
      break if up.to_s == path.to_s || up.to_s == "." && !path.to_s.start_with?("./") # MRI: a/b stops at a, ./a reaches .

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

module Kernel

  private

  #: (untyped) -> Pathname
  def Pathname(path) = Pathname.new(path)
end
