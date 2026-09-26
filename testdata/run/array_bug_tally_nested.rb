# skip: tally/group_by keyed by arrays count equal arrays separately: Hash keys are Go map keys, so *Array keys compare by pointer (same root as hash_bug_array_keys)

# rbs_inline: enabled
grid = [[1], [1], [2]] #: Array[Array[Integer]]
puts grid.tally.inspect
puts grid.group_by { |r| r }.size
