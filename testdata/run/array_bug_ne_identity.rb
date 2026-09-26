# skip: Array#!= goes to BasicObject#!=, which negates identity instead of Array#==

# rbs_inline: enabled
a = [1, 2] #: Array[Integer]
b = [1, 2] #: Array[Integer]
puts a != b, a != a, a != [3], [1] != [1]
