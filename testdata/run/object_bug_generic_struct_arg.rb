# skip: a Klass.new value or class constant passed straight to a generic method (then, reduce's seed, include?) has Go type *Vec / *Vec_Meta while the element or block type is VecI / Vec_MetaI, so Go infers the wrong type parameter and go build fails

# rbs_inline: enabled

class Vec
  attr_reader :v #: Integer

  #: (Integer) -> void
  def initialize(v)
    @v = v
  end

  #: (Vec) -> Vec
  def plus(o) = Vec.new(v + o.v)
end

class Vec3 < Vec
end

vs = [Vec.new(1), Vec.new(-2), Vec.new(5)] #: Array[Vec]
puts Vec.new(4).then { |b| b.v * 2 }.inspect
puts Vec.new(1).then { |b| "v=#{b.v}" }
puts vs.reduce(Vec.new(0)) { |a, b| a.plus(b) }.v.inspect
puts vs.include?(Vec.new(5)).inspect
klasses = [Vec, Vec3] #: Array[singleton(Vec)]
puts klasses.include?(Vec3).inspect
