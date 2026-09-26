# skip: Array#uniq dedupes through a Go map keyed by the element, so equal inner arrays, Structs and Data (pointers) are all kept; MRI uses eql?/hash (by value)

# rbs_inline: enabled
Pair = Struct.new(:a, :b) #: [Integer, Integer]
Val = Data.define(:v) #: [Integer]
grid = [[1], [1], [2]] #: Array[Array[Integer]]
puts grid.uniq.inspect
ps = [Pair.new(1, 2), Pair.new(1, 2)] #: Array[Pair]
puts ps.uniq.size
vs = [Val.new(1), Val.new(1), Val.new(2)] #: Array[Val]
puts vs.uniq.size
