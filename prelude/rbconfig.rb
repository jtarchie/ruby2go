# rbs_inline: enabled

# RbConfig: what MRI was built for, as this binary's platform (Go's GOOS/GOARCH) answers it.
module RbConfig
  #: () -> String
  def self.__os = %x{ return String(runtime.GOOS) }

  #: () -> String
  def self.__arch = %x{
    if runtime.GOARCH == "arm64" && runtime.GOOS == "linux" {
      return String("aarch64")
    }
    return String(map[string]string{"amd64": "x86_64", "386": "i386"}[runtime.GOARCH] + map[bool]string{true: "", false: runtime.GOARCH}[runtime.GOARCH == "amd64" || runtime.GOARCH == "386"])
  }

  #: () -> String
  def self.ruby = %x{
    exe, err := os.Executable()
    if err != nil {
      return String("")
    }
    return String(exe)
  }

  # MRI's library dirs: a path under rubylibdir names a library compiled in, which a computed require answers (decision 296).
  #: () -> String
  def self.__libdir = %x{ return String(rbLibDir) }

  CONFIG = {
    "host_os" => __os,
    "target_os" => __os,
    "arch" => "#{__arch}-#{__os}",
    "host_cpu" => __arch,
    "ruby_version" => "4.0.0",
    "MAJOR" => "4",
    "MINOR" => "0",
    "TEENY" => "0",
    "ENABLE_SHARED" => "no",
    "ruby_install_name" => "ruby",
    "EXEEXT" => "",
    "prefix" => "",
    "bindir" => "",
    "sitedir" => "",
    "rubylibdir" => __libdir,
    "archdir" => "#{__libdir}/#{__arch}-#{__os}",
  } #: Hash[String, String]
end
