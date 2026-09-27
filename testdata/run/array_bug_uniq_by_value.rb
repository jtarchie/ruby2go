# rbs_inline: enabled
Pair = Struct.new(:a, :b) #: [Integer, Integer]
Val = Data.define(:v) #: [Integer]
grid = [[1], [1], [2]] #: Array[Array[Integer]]
puts grid.uniq.inspect
ps = [Pair.new(1, 2), Pair.new(1, 2)] #: Array[Pair]
puts ps.uniq.size
vs = [Val.new(1), Val.new(1), Val.new(2)] #: Array[Val]
puts vs.uniq.size
