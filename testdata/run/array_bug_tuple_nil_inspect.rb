# skip: inspect of a tuple with a nil member panics (Tuple2.Inspect calls rbInspect on a typed nil)

# rbs_inline: enabled
#: (Array[Integer]) -> [Integer?, Integer?]
def bounds(nums) = [nums.min, nums.max]

puts bounds([]).inspect
puts bounds([2]).inspect
