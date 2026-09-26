# skip: Array has no <=> (Cmp), so sort/min/max/sort_by/max_by over homogeneous array values or keys panic at run time

# rbs_inline: enabled
nums = [3, 1, 2] #: Array[Integer]
puts nums.sort_by { |n| [n % 2, n] }.inspect
puts nums.max_by { |n| [n, -n] }.inspect
grid = [[2, 1], [1, 5]] #: Array[Array[Integer]]
puts grid.sort.inspect, grid.min.inspect
