# skip: a local first assigned Num.new is declared as the concrete *Num, so `c += x` / `c = c + x` with a user + returning Num (NumI) fails go build, even with `#: Num`
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
