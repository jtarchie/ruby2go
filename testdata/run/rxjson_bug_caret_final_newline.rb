# skip: ^ matches after a trailing newline at end of string on RE2 (?m) but not in MRI ("a\n" =~ /^$/ is nil)

# rbs_inline: enabled

puts ("a\n" =~ /^$/).inspect, ("a\n" =~ /^\z/).inspect, "a\n".match?(/\n^/)
