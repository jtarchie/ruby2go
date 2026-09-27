# rbs_inline: enabled
grid = [[1], [1], [2]] #: Array[Array[Integer]]
puts grid.tally.inspect
puts grid.group_by { |r| r }.size
