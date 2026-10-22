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

  #: (String | Pathname | Array[String | Pathname], ?mode: Integer?, ?noop: bool?, ?verbose: bool?) -> nil
  def self.mkdir_p(list, mode: nil, noop: nil, verbose: nil)
    paths = __paths(list)
    __say "mkdir -p #{__mflag(mode)}#{paths.join(" ")}" if verbose
    return nil if noop

    paths.each { |p| __mkdir_p1(p, mode) }
    nil
  end

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

  #: (String | Pathname | Array[String | Pathname], ?noop: bool?, ?verbose: bool?, ?secure: bool?) -> nil
  def self.rm_rf(list, noop: nil, verbose: nil, secure: nil) = rm_r(list, force: true, noop: noop, verbose: verbose)

  #: (String) -> nil
  def self.__rm_rf1(path) = %x{ _ = os.RemoveAll(string(path)) }

  # force swallows any error, as MRI's remove_file.
  #: (String | Pathname | Array[String | Pathname], ?noop: bool?, ?verbose: bool?) -> nil
  def self.rm_f(list, noop: nil, verbose: nil) = rm(list, force: true, noop: noop, verbose: verbose)

  # File.delete already raises MRI's exact apply2files message (decision 62).
  #: (String | Pathname | Array[String | Pathname], ?force: bool?, ?noop: bool?, ?verbose: bool?) -> nil
  def self.rm(list, force: nil, noop: nil, verbose: nil)
    paths = __paths(list)
    __say "rm#{force ? " -f" : ""} #{paths.join(" ")}" if verbose
    return nil if noop

    paths.each { |p| __rm1(p, force) }
    nil
  end

  #: (String, bool?) -> nil
  def self.__rm1(path, force)
    File.delete(path)
    nil
  rescue StandardError
    raise unless force
    nil
  end

  # src is a path or a list of them; into a directory dest, each keeps its basename.
  #: (String | Pathname | Array[String | Pathname], String | Pathname, ?preserve: bool?, ?noop: bool?, ?verbose: bool?) -> nil
  def self.cp(src, dest, preserve: nil, noop: nil, verbose: nil)
    srcs = __paths(src)
    d = dest.to_s
    __say "cp#{preserve ? " -p" : ""} #{(srcs + [d]).join(" ")}" if verbose
    return nil if noop

    srcs.each { |s| __cp_file(s, File.directory?(d) ? File.join(d, File.basename(s)) : d, preserve) }
    nil
  end

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

  #: (String | Pathname | Array[String | Pathname], String | Pathname, ?preserve: bool?, ?noop: bool?, ?verbose: bool?) -> nil
  def self.cp_r(src, dest, preserve: nil, noop: nil, verbose: nil)
    srcs = __paths(src)
    d = dest.to_s
    __say "cp -r#{preserve ? "p" : ""} #{(srcs + [d]).join(" ")}" if verbose
    return nil if noop

    srcs.each do |s|
      t = File.directory?(d) ? File.join(d, File.basename(s)) : d
      if File.directory?(s)
        __cp_tree(s, t, preserve)
      else
        __cp_file(s, t, preserve)
      end
    end
    nil
  end

  #: (String, String, bool?) -> nil
  def self.__cp_tree(src, dest, preserve) = %x{
    if err := rbCopyTree(string(src), string(dest), preserve != nil && bool(*preserve)); err != nil {
      panic(rbFileErr(err, "rb_sysopen", string(src)))
    }
  }

  #: (String | Pathname | Array[String | Pathname], String | Pathname, ?force: bool?, ?noop: bool?, ?verbose: bool?, ?secure: bool?) -> nil
  def self.mv(src, dest, force: nil, noop: nil, verbose: nil, secure: nil)
    srcs = __paths(src)
    d = dest.to_s
    __say "mv#{force ? " -f" : ""} #{(srcs + [d]).join(" ")}" if verbose
    return nil if noop

    srcs.each { |s| __mv_entry(s, File.directory?(d) ? File.join(d, File.basename(s)) : d) }
    nil
  end

  #: (String, String) -> nil
  def self.__mv_entry(src, dest) = %x{
    if err := rbMove(string(src), string(dest)); err != nil {
      panic(rbFileErr(err, "rb_file_s_rename", "("+string(src)+", "+string(dest)+")"))
    }
  }

  # One method, not class twins: a dynamic call's wrapper cannot pass a twin its keywords.
  #: (String | Pathname | Array[String | Pathname], ?mtime: Time?, ?nocreate: bool, ?noop: bool?, ?verbose: bool?) -> nil
  def self.touch(list, mtime: nil, nocreate: false, noop: nil, verbose: nil)
    paths = __paths(list)
    __say "touch #{nocreate ? "-c " : ""}#{mtime ? mtime.strftime("-t %Y%m%d%H%M.%S ") : ""}#{paths.join(" ")}" if verbose
    return nil if noop

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
  #: (String | Pathname | Array[String | Pathname], String | Pathname, ?force: bool?, ?noop: bool?, ?verbose: bool?) -> nil
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
    nil
  end

  #: (String | Pathname | Array[String | Pathname], String | Pathname, ?noop: bool?, ?verbose: bool?) -> nil
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
  #: (String | Pathname | Array[String | Pathname], ?force: bool?, ?secure: bool?, ?verbose: bool?, ?noop: bool?) -> nil
  def self.rm_r(list, force: false, secure: false, verbose: false, noop: false)
    paths = __paths(list)
    __say "rm -r#{force ? "f" : ""} #{paths.join(" ")}" if verbose
    return nil if noop

    paths.each do |p|
      unless File.symlink?(p) || File.exist?(p)
        raise Errno::ENOENT, "No such file or directory @ rb_file_s_lstat - #{p}" unless force

        next
      end
      __rm_rf1(p)
    end
    nil
  end

  #: (String | Pathname | Array[String | Pathname], ?verbose: bool?, ?noop: bool?) -> nil
  def self.remove_entry(list, verbose: false, noop: false) = rm_r(list, noop: noop)

  #: (String | Pathname, ?bool) -> nil
  def self.remove_entry_secure(path, force = false) = rm_r(path, force: force)

  #: (String | Pathname | Array[String | Pathname], ?mode: Integer?, ?verbose: bool?, ?noop: bool?) -> nil
  def self.mkdir(list, mode: nil, verbose: false, noop: false)
    paths = __paths(list)
    __say "mkdir #{__mflag(mode)}#{paths.join(" ")}" if verbose
    return nil if noop

    paths.each { |p| __mkdir1(p, mode) }
    nil
  end

  #: (String | Pathname | Array[String | Pathname], ?parents: bool?, ?verbose: bool?, ?noop: bool?) -> nil
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
    nil
  end

  #: (String | Pathname | Array[String | Pathname], ?mode: Integer?, ?noop: bool?, ?verbose: bool?) -> nil
  def self.mkpath(list, mode: nil, noop: nil, verbose: nil) = mkdir_p(list, mode: mode, noop: noop, verbose: verbose)

  # mode is an Integer or a symbolic "u+w,go-x" / "a=r".
  #: (Integer | String, String | Pathname | Array[String | Pathname], ?verbose: bool?, ?noop: bool?) -> nil
  def self.chmod(mode, list, verbose: false, noop: false)
    paths = __paths(list)
    __say "chmod #{__mode_to_s(mode)} #{paths.join(" ")}" if verbose
    return nil if noop

    paths.each { |p| File.chmod(__mode(mode, File.lstat(p).mode), p) }
    nil
  end

  #: (Integer | String, String | Pathname | Array[String | Pathname], ?verbose: bool?, ?noop: bool?, ?force: bool?) -> nil
  def self.chmod_R(mode, list, verbose: false, noop: false, force: false)
    paths = __paths(list)
    __say "chmod -R#{force ? "f" : ""} #{__mode_to_s(mode)} #{paths.join(" ")}" if verbose
    return nil if noop

    paths.each do |root|
      __walk(root).each { |p| File.chmod(__mode(mode, File.lstat(p).mode), p) unless File.symlink?(p) }
    end
    nil
  end

  #: (String | Integer | nil, String | Integer | nil) -> String
  def self.__owner_s(user, group) = group ? "#{user}:#{group}" : (user ? user.to_s : ":")

  #: (String | Integer | nil, String | Integer | nil, String | Pathname | Array[String | Pathname], ?verbose: bool?, ?noop: bool?) -> nil
  def self.chown(user, group, list, verbose: false, noop: false)
    paths = __paths(list)
    __say "chown #{__owner_s(user, group)} #{paths.join(" ")}" if verbose
    return nil if noop

    paths.each { |p| File.chown(__uid(user), __gid(group), p) }
    nil
  end

  #: (String | Integer | nil, String | Integer | nil, String | Pathname | Array[String | Pathname], ?verbose: bool?, ?noop: bool?, ?force: bool?) -> nil
  def self.chown_R(user, group, list, verbose: false, noop: false, force: false)
    paths = __paths(list)
    __say "chown -R#{force ? "f" : ""} #{__owner_s(user, group)} #{paths.join(" ")}" if verbose
    return nil if noop

    paths.each { |root| __walk(root).each { |p| File.chown(__uid(user), __gid(group), p) } }
    nil
  end

  # A hard link; force replaces an existing dest.
  #: (String | Pathname, String | Pathname, ?force: bool?, ?verbose: bool?, ?noop: bool?) -> nil
  def self.ln(src, dest, force: false, verbose: false, noop: false)
    s = src.to_s
    d = dest.to_s
    __say "ln#{force ? " -f" : ""} #{s} #{d}" if verbose
    return nil if noop

    d = File.join(d, File.basename(s)) if File.directory?(d)
    File.delete(d) if force && (File.symlink?(d) || File.exist?(d))
    File.link(s, d)
    nil
  end

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
