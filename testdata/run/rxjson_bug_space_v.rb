# rbs_inline: enabled

puts ("\v" =~ /\s/).inspect, "a\vb".match?(/a\sb/), "\v".match?(/\S/)
