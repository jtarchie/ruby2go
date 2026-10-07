module Demo
  autoload :Lazy, "demo/lazy"

  VERSION = "1.0"

  @registry = Mutex.new
  @cache = Hash.new { |h, k| h[k] = k }

  class Base
    def base
      :base
    end
  end

  class Thing < Base
    attr_reader :name

    def initialize(name)
      @name = name
      @mutex = Mutex.new
    end

    define_method(:shout) do
      @name.upcase
    end
  end
end
