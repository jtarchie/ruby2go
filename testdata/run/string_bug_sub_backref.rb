# skip: sub/gsub replacement is literal; MRI expands \0, \&, \`, \', \1 and \\ in a String replacement even for a String pattern

# rbs_inline: enabled

puts "abc".sub("b", "\\0\\0").inspect
puts "a'b".gsub("'", "\\'").inspect
puts "x-y".gsub("-", "<\\&>").inspect
puts "abc".gsub("b", "\\`").inspect
puts "abc".sub("b", "[\\1]").inspect
puts "a".sub("a", "\\\\").inspect
