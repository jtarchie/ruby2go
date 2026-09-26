# skip: a splat whose array element type differs from the rest param's (Array[Integer] into *untyped, Array[untyped] into *Integer) spreads the slice unconverted (go build fails)

# rbs_inline: enabled
#: (*Integer) -> Integer
def total(*ns) = ns.reduce(0) { |acc, n| acc + n }

nums = [1, 2, 3] #: Array[Integer]
puts(*nums)
u = [] #: Array[untyped]
u << 4
u << 5
puts total(*u)
