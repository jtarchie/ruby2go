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

#: (Integer) -> Vec
def vec(n) = Vec.new(n)

vs = [vec(1), vec(2)] #: Array[Vec]
puts vs.include?(vec(1)).inspect, vs.include?(vec(3)).inspect
puts (vs == [vec(1), vec(2)]).inspect, (vs == [vec(2), vec(1)]).inspect
h = { a: vec(1) } #: Hash[Symbol, Vec]
puts (h == { a: vec(1) }).inspect
