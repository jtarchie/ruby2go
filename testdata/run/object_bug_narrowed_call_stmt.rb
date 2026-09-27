# rbs_inline: enabled

class Builder
  #: () -> void
  def initialize
    @parts = [] #: Array[String]
  end

  #: (String) -> self
  def add(part)
    @parts << part
    self
  end

  #: () -> String
  def build = @parts.join("-")
end

class HtmlBuilder < Builder
end

module Plugins
  VERSION = "1.0"

  class Alpha
  end
end

class Shape
  #: (Integer) -> void
  def initialize(n)
    puts "made #{n}"
  end
end

class Square < Shape
end

#: (singleton(Square)) -> void
def make(k)
  k.new(1)
end

h = HtmlBuilder.new
h.add("a")
h.add("b").add("c")
puts h.build
Plugins.const_get(:VERSION)
Plugins.const_get(:Alpha)
make(Square)
puts "ok"
