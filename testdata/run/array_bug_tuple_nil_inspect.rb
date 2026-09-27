# rbs_inline: enabled
#: (Array[Integer]) -> [Integer?, Integer?]
def bounds(nums) = [nums.min, nums.max]

puts bounds([]).inspect
puts bounds([2]).inspect
