# skip: the value of an attribute assignment (x = (obj.attr = 42), a = b.attr = v) is void, a compile error; Ruby returns the assigned value

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
