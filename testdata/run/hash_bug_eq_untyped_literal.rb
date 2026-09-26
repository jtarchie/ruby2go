# skip: Hash#== type-asserts other to the receiver's own Go Hash[K, V], so hashes with equal contents but different element types (`typed == {}`, typed vs untyped, Integer vs Float values) compare false

# rbs_inline: enabled

e = {} #: Hash[String, Integer]
puts (e == {}).inspect
t = { "a" => 1 } #: Hash[String, Integer]
u = {}
u["a"] = 1
puts (t == u).inspect, (u == t).inspect
s = { a: 1 } #: Hash[Symbol, Integer]
puts (s == { a: 1, b: "x" }.reject { |k, _v| k == :b }).inspect
# Hash[Integer, Integer] vs Hash[Integer, Float]: MRI's 1 == 1.0 is true.
puts ({ 1 => 1 } == { 1 => 1.0 }).inspect
