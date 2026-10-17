# Homebrew's Utils::Ruby.require? and Commands.valid_internal_cmd?: a require of a name built at run time (decision 246).
require "rbconfig"
require "pathname"

# Homebrew's standalone/init.rb: a top-level constant reading a top-level local (decision 296).
traced_dir = __dir__ || raise("__dir__ is not defined")
TRACED_LIB = File.basename(traced_dir).freeze

module TracedReq
  def self.require?(path)
    Kernel.require path.to_s
    true
  rescue LoadError
    false
  end

  def self.rel(name) = require(name)

  # Utils.popen: the spawn branch runs because the environment asks for it (main.rb's `# env:`); both branches compile
  # (decision 288).
  def self.popen_mode
    if ENV["TRACED_SPAWN"] == "1"
      "spawn"
    else
      "fork-only branch"
    end
  end

  # Homebrew's API.verify_jws_signature: a library required inside a method, after MRI's traced load (decision 281).
  def self.late_lib
    require "openssl"
    OpenSSL::PKey::RSA.name
  end

  # Homebrew's FastBootRequire: a library by its path under MRI's rubylibdir (decision 296).
  def self.std_lib = require(File.join(RbConfig::CONFIG.fetch("rubylibdir"), "pathname.rb"))

  # Homebrew's `from_archdir("portable_ruby_gems")`: MRI's own file, compiled in as MRI loaded it at boot.
  def self.arch_lib = require(File.join(RbConfig::CONFIG.fetch("archdir"), "rbconfig.rb"))
end
