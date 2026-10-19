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

  # Copies one file to the exact path dest (cp also takes a directory); preserve and dereference are accepted, not acted on.
  #: (String, String, ?bool, ?bool) -> nil
  def self.copy_file(src, dest, preserve = false, dereference = true) = __cp_file(src, dest)

  #: (Array[String], String) -> nil
  def self.__cp_array(srcs, dest)
    srcs.each { |s| cp(s, dest) }
    nil
  end

  #: (String, String) -> nil
  def self.__cp_file(src, dest) = %x{
    if err := rbCopyFile(string(src), string(dest)); err != nil {
      panic(rbFileErr(err, "rb_sysopen", string(src)))
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
      panic(rbFileErr(err, "rb_sysopen", string(src)))
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
      panic(rbFileErr(err, "rb_file_s_rename", "("+string(src)+", "+string(dest)+")"))
    }
  }

  # One method, not class twins: a dynamic call's wrapper cannot pass a twin its keywords.
  #: (String | Pathname | Array[String], ?mtime: Time?, ?nocreate: bool) -> nil
  def self.touch(list, mtime: nil, nocreate: false)
    paths = list.is_a?(Array) ? list : [list.to_s]
    paths.each { |p| __touch1(p, mtime, nocreate) }
    nil
  end

  #: (String, Time?, bool) -> nil
  def self.__touch1(path, mtime, nocreate) = %x{
    t := time.Now()
    if mtime != nil {
      t = (*mtime).t
    }
    if err := rbTouch(string(path), t, bool(nocreate)); err != nil {
      op := "rb_sysopen"
      if nocreate {
        op = "apply2files" // MRI's File.utime
      }
      panic(rbSysErr(err, op, string(path)))
    }
  }

  # force: replaces an existing dest (Homebrew's `FileUtils.ln_s target, symlink, force: true` of Pathnames).
  #: (String | Pathname, String | Pathname, ?force: bool) -> nil
  def self.ln_s(src, dest, force: false)
    s = src.to_s
    d = dest.to_s
    d = File.join(d, File.basename(s)) if File.directory?(d)
    File.delete(d) if force && (File.symlink?(d) || File.exist?(d))
    __ln_s_entry(s, d)
  end

  #: (String | Pathname, String | Pathname) -> nil
  def self.ln_sf(src, dest) = ln_s(src, dest, force: true)

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

  # The list forms Homebrew's keg and download code calls (#100): a path, a Pathname or an Array of them.
  #: (String | Pathname | Array[String | Pathname]) -> Array[String]
  def self.__paths(list) = list.is_a?(Array) ? list.map(&:to_s) : [list.to_s]

  # Each path and what it holds; a missing one raises Errno::ENOENT unless force.
  #: (String | Pathname | Array[String | Pathname], ?force: bool, ?secure: bool, ?verbose: bool, ?noop: bool) -> nil
  def self.rm_r(list, force: false, secure: false, verbose: false, noop: false)
    return nil if noop

    __paths(list).each do |p|
      unless File.symlink?(p) || File.exist?(p)
        raise Errno::ENOENT, "No such file or directory @ rb_file_s_lstat - #{p}" unless force

        next
      end
      rm_rf(p)
    end
    nil
  end

  #: (String | Pathname | Array[String | Pathname], ?verbose: bool, ?noop: bool) -> nil
  def self.remove_entry(list, verbose: false, noop: false) = rm_r(list, noop: noop)

  #: (String | Pathname, ?bool) -> nil
  def self.remove_entry_secure(path, force = false) = rm_r(path, force: force)

  #: (String | Pathname | Array[String | Pathname], ?mode: Integer?, ?verbose: bool, ?noop: bool) -> nil
  def self.mkdir(list, mode: nil, verbose: false, noop: false)
    return nil if noop

    __paths(list).each { |p| Dir.mkdir(p) }
    nil
  end

  #: (String | Pathname | Array[String | Pathname], ?parents: bool, ?verbose: bool, ?noop: bool) -> nil
  def self.rmdir(list, parents: false, verbose: false, noop: false)
    return nil if noop

    __paths(list).each { |p| Dir.rmdir(p) }
    nil
  end

  #: (String | Pathname | Array[String | Pathname], ?mode: Integer?, ?verbose: bool) -> nil
  def self.mkpath(list, mode: nil, verbose: false)
    __paths(list).each { |p| mkdir_p(p) }
    nil
  end

  # mode is an Integer or a symbolic "u+w,go-x" / "a=r".
  #: (Integer | String, String | Pathname | Array[String | Pathname], ?verbose: bool, ?noop: bool) -> nil
  def self.chmod(mode, list, verbose: false, noop: false)
    return nil if noop

    __paths(list).each { |p| File.chmod(__mode(mode, File.lstat(p).mode), p) }
    nil
  end

  #: (Integer | String, String | Pathname | Array[String | Pathname], ?verbose: bool, ?noop: bool, ?force: bool) -> nil
  def self.chmod_R(mode, list, verbose: false, noop: false, force: false)
    return nil if noop

    __paths(list).each do |root|
      __walk(root).each { |p| File.chmod(__mode(mode, File.lstat(p).mode), p) unless File.symlink?(p) }
    end
    nil
  end

  #: (String | Integer | nil, String | Integer | nil, String | Pathname | Array[String | Pathname], ?verbose: bool, ?noop: bool) -> nil
  def self.chown(user, group, list, verbose: false, noop: false)
    return nil if noop

    __paths(list).each { |p| File.chown(__uid(user), __gid(group), p) }
    nil
  end

  #: (String | Integer | nil, String | Integer | nil, String | Pathname | Array[String | Pathname], ?verbose: bool, ?noop: bool, ?force: bool) -> nil
  def self.chown_R(user, group, list, verbose: false, noop: false, force: false)
    return nil if noop

    __paths(list).each { |root| __walk(root).each { |p| File.chown(__uid(user), __gid(group), p) } }
    nil
  end

  # A hard link; force replaces an existing dest.
  #: (String | Pathname, String | Pathname, ?force: bool, ?verbose: bool, ?noop: bool) -> nil
  def self.ln(src, dest, force: false, verbose: false, noop: false)
    return nil if noop

    s = src.to_s
    d = dest.to_s
    d = File.join(d, File.basename(s)) if File.directory?(d)
    File.delete(d) if force && (File.symlink?(d) || File.exist?(d))
    File.link(s, d)
    nil
  end

  # Copies src to dest with mode (an Integer or symbolic), as install(1).
  #: (String | Pathname, String | Pathname, ?mode: (Integer | String)?, ?verbose: bool, ?noop: bool) -> nil
  def self.install(src, dest, mode: nil, verbose: false, noop: false)
    return nil if noop

    s = src.to_s
    d = dest.to_s
    d = File.join(d, File.basename(s)) if File.directory?(d)
    File.delete(d) if File.exist?(d)
    __cp_file(s, d)
    File.chmod(__mode(mode, File.lstat(d).mode), d) if mode
    nil
  end

  #: () -> String
  def self.pwd = Dir.pwd

  # Whether the two files hold the same bytes (MRI's compare_file; Homebrew's InstallRenamed.destination_for).
  #: (String | Pathname, String | Pathname) -> bool
  def self.compare_file(a, b) = %x{ Boolean(rbSameContents(string(rbToS(a)), string(rbToS(b)))) }

  #: (String | Pathname, String | Pathname) -> bool
  def self.identical?(a, b) = compare_file(a, b)

  #: (String | Pathname, String | Pathname) -> bool
  def self.cmp(a, b) = compare_file(a, b)

  # Whether new exists and is newer than every existing old.
  #: (String | Pathname, Array[String | Pathname]) -> bool
  def self.uptodate?(new, old_list)
    n = new.to_s
    return false unless File.exist?(n)

    t = File.mtime(n)
    old_list.all? { |o| !File.exist?(o.to_s) || File.mtime(o.to_s) < t }
  end

  # root and, for a directory, everything under it, parents first.
  #: (String) -> Array[String]
  def self.__walk(root)
    out = [root]
    return out if File.symlink?(root) || !File.directory?(root)

    Dir.children(root).sort.each { |c| out.concat(__walk(File.join(root, c))) }
    out
  end

  #: (String | Integer | nil) -> Integer?
  def self.__uid(user)
    return nil if user.nil?
    return user if user.is_a?(Integer)

    Etc.getpwnam(user).uid
  end

  #: (String | Integer | nil) -> Integer?
  def self.__gid(group)
    return nil if group.nil?
    return group if group.is_a?(Integer)

    Etc.getgrnam(group).gid
  end

  # An Integer mode as it is; a symbolic one ("u+w", "go-rwx", "a=r", comma separated) applied to current.
  #: (Integer | String | nil, Integer) -> Integer
  def self.__mode(mode, current)
    return current & 0o7777 if mode.nil?
    return mode if mode.is_a?(Integer)

    m = current & 0o7777
    mode.split(",").each do |clause|
      md = /\A([ugoa]*)([-+=])([rwxXst]*)\z/.match(clause)
      raise ArgumentError, "invalid file mode: #{mode}" unless md

      who = md[1].to_s
      who = "ugo" if who.empty? || who == "a"
      bits = 0
      md[3].to_s.each_char do |c|
        case c
        when "r" then bits |= 0o444
        when "w" then bits |= 0o222
        when "x", "X" then bits |= 0o111
        when "s" then bits |= 0o6000
        when "t" then bits |= 0o1000
        end
      end
      mask = 0
      mask |= 0o4700 if who.include?("u")
      mask |= 0o2070 if who.include?("g")
      mask |= 0o1007 if who.include?("o")
      bits &= mask
      case md[2]
      when "+" then m |= bits
      when "-" then m &= ~bits
      else m = (m & ~mask) | bits
      end
    end
    m
  end
end
