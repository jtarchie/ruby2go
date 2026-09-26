# skip: join does not flatten nested arrays; MRI joins [3, [1, [2]]] as "3,1,2"

# rbs_inline: enabled
a = [] #: Array[untyped]
a << 3
a << [1, [2]]
puts a.join(",")
grid = [[1, 2], [3]] #: Array[Array[Integer]]
puts grid.join("-")
