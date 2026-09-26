# skip: a splat mixed with other arguments for a rest param (f(10, *a), f(*a, 5), f(*a, *b)) emits `x, (*a)...`, which Go rejects (go build / gofmt fails)

# rbs_inline: enabled
#: (*Integer) -> Integer
def total(*ns) = ns.reduce(0) { |acc, n| acc + n }

nums = [1, 2, 3] #: Array[Integer]
more = [4] #: Array[Integer]
puts total(10, *nums)
puts total(*nums, 5)
puts total(*nums, *more)
