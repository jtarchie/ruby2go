# rbs_inline: enabled

require_relative "etc" # chown's user and group names
# A path list is one union-typed parameter (String | Pathname | Array of them), as MRI's fu_list; verbose: prints the shell command to $stdout (fileutils 1.8), then noop: skips the work.
module FileUtils
  #: (String) -> nil
  def self.__say(msg)
    $stdout.puts msg
    nil
  end

  #: (Integer | String | nil) -> String
  def self.__mode_to_s(mode) = mode.is_a?(String) ? mode : format("%o", mode.to_i)

  #: (Integer?) -> String
  def self.__mflag(mode) = mode ? "-m #{format("%03o", mode)} " : ""

  # chmod after the mkdir, as MRI's fu_mkdir: the umask must not narrow an explicit mode.
  #: (String, Integer?) -> nil
  def self.__mkdir1(path, mode)
    if mode
      Dir.mkdir(path, mode)
      File.chmod(mode, path)
    else
      Dir.mkdir(path)
    end
    nil
  end

  #: (String | Pathname | Array[String | Pathname], ?mode: Integer?, ?noop: bool?, ?verbose: bool?) -> Array[String]
  def self.mkdir_p(list, mode: nil, noop: nil, verbose: nil)
    paths = __paths(list)
    __say "mkdir -p #{__mflag(mode)}#{paths.join(" ")}" if verbose
    return paths if noop

    paths.each { |p| __mkdir_p1(p, mode) }
    paths
  end

  #: (String | Pathname | Array[String | Pathname], ?mode: Integer?, ?noop: bool?, ?verbose: bool?) -> Array[String]
  def self.makedirs(list, mode: nil, noop: nil, verbose: nil) = mkdir_p(list, mode: mode, noop: noop, verbose: verbose)

  # The missing parents outermost first, each tolerated if another process made it.
  #: (String, Integer?) -> nil
  def self.__mkdir_p1(item, mode)
    path = item == "/" ? item : item.chomp("/")
    stack = [] #: Array[String]
    until File.directory?(path) || File.dirname(path) == path
      stack.push(path)
      path = File.dirname(path)
    end
    stack.reverse_each { |dir| __mkdir_shared(dir, mode) }
    nil
  end

  #: (String, Integer?) -> nil
  def self.__mkdir_shared(dir, mode)
    __mkdir1(dir, mode)
  rescue SystemCallError
    raise unless File.directory?(dir)
    nil
  end

  #: (String | Pathname | Array[String | Pathname], ?noop: bool?, ?verbose: bool?, ?secure: bool?) -> Array[String]?
  def self.rm_rf(list, noop: nil, verbose: nil, secure: nil) = rm_r(list, force: true, noop: noop, verbose: verbose)

  #: (String | Pathname | Array[String | Pathname], ?noop: bool?, ?verbose: bool?, ?secure: bool?) -> Array[String]?
  def self.rmtree(list, noop: nil, verbose: nil, secure: nil) = rm_r(list, force: true, noop: noop, verbose: verbose)

  #: (String) -> nil
  def self.__rm_rf1(path) = %x{ _ = os.RemoveAll(string(path)) }

  # force swallows any error, as MRI's remove_file.
  #: (String | Pathname | Array[String | Pathname], ?noop: bool?, ?verbose: bool?) -> Array[String]?
  def self.rm_f(list, noop: nil, verbose: nil) = rm(list, force: true, noop: noop, verbose: verbose)

  #: (String | Pathname | Array[String | Pathname], ?noop: bool?, ?verbose: bool?) -> Array[String]?
  def self.safe_unlink(list, noop: nil, verbose: nil) = rm(list, force: true, noop: noop, verbose: verbose)

  # File.delete already raises MRI's exact apply2files message (decision 62).
  #: (String | Pathname | Array[String | Pathname], ?force: bool?, ?noop: bool?, ?verbose: bool?) -> Array[String]?
  def self.rm(list, force: nil, noop: nil, verbose: nil)
    paths = __paths(list)
    __say "rm#{force ? " -f" : ""} #{paths.join(" ")}" if verbose
    return nil if noop

    paths.each { |p| __rm1(p, force) }
    paths
  end

  #: (String | Pathname | Array[String | Pathname], ?force: bool?, ?noop: bool?, ?verbose: bool?) -> Array[String]?
  def self.remove(list, force: nil, noop: nil, verbose: nil) = rm(list, force: force, noop: noop, verbose: verbose)

  #: (String, bool?) -> nil
  def self.__rm1(path, force)
    File.delete(path)
    nil
  rescue StandardError
    raise unless force
    nil
  end

  # src is a path or a list of them; into a directory dest, each keeps its basename.
  #: (String | Pathname | Array[String | Pathname], String | Pathname, ?preserve: bool?, ?noop: bool?, ?verbose: bool?) -> Array[String | Pathname]?
  def self.cp(src, dest, preserve: nil, noop: nil, verbose: nil)
    srcs = __paths(src)
    d = dest.to_s
    __say "cp#{preserve ? " -p" : ""} #{(srcs + [d]).join(" ")}" if verbose
    return nil if noop

    srcs.each { |s| __cp_file(s, File.directory?(d) ? File.join(d, File.basename(s)) : d, preserve) }
    src.is_a?(Array) ? src : nil
  end

  #: (String | Pathname | Array[String | Pathname], String | Pathname, ?preserve: bool?, ?noop: bool?, ?verbose: bool?) -> Array[String | Pathname]?
  def self.copy(src, dest, preserve: nil, noop: nil, verbose: nil) = cp(src, dest, preserve: preserve, noop: noop, verbose: verbose)

  # Copies one file to the exact path dest (cp also takes a directory); dereference is accepted, not acted on.
  #: (String | Pathname, String | Pathname, ?bool, ?bool) -> nil
  def self.copy_file(src, dest, preserve = false, dereference = true) = __cp_file(src.to_s, dest.to_s, preserve)

  # preserve: the source's mode (special bits too) and times, as MRI's preserve_attributes (the owner is not carried over).
  #: (String, String, bool?) -> nil
  def self.__cp_file(src, dest, preserve)
    __cp_file0(src, dest)
    __preserve(src, dest) if preserve
    nil
  end

  #: (String, String) -> nil
  def self.__cp_file0(src, dest) = %x{
    if err := rbCopyFile(string(src), string(dest)); err != nil {
      panic(rbFileErr(err, "rb_sysopen", string(src)))
    }
  }

  #: (String, String) -> nil
  def self.__preserve(src, dest)
    st = File.stat(src)
    File.utime(st.atime, st.mtime, dest)
    File.chmod(st.mode & 0o7777, dest)
    nil
  end

  # dereference_root: a symlink given as the source is followed (its realpath is copied), else relinked; remove_destination: unlinks a file or symlink in the way first, never a directory.
  #: (String | Pathname | Array[String | Pathname], String | Pathname, ?preserve: bool?, ?noop: bool?, ?verbose: bool?, ?dereference_root: bool, ?remove_destination: bool?) -> Array[String | Pathname]?
  def self.cp_r(src, dest, preserve: nil, noop: nil, verbose: nil, dereference_root: true, remove_destination: nil)
    srcs = __paths(src)
    d = dest.to_s
    __say "cp -r#{preserve ? "p" : ""}#{remove_destination ? " --remove-destination" : ""} #{(srcs + [d]).join(" ")}" if verbose
    return nil if noop

    srcs.each do |s|
      t = File.directory?(d) ? File.join(d, File.basename(s)) : d
      root = s
      if dereference_root
        root = File.realpath(s)
      else
        File.lstat(s)
      end
      if File.symlink?(root)
        __cp_link(root, t, remove_destination)
      elsif File.directory?(root)
        __cp_tree(root, t, preserve, remove_destination)
      else
        __rm_dest(t) if remove_destination
        __cp_file(s, t, preserve)
      end
    end
    src.is_a?(Array) ? src : nil
  end

  #: (String, String, bool?, bool?) -> nil
  def self.__cp_tree(src, dest, preserve, remove_destination) = %x{
    if err := rbCopyTree(string(src), string(dest), preserve != nil && bool(*preserve), remove_destination != nil && bool(*remove_destination)); err != nil {
      panic(rbFileErr(err, "rb_sysopen", string(src)))
    }
  }

  #: (String, String, bool?) -> nil
  def self.__cp_link(src, dest, remove_destination)
    __rm_dest(dest) if remove_destination
    __ln_s_entry(File.readlink(src), dest)
  end

  #: (String) -> nil
  def self.__rm_dest(path) = %x{
    if err := rbRemoveDest(string(path)); err != nil {
      panic(rbSysErr(err, "apply2files", string(path)))
    }
  }

  #: (String | Pathname | Array[String | Pathname], String | Pathname, ?force: bool?, ?noop: bool?, ?verbose: bool?, ?secure: bool?) -> (Integer | Array[String | Pathname])?
  def self.mv(src, dest, force: nil, noop: nil, verbose: nil, secure: nil)
    srcs = __paths(src)
    d = dest.to_s
    __say "mv#{force ? " -f" : ""} #{(srcs + [d]).join(" ")}" if verbose
    return nil if noop

    srcs.each { |s| __mv_entry(s, File.directory?(d) ? File.join(d, File.basename(s)) : d) }
    src.is_a?(Array) ? src : 0
  end

  #: (String | Pathname | Array[String | Pathname], String | Pathname, ?force: bool?, ?noop: bool?, ?verbose: bool?, ?secure: bool?) -> (Integer | Array[String | Pathname])?
  def self.move(src, dest, force: nil, noop: nil, verbose: nil, secure: nil) = mv(src, dest, force: force, noop: noop, verbose: verbose, secure: secure)

  #: (String, String) -> nil
  def self.__mv_entry(src, dest) = %x{
    if err := rbMove(string(src), string(dest)); err != nil {
      panic(rbFileErr(err, "rb_file_s_rename", "("+string(src)+", "+string(dest)+")"))
    }
  }

  # One method, not class twins: a dynamic call's wrapper cannot pass a twin its keywords.
  #: (String | Pathname | Array[String | Pathname], ?mtime: Time?, ?nocreate: bool, ?noop: bool?, ?verbose: bool?) -> Array[String]?
  def self.touch(list, mtime: nil, nocreate: false, noop: nil, verbose: nil)
    paths = __paths(list)
    __say "touch #{nocreate ? "-c " : ""}#{mtime ? mtime.strftime("-t %Y%m%d%H%M.%S ") : ""}#{paths.join(" ")}" if verbose
    return nil if noop

    paths.each { |p| __touch1(p, mtime, nocreate) }
    paths
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
  #: (String | Pathname | Array[String | Pathname], String | Pathname, ?force: bool?, ?noop: bool?, ?verbose: bool?) -> (Integer | Array[String | Pathname])?
  def self.ln_s(src, dest, force: false, noop: nil, verbose: nil)
    srcs = __paths(src)
    d = dest.to_s
    __say "ln -s#{force ? "f" : ""} #{(srcs + [d]).join(" ")}" if verbose
    return nil if noop

    srcs.each do |s|
      t = File.directory?(d) ? File.join(d, File.basename(s)) : d
      File.delete(t) if force && (File.symlink?(t) || File.exist?(t))
      __ln_s_entry(s, t)
    end
    src.is_a?(Array) ? src : 0
  end

  #: (String | Pathname | Array[String | Pathname], String | Pathname, ?force: bool?, ?noop: bool?, ?verbose: bool?) -> (Integer | Array[String | Pathname])?
  def self.symlink(src, dest, force: false, noop: nil, verbose: nil) = ln_s(src, dest, force: force, noop: noop, verbose: verbose)

  #: (String | Pathname | Array[String | Pathname], String | Pathname, ?noop: bool?, ?verbose: bool?) -> (Integer | Array[String | Pathname])?
  def self.ln_sf(src, dest, noop: nil, verbose: nil) = ln_s(src, dest, force: true, noop: noop, verbose: verbose)

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
  #: (String | Pathname | Array[String | Pathname], ?force: bool?, ?secure: bool?, ?verbose: bool?, ?noop: bool?) -> Array[String]?
  def self.rm_r(list, force: false, secure: false, verbose: false, noop: false)
    paths = __paths(list)
    __say "rm -r#{force ? "f" : ""} #{paths.join(" ")}" if verbose
    return nil if noop

    paths.each { |p| __remove_entry1(p, force) }
    paths
  end

  # MRI's remove_entry: 1 for a file (File.unlink), 0 for a directory (Dir.rmdir); nil for a missing path under force.
  #: (String, bool?) -> Integer?
  def self.__remove_entry1(path, force)
    unless File.symlink?(path) || File.exist?(path)
      raise Errno::ENOENT, "No such file or directory @ rb_file_s_lstat - #{path}" unless force

      return nil
    end
    dir = File.directory?(path) && !File.symlink?(path)
    __rm_rf1(path)
    dir ? 0 : 1
  end

  #: (String | Pathname | Array[String | Pathname], ?bool, ?verbose: bool?, ?noop: bool?) -> Integer?
  def self.remove_entry(list, force = false, verbose: false, noop: false)
    return nil if noop

    r = nil #: Integer?
    __paths(list).each { |p| r = __remove_entry1(p, force) }
    r
  end

  #: (String | Pathname, ?bool) -> nil
  def self.remove_entry_secure(path, force = false)
    remove_entry(path, force)
    nil
  end

  #: (String | Pathname, ?bool) -> Integer?
  def self.remove_file(path, force = false)
    File.delete(path.to_s)
  rescue StandardError
    raise unless force
    nil
  end

  #: (String | Pathname, ?bool) -> Integer?
  def self.remove_dir(path, force = false)
    raise Errno::ENOTDIR, path.to_s unless force || File.directory?(path.to_s)

    remove_entry(path, force)
  end

  #: (String | Pathname | Array[String | Pathname], ?mode: Integer?, ?verbose: bool?, ?noop: bool?) -> Array[String]?
  def self.mkdir(list, mode: nil, verbose: false, noop: false)
    paths = __paths(list)
    __say "mkdir #{__mflag(mode)}#{paths.join(" ")}" if verbose
    return nil if noop

    paths.each { |p| __mkdir1(p, mode) }
    paths
  end

  #: (String | Pathname | Array[String | Pathname], ?parents: bool?, ?verbose: bool?, ?noop: bool?) -> Array[String]?
  def self.rmdir(list, parents: false, verbose: false, noop: false)
    paths = __paths(list)
    __say "rmdir #{parents ? "-p " : ""}#{paths.join(" ")}" if verbose
    return nil if noop

    paths.each do |item|
      dir = item == "/" ? item : item.chomp("/")
      Dir.rmdir(dir)
      next unless parents

      begin
        until (parent = File.dirname(dir)) == "." || parent == dir
          dir = parent
          Dir.rmdir(dir)
        end
      rescue Errno::ENOTEMPTY, Errno::EEXIST, Errno::ENOENT
        nil
      end
    end
    paths
  end

  #: (String | Pathname | Array[String | Pathname], ?mode: Integer?, ?noop: bool?, ?verbose: bool?) -> Array[String]
  def self.mkpath(list, mode: nil, noop: nil, verbose: nil) = mkdir_p(list, mode: mode, noop: noop, verbose: verbose)

  # mode is an Integer or a symbolic "u+w,go-x" / "a=r".
  #: (Integer | String, String | Pathname | Array[String | Pathname], ?verbose: bool?, ?noop: bool?) -> Array[String]?
  def self.chmod(mode, list, verbose: false, noop: false)
    paths = __paths(list)
    __say "chmod #{__mode_to_s(mode)} #{paths.join(" ")}" if verbose
    return nil if noop

    paths.each { |p| File.chmod(__mode(mode, File.lstat(p).mode), p) }
    paths
  end

  #: (Integer | String, String | Pathname | Array[String | Pathname], ?verbose: bool?, ?noop: bool?, ?force: bool?) -> Array[String]?
  def self.chmod_R(mode, list, verbose: false, noop: false, force: false)
    paths = __paths(list)
    __say "chmod -R#{force ? "f" : ""} #{__mode_to_s(mode)} #{paths.join(" ")}" if verbose
    return nil if noop

    paths.each do |root|
      __walk(root).each { |p| File.chmod(__mode(mode, File.lstat(p).mode), p) unless File.symlink?(p) }
    end
    paths
  end

  #: (String | Integer | nil, String | Integer | nil) -> String
  def self.__owner_s(user, group) = group ? "#{user}:#{group}" : (user ? user.to_s : ":")

  #: (String | Integer | nil, String | Integer | nil, String | Pathname | Array[String | Pathname], ?verbose: bool?, ?noop: bool?) -> Array[String]?
  def self.chown(user, group, list, verbose: false, noop: false)
    paths = __paths(list)
    __say "chown #{__owner_s(user, group)} #{paths.join(" ")}" if verbose
    return nil if noop

    paths.each { |p| File.chown(__uid(user), __gid(group), p) }
    paths
  end

  #: (String | Integer | nil, String | Integer | nil, String | Pathname | Array[String | Pathname], ?verbose: bool?, ?noop: bool?, ?force: bool?) -> Array[String]?
  def self.chown_R(user, group, list, verbose: false, noop: false, force: false)
    paths = __paths(list)
    __say "chown -R#{force ? "f" : ""} #{__owner_s(user, group)} #{paths.join(" ")}" if verbose
    return nil if noop

    paths.each { |root| __walk(root).each { |p| File.chown(__uid(user), __gid(group), p) } }
    paths
  end

  # A hard link; force replaces an existing dest.
  #: (String | Pathname | Array[String | Pathname], String | Pathname, ?force: bool?, ?verbose: bool?, ?noop: bool?) -> (Integer | Array[String | Pathname])?
  def self.ln(src, dest, force: false, verbose: false, noop: false)
    srcs = __paths(src)
    d = dest.to_s
    __say "ln#{force ? " -f" : ""} #{(srcs + [d]).join(" ")}" if verbose
    return nil if noop

    srcs.each do |s|
      t = File.directory?(d) ? File.join(d, File.basename(s)) : d
      File.delete(t) if force && (File.symlink?(t) || File.exist?(t))
      File.link(s, t)
    end
    src.is_a?(Array) ? src : 0
  end

  #: (String | Pathname | Array[String | Pathname], String | Pathname, ?force: bool?, ?verbose: bool?, ?noop: bool?) -> (Integer | Array[String | Pathname])?
  def self.link(src, dest, force: false, verbose: false, noop: false) = ln(src, dest, force: force, verbose: verbose, noop: noop)

  # Copies src to dest with mode (an Integer or symbolic), as install(1).
  #: (String | Pathname, String | Pathname, ?mode: (Integer | String)?, ?verbose: bool?, ?noop: bool?) -> nil
  def self.install(src, dest, mode: nil, verbose: false, noop: false)
    s = src.to_s
    d = dest.to_s
    __say "install -c#{mode ? " -m #{__mode_to_s(mode)}" : ""} #{s} #{d}" if verbose
    return nil if noop

    d = File.join(d, File.basename(s)) if File.directory?(d)
    File.delete(d) if File.exist?(d)
    __cp_file(s, d, false)
    File.chmod(__mode(mode, File.lstat(d).mode), d) if mode
    nil
  end

  #: () -> String
  def self.pwd = Dir.pwd

  #: () -> String
  def self.getwd = Dir.pwd

  #: [T] (String | Pathname, ?verbose: bool?) { (String) -> T } -> T
  def self.cd(dir, verbose: nil)
    d = dir.to_s
    __say "cd #{d}" if verbose
    r = Dir.chdir(d) { |x| yield x }
    __say "cd -" if verbose
    r
  end

  #: (String | Pathname, ?verbose: bool?) -> Integer
  def self.__cd_enum(dir, verbose: nil)
    d = dir.to_s
    __say "cd #{d}" if verbose
    Dir.chdir(d)
  end

  #: [T] (String | Pathname, ?verbose: bool?) { (String) -> T } -> T
  def self.chdir(dir, verbose: nil) = cd(dir, verbose: verbose) { |x| yield x }

  #: (String | Pathname, ?verbose: bool?) -> Integer
  def self.__chdir_enum(dir, verbose: nil) = __cd_enum(dir, verbose: verbose)

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
