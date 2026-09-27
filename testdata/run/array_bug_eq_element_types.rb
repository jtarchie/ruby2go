# rbs_inline: enabled
ints = [1, 2] #: Array[Integer]
floats = [1.0, 2.0] #: Array[Float]
u = [1, 2] #: Array[untyped]
puts ints == floats, ints == u, u == ints
nested = [[1, 2], [], [3]] #: Array[Array[Integer]]
puts nested == [[1, 2], [], [3]]
