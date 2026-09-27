# rbs_inline: enabled

class Box
  attr_accessor :n #: Integer

  #: () -> void
  def initialize
    @n = 0
  end
end

b = Box.new
r = (b.n = 42)
puts r.inspect, b.n.inspect
c = Box.new
x = c.n = 7
puts x.inspect
