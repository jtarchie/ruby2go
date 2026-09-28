# rbs_inline: enabled

# FileUtils on os/io/path-filepath; pathlist args dispatch String vs Array via decision 12's `__<name>_array` overload convention (decision 63).
module FileUtils
  #: (String) -> nil
  def self.mkdir_p(path) = %x{
    err := os.MkdirAll(string(path), 0o777) //nolint:gosec // MRI's mode; the umask applies
    if err != nil {
      panic(rbSysErr(err, "mkdir_p", string(path)))
    }
  }

  #: (Array[String]) -> nil
  def self.__mkdir_p_array(paths)
    paths.each { |p| mkdir_p(p) }
    nil
  end

  #: (String) -> nil
  def self.rm_rf(path) = %x{ _ = os.RemoveAll(string(path)) }

  #: (Array[String]) -> nil
  def self.__rm_rf_array(paths)
    paths.each { |p| rm_rf(p) }
    nil
  end

  #: (String) -> nil
  def self.rm_f(path)
    File.delete(path)
    nil
  rescue StandardError
    nil
  end

  #: (Array[String]) -> nil
  def self.__rm_f_array(paths)
    paths.each { |p| rm_f(p) }
    nil
  end

  #: (String) -> nil
  def self.rm(path) # File.delete already raises MRI's exact apply2files message (decision 62)
    File.delete(path)
    nil
  end

  #: (Array[String]) -> nil
  def self.__rm_array(paths)
    paths.each { |p| rm(p) }
    nil
  end

  #: (String, String) -> nil
  def self.cp(src, dest)
    dest = File.join(dest, File.basename(src)) if File.directory?(dest)
    __cp_file(src, dest)
  end

  #: (Array[String], String) -> nil
  def self.__cp_array(srcs, dest)
    srcs.each { |s| cp(s, dest) }
    nil
  end

  #: (String, String) -> nil
  def self.__cp_file(src, dest) = %x{
    if err := rbCopyFile(string(src), string(dest)); err != nil {
      panic(rbSysErr(err, "rb_sysopen", string(src)))
    }
  }

  #: (String, String) -> nil
  def self.cp_r(src, dest)
    dest = File.join(dest, File.basename(src)) if File.directory?(dest)
    __cp_r_entry(src, dest)
  end

  #: (Array[String], String) -> nil
  def self.__cp_r_array(srcs, dest)
    srcs.each { |s| cp_r(s, dest) }
    nil
  end

  #: (String, String) -> nil
  def self.__cp_r_entry(src, dest)
    if File.directory?(src)
      __cp_tree(src, dest)
    else
      __cp_file(src, dest)
    end
  end

  #: (String, String) -> nil
  def self.__cp_tree(src, dest) = %x{
    if err := rbCopyTree(string(src), string(dest)); err != nil {
      panic(rbSysErr(err, "rb_sysopen", string(src)))
    }
  }

  #: (String, String) -> nil
  def self.mv(src, dest)
    dest = File.join(dest, File.basename(src)) if File.directory?(dest)
    __mv_entry(src, dest)
  end

  #: (Array[String], String) -> nil
  def self.__mv_array(srcs, dest)
    srcs.each { |s| mv(s, dest) }
    nil
  end

  #: (String, String) -> nil
  def self.__mv_entry(src, dest) = %x{
    if err := rbMove(string(src), string(dest)); err != nil {
      panic(rbSysErr(err, "rb_file_s_rename", "("+string(src)+", "+string(dest)+")"))
    }
  }

  #: (String) -> nil
  def self.touch(path) = %x{
    if err := rbTouch(string(path)); err != nil {
      panic(rbSysErr(err, "rb_sysopen", string(path)))
    }
  }

  #: (Array[String]) -> nil
  def self.__touch_array(paths)
    paths.each { |p| touch(p) }
    nil
  end

  #: (String, String) -> nil
  def self.ln_s(src, dest)
    dest = File.join(dest, File.basename(src)) if File.directory?(dest)
    __ln_s_entry(src, dest)
  end

  #: (Array[String], String) -> nil
  def self.__ln_s_array(srcs, dest)
    srcs.each { |s| ln_s(s, dest) }
    nil
  end

  #: (String, String) -> nil
  def self.__ln_s_entry(src, dest) = %x{
    if err := os.Symlink(string(src), string(dest)); err != nil {
      panic(rbSysErr(err, "rb_file_s_symlink", "("+string(src)+", "+string(dest)+")"))
    }
  }
end
