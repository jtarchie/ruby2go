# rbs_inline: enabled

# Instance variables narrow like locals: after `@x ||= ...`, an assignment
# or a nil check, rb2go knows @x is not nil, so no extra check is needed.

class Settings
  #: (Hash[String, String]) -> void
  def initialize(env)
    @env = env
    @home = nil #: String?
  end

  # memoized: computed on first use, nil before that
  def home
    @home ||= @env.fetch("HOME", "/tmp")
    File.join(@home, ".config")
  end

  # a list built on first read; nothing assigns it in initialize
  def plugins
    unless @plugins
      @plugins = %w[core]
      @plugins << "extra" if @env.key?("EXTRA")
    end
    @plugins
  end

  def describe_home
    return "home unset" if @home.nil?
    "home is #{@home.length} chars"
  end
end

s = Settings.new({ "HOME" => "/home/ann", "EXTRA" => "1" })
puts s.describe_home
puts s.home
puts s.describe_home
p s.plugins
p s.plugins.equal?(s.plugins)
