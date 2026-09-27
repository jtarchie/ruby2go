# frozen_string_literal: true

# rbs_inline: enabled

puts "lit".frozen?.inspect
puts ("a" + "b").frozen?.inspect
puts "x".dup.frozen?.inspect
