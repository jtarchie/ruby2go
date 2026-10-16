# Homebrew's Utils::Ruby.require? and Commands.valid_internal_cmd?: a require of a name built at run time (decision 246).
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
end
