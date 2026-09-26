# skip: []= past the end pads a non-nilable Array[Integer] with 0 where MRI pads with nil

# rbs_inline: enabled
nums = [10, 20] #: Array[Integer]
nums[4] = 50
puts nums.inspect, nums.size
puts nums.delete(0).inspect
