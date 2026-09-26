# skip: a Struct/Data member named `other` breaks the generated ==, whose parameter is also named other (`other == other.other`), so equal values compare false

# rbs_inline: enabled

Edge = Struct.new(:from, :other) #: [String, String]
Link = Data.define(:other) #: [Integer]

puts (Edge.new("a", "b") == Edge.new("a", "b")).inspect, (Edge.new("a", "b") == Edge.new("a", "c")).inspect
puts (Link.new(1) == Link.new(1)).inspect, (Link.new(1) == Link.new(2)).inspect
