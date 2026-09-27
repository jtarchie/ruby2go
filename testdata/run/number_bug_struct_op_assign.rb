# rbs_inline: enabled

class Num
  attr_reader :v #: Integer

  #: (Integer) -> void
  def initialize(v)
    @v = v
  end

  #: (Num) -> Num
  def +(o) = Num.new(v + o.v)
end

c = Num.new(1) #: Num
c += Num.new(2)
c += Num.new(3)
puts c.v.inspect
d = Num.new(10)
d = d + Num.new(5)
puts d.v.inspect
