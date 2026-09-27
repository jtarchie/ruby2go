# rbs_inline: enabled
#: (*Integer) -> Integer
def total(*ns) = ns.reduce(0) { |acc, n| acc + n }

nums = [1, 2, 3] #: Array[Integer]
more = [4] #: Array[Integer]
puts total(10, *nums)
puts total(*nums, 5)
puts total(*nums, *more)
