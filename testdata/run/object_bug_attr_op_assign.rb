# skip: operator-assign through an attribute writer (obj.x += 1, self.x -= n, obj.x ||= v, a Struct member pt.x += 1) is "unsupported syntax: CallOperatorWriteNode"

# rbs_inline: enabled

class Counter
  attr_accessor :n #: Integer
  attr_accessor :label #: String?

  #: () -> void
  def initialize
    @n = 0
    @label = nil
  end

  #: (Integer) -> void
  def drop(k)
    self.n -= k
  end
end

Point = Struct.new(:x, :y) #: [Integer, Integer]

c = Counter.new
c.n += 3
c.n *= 4
c.drop(20)
puts c.n.inspect
c.label ||= "first"
c.label ||= "second"
puts c.label.inspect
pt = Point.new(1, 2)
pt.x += 10
pt.y -= 1
puts pt.inspect
