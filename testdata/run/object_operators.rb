# rbs_inline: enabled

# Operator methods, a custom ==, a hand-written setter, predicate names and inspect overrides on a user class.

class Vec
  attr_reader :x #: Integer
  attr_reader :y #: Integer

  #: (Integer, Integer) -> void
  def initialize(x, y)
    @x = x
    @y = y
  end

  #: (Vec) -> Vec
  def +(o) = Vec.new(x + o.x, y + o.y)

  #: (Vec) -> Vec
  def -(o) = Vec.new(x - o.x, y - o.y)

  #: (Integer) -> Vec
  def *(k) = Vec.new(x * k, y * k)

  #: () -> Vec
  def -@ = Vec.new(-x, -y)

  #: (Vec) -> bool
  def ==(o) = x == o.x && y == o.y

  #: (Integer) -> Integer
  def [](i) = i == 0 ? x : y

  #: (Integer, Integer) -> void
  def []=(i, v)
    if i == 0
      @x = v
    else
      @y = v
    end
  end

  #: () -> String
  def to_s = "(#{x}, #{y})"

  #: () -> String
  def inspect = "#<Vec #{x},#{y}>"

  #: () -> bool
  def zero? = x == 0 && y == 0

  #: () -> Vec
  def negate! = self * -1

  #: (Integer) -> void
  def x=(v)
    @x = v
  end
end

a = Vec.new(1, 2)
b = Vec.new(3, -4)
puts a + b, a - b, a * 3, -a, a * 0, a + b - b
puts (a == Vec.new(1, 2)).inspect, (a == b).inspect
puts a[0].inspect, a[1].inspect, a.zero?.inspect, Vec.new(0, 0).zero?.inspect, a.negate!
puts [a, b].inspect, a.inspect, "#{a}", [[a]].inspect
a.x = 10
a[1] = -7
puts a, a.inspect
list = [a, b] #: Array[Vec]
puts list.map { |v| -v }.inspect, list.reduce(list.fetch(0)) { |acc, v| acc + v }

# << returning self chains; unary +@ and ~; an ivar mutated inside a block.
class Bag
  #: () -> void
  def initialize
    @items = [] #: Array[Integer]
    @total = 0
  end

  #: (Integer) -> self
  def <<(x)
    @items << x
    self
  end

  #: () -> Integer
  def sum
    @total = 0
    @items.each { |i| @total += i }
    @total
  end

  #: () -> Integer
  def +@ = @items.size

  #: () -> Integer
  def ~ = -@items.size

  #: () -> Array[Integer]
  def evens = @items.select { |i| i.even? && i > @total / 100 }
end

bag = Bag.new
bag << 1 << 2 << 3
bag << 4
puts bag.sum.inspect, (+bag).inspect, (~bag).inspect, bag.evens.inspect

# The default to_s/inspect name the class (MRI appends an address, so only the prefix is compared);
# inspect does not call a user's to_s.
class Plain
end

class Shown
  #: () -> String
  def to_s = "shown!"
end

module Deep
  class Thing
    #: () -> void
    def initialize
      @a = 1
    end
  end
end

pl = Plain.new
sh = Shown.new
th = Deep::Thing.new
puts pl.inspect.start_with?("#<Plain"), pl.to_s.start_with?("#<Plain"), sh.inspect.include?("shown!"), sh.inspect.start_with?("#<Shown")
puts th.to_s.start_with?("#<Deep::Thing"), [pl].inspect.start_with?("[#<Plain"), "#{pl}".start_with?("#<Plain"), "#{sh}"

# == taking untyped and checking the class first.
class Coin
  attr_reader :cents #: Integer

  #: (Integer) -> void
  def initialize(cents)
    @cents = cents
  end

  #: (untyped) -> bool
  def ==(other) = other.is_a?(Coin) && cents == other.cents
end

c1 = Coin.new(5)
puts (c1 == Coin.new(5)).inspect, (c1 == Coin.new(6)).inspect, (c1 == 5).inspect, (c1 == "5").inspect, (c1 == nil).inspect
#: (Integer) -> Coin
def coin(n) = Coin.new(n)

coins = [c1, coin(1)] #: Array[Coin]
puts coins.include?(coin(1)).inspect, coins.include?(coin(2)).inspect, coins.uniq.size.inspect, [c1, coin(5)].uniq.size.inspect
puts list.fetch(1).then { |v| v.x * 2 }.inspect, coin(3).then { |c| "c=#{c.cents}" }, coins.reduce(coin(0)) { |a, c| coin(a.cents + c.cents) }.cents.inspect
