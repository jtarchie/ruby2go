# prelude/pathname.rb
# rbs_inline: enabled
#
# Pathname: a frozen-string wrapper with a fluent API over File/Dir
# (decision 62). See README decision 65 for scope and limitations.

# @go_type string
class Pathname < Object
  include Comparable

  #: (untyped) -> Pathname
  def self.new(path) = %x{ Pathname(string(rbToS(path))) }

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
