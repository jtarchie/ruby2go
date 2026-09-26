# rbs_inline: enabled

# Decision 3: every overloadable operator gets its own Go name, distinct from the camel-cased names of ordinary methods.
class Num
  attr_reader :v #: Integer

  #: (Integer) -> void
  def initialize(v)
    @v = v
  end

  #: (Num) -> Num
  def +(o) = Num.new(v + o.v)
  #: (Num) -> Num
  def -(o) = Num.new(v - o.v)
  #: (Num) -> Num
  def *(o) = Num.new(v * o.v)
  #: (Num) -> Num
  def /(o) = Num.new(v / o.v)
  #: (Num) -> Num
  def %(o) = Num.new(v % o.v)
  #: (Integer) -> Num
  def **(e) = Num.new(v ** e)
  #: () -> Num
  def -@ = Num.new(-v)
  #: () -> Num
  def +@ = Num.new(v.abs)
  #: () -> Num
  def ~ = Num.new(-v - 1)
  #: (Integer) -> Num
  def <<(k) = Num.new(v * (2 ** k))
  #: (Integer) -> Num
  def >>(k) = Num.new(v / (2 ** k))
  #: (Num) -> Num
  def &(o) = Num.new(v < o.v ? v : o.v)
  #: (Num) -> Num
  def |(o) = Num.new(v > o.v ? v : o.v)
  #: (Num) -> Num
  def ^(o) = Num.new((v - o.v).abs)
  #: (untyped) -> bool
  def ==(o) = o.is_a?(Num) && o.v == v
  #: (Num) -> bool
  def !=(o) = v != o.v
  #: (Num) -> Integer
  def <=>(o) = v <=> o.v
  #: (Num) -> bool
  def <(o) = v < o.v
  #: (Num) -> bool
  def <=(o) = v <= o.v
  #: (Num) -> bool
  def >(o) = v > o.v
  #: (Num) -> bool
  def >=(o) = v >= o.v
  #: (Integer) -> bool
  def ===(x) = x == v
  #: (Integer) -> bool
  def =~(x) = x % v == 0
  #: (Integer) -> bool
  def !~(x) = x % v != 0
  #: (Integer) -> Integer
  def [](i) = v * i
  #: (Integer, Integer) -> Integer
  def []=(i, x)
    puts "[]= #{v + i + x}"
    x
  end
  #: () -> bool
  def ! = v == 0
  #: (Integer) -> Integer
  def index(i) = v * 1000 + i
  #: (Integer) -> bool
  def match(x) = x == v
  #: () -> String
  def to_s = "Num(#{v})"
  # Non-operator names: ? -> Q, ! -> Bang, = -> Set, leading underscores kept.
  #: (Integer) -> Integer
  def v=(x)
    @v = x
    x
  end
  #: () -> Num
  def reset!
    @v = 0
    self
  end
  #: () -> bool
  def big? = v > 10
  #: () -> Integer
  def __raw = v
  #: () -> Integer
  def _half = v / 2
end

a = Num.new(7)
b = Num.new(-2)
puts (a + b).to_s, (a - b).to_s, (a * b).to_s, (a / b).to_s, (a % b).to_s, (a ** 2).to_s
puts (-a).to_s, (+b).to_s, (~a).to_s, (a << 3).to_s, (a >> 1).to_s
puts (a & b).to_s, (a | b).to_s, (a ^ b).to_s
puts (a == Num.new(7)).inspect, (a == b).inspect, (a == 7).inspect
puts (a <=> b).inspect, (b <=> a).inspect, (a <=> Num.new(7)).inspect
puts (a < b).inspect, (a <= a).inspect, (a > b).inspect, (b >= a).inspect
puts (a === 7).inspect, (a === 8).inspect, (a =~ 21).inspect, (a =~ 22).inspect, (a !~ 21).inspect, (a !~ 22).inspect
puts a[3].inspect, a.index(3).inspect, a.match(7).inspect, a.match(8).inspect
a[1] = 5
puts (!a).inspect, (!Num.new(0)).inspect
puts (a != Num.new(7)).inspect, (a != b).inspect

puts "-- names that are not operators"
c = Num.new(12)
puts c.big?.inspect, c.__raw.inspect, c._half.inspect
c.v = 3
puts c.v.inspect, c.big?.inspect, c.reset!.v.inspect

puts "-- operators called by name"
puts a.+(b).to_s, 1.+(2).inspect, 7.send(:-, 2).inspect, 2.5.public_send(:*, 2.0).inspect
puts 3.send(:<=>, 4).inspect, 6.send(:%, 4).inspect, 2.send(:**, 5).inspect, 1.send(:==, 1.0).inspect, a.send(:<, b).inspect
puts [1, -2, 3].map(&:-@).inspect, [1.5, -2.5].map(&:abs).inspect, [3, 4].map(&:to_s).inspect
