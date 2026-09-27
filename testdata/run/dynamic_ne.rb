# rbs_inline: enabled
class Vec
  attr_reader :x #: Integer

  #: (Integer) -> void
  def initialize(x)
    @x = x
  end

  #: (untyped) -> bool
  def ==(o) = o.is_a?(Vec) && x == o.x
end

m = [:!=, :==][0] #: Symbol?
a = [1, 2] #: Array[Integer]
v = Vec.new(1) #: untyped
puts a.send(m || :x, [1, 2]).inspect
puts v.send(m || :x, Vec.new(1)).inspect
puts (v != Vec.new(1)).inspect
w = Vec.new(1)
puts (w != v).inspect
p1 = [1, "a"] #: [Integer, String]
puts (p1 != [1, "a"]).inspect
o = nil #: Integer?
o = 3 if a.size > 1
puts (o != 3).inspect, (o != nil).inspect
