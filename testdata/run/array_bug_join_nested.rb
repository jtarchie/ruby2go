
# rbs_inline: enabled
a = [] #: Array[untyped]
a << 3
a << [1, [2]]
puts a.join(",")
grid = [[1, 2], [3]] #: Array[Array[Integer]]
puts grid.join("-")
