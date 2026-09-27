# rbs_inline: enabled

Point = Data.define(:x, :y) #: [Integer, Integer]
h = {} #: Hash[Point, String]
h[Point.new(x: 1, y: 2)] = "p"
puts h[Point.new(x: 1, y: 2)].inspect
h[Point.new(x: 1, y: 2)] = "q"
puts h.size
Pair = Struct.new(:a, :b) #: [String, Integer]
s = {} #: Hash[Pair, Integer]
s[Pair.new("k", 1)] = 1
puts s.key?(Pair.new("k", 1)).inspect, [Pair.new("k", 1), Pair.new("k", 1)].tally.size
