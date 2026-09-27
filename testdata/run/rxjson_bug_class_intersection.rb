# rbs_inline: enabled

puts "aebcd".match(/[a-z&&[^aeiou]]+/).inspect, "&]".match(/[a-z&&[^aeiou]]/).inspect
puts "b".match?(/[a[bc]]/), "b]".match(/[a[bc]]/).inspect
