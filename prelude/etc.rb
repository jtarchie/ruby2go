# rbs_inline: enabled

# Etc: getlogin/nprocessors/getpwuid/getpwnam/systmpdir, backed by os/user and runtime; loads only on a literal `require "etc"` (decisions 50, 155).
module Etc
  Passwd = Struct.new(:name, :passwd, :uid, :gid, :gecos, :dir, :shell, :change, :uclass, :expire) #: [String, String?, Integer, Integer, String, String, String?, Integer?, String?, Integer?]
  Group = Struct.new(:name, :passwd, :gid, :mem) #: [String, String?, Integer, Array[String]]

  #: () -> String?
  def self.getlogin = %x{
    u, err := user.Current()
    if err != nil || u.Username == "" {
      return nil
    }
    return Ref(String(u.Username))
  }

  #: () -> Integer
  def self.nprocessors = %x{ Integer(runtime.NumCPU()) }

  # uname(2)'s fields, read from uname(1) (decision 233): Go's syscall.Uname is Linux-only.
  #: () -> Hash[Symbol, String]
  def self.uname
    { sysname: __uname("-s"), nodename: __uname("-n"), release: __uname("-r"), version: __uname("-v"), machine: __uname("-m") }
  end

  #: (String) -> String
  def self.__uname(flag) = %x{
    b, _ := exec.Command("uname", string(flag)).Output()
    return String(strings.TrimSpace(string(b)))
  }

  #: () -> String
  def self.systmpdir = %x{ String(os.TempDir()) }

  #: (?Integer?) -> Passwd
  def self.getpwuid(uid = nil) = %x{
    var u *user.User
    var err error
    if !uid.ok {
      u, err = user.Current()
    } else {
      u, err = user.LookupId(strconv.Itoa(int(uid.v)))
    }
    if err != nil {
      id := strconv.Itoa(os.Getuid())
      if uid.ok {
        id = strconv.Itoa(int(uid.v))
      }
      panic(NewArgumentError(Ref(String("can't find user for " + id))))
    }
    return rbEtcPasswd(u)
  }

  #: (String) -> Passwd
  def self.getpwnam(name) = %x{
    u, err := user.Lookup(string(name))
    if err != nil {
      panic(NewArgumentError(Ref(String("can't find user for " + string(name)))))
    }
    return rbEtcPasswd(u)
  }

  # mem is empty: os/user does not list a group's members.
  #: (String) -> Group
  def self.getgrnam(name) = %x{
    g, err := user.LookupGroup(string(name))
    if err != nil {
      panic(NewArgumentError(Ref(String("can't find group for " + string(name)))))
    }
    return rbEtcGroup(g)
  }

  #: (?Integer?) -> Group
  def self.getgrgid(gid = nil) = %x{
    id := os.Getgid()
    if gid.ok {
      id = int(gid.v)
    }
    g, err := user.LookupGroupId(strconv.Itoa(id))
    if err != nil {
      panic(NewArgumentError(Ref(String("can't find group for " + strconv.Itoa(id)))))
    }
    return rbEtcGroup(g)
  }
end
