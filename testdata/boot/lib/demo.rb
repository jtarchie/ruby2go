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

    # Nothing calls it, so it is never typed (decision 169): an &block with no signature would not compile.
    def never_called(&blk) = blk
  end

  # Nothing names it, so none of its methods is compiled (decision 169).
  class Unused
    def initialize(x) = @x = x.frobnicate(&nil)

    def broken(&blk) = blk
  end
end
