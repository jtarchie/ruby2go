# rbs_inline: enabled

puts ("a\n" =~ /^$/).inspect, ("a\n" =~ /^\z/).inspect, "a\n".match?(/\n^/)
