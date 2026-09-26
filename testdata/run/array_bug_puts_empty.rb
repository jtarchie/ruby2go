# skip: puts [] prints a newline; MRI prints nothing for an empty array (also nested: puts [[], [1]])

# rbs_inline: enabled
empty = [] #: Array[Integer]
puts empty
puts [[], [1]]
puts "end"
