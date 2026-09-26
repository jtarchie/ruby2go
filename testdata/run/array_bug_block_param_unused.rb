# skip: a block param that is unused in one block but read in another block of the same method gets no `_ = n` (noteUnused counts reads per name, method-wide), so go build fails "declared and not used" (iterator blocks and tuple destructuring; same root as hash_bug_unused_block_param)

# rbs_inline: enabled
nums = [1, 2] #: Array[Integer]
nums.each { |n| puts n }
nums.each_with_index { |n, i| puts i }
pairs = [[1, "a"]] #: Array[[Integer, String]]
pairs.each { |k, s| puts k, s }
puts pairs.map { |k, s| k }.inspect
