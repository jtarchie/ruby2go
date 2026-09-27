# rbs_inline: enabled
nums = [1, 2] #: Array[Integer]
nums.each { |n| puts n }
nums.each_with_index { |n, i| puts i }
pairs = [[1, "a"]] #: Array[[Integer, String]]
pairs.each { |k, s| puts k, s }
puts pairs.map { |k, s| k }.inspect
