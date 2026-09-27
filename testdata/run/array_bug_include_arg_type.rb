# rbs_inline: enabled
class Pt
  attr_reader :x #: Integer

  #: (Integer) -> void
  def initialize(x)
    @x = x
  end
end

a = Pt.new(1)
pts = [a, Pt.new(2)] #: Array[Pt]
puts pts.include?(a), pts.include?(Pt.new(1))
u = []
u << 3
u << "x"
puts u.include?(3).inspect, u.include?("x").inspect, u.include?("y").inspect
