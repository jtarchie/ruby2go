# rbs_inline: enabled

require_relative "fileutils"
require_relative "etc"

# `require "tmpdir"`: Dir.mktmpdir. MRI's tmpdir.rb loads fileutils and etc.
class Dir
  # The directory and its contents go when the block returns.
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
