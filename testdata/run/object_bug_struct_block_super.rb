# skip: a method in a Struct.new block replaces the generated one instead of overriding it, so `super` skips it: initialize's super silently does nothing (x=0, y=0) and inspect's super reaches Kernel#inspect ("#<Pair>")

# rbs_inline: enabled

Point = Struct.new(:x, :y) do
  #: (Integer, ?Integer) -> void
  def initialize(x, y = 0)
    super(x, y)
  end

  #: () -> Integer
  def sum = x + y
end #: [Integer, Integer]

Pair = Struct.new(:a, :b) do
  #: () -> String
  def inspect = "P" + super
end #: [Integer, Integer]

puts Point.new(1).inspect, Point.new(1, 2).sum.inspect
puts Pair.new(1, 2).inspect
