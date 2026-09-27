# rbs_inline: enabled

class Vec
  attr_reader :x #: Integer

  #: (Integer) -> void
  def initialize(x)
    @x = x
  end

  #: (Vec) -> bool
  def ==(o) = x == o.x
end

Point = Struct.new(:x, :y) #: [Integer, Integer]

a = Vec.new(1)
puts (a == Vec.new(1)).inspect, (a != Vec.new(1)).inspect, (a != Vec.new(2)).inspect
puts (Point.new(1, 2) != Point.new(1, 2)).inspect, (Point.new(1, 2) != Point.new(2, 1)).inspect
