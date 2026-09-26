# skip: frozen? is always true; MRI (frozen_string_literal) reports false for strings built at run time

# frozen_string_literal: true

# rbs_inline: enabled

puts "lit".frozen?.inspect
puts ("a" + "b").frozen?.inspect
puts "x".dup.frozen?.inspect
