# rbs_inline: enabled
#: (*Integer) -> Integer
def total(*ns) = ns.reduce(0) { |acc, n| acc + n }

nums = [1, 2, 3] #: Array[Integer]
puts(*nums)
u = [] #: Array[untyped]
u << 4
u << 5
puts total(*u)
