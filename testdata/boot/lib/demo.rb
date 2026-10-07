module Demo
  autoload :Lazy, "demo/lazy"

  class Base
    def base = :base
  end

  class Thing < Base
    attr_reader :name #: String

    #: (String) -> void
    def initialize(name)
      @name = name
    end

    #: () -> String
    def greet
      "hello #{@name}"
    end
  end
end
