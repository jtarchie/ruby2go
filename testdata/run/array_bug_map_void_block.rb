# skip: map with a block whose value is nil (last expression puts) emits a closure with no result, which does not match func(E) U (go build fails); MRI returns [nil, nil]

# rbs_inline: enabled
nums = [1, 2] #: Array[Integer]
puts nums.map { |n| puts n }.inspect
