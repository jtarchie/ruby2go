# skip: String#inspect turns an invalid UTF-8 byte into U+FFFD where MRI writes \xFF

# rbs_inline: enabled

puts "\xff".inspect
puts "a\xe1b".inspect
