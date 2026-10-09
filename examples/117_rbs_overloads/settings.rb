class Settings
  PATH = "PATH_INFO"
  PORT = "SERVER_PORT"

  def initialize(vars)
    @vars = vars
  end

  def [](key)
    @vars[key]
  end

  def double(x)
    x * 2
  end

  def greet(name = "world", punct = "!")
    "hello #{name}#{punct}"
  end
end
