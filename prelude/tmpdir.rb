# rbs_inline: enabled

require_relative "fileutils"
require_relative "etc"

# `require "tmpdir"`: Dir.mktmpdir. MRI's tmpdir.rb loads fileutils and etc.
class Dir
  # The directory and its contents go when the block returns. prefix_suffix is a prefix String or a [prefix, suffix]
  # pair; tmpdir is where it goes (Dir.tmpdir by default): a String or a Pathname.
  #: [T] (?untyped, ?untyped) { (String) -> T } -> T
  def self.mktmpdir(prefix_suffix = nil, tmpdir = nil)
    dir = __mktmp(prefix_suffix, tmpdir)
    begin
      yield dir
    ensure
      __rm_rf(dir)
    end
  end

  # Without a block: the new directory's path, left for the caller to remove.
  #: () -> String
  def self.__mktmpdir_0 = __mktmp(nil, nil)

  #: (untyped) -> String
  def self.__mktmpdir_1(prefix_suffix) = __mktmp(prefix_suffix, nil)

  #: (untyped, untyped) -> String
  def self.__mktmpdir_2(prefix_suffix, tmpdir) = __mktmp(prefix_suffix, tmpdir)

  #: (untyped, untyped) -> String
  def self.__mktmp(prefix_suffix, tmpdir) = %x{
    prefix, suffix := "d", ""
    switch v := rbUnbox(prefix_suffix).(type) {
    case String:
      prefix = string(v)
    case Array_Any:
      if a := v._ToAny().s; len(a) > 0 {
        prefix = string(rbToS(a[0]))
        if len(a) > 1 {
          suffix = string(rbToS(a[1]))
        }
      }
    }
    dir := ""
    if t := rbUnbox(tmpdir); t != nil {
      dir = string(rbToS(t))
    }
    d, err := os.MkdirTemp(dir, prefix+"*"+suffix)
    if err != nil {
      if dir == "" {
        dir = os.TempDir()
      }
      // MRI names the directory it tried (tmpdir/prefix + a random tail), not the system temp dir
      panic(rbSysErr(err, "dir_s_mkdir", filepath.Join(dir, prefix+suffix)))
    }
    return String(d)
  }

  #: (String) -> nil
  def self.__rm_rf(path) = %x{ _ = os.RemoveAll(string(path)) }
end
