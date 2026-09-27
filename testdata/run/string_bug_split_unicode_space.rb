# rbs_inline: enabled

puts "a\u00a0b\u3000c d".split.inspect
puts "x\u00a0y".split(nil).size
