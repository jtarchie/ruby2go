# skip: wontfix: Array[Integer] is []Integer and cannot hold nil, so []= past the end pads with 0 where MRI pads with nil (README decision 7)

# rbs_inline: enabled
nums = [10, 20] #: Array[Integer]
nums[4] = 50
puts nums.inspect, nums.size
puts nums.delete(0).inspect
