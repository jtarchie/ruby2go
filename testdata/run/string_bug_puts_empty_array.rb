# skip: puts [] prints an empty line; Ruby 4 prints nothing for an empty array

# rbs_inline: enabled

puts "a"
puts []
puts "b"
puts [[], []]
puts "c"
